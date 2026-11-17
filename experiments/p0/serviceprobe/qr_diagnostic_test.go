package serviceprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type qrTransport struct {
	t      *testing.T
	body   []byte
	status int
	err    error
	calls  int
	opened *trackedBody
	idle   bool
}

func (q *qrTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	q.t.Helper()
	q.calls++
	if q.calls != 1 {
		q.t.Fatal("QR diagnostic made more than one request")
	}
	if r.Method != "GET" || r.URL.String() != "https://passport.bilibili.com/x/passport-login/web/qrcode/generate" {
		q.t.Fatal("QR diagnostic left fixed request")
	}
	if len(r.Header) != 2 || r.Header.Get("User-Agent") != "my-bilibili-p0-feasibility/1" || r.Header.Get("Referer") != "https://www.bilibili.com/" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
		q.t.Fatal("unexpected diagnostic headers")
	}
	if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) <= 0 || time.Until(deadline) > requestLimit {
		q.t.Fatal("missing request deadline")
	}
	if q.err != nil {
		return nil, q.err
	}
	q.opened = &trackedBody{Reader: bytes.NewReader(q.body)}
	status := q.status
	if status == 0 {
		status = 200
	}
	return &http.Response{StatusCode: status, Body: q.opened, Header: http.Header{"Set-Cookie": {"SESSDATA=" + sentinel}, "Location": {"https://" + sentinel + ".invalid/"}}}, nil
}
func (q *qrTransport) CloseIdleConnections() { q.idle = true }
func (q *qrTransport) verify() {
	q.t.Helper()
	if q.calls != 1 || !q.idle || q.opened != nil && !q.opened.closed {
		q.t.Fatal("diagnostic request count or cleanup failed")
	}
}
func qrInt(v int) *int    { return &v }
func qrBool(v bool) *bool { return &v }
func qrFixture(t *testing.T, body string) QRDiagnostic {
	t.Helper()
	q := &qrTransport{t: t, body: []byte(body)}
	r := RunQRDiagnostic(context.Background(), q)
	q.verify()
	qrPrivate(t, r)
	return r
}
func qrPrivate(t *testing.T, r QRDiagnostic) {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{sentinel, "https://", "SESSDATA", "qrcode_key", "refresh_token", "passport.bilibili.com", "account.bilibili.com", "user:password"} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatal("QR report contains response-derived private data")
		}
	}
	if r.Facts != nil {
		for _, kind := range []string{r.Facts.DataKind, r.Facts.KeyKind, r.Facts.URLKind} {
			switch kind {
			case "absent", "null", "string", "object", "other":
			default:
				t.Fatal("response text used as a kind enum")
			}
		}
	}
}

