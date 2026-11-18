package desktop

import (
	"fmt"
	"strconv"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
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
	w, h := c.Size()
	v.render(c, w, h, true)
}

// RenderSized presents a pane without pumping playback or source work. The
// client composition owns that pump, including while its window is minimized.
func (v *View) RenderSized(c *gui.Context, width, height float32) {
	v.render(c, max(width, 1), max(height, 1), true)
}

func (v *View) render(c *gui.Context, w, h float32, shortcuts bool) {
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
	if !v.RealContent || !qualityReady(s) || s.PlaybackPart != v.qualityPart {
		v.qualityOpen = false
	}
	if s.PlaybackPart != v.qualityPart {
		v.lastDanmakuError = catalog.None
	}
	v.qualityPart = s.PlaybackPart
	if s.Resuming && !v.resuming {
		// Retire native capture even when resume begins within the same sampled
		// generation; a later release must not submit the old drag target.
		v.seekEpoch++
		v.seek = sliderState{}
	}
	v.resuming = s.Resuming
	if v.lastActivity.IsZero() {
		v.active()
	}
	if s.Playback.Generation != v.generation {
		v.generation = s.Playback.Generation
		v.seek, v.volume = sliderState{}, sliderState{}
		v.qualityOpen = false
		v.pendingClick = time.Time{}
		v.suppressClick = false
		v.active()
	}
	if s.Closing || s.Playback.State != playback.Closed {
		c.After(playback.PollInterval)
	}
	fullscreen := v.Fullscreen()
	if fullscreen {
		v.player(c, s, w, h, true)
	} else if v.RealContent {
		v.realPage(c, s, w, h)
	} else {
		content := max(float32(1), w-48)
		wide := content >= 1000
		playerWidth := content
		if wide {
			playerWidth -= 280
		}
		gui.Scroll(c.Key("preview-page")).Size(w, h).Padding(24).Gap(18).Background(gui.RGB(246, 247, 250)).Children(func() {
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
	if shortcuts {
		v.shortcuts(c, s)
	}
}

func (v *View) realPage(c *gui.Context, s app.State, w, h float32) {
	content := max(float32(1), w-24)
	theme := c.Theme()
	gui.Scroll(c.Key("client-player-pane")).Size(w, h).Padding(12).Gap(12).Background(theme.Background).Children(func() {
		gui.Row(c).Width(content).Height(64).Gap(12).AlignItems(gui.Center).Children(func() {
			gui.Box(c.Key("preview-cover")).Size(80, 45).Background(theme.Surface).Role(gui.RoleImage).Label("视频封面").Children(func() {
				if v.Cover != nil && s.DetailPhase == app.DetailReady {
					v.Cover(c, s.Video.CoverID)
				}
			})
			gui.Column(c).Width(max(content-92, 1)).Gap(3).Children(func() {
				gui.Text(c, "视频详情").FontSize(11).TextColor(theme.TextMuted)
				gui.Text(c.Key("video-title"), s.Video.Title).Width(max(content-92, 1)).SingleLine().FontSize(18).TextColor(theme.Text)
				gui.Text(c, s.Video.Creator).Width(max(content-92, 1)).SingleLine().FontSize(12).TextColor(theme.TextMuted)
			})
		})
		// Keep transport visible at the compact window minimum. The surface is
		// fitted inside this box, so limiting height never stretches the picture.
		height := min(content*9/16, max(float32(200), h-100))
		v.player(c, s, content, height, false)
		v.parts(c, s, content)
		gui.Text(c.Key("video-description"), s.Video.Description).Width(content).FontSize(14).TextColor(theme.TextMuted)
	})
}

func (v *View) player(c *gui.Context, s app.State, w, h float32, fullscreen bool) {
	video := gui.Box(c.Key("preview-video")).Size(w, h).Background(gui.RGB(0, 0, 0)).Role(gui.RoleGroup).Label(v.words("Video surface", "视频画面"))
	popupWasOpen := v.partsOpen || v.speedOpen || v.danmakuOpen || v.qualityOpen
	video.Children(func() {
		sw, sh := w, h
		if fullscreen || v.RealContent {
			sw = min(w, h*16/9)
			sh = sw * 9 / 16
		}
		gui.Box(c).Absolute().Left((w-sw)/2).Top((h-sh)/2).Size(sw, sh).Children(func() {
			if v.Surface != nil {
				v.Surface(c, sw, sh)
			}
			if v.Danmaku != nil && !s.Resuming {
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
	if popupWasOpen && !v.partsOpen && !v.speedOpen && !v.danmakuOpen && !v.qualityOpen {
		if button, pressed := video.PressedButton(); pressed && button == 0 {
			v.pendingClick = time.Time{}
			v.suppressClick = true
		}
	}
	v.videoInput(c, s, video)
	if !v.controlsHeld && !v.partsOpen && !v.speedOpen && !v.danmakuOpen && !v.qualityOpen && s.Playback.State == playback.Playing {
		if remaining := overlayDelay - v.now().Sub(v.lastActivity); remaining > 0 {
			c.After(remaining)
		}
	}
}

func (v *View) status(c *gui.Context, s app.State, w float32) {
	white := gui.RGB(245, 245, 248)
	gui.Column(c.Key("player-status")).Absolute().Left(18).Top(18).Width(max(float32(1), w-36)).Gap(7).Children(func() {
		if s.Closing {
			gui.Text(c.Key("closing-state"), v.words("Closing…", "正在关闭播放…")).TextColor(white).FontSize(15)
		} else {
			switch s.DetailPhase {
			case app.DetailInitial:
				gui.Text(c.Key("detail-state"), v.words("Preparing video…", "正在准备视频…")).TextColor(white).FontSize(15)
			case app.DetailLoading:
				gui.Text(c.Key("detail-state"), v.words("Loading video details…", "正在加载视频详情…")).TextColor(white).FontSize(15)
			}
			if s.ResolvingPart != "" {
				message := v.words("Loading playback address for "+partTitle(s, s.ResolvingPart), "正在加载播放地址…")
				if v.RealContent && s.ResolvingPart == s.PlaybackPart && s.SelectedQuality != s.PlaybackQuality {
					message = "正在切换画质…"
				}
				gui.Text(c.Key("resolving-part"), message).TextColor(white).FontSize(15)
			}
			if s.Playback.State == playback.Loading {
				gui.Text(c.Key("loading-state"), v.words("Loading…", "正在加载画面…")).TextColor(white).FontSize(15)
			}
			if s.Resuming {
				gui.Text(c.Key("resume-state"), v.words("Restoring playback position…", "正在恢复播放进度…")).TextColor(white).FontSize(15)
			}
		}
		if s.DetailPhase == app.DetailFailed || s.Playback.State == playback.Failed || s.SelectionError != catalog.None || s.PlaybackRetryError != catalog.None || s.ResumeError != catalog.None || v.lastError != playback.NoError || v.lastContentError != catalog.None {
			gui.Column(c.Key("player-errors")).Gap(7).Padding(10).Background(gui.RGB(44, 30, 38)).Children(func() {
				if s.DetailPhase == app.DetailFailed {
					message := v.words("Could not load video details · "+string(s.DetailError), "视频详情加载未完成。"+contentMessage(s.DetailError))
					gui.Text(c, message).TextColor(gui.RGB(255, 230, 238))
					if gui.Button(c.Key("retry-detail"), v.words("Retry details", "重试详情")).Height(32).Disabled(closed(s)).Clicked() {
						v.action(c, v.Player.RetryDetail)
					}
				}
				if s.SelectionError != catalog.None {
					message := v.words("Could not select "+partTitle(s, s.SelectedPart), "播放选择未完成。"+contentMessage(s.SelectionError))
					gui.Text(c, message).FontSize(13).TextColor(gui.RGB(255, 230, 238))
					if gui.Button(c.Key("retry-selection"), v.words("Retry selection", "重试选择")).Height(32).Disabled(!partsReady(s) || s.ResolvingPart != "").Clicked() {
						v.action(c, v.Player.RetrySelection)
					}
				}
				if s.Playback.State == playback.Failed || s.PlaybackRetryError != catalog.None || s.ResumeError != catalog.None {
					if s.Playback.State == playback.Failed || s.PlaybackRetryError != catalog.None {
						message := v.words("Playback failed · "+string(s.Playback.Error), playbackMessage(s.Playback.Error))
						if s.PlaybackRetryError != catalog.None {
							message = v.words("Could not reload playback · "+string(s.PlaybackRetryError), "重新加载播放未完成。"+contentMessage(s.PlaybackRetryError))
						}
						gui.Text(c, message).TextColor(gui.RGB(255, 230, 238))
					}
					if s.ResumeError != catalog.None {
						gui.Text(c.Key("resume-error"), v.words("Could not restore playback position", "播放进度恢复未完成，播放保持暂停。")).TextColor(gui.RGB(255, 230, 238))
					}
					if gui.Button(c.Key("retry-playback"), v.words("Retry playback", "重试播放")).Height(32).Disabled(!partsReady(s) || s.ResolvingPart != "" || s.PlaybackPart == "").Clicked() {
						v.action(c, v.Player.RetryPlayback)
					}
				}
				if v.lastError != playback.NoError {
					gui.Text(c, v.words("Action unavailable · "+string(v.lastError), playbackMessage(v.lastError))).TextColor(gui.RGB(255, 230, 238))
				}
				if v.lastContentError != catalog.None {
					gui.Text(c, contentMessage(v.lastContentError)).TextColor(gui.RGB(255, 230, 238))
				}
			})
		}
		if v.RealContent {
			v.danmakuStatus(c, s, w)
		}
	})
}

func (v *View) danmakuStatus(c *gui.Context, s app.State, w float32) {
	if v.DanmakuError == catalog.None && v.DanmakuPrefetchError == catalog.None && v.lastDanmakuError == catalog.None {
		return
	}
	gui.Column(c.Key("client-danmaku-status")).Width(max(w-36, 1)).Gap(6).Children(func() {
		if v.DanmakuError != catalog.None {
			gui.Text(c.Key("client-danmaku-error"), "当前弹幕加载未完成。"+contentMessage(v.DanmakuError)).FontSize(12).TextColor(gui.RGB(255, 230, 238))
		}
		if v.DanmakuPrefetchError != catalog.None {
			gui.Text(c.Key("client-danmaku-prefetch-error"), "下一段弹幕预加载未完成，当前弹幕可继续显示。"+contentMessage(v.DanmakuPrefetchError)).FontSize(12).TextColor(gui.RGB(255, 230, 238))
		}
		if v.lastDanmakuError != catalog.None {
			gui.Text(c.Key("client-danmaku-action-error"), "弹幕重试未完成。"+contentMessage(v.lastDanmakuError)).FontSize(12).TextColor(gui.RGB(255, 230, 238))
		}
		if gui.Button(c.Key("client-retry-danmaku"), "重试弹幕").Height(32).Disabled(closed(s) || v.RetryDanmaku == nil).Clicked() {
			v.lastDanmakuError = contentCode(v.RetryDanmaku())
			v.pendingClick = time.Time{}
			v.active()
			c.Invalidate()
		}
	})
}

func (v *View) transport(c *gui.Context, s app.State, w float32) {
	white := gui.RGB(242, 245, 249)
	if v.seekEpoch == 0 {
		v.seekControl(c, s, w)
	} else {
		gui.Column(c.Key(fmt.Sprintf("seek-epoch-%d", v.seekEpoch))).Children(func() { v.seekControl(c, s, w) })
	}
	row := gui.Row(c).Gap(6).AlignItems(gui.Center)
	if v.RealContent {
		row.Width(w).Wrap()
	}
	row.Children(func() {
		toggle := v.words("Play", "播放")
		if s.DesiredPlaying {
			toggle = v.words("Pause", "暂停")
		} else if v.RealContent && s.Playback.State == playback.Ended {
			toggle = "重新播放"
		}
		if gui.Button(c.Key("play-toggle"), toggle).Height(32).MinWidth(48).Tooltip(v.words("Play or pause (Space)", "播放或暂停（空格）")).Disabled(!usable(s)).Clicked() {
			v.action(c, v.Player.Toggle)
		}
		last := len(s.Video.Parts) == 0 || s.SelectedPart == s.Video.Parts[len(s.Video.Parts)-1].ID
		if gui.Button(c.Key("next-part"), v.words("Next", "下一集")).Height(32).MinWidth(44).Tooltip(v.words("Next part", "播放下一集")).Disabled(last || !partsReady(s)).Clicked() {
			v.action(c, v.Player.NextPart)
		}
		gui.Text(c, clockText(s.Playback.Position)+" / "+clockText(s.Playback.Duration)).FontSize(12).TextColor(white)
		gui.Box(c).Grow(1)
		parts := gui.Button(c.Key("parts-menu"), v.words("Parts", "分集")).Height(32).Tooltip(v.words("Choose a part", "选择分集")).HasPopup(gui.PopupMenu).Disabled(!partsReady(s))
		if parts.Clicked() {
			v.partsOpen = !v.partsOpen
			v.speedOpen, v.danmakuOpen, v.qualityOpen = false, false, false
			v.pendingClick = time.Time{}
			v.active()
			c.Invalidate()
		}
		gui.Popover(c.Key("parts-popup"), parts, &v.partsOpen, func() { v.partButtons(c, s, true, 240) })
		speed := gui.Button(c.Key("speed-menu"), strconv.FormatFloat(s.Rate, 'f', -1, 64)+"×").Height(32).MinWidth(48).Label(v.words("Playback speed", "播放速度")).Tooltip(v.words("Playback speed", "播放速度")).HasPopup(gui.PopupMenu).Disabled(!usable(s))
		if speed.Clicked() {
			v.speedOpen = !v.speedOpen
			v.partsOpen, v.danmakuOpen, v.qualityOpen = false, false, false
			v.pendingClick = time.Time{}
			v.active()
			c.Invalidate()
		}
		gui.Popover(c.Key("speed-popup"), speed, &v.speedOpen, func() {
			gui.Column(c).Gap(4).Children(func() {
				for _, rate := range []float64{.5, .75, 1, 1.25, 1.5, 2} {
					key := strconv.FormatFloat(rate, 'f', -1, 64)
					if gui.Button(c.Key("speed-"+key), key+"×").Label(v.words("Set speed "+key+"×", "设置速度 "+key+"×")).Width(110).Height(32).AccessSelected(rate == s.Rate).Clicked() {
						v.action(c, func() error { return v.Player.SetRate(rate) })
						v.speedOpen = false
					}
				}
			})
		})
		if v.RealContent {
			v.quality(c, s)
		}
		mute := v.words("Mute", "静音")
		if s.Volume == 0 {
			mute = v.words("Unmute", "恢复音量")
		}
		if gui.Button(c.Key("mute-toggle"), mute).Height(32).Tooltip(v.words("Mute or restore volume (M)", "静音或恢复音量（M）")).Disabled(!usable(s)).Clicked() {
			v.action(c, v.Player.ToggleMute)
		}
		v.volume.sample(s.Playback.Generation, s.Volume)
		volume := gui.Slider(c.Key("volume-slider"), &v.volume.value, 0, 1).Step(.05).Width(80).Height(28).Label(v.words("Volume", "音量")).Disabled(!usable(s))
		if volume.Changed() && usable(s) {
			v.action(c, func() error { return v.Player.SetVolume(v.volume.value) })
		}
		button, captured := volume.PressedButton()
		v.volume.pressed = captured && button == 0 && usable(s)
		full := v.words("Fullscreen", "全屏")
		if v.Fullscreen() {
			full = v.words("Exit full", "退出全屏")
		}
		if gui.Button(c.Key("fullscreen-toggle"), full).Height(32).Tooltip(v.words("Toggle fullscreen (F)", "切换全屏（F）")).Disabled(closed(s)).Clicked() {
			v.fullscreen(c, !v.Fullscreen())
		}
	})
	gui.Row(c).Gap(8).AlignItems(gui.Center).Children(func() {
		gui.Text(c.Key("playback-part"), v.words("Playing: ", "正在播放：")+v.partTitle(s, s.PlaybackPart)).FontSize(11).TextColor(gui.RGB(187, 196, 210)).SingleLine().Grow(1)
		if v.Danmaku != nil {
			menu := gui.Button(c.Key("danmaku-menu"), v.words("Danmaku", "弹幕")).Height(32).Tooltip(v.words("Show or configure danmaku", "显示或设置弹幕")).HasPopup(gui.PopupMenu).Disabled(closed(s))
			if menu.Clicked() {
				v.danmakuOpen = !v.danmakuOpen
				v.partsOpen, v.speedOpen, v.qualityOpen = false, false, false
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
	if v.RealContent {
		gui.Row(c).Width(w).Wrap().Gap(12).Children(func() {
			actual := qualityName(s, s.PlaybackQuality)
			if s.PlaybackQuality == "" {
				actual = "待确认"
			}
			gui.Text(c.Key("playback-quality"), "实际画质："+actual).FontSize(11).TextColor(gui.RGB(187, 196, 210))
			if s.SelectedQuality != s.PlaybackQuality {
				gui.Text(c.Key("selected-quality"), "请求画质："+qualityName(s, s.SelectedQuality)).FontSize(11).TextColor(gui.RGB(242, 245, 249))
			}
		})
	}
}

func (v *View) seekControl(c *gui.Context, s app.State, w float32) {
	maximum := s.Playback.Duration.Seconds()
	if maximum <= 0 {
		maximum = 1
	}
	v.seek.sample(s.Playback.Generation, max(0., min(maximum, s.Playback.Position.Seconds())))
	seek := gui.Slider(c.Key("seek-slider"), &v.seek.value, 0, maximum).Step(.1).Label(v.words("Seek position", "播放进度")).Width(w).Height(28).Disabled(!seekable(s))
	if seek.Changed() && seekable(s) {
		position := time.Duration(max(0., min(maximum, v.seek.value)) * float64(time.Second))
		v.action(c, func() error { return v.Player.Seek(position) })
	}
	button, captured := seek.PressedButton()
	v.seek.pressed = captured && button == 0 && seekable(s)
}

func qualityName(s app.State, id app.QualityID) string {
	if id == "" {
		return "默认画质"
	}
	for _, q := range s.Qualities {
		if q.ID == id {
			return q.Label
		}
	}
	return "暂不可用"
}

func (v *View) quality(c *gui.Context, s app.State) {
	menu := gui.Button(c.Key("quality-menu"), "画质").Height(32).Tooltip("选择画质").HasPopup(gui.PopupMenu).Disabled(!qualityReady(s))
	if menu.Clicked() {
		v.qualityOpen = !v.qualityOpen
		v.partsOpen, v.speedOpen, v.danmakuOpen = false, false, false
		v.pendingClick = time.Time{}
		v.active()
		c.Invalidate()
	}
	gui.Popover(c.Key("quality-popup"), menu, &v.qualityOpen, func() {
		gui.Column(c).Gap(4).Children(func() {
			for i, q := range s.Qualities {
				label := q.Label
				if q.ID == s.PlaybackQuality {
					label += " · 当前播放"
				}
				if q.ID == s.SelectedQuality && q.ID != s.PlaybackQuality {
					label += " · 已请求"
				}
				if gui.Button(c.Key(fmt.Sprintf("quality-%d", i)), label).Width(240).Height(32).AccessSelected(q.ID == s.SelectedQuality).Disabled(!qualityReady(s)).Clicked() {
					v.action(c, func() error { return v.Player.SelectQuality(q.ID) })
					v.qualityOpen = false
				}
			}
		})
	})
}

func (v *View) parts(c *gui.Context, s app.State, w float32) {
	background, text := gui.RGB(255, 255, 255), ink
	if v.RealContent {
		background, text = c.Theme().Surface, c.Theme().Text
	}
	gui.Column(c.Key("parts-panel")).Width(w).Padding(14).Gap(10).Background(background).Children(func() {
		gui.Text(c, v.words("Parts", "分集")).FontSize(16).TextColor(text)
		v.partButtons(c, s, false, max(w-28, 1))
	})
}
func (v *View) partButtons(c *gui.Context, s app.State, popup bool, width float32) {
	gui.Column(c).Gap(6).Children(func() {
		for i, p := range s.Video.Parts {
			key := "list-part-" + string(p.ID)
			if popup {
				key = "part-" + string(p.ID)
			}
			label := fmt.Sprintf("P%d · %s", i+1, p.Title)
			if p.ID == s.PlaybackPart {
				label += v.words(" · playing", " · 正在播放")
			} else if p.ID == s.SelectedPart {
				label += v.words(" · requested", " · 已请求")
			}
			b := gui.Button(c.Key(key), label).Height(36).MinWidth(220).Tooltip(label).AccessSelected(p.ID == s.SelectedPart).Disabled(!partsReady(s))
			if v.RealContent {
				b.Width(width)
			}
			if b.Clicked() {
				v.action(c, func() error { return v.Player.SelectPart(p.ID) })
				v.partsOpen = false
			}
		}
	})
}
