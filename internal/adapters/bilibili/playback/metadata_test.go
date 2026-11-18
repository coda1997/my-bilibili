package playback

import (
	"context"
	"strings"
	"testing"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
)

func TestAdvertisedQualityMetadataIsBoundedButNeverAvailability(t *testing.T) {
	for _, name := range []string{"accept_quality", "accept_description", "support_formats"} {
		for _, count := range []int{16, 17} {
			f := newFixture(t)
			values := make([]any, 0, count)
			for range count {
				var value any = 116
				if name == "accept_description" {
					value = "<b>" + privateMarker + "</b>"
				}
				if name == "support_formats" {
					value = map[string]any{"quality": 116, "new_description": privateMarker, "codecs": nil}
				}
				values = append(values, value)
			}
			f.media[name] = values
			f.load(t)
			result, err := f.source.Resolve(context.Background(), app.ResolveRequest{Part: "101"})
			if count == 16 {
				if err != nil || result.Quality.ID != "80" || len(result.Qualities) != 2 {
					t.Fatal("bounded advertised metadata changed actual availability")
				}
			} else if err != catalog.InvalidResponse || result.Input.URL != "" {
				t.Fatal("advertised quality metadata exceeded bound")
			}
		}
	}
	for _, name := range []string{"accept_quality", "accept_description", "support_formats"} {
		f := newFixture(t)
		f.media[name] = nil
		delete(f.media, "quality")
		f.load(t)
		if f.resolve(t, "").Quality.ID != "80" {
			t.Fatal("optional absent advertising hid returned streams")
		}
	}
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["quality"] = nil }, func(d map[string]any) { d["quality"] = "80" }, func(d map[string]any) { d["accept_quality"] = []any{nil} }, func(d map[string]any) { d["accept_quality"] = []any{"80"} },
		func(d map[string]any) { d["accept_description"] = []any{7} }, func(d map[string]any) { d["accept_description"] = []any{strings.Repeat("x", 257)} }, func(d map[string]any) { d["accept_description"] = []any{"bad\nmetadata"} },
		func(d map[string]any) { d["accept_format"] = strings.Repeat("x", 4097) }, func(d map[string]any) { d["support_formats"] = []any{nil} }, func(d map[string]any) { d["support_formats"] = []any{map[string]any{"quality": nil}} },
		func(d map[string]any) {
			d["support_formats"] = []any{map[string]any{"quality": 80, "new_description": strings.Repeat("x", 257)}}
		}, func(d map[string]any) {
			d["support_formats"] = []any{map[string]any{"quality": 80, "codecs": []any{strings.Repeat("c", 129)}}}
		},
		func(d map[string]any) {
			d["support_formats"] = []any{map[string]any{"quality": 80, "codecs": make([]any, 17)}}
		},
	} {
		data := mediaData()
		mutate(data)
		expectMediaError(t, data, catalog.InvalidResponse)
	}
	f := newFixture(t)
	f.media["accept_description"] = []any{strings.Repeat("x", 256)}
	f.media["accept_format"] = strings.Repeat("x", 4096)
	f.media["support_formats"] = []any{map[string]any{"quality": 80, "new_description": strings.Repeat("d", 256), "codecs": []any{strings.Repeat("c", 128)}}}
	f.load(t)
	if f.resolve(t, "").Quality.ID != "80" {
		t.Fatal("exact metadata string bounds rejected")
	}
}
