package catalogview

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
)

type fakeController struct {
	state catalog.SearchState
	calls []string
	err   error
}

func (f *fakeController) State() catalog.SearchState {
	s := f.state
	s.Results = slices.Clone(s.Results)
	return s
}
func (f *fakeController) Search(query string) error {
	value, err := catalog.ValidateQuery(query)
	if err != nil {
		return err
	}
	f.calls = append(f.calls, "search:"+value)
	if f.err != nil {
		return f.err
	}
	f.state.Query = value
	f.state.Phase = catalog.SearchLoading
	f.state.Results = nil
	f.state.Generation++
	return nil
}
func (f *fakeController) NextPage() error {
	f.calls = append(f.calls, "next")
	if f.err == nil {
		f.state.LoadingMore = true
	}
	return f.err
}
func (f *fakeController) Retry() error { f.calls = append(f.calls, "retry"); return f.err }

type fakeCovers struct{ drawn map[string]bool }

func (f *fakeCovers) Render(c *gui.Context, id string, w, h float32) {
	f.drawn[id] = true
	gui.Box(c).Size(w, h).Background(c.Theme().Surface)
}

type harness struct {
	view          *View
	controller    *fakeController
	covers        *fakeCovers
	ui            *gui.Tester
	opened        []catalog.Entry
	shortcuts     int
	width, height float32
}

func newHarness(t *testing.T, state catalog.SearchState) *harness {
	t.Helper()
	h := &harness{controller: &fakeController{state: state}, covers: &fakeCovers{drawn: map[string]bool{}}, width: 280, height: 500}
	v, err := New(h.controller, func(e catalog.Entry) error { h.opened = append(h.opened, e); return h.controller.err }, h.covers)
	if err != nil {
		t.Fatal(err)
	}
	h.view = v
	h.ui = gui.NewTester(func(c *gui.Context) {
		h.view.Render(c, h.width, h.height)
		h.view.VisibleCoverIDs()
		for _, key := range []gui.Key{gui.KeySpace, gui.KeyF, gui.KeyM, gui.KeyRight} {
			if c.Shortcut(0, key) {
				h.shortcuts++
			}
		}
	}, 800, 600)
	return h
}
func click(t *testing.T, h *harness, label string) {
	t.Helper()
	if err := h.ui.Click(label); err != nil {
		t.Fatal(err)
	}
}
func summary(n int) catalog.Summary {
	return catalog.Summary{ID: catalog.VideoID(fmt.Sprintf("BV%010d", n)), Title: fmt.Sprintf("原创测试视频 %d", n), Creator: "测试作者", CoverID: fmt.Sprintf("cover-%d", n)}
}

func TestExplicitSearchOpenAndNativeInputFocus(t *testing.T) {
	h := newHarness(t, catalog.SearchState{Phase: catalog.SearchInitial})
	if len(h.controller.calls) != 0 || len(h.opened) != 0 {
		t.Fatal("construction/render dispatched action")
	}
	click(t, h, "搜索关键词")
	h.ui.Type("中文测试")
	for range 5 {
		h.ui.Frame()
	}
	if len(h.controller.calls) != 0 {
		t.Fatal("typing searched")
	}
	h.ui.Key(0, gui.KeyEnter)
	if !slices.Equal(h.controller.calls, []string{"search:中文测试"}) || len(h.opened) != 0 {
		t.Fatalf("search action %+v %+v", h.controller.calls, h.opened)
	}
	if !h.ui.Focused("搜索关键词") {
		t.Fatal("search completion stole input focus")
	}
	h.ui.TypeKey(0, gui.KeyF, "f")
	h.ui.TypeKey(0, gui.KeySpace, " ")
	h.ui.Key(0, gui.KeyRight)
	if h.shortcuts != 0 {
		t.Fatal("player shortcuts stole input")
	}
	entry := newHarness(t, catalog.SearchState{Phase: catalog.SearchInitial})
	click(t, entry, "链接或视频编号")
	entry.ui.Type("https://www.bilibili.com/video/BV0123456789?p=2")
	entry.ui.Key(0, gui.KeyEnter)
	if len(entry.opened) != 1 || entry.opened[0] != (catalog.Entry{Kind: catalog.BVEntry, ID: "BV0123456789", Page: 2}) || len(entry.controller.calls) != 0 {
		t.Fatalf("entry action %+v", entry.opened)
	}
	if !entry.ui.Focused("链接或视频编号") {
		t.Fatal("entry submit stole focus")
	}
}

