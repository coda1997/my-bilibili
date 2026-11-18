package history

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type callGate struct {
	entered chan struct{}
	release chan struct{}
}

func newGate() *callGate {
	return &callGate{entered: make(chan struct{}), release: make(chan struct{})}
}

type fakeStore struct {
	mu           sync.Mutex
	data         []Record
	calls        []string
	saved        [][]Record
	errors       map[string][]error
	gates        map[string][]*callGate
	held, closed bool
	active, max  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: []Record{}, errors: map[string][]error{}, gates: map[string][]*callGate{}}
}

func (s *fakeStore) call(operation string, records []Record) ([]Record, error) {
	s.mu.Lock()
	s.calls = append(s.calls, operation)
	s.active++
	s.max = max(s.max, s.active)
	var gate *callGate
	if gates := s.gates[operation]; len(gates) > 0 {
		gate, s.gates[operation] = gates[0], gates[1:]
	}
	var err error
	if failures := s.errors[operation]; len(failures) > 0 {
		err, s.errors[operation] = failures[0], failures[1:]
	}
	s.mu.Unlock()
	if gate != nil {
		close(gate.entered)
		<-gate.release
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active--
	if operation != "close" && s.closed {
		return nil, Closed
	}
	if operation == "close" {
		s.closed = true
	}
	if err != nil {
		return nil, err
	}
	switch operation {
	case "acquire":
		s.held = true
	case "load":
		if !s.held {
			return nil, Storage
		}
		return slices.Clone(s.data), nil
	case "save":
		if !s.held {
			return nil, Storage
		}
		s.data = slices.Clone(records)
		s.saved = append(s.saved, slices.Clone(records))
	case "close":
		s.held = false
	}
	return nil, nil
}

func (s *fakeStore) Acquire() error              { _, err := s.call("acquire", nil); return err }
func (s *fakeStore) Load() ([]Record, error)     { return s.call("load", nil) }
func (s *fakeStore) Save(records []Record) error { _, err := s.call("save", records); return err }
func (s *fakeStore) Close() error                { _, err := s.call("close", nil); return err }
func (s *fakeStore) count(operation string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, call := range s.calls {
		if call == operation {
			n++
		}
	}
	return n
}
func (s *fakeStore) snapshot() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data)
}

func pump(t *testing.T, manager *Manager, predicate func(State) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		manager.Tick()
		if predicate(manager.State()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("history did not settle: %+v", manager.State())
}

func readyManager(t *testing.T, store *fakeStore) *Manager {
	t.Helper()
	manager, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Start(); err != nil {
		t.Fatal(err)
	}
	pump(t, manager, func(state State) bool { return state.Phase == Ready && !state.Busy })
	t.Cleanup(func() {
		if manager.State().Phase == Done {
			return
		}
		manager.RequestClose(true)
		pump(t, manager, func(state State) bool { return state.Phase == Done })
	})
	return manager
}

func idle(t *testing.T, manager *Manager) {
	t.Helper()
	pump(t, manager, func(state State) bool { return !state.Busy })
}

func waitGate(t *testing.T, gate *callGate) {
	t.Helper()
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("store call did not start")
	}
}

func sample(visit uint64, position time.Duration, state playback.State) Sample {
	return Sample{Visit: visit, Record: Record{
		Ref:   catalog.PartRef{Video: "BV0123456789", Part: "123"},
		Title: "Synthetic video", PartTitle: "Synthetic part",
		Position: position, Duration: 2 * time.Minute,
	}, Playback: state}
}

func observe(t *testing.T, manager *Manager, value Sample) {
	t.Helper()
	if err := manager.Observe(value); err != nil {
		t.Fatal(err)
	}
}

