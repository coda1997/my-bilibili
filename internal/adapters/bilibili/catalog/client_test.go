package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/bilibili/transport"
	domain "my-bilibili/internal/catalog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body []byte) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}
func envelope(data any) []byte {
	b, _ := json.Marshal(map[string]any{"code": 0, "data": data})
	return b
}
func detailData() map[string]any {
	return map[string]any{
		"bvid": "BV0000000001", "aid": int64(1), "cid": int64(101), "title": "Original video", "owner": map[string]any{"name": "Creator"}, "desc": "", "pic": "https://cover.example.test/one.png",
		"rights":               map[string]any{"movie": 0, "pay": 0, "ugc_pay": 0, "is_stein_gate": 0, "ugc_pay_preview": 0, "arc_pay": 0},
		"is_chargeable_season": false, "is_upower_exclusive": false, "is_upower_preview": false,
		"pages": []any{map[string]any{"cid": 101, "page": 1, "part": "First", "duration": 30}, map[string]any{"cid": 102, "page": 2, "part": "Second", "duration": 45}},
	}
}
func searchData() map[string]any {
	return map[string]any{"page": 1, "numPages": 2, "numResults": 21, "result": []any{
		map[string]any{"type": "video", "bvid": "BV0000000001", "author": "Creator", "title": "A <em class=\"keyword\">Go &amp; 字</em> <b>literal</b>", "duration": "65:09", "pic": "https://cover.example.test/one.png"},
	}}
}

func testClient(t *testing.T, serve func(*http.Request) (*http.Response, error), covers http.RoundTripper) (*Client, *transport.Client) {
	t.Helper()
	service := transport.New(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/x/web-interface/nav":
			return response(envelope(map[string]any{"isLogin": false, "wbi_img": map[string]any{"img_url": "https://keys.example.test/0123456789abcdef0123456789abcdef.png", "sub_url": "https://keys.example.test/fedcba9876543210fedcba9876543210.png"}})), nil
		case "/":
			r := response([]byte("synthetic homepage"))
			r.Header.Add("Set-Cookie", "buvid3=SYNTHETIC; Domain=.bilibili.com; Path=/; Secure; Max-Age=3600")
			return r, nil
		}
		return serve(req)
	}))
	scope, err := service.Bind()
	if err != nil {
		t.Fatal(err)
	}
	c := New(scope, covers)
	t.Cleanup(func() {
		c.Close()
		service.Close()
		select {
		case <-service.Done():
		case <-time.After(5 * time.Second):
			t.Error("transport ownership not joined")
		}
	})
	return c, service
}

func TestDetailCanonicalIdentityPlainTextAndPrivateCover(t *testing.T) {
	data := detailData()
	data["desc"] = "line one\nline two\textra"
	c, _ := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" || req.URL.Host != "api.bilibili.com" || req.URL.Path != "/x/web-interface/view" || len(req.URL.Query()) != 1 {
			t.Error("detail route changed")
		}
		return response(envelope(data)), nil
	}, nil)
	for _, entry := range []domain.Entry{{Kind: domain.BVEntry, ID: "BV0000000001", Page: 2}, {Kind: domain.AVEntry, ID: "1"}} {
		video, err := c.Detail(context.Background(), entry)
		if err != nil || video.ID != "BV0000000001" || len(video.Parts) != 2 || video.Parts[0].ID != "101" || video.Parts[1].ID != "102" || video.Description != "line one line two extra" || video.CoverID == "" {
			t.Fatal("detail conversion", video, err)
		}
		if strings.Contains(fmt.Sprint(video), "https://") || strings.Contains(fmt.Sprintf("%#v", c), "cover.example") {
			t.Fatal("private cover location leaked")
		}
		video.Parts[0].Title = "mutated"
	}
}

func TestCatalogRejectsInvalidEntryBeforeHTTP(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(envelope(detailData())), nil
	}, nil)
	for _, entry := range []domain.Entry{{}, {Kind: domain.BVEntry, ID: "https://other.test/BV0000000001"}, {Kind: domain.AVEntry, ID: "0"}, {Kind: domain.BVEntry, ID: "BV0000000001", Page: -1}} {
		if _, err := c.Detail(context.Background(), entry); err != domain.Invalid {
			t.Fatal("invalid entry reached adapter", err)
		}
	}
	if _, err := c.Search(context.Background(), "bad\nquery", 1); err != domain.Invalid {
		t.Fatal(err)
	}
	if _, err := c.Search(context.Background(), "valid", 51); err != domain.Invalid {
		t.Fatal(err)
	}
	if _, err := c.Detail(nil, domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"}); err != domain.Invalid {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached HTTP")
	}
}

