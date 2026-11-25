package runlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"my-bilibili/internal/spec"
)

const (
	DefaultMaxReviewRounds     = 3
	DefaultMaxFixIterations    = 3
	DefaultMaxRepeatedFailures = 3
	MaxOverrideDelta           = 100
	MaxScopeAllowance          = 1000
	RunScopeKey                = "_run"
)

type Finding struct {
	ID            string `json:"id"`
	Tier          string `json:"tier"`
	BlockingClass int    `json:"blocking_class,omitempty"`
	Severity      string `json:"severity,omitempty"`
	File          string `json:"file,omitempty"`
	RecurringKey  string `json:"recurring_key,omitempty"`
	Disposition   string `json:"disposition,omitempty"`
}

type HumanOverride struct {
	ExtraReviewRounds      int  `json:"extra_review_rounds"`
	ExtraFixIterations     int  `json:"extra_fix_iterations"`
	ResetFailureSignatures bool `json:"reset_failure_signatures"`
}

type BudgetStatus struct {
	ReviewRounds         int            `json:"review_rounds"`
	AllowedReviewRounds  int            `json:"allowed_review_rounds"`
	FixIterations        int            `json:"fix_iterations"`
	AllowedFixIterations int            `json:"allowed_fix_iterations"`
	FailureCounts        map[string]int `json:"failure_counts,omitempty"`
	FailureHalted        bool           `json:"failure_halted"`
	OpenBlocking         []string       `json:"open_blocking,omitempty"`
}

type NoteOptions struct {
	Phase            string
	Message          string
	Evidence         []string
	TaskID           string
	Role             string
	ReviewRound      int
	FixIteration     int
	FailureSignature string
	DurationMS       int64
	Findings         []Finding
}

var findingIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type scopeState struct {
	reviewRounds         int
	allowedReviewRounds  int
	fixIterations        int
	allowedFixIterations int
	failureCounts        map[string]int
	failureHalted        bool
	openBlocking         map[string]bool
}

type budgetTracker struct {
	schemaVersion int
	validTasks    map[string]bool
	scopes        map[string]*scopeState
}

func newBudgetTracker(schemaVersion int, validTasks map[string]bool) *budgetTracker {
	return &budgetTracker{
		schemaVersion: schemaVersion,
		validTasks:    validTasks,
		scopes:        make(map[string]*scopeState),
	}
}

func (t *budgetTracker) scopeFor(key string) *scopeState {
	if s, ok := t.scopes[key]; ok {
		return s
	}
	s := &scopeState{
		allowedReviewRounds:  DefaultMaxReviewRounds,
		allowedFixIterations: DefaultMaxFixIterations,
		failureCounts:        make(map[string]int),
		openBlocking:         make(map[string]bool),
	}
	t.scopes[key] = s
	return s
}

func (t *budgetTracker) anyFailureHalted() bool {
	for _, s := range t.scopes {
		if s.failureHalted {
			return true
		}
	}
	return false
}

func (t *budgetTracker) budgets() map[string]BudgetStatus {
	if len(t.scopes) == 0 {
		return nil
	}
	out := make(map[string]BudgetStatus, len(t.scopes))
	for key, s := range t.scopes {
		status := BudgetStatus{
			ReviewRounds:         s.reviewRounds,
			AllowedReviewRounds:  s.allowedReviewRounds,
			FixIterations:        s.fixIterations,
			AllowedFixIterations: s.allowedFixIterations,
			FailureHalted:        s.failureHalted,
		}
		if len(s.failureCounts) > 0 {
			counts := make(map[string]int, len(s.failureCounts))
			for sig, n := range s.failureCounts {
				counts[sig] = n
			}
			status.FailureCounts = counts
		}
		if len(s.openBlocking) > 0 {
			open := make([]string, 0, len(s.openBlocking))
			for id := range s.openBlocking {
				open = append(open, id)
			}
			sort.Strings(open)
			status.OpenBlocking = open
		}
		out[key] = status
	}
	return out
}

