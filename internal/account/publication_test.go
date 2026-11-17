package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

type memoryPublisher struct {
	active   atomic.Uint64
	calls    atomic.Int32
	revokes  atomic.Int32
	activate func(uint64, Material) error
}

func (p *memoryPublisher) Activate(epoch uint64, material Material) error {
	p.calls.Add(1)
	if p.activate != nil {
		if err := p.activate(epoch, material); err != nil {
			return err
		}
	}
	p.active.Store(epoch)
	return nil
}
func (p *memoryPublisher) Revoke(epoch uint64) {
	if p.active.CompareAndSwap(epoch, 0) {
		p.revokes.Add(1)
	}
}
func doneClosed(m *Manager) bool {
	select {
	case <-m.Done():
		return true
	default:
		return false
	}
}

func TestPublisherTransactionAndMatchingEpoch(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return material(), nil }
	p := new(memoryPublisher)
	m := NewWithPublisher(g, s, p)
	p.activate = func(epoch uint64, value Material) error {
		if m.mu.TryLock() {
			m.mu.Unlock()
			t.Error("publication was outside Manager transaction")
		}
		if m.state.Phase != Verifying || m.state.SessionEpoch != 0 || epoch == 0 || value.Empty() {
			t.Error("usable state preceded publication")
		}
		return nil
	}
	if m.Restore() != nil {
		t.Fatal("restore failed")
	}
	a := waitState(t, m, SignedIn, false).SessionEpoch
	if a == 0 || p.active.Load() != a || doneClosed(m) {
		t.Fatal("verified epoch not active")
	}
	if m.SessionExpired(0) || m.SessionExpired(a+1) || !m.SessionExpired(a) || m.SessionExpired(a) {
		t.Fatal("matching expiry acceptance wrong")
	}
	state := m.Snapshot()
	if state.Phase != Expired || state.SessionEpoch != 0 || !state.Stored || p.active.Load() != 0 || s.deletes.Load() != 0 {
		t.Fatal("expiry did not revoke while preserving storage")
	}
	if m.Begin() != nil {
		t.Fatal("replacement sign-in failed")
	}
	b := waitState(t, m, SignedIn, false).SessionEpoch
	if b <= a || m.SessionExpired(a) || p.active.Load() != b {
		t.Fatal("stale A affected B")
	}
	if !m.RequestClose(false) || !doneClosed(m) || p.active.Load() != 0 || m.Snapshot().SessionEpoch != 0 || s.deletes.Load() != 0 {
		t.Fatal("normal close did not synchronously retire B")
	}
	if !m.RequestClose(false) || p.revokes.Load() != 2 {
		t.Fatal("repeated close revoked twice")
	}
}

func TestActivationFailureIsSafeAndRetryAllocatesNewEpoch(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return material(), nil }
	p := new(memoryPublisher)
	p.activate = func(uint64, Material) error {
		if p.calls.Load() == 1 {
			return errors.New("PRIVATE_PUBLISHER_FAILURE")
		}
		return nil
	}
	m := NewWithPublisher(g, s, p)
	_ = m.Restore()
	state := waitState(t, m, VerifyFailed, false)
	if state.Error != Invalid || state.SessionEpoch != 0 || p.active.Load() != 0 || !state.Stored {
		t.Fatal("activation failure became usable or unsafe")
	}
	if m.RetryVerify() != nil {
		t.Fatal("activation retry failed")
	}
	state = waitState(t, m, SignedIn, false)
	if state.SessionEpoch != 2 || p.calls.Load() != 2 || g.begins.Load() != 0 || s.deletes.Load() != 0 {
		t.Fatal("retry reused epoch or changed saved login")
	}
	m.RequestClose(false)
}

func TestAccountEpochDoesNotWrapToUsableZero(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return material(), nil }
	p := new(memoryPublisher)
	m := NewWithPublisher(g, s, p)
	m.nextEpoch = ^uint64(0)
	_ = m.Restore()
	state := waitState(t, m, VerifyFailed, false)
	if state.Error != Invalid || state.SessionEpoch != 0 || p.calls.Load() != 0 {
		t.Fatal("exhausted epoch published")
	}
	m.RequestClose(false)
}

func TestCloseSuppressesLateActivationAndJoinsWorker(t *testing.T) {
	for _, discard := range []bool{false, true} {
		g, s, _ := ports()
		s.load = func() (Material, error) { return material(), nil }
		entered, release := make(chan struct{}), make(chan struct{})
		g.verify = func(ctx context.Context, _ Material) error { close(entered); <-release; return nil }
		p := new(memoryPublisher)
		m := NewWithPublisher(g, s, p)
		_ = m.Restore()
		<-entered
		if m.RequestClose(discard) || doneClosed(m) {
			t.Fatal("close abandoned verification")
		}
		close(release)
		waitState(t, m, Done, false)
		if !doneClosed(m) || p.calls.Load() != 0 || m.Snapshot().SessionEpoch != 0 || s.deletes.Load() != 0 {
			t.Fatal("late verify published or join absent")
		}
	}
}

