package account

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	domain "my-bilibili/internal/account"
)

type paneGateway struct {
	attempt          domain.Attempt
	verify           func(context.Context) error
	begins, verifies atomic.Int32
}

func (g *paneGateway) Begin(context.Context) (domain.Attempt, error) {
	g.begins.Add(1)
	return g.attempt, nil
}
func (g *paneGateway) Verify(ctx context.Context, _ domain.Material) error {
	g.verifies.Add(1)
	if g.verify != nil {
		return g.verify(ctx)
	}
	return nil
}

type paneAttempt struct {
	poll   func(context.Context) (domain.PollResult, error)
	closed atomic.Bool
}

func (*paneAttempt) QR() domain.QR { return domain.NewQR(privateQR) }
func (a *paneAttempt) Poll(ctx context.Context) (domain.PollResult, error) {
	if a.poll != nil {
		return a.poll(ctx)
	}
	return domain.PollResult{Status: domain.PollComplete, Material: paneMaterial()}, nil
}
func (a *paneAttempt) Close() { a.closed.Store(true) }

type paneStore struct {
	stored         bool
	save           func() error
	saves, deletes atomic.Int32
}

func (s *paneStore) Load() (domain.Material, error) {
	if s.stored {
		return paneMaterial(), nil
	}
	return domain.Material{}, domain.Missing
}
func (s *paneStore) Save(domain.Material) error {
	s.saves.Add(1)
	if s.save != nil {
		return s.save()
	}
	return nil
}
func (s *paneStore) Delete() error { s.deletes.Add(1); return nil }
func paneMaterial() domain.Material {
	return domain.NewMaterial([]byte("ORIGINAL_SYNTHETIC_PANE_SESSION"))
}
func releaseGate() (<-chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}
func waitAccount(t *testing.T, m *domain.Manager, phase domain.Phase) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		state := m.Snapshot()
		if state.Phase == phase && !state.Busy {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("account did not reach the expected safe phase")
}
func managerStillOpen(t *testing.T, m *domain.Manager) {
	t.Helper()
	select {
	case <-m.Done():
		t.Fatal("pane closed its reusable manager")
	default:
	}
}
func cleanupManager(t *testing.T, m *domain.Manager) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for m.Snapshot().Busy && time.Now().Before(end) {
		time.Sleep(time.Millisecond)
	}
	if !m.RequestClose(true) {
		t.Error("synthetic manager cleanup remained busy")
		return
	}
	select {
	case <-m.Done():
	case <-time.After(time.Second):
		t.Error("synthetic manager did not finish close")
	}
}
func testClick(t *testing.T, ui *gui.Tester, label string) {
	t.Helper()
	if ui.Click(label) != nil {
		t.Fatal("account pane action unavailable")
	}
	ui.Frame()
}

func TestEmbeddedUnsavedDiscardPreservesStoredDataAndManager(t *testing.T) {
	for _, stored := range []bool{false, true} {
		g := &paneGateway{attempt: &paneAttempt{}}
		store := &paneStore{stored: stored, save: func() error { return domain.Storage }}
		m := domain.New(g, store)
		t.Cleanup(func() { cleanupManager(t, m) })
		if m.Restore() != nil {
			t.Fatal("synthetic restore failed")
		}
		if stored {
			waitAccount(t, m, domain.SignedIn)
			if !m.SessionExpired(m.Snapshot().SessionEpoch) {
				t.Fatal("synthetic old-session expiry failed")
			}
		} else {
			waitAccount(t, m, domain.SignedOut)
		}
		if m.Begin() != nil {
			t.Fatal("synthetic attempt failed")
		}
		waitAccount(t, m, domain.SaveFailed)
		dismisses := 0
		v := NewPane(m, func() { dismisses++ })
		v.Show()
		ui := gui.NewTester(v.Render, 440, 640)
		testClick(t, ui, "关闭面板")
		if state := m.Snapshot(); state.Phase != domain.SaveFailed || !state.Unsaved || state.CloseRequested || state.Stored != stored {
			t.Fatal("pane dismissal changed pending save disposition")
		}
		managerStillOpen(t, m)
		v.Show()
		ui.Frame()
		testClick(t, ui, "放弃本次登录")
		want := domain.SignedOut
		if stored {
			want = domain.Expired
		}
		waitAccount(t, m, want)
		if state := m.Snapshot(); state.Unsaved || state.Stored != stored || state.CloseRequested || !v.Visible() || dismisses != 1 || g.begins.Load() != 1 || store.saves.Load() != 1 || store.deletes.Load() != 0 {
			t.Fatal("embedded discard changed storage or manager lifetime")
		}
		managerStillOpen(t, m)
		if !m.RequestClose(false) {
			t.Fatal("composition could not close idle manager")
		}
	}
}

