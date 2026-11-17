package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

type observed = previewbundle.Observation

func ready(s observed) bool {
	return (s.State.Playback.State == playback.Ready || s.State.Playback.State == playback.Paused) && s.State.Playback.Duration > 0 && s.State.Playback.Error == playback.NoError && s.Tracks.Video && s.Tracks.Audio && s.Resources.Cores > 0 && s.Resources.Renderers > 0 && s.Resources.Workers > 0
}
func disposed(s observed, code playback.ErrorCode) bool {
	return s.State.Playback.State == playback.Closed && s.State.Playback.Error == code && s.Resources == (mpv.Resources{}) && s.DisposalError == playback.NoError && !s.Tracks.Video && !s.Tracks.Audio && !s.State.DesiredPlaying
}
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (c *client) change(ctx context.Context, action func() error, predicate func(observed) bool) (observed, error) {
	if err := action(); err != nil {
		return observed{}, err
	}
	return c.until(ctx, 2*time.Second, predicate)
}
func (c *client) startPlaying(ctx context.Context) (observed, error) {
	s, err := c.until(ctx, 10*time.Second, ready)
	if err != nil {
		return s, err
	}
	before := s.State.Playback
	return c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, func(o observed) bool {
		return o.State.Playback.Generation == before.Generation && o.State.Playback.State == playback.Playing && o.State.Playback.Position >= before.Position+300*time.Millisecond && o.Tracks.Video && o.Tracks.Audio
	})
}
func (c *client) survive(ctx context.Context, p *ownedProcess, code playback.ErrorCode, r *report) error {
	start := time.Now()
	for {
		if p.exited() {
			return errors.New("preview exited during required live post-disposal survival")
		}
		s, err := c.snapshot(ctx)
		if err != nil {
			return err
		}
		if !disposed(s, code) {
			return errors.New("post-disposal state or ownership changed")
		}
		if p.exited() {
			return errors.New("preview exited after post-disposal sample")
		}
		if time.Since(start) >= 8*time.Second {
			r.pass("eight-second-live-disposal-survival", fmt.Sprintf("still-live owned process; complete zero counters for %s", time.Since(start)), &s)
			return nil
		}
		if err = wait(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
}
func (c *client) cancelParent(ctx context.Context, p *ownedProcess, r *report) error {
	before, err := c.snapshot(ctx)
	if err != nil {
		return err
	}
	// This single deadline spans initiating the UI action, native teardown and
	// obtaining a complete disposed observation. No post-action budget reset.
	start := time.Now()
	cancelCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if err = c.act(cancelCtx, "e2e-cancel"); err != nil {
		return err
	}
	s, err := c.until(cancelCtx, 200*time.Millisecond, func(o observed) bool {
		return o.State.Playback.Generation == before.State.Playback.Generation && disposed(o, playback.Canceled)
	})
	elapsed := time.Since(start)
	if err != nil || elapsed > 200*time.Millisecond {
		return errors.New("external cancellation did not reach complete disposal within 200ms")
	}
	r.pass("external-cancellation", fmt.Sprintf("initiating action through observed disposal: %s (limit 200ms)", elapsed), &s)
	return c.survive(ctx, p, playback.Canceled, r)
}
func (c *client) layout(ctx context.Context, wide bool) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return err
		}
		video, err := uniqueNode(nodes, "preview-video")
		if err != nil {
			return err
		}
		part, err := uniqueNode(nodes, "list-part-part-1")
		if err != nil {
			return err
		}
		w, windowErr := c.identify(ctx)
		if windowErr != nil {
			return windowErr
		}
		v, p := video.Bounds, part.Bounds
		valid := v.Width > 0 && v.Height > 0 && p.Width > 0 && p.Height > 0 && math.Abs(v.Width/v.Height-16.0/9) < .02
		beside := p.X >= v.X+v.Width && p.Y >= v.Y && p.Y < v.Y+v.Height
		below := p.Y >= v.Y+v.Height && p.X >= v.X && p.X < v.X+v.Width
		if valid && (wide && beside && w.ContentBounds.Width >= 1048 || !wide && below && w.ContentBounds.Width == 800) {
			return nil
		}
		if err = wait(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) product(ctx context.Context, p *ownedProcess, dir string, r *report) error {
	s, err := c.until(ctx, 10*time.Second, ready)
	if err != nil {
		return fmt.Errorf("startup readiness: %w", err)
	}
	if s.State.DesiredPlaying || s.State.PlaybackPart != "part-1" || s.State.Playback.Position > 150*time.Millisecond || s.State.Rate != 1 || s.State.Volume != 1 {
		return errors.New("startup is not paused P1 with default settings")
	}
	initial := s.State.Playback
	if err = wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil {
		return err
	}
	if !ready(s) || s.State.DesiredPlaying || s.State.Playback.Generation != initial.Generation || absDuration(s.State.Playback.Position-initial.Position) > 150*time.Millisecond {
		return errors.New("startup paused playback drifted")
	}
	pictureBefore, err := c.picture(ctx, dir, "part-1-paused.png")
	if err != nil {
		return err
	}
	r.pass("bundle-outside-cwd-startup-pause-picture", "P1 paused with selected video/audio and captured synthetic picture; cover and fixture files resolved from bundle", &s)
	s, err = c.startPlaying(ctx)
	if err != nil {
		return err
	}
	pictureAfter, err := c.picture(ctx, dir, "part-1-playing.png")
	if err != nil {
		return err
	}
	if pictureBefore == pictureAfter {
		return errors.New("native picture did not change during sampled playback progress")
	}
	r.pass("part-1-progress-frames", "native position advanced >=300ms and captured video pixels changed", &s)
	s, err = c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, func(o observed) bool { return o.State.Playback.State == playback.Paused && !o.State.DesiredPlaying })
	if err != nil {
		return err
	}
	paused := s.State.Playback
	if err = wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	s, err = c.snapshot(ctx)
	if err != nil {
		return err
	}
	if s.State.Playback.State != playback.Paused || absDuration(s.State.Playback.Position-paused.Position) > 150*time.Millisecond {
		return errors.New("pause drift exceeded 150ms")
	}
	r.pass("pause", "sampled native pause and <=150ms drift over 500ms", &s)
	s, err = c.change(ctx, func() error { return c.slider(ctx, "seek-slider", "5") }, func(o observed) bool {
		return o.State.Playback.Generation == paused.Generation && o.State.Playback.State == playback.Paused && absDuration(o.State.Playback.Position-5*time.Second) < 500*time.Millisecond
	})
	if err != nil {
		return fmt.Errorf("seek control: %w", err)
	}
	r.pass("seek-slider", "real slider pointer capture/release; sampled native position 5s +/-500ms", &s)
	s, err = c.change(ctx, func() error { return c.selectPopup(ctx, "speed-menu", "speed-2") }, func(o observed) bool { return o.State.Rate == 2 && o.State.Playback.Rate == 2 })
	if err != nil {
		return err
	}
	r.pass("rate-menu", "2x observed in native snapshot; physical rate not measured", &s)
	s, err = c.change(ctx, func() error { return c.slider(ctx, "volume-slider", "0.35") }, func(o observed) bool {
		return math.Abs(o.State.Playback.Volume-.35) < .01 && math.Abs(o.State.Volume-.35) < .01
	})
	if err != nil {
		return err
	}
	r.pass("volume-slider", "0.35 native volume through real slider", &s)
	s, err = c.change(ctx, func() error { return c.act(ctx, "mute-toggle") }, func(o observed) bool { return o.State.Playback.Volume == 0 && o.State.Volume == 0 })
	if err != nil {
		return err
	}
	r.pass("mute", "native volume zero", &s)
	s, err = c.change(ctx, func() error { return c.act(ctx, "mute-toggle") }, func(o observed) bool { return math.Abs(o.State.Playback.Volume-.35) < .01 })
	if err != nil {
		return err
	}
	r.pass("unmute", "native volume restored to chosen 0.35", &s)
	preserved := s
	for _, size := range []struct {
		key, name string
		wide      bool
	}{{"e2e-narrow", "narrow-layout.png", false}, {"e2e-wide", "wide-layout.png", true}} {
		if err = c.act(ctx, size.key); err != nil {
			return err
		}
		if err = c.layout(ctx, size.wide); err != nil {
			return err
		}
		s, err = c.snapshot(ctx)
		if err != nil {
			return err
		}
		if !preservesPaused(preserved, s) {
			return errors.New("resize changed playback session or settings")
		}
		if _, err = c.picture(ctx, dir, size.name); err != nil {
			return err
		}
		r.pass(size.key, "native size and responsive layout observed; picture/session/intent/settings preserved", &s)
	}
	s, err = c.fullscreenCycles(ctx, dir, r, preserved)
	if err != nil {
		return err
	}
	if r.Stage == "fullscreen" {
		return nil
	}
	c.phase = "parts-replay-disposal"
	gen := s.State.Playback.Generation
	s, err = c.change(ctx, func() error { return c.act(ctx, "next-part") }, func(o observed) bool {
		return o.State.Playback.Generation > gen && o.State.PlaybackPart == "part-2" && o.State.Playback.State == playback.Playing && o.State.Playback.Position >= 300*time.Millisecond && o.Tracks.Video && o.Tracks.Audio
	})
	if err != nil {
		return err
	}
	if _, err = c.picture(ctx, dir, "part-2-playing.png"); err != nil {
		return err
	}
	if s.State.Rate != 2 || math.Abs(s.State.Volume-.35) > .01 {
		return errors.New("part replacement lost selected settings")
	}
	r.pass("part-2-next-control", "P2 selected through Next; fresh generation, native progress/tracks and picture; settings persisted", &s)
	gen = s.State.Playback.Generation
	s, err = c.change(ctx, func() error { return c.selectPopup(ctx, "parts-menu", "part-part-1") }, func(o observed) bool {
		return o.State.Playback.Generation > gen && o.State.PlaybackPart == "part-1" && o.State.Playback.State == playback.Playing
	})
	if err != nil {
		return err
	}
	r.pass("parts-menu-selection", "returned to P1 through real part popup", &s)
	if err = c.slider(ctx, "seek-slider", "29"); err != nil {
		return err
	}
	s, err = c.until(ctx, 4*time.Second, func(o observed) bool { return o.State.Playback.State == playback.Ended && !o.State.DesiredPlaying })
	if err != nil {
		return err
	}
	r.pass("ended", "native end reached without auto-next", &s)
	gen = s.State.Playback.Generation
	s, err = c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, func(o observed) bool {
		return o.State.Playback.Generation > gen && o.State.PlaybackPart == "part-1" && o.State.Playback.State == playback.Playing && o.State.Playback.Position > 0 && o.State.Playback.Position < 3*time.Second
	})
	if err != nil {
		return err
	}
	r.pass("ended-replay", "Play reopened P1 at zero and native progress resumed", &s)
	if err = c.act(ctx, "e2e-dispose"); err != nil {
		return err
	}
	s, err = c.until(ctx, time.Second, func(o observed) bool { return disposed(o, playback.NoError) })
	if err != nil {
		return err
	}
	r.pass("dispose-without-quit", "Player.Close completed and every native ownership counter is zero", &s)
	return c.survive(ctx, p, playback.NoError, r)
}

