package desktop

import (
	"fmt"
	"strconv"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/playback"
)

var (
	ink    = gui.RGB(28, 36, 47)
	muted  = gui.RGB(106, 115, 128)
	accent = gui.RGB(251, 114, 153)
)

// Render pumps UI-thread events through Tick, which can accept source results
// and open media. Outside that pump and explicit user actions, sampling State
// and rebuilding the layout do not issue player commands or initiate loading.
func (v *View) Render(c *gui.Context) {
	v.Player.Tick()
	s := v.Player.State()
	if !partsReady(s) {
		v.partsOpen = false
	}
	if !usable(s) {
		v.speedOpen = false
	}
	if v.Danmaku == nil || closed(s) {
		v.danmakuOpen = false
	}
	if v.lastActivity.IsZero() {
		v.active()
	}
	if s.Playback.Generation != v.generation {
		v.generation = s.Playback.Generation
		v.seek, v.volume = sliderState{}, sliderState{}
		v.pendingClick = time.Time{}
		v.suppressClick = false
		v.active()
	}
	if s.Closing || s.Playback.State != playback.Closed {
		c.After(playback.PollInterval)
	}
	w, h := c.Size()
	fullscreen := v.Fullscreen()
	if fullscreen {
		v.player(c, s, w, h, true)
	} else {
		content := max(float32(1), w-48)
		wide := content >= 1000
		playerWidth := content
		if wide {
			playerWidth -= 280
		}
		gui.Scroll(c.Key("preview-page")).Fill().Padding(24).Gap(18).Background(gui.RGB(246, 247, 250)).Children(func() {
			gui.Row(c).Gap(12).AlignItems(gui.Center).Children(func() {
				gui.Box(c.Key("preview-cover")).Size(80, 45).Background(gui.RGB(225, 229, 236)).Role(gui.RoleImage).Label("Cover for " + s.Video.Title).Children(func() {
					if v.Cover != nil && s.DetailPhase == app.DetailReady {
						v.Cover(c, s.Video.CoverID)
					}
				})
				gui.Column(c).Gap(4).Children(func() {
					gui.Text(c, "LOCAL PREVIEW").FontSize(11).TextColor(accent)
					gui.Text(c.Key("video-title"), s.Video.Title).FontSize(23).TextColor(ink)
					gui.Text(c, s.Video.Creator).FontSize(13).TextColor(muted)
				})
			})
			if wide {
				gui.Row(c.Key("wide-layout")).Gap(20).AlignItems(gui.Start).Children(func() {
					v.player(c, s, playerWidth, playerWidth*9/16, false)
					v.parts(c, s, 260)
				})
			} else {
				v.player(c, s, playerWidth, playerWidth*9/16, false)
				v.parts(c, s, content)
			}
			gui.Text(c.Key("video-description"), s.Video.Description).Width(content).FontSize(14).TextColor(muted)
		})
	}
	v.shortcuts(c, s)
}

func (v *View) player(c *gui.Context, s app.State, w, h float32, fullscreen bool) {
	video := gui.Box(c.Key("preview-video")).Size(w, h).Background(gui.RGB(0, 0, 0)).Role(gui.RoleGroup).Label("Video surface")
	popupWasOpen := v.partsOpen || v.speedOpen || v.danmakuOpen
	video.Children(func() {
		sw, sh := w, h
		if fullscreen {
			sw = min(w, h*16/9)
			sh = sw * 9 / 16
		}
		gui.Box(c).Absolute().Left((w-sw)/2).Top((h-sh)/2).Size(sw, sh).Children(func() {
			if v.Surface != nil {
				v.Surface(c, sw, sh)
			}
			if v.Danmaku != nil {
				v.Danmaku.Render(c, s.Playback, sw, sh)
			}
		})
		if v.overlayVisible(s) {
			// A new playback generation retires native slider capture as well as
			// our sampled drag values. Child automation keys remain unchanged.
			controls := gui.Column(c.Key(fmt.Sprintf("player-controls-%d", s.Playback.Generation))).Absolute().Left(0).Bottom(0).Width(w).Padding(12).Gap(7).Background(gui.RGBA(15, 18, 24, .92))
			controls.Children(func() { v.transport(c, s, max(float32(1), w-24)) })
			v.controlsHeld = controls.Hovered() || controls.FocusWithin()
		} else {
			v.controlsHeld = false
		}
		v.status(c, s, w)
	})
	// Popover dismisses on outside press and lets that press reach its target.
	// Swallow only a captured video press, not Escape or another control.
	if popupWasOpen && !v.partsOpen && !v.speedOpen && !v.danmakuOpen {
		if button, pressed := video.PressedButton(); pressed && button == 0 {
			v.pendingClick = time.Time{}
			v.suppressClick = true
		}
	}
	v.videoInput(c, s, video)
	if !v.controlsHeld && !v.partsOpen && !v.speedOpen && !v.danmakuOpen && s.Playback.State == playback.Playing {
		if remaining := overlayDelay - v.now().Sub(v.lastActivity); remaining > 0 {
			c.After(remaining)
		}
	}
}

