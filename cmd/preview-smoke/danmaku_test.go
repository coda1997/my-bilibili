package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func danmakuSample(t *testing.T, position time.Duration) observed {
	t.Helper()
	o := completeObservation(t)
	o.State.Playback.State, o.State.Playback.Error = playback.Paused, playback.NoError
	o.State.Playback.Position, o.State.Playback.Duration = position, 30*time.Second
	o.Resources = mpv.Resources{Cores: 1, Renderers: 1, Workers: 1}
	o.Tracks = mpv.Tracks{Video: true, Audio: true}
	d := previewbundle.DanmakuObservation{Generation: 1, Revision: 2, Position: position, Width: 640, Height: 360, Lanes: 10, Settings: danmaku.DefaultSettings(), Stats: danmaku.Stats{Input: 64, Loaded: 64, Overflow: 48}, Active: 4, Layouts: 4}
	for _, item := range []struct {
		index, lane int
		mode        danmaku.Mode
		at          time.Duration
		width, y    float64
	}{
		{0, 0, danmaku.FixedTop, 0, 144, 2},
		{1, 2, danmaku.ScrollRTL, time.Second, 168, 70},
		{2, 3, danmaku.ScrollLTR, 1500 * time.Millisecond, 240, 104},
		{3, 9, danmaku.FixedBottom, 1750 * time.Millisecond, 264, 328},
	} {
		seconds := 4.0
		if item.mode == danmaku.ScrollRTL || item.mode == danmaku.ScrollLTR {
			seconds = 8
		}
		progress := (position - item.at).Seconds() / seconds
		x := (640 - item.width) / 2
		if item.mode == danmaku.ScrollRTL {
			x = 640 - progress*(640+item.width)
		} else if item.mode == danmaku.ScrollLTR {
			x = -item.width + progress*(640+item.width)
		}
		d.Items = append(d.Items, previewbundle.DanmakuItemObservation{Index: item.index, Mode: item.mode, Lane: item.lane, Progress: progress, X: x, Y: item.y, Width: item.width, Height: 30})
	}
	o.Danmaku = d
	return o
}

func TestDanmakuDriverRequiresActualClosedOwnership(t *testing.T) {
	for name, change := range map[string]func(*observed){
		"not closed": func(o *observed) { o.Danmaku.Closed = false },
		"active":     func(o *observed) { o.Danmaku.Active = 1 },
		"layouts":    func(o *observed) { o.Danmaku.Layouts = 1 },
		"items": func(o *observed) {
			o.Danmaku.Items = append(o.Danmaku.Items, previewbundle.DanmakuItemObservation{})
		},
		"input":    func(o *observed) { o.Danmaku.Stats.Input = 1 },
		"loaded":   func(o *observed) { o.Danmaku.Stats.Loaded = 1 },
		"empty":    func(o *observed) { o.Danmaku.Stats.SkippedEmpty = 1 },
		"overflow": func(o *observed) { o.Danmaku.Stats.Overflow = 1 },
		"position": func(o *observed) { o.Danmaku.Position = 1 },
		"width":    func(o *observed) { o.Danmaku.Width = 1 },
		"height":   func(o *observed) { o.Danmaku.Height = 1 },
		"lanes":    func(o *observed) { o.Danmaku.Lanes = 1 },
		"error":    func(o *observed) { o.Danmaku.Error = danmaku.Unavailable },
	} {
		t.Run(name, func(t *testing.T) {
			o := completeObservation(t)
			if !disposed(o, playback.Canceled) {
				t.Fatal("zero closed observation rejected")
			}
			change(&o)
			if disposed(o, playback.Canceled) {
				t.Fatal("positive danmaku ownership became successful disposal")
			}
		})
	}
}

