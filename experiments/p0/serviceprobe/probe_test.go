package serviceprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const sentinel = "PRIVATE_SYNTHETIC_SENTINEL"
const challenge = "https://passport.bilibili.com/scan?secret=" + sentinel

var successBodies = [][]byte{
	[]byte(`{"code":-101,"message":"PRIVATE_SYNTHETIC_SENTINEL"}`),
	[]byte(`{"code":0,"data":{"url":"` + challenge + `","qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL &?"}}`),
	[]byte(`{"code":0,"data":{"code":86101,"refresh_token":"PRIVATE_SYNTHETIC_SENTINEL"}}`),
	[]byte(`{"code":0,"data":{"bvid":"BV1Zs411o7Ee","aid":899574,"pages":[{"cid":1445374}],"title":"PRIVATE_SYNTHETIC_SENTINEL"}}`),
	fieldBytes(1, entry(0, 1, []byte(sentinel))),
}

type trackedBody struct {
	io.Reader
	closed   bool
	closeErr error
}

func (b *trackedBody) Close() error { b.closed = true; return b.closeErr }

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type script struct {
	t          *testing.T
	bodies     [][]byte
	calls      int
	opened     []*trackedBody
	idleClosed bool
}

func (s *script) RoundTrip(r *http.Request) (*http.Response, error) {
	s.t.Helper()
	i := s.calls
	s.calls++
	if i >= len(s.bodies) {
		s.t.Fatal("unexpected retry or extra request")
		return nil, errors.New("unexpected request")
	}
	paths := []string{"/x/web-interface/nav", "/x/passport-login/web/qrcode/generate", "/x/passport-login/web/qrcode/poll", "/x/web-interface/view", "/x/v2/dm/web/seg.so"}
	hosts := []string{"api.bilibili.com", "passport.bilibili.com", "passport.bilibili.com", "api.bilibili.com", "api.bilibili.com"}
	if r.Method != "GET" || r.URL.Scheme != "https" || r.URL.Host != hosts[i] || r.URL.Path != paths[i] {
		s.t.Fatal("request left fixed sequence")
	}
	if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || len(r.Header) != 2 || r.Header.Get("User-Agent") != "my-bilibili-p0-feasibility/1" || r.Header.Get("Referer") != "https://www.bilibili.com/" {
		s.t.Fatal("unexpected request metadata")
	}
	q := r.URL.Query()
	switch i {
	case 0, 1:
		if len(q) != 0 {
			s.t.Fatal("unexpected query")
		}
	case 2:
		if len(q) != 1 || q.Get("qrcode_key") != sentinel+" &?" {
			s.t.Fatal("private query not encoded faithfully")
		}
	case 3:
		if len(q) != 1 || q.Get("bvid") != videoID {
			s.t.Fatal("wrong video query")
		}
	case 4:
		if len(q) != 3 || q.Get("type") != "1" || q.Get("oid") != "1445374" || q.Get("segment_index") != "1" {
			s.t.Fatal("wrong segment query")
		}
	}
	deadline, ok := r.Context().Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > requestLimit {
		s.t.Fatal("request has no finite deadline")
	}
	b := &trackedBody{Reader: bytes.NewReader(s.bodies[i])}
	s.opened = append(s.opened, b)
	return &http.Response{StatusCode: 200, Body: b, Header: http.Header{"Set-Cookie": {"SESSDATA=" + sentinel}}}, nil
}
func (s *script) CloseIdleConnections() { s.idleClosed = true }
func (s *script) verify() {
	s.t.Helper()
	if !s.idleClosed {
		s.t.Fatal("idle connections not closed")
	}
	for _, b := range s.opened {
		if !b.closed {
			s.t.Fatal("response body not closed")
		}
	}
}
func fixtures() [][]byte { return append([][]byte(nil), successBodies...) }
func checkPrivate(t *testing.T, r Report) {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{sentinel, "https://", "qrcode_key", "SESSDATA", "refresh_token", "message", "title"} {
		if bytes.Contains(b, []byte(forbidden)) {
			t.Fatal("serialized report contains private/arbitrary data")
		}
	}
}
func failedAt(t *testing.T, r Report, step int, category, status string) {
	t.Helper()
	if r.Passed() || len(r.Observations) != 5 {
		t.Fatal("failure became success or dropped remaining observations")
	}
	for i, o := range r.Observations {
		want := "pass"
		if i == step {
			want = status
		} else if i > step {
			want = "not-tested"
		}
		if o.Status != want {
			t.Fatalf("step %d status %s, want %s", i, o.Status, want)
		}
	}
	if r.Observations[step].Category != category {
		t.Fatalf("category %q, want %q", r.Observations[step].Category, category)
	}
	checkPrivate(t, r)
}

