package danmaku

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

const loadTimeout = 10 * time.Second
const previousOverlap = 16 * time.Second

type segmentSlot struct {
	index     int
	ready     bool
	err       catalog.Code
	segment   Segment
	textBytes int
}

type loadRequest struct {
	sequence, binding uint64
	ref               catalog.PartRef
	duration          time.Duration
	index             int // zero is metadata
	ctx               context.Context
}

type loadAttempt struct {
	request loadRequest
	cancel  context.CancelFunc
}

type loadResult struct {
	request   loadRequest
	metadata  Metadata
	segment   Segment
	textBytes int
	err       error
	consumed  chan struct{}
}

// Loader owns one part's bounded window on the UI thread. Its worker produces
// immutable results; only Tick accepts them. Native generation is presentation
// identity, separate from the part-bound identity of a pending data request.
type Loader struct {
	source                        SegmentSource
	ctx                           context.Context
	cancel                        context.CancelFunc
	started, closed               bool
	ref                           catalog.PartRef
	duration, position            time.Duration
	generation, revision, binding uint64
	current                       int
	slots                         [2]*segmentSlot
	metadataReady                 bool
	metadataError                 catalog.Code
	sequence                      uint64
	pending                       *loadAttempt
	requests                      chan loadRequest
	results                       chan loadResult
	closing, done                 chan struct{}
	active                        atomic.Int32
	requestContext                func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

func NewLoader(source SegmentSource) (*Loader, error) {
	if source == nil {
		return nil, catalog.Invalid
	}
	return &Loader{source: source, requests: make(chan loadRequest, 1), results: make(chan loadResult, 1),
		closing: make(chan struct{}), done: make(chan struct{}), requestContext: context.WithTimeout}, nil
}

func (l *Loader) Start(parent context.Context) error {
	if l.closed {
		return catalog.Closed
	}
	if parent == nil {
		return catalog.Invalid
	}
	if !l.started {
		l.ctx, l.cancel = context.WithCancel(parent)
		l.started = true
		go l.work()
	}
	return l.live()
}

func (l *Loader) live() error {
	if l.closed {
		return catalog.Closed
	}
	if !l.started {
		return catalog.Invalid
	}
	if err := loadContextError(l.ctx); err != nil {
		l.Close()
		return loadCode(err)
	}
	return nil
}

func (l *Loader) Bind(ref catalog.PartRef, duration time.Duration, generation uint64) error {
	if err := l.live(); err != nil {
		return err
	}
	if ref.Video == "" || ref.Part == "" || duration <= 0 || duration > 7*24*time.Hour || generation == 0 || generation < l.generation {
		return catalog.Invalid
	}
	if ref == l.ref && duration == l.duration {
		if generation != l.generation {
			l.generation = generation
			l.revision++
		}
		return nil
	}
	l.retire()
	l.binding++
	l.ref, l.duration, l.generation = ref, duration, generation
	l.position, l.current = 0, 1
	l.slots = [2]*segmentSlot{}
	l.metadataReady, l.metadataError = false, catalog.None
	l.revision++
	l.windowAt(0)
	l.schedule()
	return nil
}

func loadContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func loadCode(err error) catalog.Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return catalog.Timeout
	}
	if errors.Is(err, context.Canceled) {
		return catalog.Canceled
	}
	var code catalog.Code
	if errors.As(err, &code) {
		switch code {
		case catalog.Invalid, catalog.Unsupported, catalog.Canceled, catalog.Timeout, catalog.Network, catalog.InvalidResponse,
			catalog.Unavailable, catalog.Restricted, catalog.LoginRequired, catalog.SessionExpired, catalog.VerificationRequired,
			catalog.RateLimited, catalog.ServiceUnavailable, catalog.QualityUnavailable, catalog.Closed:
			return code
		}
	}
	return catalog.Unavailable
}

func (l *Loader) slot(index int) *segmentSlot {
	for _, slot := range l.slots {
		if slot != nil && slot.index == index {
			return slot
		}
	}
	return nil
}