func TestConstructionAcquisitionAndFreshRetry(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, Invalid) {
		t.Fatal(err)
	}
	store := newFakeStore()
	gate := newGate()
	store.gates["acquire"] = []*callGate{gate}
	store.errors["acquire"] = []error{InUse}
	manager, _ := New(store)
	if len(store.calls) != 0 || manager.worker {
		t.Fatal("constructor started I/O")
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitGate(t, gate)
	for _, err := range []error{manager.Observe(sample(1, time.Second, playback.Playing)), manager.Delete(sample(1, 0, playback.Ready).Record.Ref), manager.Clear(), manager.ResetCorrupt()} {
		if !errors.Is(err, Invalid) {
			t.Fatalf("mutation before load = %v", err)
		}
	}
	if store.count("load") != 0 || store.count("save") != 0 {
		t.Fatal("I/O before ownership")
	}
	close(gate.release)
	pump(t, manager, func(state State) bool { return state.Phase == Failed })
	if state := manager.State(); state.Error != InUse || state.Dirty || len(state.Records) != 0 || store.count("acquire") != 1 {
		t.Fatalf("losing state %+v", state)
	}
	if err := manager.ResetCorrupt(); !errors.Is(err, Invalid) {
		t.Fatal("reset after failed acquisition", err)
	}
	fresh := sample(3, 21*time.Second, playback.Paused).Record
	store.mu.Lock()
	store.data = []Record{fresh}
	store.mu.Unlock()
	if err := manager.Retry(); err != nil {
		t.Fatal(err)
	}
	pump(t, manager, func(state State) bool { return state.Phase == Ready })
	if got := manager.State().Records; len(got) != 1 || got[0].Position != fresh.Position {
		t.Fatalf("not a fresh load: %+v", got)
	}
	if store.count("load") != 1 || store.count("save") != 0 {
		t.Fatal("stale pre-load mutation escaped")
	}
	manager.RequestClose(false)
	pump(t, manager, func(state State) bool { return state.Phase == Done })
}

func TestLoadFailureRequiresExplicitReset(t *testing.T) {
	for _, code := range []Code{Corrupt, UnsupportedVersion, Storage} {
		t.Run(string(code), func(t *testing.T) {
			store := newFakeStore()
			store.data = []Record{sample(1, time.Second, playback.Playing).Record}
			store.errors["load"] = []error{code, code}
			manager, _ := New(store)
			manager.Start()
			pump(t, manager, func(state State) bool { return state.Phase == Failed })
			if !manager.acquired || manager.loaded || len(manager.State().Records) != 0 {
				t.Fatal("partial load published")
			}
			if err := manager.Clear(); !errors.Is(err, Invalid) {
				t.Fatal(err)
			}
			if err := manager.Retry(); err != nil {
				t.Fatal(err)
			}
			pump(t, manager, func(state State) bool { return state.Phase == Failed })
			if store.count("save") != 0 || len(store.snapshot()) != 1 {
				t.Fatal("retry reset data")
			}
			if err := manager.ResetCorrupt(); err != nil {
				t.Fatal(err)
			}
			pump(t, manager, func(state State) bool { return state.Phase == Ready && !state.Busy })
			if state := manager.State(); state.Dirty || len(state.Records) != 0 || store.count("save") != 1 || len(store.snapshot()) != 0 {
				t.Fatalf("reset %+v", state)
			}
			manager.RequestClose(false)
			pump(t, manager, func(state State) bool { return state.Phase == Done })
		})
	}
}

func TestUsefulSamplesAndFiveSecondCheckpoint(t *testing.T) {
	store := newFakeStore()
	manager := readyManager(t, store)
	clock := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return clock }
	loading := sample(1, 0, playback.Loading)
	loading.Record.Duration = 0
	observe(t, manager, loading)
	observe(t, manager, sample(1, 0, playback.Ready))
	if store.count("save") != 0 || len(manager.State().Records) != 0 {
		t.Fatal("detail-only history")
	}
	observe(t, manager, sample(1, 0, playback.Playing))
	idle(t, manager)
	if store.count("save") != 1 {
		t.Fatal("playing visit not checkpointed")
	}
	clock = clock.Add(time.Second)
	observe(t, manager, sample(1, time.Second, playback.Playing))
	clock = clock.Add(4*time.Second - time.Nanosecond)
	observe(t, manager, sample(1, 4*time.Second, playback.Playing))
	if store.count("save") != 1 || !manager.State().Dirty {
		t.Fatal("checkpoint was not coalesced")
	}
	clock = clock.Add(time.Nanosecond)
	manager.Tick()
	idle(t, manager)
	if got := store.snapshot()[0].Position; got != 4*time.Second || store.count("save") != 2 {
		t.Fatalf("checkpoint position %v", got)
	}
	clock = clock.Add(time.Minute)
	for range 10 {
		manager.Tick()
		observe(t, manager, sample(1, 4*time.Second, playback.Playing))
	}
	if store.count("save") != 2 {
		t.Fatal("unchanged ticks wrote")
	}
	observe(t, manager, sample(1, 37*time.Second, playback.Paused))
	idle(t, manager)
	if got := store.snapshot()[0].Position; got != 37*time.Second {
		t.Fatalf("paused seek %v", got)
	}
	observe(t, manager, sample(1, 0, playback.Ready))
	if got := manager.State().Records[0].Position; got != 37*time.Second {
		t.Fatalf("unplayed replacement readiness erased confirmed progress: %v", got)
	}
	observe(t, manager, sample(1, 0, playback.Paused))
	idle(t, manager)
	if got := store.snapshot()[0].Position; got != 0 {
		t.Fatalf("confirmed paused seek to zero was lost: %v", got)
	}
	observe(t, manager, sample(1, -time.Second, playback.Playing))
	if manager.State().Records[0].Position != 0 {
		t.Fatal("negative position not clamped")
	}
	observe(t, manager, sample(1, 3*time.Minute, playback.Paused))
	idle(t, manager)
	if record := store.snapshot()[0]; record.Position != 2*time.Minute || record.Completed {
		t.Fatalf("clamp/completion %+v", record)
	}
	observe(t, manager, sample(1, time.Minute, playback.Ended))
	idle(t, manager)
	if !store.snapshot()[0].Completed {
		t.Fatal("native Ended not recorded")
	}
	observe(t, manager, sample(2, 0, playback.Playing))
	idle(t, manager)
	if store.snapshot()[0].Completed {
		t.Fatal("replay stayed completed")
	}
}