func TestInvalidEntrySafeErrorsAndRetiredActions(t *testing.T) {
	h := newHarness(t, catalog.SearchState{Phase: catalog.SearchInitial})
	h.view.entry = "https://private.invalid/unsupported"
	h.ui.Frame()
	click(t, h, "打开链接或编号")
	if len(h.opened) != 0 || !h.ui.HasText("暂不支持此类内容或链接。") {
		t.Fatal("invalid entry reached callback")
	}
	h.view.query = ""
	click(t, h, "搜索")
	if len(h.controller.calls) != 0 {
		t.Fatal("invalid query reached service action")
	}
	h.view.query = "original"
	h.controller.err = errors.New("https://secret.invalid/SERVER_SENTINEL")
	h.ui.Frame()
	click(t, h, "搜索")
	if !h.ui.HasText("当前内容暂不可用，请重试。") || strings.Contains(h.view.actionError, "SERVER_SENTINEL") || strings.Contains(fmt.Sprint(h.ui.Texts(), h.ui.AccessAll()), "SERVER_SENTINEL") {
		t.Fatal("unsafe source error")
	}
	h.controller.err = catalog.Code("PRIVATE_CODE_SENTINEL")
	click(t, h, "搜索")
	if strings.Contains(h.view.actionError, "PRIVATE_CODE_SENTINEL") {
		t.Fatal("unsafe code retained")
	}
	before := len(h.controller.calls)
	h.view.Close()
	h.ui.Frame()
	if len(h.controller.calls) != before || h.view.controller != nil || h.view.covers != nil || h.view.open != nil || len(h.view.VisibleCoverIDs()) != 0 {
		t.Fatal("retirement retained actions or references")
	}
	closed := newHarness(t, catalog.SearchState{Phase: catalog.SearchClosed})
	closed.view.query = "new"
	closed.ui.Frame()
	click(t, closed, "搜索")
	if len(closed.controller.calls) != 0 {
		t.Fatal("closed state submitted")
	}
}

func TestSearchLoadingEmptyAppendRetryAndPageBounds(t *testing.T) {
	for _, tc := range []struct {
		state catalog.SearchState
		text  string
	}{
		{catalog.SearchState{Phase: catalog.SearchLoading}, "正在搜索…"},
		{catalog.SearchState{Phase: catalog.SearchReady}, "没有找到相关视频。"},
		{catalog.SearchState{Phase: catalog.SearchFailed, Error: catalog.Restricted}, "当前内容受限，暂时无法查看。"},
		{catalog.SearchState{Phase: catalog.SearchFailed, Error: catalog.LoginRequired}, "需要登录后才能查看此内容。"},
		{catalog.SearchState{Phase: catalog.SearchFailed, Error: catalog.SessionExpired}, "登录已失效，请重新登录。"},
		{catalog.SearchState{Phase: catalog.SearchFailed, Error: catalog.VerificationRequired}, "服务需要进一步验证。"},
	} {
		h := newHarness(t, tc.state)
		if !h.ui.HasText(tc.text) {
			t.Fatalf("missing state %s", tc.text)
		}
	}
	state := catalog.SearchState{Phase: catalog.SearchReady, Results: []catalog.Summary{summary(1)}, Page: 1, TotalPages: 3, LoadingMore: true}
	h := newHarness(t, state)
	if !h.ui.HasText("原创测试视频 1") || !h.ui.HasText("正在加载下一页…") {
		t.Fatal("append removed accepted rows")
	}
	click(t, h, "加载下一页")
	if len(h.controller.calls) != 0 {
		t.Fatal("duplicate page queued")
	}
	h.controller.state.LoadingMore = false
	h.controller.state.Error = catalog.Network
	h.ui.Frame()
	if !h.ui.HasText("原创测试视频 1") {
		t.Fatal("failed append removed rows")
	}
	click(t, h, "重试搜索")
	if !slices.Equal(h.controller.calls, []string{"retry"}) {
		t.Fatal("wrong retry")
	}
	h.controller.calls = nil
	h.controller.state.Error = catalog.None
	h.ui.Frame()
	click(t, h, "加载下一页")
	if !slices.Equal(h.controller.calls, []string{"next"}) {
		t.Fatal("next page not dispatched")
	}
	h.controller.calls = nil
	h.controller.state.LoadingMore = false
	h.controller.state.AtLimit = true
	h.ui.Frame()
	click(t, h, "加载下一页")
	if len(h.controller.calls) != 0 || !h.ui.HasText("已达到本次搜索的显示上限。") {
		t.Fatal("limit not preserved")
	}
	h.controller.state.AtLimit = false
	h.controller.state.Page = 3
	h.ui.Frame()
	click(t, h, "加载下一页")
	if len(h.controller.calls) != 0 {
		t.Fatal("past final page")
	}
}

