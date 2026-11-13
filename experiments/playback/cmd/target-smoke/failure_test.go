package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

// Decode the serialized public result so these regressions can first run against
// the old driver, and assert the actual evidence contract rather than its storage.
type decodedFailure struct {
	Phase        string             `json:"phase"`
	Code         contract.ErrorCode `json:"code"`
	Elapsed      time.Duration      `json:"elapsed_ns"`
	LastSnapshot *struct {
		Phase   string        `json:"phase"`
		Elapsed time.Duration `json:"elapsed_ns"`
		Value   observed      `json:"value"`
	} `json:"last_snapshot"`
	LastResources *struct {
		Phase   string             `json:"phase"`
		Elapsed time.Duration      `json:"elapsed_ns"`
		Value   mpvprobe.Resources `json:"value"`
	} `json:"last_resources"`
}

func failures(t *testing.T, r *report) (*decodedFailure, *decodedFailure) {
	t.Helper()
	var d struct {
		Session *decodedFailure `json:"session_failure"`
		Cleanup *decodedFailure `json:"cleanup_failure"`
	}
	if err := json.Unmarshal([]byte(mustJSON(r)), &d); err != nil {
		t.Fatal(err)
	}
	return d.Session, d.Cleanup
}
func TestFailureEvidenceTimeoutPhases(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		state       contract.State
		hidden      bool
	}{
		{"loading", "ready", contract.Loading, false},
		{"hidden-ready", "ready", contract.Ready, true},
		{"picture-pending", "picture-tracks", contract.Ready, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := observed{Snapshot: contract.Snapshot{State: contract.Idle, Rate: 1, Volume: 1}}
			var actions []string
			c, r := pipeClient(t, func(q fakeRequest) any {
				if q.Method == "act" {
					actions = append(actions, q.Params.Key)
					s.Generation = 1
					s.State = tc.state
					s.Owned = true
					s.WindowHidden = tc.hidden
					s.Duration = time.Minute
					return map[string]bool{"ok": true}
				}
				return fakeNodes(s, false, true, mpvprobe.Resources{Cores: 1, Renderers: 1, Workers: 1})
			})
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			if err := c.capabilities(ctx, r); safeCode(err) != contract.Timeout {
				t.Fatalf("want timeout: %v", err)
			}
			f, cleanup := failures(t, r)
			if f == nil {
				t.Fatal("missing normalized primary failure evidence")
			}
			if cleanup != nil || f.Phase != tc.phase || f.Code != contract.Timeout || f.LastSnapshot == nil || f.LastResources == nil {
				t.Fatalf("failure=%+v cleanup=%+v", f, cleanup)
			}
			sample := f.LastSnapshot
			if sample.Value.State != tc.state || sample.Value.WindowHidden != tc.hidden || sample.Value.Generation != 1 || !sample.Value.Owned || sample.Phase != tc.phase || sample.Elapsed <= 0 || sample.Elapsed > f.Elapsed {
				t.Fatalf("bad state provenance: %+v", sample)
			}
			if f.LastResources.Phase != tc.phase || f.LastResources.Value.Cores != 1 || f.LastResources.Elapsed > f.Elapsed {
				t.Fatalf("bad resource provenance: %+v", f.LastResources)
			}
			if strings.Join(actions, ",") != "open" {
				t.Fatalf("extra actions: %v", actions)
			}
		})
	}
}
func TestFailureEvidenceAbsentOrMalformedSnapshot(t *testing.T) {
	valid := mustJSON(observed{Snapshot: contract.Snapshot{State: contract.Idle, Rate: 1, Volume: 1}})
	for name, label := range map[string]string{
		"absent":       "unrelated SECRET_RAW_LOCATOR",
		"malformed":    "snapshot {SECRET_RAW_LOCATOR",
		"partial":      `snapshot {"state":"idle"}`,
		"null":         "snapshot " + strings.Replace(valid, `"owned":false`, `"owned":null`, 1),
		"duplicate":    "snapshot " + strings.Replace(valid, `"state":"idle"`, `"state":"idle","state":"ready"`, 1),
		"invalid-code": "snapshot " + strings.Replace(valid, `"error":""`, `"error":"SECRET_RAW_LOCATOR"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			c, r := pipeClient(t, func(q fakeRequest) any {
				if q.Method != "tree" {
					t.Error("invalid snapshot led to action")
				}
				return map[string]any{"nodes": []map[string]any{{"label": label}}}
			})
			if err := c.capabilities(context.Background(), r); safeCode(err) != contract.Media {
				t.Fatalf("want safe failure: %v", err)
			}
			f, _ := failures(t, r)
			if f == nil || f.Phase != "initial-snapshot" || f.LastSnapshot != nil || f.LastResources != nil {
				t.Fatalf("fabricated or missing evidence: %+v", f)
			}
			if strings.Contains(mustJSON(r), "SECRET") {
				t.Fatal("raw snapshot leaked")
			}
		})
	}
}
func TestFailureEvidenceKeepsEarlierValidSamples(t *testing.T) {
	n := 0
	original := observed{Snapshot: contract.Snapshot{State: contract.Idle, Rate: 1, Volume: 1}}
	c, r := pipeClient(t, func(q fakeRequest) any {
		if q.Method == "act" {
			return map[string]bool{"ok": true}
		}
		n++
		if n == 1 {
			return fakeNodes(original, false, true, mpvprobe.Resources{})
		}
		return map[string]any{"nodes": []map[string]any{{"label": `snapshot {"state":"ready"}`}, {"label": `resources {"Cores":1}`}}}
	})
	if err := c.capabilities(context.Background(), r); safeCode(err) != contract.Media {
		t.Fatalf("want failure: %v", err)
	}
	f, _ := failures(t, r)
	if f == nil || f.Phase != "ready" || f.LastSnapshot == nil || f.LastResources == nil {
		t.Fatalf("lost prior sample: %+v", f)
	}
	if f.LastSnapshot.Phase != "initial-snapshot" || f.LastSnapshot.Value != original || f.LastResources.Phase != "initial-snapshot" || f.LastResources.Value != (mpvprobe.Resources{}) || f.LastSnapshot.Elapsed > f.Elapsed {
		t.Fatalf("stale sample redated/overwritten: %+v", f)
	}
}
func TestFailureEvidenceDropsUnknownFieldsAndInvalidResources(t *testing.T) {
	for _, resource := range []string{`{"Cores":-1,"Renderers":0,"Workers":0}`, `{"Cores":0}`, `{"Cores":null,"Renderers":0,"Workers":0}`, `{"Cores":0,"Cores":1,"Renderers":0,"Workers":0}`} {
		t.Run(resource, func(t *testing.T) {
			raw := mustJSON(observed{Snapshot: contract.Snapshot{Generation: 1, State: contract.Failed, Error: contract.Media}})
			raw = strings.TrimSuffix(raw, "}") + `,"url":"SECRET_RAW_LOCATOR","State":"SECRET_RAW_LOCATOR"}`
			c, r := pipeClient(t, func(fakeRequest) any {
				return map[string]any{"nodes": []map[string]any{{"label": "snapshot " + raw}, {"label": "resources " + resource}, {"label": "SECRET_RAW_LOCATOR"}}}
			})
			// Initial state is valid; the subsequent failure is from the actual until path.
			if err := c.capabilities(context.Background(), r); safeCode(err) != contract.Media {
				t.Fatalf("want failure: %v", err)
			}
			f, _ := failures(t, r)
			if f == nil || f.LastSnapshot == nil || f.LastSnapshot.Value.State != contract.Failed || f.LastResources != nil {
				t.Fatalf("unsafe normalization: %+v", f)
			}
			if strings.Contains(mustJSON(r), "SECRET") || strings.Contains(mustJSON(r), `"url"`) {
				t.Fatal("unknown fields leaked")
			}
		})
	}
}
func TestFailureEvidencePrimarySurvivesForcedCleanup(t *testing.T) {
	c, r := pipeClient(t, func(fakeRequest) any {
		return fakeNodes(observed{Snapshot: contract.Snapshot{State: contract.Failed, Error: contract.Unsupported}}, false, true, mpvprobe.Resources{})
	})
	if err := c.capabilities(context.Background(), r); safeCode(err) != contract.Unsupported {
		t.Fatalf("primary: %v", err)
	}
	primary, _ := failures(t, r)
	if primary == nil {
		t.Fatal("missing primary failure")
	}
	before := mustJSON(primary)
	p, socket := helperProcess(t, "hang")
	if err := shutdown(p, socket, r, 20*time.Millisecond); err == nil || !r.ForcedCleanup || !p.reaped {
		t.Fatalf("missing forced cleanup: %v", err)
	}
	primary, cleanup := failures(t, r)
	if mustJSON(primary) != before || cleanup == nil || cleanup.Phase != "cleanup-exit" || cleanup.Code != contract.Timeout {
		t.Fatalf("primary overwritten or cleanup absent: %+v %+v", primary, cleanup)
	}
	if cleanup.LastSnapshot == nil || cleanup.LastSnapshot.Elapsed != primary.LastSnapshot.Elapsed || cleanup.LastSnapshot.Phase != primary.LastSnapshot.Phase {
		t.Fatal("cleanup relabeled stale state")
	}
}

// Distinct fixed sample times test preservation without assuming the platform
// clock advances between adjacent RPC observations and the resulting failure.
func TestFailureEvidencePreservesSeededProvenance(t *testing.T) {
	original := observed{Snapshot: contract.Snapshot{State: contract.Idle, Rate: 1, Volume: 1}}
	resources := mpvprobe.Resources{Cores: 1, Renderers: 1, Workers: 2}
	r := &report{
		started:       time.Now().Add(-time.Second),
		lastSnapshot:  &sample[observed]{phaseInitialSnapshot, 17 * time.Millisecond, original},
		lastResources: &sample[mpvprobe.Resources]{phaseInitialSnapshot, 23 * time.Millisecond, resources},
	}
	r.setPhase(phaseReady)
	r.observe([]node{{Label: `snapshot {"state":"ready"}`}, {Label: `resources {"Cores":0}`}})
	r.sessionFailed(contract.Media)
	r.setPhase(phaseCleanupExit)
	r.observe([]node{{Label: "snapshot null"}, {Label: "resources null"}})
	r.CleanupFailure = r.failure(contract.Timeout)
	primary, cleanup := failures(t, r)
	for _, f := range []*decodedFailure{primary, cleanup} {
		if f == nil || f.LastSnapshot == nil || f.LastResources == nil {
			t.Fatalf("lost seeded provenance: %+v", f)
		}
		if f.LastSnapshot.Value != original || f.LastSnapshot.Phase != "initial-snapshot" || f.LastSnapshot.Elapsed != 17*time.Millisecond || f.LastResources.Value != resources || f.LastResources.Phase != "initial-snapshot" || f.LastResources.Elapsed != 23*time.Millisecond {
			t.Fatalf("seeded provenance changed: %+v %+v", f.LastSnapshot, f.LastResources)
		}
	}
	if primary.Phase != "ready" || primary.Code != contract.Media || cleanup.Phase != "cleanup-exit" || cleanup.Code != contract.Timeout {
		t.Fatalf("failure attribution changed: %+v %+v", primary, cleanup)
	}
}