func TestDanmakuPausedAndSeekReturnUseNativePosition(t *testing.T) {
	a := danmakuSample(t, 2*time.Second)
	for _, delta := range []time.Duration{0, -80 * time.Millisecond, 100 * time.Millisecond} {
		b := danmakuSample(t, 2*time.Second+delta)
		if err := returnedDanmaku(a, b, 100*time.Millisecond); err != nil {
			t.Fatal(delta, err)
		}
	}
	for name, change := range map[string]func(*observed){
		"too far": func(o *observed) { *o = danmakuSample(t, 2101*time.Millisecond) },
		"not paused": func(o *observed) {
			o.State.Playback.State = playback.Playing
		},
		"generation":      func(o *observed) { o.Danmaku.Generation++ },
		"revision":        func(o *observed) { o.Danmaku.Revision++ },
		"settings":        func(o *observed) { o.Danmaku.Settings.Speed = 2 },
		"position source": func(o *observed) { o.Danmaku.Position++ },
		"lane":            func(o *observed) { o.Danmaku.Items[1].Lane++ },
		"geometry":        func(o *observed) { o.Danmaku.Items[1].X++ },
		"progress":        func(o *observed) { o.Danmaku.Items[1].Progress += .01 },
		"item identity":   func(o *observed) { o.Danmaku.Items[1].Index++ },
		"no items": func(o *observed) {
			o.Danmaku.Items, o.Danmaku.Active, o.Danmaku.Layouts = nil, 0, 0
		},
	} {
		t.Run(name, func(t *testing.T) {
			b := danmakuSample(t, 2*time.Second)
			change(&b)
			if returnedDanmaku(a, b, 100*time.Millisecond) == nil {
				t.Fatal("changed paused/seek-return evidence accepted")
			}
		})
	}
	if returnedDanmaku(a, danmakuSample(t, 2001*time.Millisecond), 0) == nil {
		t.Fatal("paused crop allowed nonzero media-time drift")
	}
}

func TestDanmakuMovementUsesMeasuredWidthAndPositionOnce(t *testing.T) {
	for _, rate := range []float64{1, 2} {
		pair := func() (observed, observed) {
			a, b := danmakuSample(t, 2*time.Second), danmakuSample(t, 2500*time.Millisecond)
			for _, o := range []*observed{&a, &b} {
				o.State.Playback.State, o.State.DesiredPlaying = playback.Playing, true
				o.State.Rate, o.State.Playback.Rate = rate, rate
			}
			return a, b
		}
		a, b := pair()
		if err := movingDanmaku(a, b, rate); err != nil {
			t.Fatal(err)
		}
		for name, change := range map[string]func(observed, *observed){
			"no movement": func(a observed, b *observed) { *b = a },
			"wrong direction": func(a observed, b *observed) {
				b.Danmaku.Items[1].X = a.Danmaku.Items[1].X + 50.5
			},
			"without shaped width": func(a observed, b *observed) {
				b.Danmaku.Items[1].X = a.Danmaku.Items[1].X - .5/8*640
			},
			"rate applied twice": func(a observed, b *observed) {
				for i := 1; i <= 2; i++ {
					b.Danmaku.Items[i].Progress = a.Danmaku.Items[i].Progress + .5/8*2
					b.Danmaku.Items[i].X = a.Danmaku.Items[i].X + (b.Danmaku.Items[i].X-a.Danmaku.Items[i].X)*2
				}
			},
			"stale position": func(_ observed, b *observed) { b.Danmaku.Position-- },
			"wrong native rate": func(_ observed, b *observed) {
				b.State.Playback.Rate = rate + .5
			},
			"missing other direction": func(_ observed, b *observed) {
				b.Danmaku.Items = append(b.Danmaku.Items[:2:2], b.Danmaku.Items[3:]...)
				b.Danmaku.Active, b.Danmaku.Layouts = 3, 3
			},
		} {
			t.Run(name+strconvRate(rate), func(t *testing.T) {
				a, b := pair()
				change(a, &b)
				if movingDanmaku(a, b, rate) == nil {
					t.Fatal("invalid scroll evidence accepted")
				}
			})
		}
	}
}

func strconvRate(rate float64) string {
	if rate == 1 {
		return "-rate1"
	}
	return "-rate2"
}

func TestDanmakuSettingsNeedMeasuredEffect(t *testing.T) {
	a, b := danmakuSample(t, 2*time.Second), danmakuSample(t, 2*time.Second)
	b.Danmaku.Settings.FontSize = 32
	if largerDanmakuText(a, b) {
		t.Fatal("font setting alone became glyph-size evidence")
	}
	b.Danmaku.Items[0].Width, b.Danmaku.Items[0].Height = 192, 40
	if !largerDanmakuText(a, b) {
		t.Fatal("increased measured native text rejected")
	}
	b = danmakuSample(t, 2*time.Second)
	b.Danmaku.Settings.Speed = 1.5
	if changedDanmakuSpeed(a, b) {
		t.Fatal("speed setting alone became recomputed progress evidence")
	}
	p := float64(time.Second) / 5333333333
	b.Danmaku.Items[1].Progress = p
	b.Danmaku.Items[1].X = 640 - p*(640+168)
	if !changedDanmakuSpeed(a, b) {
		t.Fatal("exact truncated-lifetime speed progression rejected")
	}
	b.Danmaku.Items[1].X++
	if changedDanmakuSpeed(a, b) {
		t.Fatal("wrong speed-dependent shaped travel accepted")
	}
}

