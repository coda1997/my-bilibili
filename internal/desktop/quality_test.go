package desktop

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

func newRealHarness(t *testing.T) *harness {
	t.Helper()
	h := newDanmakuHarness(t)
	t.Cleanup(h.view.Danmaku.Close)
	h.view.RealContent = true
	h.player.state.Video.Title, h.player.state.Video.Creator, h.player.state.Video.Description = "测试视频", "测试作者", "测试简介"
	h.player.state.Video.Parts[0].Title, h.player.state.Video.Parts[1].Title = "第一集", "第二集"
	h.player.state.Qualities = []app.Quality{
		{ID: "returned-low-exact", Label: "画质一", Width: 1920, Height: 1080},
		{ID: "returned-high-exact", Label: "画质二", Width: 1280, Height: 720},
	}
	h.player.state.SelectedQuality, h.player.state.PlaybackQuality = "returned-low-exact", "returned-low-exact"
	h.player.calls, h.player.ticks = nil, 0
	h.ui = gui.NewTester(func(c *gui.Context) { w, height := c.Size(); h.view.RenderSized(c, w, height) }, 800, 600)
	return h
}

func realVideoClick(t *testing.T, h *harness) {
	t.Helper()
	r := rect(t, h, "视频画面")
	h.ui.ClickAt(r.X+r.W/2, r.Y+20)
}

func TestRenderSizedIsPureAndLegacyRenderStillTicks(t *testing.T) {
	h := newHarness()
	frames := 0
	h.player.ticks = 0
	h.ui = gui.NewTester(func(c *gui.Context) { frames++; h.view.Render(c) }, 1120, 760)
	for range 3 {
		h.ui.Frame()
	}
	if frames == 0 || h.player.ticks != frames || !h.ui.HasText("LOCAL PREVIEW") {
		t.Fatalf("legacy pump changed: frames=%d ticks=%d", frames, h.player.ticks)
	}
	h.player.ticks = 0
	h.ui = gui.NewTester(func(c *gui.Context) { h.view.RenderSized(c, 800, 600) }, 1300, 900)
	for range 3 {
		h.ui.Frame()
	}
	if h.player.ticks != 0 || h.surfaceW != 752 || h.surfaceH != 423 || len(h.player.calls) != 0 {
		t.Fatalf("pane presentation pumped work or used window size: ticks=%d surface=%vx%v calls=%v", h.player.ticks, h.surfaceW, h.surfaceH, h.player.calls)
	}
	if _, exists := h.ui.Find("画质"); exists {
		t.Fatal("default fixture gained quality controls")
	}
}

func TestQualityShowsRequestedAndAcceptedAndDispatchesExactChoice(t *testing.T) {
	h := newRealHarness(t)
	h.player.state.SelectedQuality = "returned-high-exact"
	h.player.state.ResolvingPart = h.player.state.PlaybackPart
	h.ui.Frame()
	for _, text := range []string{"实际画质：画质一", "请求画质：画质二", "正在切换画质…"} {
		rect(t, h, text)
	}
	if h.player.ticks != 0 {
		t.Fatal("real paint pumped source")
	}
	click(t, h, "画质")
	if !access(t, h, "画质二 · 已请求").Checked || access(t, h, "画质一 · 当前播放").Checked {
		t.Fatal("pending quality was presented as accepted")
	}
	click(t, h, "画质一 · 当前播放")
	click(t, h, "画质")
	click(t, h, "画质二 · 已请求")
	wantCalls(t, h, call{name: "quality", quality: "returned-low-exact"}, call{name: "quality", quality: "returned-high-exact"})
	if h.view.qualityOpen || h.player.state.PlaybackQuality != "returned-low-exact" {
		t.Fatal("view changed accepted quality")
	}
	if h.ui.HasText("1080") || h.ui.HasText("720") || h.ui.HasText("returned-") {
		t.Fatal("view guessed labels from dimensions or exposed opaque IDs")
	}
}

