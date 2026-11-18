package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	domain "my-bilibili/internal/account"
	"net/http"
	"strings"
	"testing"
	"time"
)

const materialLimit = 2560

var names = []string{"SESSDATA", "bili_jct", "DedeUserID", "DedeUserID__ckMd5"}

const testKey = "0123456789abcdef0123456789abcdef"
const secret = "PRIVATE_SYNTHETIC_SECRET"
const cookieLine = "SESSDATA=" + secret + "%2F%3D; Domain=.bilibili.com; Path=/; HttpOnly; Secure"

func generated(host string) string {
	return `{"code":0,"data":{"url":"https://` + host + `/scan?qrcode_key=` + testKey + `","qrcode_key":"` + testKey + `"}}`
}
func polled(code string) string {
	return `{"code":0,"data":{"code":` + code + `,"refresh_token":"PRIVATE_SYNTHETIC_SECRET"}}`
}

type testBody struct {
	io.Reader
	closed bool
}

func (b *testBody) Close() error { b.closed = true; return nil }

type response struct {
	path, body    string
	status        int
	cookies       []string
	err           error
	authenticated bool
}
type sequence struct {
	t      *testing.T
	steps  []response
	calls  int
	bodies []*testBody
	closed bool
}

func (s *sequence) RoundTrip(req *http.Request) (*http.Response, error) {
	s.t.Helper()
	i := s.calls
	s.calls++
	if i >= len(s.steps) {
		s.t.Fatal("unplanned request, retry or redirect")
	}
	step := s.steps[i]
	host := "passport.bilibili.com"
	if step.authenticated {
		host = "api.bilibili.com"
	}
	if req.Method != "GET" || req.URL.Scheme != "https" || req.URL.Host != host || req.URL.Path != step.path || req.URL.User != nil {
		s.t.Fatal("request left fixed origin/path")
	}
	if req.Header.Get("Authorization") != "" || req.Header.Get("User-Agent") != "my-bilibili-p0-feasibility/1" || req.Header.Get("Referer") != "https://www.bilibili.com/" {
		s.t.Fatal("unexpected request headers")
	}
	if step.authenticated {
		cookies := req.Cookies()
		if len(cookies) != 1 || cookies[0].Name != "SESSDATA" || cookies[0].Value != secret+"%2F%3D" {
			s.t.Fatal("credential values changed or went to wrong request")
		}
	} else if req.Header.Get("Cookie") != "" {
		s.t.Fatal("credentials sent to passport request")
	}
	if strings.HasSuffix(step.path, "/poll") {
		if len(req.URL.Query()) != 1 || req.URL.Query().Get("qrcode_key") != testKey {
			s.t.Fatal("poll key changed")
		}
	} else if req.URL.RawQuery != "" {
		s.t.Fatal("unexpected query")
	}
	if end, ok := req.Context().Deadline(); !ok || time.Until(end) > requestLimit || time.Until(end) <= 0 {
		s.t.Fatal("request lifetime unbounded")
	}
	if step.err != nil {
		return nil, step.err
	}
	b := &testBody{Reader: strings.NewReader(step.body)}
	s.bodies = append(s.bodies, b)
	status := step.status
	if status == 0 {
		status = 200
	}
	return &http.Response{StatusCode: status, Body: b, Header: http.Header{"Set-Cookie": step.cookies, "Location": {"https://private.invalid/" + secret}}}, nil
}
func (s *sequence) CloseIdleConnections() { s.closed = true }
func (s *sequence) verify() {
	s.t.Helper()
	if s.calls != len(s.steps) || !s.closed {
		s.t.Fatal("request count or idle cleanup wrong")
	}
	for _, b := range s.bodies {
		if !b.closed {
			s.t.Fatal("body leaked")
		}
	}
}
func beginStep(body string) response {
	return response{path: "/x/passport-login/web/qrcode/generate", body: body}
}
func pollStep(code string, cookies ...string) response {
	return response{path: "/x/passport-login/web/qrcode/poll", body: polled(code), cookies: cookies}
}
func assertSafe(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	for _, marker := range []string{secret, testKey, "https://"} {
		if strings.Contains(fmt.Sprintf("%+v", err), marker) {
			t.Fatal("private data in error")
		}
	}
}

