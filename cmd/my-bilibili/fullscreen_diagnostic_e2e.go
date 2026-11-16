//go:build e2e

package main

import (
	"encoding/json"
	"time"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
)

const fullscreenEventLimit = 64

type fullscreenBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}
type fullscreenEvent struct {
	Sequence      uint64           `json:"sequence"`
	RelativeNS    int64            `json:"relative_ns"`
	Kind          string           `json:"kind"`
	Desired       *bool            `json:"desired,omitempty"`
	Style         bool             `json:"style"`
	ContentBounds fullscreenBounds `json:"content_bounds"`
}
type fullscreenObservation struct {
	SchemaVersion int               `json:"schema_version"`
	Sequence      uint64            `json:"sequence"`
	Requests      uint64            `json:"requests"`
	DidEnter      uint64            `json:"did_enter"`
	DidExit       uint64            `json:"did_exit"`
	Dropped       uint64            `json:"dropped"`
	Events        []fullscreenEvent `json:"events"`
}

// Composition hooks and rendering all run on the native main thread. The
// recorder only observes requests/callbacks; it never schedules a transition,
// redraw or retry. A callback has no invented request association or desired bit.
type fullscreenTelemetry struct {
	started                               time.Time
	sequence, requests, didEnter, didExit uint64
	events                                []fullscreenEvent
}

func (a *application) recordFullscreen(kind string, desired *bool) {
	at := time.Now()
	style := a.window.IsFullScreen()
	bounds := a.window.ContentBounds()
	a.fullscreen.record(at, kind, desired, style, bounds)
}
func (f *fullscreenTelemetry) record(at time.Time, kind string, desired *bool, style bool, bounds mygo.Rectangle) {
	if f.started.IsZero() {
		f.started = at
	}
	f.sequence++
	switch kind {
	case "request":
		f.requests++
	case "did-enter":
		f.didEnter++
	case "did-exit":
		f.didExit++
	}
	event := fullscreenEvent{Sequence: f.sequence, RelativeNS: at.Sub(f.started).Nanoseconds(), Kind: kind, Style: style, ContentBounds: fullscreenBounds{bounds.X, bounds.Y, bounds.Width, bounds.Height}}
	if desired != nil {
		value := *desired
		event.Desired = &value
	}
	if len(f.events) == fullscreenEventLimit {
		copy(f.events, f.events[1:])
		f.events[len(f.events)-1] = event
	} else {
		f.events = append(f.events, event)
	}
}
func (f *fullscreenTelemetry) snapshot() fullscreenObservation {
	events := make([]fullscreenEvent, len(f.events))
	copy(events, f.events)
	for i := range events {
		if events[i].Desired != nil {
			value := *events[i].Desired
			events[i].Desired = &value
		}
	}
	return fullscreenObservation{SchemaVersion: 1, Sequence: f.sequence, Requests: f.requests, DidEnter: f.didEnter, DidExit: f.didExit, Dropped: f.sequence - uint64(len(events)), Events: events}
}
func (a *application) renderFullscreen(c *gui.Context) {
	// This is a render-cached copy of prior recorded events. Reading the node
	// does not query native state, acknowledge an action, or trigger a new one.
	data, _ := json.Marshal(a.fullscreen.snapshot())
	gui.Text(c.Key("e2e-fullscreen"), string(data)).Absolute().Left(-20000).Top(-20000).Size(1, 1)
}
