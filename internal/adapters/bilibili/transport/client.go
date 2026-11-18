// Package transport owns fixed, scope-bound read-only Bilibili requests.
package transport

import (
	"context"
	"fmt"
	"io"
	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/bilibili/sessiondata"
	"my-bilibili/internal/catalog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const requestLimit = 10 * time.Second
const jsonLimit = 1 << 20
const segmentLimit = 4 << 20
const concurrencyLimit = 4
const publicReferer = "https://www.bilibili.com/"
const publicUserAgent = "my-bilibili-p0/1"

type SessionState struct {
	Revision, AccountEpoch, ExpiredEpoch uint64
	Authenticated, Closed                bool
}

type Route uint8

const (
	Nav Route = iota + 1
	Search
	Detail
	PlayURL
	DanmakuView
	DanmakuSegment
)

type Client struct {
	mu                     sync.Mutex
	roundTripper           http.RoundTripper
	slots                  chan struct{}
	current                *Scope
	revision, expiredEpoch uint64
	closed                 bool
	workers                sync.WaitGroup
	done                   chan struct{}
	now                    func() time.Time
}

// Scope is opaque and remains bound to its original account revision. All of
// its mutable fields are protected by the owning Client's mutex.
type Scope struct {
	client                        *Client
	ctx                           context.Context
	cancel                        context.CancelFunc
	revision, epoch, expiredEpoch uint64
	retired                       bool
	session                       *sessiondata.Session
	keys                          keyset
	browser                       browserState
	preparing                     *preparation
}

func New(roundTripper http.RoundTripper) *Client {
	if roundTripper == nil {
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		roundTripper = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: requestLimit}).DialContext, TLSHandshakeTimeout: requestLimit, DisableKeepAlives: true, Protocols: protocols}
	}
	c := &Client{roundTripper: roundTripper, slots: make(chan struct{}, concurrencyLimit), done: make(chan struct{}), now: time.Now}
	c.replace(0, nil, 0)
	return c
}

func (*Client) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private service client]") }
func (*Client) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
func (Scope) Format(s fmt.State, _ rune)     { _, _ = io.WriteString(s, "[private service scope]") }
func (Scope) MarshalJSON() ([]byte, error)   { return []byte("null"), nil }

// Activate and Revoke are memory-only publisher leaf transactions. They never
// call injected transports, clocks, managers or callbacks, nor join workers.
func (c *Client) Activate(epoch uint64, material account.Material) error {
	if epoch == 0 {
		return account.Invalid
	}
	session, err := sessiondata.Decode(material, time.Now())
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return account.Closed
	}
	c.replace(epoch, session, 0)
	return nil
}
func (c *Client) Revoke(epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || epoch == 0 || c.current.epoch != epoch || c.current.retired {
		return
	}
	c.replace(0, nil, 0)
}

func (c *Client) replace(epoch uint64, session *sessiondata.Session, expired uint64) {
	if c.current != nil {
		c.current.retire()
	}
	c.revision++
	ctx, cancel := context.WithCancel(context.Background())
	c.current = &Scope{client: c, ctx: ctx, cancel: cancel, revision: c.revision, epoch: epoch, session: session, expiredEpoch: expired}
	c.expiredEpoch = expired
}
func (s *Scope) retire() {
	s.retired = true
	s.session = nil
	s.keys = keyset{}
	s.browser = browserState{}
	s.preparing = nil
	s.cancel()
}
func (c *Client) State() SessionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.current.state()
	state.ExpiredEpoch = c.expiredEpoch
	return state
}
func (s *Scope) State() SessionState {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	return s.state()
}
func (s *Scope) state() SessionState {
	closed := s.retired || s.client.closed
	return SessionState{Revision: s.revision, AccountEpoch: s.epoch, ExpiredEpoch: s.expiredEpoch, Authenticated: s.epoch > 0 && !closed, Closed: closed}
}
func (c *Client) Bind() (*Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, catalog.Closed
	}
	return c.current, nil
}
func (s *Scope) Context() context.Context { return s.ctx }

// Close cancels synchronously; Done joins scope-owned preparation and transport
// cleanup. Feature owners remain responsible for joining their request callers.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.current.retire()
	c.mu.Unlock()
	go func() {
		c.workers.Wait()
		if closer, ok := c.roundTripper.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
		close(c.done)
	}()
}
func (c *Client) Done() <-chan struct{} { return c.done }

func (s *Scope) live() error {
	if s.client.closed {
		return catalog.Closed
	}
	if s.retired || s.client.current != s || s.ctx.Err() != nil {
		return catalog.Canceled
	}
	return nil
}
func contextCode(err error) catalog.Code {
	if err == context.DeadlineExceeded {
		return catalog.Timeout
	}
	return catalog.Canceled
}
func (s *Scope) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return contextCode(err)
	}
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	return s.live()
}

func (s *Scope) operation(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(parent, requestLimit)
	joined := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() { cancel(); close(joined) })
	return ctx, func() {
		cancel()
		if !stop() {
			<-joined
		}
	}
}

func (s *Scope) JSON(ctx context.Context, route Route, params url.Values) ([]byte, error) {
	return s.get(ctx, route, params, true)
}
func (s *Scope) Protobuf(ctx context.Context, route Route, params url.Values) ([]byte, error) {
	return s.get(ctx, route, params, false)
}
func (s *Scope) get(parent context.Context, route Route, params url.Values, json bool) ([]byte, error) {
	ctx, finish := s.operation(parent)
	defer finish()
	request, err := routeRequest(route, params, json)
	if err != nil {
		return nil, err
	}
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	s.client.mu.Lock()
	authenticated := s.epoch > 0
	s.client.mu.Unlock()
	if route == DanmakuView && !authenticated {
		return nil, catalog.LoginRequired
	}
	if err := s.prepare(ctx, request.needs); err != nil {
		return nil, err
	}
	query := request.params.Encode()
	if request.needs&wbiPreparation != 0 {
		now := s.client.now()
		s.client.mu.Lock()
		if err := s.live(); err != nil {
			s.client.mu.Unlock()
			return nil, err
		}
		keys := s.keys
		s.client.mu.Unlock()
		if !keys.usable(now) {
			return nil, catalog.ServiceUnavailable
		}
		query = sign(request.params, keys.mixin, now.Unix())
	}
	body, _, err := s.request(ctx, "https://api.bilibili.com"+request.path, query, request.limit, request.needs&browserPreparation != 0, true)
	if err != nil {
		return nil, err
	}
	if json {
		if _, err := s.acceptJSON(ctx, route, body); err != nil {
			return nil, err
		}
	} else if err := s.acceptProtobuf(ctx, body); err != nil {
		return nil, err
	}
	return body, nil
}
