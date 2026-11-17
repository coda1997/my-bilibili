package desktop

import (
	"bytes"
	"math"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/danmaku"
	danmakuview "my-bilibili/internal/desktop/danmaku"
	"my-bilibili/internal/playback"
)

func newDanmakuHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness()
	h.view.Danmaku = danmakuview.New()
	if err := h.view.Danmaku.Reset(1); err != nil {
		t.Fatal(err)
	}
	if err := h.view.Danmaku.SetEntries(1, []danmaku.Entry{{Mode: danmaku.FixedTop, Text: "中文弹幕测试", Color: 0xffffff}}); err != nil {
		t.Fatal(err)
	}
	h.ui.Frame()
	return h
}

func wantPopup(t *testing.T, h *harness, name string) {
	t.Helper()
	if h.view.partsOpen != (name == "Parts") || h.view.speedOpen != (name == "Playback speed") || h.view.danmakuOpen != (name == "Danmaku") {
		t.Fatalf("popup %q: parts=%v speed=%v danmaku=%v", name, h.view.partsOpen, h.view.speedOpen, h.view.danmakuOpen)
	}
}

func TestDanmakuFittedBoundsAndSecondControlRow(t *testing.T) {
	h := newDanmakuHarness(t)
	plain := newHarness()
	for _, size := range []struct {
		w, h int
		full bool
	}{{1120, 760, false}, {800, 600, false}, {800, 600, true}, {1200, 600, true}, {800, 1000, true}} {
		h.full, plain.full = size.full, size.full
		h.ui.SetSize(size.w, size.h)
		plain.ui.SetSize(size.w, size.h)
		overlay, surface, video := rect(t, h, "Danmaku overlay"), rect(t, h, "Injected surface"), rect(t, h, "Video surface")
		if overlay != surface || overlay.X < video.X || overlay.Y < video.Y || overlay.X+overlay.W > video.X+video.W+1 || overlay.Y+overlay.H > video.Y+video.H+1 {
			t.Fatalf("overlay is not the fitted video: overlay=%+v surface=%+v video=%+v", overlay, surface, video)
		}
		state := h.view.Danmaku.State()
		if state.Width != float64(h.surfaceW) || state.Height != float64(h.surfaceH) || state.Active != 1 {
			t.Fatalf("renderer did not prepare the fitted frame: %+v", state)
		}
		if size.full && size.w == 800 && (overlay.W != 800 || overlay.H != 450 || overlay.Y != float32(size.h-450)/2) {
			t.Fatalf("letterbox geometry: %+v", overlay)
		}
		fullLabel := "Fullscreen"
		if size.full {
			fullLabel = "Exit full"
		}
		for _, label := range []string{"Play", "Next", "Parts", "Playback speed", "Mute", "Volume", fullLabel} {
			got, before := access(t, h, label).Bounds, access(t, plain, label).Bounds
			if got.X != before.X || got.W != before.W {
				t.Fatalf("Danmaku changed the first transport row: %s got=%+v before=%+v", label, got, before)
			}
		}
		menu, playing, play := access(t, h, "Danmaku"), rect(t, h, "Playing: P1 · First"), access(t, h, "Play").Bounds
		if !menu.Focusable || menu.Disabled || menu.Bounds.W < 28 || menu.Bounds.H < 28 || menu.Bounds.Y < play.Y+play.H || playing.Y < menu.Bounds.Y || playing.Y+playing.H > menu.Bounds.Y+menu.Bounds.H {
			t.Fatalf("Danmaku did not share the second row: menu=%+v playing=%+v play=%+v", menu, playing, play)
		}
		click(t, h, "Danmaku")
		for _, label := range []string{"Show danmaku", "Danmaku opacity", "Danmaku text size", "Danmaku speed"} {
			a := access(t, h, label)
			if !a.Focusable || a.Disabled || a.Bounds.W < 28 || a.Bounds.H < 28 || a.Bounds.X < 0 || a.Bounds.Y < 0 || a.Bounds.X+a.Bounds.W > float32(size.w) || a.Bounds.Y+a.Bounds.H > float32(size.h) {
				t.Fatalf("settings outside %dx%d: %s %+v", size.w, size.h, label, a)
			}
		}
		h.ui.Key(0, gui.KeyEscape)
		wantPopup(t, h, "")
		if h.full != size.full {
			t.Fatal("popup Escape changed fullscreen")
		}
	}
	wantCalls(t, h)
}

func TestDanmakuPaintsAboveSurface(t *testing.T) {
	h := newDanmakuHarness(t)
	h.view.Surface = func(c *gui.Context, w, height float32) {
		gui.Box(c).Size(w, height).Background(gui.RGB(20, 40, 60)).Label("Injected surface")
	}
	h.ui.Frame()
	shown := bytes.Clone(h.ui.Image().Pix)
	settings := h.view.Danmaku.Settings()
	settings.Visible = false
	if err := h.view.Danmaku.Configure(settings); err != nil {
		t.Fatal(err)
	}
	h.ui.Frame()
	if bytes.Equal(shown, h.ui.Image().Pix) {
		t.Fatal("overlay pixels are hidden behind the opaque video surface")
	}
	wantCalls(t, h)
}

