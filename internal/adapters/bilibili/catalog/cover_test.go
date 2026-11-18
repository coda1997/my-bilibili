package catalog

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "my-bilibili/internal/catalog"
)

func syntheticImage(t *testing.T, format string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 180, G: 70, B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg":
		err = jpeg.Encode(&buf, img, nil)
	case "gif":
		err = gif.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
func withCover(t *testing.T, rt http.RoundTripper) (*Client, string) {
	t.Helper()
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(detailData())), nil }, rt)
	v, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
	if err != nil {
		t.Fatal(err)
	}
	return c, v.CoverID
}

func TestCoverNormalizationAndCookieFreeRequest(t *testing.T) {
	for _, format := range []string{"png", "jpeg", "gif"} {
		t.Run(format, func(t *testing.T) {
			body := syntheticImage(t, format, 800, 400)
			var calls atomic.Int32
			c, id := withCover(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				if req.Method != "GET" || req.URL.String() != "https://cover.example.test/one.png" || req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" || req.Header.Get("Referer") != publicReferer || req.Header.Get("User-Agent") != publicUserAgent {
					t.Error("cover request leaked/changed metadata")
				}
				return response(body), nil
			}))
			cover, err := c.Cover(context.Background(), id)
			if err != nil || cover.Width != 320 || cover.Height != 160 || len(cover.RGBA) != 320*160*4 || calls.Load() != 1 {
				t.Fatal("cover normalization", cover.Width, cover.Height, err)
			}
			cover.RGBA[0] = 0
			next, err := c.Cover(context.Background(), id)
			if err != nil || next.RGBA[0] == 0 {
				t.Fatal("returned pixels aliased next image")
			}
		})
	}
	cover, err := decodeCover(syntheticImage(t, "png", 400, 800))
	if err != nil || cover.Width != 90 || cover.Height != 180 {
		t.Fatal("portrait aspect/bounds")
	}
}

func TestBadCoverIsOnlyPlaceholderAndRegistryIsBounded(t *testing.T) {
	for _, location := range []any{"", nil, 42, "http://cover.example.test/one.png", "//cover.example.test/one.png", "https://user@cover.test/a", "https://cover.test/a#x", "https://cover.test:0/a", "https://[bad]/a", "https://cover.test/a%0a", "https://cover.test/a?q=%zz"} {
		data := detailData()
		data["pic"] = location
		c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(data)), nil }, nil)
		video, err := c.Detail(context.Background(), domain.Entry{Kind: domain.BVEntry, ID: "BV0000000001"})
		if err != nil || video.CoverID != "" || len(video.Parts) != 2 {
			t.Fatal("bad cover damaged detail", err)
		}
	}
	c, _ := testClient(t, func(*http.Request) (*http.Response, error) { return response(envelope(detailData())), nil }, nil)
	first := c.rememberCover("https://covers.test/first")
	if first == "" || c.rememberCover("https://covers.test/first") != first {
		t.Fatal("stable cover handle")
	}
	for i := 0; i < 1001; i++ {
		if c.rememberCover(fmt.Sprintf("https://covers.test/%d", i)) == "" {
			t.Fatal("valid cover rejected")
		}
	}
	if len(c.urls) != 1001 || len(c.order) != 1001 || c.urls[first] != "" {
		t.Fatal("private URL registry exceeded bound")
	}
	c.Close()
	if len(c.urls) != 0 || c.rememberCover("https://covers.test/late") != "" {
		t.Fatal("closed registry retained/accepted locations")
	}
	if _, err := c.Cover(context.Background(), first); err != domain.Closed {
		t.Fatal("closed cover fetched", err)
	}
}

type checkedBody struct {
	reader io.Reader
	closed int
	err    error
}

func (b *checkedBody) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *checkedBody) Close() error               { b.closed++; return b.err }

