package desktop

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type call struct {
	name  string
	value float64
	part  catalog.PartID
}
type fakePlayer struct {
	state app.State
	calls []call
	ticks int
	err   error
}

func (p *fakePlayer) State() app.State { return p.state }
func (p *fakePlayer) record(name string) error {
	p.calls = append(p.calls, call{name: name})
	return p.err
}
func (p *fakePlayer) Start(context.Context) error       { return p.record("start") }
func (p *fakePlayer) RetryDetail() error                { return p.record("retry-detail") }
func (p *fakePlayer) SelectQuality(app.QualityID) error { return nil }
func (p *fakePlayer) SelectPart(id catalog.PartID) error {
	p.calls = append(p.calls, call{name: "part", part: id})
	return p.err
}
func (p *fakePlayer) NextPart() error       { return p.record("next") }
func (p *fakePlayer) RetrySelection() error { return p.record("retry-selection") }
func (p *fakePlayer) RetryPlayback() error  { return p.record("retry-playback") }
func (p *fakePlayer) Play() error           { return p.record("play") }
func (p *fakePlayer) Pause() error          { return p.record("pause") }
func (p *fakePlayer) Toggle() error         { return p.record("toggle") }
func (p *fakePlayer) Seek(d time.Duration) error {
	p.calls = append(p.calls, call{name: "seek", value: d.Seconds()})
	return p.err
}
func (p *fakePlayer) SetVolume(x float64) error {
	p.calls = append(p.calls, call{name: "volume", value: x})
	return p.err
}
func (p *fakePlayer) SetRate(x float64) error {
	p.calls = append(p.calls, call{name: "rate", value: x})
	return p.err
}
func (p *fakePlayer) ToggleMute() error { return p.record("mute") }
func (p *fakePlayer) Tick()             { p.ticks++ }
func (p *fakePlayer) Close() error      { return p.record("close") }

type harness struct {
	player             *fakePlayer
	view               *View
	ui                 *gui.Tester
	full               bool
	fullscreenCalls    int
	surfaceW, surfaceH float32
}

func newHarness() *harness {
	h := &harness{player: &fakePlayer{state: app.State{
		DetailPhase:  app.DetailReady,
		Video:        catalog.Video{ID: "preview-video", Title: "Fixture preview", Creator: "Preview creator", Description: "Original local fixture description", CoverID: "preview-cover", Parts: []catalog.Part{{ID: "part-1", Title: "First", Duration: 10 * time.Second}, {ID: "part-2", Title: "Second", Duration: 10 * time.Second}}},
		SelectedPart: "part-1", PlaybackPart: "part-1", Volume: .2, Rate: 1,
		Playback: playback.Snapshot{State: playback.Paused, Generation: 1, Duration: 10 * time.Second, Position: time.Second},
	}}}
	h.view = &View{Player: h.player, DoubleClickInterval: 500 * time.Millisecond, Fullscreen: func() bool { return h.full }, SetFullscreen: func(b bool) { h.full = b; h.fullscreenCalls++ }, Surface: func(c *gui.Context, w, hgt float32) {
		h.surfaceW, h.surfaceH = w, hgt
		gui.Box(c).Size(w, hgt).Label("Injected surface")
	}}
	h.view.Now = func() time.Time {
		if h.ui == nil {
			return time.Unix(0, 0)
		}
		return h.ui.Now()
	}
	h.ui = gui.NewTester(h.view.Render, 1120, 760)
	h.view.active()
	return h
}
func rect(t *testing.T, h *harness, label string) gui.Rect {
	t.Helper()
	r, ok := h.ui.Find(label)
	if !ok {
		t.Fatalf("missing %q; texts=%v", label, h.ui.Texts())
	}
	return r
}
func access(t *testing.T, h *harness, label string) gui.AccessInfo {
	t.Helper()
	a, ok := h.ui.Access(label)
	if !ok {
		t.Fatalf("missing accessibility %q", label)
	}
	return a
}
func click(t *testing.T, h *harness, label string) {
	t.Helper()
	if err := h.ui.Click(label); err != nil {
		t.Fatal(err)
	}
}
func wantCalls(t *testing.T, h *harness, want ...call) {
	t.Helper()
	if !reflect.DeepEqual(h.player.calls, want) {
		t.Fatalf("calls=%+v want=%+v", h.player.calls, want)
	}
}
func videoClick(t *testing.T, h *harness) {
	t.Helper()
	r := rect(t, h, "Video surface")
	h.ui.ClickAt(r.X+r.W/2, r.Y+30)
}

