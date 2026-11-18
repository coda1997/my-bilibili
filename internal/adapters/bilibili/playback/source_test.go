package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	player "my-bilibili/internal/playback"
)

const videoID = "BV0000000001"
const primaryVideo = "https://video.example.test:9443/preferred.m4s?sentinel=ORIGINAL_MEDIA&deadline=1&literal=%2F"
const primaryAudio = "https://audio.example.test/preferred.m4s?sentinel=ORIGINAL_AUDIO"
const privateMarker = "PRIVATE_SYNTHETIC_MEDIA"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type body struct {
	io.Reader
	closed atomic.Int32
}

func (b *body) Close() error { b.closed.Add(1); return nil }

type requestRecord struct {
	path, part, quality string
	deadline            time.Time
}
type fixture struct {
	service     *transport.Client
	scope       *transport.Scope
	details     *catalogapi.Client
	source      *Source
	detail      map[string]any
	media       map[string]any
	mediaStatus int
	serveMedia  func(*http.Request) ([]byte, error)
	mu          sync.Mutex
	requests    []requestRecord
	bodies      []*body
}

func envelope(data any) []byte {
	value, _ := json.Marshal(map[string]any{"code": 0, "data": data})
	return value
}
func detailData() map[string]any {
	return map[string]any{
		"bvid": videoID, "aid": 1, "cid": 101, "title": "Original synthetic video", "owner": map[string]any{"name": "Synthetic Creator"}, "desc": "", "pic": "https://cover.example.test/unused.png",
		"rights": map[string]any{"movie": 0, "pay": 0, "ugc_pay": 0, "is_stein_gate": 0, "ugc_pay_preview": 0, "arc_pay": 0}, "is_chargeable_season": false, "is_upower_exclusive": false, "is_upower_preview": false,
		"pages": []any{map[string]any{"cid": 101, "page": 1, "part": "First", "duration": 120}, map[string]any{"cid": 102, "page": 2, "part": "Second", "duration": 180}},
	}
}
func videoRow(id, width, height, bandwidth int, location string) map[string]any {
	return map[string]any{"id": id, "width": width, "height": height, "bandwidth": bandwidth, "codecid": 7, "codecs": "avc1.640028", "mimeType": "video/mp4", "baseUrl": location}
}
func audioRow(id, bandwidth int, location string) map[string]any {
	return map[string]any{"id": id, "bandwidth": bandwidth, "codecid": 0, "codecs": "mp4a.40.2", "mimeType": "audio/mp4", "baseUrl": location}
}
func mediaData() map[string]any {
	return map[string]any{
		"quality": 116, "accept_quality": []any{116, 112, 80, 64}, "accept_description": []any{"<b>" + privateMarker + "</b>"}, "accept_format": "advertised,untrusted",
		"support_formats": []any{map[string]any{"quality": 116, "format": "<b>" + privateMarker + "</b>", "new_description": privateMarker, "codecs": []any{"avc1.640028", "hev1.1.6.L93"}}},
		"headers":         map[string]any{"Cookie": privateMarker, "Authorization": privateMarker},
		"dash": map[string]any{"video": []any{videoRow(64, 1280, 720, 2000000, "https://video.example.test/720.m4s"), videoRow(80, 1920, 1080, 5000000, "https://video.example.test/lower.m4s"), videoRow(80, 1920, 1080, 6000000, primaryVideo), videoRow(80, 1920, 1080, 6000000, "https://video.example.test/later-tie.m4s")},
			"audio": []any{audioRow(30216, 64000, "https://audio.example.test/low.m4s"), audioRow(30280, 192000, primaryAudio), audioRow(30232, 128000, "https://audio.example.test/middle.m4s")}},
	}
}
func dash(data map[string]any) map[string]any { return data["dash"].(map[string]any) }
func videoAt(data map[string]any, index int) map[string]any {
	return dash(data)["video"].([]any)[index].(map[string]any)
}
func audioAt(data map[string]any, index int) map[string]any {
	return dash(data)["audio"].([]any)[index].(map[string]any)
}
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{detail: detailData(), media: mediaData()}
	f.service = transport.New(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "api.bilibili.com" || req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" || req.Header.Get("Referer") != "https://www.bilibili.com/" || req.Header.Get("User-Agent") != "my-bilibili-p0/1" {
			t.Error("request left fixed anonymous test transport")
		}
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) > 10*time.Second {
			t.Error("request deadline absent or extended")
		}
		f.mu.Lock()
		f.requests = append(f.requests, requestRecord{req.URL.Path, req.URL.Query().Get("cid"), req.URL.Query().Get("qn"), deadline})
		f.mu.Unlock()
		var data []byte
		status := http.StatusOK
		switch req.URL.Path {
		case "/x/web-interface/view":
			data = envelope(f.detail)
		case "/x/web-interface/nav":
			data = []byte(`{"code":-101,"data":{"isLogin":false,"wbi_img":{"img_url":"https://keys.example.test/0123456789abcdef0123456789abcdef.png","sub_url":"https://keys.example.test/fedcba9876543210fedcba9876543210.png"}}}`)
		case "/x/player/wbi/playurl":
			if f.mediaStatus != 0 {
				status = f.mediaStatus
			}
			query := req.URL.Query()
			if query.Get("bvid") != videoID || query.Get("fnver") != "0" || query.Get("fnval") != "16" || query.Get("wts") == "" || query.Get("w_rid") == "" || len(query) != 7 {
				t.Error("media request was not the fixed signed DASH route")
			}
			if f.serveMedia != nil {
				var err error
				data, err = f.serveMedia(req)
				if err != nil {
					return nil, err
				}
			} else {
				data = envelope(f.media)
			}
		default:
			t.Error("unplanned service, image or fallback request")
			return nil, errors.New("unplanned synthetic request")
		}
		b := &body{Reader: bytes.NewReader(data)}
		f.mu.Lock()
		f.bodies = append(f.bodies, b)
		f.mu.Unlock()
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: b}, nil
	}))
	var err error
	f.scope, err = f.service.Bind()
	if err != nil {
		t.Fatal("test scope failed")
	}
	f.details = catalogapi.New(f.scope, roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("media source fetched a cover")
		return nil, errors.New("unplanned cover")
	}))
	f.source, err = NewSource(f.scope, f.details, catalog.Entry{Kind: catalog.BVEntry, ID: videoID})
	if err != nil {
		t.Fatal("test source construction failed")
	}
	t.Cleanup(func() {
		f.details.Close()
		f.service.Close()
		select {
		case <-f.service.Done():
		case <-time.After(3 * time.Second):
			t.Error("transport preparation not joined")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, body := range f.bodies {
			if body.closed.Load() != 1 {
				t.Error("service response body not closed")
			}
		}
	})
	return f
}
func (f *fixture) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if r.path == path {
			n++
		}
	}
	return n
}
func (f *fixture) load(t *testing.T) {
	t.Helper()
	if _, err := f.source.Detail(context.Background()); err != nil {
		t.Fatal("synthetic detail failed")
	}
}
func (f *fixture) resolve(t *testing.T, quality app.QualityID) app.Resolution {
	t.Helper()
	r, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101", Quality: quality})
	if err != nil {
		t.Fatal("synthetic media resolution failed")
	}
	return r
}
func assertSafe(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		if _, ok := err.(catalog.Code); !ok || strings.Contains(fmt.Sprintf("%+v", err), privateMarker) {
			t.Fatal("raw media error escaped")
		}
	}
}