func TestDanmakuPopupExclusivity(t *testing.T) {
	for _, first := range []string{"Parts", "Playback speed", "Danmaku"} {
		for _, second := range []string{"Parts", "Playback speed", "Danmaku"} {
			if first == second {
				continue
			}
			t.Run(first+" to "+second, func(t *testing.T) {
				h := newDanmakuHarness(t)
				h.ui.SetSize(800, 600)
				click(t, h, first)
				wantPopup(t, h, first)
				click(t, h, second)
				wantPopup(t, h, second)
				h.ui.Advance(time.Second)
				wantCalls(t, h)
			})
		}
	}
}

func TestDanmakuKeyboardMenuSwitch(t *testing.T) {
	// A non-modal popup can stay open while another anchor owns focus. Key
	// activation has no outside press to close the other popup for the parent.
	for _, other := range []string{"Parts", "Playback speed"} {
		for _, toDanmaku := range []bool{true, false} {
			first, second := other, "Danmaku"
			if !toDanmaku {
				first, second = second, first
			}
			t.Run(first+" to "+second, func(t *testing.T) {
				h := newDanmakuHarness(t)
				click(t, h, second)
				click(t, h, second)
				wantPopup(t, h, "")
				h.view.partsOpen, h.view.speedOpen, h.view.danmakuOpen = first == "Parts", first == "Playback speed", first == "Danmaku"
				h.ui.Frame()
				if !h.ui.Focused(second) {
					t.Fatal("menu anchor did not retain keyboard focus")
				}
				h.ui.Key(0, gui.KeySpace)
				wantPopup(t, h, second)
				h.ui.Advance(time.Second)
				wantCalls(t, h)
			})
		}
	}
}

func TestDanmakuPopupHoldsControlsAndRefreshesActivity(t *testing.T) {
	h := newDanmakuHarness(t)
	h.player.state.Playback.State = playback.Playing
	h.ui.Frame()
	click(t, h, "Danmaku")
	click(t, h, "Show danmaku")
	h.ui.Move(0, 0)
	h.ui.Advance(4 * time.Second)
	if h.view.controlsHeld || h.view.seek.pressed || h.view.volume.pressed || h.view.now().Sub(h.view.lastActivity) < overlayDelay {
		t.Fatal("test did not isolate the open-popup visibility hold")
	}
	rect(t, h, "Seek position")
	// Keep the pointer outside the player: movement must not mask the activity
	// recorded by a settings change through the focused checkbox.
	h.ui.Key(0, gui.KeySpace)
	if h.view.lastActivity != h.ui.Now() || h.view.Danmaku.State().Active != 1 {
		t.Fatal("setting change did not refresh activity and the prepared frame")
	}
	h.ui.Key(0, gui.KeyEscape)
	h.ui.ClickAt(0, 0)
	h.ui.Advance(4 * time.Second)
	if _, shown := h.ui.Find("Seek position"); shown {
		t.Fatal("closed popup kept controls visible without activity or focus")
	}
	wantCalls(t, h)
}

func TestDanmakuPopupOutsidePressAndVideoInput(t *testing.T) {
	t.Run("opening cancels deferred click", func(t *testing.T) {
		h := newDanmakuHarness(t)
		videoClick(t, h)
		click(t, h, "Danmaku")
		h.ui.Advance(time.Second)
		wantCalls(t, h)
		if !h.view.pendingClick.IsZero() {
			t.Fatal("opening settings retained a deferred video toggle")
		}
	})
	t.Run("video dismiss consumes one click", func(t *testing.T) {
		h := newDanmakuHarness(t)
		click(t, h, "Danmaku")
		videoClick(t, h)
		h.ui.Advance(time.Second)
		wantPopup(t, h, "")
		wantCalls(t, h)
		videoClick(t, h)
		h.ui.Advance(time.Second)
		wantCalls(t, h, call{name: "toggle"})
	})
	t.Run("transport dismiss still activates target", func(t *testing.T) {
		h := newDanmakuHarness(t)
		click(t, h, "Danmaku")
		click(t, h, "Mute")
		h.ui.Advance(time.Second)
		wantPopup(t, h, "")
		wantCalls(t, h, call{name: "mute"})
	})
	t.Run("paint and errors pass clicks through", func(t *testing.T) {
		for _, unavailable := range []bool{false, true} {
			h := newDanmakuHarness(t)
			if unavailable {
				if err := h.view.Danmaku.SetUnavailable(1); err != nil {
					t.Fatal(err)
				}
				h.ui.Frame()
			}
			videoClick(t, h)
			h.ui.Advance(time.Second)
			wantCalls(t, h, call{name: "toggle"})
			h.player.calls = nil
			videoClick(t, h)
			h.ui.Advance(100 * time.Millisecond)
			videoClick(t, h)
			h.ui.Advance(time.Second)
			wantCalls(t, h)
			if !h.full || h.fullscreenCalls != 1 {
				t.Fatal("overlay intercepted the fullscreen double click")
			}
		}
	})
}

