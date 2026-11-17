package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func TestCatalogMetadataBoundariesAtAcceptance(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*catalog.Video)
		valid  bool
	}{
		{"empty-description-and-cover", func(v *catalog.Video) { v.Description, v.CoverID = "", "" }, true},
		{"maximum-text-and-duration", func(v *catalog.Video) {
			v.Title, v.Creator, v.Description = strings.Repeat("t", 512), strings.Repeat("c", 256), strings.Repeat("d", 65536)
			v.Parts[0].Title, v.Parts[0].Duration = strings.Repeat("p", 512), 7*24*time.Hour
		}, true},
		{"maximum-parts-preserve-order", func(v *catalog.Video) {
			for i := len(v.Parts); i < 1000; i++ {
				v.Parts = append(v.Parts, catalog.Part{ID: catalog.PartID(fmt.Sprintf("p%d", i+1)), Title: "Part", Duration: time.Second})
			}
		}, true},
		{"title-byte-bound", func(v *catalog.Video) { v.Title = strings.Repeat("字", 171) }, false},
		{"creator-byte-bound", func(v *catalog.Video) { v.Creator = strings.Repeat("c", 257) }, false},
		{"description-byte-bound", func(v *catalog.Video) { v.Description = strings.Repeat("d", 65537) }, false},
		{"part-title-byte-bound", func(v *catalog.Video) { v.Parts[0].Title = strings.Repeat("p", 513) }, false},
		{"too-many-parts", func(v *catalog.Video) {
			for i := len(v.Parts); i < 1001; i++ {
				v.Parts = append(v.Parts, catalog.Part{ID: catalog.PartID(fmt.Sprintf("p%d", i+1)), Title: "Part", Duration: time.Second})
			}
		}, false},
		{"duration-upper-bound", func(v *catalog.Video) { v.Parts[0].Duration = 7*24*time.Hour + 1 }, false},
		{"title-control", func(v *catalog.Video) { v.Title = "Title\n" }, false},
		{"creator-control", func(v *catalog.Video) { v.Creator = "Creator\u0085" }, false},
		{"description-control", func(v *catalog.Video) { v.Description = "Description\t" }, false},
		{"cover-control", func(v *catalog.Video) { v.CoverID = "cover\x00" }, false},
		{"video-identity-control", func(v *catalog.Video) { v.ID = "video\n" }, false},
		{"part-identity-control", func(v *catalog.Video) { v.Parts[0].ID = "part\n" }, false},
		{"part-title-control", func(v *catalog.Video) { v.Parts[0].Title = "Part\r" }, false},
		{"title-utf8", func(v *catalog.Video) { v.Title = "title\xff" }, false},
		{"description-utf8", func(v *catalog.Video) { v.Description = "description\xff" }, false},
		{"cover-utf8", func(v *catalog.Video) { v.CoverID = "cover\xff" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t)
			h.source.edit(func(source *fakeSource) { test.change(&source.video) })
			h.start(t)
			state := h.s.State()
			if test.valid {
				if state.DetailPhase != DetailReady || state.DetailError != catalog.None || len(h.opener.inputs) != 1 {
					t.Fatal("valid bounded metadata was rejected", state.DetailPhase, state.DetailError, len(h.opener.inputs))
				}
				for i, part := range state.Video.Parts {
					if part.ID != h.source.video.Parts[i].ID {
						t.Fatal("detail order changed")
					}
				}
			} else if state.DetailPhase != DetailFailed || state.DetailError != catalog.Invalid || len(state.Video.Parts) != 0 || len(h.opener.inputs) != 0 {
				t.Fatal("invalid metadata reached the application", state.DetailPhase, state.DetailError, len(h.opener.inputs))
			}
		})
	}
}

func TestContentCodesAtSourceBoundary(t *testing.T) {
	for _, code := range []catalog.Code{catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network,
		catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired,
		catalog.VerificationRequired, catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed} {
		t.Run(string(code), func(t *testing.T) {
			h := newHarness(t)
			h.source.edit(func(source *fakeSource) { source.detailErr = fmt.Errorf("private-sentinel: %w", code) })
			h.start(t)
			state := h.s.State()
			if state.DetailError != code || state.DetailPhase != DetailFailed || len(h.opener.inputs) != 0 || strings.Contains(fmt.Sprint(state), "private-sentinel") {
				t.Fatal("safe source category was lost or exposed its wrapper", state)
			}
		})
	}
	for _, test := range []struct {
		err  error
		want catalog.Code
	}{
		{playback.Invalid, catalog.Invalid}, {playback.Unsupported, catalog.Unsupported},
		{playback.Canceled, catalog.Canceled}, {playback.Timeout, catalog.Timeout},
		{playback.Media, catalog.Unavailable}, {playback.IsClosed, catalog.Closed},
		{context.Canceled, catalog.Canceled}, {context.DeadlineExceeded, catalog.Timeout},
		{catalog.Code("private-sentinel"), catalog.Unavailable},
		{playback.ErrorCode("private-sentinel"), catalog.Unavailable},
		{errors.New("private-sentinel"), catalog.Unavailable},
		{catalog.None, catalog.Unavailable},
	} {
		t.Run(fmt.Sprintf("%T-%s", test.err, test.err), func(t *testing.T) {
			h := newHarness(t)
			h.source.edit(func(source *fakeSource) { source.detailErr = fmt.Errorf("private-sentinel: %w", test.err) })
			h.start(t)
			if state := h.s.State(); state.DetailError != test.want || state.DetailPhase != DetailFailed || strings.Contains(fmt.Sprint(state), "private-sentinel") {
				t.Fatal("source error did not cross the fixed content mapping", state)
			}
		})
	}
}