func TestSourceConstructorAndMembershipBeforeMediaAccess(t *testing.T) {
	f := newFixture(t)
	if len(f.requests) != 0 {
		t.Fatal("constructor performed network work")
	}
	if _, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"}); err != catalog.Invalid || len(f.requests) != 0 {
		t.Fatal("media resolved before matching detail")
	}
	for _, entry := range []catalog.Entry{{}, {Kind: catalog.BVEntry, ID: "bad"}, {Kind: catalog.BVEntry, ID: videoID, Page: -1}, {Kind: catalog.BVEntry, ID: videoID, Page: 1001}, {Kind: catalog.AVEntry, ID: "0"}, {Kind: catalog.BVEntry, ID: "https://www.bilibili.com/video/" + videoID}} {
		if _, err := NewSource(f.scope, f.details, entry); err != catalog.Invalid {
			t.Fatal("invalid entry accepted by pure source constructor")
		}
	}
	f.load(t)
	for _, part := range []catalog.PartID{"", "999", "00101", "PRIVATE_SYNTHETIC_MEDIA"} {
		if _, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: part}); err != catalog.Invalid {
			t.Fatal("unestablished CID accepted")
		}
	}
	for _, quality := range []app.QualityID{"120", "064", "PRIVATE_SYNTHETIC_MEDIA"} {
		if _, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101", Quality: quality}); err != catalog.QualityUnavailable {
			t.Fatal("unreturned quality identifier forwarded")
		}
	}
	if f.count("/x/web-interface/view") != 1 || f.count("/x/player/wbi/playurl") != 0 || f.count("/x/web-interface/nav") != 0 {
		t.Fatal("rejected input initiated preparation/media")
	}
}

