package playback

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// PublicReferer and PublicUserAgent are the only metadata sent with P0 media.
// Account and browser cookies belong to the fixed API transport, never here.
const PublicReferer = "https://www.bilibili.com/"
const PublicUserAgent = "my-bilibili-p0/1"

// HTTPSPolicy explicitly opts into remote media. A nil policy at any playback
// boundary continues to validate only local and loopback fixture inputs.
type HTTPSPolicy struct{}

func (HTTPSPolicy) Validate(in Input) error {
	if in.FilePath != "" || !httpsMedia(in.URL) || in.AudioURL != "" && !httpsMedia(in.AudioURL) ||
		len(in.Headers) != 2 || in.Headers["Referer"] != PublicReferer || in.Headers["User-Agent"] != PublicUserAgent {
		return Invalid
	}
	return nil
}

func httpsMedia(raw string) bool {
	if raw == "" || len(raw) > 4096 || !clean(raw) || strings.ContainsAny(raw, " #") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Opaque != "" || !clean(u.Host) {
		return false
	}
	query, err := url.QueryUnescape(u.RawQuery)
	if err != nil || !clean(query) || !clean(u.Path) || strings.HasSuffix(u.Host, ":") {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.ParseUint(port, 10, 16)
		if err != nil || n == 0 {
			return false
		}
	}
	if strings.HasPrefix(u.Host, "[") {
		ip, err := netip.ParseAddr(u.Hostname())
		return err == nil && ip.Is6()
	}
	return !strings.Contains(u.Hostname(), ":")
}
