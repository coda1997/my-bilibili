package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type fullscreenCompletion struct{ requests, entered, exited uint64 }

// The e2e node carries counters recorded by the existing native DidEnter and
// DidExit callbacks. Geometry and the style bit can change before these callbacks.
// Decode this separate diagnostic boundary without changing playback observations.
func nativeCompletion(nodes []node) (fullscreenCompletion, error) {
	n, err := uniqueNode(nodes, "e2e-fullscreen")
	if err != nil {
		return fullscreenCompletion{}, errors.New("native fullscreen completion unavailable")
	}
	invalid := errors.New("invalid native fullscreen completion")
	d := json.NewDecoder(strings.NewReader(n.Label))
	if token, e := d.Token(); e != nil || token != json.Delim('{') {
		return fullscreenCompletion{}, invalid
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, e := d.Token()
		key, ok := token.(string)
		if e != nil || !ok {
			return fullscreenCompletion{}, invalid
		}
		if _, duplicate := fields[key]; duplicate {
			return fullscreenCompletion{}, invalid
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return fullscreenCompletion{}, invalid
		}
		fields[key] = value
	}
	if token, e := d.Token(); e != nil || token != json.Delim('}') {
		return fullscreenCompletion{}, invalid
	}
	if _, e := d.Token(); e != io.EOF {
		return fullscreenCompletion{}, invalid
	}
	var schema, sequence uint64
	var c fullscreenCompletion
	for key, target := range map[string]*uint64{"schema_version": &schema, "sequence": &sequence, "requests": &c.requests, "did_enter": &c.entered, "did_exit": &c.exited} {
		raw, ok := fields[key]
		if !ok || string(raw) == "null" || json.Unmarshal(raw, target) != nil {
			return fullscreenCompletion{}, invalid
		}
	}
	if schema != 1 || sequence != c.requests+c.entered+c.exited {
		return fullscreenCompletion{}, invalid
	}
	return c, nil
}