func TestQRDiagnosticKindsAndPriorValidatorParity(t *testing.T) {
	tests := []struct {
		name, data string
		want       QRDiagnosticFacts
	}{
		{"absent", "", QRDiagnosticFacts{DataKind: "absent", KeyKind: "absent", URLKind: "absent"}},
		{"null", "null", QRDiagnosticFacts{DataKind: "null", KeyKind: "absent", URLKind: "absent"}},
		{"string", `"PRIVATE_SYNTHETIC_SENTINEL"`, QRDiagnosticFacts{DataKind: "string", KeyKind: "absent", URLKind: "absent"}},
		{"array", "[]", QRDiagnosticFacts{DataKind: "other", KeyKind: "absent", URLKind: "absent"}},
		{"boolean", "false", QRDiagnosticFacts{DataKind: "other", KeyKind: "absent", URLKind: "absent"}},
		{"number", "1e400", QRDiagnosticFacts{DataKind: "other", KeyKind: "absent", URLKind: "absent"}},
		{"empty-object", "{}", QRDiagnosticFacts{DataKind: "object", KeyKind: "absent", URLKind: "absent"}},
		{"null-fields", `{"qrcode_key":null,"url":null}`, QRDiagnosticFacts{DataKind: "object", KeyKind: "null", URLKind: "null"}},
		{"object-fields", `{"qrcode_key":{},"url":{}}`, QRDiagnosticFacts{DataKind: "object", KeyKind: "object", URLKind: "object"}},
		{"other-fields", `{"qrcode_key":[],"url":1e400}`, QRDiagnosticFacts{DataKind: "object", KeyKind: "other", URLKind: "other"}},
		{"empty-strings", `{"qrcode_key":"","url":""}`, QRDiagnosticFacts{DataKind: "object", KeyKind: "string", URLKind: "string", KeyBytes: qrInt(0), URLBytes: qrInt(0)}},
		{"unicode-key", `{"qrcode_key":"密钥","url":null}`, QRDiagnosticFacts{DataKind: "object", KeyKind: "string", URLKind: "null", KeyBytes: qrInt(6)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"code":0}`
			if tt.data != "" {
				body = `{"code":0,"data":` + tt.data + `}`
			}
			r := qrFixture(t, body)
			if r.Observation.Step != "qr-format-diagnostic" || r.Observation.Status != "pass" || r.Observation.Category != "observed-format" || r.Observation.ServiceCode == nil || *r.Observation.ServiceCode != 0 {
				t.Fatal("valid code-zero format not observed")
			}
			_, prior := decodeQR(json.RawMessage(tt.data), "", false)
			if r.Facts == nil || r.Facts.PriorValidatorAccepts != (prior == "") {
				t.Fatal("prior-validator parity lost")
			}
			if !reflect.DeepEqual(r.Facts, &tt.want) {
				t.Fatalf("unexpected fixed facts: %+v", r.Facts)
			}
		})
	}
}

func TestQRDiagnosticURLFactsAndUnchangedAcceptance(t *testing.T) {
	old := "https://passport.bilibili.com/scan?secret=" + sentinel
	account := "https://account.bilibili.com/h5/account-h5/auth/scan-web?secret=" + sentinel
	for _, tt := range []struct {
		name, value                            string
		parsed, https, passport, account, user *bool
		accept                                 bool
	}{
		{"passport", old, qrBool(true), qrBool(true), qrBool(true), qrBool(false), qrBool(false), true},
		{"account", account, qrBool(true), qrBool(true), qrBool(false), qrBool(true), qrBool(false), false},
		{"unrecognized", "https://" + sentinel + ".invalid/scan", qrBool(true), qrBool(true), qrBool(false), qrBool(false), qrBool(false), false},
		{"http", "http://passport.bilibili.com/scan", qrBool(true), qrBool(false), qrBool(true), qrBool(false), qrBool(false), false},
		{"userinfo", "https://user:password@passport.bilibili.com/scan", qrBool(true), qrBool(true), qrBool(true), qrBool(false), qrBool(true), false},
		{"explicit-port", "https://account.bilibili.com:443/scan", qrBool(true), qrBool(true), qrBool(false), qrBool(false), qrBool(false), false},
		{"malformed", "https://passport.bilibili.com/%zz", qrBool(false), nil, nil, nil, nil, false},
		{"empty", "", nil, nil, nil, nil, nil, false},
		{"oversized", strings.Repeat("a", 2049), nil, nil, nil, nil, nil, false},
		{"exact-limit", "https://passport.bilibili.com/" + strings.Repeat("a", 2048-len("https://passport.bilibili.com/")), qrBool(true), qrBool(true), qrBool(true), qrBool(false), qrBool(false), true},
		{"unicode", "https://passport.bilibili.com/中文", qrBool(true), qrBool(true), qrBool(true), qrBool(false), qrBool(false), true},
		{"relative", "relative/" + sentinel, qrBool(true), qrBool(false), qrBool(false), qrBool(false), qrBool(false), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(map[string]string{"qrcode_key": sentinel, "url": tt.value})
			if err != nil {
				t.Fatal(err)
			}
			r := qrFixture(t, `{"code":0,"data":`+string(data)+`}`)
			if r.Observation.Status != "pass" || r.Observation.Category != "observed-format" {
				t.Fatal("format observation confused with acceptance")
			}
			want := &QRDiagnosticFacts{DataKind: "object", KeyKind: "string", URLKind: "string", KeyBytes: qrInt(len(sentinel)), URLBytes: qrInt(len(tt.value)), URLParsed: tt.parsed, HTTPS: tt.https, PassportHostMatch: tt.passport, AccountHostMatch: tt.account, UserInfoPresent: tt.user, PriorValidatorAccepts: tt.accept}
			if !reflect.DeepEqual(r.Facts, want) {
				t.Fatalf("unexpected safe URL facts: %+v", r.Facts)
			}
			_, prior := decodeQR(data, "", false)
			if r.Facts.PriorValidatorAccepts != (prior == "") {
				t.Fatal("unchanged validator parity lost")
			}
		})
	}
	for _, n := range []int{0, 256, 257} {
		t.Run("key-size", func(t *testing.T) {
			data, _ := json.Marshal(map[string]string{"qrcode_key": strings.Repeat("k", n), "url": old})
			r := qrFixture(t, `{"code":0,"data":`+string(data)+`}`)
			if r.Observation.Status != "pass" || r.Facts.KeyBytes == nil || *r.Facts.KeyBytes != n || r.Facts.PriorValidatorAccepts != (n == 256) {
				t.Fatal("key-size fact or old bound changed")
			}
		})
	}
	// Raw duplicate-field semantics matter: old struct decoding rejects the first
	// wrong-type field even when a later value is valid. Do not rebuild its input.
	data := `{"qrcode_key":7,"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":"` + old + `"}`
	r := qrFixture(t, `{"code":0,"data":`+data+`}`)
	if r.Observation.Status != "pass" || r.Facts.KeyKind != "string" || r.Facts.PriorValidatorAccepts {
		t.Fatal("raw-data validator parity lost for duplicate fields")
	}
}

func TestQRDiagnosticRejectedEnvelopesHaveNoFacts(t *testing.T) {
	for _, body := range []string{"", sentinel, "null", "[]", `"PRIVATE_SYNTHETIC_SENTINEL"`, "false", "0", "{}", `{"code":null}`, `{"code":"0"}`, `{"code":0.0}`, `{"code":9223372036854775808}`, `{"code":0} trailing`} {
		r := qrFixture(t, body)
		if r.Observation.Status != "fail" || r.Observation.Category != "invalid-response" || r.Facts != nil || r.Observation.ServiceCode != nil {
			t.Fatal("invalid envelope acquired diagnostic facts")
		}
	}
	r := qrFixture(t, `{"code":-412,"message":"PRIVATE_SYNTHETIC_SENTINEL","data":{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL"}}`)
	if r.Observation.Status != "fail" || r.Observation.Category != "service-rejected" || r.Facts != nil || r.Observation.ServiceCode == nil || *r.Observation.ServiceCode != -412 {
		t.Fatal("denied service acquired diagnostic facts")
	}
}

func TestQRDiagnosticTransportFailuresAndPrivacy(t *testing.T) {
	for _, tt := range []struct {
		name     string
		status   int
		body     []byte
		err      error
		category string
	}{
		{"redirect", 302, nil, nil, "redirect"},
		{"denied", 403, []byte(sentinel), nil, "http-status"},
		{"transport", 0, nil, errors.New("https://private.invalid/?key=" + sentinel), "transport"},
		{"body-limit", 200, bytes.Repeat([]byte(" "), jsonLimit+1), nil, "body-limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := &qrTransport{t: t, status: tt.status, body: tt.body, err: tt.err}
			r := RunQRDiagnostic(context.Background(), q)
			q.verify()
			qrPrivate(t, r)
			if r.Observation.Status != "fail" || r.Observation.Category != tt.category || r.Facts != nil {
				t.Fatal("transport failure became format observation")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q := &qrTransport{t: t}
	r := RunQRDiagnostic(ctx, q)
	if q.calls != 0 || !q.idle || r.Observation.Category != "canceled" || r.Facts != nil {
		t.Fatal("canceled diagnostic requested or acquired facts")
	}
}

func TestQRDiagnosticFactsCannotSerializePrivateStrings(t *testing.T) {
	body := `{"code":0,"message":"PRIVATE_SYNTHETIC_SENTINEL","data":{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":"https://account.bilibili.com/PRIVATE_SYNTHETIC_SENTINEL?token=PRIVATE_SYNTHETIC_SENTINEL","PRIVATE_SYNTHETIC_SENTINEL":"PRIVATE_SYNTHETIC_SENTINEL"}}`
	r := qrFixture(t, body)
	qrPrivate(t, r)
	typ := reflect.TypeOf(QRDiagnosticFacts{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() == reflect.String && field.Name != "DataKind" && field.Name != "KeyKind" && field.Name != "URLKind" {
			t.Fatal("facts schema gained arbitrary string storage")
		}
	}
	b, err := json.Marshal(qrFixture(t, `{"code":0,"data":{"qrcode_key":"","url":""}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"key_bytes":0`)) || !bytes.Contains(b, []byte(`"url_bytes":0`)) || bytes.Contains(b, []byte(`"url_parsed"`)) {
		t.Fatal("zero length and unassessed parse were conflated")
	}
}
