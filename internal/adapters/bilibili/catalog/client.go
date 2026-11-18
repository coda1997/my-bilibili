// Package catalog converts the fixed web catalog API into plain display data.
package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"my-bilibili/internal/adapters/bilibili/transport"
	domain "my-bilibili/internal/catalog"
)

const publicReferer = "https://www.bilibili.com/"
const publicUserAgent = "my-bilibili-p0/1"

type Client struct {
	scope          *transport.Scope
	coverTransport http.RoundTripper
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	closed         bool
	urls           map[string]string
	order          []string
	sequence       uint64
	coverSlot      chan struct{}
}

// New only creates memory-owned state. Callers own/join their API and cover
// operations; Close cancels them and retires all private cover locations.
func New(scope *transport.Scope, coverTransport http.RoundTripper) *Client {
	parent := context.Background()
	if scope != nil {
		parent = scope.Context()
	}
	ctx, cancel := context.WithCancel(parent)
	if coverTransport == nil {
		coverTransport = &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 10 * time.Second}
	}
	return &Client{scope: scope, coverTransport: coverTransport, ctx: ctx, cancel: cancel, urls: make(map[string]string), coverSlot: make(chan struct{}, 1)}
}

func (*Client) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private catalog client]") }
func (*Client) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func (c *Client) check(ctx context.Context) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return domain.Closed
	}
	if c.scope == nil || ctx == nil {
		return domain.Invalid
	}
	if err := ctx.Err(); err != nil {
		return contextCode(err)
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return domain.Timeout
	}
	if c.ctx.Err() != nil {
		return domain.Canceled
	}
	return nil
}

func contextCode(err error) domain.Code {
	if err == context.DeadlineExceeded {
		return domain.Timeout
	}
	return domain.Canceled
}

func (c *Client) operation(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	joined := make(chan struct{})
	stop := context.AfterFunc(c.ctx, func() { cancel(); close(joined) })
	return ctx, func() {
		cancel()
		if !stop() {
			<-joined
		}
	}
}

func (c *Client) Search(parent context.Context, query string, page int) (domain.Page, error) {
	query, err := domain.ValidateQuery(query)
	if err != nil || page < 1 || page > 50 {
		return domain.Page{}, domain.Invalid
	}
	if err := c.check(parent); err != nil {
		return domain.Page{}, err
	}
	ctx, finish := c.operation(parent)
	defer finish()
	body, err := c.scope.JSON(ctx, transport.Search, url.Values{"keyword": {query}, "page": {strconv.Itoa(page)}})
	if err != nil {
		return domain.Page{}, err
	}
	result, locations, err := decodeSearch(body, page)
	if err != nil {
		return domain.Page{}, err
	}
	if err := c.check(ctx); err != nil {
		return domain.Page{}, err
	}
	for i, location := range locations {
		result.Videos[i].CoverID = c.rememberCover(location)
	}
	if err := c.check(ctx); err != nil {
		return domain.Page{}, err
	}
	return result, nil
}

func entryParams(entry domain.Entry) (url.Values, error) {
	text, key := entry.ID, "bvid"
	if entry.Kind == domain.AVEntry {
		text, key = "av"+entry.ID, "aid"
	} else if entry.Kind != domain.BVEntry {
		return nil, domain.Invalid
	}
	parsed, err := domain.ParseEntry(text)
	if err != nil || parsed.Kind != entry.Kind || parsed.ID != entry.ID || entry.Page < 0 || entry.Page > 1000 {
		return nil, domain.Invalid
	}
	return url.Values{key: {entry.ID}}, nil
}

func (c *Client) Detail(parent context.Context, entry domain.Entry) (domain.Video, error) {
	params, err := entryParams(entry)
	if err != nil {
		return domain.Video{}, err
	}
	if err := c.check(parent); err != nil {
		return domain.Video{}, err
	}
	ctx, finish := c.operation(parent)
	defer finish()
	body, err := c.scope.JSON(ctx, transport.Detail, params)
	if err != nil {
		return domain.Video{}, err
	}
	video, location, err := decodeDetail(body, entry)
	if err != nil {
		return domain.Video{}, err
	}
	if err := c.check(ctx); err != nil {
		return domain.Video{}, err
	}
	video.CoverID = c.rememberCover(location)
	if err := c.check(ctx); err != nil {
		return domain.Video{}, err
	}
	return video, nil
}

func (c *Client) rememberCover(location string) string {
	if !validCoverURL(location) {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.ctx.Err() != nil {
		return ""
	}
	for id, current := range c.urls {
		if current == location {
			return id
		}
	}
	if len(c.order) == 1001 {
		delete(c.urls, c.order[0])
		c.order = c.order[1:]
	}
	c.sequence++
	id := "cover-" + strconv.FormatUint(c.sequence, 10)
	c.urls[id] = location
	c.order = append(c.order, id)
	return id
}

// MatchesScope lets a composed media source verify this adapter's actual bound
// lifetime. Revisions alone are not identities across independent clients.
// This is a memory-only query and reveals no credential or request data.
func (c *Client) MatchesScope(scope *transport.Scope) bool {
	if c == nil || scope == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.scope == scope && !c.closed && c.ctx.Err() == nil
}

func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.urls, c.order = nil, nil
	c.mu.Unlock()
	c.cancel()
}
