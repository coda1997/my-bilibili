package main

import (
	"bufio"
	"context"
	"encoding/json"
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
