package mpv

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/playback"
)

func pendingPicture(t *testing.T) *engine {
	t.Helper()
	f := fake()
	var rendered atomic.Bool
	f.a.renderUpdate = func(uintptr) uint64 {
		if rendered.CompareAndSwap(false, true) {
			return 1
		}
		return 0
	}
	e := openFake(t, f)
	await(t, func() bool {
		e.framesMu.Lock()
		defer e.framesMu.Unlock()
		return e.pending
	})
	return e
}

func TestTargetPictureRequiresActualRasterPaint(t *testing.T) {
	e := pendingPicture(t)
	if PicturePresented(e) {
		t.Fatal("pending native render was mistaken for a painted bitmap")
	}
	firstBuild, preparedObservation := true, true
	tt := gui.NewTester(func(c *gui.Context) {
		DrawTargetSized(c, e, 320, 180)
		if firstBuild {
			firstBuild, preparedObservation = false, PicturePresented(e)
		}
	}, 640, 360)
	if preparedObservation || !PicturePresented(e) {
		t.Fatal("presentation did not distinguish node construction from actual paint")
	}
	r, ok := tt.Find("Authorized target video")
	if !ok || r.W != 320 || r.H != 180 {
		t.Fatal("target size/label missing", r, ok)
	}
	if pixel := tt.Image().RGBAAt(int(r.X+r.W/2), int(r.Y+r.H/2)); pixel != (color.RGBA{R: 33, G: 33, B: 33, A: 255}) {
		t.Fatal("actual target raster did not contain the successful render", pixel)
	}
	legacy := gui.NewTester(func(c *gui.Context) { DrawTarget(c, e) }, 800, 600)
	r, ok = legacy.Find("Authorized target video")
	if !ok || r.W != width || r.H != height {
		t.Fatal("fixed-size target compatibility changed", r, ok)
	}
}

func TestPreparedOffscreenPictureDoesNotCountUntilPaint(t *testing.T) {
	e := pendingPicture(t)
	left := float32(-20000)
	tt := gui.NewTester(func(c *gui.Context) {
		gui.Column(c.Key("mount")).Absolute().Left(left).Top(0).Size(320, 180).Children(func() {
			DrawTargetSized(c, e, 320, 180)
		})
	}, 640, 360)
	if e.bitmap == nil || e.pending || PicturePresented(e) {
		t.Fatal("a prepared but culled image became picture evidence")
	}
	left = 0
	tt.Frame()
	if !PicturePresented(e) || tt.Image().RGBAAt(160, 90) != (color.RGBA{R: 33, G: 33, B: 33, A: 255}) {
		t.Fatal("visible prepared image did not paint")
	}
}

func TestSelectedTracksAndAdvancingTimeDoNotProvePicture(t *testing.T) {
	e := &engine{s: playback.Snapshot{Generation: 7, State: playback.Playing, Position: time.Second, Duration: 30 * time.Second, Rate: 1, Volume: 1}, tracks: Tracks{Video: true, Audio: true}}
	tt := gui.NewTester(func(c *gui.Context) { DrawTargetSized(c, e, 320, 180) }, 640, 360)
	e.s.Position += time.Second
	tt.Frame()
	if !Inspect(e).Video || !Inspect(e).Audio || e.Snapshot().Position <= time.Second {
		t.Fatal("no-picture control did not establish tracks/progress")
	}
	if PicturePresented(e) || e.bitmap != nil {
		t.Fatal("tracks and advancing time substituted for picture paint")
	}
}

func TestPresentationRejectsInvalidSizesAndRetainedCallbacks(t *testing.T) {
	for _, size := range [][2]float32{{0, 180}, {-1, 180}, {320, 0}, {320, -1}, {float32(math.NaN()), 180}, {320, float32(math.NaN())}, {float32(math.Inf(1)), 180}, {320, float32(math.Inf(1))}} {
		e := &engine{frames: [2]*image.RGBA{frame(), frame()}, pending: true}
		tt := gui.NewTester(func(c *gui.Context) { DrawTargetSized(c, e, size[0], size[1]) }, 640, 360)
		if _, ok := tt.Find("Authorized target video"); ok || e.bitmap != nil || !e.pending || PicturePresented(e) {
			t.Fatal("invalid presentation size consumed a frame or created picture evidence")
		}
	}
	e := pendingPicture(t)
	gui.NewTester(func(c *gui.Context) { DrawTargetSized(c, e, 320, 180) }, 640, 360)
	prepared := e.bitmap
	callback := e.presentationCallback(prepared)
	e.picturePresented = false
	for _, rect := range []gui.Rect{{}, {W: -1, H: 1}, {W: 1, H: float32(math.NaN())}, {W: float32(math.Inf(1)), H: 1}} {
		callback(nil, rect)
		if PicturePresented(e) {
			t.Fatal("invalid actual paint bounds became picture evidence")
		}
	}
	e.bitmap = gui.NewBitmap(frame())
	callback(nil, gui.Rect{W: 320, H: 180})
	if PicturePresented(e) {
		t.Fatal("retired bitmap callback marked its replacement")
	}
	e.bitmap = prepared
	callback(nil, gui.Rect{W: 320, H: 180})
	if !PicturePresented(e) {
		t.Fatal("owned callback did not mark its bitmap")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	callback(nil, gui.Rect{W: 320, H: 180})
	if PicturePresented(e) || e.picturePresented || e.bitmap != nil {
		t.Fatal("retained callback revived a closed engine")
	}
	if PicturePresented(&engine{s: playback.Snapshot{Generation: 7}}) {
		t.Fatal("new engine inherited a previous picture")
	}
}

type noPictureEngine struct{}

func (noPictureEngine) Snapshot() playback.Snapshot    { return playback.Snapshot{} }
func (noPictureEngine) Command(playback.Command) error { return nil }
func (noPictureEngine) Close() error                   { return nil }

func TestPicturePresentedRejectsNilOtherAndWorkerClosed(t *testing.T) {
	var typedNil *engine
	for _, plain := range []playback.Engine{nil, typedNil, noPictureEngine{}, &engine{closed: true, picturePresented: true}} {
		if PicturePresented(plain) {
			t.Fatal("nil, other or closed engine produced picture evidence")
		}
		gui.NewTester(func(c *gui.Context) { DrawTargetSized(c, plain, 320, 180) }, 640, 360)
	}
}

func TestWorkerClosedRejectsRetainedUnpaintedCallback(t *testing.T) {
	e := pendingPicture(t)
	gui.NewTester(func(c *gui.Context) {
		gui.Column(c.Key("offscreen")).Absolute().Left(-20000).Top(0).Size(320, 180).Children(func() {
			DrawTargetSized(c, e, 320, 180)
		})
	}, 640, 360)
	prepared := e.bitmap
	if prepared == nil || e.picturePresented {
		t.Fatal("worker-exit case did not prepare an unpainted bitmap")
	}
	callback := e.presentationCallback(prepared)
	// This is the existing failure path: run observes failure at its next
	// bounded poll, retires native owners, sets closed, then closes done.
	e.fail(playback.Media)
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("fake native worker did not finish its existing failure path")
	}
	if e.Snapshot().State != playback.Failed || e.bitmap != prepared {
		t.Fatal("worker-closed state before UI Close was not established")
	}
	callback(nil, gui.Rect{W: 320, H: 180})
	if PicturePresented(e) || e.picturePresented {
		t.Fatal("worker-closed engine accepted its retained paint callback")
	}
}
