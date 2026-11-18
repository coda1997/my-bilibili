package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientConfirmationRequiresItsActualPopoverAncestry(t *testing.T) {
	bounds := rectangle{Width: 100, Height: 30, complete: true}
	nodes := []node{
		{ID: "1", Key: "history-delete-confirmation", Bounds: bounds},
		{ID: "2", Parent: "1", Key: "layout", Bounds: bounds},
		{ID: "3", Parent: "2", Key: "confirm", Bounds: bounds},
		{ID: "4", Key: "unrelated", Bounds: bounds},
		{ID: "5", Parent: "4", Key: "confirm", Bounds: bounds},
	}
	if id, ready, err := scopedClientAction(nodes, "history-delete-confirmation", "confirm"); err != nil || !ready || id != "3" {
		t.Fatal("confirmation not scoped to its own popover", id, ready, err)
	}
	if _, ready, err := scopedClientAction(nodes, "history-clear-confirmation", "confirm"); err != nil || ready {
		t.Fatal("other popover's confirmation became available")
	}
	for name, mutate := range map[string]func([]node) []node{
		"outside":          func(n []node) []node { n[2].Parent = "4"; return n },
		"disabled":         func(n []node) []node { n[2].States = []string{"disabled"}; return n },
		"offscreen":        func(n []node) []node { n[2].States = []string{"offscreen"}; return n },
		"unpainted":        func(n []node) []node { n[2].Bounds.Width = 0; return n },
		"missing-geometry": func(n []node) []node { n[2].Bounds.complete = false; return n },
		"missing-parent":   func(n []node) []node { n[2].Parent = "missing"; return n },
		"cycle":            func(n []node) []node { n[1].Parent = "2"; return n },
		"duplicate-id":     func(n []node) []node { n[4].ID = "3"; return n },
		"duplicate-action": func(n []node) []node { n[4].Parent = "1"; return n },
		"duplicate-scope":  func(n []node) []node { n[3].Key = "history-delete-confirmation"; return n },
	} {
		t.Run(name, func(t *testing.T) {
			copy := append([]node{}, nodes...)
			_, ready, _ := scopedClientAction(mutate(copy), "history-delete-confirmation", "confirm")
			if ready {
				t.Fatal("invalid confirmation became actionable")
			}
		})
	}
}

func TestClientTextUsesApplicationValueWithoutTracingText(t *testing.T) {
	const value = "synthetic private query sentinel"
	a, b := net.Pipe()
	done := make(chan struct{})
	t.Cleanup(func() { _ = a.Close(); _ = b.Close(); <-done })
	go func() {
		defer close(done)
		current := ""
		scan := bufio.NewScanner(b)
		for scan.Scan() {
			var request struct {
				ID     int
				Method string
				Params struct{ Key, Action, Text string }
			}
			if json.Unmarshal(scan.Bytes(), &request) != nil {
				return
			}
			if request.Method == "act" && request.Params.Key == "client-search-text" && request.Params.Action == "setValue" {
				current = request.Params.Text
			}
			var result any = map[string]bool{"ok": true}
			if request.Method == "tree" {
				result = treeResult{Nodes: []node{{Key: "client-search-text", Value: current, Bounds: rectangle{Width: 200, Height: 34}}}}
			}
			data, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if _, err := b.Write(append(data, 10)); err != nil {
				return
			}
		}
	}()
	c := newClient(a)
	c.report = &report{Scenario: "search", traceStart: time.Now()}
	if err := c.clientText(context.Background(), "client-search-text", value); err != nil {
		t.Fatal(err)
	}
	if err := c.clientText(context.Background(), "foreign-input", value); err == nil {
		t.Fatal("unassigned text node accepted")
	}
	encoded, _ := json.Marshal(c.report)
	if strings.Contains(string(encoded), value) || strings.Contains(string(encoded), "private") {
		t.Fatal("input content entered trace")
	}
}

func TestClientNavigationWaitsForItsSelectedFrame(t *testing.T) {
	for _, key := range []string{"client-catalog", "client-player", "client-history", "client-account"} {
		t.Run(key, func(t *testing.T) {
			queued, rendered := false, false
			postActionFrames := 0
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				if request.Method == "act" {
					if request.Params.Key != key {
						t.Error("unexpected navigation action")
					}
					queued = true
					return popupOK()
				}
				tab, menu := popupNode(key), popupNode("quality-menu")
				menu["id"] = "prior-mount"
				if queued {
					postActionFrames++
					if postActionFrames == 2 {
						rendered = true
						tab["states"] = []string{"selected"}
						menu["id"] = "current-mount"
					}
				}
				return popupTree(tab, popupNode("other-tab", "selected"), menu)
			})
			if err := c.clientAction(context.Background(), key); err != nil {
				t.Fatal(err)
			}
			if !rendered {
				t.Fatal("queued acknowledgment or another selected tab was accepted before the target frame")
			}
			if trace.count("act", key) != 1 || trace.count("tree", "") != 3 {
				t.Fatal("navigation was replayed or returned without its rendered frame")
			}
		})
	}
}