// These are observations, not native transition-completion acknowledgments.
// Window origins are screen coordinates; node bounds are content-local points.
// Compare restored native bounds only with native baseline bounds, and rendered
// full-size video only with content dimensions. Pinned mygo exports float32
// content dimensions unchanged as float64; no pixel-scale tolerance is needed.
type fullscreenBaseline struct {
	content, video, part rectangle
}
type fullscreenSample struct {
	state          observed
	content, video rectangle
}

func sameRectangle(a, b rectangle) bool {
	return a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height
}
func fullscreenRectangle(r rectangle) bool {
	return r.complete && r.Width > 0 && r.Height > 0
}
func containedRectangle(inner, outer rectangle) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}
func fullscreenNodes(nodes []node, preserved observed) (observed, node, node, error) {
	s, err := observation(nodes)
	if err != nil {
		return s, node{}, node{}, err
	}
	if !preservesPaused(preserved, s) {
		return s, node{}, node{}, errors.New("fullscreen changed paused session or settings")
	}
	v, err := uniqueNode(nodes, "preview-video")
	if err != nil {
		return s, node{}, node{}, err
	}
	button, err := uniqueNode(nodes, "fullscreen-toggle")
	if err != nil {
		return s, node{}, node{}, err
	}
	if !fullscreenRectangle(v.Bounds) || !fullscreenRectangle(button.Bounds) {
		return s, node{}, node{}, errors.New("fullscreen node geometry unavailable")
	}
	return s, v, button, nil
}
func (c *client) ordinaryFullscreenBaseline(ctx context.Context, preserved observed) (fullscreenBaseline, error) {
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return fullscreenBaseline{}, err
		}
		completion, err := nativeCompletion(nodes)
		if err != nil {
			return fullscreenBaseline{}, err
		}
		if completion != (fullscreenCompletion{}) {
			return fullscreenBaseline{}, errors.New("unexpected fullscreen transition before baseline")
		}
		s, v, button, err := fullscreenNodes(nodes, preserved)
		if err != nil {
			return fullscreenBaseline{}, err
		}
		w, err := c.identify(ctx)
		if err != nil {
			return fullscreenBaseline{}, err
		}
		if !fullscreenRectangle(w.ContentBounds) {
			return fullscreenBaseline{}, errors.New("fullscreen content geometry unavailable")
		}
		// The product prefix restored the wide layout: 24pt page padding,
		// 280pt parts/gap column, and a 16:9 player (desktop/render.go).
		width := w.ContentBounds.Width - 328
		if !s.Fullscreen && button.Label == "Fullscreen" && w.ContentBounds.Width >= 1048 && v.Bounds.X == 24 && v.Bounds.Width == width && v.Bounds.Height == width*9/16 && containedRectangle(button.Bounds, v.Bounds) {
			part, err := uniqueNode(nodes, "list-part-part-1")
			if err != nil {
				return fullscreenBaseline{}, err
			}
			if !fullscreenRectangle(part.Bounds) {
				return fullscreenBaseline{}, errors.New("fullscreen normal layout geometry unavailable")
			}
			// The part button is nested below the panel title and padding.
			if part.Bounds.X >= v.Bounds.X+v.Bounds.Width && part.Bounds.Y >= v.Bounds.Y && part.Bounds.Y < v.Bounds.Y+v.Bounds.Height {
				return fullscreenBaseline{w.ContentBounds, v.Bounds, part.Bounds}, nil
			}
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return fullscreenBaseline{}, err
		}
	}
}
func fullscreenMatches(nodes []node, w window, desired bool, baseline fullscreenBaseline, preserved observed) (fullscreenSample, bool, error) {
	s, v, button, err := fullscreenNodes(nodes, preserved)
	if err != nil {
		return fullscreenSample{}, false, err
	}
	if !fullscreenRectangle(w.ContentBounds) {
		return fullscreenSample{}, false, errors.New("fullscreen content geometry unavailable")
	}
	sample := fullscreenSample{s, w.ContentBounds, v.Bounds}
	if s.Fullscreen != desired || !containedRectangle(button.Bounds, v.Bounds) {
		return sample, false, nil
	}
	for _, state := range button.States {
		if state == "disabled" || state == "offscreen" {
			return sample, false, nil
		}
	}
	if desired {
		// A fullscreen render at ordinary size is a transition, not entry.
		geometry := w.ContentBounds.Width != baseline.content.Width || w.ContentBounds.Height != baseline.content.Height
		layout := sameRectangle(v.Bounds, rectangle{Width: w.ContentBounds.Width, Height: w.ContentBounds.Height})
		return sample, geometry && layout && button.Label == "Exit full", nil
	}
	// Cached bool, button and layout can straddle frames during transition.
	// Require a normal-mode correspondence before demanding its part node.
	if button.Label != "Fullscreen" || !sameRectangle(w.ContentBounds, baseline.content) || !sameRectangle(v.Bounds, baseline.video) {
		return sample, false, nil
	}
	part, err := uniqueNode(nodes, "list-part-part-1")
	if err != nil {
		return sample, false, err
	}
	if !fullscreenRectangle(part.Bounds) {
		return sample, false, errors.New("fullscreen normal layout geometry unavailable")
	}
	return sample, sameRectangle(part.Bounds, baseline.part), nil
}
func (c *client) awaitFullscreen(ctx context.Context, desired bool, baseline fullscreenBaseline, preserved observed) (fullscreenSample, error) {
	for {
		var first fullscreenSample
		coherent := true
		// Two tree/native pairs corroborate each other. Tree is last-rendered;
		// windows queries native geometry now. The separate e2e node carries
		// recorded native completion counters; geometry alone is insufficient.
		for pair := 0; pair < 2; pair++ {
			nodes, err := c.tree(ctx)
			if err != nil {
				return fullscreenSample{}, err
			}
			w, err := c.identify(ctx)
			if err != nil {
				return fullscreenSample{}, err
			}
			sample, matches, err := fullscreenMatches(nodes, w, desired, baseline, preserved)
			if err != nil {
				return fullscreenSample{}, err
			}
			coherent = coherent && matches
			completion, err := nativeCompletion(nodes)
			if err != nil {
				return fullscreenSample{}, err
			}
			coherent = coherent && completion == c.completion
			if pair == 0 {
				first = sample
			} else {
				coherent = coherent && sameRectangle(first.content, sample.content) && sameRectangle(first.video, sample.video)
				if err := ctx.Err(); err != nil {
					return sample, err
				}
				if coherent {
					return sample, nil
				}
			}
		}
		if err := wait(ctx, 25*time.Millisecond); err != nil {
			return fullscreenSample{}, err
		}
	}
}
func (c *client) fullscreenEntry(ctx context.Context, dir string, cycle int, baseline fullscreenBaseline, preserved observed) (observed, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c.phase = "entry-precondition"
	if _, err := c.awaitFullscreen(ctx, false, baseline, preserved); err != nil {
		return observed{}, err
	}
	c.phase = "entry-action"
	c.completion.requests++
	c.completion.entered++
	if err := c.act(ctx, "fullscreen-toggle"); err != nil {
		return observed{}, err
	}
	c.phase = "entry-correspondence"
	if _, err := c.awaitFullscreen(ctx, true, baseline, preserved); err != nil {
		return observed{}, err
	}
	c.phase = "entry-existing-wait"
	if err := wait(ctx, 500*time.Millisecond); err != nil {
		return observed{}, err
	}
	c.phase = "entry-pre-capture"
	picture, err := c.awaitFullscreen(ctx, true, baseline, preserved)
	if err != nil {
		return observed{}, err
	}
	c.phase = "entry-capture"
	if _, err := c.capturePicture(ctx, dir, fmt.Sprintf("fullscreen-%d.png", cycle), picture.video, picture.content); err != nil {
		return observed{}, err
	}
	// Capture rasterizes the last content frame and may outlast its sampled
	// precondition. Reobserve within this same entry deadline before reversing.
	c.phase = "entry-post-capture"
	last, err := c.awaitFullscreen(ctx, true, baseline, preserved)
	return last.state, err
}
func (c *client) fullscreenExit(ctx context.Context, cycle int, baseline fullscreenBaseline, preserved observed) (observed, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var err error
	c.phase = "exit-action"
	c.completion.requests++
	c.completion.exited++
	if cycle == 3 {
		err = c.key(ctx, "Escape")
	} else {
		err = c.act(ctx, "fullscreen-toggle")
	}
	if err != nil {
		return observed{}, err
	}
	c.phase = "exit-correspondence"
	s, err := c.awaitFullscreen(ctx, false, baseline, preserved)
	return s.state, err
}
func (c *client) fullscreenCycles(ctx context.Context, dir string, r *report, preserved observed) (observed, error) {
	c.phase = "fullscreen-baseline"
	initial, cancel := context.WithTimeout(ctx, 5*time.Second)
	baseline, err := c.ordinaryFullscreenBaseline(initial, preserved)
	cancel()
	if err != nil {
		return observed{}, err
	}
	s := preserved
	for cycle := 1; cycle <= 3; cycle++ {
		c.cycle = cycle
		if s, err = c.fullscreenEntry(ctx, dir, cycle, baseline, preserved); err != nil {
			return s, fmt.Errorf("fullscreen cycle %d %s: %w", cycle, c.phase, err)
		}
		if s, err = c.fullscreenExit(ctx, cycle, baseline, preserved); err != nil {
			return s, fmt.Errorf("fullscreen cycle %d %s: %w", cycle, c.phase, err)
		}
		r.pass(fmt.Sprintf("fullscreen-cycle-%d", cycle), "native DidEnter/DidExit counts acknowledge each request; cached state, content bounds and layout corroborate entry/exit; ordinary bounds, paused session/settings and content picture preserved", &s)
	}
	return s, nil
}
func preservesPaused(before, after observed) bool {
	a, b := before.State, after.State
	return a.Playback.Generation == b.Playback.Generation && b.Playback.State == playback.Paused && !b.DesiredPlaying && a.SelectedPart == b.SelectedPart && a.PlaybackPart == b.PlaybackPart && a.SelectionError == b.SelectionError && a.Playback.Error == b.Playback.Error && a.Volume == b.Volume && a.Rate == b.Rate && b.Playback.Rate == a.Playback.Rate && b.Playback.Volume == a.Playback.Volume && absDuration(a.Playback.Position-b.Playback.Position) < 150*time.Millisecond
}
func run(ctx context.Context, app, dir string, r *report) error {
	if err := invocation(ctx, app, "product", "", dir, r, func(ctx context.Context, c *client, p *ownedProcess) error { return c.product(ctx, p, dir, r) }); err != nil {
		return err
	}
	if r.Stage == "fullscreen" {
		return nil
	}
	if err := invocation(ctx, app, "playing-cancel", "", dir, r, func(ctx context.Context, c *client, p *ownedProcess) error {
		if _, err := c.startPlaying(ctx); err != nil {
			return err
		}
		return c.cancelParent(ctx, p, r)
	}); err != nil {
		return err
	}
	if err := invocation(ctx, app, "loading-cancel", "delayed", dir, r, func(ctx context.Context, c *client, p *ownedProcess) error {
		if _, err := c.until(ctx, 3*time.Second, func(s observed) bool {
			return s.State.Playback.State == playback.Loading && s.Resources.Cores > 0 && s.Delay.Requests > 0 && s.Delay.Active > 0
		}); err != nil {
			return err
		}
		return c.cancelParent(ctx, p, r)
	}); err != nil {
		return err
	}
	if err := invocation(ctx, app, "playing-window-close", "", dir, r, func(ctx context.Context, c *client, p *ownedProcess) error {
		s, err := c.startPlaying(ctx)
		if err == nil {
			r.pass("fresh-playing-before-window-close", "separate invocation; native playback advanced", &s)
		}
		return err
	}); err != nil {
		return err
	}
	return invocation(ctx, app, "loading-window-close", "delayed", dir, r, func(ctx context.Context, c *client, p *ownedProcess) error {
		s, err := c.until(ctx, 3*time.Second, func(s observed) bool {
			return s.State.Playback.State == playback.Loading && s.Resources.Cores > 0 && s.Delay.Requests > 0 && s.Delay.Active > 0
		})
		if err == nil {
			r.pass("fresh-loading-before-window-close", "separate delayed invocation; actual delayed request active during native loading", &s)
		}
		return err
	})
}
