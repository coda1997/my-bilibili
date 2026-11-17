package historyview

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

type fakeController struct {
	state history.State
	calls []string
	err   error
}

func (f *fakeController) State() history.State {
	s := f.state
	s.Records = slices.Clone(s.Records)
	return s
}
func (f *fakeController) Delete(ref catalog.PartRef) error {
	f.calls = append(f.calls, "delete:"+string(ref.Part))
	if f.err == nil {
		f.state.Records = slices.DeleteFunc(f.state.Records, func(r history.Record) bool { return r.Ref == ref })
		f.state.Revision++
	}
	return f.err
}
func (f *fakeController) Clear() error {
	f.calls = append(f.calls, "clear")
	if f.err == nil {
		f.state.Records = nil
		f.state.Revision++
	}
	return f.err
}
func (f *fakeController) ResetCorrupt() error {
	f.calls = append(f.calls, "reset")
	if f.err == nil {
		f.state = history.State{Phase: history.Ready}
	}
	return f.err
}
func (f *fakeController) Retry() error { f.calls = append(f.calls, "retry"); return f.err }

type harness struct {
	view                *View
	controller          *fakeController
	ui                  *gui.Tester
	continued, replayed []history.Record
	closes              []bool
	closing             bool
	width, height       float32
}

func record(part string) history.Record {
	return history.Record{Ref: catalog.PartRef{Video: "BV0123456789", Part: catalog.PartID(part)}, Title: "原创历史视频", PartTitle: "分集 " + part, Position: 17 * time.Second, Duration: 2 * time.Minute, UpdatedAt: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)}
}
func newHarness(t *testing.T, state history.State) *harness {
	t.Helper()
	h := &harness{controller: &fakeController{state: state}, width: 280, height: 500}
	view, err := New(h.controller, Actions{
		Continue:     func(r history.Record) error { h.continued = append(h.continued, r); return h.controller.err },
		Replay:       func(r history.Record) error { h.replayed = append(h.replayed, r); return h.controller.err },
		RequestClose: func(discard bool) { h.closes = append(h.closes, discard) },
	})
	if err != nil {
		t.Fatal(err)
	}
	h.view = view
	h.ui = gui.NewTester(func(c *gui.Context) { h.view.Render(c, h.width, h.height, h.closing) }, 800, 600)
	return h
}
func click(t *testing.T, h *harness, label string) {
	t.Helper()
	if err := h.ui.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestCopiedContinueReplayAndPureLifetime(t *testing.T) {
	r := record("123")
	h := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{r}})
	if len(h.controller.calls) != 0 || len(h.continued) != 0 || len(h.closes) != 0 {
		t.Fatal("construction/render performed action")
	}
	click(t, h, "继续播放")
	click(t, h, "从头播放")
	if len(h.continued) != 1 || len(h.replayed) != 1 || h.continued[0] != r || h.replayed[0] != r {
		t.Fatal("lost exact copied record or action intent")
	}
	h.continued[0].Position = 0
	if h.controller.state.Records[0].Position != 17*time.Second {
		t.Fatal("callback aliases manager")
	}
	h.view.Hide()
	h.view.Close()
	h.ui.Frame()
	if len(h.controller.calls) != 0 || len(h.closes) != 0 || h.view.controller != nil || h.view.actions.RequestClose != nil || h.view.ref != (catalog.PartRef{}) {
		t.Fatal("pane retirement closed owner or retained callback")
	}
	r.Completed = true
	completed := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{r}})
	if completed.ui.HasText("继续播放") {
		t.Fatal("completed row offered silent continue")
	}
	click(t, completed, "从头播放")
	if len(completed.replayed) != 1 || len(completed.continued) != 0 {
		t.Fatal("completed replay not explicit")
	}
	if _, err := New(nil, Actions{}); err != history.Invalid {
		t.Fatal(err)
	}
}

