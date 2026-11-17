package localhistory

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
	"time"

	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func privateDir(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	return directory
}

func privateStore(t *testing.T, directory string) *Store {
	t.Helper()
	if !lockSupported {
		t.Skip("native lock unsupported on this test platform")
	}
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("store cleanup: %v", err)
		}
	})
	return store
}

func mustAcquire(t *testing.T, store *Store) {
	t.Helper()
	if err := store.Acquire(); err != nil {
		t.Fatal(err)
	}
}
func mustSave(t *testing.T, store *Store, records []history.Record) {
	t.Helper()
	if err := store.Save(records); err != nil {
		t.Fatal(err)
	}
}
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestConstructorOwnershipAndUnsupportedPlatform(t *testing.T) {
	if _, err := New("relative"); !errors.Is(err, history.Invalid) {
		t.Fatal(err)
	}
	directory := filepath.Join(privateDir(t), "absent")
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("constructor accessed files")
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("unacquired Load succeeded")
	}
	if err := store.Save([]history.Record{original("123")}); err == nil {
		t.Fatal("unacquired Save succeeded")
	}
	if !lockSupported {
		if err := store.Acquire(); !errors.Is(err, history.LockUnsupported) {
			t.Fatal(err)
		}
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("unsupported lock touched directory")
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Acquire(); !errors.Is(err, history.Closed) {
		t.Fatal("retired Store acquired", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultDirectoryResolvedOnlyOnAcquire(t *testing.T) {
	if !lockSupported {
		t.Skip("unsupported lock")
	}
	store, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	base := privateDir(t)
	called := 0
	store.ops.configDir = func() (string, error) { called++; return base, nil }
	if called != 0 {
		t.Fatal("config lookup in constructor")
	}
	mustAcquire(t, store)
	want, err := filepath.EvalSymlinks(filepath.Join(base, "my-bilibili"))
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 || store.canonical != want {
		t.Fatal("default path")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExclusiveOwnersAliasesStableLockAndFreshLoad(t *testing.T) {
	directory := privateDir(t)
	a := privateStore(t, directory)
	mustAcquire(t, a)
	lockPath := filepath.Join(directory, "history.lock")
	firstLock, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lockFile := a.lockFile
	mustAcquire(t, a)
	if a.lockFile != lockFile {
		t.Fatal("idempotent Acquire opened another descriptor")
	}
	if got, err := a.Load(); err != nil || len(got) != 0 {
		t.Fatal("missing file", err)
	}
	mustSave(t, a, []history.Record{original("123")})
	firstData, err := os.Stat(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(privateDir(t), "alias")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	b := privateStore(t, filepath.Join(alias, "."))
	if err := b.Acquire(); !errors.Is(err, history.InUse) || b.lockFile != nil {
		t.Fatalf("competing owner: %v", err)
	}
	if b.canonical != a.canonical {
		t.Fatal("directory aliases did not canonicalize")
	}
	helper(t, directory, "busy")
	for _, records := range [][]history.Record{{original("456")}, {}} {
		mustSave(t, a, records)
		before := readBytes(t, a.dataPath)
		if _, err := b.Load(); err == nil {
			t.Fatal("losing owner read history")
		}
		if err := b.Save([]history.Record{original("123")}); err == nil {
			t.Fatal("losing owner saved stale history")
		}
		if err := b.Save(nil); err == nil {
			t.Fatal("losing owner reset history")
		}
		if !bytes.Equal(before, readBytes(t, a.dataPath)) {
			t.Fatal("losing owner changed accepted deletion/clear")
		}
		currentLock, err := os.Stat(lockPath)
		if err != nil || !os.SameFile(firstLock, currentLock) {
			t.Fatal("lock inode replaced")
		}
	}
	lastData, err := os.Stat(a.dataPath)
	if err != nil || os.SameFile(firstData, lastData) {
		t.Fatal("data was not atomically replaced")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Load(); !errors.Is(err, history.Closed) {
		t.Fatal(err)
	}
	if err := a.Save([]history.Record{original("123")}); !errors.Is(err, history.Closed) {
		t.Fatal(err)
	}
	if err := a.Acquire(); !errors.Is(err, history.Closed) {
		t.Fatal(err)
	}
	helper(t, directory, "released")
	mustAcquire(t, b)
	if records, err := b.Load(); err != nil || len(records) != 0 {
		t.Fatalf("not the latest cleared file: %+v %v", records, err)
	}
	mustSave(t, b, []history.Record{original("789")})
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	finalLock, err := os.Stat(lockPath)
	if err != nil || !os.SameFile(firstLock, finalLock) {
		t.Fatal("close removed or replaced stable lock")
	}
}

func helper(t *testing.T, directory, expected string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestHistoryHelperProcess$")
	command.Env = append(os.Environ(), "MY_BILIBILI_HISTORY_TEST_DIR="+directory, "MY_BILIBILI_HISTORY_TEST_EXPECT="+expected)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("helper failed: %v: %s", err, output)
	}
}

func TestHistoryHelperProcess(t *testing.T) {
	directory := os.Getenv("MY_BILIBILI_HISTORY_TEST_DIR")
	if directory == "" {
		return
	}
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	err = store.Acquire()
	if os.Getenv("MY_BILIBILI_HISTORY_TEST_EXPECT") == "busy" {
		if !errors.Is(err, history.InUse) {
			t.Fatalf("competing process acquired: %v", err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatal("competing process read")
		}
		if err := store.Save([]history.Record{original("999")}); err == nil {
			t.Fatal("competing process wrote")
		}
		return
	}
	if err != nil {
		t.Fatal("released location still busy", err)
	}
	if records, err := store.Load(); err != nil || len(records) != 0 {
		t.Fatal("process did not read latest clear", err)
	}
}

func pumpHistory(t *testing.T, manager *history.Manager, predicate func(history.State) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		manager.Tick()
		if predicate(manager.State()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("manager did not settle: %+v", manager.State())
}

func TestIndependentManagersRetryLatestOwnedSnapshot(t *testing.T) {
	for _, clearAll := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "clear"}[clearAll], func(t *testing.T) {
			directory := privateDir(t)
			a := privateStore(t, directory)
			b := privateStore(t, directory)
			first, _ := history.New(a)
			second, _ := history.New(b)
			first.Start()
			pumpHistory(t, first, func(state history.State) bool { return state.Phase == history.Ready })
			first.Observe(history.Sample{Visit: 1, Record: original("123"), Playback: playback.Paused})
			pumpHistory(t, first, func(state history.State) bool { return !state.Busy && !state.Dirty })
			second.Start()
			pumpHistory(t, second, func(state history.State) bool { return state.Phase == history.Failed })
			if second.State().Error != history.InUse {
				t.Fatal("second manager was not excluded")
			}
			for _, err := range []error{second.Observe(history.Sample{Visit: 2, Record: original("999"), Playback: playback.Playing}), second.Delete(original("123").Ref), second.Clear(), second.ResetCorrupt()} {
				if !errors.Is(err, history.Invalid) {
					t.Fatal("losing manager mutated", err)
				}
			}
			if clearAll {
				first.Clear()
			} else {
				first.Delete(original("123").Ref)
			}
			pumpHistory(t, first, func(state history.State) bool { return !state.Busy && !state.Dirty })
			before := readBytes(t, a.dataPath)
			second.Tick()
			if !bytes.Equal(before, readBytes(t, a.dataPath)) {
				t.Fatal("rejected mutation escaped")
			}
			first.RequestClose(false)
			pumpHistory(t, first, func(state history.State) bool { return state.Phase == history.Done })
			if err := second.Retry(); err != nil {
				t.Fatal(err)
			}
			pumpHistory(t, second, func(state history.State) bool { return state.Phase == history.Ready })
			if len(second.State().Records) != 0 || second.State().Dirty {
				t.Fatal("retry restored an earlier snapshot")
			}
			second.RequestClose(false)
			pumpHistory(t, second, func(state history.State) bool { return state.Phase == history.Done })
		})
	}
}

func TestCorruptPreservationAndPrivateFiles(t *testing.T) {
	for _, contents := range [][]byte{[]byte("broken"), []byte(`{"version":2,"records":[]}`), bytes.Repeat([]byte("x"), historyFileBytes+1)} {
		t.Run(string(contents[:min(12, len(contents))]), func(t *testing.T) {
			directory := privateDir(t)
			store := privateStore(t, directory)
			mustAcquire(t, store)
			if err := os.WriteFile(store.dataPath, contents, 0600); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := store.Load(); err == nil {
					t.Fatal("bad file accepted")
				}
			}
			if !bytes.Equal(readBytes(t, store.dataPath), contents) {
				t.Fatal("failed load changed bytes")
			}
			competitor := privateStore(t, directory)
			if err := competitor.Acquire(); !errors.Is(err, history.InUse) {
				t.Fatal("corrupt load released lock", err)
			}
			// Explicit reset is a Save from the manager's confirmed recovery action.
			mustSave(t, store, nil)
			if records, err := store.Load(); err != nil || len(records) != 0 {
				t.Fatal("explicit reset", err)
			}
			for _, path := range []string{store.dataPath, filepath.Join(directory, "history.lock")} {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatal("nonprivate file", err)
				}
			}
			info, err := os.Stat(directory)
			if err != nil || info.Mode().Perm() != 0700 {
				t.Fatal("nonprivate directory", err)
			}
		})
	}
}

func TestSymlinksNonregularAndPermissions(t *testing.T) {
	if !lockSupported {
		t.Skip("unsupported lock")
	}
	for _, name := range []string{"history.lock", "history.json"} {
		for _, kind := range []string{"symlink", "directory", "public"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				directory := privateDir(t)
				target := filepath.Join(directory, name)
				outside := filepath.Join(privateDir(t), "untouched")
				os.WriteFile(outside, []byte("sentinel"), 0600)
				switch kind {
				case "symlink":
					if err := os.Symlink(outside, target); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(target, 0700); err != nil {
						t.Fatal(err)
					}
				case "public":
					if err := os.WriteFile(target, []byte(originalDocument), 0644); err != nil {
						t.Fatal(err)
					}
					os.Chmod(target, 0644)
				}
				store := privateStore(t, directory)
				if name == "history.lock" {
					if err := store.Acquire(); !errors.Is(err, history.Storage) {
						t.Fatal("unsafe lock accepted", err)
					}
				} else {
					mustAcquire(t, store)
					if _, err := store.Load(); !errors.Is(err, history.Storage) {
						t.Fatal("unsafe data read", err)
					}
					if err := store.Save(nil); !errors.Is(err, history.Storage) {
						t.Fatal("unsafe data replaced", err)
					}
				}
				if string(readBytes(t, outside)) != "sentinel" {
					t.Fatal("followed symlink target")
				}
			})
		}
	}
	directory := privateDir(t)
	os.Chmod(directory, 0755)
	store := privateStore(t, directory)
	if err := store.Acquire(); !errors.Is(err, history.Storage) {
		t.Fatal("public directory accepted", err)
	}
}

func TestAcquisitionReadAndBoundFailures(t *testing.T) {
	for _, stage := range []string{"open", "lock"} {
		t.Run(stage, func(t *testing.T) {
			store := privateStore(t, privateDir(t))
			originalOps := store.ops
			if stage == "open" {
				store.ops.open = func(string, int, os.FileMode) (*os.File, error) { return nil, errors.New("private sentinel") }
			} else {
				store.ops.lock = func(*os.File) error { return errors.New("private sentinel") }
			}
			if err := store.Acquire(); err != history.Storage || store.lockFile != nil || store.locked {
				t.Fatal("failed acquisition retained owner", err)
			}
			if _, err := store.Load(); err == nil {
				t.Fatal("Load after failed acquire")
			}
			store.ops = originalOps
			mustAcquire(t, store)
		})
	}
	store := privateStore(t, privateDir(t))
	mustAcquire(t, store)
	mustSave(t, store, []history.Record{original("123")})
	before := readBytes(t, store.dataPath)
	read := store.ops.read
	store.ops.read = func(io.Reader) ([]byte, error) { return nil, errors.New("private read sentinel") }
	if _, err := store.Load(); err != history.Storage {
		t.Fatal(err)
	}
	store.ops.read = func(io.Reader) ([]byte, error) { return make([]byte, historyFileBytes+1), nil }
	if _, err := store.Load(); err != history.Corrupt {
		t.Fatal("read grew past stat bound", err)
	}
	store.ops.read = read
	if !bytes.Equal(before, readBytes(t, store.dataPath)) {
		t.Fatal("failed read changed data")
	}
}

func TestAtomicSaveFailuresAndUncertainty(t *testing.T) {
	for _, stage := range []string{"create", "write", "short-write", "sync", "close", "rename", "directory-open", "directory-sync", "directory-close"} {
		t.Run(stage, func(t *testing.T) {
			store := privateStore(t, privateDir(t))
			mustAcquire(t, store)
			mustSave(t, store, []history.Record{original("123")})
			before := readBytes(t, store.dataPath)
			originalOps := store.ops
			sentinel := errors.New("private write sentinel")
			switch stage {
			case "create":
				store.ops.create = func(string, string) (*os.File, error) { return nil, sentinel }
			case "write":
				store.ops.write = func(*os.File, []byte) (int, error) { return 0, sentinel }
			case "short-write":
				store.ops.write = func(_ *os.File, data []byte) (int, error) { return len(data) - 1, nil }
			case "sync":
				store.ops.sync = func(*os.File) error { return sentinel }
			case "close":
				failed := false
				store.ops.close = func(file *os.File) (bool, error) {
					if !failed {
						failed = true
						return false, sentinel
					}
					return originalOps.close(file)
				}
			case "rename":
				store.ops.rename = func(string, string) error { return sentinel }
			case "directory-open":
				store.ops.open = func(path string, flag int, mode os.FileMode) (*os.File, error) {
					if path == store.canonical {
						return nil, sentinel
					}
					return originalOps.open(path, flag, mode)
				}
			case "directory-sync":
				store.ops.sync = func(file *os.File) error {
					if file.Name() == store.canonical {
						return sentinel
					}
					return originalOps.sync(file)
				}
			case "directory-close":
				store.ops.close = func(file *os.File) (bool, error) {
					if file.Name() == store.canonical {
						return false, sentinel
					}
					return originalOps.close(file)
				}
			}
			err := store.Save([]history.Record{original("456")})
			replaced := strings.HasPrefix(stage, "directory-")
			want := history.Storage
			if replaced {
				want = history.Uncertain
			}
			if err != want {
				t.Fatalf("save error = %v, want %v", err, want)
			}
			after := readBytes(t, store.dataPath)
			if replaced == bytes.Equal(before, after) {
				t.Fatal("wrong pre/post-rename result")
			}
			if pending, _ := filepath.Glob(filepath.Join(store.canonical, ".history-*")); len(pending) != 0 {
				t.Fatal("temporary leaked")
			}
			store.ops = originalOps
			mustSave(t, store, []history.Record{original("789")})
			if records, err := store.Load(); err != nil || len(records) != 1 || records[0].Ref.Part != "789" {
				t.Fatal("explicit retry failed", err)
			}
		})
	}
}

func TestCleanupFailuresRemainTerminalAndRetryOnlyUnfinished(t *testing.T) {
	t.Run("temporary", func(t *testing.T) {
		store := privateStore(t, privateDir(t))
		mustAcquire(t, store)
		originalOps := store.ops
		store.ops.write = func(*os.File, []byte) (int, error) { return 0, history.Storage }
		store.ops.remove = func(string) error { return history.Storage }
		if err := store.Save(nil); err != history.Storage || len(store.pending) != 1 {
			t.Fatal("cleanup error discarded", err)
		}
		if err := store.Close(); err != history.Storage || !store.locked || store.closed {
			t.Fatal("cleanup failure claimed release", err)
		}
		if err := store.Acquire(); err != history.Closed {
			t.Fatal(err)
		}
		if err := store.Save(nil); err != history.Closed {
			t.Fatal(err)
		}
		store.ops = originalOps
		if err := store.Close(); err != nil || len(store.pending) != 0 {
			t.Fatal(err)
		}
		if _, err := os.Stat(store.dataPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cleanup retry wrote data")
		}
	})
	for _, stage := range []string{"unlock", "close"} {
		t.Run(stage, func(t *testing.T) {
			directory := privateDir(t)
			store := privateStore(t, directory)
			mustAcquire(t, store)
			mustSave(t, store, []history.Record{original("123")})
			originalOps := store.ops
			unlocks, closes := 0, 0
			store.ops.unlock = func(file *os.File) error {
				unlocks++
				if stage == "unlock" && unlocks == 1 {
					return history.Storage
				}
				return originalOps.unlock(file)
			}
			store.ops.close = func(file *os.File) (bool, error) {
				closes++
				if stage == "close" && closes == 1 {
					return false, history.Storage
				}
				return originalOps.close(file)
			}
			if err := store.Close(); err != history.Storage || store.closed {
				t.Fatal("failed cleanup was successful", err)
			}
			if store.lockFile == nil {
				t.Fatal("unfinished descriptor forgotten")
			}
			if stage == "unlock" {
				competitor := privateStore(t, directory)
				if err := competitor.Acquire(); err != history.InUse {
					t.Fatal("failed unlock lost ownership", err)
				}
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if stage == "close" && unlocks != 1 || stage == "unlock" && closes != 1 {
				t.Fatal("cleanup repeated completed operation")
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRetiredDescriptorErrorNeverClaimsCleanClose(t *testing.T) {
	if !lockSupported {
		t.Skip("unsupported lock")
	}
	store, _ := New(privateDir(t))
	mustAcquire(t, store)
	closeFile := store.ops.close
	calls := 0
	store.ops.close = func(file *os.File) (bool, error) {
		calls++
		retired, err := closeFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return retired, history.Storage
	}
	if err := store.Close(); err != history.Storage || !store.uncertain || store.closed {
		t.Fatal("retired close error ignored", err)
	}
	if err := store.Close(); err != history.Storage || calls != 1 {
		t.Fatal("retried retired descriptor or invented clean completion", err)
	}
	if err := store.Acquire(); err != history.Closed {
		t.Fatal(err)
	}
	// The synthetic post-close error has actually closed the test descriptor.
	// The product correctly keeps uncertainty; the test does not force recovery.
}

func TestFailedAcquireCleanupBeforeRetry(t *testing.T) {
	directory := privateDir(t)
	owner := privateStore(t, directory)
	mustAcquire(t, owner)
	mustSave(t, owner, []history.Record{original("456")})
	loser := privateStore(t, directory)
	closeFile := loser.ops.close
	loser.ops.close = func(*os.File) (bool, error) { return false, history.Storage }
	if err := loser.Acquire(); err != history.Storage || loser.locked || loser.lockFile == nil {
		t.Fatal("failed acquisition cleanup was hidden", err)
	}
	if _, err := loser.Load(); err == nil {
		t.Fatal("unowned read during cleanup failure")
	}
	if err := loser.Save(nil); err == nil {
		t.Fatal("unowned reset during cleanup failure")
	}
	loser.ops.close = closeFile
	if err := loser.Acquire(); err != history.InUse || loser.lockFile != nil {
		t.Fatal("retry did not finish cleanup before competing again", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	mustAcquire(t, loser)
	if records, err := loser.Load(); err != nil || len(records) != 1 || records[0].Ref.Part != "456" {
		t.Fatal("retry did not load the owner's latest data", err)
	}
}

func TestFinalSaveHoldsLockThroughManagerCleanup(t *testing.T) {
	directory := privateDir(t)
	store := privateStore(t, directory)
	competitor := privateStore(t, directory)
	entered, release := make(chan struct{}), make(chan struct{})
	write := store.ops.write
	store.ops.write = func(file *os.File, data []byte) (int, error) {
		close(entered)
		<-release
		return write(file, data)
	}
	manager, _ := history.New(store)
	manager.Start()
	pumpHistory(t, manager, func(state history.State) bool { return state.Phase == history.Ready })
	manager.Observe(history.Sample{Visit: 1, Record: original("123"), Playback: playback.Paused})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("save missing")
	}
	manager.RequestClose(false)
	if err := competitor.Acquire(); err != history.InUse {
		t.Fatal("lock released during final write", err)
	}
	select {
	case <-manager.Done():
		t.Fatal("Done during final write")
	default:
	}
	close(release)
	pumpHistory(t, manager, func(state history.State) bool { return state.Phase == history.Done })
	mustAcquire(t, competitor)
	if records, err := competitor.Load(); err != nil || len(records) != 1 || records[0].Position != 1700*time.Millisecond {
		t.Fatal("release preceded final saved bytes", err)
	}
}
