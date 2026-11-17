package app

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/internal/catalog"
)

// Session owns application intent on the same UI thread as its Controller.
// Construction and State never open media or issue playback commands.
type Session struct {
	source     Source
	controller *contract.Controller
	video      catalog.Video
	ctx        context.Context
	started    bool
	closed     bool

	selected, playback  catalog.PartID
	selectionError      contract.ErrorCode
	selectionRetry      catalog.PartID
	selectionIntent     bool
	input               contract.Input
	hasInput            bool
	intent, retryIntent bool
	initializing        bool
	generation          uint64
	volume, rate        float64
	unmutedVolume       float64
}

var _ Player = (*Session)(nil)

func New(source Source, controller *contract.Controller) (*Session, error) {
	if source == nil || controller == nil {
		return nil, contract.Invalid
	}
	video, err := source.Detail()
	if err != nil {
		return nil, safeError(err)
	}
	if strings.TrimSpace(string(video.ID)) == "" || strings.TrimSpace(video.Title) == "" ||
		strings.TrimSpace(video.Creator) == "" || strings.TrimSpace(video.Description) == "" ||
		strings.TrimSpace(video.CoverID) == "" || len(video.Parts) == 0 {
		return nil, contract.Invalid
	}
	seen := make(map[catalog.PartID]bool, len(video.Parts))
	for _, part := range video.Parts {
		if strings.TrimSpace(string(part.ID)) == "" || seen[part.ID] || strings.TrimSpace(part.Title) == "" || part.Duration <= 0 {
			return nil, contract.Invalid
		}
		seen[part.ID] = true
	}
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return &Session{source: source, controller: controller, video: video, volume: 1, rate: 1, unmutedVolume: 1}, nil
}

func safeError(err error) contract.ErrorCode {
	var code contract.ErrorCode
	if errors.As(err, &code) {
		switch code {
		case contract.Invalid, contract.Unsupported, contract.Canceled, contract.Timeout, contract.Media, contract.IsClosed:
			return code
		}
	}
	return contract.Media
}

func (s *Session) State() State {
	video := s.video
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return State{Video: video, SelectedPart: s.selected, PlaybackPart: s.playback,
		SelectionError: s.selectionError, Playback: s.controller.Snapshot(),
		DesiredPlaying: s.intent, Volume: s.volume, Rate: s.rate}
}

// Engine exposes the original engine to composition, never through Player.
func (s *Session) Engine() contract.Engine { return s.controller.Engine() }

func (s *Session) Start(ctx context.Context) error {
	if s.closed {
		return contract.IsClosed
	}
	if ctx == nil {
		return contract.Invalid
	}
	if s.started {
		return nil
	}
	s.started, s.ctx = true, ctx
	return s.selectPart(s.video.Parts[0].ID, false)
}

func (s *Session) active() error {
	if s.closed {
		return contract.IsClosed
	}
	if !s.started {
		return contract.Invalid
	}
	s.Tick()
	if s.closed {
		return contract.IsClosed
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
		return contract.Invalid
	}
	s.selected = id
	if id == s.playback && s.hasInput {
		s.selectionError, s.selectionRetry = contract.NoError, ""
		switch s.controller.Snapshot().State {
		case contract.Loading:
			s.intent, s.retryIntent = true, true
			return nil
		case contract.Playing:
			return nil
		case contract.Ready, contract.Paused:
			return s.play(true)
		case contract.Ended, contract.Failed:
			return s.open(id, s.input, true)
		}
	}
	return s.selectPart(id, true)
}

func (s *Session) selectPart(id catalog.PartID, intent bool) error {
	s.selected = id
	in, err := s.source.Resolve(id)
	if err == nil {
		err = in.Validate()
	}
	if err != nil {
		s.selectionError = safeError(err)
		s.selectionRetry, s.selectionIntent = id, intent
		return s.selectionError
	}
	s.selectionError, s.selectionRetry = contract.NoError, ""
	return s.open(id, in, intent)
}

