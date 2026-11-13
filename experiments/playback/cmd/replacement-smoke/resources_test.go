package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"my-bilibili/experiments/playback/mpvprobe"
)

func resourceClient(t *testing.T, labels []string) *client {
	t.Helper()
	nodes := make([]node, len(labels))
	for i, label := range labels {
		nodes[i].Label = label
	}
	payload, err := json.Marshal(map[string]any{"id": 1, "result": map[string]any{"nodes": nodes}})
	if err != nil {
		t.Fatal(err)
	}
	return newClient(&failedConnection{response: bytes.NewReader(append(payload, '\n'))})
}

func TestResourceObservationsRequireCompleteCounters(t *testing.T) {
	const zero = `{"Cores":0,"Renderers":0,"Workers":0}`
	cases := []struct {
		name   string
		labels []string
	}{
		{"absent", []string{"unrelated"}},
		{"empty", []string{"resources {}"}},
		{"null", []string{"resources null"}},
		{"array", []string{"resources []"}},
		{"malformed", []string{`resources {"Cores":`}},
		{"trailing-value", []string{"resources " + zero + " null"}},
		{"duplicate-required-key", []string{`resources {"Cores":1,"Cores":0,"Renderers":0,"Workers":0}`}},
		{"duplicate-unknown-key", []string{`resources {"Cores":0,"Renderers":0,"Workers":0,"extra":1,"extra":2}`}},
		{"multiple-observations", []string{"resources " + zero, "resources " + zero}},
		{"invalid-before-valid", []string{"resources {}", "resources " + zero}},
		{"invalid-after-valid", []string{"resources " + zero, "resources {}"}},
		{"all-null", []string{`resources {"Cores":null,"Renderers":null,"Workers":null}`}},
	}
	for _, field := range []string{"Cores", "Renderers", "Workers"} {
		needle := `"` + field + `":0`
		for _, value := range []string{"null", "-1", "0.5", "1e0", "9223372036854775808", `"0"`, "false"} {
			cases = append(cases, struct {
				name   string
				labels []string
			}{field + "-" + value, []string{"resources " + strings.Replace(zero, needle, `"`+field+`":`+value, 1)}})
		}
		for _, replacement := range []string{`"ignored":0`, `"` + strings.ToLower(field) + `":0`} {
			cases = append(cases, struct {
				name   string
				labels []string
			}{field + "-missing-" + replacement, []string{"resources " + strings.Replace(zero, needle, replacement, 1)}})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := resourceClient(t, tc.labels).resources(context.Background()); err == nil {
				t.Errorf("invalid resource observation accepted: %+v", got)
			}
			if err := resourceClient(t, tc.labels).zeroResources(context.Background()); err == nil {
				t.Error("invalid resource observation passed cleanup")
			}
		})
	}
}

func TestResourceObservationsPreserveValidCounters(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want mpvprobe.Resources
	}{
		{"zero", `{"Cores":0,"Renderers":0,"Workers":0}`, mpvprobe.Resources{}},
		{"positive", `{"Cores":1,"Renderers":2,"Workers":3}`, mpvprobe.Resources{Cores: 1, Renderers: 2, Workers: 3}},
		{"maximum", `{"Cores":9223372036854775807,"Renderers":0,"Workers":0}`, mpvprobe.Resources{Cores: 9223372036854775807}},
		{"unknown-fields", `{"Cores":0,"Renderers":0,"Workers":0,"extra":{"nested":true},"cores":4}`, mpvprobe.Resources{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{"unrelated", "resources " + tc.text}
			got, err := resourceClient(t, labels).resources(context.Background())
			if err != nil || got != tc.want {
				t.Fatalf("valid counters changed: got=%+v want=%+v err=%v", got, tc.want, err)
			}
			err = resourceClient(t, labels).zeroResources(context.Background())
			if (err == nil) != (tc.want == (mpvprobe.Resources{})) {
				t.Fatalf("cleanup result disagrees with observed ownership: counters=%+v err=%v", tc.want, err)
			}
		})
	}
}

func TestResourceObservationPreservesTransportFailure(t *testing.T) {
	c := newClient(&failedConnection{response: strings.NewReader("")})
	if _, err := c.resources(context.Background()); err == nil || !strings.Contains(err.Error(), "cause=eof") {
		t.Fatalf("resource observation lost transport failure: %v", err)
	}
}
