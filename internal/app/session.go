package app

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

// Session owns application intent on the same UI thread as its Controller.
// Construction and State never open media or issue playback commands.
type Session struct {
	source     Source
	controller *playback.Controller
	video      catalog.Video
	ctx        context.Context
	started    bool
	closed     bool

	selected, playback  catalog.PartID
	selectionError      playback.ErrorCode
	selectionRetry      catalog.PartID
	selectionIntent     bool
	input               playback.Input
	expiresAt           time.Time
	hasInput            bool
	intent, retryIntent bool
	initializing        bool
	generation          uint64
	volume, rate        float64
	unmutedVolume       float64
	now                 func() time.Time
}

var _ Player = (*Session)(nil)

// New reads the video detail once with ctx. ctx bounds only that call; Start
// supplies the context for later resolution and playback.
func New(ctx context.Context, source Source, controller *playback.Controller) (*Session, error) {
	if ctx == nil || source == nil || controller == nil {
		return nil, playback.Invalid
	}
	video, err := source.Detail(ctx)
	if err != nil {
		return nil, safeError(err)
	}
	if strings.TrimSpace(string(video.ID)) == "" || strings.TrimSpace(video.Title) == "" ||
		strings.TrimSpace(video.Creator) == "" || strings.TrimSpace(video.Description) == "" ||
		strings.TrimSpace(video.CoverID) == "" || len(video.Parts) == 0 {
		return nil, playback.Invalid
	}
	seen := make(map[catalog.PartID]bool, len(video.Parts))
	for _, part := range video.Parts {
		if strings.TrimSpace(string(part.ID)) == "" || seen[part.ID] || strings.TrimSpace(part.Title) == "" || part.Duration <= 0 {
			return nil, playback.Invalid
		}
		seen[part.ID] = true
	}
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return &Session{source: source, controller: controller, video: video, volume: 1, rate: 1, unmutedVolume: 1, now: time.Now}, nil
}

func safeError(err error) playback.ErrorCode {
	var code playback.ErrorCode
	if errors.As(err, &code) {
		switch code {
		case playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
			return code
		}
	}
	return playback.Media
}

func (s *Session) State() State {
	video := s.video
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return State{Video: video, SelectedPart: s.selected, PlaybackPart: s.playback,
		SelectionError: s.selectionError, Playback: s.controller.Snapshot(),
		DesiredPlaying: s.intent, Volume: s.volume, Rate: s.rate}
}

// Engine exposes the original engine to composition, never through Player.
func (s *Session) Engine() playback.Engine { return s.controller.Engine() }

func (s *Session) Start(ctx context.Context) error {
	if s.closed {
		return playback.IsClosed
	}
	if ctx == nil {
		return playback.Invalid
	}
	if s.started {
		return nil
	}
	s.started, s.ctx = true, ctx
	return s.selectPart(s.video.Parts[0].ID, false)
}

func (s *Session) active() error {
	if s.closed {
		return playback.IsClosed
	}
	if !s.started {
		return playback.Invalid
	}
	s.Tick()
	if s.closed {
		return playback.IsClosed
	}
	return nil
}

func (s *Session) partIndex(id catalog.PartID) int {
	for i, part := range s.video.Parts {
		if part.ID == id {
			return i
		}
	}
	return -1
}

func (s *Session) SelectPart(id catalog.PartID) error {
	if err := s.active(); err != nil {
		return err
	}
	if s.partIndex(id) < 0 {
		return playback.Invalid
	}
	s.selected = id
	if id == s.playback && s.hasInput {
		s.selectionError, s.selectionRetry = playback.NoError, ""
		switch s.controller.Snapshot().State {
		case playback.Loading:
			s.intent, s.retryIntent = true, true
			return nil
		case playback.Playing:
			return nil
		case playback.Ready, playback.Paused:
			return s.play(true)
		case playback.Ended, playback.Failed:
			return s.reopen(true)
		}
	}
	return s.selectPart(id, true)
}

func (s *Session) selectPart(id catalog.PartID, intent bool) error {
	s.selected = id
	return s.resolveAndOpen(id, intent)
}

// resolveAndOpen resolves id and opens it. A failure is recorded as a
// selection error that RetrySelection repeats; the current selection is kept.
func (s *Session) resolveAndOpen(id catalog.PartID, intent bool) error {
	r, err := s.resolve(id)
	if err != nil {
		s.selectionError = safeError(err)
		s.selectionRetry, s.selectionIntent = id, intent
		return s.selectionError
	}
	s.selectionError, s.selectionRetry = playback.NoError, ""
	return s.open(id, r.Input, r.ExpiresAt, intent)
}

// resolve returns a validated resolution that is usable now.
func (s *Session) resolve(id catalog.PartID) (Resolution, error) {
	r, err := s.source.Resolve(s.ctx, id)
	if err == nil {
		err = r.Input.Validate()
	}
	if err == nil && !s.usable(r.ExpiresAt) {
		err = playback.Media
	}
	return r, err
}

// usable reports whether an input with this expiry may still be opened.
func (s *Session) usable(expiresAt time.Time) bool {
	return expiresAt.IsZero() || s.now().Before(expiresAt.Add(-ResolutionMargin))
}

// reopen restarts the accepted playback part, resolving it again when the
// cached input has expired. Selection state belongs to the selected part: when
// a different part is selected, its pending error and retry are left intact
// and a failed re-resolution is returned without becoming a selection retry.
func (s *Session) reopen(intent bool) error {
	if s.usable(s.expiresAt) {
		return s.open(s.playback, s.input, s.expiresAt, intent)
	}
	if s.playback == s.selected {
		return s.resolveAndOpen(s.playback, intent)
	}
	r, err := s.resolve(s.playback)
	if err != nil {
		return safeError(err)
	}
	return s.open(s.playback, r.Input, r.ExpiresAt, intent)
}

