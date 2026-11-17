package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type callActivity struct {
	mu                  sync.Mutex
	calls, active, peak int
}
type activitySnapshot struct{ calls, active, peak int }

func (a *callActivity) begin() func() {
	a.mu.Lock()
	a.calls++
	a.active++
	a.peak = max(a.peak, a.active)
	a.mu.Unlock()
	return func() { a.mu.Lock(); a.active--; a.mu.Unlock() }
}
func (a *callActivity) snapshot() activitySnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return activitySnapshot{calls: a.calls, active: a.active, peak: a.peak}
}

type clientMetrics struct {
	source, search, api, cover callActivity
	mediaOpens                 atomic.Int64
}

type countedSource struct {
	source  app.Source
	metrics *clientMetrics
}

func (s *countedSource) Detail(ctx context.Context) (catalog.Video, error) {
	end := s.metrics.source.begin()
	defer end()
	return s.source.Detail(ctx)
}
func (s *countedSource) Resolve(ctx context.Context, request app.ResolveRequest) (app.Resolution, error) {
	end := s.metrics.source.begin()
	defer end()
	return s.source.Resolve(ctx, request)
}

type countedSearch struct {
	source  catalog.SearchSource
	metrics *clientMetrics
}

func (s *countedSearch) Search(ctx context.Context, query string, page int) (catalog.Page, error) {
	end := s.metrics.search.begin()
	defer end()
	return s.source.Search(ctx, query, page)
}

type observedOpener struct {
	delegate playback.Opener
	metrics  *clientMetrics
	accepted func(playback.Input)
}

func (o observedOpener) Open(ctx context.Context, input playback.Input) (playback.Engine, error) {
	o.metrics.mediaOpens.Add(1)
	engine, err := o.delegate.Open(ctx, input)
	if err == nil && o.accepted != nil {
		o.accepted(input)
	}
	return engine, err
}

// A request remains active through body close, rather than disappearing when
// only its headers arrive. This counts owned calls, not every OS socket/cache.
type countedTransport struct {
	delegate http.RoundTripper
	activity *callActivity
}

func (t countedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	end := t.activity.begin()
	response, err := t.delegate.RoundTrip(request)
	if response == nil || response.Body == nil {
		end()
		return response, err
	}
	response.Body = &countedBody{ReadCloser: response.Body, end: end}
	return response, err
}
func (t countedTransport) CloseIdleConnections() {
	if closer, ok := t.delegate.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type countedBody struct {
	io.ReadCloser
	end  func()
	once sync.Once
	err  error
}

func (b *countedBody) Close() error {
	b.once.Do(func() { b.err = b.ReadCloser.Close(); b.end() })
	return b.err
}
func clientHTTPTransport() *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	return &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second, DisableKeepAlives: true, Protocols: protocols}
}