func TestFixedSequenceAndSanitizedSuccess(t *testing.T) {
	for _, nav := range [][]byte{successBodies[0], []byte(`{"code":0,"data":{"isLogin":false,"uname":"PRIVATE_SYNTHETIC_SENTINEL"}}`)} {
		bodies := fixtures()
		bodies[0] = nav
		s := &script{t: t, bodies: bodies}
		r := Run(context.Background(), s)
		s.verify()
		if !r.Passed() || s.calls != 5 {
			t.Fatalf("five-step success unavailable: %+v", r)
		}
		if o := r.Observations[4]; o.Entries != 1 || o.Supported != 1 || o.Unsupported != 0 {
			t.Fatal("wrong segment counts")
		}
		if r.Observations[1].ServiceCode == nil || *r.Observations[1].ServiceCode != 0 || r.Observations[2].PollCode == nil || *r.Observations[2].PollCode != 86101 {
			t.Fatal("explicit codes missing")
		}
		checkPrivate(t, r)
	}
}

func TestJSONFailuresStopWithoutFallback(t *testing.T) {
	tests := []struct {
		name           string
		step           int
		body, category string
	}{
		{"missing-code", 0, `{"data":{"isLogin":false}}`, "invalid-response"},
		{"null-code", 0, `{"code":null}`, "invalid-response"},
		{"null-envelope", 0, `null`, "invalid-response"},
		{"invalid-json", 0, `PRIVATE_SYNTHETIC_SENTINEL`, "invalid-response"},
		{"wrong-code-type", 0, `{"code":"0"}`, "invalid-response"},
		{"service-denial", 0, `{"code":-412,"message":"PRIVATE_SYNTHETIC_SENTINEL"}`, "service-rejected"},
		{"missing-nav-data", 0, `{"code":0}`, "invalid-response"},
		{"null-nav-data", 0, `{"code":0,"data":null}`, "invalid-response"},
		{"missing-login-bool", 0, `{"code":0,"data":{}}`, "invalid-response"},
		{"null-login-bool", 0, `{"code":0,"data":{"isLogin":null}}`, "invalid-response"},
		{"logged-in", 0, `{"code":0,"data":{"isLogin":true}}`, "unexpected-session"},
		{"null-qr", 1, `{"code":0,"data":null}`, "invalid-response"},
		{"missing-key", 1, `{"code":0,"data":{"url":"` + challenge + `"}}`, "invalid-response"},
		{"qr-denial", 1, `{"code":-400}`, "service-rejected"},
		{"poll-missing-code", 2, `{"code":0,"data":{}}`, "invalid-response"},
		{"poll-null-code", 2, `{"code":0,"data":{"code":null}}`, "invalid-response"},
		{"poll-denied", 2, `{"code":-400}`, "service-rejected"},
		{"poll-scanned", 2, `{"code":0,"data":{"code":86090}}`, "unexpected-qr-state"},
		{"poll-expired", 2, `{"code":0,"data":{"code":86038}}`, "unexpected-qr-state"},
		{"poll-success", 2, `{"code":0,"data":{"code":0,"url":"PRIVATE_SYNTHETIC_SENTINEL"}}`, "unexpected-qr-state"},
		{"poll-unknown", 2, `{"code":0,"data":{"code":99999}}`, "unexpected-qr-state"},
		{"detail-null", 3, `{"code":0,"data":null}`, "invalid-response"},
		{"detail-denied", 3, `{"code":-404}`, "service-rejected"},
		{"detail-wrong-video", 3, `{"code":0,"data":{"bvid":"OTHER","aid":899574,"pages":[{"cid":1445374}]}}`, "invalid-response"},
		{"detail-zero-aid", 3, `{"code":0,"data":{"bvid":"BV1Zs411o7Ee","aid":0,"pages":[{"cid":1445374}]}}`, "invalid-response"},
		{"detail-wrong-cid", 3, `{"code":0,"data":{"bvid":"BV1Zs411o7Ee","aid":899574,"pages":[{"cid":1}]}}`, "invalid-response"},
		{"detail-missing-pages", 3, `{"code":0,"data":{"bvid":"BV1Zs411o7Ee","aid":899574}}`, "invalid-response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodies := fixtures()
			bodies[tt.step] = []byte(tt.body)
			s := &script{t: t, bodies: bodies[:tt.step+1]}
			r := Run(context.Background(), s)
			s.verify()
			failedAt(t, r, tt.step, tt.category, "fail")
			if s.calls != tt.step+1 {
				t.Fatal("wrong stop point")
			}
		})
	}
}

