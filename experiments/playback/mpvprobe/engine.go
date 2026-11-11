package mpvprobe

import (
	"context"
	"image"
	"math"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

const width, height = 640, 360

var cores, renderers, workers atomic.Int64

// Resources counts owned native objects and workers, not all OS allocations.
// Counts decrease only after the actual native destructor/worker returns.
type Resources struct{ Cores, Renderers, Workers int64 }

func Owned() Resources { return Resources{cores.Load(), renderers.Load(), workers.Load()} }

type Opener struct {
	Library string
	loader  func(string) (*nativeAPI, error)
}

func (o Opener) Open(ctx context.Context, in contract.Input) (contract.Engine, error) {
	if ctx == nil || in.Validate() != nil {
		return nil, contract.Invalid
	}
	if ctx.Err() != nil {
		return nil, contract.Canceled
	}
	// Gate one evaluates combined fixtures only. Broader transport capability
	// work remains RPL-002, after independently reviewed first-gate evidence.
	if len(in.Headers) != 0 || in.AudioURL != "" {
		return nil, contract.Unsupported
	}
	loader := o.loader
	if loader == nil {
		loader = load
	}
	e := &engine{commands: make(chan []string, 32), stop: make(chan struct{}), done: make(chan struct{}),
		s: contract.Snapshot{State: contract.Loading, Rate: 1, Volume: 1}}
	go e.run(ctx, in, o.Library, loader)
	return e, nil
}

type engine struct {
	mu       sync.RWMutex
	s        contract.Snapshot
	closed   bool
	failure  contract.ErrorCode
	once     sync.Once
	commands chan []string
	stop     chan struct{}
	done     chan struct{}

	// Two frames total. A render holds framesMu while writing the non-displayed
	// buffer; Draw uses TryLock and keeps the previous bitmap if rendering waits.
	// Bitmap.Update retains its pixels, so the displayed index is never written.
	framesMu sync.Mutex
	frames   [2]*image.RGBA
	front    int
	pending  bool
	bitmap   *gui.Bitmap // UI thread only
}

func (e *engine) Snapshot() contract.Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.s
}

func (e *engine) Command(c contract.Command) error {
	if err := c.Validate(); err != nil {
		return err
	}
	args := []string{"set", "pause", "yes"}
	switch c.Kind {
	case contract.Play:
		args[2] = "no"
	case contract.Seek:
		args = []string{"seek", strconv.FormatFloat(c.Position.Seconds(), 'f', 9, 64), "absolute+exact"}
	case contract.Volume:
		args = []string{"set", "volume", strconv.FormatFloat(c.Value*100, 'f', -1, 64)}
	case contract.Rate:
		args = []string{"set", "speed", strconv.FormatFloat(c.Value, 'f', -1, 64)}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return contract.IsClosed
	}
	select {
	case e.commands <- args:
		return nil
	default:
		return contract.Media
	}
}

func (e *engine) Close() error {
	e.once.Do(func() {
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()
		close(e.stop) // Out of band: a full command queue cannot suppress shutdown.
	})
	<-e.done // Never publish disposal while native objects/workers remain owned.
	e.mu.Lock()
	e.s.State, e.s.Error = contract.Closed, contract.NoError
	e.mu.Unlock()
	// Close and Draw are UI-thread operations. Worker buffer ownership ended.
	e.frames, e.bitmap = [2]*image.RGBA{}, nil
	return nil
}

func (e *engine) fail(code contract.ErrorCode) {
	e.mu.Lock()
	e.failure = code
	e.mu.Unlock()
}

