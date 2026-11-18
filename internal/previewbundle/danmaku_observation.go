package previewbundle

import (
	"math"
	"time"

	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
)

// DanmakuObservation describes the last prepared frame, not a paint receipt.
// Its plain fields deliberately exclude text, native layouts and UI types.
type DanmakuObservation struct {
	Generation, Revision uint64
	Position             time.Duration
	Width, Height        float64
	Lanes                int
	Settings             danmaku.Settings
	Stats                danmaku.Stats
	Active, Layouts      int
	Items                []DanmakuItemObservation
	Error                danmaku.ErrorCode
	Closed               bool
}

type DanmakuItemObservation struct {
	Index                         int
	Mode                          danmaku.Mode
	Lane                          int
	Progress, X, Y, Width, Height float64
}

// ParseObservation has already required every field and decoded finite JSON
// numbers. This validates their relationships without changing any counts.
func validDanmakuObservation(d DanmakuObservation, snapshot playback.Snapshot) bool {
	switch d.Error {
	case danmaku.NoError, danmaku.Invalid, danmaku.Stale, danmaku.IsClosed, danmaku.Unavailable:
	default:
		return false
	}
	settings := d.Settings
	if d.Position < 0 || d.Width < 0 || d.Height < 0 ||
		settings.Opacity < 0 || settings.Opacity > 1 ||
		settings.FontSize < 16 || settings.FontSize > 40 ||
		settings.Speed < .5 || settings.Speed > 2 {
		return false
	}
	stats := d.Stats
	if stats.Input < 0 || stats.Input > 64 || stats.Loaded < 0 || stats.Loaded > stats.Input ||
		stats.SkippedEmpty != stats.Input-stats.Loaded || stats.Overflow < 0 || stats.Overflow > stats.Loaded {
		return false
	}
	if d.Layouts < 0 || d.Layouts > danmaku.MaxLanes || d.Active != len(d.Items) {
		return false
	}
	if d.Closed {
		return d.Active == 0 && d.Layouts == 0 && stats == (danmaku.Stats{}) &&
			d.Position == 0 && d.Width == 0 && d.Height == 0 && d.Lanes == 0
	}
	lineHeight := math.Ceil(settings.FontSize*1.25) + 4
	lanes := 0
	if d.Width > 0 {
		lanes = int(min(float64(danmaku.MaxLanes), math.Floor(d.Height/lineHeight)))
	}
	if d.Lanes != lanes {
		return false
	}
	canShow := snapshot.State == playback.Ready || snapshot.State == playback.Paused || snapshot.State == playback.Playing
	if !canShow || d.Generation != snapshot.Generation || !settings.Visible || settings.Opacity == 0 {
		return d.Active == 0
	}
	if d.Active > 0 && d.Position != snapshot.Position {
		return false
	}
	indices, occupied := make(map[int]bool, d.Active), make(map[int]bool, d.Active)
	for _, item := range d.Items {
		if item.Index < 0 || item.Index >= stats.Input || indices[item.Index] || occupied[item.Lane] {
			return false
		}
		if !validDanmakuItem(item, d, lineHeight) {
			return false
		}
		indices[item.Index], occupied[item.Lane] = true, true
	}
	// Matching item counts and distinct eligible lanes imply 0 <= Active <= Lanes.
	return true
}

func validDanmakuItem(item DanmakuItemObservation, d DanmakuObservation, lineHeight float64) bool {
	if item.Progress < 0 || item.Progress >= 1 || item.Width <= 0 || item.Height <= 0 {
		return false
	}
	x := (d.Width - item.Width) / 2
	switch item.Mode {
	case danmaku.ScrollRTL:
		x = d.Width - item.Progress*(d.Width+item.Width)
	case danmaku.ScrollLTR:
		x = -item.Width + item.Progress*(d.Width+item.Width)
	case danmaku.FixedTop, danmaku.FixedBottom:
	default:
		return false
	}
	start, end := 0, d.Lanes
	if d.Lanes == 2 {
		switch item.Mode {
		case danmaku.FixedTop:
			end = 1
		case danmaku.FixedBottom:
			start = 1
		}
	} else if d.Lanes >= 3 {
		fixed := max(1, d.Lanes/4)
		switch item.Mode {
		case danmaku.FixedTop:
			end = fixed
		case danmaku.FixedBottom:
			start = d.Lanes - fixed
		default:
			start, end = fixed, d.Lanes-fixed
		}
	}
	if item.Lane < start || item.Lane >= end {
		return false
	}
	top := float64(item.Lane) * lineHeight
	if d.Lanes == 1 {
		top = (d.Height - lineHeight) / 2
	} else if d.Lanes == 2 && item.Lane == 1 || d.Lanes >= 3 && item.Lane >= d.Lanes-max(1, d.Lanes/4) {
		top = d.Height - float64(d.Lanes-item.Lane)*lineHeight
	}
	return item.X == x && item.Y == top+(lineHeight-item.Height)/2
}
