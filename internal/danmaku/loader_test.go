package danmaku

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

type loadReply struct {
	metadata Metadata
	segment  Segment
	err      error
}

// A context whose deadline is known but whose timer has not delivered Err.
type delayedLoadDeadline struct {
	context.Context
	at time.Time
}

func (c delayedLoadDeadline) Deadline() (time.Time, bool) { return c.at, true }

type loadCall struct {
	ctx   context.Context
	ref   catalog.PartRef
	index int
	reply chan loadReply
}

type controlledSegments struct {
	calls               chan *loadCall
	abort               chan struct{}
	ignoreCancel        bool
	count, active, peak atomic.Int32
}

func (s *controlledSegments) call(ctx context.Context, ref catalog.PartRef, index int) loadReply {
	s.count.Add(1)
	active := s.active.Add(1)
	for old := s.peak.Load(); active > old && !s.peak.CompareAndSwap(old, active); old = s.peak.Load() {
	}
	defer s.active.Add(-1)
	call := &loadCall{ctx: ctx, ref: ref, index: index, reply: make(chan loadReply, 1)}
	s.calls <- call
	done := ctx.Done()
	if s.ignoreCancel {
		done = nil
	}
	select {
	case reply := <-call.reply:
		return reply
	case <-done:
		return loadReply{err: ctx.Err()}
	case <-s.abort:
		return loadReply{err: context.Canceled}
	}
}

func (s *controlledSegments) Metadata(ctx context.Context, ref catalog.PartRef) (Metadata, error) {
	result := s.call(ctx, ref, 0)
	return result.metadata, result.err
}

func (s *controlledSegments) Segment(ctx context.Context, ref catalog.PartRef, index int) (Segment, error) {
	result := s.call(ctx, ref, index)
	return result.segment, result.err
}

type loadHarness struct {
	loader *Loader
	source *controlledSegments
	ref    catalog.PartRef
}

func newLoadHarness(t *testing.T, ignoreCancel bool) *loadHarness {
	t.Helper()
	source := &controlledSegments{calls: make(chan *loadCall, 20), abort: make(chan struct{}), ignoreCancel: ignoreCancel}
	loader, err := NewLoader(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { loader.Close(); close(source.abort); awaitLoad(t, loader.Done()) })
	return &loadHarness{loader: loader, source: source, ref: catalog.PartRef{Video: "video", Part: "part-1"}}
}

func awaitLoad[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case result := <-channel:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("owned controlled load did not finish")
	}
	var zero T
	return zero
}

func (h *loadHarness) next(t *testing.T, index int) *loadCall {
	t.Helper()
	call := awaitLoad(t, h.source.calls)
	if call.index != index {
		t.Fatal("unexpected controlled source operation", call.index, index)
	}
	return call
}

func (h *loadHarness) quiet(t *testing.T) {
	t.Helper()
	synctest.Wait()
	select {
	case call := <-h.source.calls:
		t.Fatal("unrequested controlled source operation", call.index)
	default:
	}
}

func (h *loadHarness) sample(at time.Duration) playback.Snapshot {
	return playback.Snapshot{Generation: h.loader.generation, State: playback.Paused, Position: at, Duration: h.loader.duration}
}

func loadResultReady(t *testing.T, loader *Loader) {
	t.Helper()
	result := awaitLoad(t, loader.results)
	loader.results <- result
}

func (h *loadHarness) finish(t *testing.T, call *loadCall, reply loadReply, at time.Duration) {
	t.Helper()
	call.reply <- reply
	loadResultReady(t, h.loader)
	h.loader.Tick(h.sample(at))
}

func (h *loadHarness) begin(t *testing.T, duration time.Duration) *loadCall {
	t.Helper()
	if err := h.loader.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.loader.Bind(h.ref, duration, 1); err != nil {
		t.Fatal(err)
	}
	h.finish(t, h.next(t, 0), loadReply{}, 0)
	return h.next(t, 1)
}

