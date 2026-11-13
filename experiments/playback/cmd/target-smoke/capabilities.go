package main

import (
	"context"
	"errors"
	"math"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	"my-bilibili/experiments/playback/targetprobe"
)

func validateDescriptor(ctx context.Context, b []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err := targetprobe.Parse(b, time.Now)
	return err
}
func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
func active(s observed, generation uint64) bool {
	return s.Generation == generation && s.Owned && !s.WindowHidden && s.Error == contract.NoError && s.ControlError == contract.NoError
}
func (c *client) progress(ctx context.Context) (observed, error) {
	before, err := c.snapshot(ctx)
	if err != nil {
		return before, err
	}
	return c.actionUntil(ctx, "play", 2*time.Second, func(s observed) bool {
		return active(s, before.Generation) && s.State == contract.Playing && s.Position-before.Position >= 300*time.Millisecond
	})
}
func (c *client) capabilities(ctx context.Context, r *report) (err error) {
	defer func() { r.sessionFailed(err) }()
	r.setPhase(phaseInitialSnapshot)
	before, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	start := time.Now()
	readyCtx, readyCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readyCancel()
	r.setPhase(phaseReady)
	s, err := c.actionUntil(readyCtx, "open", 10*time.Second, func(s observed) bool {
		return s.Generation > before.Generation && active(s, s.Generation) && s.State == contract.Ready && s.Duration > 0
	})
	if err != nil {
		return err
	}
	generation := s.Generation
	r.setPhase(phasePictureTracks)
	// The first ready frame may precede the renderer's first bitmap. Poll only
	// this owned session's UI evidence; never trigger another open or rendition.
	for {
		if s, err = c.pictureTracks(readyCtx, generation); err == nil {
			break
		}
		if !errors.Is(err, errPicturePending) {
			return err
		}
		if e := wait(readyCtx, 25*time.Millisecond); e != nil {
			return e
		}
		s, err = c.snapshot(readyCtx)
		if err != nil {
			return err
		}
		if !active(s, generation) || s.State != contract.Ready {
			return contract.Media
		}
	}
	r.setPhase(phaseReadyResources)
	resources, err := c.resources(readyCtx)
	if err != nil {
		return err
	}
	if resources.Cores != 1 || resources.Renderers != 1 || resources.Workers < 1 {
		return contract.Media
	}
	record(r, "ready-picture-selected-tracks", "visible owned window; rendered Image node; selected video and external audio; no frame capture", s, time.Since(start))
	readyCancel()
	start = time.Now()
	r.setPhase(phaseProgress)
	s, err = c.progress(ctx)
	if err != nil {
		return err
	}
	record(r, "playhead-progression", "position advanced at least 300ms within 2s", s, time.Since(start))
	r.setPhase(phasePause)
	s, err = c.actionUntil(ctx, "pause", time.Second, func(s observed) bool { return active(s, generation) && s.State == contract.Paused })
	if err != nil {
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
	if !active(a, generation) || !active(s, generation) || a.State != contract.Paused || s.State != contract.Paused || abs(s.Position-a.Position) > 150*time.Millisecond {
		return contract.Media
	}
	record(r, "pause-stability", "paused drift at most 150ms across 500ms after 200ms settling", s, 0)
	start = time.Now()
	r.setPhase(phaseSeek)
	s, err = c.actionUntil(ctx, "seek", 2*time.Second, func(s observed) bool {
		return active(s, generation) && s.State == contract.Paused && abs(s.Position-3*time.Second) <= 500*time.Millisecond
	})
	if err != nil {
		return err
	}
	record(r, "seek", "paused native position 3s +/-500ms within 2s", s, time.Since(start))
	r.setPhase(phaseVolume)
	s, err = c.actionUntil(ctx, "volume", time.Second, func(s observed) bool { return active(s, generation) && math.Abs(s.Volume-.25) < .005 })
	if err != nil {
		return err
	}
	record(r, "volume-configuration", "native volume .25; no calibrated audible measurement", s, 0)
	r.setPhase(phaseRate)
	s, err = c.actionUntil(ctx, "rate", time.Second, func(s observed) bool { return active(s, generation) && s.Rate == 1.5 })
	if err != nil {
		return err
	}
	record(r, "rate-configuration", "native rate 1.5; actual speed and physical synchronization not measured", s, 0)
	r.setPhase(phaseResume)
	if _, err = c.progress(ctx); err != nil {
		return err
	}
	start = time.Now()
	r.setPhase(phaseCancel)
	s, err = c.actionUntil(ctx, "cancel", 200*time.Millisecond, func(s observed) bool {
		return s.Generation == generation && s.State == contract.Closed && s.Error == contract.Canceled && !s.Owned
	})
	elapsed := time.Since(start)
	if err != nil || elapsed > 200*time.Millisecond {
		r.Checks = append(r.Checks, check{Name: "playing-cancel", Result: "fail", Detail: "external cancellation missed 200ms bound", Elapsed: elapsed})
		if err != nil {
			return err
		}
		return contract.Timeout
	}
	r.setPhase(phaseCancelResources)
	resources, err = c.resources(ctx)
	if err != nil {
		return err
	}
	if resources != (mpvprobe.Resources{}) {
		return contract.Media
	}
	record(r, "playing-cancel", "closed/canceled and no ownership within 200ms; native counters zero", s, elapsed)
	r.Checks = append(r.Checks, check{Name: "loading-cancel", Result: "not-tested", Detail: "single target session exercises playing cancellation; no second open or retry"})
	return nil
}
