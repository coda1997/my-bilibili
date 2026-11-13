package main

import (
	"math"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
)

// Verify the driver's key oracle against the pinned public widget, independently
// of mpv and automation transport. PageUp means a tenth of the range, snapped
// to the step, rather than ten steps. The native run preserves this regression.
func TestSeekSliderPlanMatchesPinnedWidget(t *testing.T) {
	value := 3.0
	tt := gui.NewTester(func(c *gui.Context) { gui.Slider(c, &value, 0, 8).Step(.25).Label("probe seek").Width(640) }, 760, 200)
	if err := tt.Click("probe seek"); err != nil {
		t.Fatal(err)
	}
	value = 3
	tt.Frame()
	keys, targets := seekSliderPlan()
	if len(keys) != len(targets) {
		t.Fatal("incomplete oracle")
	}
	for i, key := range keys {
		k, ok := map[string]gui.Key{"Home": gui.KeyHome, "PageUp": gui.KeyPageUp, "Right": gui.KeyRight}[key]
		if !ok {
			t.Fatal("unknown key")
		}
		tt.Key(0, k)
		if math.Abs(value-targets[i].Seconds()) > 1e-6 {
			t.Fatalf("key %d %s produced %.2fs; oracle expects %s", i, key, value, targets[i])
		}
	}
	if value != 4 || targets[len(targets)-1] != 4*time.Second {
		t.Fatal("slider plan does not reach original native target")
	}
}