func (l *Loader) windowAt(position time.Duration) {
	position = min(position, l.duration)
	count := int((l.duration-1)/SegmentDuration) + 1
	current := min(int(position/SegmentDuration)+1, count)
	neighbor := current + 1
	if position-time.Duration(current-1)*SegmentDuration < previousOverlap {
		neighbor = current - 1
	}
	if neighbor < 1 || neighbor > count {
		neighbor = 0
	}
	if l.current == current && l.slots[0] != nil && l.slots[0].index == current &&
		(neighbor == 0 && l.slots[1] == nil || l.slots[1] != nil && l.slots[1].index == neighbor) {
		l.position = position
		return
	}
	var selected [2]*segmentSlot
	for i, index := range [2]int{current, neighbor} {
		if index != 0 {
			selected[i] = l.slot(index)
			if selected[i] == nil {
				selected[i] = &segmentSlot{index: index}
			}
		}
	}
	l.slots, l.current, l.position = selected, current, position
	l.revision++
}

// Retiring a logical request does not decrement physical ownership. Its
// worker remains the only caller until Source actually returns.
func (l *Loader) retire() {
	l.sequence++
	if l.pending != nil {
		l.pending.cancel()
		l.pending = nil
	}
	select {
	case <-l.requests:
	default:
	}
}

func (l *Loader) submit(index int) {
	l.retire()
	ctx, cancel := l.requestContext(l.ctx, loadTimeout)
	request := loadRequest{sequence: l.sequence, binding: l.binding, ref: l.ref, duration: l.duration, index: index, ctx: ctx}
	l.pending = &loadAttempt{request: request, cancel: cancel}
	l.requests <- request
}

func (l *Loader) schedule() {
	if l.closed || l.ref == (catalog.PartRef{}) {
		return
	}
	target := -1
	if !l.metadataReady {
		if l.metadataError == catalog.None {
			target = 0
		}
	} else if current := l.slot(l.current); current != nil && current.err == catalog.None {
		if !current.ready {
			target = current.index
		} else if neighbor := l.slots[1]; neighbor != nil && !neighbor.ready && neighbor.err == catalog.None {
			target = neighbor.index
		}
	}
	if l.pending != nil && l.pending.request.index == target {
		return
	}
	if target < 0 {
		if l.pending != nil {
			l.retire()
		}
		return
	}
	l.submit(target)
}

func (l *Loader) fail(request loadRequest, err error) {
	code := loadCode(err)
	if request.index == 0 {
		l.metadataError = code
	} else if slot := l.slot(request.index); slot != nil {
		slot.err = code
	}
}

func (l *Loader) matches(request loadRequest) bool {
	return !l.closed && loadContextError(l.ctx) == nil && l.pending != nil &&
		request.sequence == l.pending.request.sequence && request.binding == l.binding && request.ref == l.ref
}

func (l *Loader) accept(result loadResult) {
	defer close(result.consumed)
	if !l.matches(result.request) {
		return
	}
	err := result.err
	if cause := loadContextError(result.request.ctx); cause != nil {
		err = cause
	}
	if err == nil && result.request.index != 0 {
		err = l.validateNeighbor(result.segment)
	}
	if err != nil {
		l.fail(result.request, err)
		l.retire()
		return
	}
	l.retire()
	if result.request.index == 0 {
		if result.metadata.Known && !result.metadata.Available {
			l.metadataError = catalog.Unavailable
		} else {
			l.metadataReady = true
		}
		return
	}
	slot := l.slot(result.request.index)
	slot.ready, slot.err = true, catalog.None
	slot.segment, slot.textBytes = result.segment, result.textBytes
	l.revision++
}

