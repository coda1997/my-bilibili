package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

// The test chooses each call's completion order. Uncooperative calls ignore
// request cancellation, but the test's abort channel always owns their cleanup.
type controlledSource struct {
	calls        chan *sourceCall
	abort        chan struct{}
	ignoreCancel bool
	active, peak atomic.Int32
}

type sourceCall struct {
	ctx      context.Context
	part     catalog.PartID
	quality  QualityID
	reply    chan sourceReply
	returned chan struct{}
}

type sourceReply struct {
	video      catalog.Video
	resolution Resolution
	err        error
}

func (s *controlledSource) call(ctx context.Context, part catalog.PartID, quality QualityID) sourceReply {
	active := s.active.Add(1)
	for old := s.peak.Load(); active > old && !s.peak.CompareAndSwap(old, active); old = s.peak.Load() {
	}
	defer s.active.Add(-1)
	call := &sourceCall{ctx: ctx, part: part, quality: quality, reply: make(chan sourceReply, 1), returned: make(chan struct{})}
	defer close(call.returned)
	select {
	case s.calls <- call:
	case <-s.abort:
		return sourceReply{err: context.Canceled}
	}
	done := ctx.Done()
	if s.ignoreCancel {
		done = nil
	}
	select {
	case reply := <-call.reply:
		return reply
	case <-done:
		return sourceReply{err: ctx.Err()}
	case <-s.abort:
		return sourceReply{err: context.Canceled}
	}
}

func (s *controlledSource) Detail(ctx context.Context) (catalog.Video, error) {
	r := s.call(ctx, "", "")
	return r.video, r.err
}

func (s *controlledSource) Resolve(ctx context.Context, request ResolveRequest) (Resolution, error) {
	r := s.call(ctx, request.Part, request.Quality)
	return r.resolution, r.err
}

func (s *controlledSource) next(t *testing.T, part catalog.PartID) *sourceCall {
	t.Helper()
	select {
	case call := <-s.calls:
		if call.part != part {
			t.Fatalf("source call=%q, want %q", call.part, part)
		}
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("controlled source was not called")
		return nil
	}
}

func (s *controlledSource) noCall(t *testing.T) {
	t.Helper()
	select {
	case call := <-s.calls:
		t.Fatalf("unexpected source call %q", call.part)
	default:
	}
}

func controlledHarness(t *testing.T, ignoreCancel bool) (*harness, *controlledSource) {
	t.Helper()
	h := newHarness(t)
	source := &controlledSource{calls: make(chan *sourceCall, 256), abort: make(chan struct{}), ignoreCancel: ignoreCancel}
	h.s.source = source
	t.Cleanup(func() { close(source.abort) })
	return h, source
}

func initialResolve(t *testing.T, h *harness, source *controlledSource) *sourceCall {
	t.Helper()
	requireError(t, h.s.Start(context.Background()), nil)
	detail := source.next(t, "")
	detail.reply <- sourceReply{video: cloneVideo(h.source.video)}
	resultReady(t, h.s)
	h.s.Tick()
	return source.next(t, "p1")
}

func completeResolution(t *testing.T, h *harness, call *sourceCall, expires time.Time) {
	t.Helper()
	call.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/fixture-" + string(call.part) + ".mp4"}, expires)}
	resultReady(t, h.s)
	h.s.Tick()
}

