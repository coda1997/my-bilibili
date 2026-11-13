package mpvprobe

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"
	"unsafe"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

type fakeNative struct {
	a        *nativeAPI
	events   chan *event
	sent     chan uint64
	empty    event
	mu       sync.Mutex
	order    []string
	release  <-chan struct{}
	rendered chan struct{}
}

func fake() *fakeNative {
	f := &fakeNative{events: make(chan *event, 300), sent: make(chan uint64, 40), rendered: make(chan struct{}, 20)}
	f.a = &nativeAPI{
		create: func() uintptr { return 1 }, option: func(uintptr, string, string) int32 { return 0 },
		optionNode: func(uintptr, string, int32, unsafe.Pointer) int32 { return 0 },
		getProperty: func(_ uintptr, _ string, _ int32, p unsafe.Pointer) int32 {
			*(*nodeValue)(p) = nodeValue{Format: 7}
			return -1
		},
		freeNode:   func(unsafe.Pointer) {},
		initialize: func(uintptr) int32 { return 0 }, observe: func(uintptr, uint64, string, int32) int32 { return 0 },
		command: func(_ uintptr, id uint64, p unsafe.Pointer) int32 {
			args := commandArgs(p)
			if len(args) == 3 && args[0] == "set" && args[1] == "vid" && args[2] == "no" {
				f.events <- &event{ID: 5, Reply: id}
				return 0
			}
			f.sent <- id
			return 0
		},
		wait: func(_ uintptr, timeout float64) unsafe.Pointer {
			if timeout > 0 {
				select {
				case ev := <-f.events:
					return unsafe.Pointer(ev)
				case <-time.After(time.Duration(timeout * float64(time.Second))):
					return unsafe.Pointer(&f.empty)
				}
			}
			select {
			case ev := <-f.events:
				return unsafe.Pointer(ev)
			default:
				return unsafe.Pointer(&f.empty)
			}
		},
		destroy: func(uintptr) { f.add("destroy") }, unload: func() { f.add("unload") },
		renderCreate: func(result *uintptr, _ uintptr, _ unsafe.Pointer) int32 { *result = 2; return 0 },
		renderUpdate: func(uintptr) uint64 { return 1 },
		render: func(_ uintptr, p unsafe.Pointer) int32 {
			params := unsafe.Slice((*renderParam)(p), 6)
			pixels := unsafe.Slice((*byte)(params[3].Data), width*height*4)
			for i := range pixels {
				pixels[i] = 33
			}
			select {
			case f.rendered <- struct{}{}:
			default:
			}
			return 0
		},
		renderFree: func(uintptr) {
			if f.release != nil {
				<-f.release
			}
			f.add("render-free")
		},
	}
	return f
}

func (f *fakeNative) add(s string) { f.mu.Lock(); f.order = append(f.order, s); f.mu.Unlock() }

func openFake(t *testing.T, f *fakeNative) *engine {
	t.Helper()
	plain, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), contract.Input{FilePath: "synthetic.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	e := plain.(*engine)
	t.Cleanup(func() { _ = e.Close() })
	select {
	case <-f.sent:
	case <-time.After(time.Second):
		t.Fatal("load was not queued")
	}
	return e
}

