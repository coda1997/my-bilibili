package playback

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
)

func expectMediaError(t *testing.T, data map[string]any, want catalog.Code) {
	t.Helper()
	f := newFixture(t)
	f.media = data
	f.load(t)
	result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"})
	if err != want || result.Input.URL != "" || result.Input.AudioURL != "" || len(result.Qualities) != 0 {
		t.Fatal("invalid/unsupported media became a resolution or wrong safe category")
	}
	if f.count("/x/player/wbi/playurl") != 1 {
		t.Fatal("failed resolution retried or used another route")
	}
	assertSafe(t, err)
}

func TestOnlyActuallyReturnedSupportedVideoFamiliesBecomeChoices(t *testing.T) {
	cases := []struct{ id, width, height int }{{16, 640, 360}, {32, 854, 480}, {64, 1280, 720}, {74, 1280, 720}, {80, 1920, 1080}, {112, 1920, 1080}, {116, 1920, 1080}}
	f := newFixture(t)
	var rows []any
	for _, item := range cases {
		rows = append(rows, videoRow(item.id, item.width, item.height, 10000+item.id, fmt.Sprintf("https://video.example.test/%d.m4s", item.id)))
	}
	dash(f.media)["video"] = rows
	f.load(t)
	r := f.resolve(t, "")
	if r.Quality.ID != "116" || len(r.Qualities) != 7 {
		t.Fatal("fixed supported quality set incomplete or wrong default")
	}
	for _, item := range cases {
		r = f.resolve(t, app.QualityID(fmt.Sprint(item.id)))
		if r.Quality.ID != app.QualityID(fmt.Sprint(item.id)) || r.Quality.Width != item.width || r.Quality.Height != item.height {
			t.Fatal("supported explicit quality lost actual dimensions")
		}
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["id"] = 120 }, func(v map[string]any) { v["id"] = 125 }, func(v map[string]any) { v["id"] = 126 },
		func(v map[string]any) { v["codecid"] = 12 }, func(v map[string]any) { v["codecs"] = "hev1.1.6.L93" }, func(v map[string]any) { v["codecs"] = "av01.0.08M.08" }, func(v map[string]any) { v["codecs"] = "avc1.GARBAG" },
		func(v map[string]any) { v["mimeType"] = "video/webm" }, func(v map[string]any) { v["width"] = 1921 }, func(v map[string]any) { v["height"] = 1081 },
	} {
		data := mediaData()
		row := videoRow(80, 1920, 1080, 1000000, primaryVideo)
		mutate(row)
		dash(data)["video"] = []any{row}
		expectMediaError(t, data, catalog.Unsupported)
		f := newFixture(t)
		dash(f.media)["video"] = []any{row, videoRow(64, 1280, 720, 500000, "https://video.example.test/supported.m4s")}
		f.load(t)
		if result := f.resolve(t, ""); result.Quality.ID != "64" || len(result.Qualities) != 1 {
			t.Fatal("unsupported row displaced a supported choice")
		}
	}
}

func TestSilentAudioIsExplicitAndUnsupportedAudioIsNotDropped(t *testing.T) {
	for _, silent := range []any{nil, []any{}} {
		f := newFixture(t)
		dash(f.media)["audio"] = silent
		f.load(t)
		if result := f.resolve(t, ""); result.Input.AudioURL != "" {
			t.Fatal("explicit silent response acquired audio")
		}
	}
	missing := mediaData()
	delete(dash(missing), "audio")
	expectMediaError(t, missing, catalog.InvalidResponse)
	for _, mutate := range []func(map[string]any){func(a map[string]any) { a["id"] = 30300 }, func(a map[string]any) { a["codecs"] = "mp4a.40.5" }, func(a map[string]any) { a["codecs"] = "opus" }, func(a map[string]any) { a["mimeType"] = "audio/webm" }} {
		row := audioRow(30280, 900000, "https://audio.example.test/unsupported.m4s")
		mutate(row)
		data := mediaData()
		dash(data)["audio"] = []any{row}
		expectMediaError(t, data, catalog.Unsupported)
		f := newFixture(t)
		dash(f.media)["audio"] = []any{row, audioRow(30216, 64000, primaryAudio)}
		f.load(t)
		if result := f.resolve(t, ""); result.Input.AudioURL != primaryAudio {
			t.Fatal("unsupported high-bandwidth audio displaced AAC-LC")
		}
	}
	f := newFixture(t)
	dash(f.media)["audio"] = append(dash(f.media)["audio"].([]any), audioRow(30280, 192000, "https://audio.example.test/later-tie.m4s"))
	f.load(t)
	if f.resolve(t, "").Input.AudioURL != primaryAudio {
		t.Fatal("equal-bandwidth audio did not preserve first response index")
	}
}