func TestDetailCriticalFieldsIdentityOrderAndContentClass(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   domain.Code
	}{
		{"bvid", func(d map[string]any) { d["bvid"] = "BV0000000002" }, domain.InvalidResponse},
		{"aid", func(d map[string]any) { d["aid"] = 0 }, domain.InvalidResponse},
		{"title-null", func(d map[string]any) { d["title"] = nil }, domain.InvalidResponse},
		{"title-empty", func(d map[string]any) { d["title"] = "" }, domain.InvalidResponse},
		{"title-limit", func(d map[string]any) { d["title"] = strings.Repeat("x", 513) }, domain.InvalidResponse},
		{"owner-null", func(d map[string]any) { d["owner"] = nil }, domain.InvalidResponse},
		{"desc-missing", func(d map[string]any) { delete(d, "desc") }, domain.InvalidResponse},
		{"desc-control", func(d map[string]any) { d["desc"] = "bad\x00" }, domain.InvalidResponse},
		{"desc-limit", func(d map[string]any) { d["desc"] = strings.Repeat("x", 65537) }, domain.InvalidResponse},
		{"parts-empty", func(d map[string]any) { d["pages"] = []any{} }, domain.InvalidResponse},
		{"cid", func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["cid"] = 0 }, domain.InvalidResponse},
		{"order", func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["page"] = 2 }, domain.InvalidResponse},
		{"duration", func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["duration"] = 0 }, domain.InvalidResponse},
		{"duration-limit", func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["duration"] = 604801 }, domain.InvalidResponse},
		{"duplicate-cid", func(d map[string]any) { d["pages"].([]any)[1].(map[string]any)["cid"] = 101 }, domain.InvalidResponse},
		{"contradictory-first-cid", func(d map[string]any) { d["cid"] = 102 }, domain.InvalidResponse},
		{"redirect", func(d map[string]any) { d["redirect_url"] = "https://unfollowed.example.test/" }, domain.Unsupported},
		{"redirect-type", func(d map[string]any) { d["redirect_url"] = 42 }, domain.InvalidResponse},
	}
	for _, name := range []string{"movie", "pay", "ugc_pay", "is_stein_gate", "ugc_pay_preview", "arc_pay"} {
		cases = append(cases, struct {
			name   string
			mutate func(map[string]any)
			want   domain.Code
		}{name, func(d map[string]any) { d["rights"].(map[string]any)[name] = 1 }, domain.Unsupported})
		cases = append(cases, struct {
			name   string
			mutate func(map[string]any)
			want   domain.Code
		}{name + "-unknown", func(d map[string]any) { delete(d["rights"].(map[string]any), name) }, domain.InvalidResponse})
	}
	for _, name := range []string{"is_chargeable_season", "is_upower_exclusive", "is_upower_preview"} {
		cases = append(cases, struct {
			name   string
			mutate func(map[string]any)
			want   domain.Code
		}{name, func(d map[string]any) { d[name] = true }, domain.Unsupported})
		cases = append(cases, struct {
			name   string
			mutate func(map[string]any)
			want   domain.Code
		}{name + "-type", func(d map[string]any) { d[name] = 0 }, domain.InvalidResponse})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data := detailData()
			test.mutate(data)
			c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
			video, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
			if err != test.want || video.ID != "" {
				t.Fatal("bad detail accepted/category", err)
			}
		})
	}
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(detailData())), nil }, nil)
	if _, err := c.Detail(context.Background(), domain.Entry{Kind: domain.AVEntry, ID: "2"}); err != domain.InvalidResponse {
		t.Fatal("AID mismatch accepted")
	}
}

func TestCatalogSearchUsesFixedRouteAndNormalizesHighlights(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		q := req.URL.Query()
		if req.URL.Path != "/x/web-interface/wbi/search/type" || q.Get("keyword") != "Go 中文" || q.Get("page") != "1" || q.Get("search_type") != "video" || q.Get("order") != "totalrank" || q.Get("w_rid") == "" {
			t.Error("search route/signing")
		}
		return response(envelope(searchData())), nil
	}, nil)
	page, err := c.Search(context.Background(), " Go 中文 ", 1)
	if err != nil || len(page.Videos) != 1 || page.Videos[0].Title != "A Go & 字 <b>literal</b>" || page.Videos[0].Duration != 3909*time.Second || page.Number != 1 || page.TotalResults != 21 || calls.Load() != 1 {
		t.Fatal("search conversion", page, err)
	}
	if strings.Contains(fmt.Sprint(page), "https://") || page.Videos[0].CoverID == "" {
		t.Fatal("cover identity was lost/leaked")
	}
}

