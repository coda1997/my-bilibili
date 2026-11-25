package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVerifyScopeCLI(t *testing.T) {
	t.Run("unborn HEAD checks staged and untracked files", func(t *testing.T) {
		root := testRepository(t)
		// testRepository writes files outside "cmd", so unborn HEAD fails with violations.
		var out bytes.Buffer
		err := execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard)
		if err == nil {
			t.Fatal("accepted out-of-scope untracked files on unborn HEAD")
		}
		var result scopeResult
		if decErr := json.Unmarshal(out.Bytes(), &result); decErr != nil {
			t.Fatalf("expected structured JSON on violation, got %q: %v", out.String(), decErr)
		}
		if result.OK || result.Base != "unborn" || len(result.Violations) == 0 {
			t.Fatalf("unexpected unborn result: %+v", result)
		}
	})

	t.Run("clean commit, in-scope, out-of-scope, staged-only reverted, renamed, and base..head", func(t *testing.T) {
		root := testRepository(t)
		if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "cmd", "a.go"), []byte("package a\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitCommand(t, root, "add", ".")
		gitCommand(t, root, "commit", "--quiet", "-m", "Baseline")

		// Clean baseline passes.
		var out bytes.Buffer
		if err := execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard); err != nil {
			t.Fatalf("clean baseline failed: %v", err)
		}
		var result scopeResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || len(result.Violations) != 0 || len(result.CheckedPaths) != 0 || len(result.Base) < 40 {
			t.Fatalf("unexpected clean result: %+v", result)
		}

		// In-scope edit and new untracked file under cmd/ pass.
		if err := os.WriteFile(filepath.Join(root, "cmd", "b.go"), []byte("package b\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard); err != nil {
			t.Fatalf("in-scope change failed: %v", err)
		}

		// Staged-only out-of-scope file later removed from worktree must STILL be caught.
		outside := filepath.Join(root, "outside.txt")
		if err := os.WriteFile(outside, []byte("staged only\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitCommand(t, root, "add", "outside.txt")
		if err := os.Remove(outside); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		err := execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard)
		if err == nil {
			t.Fatal("missed staged-only out-of-scope file removed from working tree")
		}
		result = scopeResult{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.OK || !slices.Equal(result.Violations, []string{"outside.txt"}) {
			t.Fatalf("unexpected staged-only violation result: %+v", result)
		}
		gitCommand(t, root, "reset", "--quiet", "HEAD", "--", "outside.txt")

		// Moving an in-scope file out of scope (rename) must be caught via --no-renames.
		gitCommand(t, root, "mv", "cmd/a.go", "moved_out.go")
		out.Reset()
		err = execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard)
		if err == nil {
			t.Fatal("missed file renamed out of write_paths")
		}
		result = scopeResult{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(result.Violations, []string{"moved_out.go"}) {
			t.Fatalf("expected moved_out.go violation, got %+v", result)
		}

		// Moving an out-of-scope file INTO write_paths must also be caught via --no-renames
		// (default git diff -M drops the source path and would miss README.md).
		gitCommand(t, root, "reset", "--hard", "--quiet", "HEAD")
		gitCommand(t, root, "mv", "README.md", "cmd/moved_in.md")
		out.Reset()
		err = execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1",
		}, &out, io.Discard)
		if err == nil {
			t.Fatal("missed out-of-scope file renamed into write_paths (working tree)")
		}
		result = scopeResult{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(result.Violations, []string{"README.md"}) {
			t.Fatalf("expected README.md source violation on rename-in, got %+v", result)
		}
		// Verify committed --base..--head rename-in is also caught via --no-renames.
		gitCommand(t, root, "commit", "--quiet", "-m", "Rename out-of-scope file into cmd")
		out.Reset()
		err = execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1", "--base", "HEAD~1", "--head", "HEAD",
		}, &out, io.Discard)
		if err == nil {
			t.Fatal("missed out-of-scope file renamed into write_paths (committed range)")
		}
		result = scopeResult{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(result.Violations, []string{"README.md"}) {
			t.Fatalf("expected README.md violation on committed rename-in, got %+v", result)
		}
		gitCommand(t, root, "reset", "--hard", "--quiet", "HEAD~1")

		// Committed range --base .. --head ignores dirty working tree.
		gitCommand(t, root, "reset", "--hard", "--quiet", "HEAD")
		if err := os.WriteFile(filepath.Join(root, "cmd", "c.go"), []byte("package c\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitCommand(t, root, "add", "cmd/c.go")
		gitCommand(t, root, "commit", "--quiet", "-m", "In-scope commit")
		// Add unrelated untracked file that should NOT affect --base HEAD~1 --head HEAD.
		if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := execute(context.Background(), root, []string{
			"verify-scope", "--spec", "specs/test", "--task", "T1", "--base", "HEAD~1", "--head", "HEAD",
		}, &out, io.Discard); err != nil {
			t.Fatalf("committed in-scope range failed: %v", err)
		}
		result = scopeResult{}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Head == "" || !slices.Equal(result.CheckedPaths, []string{"cmd/c.go"}) {
			t.Fatalf("unexpected committed range result: %+v", result)
		}
	})

	t.Run("rejects flag-like revisions and unknown task before printing JSON", func(t *testing.T) {
		root := testRepository(t)
		for _, args := range [][]string{
			{"verify-scope", "--spec", "specs/test", "--task", "T1", "--base", "--cached"},
			{"verify-scope", "--spec", "specs/test", "--task", "T1", "--head", "--no-index"},
			{"verify-scope", "--spec", "specs/test", "--task", "UNKNOWN"},
			{"verify-scope", "--spec", "specs/test", "--task", "T1", "--base", "nonexistent-rev"},
		} {
			var out bytes.Buffer
			if err := execute(context.Background(), root, args, &out, io.Discard); err == nil {
				t.Fatalf("accepted invalid verify-scope args %v", args)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no JSON stdout on pre-evaluation error %v, got %q", args, out.String())
			}
		}
	})
}

func TestStructuredNoteAndOverrideCLI(t *testing.T) {
	root := testRepository(t)
	_, id := newTestRun(t, root)

	findingsPath := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(findingsPath, []byte(`[
		{"id":"CR-01","tier":"BLOCKING","blocking_class":1,"severity":"P1","file":"cmd/a.go","recurring_key":"class1:cmd","disposition":"open"}
	]`), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := execute(context.Background(), root, []string{
		"run", "note",
		"--id", id,
		"--phase", "review",
		"--message", "Round 1 code review",
		"--task", "T1",
		"--role", "code-review",
		"--review-round", "1",
		"--duration-ms", "250",
		"--findings-file", findingsPath,
	}, &out, io.Discard); err != nil {
		t.Fatalf("structured note failed: %v", err)
	}
	if !strings.Contains(out.String(), `"open_blocking"`) || !strings.Contains(out.String(), `"CR-01"`) {
		t.Fatalf("status summary missing open_blocking budget: %s", out.String())
	}

	// Unknown JSON field in findings file fails closed.
	badFindings := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badFindings, []byte(`[{"id":"CR-02","tier":"ADVISORY","extra":true}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execute(context.Background(), root, []string{
		"run", "note", "--id", id, "--phase", "review", "--message", "Bad findings", "--findings-file", badFindings,
	}, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted findings file with unknown JSON field")
	}

	// Hit 3 identical failures -> circuit breaker halts T1.
	for range 3 {
		if err := execute(context.Background(), root, []string{
			"run", "note", "--id", id, "--phase", "validation", "--message", "Failed test",
			"--task", "T1", "--failure-signature", "cmd:TestBroken",
		}, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if err := execute(context.Background(), root, []string{
		"run", "note", "--id", id, "--phase", "implementation", "--message", "Blocked edit", "--task", "T1",
	}, io.Discard, io.Discard); err == nil {
		t.Fatal("allowed implementation after 3 identical failures")
	}

	// Override without --confirm-human-override fails closed.
	if err := execute(context.Background(), root, []string{
		"run", "override", "--id", id, "--task", "T1", "--message", "Human reset", "--reset-failure-signatures",
	}, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted dev run override without --confirm-human-override")
	}

	// Override with --confirm-human-override unlocks T1.
	out.Reset()
	if err := execute(context.Background(), root, []string{
		"run", "override", "--id", id, "--task", "T1",
		"--message", "Owner approved fresh root-cause attempt",
		"--reset-failure-signatures", "--confirm-human-override",
	}, &out, io.Discard); err != nil {
		t.Fatalf("override failed: %v", err)
	}
	if !strings.Contains(out.String(), `"failure_halted": false`) {
		t.Fatalf("expected failure_halted false after override: %s", out.String())
	}
	if err := execute(context.Background(), root, []string{
		"run", "note", "--id", id, "--phase", "implementation", "--message", "Resumed fix", "--task", "T1", "--fix-iteration", "1",
	}, io.Discard, io.Discard); err != nil {
		t.Fatalf("resumed implementation failed: %v", err)
	}
}

func TestLoadFindingsFileGuards(t *testing.T) {
	root := testRepository(t)
	_, id := newTestRun(t, root)

	t.Run("rejects directory (non-regular file)", func(t *testing.T) {
		dir := t.TempDir()
		if err := execute(context.Background(), root, []string{
			"run", "note", "--id", id, "--phase", "review", "--message", "Dir findings", "--findings-file", dir,
		}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("want regular file rejection, got %v", err)
		}
	})

	t.Run("rejects oversized findings file (> 1 MiB)", func(t *testing.T) {
		big := filepath.Join(t.TempDir(), "oversized.json")
		payload := append([]byte("["), bytes.Repeat([]byte(" "), maxFindingsFileBytes)...)
		payload = append(payload, ']')
		if err := os.WriteFile(big, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := execute(context.Background(), root, []string{
			"run", "note", "--id", id, "--phase", "review", "--message", "Big findings", "--findings-file", big,
		}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("want oversized rejection, got %v", err)
		}
	})

	t.Run("rejects trailing JSON", func(t *testing.T) {
		trailing := filepath.Join(t.TempDir(), "trailing.json")
		if err := os.WriteFile(trailing, []byte(`[][]`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := execute(context.Background(), root, []string{
			"run", "note", "--id", id, "--phase", "review", "--message", "Trailing JSON", "--findings-file", trailing,
		}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "exactly one JSON array") {
			t.Fatalf("want trailing JSON rejection, got %v", err)
		}
	})

	t.Run("rejects JSON null", func(t *testing.T) {
		nullFile := filepath.Join(t.TempDir(), "null.json")
		if err := os.WriteFile(nullFile, []byte(`null`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := execute(context.Background(), root, []string{
			"run", "note", "--id", id, "--phase", "review", "--message", "Null findings", "--findings-file", nullFile,
		}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "JSON array") {
			t.Fatalf("want JSON array rejection for null, got %v", err)
		}
	})
}
