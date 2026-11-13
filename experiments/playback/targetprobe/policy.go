// Package targetprobe owns the one reviewed target input and its safe lifecycle.
// It performs no service discovery, metadata requests or media capture.
package targetprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"my-bilibili/experiments/playback/contract"
)

const DescriptorLimit = 32 << 10
const SessionLimit = 90 * time.Second
const PolicyID = "0008-source1-all-https-hosts-v2"
const PublicReferer = "https://www.bilibili.com/video/BV1Zs411o7Ee/"
const PublicUserAgent = "my-bilibili-m1-probe/0008"

type Source struct {
	BVID string `json:"bvid"`
	AVID int64  `json:"avid"`
	CID  int64  `json:"cid"`
	Page int    `json:"page"`
}
type Rendition struct {
	VideoQuality int    `json:"video_quality"`
	VideoCodec   string `json:"video_codec"`
	AudioID      int    `json:"audio_id"`
	AudioCodec   string `json:"audio_codec"`
}
type Grant struct {
	Kind    string `json:"kind"`
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
}
type descriptor struct {
	Version          int               `json:"version"`
	Source           Source            `json:"source"`
	Rendition        Rendition         `json:"rendition"`
	Grant            Grant             `json:"grant"`
	ObservedAt       time.Time         `json:"observed_at"`
	FreshUntil       time.Time         `json:"fresh_until"`
	ServiceExpiresAt *time.Time        `json:"service_expires_at"`
	VideoURL         string            `json:"video_url"`
	AudioURL         string            `json:"audio_url"`
	Headers          map[string]string `json:"headers"`
}

// Metadata is the complete public evidence boundary. It has no locator/header fields.
type Metadata struct {
	Policy           string     `json:"policy"`
	Source           Source     `json:"source"`
	Rendition        Rendition  `json:"rendition"`
	Grant            Grant      `json:"grant"`
	ObservedAt       time.Time  `json:"observed_at"`
	FreshUntil       time.Time  `json:"fresh_until"`
	ServiceExpiresAt *time.Time `json:"service_expires_at"`
}

// Policy has no exported mutable state. A descriptor can only match this independently
// reviewed source/rendition policy; it cannot extend it. Hosts are unrestricted.
type Policy struct {
	d   descriptor
	now func() time.Time
}

func clean(s string) bool { return utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl) }

// ValidateMediaURL checks bounded HTTPS URL syntax without restricting hosts or
// resolving them. It does not normalize or reconstruct the private locator.
func ValidateMediaURL(raw string) error {
	if len(raw) == 0 || len(raw) > 4096 || !clean(raw) || strings.Contains(raw, " ") {
		return contract.Invalid
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || !clean(u.Host) || u.Opaque != "" || strings.Contains(raw, "#") {
		return contract.Invalid
	}
	q, e := url.QueryUnescape(u.RawQuery)
	if e != nil || !clean(q) || !clean(u.Path) {
		return contract.Invalid
	}
	if strings.HasSuffix(u.Host, ":") {
		return contract.Invalid
	}
	if port := u.Port(); port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return contract.Invalid
		}
	}
	if strings.HasPrefix(u.Host, "[") {
		addr, err := netip.ParseAddr(u.Hostname())
		if err != nil || !addr.Is6() {
			return contract.Invalid
		}
	} else if strings.Contains(u.Hostname(), ":") {
		return contract.Invalid
	}
	return nil
}

// ReadDescriptor owns r for this read, closes it on every outcome and never returns
// partial input. Callers must supply a deadline (the native/driver startup uses 10s).
func ReadDescriptor(ctx context.Context, r io.ReadCloser) ([]byte, error) {
	if ctx == nil || r == nil {
		return nil, contract.Invalid
	}
	if _, ok := ctx.Deadline(); !ok {
		_ = r.Close()
		return nil, contract.Invalid
	}
	type result struct {
		b []byte
		e error
	}
	done := make(chan result, 1)
	go func() { b, e := io.ReadAll(io.LimitReader(r, DescriptorLimit+1)); done <- result{b, e} }()
	select {
	case out := <-done:
		_ = r.Close()
		if ctx.Err() != nil {
			return nil, contextCode(ctx)
		}
		if out.e != nil || len(out.b) == 0 || len(out.b) > DescriptorLimit {
			return nil, contract.Invalid
		}
		return out.b, nil
	case <-ctx.Done():
		_ = r.Close()
		return nil, contextCode(ctx)
	}
}
func contextCode(ctx context.Context) contract.ErrorCode {
	if ctx.Err() == context.DeadlineExceeded {
		return contract.Timeout
	}
	return contract.Canceled
}

