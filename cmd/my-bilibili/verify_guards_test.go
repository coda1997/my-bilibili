package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type verifyAccountSpy struct {
	loads, saves, deletes, restores, begins, cancels, retrySaves, verifies, logouts, closes int
	loadErr                                                                                 error
}

func (s *verifyAccountSpy) Load() (account.Material, error) {
	s.loads++
	return account.Material{}, s.loadErr
}
func (s *verifyAccountSpy) Save(account.Material) error { s.saves++; return nil }
func (s *verifyAccountSpy) Delete() error               { s.deletes++; return nil }
func (*verifyAccountSpy) Snapshot() account.State {
	return account.State{Phase: account.SignedIn, Stored: true, Restored: true, SessionEpoch: 1}
}
func (s *verifyAccountSpy) Restore() error         { s.restores++; return nil }
func (s *verifyAccountSpy) Begin() error           { s.begins++; return nil }
func (s *verifyAccountSpy) Cancel() error          { s.cancels++; return nil }
func (s *verifyAccountSpy) RetrySave() error       { s.retrySaves++; return nil }
func (s *verifyAccountSpy) RetryVerify() error     { s.verifies++; return nil }
func (s *verifyAccountSpy) Logout() error          { s.logouts++; return nil }
func (s *verifyAccountSpy) RequestClose(bool) bool { s.closes++; return true }

func TestVerificationAccountRestoresButCannotMutateOrBeginQR(t *testing.T) {
	r := &verificationRun{}
	spy := &verifyAccountSpy{}
	store := r.AccountStore(spy)
	pane := r.AccountController(spy)
	if _, err := store.Load(); err != nil || store.Save(account.Material{}) != account.ServiceRejected || store.Delete() != account.ServiceRejected {
		t.Fatal("restore-only Store boundary failed")
	}
	if pane.Begin() != account.ServiceRejected || pane.RetrySave() != account.ServiceRejected || pane.Logout() != account.ServiceRejected || pane.DiscardUnsaved() != account.ServiceRejected || pane.RequestClose(true) {
		t.Fatal("mutating account action was admitted")
	}
	if pane.Restore() != nil || pane.RetryVerify() != nil || pane.Cancel() != nil || !pane.RequestClose(false) || !pane.Snapshot().Restored {
		t.Fatal("ordinary restore/verification/cancel/close did not delegate")
	}
	if spy.loads != 1 || spy.saves != 0 || spy.deletes != 0 || spy.begins != 0 || spy.retrySaves != 0 || spy.logouts != 0 || spy.restores != 1 || spy.verifies != 1 || spy.cancels != 1 || spy.closes != 1 {
		t.Fatal("verification reached a forbidden account operation")
	}
	spy.loadErr = errors.New(verifyTestPrivate)
	if _, err := store.Load(); err != account.Storage {
		t.Fatal("raw account Store error escaped")
	}
	spy.loadErr = account.Missing
	if _, err := store.Load(); err != account.Missing {
		t.Fatal("missing stored login lost safe meaning")
	}
	if _, err := r.AccountStore(nil).Load(); err != account.Storage || r.AccountController(nil).Restore() != account.Invalid {
		t.Fatal("missing verification dependency became usable")
	}
}

type verificationRTFunc func(*http.Request) (*http.Response, error)

