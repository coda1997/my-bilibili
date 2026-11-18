package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const contractRequestLimit = 10 * time.Second
const contractJSONLimit = 1 << 20
const contractSegmentLimit = 4 << 20
const contractConcurrency = 4

const private = "PRIVATE_SYNTHETIC_TRANSPORT"
const syntheticBV = "BV0123456789"
const imgKey = "0123456789abcdef0123456789abcdef"
const subKey = "00112233445566778899aabbccddeeff"
const signedNav = `{"code":0,"data":{"isLogin":true,"wbi_img":{"img_url":"https://wbi.invalid/` + imgKey + `.png","sub_url":"https://wbi.invalid/` + subKey + `.png"}}}`
const anonymousNav = `{"code":-101,"data":{"isLogin":false,"wbi_img":{"img_url":"https://wbi.invalid/` + imgKey + `.png","sub_url":"https://wbi.invalid/` + subKey + `.png"}}}`
const browserLine = "buvid3=SYNTHETIC_BROWSER; Domain=.bilibili.com; Path=/; Secure"

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type observedBody struct {
	io.Reader
	closed   atomic.Int32
	closeErr error
}

func (b *observedBody) Close() error { b.closed.Add(1); return b.closeErr }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: &observedBody{Reader: strings.NewReader(body)}, Header: make(http.Header)}
}
func testMaterial() account.Material {
	return account.NewMaterial([]byte(`{"version":1,"scope":"bilibili.com/","cookies":[{"name":"SESSDATA","value":"` + private + `%2F","expires":0}]}`))
}
func testClient(t *testing.T, trip roundTripFunc) *Client {
	t.Helper()
	c := New(trip)
	t.Cleanup(func() { c.Close(); await(t, c.Done()) })
	return c
}
func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("owned operation did not finish")
	}
	var zero T
	return zero
}
func bind(t *testing.T, c *Client) *Scope {
	t.Helper()
	s, err := c.Bind()
	if err != nil {
		t.Fatal("binding failed")
	}
	return s
}
func detailParams() url.Values { return url.Values{"bvid": {syntheticBV}} }
func playParams() url.Values   { return url.Values{"bvid": {syntheticBV}, "cid": {"123"}, "qn": {"0"}} }
func searchParams() url.Values { return url.Values{"keyword": {"中文 搜索"}, "page": {"1"}} }
func safeError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if _, ok := err.(catalog.Code); !ok || strings.Contains(fmt.Sprintf("%+v", err), private) {
		t.Fatal("raw error escaped transport")
	}
}

func TestProductionTransportAndPublisherAreIsolated(t *testing.T) {
	c := New(nil)
	defer func() { c.Close(); await(t, c.Done()) }()
	tr := c.roundTripper.(*http.Transport)
	if tr.Proxy != nil || !tr.DisableKeepAlives || !tr.Protocols.HTTP1() || tr.Protocols.HTTP2() {
		t.Fatal("production transport isolation changed")
	}
	c.now = func() time.Time { t.Fatal("publisher invoked an injected callback"); return time.Time{} }
	c.roundTripper = roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("publisher performed HTTP"); return nil, nil })
	a := bind(t, c)
	if a.State().Revision == 0 || a.State().Authenticated || a.State().AccountEpoch != 0 {
		t.Fatal("initial anonymous scope invalid")
	}
	if c.Activate(0, testMaterial()) != account.Invalid || c.Activate(1, account.NewMaterial([]byte(private))) != account.Invalid || c.State().Revision != a.State().Revision {
		t.Fatal("invalid activation changed scope")
	}
	if c.Activate(7, testMaterial()) != nil {
		t.Fatal("valid activation failed")
	}
	b := bind(t, c)
	if b.State().Revision <= a.State().Revision || !b.State().Authenticated || b.State().AccountEpoch != 7 || !a.State().Closed || a.Context().Err() == nil {
		t.Fatal("activation did not retire original scope")
	}
	c.Revoke(6)
	if c.State().Revision != b.State().Revision || b.Context().Err() != nil {
		t.Fatal("stale revoke affected current scope")
	}
	c.Revoke(7)
	state := c.State()
	if state.Authenticated || state.AccountEpoch != 0 || state.Revision <= b.State().Revision || b.Context().Err() == nil {
		t.Fatal("matching revoke did not replace scope")
	}
	c.Revoke(7)
	if c.State() != state {
		t.Fatal("repeat revoke changed state")
	}
	c.Close()
	await(t, c.Done())
	if c.Activate(8, testMaterial()) != account.Closed || !c.State().Closed {
		t.Fatal("closed client activated")
	}
	if _, err := c.Bind(); err != catalog.Closed {
		t.Fatal("closed client bound")
	}
}