func loadEntry(id string, at time.Duration, text string) SegmentEntry {
	return SegmentEntry{ID: id, Entry: Entry{At: at, Mode: FixedTop, Text: text, Color: 0xffffff}}
}

func segment(index int, entries ...SegmentEntry) Segment {
	return Segment{Index: index, Encoded: len(entries), Entries: entries}
}

func TestLoaderConstructionBindingAndReadOnlyState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if _, err := NewLoader(nil); err != catalog.Invalid {
			t.Fatal("nil source accepted")
		}
		h := newLoadHarness(t, false)
		if state := h.loader.State(); state.Phase != LoadInitial || state.ActiveRequests != 0 || h.loader.Window().Entries == nil || h.source.count.Load() != 0 {
			t.Fatal("construction is not pure", state)
		}
		if h.loader.Bind(h.ref, time.Hour, 1) != catalog.Invalid || h.loader.Start(nil) != catalog.Invalid {
			t.Fatal("unstarted or nil-context operation accepted")
		}
		if err := h.loader.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		h.quiet(t)
		for _, test := range []struct {
			ref catalog.PartRef
			dur time.Duration
			gen uint64
		}{{catalog.PartRef{}, time.Hour, 1}, {h.ref, 0, 1}, {h.ref, -1, 1}, {h.ref, 7*24*time.Hour + 1, 1}, {h.ref, time.Hour, 0}} {
			if err := h.loader.Bind(test.ref, test.dur, test.gen); err != catalog.Invalid {
				t.Fatal("invalid bind accepted", err)
			}
		}
		h.quiet(t)
		if h.loader.Retry() != catalog.Invalid {
			t.Fatal("retry without a failure accepted")
		}
		if err := h.loader.Bind(h.ref, time.Hour, 3); err != nil {
			t.Fatal(err)
		}
		metadata := h.next(t, 0)
		before := h.loader.State()
		if h.loader.Bind(h.ref, time.Hour, 2) != catalog.Invalid || h.loader.State() != before || metadata.ctx.Err() != nil {
			t.Fatal("stale generation changed a valid binding")
		}
		metadata.reply <- loadReply{}
		loadResultReady(t, h.loader)
		for range 4 {
			if h.loader.State().Phase != LoadLoading || len(h.loader.Window().Entries) != 0 {
				t.Fatal("State or Window accepted worker data")
			}
		}
		h.quiet(t)
		h.loader.Tick(h.sample(0))
		h.next(t, 1)
		h.loader.Close()
		awaitLoad(t, h.loader.Done())
		if h.loader.Start(context.Background()) != catalog.Closed || h.loader.Bind(h.ref, time.Hour, 4) != catalog.Closed || h.loader.Retry() != catalog.Closed {
			t.Fatal("closed loader restarted")
		}
		if state := h.loader.State(); state.Phase != LoadClosed || state.ActiveRequests != 0 || state.CachedSegments != 0 || h.loader.Window().Entries == nil {
			t.Fatal("incomplete closed state", state)
		}
	})
}