func TestResponsiveLayoutAndFullscreenPreservePlayer(t *testing.T) {
	h := newHarness()
	before := h.player.state
	for _, size := range []struct {
		w, h int
		wide bool
	}{{1120, 760, true}, {1048, 760, true}, {1047, 760, false}, {800, 600, false}} {
		h.ui.SetSize(size.w, size.h)
		video, part := rect(t, h, "Video surface"), rect(t, h, "P1 · First · playing")
		if size.wide {
			if part.X < video.X+video.W {
				t.Fatalf("wide part=%+v video=%+v", part, video)
			}
		} else if part.Y < video.Y+video.H {
			t.Fatalf("narrow part=%+v video=%+v", part, video)
		}
		if video.W/video.H != float32(16)/9 {
			t.Fatalf("ratio=%v", video.W/video.H)
		}
		for _, label := range []string{"Fixture preview", "Preview creator", "Original local fixture description", "Cover for Fixture preview"} {
			rect(t, h, label)
		}
		for _, label := range []string{"Play", "Next", "Seek position", "Playback speed", "Mute", "Volume", "Fullscreen"} {
			a := access(t, h, label)
			if a.Bounds.W < 28 || a.Bounds.H < 28 || !a.Focusable {
				t.Fatalf("control %s: %+v", label, a)
			}
			if a.Bounds.X+a.Bounds.W > video.X+video.W+1 {
				t.Fatalf("control %s exceeds video: %+v video=%+v", label, a.Bounds, video)
			}
		}
	}
	for range 2 {
		h.ui.Key(0, gui.KeyF)
		if !h.full {
			t.Fatal("fullscreen not requested")
		}
		if h.surfaceW != 800 || h.surfaceH != 450 {
			t.Fatalf("fullscreen fit=%vx%v", h.surfaceW, h.surfaceH)
		}
		h.ui.Key(0, gui.KeyEscape)
		if h.full {
			t.Fatal("escape did not exit")
		}
	}
	h.full = true
	h.ui.Frame()
	h.full = false
	h.ui.Frame()
	if !reflect.DeepEqual(before, h.player.state) || len(h.player.calls) != 0 || h.fullscreenCalls != 4 {
		t.Fatal("layout/fullscreen changed player")
	}
}

func TestControlsMenusAndErrors(t *testing.T) {
	h := newHarness()
	click(t, h, "Play")
	click(t, h, "Next")
	click(t, h, "Mute")
	wantCalls(t, h, call{name: "toggle"}, call{name: "next"}, call{name: "mute"})
	click(t, h, "Parts")
	click(t, h, "P2 · Second")
	if h.view.partsOpen || h.player.calls[len(h.player.calls)-1].part != "part-2" {
		t.Fatal("part selection")
	}
	for _, rate := range []struct {
		label string
		value float64
	}{{"0.5×", .5}, {"0.75×", .75}, {"1×", 1}, {"1.25×", 1.25}, {"1.5×", 1.5}, {"2×", 2}} {
		click(t, h, "Playback speed")
		click(t, h, "Set speed "+rate.label)
		if h.view.speedOpen || h.player.calls[len(h.player.calls)-1] != (call{name: "rate", value: rate.value}) {
			t.Fatalf("rate %v: %+v", rate, h.player.calls)
		}
	}
	count := len(h.player.calls)
	h.ui.Advance(time.Second)
	if len(h.player.calls) != count {
		t.Fatal("controls bubbled to video")
	}
	h.player.state.SelectedPart = "part-2"
	h.player.state.SelectionError = catalog.Unavailable
	h.player.state.Playback.State = playback.Failed
	h.player.state.Playback.Error = playback.Unsupported
	h.ui.Frame()
	rect(t, h, "Playing: P1 · First")
	rect(t, h, "P2 · Second · requested")
	if !access(t, h, "Next").Disabled {
		t.Fatal("last part next enabled")
	}
	click(t, h, "Retry selection")
	click(t, h, "Retry playback")
	if h.player.calls[len(h.player.calls)-2].name != "retry-selection" || h.player.calls[len(h.player.calls)-1].name != "retry-playback" {
		t.Fatal("retry targets")
	}
	h.player.err = errors.New("private locator must not be shown")
	click(t, h, "Play")
	rect(t, h, "Action unavailable · "+string(playback.Media))
	if h.ui.HasText("private locator must not be shown") {
		t.Fatal("raw error exposed")
	}
}

