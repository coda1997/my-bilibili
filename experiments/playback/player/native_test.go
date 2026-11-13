package player

import (
	"context"
	"errors"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

type fake struct {
	ready, playing, ended bool
	err                   error
	position              time.Duration
	rate, volume          float64
	closed                int
}

func (p *fake) Ready() bool                          { return p.ready }
func (p *fake) Playing() bool                        { return p.playing }
func (p *fake) Ended() bool                          { return p.ended }
func (p *fake) Err() error                           { return p.err }
func (p *fake) Position() time.Duration              { return p.position }
func (p *fake) Duration() time.Duration              { return 8 * time.Second }
func (p *fake) Rate() float64                        { return p.rate }
func (p *fake) Volume() float64                      { return p.volume }
func (p *fake) Play()                                { p.playing = true }
func (p *fake) Pause()                               { p.playing = false }
func (p *fake) Seek(d time.Duration)                 { p.position = d }
func (p *fake) SetRate(v float64) *gui.VideoPlayer   { p.rate = v; return nil }
func (p *fake) SetVolume(v float64) *gui.VideoPlayer { p.volume = v; return nil }
func (p *fake) Close()                               { p.closed++ }

func TestAdapterRejectsWithoutLosingMetadata(t *testing.T) {
	calls := 0
	o := Opener{url: func(string) (backend, error) { calls++; return &fake{}, nil }}
	for _, in := range []contract.Input{{URL: "http://127.0.0.1/plain", Headers: map[string]string{"X-Test": "sentinel"}}, {URL: "http://127.0.0.1/video", AudioURL: "http://127.0.0.1/audio"}} {
		if e, err := o.Open(context.Background(), in); e != nil || err != contract.Unsupported {
			t.Fatal("unsupported capability silently accepted")
		}
	}
	if calls != 0 {
		t.Fatal("native access despite unsupported metadata")
	}
}

func TestAdapterStateControlsAndClose(t *testing.T) {
	p := &fake{playing: true, rate: 1, volume: 1}
	o := Opener{file: func(string) (backend, error) { return p, nil }}
	e, err := o.Open(context.Background(), contract.Input{FilePath: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Snapshot().State != contract.Loading {
		t.Fatal("open did not expose loading")
	}
	p.ready = true
	if e.Snapshot().State != contract.Ready {
		t.Fatal("not ready")
	}
	if p.playing || p.position != 0 {
		t.Fatal("first-picture priming not paused/reset before Ready")
	}
	for _, c := range []contract.Command{{Kind: contract.Play}, {Kind: contract.Pause}, {Kind: contract.Seek, Position: 3 * time.Second}, {Kind: contract.Rate, Value: 1.5}, {Kind: contract.Volume, Value: .25}} {
		if err = e.Command(c); err != nil {
			t.Fatal(err)
		}
	}
	s := e.Snapshot()
	if s.State != contract.Paused || s.Position != 3*time.Second || s.Rate != 1.5 || s.Volume != .25 {
		t.Fatalf("bad readback: %+v", s)
	}
	p.ended = true
	if e.Snapshot().State != contract.Ended {
		t.Fatal("ended state")
	}
	p.err = errors.New("private locator")
	if e.Snapshot().Error != contract.Media {
		t.Fatal("unsafe native error")
	}
	_ = e.Close()
	_ = e.Close()
	if p.closed != 1 || e.Command(contract.Command{Kind: contract.Play}) != contract.IsClosed || e.Snapshot().State != contract.Closed {
		t.Fatal("close lifetime")
	}
}

func TestAdapterFailureAndCanceledOpen(t *testing.T) {
	p := &fake{}
	o := Opener{file: func(string) (backend, error) { return p, errors.New("private locator") }}
	if _, err := o.Open(context.Background(), contract.Input{FilePath: "a"}); err != contract.Media || p.closed != 1 {
		t.Fatal("open failure not sanitized/disposed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := o.Open(ctx, contract.Input{FilePath: "a"}); err != contract.Canceled || p.closed != 1 {
		t.Fatal("canceled open accessed native backend")
	}
}

func TestLoadingPauseAndSeekSurviveFirstPicturePriming(t *testing.T) {
	p := &fake{playing: true}
	o := Opener{file: func(string) (backend, error) { return p, nil }}
	e, err := o.Open(context.Background(), contract.Input{FilePath: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Command(contract.Command{Kind: contract.Pause}) != nil || e.Command(contract.Command{Kind: contract.Seek, Position: 3 * time.Second}) != nil {
		t.Fatal("loading command rejected")
	}
	if !p.playing || p.position != 0 {
		t.Fatal("loading controls stalled first-picture priming")
	}
	p.ready = true
	if s := e.Snapshot(); s.State != contract.Paused || s.Position != 3*time.Second || p.playing {
		t.Fatalf("loading intent lost: %+v", s)
	}
	_ = e.Close()
}
