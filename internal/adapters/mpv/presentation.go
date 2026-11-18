package mpv

import (
	"math"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/playback"
)

func presentationSize(width, height float32) bool {
	return width > 0 && height > 0 && !math.IsInf(float64(width), 0) &&
		!math.IsInf(float64(height), 0) && !math.IsNaN(float64(width)) && !math.IsNaN(float64(height))
}

// Bitmap creation belongs only to consumption of a successful pending render.
// Capturing that bitmap here ties a retained paint callback to its own engine.
func (e *engine) presentationCallback(prepared *gui.Bitmap) func(*gui.Painter, gui.Rect) {
	return func(_ *gui.Painter, r gui.Rect) {
		if !presentationSize(r.W, r.H) {
			return
		}
		e.mu.RLock()
		if !e.closed && prepared != nil && e.bitmap == prepared {
			e.picturePresented = true
		}
		e.mu.RUnlock()
	}
}

// PicturePresented is UI-thread-only. It proves that this engine's prepared
// bitmap reached mygo's image paint pass, not compositor completion or audio.
// Consumers must separately establish current-generation and visible-window
// readiness. No native state or pixels are retained by this observation.
func PicturePresented(plain playback.Engine) bool {
	e, ok := plain.(*engine)
	if !ok || e == nil {
		return false
	}
	e.mu.RLock()
	closed := e.closed
	e.mu.RUnlock()
	return !closed && e.picturePresented
}