func TestNativeErrorsRetainPlaybackCategory(t *testing.T) {
	h := newHarness(t)
	h.opener.err = playback.Unsupported
	h.start(t)
	state := h.s.State()
	if state.Playback.State != playback.Failed || state.Playback.Error != playback.Unsupported || state.DetailError != catalog.None || state.SelectionError != catalog.None {
		t.Fatal("native failure was recategorized as source failure", state)
	}
}

func TestResolutionChoicesTransferAndRemainOwned(t *testing.T) {
	h, source := controlledHarness(t, false)
	if qualities := h.s.State().Qualities; qualities == nil || len(qualities) != 0 {
		t.Fatal("initial choices are not a complete empty array")
	}
	call := initialResolve(t, h, source)
	resolution := testResolution(playback.Input{FilePath: "/fixture.mp4"}, time.Time{})
	resolution.Qualities = append(resolution.Qualities, Quality{ID: "other-fixture", Label: "Other fixture", Width: 320, Height: 180})
	resolution.Quality = resolution.Qualities[1]
	call.reply <- sourceReply{resolution: resolution}
	resultReady(t, h.s)
	resolution.Qualities[1].Label = "source mutation"
	h.s.Tick()
	state := h.s.State()
	if state.PlaybackQuality != "other-fixture" || state.SelectedQuality != state.PlaybackQuality || len(state.Qualities) != 2 || state.Qualities[1].Label != "Other fixture" || len(h.opener.inputs) != 1 {
		t.Fatal("accepted choices differ from the owned source result", state)
	}
	state.Qualities[0].Label = "state mutation"
	if h.s.State().Qualities[0].Label != "Fixture" {
		t.Fatal("State exposes mutable accepted choices")
	}
	requireError(t, h.s.SelectPart("p2"), nil)
	if state := h.s.State(); state.PlaybackQuality != "other-fixture" || state.Qualities[1].Label != "Other fixture" {
		t.Fatal("pending part erased accepted choices")
	}
	requireError(t, h.s.SelectPart("p1"), nil)
	if state := h.s.State(); state.SelectedQuality != "other-fixture" || state.PlaybackQuality != "other-fixture" {
		t.Fatal("accepted-part reselection lost its actual choice", state)
	}
	requireError(t, h.s.Close(), nil)
	if h.s.State().Qualities == nil {
		t.Fatal("closed state lost its complete choices array")
	}
}

func TestMalformedResolutionChoicesNeverOpen(t *testing.T) {
	for name, mutate := range map[string]func(*Resolution){
		"missing": func(r *Resolution) { r.Qualities = nil },
		"empty":   func(r *Resolution) { r.Qualities = []Quality{} },
		"too-many": func(r *Resolution) {
			for len(r.Qualities) <= 16 {
				r.Qualities = append(r.Qualities, Quality{ID: QualityID(fmt.Sprintf("q%d", len(r.Qualities))), Label: "Fixture", Width: 640, Height: 360})
			}
		},
		"empty-id":                    func(r *Resolution) { r.Qualities[0].ID = ""; r.Quality = r.Qualities[0] },
		"duplicate":                   func(r *Resolution) { r.Qualities = append(r.Qualities, r.Qualities[0]) },
		"no-chosen-member":            func(r *Resolution) { r.Quality.ID = "missing" },
		"contradictory-chosen-member": func(r *Resolution) { r.Quality.Width++ },
	} {
		t.Run(name, func(t *testing.T) {
			h, source := controlledHarness(t, false)
			call := initialResolve(t, h, source)
			resolution := testResolution(playback.Input{FilePath: "/fixture.mp4"}, time.Time{})
			mutate(&resolution)
			call.reply <- sourceReply{resolution: resolution}
			resultReady(t, h.s)
			h.s.Tick()
			state := h.s.State()
			if state.SelectionError != catalog.InvalidResponse || state.PlaybackPart != "" || state.PlaybackQuality != "" || len(state.Qualities) != 0 || len(h.opener.inputs) != 0 {
				t.Fatal("malformed choices opened media or acquired a fabricated default", state)
			}
		})
	}
}

func TestResolutionAcceptsSixteenDistinctChoices(t *testing.T) {
	h, source := controlledHarness(t, false)
	call := initialResolve(t, h, source)
	resolution := testResolution(playback.Input{FilePath: "/fixture.mp4"}, time.Time{})
	for len(resolution.Qualities) < 16 {
		resolution.Qualities = append(resolution.Qualities, Quality{ID: QualityID(fmt.Sprintf("q%d", len(resolution.Qualities))), Label: "Fixture", Width: 640, Height: 360})
	}
	resolution.Quality = resolution.Qualities[15]
	call.reply <- sourceReply{resolution: resolution}
	resultReady(t, h.s)
	h.s.Tick()
	if state := h.s.State(); len(state.Qualities) != 16 || state.PlaybackQuality != "q15" || len(h.opener.inputs) != 1 {
		t.Fatal("the supported choice limit rejected a complete source result", state)
	}
}
