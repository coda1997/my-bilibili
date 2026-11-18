package app

import (
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func qualityResult(part catalog.PartID, id QualityID, refresh bool) Resolution {
	choices := []Quality{{ID: "low", Label: "Low", Width: 640, Height: 360}, {ID: "high", Label: "High", Width: 1280, Height: 720}}
	chosen := choices[0]
	if id == "high" {
		chosen = choices[1]
	}
	return Resolution{Input: playback.Input{FilePath: "/fixture-" + string(part) + ".mp4"}, Quality: chosen, Qualities: choices, RefreshOnRetry: refresh}
}

func qualityHarness(t *testing.T, refresh bool) (*harness, *controlledSource) {
	t.Helper()
	h, source := controlledHarness(t, true)
	call := initialResolve(t, h, source)
	completeQuality(t, h, call, "low", refresh)
	h.tick(playback.Paused)
	return h, source
}

func completeQuality(t *testing.T, h *harness, call *sourceCall, id QualityID, refresh bool) {
	t.Helper()
	call.reply <- sourceReply{resolution: qualityResult(call.part, id, refresh)}
	resultReady(t, h.s)
	h.s.Tick()
}

func TestQualityPreservesUnconfirmedHistorySeek(t *testing.T) {
	const target = 12 * time.Second
	for _, when := range []string{"before-ready", "seek-in-flight"} {
		t.Run(when, func(t *testing.T) {
			h, source := controlledHarness(t, false)
			h.s.options = Options{StartAt: target, Autoplay: true}
			completeQuality(t, h, initialResolve(t, h, source), "low", false)
			old := h.engine()
			old.deferCommands = true
			if when == "seek-in-flight" {
				h.tick(playback.Ready)
			}
			if !h.s.State().Resuming || h.s.State().Playback.Position != 0 {
				t.Fatal("initial history seek already settled")
			}
			requireError(t, h.s.SelectQuality("high"), nil)
			replacement := source.next(t, "p1")
			if when == "seek-in-flight" {
				requireError(t, h.s.Pause(), nil)
			}
			visit := h.s.State().WatchSequence
			completeQuality(t, h, replacement, "high", false)
			current := h.engine()
			h.tick(playback.Ready)
			if old.closed != 1 || h.s.State().WatchSequence != visit || current.snapshot.Position != target || countKind(current, playback.Seek) != 1 || countKind(current, playback.Play) != 0 {
				t.Fatalf("replacement before confirmation: position=%v seek=%d play=%d old closed=%d", current.snapshot.Position, countKind(current, playback.Seek), countKind(current, playback.Play), old.closed)
			}
			h.tick(playback.Paused)
			wantPlay := 1
			if when == "seek-in-flight" {
				wantPlay = 0
			}
			if h.s.State().Resuming || h.s.State().PlaybackQuality != "high" || countKind(current, playback.Seek) != 1 || countKind(current, playback.Play) != wantPlay {
				t.Fatal("replacement lost latest playback intent after confirming history seek")
			}
		})
	}
}

func TestQualityCapturesLatestPositionAndTransportIntent(t *testing.T) {
	for _, playing := range []bool{true, false} {
		h, source := qualityHarness(t, false)
		requireError(t, h.s.Play(), nil)
		h.tick(playback.Playing)
		old := h.engine()
		old.snapshot.Position = 4 * time.Second
		h.tick(playback.Playing)
		watch := h.s.State().WatchSequence
		requireError(t, h.s.SelectQuality("high"), nil)
		call := source.next(t, "p1")
		if call.quality != "high" {
			t.Fatal("selected quality not sent to Source")
		}
		request := h.s.pending.request.generation
		requireError(t, h.s.SelectQuality("high"), nil)
		if h.s.pending.request.generation != request {
			t.Fatal("duplicate pending quality resubmitted")
		}
		requireError(t, h.s.SetRate(1.75), nil)
		requireError(t, h.s.SetVolume(.35), nil)
		if !playing {
			requireError(t, h.s.Pause(), nil)
		}
		old.snapshot.Position = 9 * time.Second
		state := playback.Playing
		if !playing {
			state = playback.Paused
		}
		h.tick(state)
		if s := h.s.State(); old.closed != 0 || s.PlaybackQuality != "low" || s.SelectedQuality != "high" || s.WatchSequence != watch {
			t.Fatal("pending quality replaced accepted playback", s)
		}
		completeQuality(t, h, call, "high", false)
		if s := h.s.State(); s.PlaybackQuality != "high" || s.SelectedQuality != "high" || !s.Resuming || s.WatchSequence != watch || old.closed != 1 {
			t.Fatal("quality acceptance identity", s)
		}
		e := h.engine()
		h.tick(playback.Ready)
		if e.snapshot.Position != 9*time.Second || countKind(e, playback.Seek) != 1 || countKind(e, playback.Play) != 0 {
			t.Fatal("quality captured submission time or played before seek")
		}
		h.tick(playback.Paused)
		wantPlay := 0
		if playing {
			wantPlay = 1
		}
		if h.s.State().Resuming || countKind(e, playback.Play) != wantPlay || e.snapshot.Volume != .35 || e.snapshot.Rate != 1.75 {
			t.Fatal("quality lost latest intent/settings")
		}
	}
}

func TestQualityMustBeReturnedAndResolveExactly(t *testing.T) {
	h, source := qualityHarness(t, false)
	old := h.engine()
	requireError(t, h.s.SelectQuality("advertised-only"), catalog.QualityUnavailable)
	requireError(t, h.s.SelectQuality(""), catalog.QualityUnavailable)
	source.noCall(t)
	requireError(t, h.s.SelectQuality("high"), nil)
	call := source.next(t, "p1")
	completeQuality(t, h, call, "low", false)
	if s := h.s.State(); s.SelectionError != catalog.QualityUnavailable || s.SelectedQuality != "high" || s.PlaybackQuality != "low" || old.closed != 0 || len(s.Qualities) != 2 {
		t.Fatal("source silently substituted a quality", s)
	}
	requireError(t, h.s.RetrySelection(), nil)
	call = source.next(t, "p1")
	if call.quality != "high" {
		t.Fatal("retry lost requested quality")
	}
	requireError(t, h.s.RetrySelection(), nil)
	completeQuality(t, h, call, "high", false)
	if s := h.s.State(); s.SelectionError != catalog.None || s.PlaybackQuality != "high" {
		t.Fatal("quality retry failed", s)
	}
}

func TestRetiredQualityCannotReplaceCurrentPart(t *testing.T) {
	for _, action := range []string{"accepted-quality", "other-part", "close"} {
		t.Run(action, func(t *testing.T) {
			h, source := qualityHarness(t, false)
			old := h.engine()
			requireError(t, h.s.SelectQuality("high"), nil)
			call := source.next(t, "p1")
			switch action {
			case "accepted-quality":
				requireError(t, h.s.SelectQuality("low"), nil)
			case "other-part":
				requireError(t, h.s.SelectPart("p2"), nil)
				requireError(t, h.s.SelectQuality("high"), playback.Invalid)
			case "close":
				requireError(t, h.s.Close(), nil)
			}
			if call.ctx.Err() == nil {
				t.Fatal("retired request was not canceled")
			}
			call.reply <- sourceReply{resolution: qualityResult("p1", "high", false)}
			if action == "close" {
				waitDone(t, h.s.Done())
				return
			}
			resultReady(t, h.s)
			h.s.Tick()
			if old.closed != 0 || h.s.State().PlaybackQuality != "low" || len(h.opener.inputs) != 1 {
				t.Fatal("obsolete quality opened")
			}
			if action == "other-part" {
				part := source.next(t, "p2")
				if part.quality != "" {
					t.Fatal("new part inherited quality")
				}
				completeQuality(t, h, part, "low", false)
				if h.s.State().PlaybackPart != "p2" || h.s.State().Resuming {
					t.Fatal("new part inherited resume")
				}
			}
		})
	}
}

func TestEndedRetiresOnlyQualityAndExplicitReplayStillWorks(t *testing.T) {
	h, source := qualityHarness(t, true)
	watch := h.s.State().WatchSequence
	requireError(t, h.s.SelectQuality("high"), nil)
	call := source.next(t, "p1")
	h.engine().snapshot.Position = h.engine().snapshot.Duration
	h.tick(playback.Ended)
	if s := h.s.State(); s.ResolvingPart != "" || s.SelectedQuality != "low" || s.PlaybackQuality != "low" || s.DesiredPlaying || s.WatchSequence != watch || s.Playback.State != playback.Ended {
		t.Fatal("ended quality replayed", s)
	}
	requireError(t, h.s.SelectQuality("high"), playback.Invalid)
	requireError(t, h.s.Play(), nil)
	call.reply <- sourceReply{resolution: qualityResult("p1", "high", true)}
	resultReady(t, h.s)
	h.s.Tick()
	replay := source.next(t, "p1")
	if replay.quality != "low" {
		t.Fatal("replay used retired quality")
	}
	for range 3 {
		h.s.Tick()
	}
	if h.s.pending == nil || !h.s.State().DesiredPlaying {
		t.Fatal("ended sample retired deliberate replay")
	}
	completeQuality(t, h, replay, "low", true)
	if s := h.s.State(); s.WatchSequence != watch+1 || s.Resuming || s.Playback.Position != 0 {
		t.Fatal("explicit replay did not start a fresh watch at zero", s)
	}
	h.tick(playback.Ready)
	if countKind(h.engine(), playback.Play) != 1 || countKind(h.engine(), playback.Seek) != 0 {
		t.Fatal("replay kept near-end resume")
	}
}

func TestRefreshOnRetryResolvesInsideCutoffAndRetainsWatch(t *testing.T) {
	h, source := qualityHarness(t, true)
	h.engine().snapshot.Position = 6 * time.Second
	h.tick(playback.Paused)
	watch := h.s.State().WatchSequence
	old := h.engine()
	requireError(t, h.s.RetryPlayback(), nil)
	call := source.next(t, "p1")
	if call.quality != "low" || old.closed != 0 {
		t.Fatal("fresh retry lost chosen quality or closed early")
	}
	call.reply <- sourceReply{err: catalog.Network}
	resultReady(t, h.s)
	h.s.Tick()
	if old.closed != 0 || h.s.State().SelectionError != catalog.Network || len(h.opener.inputs) != 1 {
		t.Fatal("failed refresh fell back to cached URL")
	}
	requireError(t, h.s.RetryPlayback(), nil)
	call = source.next(t, "p1")
	completeQuality(t, h, call, "low", true)
	h.tick(playback.Ready)
	h.tick(playback.Paused)
	if s := h.s.State(); s.WatchSequence != watch || s.Playback.Position != 6*time.Second || s.Resuming {
		t.Fatal("fresh retry reset accepted progress/watch", s)
	}
}

func TestWatchSequenceTracksAcceptedExplicitVisits(t *testing.T) {
	h, source := qualityHarness(t, false)
	if h.s.State().WatchSequence != 1 {
		t.Fatal("first accepted watch missing")
	}
	generation := h.s.State().Playback.Generation
	requireError(t, h.s.SelectPart("p1"), nil)
	if s := h.s.State(); s.WatchSequence != 2 || s.Playback.Generation != generation {
		t.Fatal("explicit fast-path reselect lost visit", s)
	}
	requireError(t, h.s.SelectQuality("high"), nil)
	completeQuality(t, h, source.next(t, "p1"), "high", false)
	if h.s.State().WatchSequence != 2 {
		t.Fatal("quality created visit")
	}
	requireError(t, h.s.SelectPart("p2"), nil)
	call := source.next(t, "p2")
	call.reply <- sourceReply{err: catalog.Unavailable}
	resultReady(t, h.s)
	h.s.Tick()
	if h.s.State().WatchSequence != 2 {
		t.Fatal("failed pending part created visit")
	}
	requireError(t, h.s.RetrySelection(), nil)
	completeQuality(t, h, source.next(t, "p2"), "low", false)
	if h.s.State().WatchSequence != 3 {
		t.Fatal("accepted explicit part missing visit")
	}
	h.tick(playback.Failed)
	requireError(t, h.s.RetryPlayback(), nil)
	if h.s.State().WatchSequence != 3 {
		t.Fatal("native retry created visit")
	}
}

func TestFailedQualityRetryCannotReplayEndedPlayback(t *testing.T) {
	h, source := qualityHarness(t, false)
	requireError(t, h.s.SelectQuality("high"), nil)
	call := source.next(t, "p1")
	call.reply <- sourceReply{err: catalog.Network}
	resultReady(t, h.s)
	h.s.Tick()
	h.tick(playback.Ended)
	requireError(t, h.s.RetrySelection(), playback.Invalid)
	source.noCall(t)
	if len(h.opener.inputs) != 1 || h.s.State().DesiredPlaying {
		t.Fatal("quality retry replayed ended native playback")
	}
}
