package account

import (
	"context"
	"errors"
	"sync"
	"time"
)

const attemptLifetime = 180 * time.Second
const pollInterval = 2 * time.Second
const pollLimit = 90

type Manager struct {
	mu              sync.Mutex
	gateway         Gateway
	store           Store
	state           State
	material        Material
	cancel          context.CancelFunc
	restoreMaterial bool
	cancelPhase     Phase
	now             func() time.Time
	wait            func(context.Context, time.Duration) error
}

func New(gateway Gateway, store Store) *Manager {
	return &Manager{gateway: gateway, store: store, state: State{Phase: Initial}, now: time.Now, wait: waitFor}
}
func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (m *Manager) Snapshot() State { m.mu.Lock(); defer m.mu.Unlock(); return m.state }
func (m *Manager) available() error {
	if m.state.Phase == Done {
		return Closed
	}
	if m.state.Busy {
		return Busy
	}
	return nil
}
func (m *Manager) start(phase Phase, limit time.Duration, fn func(context.Context, uint64)) {
	m.state.Generation++
	generation := m.state.Generation
	m.state.Phase, m.state.Error, m.state.Busy = phase, None, true
	var ctx context.Context
	if limit > 0 {
		ctx, m.cancel = context.WithTimeout(context.Background(), limit)
	} else {
		ctx, m.cancel = context.WithCancel(context.Background())
	}
	cancel := m.cancel
	go func() { defer cancel(); fn(ctx, generation); m.finish() }()
}
func (m *Manager) finish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Busy = false
	m.cancel = nil
	if m.state.CloseRequested && !m.state.Unsaved && m.state.Phase != DeleteFailed {
		m.done()
		return
	}
	if m.state.Phase == Canceling {
		m.state.Phase = m.cancelPhase
	}
}
func (m *Manager) done() {
	m.state.Generation++
	m.state.Phase = Done
	m.state.QR = QR{}
	m.state.ExpiresAt = time.Time{}
	m.state.Error = None
	m.material = Material{}
}
func (m *Manager) current(g uint64) bool { return m.state.Generation == g && !m.state.CloseRequested }
func gatewayCode(err error) Code {
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	var code Code
	if errors.As(err, &code) {
		switch code {
		case Invalid, Network, ServiceRejected, ExpiredSession, Canceled, Timeout:
			return code
		}
	}
	return Network
}
func (m *Manager) Restore() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	if m.state.Phase != Initial && m.state.Phase != SignedOut && !(m.state.Phase == Failed && m.state.Error == Storage) {
		return Invalid
	}
	m.start(Restoring, 0, m.restore)
	return nil
}
func (m *Manager) restore(ctx context.Context, g uint64) {
	material, err := m.store.Load()
	m.mu.Lock()
	if err != nil || material.Empty() {
		m.state.Phase, m.state.Error = Failed, Storage
		if errors.Is(err, Missing) {
			m.state.Phase, m.state.Error = SignedOut, None
			m.state.Stored = false
		}
		m.mu.Unlock()
		return
	}
	m.material = material
	m.restoreMaterial = true
	m.state.Stored = true
	m.state.Unsaved = false
	if m.state.CloseRequested {
		m.mu.Unlock()
		return
	}
	m.state.Phase = Verifying
	m.mu.Unlock()
	m.verify(ctx, g)
}
func (m *Manager) Begin() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	allowed := m.state.Phase == SignedOut || m.state.Phase == Expired || m.state.Phase == Failed && m.state.Error != Storage && !m.state.Stored && !m.state.Unsaved
	if !allowed || m.state.CloseRequested {
		return Invalid
	}
	m.restoreMaterial = false
	m.state.Restored = false
	m.state.QR = QR{}
	m.state.ExpiresAt = m.now().Add(attemptLifetime)
	m.start(Generating, attemptLifetime, m.attempt)
	return nil
}
func (m *Manager) attemptFailed(g uint64, code Code) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.current(g) {
		return
	}
	m.state.QR = QR{}
	m.state.ExpiresAt = time.Time{}
	m.state.Error = code
	m.state.Phase = Failed
	if code == ExpiredSession || code == Timeout || m.state.Stored {
		m.state.Phase = Expired
	}
}
func (m *Manager) attempt(ctx context.Context, g uint64) {
	attempt, err := m.gateway.Begin(ctx)
	if attempt != nil {
		defer attempt.Close()
	}
	if err != nil {
		m.attemptFailed(g, gatewayCode(err))
		return
	}
	if attempt == nil || attempt.QR().Empty() {
		m.attemptFailed(g, Invalid)
		return
	}
	m.mu.Lock()
	if !m.current(g) {
		m.mu.Unlock()
		return
	}
	if err := ctx.Err(); err != nil {
		m.mu.Unlock()
		m.attemptFailed(g, gatewayCode(err))
		return
	}
	m.state.Phase = Waiting
	m.state.QR = attempt.QR()
	m.mu.Unlock()
	for n := 0; n < pollLimit; n++ {
		result, err := attempt.Poll(ctx)
		if err != nil {
			m.attemptFailed(g, gatewayCode(err))
			return
		}
		m.mu.Lock()
		if !m.current(g) {
			m.mu.Unlock()
			return
		}
		if err := ctx.Err(); err != nil {
			m.mu.Unlock()
			m.attemptFailed(g, gatewayCode(err))
			return
		}
		switch result.Status {
		case PollWaiting:
			m.state.Phase = Waiting
		case PollScanned:
			m.state.Phase = Scanned
		case PollExpired:
			m.mu.Unlock()
			m.attemptFailed(g, ExpiredSession)
			return
		case PollComplete:
			if result.Material.Empty() {
				m.mu.Unlock()
				m.attemptFailed(g, Invalid)
				return
			}
			m.material = result.Material
			m.state.Unsaved = true
			m.state.QR = QR{}
			m.state.ExpiresAt = time.Time{}
			m.state.Phase = Saving
			m.mu.Unlock()
			m.save(ctx, g)
			return
		default:
			m.mu.Unlock()
			m.attemptFailed(g, Invalid)
			return
		}
		m.mu.Unlock()
		if n+1 < pollLimit {
			if err := m.wait(ctx, pollInterval); err != nil {
				m.attemptFailed(g, gatewayCode(err))
				return
			}
		}
	}
	m.attemptFailed(g, Timeout)
}
func (m *Manager) save(ctx context.Context, g uint64) {
	m.mu.Lock()
	material := m.material
	m.mu.Unlock()
	err := m.store.Save(material)
	m.mu.Lock()
	if err != nil {
		m.state.Phase, m.state.Error, m.state.Unsaved = SaveFailed, Storage, true
		m.mu.Unlock()
		return
	}
	m.state.Stored, m.state.Unsaved = true, false
	if m.state.CloseRequested {
		m.mu.Unlock()
		return
	}
	m.state.Phase, m.state.Error = Verifying, None
	m.mu.Unlock()
	m.verify(ctx, g)
}
func (m *Manager) verify(ctx context.Context, g uint64) {
	m.mu.Lock()
	material := m.material
	m.mu.Unlock()
	err := m.gateway.Verify(ctx, material)
	if err == nil {
		err = ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.current(g) {
		return
	}
	if err != nil {
		m.state.Error = gatewayCode(err)
		m.state.Phase = VerifyFailed
		if m.state.Error == ExpiredSession {
			m.state.Phase = Expired
		}
		return
	}
	m.state.Phase, m.state.Error = SignedIn, None
	m.state.Restored = m.restoreMaterial
}
func (m *Manager) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == Done {
		return Closed
	}
	if !m.state.Busy {
		return Invalid
	}
	switch m.state.Phase {
	case Generating, Waiting, Scanned:
		m.cancelPhase = SignedOut
		if m.state.Stored {
			m.cancelPhase = Expired
		}
	case Verifying:
		if m.state.Unsaved {
			return Busy
		}
		m.cancelPhase = VerifyFailed
	default:
		return Busy
	}
	m.state.Generation++
	m.state.Phase, m.state.Error = Canceling, Canceled
	m.state.QR = QR{}
	m.state.ExpiresAt = time.Time{}
	m.cancel()
	return nil
}
func (m *Manager) RetrySave() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	if m.state.Phase != SaveFailed || !m.state.Unsaved {
		return Invalid
	}
	m.start(Saving, 0, m.save)
	return nil
}
func (m *Manager) RetryVerify() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	if m.state.Phase != VerifyFailed || !m.state.Stored || m.material.Empty() || m.state.CloseRequested {
		return Invalid
	}
	m.start(Verifying, 0, m.verify)
	return nil
}
func (m *Manager) Logout() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	phase := m.state.Phase
	if !(m.state.Stored && (phase == SignedIn || phase == VerifyFailed || phase == Expired || phase == DeleteFailed)) {
		return Invalid
	}
	m.material = Material{}
	m.state.QR = QR{}
	m.state.Restored = false
	m.start(SigningOut, 0, func(_ context.Context, _ uint64) {
		err := m.store.Delete()
		m.mu.Lock()
		defer m.mu.Unlock()
		if err != nil {
			m.state.Phase, m.state.Error, m.state.Stored = DeleteFailed, Storage, true
			return
		}
		m.state.Phase, m.state.Error, m.state.Stored = SignedOut, None, false
	})
	return nil
}
func (m *Manager) RequestClose(discardUnsaved bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == Done {
		return true
	}
	if discardUnsaved && m.state.Busy {
		return false
	}
	m.state.CloseRequested = true
	m.state.QR = QR{}
	m.state.ExpiresAt = time.Time{}
	if m.state.Busy {
		switch m.state.Phase {
		case Generating, Waiting, Scanned, Verifying:
			m.state.Generation++
			m.state.Phase = Closing
			m.cancel()
		}
		return false
	}
	if !discardUnsaved && (m.state.Unsaved || m.state.Phase == DeleteFailed) {
		return false
	}
	m.state.Unsaved = false
	m.done()
	return true
}
