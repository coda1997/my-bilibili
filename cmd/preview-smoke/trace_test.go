package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRPCTraceKeepsMetadataWithoutRemotePayload(t *testing.T) {
	r := &report{Scenario: "product", traceStart: time.Now()}
	c := &client{report: r, phase: "entry-correspondence", cycle: 2, id: 8}
	remote := &treeResult{Nodes: []node{
		{Key: "fullscreen-toggle", Label: "Exit full", Bounds: rectangle{Width: 80, Height: 20}},
		{Key: "private", Label: "secret-media-url"},
		{Key: "e2e-fullscreen", Label: `{"schema_version":1,"sequence":1,"events":[{"kind":"secret-remote-error"}]}`},
	}}
	c.recordRPC(time.Now(), "tree", nil, remote, nil)
	c.recordRPC(time.Now(), "capture", nil, map[string]string{"png": "secret-image"}, nil)
	c.recordRPC(time.Now(), "act", map[string]any{"key": "secret-key"}, nil, context.DeadlineExceeded)
	b, err := json.Marshal(r.Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") {
		t.Fatalf("remote payload leaked: %s", b)
	}
	if len(r.Diagnostics) != 3 || r.Diagnostics[0].Cycle != 2 || r.Diagnostics[0].Phase != "entry-correspondence" || r.Diagnostics[0].Nodes[0].Label != "Exit full" || r.Diagnostics[2].Outcome != "deadline" {
		t.Fatalf("missing phase/result metadata: %s", b)
	}
	if r.Diagnostics[0].Native.Events[0].Kind != "unknown" {
		t.Fatal("untrusted event kind retained")
	}
}
