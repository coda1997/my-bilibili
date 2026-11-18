package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type verificationAccountStore struct{ store account.Store }

func (s verificationAccountStore) Load() (account.Material, error) {
	if s.store == nil {
		return account.Material{}, account.Storage
	}
	material, err := s.store.Load()
	if err != nil {
		if err == account.Missing {
			return account.Material{}, account.Missing
		}
		return account.Material{}, account.Storage
	}
	return material, nil
}
func (verificationAccountStore) Save(account.Material) error { return account.ServiceRejected }
func (verificationAccountStore) Delete() error               { return account.ServiceRejected }
func (*verificationRun) AccountStore(store account.Store) account.Store {
	return verificationAccountStore{store: store}
}

type verificationAccount struct{ controller account.Controller }

func (*verificationRun) AccountController(controller account.Controller) *verificationAccount {
	return &verificationAccount{controller: controller}
}
func (v *verificationAccount) Snapshot() account.State {
	if v.controller == nil {
		return account.State{Phase: account.Failed, Error: account.Invalid}
	}
	return v.controller.Snapshot()
}
func (v *verificationAccount) Restore() error {
	if v.controller == nil {
		return account.Invalid
	}
	return v.controller.Restore()
}
func (v *verificationAccount) RetryVerify() error {
	if v.controller == nil {
		return account.Invalid
	}
	return v.controller.RetryVerify()
}
func (v *verificationAccount) Cancel() error {
	if v.controller == nil {
		return account.Invalid
	}
	return v.controller.Cancel()
}
func (v *verificationAccount) RequestClose(discard bool) bool {
	return !discard && v.controller != nil && v.controller.RequestClose(false)
}
func (*verificationAccount) Begin() error          { return account.ServiceRejected }
func (*verificationAccount) RetrySave() error      { return account.ServiceRejected }
func (*verificationAccount) Logout() error         { return account.ServiceRejected }
func (*verificationAccount) DiscardUnsaved() error { return account.ServiceRejected }

type verificationCounts struct {
	APIRequests, BootstrapRequests, CoverRequests, MediaOpens int
}
type verificationBudgetState struct {
	verificationCounts
	APIActive, CoverActive int
	Error                  catalog.Code
	Stopped                bool
}
type verificationBudget struct {
	mu                       sync.Mutex
	state                    verificationBudgetState
	ctx                      context.Context
	cancel                   context.CancelFunc
	launchOpens, launchLimit int
}

func newVerificationBudget(counts verificationCounts, launch int, deadline time.Time) *verificationBudget {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	limit := 3
	if launch == 2 {
		limit = 1
	}
	return &verificationBudget{state: verificationBudgetState{verificationCounts: counts}, ctx: ctx, cancel: cancel, launchLimit: limit}
}
func (b *verificationBudget) snapshot() verificationBudgetState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
func (b *verificationBudget) fail(code catalog.Code) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state.Error == catalog.None {
		b.state.Error = code
	}
}
func (b *verificationBudget) stop() {
	b.mu.Lock()
	b.state.Stopped = true
	b.mu.Unlock()
	b.cancel()
}
func verificationContextError(ctx context.Context) catalog.Code {
	if ctx.Err() == context.DeadlineExceeded {
		return catalog.Timeout
	}
	return catalog.Canceled
}

// Counts are admitted attempts, including failed attempts. A rejected overflow
// never reaches the delegate or creates an out-of-bounds successful receipt.
func (b *verificationBudget) admit(kind string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state.Stopped {
		return catalog.Canceled
	}
	if b.state.Error != catalog.None {
		return b.state.Error
	}
	if b.ctx.Err() != nil {
		b.state.Error = verificationContextError(b.ctx)
		return b.state.Error
	}
	allowed := false
	switch kind {
	case "api":
		allowed = b.state.APIRequests < 24 && b.state.APIActive < 4
	case "bootstrap":
		allowed = b.state.BootstrapRequests < 2 && b.state.APIActive < 4
	case "cover":
		allowed = b.state.CoverRequests < 4
	case "media":
		allowed = b.state.MediaOpens < 4 && b.launchOpens < b.launchLimit
	}
	if !allowed {
		b.state.Error = catalog.Unavailable
		return b.state.Error
	}
	switch kind {
	case "api":
		b.state.APIRequests++
		b.state.APIActive++
	case "bootstrap":
		b.state.BootstrapRequests++
		b.state.APIActive++
	case "cover":
		b.state.CoverRequests++
		b.state.CoverActive++
	case "media":
		b.state.MediaOpens++
		b.launchOpens++
	}
	return nil
}
func (b *verificationBudget) release(kind string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if kind == "cover" {
		b.state.CoverActive--
	} else {
		b.state.APIActive--
	}
}

type verificationTransport struct {
	budget *verificationBudget
	base   http.RoundTripper
	cover  bool
}

func (r *verificationRun) APITransport(base http.RoundTripper) http.RoundTripper {
	return &verificationTransport{budget: r.budget, base: base}
}
func (r *verificationRun) CoverTransport(base http.RoundTripper) http.RoundTripper {
	return &verificationTransport{budget: r.budget, base: base, cover: true}
}