func TestLoaderMetadataFailureLatchAndRetry(t *testing.T) {
	for _, reply := range []loadReply{{metadata: Metadata{Known: true}}, {err: fmt.Errorf("private sentinel: %w", catalog.Network)},
		{err: errors.New("private sentinel")}, {err: catalog.Code("private sentinel")},
		{err: context.Canceled}, {err: context.DeadlineExceeded}} {
		synctest.Test(t, func(t *testing.T) {
			h := newLoadHarness(t, false)
			_ = h.loader.Start(context.Background())
			_ = h.loader.Bind(h.ref, time.Hour, 1)
			h.finish(t, h.next(t, 0), reply, 0)
			want := catalog.Unavailable
			if errors.Is(reply.err, catalog.Network) {
				want = catalog.Network
			} else if errors.Is(reply.err, context.Canceled) {
				want = catalog.Canceled
			} else if errors.Is(reply.err, context.DeadlineExceeded) {
				want = catalog.Timeout
			}
			for range 20 {
				h.loader.Tick(h.sample(0))
			}
			if state := h.loader.State(); state.Phase != LoadFailed || state.Error != want || strings.Contains(fmt.Sprint(state), "sentinel") {
				t.Fatal("metadata denial was not a safe latched failure", state)
			}
			h.quiet(t)
			if err := h.loader.Retry(); err != nil {
				t.Fatal(err)
			}
			retry := h.next(t, 0)
			pending := h.loader.pending
			if err := h.loader.Retry(); err != nil {
				t.Fatal("pending retry was not idempotent", err)
			}
			if h.loader.pending != pending || retry.ctx.Err() != nil {
				t.Fatal("duplicate retry replaced the pending attempt")
			}
			h.finish(t, retry, loadReply{metadata: Metadata{Known: true, Available: true}}, 0)
			h.finish(t, h.next(t, 1), loadReply{segment: segment(1)}, 0)
			if h.loader.State().Phase != LoadReady || h.loader.State().Error != catalog.None || h.source.count.Load() != 3 {
				t.Fatal("metadata retry did not proceed exactly once")
			}
		})
	}
}

func TestLoaderAbsoluteWindowsBoundariesAndDeterministicCopies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		first := segment(1, loadEntry("z", 5*time.Second, "z"), loadEntry("a", 5*time.Second, " a \t"), loadEntry("boundary", 360*time.Second, "boundary"))
		call := h.begin(t, 1080*time.Second)
		call.reply <- loadReply{segment: first}
		loadResultReady(t, h.loader)
		first.Entries[0].ID, first.Entries[0].Entry.Text = "mutated", "mutated"
		h.loader.Tick(h.sample(5 * time.Second))
		window := h.loader.Window()
		if len(window.Entries) != 3 || window.Entries[0].Text != "a" || window.Entries[1].Text != "z" {
			t.Fatal("source alias or non-deterministic identity order")
		}
		window.Entries[0].Text = "caller mutation"
		if h.loader.Window().Entries[0].Text != "a" {
			t.Fatal("Window slice aliases the cache")
		}
		h.loader.Tick(h.sample(15999 * time.Millisecond))
		h.quiet(t)
		h.loader.Tick(h.sample(16 * time.Second))
		second := h.next(t, 2)
		h.finish(t, second, loadReply{segment: segment(2, loadEntry("boundary", 360*time.Second, "boundary"), loadEntry("second", 361250*time.Millisecond, "second"))}, 18*time.Second)
		window = h.loader.Window()
		if state := h.loader.State(); state.Segment != 1 || state.CachedSegments != 2 || state.Entries != 5 || len(window.Entries) != 4 || window.Entries[3].At != 361250*time.Millisecond {
			t.Fatal("prefetch, boundary dedupe or absolute time changed", state)
		}
		h.loader.Tick(h.sample(365 * time.Second))
		if state := h.loader.State(); state.Segment != 2 || state.CachedSegments != 2 {
			t.Fatal("first sixteen seconds did not retain previous/current", state)
		}
		h.quiet(t)
		h.loader.Tick(h.sample(376 * time.Second))
		third := h.next(t, 3)
		if state := h.loader.State(); state.CachedSegments != 1 || state.Entries != 2 {
			t.Fatal("old segment retained after the overlap boundary", state)
		}
		h.finish(t, third, loadReply{segment: segment(3, loadEntry("third", 725*time.Second, "third"))}, 376*time.Second)
		if h.loader.State().CachedSegments != 2 || len(h.loader.Window().Entries) != 3 {
			t.Fatal("current/next window not installed")
		}
		h.loader.Tick(h.sample(1080 * time.Second))
		if state := h.loader.State(); state.Segment != 3 || state.CachedSegments != 1 {
			t.Fatal("exact final duration escaped the last segment", state)
		}
		h.quiet(t)
		h.loader.Tick(h.sample(5 * time.Second))
		if len(h.loader.Window().Entries) != 0 {
			t.Fatal("new seek retained unrelated segments")
		}
		h.finish(t, h.next(t, 1), loadReply{segment: segment(1, loadEntry("return", 5*time.Second, "return"))}, 5*time.Second)
		if h.loader.State().CachedSegments != 1 || len(h.loader.Window().Entries) != 1 {
			t.Fatal("evicted segment was not freshly loaded")
		}
	})
}

