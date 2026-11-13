package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func TestAutomationClientRejectsBadReplies(t *testing.T) {
	for _, reply := range []string{`{"id":9,"result":{}}`, `{"id":1,"error":{"code":"failed","message":"private"}}`, `broken`} {
		a, b := net.Pipe()
		go func() { defer b.Close(); _, _ = bufio.NewReader(b).ReadString('\n'); _, _ = fmt.Fprintln(b, reply) }()
		c := newClient(a)
		if c.call(context.Background(), "windows", nil, nil) == nil {
			t.Error("bad reply accepted")
		}
		_ = a.Close()
	}
}

func TestTreeDecodesNativeAutomationBounds(t *testing.T) {
	var n node
	if err := json.Unmarshal([]byte(`{"label":"warning","bounds":{"x":22,"y":85,"width":640,"height":30},"states":["disabled"]}`), &n); err != nil {
		t.Fatal(err)
	}
	if n.Bounds.X != 22 || n.Bounds.Y != 85 || n.Bounds.W != 640 || n.Bounds.H != 30 || len(n.States) != 1 {
		t.Fatalf("native JSON geometry lost: %+v", n)
	}
}

func TestSliderWaitsForEachSampleBetweenKeyInputs(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	type request struct {
		ID     int
		Method string
		Params struct {
			Key, Action string
			Events      []struct{ Type, Key string }
		}
	}
	requests := make(chan request, 5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer b.Close()
		scan := bufio.NewScanner(b)
		for i := 0; i < 5 && scan.Scan(); i++ {
			var req request
			if json.Unmarshal(scan.Bytes(), &req) != nil {
				return
			}
			requests <- req
			result := `{}`
			if req.Method == "tree" {
				position := time.Duration(0)
				if i == 4 {
					position = 250 * time.Millisecond
				}
				s, _ := json.Marshal(observed{Snapshot: contract.Snapshot{State: contract.Paused, Position: position}})
				r, _ := json.Marshal(map[string]any{"nodes": []node{{Label: "snapshot " + string(s)}}})
				result = string(r)
			}
			_, _ = fmt.Fprintf(b, "{\"id\":%d,\"result\":%s}\n", req.ID, result)
		}
	}()
	t.Cleanup(func() { _ = b.Close(); <-done })
	s, err := newClient(a).sliderKeys(context.Background(), "seek-slider", []string{"Home", "Right"}, time.Second, func(i int, s observed) bool { return s.Position == time.Duration(i)*250*time.Millisecond })
	if err != nil || s.Position != 250*time.Millisecond {
		t.Fatalf("slider input failed: %+v %v", s, err)
	}
	focus, home, sample0, right, sample1 := <-requests, <-requests, <-requests, <-requests, <-requests
	if focus.Method != "act" || focus.Params.Key != "seek-slider" || focus.Params.Action != "focus" || sample0.Method != "tree" || sample1.Method != "tree" {
		t.Fatal("missing focus/sample ordering")
	}
	for i, req := range []request{home, right} {
		if req.Method != "input" || len(req.Params.Events) != 1 {
			t.Fatalf("keys batched or wrong method: %+v", req)
		}
		if ev := req.Params.Events[0]; ev.Type != "key" || ev.Key != []string{"Home", "Right"}[i] {
			t.Fatalf("wrong slider key: %+v", ev)
		}
	}
}

func TestAutomationDeadline(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if newClient(a).call(ctx, "windows", nil, nil) == nil {
		t.Fatal("deadline ignored")
	}
}

type delayedReadConn struct {
	net.Conn
	delay time.Duration
}

func (c delayedReadConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	// Simulate resuming after the deadline with an already-read positive reply.
	time.Sleep(c.delay)
	return n, err
}

func snapshotClient(t *testing.T, responseDelay, readDelay time.Duration) *client {
	t.Helper()
	a, b := net.Pipe()
	done := make(chan struct{})
	s, err := json.Marshal(observed{Snapshot: contract.Snapshot{State: contract.Ready, Duration: 8 * time.Second}, Owned: true})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"nodes": []node{{Label: "snapshot " + string(s)}}}})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(done)
		defer b.Close()
		if _, err := bufio.NewReader(b).ReadString('\n'); err != nil {
			return
		}
		time.Sleep(responseDelay)
		_, _ = fmt.Fprintln(b, string(reply))
	}()
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
		<-done
	})
	return newClient(delayedReadConn{Conn: a, delay: readDelay})
}

func TestUntilRejectsLatePositiveSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name          string
		responseDelay time.Duration
		readDelay     time.Duration
	}{
		{name: "slow RPC", responseDelay: 80 * time.Millisecond},
		{name: "resumed after RPC", readDelay: 80 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := snapshotClient(t, tc.responseDelay, tc.readDelay)
			_, err := c.until(context.Background(), 20*time.Millisecond, func(s observed) bool { return s.State == contract.Ready })
			if err == nil {
				t.Fatal("positive snapshot arriving after assertion deadline accepted")
			}
		})
	}
}

func TestUntilAcceptsOnTimePositiveSnapshot(t *testing.T) {
	c := snapshotClient(t, 0, 0)
	s, err := c.until(context.Background(), time.Second, func(s observed) bool { return s.State == contract.Ready })
	if err != nil || s.State != contract.Ready || !s.Owned {
		t.Fatalf("on-time snapshot rejected: state=%s owned=%t err=%v", s.State, s.Owned, err)
	}
}

func TestUntilHonorsShorterParentDeadline(t *testing.T) {
	c := snapshotClient(t, 80*time.Millisecond, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.until(ctx, time.Second, func(s observed) bool { return s.State == contract.Ready }); err == nil {
		t.Fatal("positive snapshot after shorter parent deadline accepted")
	}
}

func TestUntilRejectsCanceledPositiveObservation(t *testing.T) {
	c := snapshotClient(t, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := c.until(ctx, time.Second, func(s observed) bool {
		cancel()
		return s.State == contract.Ready
	}); err == nil {
		t.Fatal("positive observation accepted after context cancellation")
	}
}

func replacementClient(t *testing.T, states []observed) *client {
	t.Helper()
	a, b := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer b.Close()
		scan := bufio.NewScanner(b)
		i := 0
		for scan.Scan() {
			var request struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			if json.Unmarshal(scan.Bytes(), &request) != nil {
				return
			}
			var result any = map[string]any{}
			if request.Method == "tree" {
				s, _ := json.Marshal(states[min(i, len(states)-1)])
				result = map[string]any{"nodes": []node{{Label: "snapshot " + string(s)}}}
				i++
			}
			reply, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if _, err := fmt.Fprintln(b, string(reply)); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = a.Close()
		_ = b.Close()
		<-done
	})
	return newClient(a)
}

func TestReadyWaitsForReplacementGeneration(t *testing.T) {
	old := observed{Snapshot: contract.Snapshot{Generation: 7, State: contract.Ready, Duration: 8 * time.Second}, Owned: true}
	next := old
	next.Generation++
	c := replacementClient(t, []observed{old, old, next})
	s, err := c.ready(context.Background(), "local")
	if err != nil || s.Generation != next.Generation {
		t.Fatalf("replacement accepted stale generation: got=%d want=%d err=%v", s.Generation, next.Generation, err)
	}
}

func TestOpenUntilBindsLoadingAndFailuresToNewGeneration(t *testing.T) {
	for _, state := range []observed{
		{Snapshot: contract.Snapshot{Generation: 7, State: contract.Loading}, Owned: true},
		{Snapshot: contract.Snapshot{Generation: 7, State: contract.Failed, Error: contract.Unsupported}},
		{Snapshot: contract.Snapshot{Generation: 7, State: contract.Failed, Error: contract.Media}},
	} {
		t.Run(string(state.State)+"/"+string(state.Error), func(t *testing.T) {
			predicate := func(s observed) bool {
				return s.State == state.State && s.Error == state.Error && s.Owned == state.Owned
			}
			next := state
			next.Generation++
			c := replacementClient(t, []observed{state, state, next})
			s, err := c.openUntil(context.Background(), "fixture", time.Second, predicate)
			if err != nil || s.Generation != next.Generation {
				t.Fatalf("stale replacement state accepted: generation=%d err=%v", s.Generation, err)
			}
			c = replacementClient(t, []observed{state})
			if _, err = c.openUntil(context.Background(), "fixture", 20*time.Millisecond, predicate); err == nil {
				t.Fatal("unchanged session passed replacement assertion")
			}
		})
	}
}
