package transport

import (
	"context"
	"my-bilibili/internal/catalog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func latch() (<-chan struct{}, func()) {
	ch := make(chan struct{})
	var once sync.Once
	return ch, func() { once.Do(func() { close(ch) }) }
}
func eventually(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("controlled state did not become ready")
}

func TestPreparationMergesNeedsAndWaitersOwnTheirCancellation(t *testing.T) {
	entered := make(chan context.Context, 1)
	release, open := latch()
	defer open()
	var navs, homes, plays, searches atomic.Int32
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/x/web-interface/nav":
			navs.Add(1)
			entered <- req.Context()
			<-release
			return response(anonymousNav), nil
		case "/":
			homes.Add(1)
			r := response("homepage")
			r.Header["Set-Cookie"] = []string{browserLine}
			return r, nil
		case "/x/player/wbi/playurl":
			plays.Add(1)
		case "/x/web-interface/wbi/search/type":
			searches.Add(1)
		default:
			t.Error("unplanned preparation route")
		}
		return response(`{"code":0,"data":{}}`), nil
	})
	s := bind(t, c)
	a, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	aDone, bDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.JSON(a, PlayURL, playParams()); aDone <- err }()
	work := await(t, entered)
	aDeadline, _ := a.Deadline()
	workDeadline, ok := work.Deadline()
	if !ok || !workDeadline.After(aDeadline) || time.Until(workDeadline) > contractRequestLimit {
		t.Fatal("preparation inherited first waiter's deadline")
	}
	go func() { _, err := s.JSON(context.Background(), Search, searchParams()); bDone <- err }()
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return s.preparing != nil && s.preparing.needs == wbiPreparation|browserPreparation
	})
	cancel()
	if await(t, aDone) != catalog.Canceled || work.Err() != nil {
		t.Fatal("canceling A canceled shared preparation")
	}
	open()
	if await(t, bDone) != nil || navs.Load() != 1 || homes.Load() != 1 || searches.Load() != 1 || plays.Load() != 0 {
		t.Fatal("merged preparation duplicated work or lost B")
	}
}

func TestPreparationFailureHasNoImplicitRetry(t *testing.T) {
	entered := make(chan struct{}, 1)
	release, open := latch()
	defer open()
	var navs, homes, plays atomic.Int32
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/x/web-interface/nav":
			if navs.Add(1) == 1 {
				entered <- struct{}{}
				<-release
				return response(`{"code":-412}`), nil
			}
			return response(anonymousNav), nil
		case "/":
			homes.Add(1)
		case "/x/player/wbi/playurl":
			plays.Add(1)
		default:
			t.Error("failed setup started an endpoint")
		}
		return response(`{"code":0,"data":{}}`), nil
	})
	s := bind(t, c)
	aDone, bDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := s.JSON(context.Background(), PlayURL, playParams()); aDone <- err }()
	await(t, entered)
	go func() { _, err := s.JSON(context.Background(), Search, searchParams()); bDone <- err }()
	eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return s.preparing.needs == wbiPreparation|browserPreparation
	})
	open()
	if await(t, aDone) != catalog.VerificationRequired || await(t, bDone) != catalog.VerificationRequired || navs.Load() != 1 || homes.Load() != 0 || plays.Load() != 0 {
		t.Fatal("failed preparation was retried or hidden")
	}
	if _, err := s.JSON(context.Background(), PlayURL, playParams()); err != nil || navs.Load() != 2 || plays.Load() != 1 {
		t.Fatal("later explicit operation could not prepare afresh")
	}
}

func TestPreparationCachesExpireAndAreScopePrivate(t *testing.T) {
	var now atomic.Int64
	now.Store(1700000000)
	var navs, homes atomic.Int32
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/x/web-interface/nav":
			navs.Add(1)
			if req.Header.Get("Cookie") == "" {
				return response(anonymousNav), nil
			}
			return response(signedNav), nil
		case "/":
			homes.Add(1)
			r := response("homepage")
			r.Header["Set-Cookie"] = []string{browserLine + "; Max-Age=2"}
			return r, nil
		}
		return response(`{"code":0,"data":{}}`), nil
	})
	c.now = func() time.Time { return time.Unix(now.Load(), 0) }
	s := bind(t, c)
	search := func() {
		t.Helper()
		if _, err := s.JSON(context.Background(), Search, searchParams()); err != nil {
			t.Fatal("controlled cached search failed")
		}
	}
	search()
	now.Add(1)
	search()
	if navs.Load() != 1 || homes.Load() != 1 {
		t.Fatal("valid cache was not reused")
	}
	now.Add(1)
	search()
	if navs.Load() != 1 || homes.Load() != 2 {
		t.Fatal("expired browser cookies remained usable or refreshed WBI unnecessarily")
	}
	now.Store(1700000900)
	search()
	if navs.Load() != 2 || homes.Load() != 3 {
		t.Fatal("WBI TTL was not enforced")
	}
	if c.Activate(1, testMaterial()) != nil {
		t.Fatal("activation failed")
	}
	if s.keys.mixin != "" || s.browser.cookies != nil || s.Context().Err() == nil {
		t.Fatal("activation retained old scope cache")
	}
	s = bind(t, c)
	search()
	if navs.Load() != 3 || homes.Load() != 4 {
		t.Fatal("new account reused anonymous private setup")
	}
}