func (v *View) status(c *gui.Context, s app.State, w float32) {
	white := gui.RGB(245, 245, 248)
	gui.Column(c.Key("player-status")).Absolute().Left(18).Top(18).Width(max(float32(1), w-36)).Gap(7).Children(func() {
		if s.Closing {
			gui.Text(c.Key("closing-state"), "Closing…").TextColor(white).FontSize(15)
		} else {
			switch s.DetailPhase {
			case app.DetailInitial:
				gui.Text(c.Key("detail-state"), "Preparing video…").TextColor(white).FontSize(15)
			case app.DetailLoading:
				gui.Text(c.Key("detail-state"), "Loading video details…").TextColor(white).FontSize(15)
			}
			if s.ResolvingPart != "" {
				gui.Text(c.Key("resolving-part"), "Loading playback address for "+partTitle(s, s.ResolvingPart)).TextColor(white).FontSize(15)
			}
			if s.Playback.State == playback.Loading {
				gui.Text(c.Key("loading-state"), "Loading…").TextColor(white).FontSize(15)
			}
		}
		if s.DetailPhase == app.DetailFailed || s.Playback.State == playback.Failed || s.SelectionError != playback.NoError || s.PlaybackRetryError != playback.NoError || v.lastError != playback.NoError {
			gui.Column(c.Key("player-errors")).Gap(7).Padding(10).Background(gui.RGB(44, 30, 38)).Children(func() {
				if s.DetailPhase == app.DetailFailed {
					gui.Text(c, "Could not load video details · "+string(s.DetailError)).TextColor(gui.RGB(255, 230, 238))
					if gui.Button(c.Key("retry-detail"), "Retry details").Height(32).Disabled(closed(s)).Clicked() {
						v.action(c, v.Player.RetryDetail)
					}
				}
				if s.SelectionError != playback.NoError {
					gui.Text(c, "Could not select "+partTitle(s, s.SelectedPart)).FontSize(13).TextColor(gui.RGB(255, 230, 238))
					if gui.Button(c.Key("retry-selection"), "Retry selection").Height(32).Disabled(!partsReady(s) || s.ResolvingPart != "").Clicked() {
						v.action(c, v.Player.RetrySelection)
					}
				}
				if s.Playback.State == playback.Failed || s.PlaybackRetryError != playback.NoError {
					message := "Playback failed · " + string(s.Playback.Error)
					if s.PlaybackRetryError != playback.NoError {
						message = "Could not reload playback · " + string(s.PlaybackRetryError)
					}
					gui.Text(c, message).TextColor(gui.RGB(255, 230, 238))
					if gui.Button(c.Key("retry-playback"), "Retry playback").Height(32).Disabled(!partsReady(s) || s.ResolvingPart != "" || s.PlaybackPart == "").Clicked() {
						v.action(c, v.Player.RetryPlayback)
					}
				}
				if v.lastError != playback.NoError {
					gui.Text(c, "Action unavailable · "+string(v.lastError)).TextColor(gui.RGB(255, 230, 238))
				}
			})
		}
	})
}