func TestDetailAndInitialResolutionAreAsynchronous(t *testing.T) {
	h, source := controlledHarness(t, false)
	source.noCall(t)
	requireError(t, h.s.Start(context.Background()), nil)
	detail := source.next(t, "")
	state := h.s.State()
	if state.DetailPhase != DetailLoading || state.Video.Parts == nil || len(state.Video.Parts) != 0 || state.SelectedPart != "" || state.Playback.Generation != 0 || state.Playback.State != playback.Idle {
		t.Fatal("incorrect pre-detail state", state)
	}
	for _, action := range []func() error{h.s.Play, h.s.Pause, h.s.Toggle, h.s.NextPart, h.s.RetryDetail, h.s.RetrySelection, h.s.RetryPlayback, func() error { return h.s.SelectPart("p1") }, func() error { return h.s.Seek(0) }} {
		requireError(t, action(), playback.Invalid)
	}
	requireError(t, h.s.SetVolume(.4), nil)
	requireError(t, h.s.SetRate(1.5), nil)
	h.s.Tick()
	if len(h.opener.inputs) != 0 || h.s.State().Volume != .4 {
		t.Fatal("loading detail blocked settings or opened media")
	}
	video := cloneVideo(h.source.video)
	detail.reply <- sourceReply{video: video}
	resultReady(t, h.s)
	video.Parts[0].Title = "after-transfer mutation"
	if h.s.State().DetailPhase != DetailLoading || len(h.opener.inputs) != 0 {
		t.Fatal("worker published application state without Tick")
	}
	h.s.Tick()
	resolve := source.next(t, "p1")
	state = h.s.State()
	if state.DetailPhase != DetailReady || state.Video.Parts[0].Title != "One" || state.SelectedPart != "p1" || state.ResolvingPart != "p1" || state.PlaybackPart != "" || state.DesiredPlaying || state.Playback.Generation != 0 {
		t.Fatal("incorrect first-resolution state or detail alias", state)
	}
	requireError(t, h.s.Play(), nil)
	requireError(t, h.s.Pause(), nil)
	requireError(t, h.s.Toggle(), nil)
	requireError(t, h.s.SetVolume(.6), nil)
	requireError(t, h.s.SetRate(1.75), nil)
	if len(h.opener.inputs) != 0 || !h.s.State().DesiredPlaying {
		t.Fatal("pending transport intent required a native engine")
	}
	input := playback.Input{URL: "http://127.0.0.1:1/private-media", Headers: map[string]string{"X-Test": "owned-header"}}
	resolve.reply <- sourceReply{resolution: testResolution(input, time.Time{})}
	resultReady(t, h.s)
	input.Headers["X-Test"] = "after-transfer mutation"
	if h.s.State().PlaybackPart != "" || len(h.opener.inputs) != 0 {
		t.Fatal("worker opened media without Tick")
	}
	h.s.Tick()
	if len(h.opener.inputs) != 1 || h.opener.inputs[0].Headers["X-Test"] != "owned-header" || h.s.State().ResolvingPart != "" {
		t.Fatal("resolution ownership or acceptance")
	}
	if resolve.ctx.Err() != context.Canceled || h.opener.contexts[0].Err() != nil {
		t.Fatal("completed request timer was retained or canceled playback")
	}
	for range 3 {
		_ = h.s.State()
		h.s.Tick()
	}
	if len(h.opener.inputs) != 1 {
		t.Fatal("completed result was accepted repeatedly")
	}
	h.tick(playback.Ready)
	if countKind(h.engine(), playback.Play) != 1 || h.engine().snapshot.Volume != .6 || h.engine().snapshot.Rate != 1.75 {
		t.Fatal("pending controls were lost", h.engine().commands)
	}
	encoded := fmt.Sprint(h.s.State())
	for _, private := range []string{input.URL, "owned-header", "after-transfer mutation"} {
		if strings.Contains(encoded, private) {
			t.Fatal("state exposed source-owned input")
		}
	}
	// Opener-owned headers must not alias the cached input used for replay.
	h.opener.inputs[0].Headers["X-Test"] = "opener mutation"
	h.tick(playback.Failed)
	requireError(t, h.s.RetryPlayback(), nil)
	if h.opener.inputs[1].Headers["X-Test"] != "owned-header" {
		t.Fatal("native opener mutated cached headers")
	}
}

func TestDetailRetryAndSafeSourceErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code catalog.Code
	}{
		{"private", errors.New("private address and cookie"), catalog.Unavailable},
		{"deadline", fmt.Errorf("private: %w", context.DeadlineExceeded), catalog.Timeout},
		{"canceled", fmt.Errorf("private: %w", context.Canceled), catalog.Canceled},
		{"unknown-code", playback.ErrorCode("private native message"), catalog.Unavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, source := controlledHarness(t, false)
			requireError(t, h.s.RetryDetail(), playback.Invalid)
			requireError(t, h.s.Start(context.Background()), nil)
			source.next(t, "").reply <- sourceReply{err: test.err}
			resultReady(t, h.s)
			h.s.Tick()
			state := h.s.State()
			if state.DetailPhase != DetailFailed || state.DetailError != test.code || len(state.Video.Parts) != 0 || state.Video.Parts == nil || state.SelectedPart != "" || len(h.opener.inputs) != 0 || strings.Contains(fmt.Sprint(state), "private") {
				t.Fatal("detail failure was hidden or leaked metadata", state)
			}
			requireError(t, h.s.RetryDetail(), nil)
			if state = h.s.State(); state.DetailPhase != DetailLoading || state.DetailError != catalog.None {
				t.Fatal("detail retry was not visible", state)
			}
			requireError(t, h.s.RetryDetail(), playback.Invalid)
			source.next(t, "").reply <- sourceReply{video: cloneVideo(h.source.video)}
			resultReady(t, h.s)
			h.s.Tick()
			completeResolution(t, h, source.next(t, "p1"), time.Time{})
			h.tick(playback.Ready)
			if h.s.State().DesiredPlaying || countKind(h.engine(), playback.Play) != 0 {
				t.Fatal("detail retry lost paused startup")
			}
			requireError(t, h.s.RetryDetail(), playback.Invalid)
		})
	}
}

func TestSupersededSourceSuccessAndFailureCannotOpen(t *testing.T) {
	for _, staleError := range []error{nil, errors.New("private stale failure")} {
		t.Run(fmt.Sprint(staleError != nil), func(t *testing.T) {
			h, source := controlledHarness(t, true)
			old := initialResolve(t, h, source)
			requireError(t, h.s.SelectPart("p2"), nil)
			current := h.s.pending
			requireError(t, h.s.SelectPart("unknown"), playback.Invalid)
			if h.s.pending != current || current.request.ctx.Err() != nil || old.ctx.Err() != context.Canceled {
				t.Fatal("invalid selection retired valid work or supersession did not cancel")
			}
			old.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/obsolete.mp4"}, time.Time{}), err: staleError}
			resultReady(t, h.s)
			h.s.Tick()
			if state := h.s.State(); len(h.opener.inputs) != 0 || state.SelectedPart != "p2" || state.ResolvingPart != "p2" || state.SelectionError != catalog.None || !state.DesiredPlaying {
				t.Fatal("stale completion changed current work", state)
			}
			completeResolution(t, h, source.next(t, "p2"), time.Time{})
			if len(h.opener.inputs) != 1 || h.s.State().PlaybackPart != "p2" || source.peak.Load() != 1 {
				t.Fatal("latest-only or source concurrency bound")
			}
		})
	}
}

func TestSelectingAcceptedPartRetiresPendingAcrossFastPaths(t *testing.T) {
	for _, native := range []playback.State{playback.Loading, playback.Paused, playback.Playing, playback.Ended, playback.Failed} {
		t.Run(string(native), func(t *testing.T) {
			h, source := controlledHarness(t, true)
			completeResolution(t, h, initialResolve(t, h, source), time.Time{})
			if native == playback.Playing {
				requireError(t, h.s.Play(), nil)
			}
			h.tick(native)
			requireError(t, h.s.SelectPart("p2"), nil)
			obsolete := source.next(t, "p2")
			requireError(t, h.s.SelectPart("p1"), nil)
			generation, opens := h.s.State().Playback.Generation, len(h.opener.inputs)
			if obsolete.ctx.Err() != context.Canceled || h.s.State().ResolvingPart != "" {
				t.Fatal("accepted fast path retained obsolete request")
			}
			completeResolution(t, h, obsolete, time.Time{})
			if state := h.s.State(); state.SelectedPart != "p1" || state.PlaybackPart != "p1" || state.Playback.Generation != generation || len(h.opener.inputs) != opens {
				t.Fatal("late result bypassed source generation", state)
			}
		})
	}
}

func TestReturningToSamePendingPartRejectsOlderGeneration(t *testing.T) {
	h, source := controlledHarness(t, true)
	old := initialResolve(t, h, source)
	requireError(t, h.s.SelectPart("p2"), nil)
	requireError(t, h.s.SelectPart("p1"), nil)
	old.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/obsolete-p1.mp4"}, time.Time{})}
	resultReady(t, h.s)
	h.s.Tick()
	if state := h.s.State(); len(h.opener.inputs) != 0 || state.SelectedPart != "p1" || state.ResolvingPart != "p1" || !state.DesiredPlaying {
		t.Fatal("same-part stale generation was accepted", state)
	}
	current := source.next(t, "p1")
	if current.ctx == old.ctx || current.ctx.Err() != nil {
		t.Fatal("returning to a superseded part reused its canceled request")
	}
	completeResolution(t, h, current, time.Time{})
	if len(h.opener.inputs) != 1 || h.opener.inputs[0].FilePath != "/fixture-p1.mp4" {
		t.Fatal("latest same-part resolution did not open exactly once")
	}
}

