// Package serviceprobe observes a fixed anonymous service sequence. Reports
// contain only allowlisted outcomes and counts, never request or response data.
package serviceprobe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	requestLimit = 10 * time.Second
	jsonLimit    = 1 << 20
	segmentLimit = 4 << 20
	videoID      = "BV1Zs411o7Ee"
	partID       = 1445374
)

// Observation deliberately has no arbitrary message, URL, header or text field.
// A nil code means absent, distinct from an explicitly observed zero.
type Observation struct {
	Step         string `json:"step"`
	Status       string `json:"status"`
	Category     string `json:"category"`
	HTTPStatus   int    `json:"http_status,omitempty"`
	ServiceCode  *int64 `json:"service_code,omitempty"`
	PollCode     *int64 `json:"poll_code,omitempty"`
	Bytes        int    `json:"bytes"`
	Entries      int    `json:"entries,omitempty"`
	Supported    int    `json:"supported,omitempty"`
	Unsupported  int    `json:"unsupported,omitempty"`
	SkippedEmpty int    `json:"skipped_empty,omitempty"`
}

type Report struct {
	Observations []Observation `json:"observations"`
}

func (r Report) Passed() bool {
	if len(r.Observations) != 5 {
		return false
	}
	for _, o := range r.Observations {
		if o.Status != "pass" {
			return false
		}
	}
	return true
}

func productionTransport() *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	return &http.Transport{
		Proxy: nil, DialContext: (&net.Dialer{Timeout: requestLimit}).DialContext,
		TLSHandshakeTimeout: requestLimit, DisableKeepAlives: true, Protocols: protocols,
	}
}

// Run performs at most five sequential requests and stops at the first failure
// or inconclusive result. The supplied context owns the invocation. A nil
// transport selects isolated production HTTP; tests inject controlled transports.
// Injected transports must obey request cancellation, as net/http transports do.
func Run(ctx context.Context, transport http.RoundTripper) Report {
	if transport == nil {
		transport = productionTransport()
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	r := Report{Observations: []Observation{
		{Step: "anonymous-session", Status: "not-tested", Category: "not-reached"},
		{Step: "qr-generate", Status: "not-tested", Category: "not-reached"},
		{Step: "qr-poll", Status: "not-tested", Category: "not-reached"},
		{Step: "video-detail", Status: "not-tested", Category: "not-reached"},
		{Step: "danmaku-segment", Status: "not-tested", Category: "not-reached"},
	}}
	key := ""
	for i := range r.Observations {
		o := &r.Observations[i]
		o.Status, o.Category = "fail", "transport"
		endpoint, limit := "", jsonLimit
		switch i {
		case 0:
			endpoint = "https://api.bilibili.com/x/web-interface/nav"
		case 1:
			endpoint = "https://passport.bilibili.com/x/passport-login/web/qrcode/generate"
		case 2:
			endpoint = "https://passport.bilibili.com/x/passport-login/web/qrcode/poll?" + url.Values{"qrcode_key": {key}}.Encode()
		case 3:
			endpoint = "https://api.bilibili.com/x/web-interface/view?bvid=" + videoID
		case 4:
			endpoint = "https://api.bilibili.com/x/v2/dm/web/seg.so?type=1&oid=1445374&segment_index=1"
			limit = segmentLimit
		}
		body, category := fetch(ctx, client, endpoint, limit, o)
		if category != "" {
			o.Category = category
			break
		}
		if i == 4 {
			counts, valid := decodeSegment(body)
			if !valid {
				o.Category = "invalid-response"
				break
			}
			o.Entries, o.Supported, o.Unsupported = counts.total, counts.supported, counts.unsupported
			o.SkippedEmpty = counts.skippedEmpty
			if counts.unavailable || counts.total == 0 {
				o.Status, o.Category = "inconclusive", "danmaku-unavailable"
				break
			}
		} else {
			var envelope struct {
				Code *int64          `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if json.Unmarshal(body, &envelope) != nil || envelope.Code == nil {
				o.Category = "invalid-response"
				break
			}
			o.ServiceCode = envelope.Code
			if i == 0 && *envelope.Code == -101 {
				o.Status, o.Category = "pass", "anonymous"
				continue
			}
			if *envelope.Code != 0 {
				o.Category = "service-rejected"
				break
			}
			key, category = decodeJSON(i, envelope.Data, key, o)
			if category != "" {
				o.Category = category
				break
			}
		}
		o.Status, o.Category = "pass", "expected-response"
	}
	return r
}

func fetch(ctx context.Context, client *http.Client, endpoint string, limit int, o *Observation) ([]byte, string) {
	ctx, cancel := context.WithTimeout(ctx, requestLimit)
	defer cancel()
	if ctx.Err() != nil {
		return nil, contextCategory(ctx.Err())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "invalid-request"
	}
	req.Header.Set("User-Agent", "my-bilibili-p0-feasibility/1")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, contextCategory(ctx.Err())
		}
		return nil, "transport"
	}
	o.HTTPStatus = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return nil, "redirect"
		}
		return nil, "http-status"
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	closeErr := resp.Body.Close()
	o.Bytes = len(body)
	if ctx.Err() != nil {
		return nil, contextCategory(ctx.Err())
	}
	if readErr != nil || closeErr != nil {
		return nil, "body-read"
	}
	if len(body) > limit {
		return nil, "body-limit"
	}
	return body, ""
}

func contextCategory(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline"
	}
	return "canceled"
}

func decodeJSON(step int, data json.RawMessage, key string, o *Observation) (string, string) {
	switch step {
	case 0:
		var v struct {
			IsLogin *bool `json:"isLogin"`
		}
		if json.Unmarshal(data, &v) != nil || v.IsLogin == nil {
			return key, "invalid-response"
		}
		if *v.IsLogin {
			return key, "unexpected-session"
		}
	case 1:
		return decodeQR(data, key, true)
	case 2:
		var v struct {
			Code *int64 `json:"code"`
		}
		if json.Unmarshal(data, &v) != nil || v.Code == nil {
			return key, "invalid-response"
		}
		o.PollCode = v.Code
		if *v.Code != 86101 {
			return key, "unexpected-qr-state"
		}
	case 3:
		var v struct {
			BVID  string `json:"bvid"`
			AID   int64  `json:"aid"`
			Pages []struct {
				CID int64 `json:"cid"`
			} `json:"pages"`
		}
		if json.Unmarshal(data, &v) != nil || v.BVID != videoID || v.AID <= 0 {
			return key, "invalid-response"
		}
		found := false
		for _, p := range v.Pages {
			found = found || p.CID == partID
		}
		if !found {
			return key, "invalid-response"
		}
	}
	return key, ""
}

// decodeQR keeps the current confirmation-host policy separate from the
// historical diagnostic. Failure preserves the caller's previous key.
func decodeQR(data json.RawMessage, key string, allowAccountHost bool) (string, string) {
	var v struct {
		URL string `json:"url"`
		Key string `json:"qrcode_key"`
	}
	if json.Unmarshal(data, &v) != nil || len(v.Key) == 0 || len(v.Key) > 256 || len(v.URL) == 0 || len(v.URL) > 2048 {
		return key, "invalid-response"
	}
	u, err := url.Parse(v.URL)
	if err != nil || u.Scheme != "https" || (u.Host != "passport.bilibili.com" && (!allowAccountHost || u.Host != "account.bilibili.com")) || u.User != nil {
		return key, "invalid-response"
	}
	return v.Key, ""
}
