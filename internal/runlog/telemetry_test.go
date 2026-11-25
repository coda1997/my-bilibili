package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"my-bilibili/internal/spec"
)

func TestStartUsesSchemaV2AndValidatesTaskJSON(t *testing.T) {
	store, manifest := testStore(t)
	if manifest.SchemaVersion != 2 {
		t.Fatalf("got SchemaVersion=%d, want 2", manifest.SchemaVersion)
	}
	view, err := store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Budgets) != 0 {
		t.Fatalf("plain run without telemetry should omit Budgets, got %+v", view.Budgets)
	}

	def := spec.Definition{SchemaVersion: 1, SpecID: "test", Tasks: []spec.Task{
		{ID: "T1", Title: "First", WritePaths: []string{"internal/test"}, Acceptance: []string{"TEST-AC-01"}, Checks: []string{"go test ./..."}},
	}}
	otherDef := spec.Definition{SchemaVersion: 1, SpecID: "test", Tasks: []spec.Task{
		{ID: "T2", Title: "Different", WritePaths: []string{"internal/test"}, Acceptance: []string{"TEST-AC-01"}, Checks: []string{"go test ./..."}},
	}}
	otherJSON, err := json.Marshal(otherDef)
	if err != nil {
		t.Fatal(err)
	}
	badSnapshot := spec.Snapshot{
		Path:       "specs/test",
		Markdown:   []byte("### TEST-AC-01\nObservable outcome.\n"),
		TaskJSON:   otherJSON,
		Definition: def,
	}
	if _, err := store.Start(badSnapshot, testSource); err == nil {
		t.Fatal("accepted mismatched TaskJSON vs Definition")
	}

	badUnknown := spec.Snapshot{
		Path:       "specs/test",
		Markdown:   []byte("### TEST-AC-01\nObservable outcome.\n"),
		TaskJSON:   []byte(`{"schema_version":1,"spec_id":"test","unknown":true,"tasks":[]}`),
		Definition: def,
	}
	if _, err := store.Start(badUnknown, testSource); err == nil {
		t.Fatal("accepted TaskJSON with unknown field")
	}
}

func TestSchemaV1CompatibilityAndImmutability(t *testing.T) {
	store, manifest := testStore(t)
	dir := runDir(t, store, manifest.ID)
	// Rewrite manifest to SchemaVersion 1 to simulate a pre-Phase-1 run on disk.
	manifest.SchemaVersion = 1
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	// Plain v1 note still succeeds and reads cleanly.
	if err := store.Note(manifest.ID, "implementation", "Legacy v1 note", nil, testSource); err != nil {
		t.Fatalf("v1 plain note failed: %v", err)
	}
	view, err := store.Read(manifest.ID)
	if err != nil || view.Manifest.SchemaVersion != 1 || len(view.Events) != 2 {
		t.Fatalf("v1 read failed: view=%+v err=%v", view, err)
	}
	// v2 telemetry fields and human-override are rejected on v1 runs.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase:   "implementation",
		Message: "v2 field on v1 run",
		TaskID:  "T1",
	}, testSource); err == nil {
		t.Fatal("accepted v2 field on SchemaVersion 1 run")
	}
	if err := store.HumanOverride(manifest.ID, "T1", "human direction", HumanOverride{ExtraFixIterations: 1}, nil, testSource); err == nil {
		t.Fatal("accepted human-override on SchemaVersion 1 run")
	}
	// Tampering an existing v1 event on disk to include v2 fields fails closed on Read.
	tampered := view.Events[1]
	tampered.TaskID = "T1"
	raw, err := json.MarshalIndent(tampered, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events", "000002.json"), append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(manifest.ID); err == nil {
		t.Fatal("Read accepted v2 field inside SchemaVersion 1 run")
	}
}

