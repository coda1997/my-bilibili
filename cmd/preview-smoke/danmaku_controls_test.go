package main

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
)

func widgetSliderClient(t *testing.T, key string, minimum, maximum, step float64) (*client, <-chan float64) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	released := make(chan float64, 1)
	go func() {
		defer b.Close()
		value := minimum
		ui := gui.NewTester(func(c *gui.Context) {
			gui.Slider(c.Key(key), &value, minimum, maximum).Step(step).Width(240).Height(28).Label("Control")
		}, 800, 600)
		scan := bufio.NewScanner(b)
		for scan.Scan() {
			var request struct {
				ID     int
				Method string
				Params struct {
					Events []struct {
						Type string
						X, Y float32
					}
				}
			}
			if json.Unmarshal(scan.Bytes(), &request) != nil {
				return
			}
			var result any = map[string]bool{"ok": true}
			if request.Method == "input" {
				for _, event := range request.Params.Events {
					switch event.Type {
					case "pointerDown":
						ui.Press(event.X, event.Y)
					case "pointerUp":
						ui.Release(event.X, event.Y)
						released <- value
					}
				}
			} else if request.Method == "tree" {
				ui.Frame()
				bounds, ok := ui.Find("Control")
				if !ok {
					return
				}
				result = map[string]any{"nodes": []node{{Key: key, Bounds: rectangle{X: float64(bounds.X), Y: float64(bounds.Y), Width: float64(bounds.W), Height: float64(bounds.H)}, Now: &value}}}
			} else {
				return
			}
			data, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if _, err := b.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}()
	return newClient(a), released
}

func TestDanmakuDriverOperatesActualNonzeroRangeSliders(t *testing.T) {
	for _, tc := range []struct {
		key                       string
		minimum, maximum, step, v float64
	}{
		{"danmaku-font-size", 16, 40, 1, 16},
		{"danmaku-font-size", 16, 40, 1, 32},
		{"danmaku-font-size", 16, 40, 1, 40},
		{"danmaku-speed", .5, 2, .1, .5},
		{"danmaku-speed", .5, 2, .1, 1.5},
		{"danmaku-speed", .5, 2, .1, 2},
	} {
		t.Run(tc.key+strconv.FormatFloat(tc.v, 'f', -1, 64), func(t *testing.T) {
			c, released := widgetSliderClient(t, tc.key, tc.minimum, tc.maximum, tc.step)
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			if err := c.slider(ctx, tc.key, strconv.FormatFloat(tc.v, 'f', -1, 64)); err != nil {
				t.Fatal(err)
			}
			select {
			case value := <-released:
				if math.Abs(value-tc.v) > 1e-6 {
					t.Fatal("real widget did not reach/release requested value", value)
				}
			default:
				t.Fatal("actual slider pointer capture was not released")
			}
		})
	}
}

func TestDanmakuSlidersValidateTargetsBeforePointerInput(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"danmaku-font-size", "15"}, {"danmaku-font-size", "41"},
		{"danmaku-speed", ".4"}, {"danmaku-speed", "2.1"},
		{"danmaku-opacity", "NaN"}, {"danmaku-opacity", "+Inf"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			var calls atomic.Int32
			c := pipeClient(t, func(int, string) any { calls.Add(1); return nil })
			if c.slider(t.Context(), tc.key, tc.value) == nil || calls.Load() != 0 {
				t.Fatal("invalid target reached automation")
			}
		})
	}
}

func TestDanmakuPopupWaitsForVisibleControls(t *testing.T) {
	frames := 0
	c, trace := popupClient(t, func(r popupRequest) map[string]any {
		if r.Method == "tree" {
			frames++
			if frames < 3 {
				return popupTree(popupNode("danmaku-menu"))
			}
			return popupTree(popupNode("danmaku-visible"))
		}
		return popupOK()
	})
	if err := c.openDanmaku(t.Context()); err != nil || frames < 3 || trace.count("act", "danmaku-menu") != 1 {
		t.Fatal("danmaku popup did not wait for actual usable controls", err)
	}
}

func TestDanmakuVideoClickUsesPointerAndAlwaysReleases(t *testing.T) {
	for _, failTree := range []bool{false, true} {
		c, released := sliderFrameClient(t, func(int) any {
			if failTree {
				return map[string]any{"nodes": []node{}}
			}
			return sliderFrame(0)
		}, false)
		err := c.clickVideo(t.Context(), 100, 200)
		if (err != nil) != failTree {
			t.Fatal("click lost its frame error", err)
		}
		if frames := requireSliderRelease(t, released); frames != 1 {
			t.Fatal("click omitted the actual held-pointer frame", frames)
		}
	}
}
