package account

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	gui "github.com/egoist/mygo/ui"
	domain "my-bilibili/internal/account"
)

type paneController struct {
	*fakeController
	reads, closes int
}

func (p *paneController) Snapshot() domain.State { p.reads++; return p.state }
func (p *paneController) RequestClose(bool) bool { p.closes++; return false }
func (p *paneController) DiscardUnsaved() error {
	if err := p.call("discard"); err != nil {
		return err
	}
	p.state.Unsaved = false
	p.state.Phase = domain.SignedOut
	if p.state.Stored {
		p.state.Phase = domain.Expired
	}
	return nil
}

type paneHarness struct {
	controller         *paneController
	view               *View
	ui                 *gui.Tester
	dismisses          int
	dismissedAfterHide bool
}

func pane(s domain.State) *paneHarness {
	h := &paneHarness{controller: &paneController{fakeController: &fakeController{state: s}}}
	h.view = NewPane(h.controller, func() {
		h.dismisses++
		h.dismissedAfterHide = !h.view.Visible() && h.view.bitmap == nil && h.view.qr.Empty()
	})
	h.ui = gui.NewTester(h.view.Render, 440, 640)
	return h
}
func (h *paneHarness) show() { h.view.Show(); h.ui.Frame() }
func paneClick(t *testing.T, h *paneHarness, label string) {
	t.Helper()
	if h.ui.Click(label) != nil {
		t.Fatal("pane action unavailable")
	}
	h.ui.Frame()
}
func safePane(t *testing.T, h *paneHarness) {
	t.Helper()
	safeView(t, &viewHarness{controller: h.controller.fakeController, view: h.view, ui: h.ui})
}

func TestPaneConstructionShowAndHiddenRenderArePure(t *testing.T) {
	h := pane(domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 1, Busy: true})
	if h.view.Visible() || h.controller.reads != 0 || len(h.controller.calls) != 0 || len(h.ui.Texts()) != 0 {
		t.Fatal("constructing hidden pane observed or changed account")
	}
	if h.view.Hide() != nil || h.controller.reads != 0 {
		t.Fatal("hidden pane repeated a hide operation")
	}
	h.view.Show()
	if h.controller.reads != 0 || len(h.controller.calls) != 0 {
		t.Fatal("Show started account work")
	}
	h.ui.Frame()
	if h.view.bitmap == nil || !h.view.Visible() {
		t.Fatal("shown QR missing")
	}
	bitmap := h.view.bitmap
	h.show()
	if h.view.bitmap != bitmap || len(h.controller.calls) != 0 {
		t.Fatal("repeated Show recreated QR or account attempt")
	}
	if h.view.Hide() != nil || h.view.bitmap != nil || !h.view.qr.Empty() || h.view.Visible() {
		t.Fatal("Hide did not immediately retire QR references")
	}
	if !reflect.DeepEqual(h.controller.calls, []string{"cancel"}) || h.controller.closes != 0 || h.dismisses != 0 {
		t.Fatal("external Hide closed controller or sent a user dismissal")
	}
	reads := h.controller.reads
	h.controller.state = domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 2, Busy: true}
	h.ui.Frame()
	if len(h.ui.Texts()) != 0 || h.controller.reads != reads || h.view.bitmap != nil || !h.view.qr.Empty() {
		t.Fatal("late QR was retained/rendered while hidden")
	}
	if h.view.Hide() != nil || len(h.controller.calls) != 1 {
		t.Fatal("repeated Hide repeated cancellation")
	}
	h.controller.state = domain.State{Phase: domain.SignedOut}
	h.show()
	if len(h.controller.calls) != 1 || !h.ui.HasText("开始扫码登录") {
		t.Fatal("reopening pane auto-started another login")
	}
}

