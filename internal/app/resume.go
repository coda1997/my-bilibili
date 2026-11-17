package app

import (
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type resumeState struct {
	generation                           uint64
	target                               time.Duration
	active, seekSent                     bool
	readyDeadline, submittedAt, deadline time.Time
	err                                  catalog.Code
}

func (s *Session) resumeFailed(code catalog.Code) {
	s.resume.active, s.resume.err = false, code
	s.intent, s.initializing = false, false
}

func (s *Session) advanceReady(state playback.Snapshot) {
	if state.Generation != s.generation || s.closed {
		return
	}
	ready := state.State == playback.Ready || state.State == playback.Paused || state.State == playback.Playing
	if s.resume.active && s.resume.generation == state.Generation && !s.resume.seekSent &&
		!s.now().Before(s.resume.readyDeadline) {
		s.resumeFailed(catalog.Timeout)
		return
	}
	if s.initializing && ready {
		s.initializing = false
		for _, cmd := range []playback.Command{{Kind: playback.Volume, Value: s.volume}, {Kind: playback.Rate, Value: s.rate}} {
			if err := s.command(cmd); err != nil {
				return
			}
		}
		if !s.resume.active && s.intent {
			_ = s.command(playback.Command{Kind: playback.Play})
		}
	}
	if !s.resume.active || s.resume.generation != state.Generation {
		return
	}
	// An initial/replacement engine opens paused. Never accept Playing as seek
	// confirmation, even if its position happens to match a requested target.
	paused := state.State == playback.Ready || state.State == playback.Paused
	if !s.resume.seekSent {
		if !paused || state.Duration <= 0 {
			return
		}
		s.resume.target = min(s.resume.target, max(time.Duration(0), state.Duration-100*time.Millisecond))
		if s.resume.target == 0 {
			s.finishResume()
			return
		}
		if err := s.command(playback.Command{Kind: playback.Seek, Position: s.resume.target}); err != nil {
			s.resumeFailed(safeContentError(err))
			return
		}
		s.resume.seekSent = true
		s.resume.submittedAt = s.now()
		s.resume.deadline = s.resume.submittedAt.Add(5 * time.Second)
		return
	}
	if !s.now().Before(s.resume.deadline) {
		s.resumeFailed(catalog.Timeout)
		return
	}
	// Controller snapshots are sampled at PollInterval. A pre-command snapshot
	// must not confirm an asynchronous native seek.
	if paused && !s.now().Before(s.resume.submittedAt.Add(playback.PollInterval)) &&
		state.Position >= s.resume.target-250*time.Millisecond && state.Position <= s.resume.target+250*time.Millisecond {
		s.finishResume()
	}
}

func (s *Session) finishResume() {
	s.resume.active, s.resume.err = false, catalog.None
	if s.intent {
		_ = s.command(playback.Command{Kind: playback.Play})
	}
}
