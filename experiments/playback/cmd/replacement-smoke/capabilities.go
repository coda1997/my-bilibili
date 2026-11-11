package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"my-bilibili/experiments/playback/cmd/fixturehttp"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
)

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (c *client) progress(ctx context.Context) (observed, error) {
	before, err := c.snapshot(ctx)
	if err != nil {
		return before, err
	}
	started := time.Now()
	if err = c.act(ctx, "play"); err != nil {
		return before, err
	}
	s, err := c.until(ctx, 2*time.Second, func(s observed) bool {
		return s.Generation == before.Generation && s.Owned && s.State == contract.Playing && s.Position-before.Position >= 300*time.Millisecond
	})
	if err == nil && time.Since(started) > 2*time.Second {
		err = errors.New("playhead advance exceeded 2s external deadline")
	}
	return s, err
}

func (c *client) playback(ctx context.Context, dir, key string, r *report) (observed, error) {
	s, err := c.ready(ctx, key)
	if err != nil {
		return s, fmt.Errorf("%s ready: %w", key, err)
	}
	if s.WindowHidden {
		r.RuntimeBlocked = true
		return s, errors.New("owned window occluded: playback not-tested")
	}
	if err = c.picture(ctx, dir, key+"-capabilities.png"); err != nil {
		return s, fmt.Errorf("%s picture: %w", key, err)
	}
	record(r, key+"-ready-picture", "pass", "positive native duration and synthetic picture", s)
	s, err = c.progress(ctx)
	if err != nil {
		return s, fmt.Errorf("%s progress: %w", key, err)
	}
	record(r, key+"-progress", "pass", "native position advanced at least 300ms within 2s", s)
	generation := s.Generation
	if err = c.act(ctx, "pause"); err != nil {
		return s, err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool { return s.Generation == generation && s.State == contract.Paused }); err != nil {
		return s, err
	}
	if err = wait(ctx, 200*time.Millisecond); err != nil {
		return s, err
	}
	a, err := c.snapshot(ctx)
	if err != nil {
		return s, err
	}
	if err = wait(ctx, 500*time.Millisecond); err != nil {
		return s, err
	}
	s, err = c.snapshot(ctx)
	if err != nil || a.Generation != generation || s.Generation != generation || a.State != contract.Paused || s.State != contract.Paused || abs(s.Position-a.Position) > 150*time.Millisecond {
		return s, errors.New("pause state/generation changed or drift exceeded 150ms")
	}
	record(r, key+"-pause", "pass", fmt.Sprintf("drift %s across 500ms after 200ms settling", s.Position-a.Position), s)
	start := time.Now()
	if err = c.act(ctx, "seek"); err != nil {
		return s, err
	}
	s, err = c.until(ctx, 2*time.Second, func(s observed) bool {
		return s.Generation == generation && s.Owned && s.State == contract.Paused && abs(s.Position-3*time.Second) <= 500*time.Millisecond
	})
	if err != nil || time.Since(start) > 2*time.Second {
		return s, errors.New("paused seek to 3s did not meet 2s/500ms bound")
	}
	record(r, key+"-seek", "pass", "paused native seek 3s +/-500ms within 2s", s)
	if err = c.act(ctx, "rate"); err != nil {
		return s, err
	}
	s, err = c.until(ctx, time.Second, func(s observed) bool { return s.Generation == generation && s.Rate == 1.5 && s.Owned })
	if err == nil {
		record(r, key+"-rate-configuration", "pass", "1.5x configuration only; actual speed and A/V sync unmeasured", s)
	}
	return s, err
}

func (c *client) zeroResources(ctx context.Context) error {
	owned, err := c.resources(ctx)
	if err != nil || owned != (mpvprobe.Resources{}) {
		return errors.New("native resources/workers remain after terminal state")
	}
	return nil
}

func (c *client) mediaFailure(ctx context.Context, key string, r *report) error {
	started := time.Now()
	s, err := c.openUntil(ctx, key, 10*time.Second, func(s observed) bool {
		return s.State == contract.Failed && s.Error == contract.Media && !s.Owned
	})
	if err != nil || time.Since(started) > 10*time.Second {
		return fmt.Errorf("%s did not produce disposed Media failure within 10s (state=%s error=%s)", key, s.State, s.Error)
	}
	if err = c.zeroResources(ctx); err != nil {
		return err
	}
	record(r, key+"-media-failure", "pass", fmt.Sprintf("sanitized native Media failure and zero ownership in %s", time.Since(started)), s)
	return nil
}