func TestDeleteClearSerializedRevisionAndVisitSuppression(t *testing.T) {
	for _, clearAll := range []bool{false, true} {
		t.Run(fmt.Sprint(clearAll), func(t *testing.T) {
			store := newFakeStore()
			first, second := newGate(), newGate()
			store.gates["save"] = []*callGate{first, second}
			manager := readyManager(t, store)
			observe(t, manager, sample(1, time.Second, playback.Playing))
			waitGate(t, first)
			observe(t, manager, sample(1, 8*time.Second, playback.Paused))
			if clearAll {
				manager.Clear()
			} else {
				manager.Delete(sample(1, 0, playback.Ready).Record.Ref)
			}
			wantRevision := manager.State().Revision
			close(first.release)
			pump(t, manager, func(state State) bool { return state.SavedRevision > 0 })
			waitGate(t, second)
			if state := manager.State(); !state.Dirty || state.SavedRevision == wantRevision || len(state.Records) != 0 {
				t.Fatalf("old save acknowledged deletion %+v", state)
			}
			observe(t, manager, sample(1, 30*time.Second, playback.Playing))
			if manager.State().Revision != wantRevision || len(manager.State().Records) != 0 {
				t.Fatal("same visit resurrected deletion")
			}
			close(second.release)
			idle(t, manager)
			if state := manager.State(); state.Dirty || state.SavedRevision != wantRevision || len(store.snapshot()) != 0 || store.count("save") != 2 {
				t.Fatalf("latest deletion not saved %+v", state)
			}
			observe(t, manager, sample(2, 0, playback.Playing))
			idle(t, manager)
			if len(store.snapshot()) != 1 {
				t.Fatal("fresh play could not create a row")
			}
			if err := manager.Observe(sample(1, time.Second, playback.Playing)); !errors.Is(err, Invalid) {
				t.Fatal("old visit accepted")
			}
			other := sample(2, time.Second, playback.Playing)
			other.Record.Ref.Part = "456"
			if err := manager.Observe(other); !errors.Is(err, Invalid) {
				t.Fatal("one visit changed part")
			}
		})
	}
}

func TestDeleteCurrentVisitBeforeUsefulProgress(t *testing.T) {
	store := newFakeStore()
	store.data = []Record{sample(1, 20*time.Second, playback.Paused).Record}
	manager := readyManager(t, store)
	loading := sample(5, 0, playback.Loading)
	loading.Record.Duration = 0
	observe(t, manager, loading)
	manager.Delete(loading.Record.Ref)
	idle(t, manager)
	observe(t, manager, sample(5, time.Second, playback.Playing))
	if len(manager.State().Records) != 0 || manager.State().Dirty {
		t.Fatal("deleted loading visit recreated")
	}
}

func TestStateCopiesTitlesAndRetention(t *testing.T) {
	store := newFakeStore()
	for n := range 501 {
		record := sample(1, time.Second, playback.Paused).Record
		record.Ref.Part = catalog.PartID(fmt.Sprint(n + 1))
		record.UpdatedAt = time.Unix(int64(n+1), 0).UTC()
		store.data = append(store.data, record)
	}
	manager := readyManager(t, store)
	idle(t, manager)
	if got := manager.State().Records; len(got) != 500 || got[0].Ref.Part != "501" || got[499].Ref.Part != "2" {
		t.Fatal("retention did not evict oldest")
	}
	copy := manager.State()
	copy.Records[0].Title = "mutated"
	if manager.State().Records[0].Title == "mutated" {
		t.Fatal("State leaked backing slice")
	}
	value := sample(1, time.Second, playback.Paused)
	value.Record.Title = strings.Repeat("中", 86)
	value.Record.PartTitle = strings.Repeat("a", 255) + "文"
	observe(t, manager, value)
	idle(t, manager)
	record := manager.State().Records[0]
	if len(record.Title) != 255 || len(record.PartTitle) != 255 || !utf8.ValidString(record.Title) || !utf8.ValidString(record.PartTitle) {
		t.Fatalf("bad truncation %+v", record)
	}
	if len(store.snapshot()) != 500 {
		t.Fatal("new visit exceeded retention limit")
	}
}

