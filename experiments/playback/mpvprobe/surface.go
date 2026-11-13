package mpvprobe

import (
	"image"
	"runtime"
	"time"
	"unsafe"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

func frame() *image.RGBA {
	// Align the native target to 64 bytes; 640*4 stride is also 64-aligned.
	b := make([]byte, width*height*4+63)
	offset := int((64 - uintptr(unsafe.Pointer(&b[0]))%64) % 64)
	return &image.RGBA{Pix: b[offset : offset+width*height*4], Stride: width * 4, Rect: image.Rect(0, 0, width, height)}
}

func (e *engine) renderLoop(a *nativeAPI, core uintptr, stop <-chan struct{}, done chan<- struct{}, created chan<- bool) {
	workers.Add(1)
	defer func() { workers.Add(-1); close(done) }()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var render uintptr
	sw := []byte{'s', 'w', 0}
	params := []renderParam{{Kind: 1, Data: unsafe.Pointer(&sw[0])}, {}}
	result := a.renderCreate(&render, core, unsafe.Pointer(&params[0]))
	runtime.KeepAlive(sw)
	runtime.KeepAlive(params)
	if result < 0 || render == 0 {
		created <- false
		return
	}
	renderers.Add(1)
	defer func() { a.renderFree(render); renderers.Add(-1) }()
	e.framesMu.Lock()
	e.frames = [2]*image.RGBA{frame(), frame()}
	e.framesMu.Unlock()
	created <- true
	// No callbacks are registered: bounded polling avoids permanent purego
	// callback slots and retained per-session callback state entirely.
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()
	discard := false
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if a.renderUpdate(render)&1 == 0 {
			continue
		}
		if discard {
			// A failed target must not strand the VO during control-side teardown.
			// The public skip parameter consumes the frame without using pixels.
			skip, block := int32(1), int32(0)
			params := []renderParam{{Kind: 13, Data: unsafe.Pointer(&skip)}, {Kind: 12, Data: unsafe.Pointer(&block)}, {}}
			if a.render(render, unsafe.Pointer(&params[0])) < 0 {
				e.fail(contract.Media)
			}
			runtime.KeepAlive(params)
			continue
		}
		e.framesMu.Lock()
		target := e.frames[1-e.front]
		size, stride, block := [2]int32{width, height}, uintptr(width*4), int32(0)
		format := []byte{'r', 'g', 'b', '0', 0}
		params := []renderParam{{Kind: 17, Data: unsafe.Pointer(&size[0])},
			{Kind: 18, Data: unsafe.Pointer(&format[0])}, {Kind: 19, Data: unsafe.Pointer(&stride)},
			{Kind: 20, Data: unsafe.Pointer(&target.Pix[0])}, {Kind: 12, Data: unsafe.Pointer(&block)}, {}}
		result := a.render(render, unsafe.Pointer(&params[0]))
		runtime.KeepAlive(params)
		runtime.KeepAlive(target)
		if result < 0 {
			e.framesMu.Unlock()
			e.fail(contract.Media)
			discard = true
			continue
		}
		// rgb0's fourth byte is unspecified; mygo RGBA needs opaque alpha.
		for i := 3; i < len(target.Pix); i += 4 {
			target.Pix[i] = 255
		}
		e.pending = true
		e.framesMu.Unlock()
	}
}

// Draw is UI-thread-only. It never waits for native rendering or decoding.
func Draw(c *gui.Context, plain contract.Engine) {
	e, ok := plain.(*engine)
	if !ok {
		return
	}
	if e.framesMu.TryLock() {
		if e.pending {
			e.front = 1 - e.front
			if e.bitmap == nil {
				e.bitmap = gui.NewBitmap(e.frames[e.front])
			} else {
				e.bitmap.Update(e.frames[e.front])
			}
			e.pending = false
		}
		e.framesMu.Unlock()
	}
	if e.bitmap != nil {
		gui.Image(c.Key("replacement-video"), e.bitmap).Width(width).Height(height).Fit(gui.Contain).Label("Synthetic replacement fixture video")
	}
}