func (c *client) tracks(ctx context.Context) (mpvprobe.Tracks, error) {
	nodes, err := c.tree(ctx)
	if err != nil {
		return mpvprobe.Tracks{}, err
	}
	for _, n := range nodes {
		if strings.HasPrefix(n.Label, "tracks ") {
			var tracks mpvprobe.Tracks
			if json.Unmarshal([]byte(strings.TrimPrefix(n.Label, "tracks ")), &tracks) == nil {
				return tracks, nil
			}
		}
	}
	return mpvprobe.Tracks{}, errors.New("selected-track evidence absent")
}

func (c *client) capabilities(ctx context.Context, dir string, r *report, srv *fixturehttp.Server) error {
	for _, key := range []string{"local", "http"} {
		if _, err := c.playback(ctx, dir, key, r); err != nil {
			return err
		}
		if key == "local" {
			if err := c.slidersAndRetry(ctx, r); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"no-headers", "wrong-headers", "failure"} {
		if err := c.mediaFailure(ctx, key, r); err != nil {
			return err
		}
	}
	before := srv.Stats().Streams["combined.mp4"]
	if _, err := c.playback(ctx, dir, "protected", r); err != nil {
		return err
	}
	after := srv.Stats().Streams["combined.mp4"]
	if after.Allowed <= before.Allowed || after.HeaderMatches <= before.HeaderMatches || after.RefererMatches <= before.RefererMatches {
		return errors.New("protected playback lacks custom-header and Referer match observations")
	}
	r.Checks = append(r.Checks, check{Name: "required-headers", Result: "pass", Detail: "comma-containing custom field and Referer matched; protected native picture/progression/seek pass"})
	before = after
	if err := c.mediaFailure(ctx, "no-headers", r); err != nil {
		return err
	}
	after = srv.Stats().Streams["combined.mp4"]
	if after.Denied <= before.Denied || after.Allowed != before.Allowed || after.HeaderMatches != before.HeaderMatches || after.RefererMatches != before.RefererMatches {
		return errors.New("later headerless session inherited prior metadata or was not denied")
	}
	r.Checks = append(r.Checks, check{Name: "header-session-isolation", Result: "pass", Detail: "new headerless core denied with neither previous field matched"})
	// Retrying a failed input must preserve its missing metadata and still fail.
	if err := c.mediaFailure(ctx, "retry", r); err != nil {
		return err
	}
	streams := srv.Stats().Streams
	s, err := c.playback(ctx, dir, "separate", r)
	if err != nil {
		return err
	}
	tracks, err := c.tracks(ctx)
	if err != nil || !tracks.Video || !tracks.Audio || !tracks.ExternalAudio {
		return errors.New("selected external audio/video not observed")
	}
	for _, file := range []string{"video.mp4", "audio.m4a"} {
		a, b := srv.Stats().Streams[file], streams[file]
		if a.Allowed <= b.Allowed || a.HeaderMatches <= b.HeaderMatches || a.RefererMatches <= b.RefererMatches {
			return errors.New("both protected streams must receive exact required metadata")
		}
	}
	record(r, "separate-track-selection-transport", "pass", "both protected paths consumed with metadata; selected video and external audio; picture/progression/seek/rate configuration pass", s)
	s, err = c.progress(ctx)
	if err != nil {
		return err
	}
	start := time.Now()
	if err = c.act(ctx, "cancel"); err != nil {
		return err
	}
	closed, err := c.until(ctx, cancelObservationLimit, func(now observed) bool {
		return now.Generation == s.Generation && now.State == contract.Closed && now.Error == contract.Canceled && !now.Owned
	})
	elapsed := time.Since(start)
	if err != nil || elapsed > cancelObservationLimit {
		record(r, "playing-cancel", "fail", fmt.Sprintf("external observation took %s (%s bound)", elapsed, cancelObservationLimit), closed)
		return errors.New("playing cancellation exceeded 200ms")
	}
	if err = c.zeroResources(ctx); err != nil {
		return err
	}
	record(r, "playing-cancel", "pass", fmt.Sprintf("closed/no ownership in %s; native/worker counters zero", elapsed), closed)
	if err = c.mediaFailure(ctx, "missing-audio", r); err != nil {
		return err
	}
	r.Checks = append(r.Checks,
		check{Name: "full-separate-track-capability", Result: "not-tested", Detail: "physical audio and calibrated A/V synchronization have no authorized observation method; fetched/selected tracks alone do not pass RPL-AC-05"},
		check{Name: "physical-audio-actual-speed-sync-IME-Windows-target-packaging-CI", Result: "not-tested", Detail: "requires separate runtime/target/measurement/release evidence"})
	return nil
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
	for i, key := range keys {
		if err = c.call(ctx, "input", map[string]any{"window": c.window, "events": []map[string]string{{"type": "key", "key": key}}}, nil); err != nil {
			return s, err
		}
		s, err = c.until(ctx, limit, func(s observed) bool { return predicate(i, s) })
		if err != nil {
			return s, fmt.Errorf("slider key %d %s: %w (generation=%d state=%s position=%s volume=%g control=%s)", i, key, err, s.Generation, s.State, s.Position, s.Volume, s.ControlError)
		}
	}
	return s, nil
}

func (c *client) slidersAndRetry(ctx context.Context, r *report) error {
	before, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	generation := before.Generation
	keys, positions := seekSliderPlan()
	s, err := c.sliderKeys(ctx, "seek-slider", keys, 2*time.Second, func(i int, s observed) bool {
		return s.Generation == generation && s.State == contract.Paused && s.Owned && abs(s.Position-positions[i]) <= 100*time.Millisecond && s.ControlError == contract.NoError
	})
	if err != nil {
		return fmt.Errorf("native seek slider: %w", err)
	}
	record(r, "seek-slider", "pass", "keyboard interaction through actual slider reaches native paused 4s within 2s/500ms", s)
	s, err = c.sliderKeys(ctx, "volume-slider", []string{"Home", "PageUp", "PageUp", "PageUp", "PageUp", "PageUp"}, time.Second, func(i int, s observed) bool {
		return s.Generation == generation && s.Owned && math.Abs(s.Volume-float64(i)/10) < .005 && s.ControlError == contract.NoError
	})
	if err != nil {
		return fmt.Errorf("native volume slider: %w", err)
	}
	for _, preset := range []struct {
		key  string
		rate float64
	}{{"speed-half", .5}, {"speed-normal", 1}, {"speed-double", 2}} {
		if err = c.act(ctx, preset.key); err != nil {
			return err
		}
		s, err = c.until(ctx, time.Second, func(s observed) bool {
			return s.Generation == generation && s.Owned && s.Rate == preset.rate && math.Abs(s.Volume-.5) < .005
		})
		if err != nil {
			return err
		}
		record(r, preset.key+"-configuration", "pass", "volume .5 and native rate configuration; no audible output or speed measurement", s)
	}
	if err = c.act(ctx, "close"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool {
		return s.Generation == generation && s.State == contract.Closed && !s.Owned && s.Error == contract.NoError
	}); err != nil {
		return err
	}
	if err = c.zeroResources(ctx); err != nil {
		return err
	}
	if err = c.act(ctx, "play"); err != nil {
		return err
	}
	if _, err = c.until(ctx, time.Second, func(s observed) bool {
		return s.Generation == generation && !s.Owned && s.ControlError == contract.IsClosed
	}); err != nil {
		return err
	}
	s, err = c.ready(ctx, "retry")
	if err != nil || s.Generation <= generation || s.ControlError != contract.NoError {
		return errors.New("explicit copied-input retry did not replace closed generation")
	}
	record(r, "explicit-close-retry", "pass", "new generation, cleared control error and closed-command rejection; failure retry separately preserves missing metadata", s)
	return nil
}

// Playback first establishes a paused 3s position. Four observed quarter-second
// arrow changes exercise the real slider to 4s under the unchanged 2s bound.
func seekSliderPlan() ([]string, []time.Duration) {
	return []string{"Right", "Right", "Right", "Right"}, []time.Duration{3250 * time.Millisecond, 3500 * time.Millisecond, 3750 * time.Millisecond, 4 * time.Second}
}