func TestClientNavigationLeavesOrdinaryActionsUnchanged(t *testing.T) {
	for _, key := range []string{"play-toggle", "quality-menu", "history-clear"} {
		t.Run(key, func(t *testing.T) {
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				if request.Method == "tree" {
					return popupTree(popupNode(key))
				}
				return popupOK()
			})
			if err := c.clientAction(context.Background(), key); err != nil {
				t.Fatal(err)
			}
			if trace.count("tree", "") != 1 || trace.count("act", key) != 1 {
				t.Fatal("ordinary action acquired a navigation predicate or replay")
			}
		})
	}
}

func TestClientNavigationHonorsCallerCancellationAndDeadline(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "parent-deadline"
		if canceled {
			name = "cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				if request.Method == "act" {
					if canceled {
						cancel()
					}
					return popupOK()
				}
				return popupTree(popupNode("client-player"))
			})
			want := context.DeadlineExceeded
			if canceled {
				want = context.Canceled
			}
			start := time.Now()
			if err := c.clientAction(ctx, "client-player"); !errors.Is(err, want) {
				t.Fatalf("lost caller context failure: %v", err)
			}
			if time.Since(start) > 500*time.Millisecond || trace.count("act", "client-player") != 1 {
				t.Fatal("navigation escaped its parent budget or replayed its action")
			}
		})
	}
}

func TestClientNavigationStopsOnMalformedFrameOrRejectedAction(t *testing.T) {
	for _, name := range []string{"missing-tab", "duplicate-tab", "invalid-states", "null-tree", "rejected-action"} {
		t.Run(name, func(t *testing.T) {
			queued := false
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				if request.Method == "act" {
					queued = true
					if name == "rejected-action" {
						return map[string]any{"error": map[string]any{"code": "failed", "message": "private sentinel"}}
					}
					return popupOK()
				}
				if !queued {
					return popupTree(popupNode("client-player"))
				}
				n := popupNode("client-player", "selected")
				switch name {
				case "missing-tab":
					return popupTree(popupNode("quality-menu"))
				case "duplicate-tab":
					return popupTree(n, popupNode("client-player", "selected"))
				case "invalid-states":
					n["states"] = "private sentinel"
				case "null-tree":
					return map[string]any{"result": nil}
				}
				return popupTree(n)
			})
			err := c.clientAction(context.Background(), "client-player")
			if err == nil || strings.Contains(err.Error(), "private sentinel") {
				t.Fatalf("navigation error was ignored or its private content leaked: %v", err)
			}
			reads := 2
			if name == "rejected-action" {
				reads = 1
			}
			if trace.count("tree", "") != reads || trace.count("act", "client-player") != 1 {
				t.Fatal("failed navigation was retried or continued")
			}
		})
	}
}

func TestClientNavigationSharesItsTwoSecondBudget(t *testing.T) {
	readyFrames, selectedFrames := 0, 0
	queued := false
	c, _ := popupClient(t, func(request popupRequest) map[string]any {
		if request.Method == "act" {
			queued = true
			return popupOK()
		}
		if !queued {
			readyFrames++
			if readyFrames == 1 {
				return popupTree(popupNode("prior-page"))
			}
			return popupTree(popupNode("client-player"))
		}
		selectedFrames++
		if selectedFrames == 1 {
			return popupTree(popupNode("client-player"))
		}
		return popupTree(popupNode("client-player", "selected"))
	})
	conn := &noticeDeadlineConn{Conn: c.conn}
	c.conn = conn
	start := time.Now()
	if err := c.clientAction(context.Background(), "client-player"); err != nil {
		t.Fatal(err)
	}
	if len(conn.deadlines) != 5 {
		t.Fatal("did not observe readiness, the one action and target-tab completion")
	}
	budget := conn.deadlines[0].Sub(start)
	if budget < 1900*time.Millisecond || budget > 2100*time.Millisecond {
		t.Fatalf("navigation does not own its two-second budget: %v", budget)
	}
	for _, deadline := range conn.deadlines[1:] {
		if !deadline.Equal(conn.deadlines[0]) {
			t.Fatal("navigation readiness, action or completion renewed its deadline")
		}
	}
}

// The native shell uses buttons for its four tabs. In its automation tree the
// active button carries checked; the button on the previous page can still be
// checked in the first frame after the target action is acknowledged.
func TestClientNavigationWaitsForCheckedTargetButton(t *testing.T) {
	for _, key := range []string{"client-catalog", "client-player", "client-history", "client-account"} {
		t.Run(key, func(t *testing.T) {
			frames := 0
			c, trace := popupClient(t, func(r popupRequest) map[string]any {
				if r.Method == "act" {
					return popupOK()
				}
				frames++
				if frames < 3 {
					return popupTree(popupNode(key, "focusable"), popupNode("prior-tab", "focusable", "checked"))
				}
				return popupTree(popupNode(key, "focusable", "checked"), popupNode("prior-tab", "focusable"))
			})
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			if err := c.clientAction(ctx, key); err != nil {
				t.Fatalf("checked target button was not recognized: %v", err)
			}
			if frames != 3 || trace.count("act", key) != 1 {
				t.Fatalf("frames=%d tab presses=%d; want three frames and one press", frames, trace.count("act", key))
			}
		})
	}
}