func TestDocumentedAliasesMustAgreeAndCannotReplaceNull(t *testing.T) {
	for _, both := range []bool{false, true} {
		f := newFixture(t)
		for _, kind := range []string{"video", "audio"} {
			for _, item := range dash(f.media)[kind].([]any) {
				row := item.(map[string]any)
				row["base_url"], row["mime_type"] = row["baseUrl"], row["mimeType"]
				if !both {
					delete(row, "baseUrl")
					delete(row, "mimeType")
				}
			}
		}
		f.load(t)
		result := f.resolve(t, "")
		if result.Input.URL != primaryVideo || result.Input.AudioURL != primaryAudio {
			t.Fatal("documented alias changed media bytes")
		}
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["base_url"] = "https://video.example.test/different.m4s" }, func(v map[string]any) { v["mime_type"] = "video/webm" },
		func(v map[string]any) { v["base_url"] = v["baseUrl"]; v["baseUrl"] = nil }, func(v map[string]any) { v["mime_type"] = v["mimeType"]; v["mimeType"] = nil },
		func(v map[string]any) { v["baseUrl"] = 7 }, func(v map[string]any) { v["mimeType"] = true }, func(v map[string]any) { delete(v, "baseUrl"); v["backupUrl"] = []any{primaryVideo} },
	} {
		data := mediaData()
		mutate(videoAt(data, 2))
		expectMediaError(t, data, catalog.InvalidResponse)
	}
}

func TestCriticalMediaFieldsHaveExactNamesTypesAndPositiveBounds(t *testing.T) {
	mutations := []func(map[string]any){
		func(d map[string]any) { delete(d, "dash") }, func(d map[string]any) { d["dash"] = nil }, func(d map[string]any) { d["dash"] = []any{} }, func(d map[string]any) { d["Dash"] = d["dash"]; delete(d, "dash") },
		func(d map[string]any) { delete(dash(d), "video") }, func(d map[string]any) { dash(d)["video"] = nil }, func(d map[string]any) { dash(d)["video"] = map[string]any{} }, func(d map[string]any) { dash(d)["video"] = []any{nil} },
		func(d map[string]any) { dash(d)["audio"] = "not an array" }, func(d map[string]any) { dash(d)["audio"] = []any{nil} },
	}
	for _, name := range []string{"id", "bandwidth", "codecid", "width", "height", "codecs", "baseUrl", "mimeType"} {
		mutations = append(mutations, func(d map[string]any) { delete(videoAt(d, 2), name) }, func(d map[string]any) { videoAt(d, 2)[name] = nil })
	}
	for _, name := range []string{"id", "bandwidth", "codecid", "width", "height"} {
		mutations = append(mutations, func(d map[string]any) { videoAt(d, 2)[name] = "1" }, func(d map[string]any) { videoAt(d, 2)[name] = 1.5 })
	}
	for _, name := range []string{"id", "bandwidth", "width", "height"} {
		mutations = append(mutations, func(d map[string]any) { videoAt(d, 2)[name] = 0 }, func(d map[string]any) { videoAt(d, 2)[name] = -1 }, func(d map[string]any) { videoAt(d, 2)[name] = int64(math.MaxInt32) + 1 })
	}
	mutations = append(mutations,
		func(d map[string]any) { videoAt(d, 2)["codecid"] = -1 }, func(d map[string]any) { videoAt(d, 2)["codecs"] = "" }, func(d map[string]any) { videoAt(d, 2)["codecs"] = strings.Repeat("c", 129) }, func(d map[string]any) { videoAt(d, 2)["codecs"] = "avc1.640028\n" },
		func(d map[string]any) { videoAt(d, 2)["mimeType"] = strings.Repeat("m", 129) }, func(d map[string]any) { videoAt(d, 2)["mimeType"] = "video/mp4\n" }, func(d map[string]any) { videoAt(d, 2)["baseUrl"] = "" },
		func(d map[string]any) { v := videoAt(d, 2); v["ID"] = v["id"]; delete(v, "id") }, func(d map[string]any) { delete(audioAt(d, 1), "bandwidth") },
	)
	for _, mutate := range mutations {
		data := mediaData()
		mutate(data)
		expectMediaError(t, data, catalog.InvalidResponse)
	}
	f := newFixture(t)
	videoAt(f.media, 2)["bandwidth"] = int64(math.MaxInt32)
	videoAt(f.media, 2)["ID"] = 120
	f.load(t)
	if f.resolve(t, "").Input.URL != primaryVideo {
		t.Fatal("valid maximum bandwidth or unknown differently-cased field changed selection")
	}
}

