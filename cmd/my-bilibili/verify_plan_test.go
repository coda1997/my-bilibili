package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

const verifyTestVideo = "BV0000000001"
const verifyTestPrivate = "PRIVATE_SYNTHETIC_VERIFICATION"

func verifyTestPlan(launch int) map[string]any {
	return map[string]any{"version": 1, "run_id": strings.Repeat("a", 32), "query": "synthetic query", "video_id": verifyTestVideo, "selected_result": 0, "part_page": 0, "quality_index": 0, "launch": launch}
}
func verifyTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal("synthetic encoding failed")
	}
	return data
}
func verifyTestSafe(t *testing.T, err error) {
	t.Helper()
	if err == nil || strings.Contains(err.Error(), verifyTestPrivate) {
		t.Fatal("expected safe failure missing or private text exposed")
	}
}

type verifyTestFiles struct{ directory, planPath, executable string }

func verifyTestPaths(t *testing.T) verifyTestFiles {
	t.Helper()
	root := t.TempDir()
	if os.Chmod(root, 0700) != nil {
		t.Fatal("private test root failed")
	}
	f := verifyTestFiles{directory: filepath.Join(root, "application"), planPath: filepath.Join(root, "plan.json"), executable: filepath.Join(root, "normal-app")}
	if os.Mkdir(f.directory, 0700) != nil || os.WriteFile(f.executable, []byte("original synthetic executable"), 0700) != nil {
		t.Fatal("synthetic paths failed")
	}
	f.writePlan(t, verifyTestPlan(1))
	return f
}
func (f verifyTestFiles) writePlan(t *testing.T, p map[string]any) {
	t.Helper()
	if os.WriteFile(f.planPath, verifyTestJSON(t, p), 0600) != nil {
		t.Fatal("synthetic plan write failed")
	}
}
func (f verifyTestFiles) prepare(t *testing.T) *verificationRun {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("successful on-disk verification requires POSIX private file modes")
	}
	r, err := prepareVerification(f.planPath, f.directory, f.executable)
	if err != nil {
		t.Fatal("synthetic verification preparation failed")
	}
	t.Cleanup(r.Stop)
	return r
}
func verifyTestRun(t *testing.T) (*verificationRun, verifyTestFiles) {
	t.Helper()
	f := verifyTestPaths(t)
	return f.prepare(t), f
}
func verifyTestContinuation(executableHash string) verificationContinuation {
	return verificationContinuation{Version: 1, ExecutableSHA256: executableHash, VideoID: verifyTestVideo, PartID: "101", PositionMS: 1200, HistorySHA256: strings.Repeat("b", 64), APIRequests: 8, BootstrapRequests: 1, CoverRequests: 2, MediaOpens: 1}
}

func TestVerificationPlanExactFieldsSelectorsAndBounds(t *testing.T) {
	p := verifyTestPlan(1)
	data := verifyTestJSON(t, p)
	got, err := parseVerificationPlan(data)
	if err != nil || got.VideoID != verifyTestVideo || got.Launch != 1 {
		t.Fatal("valid explicit plan rejected")
	}
	selected := verifyTestPlan(2)
	selected["video_id"], selected["selected_result"] = "", 1
	if _, err := parseVerificationPlan(verifyTestJSON(t, selected)); err != nil {
		t.Fatal("valid returned-result plan rejected")
	}
	for name := range p {
		for _, kind := range []string{"missing", "null", "case", "type", "duplicate"} {
			bad := verifyTestPlan(1)
			switch kind {
			case "missing":
				delete(bad, name)
			case "null":
				bad[name] = nil
			case "case":
				bad[strings.ToUpper(name)] = bad[name]
				delete(bad, name)
			case "type":
				bad[name] = map[string]any{"data": verifyTestPrivate}
			}
			raw := verifyTestJSON(t, bad)
			if kind == "duplicate" {
				raw = append(append([]byte(nil), raw[:len(raw)-1]...), []byte(",\""+name+"\":"+string(verifyTestJSON(t, bad[name]))+"}")...)
			}
			if value, err := parseVerificationPlan(raw); err == nil || value != (verificationPlan{}) {
				t.Fatal("inexact plan schema accepted")
			} else {
				verifyTestSafe(t, err)
			}
		}
	}
	for _, change := range []struct {
		name  string
		value any
	}{
		{"version", 0}, {"version", 1.5}, {"launch", 0}, {"launch", 3}, {"run_id", strings.Repeat("a", 31)}, {"run_id", strings.Repeat("A", 32)}, {"run_id", strings.Repeat("/", 32)},
		{"query", ""}, {"query", " padded "}, {"query", "bad\nquery"}, {"query", strings.Repeat("a", 129)},
		{"video_id", "av1"}, {"video_id", "https://www.bilibili.com/video/" + verifyTestVideo}, {"video_id", ""}, {"selected_result", 1}, {"selected_result", -1}, {"selected_result", 1001}, {"part_page", -1}, {"part_page", 1001}, {"quality_index", -1}, {"quality_index", 17}, {"endpoint", verifyTestPrivate}, {"cookie", verifyTestPrivate},
	} {
		bad := verifyTestPlan(1)
		bad[change.name] = change.value
		if _, err := parseVerificationPlan(verifyTestJSON(t, bad)); err == nil {
			t.Fatal("invalid plan selector/bound accepted")
		}
	}
	oversized := append(append([]byte(nil), data...), bytes.Repeat([]byte{' '}, 4097-len(data))...)
	for _, raw := range [][]byte{append([]byte{0xff}, data...), append(append([]byte(nil), data...), data...), bytes.Repeat([]byte{' '}, 4097), oversized} {
		if _, err := parseVerificationPlan(raw); err == nil {
			t.Fatal("raw plan bound/structure bypass")
		}
	}
	maximum := append(append([]byte(nil), data...), bytes.Repeat([]byte{' '}, 4096-len(data))...)
	if _, err := parseVerificationPlan(maximum); err != nil {
		t.Fatal("exact raw plan bound rejected")
	}
}

