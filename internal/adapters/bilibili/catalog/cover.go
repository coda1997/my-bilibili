package catalog

import (
	"bytes"
	"compress/gzip"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	domain "my-bilibili/internal/catalog"
)

func validCoverURL(raw string) bool {
	clean := func(value string) bool {
		return utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
	}
	if raw == "" || len(raw) > 4096 || !clean(raw) || strings.ContainsAny(raw, " #") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || !clean(u.Host) || !clean(u.Path) || strings.HasSuffix(u.Host, ":") {
		return false
	}
	query, err := url.QueryUnescape(u.RawQuery)
	if err != nil || !clean(query) {
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

func (c *Client) Cover(parent context.Context, id string) (domain.Cover, error) {
	if err := c.check(parent); err != nil {
		return domain.Cover{}, err
	}
	ctx, finish := c.operation(parent)
	defer finish()
	select {
	case c.coverSlot <- struct{}{}:
	case <-ctx.Done():
		return domain.Cover{}, contextCode(ctx.Err())
	}
	defer func() { <-c.coverSlot }()
	if err := c.check(ctx); err != nil {
		return domain.Cover{}, err
	}
	c.mu.Lock()
	location, exists := c.urls[id]
	c.mu.Unlock()
	if !exists {
		return domain.Cover{}, domain.Unavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return domain.Cover{}, domain.Invalid
	}
	req.Header.Set("Referer", publicReferer)
	req.Header.Set("User-Agent", publicUserAgent)
	response, err := c.coverTransport.RoundTrip(req)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		if failure := c.check(ctx); failure != nil {
			return domain.Cover{}, failure
		}
		return domain.Cover{}, domain.Network
	}
	if response == nil || response.Body == nil {
		return domain.Cover{}, domain.InvalidResponse
	}
	body, err := coverBody(response)
	if failure := c.check(ctx); failure != nil {
		return domain.Cover{}, failure
	}
	if err != nil {
		return domain.Cover{}, err
	}
	cover, err := decodeCover(body)
	if failure := c.check(ctx); failure != nil {
		return domain.Cover{}, failure
	}
	return cover, err
}

func coverBody(response *http.Response) (body []byte, err error) {
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			body, err = nil, domain.Network
		}
	}()
	if response.StatusCode != http.StatusOK {
		switch {
		case response.StatusCode == http.StatusTooManyRequests:
			return nil, domain.RateLimited
		case response.StatusCode >= 500:
			return nil, domain.ServiceUnavailable
		case response.StatusCode == http.StatusForbidden:
			return nil, domain.Restricted
		default:
			return nil, domain.InvalidResponse
		}
	}
	var reader io.Reader = response.Body
	switch strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip":
		decoded, openErr := gzip.NewReader(response.Body)
		if openErr != nil {
			return nil, domain.InvalidResponse
		}
		defer decoded.Close()
		reader = decoded
	default:
		return nil, domain.InvalidResponse
	}
	body, readErr := io.ReadAll(io.LimitReader(reader, 2*1024*1024+1))
	if readErr != nil {
		return nil, domain.Network
	}
	if len(body) > 2*1024*1024 {
		return nil, domain.InvalidResponse
	}
	return body, nil
}

func decodeCover(body []byte) (domain.Cover, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 1024 || config.Height > 1024 ||
		format != "jpeg" && format != "png" && format != "gif" {
		return domain.Cover{}, domain.InvalidResponse
	}
	decoded, actual, err := image.Decode(bytes.NewReader(body))
	if err != nil || actual != format {
		return domain.Cover{}, domain.InvalidResponse
	}
	bounds := decoded.Bounds()
	// A GIF's first image can occupy only part of its declared logical screen.
	// Decode and normalize that image, retaining the pre-decode canvas bound.
	width, height := bounds.Dx(), bounds.Dy()
	if width < 1 || height < 1 || width > config.Width || height > config.Height {
		return domain.Cover{}, domain.InvalidResponse
	}
	w, h := width, height
	if w > 320 {
		h = max(1, h*320/w)
		w = 320
	}
	if h > 180 {
		w = max(1, w*180/h)
		h = 180
	}
	cover := domain.Cover{Width: w, Height: h, RGBA: make([]byte, w*h*4)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := decoded.At(bounds.Min.X+x*width/w, bounds.Min.Y+y*height/h).RGBA()
			i := (y*w + x) * 4
			cover.RGBA[i], cover.RGBA[i+1], cover.RGBA[i+2], cover.RGBA[i+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
		}
	}
	return cover, nil
}
