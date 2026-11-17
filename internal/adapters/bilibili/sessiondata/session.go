// Package sessiondata owns the private version-1 web-session encoding.
package sessiondata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	domain "my-bilibili/internal/account"
	"net/http"
	"strings"
	"time"
)

const materialLimit = 2560

var names = []string{"SESSDATA", "bili_jct", "DedeUserID", "DedeUserID__ckMd5"}

// Session is an immutable decoded credential. Only Apply can consume its values.
type Session struct{ cookies []cookieRecord }

func (Session) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private web session]") }
func (Session) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// Apply only attaches a valid credential to the fixed HTTPS service origin.
func (s *Session) Apply(req *http.Request, now time.Time) error {
	if s == nil || len(s.cookies) == 0 || req == nil || req.URL == nil || req.URL.Scheme != "https" || req.URL.Host != "api.bilibili.com" || req.URL.User != nil || req.URL.Opaque != "" {
		return domain.Invalid
	}
	for _, cookie := range s.cookies {
		if *cookie.Expires > 0 && *cookie.Expires <= now.Unix() {
			return domain.ExpiredSession
		}
	}
	for _, cookie := range s.cookies {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return nil
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
func Parse(lines []string, now time.Time) (domain.Material, error) {
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
		now := now.Unix()
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
func Decode(material domain.Material, now time.Time) (*Session, error) {
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
		if *record.Expires > 0 && *record.Expires <= now.Unix() {
			return nil, domain.ExpiredSession
		}
		prior = index
	}
	return &Session{cookies: value.Cookies}, nil
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