func TestSourceWorkerBoundsBurstQueueAndCompletedResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, source := controlledHarness(t, true)
		h.source.video.Parts = append(h.source.video.Parts, catalog.Part{ID: "p3", Title: "Three", Duration: 30 * time.Second})
		old := initialResolve(t, h, source)
		for range 50 {
			requireError(t, h.s.SelectPart("p2"), nil)
			requireError(t, h.s.SelectPart("p3"), nil)
		}
		current := h.s.pending
		requireError(t, h.s.Pause(), nil)
		for range 50 {
			requireError(t, h.s.SelectPart("p3"), nil)
		}
		synctest.Wait()
		source.noCall(t)
		if h.s.pending != current || !current.intent || len(h.s.requests) != 1 || source.active.Load() != 1 || source.peak.Load() != 1 {
			t.Fatal("burst did not retain one worker and the latest coalesced request")
		}
		old.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/obsolete.mp4"}, time.Time{})}
		synctest.Wait()
		if len(h.s.results) != 1 || source.active.Load() != 0 {
			t.Fatal("missing bounded result publication")
		}
		// A result awaiting Tick owns the one completed-result slot. No next
		// source call may finish (or even start) behind that buffered result.
		source.noCall(t)
		h.s.Tick()
		latest := source.next(t, "p3")
		completeResolution(t, h, latest, time.Time{})
		if len(h.opener.inputs) != 1 || h.s.State().PlaybackPart != "p3" || source.peak.Load() != 1 {
			t.Fatal("burst opened an intermediate part or used concurrent sources")
		}
	})
}

func TestRequestDeadlinesIncludeQueueTimeAndDoNotReleaseWorkerOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, source := controlledHarness(t, true)
		h.s.requestContext = heldDeadline
		old := initialResolve(t, h, source)
		time.Sleep(2 * time.Second)
		submitted := time.Now()
		requireError(t, h.s.SelectPart("p2"), nil)
		queued := h.s.pending.request.ctx
		if deadline, ok := queued.Deadline(); !ok || !deadline.Equal(submitted.Add(10*time.Second)) {
			t.Fatal("deadline did not start at submission", deadline)
		}
		time.Sleep(4 * time.Second)
		requireError(t, h.s.SelectPart("p2"), nil)
		if h.s.pending.request.ctx != queued {
			t.Fatal("coalescing extended the attempt deadline")
		}
		time.Sleep(6 * time.Second)
		h.s.Tick()
		if state := h.s.State(); state.SelectionError != catalog.Timeout || state.ResolvingPart != "" || state.SelectedPart != "p2" || len(h.opener.inputs) != 0 {
			t.Fatal("queued timeout remained hidden behind uncooperative source", state)
		}
		if source.active.Load() != 1 || source.peak.Load() != 1 {
			t.Fatal("timed-out source ownership was abandoned")
		}
		requireError(t, h.s.RetrySelection(), nil)
		time.Sleep(10 * time.Second)
		h.s.Tick()
		if h.s.State().SelectionError != catalog.Timeout || len(h.s.requests) != 0 {
			t.Fatal("queued retry did not get its own submission deadline")
		}
		completeResolution(t, h, old, time.Time{})
		synctest.Wait()
		source.noCall(t)
		if len(h.opener.inputs) != 0 || source.active.Load() != 0 || h.now != time.Unix(100, 0) {
			t.Fatal("expired result opened or request clock changed media-expiry clock")
		}
		requireError(t, h.s.RetrySelection(), nil)
		completeResolution(t, h, source.next(t, "p2"), time.Time{})
		if len(h.opener.inputs) != 1 || h.s.State().SelectionError != catalog.None {
			t.Fatal("explicit live retry failed")
		}
	})
}

