package mpvprobe

import (
	"time"

	"my-bilibili/experiments/playback/contract"
)

// stopVideo runs on the control owner while the render owner remains active.
// With force-window=no, the pinned mpv's vid=no path uninitializes the video
// chain and destroys/joins its VO before returning the command reply. It also
// disables video selection if load has not finished yet. No more playback
// commands are sent after this barrier. Render-context free can then run without
// stranding a VO waiting for its last frame (CA-CLEANUP-002).
func (a *nativeAPI) stopVideo(core uintptr) bool {
	// Playback requests start at 1; reserve 0 for this shutdown-only barrier.
	if a.send(core, 0, []string{"set", "vid", "no"}) < 0 {
		return false
	}
	// Reserve part of the 200ms Close budget for the remaining destructors.
	// Failure never means ownership is released: the caller still runs every
	// destructor and joins the workers, even if native shutdown exceeds budget.
	deadline := time.Now().Add(2 * contract.PollInterval)
	for events := 0; events < 256; {
		left := time.Until(deadline)
		if left <= 0 {
			return false
		}
		p := a.wait(core, min(left, contract.PollInterval).Seconds())
		if p == nil {
			return false
		}
		ev := *(*event)(p)
		if ev.ID == 0 {
			continue
		}
		events++
		if ev.ID == 5 && ev.Reply == 0 {
			return ev.Error >= 0
		}
		if ev.ID == 1 || ev.ID == 24 { // shutdown or event loss
			return false
		}
		// Late playback events cannot publish state or reopen a closing session.
	}
	return false
}