func TestDanmakuFocusedSettingsKeysAndPersistence(t *testing.T) {
	h := newDanmakuHarness(t)
	h.full = true
	h.ui.SetSize(800, 600)
	click(t, h, "Danmaku")
	click(t, h, "Show danmaku")
	h.ui.Key(0, gui.KeySpace)
	if !h.view.Danmaku.Settings().Visible {
		t.Fatal("focused checkbox did not consume Space")
	}
	for _, slider := range []struct {
		label string
		want  float64
		value func(danmaku.Settings) float64
	}{
		{"Danmaku opacity", .05, func(s danmaku.Settings) float64 { return s.Opacity }},
		{"Danmaku text size", 17, func(s danmaku.Settings) float64 { return s.FontSize }},
		{"Danmaku speed", .6, func(s danmaku.Settings) float64 { return s.Speed }},
	} {
		click(t, h, slider.label)
		if !h.ui.Focused(slider.label) {
			t.Fatalf("%s did not receive focus", slider.label)
		}
		h.ui.Key(0, gui.KeyHome)
		h.ui.Key(0, gui.KeyRight)
		if value := slider.value(h.view.Danmaku.Settings()); math.Abs(value-slider.want) > 1e-10 {
			t.Fatalf("%s keyboard setting=%v want=%v", slider.label, value, slider.want)
		}
	}
	wantCalls(t, h)
	settings := h.view.Danmaku.Settings()
	h.ui.Key(0, gui.KeyEscape)
	wantPopup(t, h, "")
	if !h.full {
		t.Fatal("popup Escape exited fullscreen")
	}
	h.ui.Key(0, gui.KeyEscape)
	if h.full {
		t.Fatal("second Escape did not exit fullscreen")
	}
	h.player.state.Playback.Generation = 2
	h.player.state.PlaybackPart, h.player.state.SelectedPart = "part-2", "part-2"
	if err := h.view.Danmaku.Reset(2); err != nil {
		t.Fatal(err)
	}
	h.ui.Frame()
	h.full = true
	h.ui.Frame()
	if h.view.Danmaku.Settings() != settings {
		t.Fatal("part or fullscreen replacement reset process settings")
	}
	wantCalls(t, h)
}

func TestDanmakuClosingRetiresPopupAndCapture(t *testing.T) {
	for _, closing := range []bool{true, false} {
		h := newDanmakuHarness(t)
		click(t, h, "Danmaku")
		slider := rect(t, h, "Danmaku opacity")
		h.ui.Press(slider.X+slider.W/2, slider.Y+slider.H/2)
		settings := h.view.Danmaku.Settings()
		h.player.state.Closing = closing
		if !closing {
			h.player.state.Playback.State = playback.Closed
		}
		h.ui.Frame()
		wantPopup(t, h, "")
		if !access(t, h, "Danmaku").Disabled {
			t.Fatal("closing left the settings trigger enabled")
		}
		h.ui.Move(slider.X+slider.W, slider.Y)
		h.ui.Release(slider.X+slider.W, slider.Y)
		click(t, h, "Danmaku")
		for _, key := range []gui.Key{gui.KeySpace, gui.KeyRight, gui.KeyF, gui.KeyEscape} {
			h.ui.Key(0, key)
		}
		if h.view.Danmaku.Settings() != settings || h.view.danmakuOpen || h.full {
			t.Fatal("closing settings or capture remained active")
		}
		wantCalls(t, h)
	}
}

func TestDanmakuClosedChildAndOptionalMount(t *testing.T) {
	h := newDanmakuHarness(t)
	click(t, h, "Danmaku")
	h.view.Danmaku.Close()
	h.ui.Frame()
	settings := h.view.Danmaku.Settings()
	for _, label := range []string{"Show danmaku", "Danmaku opacity", "Danmaku text size", "Danmaku speed"} {
		if !access(t, h, label).Disabled {
			t.Fatalf("closed renderer left %s enabled", label)
		}
		click(t, h, label)
	}
	if h.view.Danmaku.Settings() != settings {
		t.Fatal("closed renderer changed settings")
	}
	h.view.Danmaku = nil
	h.ui.Frame()
	wantPopup(t, h, "")
	for _, label := range []string{"Danmaku", "Danmaku overlay", "Show danmaku"} {
		if _, found := h.ui.Find(label); found {
			t.Fatalf("optional view removal retained %s", label)
		}
	}
	wantCalls(t, h)
}