// strictObject rejects duplicate keys even in nested objects. The descriptor has
// only exact bounded integers, strings, objects and one explicit nullable time.
func strictObject(b []byte) (map[string]any, error) {
	if len(b) == 0 || len(b) > DescriptorLimit || !utf8.Valid(b) {
		return nil, contract.Invalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		t, e := d.Token()
		if e != nil {
			return nil, contract.Invalid
		}
		switch x := t.(type) {
		case json.Delim:
			if depth >= 8 || x != '{' {
				return nil, contract.Invalid
			}
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, contract.Invalid
				}
				s, ok := k.(string)
				if !ok {
					return nil, contract.Invalid
				}
				if _, ok := m[s]; ok {
					return nil, contract.Invalid
				}
				v, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				if v == nil && (depth != 0 || s != "service_expires_at") {
					return nil, contract.Invalid
				}
				m[s] = v
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return nil, contract.Invalid
			}
			return m, nil
		case json.Number:
			if _, e := x.Int64(); e != nil {
				return nil, contract.Invalid
			}
			return x, nil
		case string:
			if !clean(x) {
				return nil, contract.Invalid
			}
			return x, nil
		case nil:
			return nil, nil
		default:
			return nil, contract.Invalid
		}
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, contract.Invalid
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, contract.Invalid
	}
	return m, nil
}
func keys(m map[string]any, names ...string) bool {
	if len(m) != len(names) {
		return false
	}
	for _, n := range names {
		if _, ok := m[n]; !ok {
			return false
		}
	}
	return true
}
func objectKeys(m map[string]any, k string, names ...string) bool {
	v, ok := m[k].(map[string]any)
	return ok && keys(v, names...)
}

func Parse(b []byte, now func() time.Time) (*Policy, error) {
	m, e := strictObject(b)
	if e != nil {
		return nil, e
	}
	if !keys(m, "version", "source", "rendition", "grant", "observed_at", "fresh_until", "service_expires_at", "video_url", "audio_url", "headers") ||
		!objectKeys(m, "source", "bvid", "avid", "cid", "page") || !objectKeys(m, "rendition", "video_quality", "video_codec", "audio_id", "audio_codec") ||
		!objectKeys(m, "grant", "kind", "start_ms", "end_ms") || !objectKeys(m, "headers", "Referer", "User-Agent") {
		return nil, contract.Invalid
	}
	var d descriptor
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil {
		return nil, contract.Invalid
	}
	if d.Version != 2 || d.Source != (Source{"BV1Zs411o7Ee", 899574, 1445374, 1}) || d.Rendition != (Rendition{32, "H264/AVC", 30216, "AAC-LC"}) || d.Grant != (Grant{"full", 0, 596624}) {
		return nil, contract.Invalid
	}
	if d.ObservedAt.IsZero() || d.FreshUntil.IsZero() || !d.FreshUntil.After(d.ObservedAt) || d.FreshUntil.After(d.ObservedAt.Add(SessionLimit)) || d.ServiceExpiresAt != nil && (d.ServiceExpiresAt.IsZero() || d.FreshUntil.After(*d.ServiceExpiresAt)) {
		return nil, contract.Invalid
	}
	if now == nil {
		now = time.Now
	}
	p := &Policy{d: d, now: now}
	p.d.Headers = maps.Clone(d.Headers)
	if e := p.Validate(p.Input()); e != nil {
		return nil, e
	}
	return p, nil
}
func (p *Policy) Input() contract.Input {
	if p == nil {
		return contract.Input{}
	}
	return contract.Input{URL: p.d.VideoURL, AudioURL: p.d.AudioURL, Headers: maps.Clone(p.d.Headers)}
}
func (p *Policy) Deadline() time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.d.FreshUntil
}
func (p *Policy) Metadata() Metadata {
	if p == nil {
		return Metadata{}
	}
	m := Metadata{PolicyID, p.d.Source, p.d.Rendition, p.d.Grant, p.d.ObservedAt, p.d.FreshUntil, nil}
	if p.d.ServiceExpiresAt != nil {
		t := *p.d.ServiceExpiresAt
		m.ServiceExpiresAt = &t
	}
	return m
}
func (p *Policy) Validate(in contract.Input) error {
	if p == nil || p.now == nil {
		return contract.Invalid
	}
	now := p.now()
	if p.d.ObservedAt.After(now) {
		return contract.Invalid
	}
	if !now.Before(p.d.FreshUntil) {
		return contract.Timeout
	}
	if in.FilePath != "" || in.URL != p.d.VideoURL || in.AudioURL != p.d.AudioURL || len(in.Headers) != 2 || len(in.Headers) > 8 {
		return contract.Invalid
	}
	if ValidateMediaURL(in.URL) != nil {
		return contract.Invalid
	}
	if ValidateMediaURL(in.AudioURL) != nil {
		return contract.Invalid
	}
	size := 0
	for k, v := range in.Headers {
		size += len(k) + len(v)
		if !clean(k) || !clean(v) {
			return contract.Invalid
		}
		switch k {
		case "Referer":
			if v != PublicReferer {
				return contract.Invalid
			}
		case "User-Agent":
			if v != PublicUserAgent {
				return contract.Invalid
			}
		default:
			return contract.Invalid
		}
	}
	if size > 4096 || !maps.Equal(in.Headers, p.d.Headers) {
		return contract.Invalid
	}
	return nil
}
