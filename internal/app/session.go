package app

import (
	"context"
	"errors"
	"maps"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

// Session owns application intent on the same UI thread as its Controller.
// The source worker only produces results; Tick accepts them on this thread.
// Construction and State perform no source or playback operations.
type Session struct {
	source      Source
	controller  *playback.Controller
	video       catalog.Video
	detailPhase DetailPhase
	detailError catalog.Code
	ctx         context.Context
	cancel      context.CancelFunc
	started     bool
	closed      bool

	requests         chan sourceRequest
	results          chan sourceResult
	done             chan struct{}
	pending          *sourceAttempt
	sourceGeneration uint64
	requestContext   func(context.Context, time.Duration) (context.Context, context.CancelFunc)

	selected, playback  catalog.PartID
	selectionError      catalog.Code
	selectionRetry      catalog.PartID
	selectionIntent     bool
	playbackRetryError  catalog.Code
	selectedQuality     QualityID
	playbackQuality     QualityID
	qualities           []Quality
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

// New validates dependencies without invoking Source or starting a worker.
// Start supplies the lifetime context for both source work and playback.
func New(source Source, controller *playback.Controller) (*Session, error) {
	if source == nil || controller == nil {
		return nil, playback.Invalid
	}
	return &Session{
		source: source, controller: controller, detailPhase: DetailInitial,
		requests: make(chan sourceRequest, 1), results: make(chan sourceResult, 1), done: make(chan struct{}),
		requestContext: context.WithTimeout, now: time.Now,
		volume: 1, rate: 1, unmutedVolume: 1,
	}, nil
}

func safeError(err error) playback.ErrorCode {
	if errors.Is(err, context.DeadlineExceeded) {
		return playback.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return playback.Canceled
	}
	var code playback.ErrorCode
	if errors.As(err, &code) {
		switch code {
		case playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
			return code
		}
	}
	return playback.Media
}

func safeContentError(err error) catalog.Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return catalog.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return catalog.Canceled
	}
	var content catalog.Code
	if errors.As(err, &content) {
		switch content {
		case catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network,
			catalog.InvalidResponse, catalog.Unavailable, catalog.Restricted, catalog.LoginRequired,
			catalog.SessionExpired, catalog.VerificationRequired, catalog.RateLimited,
			catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
			return content
		}
	}
	var native playback.ErrorCode
	if errors.As(err, &native) {
		switch native {
		case playback.Invalid:
			return catalog.Invalid
		case playback.Unsupported:
			return catalog.Unsupported
		case playback.Canceled:
			return catalog.Canceled
		case playback.Timeout:
			return catalog.Timeout
		case playback.Media:
			return catalog.Unavailable
		case playback.IsClosed:
			return catalog.Closed
		}
	}
	return catalog.Unavailable
}

func (s *Session) State() State {
	state := State{DetailPhase: s.detailPhase, DetailError: s.detailError, Video: cloneVideo(s.video),
		SelectedPart: s.selected, PlaybackPart: s.playback, SelectionError: s.selectionError,
		PlaybackRetryError: s.playbackRetryError, Playback: s.controller.Snapshot(),
		SelectedQuality: s.selectedQuality, PlaybackQuality: s.playbackQuality, Qualities: cloneQualities(s.qualities),
		DesiredPlaying: s.intent, Volume: s.volume, Rate: s.rate}
	if s.pending != nil && s.pending.request.purpose != detailRequest {
		state.ResolvingPart, state.DesiredPlaying = s.pending.request.part, s.pending.intent
	}
	if s.closed {
		select {
		case <-s.done:
		default:
			state.Closing = true
		}
	}
	return state
}

// Engine exposes the original engine to composition, never through Player.
func (s *Session) Engine() playback.Engine { return s.controller.Engine() }

// Done signals source-worker termination. Receiving from it is safe off the
// UI thread; it neither disposes playback nor requests window destruction.
func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Start(ctx context.Context) error {
	if s.closed {
		return playback.IsClosed
	}
	if ctx == nil {
		return playback.Invalid
	}
	if s.started {
		return s.active()
	}
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.loadDetail()
	go runSource(s.ctx, s.source, s.requests, s.results, s.done)
	return nil
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

func (s *Session) detailReady() error {
	if err := s.active(); err != nil {
		return err
	}
	if s.detailPhase != DetailReady {
		return playback.Invalid
	}
	return nil
}

func (s *Session) RetryDetail() error {
	if err := s.active(); err != nil {
		return err
	}
	if s.detailPhase != DetailFailed {
		return playback.Invalid
	}
	s.loadDetail()
	return nil
}

func (s *Session) loadDetail() {
	s.detailPhase, s.detailError = DetailLoading, catalog.None
	s.submit(detailRequest, "", false)
}

// retireRequest also removes queued work. The worker keeps ownership of an
// in-flight call until it returns, even if cancellation is not cooperative.
func (s *Session) retireRequest() {
	s.sourceGeneration++
	if s.pending != nil {
		s.pending.cancel()
		s.pending = nil
	}
	select {
	case <-s.requests:
	default:
	}
}

func (s *Session) submit(purpose sourcePurpose, part catalog.PartID, intent bool) {
	s.retireRequest()
	ctx, cancel := s.requestContext(s.ctx, sourceTimeout)
	request := sourceRequest{generation: s.sourceGeneration, purpose: purpose, part: part, ctx: ctx}
	s.pending = &sourceAttempt{request: request, cancel: cancel, intent: intent}
	// There is one UI-thread producer. retireRequest emptied this capacity-one
	// mailbox, and the worker can only remove from it. Submission cannot wait.
	s.requests <- request
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
	if err := s.detailReady(); err != nil {
		return err
	}
	if s.partIndex(id) < 0 {
		return playback.Invalid
	}
	s.selected = id
	s.selectionError, s.selectionRetry = catalog.None, ""
	if s.pending != nil && s.pending.request.part == id {
		s.pending.intent = true
		return nil
	}
	// Invalidate B before any accepted-A fast path, including a no-op/resume
	// which does not advance the native playback generation.
	s.retireRequest()
	if id == s.playback && s.hasInput {
		s.selectedQuality = s.playbackQuality
		switch s.controller.Snapshot().State {
		case playback.Loading, playback.Playing, playback.Ready, playback.Paused:
			return s.playAccepted(true)
		case playback.Ended, playback.Failed:
			return s.reopen(true)
		}
	}
	s.selectPart(id, true)
	return nil
}

func (s *Session) selectPart(id catalog.PartID, intent bool) {
	s.selected = id
	s.selectedQuality = ""
	s.selectionError, s.selectionRetry = catalog.None, ""
	s.submit(selectionRequest, id, intent)
}

// usable reports whether an input with this expiry may still be opened.
func (s *Session) usable(expiresAt time.Time) bool {
	return expiresAt.IsZero() || s.now().Before(expiresAt.Add(-ResolutionMargin))
}

// reopen targets accepted playback. A different selected part's failure and
// retry intent remain intact, including while an expired input is refreshed.
func (s *Session) reopen(intent bool) error {
	s.playbackRetryError = catalog.None
	if s.playback == s.selected {
		s.selectionError, s.selectionRetry = catalog.None, ""
	}
	if s.usable(s.expiresAt) {
		err := s.open(s.playback, s.input, s.expiresAt, intent)
		if err != nil && !s.closed && s.playback != s.selected {
			s.playbackRetryError = safeContentError(err)
		}
		return err
	}
	s.submit(playbackRequest, s.playback, intent)
	return nil
}

func (s *Session) open(id catalog.PartID, in playback.Input, expiresAt time.Time, intent bool) error {
	if s.closed {
		return playback.IsClosed
	}
	if err := sourceContextError(s.ctx); err != nil {
		s.cancelSession(safeError(err))
		return safeError(err)
	}
	if !s.usable(expiresAt) {
		return playback.Media
	}
	in.Headers = maps.Clone(in.Headers)
	before := s.controller.Snapshot().Generation
	// The completed request's context is canceled by result consumption. The
	// native engine must instead inherit the whole session lifetime.
	err := s.controller.Open(s.ctx, in)
	after := s.controller.Snapshot()
	if after.Generation != before {
		s.playback, s.input, s.expiresAt, s.hasInput = id, in, expiresAt, true
		s.intent, s.retryIntent = intent, intent
		s.generation, s.initializing = after.Generation, true
		s.playbackRetryError = catalog.None
		s.observeTerminal(after)
	}
	if err != nil {
		return safeError(err)
	}
	return nil
}

func (s *Session) NextPart() error {
	if err := s.detailReady(); err != nil {
		return err
	}
	i := s.partIndex(s.selected)
	if i == len(s.video.Parts)-1 {
		return nil
	}
	return s.SelectPart(s.video.Parts[i+1].ID)
}

func (s *Session) RetrySelection() error {
	if err := s.detailReady(); err != nil {
		return err
	}
	if s.pending != nil {
		if s.pending.request.part == s.selected && s.selectionRetry == "" {
			return nil
		}
		return playback.Invalid
	}
	if s.selectionRetry == "" {
		return playback.Invalid
	}
	s.selectPart(s.selectionRetry, s.selectionIntent)
	return nil
}

func (s *Session) RetryPlayback() error {
	if err := s.detailReady(); err != nil {
		return err
	}
	if !s.hasInput {
		return playback.Invalid
	}
	if s.pending != nil {
		if s.pending.request.part == s.playback {
			return nil
		}
		return playback.Invalid
	}
	s.playbackRetryError = catalog.None
	if s.controller.Snapshot().State == playback.Loading {
		return nil
	}
	return s.reopen(s.retryIntent)
}

func (s *Session) Play() error  { return s.play(true) }
func (s *Session) Pause() error { return s.play(false) }

func (s *Session) play(intent bool) error {
	if err := s.detailReady(); err != nil {
		return err
	}
	return s.setPlaying(intent)
}

func (s *Session) setPlaying(intent bool) error {
	state := s.controller.Snapshot().State
	if s.pending != nil {
		s.pending.intent = intent
		// Do not replay an old ended/failed engine over a pending target, or
		// issue commands before the first native engine has been opened.
		if state != playback.Loading && state != playback.Ready && state != playback.Paused && state != playback.Playing {
			return nil
		}
	} else if state == playback.Ended && intent {
		return s.reopen(true)
	}
	return s.playAccepted(intent)
}

func (s *Session) playAccepted(intent bool) error {
	state := s.controller.Snapshot().State
	if state != playback.Loading && state != playback.Ready && state != playback.Paused && state != playback.Playing {
		return playback.Invalid
	}
	alreadyRequested := s.intent == intent
	s.intent, s.retryIntent = intent, intent
	// Native snapshots can lag commands. Suppression follows accepted intent,
	// never the separate intent of the part which is still resolving.
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
	if err := s.detailReady(); err != nil {
		return err
	}
	if s.pending != nil {
		return s.setPlaying(!s.pending.intent)
	}
	if s.controller.Snapshot().State == playback.Ended {
		return s.reopen(true)
	}
	return s.setPlaying(!s.intent)
}

func (s *Session) Seek(position time.Duration) error {
	if err := s.detailReady(); err != nil {
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
	if state != playback.Ready && state != playback.Paused && state != playback.Playing {
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
		s.stopSource()
		if s.controller.Engine() != nil {
			_ = s.controller.Close()
		}
	}
}

// failAttempt attributes failure to the requested target, not the old native
// engine. A playback refresh must not erase a different selection's failure.
func (s *Session) failAttempt(attempt *sourceAttempt, code catalog.Code) {
	switch attempt.request.purpose {
	case detailRequest:
		s.detailPhase, s.detailError = DetailFailed, code
	case playbackRequest:
		if attempt.request.part != s.selected {
			s.playbackRetryError = code
			return
		}
		fallthrough
	case selectionRequest:
		s.selectionError = code
		s.selectionRetry, s.selectionIntent = attempt.request.part, attempt.intent
	}
}

func (s *Session) matches(request sourceRequest) bool {
	return !s.closed && sourceContextError(s.ctx) == nil && s.pending != nil &&
		s.pending.request.generation == request.generation &&
		s.pending.request.purpose == request.purpose && s.pending.request.part == request.part
}

func (s *Session) accept(result sourceResult) {
	defer close(result.consumed)
	if !s.matches(result.request) {
		return
	}
	attempt := s.pending
	err := result.err
	if requestErr := sourceContextError(attempt.request.ctx); requestErr != nil {
		err = requestErr
	}
	if err == nil {
		if result.request.purpose == detailRequest {
			err = catalog.ValidateVideo(result.video)
		} else {
			err = result.resolution.Input.Validate()
			if err == nil {
				err = validateQualities(result.resolution)
			}
			if err == nil && !s.usable(result.resolution.ExpiresAt) {
				err = playback.Media
			}
		}
	}
	if !s.matches(result.request) {
		return
	}
	if requestErr := sourceContextError(attempt.request.ctx); requestErr != nil {
		err = requestErr
	}
	if err != nil {
		s.failAttempt(attempt, safeContentError(err))
		s.retireRequest()
		return
	}
	s.retireRequest()
	if result.request.purpose == detailRequest {
		s.video = cloneVideo(result.video)
		s.detailPhase, s.detailError = DetailReady, catalog.None
		s.selectPart(s.video.Parts[0].ID, false)
		return
	}
	before := s.controller.Snapshot().Generation
	err = s.open(result.request.part, result.resolution.Input, result.resolution.ExpiresAt, attempt.intent)
	if s.controller.Snapshot().Generation != before {
		s.playbackQuality = result.resolution.Quality.ID
		s.qualities = cloneQualities(result.resolution.Qualities)
		if s.selected == s.playback {
			s.selectedQuality = s.playbackQuality
		}
	}
	if err != nil && !s.closed && (result.request.purpose == playbackRequest || s.controller.Snapshot().Generation == before) {
		s.failAttempt(attempt, safeContentError(err))
	}
}

func (s *Session) Tick() {
	if s.closed || !s.started {
		return
	}
	s.controller.Tick()
	if err := sourceContextError(s.ctx); err != nil {
		s.cancelSession(safeError(err))
		return
	}
	s.observeTerminal(s.controller.Snapshot())
	if s.closed {
		return
	}
	// Logical timeout is observable even while a source ignores cancellation.
	// Its worker remains owned and prevents another concurrent source call.
	if s.pending != nil {
		if err := sourceContextError(s.pending.request.ctx); err != nil {
			s.failAttempt(s.pending, safeContentError(err))
			s.retireRequest()
		}
	}
	select {
	case result := <-s.results:
		s.accept(result)
	default:
	}
	state := s.controller.Snapshot()
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

func (s *Session) stopSource() {
	if s.closed {
		return
	}
	s.closed = true
	s.intent, s.retryIntent, s.initializing = false, false, false
	s.selectionRetry = ""
	s.retireRequest()
	if s.cancel != nil {
		s.cancel()
	}
	if !s.started {
		close(s.done)
	}
}

func (s *Session) cancelSession(code playback.ErrorCode) {
	if s.pending != nil {
		s.failAttempt(s.pending, safeContentError(code))
	}
	s.stopSource()
	// Also observe a deadline whose timer had not fired at the earlier Tick.
	// stopSource has now canceled the native engine's session context.
	s.controller.Tick()
	// Preserve its Closed/Canceled observation rather than replacing it with
	// Closed/NoError. No-engine cancellation still closes without an attempt.
	if s.controller.Snapshot().State != playback.Closed || s.controller.Engine() != nil {
		_ = s.controller.Close()
	}
}

func (s *Session) Close() error {
	if s.closed {
		return nil
	}
	// Observe native cancellation without accepting a source result just as
	// the user closes. Source completion is joined separately through Done.
	s.controller.Tick()
	if s.started {
		if err := sourceContextError(s.ctx); err != nil {
			s.cancelSession(safeError(err))
			return nil
		}
	}
	s.stopSource()
	if s.controller.Snapshot().State == playback.Closed && s.controller.Engine() == nil {
		return nil
	}
	return s.controller.Close()
}