func TestRevokePrecedesDeleteAndDoneWaitsForNativeStore(t *testing.T) {
	g, s, _ := ports()
	s.load = func() (Material, error) { return material(), nil }
	p := new(memoryPublisher)
	m := NewWithPublisher(g, s, p)
	entered, release := make(chan struct{}), make(chan struct{})
	s.del = func() error {
		if p.active.Load() != 0 || m.Snapshot().SessionEpoch != 0 {
			t.Error("Delete began before revoke or under Manager lock")
		}
		close(entered)
		<-release
		return Storage
	}
	_ = m.Restore()
	waitState(t, m, SignedIn, false)
	if m.Logout() != nil {
		t.Fatal("logout failed")
	}
	<-entered
	if m.RequestClose(true) || doneClosed(m) {
		t.Fatal("native delete abandoned")
	}
	close(release)
	state := waitState(t, m, DeleteFailed, false)
	if doneClosed(m) || state.SessionEpoch != 0 || !state.Stored || p.active.Load() != 0 {
		t.Fatal("failed deletion reactivated or closed")
	}
	if !m.RequestClose(true) || !doneClosed(m) || !m.Snapshot().Stored {
		t.Fatal("explicit acknowledgement lost persistence uncertainty")
	}
}

func TestGatewayAndStoreRemainOutsideManagerLock(t *testing.T) {
	g, s, _ := ports()
	m := New(g, s)
	s.load = func() (Material, error) { _ = m.Snapshot(); return material(), nil }
	g.verify = func(context.Context, Material) error { _ = m.Snapshot(); return nil }
	_ = m.Restore()
	waitState(t, m, SignedIn, false)
	m.RequestClose(false)
}

func TestDiscardUnsavedRetainsOlderStoreAndCannotInterruptSave(t *testing.T) {
	for _, stored := range []bool{false, true} {
		g, s, _ := ports()
		if stored {
			s.load = func() (Material, error) { return material(), nil }
		}
		m := New(g, s)
		_ = m.Restore()
		if stored {
			m.SessionExpired(waitState(t, m, SignedIn, false).SessionEpoch)
		} else {
			waitState(t, m, SignedOut, false)
		}
		entered, release := make(chan struct{}), make(chan struct{})
		s.save = func(Material) error { _ = m.Snapshot(); close(entered); <-release; return Storage }
		_ = m.Begin()
		<-entered
		if m.DiscardUnsaved() != Busy || !m.Snapshot().Unsaved {
			t.Fatal("active save discarded")
		}
		close(release)
		waitState(t, m, SaveFailed, false)
		if m.DiscardUnsaved() != nil {
			t.Fatal("explicit discard failed")
		}
		state := m.Snapshot()
		want := SignedOut
		if stored {
			want = Expired
		}
		if state.Phase != want || state.Unsaved || state.Stored != stored || !m.material.Empty() || s.deletes.Load() != 0 || doneClosed(m) {
			t.Fatal("discard changed stored credentials or closed manager")
		}
		if m.DiscardUnsaved() != Invalid {
			t.Fatal("discard accepted without unsaved material")
		}
		m.RequestClose(false)
		if m.DiscardUnsaved() != Closed {
			t.Fatal("closed discard accepted")
		}
	}
}

func TestManagerFormattingDoesNotExposePrivateMaterial(t *testing.T) {
	g, s, _ := ports()
	m := New(g, s)
	m.material = material()
	if fmt.Sprintf("%+v", m) != "[private account manager]" || fmt.Sprintf("%#v", m) != "[private account manager]" {
		t.Fatal("manager formatted private fields")
	}
	data, err := json.Marshal(m)
	if err != nil || string(data) != "null" {
		t.Fatal("manager serialized private fields")
	}
	m.RequestClose(false)
}

func TestCloseRevokesAfterPublicationBeforeWorkerFinish(t *testing.T) {
	g, store, _ := ports()
	publisher := new(memoryPublisher)
	m := NewWithPublisher(g, store, publisher)
	verified, release := make(chan struct{}), make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	m.mu.Lock()
	m.material, m.state.Stored = material(), true
	m.start(Verifying, 0, func(ctx context.Context, generation uint64) {
		m.verify(ctx, generation)
		close(verified)
		<-release
	})
	m.mu.Unlock()
	<-verified
	if m.Snapshot().SessionEpoch == 0 || !m.Snapshot().Busy {
		t.Fatal("publication checkpoint missing")
	}
	if m.RequestClose(false) || publisher.active.Load() != 0 || m.Snapshot().SessionEpoch != 0 || doneClosed(m) {
		t.Fatal("close returned before revoking an active published epoch")
	}
	close(release)
	released = true
	waitState(t, m, Done, false)
	if !doneClosed(m) {
		t.Fatal("terminal close did not join the worker")
	}
}