func (e *engine) run(ctx context.Context, in contract.Input, path string, loader func(string) (*nativeAPI, error)) {
	workers.Add(1)
	defer func() {
		workers.Add(-1)
		e.mu.Lock()
		if e.failure != contract.NoError {
			e.s.State, e.s.Error = contract.Failed, e.failure
		}
		e.mu.Unlock()
		close(e.done)
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	a, err := loader(path)
	if err != nil {
		e.fail(contract.Unsupported)
		return
	}
	defer a.unload()
	core := a.create()
	if core == 0 {
		e.fail(contract.Media)
		return
	}
	cores.Add(1)
	defer func() { a.destroy(core); cores.Add(-1) }()
	// No user config, scripts, companion media discovery, player IPC or native
	// output window. All values are explicit and this build is fixture-only.
	for _, option := range [][2]string{{"config", "no"}, {"load-scripts", "no"}, {"ytdl", "no"},
		{"audio-file-auto", "no"}, {"sub-auto", "no"}, {"cover-art-auto", "no"},
		{"autoload-files", "no"}, {"input-default-bindings", "no"}, {"input-vo-keyboard", "no"},
		{"load-unsafe-playlists", "no"}, {"access-references", "no"}, {"demuxer-lavf-format", "mov"},
		{"terminal", "no"}, {"msg-level", "all=no"}, {"vo", "libmpv"}, {"hwdec", "no"},
		{"pause", "yes"}, {"idle", "yes"}, {"keep-open", "yes"}, {"network-timeout", "3"},
		{"osd-level", "0"}} {
		result := a.option(core, option[0], option[1])
		// In the pinned minimal build scripting is compiled out, so its disabling
		// options do not exist. Only OPTION_NOT_FOUND for these two switches is
		// equivalent to disabled; every other configuration error is fatal.
		absentScript := result == -5 && (option[0] == "load-scripts" || option[0] == "ytdl")
		if result < 0 && !absentScript {
			e.fail(contract.Media)
			return
		}
	}
	if a.initialize(core) < 0 {
		e.fail(contract.Media)
		return
	}
	properties := []struct {
		name string
		kind int32
	}{{"time-pos", 5}, {"duration", 5}, {"pause", 3}, {"volume", 5}, {"speed", 5}, {"eof-reached", 3}}
	for i, p := range properties {
		if a.observe(core, uint64(i+1), p.name, p.kind) < 0 {
			e.fail(contract.Media)
			return
		}
	}
	renderStop, renderDone := make(chan struct{}), make(chan struct{})
	created := make(chan bool, 1)
	go e.renderLoop(a, core, renderStop, renderDone, created)
	defer func() {
		// Async quit must be sent before freeing the surface; the control owner
		// never waits in a synchronous command while the render owner needs it.
		_ = a.send(core, 0, []string{"quit"})
		close(renderStop)
		<-renderDone
	}()
	if !<-created {
		e.fail(contract.Media)
		return
	}
	source := in.URL
	if source == "" {
		source = in.FilePath
	}
	id := uint64(1)
	if a.send(core, id, []string{"loadfile", source, "replace"}) < 0 {
		e.fail(contract.Media)
		return
	}
	pending, loaded, paused, commanded := id, false, true, false
	ticker := time.NewTicker(contract.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stop:
			return
		case <-ticker.C:
		}
		// No more than 256 immediate event records per poll. Process/copy each
		// numeric payload before the next wait_event invalidates native storage.
		for n := 0; ; n++ {
			p := a.wait(core, 0)
			if p == nil {
				e.fail(contract.Media)
				return
			}
			ev := *(*event)(p)
			if ev.ID == 0 {
				break
			}
			if n == 256 || ev.ID == 24 || ev.ID == 1 {
				e.fail(contract.Media)
				return
			}
			switch ev.ID {
			case 5: // command reply; dependent commands wait for their own ID.
				if ev.Reply == pending {
					if ev.Error < 0 {
						e.fail(contract.Media)
						return
					}
					pending = 0
				}
			case 7: // end-file: reason/error are the first two C ints.
				if ev.Data != nil && (*[2]int32)(ev.Data)[0] == 4 {
					e.fail(contract.Media)
					return
				}
			case 8:
				loaded = true
			case 22:
				if ev.Data != nil {
					p := *(*property)(ev.Data)
					if p.Data != nil {
						e.update(ev.Reply, p, &paused)
					}
				}
			}
		}
		e.mu.Lock()
		if e.failure != contract.NoError {
			e.mu.Unlock()
			return
		}
		if loaded && e.s.Duration > 0 && e.s.State != contract.Ended {
			e.s.State = contract.Playing
			if paused {
				e.s.State = contract.Ready
				if commanded {
					e.s.State = contract.Paused
				}
			}
		}
		e.mu.Unlock()
		if pending == 0 {
			select {
			case args := <-e.commands:
				if len(args) == 3 && args[0] == "set" && args[1] == "pause" {
					commanded = true
				}
				id++
				if a.send(core, id, args) < 0 {
					e.fail(contract.Media)
					return
				}
				pending = id
			default:
			}
		}
	}
}

func (e *engine) update(id uint64, p property, paused *bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id == 3 && p.Format == 3 {
		*paused = *(*int32)(p.Data) != 0
	} else if id == 6 && p.Format == 3 && *(*int32)(p.Data) != 0 {
		e.s.State = contract.Ended
	} else if p.Format == 5 {
		value := *(*float64)(p.Data)
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return
		}
		switch id {
		case 1, 2:
			if value > float64(math.MaxInt64)/float64(time.Second) {
				return
			}
			if id == 1 {
				e.s.Position = time.Duration(value * float64(time.Second))
			} else {
				e.s.Duration = time.Duration(value * float64(time.Second))
			}
		case 4:
			e.s.Volume = value / 100
		case 5:
			e.s.Rate = value
		}
	}
}
