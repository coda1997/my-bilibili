package account

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeGateway struct {
	begin            func(context.Context) (Attempt, error)
	verify           func(context.Context, Material) error
	begins, verifies atomic.Int32
}

func (g *fakeGateway) Begin(c context.Context) (Attempt, error) { g.begins.Add(1); return g.begin(c) }
func (g *fakeGateway) Verify(c context.Context, m Material) error {
	g.verifies.Add(1)
	return g.verify(c, m)
}

type fakeStore struct {
	load                  func() (Material, error)
	save                  func(Material) error
	del                   func() error
	loads, saves, deletes atomic.Int32
}

func (s *fakeStore) Load() (Material, error) { s.loads.Add(1); return s.load() }
func (s *fakeStore) Save(m Material) error   { s.saves.Add(1); return s.save(m) }
func (s *fakeStore) Delete() error           { s.deletes.Add(1); return s.del() }

type fakeAttempt struct {
	poll   func(context.Context) (PollResult, error)
	closed atomic.Bool
	polls  atomic.Int32
}

func (a *fakeAttempt) QR() QR { return NewQR("PRIVATE_SYNTHETIC_QR") }
func (a *fakeAttempt) Poll(ctx context.Context) (PollResult, error) {
	a.polls.Add(1)
	return a.poll(ctx)
}
func (a *fakeAttempt) Close() { a.closed.Store(true) }
func material() Material      { return NewMaterial([]byte("PRIVATE_SYNTHETIC_SESSION")) }
func ports() (*fakeGateway, *fakeStore, *fakeAttempt) {
	a := &fakeAttempt{poll: func(context.Context) (PollResult, error) {
		return PollResult{Status: PollComplete, Material: material()}, nil
	}}
	g := &fakeGateway{begin: func(context.Context) (Attempt, error) { return a, nil }, verify: func(context.Context, Material) error { return nil }}
	s := &fakeStore{load: func() (Material, error) { return Material{}, Missing }, save: func(m Material) error {
		if string(m.Bytes()) != string(material().Bytes()) {
			return Storage
		}
		return nil
	}, del: func() error { return nil }}
	return g, s, a
}
func waitState(t *testing.T, m *Manager, phase Phase, busy bool) State {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.Phase == phase && s.Busy == busy {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state did not reach %s busy=%v; safe phase=%s", phase, busy, m.Snapshot().Phase)
	return State{}
}
func ready(t *testing.T, m *Manager) {
	t.Helper()
	if err := m.Restore(); err != nil {
		t.Fatal(err)
	}
	waitState(t, m, SignedOut, false)
}

func TestSaveFirstRetryDoesNotNeedAnotherScan(t *testing.T) {
	g, s, a := ports()
	var saves atomic.Int32
	s.save = func(m Material) error {
		if m.Empty() {
			return Storage
		}
		if saves.Add(1) == 1 {
			return errors.New("PRIVATE_ERROR")
		}
		return nil
	}
	g.verify = func(_ context.Context, m Material) error {
		if saves.Load() != 2 || m.Empty() {
			t.Error("verify before durable save")
		}
		return nil
	}
	m := New(g, s)
	if m.Snapshot().Phase != Initial || s.loads.Load() != 0 || g.begins.Load() != 0 {
		t.Fatal("constructor has effects")
	}
	ready(t, m)
	if err := m.Begin(); err != nil {
		t.Fatal(err)
	}
	state := waitState(t, m, SaveFailed, false)
	if !state.Unsaved || state.Stored || state.Error != Storage || !state.QR.Empty() {
		t.Fatal("save failure lost material or claimed persistence")
	}
	if err := m.RetrySave(); err != nil {
		t.Fatal(err)
	}
	state = waitState(t, m, SignedIn, false)
	if !state.Stored || state.Unsaved || g.begins.Load() != 1 || a.polls.Load() != 1 || s.saves.Load() != 2 || !a.closed.Load() {
		t.Fatal("retry regenerated QR or lost saved material")
	}
	if !m.RequestClose(false) || !m.Snapshot().Stored || s.deletes.Load() != 0 {
		t.Fatal("ordinary close deleted stored session")
	}
}
func TestVerifyRetryAndRestoreRetainSession(t *testing.T) {
	for _, restore := range []bool{false, true} {
		g, s, _ := ports()
		var calls atomic.Int32
		g.verify = func(context.Context, Material) error {
			if calls.Add(1) == 1 {
				return Network
			}
			return nil
		}
		if restore {
			s.load = func() (Material, error) { return material(), nil }
		}
		m := New(g, s)
		if restore {
			if err := m.Restore(); err != nil {
				t.Fatal(err)
			}
		} else {
			ready(t, m)
			if err := m.Begin(); err != nil {
				t.Fatal(err)
			}
		}
		state := waitState(t, m, VerifyFailed, false)
		if !state.Stored || state.Unsaved || state.Restored || state.Error != Network || s.deletes.Load() != 0 {
			t.Fatal("network failure discarded persistent state")
		}
		if err := m.RetryVerify(); err != nil {
			t.Fatal(err)
		}
		state = waitState(t, m, SignedIn, false)
		expectedBegins := int32(1)
		if restore {
			expectedBegins = 0
		}
		if state.Restored != restore || g.begins.Load() != expectedBegins || s.deletes.Load() != 0 {
			t.Fatal("retry re-scanned or lost restore provenance")
		}
	}
}
func TestCancelSuppressesLatePollSaveAndPublication(t *testing.T) {
	g, s, a := ports()
	entered, release := make(chan struct{}), make(chan struct{})
	a.poll = func(context.Context) (PollResult, error) {
		close(entered)
		<-release
		return PollResult{Status: PollComplete, Material: material()}, nil
	}
	m := New(g, s)
	ready(t, m)
	if err := m.Begin(); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := m.Cancel(); err != nil {
		t.Fatal(err)
	}
	if !m.Snapshot().QR.Empty() || m.Snapshot().Phase != Canceling || !m.Snapshot().Busy {
		t.Fatal("cancel did not immediately retire QR")
	}
	if m.Begin() != Busy {
		t.Fatal("second worker started while first is active")
	}
	close(release)
	waitState(t, m, SignedOut, false)
	if s.saves.Load() != 0 || g.verifies.Load() != 0 || !a.closed.Load() {
		t.Fatal("late canceled response saved or verified")
	}
}
func TestNativeStoreSerializationAndCloseWait(t *testing.T) {
	g, s, _ := ports()
	entered, release := make(chan struct{}), make(chan struct{})
	s.save = func(Material) error { close(entered); <-release; return nil }
	m := New(g, s)
	ready(t, m)
	if err := m.Begin(); err != nil {
		t.Fatal(err)
	}
	<-entered
	if m.Cancel() != Busy || m.Logout() != Busy || m.Restore() != Busy || m.Begin() != Busy || m.RetrySave() != Busy {
		t.Fatal("native store work allowed overlap or false cancellation")
	}
	if m.RequestClose(false) || m.RequestClose(true) || m.Snapshot().Phase == Done {
		t.Fatal("close abandoned an active native write")
	}
	close(release)
	state := waitState(t, m, Done, false)
	if !state.Stored || state.Unsaved || g.verifies.Load() != 0 || s.deletes.Load() != 0 || s.saves.Load() != 1 {
		t.Fatal("closing save lost persistence or performed extra work")
	}
}
func TestUnsavedCloseRequiresExplicitDisposition(t *testing.T) {
	for _, retry := range []bool{false, true} {
		g, s, _ := ports()
		var saves atomic.Int32
		s.save = func(m Material) error {
			if m.Empty() {
				return Storage
			}
			if saves.Add(1) == 1 {
				return Storage
			}
			return nil
		}
		m := New(g, s)
		ready(t, m)
		_ = m.Begin()
		waitState(t, m, SaveFailed, false)
		if m.RequestClose(false) || m.Snapshot().Phase == Done || !m.Snapshot().Unsaved {
			t.Fatal("ordinary close silently discarded unsaved credentials")
		}
		if retry {
			if m.RetrySave() != nil {
				t.Fatal("pending close prevented save retry")
			}
			state := waitState(t, m, Done, false)
			if !state.Stored {
				t.Fatal("close retry did not persist")
			}
		} else {
			if !m.RequestClose(true) || !m.material.Empty() {
				t.Fatal("explicit discard did not close")
			}
		}
		if g.begins.Load() != 1 || s.deletes.Load() != 0 {
			t.Fatal("close rescanned or deleted credentials")
		}
	}
}
func TestLogoutFailureIsVisibleAndRetryable(t *testing.T) {
	for _, ack := range []bool{false, true} {
		g, s, _ := ports()
		var deletes atomic.Int32
		s.load = func() (Material, error) { return material(), nil }
		s.del = func() error {
			if deletes.Add(1) == 1 {
				return errors.New("PRIVATE_ERROR")
			}
			return nil
		}
		m := New(g, s)
		_ = m.Restore()
		waitState(t, m, SignedIn, false)
		if m.Logout() != nil {
			t.Fatal("logout failed to start")
		}
		state := waitState(t, m, DeleteFailed, false)
		if !state.Stored || state.Error != Storage || !m.material.Empty() || m.Begin() != Invalid || m.Restore() != Invalid {
			t.Fatal("failed delete falsely cleared storage or allowed reuse")
		}
		if m.RequestClose(false) {
			t.Fatal("failed logout was silently acknowledged")
		}
		if ack {
			if !m.RequestClose(true) || !m.Snapshot().Stored {
				t.Fatal("acknowledgment falsely claimed deletion")
			}
		} else {
			if m.Logout() != nil {
				t.Fatal("delete retry refused")
			}
			state = waitState(t, m, Done, false)
			if state.Stored {
				t.Fatal("successful retried deletion not reflected")
			}
		}
	}
}
func TestRestoreNativeWaitAndUnknownStoreFailure(t *testing.T) {
	g, s, _ := ports()
	entered, release := make(chan struct{}), make(chan struct{})
	s.load = func() (Material, error) { close(entered); <-release; return material(), nil }
	m := New(g, s)
	_ = m.Restore()
	<-entered
	if m.Cancel() != Busy || m.RequestClose(false) || m.Begin() != Busy {
		t.Fatal("restore native operation abandoned")
	}
	close(release)
	state := waitState(t, m, Done, false)
	if !state.Stored || g.verifies.Load() != 0 {
		t.Fatal("closing restore discarded item or started nav")
	}
	g, s, _ = ports()
	s.load = func() (Material, error) { return Material{}, errors.New("PRIVATE_ERROR") }
	m = New(g, s)
	_ = m.Restore()
	waitState(t, m, Failed, false)
	if m.Begin() != Invalid || m.Snapshot().Error != Storage {
		t.Fatal("unknown saved state allowed overwrite")
	}
	s.load = func() (Material, error) { return Material{}, Missing }
	if m.Restore() != nil {
		t.Fatal("storage recovery refused")
	}
	waitState(t, m, SignedOut, false)
}
func TestPollBudgetExpiryAndErrorStates(t *testing.T) {
	g, s, a := ports()
	a.poll = func(context.Context) (PollResult, error) { return PollResult{Status: PollWaiting}, nil }
	m := New(g, s)
	var waits atomic.Int32
	m.wait = func(ctx context.Context, d time.Duration) error {
		if d != pollInterval {
			t.Error("poll interval changed")
		}
		waits.Add(1)
		return nil
	}
	ready(t, m)
	_ = m.Begin()
	state := waitState(t, m, Expired, false)
	if a.polls.Load() != 90 || waits.Load() != 89 || state.Error != Timeout || !state.QR.Empty() || s.saves.Load() != 0 {
		t.Fatal("poll loop unbounded or expiry retained QR")
	}
	for _, result := range []PollResult{{Status: PollExpired}, {Status: PollComplete}, {Status: "invalid"}} {
		g, s, a = ports()
		a.poll = func(context.Context) (PollResult, error) { return result, nil }
		m = New(g, s)
		ready(t, m)
		_ = m.Begin()
		phase := Failed
		if result.Status == PollExpired {
			phase = Expired
		}
		waitState(t, m, phase, false)
		if s.saves.Load() != 0 || !m.Snapshot().QR.Empty() {
			t.Fatal("bad completion saved credentials")
		}
	}
}
func TestCloseAndCancelNetworkVerification(t *testing.T) {
	for _, closeRequest := range []bool{false, true} {
		g, s, _ := ports()
		s.load = func() (Material, error) { return material(), nil }
		entered, release := make(chan struct{}), make(chan struct{})
		g.verify = func(context.Context, Material) error { close(entered); <-release; return nil }
		m := New(g, s)
		_ = m.Restore()
		<-entered
		if closeRequest {
			if m.RequestClose(false) {
				t.Fatal("network worker not quiescent")
			}
		} else {
			if m.Cancel() != nil {
				t.Fatal("network cancel refused")
			}
		}
		close(release)
		phase := VerifyFailed
		if closeRequest {
			phase = Done
		}
		state := waitState(t, m, phase, false)
		if !state.Stored || state.Restored || s.deletes.Load() != 0 {
			t.Fatal("late verification signed in or erased storage")
		}
	}
}

func TestCancelLateGenerationNeverPublishesQR(t *testing.T) {
	g, s, a := ports()
	entered, release := make(chan struct{}), make(chan struct{})
	g.begin = func(ctx context.Context) (Attempt, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > attemptLifetime || time.Until(deadline) < attemptLifetime-time.Second {
			t.Error("attempt deadline missing")
		}
		close(entered)
		<-release
		return a, nil
	}
	m := New(g, s)
	ready(t, m)
	if m.Begin() != nil {
		t.Fatal("begin failed")
	}
	<-entered
	if m.Cancel() != nil {
		t.Fatal("generation cancel failed")
	}
	close(release)
	waitState(t, m, SignedOut, false)
	if !m.Snapshot().QR.Empty() || !a.closed.Load() || a.polls.Load() != 0 || s.saves.Load() != 0 {
		t.Fatal("late generation revived a canceled attempt")
	}
}
func TestGatewayFailuresAreSafeAndDoNotLoseStoredMaterial(t *testing.T) {
	g, s, _ := ports()
	g.begin = func(context.Context) (Attempt, error) { return nil, errors.New("PRIVATE_GATEWAY_ERROR") }
	m := New(g, s)
	ready(t, m)
	_ = m.Begin()
	state := waitState(t, m, Failed, false)
	if state.Error != Network || !state.QR.Empty() || s.saves.Load() != 0 {
		t.Fatal("generation failure leaked or acquired session")
	}
	g, s, _ = ports()
	s.load = func() (Material, error) { return material(), nil }
	g.verify = func(context.Context, Material) error { return ExpiredSession }
	m = New(g, s)
	_ = m.Restore()
	state = waitState(t, m, Expired, false)
	if !state.Stored || state.Restored || s.deletes.Load() != 0 {
		t.Fatal("expiry erased stored credentials")
	}
}

func TestDeleteCannotBeCanceledOrAbandoned(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return material(), nil }
	entered, release := make(chan struct{}), make(chan struct{})
	s.del = func() error { close(entered); <-release; return nil }
	m := New(g, s)
	_ = m.Restore()
	waitState(t, m, SignedIn, false)
	if m.Logout() != nil {
		t.Fatal("logout failed")
	}
	<-entered
	if m.Cancel() != Busy || m.Begin() != Busy || m.RequestClose(false) || m.RequestClose(true) {
		t.Fatal("native delete abandoned or overlapped")
	}
	close(release)
	state := waitState(t, m, Done, false)
	if state.Stored || s.deletes.Load() != 1 || !m.material.Empty() {
		t.Fatal("completed delete/close reported wrong lifetime")
	}
}

func TestInitialAndClosedActionsDoNotStartWork(t *testing.T) {
	g, s, _ := ports()
	m := New(g, s)
	if m.Begin() != Invalid || m.RetrySave() != Invalid || m.RetryVerify() != Invalid || m.Logout() != Invalid || m.Cancel() != Invalid {
		t.Fatal("unrestored session allowed an action")
	}
	if !m.RequestClose(false) || !m.RequestClose(false) || m.Restore() != Closed || m.Begin() != Closed || m.Cancel() != Closed {
		t.Fatal("closed manager started work")
	}
	if s.loads.Load() != 0 || s.saves.Load() != 0 || s.deletes.Load() != 0 || g.begins.Load() != 0 {
		t.Fatal("invalid action performed I/O")
	}
}
func TestInvalidPortResultsCannotBecomeSession(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return Material{}, nil }
	m := New(g, s)
	_ = m.Restore()
	state := waitState(t, m, Failed, false)
	if state.Error != Storage || state.Stored || g.verifies.Load() != 0 || m.Begin() != Invalid {
		t.Fatal("empty successful load became a known session")
	}
	g, s, _ = ports()
	g.begin = func(context.Context) (Attempt, error) { return nil, nil }
	m = New(g, s)
	ready(t, m)
	_ = m.Begin()
	state = waitState(t, m, Failed, false)
	if state.Error != Invalid || s.saves.Load() != 0 {
		t.Fatal("missing attempt became a session")
	}
}
