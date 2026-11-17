package danmakuview

import (
	"fmt"

	gui "github.com/egoist/mygo/ui"
)

// Controls is the content of the parent's popover. It consumes widget input
// before the parent handles shortcuts, and owns no popup or player command.
func (v *View) Controls(c *gui.Context, disabled bool) bool {
	settings := v.Settings()
	disabled = disabled || v.closed
	changed := false
	gui.Column(c).Width(240).Gap(8).Children(func() {
		changed = gui.Checkbox(c.Key("danmaku-visible"), &settings.Visible, "Show danmaku").
			Height(28).Disabled(disabled).Changed() || changed
		gui.Text(c, fmt.Sprintf("Opacity · %.0f%%", settings.Opacity*100)).FontSize(13)
		changed = gui.Slider(c.Key("danmaku-opacity"), &settings.Opacity, 0, 1).Step(.05).
			Width(240).Height(28).Label("Danmaku opacity").Disabled(disabled).Changed() || changed
		gui.Text(c, fmt.Sprintf("Text size · %.0f", settings.FontSize)).FontSize(13)
		changed = gui.Slider(c.Key("danmaku-font-size"), &settings.FontSize, 16, 40).Step(1).
			Width(240).Height(28).Label("Danmaku text size").Disabled(disabled).Changed() || changed
		gui.Text(c, fmt.Sprintf("Scroll speed · %.1f×", settings.Speed)).FontSize(13)
		changed = gui.Slider(c.Key("danmaku-speed"), &settings.Speed, .5, 2).Step(.1).
			Width(240).Height(28).Label("Danmaku speed").Disabled(disabled).Changed() || changed
	})
	if !changed || settings == v.Settings() {
		return false
	}
	return v.Configure(settings) == nil
}
