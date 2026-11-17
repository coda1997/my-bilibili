package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

func (c *client) pauseDanmaku(ctx context.Context) (observed, error) {
	return c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, danmakuPaused)
}

func (c *client) seekDanmaku(ctx context.Context, position time.Duration) (observed, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.slider(ctx, "seek-slider", strconv.FormatFloat(position.Seconds(), 'f', 3, 64)); err != nil {
		return observed{}, err
	}
	for {
		before, err := c.until(ctx, 2*time.Second, func(o observed) bool {
			return danmakuPaused(o) && absDuration(o.State.Playback.Position-position) <= 100*time.Millisecond
		})
		if err != nil {
			return observed{}, err
		}
		if err = wait(ctx, 100*time.Millisecond); err != nil {
			return observed{}, err
		}
		after, err := c.snapshot(ctx)
		if err != nil {
			return observed{}, err
		}
		if samePreparedDanmaku(before, after) {
			return after, nil
		}
	}
}

func (c *client) openDanmaku(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.act(ctx, "danmaku-menu"); err != nil {
		return err
	}
	for {
		ready, err := c.popupItemReady(ctx, "danmaku-visible")
		if err != nil || ready {
			return err
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) closeDanmakuPopup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.key(ctx, "Escape"); err != nil {
		return err
	}
	for {
		nodes, err := c.tree(ctx)
		if err != nil {
			return err
		}
		if popupsClosed(nodes) {
			return nil
		}
		if err = wait(ctx, 25*time.Millisecond); err != nil {
			return err
		}
	}
}

func (c *client) setDanmaku(ctx context.Context, key, value string, settings danmaku.Settings) (observed, error) {
	if err := c.openDanmaku(ctx); err != nil {
		return observed{}, err
	}
	var err error
	if key == "danmaku-visible" {
		err = c.act(ctx, key)
	} else {
		err = c.slider(ctx, key, value)
	}
	if err != nil {
		return observed{}, err
	}
	if _, err = c.until(ctx, 2*time.Second, func(o observed) bool { return danmakuPaused(o) && o.Danmaku.Settings == settings }); err != nil {
		return observed{}, err
	}
	if err = c.closeDanmakuPopup(ctx); err != nil {
		return observed{}, err
	}
	return c.until(ctx, 2*time.Second, func(o observed) bool { return danmakuPaused(o) && o.Danmaku.Settings == settings })
}

func (c *client) danmaku(ctx context.Context, p *ownedProcess, dir string, r *report) error {
	c.phase = "danmaku-startup"
	initial, err := c.until(ctx, 10*time.Second, ready)
	if err != nil {
		return err
	}
	if initial.State.DesiredPlaying || initial.State.PlaybackPart != "part-1" || initial.State.Playback.Position > 150*time.Millisecond || initial.State.Rate != 1 || initial.State.Volume != 1 || initial.Danmaku.Settings != danmaku.DefaultSettings() {
		return errors.New("danmaku fixture did not start paused with default settings")
	}
	if _, err = c.startPlaying(ctx); err != nil {
		return err
	}
	if _, err = c.pauseDanmaku(ctx); err != nil {
		return err
	}
	if _, err = c.seekDanmaku(ctx, 2*time.Second); err != nil {
		return fmt.Errorf("danmaku initial seek: %w", err)
	}
	c.phase = "danmaku-paused-pixels"
	first, err := c.danmakuPicture(ctx, dir, "danmaku-p1-visible")
	if err != nil {
		return err
	}
	r.pass("danmaku-p1-original-capture", "settled P1 near 2s, fitted native picture and original Chinese title crop saved with popovers closed", &first.state)
	start := time.Now()
	if err = wait(ctx, 500*time.Millisecond); err != nil {
		return err
	}
	second, err := c.danmakuPicture(ctx, dir, "danmaku-p1-paused")
	if err != nil {
		return err
	}
	if err = returnedDanmaku(first.state, second.state, 0); err != nil {
		return err
	}
	if !equalDanmakuPixels(first, second) {
		return errors.New("paused danmaku video crop changed over 500ms")
	}
	r.pass("danmaku-pause-pixels-and-position", fmt.Sprintf("identical video crop, items, lanes and geometry after %s with unchanged native media position", time.Since(start)), &second.state)
	settings := first.state.Danmaku.Settings
	settings.Visible = false
	if _, err = c.setDanmaku(ctx, "danmaku-visible", "", settings); err != nil {
		return err
	}
	hidden, err := c.danmakuPicture(ctx, dir, "danmaku-p1-hidden")
	if err != nil {
		return err
	}
	changed, err := hiddenDanmakuPixels(first, hidden)
	if err != nil {
		return err
	}
	r.pass("danmaku-visible-hidden-pixels", fmt.Sprintf("actual checkbox removed %d pixels inside observed text bounds, including the original title; background outside text stayed identical", changed), &hidden.state)
	settings.Visible = true
	if _, err = c.setDanmaku(ctx, "danmaku-visible", "", settings); err != nil {
		return err
	}
	restored, err := c.danmakuPicture(ctx, dir, "danmaku-p1-restored")
	if err != nil {
		return err
	}
	if !equalDanmakuPixels(first, restored) || returnedDanmaku(first.state, restored.state, 0) != nil {
		return errors.New("visible danmaku did not return at the paused media position")
	}
	r.pass("danmaku-visibility-restored", "original pixels and current-position entries returned through the actual visibility checkbox", &restored.state)

	c.phase = "danmaku-media-time"
	for _, rate := range []float64{1, 2} {
		if rate != 1 {
			if err = c.selectPopup(ctx, "speed-menu", "speed-2"); err != nil {
				return err
			}
		}
		if _, err = c.seekDanmaku(ctx, 2*time.Second); err != nil {
			return err
		}
		before, err := c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, func(o observed) bool {
			return o.State.Playback.State == playback.Playing && o.State.Rate == rate && o.State.Playback.Rate == rate && danmakuCurrent(o)
		})
		if err != nil {
			return err
		}
		r.pass(fmt.Sprintf("danmaku-rate-%.0f-start", rate), "native playing sample before comparing common scroll items", &before)
		after, err := c.until(ctx, 2*time.Second, func(o observed) bool {
			return danmakuCurrent(o) && o.State.Playback.Position >= before.State.Playback.Position+400*time.Millisecond
		})
		if err != nil {
			return err
		}
		if err = movingDanmaku(before, after, rate); err != nil {
			return err
		}
		r.pass(fmt.Sprintf("danmaku-rate-%.0f-media-time", rate), fmt.Sprintf("both scroll directions follow observed media delta %s and actual shaped travel width; no additional native-rate multiplier", after.State.Playback.Position-before.State.Playback.Position), &after)
		if _, err = c.pauseDanmaku(ctx); err != nil {
			return err
		}
	}
	c.phase = "danmaku-seek-and-settings"
	back, err := c.seekDanmaku(ctx, 2*time.Second)
	if err != nil {
		return err
	}
	if err = returnedDanmaku(first.state, back, 100*time.Millisecond); err != nil {
		return err
	}
	r.pass("danmaku-seek-return", "same entries and lanes returned within 100ms of the original native position; geometry follows only that position delta", &back)
	retained, err := c.danmakuSettings(ctx, dir, r, back)
	if err != nil {
		return err
	}
	c.phase = "danmaku-fullscreen-and-input"
	if err = c.danmakuLayout(ctx, dir, r, retained); err != nil {
		return err
	}

	c.phase = "danmaku-overflow-and-part"
	o, err := c.seekDanmaku(ctx, 12500*time.Millisecond)
	if err != nil {
		return err
	}
	if !denseDanmaku(o) {
		return errors.New("dense original fixture did not demonstrate bounded active layouts and overflow")
	}
	r.pass("danmaku-dense-overflow", "64 original entries produce nonzero deterministic overflow with at most 16 current active items/layouts", &o)
	previous := o.State.Playback.Generation
	o, err = c.change(ctx, func() error { return c.act(ctx, "next-part") }, func(o observed) bool {
		return o.State.PlaybackPart == "part-2" && o.State.Playback.Generation > previous && o.State.Playback.State == playback.Playing && o.State.Playback.Position >= 300*time.Millisecond && o.Tracks.Video && o.Tracks.Audio && danmakuCurrent(o)
	})
	if err != nil {
		return err
	}
	if _, err = c.pauseDanmaku(ctx); err != nil {
		return err
	}
	if _, err = c.seekDanmaku(ctx, 2*time.Second); err != nil {
		return err
	}
	part2, err := c.danmakuPicture(ctx, dir, "danmaku-p2-visible")
	if err != nil {
		return err
	}
	if part2.state.Danmaku.Generation <= previous || part2.state.Danmaku.Settings != retained.Danmaku.Settings {
		return errors.New("new part retained an old danmaku generation or lost settings")
	}
	r.pass("danmaku-part-2-original-capture", "accepted P2/native and danmaku generations match; retained settings and original second-part Chinese crop saved", &part2.state)
	r.Checks = append(r.Checks, check{Scenario: r.Scenario, Name: "danmaku-Chinese-glyph-recognition", Result: "not-tested", Detail: "Independent inspection must recognize 中文弹幕测试 in danmaku-p1-visible.png and 第二分P弹幕 in danmaku-p2-visible.png, excluding tofu/rectangles and old-part text; accessibility strings are not glyph proof"})
	c.phase = "danmaku-external-cancellation"
	o, err = c.change(ctx, func() error { return c.act(ctx, "play-toggle") }, func(o observed) bool {
		return o.State.Playback.State == playback.Playing && o.State.Playback.Position >= part2.state.State.Playback.Position+200*time.Millisecond && danmakuCurrent(o) && o.Danmaku.Active > 0
	})
	if err != nil {
		return err
	}
	r.pass("danmaku-active-before-cancellation", "accepted P2 is playing with active retained text before external cancellation", &o)
	return c.cancelParent(ctx, p, r)
}