func TestDetailTimeoutAndEarlierParentDeadline(t *testing.T) {
	t.Run("retry-after-timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h, source := controlledHarness(t, true)
			h.s.requestContext = heldDeadline
			requireError(t, h.s.Start(context.Background()), nil)
			old := source.next(t, "")
			time.Sleep(10 * time.Second)
			h.s.Tick()
			if state := h.s.State(); state.DetailPhase != DetailFailed || state.DetailError != catalog.Timeout || state.Closing {
				t.Fatal("request timeout terminated the session or was hidden", state)
			}
			requireError(t, h.s.RetryDetail(), nil)
			old.reply <- sourceReply{video: cloneVideo(h.source.video)}
			resultReady(t, h.s)
			h.s.Tick()
			if state := h.s.State(); state.DetailPhase != DetailLoading || len(state.Video.Parts) != 0 || state.DetailError != catalog.None {
				t.Fatal("late detail overwrote its retry", state)
			}
			source.next(t, "").reply <- sourceReply{video: cloneVideo(h.source.video)}
			resultReady(t, h.s)
			h.s.Tick()
			completeResolution(t, h, source.next(t, "p1"), time.Time{})
		})
	})
	t.Run("earlier-parent", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h, source := controlledHarness(t, false)
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), ctxKey{}, "parent"), 3*time.Second)
			defer cancel()
			requireError(t, h.s.Start(ctx), nil)
			call := source.next(t, "")
			want, _ := ctx.Deadline()
			if deadline, _ := call.ctx.Deadline(); !deadline.Equal(want) || call.ctx.Value(ctxKey{}) != "parent" {
				t.Fatal("attempt did not inherit the earlier parent deadline and values")
			}
			time.Sleep(3 * time.Second)
			waitDone(t, h.s.Done())
			if h.s.State().DetailPhase != DetailLoading {
				t.Fatal("worker mutated state during cancellation")
			}
			h.s.Tick()
			if state := h.s.State(); state.DetailPhase != DetailFailed || state.DetailError != catalog.Timeout || state.Playback.State != playback.Closed || state.Playback.Generation != 0 || len(h.opener.inputs) != 0 {
				t.Fatal("parent deadline was not terminal before native Open", state)
			}
			requireError(t, h.s.RetryDetail(), playback.IsClosed)
		})
	})
}

// Model a deadline whose timer callback has not run yet. The virtual request
// clock advances independently of h.now, while cancellation remains explicit.
type deadlineContext struct {
	context.Context
	deadline time.Time
}

func (c deadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func heldDeadline(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	deadline := time.Now().Add(timeout)
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	return deadlineContext{Context: ctx, deadline: deadline}, cancel
}

func TestWorkerDoesNotDispatchExpiredOrCanceledQueuedRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &controlledSource{calls: make(chan *sourceCall, 8), abort: make(chan struct{})}
		ctx, cancel := context.WithCancel(context.Background())
		requests := make(chan sourceRequest, 1)
		results := make(chan sourceResult, 1)
		done := make(chan struct{})
		go runSource(ctx, source, requests, results, done)
		t.Cleanup(func() {
			cancel()
			close(source.abort)
			waitDone(t, done)
		})
		expired, stopExpired := heldDeadline(ctx, 0)
		defer stopExpired()
		canceled, stopCanceled := context.WithCancel(ctx)
		stopCanceled()
		for _, obsolete := range []context.Context{expired, canceled} {
			requests <- sourceRequest{purpose: selectionRequest, part: "obsolete", ctx: obsolete}
			synctest.Wait()
			source.noCall(t)
			if len(results) != 0 {
				t.Fatal("expired queue entry was dispatched and produced a result")
			}
		}
		requests <- sourceRequest{purpose: selectionRequest, part: "current", ctx: ctx}
		source.next(t, "current").reply <- sourceReply{}
		select {
		case result := <-results:
			close(result.consumed)
		case <-time.After(time.Second):
			t.Fatal("valid work did not follow retired queue entries")
		}
	})
}

