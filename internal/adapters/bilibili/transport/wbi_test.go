package transport

import (
	"my-bilibili/internal/catalog"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Expected bytes were derived before this Go implementation with an independent
// standard-library encoder/MD5 and original synthetic inputs. No live keys or
// SDK/example request is copied into these fixtures.
func TestWBIIndependentExpectedVectors(t *testing.T) {
	value, err := decodeEnvelope([]byte(anonymousNav))
	if err != nil {
		t.Fatal("synthetic envelope invalid")
	}
	now := time.Unix(1700000000, 0)
	keys, err := signingKeys(value, now)
	if err != nil || keys.mixin != "7722a870f9afd361b5580953dce3c34d" {
		t.Fatal("documented mixin permutation changed")
	}
	cases := []struct {
		params        url.Values
		timestamp     int64
		query, digest string
	}{
		{url.Values{"zebra": {"last"}, "alpha": {"first"}, "foo": {"bar"}}, 1700000000, "alpha=first&foo=bar&wts=1700000000&zebra=last", "b07eff80492d74049d6fb67035cfd4fd"},
		{url.Values{"foo": {"bar"}, "alpha": {"first"}, "zebra": {"last"}}, 1700000000, "alpha=first&foo=bar&wts=1700000000&zebra=last", "b07eff80492d74049d6fb67035cfd4fd"},
		{searchParamsWithConstants(), 1700000000, "keyword=%E4%B8%AD%E6%96%87%20%E6%90%9C%E7%B4%A2&order=totalrank&page=1&search_type=video&wts=1700000000", "7761b4de87196cd2f0620cfbef61d849"},
		{url.Values{"keyword": {"a!b'c(d)* e%f/g?=+&~"}, "page": {"2"}}, 1700000000, "keyword=abcd%20e%25f%2Fg%3F%3D%2B%26~&page=2&wts=1700000000", "7b04a84d1b400a3b67de43dbb87306ef"},
		{url.Values{"zebra": {"last"}, "alpha": {"first"}, "foo": {"bar"}}, 1700000001, "alpha=first&foo=bar&wts=1700000001&zebra=last", "71501376fe6d274ac302ed7a394d1ec7"},
	}
	for _, item := range cases {
		if sign(item.params, keys.mixin, item.timestamp) != item.query+"&w_rid="+item.digest {
			t.Fatal("WBI canonical query or independent digest changed")
		}
		if item.params.Has("wts") || item.params.Has("w_rid") {
			t.Fatal("signing mutated caller values")
		}
	}
	if !keys.usable(now.Add(15*time.Minute-time.Nanosecond)) || keys.usable(now.Add(15*time.Minute)) {
		t.Fatal("WBI reuse exceeded client TTL")
	}
}
func searchParamsWithConstants() url.Values {
	return url.Values{"keyword": {"中文 搜索"}, "page": {"1"}, "search_type": {"video"}, "order": {"totalrank"}}
}

func TestWBIRequiresValidatedCurrentNavKeys(t *testing.T) {
	for _, bad := range []string{"", "null", `{"code":0,"data":{"isLogin":true}}`, strings.Replace(signedNav, imgKey, "short", 1), strings.Replace(signedNav, imgKey, strings.Repeat("z", 32), 1), strings.Replace(signedNav, "https://wbi.invalid/", "http://wbi.invalid/", 1), strings.Replace(signedNav, "https://wbi.invalid/", "https://user:pass@wbi.invalid/", 1), strings.Replace(signedNav, ".png", "", 1), strings.Replace(signedNav, imgKey, "%30"+imgKey[1:], 1), strings.Replace(signedNav, imgKey, strings.Repeat("0", 2049), 1)} {
		value, err := decodeEnvelope([]byte(bad))
		if err == nil {
			_, err = signingKeys(value, time.Unix(1700000000, 0))
		}
		if err != catalog.InvalidResponse {
			t.Fatal("invalid nav acquired signing keys")
		}
	}
}

func TestBrowserCookieScopeBoundsAndExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	for _, lines := range [][]string{
		{"SESSDATA=IGNORED; Domain=bilibili.com; Path=/"},
		{"buvid3=SYNTHETIC; Path=/"}, {"buvid3=SYNTHETIC; Domain=www.bilibili.com; Path=/"}, {"buvid3=SYNTHETIC; Domain=bilibili.com; Path=/other"},
		{"buvid3=SYNTHETIC; Domain=..bilibili.com; Path=/"}, {"buvid3=SYNTHETIC; Domain=bilibili.com.; Path=/"}, {"buvid3=SYNTHETIC; Domain=bilibili.com.invalid; Path=/"},
		{"buvid3=SYNTHETIC; Domain=bilibili.com; Path=/; Max-Age=0"}, {"buvid3=SYNTHETIC; Domain=bilibili.com; Path=/; Expires=invalid"},
		{"buvid3=SYNTHETIC; Domain=bilibili.com; Path=/; Max-Age=invalid"}, {"buvid3=SYNTHETIC; Domain=bilibili.com; Path=/; Max-Age=9223372036854775807"},
		{"buvid3=; Domain=bilibili.com; Path=/"}, {"buvid3=" + strings.Repeat("a", 1025) + "; Domain=bilibili.com; Path=/"},
		{browserLine, browserLine}, {browserLine, "b_nut"}, {browserLine, "b_nut=SYNTHETIC; Domain=bilibili.com; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT"},
	} {
		state, err := browserCookies(lines, now)
		if err == nil || state.usable(now) {
			t.Fatal("invalid/missing browser cookie became usable")
		}
	}
	state, err := browserCookies([]string{"buvid3=" + strings.Repeat("a", 1024) + "; Domain=.BILIBILI.COM; Path=/; Secure; Max-Age=2", "b_nut=NUT; Domain=bilibili.com; Path=/", "unknown=IGNORED; Domain=other.invalid; Path=/"}, now)
	if err != nil || len(state.cookies) != 2 || !state.usable(now.Add(time.Second)) || state.usable(now.Add(2*time.Second)) {
		t.Fatal("browser count/value/expiry boundary wrong")
	}
	if _, err := browserCookies(nil, now); err != catalog.ServiceUnavailable {
		t.Fatal("missing required buvid3 lost explicit error")
	}
}
