package contract

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestInputLimits(t *testing.T) {
	for _, in := range []Input{{FilePath: "fixture.mp4"}, {URL: "http://127.0.0.1:1234/plain"}, {URL: "http://[::1]:1234/plain", AudioURL: "http://[::1]:1234/audio", Headers: map[string]string{"X-Test": "synthetic"}}} {
		if err := in.Validate(); err != nil {
			t.Fatalf("valid input rejected: %v", err)
		}
	}
	for name, in := range map[string]Input{
		"empty": {}, "ambiguous": {FilePath: "a", URL: "http://127.0.0.1"},
		"external": {URL: "http://example.com"}, "hostname": {URL: "http://localhost"},
		"scheme": {URL: "https://127.0.0.1"}, "credentials": {URL: "http://user:pass@127.0.0.1"},
		"port": {URL: "http://127.0.0.1:99999"}, "empty-port": {URL: "http://127.0.0.1:"},
		"fragment": {URL: "http://127.0.0.1/#"}, "controls": {URL: "http://127.0.0.1/%0a"},
		"path-size": {FilePath: strings.Repeat("a", 4097)}, "path-control": {FilePath: "a\x00b"},
		"orphan-audio":     {FilePath: "a", AudioURL: "http://127.0.0.1"},
		"header-injection": {URL: "http://127.0.0.1", Headers: map[string]string{"X-Test": "a\r\nb"}},
		"header-name":      {URL: "http://127.0.0.1", Headers: map[string]string{"Bad Name": "a"}},
		"header-size":      {URL: "http://127.0.0.1", Headers: map[string]string{"X": strings.Repeat("a", 4096)}},
		"header-count":     {URL: "http://127.0.0.1", Headers: map[string]string{"A": "", "B": "", "C": "", "D": "", "E": "", "F": "", "G": "", "H": "", "I": ""}},
		"header-duplicate": {URL: "http://127.0.0.1", Headers: map[string]string{"X": "a", "x": "b"}},
		"local-metadata":   {FilePath: "a", Headers: map[string]string{"X": "a"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := in.Validate(); err != Invalid {
				t.Fatalf("got %v, want safe invalid category", err)
			}
		})
	}
}

type fakeEngine struct {
	s             Snapshot
	closed, reads int
	commands      []Command
	err           error
}

func (e *fakeEngine) Snapshot() Snapshot      { e.reads++; return e.s }
func (e *fakeEngine) Command(c Command) error { e.commands = append(e.commands, c); return e.err }
func (e *fakeEngine) Close() error            { e.closed++; return nil }

type openFunc func(context.Context, Input) (Engine, error)

func (f openFunc) Open(c context.Context, i Input) (Engine, error) { return f(c, i) }

func TestControllerLifetime(t *testing.T) {
	now := time.Unix(1, 0)
	a := &fakeEngine{s: Snapshot{State: Loading}}
	b := &fakeEngine{s: Snapshot{State: Ready, Duration: 8 * time.Second}}
	var old context.Context
	count := 0
	c := New(openFunc(func(ctx context.Context, _ Input) (Engine, error) {
		count++
		if count == 1 {
			old = ctx
			return a, nil
		}
		if a.closed != 1 || old.Err() == nil {
			t.Fatal("old session not disposed before open")
		}
		return b, nil
	}), func() time.Time { return now })
	if err := c.Open(context.Background(), Input{FilePath: "a"}); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	c.Tick()
	if a.reads != 1 {
		t.Fatal("polling exceeds 20Hz")
	}
	if err := c.Open(context.Background(), Input{}); err != Invalid || a.closed != 0 {
		t.Fatal("invalid replacement changed current session")
	}
	if err := c.Open(context.Background(), Input{FilePath: "b"}); err != nil {
		t.Fatal(err)
	}
	a.s = Snapshot{State: Playing, Generation: 99}
	c.Tick()
	if c.Snapshot().Generation != 2 || c.Snapshot().State != Ready {
		t.Fatal("stale session revived")
	}
	if err := c.Command(Command{Kind: Seek, Position: 20 * time.Second}); err != nil {
		t.Fatal(err)
	}
	if b.commands[0].Position != 8*time.Second {
		t.Fatal("seek not clamped")
	}
	_ = c.Close()
	_ = c.Close()
	if b.closed != 1 || c.Active() || c.Snapshot().State != Closed {
		t.Fatal("close leaked engine")
	}
	if c.Command(Command{Kind: Play}) != IsClosed {
		t.Fatal("closed command accepted")
	}
	c.Tick()
	if c.Snapshot().Generation != 2 {
		t.Fatal("close changed generation")
	}
}