func TestResultExpiryIsRecheckedAtUIAcceptance(t *testing.T) {
	h, source := controlledHarness(t, false)
	call := initialResolve(t, h, source)
	call.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/fixture.mp4"}, h.now.Add(ResolutionMargin+time.Second))}
	resultReady(t, h.s)
	h.now = h.now.Add(time.Second)
	h.s.Tick()
	if state := h.s.State(); state.SelectionError != catalog.Unavailable || state.PlaybackPart != "" || len(h.opener.inputs) != 0 {
		t.Fatal("result expiring at the margin opened after waiting for Tick", state)
	}
	requireError(t, h.s.RetrySelection(), nil)
	completeResolution(t, h, source.next(t, "p1"), h.now.Add(ResolutionMargin+time.Nanosecond))
	if len(h.opener.inputs) != 1 || h.s.State().SelectionError != catalog.None {
		t.Fatal("just-outside-margin resolution was not accepted")
	}
}

func TestPendingTargetIntentAndAcceptedCommandsAreSeparate(t *testing.T) {
	h, source := controlledHarness(t, false)
	completeResolution(t, h, initialResolve(t, h, source), time.Time{})
	h.tick(playback.Paused)
	accepted := h.engine()
	accepted.commands = nil
	accepted.deferCommands = true
	requireError(t, h.s.SelectPart("p2"), nil)
	pending := source.next(t, "p2")
	if !h.s.State().DesiredPlaying || len(accepted.commands) != 0 {
		t.Fatal("selecting B altered paused A's transport")
	}
	requireError(t, h.s.Play(), nil)
	requireError(t, h.s.Play(), nil)
	requireError(t, h.s.Toggle(), nil)
	requireError(t, h.s.Pause(), nil)
	want := []playback.Command{{Kind: playback.Play}, {Kind: playback.Pause}}
	if !reflect.DeepEqual(accepted.commands, want) || h.s.State().DesiredPlaying {
		t.Fatal("pending intent suppressed an accepted-engine command", accepted.commands)
	}
	requireError(t, h.s.SetVolume(.3), nil)
	requireError(t, h.s.SetRate(2), nil)
	requireError(t, h.s.Seek(time.Second), nil)
	if countKind(accepted, playback.Seek) != 1 {
		t.Fatal("seek stopped targeting accepted playback")
	}
	completeResolution(t, h, pending, time.Time{})
	h.tick(playback.Ready)
	if accepted.closed != 1 || countKind(h.engine(), playback.Play) != 0 || h.engine().snapshot.Volume != .3 || h.engine().snapshot.Rate != 2 {
		t.Fatal("completion overwrote pending pause or settings")
	}
}

func TestOldTerminalEventDoesNotErasePendingIntentOrCompeteWithIt(t *testing.T) {
	for _, terminal := range []playback.State{playback.Ended, playback.Failed} {
		for _, pause := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pause=%v", terminal, pause), func(t *testing.T) {
				h, source := controlledHarness(t, false)
				completeResolution(t, h, initialResolve(t, h, source), time.Time{})
				h.tick(playback.Paused)
				requireError(t, h.s.SelectPart("p2"), nil)
				pending := source.next(t, "p2")
				h.tick(terminal)
				if !h.s.State().DesiredPlaying {
					t.Fatal("old terminal event cleared new target's autoplay")
				}
				requireError(t, h.s.Play(), nil)
				if pause {
					requireError(t, h.s.Toggle(), nil)
					requireError(t, h.s.Pause(), nil)
				}
				if len(h.opener.inputs) != 1 || h.s.State().ResolvingPart != "p2" {
					t.Fatal("pending transport command replayed old A")
				}
				completeResolution(t, h, pending, time.Time{})
				h.tick(playback.Ready)
				want := 1
				if pause {
					want = 0
				}
				if countKind(h.engine(), playback.Play) != want || h.s.State().DesiredPlaying == pause {
					t.Fatal("terminal or completion overwrote the latest target intent")
				}
			})
		}
	}
}