func TestFinalSaveAndWorkerRetirement(t *testing.T) {
	store := newFakeStore()
	manager := readyManager(t, store)
	clock := time.Now()
	manager.now = func() time.Time { return clock }
	observe(t, manager, sample(1, time.Second, playback.Playing))
	idle(t, manager)
	save, cleanup := newGate(), newGate()
	store.mu.Lock()
	store.gates["save"] = []*callGate{save}
	store.gates["close"] = []*callGate{cleanup}
	store.mu.Unlock()
	clock = clock.Add(time.Second)
	observe(t, manager, sample(1, 2*time.Second, playback.Playing))
	if manager.RequestClose(false) {
		t.Fatal("closed before final save")
	}
	waitGate(t, save)
	if manager.RequestClose(true) || store.count("close") != 0 {
		t.Fatal("busy discard or premature cleanup")
	}
	if err := manager.Observe(sample(1, 9*time.Second, playback.Playing)); !errors.Is(err, Closed) {
		t.Fatal(err)
	}
	select {
	case <-manager.Done():
		t.Fatal("Done before save")
	default:
	}
	close(save.release)
	pump(t, manager, func(state State) bool { return !state.Dirty })
	waitGate(t, cleanup)
	select {
	case <-manager.Done():
		t.Fatal("Done before cleanup")
	default:
	}
	close(cleanup.release)
	pump(t, manager, func(state State) bool { return state.Phase == Done })
	select {
	case <-manager.retired:
	default:
		t.Fatal("Done before worker retirement")
	}
	if !manager.RequestClose(false) || store.count("close") != 1 || store.max != 1 || store.snapshot()[0].Position != 2*time.Second {
		t.Fatal("final ordering/idempotence")
	}
	if err := manager.Start(); !errors.Is(err, Closed) {
		t.Fatal(err)
	}
}

func TestFailedSaveAndCleanupRetryOnly(t *testing.T) {
	for _, failure := range []Code{Storage, Uncertain} {
		t.Run(string(failure), func(t *testing.T) {
			store := newFakeStore()
			store.errors["save"] = []error{failure}
			store.errors["close"] = []error{Corrupt}
			manager := readyManager(t, store)
			observe(t, manager, sample(1, time.Second, playback.Playing))
			pump(t, manager, func(state State) bool { return state.Phase == Failed })
			observe(t, manager, sample(1, 17*time.Second, playback.Paused))
			manager.RequestClose(false)
			for range 10 {
				manager.Tick()
			}
			if state := manager.State(); !state.Dirty || state.Error != failure || store.count("save") != 1 {
				t.Fatalf("save failed silently %+v", state)
			}
			if err := manager.Retry(); err != nil {
				t.Fatal(err)
			}
			pump(t, manager, func(state State) bool { return state.Phase == Failed && manager.cleaning })
			if store.snapshot()[0].Position != 17*time.Second || manager.State().Error != Storage {
				t.Fatal("retry lost latest sample or cleanup error")
			}
			select {
			case <-manager.Done():
				t.Fatal("cleanup failure closed Done")
			default:
			}
			if manager.RequestClose(true) || store.count("close") != 1 {
				t.Fatal("discard waived cleanup error")
			}
			if err := manager.Retry(); err != nil {
				t.Fatal(err)
			}
			pump(t, manager, func(state State) bool { return state.Phase == Done })
			if store.count("save") != 2 || store.count("close") != 2 || store.count("acquire") != 1 || store.count("load") != 1 {
				t.Fatal("cleanup retry performed unrelated I/O")
			}
		})
	}
}