func (s *Session) open(id catalog.PartID, in playback.Input, expiresAt time.Time, intent bool) error {
	in.Headers = maps.Clone(in.Headers)
	before := s.controller.Snapshot().Generation
	err := s.controller.Open(s.ctx, in)
	after := s.controller.Snapshot()
	if after.Generation != before {
		s.playback, s.input, s.expiresAt, s.hasInput = id, in, expiresAt, true
		s.intent, s.retryIntent = intent, intent
		s.generation, s.initializing = after.Generation, true
		s.observeTerminal(after)
	}
	if err != nil {
		return safeError(err)
	}
	return nil
}

func (s *Session) NextPart() error {
	if err := s.active(); err != nil {
		return err
	}
	i := s.partIndex(s.selected)
	if i == len(s.video.Parts)-1 {
		return nil
	}
	return s.SelectPart(s.video.Parts[i+1].ID)
}

func (s *Session) RetrySelection() error {
	if err := s.active(); err != nil {
		return err
	}
	if s.selectionRetry == "" {
		return playback.Invalid
	}
	return s.selectPart(s.selectionRetry, s.selectionIntent)
}

func (s *Session) RetryPlayback() error {
	if err := s.active(); err != nil {
		return err
	}
	if !s.hasInput {
		return playback.Invalid
	}
	if s.controller.Snapshot().State == playback.Loading {
		return nil
	}
	return s.reopen(s.retryIntent)
}

func (s *Session) Play() error  { return s.play(true) }
func (s *Session) Pause() error { return s.play(false) }

func (s *Session) play(intent bool) error {
	if err := s.active(); err != nil {
		return err
	}
	state := s.controller.Snapshot().State
	if state == playback.Ended && intent {
		return s.reopen(true)
	}
	if state != playback.Loading && state != playback.Ready && state != playback.Paused && state != playback.Playing {
		return playback.Invalid
	}
	alreadyRequested := s.intent == intent
	s.intent, s.retryIntent = intent, intent
	// Tick may have just applied the pending intent at first readiness. The
	// sampled state can also lag queued commands, so only unchanged intent
	// can suppress a command once ready.
	if state == playback.Loading || alreadyRequested {
		return nil
	}
	kind := playback.Pause
	if intent {
		kind = playback.Play
	}
	return s.command(playback.Command{Kind: kind})
}

func (s *Session) Toggle() error {
	if err := s.active(); err != nil {
		return err
	}
	if s.controller.Snapshot().State == playback.Ended {
		return s.reopen(true)
	}
	return s.play(!s.intent)
}

func (s *Session) Seek(position time.Duration) error {
	if err := s.active(); err != nil {
		return err
	}
	state := s.controller.Snapshot()
	if position < 0 || state.Duration <= 0 || (state.State != playback.Ready && state.State != playback.Paused && state.State != playback.Playing) {
		return playback.Invalid
	}
	return s.command(playback.Command{Kind: playback.Seek, Position: position})
}

func (s *Session) SetVolume(volume float64) error {
	return s.setting(playback.Command{Kind: playback.Volume, Value: volume})
}

func (s *Session) SetRate(rate float64) error {
	return s.setting(playback.Command{Kind: playback.Rate, Value: rate})
}

func (s *Session) setting(cmd playback.Command) error {
	if err := s.active(); err != nil {
		return err
	}
	if err := cmd.Validate(); err != nil {
		return err
	}
	if cmd.Kind == playback.Volume {
		s.volume = cmd.Value
		if cmd.Value > 0 {
			s.unmutedVolume = cmd.Value
		}
	} else {
		s.rate = cmd.Value
	}
	state := s.controller.Snapshot().State
	if state == playback.Loading || state == playback.Failed || state == playback.Ended {
		return nil
	}
	return s.command(cmd)
}

func (s *Session) ToggleMute() error {
	volume := 0.0
	if s.volume == 0 {
		volume = s.unmutedVolume
	}
	return s.SetVolume(volume)
}

func (s *Session) command(cmd playback.Command) error {
	if err := s.controller.Command(cmd); err != nil {
		s.intent, s.initializing = false, false
		s.observeTerminal(s.controller.Snapshot())
		return safeError(err)
	}
	return nil
}

func (s *Session) observeTerminal(state playback.Snapshot) {
	switch state.State {
	case playback.Failed:
		s.intent, s.initializing = false, false
	case playback.Ended:
		s.intent, s.retryIntent, s.initializing = false, false, false
	case playback.Closed:
		s.intent, s.retryIntent, s.initializing, s.closed = false, false, false, true
	}
}

func (s *Session) Tick() {
	if s.closed || !s.started {
		return
	}
	s.controller.Tick()
	state := s.controller.Snapshot()
	s.observeTerminal(state)
	if !s.initializing || state.Generation != s.generation || (state.State != playback.Ready && state.State != playback.Paused && state.State != playback.Playing) {
		return
	}
	s.initializing = false
	for _, cmd := range []playback.Command{{Kind: playback.Volume, Value: s.volume}, {Kind: playback.Rate, Value: s.rate}} {
		if s.command(cmd) != nil {
			return
		}
	}
	if s.intent {
		_ = s.command(playback.Command{Kind: playback.Play})
	}
}

func (s *Session) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.intent, s.retryIntent, s.initializing = false, false, false
	s.selectionRetry = ""
	return s.controller.Close()
}