func TestLoaderSamePartGenerationReusesCacheAndPendingData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		first := h.begin(t, 720*time.Second)
		if err := h.loader.Bind(h.ref, 720*time.Second, 2); err != nil || first.ctx.Err() != nil {
			t.Fatal("same-part generation retired valid pending data", err)
		}
		h.finish(t, first, loadReply{segment: segment(1, loadEntry("first", 0, "first"))}, 18*time.Second)
		second := h.next(t, 2)
		revision := h.loader.Window().Revision
		if err := h.loader.Bind(h.ref, 720*time.Second, 3); err != nil || second.ctx.Err() != nil {
			t.Fatal("quality generation canceled its same-scope segment", err)
		}
		if window := h.loader.Window(); window.Generation != 3 || window.Revision <= revision || len(window.Entries) != 1 {
			t.Fatal("new presentation generation did not republish its accepted cache")
		}
		second.reply <- loadReply{segment: segment(2, loadEntry("future", 378*time.Second, "future"))}
		loadResultReady(t, h.loader)
		h.loader.Tick(playback.Snapshot{Generation: 2, State: playback.Paused})
		window := h.loader.Window()
		if window.Generation != 3 || window.Revision <= revision || len(window.Entries) != 2 || h.loader.State().CachedSegments != 2 || h.source.count.Load() != 3 {
			t.Fatal("same-part data was rejected or presentation generation stayed stale")
		}
		h.loader.Tick(playback.Snapshot{Generation: 3, State: playback.Loading})
		if h.loader.State().CachedSegments != 2 {
			t.Fatal("loading zero position evicted reusable data")
		}
		h.loader.Tick(h.sample(18 * time.Second))
		revision = h.loader.Window().Revision
		_ = h.loader.Bind(h.ref, 720*time.Second, 3)
		if h.loader.Window().Revision != revision {
			t.Fatal("same generation bind republished an unchanged window")
		}
		h.quiet(t)
	})
}

func TestLoaderResumeWindowFetchesCurrentBeforePrevious(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		_ = h.loader.Start(context.Background())
		_ = h.loader.Bind(h.ref, 720*time.Second, 1)
		metadata := h.next(t, 0)
		h.finish(t, metadata, loadReply{}, 365*time.Second)
		current := h.next(t, 2)
		h.finish(t, current, loadReply{segment: segment(2, loadEntry("current", 365*time.Second, "current"))}, 365*time.Second)
		previous := h.next(t, 1)
		h.finish(t, previous, loadReply{segment: segment(1, loadEntry("previous", 359*time.Second, "previous"))}, 365*time.Second)
		if state := h.loader.State(); state.Segment != 2 || state.CachedSegments != 2 || state.Phase != LoadReady || len(h.loader.Window().Entries) != 2 {
			t.Fatal("fresh resume did not backfill the overlap after current", state)
		}
		h.quiet(t)
	})
}

func TestLoaderRetryFailedPrefetchPreservesCurrentWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.finish(t, h.begin(t, 720*time.Second), loadReply{segment: segment(1, loadEntry("current", 0, "current"))}, 18*time.Second)
		h.finish(t, h.next(t, 2), loadReply{err: catalog.Network}, 18*time.Second)
		before := h.loader.Window()
		if err := h.loader.Retry(); err != nil {
			t.Fatal(err)
		}
		retry := h.next(t, 2)
		if h.loader.State().Phase != LoadReady || !reflect.DeepEqual(before.Entries, h.loader.Window().Entries) {
			t.Fatal("retrying neighbor removed usable current data")
		}
		h.finish(t, retry, loadReply{segment: segment(2, loadEntry("future", 378*time.Second, "future"))}, 18*time.Second)
		if state := h.loader.State(); state.CachedSegments != 2 || state.Entries != 2 || state.PrefetchError != catalog.None || state.Error != catalog.None {
			t.Fatal("prefetch retry did not replace just its failed slot", state)
		}
	})
}

