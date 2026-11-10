package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"my-bilibili/internal/runlog"
)

func testRepository(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git is required for CLI tests")
	}
	root := t.TempDir()
	gitCommand(t, root, "init", "--quiet")
	files := map[string]string{
		".gitignore":            "/.agent/\n",
		"go.mod":                "module example\n\ngo 1.27.3\n",
		"specs/test/spec.md":    "# Test\n\n### TEST-AC-01\nOne observable outcome.\n",
		"specs/test/tasks.json": `{"schema_version":1,"spec_id":"test","tasks":[{"id":"T1","title":"Task","depends_on":[],"write_paths":["cmd"],"acceptance":["TEST-AC-01"],"checks":["go test ./..."]}]}`,
	}
	for path, text := range files {
		filename := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func gitCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	options := []string{"-c", "core.hooksPath=" + filepath.Join(root, ".git", "no-hooks"), "-c", "commit.gpgsign=false", "-c", "user.name=Foundation Test", "-c", "user.email=test@example.invalid"}
	cmd := exec.Command("git", append(options, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
}

func newTestRun(t *testing.T, root string) (*runlog.Store, string) {
	t.Helper()
	var output bytes.Buffer
	if err := execute(context.Background(), root, []string{"run", "new", "--spec", "specs/test", "--allow-dirty"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	store, err := runlog.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return store, strings.TrimSpace(output.String())
}

func TestCLIRecordsAndResumes(t *testing.T) {
	root := testRepository(t)
	if err := execute(context.Background(), root, []string{"run", "new", "--spec", "specs/test"}, io.Discard, io.Discard); err == nil {
		t.Fatal("dirty baseline was accepted without explicit opt-in")
	}
	store, id := newTestRun(t, root)
	commands := [][]string{
		{"run", "note", "--id", id, "--phase", "implementation", "--message", "Next: run checks"},
		{"run", "list"},
		{"run", "status", "--id", id, "--events"},
		{"run", "finish", "--id", id, "--outcome", "needs-human", "--message", "Independent review pending"},
		{"run", "status", "--id", id},
	}
	for _, args := range commands {
		if err := execute(context.Background(), root, args, io.Discard, io.Discard); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	view, err := store.Read(id)
	if err != nil || view.Status != "needs-human" || len(view.Events) != 3 {
		t.Fatalf("unexpected view: %+v, %v", view, err)
	}
}

func TestRecordedChecksIncludePassAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "pass"
		if fail {
			name = "fail"
		}
		t.Run(name, func(t *testing.T) {
			root := testRepository(t)
			store, id := newTestRun(t, root)
			runner := func(_ context.Context, _ string, out io.Writer, race bool) error {
				if !race {
					t.Error("race setting was lost")
				}
				if _, err := io.WriteString(out, "simulated command output\n"); err != nil {
					return err
				}
				if fail {
					return errors.New("simulated test failure")
				}
				return nil
			}
			err := recordedCheck(context.Background(), root, store, id, true, io.Discard, runner)
			if (err != nil) != fail {
				t.Fatalf("check error=%v, expected failure=%t", err, fail)
			}
			view, err := store.Read(id)
			if err != nil {
				t.Fatal(err)
			}
			if len(view.Events) != 2 || len(view.Events[1].Evidence) != 1 || view.Status != "running" {
				t.Fatalf("bad recorded check: %+v", view)
			}
			last := view.Events[1]
			if fail && !strings.Contains(last.Message, "Checks failed") || !fail && !strings.Contains(last.Message, "Checks passed") {
				t.Fatalf("incorrect check message: %s", last.Message)
			}
			path := filepath.Join(root, ".agent", "runs", id, filepath.FromSlash(last.Evidence[0].Path))
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte("simulated command output")) {
				t.Fatalf("missing command output: %s, %v", data, err)
			}
		})
	}
}

func TestSourceProvenance(t *testing.T) {
	root := testRepository(t)
	source, err := sourceInfo(root)
	if err != nil || source.Commit != "unborn" || !source.Dirty {
		t.Fatalf("unexpected unborn provenance: %+v, %v", source, err)
	}
	gitCommand(t, root, "add", ".")
	gitCommand(t, root, "commit", "--quiet", "-m", "Test baseline")
	source, err = sourceInfo(root)
	if err != nil || len(source.Commit) != 40 && len(source.Commit) != 64 || source.Dirty {
		t.Fatalf("unexpected clean provenance: %+v, %v", source, err)
	}
	store, id := newTestRun(t, root)
	err = recordedCheck(context.Background(), root, store, id, false, io.Discard, func(_ context.Context, _ string, _ io.Writer, _ bool) error {
		return os.WriteFile(filepath.Join(root, "changed.txt"), []byte("source changed"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "source provenance changed") {
		t.Fatalf("source change was not detected: %v", err)
	}
	view, err := store.Read(id)
	if err != nil || !view.Events[len(view.Events)-1].Source.Dirty {
		t.Fatalf("changed source was not recorded: %+v, %v", view, err)
	}
}

func TestUnknownArguments(t *testing.T) {
	root := testRepository(t)
	for _, args := range [][]string{
		{}, {"unknown"}, {"run"}, {"run", "unknown"}, {"run", "list", "extra"},
		{"run", "new", "--unknown"}, {"check", "--unknown"},
		{"check", "--run", "invalid"},
	} {
		if err := execute(context.Background(), root, args, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted arguments %v", args)
		}
	}
}
