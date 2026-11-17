package mpv

import (
	"image"
	"testing"

	gui "github.com/egoist/mygo/ui"
)

func TestDrawSizedPreservesBuffersAndNonblockingAcquisition(t *testing.T) {
	e := &engine{frames: [2]*image.RGBA{frame(), frame()}, pending: true}
	original := e.frames
	drawW, drawH := float32(800), float32(450)
	tt := gui.NewTester(func(c *gui.Context) { DrawSized(c, e, drawW, drawH) }, 1000, 700)
	assertSize := func(w, h float32) {
		t.Helper()
		r, ok := tt.Find("Synthetic replacement fixture video")
		if !ok || r.W != w || r.H != h {
			t.Fatalf("surface=%+v found=%v want=%vx%v", r, ok, w, h)
		}
	}
	assertSize(800, 450)
	if e.pending || e.front != 1 || e.bitmap == nil {
		t.Fatal("pending frame not consumed")
	}
	drawW, drawH = 320, 180
	e.framesMu.Lock()
	tt.Frame() // The UI thread must use the previous bitmap while rendering owns the lock.
	e.framesMu.Unlock()
	assertSize(320, 180)
	if e.frames != original || e.frames[0].Rect != image.Rect(0, 0, 640, 360) || e.frames[0].Stride != 640*4 {
		t.Fatal("display sizing changed native buffers")
	}
	legacy := gui.NewTester(func(c *gui.Context) { Draw(c, e) }, 1000, 700)
	r, ok := legacy.Find("Synthetic replacement fixture video")
	if !ok || r.W != 640 || r.H != 360 {
		t.Fatalf("legacy=%+v", r)
	}
}

func TestDrawSizedWithoutFrameOrEngine(t *testing.T) {
	for _, e := range []*engine{nil, {}} {
		tt := gui.NewTester(func(c *gui.Context) {
			if e == nil {
				DrawSized(c, nil, 320, 180)
			} else {
				DrawSized(c, e, 320, 180)
			}
		}, 500, 400)
		if _, ok := tt.Find("Synthetic replacement fixture video"); ok {
			t.Fatal("image without a frame")
		}
	}
}
