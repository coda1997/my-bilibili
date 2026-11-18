// Package account adapts the fixed web login protocol to account-owned ports.
package account

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	domain "my-bilibili/internal/account"
	"my-bilibili/internal/adapters/bilibili/sessiondata"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const bodyLimit = 1 << 20
const requestLimit = 10 * time.Second

type Gateway struct {
	client *http.Client
	now    func() time.Time
}

func New(transport http.RoundTripper) *Gateway {
	if transport == nil {
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: requestLimit}).DialContext, TLSHandshakeTimeout: requestLimit, DisableKeepAlives: true, Protocols: protocols}
	}
	return &Gateway{client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now}
}
func (g *Gateway) Close() { g.client.CloseIdleConnections() }
func (g *Gateway) request(ctx context.Context, endpoint string, cookies *sessiondata.Session) ([]byte, http.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, requestLimit)
	defer cancel()
	if ctx.Err() != nil {
		return nil, nil, contextCode(ctx.Err())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, domain.Invalid
	}
	req.Header.Set("User-Agent", "my-bilibili-p0-feasibility/1")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	if cookies != nil {
		if err := cookies.Apply(req, g.now()); err != nil {
			return nil, nil, err
		}
	}
	resp, err := g.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, contextCode(ctx.Err())
		}
		return nil, nil, domain.Network
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, nil, domain.ServiceRejected
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	closeErr := resp.Body.Close()
	if ctx.Err() != nil {
		return nil, nil, contextCode(ctx.Err())
	}
	if readErr != nil || closeErr != nil {
		return nil, nil, domain.Network
	}
	if len(body) > bodyLimit {
		return nil, nil, domain.Invalid
	}
	return body, resp.Header, nil
}
func contextCode(err error) domain.Code {
	if err == context.DeadlineExceeded {
		return domain.Timeout
	}
	return domain.Canceled
}
func envelope(body []byte) (int64, json.RawMessage, error) {
	var value struct {
		Code *int64          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &value) != nil || value.Code == nil {
		return 0, nil, domain.Invalid
	}
	return *value.Code, value.Data, nil
}
func (g *Gateway) Begin(ctx context.Context) (domain.Attempt, error) {
	body, _, err := g.request(ctx, "https://passport.bilibili.com/x/passport-login/web/qrcode/generate", nil)
	if err != nil {
		return nil, err
	}
	code, data, err := envelope(body)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, domain.ServiceRejected
	}
	var value struct {
		URL string `json:"url"`
		Key string `json:"qrcode_key"`
	}
	if json.Unmarshal(data, &value) != nil || len(value.Key) != 32 || len(value.URL) == 0 || len(value.URL) > 2048 {
		return nil, domain.Invalid
	}
	for _, c := range []byte(value.Key) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return nil, domain.Invalid
		}
	}
	u, err := url.Parse(value.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host != "passport.bilibili.com" && u.Host != "account.bilibili.com" {
		return nil, domain.Invalid
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query["qrcode_key"]) != 1 || query.Get("qrcode_key") != value.Key {
		return nil, domain.Invalid
	}
	return &attempt{gateway: g, key: value.Key, qr: domain.NewQR(value.URL)}, nil
}

type attempt struct {
	mu      sync.Mutex
	gateway *Gateway
	key     string
	qr      domain.QR
}

func (a *attempt) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, "[private login attempt]") }
func (a *attempt) QR() domain.QR              { a.mu.Lock(); defer a.mu.Unlock(); return a.qr }
func (a *attempt) Close()                     { a.mu.Lock(); defer a.mu.Unlock(); a.key = ""; a.qr = domain.QR{} }
func (a *attempt) Poll(ctx context.Context) (domain.PollResult, error) {
	a.mu.Lock()
	key := a.key
	a.mu.Unlock()
	if key == "" {
		return domain.PollResult{}, domain.Closed
	}
	body, headers, err := a.gateway.request(ctx, "https://passport.bilibili.com/x/passport-login/web/qrcode/poll?"+url.Values{"qrcode_key": {key}}.Encode(), nil)
	a.mu.Lock()
	active := a.key != ""
	a.mu.Unlock()
	if !active {
		return domain.PollResult{}, domain.Canceled
	}
	if err != nil {
		return domain.PollResult{}, err
	}
	code, data, err := envelope(body)
	if err != nil {
		return domain.PollResult{}, err
	}
	if code != 0 {
		return domain.PollResult{}, domain.ServiceRejected
	}
	var value struct {
		Code *int64 `json:"code"`
	}
	if json.Unmarshal(data, &value) != nil || value.Code == nil {
		return domain.PollResult{}, domain.Invalid
	}
	switch *value.Code {
	case 86101:
		return domain.PollResult{Status: domain.PollWaiting}, nil
	case 86090:
		return domain.PollResult{Status: domain.PollScanned}, nil
	case 86038:
		return domain.PollResult{Status: domain.PollExpired}, nil
	case 0:
		material, err := a.gateway.cookies(headers.Values("Set-Cookie"))
		if err != nil {
			return domain.PollResult{}, err
		}
		return domain.PollResult{Status: domain.PollComplete, Material: material}, nil
	default:
		return domain.PollResult{}, domain.ServiceRejected
	}
}

func (g *Gateway) cookies(lines []string) (domain.Material, error) {
	return sessiondata.Parse(lines, g.now())
}
func (g *Gateway) decode(material domain.Material) (*sessiondata.Session, error) {
	return sessiondata.Decode(material, g.now())
}

func (g *Gateway) Verify(ctx context.Context, material domain.Material) error {
	cookies, err := g.decode(material)
	if err != nil {
		return err
	}
	body, _, err := g.request(ctx, "https://api.bilibili.com/x/web-interface/nav", cookies)
	if err != nil {
		return err
	}
	code, data, err := envelope(body)
	if err != nil {
		return err
	}
	if code == -101 {
		return domain.ExpiredSession
	}
	if code != 0 {
		return domain.ServiceRejected
	}
	var value struct {
		IsLogin *bool `json:"isLogin"`
	}
	if json.Unmarshal(data, &value) != nil || value.IsLogin == nil {
		return domain.Invalid
	}
	if !*value.IsLogin {
		return domain.ExpiredSession
	}
	return nil
}