func hasV2Fields(event Event) bool {
	return event.TaskID != "" ||
		event.Role != "" ||
		event.ReviewRound != 0 ||
		event.FixIteration != 0 ||
		event.FailureSignature != "" ||
		event.DurationMS != 0 ||
		len(event.Findings) > 0 ||
		event.Override != nil
}

func (t *budgetTracker) apply(event Event) error {
	if event.ReviewRound < 0 || event.FixIteration < 0 || event.DurationMS < 0 {
		return errors.New("review_round, fix_iteration, and duration_ms must be non-negative")
	}
	if t.schemaVersion == 1 {
		if event.Kind == "human-override" || hasV2Fields(event) {
			return errors.New("schema version 1 runs do not allow v2 telemetry fields or human-override events")
		}
		return nil
	}

	switch event.Kind {
	case "started":
		if hasV2Fields(event) {
			return errors.New("started event cannot carry v2 telemetry or override fields")
		}
		return nil
	case "finished":
		if hasV2Fields(event) {
			return errors.New("finished event cannot carry v2 telemetry or override fields")
		}
		if event.Outcome == "succeeded" && t.anyFailureHalted() {
			return errors.New("cannot finish run as succeeded while a task is halted by R4 repeated failures; use needs-human, failed, or an explicit human-override")
		}
		return nil
	case "human-override":
		return t.applyOverride(event)
	case "checkpoint":
		return t.applyCheckpoint(event)
	default:
		return fmt.Errorf("invalid event kind %q", event.Kind)
	}
}

func (t *budgetTracker) applyOverride(event Event) error {
	if event.Phase != "" || event.Outcome != "" {
		return errors.New("human-override event cannot set phase or outcome")
	}
	if event.Role != "" || event.ReviewRound != 0 || event.FixIteration != 0 || event.FailureSignature != "" || event.DurationMS != 0 || len(event.Findings) > 0 {
		return errors.New("human-override event cannot carry checkpoint telemetry fields")
	}
	if event.Override == nil {
		return errors.New("human-override event requires override payload")
	}
	o := event.Override
	if o.ExtraReviewRounds < 0 || o.ExtraReviewRounds > MaxOverrideDelta {
		return fmt.Errorf("extra_review_rounds must be between 0 and %d", MaxOverrideDelta)
	}
	if o.ExtraFixIterations < 0 || o.ExtraFixIterations > MaxOverrideDelta {
		return fmt.Errorf("extra_fix_iterations must be between 0 and %d", MaxOverrideDelta)
	}
	if o.ExtraReviewRounds == 0 && o.ExtraFixIterations == 0 && !o.ResetFailureSignatures {
		return errors.New("human-override must grant extra_review_rounds, extra_fix_iterations, or reset_failure_signatures")
	}
	scopeKey := RunScopeKey
	if event.TaskID != "" {
		if !t.validTasks[event.TaskID] {
			return fmt.Errorf("unknown task_id %q not in frozen tasks.json", event.TaskID)
		}
		scopeKey = event.TaskID
	} else if o.ExtraFixIterations > 0 || o.ResetFailureSignatures {
		return errors.New("run-level human-override without task_id may only grant extra_review_rounds")
	}

	s := t.scopeFor(scopeKey)
	if s.allowedReviewRounds+o.ExtraReviewRounds > MaxScopeAllowance {
		return fmt.Errorf("allowed review rounds for %s would exceed maximum %d", scopeKey, MaxScopeAllowance)
	}
	if s.allowedFixIterations+o.ExtraFixIterations > MaxScopeAllowance {
		return fmt.Errorf("allowed fix iterations for %s would exceed maximum %d", scopeKey, MaxScopeAllowance)
	}
	s.allowedReviewRounds += o.ExtraReviewRounds
	s.allowedFixIterations += o.ExtraFixIterations
	if o.ResetFailureSignatures {
		s.failureCounts = make(map[string]int)
		s.failureHalted = false
	}
	return nil
}