func await(t *testing.T, predicate func() bool) {
	t.Helper()
	end := time.Now().Add(time.Second)
	for !predicate() {
		if time.Now().After(end) {
			t.Fatal("condition not observed")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestABIAndLoaderSafeRejection(t *testing.T) {
	if unsafe.Sizeof(event{}) != 24 || unsafe.Offsetof(event{}.Data) != 16 || unsafe.Sizeof(property{}) != 24 || unsafe.Offsetof(property{}.Data) != 16 || unsafe.Sizeof(renderParam{}) != 16 {
		t.Fatal("unexpected C ABI layout")
	}
	if _, err := load("/absent-synthetic-libmpv.dylib"); err != contract.Unsupported {
		t.Fatal(err)
	}
	called := false
	o := Opener{loader: func(string) (*nativeAPI, error) { called = true; return nil, errors.New("must not load") }}
	for _, in := range []contract.Input{{}, {FilePath: "synthetic", Headers: map[string]string{"X-Test": "value"}}, {URL: "https://example.invalid/video"}} {
		if _, err := o.Open(context.Background(), in); err == nil {
			t.Fatal("invalid/unsupported input accepted")
		}
	}
	if called {
		t.Fatal("native loader invoked for rejected input")
	}
}

func TestTransportInputAcceptedWithoutDroppingFields(t *testing.T) {
	f := fake()
	entered, release := make(chan struct{}), make(chan struct{})
	in := contract.Input{URL: "http://127.0.0.1/video", AudioURL: "http://127.0.0.1/audio", Headers: map[string]string{"X-Test": "one,two"}}
	e, err := (Opener{loader: func(string) (*nativeAPI, error) { close(entered); <-release; return f.a, nil }}).Open(context.Background(), in)
	if err != nil {
		t.Fatalf("valid transport input rejected: %v", err)
	}
	<-entered
	// Mutating caller-owned metadata after Open must not race native setup.
	in.Headers["X-Test"] = "changed"
	close(release)
	defer e.Close()
}

func TestOpenIsAsyncAndCloseWaitsForRealDisposal(t *testing.T) {
	f := fake()
	blocked, entered := make(chan struct{}), make(chan struct{})
	plain, err := (Opener{loader: func(string) (*nativeAPI, error) { close(entered); <-blocked; return f.a, nil }}).Open(context.Background(), contract.Input{FilePath: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	e := plain.(*engine)
	<-entered
	if e.Snapshot().State != contract.Loading {
		t.Fatal("Open waited for initialization")
	}
	finished := make(chan struct{})
	go func() { _ = e.Close(); close(finished) }()
	select {
	case <-finished:
		t.Fatal("Close returned with initialization owned")
	case <-time.After(20 * time.Millisecond):
	}
	close(blocked)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Close failed to join")
	}
	if Owned() != (Resources{}) {
		t.Fatal(Owned())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.order) != 3 || f.order[0] != "render-free" || f.order[1] != "destroy" || f.order[2] != "unload" {
		t.Fatal(f.order)
	}
	if e.Command(contract.Command{Kind: contract.Play}) != contract.IsClosed {
		t.Fatal("closed command accepted")
	}
	_ = e.Close()
}

func TestSaturatedQueueCannotSuppressShutdownAndRenderIsFreedFirst(t *testing.T) {
	f := fake()
	release := make(chan struct{})
	f.release = release
	e := openFake(t, f)
	for i := 0; i < 32; i++ {
		if e.Command(contract.Command{Kind: contract.Play}) != nil {
			t.Fatal(i)
		}
	}
	if e.Command(contract.Command{Kind: contract.Play}) != contract.Media {
		t.Fatal("queue overflow not rejected")
	}
	finished := make(chan struct{})
	go func() { _ = e.Close(); close(finished) }()
	select {
	case <-finished:
		t.Fatal("Close returned while render was owned")
	case <-time.After(20 * time.Millisecond):
	}
	if Owned().Renderers != 1 || Owned().Cores != 1 {
		t.Fatal(Owned())
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("full queue suppressed stop")
	}
	if Owned() != (Resources{}) {
		t.Fatal(Owned())
	}
}

func TestAsyncOrderingAndNativeFailureDisposal(t *testing.T) {
	f := fake()
	e := openFake(t, f)
	_ = e.Command(contract.Command{Kind: contract.Play})
	select {
	case id := <-f.sent:
		t.Fatalf("command %d overtook load reply", id)
	case <-time.After(70 * time.Millisecond):
	}
	f.events <- &event{ID: 5, Reply: 1}
	select {
	case id := <-f.sent:
		if id != 2 {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("reply did not release next command")
	}
	f.events <- &event{ID: 5, Reply: 2, Error: -1}
	await(t, func() bool { return e.Snapshot().State == contract.Failed })
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("failure did not dispose native resources")
	}
	if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
		t.Fatal(e.Snapshot(), Owned())
	}
}

func TestPartialInitializationAndRenderFailuresDisposeOwnership(t *testing.T) {
	for _, phase := range []string{"initialize", "render-create", "render-frame", "required-option"} {
		t.Run(phase, func(t *testing.T) {
			f := fake()
			switch phase {
			case "initialize":
				f.a.initialize = func(uintptr) int32 { return -1 }
			case "render-create":
				f.a.renderCreate = func(*uintptr, uintptr, unsafe.Pointer) int32 { return -1 }
			case "render-frame":
				f.a.render = func(uintptr, unsafe.Pointer) int32 { return -1 }
			case "required-option":
				f.a.option = func(uintptr, string, string) int32 { return -5 }
			}
			plain, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(context.Background(), contract.Input{FilePath: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			e := plain.(*engine)
			defer e.Close()
			select {
			case <-e.done:
			case <-time.After(time.Second):
				t.Fatal("failure did not release all workers")
			}
			if e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
				t.Fatal(e.Snapshot(), Owned())
			}
		})
	}
}

func TestCompiledOutScriptOptionsAreDisabled(t *testing.T) {
	f := fake()
	f.a.option = func(_ uintptr, name, _ string) int32 {
		if name == "ytdl" || name == "load-scripts" {
			return -5
		}
		return 0
	}
	e := openFake(t, f)
	if e.Snapshot().Error != contract.NoError {
		t.Fatal(e.Snapshot())
	}
}

func TestFailureIsNotPublishedUntilNativeDisposalCompletes(t *testing.T) {
	f := fake()
	release := make(chan struct{})
	f.release = release
	entered := make(chan struct{})
	free := f.a.renderFree
	f.a.renderFree = func(handle uintptr) { close(entered); free(handle) }
	e := openFake(t, f)
	f.events <- &event{ID: 5, Reply: 1, Error: -1}
	// The native destructor is held at its entry. A terminal snapshot
	// must not imply that this still-owned surface/core has been released.
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("destructor not reached")
	}
	if e.Snapshot().State == contract.Failed || Owned().Renderers != 1 {
		close(release)
		t.Fatal("terminal failure published before disposal")
	}
	close(release)
	await(t, func() bool { return e.Snapshot().State == contract.Failed })
	if Owned() != (Resources{}) {
		t.Fatal(Owned())
	}
}

func TestDisplayedPixelsAreNotRecycledAndDrawDoesNotWait(t *testing.T) {
	f := fake()
	e := openFake(t, f)
	select {
	case <-f.rendered:
	case <-time.After(time.Second):
		t.Fatal("no frame")
	}
	tt := gui.NewTester(func(c *gui.Context) { Draw(c, e) }, 640, 360)
	tt.Frame()
	e.framesMu.Lock()
	front := e.front
	if e.frames[front].Pix[3] != 255 {
		t.Fatal("rgb0 garbage treated as alpha")
	}
	e.frames[front].Pix[0] = 77
	e.framesMu.Unlock()
	select {
	case <-f.rendered:
	case <-time.After(time.Second):
		t.Fatal("no next frame")
	}
	e.framesMu.Lock()
	if e.frames[front].Pix[0] != 77 {
		t.Fatal("renderer overwrote retained display buffer")
	}
	// Simulate a renderer waiting inside native code while owning its target.
	finished := make(chan struct{})
	go func() { tt.Frame(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Draw waited for render mutex")
	}
	e.framesMu.Unlock()
	if unsafe.Pointer(&e.frames[0].Pix[0]) == unsafe.Pointer(&e.frames[1].Pix[0]) || e.frames[0].Rect != image.Rect(0, 0, width, height) {
		t.Fatal("bad frame ownership")
	}
}
