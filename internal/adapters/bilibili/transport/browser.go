package transport

import (
	"math"
	"my-bilibili/internal/catalog"
	"net/http"
	"strings"
	"time"
)

type browserCookie struct {
	name, value string
	expires     int64
}
type browserState struct{ cookies []browserCookie }

func (b browserState) usable(now time.Time) bool {
	for _, cookie := range b.cookies {
		if cookie.name == "buvid3" {
			return cookie.expires == 0 || cookie.expires > now.Unix()
		}
	}
	return false
}

// Both Secure and ordinary accepted cookies are only ever transmitted to the
// fixed HTTPS API. No browser cookie is added to protected account material.
func browserCookies(lines []string, now time.Time) (browserState, error) {
	retained := map[string]browserCookie{}
	for _, line := range lines {
		first, _, _ := strings.Cut(line, ";")
		rawName, _, hasValue := strings.Cut(first, "=")
		name := strings.TrimSpace(rawName)
		if name != "buvid3" && name != "b_nut" {
			continue
		}
		if !hasValue {
			return browserState{}, catalog.InvalidResponse
		}
		cookie, err := http.ParseSetCookie(line)
		if err != nil || cookie.Valid() != nil || len(cookie.Value) == 0 || len(cookie.Value) > 1024 || cookie.Path != "/" {
			return browserState{}, catalog.InvalidResponse
		}
		for _, c := range []byte(cookie.Domain) {
			if c > 127 {
				return browserState{}, catalog.InvalidResponse
			}
		}
		if strings.ToLower(strings.TrimPrefix(cookie.Domain, ".")) != "bilibili.com" {
			return browserState{}, catalog.InvalidResponse
		}
		if _, duplicate := retained[cookie.Name]; duplicate {
			return browserState{}, catalog.InvalidResponse
		}
		for _, attr := range cookie.Unparsed {
			key, _, _ := strings.Cut(attr, "=")
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "domain", "path", "expires", "max-age":
				return browserState{}, catalog.InvalidResponse
			}
		}
		if cookie.MaxAge < 0 || cookie.RawExpires != "" && cookie.Expires.IsZero() || !cookie.Expires.IsZero() && cookie.Expires.Unix() <= now.Unix() {
			return browserState{}, catalog.InvalidResponse
		}
		expiry := int64(0)
		if !cookie.Expires.IsZero() {
			expiry = cookie.Expires.Unix()
		}
		if cookie.MaxAge > 0 {
			if int64(cookie.MaxAge) > math.MaxInt64-now.Unix() {
				return browserState{}, catalog.InvalidResponse
			}
			expiry = now.Unix() + int64(cookie.MaxAge)
		}
		retained[cookie.Name] = browserCookie{name: cookie.Name, value: cookie.Value, expires: expiry}
	}
	if _, ok := retained["buvid3"]; !ok {
		return browserState{}, catalog.ServiceUnavailable
	}
	var result browserState
	for _, name := range []string{"buvid3", "b_nut"} {
		if value, ok := retained[name]; ok {
			result.cookies = append(result.cookies, value)
		}
	}
	return result, nil
}
