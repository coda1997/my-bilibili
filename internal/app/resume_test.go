package app

import (
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func optionsHarness(t *testing.T, options Options) *harness {
	t.Helper()
	h := newHarness(t)
	var err error
	h.s, err = NewWithOptions(h.source, playback.New(h.opener, func() time.Time { return h.now }), options)
	if err != nil {
		t.Fatal(err)
	}
	h.s.now = func() time.Time { return h.now }
	return h
}

func TestOptionsValidateBeforeAnySourceOrNativeWork(t *testing.T) {
	h := newHarness(t)
	for _, option := range []Options{
		{InitialPage: -1}, {InitialPart: "p1", InitialPage: 1},
		{StartAt: -1}, {StartAt: 7*24*time.Hour + 1},
	} {
		if _, err := NewWithOptions(h.source, h.s.controller, option); err != playback.Invalid {
			t.Fatal(option, err)
		}
	}
	for _, option := range []Options{{}, {InitialPart: "p2"}, {InitialPage: 1001}, {StartAt: 7 * 24 * time.Hour}} {
		s, err := NewWithOptions(h.source, h.s.controller, option)
		if err != nil {
			t.Fatal(option, err)
		}
		if s.options != option {
			t.Fatal("options were lost")
		}
	}
	if calls, contexts := h.source.history(); len(calls) != 0 || len(contexts) != 0 || len(h.opener.inputs) != 0 {
		t.Fatal("constructor performed I/O")
	}
}

func TestInitialSelectionMustMatchFreshDetail(t *testing.T) {
	for _, option := range []Options{{InitialPart: "missing", StartAt: time.Second, Autoplay: true}, {InitialPage: 3}, {InitialPage: 1001}} {
		h := optionsHarness(t, option)
		h.start(t)
		s := h.s.State()
		if s.DetailPhase != DetailReady || s.SelectionError != catalog.Unavailable || s.PlaybackPart != "" || s.WatchSequence != 0 || s.Resuming || len(h.opener.inputs) != 0 {
			t.Fatal("missing part opened a substitute", s)
		}
		if calls, _ := h.source.history(); len(calls) != 0 {
			t.Fatal("invalid initial selection reached resolver")
		}
		requireError(t, h.s.SelectPart("p2"), nil)
		h.settle(t)
		h.tick(playback.Ready)
		if h.s.State().PlaybackPart != "p2" || countKind(h.engine(), playback.Seek) != 0 || countKind(h.engine(), playback.Play) != 1 {
			t.Fatal("explicit part retained stale resume")
		}
	}
	for _, option := range []Options{{InitialPart: "p2"}, {InitialPage: 2}} {
		h := optionsHarness(t, option)
		h.start(t)
		if s := h.s.State(); s.PlaybackPart != "p2" || s.DesiredPlaying || s.WatchSequence != 1 {
			t.Fatal(s)
		}
	}
}

func TestResumeWaitsForReadyAndLaterConfirmedSeek(t *testing.T) {
	h := optionsHarness(t, Options{InitialPart: "p2", StartAt: 12 * time.Second, Autoplay: true})
	h.start(t)
	e := h.engine()
	e.deferCommands = true
	if !h.s.State().Resuming || len(e.commands) != 0 {
		t.Fatal("resume did not begin paused")
	}
	requireError(t, h.s.Seek(time.Second), playback.Invalid)
	h.tick(playback.Ready)
	if countKind(e, playback.Seek) != 1 || countKind(e, playback.Play) != 0 || !h.s.State().Resuming {
		t.Fatal("played before initial seek confirmation", e.commands)
	}
	requireError(t, h.s.Seek(time.Second), playback.Invalid)
	for range 3 {
		h.s.Tick()
	}
	if countKind(e, playback.Seek) != 1 {
		t.Fatal("seek was repeated")
	}
	requireError(t, h.s.Pause(), nil)
	requireError(t, h.s.Play(), nil)
	requireError(t, h.s.SetVolume(.4), nil)
	requireError(t, h.s.SetRate(1.5), nil)
	if countKind(e, playback.Play) != 0 {
		t.Fatal("Play bypassed resume")
	}
	e.snapshot.Position = 12 * time.Second
	h.s.Tick()
	if !h.s.State().Resuming {
		t.Fatal("pre-command sampled state confirmed seek")
	}
	e.snapshot.Position = 12*time.Second - 251*time.Millisecond
	h.tick(playback.Paused)
	if !h.s.State().Resuming {
		t.Fatal("outside tolerance confirmed seek")
	}
	e.snapshot.Position = 12*time.Second - 250*time.Millisecond
	h.tick(playback.Paused)
	if h.s.State().Resuming || h.s.State().ResumeError != catalog.None || countKind(e, playback.Play) != 1 || h.s.State().Volume != .4 || h.s.State().Rate != 1.5 {
		t.Fatal("confirmed resume lost intent/settings")
	}
	h.tick(playback.Paused)
	if countKind(e, playback.Play) != 1 {
		t.Fatal("confirmation repeatedly played")
	}
}

func TestResumeRejectsAnUnchangedPreCommandSample(t *testing.T) {
	h := optionsHarness(t, Options{StartAt: 12 * time.Second, Autoplay: true})
	h.start(t)
	e := h.engine()
	e.deferCommands = true
	e.snapshot.Position = 12 * time.Second
	h.tick(playback.Ready)
	h.s.Tick()
	if !h.s.State().Resuming || countKind(e, playback.Play) != 0 {
		t.Fatal("old matching sample accepted the new seek")
	}
	h.tick(playback.Paused)
	if h.s.State().Resuming || countKind(e, playback.Play) != 1 {
		t.Fatal("later matching sample did not confirm")
	}
}

func TestExplicitPendingSelectionClearsInitialResume(t *testing.T) {
	h, source := controlledHarness(t, false)
	h.s.options = Options{StartAt: 12 * time.Second, Autoplay: true}
	call := initialResolve(t, h, source)
	requireError(t, h.s.SelectPart("p1"), nil)
	completeResolution(t, h, call, time.Time{})
	h.tick(playback.Ready)
	if h.s.State().Resuming || countKind(h.engine(), playback.Seek) != 0 || countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("explicit same pending part kept initial resume")
	}
}