func (t *budgetTracker) applyCheckpoint(event Event) error {
	if event.Override != nil {
		return errors.New("checkpoint event cannot carry override payload")
	}
	if !hasV2Fields(event) {
		return nil
	}
	if event.TaskID != "" && !t.validTasks[event.TaskID] {
		return fmt.Errorf("unknown task_id %q not in frozen tasks.json", event.TaskID)
	}
	if event.Role != "" && !validRole(event.Role) {
		return fmt.Errorf("invalid role %q", event.Role)
	}
	if event.ReviewRound > 0 && event.Phase != "review" {
		return errors.New("review_round is only valid on review checkpoints")
	}
	if event.FixIteration > 0 {
		if event.TaskID == "" {
			return errors.New("fix_iteration requires task_id")
		}
		if event.Phase != "implementation" && event.Phase != "validation" {
			return errors.New("fix_iteration is only valid on implementation or validation checkpoints")
		}
	}
	if event.FailureSignature != "" {
		if event.TaskID == "" {
			return errors.New("failure_signature requires task_id")
		}
		if err := validFailureSignature(event.FailureSignature); err != nil {
			return err
		}
	}
	if err := validateFindings(event.Findings); err != nil {
		return err
	}

	scopeKey := RunScopeKey
	if event.TaskID != "" {
		scopeKey = event.TaskID
	}
	s := t.scopeFor(scopeKey)

	if event.TaskID != "" && s.failureHalted {
		if event.Phase == "implementation" || event.FixIteration > 0 || event.FailureSignature != "" {
			return fmt.Errorf("R4 repeated-failure circuit breaker active for task %s; record needs-human, failed, or an explicit human-override with reset_failure_signatures", event.TaskID)
		}
	}
	if event.ReviewRound > 0 {
		if event.ReviewRound < s.reviewRounds {
			return fmt.Errorf("review_round %d cannot decrease below prior round %d for %s", event.ReviewRound, s.reviewRounds, scopeKey)
		}
		if event.ReviewRound > s.allowedReviewRounds {
			return fmt.Errorf("R4 review round limit (%d) exceeded for %s (got %d); record needs-human or an explicit human-override", s.allowedReviewRounds, scopeKey, event.ReviewRound)
		}
	}
	if event.FixIteration > 0 {
		if event.FixIteration < s.fixIterations {
			return fmt.Errorf("fix_iteration %d cannot decrease below prior iteration %d for %s", event.FixIteration, s.fixIterations, scopeKey)
		}
		if event.FixIteration > s.allowedFixIterations {
			return fmt.Errorf("R4 fix iteration limit (%d) exceeded for %s (got %d); record needs-human or an explicit human-override", s.allowedFixIterations, scopeKey, event.FixIteration)
		}
	}
	for _, f := range event.Findings {
		if f.Tier == "ADVISORY" && s.openBlocking[f.ID] {
			return fmt.Errorf("finding %q is open BLOCKING in %s and cannot be downgraded to ADVISORY", f.ID, scopeKey)
		}
	}

	// Commit validated mutations to scope state.
	if event.ReviewRound > 0 {
		s.reviewRounds = event.ReviewRound
	}
	if event.FixIteration > 0 {
		s.fixIterations = event.FixIteration
	}
	if event.FailureSignature != "" {
		s.failureCounts[event.FailureSignature]++
		if s.failureCounts[event.FailureSignature] >= DefaultMaxRepeatedFailures {
			s.failureHalted = true
		}
	}
	for _, f := range event.Findings {
		if f.Tier == "BLOCKING" {
			if f.Disposition == "closed" {
				delete(s.openBlocking, f.ID)
			} else {
				s.openBlocking[f.ID] = true
			}
		}
	}
	return nil
}