func TestHideCancelsOnlyActiveQRPhases(t *testing.T) {
	for _, phase := range []domain.Phase{domain.Initial, domain.Restoring, domain.Generating, domain.Waiting, domain.Scanned, domain.Saving, domain.SaveFailed, domain.Verifying, domain.VerifyFailed, domain.SignedIn, domain.Expired, domain.SigningOut, domain.DeleteFailed, domain.Canceling, domain.Closing, domain.Done} {
		h := pane(domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 1, Busy: true})
		h.show()
		h.view.confirmLogout = true
		h.controller.state = domain.State{Phase: phase, Stored: true, Unsaved: phase == domain.Saving || phase == domain.SaveFailed, Busy: true}
		if h.view.Hide() != nil {
			t.Fatal("valid hide returned an error")
		}
		want := []string(nil)
		if phase == domain.Generating || phase == domain.Waiting || phase == domain.Scanned {
			want = []string{"cancel"}
		}
		if !reflect.DeepEqual(h.controller.calls, want) || h.controller.closes != 0 || h.view.confirmLogout || h.view.bitmap != nil || !h.view.qr.Empty() {
			t.Fatal("Hide canceled a non-QR phase or retained presentation")
		}
	}
}

func TestPaneDismissButtonsPreserveReusableController(t *testing.T) {
	for _, item := range []struct {
		state domain.State
		label string
		want  []string
	}{
		{domain.State{Phase: domain.SignedIn, Stored: true}, "完成", nil},
		{domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Busy: true}, "关闭面板", []string{"cancel"}},
		{domain.State{Phase: domain.Saving, Unsaved: true, Busy: true}, "关闭面板", nil},
		{domain.State{Phase: domain.Verifying, Stored: true, Busy: true}, "关闭面板", nil},
		{domain.State{Phase: domain.SaveFailed, Unsaved: true}, "关闭面板", nil},
		{domain.State{Phase: domain.DeleteFailed, Stored: true}, "关闭面板", nil},
	} {
		h := pane(item.state)
		h.show()
		paneClick(t, h, item.label)
		if h.view.Visible() || h.dismisses != 1 || !h.dismissedAfterHide || h.controller.closes != 0 || !reflect.DeepEqual(h.controller.calls, item.want) {
			t.Fatal("pane dismissal requested app close, discarded state or ran before hide")
		}
		if item.state.Unsaved && !h.controller.state.Unsaved {
			t.Fatal("pane dismissal discarded an unconfirmed attempt")
		}
		h.show()
		if h.controller.closes != 0 || h.dismisses != 1 {
			t.Fatal("reopening pane changed its controller lifetime")
		}
		safePane(t, h)
	}
}

func TestPaneDiscardIsExplicitIdleAndDoesNotClose(t *testing.T) {
	for _, stored := range []bool{false, true} {
		h := pane(domain.State{Phase: domain.SaveFailed, Unsaved: true, Stored: stored})
		h.show()
		if len(h.controller.calls) != 0 {
			t.Fatal("render discarded credentials")
		}
		paneClick(t, h, "放弃本次登录")
		want := domain.SignedOut
		if stored {
			want = domain.Expired
		}
		if !reflect.DeepEqual(h.controller.calls, []string{"discard"}) || h.controller.state.Phase != want || h.controller.state.Unsaved || h.controller.state.Stored != stored || !h.view.Visible() || h.controller.closes != 0 || h.dismisses != 0 {
			t.Fatal("explicit discard did not preserve pane/store/controller semantics")
		}
	}
	for _, state := range []domain.State{{Phase: domain.SaveFailed, Unsaved: true, Busy: true}, {Phase: domain.SaveFailed, Unsaved: false}} {
		h := pane(state)
		h.show()
		paneClick(t, h, "放弃本次登录")
		if len(h.controller.calls) != 0 {
			t.Fatal("busy or absent unsaved attempt was discarded")
		}
	}
	h := pane(domain.State{Phase: domain.SaveFailed, Unsaved: true})
	h.controller.err = domain.Busy
	h.show()
	paneClick(t, h, "放弃本次登录")
	if !h.controller.state.Unsaved || !h.view.Visible() || !h.ui.HasText("当前操作尚未完成，请稍候。") {
		t.Fatal("failed discard hid pending state or safe feedback")
	}
}