func TestLoaderCurrentPriorityKeepsOneActualCallAndLatestRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, true)
		h.finish(t, h.begin(t, 1440*time.Second), loadReply{segment: segment(1)}, 5*time.Second)
		h.loader.Tick(h.sample(18 * time.Second))
		old := h.next(t, 2)
		h.loader.Tick(h.sample(720 * time.Second))
		queued := h.loader.pending
		h.loader.Tick(h.sample(1098 * time.Second))
		if old.ctx.Err() != context.Canceled || queued.request.ctx.Err() != context.Canceled || len(h.loader.requests) != 1 || h.loader.pending.request.index != 4 || h.loader.State().ActiveRequests != 1 {
			t.Fatal("current priority, cancellation or latest mailbox failed")
		}
		h.quiet(t)
		h.finish(t, old, loadReply{segment: segment(2, loadEntry("old", 361*time.Second, "old"))}, 1098*time.Second)
		current := h.next(t, 4)
		if len(h.loader.Window().Entries) != 0 || h.source.peak.Load() != 1 {
			t.Fatal("obsolete data published or calls overlapped")
		}
		h.finish(t, current, loadReply{segment: segment(4, loadEntry("current", 1090*time.Second, "current"))}, 1098*time.Second)
		if state := h.loader.State(); state.Phase != LoadReady || state.CachedSegments != 1 || state.ActiveRequests != 0 || h.source.count.Load() != 4 {
			t.Fatal("latest current request was not the only follow-up", state)
		}
	})
}

func TestLoaderQueuedDeadlineAndExplicitRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, true)
		h.finish(t, h.begin(t, 1080*time.Second), loadReply{segment: segment(1)}, 18*time.Second)
		old := h.next(t, 2)
		h.loader.requestContext = func(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			return delayedLoadDeadline{Context: ctx, at: time.Now().Add(timeout)}, cancel
		}
		h.loader.Tick(h.sample(738 * time.Second))
		deadline, _ := h.loader.pending.request.ctx.Deadline()
		if deadline.Sub(time.Now()) != 10*time.Second {
			t.Fatal("deadline did not begin at submission")
		}
		time.Sleep(10 * time.Second)
		if h.loader.pending.request.ctx.Err() != nil {
			t.Fatal("test must isolate a delayed deadline timer")
		}
		h.loader.Tick(h.sample(738 * time.Second))
		for range 10 {
			h.loader.Tick(h.sample(738 * time.Second))
		}
		if state := h.loader.State(); state.Phase != LoadFailed || state.Error != catalog.Timeout || state.ActiveRequests != 1 || len(h.loader.requests) != 0 {
			t.Fatal("queued timeout was hidden or retried", state)
		}
		h.quiet(t)
		h.finish(t, old, loadReply{err: errors.New("private obsolete error")}, 738*time.Second)
		if h.loader.State().Error != catalog.Timeout {
			t.Fatal("obsolete error replaced the current timeout")
		}
		h.quiet(t)
		if err := h.loader.Retry(); err != nil {
			t.Fatal(err)
		}
		retry := h.next(t, 3)
		if retry.ctx.Err() != nil {
			t.Fatal("explicit retry inherited old deadline")
		}
		h.finish(t, retry, loadReply{segment: segment(3, loadEntry("retry", 740*time.Second, "retry"))}, 738*time.Second)
		if state := h.loader.State(); state.Phase != LoadReady || state.Entries != 1 || state.Error != catalog.None || h.source.peak.Load() != 1 {
			t.Fatal("retry did not replace the failed slot", state)
		}
	})
}

