package transport

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// A stopped parent registration may need to finish cleanup. This controlled
// context holds that cleanup so the scope callback is observably still running.
type stopGateContext struct {
	context.Context
	deadline time.Time
	entered  chan struct{}
	release  <-chan struct{}
	stops    atomic.Int32
}

func (c *stopGateContext) Deadline() (time.Time, bool) { return c.deadline, true }
func (*stopGateContext) Value(any) any                 { return nil }
func (c *stopGateContext) AfterFunc(func()) func() bool {
	return func() bool {
		if c.stops.Add(1) == 1 {
			close(c.entered)
			<-c.release
		}
		return true
	}
}

func TestOperationStopsOrJoinsScopeCancellationCallback(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { t.Error("context test performed HTTP"); return nil, nil })
	s := bind(t, c)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	release, open := latch()
	defer open()
	parent := &stopGateContext{Context: base, deadline: time.Now().Add(5 * time.Second), entered: make(chan struct{}), release: release}
	ctx, finish := s.operation(parent)
	s.cancel()
	await(t, parent.entered)
	started, finished := make(chan struct{}), make(chan struct{})
	go func() { close(started); finish(); close(finished) }()
	await(t, started)
	select {
	case <-finished:
		t.Fatal("operation returned while its scope callback still owned cleanup")
	case <-time.After(20 * time.Millisecond):
	}
	open()
	await(t, finished)
	if ctx.Err() != context.Canceled || parent.stops.Load() != 1 {
		t.Fatal("callback cancellation/stop ownership wrong")
	}

	// Completing an ordinary operation also removes its still-pending callback.
	other := New(roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil }))
	defer func() { other.Close(); await(t, other.Done()) }()
	plain, done := bind(t, other).operation(context.Background())
	done()
	if plain.Err() != context.Canceled {
		t.Fatal("normal completion retained operation context")
	}
}