func TestUnconfirmedSaveCopyDoesNotClaimAbsentPersistence(t *testing.T) {
	s := domain.State{Phase: domain.SaveFailed, Unsaved: true, Error: domain.Storage}
	standalone := harness(s)
	embedded := pane(s)
	embedded.show()
	for _, ui := range []*gui.Tester{standalone.ui, embedded.ui} {
		text := strings.Join(ui.Texts(), " ")
		if !ui.HasText("登录保存未确认") || !strings.Contains(text, "暂时无法确认钥匙串保存结果") || !strings.Contains(text, "已写入钥匙串的数据会保留") || strings.Contains(text, "尚未保存") || strings.Contains(text, "没有保存") {
			t.Fatal("uncertain readback was presented as definite non-persistence")
		}
	}
	if !standalone.ui.HasText("放弃并关闭") || !embedded.ui.HasText("放弃本次登录") || embedded.ui.HasText("放弃并关闭") {
		t.Fatal("pane and whole-window close choices were conflated")
	}
}

func TestHideErrorsAreSafeAndConfirmationIsRetired(t *testing.T) {
	for _, item := range []struct {
		err  error
		want domain.Code
	}{{errors.New("PRIVATE_CONTROLLER_ERROR"), domain.Invalid}, {domain.Code("PRIVATE_CONTROLLER_ERROR"), domain.Invalid}, {domain.Busy, domain.Busy}} {
		h := pane(domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 1, Busy: true})
		h.show()
		h.controller.err = item.err
		if h.view.Hide() != item.want || h.view.actionError != item.want || h.view.Visible() || h.view.bitmap != nil || h.controller.closes != 0 {
			t.Fatal("failed cancellation leaked error or prevented local hide")
		}
		h.show()
		safePane(t, h)
	}
	h := pane(domain.State{Phase: domain.SignedIn, Stored: true, Generation: 7})
	h.show()
	paneClick(t, h, "退出登录…")
	if !h.view.confirmLogout {
		t.Fatal("logout confirmation missing")
	}
	if h.view.Hide() != nil {
		t.Fatal("signed-in pane hide failed")
	}
	h.show()
	if h.view.confirmLogout || h.ui.HasText("确认退出登录") || len(h.controller.calls) != 0 {
		t.Fatal("reopening pane retained stale logout confirmation")
	}
}

func TestPaneUsesControllerPortAndCannotBypassMutationRefusal(t *testing.T) {
	// This is a test-only refusal port, not a production verification mode.
	h := pane(domain.State{Phase: domain.SignedOut})
	h.controller.err = domain.Invalid
	h.show()
	paneClick(t, h, "开始扫码登录")
	if h.controller.state.Phase != domain.SignedOut || h.controller.closes != 0 || !h.ui.HasText("操作未完成，请重试。") {
		t.Fatal("view bypassed the injected controller's refusal")
	}
	safePane(t, h)
}

func TestPaneKeyboardAndHiddenActions(t *testing.T) {
	for _, dark := range []bool{false, true} {
		h := pane(domain.State{Phase: domain.SignedOut})
		h.show()
		h.ui.SetDark(dark)
		for _, size := range [][2]int{{320, 560}, {440, 640}, {800, 600}} {
			h.ui.SetSize(size[0], size[1])
			access, ok := h.ui.Access("开始扫码登录")
			if !ok || !access.Focusable || access.Bounds.X < 0 || access.Bounds.X+access.Bounds.W > float32(size[0]) {
				t.Fatal("pane action did not fit or accept focus")
			}
			safePane(t, h)
		}
		h.ui.Key(0, gui.KeyTab)
		h.ui.Key(0, gui.KeySpace)
		if !reflect.DeepEqual(h.controller.calls, []string{"begin"}) {
			t.Fatal("pane keyboard action unavailable")
		}
		if h.view.Hide() != nil {
			t.Fatal("pane hide failed")
		}
		h.ui.Frame()
		h.ui.Key(0, gui.KeySpace)
		if len(h.controller.calls) != 1 || len(h.ui.Texts()) != 0 {
			t.Fatal("hidden pane still accepted a keyboard action")
		}
	}
}