func (l *Loader) validateNeighbor(incoming Segment) error {
	// Two slots, each validated at MaxSegmentEntries / MaxSegmentTextBytes,
	// already establish the timeline's aggregate entry and text limits.
	for _, slot := range l.slots {
		if slot == nil || !slot.ready || slot.index == incoming.Index {
			continue
		}
		previous := make(map[string]Entry, len(slot.segment.Entries))
		for _, entry := range slot.segment.Entries {
			previous[entry.ID] = entry.Entry
		}
		for _, entry := range incoming.Entries {
			if old, ok := previous[entry.ID]; ok && old != entry.Entry {
				return catalog.InvalidResponse
			}
		}
	}
	return nil
}

func (l *Loader) Tick(snapshot playback.Snapshot) {
	if l.closed || !l.started {
		return
	}
	if loadContextError(l.ctx) != nil {
		l.Close()
		return
	}
	if l.ref != (catalog.PartRef{}) && snapshot.Generation == l.generation && snapshot.Position >= 0 {
		switch snapshot.State {
		case playback.Ready, playback.Paused, playback.Playing:
			l.windowAt(snapshot.Position)
		}
	}
	// Current work can supersede an obsolete prefetch even while its result is
	// waiting for acknowledgment. Decide relevance before accepting that result.
	l.schedule()
	if l.pending != nil {
		if err := loadContextError(l.pending.request.ctx); err != nil {
			l.fail(l.pending.request, err)
			l.retire()
		}
	}
	select {
	case result := <-l.results:
		l.accept(result)
	default:
	}
	l.schedule()
}

func (l *Loader) Retry() error {
	if err := l.live(); err != nil {
		return err
	}
	if l.pending != nil {
		return nil
	}
	if l.metadataError != catalog.None {
		l.metadataError = catalog.None
		l.schedule()
		return nil
	}
	for _, slot := range l.slots {
		if slot != nil && slot.err != catalog.None {
			slot.err = catalog.None
			l.schedule()
			return nil
		}
	}
	return catalog.Invalid
}

func (l *Loader) State() LoadState {
	state := LoadState{Phase: LoadInitial, Ref: l.ref, Generation: l.generation, Revision: l.revision,
		Segment: l.current, ActiveRequests: int(l.active.Load())}
	for _, slot := range l.slots {
		if slot != nil && slot.ready {
			state.CachedSegments++
			state.Entries += len(slot.segment.Entries)
			state.TextBytes += slot.textBytes
		}
	}
	if l.closed {
		state.Phase = LoadClosed
		return state
	}
	if l.ref == (catalog.PartRef{}) {
		return state
	}
	state.Phase = LoadLoading
	if l.metadataError != catalog.None {
		state.Phase, state.Error = LoadFailed, l.metadataError
		return state
	}
	if !l.metadataReady {
		return state
	}
	if current := l.slot(l.current); current != nil {
		if current.err != catalog.None {
			state.Phase, state.Error = LoadFailed, current.err
		} else if current.ready {
			state.Phase = LoadReady
		}
	}
	if neighbor := l.slots[1]; neighbor != nil {
		state.PrefetchError = neighbor.err
	}
	return state
}