func TestViewportDemandPaneSizeAndStableResults(t *testing.T) {
	state := catalog.SearchState{Phase: catalog.SearchReady, Query: "original", Page: 1, TotalPages: 2}
	for i := range 50 {
		state.Results = append(state.Results, summary(i))
	}
	h := newHarness(t, state)
	first := h.view.VisibleCoverIDs()
	if len(first) == 0 || len(first) > 6 || len(first) >= len(h.covers.drawn) {
		t.Fatalf("overscan or nonviewport demand: visible %d built %d", len(first), len(h.covers.drawn))
	}
	first[0] = "mutated"
	if h.view.VisibleCoverIDs()[0] == "mutated" {
		t.Fatal("visible demand aliases view")
	}
	for _, dark := range []bool{false, true} {
		h.ui.SetDark(dark)
		h.ui.Frame()
		for _, label := range []string{"搜索关键词", "链接或视频编号", "搜索", "打开链接或编号"} {
			info, ok := h.ui.Access(label)
			if !ok || info.Bounds.W < 28 || info.Bounds.H < 28 || info.Bounds.X < 0 || info.Bounds.X+info.Bounds.W > h.width+1 {
				t.Fatalf("pane bounds %s %+v", label, info)
			}
		}
	}
	before := slices.Clone(h.view.VisibleCoverIDs())
	info, ok := h.ui.Access("播放视频")
	if !ok {
		t.Fatal("row action missing")
	}
	h.ui.Scroll(info.Bounds.X+10, info.Bounds.Y+10, 0, 700)
	h.ui.Frame()
	after := h.view.VisibleCoverIDs()
	if len(after) == 0 || slices.Equal(before, after) {
		t.Fatal("viewport did not follow native scroll")
	}
	if len(h.controller.calls) != 0 || len(h.opened) != 0 {
		t.Fatal("scroll initiated content action")
	}
	h.view.Hide()
	if len(h.view.VisibleCoverIDs()) != 0 {
		t.Fatal("hidden pane retained demand")
	}
	h.ui.Frame()
	if len(h.view.VisibleCoverIDs()) == 0 {
		t.Fatal("shown pane did not restore viewport")
	}
	h.controller.state.Results = nil
	h.controller.state.Query = "another"
	h.controller.state.Phase = catalog.SearchLoading
	h.ui.Frame()
	if len(h.view.VisibleCoverIDs()) != 0 {
		t.Fatal("new search retained previous demand")
	}
}

func TestResultCallbackUsesCopiedCanonicalEntry(t *testing.T) {
	h := newHarness(t, catalog.SearchState{Phase: catalog.SearchReady, Results: []catalog.Summary{summary(7)}})
	click(t, h, "播放视频")
	if len(h.opened) != 1 || h.opened[0].Kind != catalog.BVEntry || h.opened[0].ID != "BV0000000007" || h.opened[0].Page != 0 {
		t.Fatalf("wrong result identity %+v", h.opened)
	}
	if _, err := New(nil, func(catalog.Entry) error { return nil }, h.covers); err != catalog.Invalid {
		t.Fatal(err)
	}
}

func TestNavigationCallbackCanRetireItsView(t *testing.T) {
	for _, label := range []string{"打开链接或编号", "播放视频"} {
		t.Run(label, func(t *testing.T) {
			h := newHarness(t, catalog.SearchState{Phase: catalog.SearchReady, Results: []catalog.Summary{summary(1), summary(2)}})
			h.view.entry = "BV0123456789"
			h.view.open = func(catalog.Entry) error { h.view.Close(); return nil }
			h.ui.Frame()
			click(t, h, label)
			h.ui.Frame()
			if !h.view.closed || h.view.covers != nil || h.view.controller != nil {
				t.Fatal("navigation did not retire view")
			}
		})
	}
}
