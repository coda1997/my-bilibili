package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"my-bilibili/internal/adapters/mpv"
)

func (c *client) resources(ctx context.Context) (mpv.Resources, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return mpv.Resources{}, err
	}
	var result mpv.Resources
	found := false
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "resources ") {
			var valid bool
			if result, valid = resourceObservation(strings.TrimPrefix(n.Label, "resources ")); found || !valid {
				return mpv.Resources{}, errors.New("native resource observation invalid")
			}
			found = true
		}
	}
	if !found {
		return mpv.Resources{}, errors.New("native resource observation absent")
	}
	return result, nil
}

func resourceObservation(text string) (mpv.Resources, bool) {
	var result mpv.Resources
	d := json.NewDecoder(strings.NewReader(text))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return mpv.Resources{}, false
	}
	fields := map[string]*int64{"Cores": &result.Cores, "Renderers": &result.Renderers, "Workers": &result.Workers}
	seen := make(map[string]bool)
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return mpv.Resources{}, false
		}
		seen[key] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return mpv.Resources{}, false
		}
		// Decode exact required fields only. Missing/null fields and case
		// aliases must not become observed zero counters or override them.
		if counter, required := fields[key]; required {
			var value *int64
			if json.Unmarshal(raw, &value) != nil || value == nil || *value < 0 {
				return mpv.Resources{}, false
			}
			*counter = *value
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || !seen["Cores"] || !seen["Renderers"] || !seen["Workers"] {
		return mpv.Resources{}, false
	}
	if _, err := d.Token(); err != io.EOF {
		return mpv.Resources{}, false
	}
	return result, true
}
