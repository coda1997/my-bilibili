package desktop

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	accountview "my-bilibili/internal/desktop/account"
	catalogview "my-bilibili/internal/desktop/catalog"
	historyview "my-bilibili/internal/desktop/history"
	"my-bilibili/internal/history"
)

type shellSearch struct {
	state catalog.SearchState
	calls []string
}

func (s *shellSearch) State() catalog.SearchState {
	value := s.state
	value.Results = slices.Clone(value.Results)
	return value
}
func (s *shellSearch) Search(query string) error {
	s.calls = append(s.calls, "search:"+query)
	return nil
}
func (s *shellSearch) NextPage() error { s.calls = append(s.calls, "next"); return nil }
func (s *shellSearch) Retry() error    { s.calls = append(s.calls, "retry"); return nil }

type shellCovers struct{}

func (shellCovers) Render(c *gui.Context, _ string, w, h float32) {
	gui.Box(c).Size(w, h).Label("Synthetic cover")
}

type shellHistory struct {
	state history.State
	calls []string
}

func (s *shellHistory) State() history.State {
	value := s.state
	value.Records = slices.Clone(value.Records)
	return value
}
func (s *shellHistory) Delete(catalog.PartRef) error { s.calls = append(s.calls, "delete"); return nil }
func (s *shellHistory) Clear() error                 { s.calls = append(s.calls, "clear"); return nil }
func (s *shellHistory) ResetCorrupt() error          { s.calls = append(s.calls, "reset"); return nil }
func (s *shellHistory) Retry() error                 { s.calls = append(s.calls, "retry"); return nil }

type shellAccount struct {
	state         account.State
	calls         []string
	reads, closes int
	err           error
}

func (s *shellAccount) Snapshot() account.State  { s.reads++; return s.state }
func (s *shellAccount) record(name string) error { s.calls = append(s.calls, name); return s.err }
func (s *shellAccount) Restore() error           { return s.record("restore") }
func (s *shellAccount) Begin() error             { return s.record("begin") }
func (s *shellAccount) Cancel() error            { return s.record("cancel") }
func (s *shellAccount) RetrySave() error         { return s.record("save") }
func (s *shellAccount) RetryVerify() error       { return s.record("verify") }
func (s *shellAccount) Logout() error            { return s.record("logout") }
func (s *shellAccount) DiscardUnsaved() error    { return s.record("discard") }
func (s *shellAccount) RequestClose(bool) bool   { s.closes++; return false }

type clientHarness struct {
	*harness
	client              *Client
	status              ClientStatus
	search              *shellSearch
	recent              *shellHistory
	account             *shellAccount
	opened              []catalog.Entry
	continued, replayed []history.Record
	closes              []bool
	onOpen              func(catalog.Entry) error
	onContinue          func(history.Record) error
}

func newClientHarness(t *testing.T, width, height int) *clientHarness {
	t.Helper()
	h := &clientHarness{
		harness: newRealHarness(t),
		search:  &shellSearch{state: catalog.SearchState{Phase: catalog.SearchReady, Page: 1, TotalPages: 1, Results: []catalog.Summary{{ID: "BV0123456789", Title: "搜索结果", Creator: "测试作者", CoverID: "synthetic-cover"}}}},
		recent:  &shellHistory{state: history.State{Phase: history.Ready, Records: []history.Record{{Ref: catalog.PartRef{Video: "BV0123456789", Part: "123"}, Title: "历史视频", PartTitle: "第一集", Position: 17 * time.Second, Duration: time.Minute}}}},
		account: &shellAccount{state: account.State{Phase: account.SignedOut}},
		status:  ClientStatus{AccountPhase: account.SignedOut, HistoryPhase: history.Ready},
	}
	h.client = &Client{Player: h.view}
	var err error
	h.client.Catalog, err = catalogview.New(h.search, func(entry catalog.Entry) error {
		h.opened = append(h.opened, entry)
		if h.onOpen != nil {
			return h.onOpen(entry)
		}
		return nil
	}, shellCovers{})
	if err != nil {
		t.Fatal(err)
	}
	h.client.History, err = historyview.New(h.recent, historyview.Actions{
		Continue: func(record history.Record) error {
			h.continued = append(h.continued, record)
			if h.onContinue != nil {
				return h.onContinue(record)
			}
			return nil
		},
		Replay:       func(record history.Record) error { h.replayed = append(h.replayed, record); return nil },
		RequestClose: func(discard bool) { h.closes = append(h.closes, discard) },
	})
	if err != nil {
		t.Fatal(err)
	}
	h.client.Account = accountview.NewPane(h.account, h.client.ShowCatalog)
	h.ui = gui.NewTester(func(c *gui.Context) { h.client.Render(c, h.status) }, width, height)
	return h
}