func TestConflictingRetriesDoNotMutatePendingWork(t *testing.T) {
	h, source := controlledHarness(t, true)
	completeResolution(t, h, initialResolve(t, h, source), h.now.Add(time.Minute))
	requireError(t, h.s.Play(), nil)
	h.tick(playback.Playing)
	requireError(t, h.s.SelectPart("p2"), nil)
	selected := source.next(t, "p2")
	before := h.s.State()
	current := h.s.pending
	requireError(t, h.s.RetryPlayback(), playback.Invalid)
	if !reflect.DeepEqual(h.s.State(), before) || h.s.pending != current {
		t.Fatal("conflicting playback retry mutated selected B")
	}
	selected.reply <- sourceReply{err: errors.New("private B error")}
	resultReady(t, h.s)
	h.s.Tick()
	h.tick(playback.Failed)
	h.now = h.now.Add(time.Minute)
	requireError(t, h.s.RetryPlayback(), nil)
	refresh := source.next(t, "p1")
	before, current = h.s.State(), h.s.pending
	requireError(t, h.s.RetrySelection(), playback.Invalid)
	requireError(t, h.s.RetryPlayback(), nil)
	if !reflect.DeepEqual(h.s.State(), before) || h.s.pending != current || current.request.ctx.Err() != nil {
		t.Fatal("retry overlap replaced accepted-A refresh")
	}
	refresh.reply <- sourceReply{err: errors.New("private A error")}
	resultReady(t, h.s)
	h.s.Tick()
	if state := h.s.State(); state.SelectedPart != "p2" || state.SelectionError != catalog.Unavailable || state.PlaybackRetryError != catalog.Unavailable || h.s.selectionRetry != "p2" {
		t.Fatal("accepted-A error erased selected-B retry", state)
	}
	requireError(t, h.s.RetryPlayback(), nil)
	refresh = source.next(t, "p1")
	if h.s.State().PlaybackRetryError != catalog.None {
		t.Fatal("explicit playback retry did not clear its error")
	}
	requireError(t, h.s.SelectPart("p2"), nil)
	completeResolution(t, h, refresh, h.now.Add(time.Minute))
	completeResolution(t, h, source.next(t, "p2"), time.Time{})
	if state := h.s.State(); state.SelectedPart != "p2" || state.PlaybackPart != "p2" || state.SelectionError != catalog.None || state.PlaybackRetryError != catalog.None {
		t.Fatal("new selection could not supersede playback refresh", state)
	}
}

func TestRefreshOpenFailureKeepsTheCorrectRetryTarget(t *testing.T) {
	for _, differentSelection := range []bool{false, true} {
		t.Run(fmt.Sprint(differentSelection), func(t *testing.T) {
			h := newHarness(t)
			h.source.edit(func(src *fakeSource) { src.expires["p1"] = h.now.Add(time.Minute) })
			h.start(t)
			if differentSelection {
				h.source.edit(func(src *fakeSource) { src.errors["p2"] = playback.Invalid })
				requireError(t, h.s.SelectPart("p2"), nil)
				h.settle(t)
			}
			h.tick(playback.Ended)
			h.now = h.now.Add(time.Minute)
			h.source.edit(func(src *fakeSource) { src.expires["p1"] = h.now.Add(time.Minute) })
			h.opener.err = errors.New("private decoder refresh error")
			requireError(t, h.s.RetryPlayback(), nil)
			h.settle(t)
			state := h.s.State()
			if state.PlaybackPart != "p1" || state.Playback.Generation != 2 || state.Playback.Error != playback.Media {
				t.Fatal("failed accepted refresh lost native identity", state)
			}
			if differentSelection {
				if state.SelectionError != catalog.Invalid || h.s.selectionRetry != "p2" || state.PlaybackRetryError != catalog.Unavailable {
					t.Fatal("refresh-open failure replaced B's error", state)
				}
			} else if state.SelectionError != catalog.Unavailable || h.s.selectionRetry != "p1" || state.PlaybackRetryError != catalog.None {
				t.Fatal("selected-A refresh failure lacked its selection retry", state)
			}
			h.opener.err = nil
			requireError(t, h.s.RetryPlayback(), nil)
			if h.s.State().PlaybackRetryError != catalog.None {
				t.Fatal("new accepted playback did not clear its retry error")
			}
		})
	}
}

