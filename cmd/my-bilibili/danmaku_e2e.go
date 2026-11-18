//go:build e2e

package main

import (
	danmakuview "my-bilibili/internal/desktop/danmaku"
	"my-bilibili/internal/previewbundle"
)

// Observe the prepared frame without text, native layouts or paint claims.
// Preserve positive ownership counts, including an inconsistent leaked count.
func observeDanmaku(state danmakuview.State) previewbundle.DanmakuObservation {
	observation := previewbundle.DanmakuObservation{
		Generation: state.Generation, Revision: state.Revision, Position: state.Position,
		Width: state.Width, Height: state.Height, Lanes: state.Lanes,
		Settings: state.Settings, Stats: state.Stats, Active: state.Active, Layouts: state.Layouts,
		Items: make([]previewbundle.DanmakuItemObservation, len(state.Items)),
		Error: state.Error, Closed: state.Closed,
	}
	for i, item := range state.Items {
		observation.Items[i] = previewbundle.DanmakuItemObservation(item)
	}
	return observation
}
