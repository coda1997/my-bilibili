package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

const verificationPlanLimit = 4096
const verificationHistoryLimit = 1 << 20
const verificationExecutableLimit = 512 << 20
const verificationProcessLimit = 240 * time.Second

type verificationPlan struct {
	Version        int    `json:"version"`
	RunID          string `json:"run_id"`
	Query          string `json:"query"`
	VideoID        string `json:"video_id"`
	SelectedResult int    `json:"selected_result"`
	PartPage       int    `json:"part_page"`
	QualityIndex   int    `json:"quality_index"`
	Launch         int    `json:"launch"`
}

func (verificationPlan) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification plan]")
}

// Both private file schemas are flat and fixed. This decoder rejects missing,
// duplicate, unknown, null and case-aliased fields before typed conversion.
func decodeVerification(data []byte, target any) error {
	if len(data) == 0 || len(data) > verificationPlanLimit || !utf8.Valid(data) {
		return catalog.Invalid
	}
	typ := reflect.TypeOf(target).Elem()
	fields := make(map[string]bool, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		fields[typ.Field(i).Tag.Get("json")] = true
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return catalog.Invalid
	}
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || !fields[key] {
			return catalog.Invalid
		}
		delete(fields, key)
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return catalog.Invalid
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || len(fields) != 0 {
		return catalog.Invalid
	}
	if _, err := d.Token(); err != io.EOF || json.Unmarshal(data, target) != nil {
		return catalog.Invalid
	}
	return nil
}

func verificationHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, c := range []byte(value) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func verificationBV(id string) bool {
	entry, err := catalog.ParseEntry(id)
	return err == nil && entry.Kind == catalog.BVEntry && entry.ID == id && entry.Page == 0
}

func verificationRef(ref catalog.PartRef) bool {
	if !verificationBV(string(ref.Video)) || len(ref.Part) == 0 || len(ref.Part) > 19 {
		return false
	}
	n, err := strconv.ParseInt(string(ref.Part), 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == string(ref.Part)
}

func parseVerificationPlan(data []byte) (verificationPlan, error) {
	var p verificationPlan
	if decodeVerification(data, &p) != nil || p.Version != 1 || !verificationHex(p.RunID, 32) || p.Launch != 1 && p.Launch != 2 || p.PartPage < 0 || p.PartPage > 1000 || p.QualityIndex < 0 || p.QualityIndex > 16 || p.SelectedResult < 0 || p.SelectedResult > 1000 {
		return verificationPlan{}, catalog.Invalid
	}
	query, err := catalog.ValidateQuery(p.Query)
	if err != nil || query != p.Query || (p.VideoID == "") == (p.SelectedResult == 0) || p.VideoID != "" && !verificationBV(p.VideoID) {
		return verificationPlan{}, catalog.Invalid
	}
	return p, nil
}

func verificationPath(path string) bool {
	return path != "" && len(path) <= 4096 && filepath.IsAbs(path) && filepath.Clean(path) == path && utf8.ValidString(path) && !strings.ContainsFunc(path, unicode.IsControl)
}

func validateVerificationOption(planPath string, fixture, e2e bool) error {
	if planPath == "" {
		return nil
	}
	if fixture || e2e || !verificationPath(planPath) {
		return catalog.Invalid
	}
	return nil
}

type verificationContinuation struct {
	Version           int    `json:"version"`
	ExecutableSHA256  string `json:"executable_sha256"`
	VideoID           string `json:"video_id"`
	PartID            string `json:"part_id"`
	PositionMS        int64  `json:"position_ms"`
	HistorySHA256     string `json:"history_sha256"`
	APIRequests       int    `json:"api_requests"`
	BootstrapRequests int    `json:"bootstrap_requests"`
	CoverRequests     int    `json:"cover_requests"`
	MediaOpens        int    `json:"media_opens"`
}

func (c verificationContinuation) ref() catalog.PartRef {
	return catalog.PartRef{Video: catalog.VideoID(c.VideoID), Part: catalog.PartID(c.PartID)}
}
func (verificationContinuation) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification continuation]")
}

