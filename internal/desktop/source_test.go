package desktop

import (
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func earlyState(phase app.DetailPhase) app.State {
	return app.State{DetailPhase: phase, Video: catalog.Video{Parts: []catalog.Part{}},
		Playback: playback.Snapshot{State: playback.Idle}, Volume: .2, Rate: 1}
}

func TestDetailLoadingAndExplicitRetry(t *testing.T) {
	for _, state := range []struct {
		phase app.DetailPhase
		label string
	}{{app.DetailInitial, "Preparing video…"}, {app.DetailLoading, "Loading video details…"}} {
		t.Run(string(state.phase), func(t *testing.T) {
			h := newHarness()
			h.player.state = earlyState(state.phase)
			coverCalls := 0
			h.view.Cover = func(*gui.Context, string) { coverCalls++ }
			h.ui.Frame()
			rect(t, h, state.label)
			if h.ui.HasText("Fixture preview") || coverCalls != 0 {
				t.Fatal("metadata or cover rendered before detail acceptance")
			}
			for _, label := range []string{"Play", "Next", "Parts", "Mute", "Volume", "Playback speed", "Seek position"} {
				if !access(t, h, label).Disabled {
					t.Fatalf("%s enabled before details", label)
				}
				click(t, h, label)
			}
			for _, key := range []gui.Key{gui.KeySpace, gui.KeyM, gui.KeyLeft, gui.KeyRight, gui.KeyUp, gui.KeyDown} {
				h.ui.Key(0, key)
			}
			videoClick(t, h)
			h.ui.Advance(time.Second)
			wantCalls(t, h)
			if h.view.partsOpen || h.view.speedOpen {
				t.Fatal("unavailable controls opened a menu")
			}
			h.ui.Key(0, gui.KeyF)
			if !h.full {
				t.Fatal("loading prevented fullscreen")
			}
			h.ui.Key(0, gui.KeyEscape)
			h.player.state.DetailPhase, h.player.state.DetailError = app.DetailFailed, catalog.Timeout
			h.ui.Frame()
			rect(t, h, "Could not load video details · timeout")
			a := access(t, h, "Retry details")
			if a.Disabled || !a.Focusable || a.Bounds.W < 28 || a.Bounds.H < 28 {
				t.Fatalf("retry accessibility: %+v", a)
			}
			h.ui.Advance(time.Second)
			wantCalls(t, h)
			click(t, h, "Retry details")
			wantCalls(t, h, call{name: "retry-detail"})
			h.player.state.DetailPhase, h.player.state.DetailError = app.DetailLoading, catalog.None
			h.ui.Frame()
			rect(t, h, "Loading video details…")
			if _, found := h.ui.Find("Retry details"); found {
				t.Fatal("retry remains available during detail loading")
			}
		})
	}
}

func TestFirstPendingResolutionAcceptsIntentAndSettings(t *testing.T) {
	h := newHarness()
	h.player.state.PlaybackPart = ""
	h.player.state.ResolvingPart = "part-1"
	h.player.state.Playback = playback.Snapshot{State: playback.Idle}
	h.ui.Frame()
	rect(t, h, "Loading playback address for P1 · First")
	rect(t, h, "P1 · First · requested")
	for _, label := range []string{"Play", "Next", "Parts", "Mute", "Volume", "Playback speed"} {
		if access(t, h, label).Disabled {
			t.Fatalf("%s disabled during a resolvable selection", label)
		}
	}
	if !access(t, h, "Seek position").Disabled {
		t.Fatal("unresolved target became seekable")
	}
	click(t, h, "Play")
	h.player.state.DesiredPlaying = true
	h.ui.Frame()
	click(t, h, "Pause")
	click(t, h, "Mute")
	click(t, h, "Playback speed")
	click(t, h, "Set speed 1.5×")
	// Clear button focus before exercising the window-level setting shortcuts.
	h.ui.ClickAt(0, 0)
	h.ui.Key(0, gui.KeyUp)
	h.ui.Key(0, gui.KeyRight)
	click(t, h, "Next")
	click(t, h, "P2 · Second")
	wantCalls(t, h, call{name: "toggle"}, call{name: "toggle"}, call{name: "mute"},
		call{name: "rate", value: 1.5}, call{name: "volume", value: .25},
		call{name: "next"}, call{name: "part", part: "part-2"})
}

func TestPendingSelectionKeepsAcceptedPlaybackControls(t *testing.T) {
	h := newHarness()
	h.player.state.SelectedPart, h.player.state.ResolvingPart = "part-2", "part-2"
	h.player.state.Playback.State = playback.Playing
	h.player.state.DesiredPlaying = true
	h.ui.Frame()
	h.ui.Advance(4 * time.Second)
	rect(t, h, "Loading playback address for P2 · Second")
	rect(t, h, "Playing: P1 · First")
	rect(t, h, "P1 · First · playing")
	rect(t, h, "P2 · Second · requested")
	if access(t, h, "Pause").Disabled || access(t, h, "Seek position").Disabled {
		t.Fatal("accepted playback controls unavailable during resolution")
	}
	click(t, h, "Pause")
	h.ui.ClickAt(0, 0)
	h.ui.Key(0, gui.KeyRight)
	click(t, h, "P1 · First · playing")
	wantCalls(t, h, call{name: "toggle"}, call{name: "seek", value: 6}, call{name: "part", part: "part-1"})
	// Selecting or seeking must not itself relabel the accepted native attempt.
	rect(t, h, "Playing: P1 · First")
}

func TestSeparateSourceRetryStatesAndConflictingRetries(t *testing.T) {
	h := newHarness()
	h.player.state.SelectedPart = "part-2"
	h.player.state.SelectionError = catalog.Unavailable
	h.player.state.PlaybackRetryError = catalog.Timeout
	// A refresh can fail while the accepted engine remains ended rather than Failed.
	h.player.state.Playback.State = playback.Ended
	h.ui.Frame()
	rect(t, h, "Could not select P2 · Second")
	rect(t, h, "Could not reload playback · timeout")
	for _, label := range []string{"Retry selection", "Retry playback"} {
		a := access(t, h, label)
		if a.Disabled || !a.Focusable {
			t.Fatalf("retry unavailable: %+v", a)
		}
		click(t, h, label)
	}
	wantCalls(t, h, call{name: "retry-selection"}, call{name: "retry-playback"})
	h.player.calls = nil
	h.player.state.ResolvingPart = "part-1"
	h.player.state.PlaybackRetryError = catalog.None
	h.player.state.Playback.State, h.player.state.Playback.Error = playback.Failed, playback.Unsupported
	h.ui.Frame()
	rect(t, h, "Loading playback address for P1 · First")
	rect(t, h, "Could not select P2 · Second")
	for _, label := range []string{"Retry selection", "Retry playback"} {
		if !access(t, h, label).Disabled {
			t.Fatalf("conflicting %s enabled", label)
		}
		click(t, h, label)
	}
	h.ui.Advance(time.Second)
	wantCalls(t, h)
	if h.fullscreenCalls != 0 {
		t.Fatal("disabled retry clicks reached the video surface")
	}
	if access(t, h, "P2 · Second · requested").Disabled {
		t.Fatal("pending resolution suppressed a new selection")
	}
	click(t, h, "P2 · Second · requested")
	wantCalls(t, h, call{name: "part", part: "part-2"})
}

func TestFirstResolutionFailureStillAllowsPartSelection(t *testing.T) {
	h := newHarness()
	h.player.state.PlaybackPart = ""
	h.player.state.Playback = playback.Snapshot{State: playback.Idle}
	h.player.state.SelectionError = catalog.Unavailable
	h.ui.Frame()
	if !access(t, h, "Play").Disabled {
		t.Fatal("failed first resolution offered transport without a target")
	}
	for _, label := range []string{"Next", "Parts", "P2 · Second", "Retry selection"} {
		if access(t, h, label).Disabled {
			t.Fatalf("%s unavailable after resolution failure", label)
		}
	}
	click(t, h, "Next")
	click(t, h, "Retry selection")
	click(t, h, "P2 · Second")
	wantCalls(t, h, call{name: "next"}, call{name: "retry-selection"}, call{name: "part", part: "part-2"})
}

func TestClosingRetiresGesturesMenusAndActions(t *testing.T) {
	for _, pending := range []string{"video click", "Parts", "Playback speed", "Volume"} {
		t.Run(pending, func(t *testing.T) {
			h := newHarness()
			var slider gui.Rect
			switch pending {
			case "video click":
				videoClick(t, h)
			case "Volume":
				slider = rect(t, h, "Volume")
				h.ui.Press(slider.X+slider.W/2, slider.Y+slider.H/2)
			default:
				click(t, h, pending)
			}
			h.player.calls = nil
			h.player.state.Closing = true
			h.player.state.Playback.State = playback.Closed
			h.ui.Frame()
			rect(t, h, "Closing…")
			if h.view.partsOpen || h.view.speedOpen || !h.view.pendingClick.IsZero() {
				t.Fatal("closing left a menu or deferred click active")
			}
			if pending == "Volume" {
				h.ui.Move(slider.X+slider.W, slider.Y)
				h.ui.Release(slider.X+slider.W, slider.Y)
			}
			for _, label := range []string{"Play", "Next", "Parts", "Mute", "Volume", "Playback speed", "Seek position", "Fullscreen", "P2 · Second"} {
				if !access(t, h, label).Disabled {
					t.Fatalf("%s enabled while closing", label)
				}
				click(t, h, label)
			}
			for _, key := range []gui.Key{gui.KeySpace, gui.KeyM, gui.KeyF, gui.KeyLeft, gui.KeyRight, gui.KeyUp, gui.KeyDown} {
				h.ui.Key(0, key)
			}
			videoClick(t, h)
			h.ui.Advance(100 * time.Millisecond)
			videoClick(t, h)
			h.ui.Advance(time.Second)
			wantCalls(t, h)
			if h.full || h.fullscreenCalls != 0 {
				t.Fatal("closing changed fullscreen")
			}
		})
	}
	t.Run("detail retry", func(t *testing.T) {
		h := newHarness()
		h.player.state = earlyState(app.DetailFailed)
		h.player.state.DetailError = catalog.Timeout
		h.player.state.Closing = true
		h.ui.Frame()
		if !access(t, h, "Retry details").Disabled {
			t.Fatal("detail retry enabled while closing")
		}
		click(t, h, "Retry details")
		wantCalls(t, h)
	})
}
