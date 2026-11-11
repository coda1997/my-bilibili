package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"my-bilibili/experiments/playback/cmd/fixturehttp"
	"my-bilibili/experiments/playback/contract"
)

// An occluded runtime is not a decoder failure or a passing playback smoke.
// Still exercise independent negative/lifetime probes, without fake readiness.
func (c *client) hiddenMedia(ctx context.Context, r *report, srv *fixturehttp.Server) error {
	r.Checks = append(r.Checks, check{Name: "visible-native-media-runtime", Result: "not-tested", Detail: "UI Context.WindowHidden=true; mygo skips native video Frame while occluded"})
	for _, name := range []string{"local-ready-render", "local-playhead", "HTTP-ready-render-play", "pause-stability", "paused-seek", "HTTP-seek", "playing-cancel", "physical-audio-and-rate-speed", "real-IME"} {
		r.Checks = append(r.Checks, check{Name: name, Result: "not-tested", Detail: "no visible native picture; requires exposed runtime window (audio/IME additionally need human observation)"})
	}
	loading := func(key string) (observed, error) {
		return c.openUntil(ctx, key, 2*time.Second, func(s observed) bool { return s.State == contract.Loading && s.Owned && s.Duration > 0 })
	}
	s, err := loading("local")
	if err != nil {
		return fmt.Errorf("local asset access: %w", err)
	}
	record(r, "local-asset-access", "pass", "native duration readable; NOT decoded-picture/playback evidence", s)
	s, err = loading("http")
	if err != nil {
		return fmt.Errorf("HTTP asset access: %w", err)
	}
	if srv.Stats().Plain == 0 {
		return errors.New("no native HTTP fixture request observed")
	}
	record(r, "HTTP-asset-access", "pass", "native HTTP requests and duration observed; NOT rendered playback", s)
	if srv.Stats().Range > 0 {
		record(r, "native-HTTP-Range", "pass", "native Range request observed by controlled server; no seek claim", s)
	}
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
	record(r, "volume-rate-configuration", "pass", "public configuration readback only, during loading", s)
	a, err := c.openUntil(ctx, "delayed", 500*time.Millisecond, func(s observed) bool { return s.State == contract.Loading && s.Owned })
	if err != nil {
		return fmt.Errorf("delayed loading before switch: %w", err)
	}
	if err = wait(ctx, 150*time.Millisecond); err != nil {
		return err
	}
	s, err = loading("local")
	if err != nil {
		return err
	}
	if s.Generation <= a.Generation || s.Position > 500*time.Millisecond {
		return errors.New("loading replacement identity/reset failed")
	}
	generation := s.Generation
	if err = wait(ctx, 3100*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil {
		return err
	}
	if s.Generation != generation || !s.Owned || s.State != contract.Loading {
		return errors.New("obsolete delayed session restored state")
	}
	record(r, "loading-switch-generation", "pass", "replacement stays current after delayed-response window; still no picture", s)
	active, err := c.openUntil(ctx, "delayed", 500*time.Millisecond, func(s observed) bool { return s.State == contract.Loading && s.Owned })
	if err != nil {
		return fmt.Errorf("delayed loading before cancellation: %w", err)
	}
	start := time.Now()
	if err = c.act(ctx, "cancel"); err != nil {
		return err
	}
	s, err = c.until(ctx, 100*time.Millisecond, func(s observed) bool {
		return s.Generation == active.Generation && s.State == contract.Closed && s.Error == contract.Canceled && !s.Owned
	})
	if err != nil {
		return fmt.Errorf("loading cancellation observation: %w", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		return errors.New("loading cancellation exceeds 100ms")
	}
	record(r, "loading-cancel", "pass", fmt.Sprintf("owned engine removed; externally observed in %s", elapsed), s)
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
			return fmt.Errorf("%s native error: %w (generation=%d state=%s error=%s owned=%t)", key, err, s.Generation, s.State, s.Error, s.Owned)
		}
		record(r, key+"-native-error", "pass", "native HTTP failure sanitized and owned engine disposed", s)
	}
	if srv.Stats().ProtectedDenied == 0 || srv.Stats().Failed == 0 {
		return errors.New("negative native HTTP routes not requested")
	}
	if _, err = loading("local"); err != nil {
		return err
	}
	if err = c.act(ctx, "close"); err != nil {
		return err
	}
	s, err = c.until(ctx, time.Second, func(s observed) bool { return s.State == contract.Closed && !s.Owned })
	if err != nil {
		return err
	}
	generation = s.Generation
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
	_, err = loading("local")
	return err
}