func (s *Session) open(id catalog.PartID, in contract.Input, intent bool) error {
	in.Headers = maps.Clone(in.Headers)
	before := s.controller.Snapshot().Generation
	err := s.controller.Open(s.ctx, in)
	after := s.controller.Snapshot()
	if after.Generation != before {
		s.playback, s.input, s.hasInput = id, in, true
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
		return contract.Invalid
	}
	return s.selectPart(s.selectionRetry, s.selectionIntent)
}

func (s *Session) RetryPlayback() error {
	if err := s.active(); err != nil {
		return err
	}
	if !s.hasInput {
		return contract.Invalid
	}
	if s.controller.Snapshot().State == contract.Loading {
		return nil
	}
	return s.open(s.playback, s.input, s.retryIntent)
}

func (s *Session) Play() error  { return s.play(true) }
func (s *Session) Pause() error { return s.play(false) }

func (s *Session) play(intent bool) error {
	if err := s.active(); err != nil {
		return err
	}
	state := s.controller.Snapshot().State
	if state == contract.Ended && intent {
		return s.open(s.playback, s.input, true)
	}
	if state != contract.Loading && state != contract.Ready && state != contract.Paused && state != contract.Playing {
		return contract.Invalid
	}
	alreadyRequested := s.intent == intent
	s.intent, s.retryIntent = intent, intent
	// Tick may have just applied the pending intent at first readiness. The
	// sampled state can also lag queued commands, so only unchanged intent
	// can suppress a command once ready.
	if state == contract.Loading || alreadyRequested {
		return nil
	}
	kind := contract.Pause
	if intent {
		kind = contract.Play
	}
	return s.command(contract.Command{Kind: kind})
}

func (s *Session) Toggle() error {
	if err := s.active(); err != nil {
		return err
	}
	if s.controller.Snapshot().State == contract.Ended {
		return s.open(s.playback, s.input, true)
	}
	return s.play(!s.intent)
}

func (s *Session) Seek(position time.Duration) error {
	if err := s.active(); err != nil {
		return err
	}
	state := s.controller.Snapshot()
	if position < 0 || state.Duration <= 0 || (state.State != contract.Ready && state.State != contract.Paused && state.State != contract.Playing) {
		return contract.Invalid
	}
	return s.command(contract.Command{Kind: contract.Seek, Position: position})
}

func (s *Session) SetVolume(volume float64) error {
	return s.setting(contract.Command{Kind: contract.Volume, Value: volume})
}

func (s *Session) SetRate(rate float64) error {
	return s.setting(contract.Command{Kind: contract.Rate, Value: rate})
}

func (s *Session) setting(cmd contract.Command) error {
	if err := s.active(); err != nil {
		return err
	}
	if err := cmd.Validate(); err != nil {
		return err
	}
	if cmd.Kind == contract.Volume {
		s.volume = cmd.Value
		if cmd.Value > 0 {
			s.unmutedVolume = cmd.Value
		}
	} else {
		s.rate = cmd.Value
	}
	state := s.controller.Snapshot().State
	if state == contract.Loading || state == contract.Failed || state == contract.Ended {
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

func (s *Session) command(cmd contract.Command) error {
	if err := s.controller.Command(cmd); err != nil {
		s.intent, s.initializing = false, false
		s.observeTerminal(s.controller.Snapshot())
		return safeError(err)
	}
	return nil
}

func (s *Session) observeTerminal(state contract.Snapshot) {
	switch state.State {
	case contract.Failed:
		s.intent, s.initializing = false, false
	case contract.Ended:
		s.intent, s.retryIntent, s.initializing = false, false, false
	case contract.Closed:
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
	if !s.initializing || state.Generation != s.generation || (state.State != contract.Ready && state.State != contract.Paused && state.State != contract.Playing) {
		return
	}
	s.initializing = false
	for _, cmd := range []contract.Command{{Kind: contract.Volume, Value: s.volume}, {Kind: contract.Rate, Value: s.rate}} {
		if s.command(cmd) != nil {
			return
		}
	}
	if s.intent {
		_ = s.command(contract.Command{Kind: contract.Play})
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