func TestClientCompactWideAccountAndFullscreenLayout(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	before := h.player.State()
	rect(t, h.harness, "搜索关键词")
	if h.client.PlayerVisible() || h.ui.HasText("视频画面") {
		t.Fatal("compact discovery mounted player")
	}
	click(t, h.harness, "正在播放")
	if !h.client.PlayerVisible() {
		t.Fatal("compact player not mounted")
	}
	video := rect(t, h.harness, "视频画面")
	if video.X < 0 || video.Y < 52 || video.X+video.W > 800 || video.Y+video.H > 600 {
		t.Fatalf("compact video bounds=%+v", video)
	}
	click(t, h.harness, "历史")
	if h.client.PlayerVisible() || h.ui.HasText("视频画面") || !h.ui.HasText("本地历史记录") {
		t.Fatal("compact history mounted wrong pane")
	}
	click(t, h.harness, "账号")
	if h.client.PlayerVisible() || h.ui.HasText("视频画面") || h.ui.HasText("搜索关键词") {
		t.Fatal("account did not replace body")
	}
	copy := "登录仅保存在这台 Mac 的钥匙串中。会话失效时需要重新登录。"
	if r := rect(t, h.harness, copy); r.X != 24 || r.W != 752 {
		t.Fatalf("account was squeezed or double padded: %+v", r)
	}
	h.ui.SetSize(1320, 800)
	if r := rect(t, h.harness, copy); r.X != 24 || r.W != 1272 {
		t.Fatalf("wide account was squeezed: %+v", r)
	}
	click(t, h.harness, "发现")
	search, video := rect(t, h.harness, "搜索关键词"), rect(t, h.harness, "视频画面")
	if !h.client.PlayerVisible() || search.X+search.W > video.X || video.X+video.W > 1320 {
		t.Fatalf("wide pane bounds: search=%+v video=%+v", search, video)
	}
	click(t, h.harness, "历史")
	click(t, h.harness, "正在播放")
	if !h.client.PlayerVisible() || !h.ui.HasText("本地历史记录") {
		t.Fatal("wide player lost remembered history sidebar")
	}
	h.full = true
	h.ui.Frame()
	video = rect(t, h.harness, "视频画面")
	if video != (gui.Rect{W: 1320, H: 800}) || !h.client.PlayerVisible() || h.ui.HasText("本地历史记录") || h.ui.HasText("My Bilibili") || h.ui.HasText("搜索关键词") {
		t.Fatalf("fullscreen retained shell: %+v", video)
	}
	if !reflect.DeepEqual(before, h.player.State()) || h.player.ticks != 0 || len(h.player.calls) != 0 {
		t.Fatal("layout or tabs changed player lifecycle/state")
	}
}

func TestClientPaneTransitionsAreExplicitAndAccountHideErrorsStayVisible(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	h.account.state.Phase = account.Generating
	h.client.ShowAccount()
	h.ui.Frame()
	h.client.ShowAccount()
	for range 3 {
		h.ui.Frame()
	}
	if !h.client.Account.Visible() || len(h.account.calls) != 0 || h.account.closes != 0 {
		t.Fatal("painting/showing an account pane started work")
	}
	h.account.err = errors.New("private cancel error sentinel")
	h.client.ShowHistory()
	h.ui.Frame()
	if h.client.Account.Visible() || !slices.Equal(h.account.calls, []string{"cancel"}) || h.account.closes != 0 {
		t.Fatal("hiding account failed cancellation or took owner close authority")
	}
	if !h.ui.HasText("账号面板已收起，但取消登录未完成") || h.ui.HasText("private cancel error sentinel") {
		t.Fatal("account Hide error was dropped or exposed")
	}
	reads := h.account.reads
	h.client.ShowPlayer()
	for range 3 {
		h.ui.Frame()
	}
	if h.account.reads != reads || len(h.account.calls) != 1 {
		t.Fatal("hidden painting or another tab repeated account Hide work")
	}
	if len(h.search.calls) != 0 || len(h.recent.calls) != 0 || len(h.closes) != 0 || len(h.player.calls) != 0 {
		t.Fatal("pane transition took controller lifecycle authority")
	}
}