func TestStructuredTelemetryAndFindingsLifecycle(t *testing.T) {
	store, manifest := testStore(t)

	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase:       "review",
		Message:     "Round 1 findings",
		TaskID:      "T1",
		Role:        "code-review",
		ReviewRound: 1,
		DurationMS:  420,
		Findings: []Finding{
			{ID: "CR-02", Tier: "BLOCKING", BlockingClass: 4, Severity: "P1", File: "internal/test/a.go", RecurringKey: "class4:internal/test", Disposition: "open"},
			{ID: "CR-01", Tier: "BLOCKING", BlockingClass: 1, Severity: "high"},
			{ID: "ADV-01", Tier: "ADVISORY", Severity: "low", Disposition: "not-filed"},
		},
	}, testSource); err != nil {
		t.Fatal(err)
	}

	view, err := store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := view.Budgets["T1"]
	if !ok {
		t.Fatalf("missing T1 budget in %+v", view.Budgets)
	}
	if b.ReviewRounds != 1 || b.AllowedReviewRounds != 3 || !slices.Equal(b.OpenBlocking, []string{"CR-01", "CR-02"}) {
		t.Fatalf("unexpected T1 budget: %+v", b)
	}
	// Summary carries the same derived Budgets with exact JSON tags.
	summaryJSON, err := json.Marshal(view.Summary())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"budgets"`, `"review_rounds":1`, `"allowed_review_rounds":3`, `"open_blocking":["CR-01","CR-02"]`} {
		if !strings.Contains(string(summaryJSON), want) {
			t.Fatalf("summary JSON missing %s: %s", want, summaryJSON)
		}
	}

	// Downgrading open BLOCKING finding to ADVISORY is rejected.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase:       "review",
		Message:     "Attempt downgrade",
		TaskID:      "T1",
		ReviewRound: 2,
		Findings:    []Finding{{ID: "CR-01", Tier: "ADVISORY", Disposition: "addressed"}},
	}, testSource); err == nil {
		t.Fatal("allowed downgrading open BLOCKING finding to ADVISORY")
	}

	// Closing via BLOCKING + closed removes CR-01 cleanly.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase:       "review",
		Message:     "Close CR-01",
		TaskID:      "T1",
		ReviewRound: 2,
		Findings:    []Finding{{ID: "CR-01", Tier: "BLOCKING", BlockingClass: 1, Disposition: "closed"}},
	}, testSource); err != nil {
		t.Fatal(err)
	}
	view, err = store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(view.Budgets["T1"].OpenBlocking, []string{"CR-02"}) {
		t.Fatalf("expected only CR-02 open, got %v", view.Budgets["T1"].OpenBlocking)
	}
}

func TestInvalidTelemetryRejectedWithoutMutatingHistory(t *testing.T) {
	store, manifest := testStore(t)

	badOpts := []struct {
		name string
		opts NoteOptions
	}{
		{"unknown task", NoteOptions{Phase: "implementation", Message: "m", TaskID: "NOPE"}},
		{"invalid role", NoteOptions{Phase: "implementation", Message: "m", TaskID: "T1", Role: "admin"}},
		{"negative review round", NoteOptions{Phase: "review", Message: "m", ReviewRound: -1}},
		{"negative fix iteration", NoteOptions{Phase: "implementation", Message: "m", TaskID: "T1", FixIteration: -1}},
		{"negative duration", NoteOptions{Phase: "implementation", Message: "m", DurationMS: -5}},
		{"review round on non-review phase", NoteOptions{Phase: "implementation", Message: "m", TaskID: "T1", ReviewRound: 1}},
		{"fix iteration without task", NoteOptions{Phase: "implementation", Message: "m", FixIteration: 1}},
		{"fix iteration on planning phase", NoteOptions{Phase: "planning", Message: "m", TaskID: "T1", FixIteration: 1}},
		{"failure signature without task", NoteOptions{Phase: "validation", Message: "m", FailureSignature: "pkg:TestX"}},
		{"failure signature too long", NoteOptions{Phase: "validation", Message: "m", TaskID: "T1", FailureSignature: strings.Repeat("a", 201)}},
		{"failure signature with newline", NoteOptions{Phase: "validation", Message: "m", TaskID: "T1", FailureSignature: "a\nb"}},
		{"failure signature leading space", NoteOptions{Phase: "validation", Message: "m", TaskID: "T1", FailureSignature: " sig"}},
		{"invalid finding id", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "../bad", Tier: "ADVISORY"}}}},
		{"duplicate finding id", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "A1", Tier: "ADVISORY"}, {ID: "A1", Tier: "ADVISORY"}}}},
		{"blocking class missing", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "B1", Tier: "BLOCKING", BlockingClass: 0}}}},
		{"blocking class out of range", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "B1", Tier: "BLOCKING", BlockingClass: 8}}}},
		{"advisory with blocking class", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "A1", Tier: "ADVISORY", BlockingClass: 1}}}},
		{"blocking with advisory disposition", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "B1", Tier: "BLOCKING", BlockingClass: 1, Disposition: "addressed"}}}},
		{"invalid severity", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "A1", Tier: "ADVISORY", Severity: "urgent"}}}},
		{"unsafe finding file", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "A1", Tier: "ADVISORY", File: "../secret.go"}}}},
		{"recurring key with space", NoteOptions{Phase: "review", Message: "m", Findings: []Finding{{ID: "A1", Tier: "ADVISORY", RecurringKey: "bad key"}}}},
	}
	for _, tc := range badOpts {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.NoteWithOptions(manifest.ID, tc.opts, testSource); err == nil {
				t.Fatalf("accepted invalid telemetry: %s", tc.name)
			}
		})
	}
	view, err := store.Read(manifest.ID)
	if err != nil || len(view.Events) != 1 {
		t.Fatalf("invalid telemetry mutated history: view=%+v err=%v", view, err)
	}
}

func TestR4ReviewAndFixLimitsAndOverrides(t *testing.T) {
	store, manifest := testStore(t)

	// Rounds 1, 2, 3 succeed (and repeated notes in same round succeed).
	for _, round := range []int{1, 1, 2, 3} {
		if err := store.NoteWithOptions(manifest.ID, NoteOptions{
			Phase:       "review",
			Message:     "Review note",
			ReviewRound: round,
		}, testSource); err != nil {
			t.Fatalf("round %d failed: %v", round, err)
		}
	}
	// Regressing review round is rejected.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "review", Message: "Regress", ReviewRound: 2,
	}, testSource); err == nil {
		t.Fatal("accepted non-monotonic review round")
	}
	// Round 4 is rejected until human-override extends run-level review rounds.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "review", Message: "Round 4", ReviewRound: 4,
	}, testSource); err == nil {
		t.Fatal("accepted 4th review round without human-override")
	}
	// Run-level override cannot set fix/failure fields.
	if err := store.HumanOverride(manifest.ID, "", "Bad run override", HumanOverride{ExtraFixIterations: 1}, nil, testSource); err == nil {
		t.Fatal("accepted run-level fix override without task_id")
	}
	if err := store.HumanOverride(manifest.ID, "", "Bad run reset", HumanOverride{ResetFailureSignatures: true}, nil, testSource); err == nil {
		t.Fatal("accepted run-level reset override without task_id")
	}
	// Overflow / empty overrides rejected.
	if err := store.HumanOverride(manifest.ID, "", "Empty", HumanOverride{}, nil, testSource); err == nil {
		t.Fatal("accepted empty override")
	}
	if err := store.HumanOverride(manifest.ID, "", "Huge", HumanOverride{ExtraReviewRounds: MaxOverrideDelta + 1}, nil, testSource); err == nil {
		t.Fatal("accepted override exceeding MaxOverrideDelta")
	}
	// Valid run-level override unlocks round 4 while keeping history.
	if err := store.HumanOverride(manifest.ID, "", "Owner authorized round 4", HumanOverride{ExtraReviewRounds: 1}, nil, testSource); err != nil {
		t.Fatal(err)
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "review", Message: "Round 4 after override", ReviewRound: 4,
	}, testSource); err != nil {
		t.Fatalf("round 4 after override failed: %v", err)
	}

	// Fix iterations 1..3 succeed; 4th rejected until task-level override.
	for _, iter := range []int{1, 2, 3} {
		if err := store.NoteWithOptions(manifest.ID, NoteOptions{
			Phase: "implementation", Message: "Fix attempt", TaskID: "T1", FixIteration: iter,
		}, testSource); err != nil {
			t.Fatalf("fix %d failed: %v", iter, err)
		}
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "implementation", Message: "Regress", TaskID: "T1", FixIteration: 2,
	}, testSource); err == nil {
		t.Fatal("accepted non-monotonic fix_iteration")
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "implementation", Message: "Fix 4", TaskID: "T1", FixIteration: 4,
	}, testSource); err == nil {
		t.Fatal("accepted 4th fix_iteration without override")
	}
	if err := store.HumanOverride(manifest.ID, "T1", "Owner authorized fix 4", HumanOverride{ExtraFixIterations: 1}, nil, testSource); err != nil {
		t.Fatal(err)
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "validation", Message: "Fix 4 verified", TaskID: "T1", FixIteration: 4,
	}, testSource); err != nil {
		t.Fatalf("fix 4 after override failed: %v", err)
	}
}

func TestR4RepeatedFailureCircuitBreakerAndTamperRead(t *testing.T) {
	store, manifest := testStore(t)
	evidence := addEvidence(t, store, manifest.ID)

	// Record 3 identical failures; 3rd failure is recorded and triggers FailureHalted.
	for i := 1; i <= 3; i++ {
		if err := store.NoteWithOptions(manifest.ID, NoteOptions{
			Phase:            "validation",
			Message:          "Same assertion failed",
			TaskID:           "T1",
			FailureSignature: "internal/test:TestFlake",
			Evidence:         []string{evidence},
		}, testSource); err != nil {
			t.Fatalf("failure %d should be recorded, got %v", i, err)
		}
	}
	view, err := store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Budgets["T1"].FailureHalted || view.Budgets["T1"].FailureCounts["internal/test:TestFlake"] != 3 {
		t.Fatalf("expected FailureHalted with count 3, got %+v", view.Budgets["T1"])
	}

	// While halted: implementation, fix_iteration, further failure_signature, and finish succeeded are rejected.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "implementation", Message: "Sneak another edit", TaskID: "T1",
	}, testSource); err == nil {
		t.Fatal("allowed implementation while task is FailureHalted")
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "validation", Message: "Another fix", TaskID: "T1", FixIteration: 1,
	}, testSource); err == nil {
		t.Fatal("allowed fix_iteration while task is FailureHalted")
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "validation", Message: "4th failure", TaskID: "T1", FailureSignature: "internal/test:TestOther",
	}, testSource); err == nil {
		t.Fatal("allowed further failure_signature while task is FailureHalted")
	}
	if err := store.Finish(manifest.ID, "succeeded", "Claim success while halted", []string{evidence}, testSource); err == nil {
		t.Fatal("allowed finish succeeded while task is FailureHalted")
	}

	// Escalation checkpoint (handoff) remains permitted while halted.
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "handoff", Message: "Escalating repeated failure to owner", TaskID: "T1",
	}, testSource); err != nil {
		t.Fatalf("handoff while halted should be allowed: %v", err)
	}

	// Human override with ResetFailureSignatures clears active window and resumes work.
	if err := store.HumanOverride(manifest.ID, "T1", "Owner approved fresh root-cause attempt", HumanOverride{ResetFailureSignatures: true}, nil, testSource); err != nil {
		t.Fatal(err)
	}
	view, err = store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Budgets["T1"].FailureHalted || len(view.Budgets["T1"].FailureCounts) != 0 {
		t.Fatalf("expected active failure window cleared, got %+v", view.Budgets["T1"])
	}
	if err := store.NoteWithOptions(manifest.ID, NoteOptions{
		Phase: "implementation", Message: "Resumed after owner override", TaskID: "T1", FixIteration: 1,
	}, testSource); err != nil {
		t.Fatalf("resumed implementation failed: %v", err)
	}
	if err := store.Finish(manifest.ID, "succeeded", "Completed after override", []string{evidence}, testSource); err != nil {
		t.Fatalf("finish succeeded after override failed: %v", err)
	}
}

func TestTamperedOnDiskEventsFailRead(t *testing.T) {
	t.Run("tampered started event with v2 field", func(t *testing.T) {
		store, manifest := testStore(t)
		dir := runDir(t, store, manifest.ID)
		view, err := store.Read(manifest.ID)
		if err != nil {
			t.Fatal(err)
		}
		started := view.Events[0]
		started.TaskID = "T1"
		raw, _ := json.MarshalIndent(started, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "events", "000001.json"), append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Read(manifest.ID); err == nil {
			t.Fatal("Read accepted tampered started event with TaskID")
		}
	})

	t.Run("tampered 4th review round on disk", func(t *testing.T) {
		store, manifest := testStore(t)
		if err := store.NoteWithOptions(manifest.ID, NoteOptions{
			Phase: "review", Message: "Round 1", ReviewRound: 1,
		}, testSource); err != nil {
			t.Fatal(err)
		}
		dir := runDir(t, store, manifest.ID)
		view, err := store.Read(manifest.ID)
		if err != nil {
			t.Fatal(err)
		}
		tampered := view.Events[1]
		tampered.ReviewRound = 4
		raw, _ := json.MarshalIndent(tampered, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "events", "000002.json"), append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Read(manifest.ID); err == nil {
			t.Fatal("Read accepted tampered on-disk 4th review round")
		}
	})

	t.Run("tampered finished event with v2 field", func(t *testing.T) {
		store, manifest := testStore(t)
		if err := store.Finish(manifest.ID, "needs-human", "Escalated", nil, testSource); err != nil {
			t.Fatal(err)
		}
		dir := runDir(t, store, manifest.ID)
		view, err := store.Read(manifest.ID)
		if err != nil {
			t.Fatal(err)
		}
		tampered := view.Events[1]
		tampered.DurationMS = 10
		raw, _ := json.MarshalIndent(tampered, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "events", "000002.json"), append(raw, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Read(manifest.ID); err == nil {
			t.Fatal("Read accepted tampered finished event with DurationMS")
		}
	})
}
