package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDanmakuGeometryUsesFittedVideoNotLetterbox(t *testing.T) {
	o := danmakuSample(t, 2*time.Second)
	content := rectangle{X: 300, Y: 80, Width: 1000, Height: 800, complete: true}
	video := rectangle{Width: 1000, Height: 800, complete: true}
	surface := rectangle{Y: 118.75, Width: 1000, Height: 562.5, complete: true}
	o.Danmaku.Width, o.Danmaku.Height = surface.Width, surface.Height
	good := []node{{Key: "preview-video", Bounds: video}, {Key: "danmaku-surface", Bounds: surface}}
	if got, err := danmakuGeometry(good, o, content); err != nil || !sameRectangle(got, surface) {
		t.Fatal("correct fitted fullscreen surface rejected", got, err)
	}
	for name, change := range map[string]func([]node, *observed){
		"whole letterbox": func(nodes []node, _ *observed) { nodes[1].Bounds = video },
		"wrong origin":    func(nodes []node, _ *observed) { nodes[1].Bounds.Y = 0 },
		"outside video":   func(nodes []node, _ *observed) { nodes[1].Bounds.X = -.01 },
		"outside native content": func(nodes []node, _ *observed) {
			nodes[0].Bounds.Width = 1001
		},
		"missing fields": func(nodes []node, _ *observed) { nodes[1].Bounds.complete = false },
		"zero width":     func(nodes []node, _ *observed) { nodes[1].Bounds.Width = 0 },
		"sampled width mismatch": func(_ []node, o *observed) {
			o.Danmaku.Width = 999
		},
		"sampled height mismatch": func(_ []node, o *observed) {
			o.Danmaku.Height = 563
		},
	} {
		t.Run(name, func(t *testing.T) {
			nodes := append([]node{}, good...)
			sample := o
			change(nodes, &sample)
			if _, err := danmakuGeometry(nodes, sample, content); err == nil {
				t.Fatal("invalid fitted clip accepted")
			}
		})
	}
	if _, err := danmakuGeometry(append(good, good[1]), o, content); err == nil {
		t.Fatal("duplicate overlay surface accepted")
	}
}

func TestDanmakuPopupEscapePreservesNativeFullscreenCompletion(t *testing.T) {
	before := danmakuSample(t, 2*time.Second)
	expected := fullscreenCompletion{requests: 1, entered: 1}
	for _, mode := range []string{"valid", "exit", "request", "popup", "settings", "not full"} {
		t.Run(mode, func(t *testing.T) {
			o := danmakuSample(t, 2*time.Second)
			o.Fullscreen = true
			trace := nativeFullscreenTrace{SchemaVersion: 1, Sequence: 2, Requests: 1, DidEnter: 1, Events: []nativeFullscreenEvent{}}
			switch mode {
			case "exit":
				trace.DidExit, trace.Sequence = 1, 3
			case "request":
				trace.Requests, trace.Sequence = 2, 3
			case "settings":
				o.Danmaku.Settings.Opacity = .5
			case "not full":
				o.Fullscreen = false
			}
			state, _ := json.Marshal(o)
			completion, _ := json.Marshal(trace)
			nodes := []node{{Key: "e2e-observation", Label: string(state)}, {Key: "e2e-fullscreen", Label: string(completion)}}
			if mode == "popup" {
				nodes = append(nodes, node{Key: "danmaku-popup"})
			}
			if _, err := danmakuPopupDismissed(nodes, before, expected); (err == nil) != (mode == "valid") {
				t.Fatal("popup Escape accepted a native exit or stale popup/settings", err)
			}
		})
	}
}

func TestDanmakuMinimumPopupControlsFitNativeContent(t *testing.T) {
	content := rectangle{Width: 800, Height: 600, complete: true}
	var good []node
	for _, key := range []string{"danmaku-popup", "danmaku-visible", "danmaku-opacity", "danmaku-font-size", "danmaku-speed"} {
		good = append(good, node{Key: key, Bounds: rectangle{X: 400, Y: 200, Width: 260, Height: 260, complete: true}})
	}
	if err := danmakuControlsBounds(good, content); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func([]node){
		"outside right":  func(nodes []node) { nodes[2].Bounds.X = 600 },
		"outside bottom": func(nodes []node) { nodes[3].Bounds.Y = 500 },
		"missing field":  func(nodes []node) { nodes[2].Bounds.complete = false },
		"disabled":       func(nodes []node) { nodes[2].States = []string{"disabled"} },
		"offscreen":      func(nodes []node) { nodes[2].States = []string{"offscreen"} },
		"absent":         func(nodes []node) { nodes[2].Key = "other" },
		"duplicate":      func(nodes []node) { nodes[3].Key = nodes[2].Key },
	} {
		t.Run(name, func(t *testing.T) {
			nodes := append([]node{}, good...)
			change(nodes)
			if danmakuControlsBounds(nodes, content) == nil {
				t.Fatal("unusable or clipped control accepted")
			}
		})
	}
}

func TestDanmakuCapturesRejectEveryOpenPopover(t *testing.T) {
	for _, key := range []string{"danmaku-popup", "parts-popup", "speed-popup"} {
		if popupsClosed([]node{{Key: key}}) {
			t.Fatal("open popup accepted for capture", key)
		}
	}
	if !popupsClosed([]node{{Key: "danmaku-menu"}, {Key: "play-toggle"}}) {
		t.Fatal("closed popup controls rejected")
	}
}

func TestDanmakuResizeKeepsGenerationOwnershipAndSettings(t *testing.T) {
	a, b := danmakuSample(t, 2*time.Second), danmakuSample(t, 2*time.Second)
	b.Danmaku.Width, b.Danmaku.Height = 1000, 562.5
	if !retainedDanmaku(a, b) {
		t.Fatal("resized current frame rejected")
	}
	for name, change := range map[string]func(*observed){
		"native generation":  func(o *observed) { o.State.Playback.Generation++ },
		"overlay generation": func(o *observed) { o.Danmaku.Generation++ },
		"ownership":          func(o *observed) { o.Resources.Cores++ },
		"visibility":         func(o *observed) { o.Danmaku.Settings.Visible = false },
		"opacity":            func(o *observed) { o.Danmaku.Settings.Opacity = 0 },
		"font":               func(o *observed) { o.Danmaku.Settings.FontSize++ },
		"speed":              func(o *observed) { o.Danmaku.Settings.Speed++ },
	} {
		t.Run(name, func(t *testing.T) {
			o := b
			change(&o)
			if retainedDanmaku(a, o) {
				t.Fatal("resize/fullscreen reset the session or settings")
			}
		})
	}
}
