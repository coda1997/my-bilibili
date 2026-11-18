package main

import (
	"errors"
	"math"
	"time"

	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func danmakuDisposed(o observed) bool {
	d := o.Danmaku
	return d.Closed && d.Error == danmaku.NoError && d.Active == 0 && d.Layouts == 0 && len(d.Items) == 0 && d.Stats == (danmaku.Stats{}) && d.Position == 0 && d.Width == 0 && d.Height == 0 && d.Lanes == 0
}

// The strict observation parser owns validity. These predicates require the
// particular original fixture and a current prepared frame for native checks.
func danmakuCurrent(o observed) bool {
	d, p := o.Danmaku, o.State.Playback
	return !d.Closed && d.Error == danmaku.NoError && d.Generation == p.Generation && d.Position == p.Position && d.Width > 0 && d.Height > 0 && d.Lanes > 0 && d.Stats.Input == 64 && d.Stats.Loaded == 64 && d.Stats.SkippedEmpty == 0
}

func danmakuPaused(o observed) bool {
	return ready(o) && o.State.Playback.State == playback.Paused && !o.State.DesiredPlaying && danmakuCurrent(o)
}

func sameDanmakuWindow(a, b observed) bool {
	x, y := a.Danmaku, b.Danmaku
	return danmakuCurrent(a) && danmakuCurrent(b) && a.State.Playback.Generation == b.State.Playback.Generation && a.State.PlaybackPart == b.State.PlaybackPart && a.Resources == b.Resources && x.Generation == y.Generation && x.Revision == y.Revision && x.Settings == y.Settings && x.Width == y.Width && x.Height == y.Height && x.Lanes == y.Lanes && x.Stats == y.Stats
}

func danmakuLife(mode danmaku.Mode, speed float64) time.Duration {
	life := 4 * time.Second
	if mode == danmaku.ScrollRTL || mode == danmaku.ScrollLTR {
		life = 8 * time.Second
	}
	return time.Duration(float64(life) / speed)
}

func itemMovement(a, b previewbundle.DanmakuItemObservation, width, speed float64, delta time.Duration) bool {
	if a.Index != b.Index || a.Mode != b.Mode || a.Lane != b.Lane || a.Width != b.Width || a.Height != b.Height || a.Y != b.Y {
		return false
	}
	progress := float64(delta) / float64(danmakuLife(a.Mode, speed))
	x := 0.0
	switch a.Mode {
	case danmaku.ScrollRTL:
		x = -progress * (width + a.Width)
	case danmaku.ScrollLTR:
		x = progress * (width + a.Width)
	}
	// Float64 progress and retained shaped widths allow sub-pixel checks. Rate
	// is deliberately absent: the sampled native position already includes it.
	return math.Abs((b.Progress-a.Progress)-progress) < 1e-6 && math.Abs((b.X-a.X)-x) < .01
}

func returnedDanmaku(a, b observed, tolerance time.Duration) error {
	if !danmakuPaused(a) || !danmakuPaused(b) || !sameDanmakuWindow(a, b) || absDuration(b.State.Playback.Position-a.State.Playback.Position) > tolerance || a.Danmaku.Active != b.Danmaku.Active || a.Danmaku.Layouts != b.Danmaku.Layouts || len(a.Danmaku.Items) != len(b.Danmaku.Items) || len(a.Danmaku.Items) == 0 {
		return errors.New("paused danmaku identity or media position changed")
	}
	for i, item := range a.Danmaku.Items {
		if !itemMovement(item, b.Danmaku.Items[i], a.Danmaku.Width, a.Danmaku.Settings.Speed, b.State.Playback.Position-a.State.Playback.Position) {
			return errors.New("paused or seek-return danmaku geometry changed")
		}
	}
	return nil
}

func movingDanmaku(a, b observed, rate float64) error {
	if !sameDanmakuWindow(a, b) || a.State.Playback.State != playback.Playing || b.State.Playback.State != playback.Playing || a.State.Rate != rate || b.State.Rate != rate || a.State.Playback.Rate != rate || b.State.Playback.Rate != rate || b.State.Playback.Position <= a.State.Playback.Position {
		return errors.New("danmaku movement lacks a matching advancing native sample")
	}
	rtl, ltr := false, false
	for _, before := range a.Danmaku.Items {
		if before.Mode != danmaku.ScrollRTL && before.Mode != danmaku.ScrollLTR {
			continue
		}
		for _, after := range b.Danmaku.Items {
			if before.Index != after.Index {
				continue
			}
			if !itemMovement(before, after, a.Danmaku.Width, a.Danmaku.Settings.Speed, b.State.Playback.Position-a.State.Playback.Position) {
				return errors.New("scroll movement does not follow native media position and shaped width once")
			}
			rtl = rtl || before.Mode == danmaku.ScrollRTL
			ltr = ltr || before.Mode == danmaku.ScrollLTR
		}
	}
	if !rtl || !ltr {
		return errors.New("common scrolling items in both directions were not observed")
	}
	return nil
}

func danmakuGeometry(nodes []node, o observed, content rectangle) (rectangle, error) {
	video, err := uniqueNode(nodes, "preview-video")
	if err != nil {
		return rectangle{}, err
	}
	surface, err := uniqueNode(nodes, "danmaku-surface")
	if err != nil {
		return rectangle{}, err
	}
	v, d := video.Bounds, surface.Bounds
	if !fullscreenRectangle(content) || !fullscreenRectangle(v) || !fullscreenRectangle(d) || !containedRectangle(v, rectangle{Width: content.Width, Height: content.Height}) || !containedRectangle(d, v) {
		return rectangle{}, errors.New("danmaku fitted surface geometry unavailable")
	}
	width := math.Min(v.Width, v.Height*16/9)
	height := width * 9 / 16
	want := rectangle{X: v.X + (v.Width-width)/2, Y: v.Y + (v.Height-height)/2, Width: width, Height: height}
	for _, pair := range [][2]float64{{d.X, want.X}, {d.Y, want.Y}, {d.Width, want.Width}, {d.Height, want.Height}, {d.Width, o.Danmaku.Width}, {d.Height, o.Danmaku.Height}} {
		if math.Abs(pair[0]-pair[1]) > .02 {
			return rectangle{}, errors.New("danmaku surface does not match the fitted video clip")
		}
	}
	return d, nil
}

func danmakuControlsBounds(nodes []node, content rectangle) error {
	for _, key := range []string{"danmaku-popup", "danmaku-visible", "danmaku-opacity", "danmaku-font-size", "danmaku-speed"} {
		n, err := uniqueNode(nodes, key)
		if err != nil || !fullscreenRectangle(n.Bounds) || !containedRectangle(n.Bounds, rectangle{Width: content.Width, Height: content.Height}) {
			return errors.New("danmaku popup or control is outside native content bounds")
		}
		for _, state := range n.States {
			if state == "disabled" || state == "offscreen" {
				return errors.New("danmaku control is not usable")
			}
		}
	}
	return nil
}

func popupsClosed(nodes []node) bool {
	for _, n := range nodes {
		if n.Key == "danmaku-popup" || n.Key == "parts-popup" || n.Key == "speed-popup" {
			return false
		}
	}
	return true
}