func TestControllerErrorsCancellationAndTimeout(t *testing.T) {
	for _, which := range []string{"cancel", "pre-cancel", "timeout", "native", "command", "unsupported", "nil-engine"} {
		t.Run(which, func(t *testing.T) {
			now := time.Unix(1, 0)
			e := &fakeEngine{s: Snapshot{State: Loading}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := New(openFunc(func(context.Context, Input) (Engine, error) {
				if which == "unsupported" {
					return e, Unsupported
				}
				if which == "nil-engine" {
					return nil, nil
				}
				return e, nil
			}), func() time.Time { return now })
			if which == "pre-cancel" {
				cancel()
			}
			_ = c.Open(ctx, Input{FilePath: "a"})
			want, state := Media, Failed
			switch which {
			case "cancel", "pre-cancel":
				cancel()
				want, state = Canceled, Closed
			case "timeout":
				now = now.Add(10 * time.Second)
				want = Timeout
			case "native":
				e.s = Snapshot{State: Failed, Error: ErrorCode("private-url")}
			case "command":
				e.err = errors.New("private-url")
				_ = c.Command(Command{Kind: Play})
			case "unsupported":
				want = Unsupported
			}
			c.Tick()
			if c.Active() || c.Snapshot().Error != want || c.Snapshot().State != state {
				t.Fatalf("got %+v", c.Snapshot())
			}
			if which != "pre-cancel" && which != "nil-engine" && e.closed != 1 {
				t.Fatal("failure did not dispose exactly once")
			}
		})
	}
}

func TestCommandValidation(t *testing.T) {
	for _, cmd := range []Command{{Kind: "?"}, {Kind: Seek, Position: -1}, {Kind: Volume, Value: 1.01}, {Kind: Volume, Value: -.01}, {Kind: Rate, Value: .24}, {Kind: Rate, Value: 3.1}, {Kind: Play, Value: math.NaN()}, {Kind: Rate, Value: math.Inf(1)}} {
		if cmd.Validate() != Invalid {
			t.Fatalf("accepted %+v", cmd)
		}
	}
	for _, cmd := range []Command{{Kind: Play}, {Kind: Pause}, {Kind: Seek}, {Kind: Volume}, {Kind: Volume, Value: 1}, {Kind: Rate, Value: .25}, {Kind: Rate, Value: 3}} {
		if cmd.Validate() != nil {
			t.Fatalf("rejected %+v", cmd)
		}
	}
}

func TestLateReadyCannotWinExpiredDeadline(t *testing.T) {
	now := time.Unix(1, 0)
	e := &fakeEngine{s: Snapshot{State: Loading}}
	c := New(openFunc(func(context.Context, Input) (Engine, error) { return e, nil }), func() time.Time { return now })
	if err := c.Open(context.Background(), Input{FilePath: "fixture"}); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	now = now.Add(11 * time.Second)
	e.s.State = Ready
	c.Tick()
	if c.Snapshot().Error != Timeout || c.Active() || e.closed != 1 {
		t.Fatalf("late ready bypassed deadline: %+v", c.Snapshot())
	}
}

func TestPollingBoundAndEstablishedReady(t *testing.T) {
	now := time.Unix(1, 0)
	e := &fakeEngine{s: Snapshot{State: Ready}}
	c := New(openFunc(func(context.Context, Input) (Engine, error) { return e, nil }), func() time.Time { return now })
	defer c.Close()
	if err := c.Open(context.Background(), Input{FilePath: "fixture"}); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	for range 49 {
		now = now.Add(time.Millisecond)
		c.Tick()
	}
	if e.reads != 1 {
		t.Fatal("polls exceed 20Hz")
	}
	now = now.Add(time.Millisecond)
	c.Tick()
	if e.reads != 2 {
		t.Fatal("50ms poll not delivered")
	}
	now = now.Add(20 * time.Second)
	c.Tick()
	if !c.Active() || c.Snapshot().State != Ready {
		t.Fatal("ready session incorrectly hit initial readiness deadline")
	}
}