func TestSingleAndDoubleVideoClicks(t *testing.T) {
	t.Run("single delayed", func(t *testing.T) {
		h := newHarness()
		videoClick(t, h)
		h.ui.Advance(499 * time.Millisecond)
		wantCalls(t, h)
		h.ui.Advance(20 * time.Millisecond)
		wantCalls(t, h, call{name: "toggle"})
	})
	t.Run("double fullscreen only", func(t *testing.T) {
		h := newHarness()
		videoClick(t, h)
		h.ui.Advance(100 * time.Millisecond)
		videoClick(t, h)
		h.ui.Advance(time.Second)
		wantCalls(t, h)
		if !h.full || h.fullscreenCalls != 1 {
			t.Fatalf("full=%v calls=%d", h.full, h.fullscreenCalls)
		}
	})
	t.Run("control cancels pending", func(t *testing.T) {
		h := newHarness()
		videoClick(t, h)
		click(t, h, "Mute")
		h.ui.Advance(time.Second)
		wantCalls(t, h, call{name: "mute"})
	})
	t.Run("popup dismiss does not toggle", func(t *testing.T) {
		h := newHarness()
		click(t, h, "Parts")
		videoClick(t, h)
		h.ui.Advance(time.Second)
		wantCalls(t, h)
		if h.view.partsOpen {
			t.Fatal("popup remains open")
		}
	})
}

func TestKeyboardRoutingAndPopupEscape(t *testing.T) {
	h := newHarness()
	for _, key := range []gui.Key{gui.KeySpace, gui.KeyLeft, gui.KeyRight, gui.KeyDown, gui.KeyUp, gui.KeyM} {
		h.ui.Key(0, key)
	}
	wantCalls(t, h, call{name: "toggle"}, call{name: "seek", value: 0}, call{name: "seek", value: 6}, call{name: "volume", value: h.player.state.Volume - .05}, call{name: "volume", value: .25}, call{name: "mute"})
	h.player.calls = nil
	click(t, h, "Play")
	h.ui.Key(0, gui.KeySpace)
	wantCalls(t, h, call{name: "toggle"}, call{name: "toggle"})
	h.player.calls = nil
	click(t, h, "Volume")
	h.ui.Key(0, gui.KeyRight)
	if len(h.player.calls) != 2 || h.player.calls[1].name != "volume" {
		t.Fatalf("focused slider key stolen: %+v", h.player.calls)
	}
	h.full = true
	h.ui.Frame()
	click(t, h, "Parts")
	h.ui.Key(0, gui.KeyEscape)
	if h.view.partsOpen || !h.full {
		t.Fatal("escape must dismiss popup first")
	}
	h.ui.Key(0, gui.KeyEscape)
	if h.full {
		t.Fatal("second escape must exit fullscreen")
	}
}

func TestSliderCaptureSamplesAndReplacement(t *testing.T) {
	for _, label := range []string{"Seek position", "Volume"} {
		t.Run(label, func(t *testing.T) {
			h := newHarness()
			r := rect(t, h, label)
			x, y := r.X+r.W/2, r.Y+r.H/2
			h.ui.Press(x, y)
			h.ui.Move(x, y+100)
			if len(h.player.calls) != 1 {
				t.Fatalf("press=%+v", h.player.calls)
			}
			h.player.state.Playback.Position = 2 * time.Second
			h.player.state.Volume = .8
			h.ui.Frame()
			h.ui.Frame()
			held := 5.
			if label == "Volume" {
				held = .5
			}
			if a := access(t, h, label); a.Now != held || len(h.player.calls) != 1 {
				t.Fatalf("held=%v calls=%+v", a.Now, h.player.calls)
			}
			h.ui.Release(x, y+100)
			h.ui.Frame()
			want := 2.
			if label == "Volume" {
				want = .8
			}
			if a := access(t, h, label); a.Now != want || len(h.player.calls) != 1 {
				t.Fatalf("release=%v calls=%+v", a.Now, h.player.calls)
			}
			h.ui.Press(x, y)
			count := len(h.player.calls)
			h.player.state.Playback.Generation++
			h.player.state.Playback.Position = 3 * time.Second
			h.player.state.Volume = .7
			h.ui.Frame()
			h.ui.Move(x+20, y)
			h.ui.Release(x+20, y)
			h.ui.Frame()
			want = 3.
			if label == "Volume" {
				want = .7
			}
			if a := access(t, h, label); a.Now != want || len(h.player.calls) != count || h.view.seek.pressed || h.view.volume.pressed {
				t.Fatalf("replacement=%v calls=%+v count=%d", a.Now, h.player.calls, count)
			}
		})
	}
}

