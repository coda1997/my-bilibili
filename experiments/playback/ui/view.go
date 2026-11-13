// Package ui is the small M1 experiment view, not the application shell.
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

type Source struct {
	Key, Label string
	Input      contract.Input
}

const ProvisionalWarning = "TEMPORARY MYGO PLAYER / slow loading may freeze the UI; headers and separate audio unsupported"

type sliderState struct {
	value      float64
	generation uint64
	pressed    bool
}

func (s *sliderState) sample(generation uint64, value float64) {
	// Retain the interaction value only during a pointer gesture. Replacing it
	// with a lagging native sample would resend the same seek on every frame.
	if !s.pressed || generation != s.generation {
		s.value = value
	}
	s.generation = generation
}

type View struct {
	Controller *contract.Controller
	Sources    []Source
	Surface    func(*gui.Context, contract.Engine)
	Quit       func()
	Text       string
	Warning    string
	lastError  contract.ErrorCode
	cancel     context.CancelFunc
	lastInput  *contract.Input
	seekUI     sliderState
	volumeUI   sliderState
}

func (v *View) open(in contract.Input) {
	if err := in.Validate(); err != nil {
		v.lastError = contract.Invalid
		return
	}
	saved := in
	saved.Headers = maps.Clone(in.Headers)
	v.lastInput = &saved
	if v.cancel != nil {
		v.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	v.lastError = ""
	if err := v.Controller.Open(ctx, in); err != nil {
		v.lastError = err.(contract.ErrorCode)
	}
}

func (v *View) retry() {
	if v.lastInput != nil && v.Controller.Snapshot().State != contract.Loading {
		v.open(*v.lastInput)
	}
}

func (v *View) command(c *gui.Context, cmd contract.Command) {
	v.lastError = ""
	if err := v.Controller.Command(cmd); err != nil {
		v.lastError = err.(contract.ErrorCode)
	}
	c.Invalidate()
}

func (v *View) transport(c *gui.Context) {
	s := v.Controller.Snapshot()
	usable := v.Controller.Active() && (s.State == contract.Ready || s.State == contract.Paused || s.State == contract.Playing || s.State == contract.Ended)
	gui.Text(c, fmt.Sprintf("Position %.2fs / %.2fs (sampled) / %s", s.Position.Seconds(), s.Duration.Seconds(), s.State)).Font("Menlo").FontSize(12)
	seekDisabled := !usable || s.Duration <= 0
	maximum := s.Duration.Seconds()
	if maximum <= 0 {
		maximum = 1
	}
	v.seekUI.sample(s.Generation, max(0, min(s.Position.Seconds(), maximum)))
	// Frame rebuilds read native state; only actual interactions send commands.
	seek := gui.Slider(c.Key("seek-slider"), &v.seekUI.value, 0, maximum).Step(.25).Label("Seek position (seconds)").Width(640).Disabled(seekDisabled)
	if seek.Changed() && !seekDisabled && !math.IsNaN(v.seekUI.value) && !math.IsInf(v.seekUI.value, 0) {
		position := max(0, min(v.seekUI.value, maximum))
		target := s.Duration
		if position < maximum {
			target = time.Duration(position * float64(time.Second))
		}
		v.command(c, contract.Command{Kind: contract.Seek, Position: target})
	}
	// Pressed is bounds-limited; slider capture continues outside those bounds.
	button, captured := seek.PressedButton()
	v.seekUI.pressed = captured && button == 0 && !seekDisabled
	v.volumeUI.sample(s.Generation, s.Volume)
	gui.Row(c).Gap(12).Children(func() {
		gui.Text(c, fmt.Sprintf("Volume %.0f%% / Speed %.1fx", s.Volume*100, s.Rate)).Font("Menlo").FontSize(12)
		volume := gui.Slider(c.Key("volume-slider"), &v.volumeUI.value, 0, 1).Label("Playback volume").Width(300).Disabled(!usable)
		if volume.Changed() && usable {
			v.command(c, contract.Command{Kind: contract.Volume, Value: v.volumeUI.value})
		}
		button, captured := volume.PressedButton()
		v.volumeUI.pressed = captured && button == 0 && usable
	})
	gui.Row(c).Gap(7).Children(func() {
		for _, preset := range []struct {
			key, label string
			rate       float64
		}{{"speed-half", "0.5x", .5}, {"speed-normal", "1x", 1}, {"speed-double", "2x", 2}} {
			if gui.Button(c.Key(preset.key), preset.label).Disabled(!usable).Clicked() && usable {
				v.command(c, contract.Command{Kind: contract.Rate, Value: preset.rate})
			}
		}
		if gui.Button(c.Key("retry"), "Retry last input").Disabled(v.lastInput == nil || s.State == contract.Loading).Clicked() {
			v.retry()
			c.Invalidate()
		}
	})
}

func (v *View) Close() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
	_ = v.Controller.Close()
}

