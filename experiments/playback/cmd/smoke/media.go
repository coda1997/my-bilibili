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

	"my-bilibili/experiments/playback/cmd/fixturehttp"
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
	if len(capture.PNG) > 4<<20 || !rendered(capture.PNG) {
		return errors.New("synthetic green/red/blue picture not rendered")
	}
	if os.WriteFile(filepath.Join(dir, name), capture.PNG, 0600) != nil {
		return errors.New("cannot save rendered fixture capture")
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

func (c *client) progress(ctx context.Context) (observed, error) {
	start, err := c.snapshot(ctx)
	if err != nil {
		return start, err
	}
	if err = c.act(ctx, "play"); err != nil {
		return start, err
	}
	return c.until(ctx, 2*time.Second, func(s observed) bool {
		return s.Generation == start.Generation && s.State == contract.Playing && s.Position-start.Position >= 300*time.Millisecond
	})
}

func (c *client) media(ctx context.Context, dir string, r *report, srv *fixturehttp.Server) error {
	initial, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	if initial.WindowHidden {
		r.RuntimeBlocked = true
		return c.hiddenMedia(ctx, r, srv)
	}
	s, err := c.ready(ctx, "local")
	if err != nil {
		return fmt.Errorf("local ready: %w", err)
	}
	if err = c.picture(ctx, dir, "local.png"); err != nil {
		return err
	}
	record(r, "local-ready-render", "pass", "native positive duration and captured synthetic picture", s)
	s, err = c.progress(ctx)
	if err != nil {
		return fmt.Errorf("local progression: %w", err)
	}
	record(r, "local-playhead", "pass", "native position advances at least 300ms within 2s", s)
	if err = c.act(ctx, "pause"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool { return s.State == contract.Paused }); err != nil {
		return err
	}
	if err = wait(ctx, 200*time.Millisecond); err != nil {
		return err
	}
	a, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	if err = wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil {
		return err
	}
	if s.Generation != a.Generation || s.State != contract.Paused || abs(s.Position-a.Position) > 150*time.Millisecond {
		return errors.New("native pause drift exceeds 150ms")
	}
	record(r, "pause-stability", "pass", fmt.Sprintf("500ms sample drift=%s after 200ms settle", s.Position-a.Position), s)
	if err = c.act(ctx, "seek"); err != nil {
		return err
	}
	s, err = c.until(ctx, 2*time.Second, func(s observed) bool {
		return s.State == contract.Paused && abs(s.Position-3*time.Second) <= 500*time.Millisecond
	})
	if err != nil {
		return fmt.Errorf("native seek: %w", err)
	}
	record(r, "paused-seek", "pass", "native position reaches 3s +/-500ms within 2s", s)
	if err = c.act(ctx, "volume"); err != nil {
		return err
	}
	if err = c.act(ctx, "rate"); err != nil {
		return err
	}
	s, err = c.until(ctx, time.Second, func(s observed) bool { return s.Volume == .25 && s.Rate == 1.5 })
	if err != nil {
		return err
	}
	record(r, "volume-rate-configuration", "pass", "native public readback; not a speed or audible-output measurement", s)
	r.Checks = append(r.Checks, check{Name: "physical-audio-and-rate-speed", Result: "not-tested", Detail: "no human speaker observation or separate speed measurement"})
	previous := s.Generation
	s, err = c.ready(ctx, "http")
	if err != nil {
		return fmt.Errorf("HTTP ready: %w", err)
	}
	if s.Generation <= previous || s.Position > 500*time.Millisecond {
		return errors.New("HTTP replacement did not reset generation/position")
	}
	if err = c.picture(ctx, dir, "http.png"); err != nil {
		return err
	}
	s, err = c.progress(ctx)
	if err != nil {
		return fmt.Errorf("HTTP progression: %w", err)
	}
	if srv.Stats().Plain == 0 {
		return errors.New("native HTTP request not observed")
	}
	record(r, "HTTP-ready-render-play", "pass", "owned loopback media rendered and native position progressed", s)
	if err = c.act(ctx, "pause"); err != nil {
		return err
	}
	if err = c.act(ctx, "seek"); err != nil {
		return err
	}
	s, err = c.until(ctx, 2*time.Second, func(s observed) bool {
		return s.State == contract.Paused && abs(s.Position-3*time.Second) <= 500*time.Millisecond
	})
	if err != nil {
		return fmt.Errorf("HTTP seek: %w", err)
	}
	record(r, "HTTP-seek", "pass", "native paused HTTP seek reaches 3s +/-500ms", s)
	if srv.Stats().Range > 0 {
		record(r, "native-HTTP-Range", "pass", "Range requests observed by fixture server", s)
	} else {
		record(r, "native-HTTP-Range", "not-tested", "no native Range request observed; Go server Range unit tests are separate", s)
	}
	loading, err := c.openUntil(ctx, "delayed", 500*time.Millisecond, func(s observed) bool { return s.State == contract.Loading && s.Owned })
	if err != nil {
		return fmt.Errorf("delayed loading before switch: %w", err)
	}
	// Let the native request reach the delayed handler before replacing it.
	if err = wait(ctx, 150*time.Millisecond); err != nil {
		return err
	}
	s, err = c.ready(ctx, "local")
	if err != nil {
		return err
	}
	if s.Generation <= loading.Generation || s.Position > 500*time.Millisecond {
		return errors.New("replacement identity/reset failed")
	}
	current := s.Generation
	if err = wait(ctx, 3100*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil {
		return err
	}
	if s.Generation != current || !s.Owned || s.State != contract.Ready {
		return errors.New("obsolete delayed session restored state")
	}
	record(r, "loading-switch-generation", "pass", "replacement stays current after old delayed-response window", s)
	for _, mode := range []string{"loading", "playing"} {
		var active observed
		if mode == "loading" {
			active, err = c.openUntil(ctx, "delayed", 500*time.Millisecond, func(s observed) bool { return s.State == contract.Loading && s.Owned })
			if err != nil {
				return fmt.Errorf("delayed loading before cancellation: %w", err)
			}
		} else {
			if _, err = c.ready(ctx, "local"); err != nil {
				return err
			}
			if active, err = c.progress(ctx); err != nil {
				return err
			}
		}
		start := time.Now()
		if err = c.act(ctx, "cancel"); err != nil {
			return err
		}
		s, err = c.until(ctx, 100*time.Millisecond, func(s observed) bool {
			return s.Generation == active.Generation && s.State == contract.Closed && s.Error == contract.Canceled && !s.Owned
		})
		if err != nil {
			return fmt.Errorf("%s cancel: %w", mode, err)
		}
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			return errors.New("cancellation observation exceeds 100ms")
		}
		record(r, mode+"-cancel", "pass", fmt.Sprintf("owned engine removed; externally observed in %s", elapsed), s)
	}
	for _, key := range []string{"headers", "separate"} {
		s, err = c.openUntil(ctx, key, time.Second, func(s observed) bool {
			return s.State == contract.Failed && s.Error == contract.Unsupported && !s.Owned
		})
		if err != nil {
			return err
		}
		record(r, key+"-safe-rejection", "pass", "typed unsupported failure; no silent metadata or audio loss", s)
		record(r, key+"-playback", "fail", "pinned public player API cannot supply this capability", s)
	}
	for _, key := range []string{"protected", "failure"} {
		s, err = c.openUntil(ctx, key, 10*time.Second, func(s observed) bool { return s.State == contract.Failed && s.Error == contract.Media && !s.Owned })
		if err != nil {
			return fmt.Errorf("%s native failure: %w (generation=%d state=%s error=%s owned=%t)", key, err, s.Generation, s.State, s.Error, s.Owned)
		}
		record(r, key+"-native-error", "pass", "native HTTP failure is sanitized and engine disposed", s)
	}
	if srv.Stats().ProtectedDenied == 0 || srv.Stats().Failed == 0 {
		return errors.New("HTTP negative routes not actually requested")
	}
	if _, err = c.ready(ctx, "local"); err != nil {
		return err
	}
	if err = c.act(ctx, "close"); err != nil {
		return err
	}
	s, err = c.until(ctx, time.Second, func(s observed) bool { return s.State == contract.Closed && !s.Owned && s.Error == contract.NoError })
	if err != nil {
		return err
	}
	generation := s.Generation
	if err = c.act(ctx, "play"); err != nil {
		return err
	}
	s, err = c.until(ctx, time.Second, func(s observed) bool {
		return s.ControlError == contract.IsClosed && !s.Owned && s.Generation == generation
	})
	if err != nil {
		return err
	}
	record(r, "close-and-closed-command", "pass", "owned engine absent; later command safely rejected without generation change", s)
	// Finish with an active session: window close must dispose it, not just idle UI.
	if _, err = c.ready(ctx, "local"); err != nil {
		return err
	}
	if _, err = c.progress(ctx); err != nil {
		return err
	}
	return nil
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