func TestMissingBrowserCookieAndRedirectCannotReachSearch(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		var calls atomic.Int32
		c := testClient(t, func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			if req.URL.Path == "/x/web-interface/nav" {
				return response(anonymousNav), nil
			}
			if req.URL.Host != "www.bilibili.com" || req.Header.Get("Cookie") != "" {
				t.Error("search or credential-bearing bootstrap followed failed setup")
			}
			r := response("homepage")
			if redirect {
				r.StatusCode = 302
				r.Header.Set("Location", "https://private.invalid/"+private)
			}
			return r, nil
		})
		_, err := bind(t, c).JSON(context.Background(), Search, searchParams())
		want := catalog.ServiceUnavailable
		if redirect {
			want = catalog.InvalidResponse
		}
		if err != want || calls.Load() != 2 {
			t.Fatal("failed browser prep became success or fallback")
		}
	}
}

func TestCloseJoinsPreparationWithoutWaitingOnItsCaller(t *testing.T) {
	entered := make(chan struct{}, 1)
	release, open := latch()
	defer open()
	var calls atomic.Int32
	body := &observedBody{Reader: strings.NewReader(anonymousNav)}
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return &http.Response{StatusCode: 200, Body: body}, nil
	})
	s := bind(t, c)
	caller := make(chan error, 1)
	go func() { _, err := s.JSON(context.Background(), Search, searchParams()); caller <- err }()
	await(t, entered)
	c.Close()
	if s.Context().Err() == nil {
		t.Fatal("close did not synchronously cancel scope")
	}
	select {
	case <-c.Done():
		t.Fatal("Done abandoned preparation")
	default:
	}
	err := await(t, caller)
	if err != catalog.Canceled && err != catalog.Closed {
		t.Fatal("closed caller did not retire")
	}
	select {
	case <-c.Done():
		t.Fatal("Done closed while preparation still held its response")
	case <-time.After(20 * time.Millisecond):
	}
	open()
	await(t, c.Done())
	if calls.Load() != 1 || body.closed.Load() != 1 || s.keys.mixin != "" || s.browser.cookies != nil {
		t.Fatal("closed preparation started dependent work or retained cache/body")
	}
}

func TestLatePreparationCannotPublishIntoReplacementScope(t *testing.T) {
	entered := make(chan struct{}, 1)
	release, open := latch()
	defer open()
	var navs atomic.Int32
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/x/web-interface/nav" {
			if navs.Add(1) == 1 {
				entered <- struct{}{}
				<-release
				return response(signedNav), nil
			}
			return response(strings.ReplaceAll(signedNav, imgKey, strings.Repeat("f", 32))), nil
		}
		return response(`{"code":0,"data":{}}`), nil
	})
	_ = c.Activate(1, testMaterial())
	a := bind(t, c)
	aDone := make(chan error, 1)
	go func() { _, err := a.JSON(context.Background(), PlayURL, playParams()); aDone <- err }()
	await(t, entered)
	c.mu.Lock()
	oldPreparation := a.preparing
	c.mu.Unlock()
	_ = c.Activate(2, testMaterial())
	b := bind(t, c)
	if _, err := b.JSON(context.Background(), PlayURL, playParams()); err != nil {
		t.Fatal("new scope could not prepare")
	}
	before := b.keys.mixin
	open()
	if await(t, aDone) != catalog.Canceled {
		t.Fatal("retired preparation caller accepted data")
	}
	await(t, oldPreparation.done)
	if before == "" || b.keys.mixin != before || a.keys.mixin != "" || navs.Load() != 2 {
		t.Fatal("old setup replaced new keys or was reused")
	}
}

func TestPreparationAndHTTPDoNotExtendOperationDeadline(t *testing.T) {
	var prepDeadline, endpointDeadline time.Time
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Error("request has no deadline")
		}
		if req.URL.Path == "/x/web-interface/nav" {
			prepDeadline = deadline
			return response(anonymousNav), nil
		}
		endpointDeadline = deadline
		return response(`{"code":0,"data":{}}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := bind(t, c).JSON(ctx, PlayURL, playParams()); err != nil {
		t.Fatal("controlled operation failed")
	}
	if endpointDeadline.IsZero() || endpointDeadline.After(prepDeadline) || time.Until(prepDeadline) > contractRequestLimit {
		t.Fatal("preparation or raw HTTP extended submission deadline")
	}
}
