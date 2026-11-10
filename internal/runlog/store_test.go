package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"my-bilibili/internal/spec"
)

var testSource = Source{Commit: "unborn", Dirty: true}

func testStore(t *testing.T) (*Store, Manifest) {
	t.Helper()
	root := t.TempDir()
	def := spec.Definition{SchemaVersion: 1, SpecID: "test", Tasks: []spec.Task{
		{ID: "T1", Title: "Test task", WritePaths: []string{"internal/test"}, Acceptance: []string{"TEST-AC-01"}, Checks: []string{"go test ./..."}},
	}}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := spec.Snapshot{Path: "specs/test", Markdown: []byte("### TEST-AC-01\nObservable outcome.\n"), TaskJSON: data, Definition: def}
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Start(snapshot, testSource)
	if err != nil {
		t.Fatal(err)
	}
	return store, manifest
}

func addEvidence(t *testing.T, store *Store, id string) string {
	t.Helper()
	file, relative, err := store.CreateEvidence(id, "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("observable test output\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return relative
}

func runDir(t *testing.T, store *Store, id string) string {
	t.Helper()
	dir, err := store.directory(id)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLifecycleAndResume(t *testing.T) {
	store, manifest := testStore(t)
	evidence := addEvidence(t, store, manifest.ID)
	if err := store.Note(manifest.ID, "validation", "Checks passed; review pending", []string{evidence}, testSource); err != nil {
		t.Fatal(err)
	}
	resumed, err := New(filepath.Dir(filepath.Dir(store.base)))
	if err != nil {
		t.Fatal(err)
	}
	view, err := resumed.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" || len(view.Events) != 2 || view.Manifest != manifest {
		t.Fatalf("unexpected resumed view: %+v", view)
	}
	if err := resumed.Finish(manifest.ID, "needs-human", "Independent review required", nil, testSource); err != nil {
		t.Fatal(err)
	}
	view, err = store.Read(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "needs-human" || len(view.Events) != 3 {
		t.Fatalf("unexpected final view: %+v", view)
	}
	if err := store.Note(manifest.ID, "handoff", "not allowed", nil, testSource); err == nil {
		t.Fatal("append after finish succeeded")
	}
	if _, _, err := store.CreateEvidence(manifest.ID, "later-*"); err == nil {
		t.Fatal("created evidence in closed run")
	}
	list, err := store.List()
	if err != nil || len(list) != 1 || list[0].Events != 3 || list[0].Status != "needs-human" {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

func TestSuccessNeedsEvidence(t *testing.T) {
	store, manifest := testStore(t)
	if err := store.Finish(manifest.ID, "succeeded", "Unsupported claim", nil, testSource); err == nil {
		t.Fatal("succeeded without evidence")
	}
	evidence := addEvidence(t, store, manifest.ID)
	if err := store.Finish(manifest.ID, "succeeded", "Execution completed, not an approval", []string{evidence}, testSource); err != nil {
		t.Fatal(err)
	}
	view, err := store.Read(manifest.ID)
	if err != nil || view.Status != "succeeded" {
		t.Fatalf("Read = %+v, %v", view, err)
	}
}

func TestInvalidWritesDoNotPublish(t *testing.T) {
	store, manifest := testStore(t)
	for _, err := range []error{
		store.Note(manifest.ID, "approved", "Not a phase", nil, testSource),
		store.Note(manifest.ID, "planning", " ", nil, testSource),
		store.Note(manifest.ID, "planning", "Invalid source", nil, Source{}),
		store.Finish(manifest.ID, "approved", "Not an outcome", nil, testSource),
	} {
		if err == nil {
			t.Fatal("invalid write succeeded")
		}
	}
	view, err := store.Read(manifest.ID)
	if err != nil || len(view.Events) != 1 {
		t.Fatalf("invalid writes changed history: %+v, %v", view, err)
	}
}

func TestInputAndEvidenceIntegrity(t *testing.T) {
	for _, filename := range []string{"inputs/spec.md", "inputs/tasks.json", "evidence"} {
		t.Run(filename, func(t *testing.T) {
			store, manifest := testStore(t)
			target := filename
			if filename == "evidence" {
				target = addEvidence(t, store, manifest.ID)
				if err := store.Note(manifest.ID, "validation", "Original result", []string{target}, testSource); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(runDir(t, store, manifest.ID), filepath.FromSlash(target))
			if err := os.WriteFile(path, []byte("changed after publication\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Read(manifest.ID); err == nil || !strings.Contains(err.Error(), "mismatch") {
				t.Fatalf("did not reject changed file: %v", err)
			}
			if err := store.Note(manifest.ID, "handoff", "Cannot hide corruption", nil, testSource); err == nil {
				t.Fatal("appended to corrupt run")
			}
		})
	}
}

func TestInvalidEventHistory(t *testing.T) {
	tests := []struct {
		name string
		edit func(*testing.T, string)
	}{
		{"missing first", func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "events", "000001.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"gap", func(t *testing.T, dir string) {
			if err := os.Rename(filepath.Join(dir, "events", "000001.json"), filepath.Join(dir, "events", "000002.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"truncated", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "events", "000001.json"), []byte(`{"sequence":`), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unknown event file", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "events", "unexpected.json"), []byte(`{}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"trailing object", func(t *testing.T, dir string) {
			path := filepath.Join(dir, "events", "000001.json")
			file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("{}"); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, manifest := testStore(t)
			tt.edit(t, runDir(t, store, manifest.ID))
			if _, err := store.Read(manifest.ID); err == nil {
				t.Fatal("invalid published history accepted")
			}
		})
	}
}

func TestUnpublishedTempIsIgnored(t *testing.T) {
	store, manifest := testStore(t)
	path := filepath.Join(runDir(t, store, manifest.ID), "events", ".pending-interrupted")
	if err := os.WriteFile(path, []byte("partial write"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Note(manifest.ID, "implementation", "Resume after interruption", nil, testSource); err != nil {
		t.Fatal(err)
	}
	view, err := store.Read(manifest.ID)
	if err != nil || len(view.Events) != 2 {
		t.Fatalf("Read = %+v, %v", view, err)
	}
}

func TestStaleLockFailsClosed(t *testing.T) {
	store, manifest := testStore(t)
	lock := filepath.Join(runDir(t, store, manifest.ID), ".write-lock")
	if err := os.Mkdir(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Note(manifest.ID, "planning", "Cannot steal a lock", nil, testSource); err == nil || !strings.Contains(err.Error(), "writer lock") {
		t.Fatalf("expected lock failure, got %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("existing lock was removed")
	}
	if _, err := store.Read(manifest.ID); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentWriters(t *testing.T) {
	store, manifest := testStore(t)
	var successes atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := store.Note(manifest.ID, "implementation", "Concurrent checkpoint", nil, testSource)
			if err == nil {
				successes.Add(1)
			} else if !strings.Contains(err.Error(), "writer lock") {
				t.Errorf("unexpected writer error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	view, err := store.Read(manifest.ID)
	if err != nil || successes.Load() < 1 || len(view.Events) != int(successes.Load())+1 {
		t.Fatalf("successes=%d, view=%+v, err=%v", successes.Load(), view, err)
	}
}

func TestUnsafeEvidenceAndIDs(t *testing.T) {
	store, manifest := testStore(t)
	for _, path := range []string{"../outside", "/etc/passwd", "inputs/spec.md", "evidence/../manifest.json", `evidence\file`, "evidence/missing.txt", "evidence"} {
		if err := store.Note(manifest.ID, "validation", "Unsafe attachment", []string{path}, testSource); err == nil {
			t.Fatalf("accepted evidence path %q", path)
		}
	}
	for _, id := range []string{"", "..", "../outside", manifest.ID + "/..", "/tmp/run"} {
		if _, err := store.Read(id); err == nil {
			t.Fatalf("accepted run ID %q", id)
		}
	}
}

func TestEvidenceSymlinkRejected(t *testing.T) {
	store, manifest := testStore(t)
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("do not collect"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(runDir(t, store, manifest.ID), "evidence", "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := store.Note(manifest.ID, "validation", "Unsafe attachment", []string{"evidence/link.txt"}, testSource); err == nil {
		t.Fatal("followed an evidence symlink")
	}
}

func TestEmptyStoreList(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

func TestManifestMustMatchStartedEvent(t *testing.T) {
	store, manifest := testStore(t)
	manifest.Source.Commit = strings.Repeat("a", 40)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runDir(t, store, manifest.ID), "manifest.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(manifest.ID); err == nil || !strings.Contains(err.Error(), "manifest provenance") {
		t.Fatalf("accepted inconsistent manifest: %v", err)
	}
}

func TestRunStorageSymlinkRejected(t *testing.T) {
	store, manifest := testStore(t)
	root := t.TempDir()
	if err := os.Symlink(filepath.Dir(store.base), filepath.Join(root, ".agent")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	other, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Read(manifest.ID); err == nil {
		t.Fatal("followed symlinked run storage")
	}
	if _, err := other.List(); err == nil {
		t.Fatal("listed symlinked run storage")
	}
}