func TestCoverBytesDimensionsEncodingAndResponseCleanup(t *testing.T) {
	good := syntheticImage(t, "png", 32, 18)
	large := append(append([]byte{}, good...), bytes.Repeat([]byte{0}, 2*1024*1024)...)
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	_, _ = z.Write(large)
	_ = z.Close()
	cases := []struct {
		name     string
		body     []byte
		encoding string
		status   int
		closeErr error
		want     domain.Code
	}{
		{"large-decompressed", compressed.Bytes(), "gzip", 200, nil, domain.InvalidResponse},
		{"large", large, "", 200, nil, domain.InvalidResponse},
		{"dimensions", syntheticImage(t, "png", 1025, 2), "", 200, nil, domain.InvalidResponse},
		{"unknown", []byte("not an image"), "", 200, nil, domain.InvalidResponse},
		{"encoding", good, "unknown", 200, nil, domain.InvalidResponse},
		{"redirect", good, "", 302, nil, domain.InvalidResponse},
		{"denied", good, "", 403, nil, domain.Restricted},
		{"rate", good, "", 429, nil, domain.RateLimited},
		{"server", good, "", 503, nil, domain.ServiceUnavailable},
		{"close-error", good, "", 200, errors.New("PRIVATE_SYNTHETIC"), domain.Network},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := &checkedBody{reader: bytes.NewReader(test.body), err: test.closeErr}
			c, id := withCover(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Header: http.Header{"Content-Encoding": {test.encoding}}, Body: body}, nil
			}))
			cover, err := c.Cover(context.Background(), id)
			if err != test.want || len(cover.RGBA) != 0 || body.closed != 1 {
				t.Fatal("invalid cover or cleanup outcome", err, body.closed)
			}
		})
	}
	valid, err := decodeCover(syntheticImage(t, "png", 1024, 1024))
	if err != nil || len(valid.RGBA) > 320*180*4 {
		t.Fatal("allowed dimension boundary rejected")
	}
}

func TestCoverSerialQueueCancellationAndLateClose(t *testing.T) {
	body := syntheticImage(t, "png", 32, 18)
	entered, release := make(chan struct{}), make(chan struct{})
	unlock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unlock)
	var calls atomic.Int32
	c, id := withCover(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) > 1 {
			return response(body), nil
		}
		close(entered)
		<-release
		return response(body), nil
	}))
	first := make(chan error, 1)
	go func() { _, err := c.Cover(context.Background(), id); first <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if _, err := c.Cover(ctx, id); err != domain.Timeout {
		t.Fatal("queued cover ignored earlier deadline", err)
	}
	if calls.Load() != 1 || len(c.coverSlot) != 1 {
		t.Fatal("concurrent cover requests")
	}
	c.Close()
	unlock()
	if err := <-first; err != domain.Closed {
		t.Fatal("late cover published after close", err)
	}
	if len(c.coverSlot) != 0 || len(c.urls) != 0 {
		t.Fatal("cover ownership retained")
	}
}

func TestGIFFirstImageMayBeSmallerThanLogicalScreen(t *testing.T) {
	palette := color.Palette{color.RGBA{R: 190, A: 255}}
	frame := image.NewPaletted(image.Rect(8, 8, 40, 26), palette)
	var body bytes.Buffer
	err := gif.EncodeAll(&body, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}, Config: image.Config{ColorModel: palette, Width: 64, Height: 36}})
	if err != nil {
		t.Fatal(err)
	}
	cover, err := decodeCover(body.Bytes())
	if err != nil || cover.Width != 32 || cover.Height != 18 || len(cover.RGBA) != 32*18*4 {
		t.Fatal("valid first GIF image rejected", err, cover.Width, cover.Height)
	}
}

func TestCoverURLSyntaxAndBounds(t *testing.T) {
	for _, raw := range []string{"https://new.example.test/a", "https://127.0.0.1:65535/a", "https://[::1]:443/a", "https://host.test/a%20b", "HTTPS://host.test/a"} {
		if !validCoverURL(raw) {
			t.Fatal("valid cover URL", raw)
		}
	}
	for _, raw := range []string{"https:opaque", "https:///a", "https://host.test:/a", "https://host.test:65536/a", "https://host.test/a\n", "https://host.test/a?x=%0a", "https://host.test/a%zz", "https://host.test/a#", "https://host.test/" + strings.Repeat("x", 4097)} {
		if validCoverURL(raw) {
			t.Fatal("bad cover URL", raw)
		}
	}
}

func TestCoverConfigAndFormatAreCheckedBeforeFullDecode(t *testing.T) {
	for _, test := range []struct {
		name, magic string
		width       int
	}{
		{"unsupported-fixture", "SYNTHETIC_UNSUPPORTED", 32},
		{"png", "SYNTHETIC_LARGE_CONFIG", 2048},
	} {
		var decoded atomic.Int32
		image.RegisterFormat(test.name, test.magic, func(io.Reader) (image.Image, error) {
			decoded.Add(1)
			return image.NewRGBA(image.Rect(0, 0, 32, 18)), nil
		},
			func(io.Reader) (image.Config, error) {
				return image.Config{ColorModel: color.RGBAModel, Width: test.width, Height: 18}, nil
			})
		if _, err := decodeCover([]byte(test.magic)); err != domain.InvalidResponse || decoded.Load() != 0 {
			t.Fatal("full decoder called before safe config/format acceptance", test.name, err, decoded.Load())
		}
	}
}