func TestLoaderExpiredSubmissionNeverCallsSource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.loader.requestContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			return delayedLoadDeadline{Context: ctx, at: time.Now().Add(-time.Second)}, cancel
		}
		_ = h.loader.Start(context.Background())
		_ = h.loader.Bind(h.ref, time.Hour, 1)
		h.quiet(t)
		if h.source.count.Load() != 0 || h.loader.State().ActiveRequests != 0 {
			t.Fatal("expired submission entered its source method")
		}
		h.loader.Tick(h.sample(0))
		if state := h.loader.State(); state.Phase != LoadFailed || state.Error != catalog.Timeout {
			t.Fatal("logical deadline was hidden until a timer callback", state)
		}
	})
}

func TestLoaderWorkerWaitsForResultAcknowledgment(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, true)
		_ = h.loader.Start(context.Background())
		_ = h.loader.Bind(h.ref, time.Hour, 1)
		old := h.next(t, 0)
		other := catalog.PartRef{Video: h.ref.Video, Part: "part-2"}
		_ = h.loader.Bind(other, time.Hour, 2)
		old.reply <- loadReply{}
		loadResultReady(t, h.loader)
		h.quiet(t)
		if h.source.count.Load() != 1 || h.loader.State().ActiveRequests != 0 || len(h.loader.results) != 1 || len(h.loader.requests) != 1 {
			t.Fatal("worker advanced while its predecessor's result was unacknowledged")
		}
		h.loader.Tick(h.sample(0))
		if current := h.next(t, 0); current.ref != other {
			t.Fatal("acknowledgment did not release precisely the latest request")
		}
	})
}

func TestLoaderPrefetchFailurePromotesWithoutAutomaticRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.finish(t, h.begin(t, 1080*time.Second), loadReply{segment: segment(1, loadEntry("first", 359*time.Second, "first"))}, 18*time.Second)
		h.finish(t, h.next(t, 2), loadReply{err: catalog.RateLimited}, 18*time.Second)
		if state := h.loader.State(); state.Phase != LoadReady || state.Error != catalog.None || state.PrefetchError != catalog.RateLimited || state.CachedSegments != 1 {
			t.Fatal("prefetch failure destroyed accepted current data", state)
		}
		_ = h.loader.Bind(h.ref, 1080*time.Second, 2)
		for range 10 {
			h.loader.Tick(h.sample(365 * time.Second))
		}
		if state := h.loader.State(); state.Phase != LoadFailed || state.Error != catalog.RateLimited || len(h.loader.Window().Entries) != 1 {
			t.Fatal("failed prefetch was not promoted intact", state)
		}
		h.quiet(t)
		_ = h.loader.Retry()
		call := h.next(t, 2)
		pending := h.loader.pending
		if err := h.loader.Retry(); err != nil {
			t.Fatal("pending retry was not idempotent", err)
		}
		if h.loader.pending != pending {
			t.Fatal("repeated retry duplicated work")
		}
		h.finish(t, call, loadReply{segment: segment(2, loadEntry("second", 365*time.Second, "second"))}, 365*time.Second)
		if state := h.loader.State(); state.Phase != LoadReady || state.Entries != 2 || state.Error != catalog.None || state.PrefetchError != catalog.None {
			t.Fatal("retry accumulated data or retained failure", state)
		}
		if h.loader.Retry() != catalog.Invalid {
			t.Fatal("successful slot silently refreshed")
		}
	})
}

func TestLoaderFailedSlotCanLoadAgainOnlyAfterEviction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.finish(t, h.begin(t, 1080*time.Second), loadReply{segment: segment(1)}, 18*time.Second)
		h.finish(t, h.next(t, 2), loadReply{err: catalog.Network}, 18*time.Second)
		h.loader.Tick(h.sample(738 * time.Second))
		h.finish(t, h.next(t, 3), loadReply{segment: segment(3)}, 738*time.Second)
		h.loader.Tick(h.sample(365 * time.Second))
		call := h.next(t, 2)
		if call.ctx.Err() != nil || h.loader.State().Error != catalog.None {
			t.Fatal("evicted failure contaminated a deliberate new seek")
		}
	})
}

