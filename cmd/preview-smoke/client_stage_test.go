package main

import (
	"context"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func TestClientProgressCannotUseAnotherGenerationNavigationOrMissingPaint(t *testing.T) {
	before := completeClientObservation()
	after := before
	after.Playback.State, after.DesiredPlaying = playback.Playing, true
	after.Playback.Position += 300 * time.Millisecond
	if !clientProgressed(before, after) {
		t.Fatal("complete progress rejected")
	}
	for _, change := range []func(*clientObserved){
		func(o *clientObserved) { o.Navigation++ },
		func(o *clientObserved) { o.Playback.Generation++ },
		func(o *clientObserved) { o.Playback.Position-- },
		func(o *clientObserved) { o.PicturePresented = false },
		func(o *clientObserved) { o.DesiredPlaying = false },
		func(o *clientObserved) { o.Resuming = true },
		func(o *clientObserved) { o.Playback.State = playback.Paused },
	} {
		o := after
		change(&o)
		if clientProgressed(before, o) {
			t.Fatal("stale, insufficient or no-picture progress accepted")
		}
	}
}

func TestClientHistoryEvidenceNeedsTheCurrentSuccessfulSave(t *testing.T) {
	o := completeClientObservation()
	o.HistoryCount, o.HistoryRevision, o.HistorySavedRevision = 1, 3, 3
	if !clientHistorySaved(o, 1) {
		t.Fatal("complete saved state rejected")
	}
	for _, change := range []func(*clientObserved){
		func(o *clientObserved) { o.HistoryCount = 0 },
		func(o *clientObserved) { o.HistorySavedRevision-- },
		func(o *clientObserved) { o.HistoryBusy = true },
		func(o *clientObserved) { o.HistoryDirty = true },
		func(o *clientObserved) { o.HistoryError = history.Uncertain },
		func(o *clientObserved) { o.HistoryPhase = history.Failed },
	} {
		bad := o
		change(&bad)
		if clientHistorySaved(bad, 1) {
			t.Fatal("unconfirmed history became restart evidence")
		}
	}
}

func TestClientRevocationRetainsPositiveOwnershipAsFailure(t *testing.T) {
	o := closedClientSample()
	o.Closing, o.Closed = false, false
	o.AccountPhase = account.Expired
	o.PlaybackPart, o.SelectedPart, o.Parts, o.Qualities, o.SelectedQuality, o.PlaybackQuality = 0, 0, 0, 0, 0, 0
	o.Playback.State = playback.Idle
	if !clientRetired(o) {
		t.Fatal("complete retirement rejected")
	}
	for name, change := range map[string]func(*clientObserved){
		"auth":        func(o *clientObserved) { o.Authenticated = true },
		"epoch":       func(o *clientObserved) { o.AccountEpoch = 1 },
		"part":        func(o *clientObserved) { o.PlaybackPart = 1 },
		"state":       func(o *clientObserved) { o.Playback.State = playback.Playing },
		"core":        func(o *clientObserved) { o.Resources.Cores = 1 },
		"renderer":    func(o *clientObserved) { o.Resources.Renderers = 1 },
		"worker":      func(o *clientObserved) { o.Resources.Workers = 1 },
		"video":       func(o *clientObserved) { o.VideoTrack = true },
		"audio":       func(o *clientObserved) { o.AudioTrack = true },
		"picture":     func(o *clientObserved) { o.PicturePresented = true },
		"source":      func(o *clientObserved) { o.SourceActive = 1 },
		"search":      func(o *clientObserved) { o.SearchActive = 1 },
		"api":         func(o *clientObserved) { o.APIActive = 1 },
		"cover":       func(o *clientObserved) { o.CoverActive = 1 },
		"cover-cache": func(o *clientObserved) { o.CoverRetained = 1 },
		"segments":    func(o *clientObserved) { o.CachedSegments = 1 },
		"entries":     func(o *clientObserved) { o.CachedEntries = 1 },
		"text":        func(o *clientObserved) { o.CachedTextBytes = 1 },
		"danmaku":     func(o *clientObserved) { o.DanmakuActiveRequests = 1 },
		"visible":     func(o *clientObserved) { o.VisibleComments = 1 },
		"layouts":     func(o *clientObserved) { o.TextLayouts = 1 },
		"resolving":   func(o *clientObserved) { o.Resolving = true },
		"resuming":    func(o *clientObserved) { o.Resuming = true },
		"intent":      func(o *clientObserved) { o.DesiredPlaying = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := o
			change(&bad)
			if clientRetired(bad) {
				t.Fatal("retired state concealed a live owner/reference")
			}
		})
	}
}

func TestClientWindowIdentityRemainsSeparateFromFixture(t *testing.T) {
	for _, mode := range []bool{false, true} {
		title := "My Bilibili Preview"
		if mode {
			title = "My Bilibili"
		}
		c := pipeClient(t, func(int, string) any {
			return windowResult{Windows: []window{{ID: 1, Title: title, Kind: "content", Visible: true}}}
		})
		c.clientMode = mode
		if _, err := c.identify(context.Background()); err != nil {
			t.Fatal("owned window not identified", err)
		}
		c.clientMode = !mode
		if _, err := c.identify(context.Background()); err == nil {
			t.Fatal("wrong application mode's window accepted")
		}
	}
}