func TestQualityEligibilityAndSamePartPendingReplacement(t *testing.T) {
	for _, test := range []struct {
		name    string
		change  func(*app.State)
		enabled bool
	}{
		{"accepted paused", func(*app.State) {}, true},
		{"accepted loading", func(s *app.State) { s.Playback.State = playback.Loading }, true},
		{"accepted failure recovery", func(s *app.State) { s.Playback.State = playback.Failed }, true},
		{"same part pending", func(s *app.State) { s.ResolvingPart = s.PlaybackPart; s.SelectedQuality = "returned-high-exact" }, true},
		{"confirmed part resuming", func(s *app.State) { s.Resuming = true }, true},
		{"no detail", func(s *app.State) { s.DetailPhase = app.DetailLoading }, false},
		{"no accepted part", func(s *app.State) { s.PlaybackPart = "" }, false},
		{"no returned choices", func(s *app.State) { s.Qualities = nil }, false},
		{"other selected part", func(s *app.State) { s.SelectedPart = "part-2" }, false},
		{"other resolving part", func(s *app.State) { s.ResolvingPart = "part-2" }, false},
		{"ended", func(s *app.State) { s.Playback.State = playback.Ended }, false},
		{"closed", func(s *app.State) { s.Playback.State = playback.Closed }, false},
		{"closing", func(s *app.State) { s.Closing = true }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newRealHarness(t)
			test.change(&h.player.state)
			h.ui.Frame()
			if access(t, h, "画质").Disabled == test.enabled {
				t.Fatal("quality eligibility disagrees with accepted-part contract")
			}
			click(t, h, "画质")
			if h.view.qualityOpen != test.enabled {
				t.Fatal("quality popup availability")
			}
			if !test.enabled {
				wantCalls(t, h)
				return
			}
			label := "画质二"
			if h.player.state.SelectedQuality == "returned-high-exact" {
				label += " · 已请求"
			}
			click(t, h, label)
			wantCalls(t, h, call{name: "quality", quality: "returned-high-exact"})
		})
	}
}

func TestQualityPopupRetiresOnUnavailablePartGenerationOrFixture(t *testing.T) {
	for _, change := range []func(*harness){
		func(h *harness) { h.player.state.Playback.Generation++ },
		func(h *harness) { h.player.state.PlaybackPart, h.player.state.SelectedPart = "part-2", "part-2" },
		func(h *harness) { h.player.state.Playback.State = playback.Ended },
		func(h *harness) { h.view.RealContent = false },
	} {
		h := newRealHarness(t)
		click(t, h, "画质")
		change(h)
		h.ui.Frame()
		if h.view.qualityOpen || h.ui.HasText("画质二") {
			t.Fatal("retired quality popup survived")
		}
		wantCalls(t, h)
	}
}

func TestQualityFailureKeepsAcceptedPlaybackAndTransportVisible(t *testing.T) {
	h := newRealHarness(t)
	h.player.state.Playback.State = playback.Playing
	h.player.state.SelectionError = catalog.Network
	h.player.state.SelectedQuality = "returned-high-exact"
	h.ui.Frame()
	h.ui.Advance(4 * time.Second)
	if h.view.controlsHeld || h.view.now().Sub(h.view.lastActivity) < overlayDelay {
		t.Fatal("test did not isolate the failure visibility hold")
	}
	for _, text := range []string{"播放进度", "实际画质：画质一", "请求画质：画质二", "重试选择"} {
		rect(t, h, text)
	}
	if h.player.state.Playback.State != playback.Playing || h.view.Danmaku.State().Active != 1 {
		t.Fatal("quality resolution failure disturbed accepted playback or comments")
	}
	wantCalls(t, h)
	click(t, h, "重试选择")
	wantCalls(t, h, call{name: "retry-selection"})
}

