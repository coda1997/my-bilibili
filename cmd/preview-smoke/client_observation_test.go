package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func completeClientObservation() clientObserved {
	return clientObserved{AccountPhase: account.SignedOut, ScopeRevision: 1, Navigation: 1, Visit: 1,
		SearchPhase: catalog.SearchInitial, DetailPhase: app.DetailReady,
		Parts: 2, SelectedPart: 1, PlaybackPart: 1, Qualities: 2, SelectedQuality: 2, PlaybackQuality: 2,
		Playback:   playback.Snapshot{Generation: 1, State: playback.Paused, Position: 5 * time.Second, Duration: 30 * time.Second, Rate: 1, Volume: 1},
		VideoTrack: true, AudioTrack: true, PicturePresented: true,
		DanmakuPhase: danmaku.LoadInitial, HistoryPhase: history.Ready,
		Resources: mpv.Resources{Cores: 1, Renderers: 1, Workers: 2}}
}

func closedClientSample() clientObserved {
	o := completeClientObservation()
	o.AccountPhase, o.SearchPhase, o.DanmakuPhase, o.HistoryPhase = account.Done, catalog.SearchClosed, danmaku.LoadClosed, history.Done
	o.Playback.State = playback.Closed
	o.VideoTrack, o.AudioTrack, o.PicturePresented = false, false, false
	o.Resources = mpv.Resources{}
	o.Closing, o.Closed = true, true
	return o
}

func TestClientReadyNeedsPictureAndCurrentVisibleOwnership(t *testing.T) {
	base := completeClientObservation()
	if !clientReady(base) {
		t.Fatal("complete client is not ready")
	}
	for name, change := range map[string]func(*clientObserved){
		"no-picture":    func(o *clientObserved) { o.PicturePresented = false; o.Playback.Position += time.Second },
		"no-video":      func(o *clientObserved) { o.VideoTrack = false },
		"no-audio":      func(o *clientObserved) { o.AudioTrack = false },
		"no-core":       func(o *clientObserved) { o.Resources.Cores = 0 },
		"no-renderer":   func(o *clientObserved) { o.Resources.Renderers = 0 },
		"no-worker":     func(o *clientObserved) { o.Resources.Workers = 0 },
		"no-generation": func(o *clientObserved) { o.Playback.Generation = 0 },
		"no-part":       func(o *clientObserved) { o.PlaybackPart = 0 },
		"no-quality":    func(o *clientObserved) { o.PlaybackQuality = 0 },
		"no-duration":   func(o *clientObserved) { o.Playback.Duration = 0 },
		"minimized":     func(o *clientObserved) { o.Minimized = true },
		"resuming":      func(o *clientObserved) { o.Resuming = true },
		"closing":       func(o *clientObserved) { o.Closing = true },
		"closed":        func(o *clientObserved) { o.Closed = true },
		"failed":        func(o *clientObserved) { o.Playback.Error = playback.Media },
	} {
		t.Run(name, func(t *testing.T) {
			o := base
			change(&o)
			if clientReady(o) {
				t.Fatal("incomplete client passed readiness")
			}
		})
	}
}

func TestClientObservationHasItsOwnStrictUniqueNode(t *testing.T) {
	o := completeClientObservation()
	b, _ := json.Marshal(o)
	good := node{Key: "e2e-client-observation", Label: string(b)}
	if got, err := clientObservation([]node{good}); err != nil || got != o {
		t.Fatal("client observation lost", err)
	}
	for _, nodes := range [][]node{nil, {good, good}, {{Key: "e2e-observation", Label: string(b)}}, {{Key: "e2e-client-observation", Label: "{}"}}} {
		if _, err := clientObservation(nodes); err == nil {
			t.Fatal("absent/ambiguous/incomplete client observation accepted")
		}
	}
	if _, err := observation([]node{good}); err == nil {
		t.Fatal("client evidence became fixture evidence")
	}
	fixture, _ := json.Marshal(completeObservation(t))
	if _, err := clientObservation([]node{{Key: "e2e-client-observation", Label: string(fixture)}}); err == nil {
		t.Fatal("fixture schema became client evidence")
	}
}

func TestClientSliderReadsActualNativeDurationThroughClientSchema(t *testing.T) {
	o := completeClientObservation()
	b, _ := json.Marshal(o)
	now := 18.0
	c := pipeClient(t, func(_ int, method string) any {
		if method == "tree" {
			return treeResult{Nodes: []node{{Key: "e2e-client-observation", Label: string(b)}, {Key: "seek-slider", Bounds: rectangle{Width: 80, Height: 28}, Now: &now}}}
		}
		return map[string]bool{"ok": true}
	})
	c.clientMode = true
	if err := c.slider(context.Background(), "seek-slider", "18"); err != nil {
		t.Fatal(err)
	}
	if err := c.slider(context.Background(), "seek-slider", "378"); err == nil {
		t.Fatal("synthetic catalog duration substituted for actual30s native duration")
	}
}

func TestClientUntilRejectsPartialEvidenceBeforePredicate(t *testing.T) {
	c := pipeClient(t, func(int, string) any { return treeResult{Nodes: []node{{Key: "e2e-client-observation", Label: "{}"}}} })
	called := false
	if _, err := c.clientUntil(context.Background(), time.Second, func(clientObserved) bool { called = true; return true }); err == nil || called {
		t.Fatal("incomplete client observation reached evidence predicate")
	}
}

func TestClientReportOwnsItsSafeSnapshot(t *testing.T) {
	o := completeClientObservation()
	r := &report{}
	r.passClient("before", "safe numeric sample", o)
	o.Playback.Position++
	if r.Checks[0].ClientObservation.Playback.Position == o.Playback.Position || r.Checks[0].Observation != nil {
		t.Fatal("client snapshot changed or polluted fixture evidence")
	}
}
