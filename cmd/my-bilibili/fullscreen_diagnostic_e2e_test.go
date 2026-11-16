//go:build e2e

package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
)

func TestFullscreenTelemetryOrdersNativeCallbacksAndRequests(t *testing.T) {
	var f fullscreenTelemetry
	start := time.Now()
	desired := true
	ordinary := mygo.Rectangle{X: 335, Y: 145, Width: 1120, Height: 760}
	full := mygo.Rectangle{X: 0, Y: 34, Width: 1710, Height: 1073}
	f.record(start, "request", &desired, false, ordinary)
	desired = false // the recorder owns the original request bit
	f.record(start.Add(17*time.Millisecond), "did-enter", nil, true, full)
	f.record(start.Add(20*time.Millisecond), "request", &desired, true, full)
	f.record(start.Add(44*time.Millisecond), "did-exit", nil, false, ordinary)
	got := f.snapshot()
	if got.SchemaVersion != 1 || got.Sequence != 4 || got.Requests != 2 || got.DidEnter != 1 || got.DidExit != 1 || got.Dropped != 0 {
		t.Fatal(got)
	}
	for i, event := range got.Events {
		if event.Sequence != uint64(i+1) || i > 0 && event.RelativeNS < got.Events[i-1].RelativeNS {
			t.Fatal("lost monotonic order", got)
		}
	}
	if !*got.Events[0].Desired || *got.Events[2].Desired || got.Events[1].Desired != nil || got.Events[3].Desired != nil {
		t.Fatal("request desired bit aliased or callback invented intent", got)
	}
	if got.Events[1].RelativeNS != 17*time.Millisecond.Nanoseconds() || got.Events[0].Style || !got.Events[1].Style || got.Events[3].Style || got.Events[1].ContentBounds.Width != 1710 {
		t.Fatal("lost observed time/style/bounds", got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Events []map[string]any `json:"events"`
	}
	if err = json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire.Events[2]["desired"]; !ok {
		t.Fatal("false request desired bit omitted")
	}
	if _, ok := wire.Events[3]["desired"]; ok {
		t.Fatal("callback claims a desired bit")
	}
}
func TestFullscreenTelemetryBoundsHistoryAndOwnsSnapshots(t *testing.T) {
	var f fullscreenTelemetry
	start := time.Now()
	desired := true
	f.record(start, "request", &desired, false, mygo.Rectangle{Width: 1120, Height: 760})
	saved := f.snapshot()
	for i := 1; i < 80; i++ {
		f.record(start.Add(time.Duration(i)*time.Millisecond), "did-enter", nil, true, mygo.Rectangle{Width: 1710, Height: 1073})
	}
	got := f.snapshot()
	if len(got.Events) != fullscreenEventLimit || got.Sequence != 80 || got.Dropped != 16 || got.Requests != 1 || got.DidEnter != 79 || got.Events[0].Sequence != 17 {
		t.Fatal("bounded events lost total counters", got)
	}
	if saved.Sequence != 1 || len(saved.Events) != 1 || saved.Events[0].Kind != "request" || !*saved.Events[0].Desired {
		t.Fatal("later recording mutated prior snapshot", saved)
	}
	*saved.Events[0].Desired = false
	fresh := fullscreenTelemetry{}
	fresh.record(start, "request", &desired, false, mygo.Rectangle{})
	copy := fresh.snapshot()
	*copy.Events[0].Desired = false
	copy.Events[0].Kind = "changed"
	if next := fresh.snapshot(); next.Events[0].Kind != "request" || !*next.Events[0].Desired {
		t.Fatal("snapshot can mutate recorder")
	}
}
func TestFullscreenTelemetryNodeIsReadOnlyAndIndependent(t *testing.T) {
	a := &application{}
	desired := false
	a.fullscreen.record(time.Now(), "request", &desired, true, mygo.Rectangle{Width: 1710, Height: 1073})
	before := a.fullscreen.snapshot()
	// A nil window is intentional: rendering only copies prior records.
	ui := gui.NewTester(a.renderFullscreen, 1120, 760)
	ui.Frame()
	texts := ui.Texts()
	if len(texts) != 1 {
		t.Fatal("expected exactly one diagnostic JSON text", texts)
	}
	var got fullscreenObservation
	if err := json.Unmarshal([]byte(texts[0]), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, got) || !reflect.DeepEqual(before, a.fullscreen.snapshot()) {
		t.Fatal("render mutated telemetry or changed schema", got)
	}
	var zero fullscreenTelemetry
	data, err := json.Marshal(zero.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	if events, ok := raw["events"].([]any); !ok || len(events) != 0 {
		t.Fatal("empty history must be an explicit empty array", string(data))
	}
}
