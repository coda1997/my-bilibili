package targetprobe

import (
	"context"
	"time"

	"my-bilibili/internal/playback"
)

// Session is confined to the same UI thread as Controller. Deadline handling
// requests complete synchronous disposal there, never native calls on a timer goroutine.
type Session struct {
	controller   *playback.Controller
	policy       *Policy
	now          func() time.Time
	cancel       context.CancelFunc
	terminal     *playback.Snapshot
	controlError playback.ErrorCode
	retryAllowed bool
	attempted    bool
	ctx          context.Context
}
type Status struct {
	playback.Snapshot
	Owned        bool               `json:"owned"`
	ControlError playback.ErrorCode `json:"control_error"`
}

func NewSession(opener playback.Opener, p *Policy, now func() time.Time) *Session {
	if now == nil {
		now = time.Now
	}
	return &Session{controller: playback.NewWithPolicy(opener, now, p), policy: p, now: now}
}
func safe(err error) playback.ErrorCode {
	if err == nil {
		return playback.NoError
	}
	if e, ok := err.(playback.ErrorCode); ok {
		switch e {
		case playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
			return e
		}
	}
	return playback.Media
}
func (s *Session) Open(ctx context.Context) error {
	if s.attempted {
		return playback.IsClosed
	}
	return s.open(ctx)
}
func (s *Session) open(ctx context.Context) error {
	if s == nil || s.policy == nil || ctx == nil {
		return playback.Invalid
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
	s.controlError = playback.NoError
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
	if !s.retryAllowed || s.controller.Snapshot().State == playback.Loading {
		return playback.IsClosed
	}
	// Never retry Media/Timeout/restriction failures. Explicit operator cancel/close
	// and an otherwise successful session may reuse only this identical fresh input.
	if s.terminal != nil && s.terminal.Error != playback.NoError && s.terminal.Error != playback.Canceled {
		return playback.IsClosed
	}
	return s.open(ctx)
}
func (s *Session) finish(state playback.State, code playback.ErrorCode) {
	snap := s.controller.Snapshot()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	_ = s.controller.Close()
	snap.State, snap.Error = state, code
	s.terminal = &snap
	if code != playback.NoError && code != playback.Canceled {
		s.retryAllowed = false
	}
}
func (s *Session) Tick() {
	if s.terminal != nil {
		return
	}
	if s.policy == nil {
		s.finish(playback.Failed, playback.Invalid)
		return
	}
	if !s.now().Before(s.policy.Deadline()) {
		s.finish(playback.Failed, playback.Timeout)
		return
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		code := contextCode(s.ctx)
		state := playback.Closed
		if code == playback.Timeout {
			state = playback.Failed
		}
		s.finish(state, code)
		return
	}
	s.controller.Tick()
	snap := s.controller.Snapshot()
	if snap.State == playback.Failed {
		s.retryAllowed = false
	}
	if s.controller.Active() && snap.Position >= time.Duration(s.policy.d.Grant.EndMS)*time.Millisecond {
		s.finish(playback.Ended, playback.NoError)
	}
}
func (s *Session) Command(cmd playback.Command) error {
	s.Tick()
	if s.terminal != nil || !s.controller.Active() {
		return playback.IsClosed
	}
	if cmd.Validate() != nil {
		return playback.Invalid
	}
	if cmd.Kind == playback.Seek && (cmd.Position < time.Duration(s.policy.d.Grant.StartMS)*time.Millisecond || cmd.Position > time.Duration(s.policy.d.Grant.EndMS)*time.Millisecond) {
		return playback.Invalid
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
func (s *Session) Engine() playback.Engine { return s.controller.Engine() }
func (s *Session) Cancel()                 { s.finish(playback.Closed, playback.Canceled) }
func (s *Session) Close()                  { s.finish(playback.Closed, playback.NoError) }
func (s *Session) Expired() bool           { return !s.now().Before(s.policy.Deadline()) }