func denseDanmaku(o observed) bool {
	d := o.Danmaku
	return danmakuCurrent(o) && d.Stats.Overflow > 0 && d.Active > 0 && d.Active <= 16 && d.Layouts == d.Active && d.Layouts <= 16
}

func (c *client) danmakuSettings(ctx context.Context, dir string, r *report, before observed) (observed, error) {
	settings := before.Danmaku.Settings
	settings.Opacity = 0
	if _, err := c.setDanmaku(ctx, "danmaku-opacity", "0", settings); err != nil {
		return observed{}, err
	}
	zero, err := c.danmakuPicture(ctx, dir, "danmaku-p1-zero-opacity")
	if err != nil {
		return observed{}, err
	}
	settings.Opacity = .5
	if _, err = c.setDanmaku(ctx, "danmaku-opacity", ".5", settings); err != nil {
		return observed{}, err
	}
	partial, err := c.danmakuPicture(ctx, dir, "danmaku-p1-partial-opacity")
	if err != nil {
		return observed{}, err
	}
	if _, err = hiddenDanmakuPixels(partial, zero); err != nil {
		return observed{}, err
	}
	r.pass("danmaku-zero-partial-opacity", "real opacity slider endpoints zero and partial 0.5 have inactive/active layouts and discriminating same-position pixels", &partial.state)
	var current observed
	for _, size := range []float64{16, 40, 32} {
		settings.FontSize = size
		current, err = c.setDanmaku(ctx, "danmaku-font-size", strconv.FormatFloat(size, 'f', 0, 64), settings)
		if err != nil {
			return observed{}, err
		}
		r.pass(fmt.Sprintf("danmaku-font-size-%.0f", size), "actual text-size slider reached the observed endpoint or interior target", &current)
	}
	if !largerDanmakuText(partial.state, current) {
		return observed{}, errors.New("larger font setting did not increase the original title's measured native layout")
	}
	r.pass("danmaku-native-font-metrics", "32-DIP title has larger shaped width and height than the original 24-DIP title at the same paused position", &current)
	speedBefore := current
	for _, speed := range []float64{.5, 2, 1.5} {
		settings.Speed = speed
		current, err = c.setDanmaku(ctx, "danmaku-speed", strconv.FormatFloat(speed, 'f', 1, 64), settings)
		if err != nil {
			return observed{}, err
		}
		r.pass(fmt.Sprintf("danmaku-scroll-speed-%.1f", speed), "actual danmaku-speed slider reached the observed endpoint or interior target", &current)
	}
	if !changedDanmakuSpeed(speedBefore, current) {
		return observed{}, errors.New("danmaku speed did not recompute scroll progress from the same media position")
	}
	r.pass("danmaku-speed-current-position", "1.5× danmaku speed recomputed the same scrolling item from its lifetime at unchanged paused media time", &current)
	return current, nil
}