func verificationRoute(req *http.Request, cover bool) string {
	if req == nil || req.URL == nil || req.Method != http.MethodGet || req.Body != nil && req.Body != http.NoBody || req.URL.Scheme != "https" || req.URL.User != nil || req.URL.Fragment != "" || req.URL.Opaque != "" {
		return ""
	}
	if cover {
		in := playback.Input{URL: req.URL.String(), Headers: map[string]string{"Referer": playback.PublicReferer, "User-Agent": playback.PublicUserAgent}}
		if req.Header.Get("Cookie") == "" && req.Header.Get("Authorization") == "" && (playback.HTTPSPolicy{}).Validate(in) == nil {
			return "cover"
		}
		return ""
	}
	if req.URL.RawPath != "" {
		return ""
	}
	if req.URL.Host == "www.bilibili.com" && req.URL.Path == "/" && req.URL.RawQuery == "" && req.Header.Get("Cookie") == "" && req.Header.Get("Authorization") == "" {
		return "bootstrap"
	}
	if req.URL.Host != "api.bilibili.com" {
		return ""
	}
	switch req.URL.Path {
	case "/x/web-interface/nav", "/x/web-interface/wbi/search/type", "/x/web-interface/view", "/x/player/wbi/playurl", "/x/v2/dm/web/view", "/x/v2/dm/web/seg.so":
		return "api"
	}
	return ""
}

func (t *verificationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	kind := verificationRoute(req, t.cover)
	if kind == "" || t.base == nil {
		t.budget.fail(catalog.Invalid)
		return nil, catalog.Invalid
	}
	if req.Context().Err() != nil {
		return nil, verificationContextError(req.Context())
	}
	if err := t.budget.admit(kind); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	joined := make(chan struct{})
	stop := context.AfterFunc(t.budget.ctx, func() { cancel(); close(joined) })
	releaseContext := func() {
		if !stop() {
			<-joined
		}
		cancel()
	}
	response, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil || response == nil || response.Body == nil {
		code := catalog.Network
		closed := true
		if ctx.Err() != nil {
			code = verificationContextError(ctx)
		}
		if response != nil && response.Body != nil {
			if response.Body.Close() != nil {
				code = catalog.Network
				closed = false
			}
		}
		releaseContext()
		if closed {
			t.budget.release(kind)
		}
		if !closed || !t.budget.snapshot().Stopped {
			t.budget.fail(code)
		}
		return nil, code
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code := catalog.ServiceUnavailable
		if response.StatusCode == http.StatusForbidden {
			code = catalog.Restricted
		}
		if response.StatusCode == http.StatusTooManyRequests {
			code = catalog.RateLimited
		}
		t.budget.fail(code)
	}
	response.Body = &verificationBody{base: response.Body, budget: t.budget, kind: kind, releaseContext: releaseContext}
	return response, nil
}

type verificationBody struct {
	base           io.ReadCloser
	budget         *verificationBudget
	kind           string
	releaseContext func()
	once           sync.Once
	err            error
}

func (b *verificationBody) Read(data []byte) (int, error) {
	n, err := b.base.Read(data)
	if err != nil && err != io.EOF {
		if !b.budget.snapshot().Stopped {
			b.budget.fail(catalog.Network)
		}
		return n, catalog.Network
	}
	return n, err
}
func (b *verificationBody) Close() error {
	b.once.Do(func() {
		if b.base.Close() != nil {
			b.err = catalog.Network
			b.budget.fail(catalog.Network)
		} else {
			b.budget.release(b.kind)
		}
		b.releaseContext()
	})
	return b.err
}

type verificationOpener struct {
	budget *verificationBudget
	base   playback.Opener
}

func (r *verificationRun) Opener(base playback.Opener) playback.Opener {
	return verificationOpener{budget: r.budget, base: base}
}
func (o verificationOpener) Open(ctx context.Context, input playback.Input) (playback.Engine, error) {
	if ctx == nil || o.base == nil {
		o.budget.fail(catalog.Invalid)
		return nil, playback.Invalid
	}
	if ctx.Err() != nil {
		return nil, playback.ErrorCode(verificationContextError(ctx))
	}
	if err := o.budget.admit("media"); err != nil {
		if err == catalog.Canceled {
			return nil, playback.Canceled
		}
		if err == catalog.Timeout {
			return nil, playback.Timeout
		}
		return nil, playback.Media
	}
	engine, err := o.base.Open(ctx, input)
	if err != nil || engine == nil {
		code := playback.Media
		var known playback.ErrorCode
		if errors.As(err, &known) {
			switch known {
			case playback.Invalid, playback.Unsupported, playback.Canceled, playback.Timeout, playback.Media, playback.IsClosed:
				code = known
			}
		}
		content := catalog.Code(code)
		if code == playback.Media {
			content = catalog.Unavailable
		}
		o.budget.fail(content)
		if engine != nil {
			if engine.Close() != nil {
				o.budget.fail(catalog.Unavailable)
			}
		}
		return nil, code
	}
	return engine, nil
}
