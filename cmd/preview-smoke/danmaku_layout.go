package main

import (
	"context"
	"errors"
	"time"

	"my-bilibili/internal/playback"
)

func retainedDanmaku(a, b observed) bool {
	return preservesPaused(a, b) && danmakuCurrent(b) && a.Resources == b.Resources && a.Danmaku.Settings == b.Danmaku.Settings && a.Danmaku.Generation == b.Danmaku.Generation
}

func danmakuPopupDismissed(nodes []node, preserved observed, expected fullscreenCompletion) (observed, error) {
	o, err := observation(nodes)
	if err != nil {
		return observed{}, err
	}
	completion, err := nativeCompletion(nodes)
	if err != nil {
		return observed{}, err
	}
	if !o.Fullscreen || !retainedDanmaku(preserved, o) || completion != expected || !popupsClosed(nodes) {
		return observed{}, errors.New("Escape exited fullscreen before dismissing the danmaku popup")
	}
	return o, nil
}

func (c *client) inspectDanmakuPopup(ctx context.Context) error {
	if err := c.openDanmaku(ctx); err != nil {
		return err
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	w, err := c.identify(ctx)
	if err != nil {
		return err
	}
	return danmakuControlsBounds(nodes, w.ContentBounds)
}

// Press, observe a frame, then release using the same pointer input path as
// sliders. No semantic action or player command can substitute for video hit
// testing. Once pressed, release is still attempted after a canceled check.
func (c *client) clickVideo(ctx context.Context, x, y float64) (err error) {
	if err = c.input(ctx, map[string]any{"type": "pointerMove", "x": x, "y": y}, map[string]any{"type": "pointerDown", "x": x, "y": y}); err != nil {
		return err
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		err = errors.Join(err, c.input(release, map[string]any{"type": "pointerUp", "x": x, "y": y}))
	}()
	_, err = c.tree(ctx)
	return err
}

func (c *client) danmakuLayout(ctx context.Context, dir string, r *report, preserved observed) error {
	baselineCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	baseline, err := c.ordinaryFullscreenBaseline(baselineCtx, preserved)
	cancel()
	if err != nil {
		return err
	}
	c.cycle = 1
	entered, err := c.fullscreenEntry(ctx, dir, 1, baseline, preserved)
	if err != nil {
		return err
	}
	if !retainedDanmaku(preserved, entered) {
		return errors.New("fullscreen changed native ownership or danmaku settings")
	}
	full, err := c.danmakuPicture(ctx, dir, "danmaku-fullscreen")
	if err != nil {
		return err
	}
	r.pass("danmaku-fullscreen-fitted-clip", "native DidEnter acknowledgement plus fitted overlay bounds and original fixture pixels; surface excludes fullscreen letterboxing", &full.state)
	if err = c.inspectDanmakuPopup(ctx); err != nil {
		return err
	}
	if err = c.closeDanmakuPopup(ctx); err != nil {
		return err
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	o, err := danmakuPopupDismissed(nodes, preserved, c.completion)
	if err != nil {
		return err
	}
	r.pass("danmaku-fullscreen-popup-escape", "all settings controls fit native fullscreen; first Escape dismisses the popup without a fullscreen request or exit callback", &o)
	exited, err := c.fullscreenExit(ctx, 3, baseline, preserved)
	if err != nil {
		return err
	}
	if !retainedDanmaku(preserved, exited) {
		return errors.New("leaving fullscreen changed native ownership or danmaku settings")
	}
	r.pass("danmaku-native-fullscreen-exit", "subsequent Escape receives native DidExit; ordinary geometry, native generation, playback and danmaku settings restored", &exited)
	c.phase = "danmaku-minimum-size"
	if err = c.act(ctx, "e2e-narrow"); err != nil {
		return err
	}
	if err = c.layout(ctx, false); err != nil {
		return err
	}
	w, err := c.identify(ctx)
	if err != nil {
		return err
	}
	if w.ContentBounds.Width != 800 || w.ContentBounds.Height != 600 {
		return errors.New("danmaku minimum-size check did not reach 800x600 native content")
	}
	narrow, err := c.danmakuPicture(ctx, dir, "danmaku-minimum-size")
	if err != nil {
		return err
	}
	if !retainedDanmaku(preserved, narrow.state) {
		return errors.New("minimum-size resize changed native ownership or danmaku settings")
	}
	if err = c.inspectDanmakuPopup(ctx); err != nil {
		return err
	}
	if err = c.closeDanmakuPopup(ctx); err != nil {
		return err
	}
	r.pass("danmaku-minimum-size-controls", "actual 800x600 native content, fitted video and popup/control bounds; native generation and all chosen settings preserved", &narrow.state)
	c.phase = "danmaku-video-pointer"
	// The point lies on the original top title, far above transport controls.
	var x, y float64
	found := false
	for _, item := range narrow.state.Danmaku.Items {
		if item.Index == 0 {
			x, y = narrow.surface.X+item.X+item.Width/2, narrow.surface.Y+item.Y+item.Height/2
			found = true
		}
	}
	if !found {
		return errors.New("original title unavailable for actual overlay hit testing")
	}
	playing, err := c.change(ctx, func() error { return c.clickVideo(ctx, x, y) }, func(o observed) bool {
		return o.State.Playback.State == playback.Playing && o.State.Playback.Generation == preserved.State.Playback.Generation && o.State.Playback.Position >= narrow.state.State.Playback.Position+200*time.Millisecond && danmakuCurrent(o) && o.Danmaku.Active > 0
	})
	if err != nil {
		return err
	}
	r.pass("danmaku-video-click-play", "ordinary pointer click through the drawn overlay started advancing native playback", &playing)
	paused, err := c.change(ctx, func() error { return c.clickVideo(ctx, x, y) }, danmakuPaused)
	if err != nil {
		return err
	}
	if paused.State.Playback.Generation != preserved.State.Playback.Generation || paused.Resources != preserved.Resources || paused.Danmaku.Settings != preserved.Danmaku.Settings {
		return errors.New("ordinary video clicks replaced playback or changed danmaku settings")
	}
	r.pass("danmaku-video-click-pause", "second ordinary pointer click paused the same native generation through the overlay", &paused)
	if err = c.act(ctx, "e2e-wide"); err != nil {
		return err
	}
	return c.layout(ctx, true)
}
