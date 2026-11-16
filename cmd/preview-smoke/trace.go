package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Diagnostic metadata is selected explicitly: never retain raw RPC envelopes,
// media inputs, arbitrary labels, native errors or encoded capture contents.
type rpcTrace struct {
	At, Elapsed                              time.Duration
	Scenario, Phase, Method, Action, Outcome string
	Cycle, ID                                int
	Fullscreen                               *bool                  `json:",omitempty"`
	Nodes                                    []traceNode            `json:",omitempty"`
	Windows                                  []rectangle            `json:",omitempty"`
	Native                                   *nativeFullscreenTrace `json:",omitempty"`
}

type nativeFullscreenTrace struct {
	SchemaVersion int                     `json:"schema_version"`
	Sequence      uint64                  `json:"sequence"`
	Requests      uint64                  `json:"requests"`
	DidEnter      uint64                  `json:"did_enter"`
	DidExit       uint64                  `json:"did_exit"`
	Dropped       uint64                  `json:"dropped"`
	Events        []nativeFullscreenEvent `json:"events"`
}
type nativeFullscreenEvent struct {
	Sequence      uint64    `json:"sequence"`
	RelativeNS    int64     `json:"relative_ns"`
	Kind          string    `json:"kind"`
	Desired       *bool     `json:"desired,omitempty"`
	Style         bool      `json:"style"`
	ContentBounds rectangle `json:"content_bounds"`
}

type traceNode struct {
	Key      string
	Bounds   rectangle
	Complete bool
	Label    string `json:",omitempty"`
}

func (c *client) recordRPC(start time.Time, method string, params, result any, err error) {
	if c.report == nil {
		return
	}
	t := rpcTrace{At: start.Sub(c.report.traceStart), Elapsed: time.Since(start), Scenario: c.report.Scenario, Phase: c.phase, Cycle: c.cycle, Method: method, ID: c.id, Outcome: "ok"}
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			t.Outcome = "deadline"
		case errors.Is(err, context.Canceled):
			t.Outcome = "canceled"
		default:
			t.Outcome = "failed"
		}
	}
	if p, ok := params.(map[string]any); ok {
		if key, ok := p["key"].(string); ok {
			switch key {
			case "fullscreen-toggle", "Escape", "e2e-close-window":
				t.Action = key
			}
		}
	}
	if err == nil {
		switch wire := result.(type) {
		case *treeResult:
			for _, n := range wire.Nodes {
				switch n.Key {
				case "e2e-fullscreen":
					var state nativeFullscreenTrace
					if json.Unmarshal([]byte(n.Label), &state) == nil && state.SchemaVersion == 1 {
						for i := range state.Events {
							switch state.Events[i].Kind {
							case "request", "did-enter", "did-exit":
							default:
								state.Events[i].Kind = "unknown"
							}
						}
						t.Native = &state
					}
				case "preview-video", "fullscreen-toggle", "list-part-part-1":
					v := traceNode{Key: n.Key.(string), Bounds: n.Bounds, Complete: n.Bounds.complete}
					if n.Key == "fullscreen-toggle" && (n.Label == "Fullscreen" || n.Label == "Exit full") {
						v.Label = n.Label
					}
					t.Nodes = append(t.Nodes, v)
				}
			}
			if o, e := observation(wire.Nodes); e == nil {
				t.Fullscreen = &o.Fullscreen
			}
		case *windowResult:
			for _, w := range wire.Windows {
				t.Windows = append(t.Windows, w.ContentBounds)
			}
		}
	}
	c.report.Diagnostics = append(c.report.Diagnostics, t)
}
