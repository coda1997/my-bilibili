package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type failOnWrite struct {
	writes int
	at     int
	err    error
}

func (w *failOnWrite) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.at {
		if w.err != nil {
			return 0, w.err
		}
		return len(p) - 1, nil
	}
	return len(p), nil
}

func TestRecordedCheckDisplayFailures(t *testing.T) {
	displayErr := errors.New("display unavailable")
	runnerErr := errors.New("simulated test failure")
	for _, stage := range []struct {
		name string
		at   int
	}{{"header", 1}, {"command", 2}, {"summary", 3}} {
		for _, short := range []bool{false, true} {
			for _, fail := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/short=%t/fail=%t", stage.name, short, fail), func(t *testing.T) {
					root := testRepository(t)
					store, id := newTestRun(t, root)
					display := &failOnWrite{at: stage.at, err: displayErr}
					wantErr := displayErr
					if short {
						display.err, wantErr = nil, io.ErrShortWrite
					}
					completed := false
					err := recordedCheck(context.Background(), root, store, id, true, display, func(_ context.Context, _ string, out io.Writer, race bool) error {
						if !race {
							t.Error("race setting was lost")
						}
						if _, err := io.WriteString(out, "simulated command output\n"); err != nil {
							return err
						}
						completed = true
						if fail {
							return runnerErr
						}
						return nil
					})
					if !completed || !errors.Is(err, wantErr) || errors.Is(err, runnerErr) != fail {
						t.Fatalf("completed=%t, error=%v; want display error %v and runner failure=%t", completed, err, wantErr, fail)
					}
					view, err := store.Read(id)
					if err != nil {
						t.Fatal(err)
					}
					if len(view.Events) != 2 || len(view.Events[1].Evidence) != 1 || view.Status != "running" {
						t.Fatalf("completed check lost its outcome/evidence: %+v", view)
					}
					last := view.Events[1]
					wantMessage := "Checks passed; independent review and acceptance are still required"
					if fail {
						wantMessage = "Checks failed: " + runnerErr.Error()
					}
					if last.Message != wantMessage {
						t.Fatalf("message=%q, want %q", last.Message, wantMessage)
					}
					path := filepath.Join(root, ".agent", "runs", id, filepath.FromSlash(last.Evidence[0].Path))
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					log := string(data)
					if !strings.HasPrefix(log, "Go: ") || !strings.Contains(log, "Source: unborn; dirty: true\n") || !strings.HasSuffix(log, "simulated command output\n"+wantMessage+"\n") {
						t.Fatalf("incomplete durable check log: %q", log)
					}
				})
			}
		}
	}
}