func TestSourceRequiresExactLiveCatalogScope(t *testing.T) {
	f := newFixture(t)
	other := newFixture(t)
	if f.scope.State() != other.scope.State() {
		t.Fatal("fixture did not exercise equal numeric scope state")
	}
	if _, err := NewSource(f.scope, other.details, catalog.Entry{Kind: catalog.BVEntry, ID: videoID}); err != catalog.Invalid {
		t.Fatal("numerically equal but different catalog scope accepted")
	}
	if _, err := NewSource(nil, f.details, catalog.Entry{Kind: catalog.BVEntry, ID: videoID}); err != catalog.Invalid {
		t.Fatal("nil scope accepted")
	}
	if _, err := NewSource(f.scope, nil, catalog.Entry{Kind: catalog.BVEntry, ID: videoID}); err != catalog.Invalid {
		t.Fatal("nil catalog accepted")
	}
	f.load(t)
	f.details.Close()
	if _, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"}); err != catalog.Canceled || f.count("/x/player/wbi/playurl") != 0 {
		t.Fatal("closed catalog owner permitted media resolution")
	}
	other.service.Close()
	if _, err := other.source.Detail(context.Background()); err != catalog.Canceled || len(other.requests) != 0 {
		t.Fatal("retired scope made detail request")
	}
}

func TestDetailEstablishesCanonicalMembershipOnceAndCopies(t *testing.T) {
	f := newFixture(t)
	var err error
	f.source, err = NewSource(f.scope, f.details, catalog.Entry{Kind: catalog.AVEntry, ID: "1", Page: 2})
	if err != nil {
		t.Fatal("AV source rejected")
	}
	first, err := f.source.Detail(context.Background())
	if err != nil || first.ID != videoID || len(first.Parts) != 2 {
		t.Fatal("detail did not canonicalize requested video")
	}
	first.Parts[0].ID = "999"
	first.Parts[0].Title = privateMarker
	second, err := f.source.Detail(context.Background())
	if err != nil || second.Parts[0].ID != "101" || second.Parts[0].Title == privateMarker || f.count("/x/web-interface/view") != 1 {
		t.Fatal("detail membership aliased caller or was re-established")
	}
	r := f.resolve(t, "")
	if r.Quality.ID != "80" {
		t.Fatal("canonical member did not resolve")
	}
	for _, value := range []any{f.source, *f.source} {
		if fmt.Sprintf("%#v", value) != "[private media source]" {
			t.Fatal("media source exposed retained identifiers")
		}
		data, _ := json.Marshal(value)
		if string(data) != "null" {
			t.Fatal("media source serialized private state")
		}
	}
}

