package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"my-bilibili/experiments/playback/contract"
)

// Preview is explicitly selected; it never substitutes for the full M1 suite.
func (c *client) preview(ctx context.Context, dir string, r *report) error {
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	warning := false
	for _, n := range nodes {
		if strings.Contains(n.Label, "TEMPORARY MYGO PLAYER") && strings.Contains(n.Label, "slow loading may freeze the UI") && strings.Contains(n.Label, "headers and separate audio unsupported") && !slices.Contains(n.States, "offscreen") && n.Bounds.W > 0 && n.Bounds.H > 0 && n.Bounds.Y >= 0 && n.Bounds.Y < 840 {
			warning = true
		}
	}
	if !warning {
		return errors.New("visible temporary-player warning not observed")
	}
	r.Checks = append(r.Checks, check{Name: "provisional-warning", Result: "pass", Detail: "visible warning identifies UI stall and unsupported headers/separate audio"})
	initial, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	if initial.WindowHidden {
		r.RuntimeBlocked = true
		r.Checks = append(r.Checks, check{Name: "preview-native-playback", Result: "not-tested", Detail: "WindowHidden=true; exposed native picture/control runtime required"})
		return nil // Shared runner still quits and checks owned cleanup, then returns errOccluded.
	}
	s, err := c.ready(ctx, "local")
	if err != nil {
		return fmt.Errorf("preview local ready: %w", err)
	}
	if err = c.picture(ctx, dir, "preview-local.png"); err != nil {
		return err
	}
	record(r, "preview-local-ready-render", "pass", "positive native duration and synthetic green/red/blue picture", s)
	s, err = c.progress(ctx)
	if err != nil {
		return fmt.Errorf("preview progress: %w", err)
	}
	record(r, "preview-local-playhead", "pass", "native position advances at least300ms within2s", s)
	generation := s.Generation
	if err = c.act(ctx, "pause"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool { return s.Generation == generation && s.State == contract.Paused }); err != nil {
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
	if a.Generation != generation || s.Generation != generation || a.State != contract.Paused || s.State != contract.Paused || abs(s.Position-a.Position) > 150*time.Millisecond {
		return errors.New("preview pause drift exceeds150ms or session/state changed")
	}
	record(r, "preview-pause-stability", "pass", fmt.Sprintf("500ms drift=%s after200ms settling", s.Position-a.Position), s)
	seekKeys := []string{"Home"}
	for i := 0; i < 16; i++ {
		seekKeys = append(seekKeys, "Right")
	}
	s, err = c.sliderKeys(ctx, "seek-slider", seekKeys, 2*time.Second, func(i int, s observed) bool {
		return s.Generation == generation && s.State == contract.Paused && s.Owned && abs(s.Position-time.Duration(i)*250*time.Millisecond) <= 100*time.Millisecond && s.ControlError == contract.NoError
	})
	if err != nil {
		return fmt.Errorf("preview slider seek: %w (generation=%d state=%s position=%s control_error=%s)", err, s.Generation, s.State, s.Position, s.ControlError)
	}
	record(r, "preview-slider-seek", "pass", "native paused position4s +/-500ms within2s; interaction through seek slider", s)
	if _, err = c.sliderKeys(ctx, "volume-slider", []string{"Home", "PageUp", "PageUp", "PageUp", "PageUp", "PageUp"}, time.Second, func(i int, s observed) bool {
		return s.Generation == generation && s.Owned && math.Abs(s.Volume-float64(i)/10) < .005 && s.ControlError == contract.NoError
	}); err != nil {
		return err
	}
	for _, preset := range []struct {
		key  string
		rate float64
	}{{"speed-half", .5}, {"speed-normal", 1}, {"speed-double", 2}} {
		if err = c.act(ctx, preset.key); err != nil {
			return err
		}
		s, err = c.until(ctx, time.Second, func(s observed) bool {
			return s.Generation == generation && s.Owned && math.Abs(s.Volume-.5) < .005 && s.Rate == preset.rate && s.ControlError == contract.NoError
		})
		if err != nil {
			return fmt.Errorf("preview volume/preset configuration: %w", err)
		}
		record(r, "preview-"+preset.key+"-configuration", "pass", "native volume .5 and rate configuration readback only, not audible output or speed measurement", s)
	}
	if err = c.act(ctx, "close"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool {
		return s.Generation == generation && s.State == contract.Closed && !s.Owned && s.Error == contract.NoError
	}); err != nil {
		return err
	}
	if err = c.act(ctx, "play"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool {
		return s.Generation == generation && s.ControlError == contract.IsClosed && !s.Owned
	}); err != nil {
		return err
	}
	s, err = c.ready(ctx, "retry")
	if err != nil {
		return fmt.Errorf("preview explicit retry: %w", err)
	}
	if s.Generation <= generation || s.ControlError != contract.NoError {
		return errors.New("preview retry did not replace generation and clear error")
	}
	record(r, "preview-close-retry", "pass", "closed input explicitly reopened with newer generation and cleared error; not network recovery evidence", s)
	s, err = c.progress(ctx)
	if err != nil {
		return err
	}
	record(r, "preview-active-before-quit", "pass", "owned playing session before graceful window close", s)
	r.Checks = append(r.Checks, check{Name: "preview-scope", Result: "pass", Detail: "bounded fixture controls only; full smoke/M1 not passed by this suite"}, check{Name: "physical-audio-rate-sync-IME-Windows-target-packaging-CI", Result: "not-tested", Detail: "requires separate runtime/target/human/release evidence"})
	return nil // Leave a playing owned session for the runner's graceful Quit.
}

func (c *client) sliderKeys(ctx context.Context, key string, keys []string, limit time.Duration, predicate func(int, observed) bool) (s observed, err error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if len(keys) == 0 || len(keys) > 20 {
		return s, errors.New("invalid slider key sequence")
	}
	if err = c.call(ctx, "act", map[string]any{"window": c.window, "key": key, "action": "focus"}, nil); err != nil {
		return s, err
	}
	// setValue is text-only at this pin. Observe each keyboard change before
	// the next relative input, under one deadline, rather than racing samples
	// with a batch of keys or relying on a synthetic held mouse position.
	for i, k := range keys {
		if err = c.call(ctx, "input", map[string]any{"window": c.window, "events": []map[string]string{{"type": "key", "key": k}}}, nil); err != nil {
			return s, err
		}
		s, err = c.until(ctx, limit, func(s observed) bool { return predicate(i, s) })
		if err != nil {
			return s, err
		}
	}
	return s, nil
}
