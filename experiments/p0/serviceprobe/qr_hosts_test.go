package serviceprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCurrentQRAccountHostFullSequence(t *testing.T) {
	bodies := fixtures()
	bodies[1] = bytes.Replace(bodies[1], []byte("https://passport.bilibili.com/"), []byte("https://account.bilibili.com/"), 1)
	transport := &script{t: t, bodies: bodies}
	report := Run(context.Background(), transport)
	transport.verify()
	checkPrivate(t, report)
	if !report.Passed() || transport.calls != 5 {
		t.Fatal("account-origin QR did not complete the fixed sequence and pending poll")
	}
	if report.Observations[1].Status != "pass" || report.Observations[2].PollCode == nil || *report.Observations[2].PollCode != 86101 {
		t.Fatal("account QR generation/poll outcome not preserved")
	}
}

func TestCurrentQRPassportHostFullSequence(t *testing.T) {
	transport := &script{t: t, bodies: fixtures()}
	report := Run(context.Background(), transport)
	transport.verify()
	checkPrivate(t, report)
	if !report.Passed() || transport.calls != 5 {
		t.Fatal("legacy passport-origin flow changed")
	}
}

func qrHostData(t *testing.T, key, value string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]string{"qrcode_key": key, "url": value})
	if err != nil {
		t.Fatal("synthetic fixture encode failed")
	}
	return data
}

func TestCurrentQRBothHostsKeepValidationBounds(t *testing.T) {
	for _, host := range []string{"passport.bilibili.com", "account.bilibili.com"} {
		t.Run(host, func(t *testing.T) {
			valid := "https://" + host + "/scan?private=" + sentinel
			for _, tt := range []struct {
				name, key, value string
				accept           bool
			}{
				{"exact-host", sentinel, valid, true},
				{"http", sentinel, "http://" + host + "/scan", false},
				{"userinfo", sentinel, "https://user:password@" + host + "/scan", false},
				{"explicit-port", sentinel, "https://" + host + ":443/scan", false},
				{"trailing-dot", sentinel, "https://" + host + "./scan", false},
				{"subdomain", sentinel, "https://child." + host + "/scan", false},
				{"suffix", sentinel, "https://" + host + ".unrelated.invalid/scan", false},
				{"prefix", sentinel, "https://not" + host + "/scan", false},
				{"uppercase", sentinel, "https://" + strings.ToUpper(host) + "/scan", false},
				{"malformed-escape", sentinel, "https://" + host + "/%zz", false},
				{"unrelated", sentinel, "https://example.invalid/scan", false},
				{"loopback", sentinel, "https://127.0.0.1/scan", false},
				{"empty-url", sentinel, "", false},
				{"url-limit", sentinel, "https://" + host + "/" + strings.Repeat("a", 2048-len("https://"+host+"/")), true},
				{"url-over-limit", sentinel, "https://" + host + "/" + strings.Repeat("a", 2049-len("https://"+host+"/")), false},
				{"empty-key", "", valid, false},
				{"key-limit", strings.Repeat("k", 256), valid, true},
				{"key-over-limit", strings.Repeat("k", 257), valid, false},
			} {
				t.Run(tt.name, func(t *testing.T) {
					data := qrHostData(t, tt.key, tt.value)
					key, category := decodeJSON(1, data, "prior-key", &Observation{})
					if tt.accept {
						if category != "" || key != tt.key {
							t.Fatal("valid exact-host boundary rejected")
						}
					} else if category != "invalid-response" || key != "prior-key" {
						t.Fatal("QR boundary or failure-key preservation changed")
					}
				})
			}
		})
	}
}

func TestCurrentQRBothHostsKeepTypesAndDuplicateSemantics(t *testing.T) {
	for _, host := range []string{"passport.bilibili.com", "account.bilibili.com"} {
		value := "https://" + host + "/scan"
		for _, data := range []string{
			"null", "[]", `"text"`, "false", "1", "{}",
			`{"qrcode_key":null,"url":"` + value + `"}`,
			`{"qrcode_key":false,"url":"` + value + `"}`,
			`{"qrcode_key":7,"url":"` + value + `"}`,
			`{"qrcode_key":{},"url":"` + value + `"}`,
			`{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":null}`,
			`{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":false}`,
			`{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":7}`,
			`{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":[]}`,
			`{"qrcode_key":7,"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":"` + value + `"}`,
			`{"qrcode_key":"PRIVATE_SYNTHETIC_SENTINEL","url":7,"url":"` + value + `"}`,
		} {
			key, category := decodeJSON(1, json.RawMessage(data), "prior-key", &Observation{})
			if category != "invalid-response" || key != "prior-key" {
				t.Fatal("invalid type/duplicate field accepted or prior key lost")
			}
		}
	}
}

func TestQRCurrentAndHistoricalPoliciesStayDistinct(t *testing.T) {
	for _, tt := range []struct {
		host        string
		priorAccept bool
	}{{"passport.bilibili.com", true}, {"account.bilibili.com", false}} {
		data := qrHostData(t, sentinel, "https://"+tt.host+"/scan?token="+sentinel)
		key, category := decodeJSON(1, data, "prior-key", &Observation{})
		if category != "" || key != sentinel {
			t.Fatal("current known-host policy failed")
		}
		historicalKey, historicalCategory := decodeQR(data, "prior-key", false)
		if (historicalCategory == "") != tt.priorAccept {
			t.Fatal("historical policy outcome changed")
		}
		if tt.priorAccept {
			if historicalKey != sentinel {
				t.Fatal("historical successful key changed")
			}
		} else if historicalKey != "prior-key" {
			t.Fatal("historical failure key changed")
		}
		diagnostic := qrFixture(t, `{"code":0,"data":`+string(data)+`}`)
		if diagnostic.Observation.Status != "pass" || diagnostic.Facts == nil || diagnostic.Facts.PriorValidatorAccepts != tt.priorAccept {
			t.Fatal("diagnostic no longer describes the original passport-only validator")
		}
	}
}