func TestMatchingSupportedDetailIsRequiredBeforeResolution(t *testing.T) {
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["bvid"] = "BV0000000002" },
		func(d map[string]any) { d["rights"].(map[string]any)["movie"] = 1 },
		func(d map[string]any) { d["rights"].(map[string]any)["pay"] = 1 },
		func(d map[string]any) { d["rights"].(map[string]any)["ugc_pay"] = 1 },
		func(d map[string]any) { d["rights"].(map[string]any)["is_stein_gate"] = 1 },
		func(d map[string]any) { d["rights"].(map[string]any)["ugc_pay_preview"] = 1 },
		func(d map[string]any) { d["rights"].(map[string]any)["arc_pay"] = 1 },
		func(d map[string]any) { d["is_chargeable_season"] = true },
		func(d map[string]any) { d["is_upower_exclusive"] = true },
		func(d map[string]any) { d["is_upower_preview"] = true },
		func(d map[string]any) { d["redirect_url"] = "https://unsupported.example.test/" },
	} {
		f := newFixture(t)
		mutate(f.detail)
		if _, err := f.source.Detail(context.Background()); err == nil {
			t.Fatal("mismatched/unsupported detail became a grant")
		}
		if _, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"}); err != catalog.Invalid || f.count("/x/player/wbi/playurl") != 0 {
			t.Fatal("failed detail established media membership")
		}
		f.detail = detailData()
		f.load(t)
		f.resolve(t, "64")
		if f.count("/x/web-interface/view") != 2 {
			t.Fatal("explicit detail retry did not establish a fresh grant")
		}
	}
}

func TestReturnedQualitiesDeduplicationAndAudioAreTruthful(t *testing.T) {
	f := newFixture(t)
	f.load(t)
	r := f.resolve(t, "")
	if r.Quality.ID != "80" || r.Input.URL != primaryVideo || r.Input.AudioURL != primaryAudio || len(r.Qualities) != 2 || r.Qualities[0].ID != "80" || r.Qualities[1].ID != "64" {
		t.Fatal("advertised metadata, response order or duplicate entry determined selection")
	}
	if r.Quality.Label != "1080P（1920×1080）" || r.Quality.Width != 1920 || r.Quality.Height != 1080 || strings.Contains(fmt.Sprint(r.Qualities), privateMarker) {
		t.Fatal("quality label did not use fixed names and actual dimensions")
	}
	if len(r.Input.Headers) != 2 || r.Input.Headers["Referer"] != "https://www.bilibili.com/" || r.Input.Headers["User-Agent"] != "my-bilibili-p0/1" || (player.HTTPSPolicy{}).Validate(r.Input) != nil || r.Input.Validate() == nil {
		t.Fatal("media input escaped explicit HTTPS/public-header policy")
	}
	f.mu.Lock()
	last := f.requests[len(f.requests)-1]
	f.mu.Unlock()
	if last.quality != "64" || last.part != "101" {
		t.Fatal("default request was not qn=64 for the member CID")
	}
	r = f.resolve(t, "64")
	if r.Quality.ID != "64" || r.Input.URL != "https://video.example.test/720.m4s" {
		t.Fatal("explicit returned quality was not selected exactly")
	}
	if result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101", Quality: "116"}); err != catalog.QualityUnavailable || result.Input.URL != "" {
		t.Fatal("advertised absent quality silently substituted")
	}
	f.mu.Lock()
	last = f.requests[len(f.requests)-1]
	f.mu.Unlock()
	if last.quality != "116" {
		t.Fatal("explicit request did not send the selected quality code")
	}
}

func TestResolutionIsFreshCopiedAndUsesOnlyFiveMinuteClientCutoff(t *testing.T) {
	f := newFixture(t)
	f.load(t)
	received := time.Unix(1700000100, 0)
	f.source.now = func() time.Time { return received }
	first := f.resolve(t, "")
	if !first.ExpiresAt.Equal(received.Add(5*time.Minute)) || !first.RefreshOnRetry {
		t.Fatal("resolution inferred URL expiry or omitted fresh retry")
	}
	first.Input.Headers["Cookie"] = privateMarker
	first.Qualities[0].ID = "999"
	first.Qualities[0].Label = privateMarker
	videoAt(f.media, 2)["baseUrl"] = "https://video.example.test/fresh.m4s?deadline=9999999999"
	received = received.Add(time.Second)
	second := f.resolve(t, "")
	if second.Input.URL == first.Input.URL || second.Input.Headers["Cookie"] != "" || second.Qualities[0].ID != "80" || strings.Contains(second.Qualities[0].Label, privateMarker) || !second.RefreshOnRetry || !second.ExpiresAt.Equal(received.Add(5*time.Minute)) || f.count("/x/player/wbi/playurl") != 2 || f.count("/x/web-interface/nav") != 1 {
		t.Fatal("retry reused locator/mutable state or extended the fixed reuse cutoff")
	}
}