func parseVerificationContinuation(data []byte) (verificationContinuation, error) {
	var c verificationContinuation
	if decodeVerification(data, &c) != nil || c.Version != 1 || !verificationHex(c.ExecutableSHA256, 64) || !verificationHex(c.HistorySHA256, 64) || !verificationRef(c.ref()) || c.PositionMS <= 0 || c.PositionMS > int64((7*24*time.Hour)/time.Millisecond) || c.APIRequests < 0 || c.APIRequests > 24 || c.BootstrapRequests < 0 || c.BootstrapRequests > 2 || c.CoverRequests < 0 || c.CoverRequests > 4 || c.MediaOpens < 1 || c.MediaOpens > 3 {
		return verificationContinuation{}, catalog.Invalid
	}
	return c, nil
}

// Setup owns all file access before normal native startup. UI Tick only uses
// the copied plan and memory-only wrappers. Finish receives ownership after
// Report and runs on the composition's final-join worker.
type verificationRun struct {
	plan                     verificationPlan
	directory, runDirectory  string
	continuationPath         string
	executableHash, planHash string
	continuation             *verificationContinuation
	continuationData         []byte
	continuationIdentity     os.FileInfo
	budget                   *verificationBudget
	history                  *verificationHistory
	started                  time.Time
	now                      func() time.Time
	writeContinuation        func(string, []byte) error
	removeContinuation       func(string) error
	finished                 bool
}

func (verificationRun) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, "[private verification run]")
}
func (verificationRun) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func prepareVerification(planPath, directory, executable string) (*verificationRun, error) {
	started := time.Now()
	if !verificationPath(planPath) {
		return nil, catalog.Invalid
	}
	data, err := verificationRead(planPath, verificationPlanLimit, false)
	if err != nil {
		return nil, catalog.Invalid
	}
	plan, err := parseVerificationPlan(data)
	if err != nil {
		return nil, err
	}
	if directory == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, history.Storage
		}
		directory = filepath.Join(base, "my-bilibili")
	}
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			return nil, catalog.Invalid
		}
	}
	if !verificationPath(directory) || !verificationPath(executable) {
		return nil, catalog.Invalid
	}
	executableHash, _, err := verificationDigest(executable, verificationExecutableLimit, false, false)
	if err != nil {
		return nil, catalog.Invalid
	}
	if os.MkdirAll(directory, 0700) != nil {
		return nil, history.Storage
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil || !verificationDirectory(directory) {
		return nil, history.Storage
	}
	parent := filepath.Join(directory, "verification")
	if err = os.Mkdir(parent, 0700); err != nil && !os.IsExist(err) || !verificationDirectory(parent) {
		return nil, history.Storage
	}
	r := &verificationRun{plan: plan, directory: directory, runDirectory: filepath.Join(parent, plan.RunID), executableHash: executableHash, started: started, now: time.Now, writeContinuation: verificationCreate, removeContinuation: verificationRemove}
	sum := sha256.Sum256(data)
	r.planHash = hex.EncodeToString(sum[:])
	r.continuationPath = filepath.Join(r.runDirectory, "continuation.json")
	counts := verificationCounts{}
	if plan.Launch == 1 {
		if os.Mkdir(r.runDirectory, 0700) != nil || !verificationDirectory(r.runDirectory) {
			return nil, catalog.Invalid
		}
	} else {
		if !verificationDirectory(r.runDirectory) {
			return nil, catalog.Invalid
		}
		data, err = verificationRead(r.continuationPath, verificationPlanLimit, true)
		if err != nil {
			return nil, catalog.Invalid
		}
		continuation, err := parseVerificationContinuation(data)
		if err != nil || continuation.ExecutableSHA256 != executableHash {
			return nil, catalog.Invalid
		}
		// This exclusive, empty marker admits exactly one second invocation even
		// when it later fails. It never contains private state or resets budgets.
		if verificationCreate(filepath.Join(r.runDirectory, "launch-2"), nil) != nil {
			return nil, catalog.Invalid
		}
		r.continuation = &continuation
		r.continuationData = append([]byte(nil), data...)
		r.continuationIdentity, err = os.Lstat(r.continuationPath)
		if err != nil {
			return nil, catalog.Invalid
		}
		counts = verificationCounts{APIRequests: continuation.APIRequests, BootstrapRequests: continuation.BootstrapRequests, CoverRequests: continuation.CoverRequests, MediaOpens: continuation.MediaOpens}
	}
	r.budget = newVerificationBudget(counts, plan.Launch, started.Add(verificationProcessLimit))
	return r, nil
}

func (r *verificationRun) Plan() verificationPlan   { return r.plan }
func (r *verificationRun) HistoryDirectory() string { return r.directory }
func (r *verificationRun) Stop()                    { r.budget.stop() }