func TestClientTabsRetireCoverDemandAndHistoryConfirmation(t *testing.T) {
	h := newClientHarness(t, 1320, 800)
	if ids := h.client.Catalog.VisibleCoverIDs(); !slices.Equal(ids, []string{"synthetic-cover"}) {
		t.Fatalf("test did not establish visible cover demand: %v", ids)
	}
	h.client.ShowAccount()
	if len(h.client.Catalog.VisibleCoverIDs()) != 0 {
		t.Fatal("explicit account transition retained hidden search cover demand")
	}
	h.client.ShowHistory()
	h.ui.Frame()
	click(t, h.harness, "清空历史记录…")
	if !h.ui.HasText("确认清空这台设备上的全部历史记录？") {
		t.Fatal("test did not open history confirmation")
	}
	h.client.ShowPlayer()
	h.client.ShowHistory()
	h.ui.Frame()
	if h.ui.HasText("确认清空这台设备上的全部历史记录？") || len(h.recent.calls) != 0 {
		t.Fatal("tab transition retained a destructive confirmation or executed it")
	}
}

func TestClientPlayerVisibilityTracksMountAndCurrentReference(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	if h.client.PlayerVisible() {
		t.Fatal("unmounted player visible")
	}
	h.client.ShowPlayer()
	if h.client.PlayerVisible() {
		t.Fatal("tab intent was treated as a completed mount")
	}
	h.ui.Frame()
	if !h.client.PlayerVisible() {
		t.Fatal("mounted player invisible")
	}
	replacement := newRealHarness(t)
	h.client.Player = replacement.view
	if h.client.PlayerVisible() {
		t.Fatal("old mount describes replacement")
	}
	h.ui.Frame()
	if !h.client.PlayerVisible() {
		t.Fatal("replacement mount invisible")
	}
	h.client.ShowAccount()
	if h.client.PlayerVisible() {
		t.Fatal("account transition retained player visibility")
	}
	h.ui.Frame()
	h.client.ShowPlayer()
	h.ui.Frame()
	replacement.full = true
	if h.client.PlayerVisible() {
		t.Fatal("old geometry described fullscreen transition")
	}
	h.ui.Frame()
	if !h.client.PlayerVisible() {
		t.Fatal("fullscreen player invisible")
	}
	replacement.full = false
	if h.client.PlayerVisible() {
		t.Fatal("old fullscreen mount survived native state change")
	}
	h.ui.Frame()
	h.client.Player = nil
	if h.client.PlayerVisible() {
		t.Fatal("nil player visible")
	}
	h.ui.Frame()
	if !h.ui.HasText("选择视频后在这里播放。") {
		t.Fatal("no-player placeholder missing")
	}
}

func TestClientReplacementAndTabsRetireCapturedAndDeferredInput(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	h.client.ShowPlayer()
	h.ui.Frame()
	r := rect(t, h.harness, "播放进度")
	x, y := r.X+r.W/2, r.Y+r.H/2
	h.ui.Press(x, y)
	if len(h.player.calls) != 1 || !h.view.seek.pressed {
		t.Fatal("test did not capture old player slider")
	}
	replacement := newRealHarness(t)
	replacement.player.state.Playback.Position = 3 * time.Second
	h.client.Player = replacement.view
	h.ui.Frame()
	h.ui.Move(x+40, y)
	h.ui.Release(x+40, y)
	h.ui.Frame()
	if len(replacement.player.calls) != 0 {
		t.Fatal("new video inherited old native-generation-one capture")
	}

	h = newClientHarness(t, 800, 600)
	h.client.ShowPlayer()
	h.ui.Frame()
	realVideoClick(t, h.harness)
	h.client.ShowCatalog()
	h.ui.Advance(time.Second)
	h.client.ShowPlayer()
	h.ui.Frame()
	wantCalls(t, h.harness)
	click(t, h.harness, "画质")
	h.client.ShowHistory()
	h.client.ShowPlayer()
	h.ui.Frame()
	realPopup(t, h.harness, "")
	if !h.view.pendingClick.IsZero() {
		t.Fatal("unmounted player retained delayed click")
	}
}