func TestDASHStreamCountBoundsBeforeUnboundedConversion(t *testing.T) {
	for _, kind := range []string{"video", "audio"} {
		limit := 64
		if kind == "audio" {
			limit = 32
		}
		for _, count := range []int{limit, limit + 1} {
			f := newFixture(t)
			rows := make([]any, 0, count)
			for i := 0; i < count; i++ {
				if kind == "video" {
					rows = append(rows, videoRow(80, 1920, 1080, 1000000, primaryVideo))
				} else {
					rows = append(rows, audioRow(30280, 128000, primaryAudio))
				}
			}
			dash(f.media)[kind] = rows
			f.load(t)
			result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"})
			if count == limit {
				if err != nil || result.Quality.ID == "" {
					t.Fatal("exact stream count bound rejected")
				}
			} else if err != catalog.InvalidResponse || result.Input.URL != "" {
				t.Fatal("stream count exceeded reviewed bound")
			}
		}
	}
}

func TestMediaPairsUseSharedHTTPSPolicyWithoutBackupFallback(t *testing.T) {
	for _, bad := range []string{"http://video.example.test/plain.m4s", "https://user:password@video.example.test/private.m4s", "https://video.example.test/media#fragment", "https://video.example.test/media?bad=%0a", "https://video.example.test:0/media", "https://video.example.test:65536/media", "https://video.example.test/%zz", "file:///private/media", "https://video.example.test/" + strings.Repeat("a", 4097)} {
		for _, audio := range []bool{false, true} {
			data := mediaData()
			row := videoAt(data, 2)
			if audio {
				row = audioAt(data, 1)
			}
			row["baseUrl"] = bad
			row["backupUrl"] = []any{primaryVideo}
			expectMediaError(t, data, catalog.InvalidResponse)
		}
	}
	for _, good := range []string{"https://new-cdn.example.test/media?literal=%2f&x=1", "https://127.0.0.1:9443/media", "https://[::1]:9443/media"} {
		f := newFixture(t)
		videoAt(f.media, 2)["baseUrl"] = good
		f.load(t)
		if f.resolve(t, "").Input.URL != good {
			t.Fatal("valid HTTPS location was restricted or rewritten")
		}
	}
	data := mediaData()
	videoAt(data, 0)["baseUrl"] = "http://video.example.test/unselected.m4s"
	expectMediaError(t, data, catalog.InvalidResponse)
}
