package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	gui "github.com/egoist/mygo/ui"
	"math"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDriverCoordinatesOperatePinnedNativeSliders(t *testing.T) {
	for _, tc := range []struct{ width, maximum, target, step float64 }{{80, 1, .35, .05}, {768, 30, 5, .1}, {768, 30, 29, .1}} {
		value := 0.0
		ui := gui.NewTester(func(c *gui.Context) {
			gui.Slider(c.Key("slider"), &value, 0, tc.maximum).Step(tc.step).Width(float32(tc.width)).Height(28).Label("Control")
		}, 1120, 760)
		bounds, ok := ui.Find("Control")
		if !ok {
			t.Fatal("slider missing")
		}
		x, y := sliderCoordinates(rectangle{X: float64(bounds.X), Y: float64(bounds.Y), Width: float64(bounds.W), Height: float64(bounds.H)}, tc.target/tc.maximum)
		ui.Press(float32(x), float32(y))
		ui.Frame()
		ui.Release(float32(x), float32(y))
		if math.Abs(value-tc.target) > .001 {
			t.Fatalf("width=%g requested=%g observed=%g", tc.width, tc.target, value)
		}
	}
}

func sliderFrameClient(t *testing.T, frame func(int) any, releaseError bool) (*client, <-chan int) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	released := make(chan int, 1)
	go func() {
		defer b.Close()
		down, frames := false, 0
		s := bufio.NewScanner(b)
		for s.Scan() {
			var request struct {
				ID     int
				Method string
				Params struct{ Events []struct{ Type string } }
			}
			if json.Unmarshal(s.Bytes(), &request) != nil {
				return
			}
			response := map[string]any{"id": request.ID, "result": map[string]bool{"ok": true}}
			if request.Method == "tree" {
				response["result"] = sliderFrame(0)
				if down {
					frames++
					response["result"] = frame(frames)
				}
			}
			for _, event := range request.Params.Events {
				switch event.Type {
				case "pointerDown":
					down = true
				case "pointerUp":
					released <- frames
					if releaseError {
						response["error"] = "release rejected"
					}
				}
			}
			data, _ := json.Marshal(response)
			if _, err := b.Write(append(data, '\n')); err != nil {
				return
			}
		}
	}()
	return newClient(a), released
}

func sliderFrame(now any) any {
	n := map[string]any{"key": "volume-slider", "bounds": rectangle{Width: 80, Height: 28}, "now": now}
	return map[string]any{"nodes": []any{n}}
}

func requireSliderRelease(t *testing.T, released <-chan int) int {
	t.Helper()
	select {
	case frames := <-released:
		return frames
	default:
		t.Fatal("pointer was not released after successful down")
		return 0
	}
}

func TestSliderWaitsForObservedTargetBeforeRelease(t *testing.T) {
	c, released := sliderFrameClient(t, func(frame int) any {
		if frame < 3 {
			return sliderFrame(0)
		}
		return sliderFrame(.35)
	}, false)
	if err := c.slider(context.Background(), "volume-slider", ".35"); err != nil {
		t.Fatal(err)
	}
	if frames := requireSliderRelease(t, released); frames < 3 {
		t.Fatalf("released after %d stale frames before target was observed", frames)
	}
}

func TestSliderRejectsInvalidObservedValueAndReleases(t *testing.T) {
	for _, value := range []any{nil, "0", true} {
		t.Run(fmtValue(value), func(t *testing.T) {
			c, released := sliderFrameClient(t, func(int) any { return sliderFrame(value) }, false)
			if err := c.slider(context.Background(), "volume-slider", "0"); err == nil {
				t.Fatal("invalid observed slider value accepted as target zero")
			}
			requireSliderRelease(t, released)
		})
	}
	t.Run("missing", func(t *testing.T) {
		c, released := sliderFrameClient(t, func(int) any {
			return map[string]any{"nodes": []any{map[string]any{"key": "volume-slider"}}}
		}, false)
		if err := c.slider(context.Background(), "volume-slider", "0"); err == nil {
			t.Fatal("missing observed slider value accepted as target zero")
		}
		requireSliderRelease(t, released)
	})
}

func fmtValue(value any) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func TestSliderTimeoutReleasesAndPreservesFailure(t *testing.T) {
	c, released := sliderFrameClient(t, func(int) any { return sliderFrame(0) }, false)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := c.slider(ctx, "volume-slider", ".35"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected target wait deadline, got %v", err)
	}
	requireSliderRelease(t, released)
}

func TestSliderPreservesObservationAndReleaseFailures(t *testing.T) {
	c, released := sliderFrameClient(t, func(int) any {
		return map[string]any{"nodes": []any{}}
	}, true)
	err := c.slider(context.Background(), "volume-slider", ".35")
	if err == nil || !strings.Contains(err.Error(), "invalid preview tree") || !strings.Contains(err.Error(), "automation input response invalid") {
		t.Fatalf("lost observation or release failure: %v", err)
	}
	requireSliderRelease(t, released)
}
