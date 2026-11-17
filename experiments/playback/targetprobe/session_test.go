package targetprobe

import (
	"context"
	"errors"
	"testing"
	"time"

	"my-bilibili/internal/playback"
)

type fakeEngine struct {
	s        playback.Snapshot
	closed   int
	commands []playback.Command
	err      error
}

func (e *fakeEngine) Snapshot() playback.Snapshot { return e.s }
func (e *fakeEngine) Command(c playback.Command) error {
	e.commands = append(e.commands, c)
	return e.err
}
func (e *fakeEngine) Close() error { e.closed++; return nil }

type openFunc func(context.Context, playback.Input) (playback.Engine, error)

func (f openFunc) Open(c context.Context, i playback.Input) (playback.Engine, error) { return f(c, i) }
func TestLoadingAndPlayingExpiryDisposes(t *testing.T) {
	for _, state := range []playback.State{playback.Loading, playback.Playing} {
		now := time.Now()
		p := testPolicy(t, &now)
		e := &fakeEngine{s: playback.Snapshot{State: state, Duration: 596624 * time.Millisecond}}
		s := NewSession(openFunc(func(context.Context, playback.Input) (playback.Engine, error) { return e, nil }), p, func() time.Time { return now })
		if s.Open(context.Background()) != nil {
			t.Fatal("open")
		}
		s.Tick()
		now = now.Add(SessionLimit)
		s.Tick()
		if got := s.Snapshot(); got.State != playback.Failed || got.Error != playback.Timeout || got.Owned || e.closed != 1 {
			t.Fatal(got, e.closed)
		}
		if s.Retry(context.Background()) == nil {
			t.Fatal("expired retry")
		}
		s.Close()
		if e.closed != 1 {
			t.Fatal("double disposal")
		}
	}
}
func TestSessionControlsCancellationAndGeneration(t *testing.T) {
	now := time.Now()
	p := testPolicy(t, &now)
	engines := []*fakeEngine{}
	contexts := []context.Context{}
	s := NewSession(openFunc(func(ctx context.Context, _ playback.Input) (playback.Engine, error) {
		e := &fakeEngine{s: playback.Snapshot{State: playback.Ready, Duration: 596624 * time.Millisecond, Rate: 1, Volume: 1}}
		engines = append(engines, e)
		contexts = append(contexts, ctx)
		return e, nil
	}), p, func() time.Time { return now })
	if s.Open(context.Background()) != nil {
		t.Fatal("open")
	}
	s.Tick()
	if s.Command(playback.Command{Kind: playback.Seek, Position: 597 * time.Second}) != playback.Invalid {
		t.Fatal("out of grant seek")
	}
	if s.Command(playback.Command{Kind: playback.Seek, Position: 3 * time.Second}) != nil {
		t.Fatal("allowed seek")
	}
	s.Cancel()
	g := s.Snapshot().Generation
	if s.Snapshot().Error != playback.Canceled || s.Snapshot().Owned || engines[0].closed != 1 {
		t.Fatal(s.Snapshot())
	}
	if s.Open(context.Background()) != playback.IsClosed {
		t.Fatal("open acted as implicit retry")
	}
	if s.Retry(context.Background()) != nil {
		t.Fatal("explicit retry")
	}
	engines[0].s.State = playback.Playing
	s.Tick()
	if s.Snapshot().Generation <= g || contexts[0].Err() != context.Canceled || engines[0].closed != 1 {
		t.Fatal("stale completion/ownership")
	}
	s.Close()
	s.Close()
	if engines[1].closed != 1 {
		t.Fatal("double close")
	}
}
func TestFailureIsSanitizedAndNotRetried(t *testing.T) {
	for _, code := range []error{playback.Media, playback.Timeout, playback.Unsupported, errors.New("PRIVATE_SYNTHETIC")} {
		now := time.Now()
		p := testPolicy(t, &now)
		s := NewSession(openFunc(func(context.Context, playback.Input) (playback.Engine, error) { return nil, code }), p, func() time.Time { return now })
		e := s.Open(context.Background())
		if e == nil || e.Error() == "PRIVATE_SYNTHETIC" {
			t.Fatal(e)
		}
		if s.Retry(context.Background()) == nil || s.Open(context.Background()) == nil {
			t.Fatal("failure retried")
		}
	}
}
func TestParentDeadlineAndGrantEndpoint(t *testing.T) {
	now := time.Now()
	p := testPolicy(t, &now)
	e := &fakeEngine{s: playback.Snapshot{State: playback.Playing, Position: 596624 * time.Millisecond}}
	s := NewSession(openFunc(func(context.Context, playback.Input) (playback.Engine, error) { return e, nil }), p, func() time.Time { return now })
	_ = s.Open(context.Background())
	s.Tick()
	if s.Snapshot().State != playback.Ended || s.Snapshot().Owned {
		t.Fatal(s.Snapshot())
	}
	s2 := NewSession(openFunc(func(context.Context, playback.Input) (playback.Engine, error) {
		return &fakeEngine{s: playback.Snapshot{State: playback.Loading}}, nil
	}), p, func() time.Time { return now })
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_ = s2.Open(ctx)
	s2.Tick()
	if s2.Snapshot().Error != playback.Timeout {
		t.Fatal("timeout lost", s2.Snapshot())
	}
}
