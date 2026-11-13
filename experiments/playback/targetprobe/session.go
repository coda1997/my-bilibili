package targetprobe

import (
	"context"
	"time"

	"my-bilibili/experiments/playback/contract"
)

// Session is confined to the same UI thread as Controller. Deadline handling
// requests complete synchronous disposal there, never native calls on a timer goroutine.
type Session struct {
	controller   *contract.Controller
	policy       *Policy
	now          func() time.Time
	cancel       context.CancelFunc
	terminal     *contract.Snapshot
	controlError contract.ErrorCode
	retryAllowed bool
	attempted    bool
	ctx          context.Context
}
type Status struct {
	contract.Snapshot
	Owned        bool               `json:"owned"`
	ControlError contract.ErrorCode `json:"control_error"`
}

func NewSession(opener contract.Opener, p *Policy, now func() time.Time) *Session {
	if now == nil {
		now = time.Now
	}
	return &Session{controller: contract.NewWithPolicy(opener, now, p), policy: p, now: now}
}
func safe(err error) contract.ErrorCode {
	if err == nil {
		return contract.NoError
	}
	if e, ok := err.(contract.ErrorCode); ok {
		switch e {
		case contract.Invalid, contract.Unsupported, contract.Canceled, contract.Timeout, contract.Media, contract.IsClosed:
			return e
		}
	}
	return contract.Media
}
func (s *Session) Open(ctx context.Context) error {
	if s.attempted {
		return contract.IsClosed
	}
	return s.open(ctx)
}
func (s *Session) open(ctx context.Context) error {
	if s == nil || s.policy == nil || ctx == nil {
		return contract.Invalid
	}
	// Validation precedes cancellation/disposal of any existing valid input.
	if e := s.policy.Validate(s.policy.Input()); e != nil {
		s.controlError = safe(e)
		return e
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.terminal = nil
	s.controlError = contract.NoError
	deadline := minTime(s.policy.Deadline(), s.now().Add(SessionLimit))
	s.attempted = true
	child, cancel := context.WithDeadline(ctx, deadline)
	s.ctx = child
	s.cancel = cancel
	e := s.controller.Open(child, s.policy.Input())
	s.controlError = safe(e)
	s.retryAllowed = e == nil
	return e
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
func (s *Session) Retry(ctx context.Context) error {
	s.Tick()
	if !s.retryAllowed || s.controller.Snapshot().State == contract.Loading {
		return contract.IsClosed
	}
	// Never retry Media/Timeout/restriction failures. Explicit operator cancel/close
	// and an otherwise successful session may reuse only this identical fresh input.
	if s.terminal != nil && s.terminal.Error != contract.NoError && s.terminal.Error != contract.Canceled {
		return contract.IsClosed
	}
	return s.open(ctx)
}
func (s *Session) finish(state contract.State, code contract.ErrorCode) {
	snap := s.controller.Snapshot()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	_ = s.controller.Close()
	snap.State, snap.Error = state, code
	s.terminal = &snap
	if code != contract.NoError && code != contract.Canceled {
		s.retryAllowed = false
	}
}
func (s *Session) Tick() {
	if s.terminal != nil {
		return
	}
	if s.policy == nil {
		s.finish(contract.Failed, contract.Invalid)
		return
	}
	if !s.now().Before(s.policy.Deadline()) {
		s.finish(contract.Failed, contract.Timeout)
		return
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		code := contextCode(s.ctx)
		state := contract.Closed
		if code == contract.Timeout {
			state = contract.Failed
		}
		s.finish(state, code)
		return
	}
	s.controller.Tick()
	snap := s.controller.Snapshot()
	if snap.State == contract.Failed {
		s.retryAllowed = false
	}
	if s.controller.Active() && snap.Position >= time.Duration(s.policy.d.Grant.EndMS)*time.Millisecond {
		s.finish(contract.Ended, contract.NoError)
	}
}
func (s *Session) Command(cmd contract.Command) error {
	s.Tick()
	if s.terminal != nil || !s.controller.Active() {
		return contract.IsClosed
	}
	if cmd.Validate() != nil {
		return contract.Invalid
	}
	if cmd.Kind == contract.Seek && (cmd.Position < time.Duration(s.policy.d.Grant.StartMS)*time.Millisecond || cmd.Position > time.Duration(s.policy.d.Grant.EndMS)*time.Millisecond) {
		return contract.Invalid
	}
	e := s.controller.Command(cmd)
	s.controlError = safe(e)
	if e != nil {
		s.retryAllowed = false
	}
	return e
}
func (s *Session) Snapshot() Status {
	snap := s.controller.Snapshot()
	if s.terminal != nil {
		snap = *s.terminal
	}
	return Status{snap, s.controller.Active(), s.controlError}
}
func (s *Session) Engine() contract.Engine { return s.controller.Engine() }
func (s *Session) Cancel()                 { s.finish(contract.Closed, contract.Canceled) }
func (s *Session) Close()                  { s.finish(contract.Closed, contract.NoError) }
func (s *Session) Expired() bool           { return !s.now().Before(s.policy.Deadline()) }
