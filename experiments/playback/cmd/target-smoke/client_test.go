package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

type fakeRequest struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Key string `json:"key"`
	} `json:"params"`
}

func pipeClient(t *testing.T, handler func(fakeRequest) any) (*client, *report) {
	t.Helper()
	a, b := net.Pipe()
	r := new(report)
	c := newClient(a, r)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer b.Close()
		scan := bufio.NewScanner(b)
		for scan.Scan() {
			var req fakeRequest
			if json.Unmarshal(scan.Bytes(), &req) != nil {
				return
			}
			result := handler(req)
			if _, err := fmt.Fprintf(b, "{\"id\":%d,\"result\":%s}\n", req.ID, mustJSON(result)); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		a.Close()
		b.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("fake automation did not stop")
		}
	})
	return c, r
}
func mustJSON(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func fakeNodes(s observed, image, external bool, resources mpvprobe.Resources) any {
	ns := []map[string]any{
		{"label": "snapshot " + mustJSON(s)},
		{"label": "resources " + mustJSON(resources)},
		{"label": "tracks " + mustJSON(mpvprobe.Tracks{Video: true, Audio: true, ExternalAudio: external})},
	}
	if image {
		ns = append(ns, map[string]any{"key": "target-video", "role": "image", "bounds": map[string]any{"width": 640, "height": 360}})
	}
	return map[string]any{"nodes": ns}
}
func TestControlSequenceNoCaptureOrRetry(t *testing.T) {
	s := observed{Snapshot: contract.Snapshot{State: contract.Idle, Rate: 1, Volume: 1}}
	var keys []string
	c, r := pipeClient(t, func(q fakeRequest) any {
		switch q.Method {
		case "windows":
			return map[string]any{"windows": []map[string]any{{"id": 1, "title": "Authorized Target Playback Probe"}}}
		case "act":
			keys = append(keys, q.Params.Key)
			switch q.Params.Key {
			case "open":
				s.Generation++
				s.Owned = true
				s.State = contract.Ready
				s.Duration = 596624 * time.Millisecond
			case "play":
				s.State = contract.Playing
			case "pause":
				s.State = contract.Paused
			case "seek":
				s.Position = 3 * time.Second
			case "volume":
				s.Volume = .25
			case "rate":
				s.Rate = 1.5
			case "cancel":
				s.Owned = false
				s.State = contract.Closed
				s.Error = contract.Canceled
			default:
				t.Errorf("unexpected action %q", q.Params.Key)
			}
			return map[string]bool{"ok": true}
		case "tree":
			if s.State == contract.Playing {
				s.Position += 400 * time.Millisecond
			}
			resources := mpvprobe.Resources{}
			if s.Owned {
				resources = mpvprobe.Resources{Cores: 1, Renderers: 1, Workers: 1}
			}
			return fakeNodes(s, s.Owned, true, resources)
		default:
			t.Errorf("unexpected method %q", q.Method)
			return nil
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.identify(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.capabilities(ctx, r); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(keys, ","); got != "open,play,pause,seek,volume,rate,play,cancel" {
		t.Fatal(got)
	}
	if len(r.Checks) != 8 {
		t.Fatalf("checks=%d", len(r.Checks))
	}
	if r.SessionFailure != nil || r.CleanupFailure != nil {
		t.Fatal("successful session reported a failure")
	}
	for _, check := range r.Checks {
		if check.Result != "pass" && !(check.Name == "loading-cancel" && check.Result == "not-tested") {
			t.Fatal(check)
		}
	}
	if err := c.act(ctx, "retry"); !errors.Is(err, contract.Invalid) {
		t.Fatal(err)
	}
	if err := c.call(ctx, "capture", nil, nil); !errors.Is(err, contract.Invalid) {
		t.Fatal(err)
	}
}
func TestRPCCancellationWakesBlockedRead(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	consumed := make(chan struct{})
	go func() { scan := bufio.NewScanner(b); scan.Scan(); close(consumed) }()
	r := new(report)
	c := newClient(a, r)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.call(ctx, "tree", nil, nil) }()
	<-consumed
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("blocked cancellation")
	}
	if time.Since(start) > 300*time.Millisecond {
		t.Fatal("late cancellation")
	}
	if r.Automation[0].Result != contract.Canceled {
		t.Fatal(r.Automation)
	}
}
func TestActionBudgetIncludesRPC(t *testing.T) {
	c, _ := pipeClient(t, func(q fakeRequest) any {
		if q.Method == "act" {
			time.Sleep(250 * time.Millisecond)
			return map[string]bool{"ok": true}
		}
		return fakeNodes(observed{Snapshot: contract.Snapshot{Generation: 1, State: contract.Closed, Error: contract.Canceled}}, false, true, mpvprobe.Resources{})
	})
	start := time.Now()
	_, err := c.actionUntil(context.Background(), "cancel", 200*time.Millisecond, func(s observed) bool { return s.State == contract.Closed })
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, contract.Timeout) {
		t.Fatal(err)
	}
	if time.Since(start) > 350*time.Millisecond {
		t.Fatal("RPC was outside action budget")
	}
}
func TestProtocolFailuresAreSanitized(t *testing.T) {
	for _, response := range []string{
		`{"id":99,"result":{"secret":"SECRET_RAW_LOCATOR"}}`,
		`{"id":1,"error":{"code":"SECRET_RAW_LOCATOR"}}`,
		`{"id":1,"result":"SECRET_RAW_LOCATOR"}`,
		strings.Repeat("SECRET_RAW_LOCATOR", 70000),
	} {
		t.Run(fmt.Sprint(len(response)), func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				scan := bufio.NewScanner(b)
				scan.Scan()
				_, _ = io.WriteString(b, response+"\n")
			}()
			r := new(report)
			c := newClient(a, r)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var result struct {
				Nodes []node `json:"nodes"`
			}
			err := c.call(ctx, "tree", nil, &result)
			if err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatal("missing/sensitive error")
			}
			if strings.Contains(mustJSON(r), "SECRET") {
				t.Fatal("report leaked body")
			}
			a.Close()
			b.Close()
			<-done
		})
	}
}
func TestSnapshotRejectsUntrustedStringsAndValues(t *testing.T) {
	for _, s := range []observed{
		{Snapshot: contract.Snapshot{State: "SECRET_RAW_LOCATOR"}},
		{Snapshot: contract.Snapshot{State: contract.Failed, Error: "SECRET_RAW_LOCATOR"}},
		{Snapshot: contract.Snapshot{State: contract.Ready}, ControlError: "SECRET_RAW_LOCATOR"},
		{Snapshot: contract.Snapshot{State: contract.Ready, Position: -1}},
		{Snapshot: contract.Snapshot{State: contract.Ready, Volume: 2}},
	} {
		c, r := pipeClient(t, func(fakeRequest) any { return fakeNodes(s, false, true, mpvprobe.Resources{}) })
		got, err := c.snapshot(context.Background())
		if err == nil || got != (observed{}) {
			t.Fatal("unsafe snapshot accepted")
		}
		if strings.Contains(mustJSON(r), "SECRET") {
			t.Fatal("unsafe trace")
		}
	}
}
func TestPictureRequiresRenderedImageAndExternalAudio(t *testing.T) {
	for _, tc := range []struct{ image, audio bool }{{false, true}, {true, false}, {true, true}} {
		c, _ := pipeClient(t, func(fakeRequest) any {
			return fakeNodes(observed{Snapshot: contract.Snapshot{Generation: 1, State: contract.Ready}, Owned: true}, tc.image, tc.audio, mpvprobe.Resources{})
		})
		_, err := c.pictureTracks(context.Background(), 1)
		if (err == nil) != (tc.image && tc.audio) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
func TestFailureAndDeadlineNeverPassUntil(t *testing.T) {
	c, _ := pipeClient(t, func(fakeRequest) any {
		return fakeNodes(observed{Snapshot: contract.Snapshot{State: contract.Failed, Error: contract.Timeout}}, false, true, mpvprobe.Resources{})
	})
	_, err := c.until(context.Background(), func(observed) bool { return true })
	if !errors.Is(err, contract.Timeout) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.until(ctx, func(observed) bool { return true })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPictureAndVisibleGenerationComeFromSameTree(t *testing.T) {
	for _, s := range []observed{
		{Snapshot: contract.Snapshot{Generation: 2, State: contract.Ready}, Owned: true},
		{Snapshot: contract.Snapshot{Generation: 1, State: contract.Ready}, Owned: true, WindowHidden: true},
		{Snapshot: contract.Snapshot{Generation: 1, State: contract.Closed}},
	} {
		c, _ := pipeClient(t, func(fakeRequest) any { return fakeNodes(s, true, true, mpvprobe.Resources{}) })
		if _, err := c.pictureTracks(context.Background(), 1); err == nil {
			t.Fatal("wrong generation, invisible or unowned picture passed")
		}
	}
}