func (f verificationRTFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type verificationOpenFunc func(context.Context, playback.Input) (playback.Engine, error)

func (f verificationOpenFunc) Open(ctx context.Context, in playback.Input) (playback.Engine, error) {
	return f(ctx, in)
}

type verificationFakeEngine struct {
	closed   int
	closeErr error
}

func (*verificationFakeEngine) Snapshot() playback.Snapshot {
	return playback.Snapshot{State: playback.Paused}
}
func (*verificationFakeEngine) Command(playback.Command) error { return nil }
func (e *verificationFakeEngine) Close() error                 { e.closed++; return e.closeErr }

type verificationCloseBody struct {
	io.Reader
	closed   atomic.Int32
	closeErr error
}

func (b *verificationCloseBody) Close() error { b.closed.Add(1); return b.closeErr }
func verifyTestBudget(t *testing.T, counts verificationCounts, launch int) *verificationRun {
	t.Helper()
	now := time.Now()
	r := &verificationRun{plan: verificationPlan{Launch: launch}, started: now, now: time.Now, budget: newVerificationBudget(counts, launch, now.Add(verificationProcessLimit))}
	t.Cleanup(r.Stop)
	return r
}
func verifyTestRequest(t *testing.T, location string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, location, nil)
	if err != nil {
		t.Fatal("synthetic request failed")
	}
	return req
}
func verifyTestResponse() *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{}}`))}
}

func TestVerificationWholePlanBudgetsDoNotResetAcrossLaunches(t *testing.T) {
	for _, kind := range []string{"api", "bootstrap", "cover", "media"} {
		t.Run(kind, func(t *testing.T) {
			r := verifyTestBudget(t, verificationCounts{APIRequests: 23, BootstrapRequests: 1, CoverRequests: 3, MediaOpens: 3}, 2)
			calls := 0
			base := verificationRTFunc(func(*http.Request) (*http.Response, error) { calls++; return verifyTestResponse(), nil })
			perform := func() error {
				if kind == "media" {
					_, err := r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) {
						calls++
						return &verificationFakeEngine{}, nil
					})).Open(context.Background(), playback.Input{})
					return err
				}
				wrapped := r.APITransport(base)
				location := "https://api.bilibili.com/x/web-interface/nav"
				if kind == "bootstrap" {
					location = "https://www.bilibili.com/"
				}
				if kind == "cover" {
					wrapped = r.CoverTransport(base)
					location = "https://cover.example.test/original.png"
				}
				response, err := wrapped.RoundTrip(verifyTestRequest(t, location))
				if err == nil {
					err = response.Body.Close()
				}
				return err
			}
			if perform() != nil || perform() == nil || calls != 1 {
				t.Fatal("remaining whole-plan allowance reset or overflow delegated")
			}
			state := r.budget.snapshot()
			if state.APIRequests > 24 || state.BootstrapRequests > 2 || state.CoverRequests > 4 || state.MediaOpens > 4 || state.Error == catalog.None {
				t.Fatal("budget overflow became successful bounded state")
			}
		})
	}
	for _, launch := range []int{1, 2} {
		r := verifyTestBudget(t, verificationCounts{}, launch)
		calls := 0
		opener := r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) {
			calls++
			return &verificationFakeEngine{}, nil
		}))
		limit := 3
		if launch == 2 {
			limit = 1
		}
		for range limit {
			if _, err := opener.Open(context.Background(), playback.Input{}); err != nil {
				t.Fatal("allowed native attempt rejected")
			}
		}
		if _, err := opener.Open(context.Background(), playback.Input{}); err == nil || calls != limit {
			t.Fatal("per-launch media limit bypassed")
		}
	}
}

func TestVerificationTransportOnlyAdmitsFixedRoutesAndCookieFreeCoversBootstrap(t *testing.T) {
	for _, path := range []string{"/x/web-interface/nav", "/x/web-interface/wbi/search/type", "/x/web-interface/view", "/x/player/wbi/playurl", "/x/v2/dm/web/view", "/x/v2/dm/web/seg.so"} {
		r := verifyTestBudget(t, verificationCounts{}, 1)
		calls := 0
		rt := r.APITransport(verificationRTFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) > 10*time.Second {
				t.Error("raw request deadline was extended")
			}
			return verifyTestResponse(), nil
		}))
		response, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com"+path))
		if err != nil || response.Body.Close() != nil || calls != 1 || r.budget.snapshot().APIActive != 0 {
			t.Fatal("fixed route or body lifetime failed")
		}
	}
	for _, kind := range []string{"route", "host", "http", "post", "fragment", "user", "body", "bootstrap-cookie", "cover-cookie", "cover-auth", "cover-http", "cover-port", "cover-escape"} {
		r := verifyTestBudget(t, verificationCounts{}, 1)
		calls := 0
		base := verificationRTFunc(func(*http.Request) (*http.Response, error) { calls++; return verifyTestResponse(), nil })
		rt := r.APITransport(base)
		req := verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav")
		switch kind {
		case "route":
			req.URL.Path = "/not-reviewed"
		case "host":
			req.URL.Host = "other.example.test"
		case "http":
			req.URL.Scheme = "http"
		case "post":
			req.Method = http.MethodPost
		case "fragment":
			req.URL.Fragment = verifyTestPrivate
		case "user":
			req.URL.User = url.User(verifyTestPrivate)
		case "body":
			req.Body = io.NopCloser(strings.NewReader(verifyTestPrivate))
		case "bootstrap-cookie":
			req = verifyTestRequest(t, "https://www.bilibili.com/")
			req.Header.Set("Cookie", verifyTestPrivate)
		case "cover-cookie", "cover-auth", "cover-http", "cover-port", "cover-escape":
			rt = r.CoverTransport(base)
			req = verifyTestRequest(t, "https://cover.example.test/original.png")
			if kind == "cover-http" {
				req.URL.Scheme = "http"
			} else if kind == "cover-port" {
				req.URL.Host = "cover.example.test:0"
			} else if kind == "cover-escape" {
				req.URL.RawQuery = "bad=%0a"
			} else if kind == "cover-cookie" {
				req.Header.Set("Cookie", verifyTestPrivate)
			} else {
				req.Header.Set("Authorization", verifyTestPrivate)
			}
		}
		if response, err := rt.RoundTrip(req); err != catalog.Invalid || response != nil || calls != 0 {
			t.Fatal("unplanned route/destination/material delegated")
		}
	}
	r := verifyTestBudget(t, verificationCounts{}, 1)
	rt := r.APITransport(verificationRTFunc(func(*http.Request) (*http.Response, error) { return verifyTestResponse(), nil }))
	var responses []*http.Response
	for range 4 {
		response, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav"))
		if err != nil {
			t.Fatal("four active requests rejected")
		}
		responses = append(responses, response)
	}
	if _, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav")); err == nil {
		t.Fatal("active request cap exceeded")
	}
	for _, response := range responses {
		if response.Body.Close() != nil {
			t.Fatal("active body cleanup failed")
		}
	}
}

func verifyTestAwait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("owned verification operation did not finish")
	}
	var zero T
	return zero
}
func TestVerificationBudgetCancellationJoinsRequestsAndNeverExtendsCallerDeadline(t *testing.T) {
	for _, stopRun := range []bool{false, true} {
		r := verifyTestBudget(t, verificationCounts{}, 1)
		entered := make(chan struct{})
		done := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		deadline, _ := ctx.Deadline()
		rt := r.APITransport(verificationRTFunc(func(req *http.Request) (*http.Response, error) {
			end, ok := req.Context().Deadline()
			if !ok || end != deadline {
				t.Error("earlier caller deadline extended")
			}
			close(entered)
			<-req.Context().Done()
			return nil, errors.New(verifyTestPrivate)
		}))
		go func() {
			req := verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav").WithContext(ctx)
			_, err := rt.RoundTrip(req)
			done <- err
		}()
		verifyTestAwait(t, entered)
		if stopRun {
			r.Stop()
		}
		err := verifyTestAwait(t, done)
		if stopRun && err != catalog.Canceled || !stopRun && err != catalog.Timeout {
			t.Fatal("request cancellation/deadline lost safe category")
		}
		if r.budget.snapshot().APIActive != 0 {
			t.Fatal("canceled delegate or context callback remained owned")
		}
		if stopRun && r.budget.snapshot().Error != catalog.None {
			t.Fatal("ordinary close cancellation invented a new refusal")
		}
	}
}

func TestVerificationRefusalAndCleanupFailuresRemainLatched(t *testing.T) {
	for _, status := range []int{302, 403, 429, 503} {
		r := verifyTestBudget(t, verificationCounts{}, 1)
		calls := 0
		rt := r.APITransport(verificationRTFunc(func(*http.Request) (*http.Response, error) {
			calls++
			response := verifyTestResponse()
			response.StatusCode = status
			return response, nil
		}))
		response, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav"))
		if err != nil || response.Body.Close() != nil {
			t.Fatal("status body did not remain caller-owned")
		}
		if _, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav")); err == nil || calls != 1 || r.budget.snapshot().Error == catalog.None {
			t.Fatal("unexpected denial permitted an automatic retry")
		}
	}
	r := verifyTestBudget(t, verificationCounts{}, 1)
	body := &verificationCloseBody{Reader: strings.NewReader("synthetic"), closeErr: errors.New(verifyTestPrivate)}
	rt := r.APITransport(verificationRTFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
	}))
	response, err := rt.RoundTrip(verifyTestRequest(t, "https://api.bilibili.com/x/web-interface/nav"))
	if err != nil {
		t.Fatal("test body request failed")
	}
	if response.Body.Close() != catalog.Network || response.Body.Close() != catalog.Network || body.closed.Load() != 1 || r.budget.snapshot().APIActive != 1 || r.budget.snapshot().Error != catalog.Network {
		t.Fatal("uncertain body close became zero ownership or safe success")
	}
	for _, code := range []playback.ErrorCode{playback.Unsupported, playback.Timeout, playback.Canceled, playback.Media} {
		r := verifyTestBudget(t, verificationCounts{}, 1)
		engine := &verificationFakeEngine{}
		opened, err := r.Opener(verificationOpenFunc(func(context.Context, playback.Input) (playback.Engine, error) { return engine, code })).Open(context.Background(), playback.Input{})
		if opened != nil || err != code || engine.closed != 1 || r.budget.snapshot().Error == catalog.None {
			t.Fatal("native error/category or failed-opener ownership lost")
		}
	}
}

func TestVerificationTransportPreservesServiceRefusalWithPlayableData(t *testing.T) {
	r := verifyTestBudget(t, verificationCounts{}, 1)
	calls := 0
	base := verificationRTFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"code":-403,"data":{"dash":{"video":[{"id":80,"codecid":7,"codecs":"avc1.640028","mimeType":"video/mp4","width":1920,"height":1080,"bandwidth":2000000,"baseUrl":"https://video.example.test/only-synthetic.m4s"}],"audio":null}}}`
		if req.URL.Path == "/x/web-interface/nav" {
			body = `{"code":-101,"data":{"isLogin":false,"wbi_img":{"img_url":"https://keys.example.test/0123456789abcdef0123456789abcdef.png","sub_url":"https://keys.example.test/fedcba9876543210fedcba9876543210.png"}}}`
		} else if req.URL.Path != "/x/player/wbi/playurl" {
			t.Error("unexpected synthetic endpoint")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client := transport.New(r.APITransport(base))
	scope, err := client.Bind()
	if err != nil {
		t.Fatal("test scope failed")
	}
	data, err := scope.JSON(context.Background(), transport.PlayURL, url.Values{"bvid": {verifyTestVideo}, "cid": {"101"}, "fnver": {"0"}, "fnval": {"16"}, "qn": {"64"}})
	if err != catalog.Restricted || data != nil || calls != 2 || r.budget.snapshot().APIRequests != 2 {
		t.Fatal("budget wrapper bypassed actual transport refusal")
	}
	client.Close()
	verifyTestAwait(t, client.Done())
}