func TestRecordedCheckBrokenPipe(t *testing.T) {
	if id := os.Getenv("MY_BILIBILI_TEST_BROKEN_PIPE"); id != "" {
		os.Args = []string{"dev", "check", "--run", id}
		main()
		os.Exit(0)
	}
	for _, outcome := range []string{"pass", "fail"} {
		t.Run(outcome, func(t *testing.T) {
			root := testRepository(t)
			if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte("package fixture\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if outcome == "fail" {
				source := "package fixture\n\nimport \"testing\"\n\nfunc TestFixture(t *testing.T) {\n\tt.Fatal(\"controlled fixture failure\")\n}\n"
				if err := os.WriteFile(filepath.Join(root, "fixture_test.go"), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			store, id := newTestRun(t, root)
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer write.Close()
			if err := read.Close(); err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			// Use the real entrypoint and fd 1, not a mock writer that bypasses SIGPIPE.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestRecordedCheckBrokenPipe$")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "MY_BILIBILI_TEST_BROKEN_PIPE="+id)
			var stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = write, &stderr
			err = cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(stderr.String(), "display check output") {
				t.Fatalf("expected reported display error, got %v; stderr=%q; context=%v", err, stderr.String(), ctx.Err())
			}
			view, err := store.Read(id)
			if err != nil || len(view.Events) != 2 || len(view.Events[1].Evidence) != 1 {
				t.Fatalf("broken pipe lost durable result: %+v, %v", view, err)
			}
			last := view.Events[1]
			want := "Checks passed"
			if outcome == "fail" {
				want = "Checks failed: go test -count=1 ./..."
			}
			if !strings.HasPrefix(last.Message, want) {
				t.Fatalf("unexpected result: %s", last.Message)
			}
			log, err := os.ReadFile(filepath.Join(root, ".agent", "runs", id, filepath.FromSlash(last.Evidence[0].Path)))
			if err != nil || !bytes.HasSuffix(log, []byte(last.Message+"\n")) {
				t.Fatalf("incomplete recorded result: %q, %v", log, err)
			}
			if outcome == "fail" && !bytes.Contains(log, []byte("controlled fixture failure")) {
				t.Fatalf("missing actual failure output: %q", log)
			}
		})
	}
}

func TestCheckOutputPersistenceFailures(t *testing.T) {
	writeErr := errors.New("evidence unavailable")
	for at := 1; at <= 3; at++ {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("write=%d/short=%t", at, short), func(t *testing.T) {
				log := &failOnWrite{at: at, err: writeErr}
				wantErr := writeErr
				if short {
					log.err, wantErr = nil, io.ErrShortWrite
				}
				var display bytes.Buffer
				out := &checkOutput{log: log, display: &display}
				for i := 1; i < at; i++ {
					if n, err := out.Write([]byte("output\n")); n != 7 || err != nil {
						t.Fatalf("healthy write: n=%d, error=%v", n, err)
					}
				}
				if _, err := out.Write([]byte("failure\n")); !errors.Is(err, wantErr) {
					t.Fatalf("lost evidence error: %v", err)
				}
				if n, err := out.Write([]byte("later output\n")); n != 0 || !errors.Is(err, wantErr) {
					t.Fatalf("evidence error is not sticky: n=%d, error=%v", n, err)
				}
				if log.writes != at || display.String() != strings.Repeat("output\n", at-1) {
					t.Fatalf("continued after evidence failure: writes=%d, display=%q", log.writes, display.String())
				}
			})
		}
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestRecordedCheckPersistenceFailures(t *testing.T) {
	writeErr := errors.New("evidence unavailable")
	runnerErr := errors.New("simulated test failure")
	for _, kind := range []string{"write", "short write", "closed during command", "closed at finalization"} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fail=%t", kind, fail), func(t *testing.T) {
				root := testRepository(t)
				store, id := newTestRun(t, root)
				wantErr := writeErr
				var log *os.File
				display := writerFunc(func(p []byte) (int, error) {
					if kind == "closed at finalization" && bytes.HasPrefix(p, []byte("Checks ")) {
						if err := log.Close(); err != nil {
							t.Fatal(err)
						}
					}
					return len(p), nil
				})
				completed := false
				err := recordedCheck(context.Background(), root, store, id, false, display, func(_ context.Context, _ string, out io.Writer, _ bool) error {
					w := out.(*checkOutput)
					log = w.log.(*os.File)
					switch kind {
					case "write":
						w.log = &failOnWrite{at: 1, err: writeErr}
					case "short write":
						w.log, wantErr = &failOnWrite{at: 1}, io.ErrShortWrite
					case "closed during command":
						if err := log.Close(); err != nil {
							return err
						}
						wantErr = os.ErrClosed
					case "closed at finalization":
						wantErr = os.ErrClosed
					}
					_, ignoredErr := io.WriteString(out, "simulated command output\n")
					if (ignoredErr == nil) != (kind == "closed at finalization") {
						t.Errorf("unexpected injected write result: %v", ignoredErr)
					}
					completed = true
					if fail {
						return runnerErr
					}
					return nil
				})
				if !completed || !errors.Is(err, wantErr) || errors.Is(err, runnerErr) != fail {
					t.Fatalf("completed=%t, error=%v; want evidence error %v and runner failure=%t", completed, err, wantErr, fail)
				}
				view, err := store.Read(id)
				if err != nil || len(view.Events) != 1 || view.Status != "running" {
					t.Fatalf("published a result without durable evidence: %+v, %v", view, err)
				}
			})
		}
	}
}

func TestUnicodeEvidenceRoundTrip(t *testing.T) {
	for _, name := range []string{"caf\u00e9.txt", "cafe\u0301.txt", "\u8bb0\u5f55.txt"} {
		t.Run(name, func(t *testing.T) {
			root := testRepository(t)
			store, id := newTestRun(t, root)
			path := "evidence/" + name
			if err := os.WriteFile(filepath.Join(root, ".agent", "runs", id, filepath.FromSlash(path)), []byte("Unicode evidence filename\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			source, err := sourceInfo(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Note(id, "validation", "Unicode evidence recorded", []string{path}, source); err != nil {
				t.Fatal(err)
			}
			if err := store.Finish(id, "needs-human", "Independent review pending", nil, source); err != nil {
				t.Fatal(err)
			}
			reopened, err := runlog.New(root)
			if err != nil {
				t.Fatal(err)
			}
			view, err := reopened.Read(id)
			if err != nil || view.Status != "needs-human" || len(view.Events) != 3 {
				t.Fatalf("Unicode evidence broke closed-run readability: %+v, %v", view, err)
			}
			if got := view.Events[1].Evidence; len(got) != 1 || got[0].Path != path {
				t.Fatalf("Unicode evidence path changed: %+v", got)
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
