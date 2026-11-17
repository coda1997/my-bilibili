package catalog

import (
	"context"
	"errors"
	"math"
	"time"
)

const searchTimeout = 10 * time.Second

type searchRequest struct {
	generation uint64
	query      string
	page       int
	ctx        context.Context
}
type searchAttempt struct {
	request searchRequest
	cancel  context.CancelFunc
}
type searchResult struct {
	request  searchRequest
	page     Page
	err      error
	consumed chan struct{}
}

// Browser owns UI state. Its single worker transfers immutable responses and
// waits for acknowledgment; it never publishes results or starts native work.
type Browser struct {
	source          SearchSource
	state           SearchState
	started, closed bool
	ctx             context.Context
	cancel          context.CancelFunc
	requests        chan searchRequest
	results         chan searchResult
	done            chan struct{}
	pending         *searchAttempt
	retry           *searchRequest
	generation      uint64
	requestContext  func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

func NewBrowser(source SearchSource) (*Browser, error) {
	if source == nil {
		return nil, Invalid
	}
	return &Browser{source: source, state: SearchState{Phase: SearchInitial, Results: []Summary{}},
		requests: make(chan searchRequest, 1), results: make(chan searchResult, 1), done: make(chan struct{}), requestContext: context.WithTimeout}, nil
}

func (b *Browser) Start(ctx context.Context) error {
	if b.closed {
		return Closed
	}
	if ctx == nil {
		return Invalid
	}
	if b.started {
		return b.active()
	}
	b.started = true
	b.ctx, b.cancel = context.WithCancel(ctx)
	go b.work()
	return nil
}

func (b *Browser) active() error {
	if b.closed {
		return Closed
	}
	if !b.started {
		return Invalid
	}
	b.Tick()
	if b.closed {
		return Closed
	}
	return nil
}

func (b *Browser) Search(query string) error {
	query, err := ValidateQuery(query)
	if err != nil {
		return err
	}
	if err := b.active(); err != nil {
		return err
	}
	b.state = SearchState{Phase: SearchLoading, Query: query, Results: []Summary{}}
	b.submit(query, 1)
	return nil
}

func (b *Browser) NextPage() error {
	if err := b.active(); err != nil {
		return err
	}
	if b.pending != nil {
		return nil
	}
	if b.state.Phase != SearchReady {
		return Invalid
	}
	if b.state.AtLimit || b.state.Page >= b.state.TotalPages {
		return nil
	}
	b.submit(b.state.Query, b.state.Page+1)
	return nil
}

func (b *Browser) Retry() error {
	if err := b.active(); err != nil {
		return err
	}
	if b.pending != nil {
		return nil
	}
	if b.retry == nil {
		return Invalid
	}
	b.submit(b.retry.query, b.retry.page)
	return nil
}

func (b *Browser) retire() {
	b.generation++
	if b.pending != nil {
		b.pending.cancel()
		b.pending = nil
	}
	select {
	case <-b.requests:
	default:
	}
}

func (b *Browser) submit(query string, page int) {
	b.retire()
	ctx, cancel := b.requestContext(b.ctx, searchTimeout)
	request := searchRequest{generation: b.generation, query: query, page: page, ctx: ctx}
	b.pending, b.retry = &searchAttempt{request: request, cancel: cancel}, nil
	b.state.Generation, b.state.Error = request.generation, None
	if page == 1 {
		b.state.Phase = SearchLoading
	} else {
		b.state.Phase, b.state.LoadingMore = SearchReady, true
	}
	b.requests <- request
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func copySummaries(values []Summary) []Summary {
	result := make([]Summary, len(values))
	copy(result, values)
	return result
}

func (b *Browser) work() {
	defer func() {
		// Close owns the drain once Tick stops accepting. A racing canceled send
		// must not leave query/result data buffered after Done.
		select {
		case <-b.results:
		default:
		}
		close(b.done)
	}()
	for {
		var request searchRequest
		select {
		case <-b.ctx.Done():
			return
		case request = <-b.requests:
		}
		if contextError(b.ctx) != nil {
			return
		}
		if contextError(request.ctx) != nil {
			continue
		}
		page, err := b.source.Search(request.ctx, request.query, request.page)
		page.Videos = copySummaries(page.Videos)
		result := searchResult{request: request, page: page, err: err, consumed: make(chan struct{})}
		select {
		case <-b.ctx.Done():
			return
		case b.results <- result:
		}
		select {
		case <-b.ctx.Done():
			return
		case <-result.consumed:
		}
	}
}

func sourceError(err error) Code {
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	var code Code
	if errors.As(err, &code) {
		switch code {
		case Invalid, Unsupported, Canceled, Timeout, Network, InvalidResponse, Unavailable, Restricted,
			LoginRequired, SessionExpired, VerificationRequired, RateLimited, ServiceUnavailable, QualityUnavailable, Closed:
			return code
		}
	}
	return Unavailable
}

func validPage(page Page, request searchRequest) bool {
	if page.Number != request.page || page.TotalPages < 0 || page.TotalPages > math.MaxInt32 || page.TotalResults < 0 || page.TotalResults > math.MaxInt32 {
		return false
	}
	for _, video := range page.Videos {
		if !nonemptyIdentity(string(video.ID)) || !boundedText(video.Title, 1, 512) || !boundedText(video.Creator, 1, 256) ||
			!plainText(video.CoverID) || video.Duration <= 0 || video.Duration > 7*24*time.Hour {
			return false
		}
	}
	return true
}

func (b *Browser) failed(request searchRequest, err error) {
	b.state.Phase, b.state.LoadingMore, b.state.Error = SearchFailed, false, sourceError(err)
	b.retry = &request
	b.retire()
}

func (b *Browser) accept(result searchResult) {
	defer close(result.consumed)
	if b.pending == nil || b.pending.request.generation != result.request.generation {
		return
	}
	err := result.err
	if failure := contextError(result.request.ctx); failure != nil {
		err = failure
	}
	if err == nil && !validPage(result.page, result.request) {
		err = InvalidResponse
	}
	if failure := contextError(result.request.ctx); failure != nil {
		err = failure
	}
	if err != nil {
		b.failed(result.request, err)
		return
	}
	seen := make(map[VideoID]bool, len(b.state.Results))
	for _, video := range b.state.Results {
		seen[video.ID] = true
	}
	for _, video := range result.page.Videos {
		if seen[video.ID] {
			continue
		}
		if len(b.state.Results) == 1000 {
			break
		}
		b.state.Results = append(b.state.Results, video)
		seen[video.ID] = true
	}
	b.state.Page, b.state.TotalPages, b.state.TotalResults = result.page.Number, result.page.TotalPages, result.page.TotalResults
	b.state.Phase, b.state.LoadingMore, b.state.Error = SearchReady, false, None
	b.state.AtLimit = b.state.Page >= 50 || len(b.state.Results) >= 1000
	b.retire()
}

func (b *Browser) Tick() {
	if !b.started || b.closed {
		return
	}
	if contextError(b.ctx) != nil {
		b.Close()
		return
	}
	if b.pending != nil {
		if err := contextError(b.pending.request.ctx); err != nil {
			b.failed(b.pending.request, err)
		}
	}
	select {
	case result := <-b.results:
		b.accept(result)
	default:
	}
}

func (b *Browser) State() SearchState {
	state := b.state
	state.Results = copySummaries(state.Results)
	return state
}
func (b *Browser) Done() <-chan struct{} { return b.done }
func (b *Browser) Close() {
	if b.closed {
		return
	}
	b.closed = true
	b.retire()
	b.retry = nil
	b.state = SearchState{Phase: SearchClosed, Generation: b.generation, Results: []Summary{}}
	if b.cancel != nil {
		b.cancel()
	}
	if !b.started {
		close(b.done)
	}
}