func TestConcreteHandlesRedactCredentialsAndKeys(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New(private) })
	if c.Activate(1, testMaterial()) != nil {
		t.Fatal("activation failed")
	}
	s := bind(t, c)
	s.keys.mixin = private
	s.browser.cookies = []browserCookie{{name: "buvid3", value: private}}
	for _, value := range []any{c, s, *s} {
		formatted := fmt.Sprintf("%#v", value)
		if formatted != "[private service client]" && formatted != "[private service scope]" {
			t.Fatal("private handle fields formatted")
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != "null" {
			t.Fatal("private handle serialized")
		}
	}
	_, err := s.JSON(context.Background(), Detail, detailParams())
	if err != catalog.Network {
		t.Fatal("unexpected safe network category")
	}
	safeError(t, err)
	c.Revoke(1)
	if s.session != nil || s.keys.mixin != "" || s.browser.cookies != nil {
		t.Fatal("retired scope retained private caches")
	}
}

func TestFixedRoutesPublicMetadataAndCookieDestinations(t *testing.T) {
	var calls atomic.Int32
	var bodies []*observedBody
	c := testClient(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.User != nil || req.Header.Get("Referer") != publicReferer || req.Header.Get("User-Agent") != publicUserAgent || req.Header.Get("Authorization") != "" {
			t.Error("fixed request metadata changed")
		}
		end, ok := req.Context().Deadline()
		if !ok || time.Until(end) > contractRequestLimit || time.Until(end) <= 0 {
			t.Error("raw request deadline unbounded")
		}
		result := response(`{"code":0,"data":{}}`)
		query := req.URL.Query()
		if req.URL.Host == "www.bilibili.com" {
			if req.URL.Path != "/" || len(query) != 0 || req.Header.Get("Cookie") != "" {
				t.Error("bootstrap carried account data or wrong route")
			}
			result.Header["Set-Cookie"] = []string{browserLine, "b_nut=SYNTHETIC_NUT; Domain=bilibili.com; Path=/", "SESSDATA=IGNORED_BOOTSTRAP_COOKIE; Domain=bilibili.com; Path=/"}
		} else {
			if req.URL.Host != "api.bilibili.com" {
				t.Error("request left fixed API origin")
			}
			cookie, err := req.Cookie("SESSDATA")
			if err != nil || cookie.Value != private+"%2F" {
				t.Error("account cookie missing or changed")
			}
			switch req.URL.Path {
			case "/x/web-interface/nav":
				if len(query) != 0 || len(req.Cookies()) != 1 {
					t.Error("nav request gained parameters or browser cookies")
				}
				result = response(signedNav)
			case "/x/web-interface/wbi/search/type":
				if len(query) != 6 || query.Get("keyword") != "中文 搜索" || query.Get("page") != "1" || query.Get("search_type") != "video" || query.Get("order") != "totalrank" || query.Get("wts") != "1700000000" || len(query.Get("w_rid")) != 32 || strings.Contains(req.URL.RawQuery, "+") {
					t.Error("search route or WBI query wrong")
				}
				if len(req.Cookies()) != 3 {
					t.Error("search browser cookies missing or unbounded")
				}
			case "/x/web-interface/view":
				if len(query) != 1 || query.Get("bvid") != syntheticBV && query.Get("aid") != "123" || len(req.Cookies()) != 1 {
					t.Error("detail route wrong")
				}
			case "/x/player/wbi/playurl":
				if len(query) != 7 || query.Get("bvid") != syntheticBV || query.Get("cid") != "123" || query.Get("qn") != "0" || query.Get("fnver") != "0" || query.Get("fnval") != "16" || len(req.Cookies()) != 1 {
					t.Error("play URL route wrong")
				}
			case "/x/v2/dm/web/view":
				if len(query) != 2 || query.Get("oid") != "123" || query.Get("type") != "1" {
					t.Error("danmaku view route wrong")
				}
				result = response("\x10\x01")
			case "/x/v2/dm/web/seg.so":
				if len(query) != 3 || query.Get("oid") != "123" || query.Get("type") != "1" || query.Get("segment_index") != "2" {
					t.Error("unsigned segment route wrong")
				}
				result = response("")
			default:
				t.Error("unplanned API route or image fetch")
			}
		}
		bodies = append(bodies, result.Body.(*observedBody))
		return result, nil
	})
	c.now = func() time.Time { return time.Unix(1700000000, 0) }
	if c.Activate(1, testMaterial()) != nil {
		t.Fatal("activation failed")
	}
	s := bind(t, c)
	for _, request := range []struct {
		route  Route
		params url.Values
	}{{Nav, nil}, {Search, searchParams()}, {Detail, detailParams()}, {Detail, url.Values{"aid": {"123"}}}, {PlayURL, playParams()}, {Search, searchParams()}} {
		if _, err := s.JSON(context.Background(), request.route, request.params); err != nil {
			t.Fatal("fixed JSON route failed")
		}
	}
	for _, request := range []struct {
		route  Route
		params url.Values
	}{{DanmakuView, url.Values{"oid": {"123"}}}, {DanmakuSegment, url.Values{"oid": {"123"}, "segment_index": {"2"}}}} {
		if _, err := s.Protobuf(context.Background(), request.route, request.params); err != nil {
			t.Fatal("fixed protobuf route failed")
		}
	}
	if calls.Load() != 10 || len(bodies) != 10 {
		t.Fatal("unexpected request/preparation count")
	}
	for _, body := range bodies {
		if body.closed.Load() != 1 {
			t.Fatal("response body not closed exactly once")
		}
	}
}

