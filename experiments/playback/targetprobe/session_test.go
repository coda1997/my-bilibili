package targetprobe

import (
	"context"
	"errors"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

type fakeEngine struct {
	s        contract.Snapshot
	closed   int
	commands []contract.Command
	err      error
}

func (e *fakeEngine) Snapshot() contract.Snapshot { return e.s }
func (e *fakeEngine) Command(c contract.Command) error {
	e.commands = append(e.commands, c)
	return e.err
}
func (e *fakeEngine) Close() error { e.closed++; return nil }

type openFunc func(context.Context, contract.Input) (contract.Engine, error)

func (f openFunc) Open(c context.Context, i contract.Input) (contract.Engine, error) { return f(c, i) }
func TestLoadingAndPlayingExpiryDisposes(t *testing.T) {
	for _, state := range []contract.State{contract.Loading, contract.Playing} {
		now := time.Now()
		p := testPolicy(t, &now)
		e := &fakeEngine{s: contract.Snapshot{State: state, Duration: 596624 * time.Millisecond}}
		s := NewSession(openFunc(func(context.Context, contract.Input) (contract.Engine, error) { return e, nil }), p, func() time.Time { return now })
		if s.Open(context.Background()) != nil {
			t.Fatal("open")
		}
		s.Tick()
		now = now.Add(SessionLimit)
		s.Tick()
		if got := s.Snapshot(); got.State != contract.Failed || got.Error != contract.Timeout || got.Owned || e.closed != 1 {
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
	s := NewSession(openFunc(func(ctx context.Context, _ contract.Input) (contract.Engine, error) {
		e := &fakeEngine{s: contract.Snapshot{State: contract.Ready, Duration: 596624 * time.Millisecond, Rate: 1, Volume: 1}}
		engines = append(engines, e)
		contexts = append(contexts, ctx)
		return e, nil
	}), p, func() time.Time { return now })
	if s.Open(context.Background()) != nil {
		t.Fatal("open")
	}
	s.Tick()
	if s.Command(contract.Command{Kind: contract.Seek, Position: 597 * time.Second}) != contract.Invalid {
		t.Fatal("out of grant seek")
	}
	if s.Command(contract.Command{Kind: contract.Seek, Position: 3 * time.Second}) != nil {
		t.Fatal("allowed seek")
	}
	s.Cancel()
	g := s.Snapshot().Generation
	if s.Snapshot().Error != contract.Canceled || s.Snapshot().Owned || engines[0].closed != 1 {
		t.Fatal(s.Snapshot())
	}
	if s.Open(context.Background()) != contract.IsClosed {
		t.Fatal("open acted as implicit retry")
	}
	if s.Retry(context.Background()) != nil {
		t.Fatal("explicit retry")
	}
	engines[0].s.State = contract.Playing
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
	for _, code := range []error{contract.Media, contract.Timeout, contract.Unsupported, errors.New("PRIVATE_SYNTHETIC")} {
		now := time.Now()
		p := testPolicy(t, &now)
		s := NewSession(openFunc(func(context.Context, contract.Input) (contract.Engine, error) { return nil, code }), p, func() time.Time { return now })
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
	e := &fakeEngine{s: contract.Snapshot{State: contract.Playing, Position: 596624 * time.Millisecond}}
	s := NewSession(openFunc(func(context.Context, contract.Input) (contract.Engine, error) { return e, nil }), p, func() time.Time { return now })
	_ = s.Open(context.Background())
	s.Tick()
	if s.Snapshot().State != contract.Ended || s.Snapshot().Owned {
		t.Fatal(s.Snapshot())
	}
	s2 := NewSession(openFunc(func(context.Context, contract.Input) (contract.Engine, error) {
		return &fakeEngine{s: contract.Snapshot{State: contract.Loading}}, nil
	}), p, func() time.Time { return now })
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_ = s2.Open(ctx)
	s2.Tick()
	if s2.Snapshot().Error != contract.Timeout {
		t.Fatal("timeout lost", s2.Snapshot())
	}
}
