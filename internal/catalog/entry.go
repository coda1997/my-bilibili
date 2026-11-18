package catalog

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func videoEntry(text string) (Entry, bool) {
	if len(text) == 12 && strings.HasPrefix(text, "BV") {
		for _, c := range []byte(text[2:]) {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
				return Entry{}, false
			}
		}
		return Entry{Kind: BVEntry, ID: text}, true
	}
	if strings.HasPrefix(text, "av") {
		value := text[2:]
		if len(value) == 0 || len(value) > 19 {
			return Entry{}, false
		}
		for _, c := range []byte(value) {
			if c < '0' || c > '9' {
				return Entry{}, false
			}
		}
		aid, err := strconv.ParseInt(value, 10, 64)
		if err != nil || aid <= 0 {
			return Entry{}, false
		}
		return Entry{Kind: AVEntry, ID: strconv.FormatInt(aid, 10)}, true
	}
	return Entry{}, false
}

// ParseEntry accepts one identity or a canonical video URL. Tracking parameters
// are discarded here and never become API query values or implicit searches.
func ParseEntry(text string) (Entry, error) {
	if len(text) > 2048 || !utf8.ValidString(text) {
		return Entry{}, Invalid
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsFunc(text, unicode.IsControl) {
		return Entry{}, Invalid
	}
	if entry, ok := videoEntry(text); ok {
		return entry, nil
	}
	u, err := url.Parse(text)
	if err != nil {
		return Entry{}, Invalid
	}
	if u.Scheme == "" && u.Host == "" {
		return Entry{}, Invalid
	}
	if u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "www.bilibili.com") {
		return Entry{}, Unsupported
	}
	if u.User != nil || u.Opaque != "" || strings.Contains(text, "#") || u.RawPath != "" ||
		strings.HasSuffix(u.Host, ":") || u.Port() != "" && u.Port() != "443" {
		return Entry{}, Invalid
	}
	path := strings.TrimSuffix(u.Path, "/")
	if !strings.HasPrefix(path, "/video/") {
		return Entry{}, Unsupported
	}
	entry, ok := videoEntry(strings.TrimPrefix(path, "/video/"))
	if !ok {
		return Entry{}, Invalid
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Entry{}, Invalid
	}
	if pages, exists := query["p"]; exists {
		if len(pages) != 1 || pages[0] == "" {
			return Entry{}, Invalid
		}
		for _, c := range []byte(pages[0]) {
			if c < '0' || c > '9' {
				return Entry{}, Invalid
			}
		}
		page, err := strconv.ParseUint(pages[0], 10, 16)
		if err != nil || page == 0 || page > 1000 {
			return Entry{}, Invalid
		}
		entry.Page = int(page)
	}
	return entry, nil
}

func ValidateQuery(text string) (string, error) {
	if !utf8.ValidString(text) {
		return "", Invalid
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 512 || utf8.RuneCountInString(text) > 128 || strings.ContainsFunc(text, unicode.IsControl) {
		return "", Invalid
	}
	return text, nil
}