func TestSearchRejectsMalformedAndKeepsLegitimateEmpty(t *testing.T) {
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["page"] = 2 }, func(d map[string]any) { d["numPages"] = -1 }, func(d map[string]any) { d["numResults"] = int64(1) << 32 }, func(d map[string]any) { d["result"] = nil },
		func(d map[string]any) { d["result"].([]any)[0].(map[string]any)["type"] = "live_room" },
		func(d map[string]any) { d["result"].([]any)[0].(map[string]any)["bvid"] = "invalid" },
		func(d map[string]any) { d["result"].([]any)[0].(map[string]any)["title"] = "bad\n" },
		func(d map[string]any) { d["result"].([]any)[0].(map[string]any)["author"] = nil },
		func(d map[string]any) { d["result"].([]any)[0].(map[string]any)["duration"] = "0:00" },
	} {
		data := searchData()
		mutate(data)
		c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
		if _, err := c.Search(context.Background(), "query", 1); err != domain.InvalidResponse {
			t.Fatal("malformed search became success", err)
		}
	}
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) {
		return response(envelope(map[string]any{"page": 1, "numPages": 0, "numResults": 0, "result": []any{}})), nil
	}, nil)
	page, err := c.Search(context.Background(), "empty", 1)
	if err != nil || page.Videos == nil || len(page.Videos) != 0 || page.TotalPages != 0 {
		t.Fatal("empty page not preserved", err)
	}
	for _, input := range []string{"", "1", "1:60", "1:2:3:4", "-1:02", "1:+02", "10080:01", "999999999999999999999999:00"} {
		raw, _ := json.Marshal(input)
		if _, ok := durationText(raw); ok {
			t.Fatal("invalid textual duration accepted", input)
		}
	}
	for input, want := range map[string]time.Duration{"01:02:03": 3723 * time.Second, "10080:00": 7 * 24 * time.Hour, "0:01": time.Second} {
		raw, _ := json.Marshal(input)
		got, ok := durationText(raw)
		if !ok || got != want {
			t.Fatal("valid textual duration", input, got, ok)
		}
	}
	if got := stripHighlights("</em> <em>literal</em> <em class=\"keyword\">unclosed"); got != "</em> <em>literal</em> <em class=\"keyword\">unclosed" {
		t.Fatal("literal title characters were removed")
	}
}

func TestScopeStructuralGuardsAndRevocationReachCatalog(t *testing.T) {
	for _, body := range [][]byte{
		[]byte("{\"code\":0,\"code\":0,\"data\":{}}"),
		[]byte("{\"code\":0,\"data\":{\"title\":\"bad\xff\"}}"),
	} {
		c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(body), nil }, nil)
		if _, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"}); err != domain.InvalidResponse {
			t.Fatal("transport guard bypassed", err)
		}
	}
	entered, release := make(chan struct{}), make(chan struct{})
	c, service := testClient(t, func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return response(envelope(detailData())), nil
	}, nil)
	done := make(chan error, 1)
	go func() {
		_, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
		done <- err
	}()
	<-entered
	service.Close()
	close(release)
	if err := <-done; err != domain.Canceled && err != domain.Closed {
		t.Fatal("retired scope published catalog", err)
	}
	if len(c.urls) != 0 {
		t.Fatal("retired detail installed cover")
	}
}

func TestAuthenticatedAPIStillCannotSendCookieToCover(t *testing.T) {
	var apiCookie atomic.Bool
	imageBytes := syntheticImage(t, "png", 16, 9)
	coverRT := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
			t.Error("authenticated material reached cover host")
		}
		return response(imageBytes), nil
	})
	_, service := testClient(t, func(req *http.Request) (*http.Response, error) {
		cookie, err := req.Cookie("SESSDATA")
		apiCookie.Store(err == nil && cookie.Value == "SYNTHETIC_SESSION")
		return response(envelope(detailData())), nil
	}, coverRT)
	material := account.NewMaterial([]byte("{\"version\":1,\"scope\":\"bilibili.com/\",\"cookies\":[{\"name\":\"SESSDATA\",\"value\":\"SYNTHETIC_SESSION\",\"expires\":0}]}"))
	if err := service.Activate(7, material); err != nil {
		t.Fatal(err)
	}
	scope, err := service.Bind()
	if err != nil {
		t.Fatal(err)
	}
	c := New(scope, coverRT)
	t.Cleanup(c.Close)
	video, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
	if err != nil || !apiCookie.Load() {
		t.Fatal("test did not exercise an authenticated API lease", err)
	}
	if _, err := c.Cover(context.Background(), video.CoverID); err != nil {
		t.Fatal(err)
	}
}

func TestDetailSharedPartBoundsAndOverflowBeforeDurationConversion(t *testing.T) {
	for _, count := range []int{1000, 1001} {
		data := detailData()
		pages := make([]any, 0, count)
		for i := 0; i < count; i++ {
			pages = append(pages, map[string]any{"cid": 101 + i, "page": i + 1, "part": "Part", "duration": 30})
		}
		data["pages"] = pages
		c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
		video, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
		if count == 1000 {
			if err != nil || len(video.Parts) != 1000 {
				t.Fatal("valid part boundary rejected", err)
			}
		} else if err != domain.InvalidResponse {
			t.Fatal("part bound not enforced", err)
		}
	}
	data := detailData()
	data["pages"].([]any)[0].(map[string]any)["duration"] = int64(18446744074)
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
	if _, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"}); err != domain.InvalidResponse {
		t.Fatal("overflowing seconds became a short valid duration", err)
	}
	for _, field := range []string{"title", "author"} {
		data := searchData()
		length := 513
		if field == "author" {
			length = 257
		}
		data["result"].([]any)[0].(map[string]any)[field] = strings.Repeat("x", length)
		c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
		if _, err := c.Search(context.Background(), "query", 1); err != domain.InvalidResponse {
			t.Fatal("oversized search text accepted", field, err)
		}
	}
}
