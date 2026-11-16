package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Analyze only the actual video surface interior, excluding the cover, metadata
// and control overlay. The original fixture contains broad red/green/blue bands;
// black video or colorful page furniture cannot satisfy this check.
func pictureDigest(data []byte, video rectangle, content rectangle) (string, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || len(data) > 4<<20 || cfg.Width < 320 || cfg.Height < 180 || cfg.Width > 8192 || cfg.Height > 8192 || content.Width < 1 || content.Height < 1 || video.Width < 160 || video.Height < 90 {
		return "", errors.New("invalid native capture geometry")
	}
	sx, sy := float64(cfg.Width)/content.Width, float64(cfg.Height)/content.Height
	if math.Abs(sx-sy) > .05 {
		return "", errors.New("native capture scale mismatch")
	}
	bounds := image.Rect(int((video.X+video.Width*.1)*sx), int((video.Y+video.Height*.1)*sy), int((video.X+video.Width*.9)*sx), int((video.Y+video.Height*.65)*sy))
	if !bounds.In(image.Rect(0, 0, cfg.Width, cfg.Height)) || bounds.Empty() {
		return "", errors.New("video surface outside native capture")
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errors.New("native capture decode failed")
	}
	red, green, blue := 0, 0, 0
	h := sha256.New()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			_, _ = h.Write([]byte{byte(r), byte(g), byte(b)})
			switch {
			case r > 150 && g < 110 && b < 110:
				red++
			case g > 110 && r < 110 && b < 140:
				green++
			case b > 130 && r < 130 && g < 140:
				blue++
			}
		}
	}
	minimum := bounds.Dx() * bounds.Dy() / 100
	if red < minimum || green < minimum || blue < minimum {
		return "", errors.New("synthetic video bands absent from native frame")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (c *client) picture(ctx context.Context, dir, name string) (string, error) {
	w, err := c.identify(ctx)
	if err != nil {
		return "", err
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return "", err
	}
	v, err := uniqueNode(nodes, "preview-video")
	if err != nil {
		return "", err
	}
	return c.capturePicture(ctx, dir, name, v.Bounds, w.ContentBounds)
}

func (c *client) capturePicture(ctx context.Context, dir, name string, video, content rectangle) (string, error) {
	var capture struct {
		PNG []byte `json:"png"`
	}
	if err := c.call(ctx, "capture", map[string]any{"window": c.window}, &capture); err != nil {
		return "", err
	}
	if len(capture.PNG) > 4<<20 {
		return "", errors.New("native capture exceeds bound")
	}
	if err := os.WriteFile(filepath.Join(dir, name), capture.PNG, 0600); err != nil {
		return "", errors.New("native capture could not be saved")
	}
	return pictureDigest(capture.PNG, video, content)
}