func TestQRURLAndSizeBoundary(t *testing.T) {
	for _, u := range []string{"", "http://passport.bilibili.com/scan", "https://other.invalid/scan", "https://user:pass@passport.bilibili.com/scan", "https://passport.bilibili.com:443/scan", "://bad", strings.Repeat("a", 2049)} {
		raw, _ := json.Marshal(map[string]string{"url": u, "qrcode_key": "test"})
		if _, category := decodeJSON(1, raw, "", &Observation{}); category != "invalid-response" {
			t.Fatal("invalid QR URL accepted")
		}
	}
	for _, n := range []int{0, 256, 257} {
		raw, _ := json.Marshal(map[string]string{"url": challenge, "qrcode_key": strings.Repeat("k", n)})
		key, category := decodeJSON(1, raw, "", &Observation{})
		if n == 256 {
			if category != "" || len(key) != 256 {
				t.Fatal("valid key boundary rejected")
			}
		} else if category != "invalid-response" {
			t.Fatal("invalid key bound accepted")
		}
	}
	for _, n := range []int{2048, 2049} {
		prefix := "https://passport.bilibili.com/"
		raw, _ := json.Marshal(map[string]string{"url": prefix + strings.Repeat("a", n-len(prefix)), "qrcode_key": "test"})
		_, category := decodeJSON(1, raw, "", &Observation{})
		if (category == "") != (n == 2048) {
			t.Fatal("QR URL size boundary incorrect")
		}
	}
}

func TestReadLimitsAndBodyClosure(t *testing.T) {
	for _, step := range []int{0, 4} {
		limit := jsonLimit
		if step == 4 {
			limit = segmentLimit
		}
		for _, extra := range []int{0, 1} {
			t.Run(string(rune('0'+step))+string(rune('0'+extra)), func(t *testing.T) {
				bodies := fixtures()
				if step == 0 {
					bodies[0] = append(bytes.Repeat([]byte(" "), limit+extra-len(successBodies[0])), successBodies[0]...)
				} else {
					// Unknown bytes field makes a valid segment exactly at the response limit.
					padding := limit + extra - len(successBodies[4]) - 6
					bodies[4] = append(append([]byte(nil), successBodies[4]...), fieldBytes(99, bytes.Repeat([]byte("p"), padding))...)
					if len(bodies[4]) != limit+extra {
						t.Fatal("test fixture length")
					}
				}
				s := &script{t: t, bodies: bodies}
				r := Run(context.Background(), s)
				s.verify()
				if extra == 0 {
					if !r.Passed() {
						t.Fatalf("exact response limit rejected: %+v", r)
					}
				} else {
					failedAt(t, r, step, "body-limit", "fail")
				}
				if r.Observations[step].Bytes != limit+extra {
					t.Fatal("incorrect bounded byte count")
				}
			})
		}
	}
}

