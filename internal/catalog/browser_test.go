package catalog

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type searchCall struct {
	ctx      context.Context
	query    string
	page     int
	response chan searchResult
}
type controlledSearch struct {
	calls        chan *searchCall
	abort        chan struct{}
	ignore       bool
	active, peak atomic.Int32
}

func (s *controlledSearch) Search(ctx context.Context, query string, page int) (Page, error) {
	active := s.active.Add(1)
	for old := s.peak.Load(); active > old && !s.peak.CompareAndSwap(old, active); old = s.peak.Load() {
	}
	defer s.active.Add(-1)
	call := &searchCall{ctx: ctx, query: query, page: page, response: make(chan searchResult, 1)}
	select {
	case s.calls <- call:
	case <-s.abort:
		return Page{}, context.Canceled
	}
	done := ctx.Done()
	if s.ignore {
		done = nil
	}
	select {
	case result := <-call.response:
		return result.page, result.err
	case <-done:
		return Page{}, ctx.Err()
	case <-s.abort:
		return Page{}, context.Canceled
	}
}
func browserHarness(t *testing.T, ignore bool) (*Browser, *controlledSearch) {
	t.Helper()
	source := &controlledSearch{calls: make(chan *searchCall, 64), abort: make(chan struct{}), ignore: ignore}
	b, err := NewBrowser(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close(); close(source.abort); waitBrowser(t, b) })
	return b, source
}
func waitBrowser(t *testing.T, b *Browser) {
	t.Helper()
	select {
	case <-b.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("search worker did not retire")
	}
}
func nextSearch(t *testing.T, s *controlledSearch, query string, page int) *searchCall {
	t.Helper()
	select {
	case call := <-s.calls:
		if call.query != query || call.page != page {
			t.Fatal("wrong submitted query/page")
		}
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("expected search not called")
		return nil
	}
}
func searchResultReady(t *testing.T, b *Browser) {
	t.Helper()
	select {
	case result := <-b.results:
		b.results <- result
	case <-time.After(5 * time.Second):
		t.Fatal("search result not published")
	}
}
func summary(id string) Summary {
	return Summary{ID: VideoID(id), Title: "Original title", Creator: "Creator", Duration: time.Minute}
}
func answerSearch(t *testing.T, b *Browser, call *searchCall, ids ...string) {
	t.Helper()
	page := Page{Number: call.page, TotalPages: 60, TotalResults: 1200, Videos: []Summary{}}
	for _, id := range ids {
		page.Videos = append(page.Videos, summary(id))
	}
	call.response <- searchResult{page: page}
	searchResultReady(t, b)
	b.Tick()
}

func TestBrowserExplicitActionsCopyAndPageRetry(t *testing.T) {
	b, s := browserHarness(t, false)
	if len(s.calls) != 0 || b.State().Results == nil || b.State().Phase != SearchInitial {
		t.Fatal("constructor performed search")
	}
	if b.Start(nil) != Invalid || b.Start(context.Background()) != nil {
		t.Fatal("start contract")
	}
	if b.Search(" ") != Invalid || len(s.calls) != 0 {
		t.Fatal("invalid query dispatched")
	}
	if err := b.Search(" first "); err != nil {
		t.Fatal(err)
	}
	call := nextSearch(t, s, "first", 1)
	page := Page{Number: 1, TotalPages: 3, TotalResults: 4, Videos: []Summary{summary("one"), summary("two")}}
	call.response <- searchResult{page: page}
	searchResultReady(t, b)
	page.Videos[0].Title = "mutated source"
	if b.State().Phase != SearchLoading || len(b.State().Results) != 0 {
		t.Fatal("worker published state without Tick")
	}
	b.Tick()
	state := b.State()
	state.Results[0].Title = "mutated state"
	if b.State().Results[0].Title != "Original title" {
		t.Fatal("result alias escaped")
	}
	if err := b.NextPage(); err != nil {
		t.Fatal(err)
	}
	call = nextSearch(t, s, "first", 2)
	generation := b.State().Generation
	for range 5 {
		if b.NextPage() != nil {
			t.Fatal("pending next rejected")
		}
	}
	if b.State().Generation != generation || !b.State().LoadingMore {
		t.Fatal("duplicate page queued")
	}
	call.response <- searchResult{err: fmt.Errorf("private: %w", Network)}
	searchResultReady(t, b)
	b.Tick()
	if state = b.State(); state.Page != 1 || len(state.Results) != 2 || state.Error != Network || state.LoadingMore {
		t.Fatal("failed append erased accepted page", state)
	}
	if b.Retry() != nil {
		t.Fatal("retry refused")
	}
	call = nextSearch(t, s, "first", 2)
	answerSearch(t, b, call, "two", "three", "one")
	if state = b.State(); state.Page != 2 || len(state.Results) != 3 || state.Results[2].ID != "three" || state.Error != None {
		t.Fatal("append order/dedupe/retry", state)
	}
	if b.Retry() != Invalid {
		t.Fatal("retry without failure")
	}
}