func TestDanmakuStageIsAdmittedWithoutWeakeningIdentity(t *testing.T) {
	err := command([]string{"-stage", "danmaku", "-app", "/absent-synthetic-preview-app", "-output", "/absent-synthetic-preview-output", "-app-sha256", strings.Repeat("0", 64), "-driver-sha256", strings.Repeat("0", 64)}, io.Discard, io.Discard)
	if err == nil || strings.Contains(err.Error(), "stage must") {
		t.Fatal("danmaku stage missing or unpinned native invocation accepted", err)
	}
}

func TestDanmakuSnapshotMatchesStrictObservation(t *testing.T) {
	o := danmakuSample(t, 2*time.Second)
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := previewbundle.ParseObservation(data); err != nil {
		t.Fatal("test fixture is not a valid strict observation", err)
	}
}

func TestDanmakuSeekWaitsForTwoStablePreparedFrames(t *testing.T) {
	o := danmakuSample(t, 2*time.Second)
	var trees int
	c := pipeClient(t, func(_ int, method string) any {
		if method == "tree" {
			trees++
			data, _ := json.Marshal(o)
			return map[string]any{"nodes": []node{{Key: "e2e-observation", Label: string(data)}, {Key: "seek-slider", Bounds: rectangle{Width: 768, Height: 28}, Now: floatPointer(2)}}}
		}
		return map[string]bool{"ok": true}
	})
	start := time.Now()
	got, err := c.seekDanmaku(context.Background(), 2*time.Second)
	if err != nil || got.State.Playback.Position != 2*time.Second || trees < 4 || time.Since(start) < 100*time.Millisecond {
		t.Fatal("seek skipped actual pointer/release or stable-frame interval", err, trees)
	}
}

func TestDanmakuDenseEvidenceRequiresOverflowAndCurrentBoundedLayouts(t *testing.T) {
	for name, change := range map[string]func(*observed){
		"no overflow": func(o *observed) { o.Danmaku.Stats.Overflow = 0 },
		"no active":   func(o *observed) { o.Danmaku.Active = 0 },
		"too many": func(o *observed) {
			o.Danmaku.Active, o.Danmaku.Layouts = 17, 17
		},
		"missing layouts": func(o *observed) { o.Danmaku.Layouts = 0 },
		"old layouts":     func(o *observed) { o.Danmaku.Layouts = 5 },
		"stale":           func(o *observed) { o.Danmaku.Generation = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			o := danmakuSample(t, 2*time.Second)
			if !denseDanmaku(o) {
				t.Fatal("current bounded dense evidence rejected")
			}
			change(&o)
			if denseDanmaku(o) {
				t.Fatal("absent, unbounded or stale dense evidence accepted")
			}
		})
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestDanmakuCurrentRejectsPendingOrUnavailableFrame(t *testing.T) {
	for name, change := range map[string]func(*observed){
		"closed":     func(o *observed) { o.Danmaku.Closed = true },
		"error":      func(o *observed) { o.Danmaku.Error = danmaku.Invalid },
		"generation": func(o *observed) { o.Danmaku.Generation-- },
		"time":       func(o *observed) { o.Danmaku.Position++ },
		"width":      func(o *observed) { o.Danmaku.Width = 0 },
		"height":     func(o *observed) { o.Danmaku.Height = 0 },
		"lanes":      func(o *observed) { o.Danmaku.Lanes = 0 },
		"input":      func(o *observed) { o.Danmaku.Stats.Input-- },
		"loaded":     func(o *observed) { o.Danmaku.Stats.Loaded-- },
		"skipped":    func(o *observed) { o.Danmaku.Stats.SkippedEmpty++ },
	} {
		t.Run(name, func(t *testing.T) {
			o := danmakuSample(t, 2*time.Second)
			if !danmakuCurrent(o) {
				t.Fatal("current fixture frame rejected")
			}
			change(&o)
			if danmakuCurrent(o) {
				t.Fatal("pending or unavailable prepared frame accepted")
			}
		})
	}
	if danmakuLife(danmaku.FixedTop, 1.5) != 2666666666 {
		t.Fatal("fixed lifespan lost nanosecond truncation")
	}
}
