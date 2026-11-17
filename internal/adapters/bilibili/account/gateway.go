// Package account adapts the fixed web login protocol to account-owned ports.
package account

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	domain "my-bilibili/internal/account"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const bodyLimit = 1 << 20
const materialLimit = 2560
const requestLimit = 10 * time.Second

var names = []string{"SESSDATA", "bili_jct", "DedeUserID", "DedeUserID__ckMd5"}

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
func (g *Gateway) request(ctx context.Context, endpoint string, cookies []cookieRecord) ([]byte, http.Header, error) {
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
	for _, cookie := range cookies {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
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

type cookieRecord struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Expires *int64 `json:"expires"`
}
type session struct {
	Version int            `json:"version"`
	Scope   string         `json:"scope"`
	Cookies []cookieRecord `json:"cookies"`
}

func nameIndex(name string) int {
	for i, n := range names {
		if name == n {
			return i
		}
	}
	return -1
}
func validValue(name, value string) bool {
	return len(value) > 0 && len(value) <= 1024 && (&http.Cookie{Name: name, Value: value}).Valid() == nil
}
func parentScope(cookie *http.Cookie) bool {
	for _, c := range []byte(cookie.Domain) {
		if c > 127 {
			return false
		}
	}
	return strings.ToLower(strings.TrimPrefix(cookie.Domain, ".")) == "bilibili.com" && cookie.Path == "/"
}
func (g *Gateway) cookies(lines []string) (domain.Material, error) {
	retained := make(map[string]cookieRecord)
	for _, line := range lines {
		first, _, _ := strings.Cut(line, ";")
		rawName, _, hasValue := strings.Cut(first, "=")
		name := strings.TrimSpace(rawName)
		if nameIndex(name) < 0 {
			continue
		}
		if !hasValue {
			return domain.Material{}, domain.Invalid
		}
		cookie, err := http.ParseSetCookie(line)
		if err != nil || cookie.Valid() != nil || !validValue(cookie.Name, cookie.Value) || !parentScope(cookie) {
			return domain.Material{}, domain.Invalid
		}
		if _, duplicate := retained[cookie.Name]; duplicate {
			return domain.Material{}, domain.Invalid
		}
		for _, attr := range cookie.Unparsed {
			key, _, _ := strings.Cut(attr, "=")
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "domain", "path", "expires", "max-age":
				return domain.Material{}, domain.Invalid
			}
		}
		now := g.now().Unix()
		if cookie.MaxAge < 0 || cookie.RawExpires != "" && cookie.Expires.IsZero() || !cookie.Expires.IsZero() && cookie.Expires.Unix() <= now {
			return domain.Material{}, domain.Invalid
		}
		expiry := int64(0)
		if !cookie.Expires.IsZero() {
			expiry = cookie.Expires.Unix()
		}
		if cookie.MaxAge > 0 {
			if int64(cookie.MaxAge) > math.MaxInt64-now {
				return domain.Material{}, domain.Invalid
			}
			expiry = now + int64(cookie.MaxAge)
		}
		retained[cookie.Name] = cookieRecord{Name: cookie.Name, Value: cookie.Value, Expires: &expiry}
	}
	if _, ok := retained["SESSDATA"]; !ok {
		return domain.Material{}, domain.Invalid
	}
	value := session{Version: 1, Scope: "bilibili.com/"}
	for _, name := range names {
		if record, ok := retained[name]; ok {
			value.Cookies = append(value.Cookies, record)
		}
	}
	body, err := json.Marshal(value)
	if err != nil || len(body) > materialLimit {
		return domain.Material{}, domain.Invalid
	}
	return domain.NewMaterial(body), nil
}
func (g *Gateway) decode(material domain.Material) ([]cookieRecord, error) {
	body := material.Bytes()
	if len(body) == 0 || len(body) > materialLimit || !validSessionJSON(body) {
		return nil, domain.Invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var value session
	if decoder.Decode(&value) != nil || value.Version != 1 || value.Scope != "bilibili.com/" || len(value.Cookies) == 0 || len(value.Cookies) > len(names) {
		return nil, domain.Invalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, domain.Invalid
	}
	prior := -1
	for _, record := range value.Cookies {
		index := nameIndex(record.Name)
		if index <= prior || record.Expires == nil || *record.Expires < 0 || !validValue(record.Name, record.Value) {
			return nil, domain.Invalid
		}
		if prior < 0 && index != 0 {
			return nil, domain.Invalid
		}
		if *record.Expires > 0 && *record.Expires <= g.now().Unix() {
			return nil, domain.ExpiredSession
		}
		prior = index
	}
	return value.Cookies, nil
}

// Validate exact schema keys and reject duplicates before typed decoding. The
// only objects are the session and cookie records, bounding nesting as well.
func validSessionJSON(body []byte) bool {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		token, err := d.Token()
		if err != nil {
			return false
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delimiter {
		case '{':
			if depth != 0 && depth != 2 {
				return false
			}
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return false
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return false
				}
				if depth == 0 && name != "version" && name != "scope" && name != "cookies" {
					return false
				}
				if depth == 2 && name != "name" && name != "value" && name != "expires" {
					return false
				}
				seen[name] = true
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}') && len(seen) == 3
		case '[':
			if depth != 1 {
				return false
			}
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !value(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
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
