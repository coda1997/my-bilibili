package serviceprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// QRDiagnostic describes a response format, not acceptance of a QR challenge.
// A passing observation may have PriorValidatorAccepts=false.
type QRDiagnostic struct {
	Observation Observation        `json:"observation"`
	Facts       *QRDiagnosticFacts `json:"facts,omitempty"`
}

// QRDiagnosticFacts contains no service-derived string. The three Kind fields
// are fixed enums; nil pointers distinguish unassessed facts from zero/false.
type QRDiagnosticFacts struct {
	DataKind              string `json:"data_kind"`
	KeyKind               string `json:"key_kind"`
	URLKind               string `json:"url_kind"`
	KeyBytes              *int   `json:"key_bytes,omitempty"`
	URLBytes              *int   `json:"url_bytes,omitempty"`
	URLParsed             *bool  `json:"url_parsed,omitempty"`
	HTTPS                 *bool  `json:"https,omitempty"`
	PassportHostMatch     *bool  `json:"passport_host_match,omitempty"`
	AccountHostMatch      *bool  `json:"account_host_match,omitempty"`
	UserInfoPresent       *bool  `json:"userinfo_present,omitempty"`
	PriorValidatorAccepts bool   `json:"prior_validator_accepts"`
}

// RunQRDiagnostic makes one fixed QR-generation request. It does not display,
// visit, persist or poll the returned challenge. Injected transports obey the
// same cancellation contract as Run; nil selects the existing private transport.
func RunQRDiagnostic(ctx context.Context, transport http.RoundTripper) QRDiagnostic {
	if transport == nil {
		transport = productionTransport()
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	r := QRDiagnostic{Observation: Observation{Step: "qr-format-diagnostic", Status: "fail", Category: "transport"}}
	body, category := fetch(ctx, client, "https://passport.bilibili.com/x/passport-login/web/qrcode/generate", jsonLimit, &r.Observation)
	if category != "" {
		r.Observation.Category = category
		return r
	}
	var envelope struct {
		Code *int64          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Code == nil {
		r.Observation.Category = "invalid-response"
		return r
	}
	r.Observation.ServiceCode = envelope.Code
	if *envelope.Code != 0 {
		r.Observation.Category = "service-rejected"
		return r
	}
	facts, ok := describeQR(envelope.Data)
	if !ok {
		r.Observation.Category = "invalid-response"
		return r
	}
	_, priorCategory := decodeQR(envelope.Data, "", false)
	facts.PriorValidatorAccepts = priorCategory == ""
	r.Facts = facts
	r.Observation.Status, r.Observation.Category = "pass", "observed-format"
	return r
}

// Raw values originate in the already decoded envelope. Inspect only expected
// fields, preserving absent/null and the original raw data for old-validator
// parity (including its duplicate-field behavior).
func describeQR(data json.RawMessage) (*QRDiagnosticFacts, bool) {
	f := &QRDiagnosticFacts{DataKind: qrKind(data), KeyKind: "absent", URLKind: "absent"}
	if f.DataKind != "object" {
		return f, true
	}
	var fields struct {
		Key json.RawMessage `json:"qrcode_key"`
		URL json.RawMessage `json:"url"`
	}
	if json.Unmarshal(data, &fields) != nil {
		return nil, false
	}
	f.KeyKind, f.URLKind = qrKind(fields.Key), qrKind(fields.URL)
	if f.KeyKind == "string" {
		var key string
		if json.Unmarshal(fields.Key, &key) != nil {
			return nil, false
		}
		n := len(key)
		f.KeyBytes = &n
	}
	if f.URLKind == "string" {
		var value string
		if json.Unmarshal(fields.URL, &value) != nil {
			return nil, false
		}
		n := len(value)
		f.URLBytes = &n
		if n > 0 && n <= 2048 {
			u, err := url.Parse(value)
			parsed := err == nil
			f.URLParsed = &parsed
			if err == nil {
				https, passport, account, userinfo := u.Scheme == "https", u.Host == "passport.bilibili.com", u.Host == "account.bilibili.com", u.User != nil
				f.HTTPS, f.PassportHostMatch, f.AccountHostMatch, f.UserInfoPresent = &https, &passport, &account, &userinfo
			}
		}
	}
	return f, true
}

func qrKind(value json.RawMessage) string {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return "absent"
	}
	switch value[0] {
	case 'n':
		return "null"
	case '"':
		return "string"
	case '{':
		return "object"
	default:
		return "other"
	}
}