func largerDanmakuText(a, b observed) bool {
	for _, old := range a.Danmaku.Items {
		for _, next := range b.Danmaku.Items {
			if old.Index == 0 && next.Index == 0 {
				return a.State.Playback.Position == b.State.Playback.Position && b.Danmaku.Settings.FontSize > a.Danmaku.Settings.FontSize && next.Width > old.Width && next.Height > old.Height
			}
		}
	}
	return false
}

func changedDanmakuSpeed(a, b observed) bool {
	if a.State.Playback.Position != b.State.Playback.Position || a.Danmaku.Settings.Speed == b.Danmaku.Settings.Speed {
		return false
	}
	for _, old := range a.Danmaku.Items {
		for _, next := range b.Danmaku.Items {
			if old.Index == 1 && next.Index == 1 && old.Mode == danmaku.ScrollRTL && next.Mode == old.Mode {
				ratio := float64(danmakuLife(old.Mode, a.Danmaku.Settings.Speed)) / float64(danmakuLife(old.Mode, b.Danmaku.Settings.Speed))
				return old.Progress > 0 && math.Abs(next.Progress-old.Progress*ratio) < 1e-6 && math.Abs(next.X-(b.Danmaku.Width-next.Progress*(b.Danmaku.Width+next.Width))) < .01
			}
		}
	}
	return false
}