func TestEmbeddedHideAllowsSaveAndVerificationInBackground(t *testing.T) {
	for _, stage := range []string{"saving", "verifying"} {
		entered := make(chan context.Context, 1)
		release, open := releaseGate()
		g := &paneGateway{attempt: &paneAttempt{}}
		store := &paneStore{stored: stage == "verifying"}
		if stage == "saving" {
			store.save = func() error { entered <- nil; <-release; return nil }
		} else {
			g.verify = func(ctx context.Context) error { entered <- ctx; <-release; return ctx.Err() }
		}
		m := domain.New(g, store)
		t.Cleanup(func() { open(); cleanupManager(t, m) })
		if m.Restore() != nil {
			t.Fatal("synthetic restore failed")
		}
		if stage == "saving" {
			waitAccount(t, m, domain.SignedOut)
			if m.Begin() != nil {
				t.Fatal("synthetic begin failed")
			}
		}
		var verifyContext context.Context
		select {
		case verifyContext = <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("background account operation not entered")
		}
		v := NewPane(m, nil)
		v.Show()
		ui := gui.NewTester(v.Render, 440, 640)
		if v.Hide() != nil || v.Visible() || m.Snapshot().CloseRequested || !m.Snapshot().Busy {
			t.Fatal("hide canceled or closed background account work")
		}
		if verifyContext != nil && verifyContext.Err() != nil {
			t.Fatal("hide canceled verification context")
		}
		managerStillOpen(t, m)
		ui.Frame()
		if len(ui.Texts()) != 0 || v.bitmap != nil {
			t.Fatal("hidden pane kept presenting background work")
		}
		open()
		waitAccount(t, m, domain.SignedIn)
		if !m.Snapshot().Stored || m.Snapshot().SessionEpoch == 0 || store.deletes.Load() != 0 || g.verifies.Load() != 1 {
			t.Fatal("hidden save/verify did not publish its verified result")
		}
		managerStillOpen(t, m)
		v.Show()
		ui.Frame()
		if !ui.HasText("登录已保存") && !ui.HasText("已恢复登录") {
			t.Fatal("reopened pane lost completed login")
		}
		if !m.RequestClose(false) {
			t.Fatal("composition close failed")
		}
	}
}

func TestEmbeddedHideCancelsLateQRCompletionWithoutStoreMutation(t *testing.T) {
	entered := make(chan struct{})
	release, open := releaseGate()
	defer open()
	a := &paneAttempt{poll: func(context.Context) (domain.PollResult, error) {
		close(entered)
		<-release
		return domain.PollResult{Status: domain.PollComplete, Material: paneMaterial()}, nil
	}}
	g := &paneGateway{attempt: a}
	store := &paneStore{}
	m := domain.New(g, store)
	t.Cleanup(func() { open(); cleanupManager(t, m) })
	_ = m.Restore()
	waitAccount(t, m, domain.SignedOut)
	if m.Begin() != nil {
		t.Fatal("synthetic begin failed")
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("QR poll not entered")
	}
	v := NewPane(m, nil)
	v.Show()
	ui := gui.NewTester(v.Render, 440, 640)
	if v.bitmap == nil {
		t.Fatal("active QR not shown")
	}
	if v.Hide() != nil || v.bitmap != nil || !v.qr.Empty() {
		t.Fatal("QR hide failed")
	}
	open()
	waitAccount(t, m, domain.SignedOut)
	if store.saves.Load() != 0 || store.deletes.Load() != 0 || g.verifies.Load() != 0 || !a.closed.Load() || m.Snapshot().CloseRequested {
		t.Fatal("late QR completion survived pane cancellation")
	}
	managerStillOpen(t, m)
	v.Show()
	ui.Frame()
	if g.begins.Load() != 1 || v.bitmap != nil || !ui.HasText("开始扫码登录") {
		t.Fatal("reopening pane revived canceled QR or auto-started login")
	}
}
