package sessiondata

import (
	"encoding/json"
	"fmt"
	domain "my-bilibili/internal/account"
	"net/http"
	"strings"
	"testing"
	"time"
)

const synthetic = "SYNTHETIC_SESSION%2F%3D"
const original = `{"version":1,"scope":"bilibili.com/","cookies":[{"name":"SESSDATA","value":"SYNTHETIC_SESSION%2F%3D","expires":0},{"name":"bili_jct","value":"SYNTHETIC_CSRF","expires":0}]}`

func TestVersionOneExactBytesAndValues(t *testing.T) {
	now := time.Unix(1700000000, 0)
	material, err := Parse([]string{"bili_jct=SYNTHETIC_CSRF; Domain=.bilibili.com; Path=/; Secure", "SESSDATA=" + synthetic + "; Domain=.bilibili.com; Path=/; HttpOnly"}, now)
	if err != nil || string(material.Bytes()) != original {
		t.Fatal("protected version-1 encoding changed")
	}
	decoded, err := Decode(domain.NewMaterial([]byte(original)), now)
	if err != nil {
		t.Fatal("original synthetic bytes rejected")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.bilibili.com/x/web-interface/nav", nil)
	if decoded.Apply(req, now) != nil || req.Header.Get("Cookie") != "SESSDATA="+synthetic+"; bili_jct=SYNTHETIC_CSRF" {
		t.Fatal("stored cookie values or order changed")
	}
	for _, value := range []any{decoded, *decoded} {
		if fmt.Sprintf("%#v", value) != "[private web session]" {
			t.Fatal("decoded handle exposes fields")
		}
		data, err := json.Marshal(value)
		if err != nil || string(data) != "null" {
			t.Fatal("decoded credential serialized")
		}
	}
}

func TestApplyRejectsCredentialDestinationsBeforeMutation(t *testing.T) {
	now := time.Unix(1700000000, 0)
	decoded, err := Decode(domain.NewMaterial([]byte(original)), now)
	if err != nil {
		t.Fatal("fixture decode failed")
	}
	for _, endpoint := range []string{"http://api.bilibili.com/", "https://www.bilibili.com/", "https://passport.bilibili.com/", "https://api.bilibili.com.invalid/", "https://api.bilibili.com:443/", "https://user:pass@api.bilibili.com/", "https://media.invalid/"} {
		req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
		if decoded.Apply(req, now) != domain.Invalid || req.Header.Get("Cookie") != "" {
			t.Fatal("credential destination guard failed")
		}
	}
	if decoded.Apply(nil, now) != domain.Invalid || decoded.Apply(&http.Request{}, now) != domain.Invalid {
		t.Fatal("missing destination accepted")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.bilibili.com/", nil)
	if new(Session).Apply(req, now) != domain.Invalid || (*Session)(nil).Apply(req, now) != domain.Invalid {
		t.Fatal("empty credential became successful application")
	}
}

func TestApplyRechecksExpiryAndDoesNotPartlyAttach(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := strings.Replace(original, `"value":"SYNTHETIC_CSRF","expires":0`, `"value":"SYNTHETIC_CSRF","expires":1700000001`, 1)
	decoded, err := Decode(domain.NewMaterial([]byte(body)), now)
	if err != nil {
		t.Fatal("valid expiring fixture rejected")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.bilibili.com/", nil)
	if decoded.Apply(req, now.Add(time.Second)) != domain.ExpiredSession || req.Header.Get("Cookie") != "" {
		t.Fatal("expired lease attached partial credentials")
	}
}