func TestLoaderNewBindingAndCanceledLifetimeCannotPublishOldResults(t *testing.T) {
	for _, late := range []loadReply{{metadata: Metadata{Known: true}}, {err: catalog.Restricted}} {
		synctest.Test(t, func(t *testing.T) {
			h := newLoadHarness(t, true)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_ = h.loader.Start(ctx)
			_ = h.loader.Bind(h.ref, time.Hour, 1)
			old := h.next(t, 0)
			other := catalog.PartRef{Video: "video", Part: "part-2"}
			_ = h.loader.Bind(other, time.Hour, 2)
			h.finish(t, old, late, 0)
			current := h.next(t, 0)
			if current.ref != other || h.loader.State().Error != catalog.None || len(h.loader.Window().Entries) != 0 {
				t.Fatal("old binding published availability/error into a new part")
			}
			h.finish(t, current, loadReply{}, 0)
			h.finish(t, h.next(t, 1), loadReply{segment: segment(1, loadEntry("new", 0, "new"))}, 0)
			cancel()
			if len(h.loader.Window().Entries) != 0 {
				t.Fatal("canceled scope published its retained text")
			}
			h.loader.Tick(h.sample(0))
			awaitLoad(t, h.loader.Done())
			if h.loader.State().Phase != LoadClosed || h.loader.State().CachedSegments != 0 {
				t.Fatal("canceled lifetime did not retire its cache")
			}
		})
	}
}

func TestLoaderCloseJoinsActualCallAndDiscardsBufferedResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, true)
		_ = h.loader.Start(context.Background())
		_ = h.loader.Bind(h.ref, time.Hour, 1)
		call := h.next(t, 0)
		h.loader.Close()
		if state := h.loader.State(); state.Phase != LoadClosed || state.ActiveRequests != 1 || state.CachedSegments != 0 || len(h.loader.Window().Entries) != 0 {
			t.Fatal("close invented zero physical ownership", state)
		}
		select {
		case <-h.loader.Done():
			t.Fatal("Done closed while Source still owned its call")
		default:
		}
		call.reply <- loadReply{}
		awaitLoad(t, h.loader.Done())
		if h.loader.State().ActiveRequests != 0 || len(h.loader.results) != 0 || len(h.loader.requests) != 0 {
			t.Fatal("source/result ownership survived Done")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		call := h.begin(t, time.Hour)
		call.reply <- loadReply{segment: segment(1, loadEntry("private", 0, "private buffered text"))}
		loadResultReady(t, h.loader)
		h.loader.Close()
		// There is deliberately no Tick or render after Close.
		awaitLoad(t, h.loader.Done())
		if len(h.loader.results) != 0 || len(h.loader.Window().Entries) != 0 || h.loader.State().TextBytes != 0 {
			t.Fatal("buffered text remained owned after terminal close")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.finish(t, h.begin(t, time.Hour), loadReply{segment: segment(1, loadEntry("owned", 0, "owned cached text"))}, 0)
		h.loader.Close()
		awaitLoad(t, h.loader.Done())
		if h.loader.slots != ([2]*segmentSlot{}) || h.loader.State().CachedSegments != 0 || h.loader.State().TextBytes != 0 {
			t.Fatal("closed state hid retained cache ownership")
		}
	})
}