func TestClientFeatureInputsOwnKeysBeforePlayerShortcuts(t *testing.T) {
	h := newClientHarness(t, 1320, 800)
	click(t, h.harness, "搜索关键词")
	h.ui.Type("测试")
	h.ui.TypeKey(0, gui.KeyF, "f")
	h.ui.TypeKey(0, gui.KeyM, "m")
	h.ui.TypeKey(0, gui.KeySpace, " ")
	for _, key := range []gui.Key{gui.KeyLeft, gui.KeyRight, gui.KeyUp, gui.KeyDown, gui.KeyEnter} {
		h.ui.Key(0, key)
	}
	if len(h.search.calls) != 1 || !strings.HasPrefix(h.search.calls[0], "search:") || h.full {
		t.Fatal("focused search input lost its native keys")
	}
	wantCalls(t, h.harness)
	click(t, h.harness, "链接或视频编号")
	h.ui.Type("BV0123456789")
	h.ui.Key(0, gui.KeyEnter)
	if len(h.opened) != 1 || h.opened[0] != (catalog.Entry{Kind: catalog.BVEntry, ID: "BV0123456789"}) {
		t.Fatal("native Enter did not submit exact parsed entry")
	}
	wantCalls(t, h.harness)
	h.ui.SetSize(800, 600)
	h.client.ShowHistory()
	h.ui.Frame()
	h.ui.ClickAt(0, 0)
	for _, key := range []gui.Key{gui.KeySpace, gui.KeyM, gui.KeyF, gui.KeyLeft, gui.KeyRight} {
		h.ui.Key(0, key)
	}
	if h.full {
		t.Fatal("hidden player consumed fullscreen shortcut")
	}
	wantCalls(t, h.harness)
	h.client.ShowPlayer()
	h.ui.Frame()
	h.ui.ClickAt(0, 0)
	h.ui.Key(0, gui.KeySpace)
	h.ui.Key(0, gui.KeyM)
	h.ui.Key(0, gui.KeyRight)
	wantCalls(t, h.harness, call{name: "toggle"}, call{name: "mute"}, call{name: "seek", value: 6})
}

func TestClientToleratesSynchronousPaneAndPlayerRetirement(t *testing.T) {
	for _, feature := range []string{"catalog", "history"} {
		t.Run(feature, func(t *testing.T) {
			h := newClientHarness(t, 1320, 800)
			retired, stalePaints := false, 0
			h.view.Surface = func(c *gui.Context, w, height float32) {
				if retired {
					stalePaints++
				}
				gui.Box(c).Size(w, height)
			}
			retire := func() {
				retired = true
				h.client.Player = nil
				if feature == "catalog" {
					h.client.Catalog.Close()
					h.client.Catalog = nil
				} else {
					h.client.History.Close()
					h.client.History = nil
				}
				h.client.ShowPlayer()
			}
			if feature == "catalog" {
				h.onOpen = func(catalog.Entry) error { retire(); return nil }
			} else {
				h.onContinue = func(history.Record) error { retire(); return nil }
				h.client.ShowHistory()
			}
			h.ui.Frame()
			label := "播放视频"
			if feature == "history" {
				label = "继续播放"
			}
			click(t, h.harness, label)
			if !retired || stalePaints != 0 || h.client.PlayerVisible() || !h.ui.HasText("选择视频后在这里播放。") {
				t.Fatalf("retired pane or player was used again: stale=%d", stalePaints)
			}
			wantCalls(t, h.harness)
		})
	}
}

func TestClientNavigationRetryIsExplicitSafeAndRetirementTolerant(t *testing.T) {
	h := newClientHarness(t, 1320, 800)
	retries := 0
	h.status.Error, h.status.CanRetry = catalog.Timeout, true
	h.client.RetryNavigation = func() error { retries++; return errors.New("private navigation error sentinel") }
	h.ui.Frame()
	for range 3 {
		h.ui.Frame()
	}
	if retries != 0 {
		t.Fatal("painting retried navigation")
	}
	for _, phase := range []string{"switching", "closing"} {
		h.status.Switching, h.status.Closing = phase == "switching", phase == "closing"
		h.ui.Frame()
		if !access(t, h.harness, "重试打开视频").Disabled {
			t.Fatal("conflicting retry enabled")
		}
		click(t, h.harness, "重试打开视频")
	}
	if retries != 0 {
		t.Fatal("disabled retry invoked callback")
	}
	h.status.Switching, h.status.Closing = false, false
	h.ui.Frame()
	click(t, h.harness, "重试打开视频")
	if retries != 1 || !h.ui.HasText("当前内容暂不可用，请重试。") || h.ui.HasText("private navigation error sentinel") {
		t.Fatal("retry error was dropped or exposed")
	}
	h.client.RetryNavigation = func() error { retries++; h.client.Player = nil; return nil }
	click(t, h.harness, "重试打开视频")
	if retries != 2 || h.client.PlayerVisible() {
		t.Fatal("retry callback retirement was ignored")
	}
	h.status.CanRetry = false
	h.ui.Frame()
	if _, shown := h.ui.Find("重试打开视频"); shown {
		t.Fatal("unavailable navigation retry shown")
	}
	h.status.CanRetry = true
	h.client.RetryNavigation = nil
	h.ui.Frame()
	if _, shown := h.ui.Find("重试打开视频"); shown {
		t.Fatal("nil navigation callback shown")
	}
}