func validRole(role string) bool {
	switch role {
	case "lead", "architecture-review", "implementation", "code-review", "controller", "handoff":
		return true
	default:
		return false
	}
}

func validFailureSignature(sig string) error {
	if len(sig) == 0 || len(sig) > 200 {
		return errors.New("failure_signature must be 1..200 UTF-8 bytes")
	}
	if strings.TrimSpace(sig) != sig {
		return errors.New("failure_signature cannot have leading or trailing whitespace")
	}
	for _, r := range sig {
		if unicode.IsControl(r) {
			return errors.New("failure_signature cannot contain control characters")
		}
	}
	return nil
}

func validateFindings(findings []Finding) error {
	seen := make(map[string]bool, len(findings))
	for _, f := range findings {
		if len(f.ID) == 0 || len(f.ID) > 64 || !findingIDPattern.MatchString(f.ID) {
			return fmt.Errorf("invalid finding id %q", f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("duplicate finding id %q in the same event", f.ID)
		}
		seen[f.ID] = true
		switch f.Tier {
		case "BLOCKING":
			if f.BlockingClass < 1 || f.BlockingClass > 7 {
				return fmt.Errorf("BLOCKING finding %q requires blocking_class 1..7", f.ID)
			}
			switch f.Disposition {
			case "", "open", "closed":
			default:
				return fmt.Errorf("BLOCKING finding %q has invalid disposition %q (only open or closed permitted)", f.ID, f.Disposition)
			}
		case "ADVISORY":
			if f.BlockingClass != 0 {
				return fmt.Errorf("ADVISORY finding %q cannot set blocking_class", f.ID)
			}
			switch f.Disposition {
			case "", "open", "closed", "addressed", "deferred", "not-filed":
			default:
				return fmt.Errorf("ADVISORY finding %q has invalid disposition %q", f.ID, f.Disposition)
			}
		default:
			return fmt.Errorf("finding %q has invalid tier %q", f.ID, f.Tier)
		}
		if f.Severity != "" {
			switch f.Severity {
			case "P0", "P1", "P2", "P3", "critical", "high", "medium", "low":
			default:
				return fmt.Errorf("finding %q has invalid severity %q", f.ID, f.Severity)
			}
		}
		if f.File != "" && !spec.ValidPath(f.File) {
			return fmt.Errorf("finding %q has unsafe file path %q", f.ID, f.File)
		}
		if f.RecurringKey != "" {
			if len(f.RecurringKey) > 128 {
				return fmt.Errorf("finding %q recurring_key exceeds 128 bytes", f.ID)
			}
			for _, r := range f.RecurringKey {
				if unicode.IsSpace(r) || unicode.IsControl(r) {
					return fmt.Errorf("finding %q recurring_key cannot contain whitespace or control characters", f.ID)
				}
			}
		}
	}
	return nil
}

func decodeStrictTaskDefinition(data []byte) (spec.Definition, error) {
	var def spec.Definition
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&def); err != nil {
		return spec.Definition{}, fmt.Errorf("decode frozen tasks.json: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return spec.Definition{}, errors.New("frozen tasks.json must contain exactly one JSON object")
	}
	return def, nil
}

func definitionsEqual(a, b spec.Definition) bool {
	if a.SchemaVersion != b.SchemaVersion || a.SpecID != b.SpecID || len(a.Tasks) != len(b.Tasks) {
		return false
	}
	for i := range a.Tasks {
		ta, tb := a.Tasks[i], b.Tasks[i]
		if ta.ID != tb.ID || ta.Title != tb.Title ||
			!slices.Equal(ta.DependsOn, tb.DependsOn) ||
			!slices.Equal(ta.WritePaths, tb.WritePaths) ||
			!slices.Equal(ta.Acceptance, tb.Acceptance) ||
			!slices.Equal(ta.Checks, tb.Checks) {
			return false
		}
	}
	return true
}