func TestBrowserLatestQueryOwnsSingleWorkerAndResult(t *testing.T) {
	b, s := browserHarness(t, true)
	_ = b.Start(context.Background())
	_ = b.Search("old")
	old := nextSearch(t, s, "old", 1)
	for i := 0; i < 40; i++ {
		_ = b.Search(fmt.Sprint("query", i))
	}
	if old.ctx.Err() == nil || s.active.Load() != 1 || s.peak.Load() != 1 || len(b.requests) > 1 {
		t.Fatal("supersession lost bounded ownership")
	}
	old.response <- searchResult{page: Page{Number: 1, TotalPages: 1, TotalResults: 1, Videos: []Summary{summary("obsolete")}}}
	searchResultReady(t, b)
	if len(s.calls) != 0 {
		t.Fatal("worker passed unacknowledged result")
	}
	b.Tick()
	latest := nextSearch(t, s, "query39", 1)
	if len(b.State().Results) != 0 {
		t.Fatal("obsolete query accepted")
	}
	answerSearch(t, b, latest, "latest")
	if b.State().Results[0].ID != "latest" || s.peak.Load() != 1 {
		t.Fatal("latest query not accepted")
	}
}

func TestBrowserMalformedPageAndSafeErrors(t *testing.T) {
	for _, mutate := range []func(*Page){
		func(p *Page) { p.Number = 2 }, func(p *Page) { p.TotalPages = -1 }, func(p *Page) { p.TotalResults = -1 },
		func(p *Page) { p.TotalResults = 1 << 32 }, func(p *Page) { p.Videos[0].Title = "" }, func(p *Page) { p.Videos[0].Creator = "" },
		func(p *Page) { p.Videos[0].ID = "" }, func(p *Page) { p.Videos[0].Duration = 0 }, func(p *Page) { p.Videos[0].CoverID = "bad\n" },
	} {
		b, s := browserHarness(t, false)
		_ = b.Start(context.Background())
		_ = b.Search("query")
		call := nextSearch(t, s, "query", 1)
		page := Page{Number: 1, TotalPages: 1, TotalResults: 1, Videos: []Summary{summary("one")}}
		mutate(&page)
		call.response <- searchResult{page: page}
		searchResultReady(t, b)
		b.Tick()
		if state := b.State(); state.Phase != SearchFailed || state.Error != InvalidResponse || len(state.Results) != 0 || state.Page != 0 {
			t.Fatal("malformed page became accepted", state)
		}
	}
	for _, test := range []struct {
		err  error
		want Code
	}{{errors.New("private"), Unavailable}, {fmt.Errorf("private:%w", context.DeadlineExceeded), Timeout}, {SessionExpired, SessionExpired}, {Code("private-code"), Unavailable}} {
		b, s := browserHarness(t, false)
		_ = b.Start(context.Background())
		_ = b.Search("query")
		nextSearch(t, s, "query", 1).response <- searchResult{err: test.err}
		searchResultReady(t, b)
		b.Tick()
		if b.State().Error != test.want {
			t.Fatal("unsafe/incorrect error", b.State().Error)
		}
	}
}

