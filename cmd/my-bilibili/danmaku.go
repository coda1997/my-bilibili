package main

import (
	"my-bilibili/internal/app"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
)

// Bind before desktop.Render pumps Tick. If Tick accepts a new native attempt,
// the renderer's generation guard hides the old window until the next rebuild.
func (a *application) bindDanmaku(state app.State) {
	if a.danmaku == nil {
		return
	}
	if state.Playback.State == playback.Closed {
		a.danmaku.Close()
		return
	}
	generation := state.Playback.Generation
	if generation == 0 || generation == a.danmakuGeneration {
		return
	}
	if err := a.danmaku.Reset(generation); err != nil {
		// Stale or terminal bindings are retired without changing visible state.
		return
	}
	a.danmakuGeneration = generation
	entries, err := fixture.Danmaku(state.PlaybackPart)
	if err != nil {
		// Reset established this exact live generation. The view publishes the
		// fixed nonblocking Unavailable state; player ownership is unchanged.
		_ = a.danmaku.SetUnavailable(generation)
		return
	}
	// Validation failures are presented by the view's safe error state. They
	// never become player errors or leave the previous generation's comments.
	_ = a.danmaku.SetEntries(generation, entries)
}
