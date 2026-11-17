package transport

import (
	"compress/gzip"
	"context"
	"io"
	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	"net/http"
	"strings"
)

// request performs exactly one RoundTrip. Using the RoundTripper directly
// deliberately gives no redirect-following or automatic operation retry path.
func (s *Scope) request(parent context.Context, endpoint, query string, limit int, browser, credentials bool) ([]byte, http.Header, error) {
	ctx, cancel := context.WithTimeout(parent, requestLimit)
	defer cancel()
	if err := s.check(ctx); err != nil {
		return nil, nil, err
	}
	select {
	case s.client.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, contextCode(ctx.Err())
	case <-s.ctx.Done():
		return nil, nil, catalog.Canceled
	}
	defer func() { <-s.client.slots }()
	if err := s.check(ctx); err != nil {
		return nil, nil, err
	}
	if query != "" {
		endpoint += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, catalog.Invalid
	}
	req.Header.Set("Referer", publicReferer)
	req.Header.Set("User-Agent", publicUserAgent)
	now := s.client.now()
	s.client.mu.Lock()
	err = s.live()
	session, cookies := s.session, s.browser
	s.client.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if credentials && session != nil {
		if err := session.Apply(req, now); err != nil {
			if err == account.ExpiredSession {
				return nil, nil, catalog.SessionExpired
			}
			return nil, nil, catalog.Invalid
		}
	}
	if browser {
		if !cookies.usable(now) {
			return nil, nil, catalog.ServiceUnavailable
		}
		for _, cookie := range cookies.cookies {
			if cookie.expires == 0 || cookie.expires > now.Unix() {
				req.AddCookie(&http.Cookie{Name: cookie.name, Value: cookie.value})
			}
		}
	}
	if err := s.check(ctx); err != nil {
		return nil, nil, err
	}
	resp, err := s.client.roundTripper.RoundTrip(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if cause := s.check(ctx); cause != nil {
			return nil, nil, cause
		}
		return nil, nil, catalog.Network
	}
	if resp == nil || resp.Body == nil {
		return nil, nil, catalog.InvalidResponse
	}
	if resp.StatusCode != http.StatusOK {
		closeErr := resp.Body.Close()
		if cause := s.check(ctx); cause != nil {
			return nil, nil, cause
		}
		if closeErr != nil {
			return nil, nil, catalog.Network
		}
		return nil, nil, statusCode(resp.StatusCode)
	}
	body, err := readBody(resp, limit)
	if cause := s.check(ctx); cause != nil {
		return nil, nil, cause
	}
	if err != nil {
		return nil, nil, err
	}
	return body, resp.Header, nil
}

func statusCode(status int) catalog.Code {
	switch {
	case status == http.StatusTooManyRequests:
		return catalog.RateLimited
	case status >= 500 && status <= 599:
		return catalog.ServiceUnavailable
	case status == http.StatusForbidden:
		return catalog.Restricted
	case status == http.StatusUnauthorized:
		return catalog.LoginRequired
	default:
		return catalog.InvalidResponse
	}
}

func readBody(resp *http.Response, limit int) (body []byte, err error) {
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			body, err = nil, catalog.Network
		}
	}()
	var reader io.Reader = resp.Body
	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip":
		decoded, openErr := gzip.NewReader(resp.Body)
		if openErr != nil {
			return nil, catalog.InvalidResponse
		}
		defer decoded.Close()
		reader = decoded
	default:
		return nil, catalog.InvalidResponse
	}
	body, readErr := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if readErr != nil {
		return nil, catalog.Network
	}
	if len(body) > limit {
		return nil, catalog.InvalidResponse
	}
	return body, nil
}