func TestClientKeepAccountCloseOnlyForIdleDeleteFailure(t *testing.T) {
	for _, test := range []struct {
		name                    string
		closing, busy, callback bool
		phase                   account.Phase
		allowed                 bool
	}{
		{"idle close failure", true, false, true, account.DeleteFailed, true},
		{"busy close failure", true, true, true, account.DeleteFailed, false},
		{"normal failure", false, false, true, account.DeleteFailed, false},
		{"normal signed in close", true, false, true, account.SignedIn, false},
		{"unsaved failure", true, false, true, account.SaveFailed, false},
		{"no close callback", true, false, false, account.DeleteFailed, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newClientHarness(t, 800, 600)
			keeps := 0
			if test.callback {
				h.client.KeepAccountAndClose = func() { keeps++ }
			}
			h.status.Closing, h.status.AccountBusy, h.status.AccountPhase = test.closing, test.busy, test.phase
			h.ui.Frame()
			_, shown := h.ui.Find("保留登录并关闭")
			if shown != test.allowed || keeps != 0 {
				t.Fatal("keep-close gate or repaint action violated")
			}
			if shown {
				click(t, h.harness, "保留登录并关闭")
				h.ui.Frame()
				if keeps != 1 {
					t.Fatal("keep-close was not one explicit action")
				}
			}
			if h.account.closes != 0 || len(h.account.calls) != 0 || len(h.closes) != 0 {
				t.Fatal("shell independently closed or changed a domain owner")
			}
		})
	}
}

func TestClientPassesHistoryClosingStateWithoutAutomaticRecovery(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	h.status.Closing, h.status.HistoryPhase, h.status.HistoryError = true, history.Failed, history.Storage
	h.recent.state.Phase, h.recent.state.Error = history.Failed, history.Storage
	h.client.ShowHistory()
	h.ui.Frame()
	_, closeOffered := h.ui.AccessByRole(gui.RoleButton, "关闭应用")
	_, discardOffered := h.ui.AccessByRole(gui.RoleButton, "不确认保存并关闭")
	if !h.ui.HasText("历史记录清理未完成，请重试。") || closeOffered || discardOffered {
		t.Fatal("terminal cleanup failure was treated as ordinary history load failure")
	}
	if len(h.recent.calls) != 0 || len(h.closes) != 0 {
		t.Fatal("history failure recovered automatically")
	}
	click(t, h.harness, "重试")
	if !slices.Equal(h.recent.calls, []string{"retry"}) {
		t.Fatal("history recovery lost explicit action")
	}
	h.recent.state.Dirty, h.status.HistoryDirty = true, true
	h.ui.Frame()
	click(t, h.harness, "不确认保存并关闭")
	if !slices.Equal(h.closes, []bool{true}) {
		t.Fatal("history discard choice did not reach its existing controller action")
	}
}

func TestClientStatusCopyIsSafeAndNilViewsRemainUsable(t *testing.T) {
	h := newClientHarness(t, 800, 600)
	for _, phase := range []struct {
		phase account.Phase
		text  string
	}{{account.SignedOut, "账号 · 未登录"}, {account.SignedIn, "账号 · 已登录"}, {account.Expired, "账号 · 登录失效"}} {
		h.status.AccountPhase = phase.phase
		h.ui.Frame()
		if !h.ui.HasText(phase.text) {
			t.Fatalf("account phase not distinguishable: %s", phase.text)
		}
	}
	sentinel := "private-status-sentinel"
	h.status = ClientStatus{AccountPhase: account.Phase(sentinel), AccountError: account.Code(sentinel), HistoryPhase: history.Phase(sentinel), HistoryError: history.Code(sentinel), Error: catalog.Code(sentinel)}
	h.ui.Frame()
	if h.ui.HasText(sentinel) || h.ui.HasText("LOCAL PREVIEW") {
		t.Fatal("shell exposed arbitrary status text")
	}
	empty := &Client{}
	ui := gui.NewTester(func(c *gui.Context) { empty.Render(c, ClientStatus{}) }, 800, 600)
	for _, show := range []func(){empty.ShowCatalog, empty.ShowHistory, empty.ShowPlayer, empty.ShowAccount} {
		show()
		ui.Frame()
		if empty.PlayerVisible() {
			t.Fatal("nil pane produced player visibility")
		}
	}
}