func TestOverlayClockAndState(t *testing.T) {
	for _, state := range []playback.State{playback.Playing, playback.Paused, playback.Loading, playback.Failed, playback.Ended} {
		t.Run(string(state), func(t *testing.T) {
			h := newHarness()
			h.player.state.Playback.State = state
			h.ui.Frame()
			h.ui.Advance(3100 * time.Millisecond)
			_, visible := h.ui.Find("Seek position")
			if visible != (state != playback.Playing) {
				t.Fatalf("visible=%v", visible)
			}
			if state == playback.Playing {
				r := rect(t, h, "Video surface")
				h.ui.Move(r.X+20, r.Y+20)
				h.ui.Frame()
				rect(t, h, "Seek position")
			}
		})
	}
	for _, keep := range []string{"hover", "focus", "drag", "popup"} {
		t.Run(keep, func(t *testing.T) {
			h := newHarness()
			h.player.state.Playback.State = playback.Playing
			h.ui.Frame()
			switch keep {
			case "hover":
				r := rect(t, h, "Play")
				h.ui.Move(r.X+r.W/2, r.Y+r.H/2)
			case "focus":
				click(t, h, "Play")
				h.ui.Move(0, 0)
			case "drag":
				r := rect(t, h, "Seek position")
				h.ui.Press(r.X+r.W/2, r.Y+r.H/2)
				h.ui.Move(0, 0)
			case "popup":
				click(t, h, "Parts")
				h.ui.Move(0, 0)
			}
			h.ui.Advance(3100 * time.Millisecond)
			rect(t, h, "Seek position")
		})
	}
}

func TestSampledUpdatesAndDisabledControls(t *testing.T) {
	h := newHarness()
	h.player.state.Playback.Position = 2500 * time.Millisecond
	h.player.state.Volume = .75
	h.player.state.Rate = 1.5
	h.ui.Frame()
	h.ui.Frame()
	wantCalls(t, h)
	if access(t, h, "Seek position").Now != 2.5 || access(t, h, "Volume").Now != .75 {
		t.Fatal("sample not presented")
	}
	for _, state := range []playback.State{playback.Idle, playback.Loading, playback.Ready, playback.Paused, playback.Playing, playback.Ended, playback.Failed, playback.Closed} {
		for _, duration := range []time.Duration{0, 10 * time.Second} {
			h.player.state.Playback.State = state
			h.player.state.Playback.Duration = duration
			h.ui.Frame()
			seekEnabled := duration > 0 && (state == playback.Ready || state == playback.Paused || state == playback.Playing)
			if access(t, h, "Seek position").Disabled == seekEnabled {
				t.Fatalf("seek disabled state=%s duration=%s", state, duration)
			}
			for _, label := range []string{"Play", "Mute", "Volume", "Playback speed"} {
				if access(t, h, label).Disabled != (state == playback.Idle || state == playback.Closed) {
					t.Fatalf("%s disabled in %s", label, state)
				}
			}
		}
	}
	wantCalls(t, h)
}

func TestShortcutClampsAndRepeatedToggle(t *testing.T) {
	h := newHarness()
	h.player.state.Playback.Position = 9 * time.Second
	h.player.state.Volume = .99
	h.ui.Frame()
	h.ui.Key(0, gui.KeyRight)
	h.ui.Key(0, gui.KeyUp)
	h.player.state.Playback.Position = time.Second
	h.player.state.Volume = .01
	h.ui.Frame()
	h.ui.Key(0, gui.KeyLeft)
	h.ui.Key(0, gui.KeyDown)
	wantCalls(t, h, call{name: "seek", value: 10}, call{name: "volume", value: 1}, call{name: "seek", value: 0}, call{name: "volume", value: 0})
	h.player.calls = nil
	h.ui.KeyRepeat(0, gui.KeySpace)
	h.ui.KeyRepeat(0, gui.KeyM)
	h.ui.KeyRepeat(0, gui.KeyF)
	wantCalls(t, h)
	if h.full {
		t.Fatal("key repeat toggled fullscreen")
	}
}
