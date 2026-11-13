package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"my-bilibili/experiments/playback/contract"
)

func record(r *report, name, result, detail string, s observed) {
	r.Checks = append(r.Checks, check{Name: name, Result: result, Detail: detail, Snapshot: &s})
}

func rendered(pngBytes []byte) bool {
	config, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil || config.Width > 4096 || config.Height > 4096 || config.Width < 320 || config.Height < 180 {
		return false
	}
	im, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return false
	}
	green, red, blue := 0, 0, 0
	for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			r >>= 8
			g >>= 8
			b >>= 8
			switch {
			case g > 130 && r < 100 && b < 150:
				green++
			case r > 160 && g < 100 && b < 100:
				red++
			case b > 140 && r < 120 && g < 150:
				blue++
			}
		}
	}
	return green > 10000 && red > 1000 && blue > 1000
}

func (c *client) picture(ctx context.Context, dir, name string) error {
	var capture struct {
		PNG []byte `json:"png"`
	}
	if err := c.call(ctx, "capture", map[string]any{"window": c.window}, &capture); err != nil {
		return err
	}
	if len(capture.PNG) > 4<<20 {
		return errors.New("owned window capture exceeds bound")
	}
	if os.WriteFile(filepath.Join(dir, name), capture.PNG, 0600) != nil {
		return errors.New("cannot save rendered fixture capture")
	}
	if !rendered(capture.PNG) {
		return errors.New("synthetic green/red/blue picture not rendered")
	}
	return nil
}

func (c *client) ready(ctx context.Context, key string) (observed, error) {
	s, err := c.openUntil(ctx, key, 10*time.Second, func(s observed) bool {
		return s.State == contract.Ready && s.Duration > 0 && s.Owned && s.Error == contract.NoError
	})
	if err != nil {
		return s, fmt.Errorf("%w (state=%s error=%s duration=%s owned=%t)", err, s.State, s.Error, s.Duration, s.Owned)
	}
	return s, nil
}