func TestRouteValidationRejectsBeforeAnyRequest(t *testing.T) {
	var calls atomic.Int32
	c := testClient(t, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(`{"code":0,"data":{}}`), nil
	})
	s := bind(t, c)
	cases := []struct {
		route  Route
		params url.Values
		json   bool
	}{
		{0, nil, true}, {99, nil, false}, {Detail, detailParams(), false}, {DanmakuSegment, url.Values{"oid": {"123"}, "segment_index": {"1"}}, true},
		{Nav, url.Values{"url": {"https://private.invalid/"}}, true}, {Detail, nil, true},
		{Detail, url.Values{"bvid": {syntheticBV}, "aid": {"123"}}, true}, {Detail, url.Values{"bvid": {syntheticBV, syntheticBV}}, true},
		{Detail, url.Values{"bvid": {"BVshort"}}, true}, {Detail, url.Values{"aid": {"0"}}, true}, {Detail, url.Values{"aid": {"-1"}}, true}, {Detail, url.Values{"aid": {"9223372036854775808"}}, true},
		{Search, url.Values{"keyword": {" "}, "page": {"1"}}, true}, {Search, url.Values{"keyword": {"bad\ntext"}, "page": {"1"}}, true},
		{Search, url.Values{"keyword": {string([]byte{0xff})}, "page": {"1"}}, true}, {Search, url.Values{"keyword": {strings.Repeat("a", 129)}, "page": {"1"}}, true},
		{Search, url.Values{"keyword": {strings.Repeat("界", 513)}, "page": {"1"}}, true}, {Search, url.Values{"keyword": {"word"}, "page": {"0"}}, true},
		{Search, url.Values{"keyword": {"word"}, "page": {"1"}, "search_type": {"live"}}, true},
		{Search, url.Values{"keyword": {"word"}, "page": {"1"}, "wts": {"0"}}, true},
		{PlayURL, url.Values{"bvid": {syntheticBV}, "cid": {"123"}, "qn": {"0"}, "fnval": {"4048"}}, true},
		{PlayURL, url.Values{"bvid": {syntheticBV}, "cid": {"0"}, "qn": {"0"}}, true}, {PlayURL, url.Values{"bvid": {syntheticBV}, "cid": {"123"}, "qn": {"-1"}}, true},
		{DanmakuView, url.Values{"oid": {"123"}, "pid": {"123"}}, false}, {DanmakuSegment, url.Values{"oid": {"123"}, "segment_index": {"0"}}, false},
	}
	for _, request := range cases {
		var err error
		if request.json {
			_, err = s.JSON(context.Background(), request.route, request.params)
		} else {
			_, err = s.Protobuf(context.Background(), request.route, request.params)
		}
		if err != catalog.Invalid {
			t.Fatal("invalid fixed-route input was accepted")
		}
	}
	if _, err := s.Protobuf(context.Background(), DanmakuView, url.Values{"oid": {"123"}}); err != catalog.LoginRequired {
		t.Fatal("anonymous metadata request allowed")
	}
	if calls.Load() != 0 {
		t.Fatal("rejected input reached transport")
	}
}