func TestVerificationOptionIsExplicitAbsoluteAndBuildIsolated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	if validateVerificationOption("", true, true) != nil || validateVerificationOption(path, false, false) != nil {
		t.Fatal("normal optional flag validation failed")
	}
	for _, value := range []struct {
		path         string
		fixture, e2e bool
	}{{path, true, false}, {path, false, true}, {"relative.json", false, false}, {path + "\n", false, false}, {"/tmp/../plan", false, false}, {"/" + strings.Repeat("a", 4096), false, false}} {
		if validateVerificationOption(value.path, value.fixture, value.e2e) != catalog.Invalid {
			t.Fatal("verification escaped explicit build/path boundary")
		}
	}
}

func TestVerificationSetupRefusesNonPrivateDirectoryBeforeAdmission(t *testing.T) {
	f := verifyTestPaths(t)
	if err := os.Chmod(f.directory, 0755); err != nil {
		t.Fatal(err)
	}
	if run, err := prepareVerification(f.planPath, f.directory, f.executable); run != nil || err != history.Storage {
		t.Fatalf("unprotected directory: run admitted=%v, error=%v", run != nil, err)
	}
	if _, err := os.Lstat(filepath.Join(f.directory, "verification")); !os.IsNotExist(err) {
		t.Fatal("unprotected directory received verification state")
	}
}

func TestVerificationSetupOwnsFreshRunWithoutReadingHistory(t *testing.T) {
	r, f := verifyTestRun(t)
	canonical, err := filepath.EvalSymlinks(f.directory)
	if err != nil || r.Plan().Launch != 1 || r.HistoryDirectory() != canonical || !verificationDirectory(r.runDirectory) || r.budget.snapshot().verificationCounts != (verificationCounts{}) {
		t.Fatal("fresh plan setup lost identity or budgets")
	}
	if _, err := os.Stat(filepath.Join(f.directory, "history.json")); !os.IsNotExist(err) {
		t.Fatal("setup accessed or created history before Acquire")
	}
	data, err := os.ReadFile(f.planPath)
	if err != nil {
		t.Fatal("test plan read failed")
	}
	sum := sha256.Sum256(data)
	if r.planHash != hex.EncodeToString(sum[:]) {
		t.Fatal("exact plan hash not retained")
	}
	if duplicate, err := prepareVerification(f.planPath, f.directory, f.executable); err == nil || duplicate != nil {
		t.Fatal("run ID reused after first admission")
	}
	if !strings.Contains(fmt.Sprintf("%+v", r), "private verification run") || strings.Contains(fmt.Sprintf("%#v", r.Plan()), "synthetic query") {
		t.Fatal("private setup fields formatted")
	}
	encoded, _ := json.Marshal(r)
	if string(encoded) != "null" {
		t.Fatal("private run serialized")
	}
}

func TestVerificationContinuationExactSchema(t *testing.T) {
	c := verifyTestContinuation(strings.Repeat("a", 64))
	raw := verifyTestJSON(t, c)
	if parsed, err := parseVerificationContinuation(raw); err != nil || parsed.ref() != c.ref() {
		t.Fatal("valid private continuation rejected")
	}
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("synthetic continuation map failed")
	}
	for key := range fields {
		var bad map[string]any
		_ = json.Unmarshal(raw, &bad)
		delete(bad, key)
		if _, err := parseVerificationContinuation(verifyTestJSON(t, bad)); err == nil {
			t.Fatal("missing continuation field accepted")
		}
		_ = json.Unmarshal(raw, &bad)
		bad[key] = nil
		if _, err := parseVerificationContinuation(verifyTestJSON(t, bad)); err == nil {
			t.Fatal("null continuation field accepted")
		}
	}
	for _, mutate := range []func(*verificationContinuation){func(c *verificationContinuation) { c.Version = 2 }, func(c *verificationContinuation) { c.ExecutableSHA256 = "bad" }, func(c *verificationContinuation) { c.HistorySHA256 = strings.Repeat("B", 64) }, func(c *verificationContinuation) { c.VideoID = "av1" }, func(c *verificationContinuation) { c.PartID = "0101" }, func(c *verificationContinuation) { c.PositionMS = 0 }, func(c *verificationContinuation) { c.PositionMS = int64((7*24*time.Hour)/time.Millisecond) + 1 }, func(c *verificationContinuation) { c.APIRequests = 25 }, func(c *verificationContinuation) { c.BootstrapRequests = 3 }, func(c *verificationContinuation) { c.CoverRequests = 5 }, func(c *verificationContinuation) { c.MediaOpens = 4 }} {
		bad := c
		mutate(&bad)
		if _, err := parseVerificationContinuation(verifyTestJSON(t, bad)); err == nil {
			t.Fatal("invalid private continuation admitted")
		}
	}
}