func realPopup(t *testing.T, h *harness, name string) {
	t.Helper()
	if h.view.partsOpen != (name == "分集") || h.view.speedOpen != (name == "播放速度") || h.view.danmakuOpen != (name == "弹幕") || h.view.qualityOpen != (name == "画质") {
		t.Fatalf("menus are not exclusive for %s: parts=%v speed=%v danmaku=%v quality=%v", name, h.view.partsOpen, h.view.speedOpen, h.view.danmakuOpen, h.view.qualityOpen)
	}
}

func TestFourMenusKeyboardExclusivity(t *testing.T) {
	menus := []string{"分集", "播放速度", "弹幕", "画质"}
	for _, first := range menus {
		for _, second := range menus {
			if first == second {
				continue
			}
			t.Run(first+" to "+second, func(t *testing.T) {
				h := newRealHarness(t)
				click(t, h, second)
				click(t, h, second)
				h.view.partsOpen, h.view.speedOpen, h.view.danmakuOpen, h.view.qualityOpen = first == "分集", first == "播放速度", first == "弹幕", first == "画质"
				h.ui.Frame()
				if !h.ui.Focused(second) {
					t.Fatal("test did not retain the target anchor's native focus")
				}
				h.ui.Key(0, gui.KeySpace)
				realPopup(t, h, second)
				h.ui.Advance(time.Second)
				wantCalls(t, h)
			})
		}
	}
}

func TestQualityMenuHoldsControlsEscapesAndSuppressesClickThrough(t *testing.T) {
	h := newRealHarness(t)
	h.player.state.Playback.State = playback.Playing
	h.ui.Frame()
	realVideoClick(t, h)
	click(t, h, "画质")
	if !h.view.pendingClick.IsZero() {
		t.Fatal("opening quality retained a deferred video toggle")
	}
	// An anchor retains focus after opening. Clear it, then expose the same
	// popup state without an anchor focus hold, to isolate menu visibility.
	h.ui.Key(0, gui.KeyEscape)
	h.ui.ClickAt(0, 0)
	h.view.qualityOpen = true
	h.ui.Frame()
	h.ui.Move(0, 0)
	h.ui.Advance(4 * time.Second)
	if h.view.controlsHeld || !h.view.pendingClick.IsZero() || h.view.now().Sub(h.view.lastActivity) < overlayDelay {
		t.Fatal("open popup hold was not isolated from pointer, focus or delayed click")
	}
	rect(t, h, "播放进度")
	wantCalls(t, h)
	realVideoClick(t, h)
	h.ui.Advance(time.Second)
	realPopup(t, h, "")
	wantCalls(t, h)
	realVideoClick(t, h)
	h.ui.Advance(time.Second)
	wantCalls(t, h, call{name: "toggle"})
	h.player.calls = nil
	h.full = true
	h.ui.Frame()
	click(t, h, "画质")
	h.ui.Key(0, gui.KeyEscape)
	realPopup(t, h, "")
	if !h.full {
		t.Fatal("first Escape exited fullscreen")
	}
	h.ui.Key(0, gui.KeyEscape)
	if h.full {
		t.Fatal("second Escape did not exit fullscreen")
	}
	wantCalls(t, h)
}

func TestResumeDisablesSeekKeepsIntentAndRetiresCapture(t *testing.T) {
	h := newRealHarness(t)
	h.player.state.Resuming = true
	h.player.state.Playback.Position = 2 * time.Second
	h.ui.Frame()
	rect(t, h, "正在恢复播放进度…")
	if a := access(t, h, "播放进度"); !a.Disabled || a.Now != 2 {
		t.Fatalf("resume displayed an unsampled command target: %+v", a)
	}
	h.ui.Key(0, gui.KeyLeft)
	h.ui.Key(0, gui.KeyRight)
	click(t, h, "播放进度")
	wantCalls(t, h)
	click(t, h, "播放")
	h.player.state.DesiredPlaying = true
	h.ui.Frame()
	click(t, h, "暂停")
	wantCalls(t, h, call{name: "toggle"}, call{name: "toggle"})

	drag := newRealHarness(t)
	r := rect(t, drag, "播放进度")
	x, y := r.X+r.W/2, r.Y+r.H/2
	drag.ui.Press(x, y)
	if len(drag.player.calls) != 1 || !drag.view.seek.pressed {
		t.Fatal("test did not capture seek")
	}
	drag.player.state.Resuming = true
	drag.player.state.Playback.Position = 3 * time.Second
	drag.ui.Frame()
	if drag.view.seek.pressed || access(t, drag, "播放进度").Now != 3 {
		t.Fatal("resume retained a command-target drag sample")
	}
	drag.player.state.Resuming = false
	drag.ui.Frame()
	drag.ui.Move(x+40, y)
	drag.ui.Release(x+40, y)
	drag.ui.Frame()
	if len(drag.player.calls) != 1 || access(t, drag, "播放进度").Now != 3 {
		t.Fatal("retired seek capture submitted after resume")
	}
}