func TestResumeUsesNativeDurationAndLatestPauseIntent(t *testing.T) {
	for _, duration := range []time.Duration{50 * time.Millisecond, 5 * time.Second} {
		h := optionsHarness(t, Options{StartAt: 20 * time.Second, Autoplay: true})
		h.start(t)
		h.engine().snapshot.Duration = duration
		requireError(t, h.s.Pause(), nil)
		h.tick(playback.Ready)
		h.tick(playback.Paused)
		want := max(time.Duration(0), duration-100*time.Millisecond)
		if h.engine().snapshot.Position != want || h.s.State().Resuming || h.s.State().DesiredPlaying || countKind(h.engine(), playback.Play) != 0 {
			t.Fatal("native clamp or pause intent lost", h.s.State())
		}
		if duration <= 100*time.Millisecond && countKind(h.engine(), playback.Seek) != 0 {
			t.Fatal("zero target unnecessarily sought")
		}
	}
	h := optionsHarness(t, Options{StartAt: time.Second, Autoplay: true})
	h.start(t)
	h.engine().snapshot.Duration = 0
	h.tick(playback.Ready)
	if countKind(h.engine(), playback.Seek) != 0 || countKind(h.engine(), playback.Play) != 0 {
		t.Fatal("unknown native duration was used")
	}
	h.now = h.now.Add(10 * time.Second)
	h.tick(playback.Ready)
	if h.s.State().ResumeError != catalog.Timeout || h.s.State().Resuming {
		t.Fatal("unknown-duration resume did not time out")
	}
}

func TestResumeTimeoutStaysPausedAndRetriesExactTarget(t *testing.T) {
	h := optionsHarness(t, Options{StartAt: 12 * time.Second, Autoplay: true})
	h.start(t)
	e := h.engine()
	e.deferCommands = true
	h.tick(playback.Ready)
	h.now = h.now.Add(5*time.Second - playback.PollInterval - time.Nanosecond)
	h.tick(playback.Paused)
	if h.s.State().ResumeError != catalog.None || !h.s.State().Resuming {
		t.Fatal("seek timed out before its deadline")
	}
	h.now = h.now.Add(time.Nanosecond)
	h.s.Tick()
	if s := h.s.State(); s.Resuming || s.ResumeError != catalog.Timeout || s.DesiredPlaying || s.Playback.State != playback.Paused || countKind(e, playback.Play) != 0 || countKind(e, playback.Seek) != 1 {
		t.Fatal("unconfirmed resume was accepted", s)
	}
	requireError(t, h.s.Play(), playback.Invalid)
	visit := h.s.State().WatchSequence
	requireError(t, h.s.RetryPlayback(), nil)
	if e.closed != 1 || h.s.State().WatchSequence != visit {
		t.Fatal("retry lost accepted watch ownership")
	}
	h.tick(playback.Ready)
	h.tick(playback.Paused)
	if h.s.State().ResumeError != catalog.None || h.s.State().Resuming || h.engine().snapshot.Position != 12*time.Second || countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("retry did not confirm same target")
	}
}

func TestResumeRetiresOnPartSelectionAndClose(t *testing.T) {
	for _, action := range []string{"other", "same", "close"} {
		t.Run(action, func(t *testing.T) {
			h := optionsHarness(t, Options{StartAt: 9 * time.Second, Autoplay: true})
			h.start(t)
			old := h.engine()
			old.deferCommands = true
			h.tick(playback.Ready)
			if action == "close" {
				requireError(t, h.s.Close(), nil)
				old.snapshot.Position = 9 * time.Second
				h.s.Tick()
				if h.s.State().Resuming || countKind(old, playback.Play) != 0 {
					t.Fatal("closed resume played")
				}
				return
			}
			id := catalog.PartID("p2")
			if action == "same" {
				id = "p1"
			}
			requireError(t, h.s.SelectPart(id), nil)
			h.settle(t)
			old.snapshot.Position = 9 * time.Second
			h.tick(playback.Ready)
			if s := h.s.State(); s.PlaybackPart != id || s.Resuming || s.WatchSequence != 2 || old.closed != 1 || countKind(old, playback.Play) != 0 || countKind(h.engine(), playback.Seek) != 0 || countKind(h.engine(), playback.Play) != 1 {
				t.Fatal("obsolete resume affected explicit selection", s)
			}
		})
	}
}

func TestNativeSeekFailureCannotBecomeResumeSuccess(t *testing.T) {
	h := optionsHarness(t, Options{StartAt: time.Second, Autoplay: true})
	h.start(t)
	e := h.engine()
	e.failKind, e.failErr = playback.Seek, playback.Media
	h.tick(playback.Ready)
	if s := h.s.State(); s.ResumeError != catalog.Unavailable || s.Resuming || s.DesiredPlaying || s.Playback.State != playback.Failed || countKind(e, playback.Play) != 0 || e.closed != 1 {
		t.Fatal("native seek failure concealed", s)
	}
}