func TestDiscardDoesNotRewriteAndEarlyClose(t *testing.T) {
	t.Run("discard", func(t *testing.T) {
		store := newFakeStore()
		store.errors["save"] = []error{Storage}
		manager := readyManager(t, store)
		observe(t, manager, sample(1, time.Second, playback.Playing))
		pump(t, manager, func(state State) bool { return state.Phase == Failed })
		manager.RequestClose(true)
		pump(t, manager, func(state State) bool { return state.Phase == Done })
		if store.count("save") != 1 || manager.State().Dirty || len(store.snapshot()) != 0 {
			t.Fatal("discard wrote old snapshot")
		}
	})
	for _, stage := range []string{"new", "acquire", "load"} {
		t.Run(stage, func(t *testing.T) {
			store := newFakeStore()
			manager, _ := New(store)
			if stage != "new" {
				store.errors[stage] = []error{Storage}
				manager.Start()
				pump(t, manager, func(state State) bool { return state.Phase == Failed })
			}
			manager.RequestClose(false)
			pump(t, manager, func(state State) bool { return state.Phase == Done })
			if store.count("close") != 1 || store.count("save") != 0 || stage == "new" && (store.count("acquire") != 0 || store.count("load") != 0) {
				t.Fatal("early close performed I/O before cleanup")
			}
		})
	}
}

func TestOnePendingSnapshotAndAcknowledgedResult(t *testing.T) {
	store := newFakeStore()
	gate := newGate()
	store.gates["save"] = []*callGate{gate}
	manager := readyManager(t, store)
	observe(t, manager, sample(1, time.Second, playback.Paused))
	waitGate(t, gate)
	for n := 2; n <= 20; n++ {
		observe(t, manager, sample(1, time.Duration(n)*time.Second, playback.Paused))
	}
	if store.count("save") != 1 || !manager.State().Busy {
		t.Fatal("more than one active Store call")
	}
	close(gate.release)
	// Receive without acknowledging on the UI thread. The worker cannot start
	// the pending snapshot and the accepted state must still identify Dirty.
	var result storeResult
	select {
	case result = <-manager.results:
	case <-time.After(2 * time.Second):
		t.Fatal("save result missing")
	}
	if store.count("save") != 1 || !manager.State().Dirty || manager.State().SavedRevision != 0 {
		t.Fatal("unacknowledged result advanced history")
	}
	if got := store.snapshot()[0].Position; got != time.Second {
		t.Fatal("queued request did not own its copy", got)
	}
	manager.accept(result)
	idle(t, manager)
	if got := store.snapshot()[0].Position; got != 20*time.Second || store.count("save") != 2 || manager.State().Dirty {
		t.Fatalf("pending snapshots did not coalesce: %v %+v", got, manager.State())
	}
}

func TestCloseWhileLoadPendingDoesNotNeedPaint(t *testing.T) {
	store := newFakeStore()
	gate := newGate()
	store.gates["load"] = []*callGate{gate}
	manager, _ := New(store)
	manager.Start()
	waitGate(t, gate)
	if manager.RequestClose(false) {
		t.Fatal("pending load claimed close")
	}
	if store.count("close") != 0 {
		t.Fatal("cleanup raced load")
	}
	close(gate.release)
	pump(t, manager, func(state State) bool { return state.Phase == Done })
	if store.count("save") != 0 || store.count("close") != 1 {
		t.Fatal("load close wrote or omitted cleanup")
	}
}

func TestWorkerWaitsForAcknowledgment(t *testing.T) {
	store := newFakeStore()
	gate := newGate()
	store.gates["save"] = []*callGate{gate}
	requests := make(chan storeRequest, 1)
	results := make(chan storeResult, 1)
	retired := make(chan struct{})
	go runStore(store, requests, results, retired)
	requests <- storeRequest{operation: loadStore}
	load := <-results
	requests <- storeRequest{operation: saveStore, records: []Record{sample(1, time.Second, playback.Paused).Record}}
	select {
	case <-gate.entered:
		t.Fatal("second operation ran before first result acknowledgment")
	case <-time.After(20 * time.Millisecond):
	}
	close(load.consumed)
	waitGate(t, gate)
	close(gate.release)
	saved := <-results
	close(saved.consumed)
	requests <- storeRequest{operation: closeStore}
	closed := <-results
	close(closed.consumed)
	select {
	case <-retired:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not retire")
	}
}

func TestDoneWaitsForBothCleanupAcceptanceAndRetirement(t *testing.T) {
	manager, _ := New(newFakeStore())
	manager.closing, manager.cleaning = true, true
	manager.state.Phase, manager.state.Busy = Closing, true
	manager.results <- storeResult{request: storeRequest{operation: closeStore}, consumed: make(chan struct{})}
	manager.Tick()
	select {
	case <-manager.Done():
		t.Fatal("cleanup result alone manufactured worker retirement")
	default:
	}
	close(manager.retired)
	manager.Tick()
	select {
	case <-manager.Done():
	default:
		t.Fatal("accepted cleanup and retirement did not close Done")
	}
}