func TestDeleteConfirmCancelStableIdentityAndOnce(t *testing.T) {
	a := record("123")
	h := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{a}, Busy: true})
	click(t, h, "删除记录…")
	if len(h.controller.calls) != 0 || !h.ui.HasText("确认删除这条本地播放记录？") {
		t.Fatal("delete skipped confirmation or disabled during save")
	}
	click(t, h, "取消")
	if len(h.controller.calls) != 0 || h.view.confirm != noConfirmation {
		t.Fatal("cancel mutated history")
	}
	click(t, h, "删除记录…")
	h.controller.state.Revision++
	h.controller.state.Records[0].Position = 22 * time.Second
	h.ui.Frame()
	if !h.ui.HasText("确认删除这条本地播放记录？") {
		t.Fatal("progress dismissed question")
	}
	h.controller.state.Records = append([]history.Record{record("456")}, h.controller.state.Records...)
	h.controller.state.Revision++
	h.ui.Frame()
	click(t, h, "确认")
	h.ui.Frame()
	if !slices.Equal(h.controller.calls, []string{"delete:123"}) || len(h.controller.state.Records) != 1 || h.controller.state.Records[0].Ref.Part != "456" {
		t.Fatalf("confirmation redirected or repeated: %+v", h.controller.calls)
	}
}

func TestClearResetConfirmationsAndRetirement(t *testing.T) {
	h := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{record("123")}})
	click(t, h, "清空历史记录…")
	if len(h.controller.calls) != 0 {
		t.Fatal("clear before confirmation")
	}
	h.view.Hide()
	h.ui.Frame()
	if h.ui.HasText("确认清空这台设备上的全部历史记录？") {
		t.Fatal("hidden pane retained confirmation")
	}
	click(t, h, "清空历史记录…")
	click(t, h, "确认")
	h.ui.Frame()
	if !slices.Equal(h.controller.calls, []string{"clear"}) || !h.ui.HasText("还没有播放记录。") {
		t.Fatal("clear not exactly once")
	}
	for _, code := range []history.Code{history.Corrupt, history.UnsupportedVersion} {
		reset := newHarness(t, history.State{Phase: history.Failed, Error: code})
		click(t, reset, "重置历史记录…")
		if len(reset.controller.calls) != 0 {
			t.Fatal("reset before confirmation")
		}
		click(t, reset, "取消")
		if len(reset.controller.calls) != 0 {
			t.Fatal("reset cancel called store")
		}
		click(t, reset, "重置历史记录…")
		click(t, reset, "确认")
		reset.ui.Frame()
		if !slices.Equal(reset.controller.calls, []string{"reset"}) {
			t.Fatal("reset not explicit once")
		}
	}
	busy := newHarness(t, history.State{Phase: history.Failed, Error: history.Corrupt, Busy: true})
	click(t, busy, "重置历史记录…")
	if busy.view.confirm != noConfirmation || len(busy.controller.calls) != 0 {
		t.Fatal("busy reset accepted")
	}
}