func TestVerificationSecondLaunchAdmission(t *testing.T) {
	r, f := verifyTestRun(t)
	c := verifyTestContinuation(r.executableHash)
	raw := verifyTestJSON(t, c)
	if verificationCreate(r.continuationPath, raw) != nil {
		t.Fatal("synthetic continuation write failed")
	}
	// Preparation must not read history; the wrapper performs that only after
	// the real Store owns its lock, even if current bytes are unreadable/invalid.
	if os.WriteFile(filepath.Join(f.directory, "history.json"), []byte(verifyTestPrivate), 0600) != nil {
		t.Fatal("synthetic sentinel history write failed")
	}
	p := verifyTestPlan(2)
	p["video_id"] = "BV0000000002"
	f.writePlan(t, p)
	second := f.prepare(t)
	if second.continuation.ref() != c.ref() || second.budget.snapshot().MediaOpens != 1 || second.Plan().VideoID == string(second.continuation.ref().Video) {
		t.Fatal("second plan target replaced private continuation or reset counts")
	}
	if duplicate, err := prepareVerification(f.planPath, f.directory, f.executable); err == nil || duplicate != nil {
		t.Fatal("concurrent/repeated second launch admitted")
	}
	stored, err := os.ReadFile(r.continuationPath)
	if err != nil || !bytes.Equal(stored, raw) {
		t.Fatal("second admission changed continuation")
	}
}

func TestVerificationSetupRejectsUnownedChangedAndOversizedFiles(t *testing.T) {
	for _, kind := range []string{"plan-symlink", "plan-large", "executable-symlink", "private-run-mode", "changed-executable", "continuation-symlink", "continuation-mode", "continuation-large", "missing-continuation"} {
		t.Run(kind, func(t *testing.T) {
			f := verifyTestPaths(t)
			if kind == "plan-symlink" || kind == "plan-large" || kind == "executable-symlink" {
				switch kind {
				case "plan-symlink":
					target := f.planPath + "-target"
					if os.Rename(f.planPath, target) != nil || os.Symlink(target, f.planPath) != nil {
						t.Fatal("test symlink failed")
					}
				case "plan-large":
					if os.WriteFile(f.planPath, bytes.Repeat([]byte{' '}, 4097), 0600) != nil {
						t.Fatal("test bound file failed")
					}
				case "executable-symlink":
					target := f.executable + "-target"
					if os.Rename(f.executable, target) != nil || os.Symlink(target, f.executable) != nil {
						t.Fatal("test executable alias failed")
					}
				}
			} else {
				r := f.prepare(t)
				raw := verifyTestJSON(t, verifyTestContinuation(r.executableHash))
				if kind != "missing-continuation" && verificationCreate(r.continuationPath, raw) != nil {
					t.Fatal("test continuation failed")
				}
				f.writePlan(t, verifyTestPlan(2))
				switch kind {
				case "private-run-mode":
					if os.Chmod(r.runDirectory, 0755) != nil {
						t.Fatal("test chmod failed")
					}
				case "changed-executable":
					if os.WriteFile(f.executable, []byte("different synthetic executable"), 0700) != nil {
						t.Fatal("test executable replacement failed")
					}
				case "continuation-symlink":
					target := r.continuationPath + "-target"
					if os.Rename(r.continuationPath, target) != nil || os.Symlink(target, r.continuationPath) != nil {
						t.Fatal("test continuation alias failed")
					}
				case "continuation-mode":
					if os.Chmod(r.continuationPath, 0644) != nil {
						t.Fatal("test chmod failed")
					}
				case "continuation-large":
					if os.WriteFile(r.continuationPath, bytes.Repeat([]byte{' '}, 4097), 0600) != nil {
						t.Fatal("test oversized continuation failed")
					}
				}
			}
			if run, err := prepareVerification(f.planPath, f.directory, f.executable); err != catalog.Invalid || run != nil {
				t.Fatalf("unowned/changed/oversized setup: run admitted=%v, error=%v", run != nil, err)
			} else {
				verifyTestSafe(t, err)
			}
		})
	}
}