func (v *View) Render(c *gui.Context) {
	v.Controller.Tick()
	if v.Controller.Active() {
		c.After(contract.PollInterval)
	}
	ink := gui.RGB(35, 44, 49)
	gui.Scroll(c.Key("lab-scroll")).Fill().Padding(22).Gap(10).Background(gui.RGB(245, 241, 231)).Children(func() {
		gui.Text(c, "M1 / NATIVE PLAYBACK LAB").Font("Menlo").FontSize(13).TextColor(ink)
		gui.Text(c, "A small window. Real evidence.").Font("Georgia").FontSize(27).TextColor(ink)
		warning := v.Warning
		if warning == "" {
			warning = ProvisionalWarning
		}
		gui.Text(c, warning).FontSize(12).TextColor(gui.RGB(125, 57, 24)).Width(640)
		gui.TextInput(c.Key("chinese"), &v.Text).Label("Chinese input (injected, not IME)").Width(600)
		gui.Box(c).Width(640).Height(360).Background(gui.RGB(24, 30, 34)).Children(func() {
			if v.Surface != nil && v.Controller.Engine() != nil {
				v.Surface(c, v.Controller.Engine())
			}
		})
		for i := 0; i < len(v.Sources); i += 4 {
			gui.Row(c).Gap(7).Children(func() {
				for _, source := range v.Sources[i:min(i+4, len(v.Sources))] {
					if gui.Button(c.Key(source.Key), source.Label).Clicked() {
						v.open(source.Input)
						c.Invalidate()
					}
				}
			})
		}
		v.transport(c)
		gui.Row(c).Gap(7).Children(func() {
			for _, b := range []struct {
				key, label string
				cmd        contract.Command
			}{
				{"play", "Play", contract.Command{Kind: contract.Play}},
				{"pause", "Pause", contract.Command{Kind: contract.Pause}},
				{"seek", "Seek 3s", contract.Command{Kind: contract.Seek, Position: 3 * time.Second}},
				{"volume", "Volume 25%", contract.Command{Kind: contract.Volume, Value: .25}},
				{"rate", "Rate 1.5x", contract.Command{Kind: contract.Rate, Value: 1.5}},
			} {
				if gui.Button(c.Key(b.key), b.label).Clicked() {
					v.command(c, b.cmd)
				}
			}
		})
		gui.Row(c).Gap(7).Children(func() {
			if gui.Button(c.Key("cancel"), "Cancel").Clicked() {
				if v.cancel != nil {
					v.cancel()
				}
				v.Controller.Tick()
				c.Invalidate()
			}
			if gui.Button(c.Key("close"), "Close media").Clicked() {
				v.Close()
				c.Invalidate()
			}
			if gui.Button(c.Key("quit"), "Quit lab").Clicked() {
				v.Close()
				if v.Quit != nil {
					v.Quit()
				}
			}
		})
		s := v.Controller.Snapshot()
		status, _ := json.Marshal(struct {
			contract.Snapshot
			Owned        bool               `json:"owned"`
			ControlError contract.ErrorCode `json:"control_error"`
			WindowHidden bool               `json:"window_hidden"`
		}{s, v.Controller.Active(), v.lastError, c.WindowHidden()})
		// This observable text is native sampled state, not a Play-command echo.
		gui.Text(c, "snapshot "+string(status)).Font("Menlo").FontSize(10).TextColor(ink)
		gui.Text(c, "Synthetic fixtures only / Windows and real service access not tested").FontSize(12).TextColor(ink)
	})
}