func TestLoaderRejectsMalformedPortResultsTransactionally(t *testing.T) {
	for name, mutate := range map[string]func(*Segment){
		"index":                  func(s *Segment) { s.Index = 2 },
		"negative-encoded":       func(s *Segment) { s.Encoded = -1 },
		"over-encoded":           func(s *Segment) { s.Encoded = 20001 },
		"missing-encoded":        func(s *Segment) { s.Encoded = 0 },
		"negative-skips":         func(s *Segment) { s.SkippedEmpty = -1 },
		"impossible-skips":       func(s *Segment) { s.SkippedEmpty = 2 },
		"impossible-unsupported": func(s *Segment) { s.Unsupported = 2 },
		"empty-identity":         func(s *Segment) { s.Entries[0].ID = "" },
		"interval":               func(s *Segment) { s.Entries[0].Entry.At = 360*time.Second + 1 },
		"negative-time":          func(s *Segment) { s.Entries[0].Entry.At = -1 },
		"color":                  func(s *Segment) { s.Entries[0].Entry.Color = 0x1000000 },
		"mode":                   func(s *Segment) { s.Entries[0].Entry.Mode = 2 },
		"control":                func(s *Segment) { s.Entries[0].Entry.Text = "\x00" },
		"utf8":                   func(s *Segment) { s.Entries[0].Entry.Text = "\xff" },
		"text-bound":             func(s *Segment) { s.Entries[0].Entry.Text = strings.Repeat("x", 1025) },
		"contradictory-identity": func(s *Segment) { s.Encoded++; s.Entries = append(s.Entries, loadEntry("x", 0, "other")) },
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newLoadHarness(t, false)
				call := h.begin(t, time.Hour)
				data := segment(1, loadEntry("x", 0, "x"))
				mutate(&data)
				h.finish(t, call, loadReply{segment: data}, 0)
				if state := h.loader.State(); state.Phase != LoadFailed || state.Error != catalog.InvalidResponse || state.CachedSegments != 0 || len(h.loader.Window().Entries) != 0 {
					t.Fatal("malformed source result became a cache window", state)
				}
			})
		})
	}
}

func TestLoaderNeighborConflictDoesNotDestroyCurrentData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newLoadHarness(t, false)
		h.finish(t, h.begin(t, 720*time.Second), loadReply{segment: segment(1, loadEntry("boundary", 360*time.Second, "accepted"))}, 18*time.Second)
		before := h.loader.Window()
		h.finish(t, h.next(t, 2), loadReply{segment: segment(2, loadEntry("boundary", 360*time.Second, "contradiction"))}, 18*time.Second)
		after := h.loader.Window()
		if state := h.loader.State(); state.PrefetchError != catalog.InvalidResponse || state.Error != catalog.None || state.Phase != LoadReady || !reflect.DeepEqual(before.Entries, after.Entries) {
			t.Fatal("conflicting neighbor replaced accepted current data", state)
		}
	})
}

func TestLoaderRetainsExactEntryAndTextWindowLimits(t *testing.T) {
	for _, test := range []struct {
		count, text int
	}{{20000, 1}, {4096, 1024}} {
		synctest.Test(t, func(t *testing.T) {
			h := newLoadHarness(t, false)
			makeSegment := func(index, count int) Segment {
				entries := make([]SegmentEntry, count)
				for i := range entries {
					entries[i] = loadEntry(fmt.Sprintf("%d-%d", index, i), time.Duration(index-1)*360*time.Second, strings.Repeat("x", test.text))
				}
				return segment(index, entries...)
			}
			h.finish(t, h.begin(t, 720*time.Second), loadReply{segment: makeSegment(1, test.count)}, 18*time.Second)
			h.finish(t, h.next(t, 2), loadReply{segment: makeSegment(2, test.count)}, 18*time.Second)
			if state := h.loader.State(); state.CachedSegments != 2 || state.Entries != 2*test.count || state.TextBytes != 2*test.count*test.text || len(h.loader.Window().Entries) != 2*test.count {
				t.Fatal("exact valid cache bound lost data", state)
			}
			h.loader.Tick(h.sample(5 * time.Second))
			h.loader.Tick(h.sample(18 * time.Second))
			h.finish(t, h.next(t, 2), loadReply{segment: makeSegment(2, test.count+1)}, 18*time.Second)
			if state := h.loader.State(); state.PrefetchError != catalog.InvalidResponse || state.CachedSegments != 1 || state.Entries != test.count || state.TextBytes != test.count*test.text {
				t.Fatal("overflowing result partially changed the cache", state)
			}
		})
	}
}