func TestCloseAndParentCancelOwnPendingSourceUntilDone(t *testing.T) {
	for _, stage := range []string{"detail", "initial-resolve", "accepted-playback"} {
		for _, parentCancel := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/parent=%v", stage, parentCancel), func(t *testing.T) {
				h, source := controlledHarness(t, true)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				requireError(t, h.s.Start(ctx), nil)
				call := source.next(t, "")
				if stage != "detail" {
					call.reply <- sourceReply{video: cloneVideo(h.source.video)}
					resultReady(t, h.s)
					h.s.Tick()
					call = source.next(t, "p1")
				}
				if stage == "accepted-playback" {
					completeResolution(t, h, call, time.Time{})
					requireError(t, h.s.SelectPart("p2"), nil)
					call = source.next(t, "p2")
				}
				opens := len(h.opener.inputs)
				if parentCancel {
					cancel()
					h.s.Tick()
				} else {
					requireError(t, h.s.Close(), nil)
				}
				state := h.s.State()
				if state.Playback.State != playback.Closed || !state.Closing || state.ResolvingPart != "" || state.DesiredPlaying || h.s.Engine() != nil || call.ctx.Err() != context.Canceled {
					t.Fatal("close did not retire pending/native work without waiting", state)
				}
				if parentCancel && stage == "accepted-playback" && state.Playback.Error != playback.Canceled {
					t.Fatal("session replaced the native canceled observation")
				}
				if parentCancel && stage == "detail" && (state.DetailPhase != DetailFailed || state.DetailError != catalog.Canceled) {
					t.Fatal("parent cancellation before detail was not observable")
				}
				if parentCancel && stage == "initial-resolve" && state.SelectionError != catalog.Canceled {
					t.Fatal("parent cancellation before first Open was not observable")
				}
				select {
				case <-h.s.Done():
					t.Fatal("Done dropped ownership of a still-running source")
				default:
				}
				for _, action := range []func() error{func() error { return h.s.Start(context.Background()) }, h.s.RetryDetail, h.s.Play, h.s.Pause, h.s.Toggle, h.s.NextPart, h.s.RetrySelection, h.s.RetryPlayback, func() error { return h.s.SelectPart("p1") }, func() error { return h.s.Seek(0) }, func() error { return h.s.SetRate(1) }, func() error { return h.s.SetVolume(1) }, h.s.ToggleMute} {
					requireError(t, action(), playback.IsClosed)
				}
				call.reply <- sourceReply{video: cloneVideo(h.source.video), resolution: testResolution(playback.Input{FilePath: "/late.mp4"}, time.Time{})}
				// No Tick or paint is necessary to finish worker ownership.
				waitDone(t, h.s.Done())
				requireError(t, h.s.Close(), nil)
				h.s.Tick()
				if len(h.opener.inputs) != opens || h.s.State().Closing || h.s.State().Playback.Error != state.Playback.Error || source.active.Load() != 0 {
					t.Fatal("late result revived playback or repeated Close changed terminal state")
				}
				if opens != 0 && h.engine().closed != 1 {
					t.Fatal("native cleanup was not exactly once")
				}
			})
		}
	}
}

func TestCloseBeforeStartAndAlreadyCanceledStart(t *testing.T) {
	h, source := controlledHarness(t, false)
	requireError(t, h.s.Close(), nil)
	waitDone(t, h.s.Done())
	requireError(t, h.s.Close(), nil)
	requireError(t, h.s.Start(context.Background()), playback.IsClosed)
	if h.s.State().Closing || h.s.State().Video.Parts == nil {
		t.Fatal("close-before-start state")
	}
	source.noCall(t)
	x, other := controlledHarness(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requireError(t, x.s.Start(ctx), nil)
	waitDone(t, x.s.Done())
	x.s.Tick()
	other.noCall(t)
	if state := x.s.State(); state.DetailPhase != DetailFailed || state.DetailError != catalog.Canceled || state.Playback.Generation != 0 || state.Playback.State != playback.Closed {
		t.Fatal("already-canceled Start was not terminal", state)
	}
}

func TestCloseReleasesACompletedResultWithoutAnotherTick(t *testing.T) {
	h, source := controlledHarness(t, false)
	call := initialResolve(t, h, source)
	call.reply <- sourceReply{resolution: testResolution(playback.Input{FilePath: "/fixture.mp4"}, time.Time{})}
	resultReady(t, h.s)
	requireError(t, h.s.Close(), nil)
	waitDone(t, h.s.Done())
	if state := h.s.State(); len(h.opener.inputs) != 0 || state.Closing || state.Playback.State != playback.Closed {
		t.Fatal("completed-result handoff blocked close or opened media", state)
	}
}