func TestResumeHidesDanmakuWithoutChangingSamplesOrSettings(t *testing.T) {
	h := newRealHarness(t)
	before, settings := h.view.Danmaku.State(), h.view.Danmaku.Settings()
	if before.Active != 1 {
		t.Fatal("test did not establish a rendered comment")
	}
	h.player.state.Resuming = true
	h.player.state.Playback.Position = 0
	h.ui.Frame()
	if _, shown := h.ui.Find("Danmaku overlay"); shown {
		t.Fatal("resume displayed the intermediate zero-position frame")
	}
	if !reflect.DeepEqual(h.view.Danmaku.State(), before) || h.view.Danmaku.Settings() != settings {
		t.Fatal("hiding fed a fabricated native sample or changed settings")
	}
	h.player.state.Resuming = false
	h.player.state.Playback.Position = 2 * time.Second
	h.ui.Frame()
	rect(t, h, "Danmaku overlay")
	if h.view.Danmaku.State().Active != 1 || h.view.Danmaku.Settings() != settings {
		t.Fatal("confirmed native sample did not restore presentation")
	}
	wantCalls(t, h)
}

func TestResumeFailureHasOneExplicitPlaybackRetry(t *testing.T) {
	for _, otherFailure := range []bool{false, true} {
		h := newRealHarness(t)
		h.player.state.ResumeError = catalog.Timeout
		if otherFailure {
			h.player.state.Playback.State, h.player.state.Playback.Error = playback.Failed, playback.Media
			h.player.state.PlaybackRetryError = catalog.Network
		}
		h.ui.Frame()
		rect(t, h, "播放进度恢复未完成，播放保持暂停。")
		count := 0
		for _, a := range h.ui.AccessAll() {
			if a.Role == gui.RoleButton && a.Name == "重试播放" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("playback retry controls=%d", count)
		}
		h.ui.Advance(time.Second)
		wantCalls(t, h)
		click(t, h, "重试播放")
		wantCalls(t, h, call{name: "retry-playback"})
	}
}

func TestDanmakuLoaderFailuresHaveIndependentExplicitRetry(t *testing.T) {
	h := newRealHarness(t)
	retries := 0
	h.view.RetryDanmaku = func() error { retries++; return errors.New("private retry error sentinel") }
	h.view.DanmakuPrefetchError = catalog.Network
	h.ui.Frame()
	if !h.ui.HasText("下一段弹幕预加载未完成") || h.ui.HasText("当前弹幕加载未完成") || h.view.Danmaku.State().Active != 1 {
		t.Fatal("prefetch failure hid or misrepresented current comments")
	}
	h.ui.Advance(time.Second)
	if retries != 0 {
		t.Fatal("painting retried loader")
	}
	click(t, h, "重试弹幕")
	if retries != 1 || !h.ui.HasText("弹幕重试未完成") || h.ui.HasText("private retry error sentinel") || h.view.lastError != playback.NoError {
		t.Fatal("danmaku retry failed privacy or changed playback error")
	}
	h.view.DanmakuError = catalog.Timeout
	h.view.DanmakuPrefetchError = catalog.None
	h.ui.Frame()
	if !h.ui.HasText("当前弹幕加载未完成") || h.ui.HasText("下一段弹幕预加载未完成") {
		t.Fatal("current and prefetch failures were conflated")
	}
	h.view.RetryDanmaku = nil
	h.ui.Frame()
	if !access(t, h, "重试弹幕").Disabled {
		t.Fatal("nil retry callback enabled")
	}
	h.view.RetryDanmaku = func() error { retries++; return nil }
	h.player.state.Closing = true
	h.ui.Frame()
	if !access(t, h, "重试弹幕").Disabled {
		t.Fatal("closing loader retry enabled")
	}
	click(t, h, "重试弹幕")
	if retries != 1 {
		t.Fatal("disabled retry ran")
	}
	wantCalls(t, h)
}