func TestWebLoginPollAndFixedNavValueRoundTrip(t *testing.T) {
	for _, host := range []string{"passport.bilibili.com", "account.bilibili.com"} {
		steps := []response{beginStep(generated(host)), pollStep("86101"), pollStep("86090"), pollStep("0", cookieLine, "unknown="+secret+"; Domain=.other.invalid; Path=/"), {path: "/x/web-interface/nav", body: `{"code":0,"data":{"isLogin":true,"uname":"PRIVATE_SYNTHETIC_SECRET"}}`, authenticated: true}}
		script := &sequence{t: t, steps: steps}
		g := New(script)
		a, err := g.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if a.QR().Empty() || strings.Contains(fmt.Sprintf("%#v", a), testKey) {
			t.Fatal("missing private QR or attempt formatted key")
		}
		for _, expected := range []domain.PollStatus{domain.PollWaiting, domain.PollScanned, domain.PollComplete} {
			result, err := a.Poll(context.Background())
			if err != nil || result.Status != expected {
				t.Fatal("poll state mismatch")
			}
			if expected == domain.PollComplete {
				if result.Material.Empty() {
					t.Fatal("complete lacks material")
				}
				if g.Verify(context.Background(), result.Material) != nil {
					t.Fatal("saved material failed nav round trip")
				}
				encoded, _ := json.Marshal(result.Material)
				if string(encoded) != "null" {
					t.Fatal("material serialized secrets")
				}
			}
		}
		a.Close()
		if !a.QR().Empty() {
			t.Fatal("closed attempt retained QR")
		}
		if _, err := a.Poll(context.Background()); err != domain.Closed {
			t.Fatal("closed attempt polled")
		}
		g.Close()
		script.verify()
	}
}
func TestGenerationValidationAndSafeErrors(t *testing.T) {
	valid := generated("account.bilibili.com")
	cases := []string{"null", "[]", "{}", `{"code":null}`, `{"code":-412,"message":"PRIVATE_SYNTHETIC_SECRET"}`, `{"code":0,"data":null}`,
		strings.ReplaceAll(valid, testKey, "short"), strings.ReplaceAll(valid, testKey, strings.Repeat("z", 32)),
		strings.Replace(valid, "https://", "http://", 1), strings.Replace(valid, "account.bilibili.com", "account.bilibili.com:443", 1), strings.Replace(valid, "account.bilibili.com", "sub.account.bilibili.com", 1), strings.Replace(valid, "https://", "https://user:password@", 1),
		strings.Replace(valid, "qrcode_key="+testKey, "qrcode_key=OTHER", 1), strings.Replace(valid, "qrcode_key="+testKey, "qrcode_key="+testKey+"&qrcode_key="+testKey, 1), strings.Replace(valid, "qrcode_key="+testKey, "qrcode_key=%zz", 1),
		strings.Replace(valid, "/scan?", "/"+strings.Repeat("a", 2048)+"?", 1),
	}
	for _, body := range cases {
		script := &sequence{t: t, steps: []response{beginStep(body)}}
		g := New(script)
		a, err := g.Begin(context.Background())
		if err == nil || a != nil {
			t.Fatal("invalid challenge accepted")
		}
		assertSafe(t, err)
		g.Close()
		script.verify()
	}
	for _, step := range []response{{path: "/x/passport-login/web/qrcode/generate", status: 302, body: secret}, {path: "/x/passport-login/web/qrcode/generate", err: errors.New("https://private.invalid/" + secret)}, {path: "/x/passport-login/web/qrcode/generate", body: strings.Repeat(" ", bodyLimit+1)}} {
		script := &sequence{t: t, steps: []response{step}}
		g := New(script)
		_, err := g.Begin(context.Background())
		if err == nil {
			t.Fatal("HTTP/limit failure accepted")
		}
		assertSafe(t, err)
		g.Close()
		script.verify()
	}
}
func TestCookieScopeAndRecordValidation(t *testing.T) {
	g := New(nil)
	defer g.Close()
	for _, domainAttr := range []string{"Domain=bilibili.com", "Domain=.bilibili.com", "Domain=.BILIBILI.COM"} {
		m, err := g.cookies([]string{"SESSDATA=" + secret + "; " + domainAttr + "; Path=/"})
		if err != nil || m.Empty() {
			t.Fatal("explicit parent-root cookie rejected")
		}
	}
	rejected := [][]string{
		{"SESSDATA=" + secret + "; Path=/"},
		{"SESSDATA=" + secret + "; Domain=passport.bilibili.com; Path=/"},
		{"SESSDATA=" + secret + "; Domain=.bilibili.com"},
		{"SESSDATA=" + secret + "; Domain=.bilibili.com; Path=/login"},
		{"SESSDATA=" + secret + "; Domain=..bilibili.com; Path=/"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com.; Path=/"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com.invalid; Path=/"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com; Path=/; Max-Age=0"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com; Path=/; Expires=invalid"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com; Path=/; Max-Age=invalid"},
		{"SESSDATA=" + secret + "; Domain=bilibili.com; Path=/; Max-Age=9223372036854775807"},
		{"SESSDATA=; Domain=bilibili.com; Path=/"},
		{"SESSDATA=bad\x01value; Domain=bilibili.com; Path=/"},
		{"SESSDATA=" + strings.Repeat("a", 1025) + "; Domain=bilibili.com; Path=/"},
		{cookieLine, cookieLine},
		{cookieLine, "bili_jct=" + secret + "; Path=/"},
		{"unknown=" + secret + "; Domain=bilibili.com; Path=/"},
	}
	for _, lines := range rejected {
		material, err := g.cookies(lines)
		if err != domain.Invalid || !material.Empty() {
			t.Fatal("invalid cookie became publishable material")
		}
	}
	material, err := g.cookies([]string{"SESSDATA=" + strings.Repeat("a", 1024) + "; Domain=bilibili.com; Path=/"})
	if err != nil || material.Empty() {
		t.Fatal("exact cookie value bound rejected")
	}
	var large []string
	for _, name := range names {
		large = append(large, name+"="+strings.Repeat("a", 1024)+"; Domain=bilibili.com; Path=/")
	}
	if _, err := g.cookies(large); err != domain.Invalid {
		t.Fatal("credential blob bound ignored")
	}
}
func TestScopeRejectedPollCannotComplete(t *testing.T) {
	script := &sequence{t: t, steps: []response{beginStep(generated("account.bilibili.com")), pollStep("0", "SESSDATA="+secret+"; Path=/")}}
	g := New(script)
	a, err := g.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	result, err := a.Poll(context.Background())
	if err != domain.Invalid || result.Status == domain.PollComplete || !result.Material.Empty() {
		t.Fatal("unsupported cookie scope published complete material")
	}
	g.Close()
	script.verify()
}
func TestStoredScopeAndSchemaRejectedBeforeNetwork(t *testing.T) {
	script := &sequence{t: t}
	g := New(script)
	defer g.Close()
	valid, err := g.cookies([]string{cookieLine})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if json.Unmarshal(valid.Bytes(), &value) != nil {
		t.Fatal("synthetic encoding failed")
	}
	cases := []string{"", secret, "null", "[]", strings.Repeat(" ", materialLimit+1)}
	original := string(valid.Bytes())
	for _, pair := range [][2]string{{`"version":1`, `"version":2`}, {`"scope":"bilibili.com/"`, `"scope":"passport.bilibili.com/"`}, {`"scope":"bilibili.com/",`, ""}, {`"scope":"bilibili.com/"`, `"scope":"bilibili.com/","domain":"bilibili.com"`}, {`"scope":"bilibili.com/"`, `"scope":"other/","scope":"bilibili.com/"`}, {`"name":"SESSDATA"`, `"name":"unknown"`}, {`"expires":0`, `"expires":null`}, {`"expires":0`, `"expires":-1`}, {`"expires":0`, `"expires":1`}, {`"value":"PRIVATE_SYNTHETIC_SECRET%2F%3D"`, `"value":""`}} {
		cases = append(cases, strings.Replace(original, pair[0], pair[1], 1))
	}
	for _, body := range cases {
		err := g.Verify(context.Background(), domain.NewMaterial([]byte(body)))
		if err == nil {
			t.Fatal("bad restore material accepted")
		}
		assertSafe(t, err)
	}
	if script.calls != 0 {
		t.Fatal("invalid restored material reached network")
	}
}
func TestNavAndPollFailureCategories(t *testing.T) {
	for _, body := range []string{`{"code":-101}`, `{"code":0,"data":{"isLogin":false}}`, `{"code":-412,"message":"PRIVATE_SYNTHETIC_SECRET"}`, `{"code":0,"data":{}}`, `{"code":null}`} {
		script := &sequence{t: t, steps: []response{{path: "/x/web-interface/nav", body: body, authenticated: true}}}
		g := New(script)
		material, err := g.cookies([]string{cookieLine})
		if err != nil {
			t.Fatal(err)
		}
		err = g.Verify(context.Background(), material)
		if err == nil {
			t.Fatal("invalid nav became signed in")
		}
		assertSafe(t, err)
		if strings.Contains(body, "-101") || strings.Contains(body, "false") {
			if err != domain.ExpiredSession {
				t.Fatal("expiry not distinguished")
			}
		}
		g.Close()
		script.verify()
	}
	for _, code := range []string{"86038", "99999", "null"} {
		script := &sequence{t: t, steps: []response{beginStep(generated("passport.bilibili.com")), pollStep(code)}}
		g := New(script)
		a, err := g.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		result, err := a.Poll(context.Background())
		if code == "86038" {
			if err != nil || result.Status != domain.PollExpired {
				t.Fatal("poll expiry not distinguished")
			}
		} else if err == nil {
			t.Fatal("unknown poll succeeded")
		}
		a.Close()
		g.Close()
		script.verify()
	}
}
func TestProductionIsolationAndCancellation(t *testing.T) {
	g := New(nil)
	defer g.Close()
	tr := g.client.Transport.(*http.Transport)
	if tr.Proxy != nil || !tr.DisableKeepAlives || !tr.Protocols.HTTP1() || tr.Protocols.HTTP2() || g.client.Jar != nil {
		t.Fatal("production HTTP isolation changed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	script := &sequence{t: t}
	g = New(script)
	if _, err := g.Begin(ctx); err != domain.Canceled || script.calls != 0 {
		t.Fatal("canceled generation made request")
	}
	g.Close()
}
func TestCredentialSchemaRejectsDuplicateRecordsAndScopeFields(t *testing.T) {
	g := New(nil)
	defer g.Close()
	m, err := g.cookies([]string{cookieLine})
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Version int               `json:"version"`
		Scope   string            `json:"scope"`
		Cookies []json.RawMessage `json:"cookies"`
	}
	if json.Unmarshal(m.Bytes(), &value) != nil {
		t.Fatal("fixture decode")
	}
	value.Cookies = append(value.Cookies, value.Cookies[0])
	body, _ := json.Marshal(value)
	if _, err := g.decode(domain.NewMaterial(body)); err != domain.Invalid {
		t.Fatal("duplicate credential record accepted")
	}
	body = bytes.Replace(m.Bytes(), []byte(`"expires":0`), []byte(`"expires":0,"path":"/"`), 1)
	if _, err := g.decode(domain.NewMaterial(body)); err != domain.Invalid {
		t.Fatal("record scope override accepted")
	}
}

func TestMalformedKnownCookieCannotBeSilentlyDropped(t *testing.T) {
	g := New(nil)
	defer g.Close()
	for _, invalid := range []string{"SESSDATA", "bili_jct; Domain=bilibili.com; Path=/"} {
		material, err := g.cookies([]string{cookieLine, invalid})
		if err != domain.Invalid || !material.Empty() {
			t.Fatal("malformed known cookie silently dropped")
		}
	}
}

func TestCredentialSchemaRequiresExactFieldNames(t *testing.T) {
	g := New(nil)
	defer g.Close()
	m, err := g.cookies([]string{cookieLine})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{`"scope":`, `"Scope":`}, {`"name":`, `"Name":`}, {`"version":1`, `"version":1,"Version":1`}} {
		blob := strings.Replace(string(m.Bytes()), pair[0], pair[1], 1)
		if _, err := g.decode(domain.NewMaterial([]byte(blob))); err != domain.Invalid {
			t.Fatal("case-alias field bypassed exact credential schema")
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type deadlineBody struct {
	ctx    context.Context
	closed bool
}

func (b *deadlineBody) Read([]byte) (int, error) { <-b.ctx.Done(); return 0, errors.New(secret) }
func (b *deadlineBody) Close() error             { b.closed = true; return nil }
func TestRequestAndBodyCancellationRemainSafe(t *testing.T) {
	for _, inBody := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		var body *deadlineBody
		g := New(roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if inBody {
				body = &deadlineBody{ctx: req.Context()}
				return &http.Response{StatusCode: 200, Body: body}, nil
			}
			<-req.Context().Done()
			return nil, errors.New(secret)
		}))
		_, err := g.Begin(ctx)
		cancel()
		g.Close()
		assertSafe(t, err)
		if err != domain.Timeout || inBody && !body.closed {
			t.Fatal("deadline or body cleanup lost")
		}
	}
}