func (v *View) transport(c *gui.Context, s app.State, w float32) {
	white := gui.RGB(242, 245, 249)
	maximum := s.Playback.Duration.Seconds()
	if maximum <= 0 {
		maximum = 1
	}
	v.seek.sample(s.Playback.Generation, max(0., min(maximum, s.Playback.Position.Seconds())))
	seek := gui.Slider(c.Key("seek-slider"), &v.seek.value, 0, maximum).Step(.1).Label("Seek position").Width(w).Height(28).Disabled(!seekable(s))
	if seek.Changed() && seekable(s) {
		position := time.Duration(max(0., min(maximum, v.seek.value)) * float64(time.Second))
		v.action(c, func() error { return v.Player.Seek(position) })
	}
	button, captured := seek.PressedButton()
	v.seek.pressed = captured && button == 0 && seekable(s)
	gui.Row(c).Gap(6).AlignItems(gui.Center).Children(func() {
		toggle := "Play"
		if s.DesiredPlaying {
			toggle = "Pause"
		}
		if gui.Button(c.Key("play-toggle"), toggle).Height(32).MinWidth(48).Tooltip("Play or pause (Space)").Disabled(!usable(s)).Clicked() {
			v.action(c, v.Player.Toggle)
		}
		last := len(s.Video.Parts) == 0 || s.SelectedPart == s.Video.Parts[len(s.Video.Parts)-1].ID
		if gui.Button(c.Key("next-part"), "Next").Height(32).MinWidth(44).Tooltip("Next part").Disabled(last || !partsReady(s)).Clicked() {
			v.action(c, v.Player.NextPart)
		}
		gui.Text(c, clockText(s.Playback.Position)+" / "+clockText(s.Playback.Duration)).FontSize(12).TextColor(white)
		gui.Box(c).Grow(1)
		parts := gui.Button(c.Key("parts-menu"), "Parts").Height(32).Tooltip("Choose a part").HasPopup(gui.PopupMenu).Disabled(!partsReady(s))
		if parts.Clicked() {
			v.partsOpen = !v.partsOpen
			v.speedOpen, v.danmakuOpen = false, false
			v.pendingClick = time.Time{}
			v.active()
			c.Invalidate()
		}
		gui.Popover(c.Key("parts-popup"), parts, &v.partsOpen, func() { v.partButtons(c, s, true) })
		speed := gui.Button(c.Key("speed-menu"), strconv.FormatFloat(s.Rate, 'f', -1, 64)+"×").Height(32).MinWidth(48).Label("Playback speed").Tooltip("Playback speed").HasPopup(gui.PopupMenu).Disabled(!usable(s))
		if speed.Clicked() {
			v.speedOpen = !v.speedOpen
			v.partsOpen, v.danmakuOpen = false, false
			v.pendingClick = time.Time{}
			v.active()
			c.Invalidate()
		}
		gui.Popover(c.Key("speed-popup"), speed, &v.speedOpen, func() {
			gui.Column(c).Gap(4).Children(func() {
				for _, rate := range []float64{.5, .75, 1, 1.25, 1.5, 2} {
					key := strconv.FormatFloat(rate, 'f', -1, 64)
					if gui.Button(c.Key("speed-"+key), key+"×").Label("Set speed " + key + "×").Width(110).Height(32).AccessSelected(rate == s.Rate).Clicked() {
						v.action(c, func() error { return v.Player.SetRate(rate) })
						v.speedOpen = false
					}
				}
			})
		})
		mute := "Mute"
		if s.Volume == 0 {
			mute = "Unmute"
		}
		if gui.Button(c.Key("mute-toggle"), mute).Height(32).Tooltip("Mute or restore volume (M)").Disabled(!usable(s)).Clicked() {
			v.action(c, v.Player.ToggleMute)
		}
		v.volume.sample(s.Playback.Generation, s.Volume)
		volume := gui.Slider(c.Key("volume-slider"), &v.volume.value, 0, 1).Step(.05).Width(80).Height(28).Label("Volume").Disabled(!usable(s))
		if volume.Changed() && usable(s) {
			v.action(c, func() error { return v.Player.SetVolume(v.volume.value) })
		}
		button, captured := volume.PressedButton()
		v.volume.pressed = captured && button == 0 && usable(s)
		full := "Fullscreen"
		if v.Fullscreen() {
			full = "Exit full"
		}
		if gui.Button(c.Key("fullscreen-toggle"), full).Height(32).Tooltip("Toggle fullscreen (F)").Disabled(closed(s)).Clicked() {
			v.fullscreen(c, !v.Fullscreen())
		}
	})
	gui.Row(c).Gap(8).AlignItems(gui.Center).Children(func() {
		gui.Text(c.Key("playback-part"), "Playing: "+partTitle(s, s.PlaybackPart)).FontSize(11).TextColor(gui.RGB(187, 196, 210)).SingleLine().Grow(1)
		if v.Danmaku != nil {
			menu := gui.Button(c.Key("danmaku-menu"), "Danmaku").Height(32).Tooltip("Show or configure danmaku").HasPopup(gui.PopupMenu).Disabled(closed(s))
			if menu.Clicked() {
				v.danmakuOpen = !v.danmakuOpen
				v.partsOpen, v.speedOpen = false, false
				v.pendingClick = time.Time{}
				v.active()
				c.Invalidate()
			}
			gui.Popover(c.Key("danmaku-popup"), menu, &v.danmakuOpen, func() {
				if v.Danmaku.Controls(c, closed(s)) {
					v.pendingClick = time.Time{}
					v.active()
					c.Invalidate()
				}
			})
		}
	})
}

func (v *View) parts(c *gui.Context, s app.State, w float32) {
	gui.Column(c.Key("parts-panel")).Width(w).Padding(14).Gap(10).Background(gui.RGB(255, 255, 255)).Children(func() {
		gui.Text(c, "Parts").FontSize(16).TextColor(ink)
		v.partButtons(c, s, false)
	})
}
func (v *View) partButtons(c *gui.Context, s app.State, popup bool) {
	gui.Column(c).Gap(6).Children(func() {
		for i, p := range s.Video.Parts {
			key := "list-part-" + string(p.ID)
			if popup {
				key = "part-" + string(p.ID)
			}
			label := fmt.Sprintf("P%d · %s", i+1, p.Title)
			if p.ID == s.PlaybackPart {
				label += " · playing"
			} else if p.ID == s.SelectedPart {
				label += " · requested"
			}
			b := gui.Button(c.Key(key), label).Height(36).MinWidth(220).Tooltip(label).AccessSelected(p.ID == s.SelectedPart).Disabled(!partsReady(s))
			if b.Clicked() {
				v.action(c, func() error { return v.Player.SelectPart(p.ID) })
				v.partsOpen = false
			}
		}
	})
}
