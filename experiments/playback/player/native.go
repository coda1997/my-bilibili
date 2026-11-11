// Package player adapts the pinned mygo system-media player. All methods,
// including Open, Draw and Close, must be called on the UI/main thread.
package player

import (
	"context"
	"time"

	gui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"my-bilibili/experiments/playback/contract"
)

type backend interface {
	Ready() bool
	Playing() bool
	Ended() bool
	Err() error
	Position() time.Duration
	Duration() time.Duration
	Rate() float64
	Volume() float64
	Play()
	Pause()
	Seek(time.Duration)
	SetRate(float64) *gui.VideoPlayer
	SetVolume(float64) *gui.VideoPlayer
	Close()
}

type Opener struct {
	file func(string) (backend, error)
	url  func(string) (backend, error)
}

func (o Opener) Open(ctx context.Context, in contract.Input) (contract.Engine, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, contract.Invalid
	}
	if ctx.Err() != nil {
		return nil, contract.Canceled
	}
	if len(in.Headers) > 0 || in.AudioURL != "" {
		return nil, contract.Unsupported
	}
	f, u := o.file, o.url
	if f == nil {
		f = func(s string) (backend, error) {
			p, err := video.Open(s)
			if p == nil {
				return nil, err
			}
			return p, err
		}
	}
	if u == nil {
		u = func(s string) (backend, error) {
			p, err := video.OpenURL(s)
			if p == nil {
				return nil, err
			}
			return p, err
		}
	}
	var p backend
	var err error
	if in.FilePath != "" {
		p, err = f(in.FilePath)
	} else {
		p, err = u(in.URL)
	}
	if err != nil || p == nil {
		if p != nil {
			p.Close()
		}
		return nil, contract.Media
	}
	// AVFoundation needs mygo's initial play to produce its first picture.
	// Snapshot pauses/resets that short priming run before publishing Ready.
	return &engine{p: p}, nil
}

type engine struct {
	p           backend
	closed      bool
	commanded   bool
	primed      bool
	wantPlay    bool
	initialSeek time.Duration
}

func (e *engine) Snapshot() contract.Snapshot {
	s := contract.Snapshot{State: contract.Loading, Rate: 1, Volume: 1}
	if e.closed {
		s.State = contract.Closed
		return s
	}
	if e.p.Err() != nil {
		s.State, s.Error = contract.Failed, contract.Media
		return s
	}
	if !e.primed && e.p.Ready() {
		e.primed = true
		if !e.wantPlay {
			e.p.Pause()
		}
		e.p.Seek(e.initialSeek)
	}
	s.Position, s.Duration = e.p.Position(), e.p.Duration()
	s.Rate, s.Volume = e.p.Rate(), e.p.Volume()
	switch {
	case e.p.Ended():
		s.State = contract.Ended
	case !e.p.Ready():
		s.State = contract.Loading
	case e.p.Playing():
		s.State = contract.Playing
	case e.commanded:
		s.State = contract.Paused
	default:
		s.State = contract.Ready
	}
	return s
}

func (e *engine) Command(c contract.Command) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if e.closed {
		return contract.IsClosed
	}
	switch c.Kind {
	case contract.Play:
		e.wantPlay = true
		if e.primed {
			e.p.Play()
		}
		e.commanded = true
	case contract.Pause:
		e.wantPlay = false
		if e.primed {
			e.p.Pause()
		}
		e.commanded = true
	case contract.Seek:
		if e.primed {
			e.p.Seek(c.Position)
		} else {
			e.initialSeek = c.Position
		}
	case contract.Volume:
		e.p.SetVolume(c.Value)
	case contract.Rate:
		e.p.SetRate(c.Value)
	}
	return nil
}

func (e *engine) Close() error {
	if !e.closed {
		e.closed = true
		e.p.Close()
		e.p = nil
	}
	return nil
}

func Draw(c *gui.Context, plain contract.Engine) {
	e, ok := plain.(*engine)
	if !ok || e.closed {
		return
	}
	p, ok := e.p.(*gui.VideoPlayer)
	if ok {
		gui.Video(c.Key("video"), p).Width(640).Height(360).Fit(gui.Contain).Label("Synthetic fixture video")
	}
}