func TestRealCopyAndErrorsNeverExposeOpaqueValues(t *testing.T) {
	h := newRealHarness(t)
	if !h.ui.HasText("视频详情") || !h.ui.HasText("测试视频") || h.ui.HasText("LOCAL PREVIEW") {
		t.Fatal("real page uses fixture headings")
	}
	h.player.err = catalog.QualityUnavailable
	click(t, h, "画质")
	click(t, h, "画质二")
	rect(t, h, "所选画质暂不可用。")
	if h.player.state.PlaybackQuality != "returned-low-exact" {
		t.Fatal("failed selection claimed a new accepted quality")
	}
	sentinel := "private-error-https://invalid.local/secret"
	h.player.state.DetailPhase = app.DetailFailed
	h.player.state.DetailError = catalog.Code(sentinel)
	h.player.state.SelectionError = catalog.Code(sentinel)
	h.player.state.PlaybackRetryError = catalog.Code(sentinel)
	h.player.state.Playback.State, h.player.state.Playback.Error = playback.Failed, playback.ErrorCode(sentinel)
	h.view.lastError = playback.ErrorCode(sentinel)
	h.view.lastContentError = catalog.Code(sentinel)
	h.view.DanmakuError, h.view.DanmakuPrefetchError = catalog.Code(sentinel), catalog.Code(sentinel)
	h.player.state.PlaybackQuality, h.player.state.SelectedQuality = app.QualityID(sentinel), app.QualityID(sentinel+"2")
	h.ui.Frame()
	if strings.Contains(strings.Join(h.ui.Texts(), " "), sentinel) {
		t.Fatal("real UI exposed an arbitrary error or opaque quality identity")
	}
	if !h.ui.HasText("实际画质：暂不可用") || !h.ui.HasText("请求画质：暂不可用") {
		t.Fatal("missing choice gained an invented label")
	}
}

func TestRealControlsWrapWithinPaneAndQualityPopupFits(t *testing.T) {
	h := newRealHarness(t)
	h.ui = gui.NewTester(func(c *gui.Context) { h.view.RenderSized(c, 540, 500) }, 800, 600)
	video := rect(t, h, "视频画面")
	for _, label := range []string{"播放", "下一集", "分集", "播放速度", "画质", "静音", "音量", "全屏", "弹幕", "播放进度"} {
		a := access(t, h, label)
		if a.Bounds.W < 28 || a.Bounds.H < 28 || !a.Focusable || a.Offscreen || a.Bounds.X < video.X || a.Bounds.X+a.Bounds.W > video.X+video.W+1 || a.Bounds.Y+a.Bounds.H > video.Y+video.H+1 {
			t.Fatalf("control outside player pane: %s %+v video=%+v", label, a, video)
		}
	}
	click(t, h, "画质")
	for _, label := range []string{"画质一 · 当前播放", "画质二"} {
		a := access(t, h, label)
		if a.Bounds.X < 0 || a.Bounds.Y < 0 || a.Bounds.X+a.Bounds.W > 800 || a.Bounds.Y+a.Bounds.H > 600 {
			t.Fatalf("quality option outside window: %+v", a)
		}
	}
}
