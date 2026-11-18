package danmakuview

import (
	"math"
	"testing"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/danmaku"
)

func TestControlsPointerFocusedKeysAndDisabledState(t *testing.T) {
	v := New()
	disabled, changes, shortcuts := false, 0, 0
	ui := gui.NewTester(func(c *gui.Context) {
		gui.Box(c).Fill().Padding(12).Children(func() {
			if v.Controls(c, disabled) {
				changes++
			}
		})
		if c.Shortcut(0, gui.KeySpace) || c.Shortcut(0, gui.KeyRight) || c.Shortcut(0, gui.KeyHome) || c.Shortcut(0, gui.KeyEnd) {
			shortcuts++
		}
	}, 280, 320)
	click := func(label string) {
		t.Helper()
		if err := ui.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if changes != 0 {
		t.Fatal("building controls reports an unrequested setting change")
	}
	click("Show danmaku")
	if v.Settings().Visible || changes != 1 || !ui.Focused("Show danmaku") {
		t.Fatal("checkbox click did not change the setting and retain focus")
	}
	ui.Key(0, gui.KeySpace)
	if !v.Settings().Visible || changes != 2 || shortcuts != 0 {
		t.Fatal("focused checkbox did not take Space before global shortcuts")
	}
	for _, slider := range []struct {
		label          string
		min, max, step float64
		value          func(danmaku.Settings) float64
	}{
		{"Danmaku opacity", 0, 1, .05, func(s danmaku.Settings) float64 { return s.Opacity }},
		{"Danmaku text size", 16, 40, 1, func(s danmaku.Settings) float64 { return s.FontSize }},
		{"Danmaku speed", .5, 2, .1, func(s danmaku.Settings) float64 { return s.Speed }},
	} {
		a, ok := ui.Access(slider.label)
		if !ok || a.Role != gui.RoleSlider || !a.Focusable || a.Disabled || a.Bounds.W != 240 || a.Bounds.H < 28 || a.Bounds.X+a.Bounds.W > 280 || a.Bounds.Y+a.Bounds.H > 320 {
			t.Fatalf("slider not usable in its popup bounds: %+v", a)
		}
		before := changes
		click(slider.label)
		if !ui.Focused(slider.label) || slider.value(v.Settings()) <= slider.min || slider.value(v.Settings()) >= slider.max || changes <= before {
			t.Fatalf("slider %s did not bind pointer input", slider.label)
		}
		ui.Key(0, gui.KeyHome)
		if value := slider.value(v.Settings()); value != slider.min {
			t.Fatalf("%s Home = %v", slider.label, value)
		}
		ui.Key(0, gui.KeyRight)
		if value := slider.value(v.Settings()); math.Abs(value-(slider.min+slider.step)) > 1e-10 {
			t.Fatalf("%s Right = %v, want step %v", slider.label, value, slider.step)
		}
		ui.Key(0, gui.KeyEnd)
		if value := slider.value(v.Settings()); value != slider.max || shortcuts != 0 {
			t.Fatalf("%s End = %v; global shortcuts = %d", slider.label, value, shortcuts)
		}
	}
	before := changes
	ui.Frame()
	if changes != before {
		t.Fatal("unchanged controls report a new change")
	}
	disabled = true
	ui.Frame()
	settings := v.Settings()
	labels := []string{"Show danmaku", "Danmaku opacity", "Danmaku text size", "Danmaku speed"}
	for _, label := range labels {
		if a, ok := ui.Access(label); !ok || !a.Disabled {
			t.Fatalf("disabled control missing or enabled: %s", label)
		}
		click(label)
		ui.Key(0, gui.KeyHome)
		ui.Key(0, gui.KeyRight)
		if v.Settings() != settings || changes != before {
			t.Fatalf("disabled %s changed settings", label)
		}
	}
	v.Close()
	disabled = false
	ui.Frame()
	for _, label := range labels {
		if a, ok := ui.Access(label); !ok || !a.Disabled {
			t.Fatalf("closed view exposes active control: %s", label)
		}
	}
}