func (l *Loader) Window() Window {
	window := Window{Generation: l.generation, Revision: l.revision, Entries: []Entry{}}
	if l.closed || !l.started || loadContextError(l.ctx) != nil {
		return window
	}
	var merged []SegmentEntry
	seen := make(map[string]bool)
	for _, slot := range l.slots {
		if slot == nil || !slot.ready {
			continue
		}
		for _, entry := range slot.segment.Entries {
			if !seen[entry.ID] {
				seen[entry.ID] = true
				merged = append(merged, entry)
			}
		}
	}
	slices.SortFunc(merged, func(a, b SegmentEntry) int {
		if order := cmp.Compare(a.Entry.At, b.Entry.At); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	for _, entry := range merged {
		window.Entries = append(window.Entries, entry.Entry)
	}
	return window
}

func (l *Loader) discardResult() {
	select {
	case result := <-l.results:
		close(result.consumed)
	default:
	}
}

func (l *Loader) Close() {
	if l.closed {
		return
	}
	l.closed = true
	l.retire()
	l.slots = [2]*segmentSlot{}
	l.revision++
	l.metadataReady, l.metadataError = false, catalog.None
	l.discardResult()
	// UI-owned cache retirement precedes worker acknowledgment even if the
	// parent context had already canceled while the window could not paint.
	close(l.closing)
	if !l.started {
		close(l.done)
		return
	}
	l.cancel()
}

// Done acknowledges terminal Close and actual worker/result cleanup. Merely
// canceling an operation never pretends the loader's UI-owned cache is closed.
func (l *Loader) Done() <-chan struct{} { return l.done }

func (l *Loader) work() {
	defer func() {
		l.discardResult()
		select {
		case <-l.requests:
		default:
		}
		<-l.closing
		close(l.done)
	}()
	for {
		var request loadRequest
		select {
		case <-l.ctx.Done():
			return
		case request = <-l.requests:
		}
		if loadContextError(l.ctx) != nil {
			return
		}
		if loadContextError(request.ctx) != nil {
			continue
		}
		result := l.invoke(request)
		if loadContextError(l.ctx) != nil {
			return
		}
		select {
		case <-l.ctx.Done():
			return
		case l.results <- result:
		}
		select {
		case <-l.ctx.Done():
			return
		case <-result.consumed:
		}
	}
}

func (l *Loader) invoke(request loadRequest) loadResult {
	l.active.Add(1)
	defer l.active.Add(-1)
	result := loadResult{request: request, consumed: make(chan struct{})}
	if request.index == 0 {
		result.metadata, result.err = l.source.Metadata(request.ctx, request.ref)
	} else {
		result.segment, result.err = l.source.Segment(request.ctx, request.ref, request.index)
		if result.err == nil {
			result.segment, result.textBytes, result.err = ownSegment(result.segment, request.index, request.duration)
		}
	}
	return result
}

func ownSegment(segment Segment, index int, duration time.Duration) (Segment, int, error) {
	if segment.Index != index || segment.Encoded < 0 || segment.Encoded > MaxSegmentEntries ||
		segment.SkippedEmpty < 0 || segment.SkippedEmpty > segment.Encoded || segment.Unsupported < 0 ||
		segment.Unsupported > segment.Encoded-segment.SkippedEmpty ||
		len(segment.Entries) > segment.Encoded-segment.SkippedEmpty-segment.Unsupported {
		return Segment{}, 0, catalog.InvalidResponse
	}
	start, end := time.Duration(index-1)*SegmentDuration, min(time.Duration(index)*SegmentDuration, duration)
	bytes := 0
	for _, value := range segment.Entries {
		entry := value.Entry
		bytes += len(entry.Text)
		if value.ID == "" || entry.At < start || entry.At > end || entry.Color > 0xffffff ||
			len(entry.Text) > MaxTextBytes || bytes > MaxSegmentTextBytes || !utf8.ValidString(entry.Text) {
			return Segment{}, 0, catalog.InvalidResponse
		}
		switch entry.Mode {
		case ScrollRTL, FixedBottom, FixedTop, ScrollLTR:
		default:
			return Segment{}, 0, catalog.InvalidResponse
		}
		for _, r := range entry.Text {
			if unicode.IsControl(r) && !unicode.IsSpace(r) {
				return Segment{}, 0, catalog.InvalidResponse
			}
		}
	}
	owned := make([]SegmentEntry, 0, len(segment.Entries))
	seen := make(map[string]Entry, len(segment.Entries))
	bytes = 0
	for _, value := range segment.Entries {
		value.Entry.Text = normalize(value.Entry.Text)
		if previous, ok := seen[value.ID]; ok {
			if previous != value.Entry {
				return Segment{}, 0, catalog.InvalidResponse
			}
			continue
		}
		value.ID = strings.Clone(value.ID)
		seen[value.ID] = value.Entry
		owned = append(owned, value)
		bytes += len(value.Entry.Text)
	}
	segment.Entries = owned
	return segment, bytes, nil
}
