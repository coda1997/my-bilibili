package transport

import (
	"crypto/md5"
	"encoding/hex"
	"my-bilibili/internal/catalog"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const signingLifetime = 15 * time.Minute

// The fixed protocol permutation is documented at the pinned WBI reference
// in specs/0027-p0-real-client; keys themselves always come from current nav.
var mixinPermutation = [...]uint8{46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52}

type keyset struct {
	mixin   string
	expires time.Time
}

func (k keyset) usable(now time.Time) bool { return k.mixin != "" && now.Before(k.expires) }
func signingKeys(value envelope, now time.Time) (keyset, error) {
	fields, ok := object(value.data["wbi_img"])
	if !ok {
		return keyset{}, catalog.InvalidResponse
	}
	imgURL, imgOK := text(fields["img_url"])
	subURL, subOK := text(fields["sub_url"])
	if !imgOK || !subOK {
		return keyset{}, catalog.InvalidResponse
	}
	img, imgOK := keyName(imgURL)
	sub, subOK := keyName(subURL)
	if !imgOK || !subOK {
		return keyset{}, catalog.InvalidResponse
	}
	keys := img + sub
	var mixed [32]byte
	for i := range mixed {
		mixed[i] = keys[mixinPermutation[i]]
	}
	return keyset{mixin: string(mixed[:]), expires: now.Add(signingLifetime)}, nil
}
func keyName(value string) (string, bool) {
	if len(value) > 2048 {
		return "", false
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || u.RawPath != "" {
		return "", false
	}
	name := path.Base(u.Path)
	ext := path.Ext(name)
	if ext == "" {
		return "", false
	}
	name = strings.TrimSuffix(name, ext)
	if len(name) != 32 {
		return "", false
	}
	for _, c := range []byte(name) {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", false
		}
	}
	return name, true
}
func sign(params url.Values, mixin string, timestamp int64) string {
	names := make([]string, 0, len(params)+1)
	for name := range params {
		names = append(names, name)
	}
	names = append(names, "wts")
	sort.Strings(names)
	var query strings.Builder
	for _, name := range names {
		value := params.Get(name)
		if name == "wts" {
			value = strconv.FormatInt(timestamp, 10)
		}
		value = strings.Map(func(c rune) rune {
			if strings.ContainsRune("!'()*", c) {
				return -1
			}
			return c
		}, value)
		if query.Len() != 0 {
			query.WriteByte('&')
		}
		query.WriteString(percent(name))
		query.WriteByte('=')
		query.WriteString(percent(value))
	}
	canonical := query.String()
	digest := md5.Sum([]byte(canonical + mixin))
	return canonical + "&w_rid=" + hex.EncodeToString(digest[:])
}
func percent(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }
