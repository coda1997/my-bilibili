// Package desktop presents the fixture preview without owning playback or windows.
package desktop

import (
	"errors"
	"fmt"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

const overlayDelay = 3 * time.Second

// View is UI-thread-only and lives for the window's lifetime. Composition supplies
// the native fullscreen state and positive platform double-click interval. Now
// may be nil to use time.Now; tests inject their clock. Render pumps source and
// playback events on the UI thread before rebuilding the view.
type View struct {
	Player              app.Player
	Surface             func(*gui.Context, float32, float32)
	Cover               func(*gui.Context, string)
	Fullscreen          func() bool
	SetFullscreen       func(bool)
	Now                 func() time.Time
	DoubleClickInterval time.Duration

	lastActivity         time.Time
	pendingClick         time.Time
	suppressClick        bool
	pointerX, pointerY   float32
	pointerOver          bool
	generation           uint64
	seek, volume         sliderState
	partsOpen, speedOpen bool
	controlsHeld         bool
	lastError            playback.ErrorCode
}

type sliderState struct {
	value      float64
	generation uint64
	pressed    bool
}

func (s *sliderState) sample(generation uint64, value float64) {
	if generation != s.generation {
		s.pressed = false
	}
	if !s.pressed {
		s.value = value
	}
	s.generation = generation
}

func (v *View) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

func (v *View) active() { v.lastActivity = v.now() }

func (v *View) action(c *gui.Context, fn func() error) {
	v.pendingClick = time.Time{}
	v.lastError = playback.NoError
	if err := fn(); err != nil {
		var code playback.ErrorCode
		if errors.As(err, &code) {
			v.lastError = code
		} else {
			v.lastError = playback.Media
		}
	}
	v.active()
	c.Invalidate()
}

func (v *View) fullscreen(c *gui.Context, enabled bool) {
	v.pendingClick = time.Time{}
	v.SetFullscreen(enabled)
	v.active()
	c.Invalidate()
}

func (v *View) overlayVisible(s app.State) bool {
	return s.Closing || s.ResolvingPart != "" || s.Playback.State != playback.Playing || v.controlsHeld || v.seek.pressed || v.volume.pressed || v.partsOpen || v.speedOpen || v.now().Sub(v.lastActivity) < overlayDelay
}

func (v *View) videoInput(c *gui.Context, s app.State, video gui.Element) {
	if closed(s) {
		v.pendingClick = time.Time{}
		v.suppressClick = false
		return
	}
	x, y, over := video.PointerPosition()
	if over && (!v.pointerOver || x != v.pointerX || y != v.pointerY) {
		v.active()
	}
	v.pointerX, v.pointerY, v.pointerOver = x, y, over
	clicked, doubled := video.Clicked(), video.DoubleClicked()
	if v.suppressClick {
		if clicked {
			v.suppressClick = false
		}
	} else if doubled {
		v.suppressClick = !clicked // native double-click arrives on the second press
		v.fullscreen(c, !v.Fullscreen())
	} else if clicked && usable(s) {
		v.pendingClick = v.now().Add(v.DoubleClickInterval)
		v.active()
	}
	if !usable(s) {
		v.pendingClick = time.Time{}
	}
	if !v.pendingClick.IsZero() {
		remaining := v.pendingClick.Sub(v.now())
		if remaining <= 0 {
			v.action(c, v.Player.Toggle)
		} else {
			c.After(remaining)
		}
	}
}

func (v *View) shortcuts(c *gui.Context, s app.State) {
	if closed(s) {
		return
	}
	// Built after the controls, so their focused native input handlers and popup
	// scopes get first refusal (slider arrows, button Space, text input, Escape).
	if c.Shortcut(0, gui.KeyEscape) {
		if v.partsOpen || v.speedOpen {
			v.partsOpen, v.speedOpen = false, false
			v.active()
			c.Invalidate()
		} else if v.Fullscreen() {
			v.fullscreen(c, false)
		}
	}
	for _, key := range []gui.Key{gui.KeySpace, gui.KeyM, gui.KeyF} {
		if !c.Shortcut(0, key) || c.ShortcutRepeated() {
			continue
		}
		switch key {
		case gui.KeySpace:
			if usable(s) {
				v.action(c, v.Player.Toggle)
			}
		case gui.KeyM:
			if usable(s) {
				v.action(c, v.Player.ToggleMute)
			}
		case gui.KeyF:
			v.fullscreen(c, !v.Fullscreen())
		}
	}
	for _, step := range []struct {
		key   gui.Key
		delta time.Duration
	}{{gui.KeyLeft, -5 * time.Second}, {gui.KeyRight, 5 * time.Second}} {
		if c.Shortcut(0, step.key) && seekable(s) {
			position := max(time.Duration(0), min(s.Playback.Duration, s.Playback.Position+step.delta))
			v.action(c, func() error { return v.Player.Seek(position) })
		}
	}
	for _, step := range []struct {
		key   gui.Key
		delta float64
	}{{gui.KeyDown, -.05}, {gui.KeyUp, .05}} {
		if c.Shortcut(0, step.key) && usable(s) {
			level := max(0., min(1., s.Volume+step.delta))
			v.action(c, func() error { return v.Player.SetVolume(level) })
		}
	}
}

func closed(s app.State) bool {
	return s.Closing || s.Playback.State == playback.Closed
}
func partsReady(s app.State) bool {
	return s.DetailPhase == app.DetailReady && !closed(s)
}
func usable(s app.State) bool {
	return partsReady(s) && (s.ResolvingPart != "" || s.Playback.State != playback.Idle)
}
func seekable(s app.State) bool {
	return partsReady(s) && s.Playback.Duration > 0 && (s.Playback.State == playback.Ready || s.Playback.State == playback.Paused || s.Playback.State == playback.Playing)
}
func partTitle(s app.State, id catalog.PartID) string {
	for i, p := range s.Video.Parts {
		if p.ID == id {
			return fmt.Sprintf("P%d · %s", i+1, p.Title)
		}
	}
	return "No part loaded"
}
func clockText(d time.Duration) string {
	seconds := int(max(time.Duration(0), d) / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}
