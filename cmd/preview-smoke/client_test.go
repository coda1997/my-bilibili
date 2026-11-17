package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
)

func completeObservation(t *testing.T) observed {
	t.Helper()
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	video, err := source.Detail(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return observed{State: app.State{Video: video, SelectedPart: "part-1", PlaybackPart: "part-1", Volume: 1, Rate: 1, Playback: playback.Snapshot{Generation: 1, State: playback.Closed, Error: playback.Canceled, Volume: 1, Rate: 1}}}
}
func TestObservationNodesAreUniqueAndStrict(t *testing.T) {
	o := completeObservation(t)
	b, _ := json.Marshal(o)
	good := node{Key: "e2e-observation", Label: string(b)}
	got, err := observation([]node{good})
	if err != nil || !disposed(got, playback.Canceled) {
		t.Fatal(got, err)
	}
	for _, nodes := range [][]node{nil, {good, good}, {{Key: "other", Label: string(b)}}, {{Key: "e2e-observation", Label: `{}`}}, {{Key: "e2e-observation", Label: `null`}}} {
		if _, err := observation(nodes); err == nil {
			t.Fatal("accepted absent or ambiguous observation")
		}
	}
	o.Resources.Cores = 1
	if disposed(o, playback.Canceled) {
		t.Fatal("positive ownership passed disposal")
	}
	o.Resources.Cores = 0
	o.State.Playback.Error = playback.NoError
	if disposed(o, playback.Canceled) {
		t.Fatal("ordinary close passed parent cancellation")
	}
}

func pipeClient(t *testing.T, reply func(int, string) any) *client {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	go func() {
		defer b.Close()
		s := bufio.NewScanner(b)
		for s.Scan() {
			var request struct {
				ID     int
				Method string
			}
			if json.Unmarshal(s.Bytes(), &request) != nil {
				return
			}
			result := reply(request.ID, request.Method)
			data, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if _, err := b.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}()
	return newClient(a)
}
func TestCancellationDeadlineIncludesActionAndObservation(t *testing.T) {
	o := completeObservation(t)
	data, _ := json.Marshal(o)
	c := pipeClient(t, func(id int, method string) any {
		if method == "act" {
			time.Sleep(120 * time.Millisecond)
			return map[string]bool{"ok": true}
		}
		if id > 1 {
			time.Sleep(120 * time.Millisecond)
		}
		return map[string]any{"nodes": []node{{Key: "e2e-observation", Label: string(data)}}}
	})
	start := time.Now()
	r := &report{}
	err := c.cancelParent(context.Background(), nil, r)
	if err == nil {
		t.Fatal("240ms cancellation passed 200ms bound")
	}
	if time.Since(start) > 350*time.Millisecond {
		t.Fatal("cancel observation ignored shared deadline")
	}
	if len(r.Checks) != 0 {
		t.Fatal("late cancellation recorded a pass", r.Checks)
	}
}
func TestUntilRejectsMalformedInsteadOfAssumingClosed(t *testing.T) {
	c := pipeClient(t, func(int, string) any {
		return map[string]any{"nodes": []node{{Key: "e2e-observation", Label: `{"Resources":{}}`}}}
	})
	called := false
	if _, err := c.until(context.Background(), time.Second, func(observed) bool { called = true; return true }); err == nil || called {
		t.Fatal("invalid snapshot reached predicate")
	}
}
func TestAutomationRejectsNullResult(t *testing.T) {
	c := pipeClient(t, func(int, string) any { return nil })
	if _, err := c.tree(context.Background()); err == nil {
		t.Fatal("null tree accepted")
	}
}
func TestSmokeRequiresExactIdentityArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"-app", "relative"}, {"-app", "/tmp/app", "-output", "/tmp/evidence", "-app-sha256", "bad", "-driver-sha256", "bad"}} {
		if err := command(args, io.Discard, io.Discard); err == nil {
			t.Fatal("accepted unpinned invocation")
		}
	}
}

func TestActionErrorsKeepSafeKeyAndProtocolCategory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response any
		category string
	}{
		{"known code", map[string]any{"code": "no_element", "message": "private native payload"}, "no_element"},
		{"unknown code", map[string]any{"code": "private remote code", "message": "private native payload"}, "remote_error"},
		{"malformed error", "private native payload", "response invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, trace := popupClient(t, func(popupRequest) map[string]any {
				return map[string]any{"error": tc.response}
			})
			err := c.act(context.Background(), "speed-2")
			if err == nil || !strings.Contains(err.Error(), "speed-2") || !strings.Contains(err.Error(), tc.category) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unsafe or missing action error context: %v", err)
			}
			if trace.count("act", "speed-2") != 1 {
				t.Fatal("failed action was retried")
			}
		})
	}
}

func TestActionErrorPreservesContextIdentity(t *testing.T) {
	c := pipeClient(t, func(int, string) any { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.act(ctx, "speed-menu")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "speed-menu") {
		t.Fatalf("lost canceled action identity/context: %v", err)
	}
}

func TestActionReadTimeoutPreservesDeadlineIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c, trace := popupClient(t, func(popupRequest) map[string]any {
		<-ctx.Done()
		return popupOK()
	})
	err := c.act(ctx, "speed-2")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "speed-2") {
		t.Fatalf("lost action read deadline identity/context: %v", err)
	}
	if trace.count("act", "speed-2") != 1 {
		t.Fatal("timed out action was retried")
	}
}