func TestHistoryFailureStatesRetryAndCleanupCannotBeDiscarded(t *testing.T) {
	for _, tc := range []struct {
		code history.Code
		text string
	}{
		{history.InUse, "另一个应用正在使用本地历史记录。当前播放进度暂不保存。"},
		{history.LockUnsupported, "当前系统无法安全访问本地历史记录。当前播放进度暂不保存。"},
		{history.Corrupt, "本地历史记录无法读取，原文件已保留。"},
		{history.UnsupportedVersion, "历史记录来自不支持的版本，原文件已保留。"},
		{history.Storage, "本地历史记录暂时不可用，请重试。"},
	} {
		h := newHarness(t, history.State{Phase: history.Failed, Error: tc.code})
		if !h.ui.HasText(tc.text) || h.ui.HasText("还没有播放记录。") {
			t.Fatal("failure became empty success", tc.code)
		}
		click(t, h, "重试")
		click(t, h, "关闭应用")
		if !slices.Equal(h.controller.calls, []string{"retry"}) || !slices.Equal(h.closes, []bool{false}) {
			t.Fatal("retry/close routing")
		}
		if tc.code == history.InUse || tc.code == history.LockUnsupported || tc.code == history.Storage {
			if h.ui.HasText("重置历史记录…") {
				t.Fatal("reset exposed without acquired corruption")
			}
		}
	}
	dirty := newHarness(t, history.State{Phase: history.Failed, Error: history.Uncertain, Dirty: true, Records: []history.Record{record("123")}})
	if !dirty.ui.HasText("最新播放进度尚未确认保存，请重试。") {
		t.Fatal("uncertainty hidden")
	}
	click(t, dirty, "不确认保存并关闭")
	if !slices.Equal(dirty.closes, []bool{true}) {
		t.Fatal("explicit discard not routed")
	}
	dirty.controller.state.Busy = true
	dirty.ui.Frame()
	click(t, dirty, "不确认保存并关闭")
	click(t, dirty, "重试")
	if len(dirty.closes) != 1 || len(dirty.controller.calls) != 0 {
		t.Fatal("busy discard/retry accepted")
	}
	cleanup := newHarness(t, history.State{Phase: history.Failed, Error: history.Storage})
	cleanup.closing = true
	cleanup.ui.Frame()
	if !cleanup.ui.HasText("历史记录清理未完成，请重试。") || cleanup.ui.HasText("不确认保存并关闭") || cleanup.ui.HasText("关闭应用") {
		t.Fatal("cleanup error offered bypass")
	}
	click(t, cleanup, "重试")
	if !slices.Equal(cleanup.controller.calls, []string{"retry"}) || len(cleanup.closes) != 0 {
		t.Fatal("cleanup retry closed app")
	}
}

func TestSafeErrorsLayoutAndClosingGates(t *testing.T) {
	h := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{record("123")}})
	h.controller.err = errors.New("https://private.invalid/PRIVATE_HISTORY_ERROR")
	click(t, h, "继续播放")
	if !h.ui.HasText("操作未完成，请重试。") || strings.Contains(fmt.Sprint(h.ui.Texts(), h.ui.AccessAll()), "PRIVATE_HISTORY_ERROR") {
		t.Fatal("private error displayed")
	}
	for _, dark := range []bool{false, true} {
		h.ui.SetDark(dark)
		h.ui.Frame()
		for _, label := range []string{"继续播放", "从头播放", "删除记录…", "清空历史记录…"} {
			info, ok := h.ui.Access(label)
			if !ok || !info.Focusable || info.Bounds.W < 28 || info.Bounds.H < 28 || info.Bounds.X < 0 || info.Bounds.X+info.Bounds.W > h.width+1 {
				t.Fatalf("pane action bounds %s %+v", label, info)
			}
		}
	}
	h.closing = true
	h.ui.Frame()
	before := len(h.continued)
	click(t, h, "继续播放")
	click(t, h, "清空历史记录…")
	if len(h.continued) != before || len(h.controller.calls) != 0 || h.view.confirm != noConfirmation {
		t.Fatal("closing accepted history mutation")
	}
	loading := newHarness(t, history.State{Phase: history.Loading, Busy: true})
	if !loading.ui.HasText("正在读取本地历史记录…") {
		t.Fatal("loading not visible")
	}
}

func TestPlayCallbackCanRetireHistoryView(t *testing.T) {
	h := newHarness(t, history.State{Phase: history.Ready, Records: []history.Record{record("123"), record("456")}})
	h.view.actions.Continue = func(history.Record) error { h.view.Close(); return nil }
	click(t, h, "继续播放")
	h.ui.Frame()
	if !h.view.closed || h.view.list != nil || h.view.controller != nil {
		t.Fatal("retired history view retained ownership")
	}
}