func TestBrowserEmptyAndRetentionCaps(t *testing.T) {
	b, s := browserHarness(t, false)
	_ = b.Start(context.Background())
	_ = b.Search("empty")
	call := nextSearch(t, s, "empty", 1)
	call.response <- searchResult{page: Page{Number: 1, Videos: []Summary{}}}
	searchResultReady(t, b)
	b.Tick()
	if state := b.State(); state.Phase != SearchReady || state.Page != 1 || state.Results == nil || len(state.Results) != 0 || state.Error != None {
		t.Fatal("empty page treated as error")
	}
	if b.NextPage() != nil || len(s.calls) != 0 {
		t.Fatal("empty result requested another page")
	}
	_ = b.Search("many")
	call = nextSearch(t, s, "many", 1)
	page := Page{Number: 1, TotalPages: 60, TotalResults: 5000, Videos: []Summary{}}
	for i := 0; i < 1001; i++ {
		page.Videos = append(page.Videos, summary(fmt.Sprint(i)))
	}
	call.response <- searchResult{page: page}
	searchResultReady(t, b)
	b.Tick()
	if len(b.State().Results) != 1000 || !b.State().AtLimit {
		t.Fatal("unique retention cap missing")
	}
	_ = b.NextPage()
	if len(s.calls) != 0 {
		t.Fatal("requested past retention cap")
	}
	_ = b.Search("pages")
	for i := 1; i <= 50; i++ {
		call = nextSearch(t, s, "pages", i)
		answerSearch(t, b, call, "duplicate")
		_ = b.NextPage()
	}
	if !b.State().AtLimit || b.State().Page != 50 || len(b.State().Results) != 1 || len(s.calls) != 0 {
		t.Fatal("page cap/dedupe missing")
	}
}

func TestBrowserExpiredQueuedWorkAndTerminalClose(t *testing.T) {
	b, s := browserHarness(t, true)
	_ = b.Start(context.Background())
	_ = b.Search("held")
	held := nextSearch(t, s, "held", 1)
	b.requestContext = func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		return context.WithDeadline(parent, time.Now().Add(-time.Second))
	}
	_ = b.Search("expired")
	b.Tick()
	if state := b.State(); state.Error != Timeout || state.Phase != SearchFailed || s.active.Load() != 1 {
		t.Fatal("queue deadline released ownership", state)
	}
	held.response <- searchResult{err: Canceled}
	searchResultReady(t, b)
	b.Tick()
	if len(s.calls) != 0 {
		t.Fatal("expired queued call dispatched")
	}
	b.Close()
	waitBrowser(t, b)
	if b.State().Phase != SearchClosed || b.State().Query != "" || b.State().Results == nil || len(b.results) != 0 {
		t.Fatal("close retained result data")
	}
	if b.Search("new") != Closed || b.NextPage() != Closed || b.Retry() != Closed {
		t.Fatal("closed browser accepted action")
	}

	x, source := browserHarness(t, false)
	_ = x.Start(context.Background())
	_ = x.Search("done")
	call := nextSearch(t, source, "done", 1)
	call.response <- searchResult{page: Page{Number: 1, Videos: []Summary{}}}
	searchResultReady(t, x)
	x.Close()
	waitBrowser(t, x)
	if len(x.results) != 0 {
		t.Fatal("completed result survived terminal worker")
	}
}

func TestBrowserSubmissionDeadlineAndEarlierParentArePreserved(t *testing.T) {
	b, s := browserHarness(t, true)
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_ = b.Start(parent)
	_ = b.Search("held")
	held := nextSearch(t, s, "held", 1)
	deadline, ok := held.ctx.Deadline()
	if !ok || deadline.After(time.Now().Add(10*time.Second)) {
		t.Fatal("search lacks ten-second bound")
	}
	_ = b.Search("queued")
	queuedDeadline, _ := b.pending.request.ctx.Deadline()
	held.response <- searchResult{err: Canceled}
	searchResultReady(t, b)
	b.Tick()
	queued := nextSearch(t, s, "queued", 1)
	actual, _ := queued.ctx.Deadline()
	if !actual.Equal(queuedDeadline) {
		t.Fatal("queue wait reset the submission deadline")
	}
	answerSearch(t, b, queued, "one")

	x, source := browserHarness(t, false)
	earlier, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	_ = x.Start(earlier)
	_ = x.Search("early")
	call := nextSearch(t, source, "early", 1)
	got, _ := call.ctx.Deadline()
	want, _ := earlier.Deadline()
	if !got.Equal(want) {
		t.Fatal("child extended parent deadline")
	}
}