func TestHTTPFailuresAndTransportPrivacy(t *testing.T) {
	for _, status := range []int{301, 302, 307, 308, 401, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			b := &trackedBody{Reader: strings.NewReader(sentinel)}
			r := Run(context.Background(), transportFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 {
					t.Fatal("redirect followed or request retried")
				}
				return &http.Response{StatusCode: status, Body: b, Header: http.Header{"Location": {"https://api.bilibili.com/" + sentinel}, "Set-Cookie": {"SESSDATA=" + sentinel}}}, nil
			}))
			category := "http-status"
			if status < 400 {
				category = "redirect"
			}
			failedAt(t, r, 0, category, "fail")
			if !b.closed || calls != 1 || r.Observations[0].HTTPStatus != status {
				t.Fatal("HTTP failure not closed/recorded")
			}
		})
	}
	calls := 0
	r := Run(context.Background(), transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("https://passport.bilibili.com/?key=" + sentinel)
	}))
	failedAt(t, r, 0, "transport", "fail")
	if calls != 1 {
		t.Fatal("transport error retried")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New(sentinel) }

type canceledReader struct{ ctx context.Context }

func (r canceledReader) Read([]byte) (int, error) { <-r.ctx.Done(); return 0, errors.New(sentinel) }

func TestBodyErrorsCancellationAndDeadline(t *testing.T) {
	for _, closeFailure := range []bool{false, true} {
		b := &trackedBody{Reader: errorReader{}}
		if closeFailure {
			b.Reader = bytes.NewReader(successBodies[0])
			b.closeErr = errors.New(sentinel)
		}
		r := Run(context.Background(), transportFunc(func(*http.Request) (*http.Response, error) { return &http.Response{StatusCode: 200, Body: b}, nil }))
		failedAt(t, r, 0, "body-read", "fail")
		if !b.closed {
			t.Fatal("failed body leaked")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	r := Run(ctx, transportFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New(sentinel) }))
	failedAt(t, r, 0, "canceled", "fail")
	if calls != 0 {
		t.Fatal("canceled invocation made a request")
	}
	for _, duringBody := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		var body *trackedBody
		started := time.Now()
		r := Run(ctx, transportFunc(func(req *http.Request) (*http.Response, error) {
			if duringBody {
				body = &trackedBody{Reader: canceledReader{req.Context()}}
				return &http.Response{StatusCode: 200, Body: body}, nil
			}
			<-req.Context().Done()
			return nil, errors.New(sentinel)
		}))
		cancel()
		failedAt(t, r, 0, "deadline", "fail")
		if time.Since(started) > time.Second || duringBody && !body.closed {
			t.Fatal("deadline or body cleanup failed")
		}
	}
}

func TestSegmentObservationAndReportCompleteness(t *testing.T) {
	for _, tt := range []struct {
		name             string
		body             []byte
		category, status string
	}{
		{"empty", nil, "danmaku-unavailable", "inconclusive"},
		{"disabled", append(append([]byte(nil), successBodies[4]...), fieldNumber(2, 1)...), "danmaku-unavailable", "inconclusive"},
		{"malformed", []byte{0xff}, "invalid-response", "fail"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bodies := fixtures()
			bodies[4] = tt.body
			s := &script{t: t, bodies: bodies}
			r := Run(context.Background(), s)
			s.verify()
			failedAt(t, r, 4, tt.category, tt.status)
		})
	}
	if (Report{}).Passed() || (Report{Observations: []Observation{{Status: "pass"}}}).Passed() {
		t.Fatal("incomplete report passes")
	}
}

func TestProductionTransportIsolation(t *testing.T) {
	transport := productionTransport()
	defer transport.CloseIdleConnections()
	if transport.Proxy != nil || !transport.DisableKeepAlives || transport.DialContext == nil || transport.TLSHandshakeTimeout != requestLimit || transport.Protocols == nil || !transport.Protocols.HTTP1() || transport.Protocols.HTTP2() || transport.Protocols.UnencryptedHTTP2() {
		t.Fatal("production HTTP is not isolated, bounded HTTP/1 without connection reuse")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification disabled")
	}
}
