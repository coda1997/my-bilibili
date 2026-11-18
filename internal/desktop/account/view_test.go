package account

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	domain "my-bilibili/internal/account"
)

const privateQR = "https://account.bilibili.com/h5/account-h5/auth/scan-web?qrcode_key=SYNTHETIC_PRIVATE_QR"

type fakeController struct {
	state domain.State
	calls []string
	err   error
}

func (f *fakeController) Snapshot() domain.State { return f.state }
func (f *fakeController) call(s string) error    { f.calls = append(f.calls, s); return f.err }
func (f *fakeController) Restore() error         { return f.call("restore") }
func (f *fakeController) Begin() error           { return f.call("begin") }
func (f *fakeController) RetrySave() error       { return f.call("save") }
func (f *fakeController) RetryVerify() error     { return f.call("verify") }
func (f *fakeController) Logout() error          { return f.call("logout") }
func (f *fakeController) Cancel() error {
	err := f.call("cancel")
	if err == nil {
		f.state.QR = domain.QR{}
		f.state.Phase = domain.Canceling
		f.state.Busy = true
	}
	return err
}
func (f *fakeController) RequestClose(bool) bool { return false }

type viewHarness struct {
	controller *fakeController
	view       *View
	ui         *gui.Tester
	closes     []bool
}

func harness(s domain.State) *viewHarness {
	h := &viewHarness{controller: &fakeController{state: s}}
	h.view = New(h.controller, func(discard bool) { h.closes = append(h.closes, discard) })
	h.ui = gui.NewTester(h.view.Render, 440, 640)
	return h
}
func click(t *testing.T, h *viewHarness, label string) {
	t.Helper()
	if err := h.ui.Click(label); err != nil {
		t.Fatal(err)
	}
}
func safeView(t *testing.T, h *viewHarness) {
	t.Helper()
	text := strings.Join(h.ui.Texts(), " ") + fmt.Sprint(h.ui.AccessAll())
	for _, secret := range []string{privateQR, "SYNTHETIC_PRIVATE_QR", "PRIVATE_CONTROLLER_ERROR"} {
		if strings.Contains(text, secret) {
			t.Fatal("private value reached text or accessibility metadata")
		}
	}
}
func TestQRImageQuietZoneAndIntegerPixels(t *testing.T) {
	for _, limit := range []int{272, 320} {
		img, err := qrImage(domain.NewQR(privateQR), limit)
		if err != nil {
			t.Fatal("synthetic QR encode failed")
		}
		n := img.Bounds().Dx()
		if n != img.Bounds().Dy() || n > limit {
			t.Fatal("QR does not fit square")
		}
		black := func(x, y int) bool {
			r, g, b, a := img.At(x, y).RGBA()
			if a != 65535 || r != g || g != b || (r != 0 && r != 65535) {
				t.Fatal("QR pixel is translucent or antialiased")
			}
			return r == 0
		}
		top, left := -1, -1
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				if black(x, y) && top < 0 {
					top, left = y, x
				}
			}
		}
		if top < 0 {
			t.Fatal("QR is blank")
		}
		run := 0
		for x := left; x < n && black(x, top); x++ {
			run++
		}
		if run%7 != 0 || run == 0 {
			t.Fatal("QR finder pattern lost integer modules")
		}
		scale := run / 7
		if top != 4*scale || left != 4*scale || n%scale != 0 {
			t.Fatal("QR quiet zone or scaling is wrong")
		}
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				if (x < 4*scale || y < 4*scale || x >= n-4*scale || y >= n-4*scale) && black(x, y) {
					t.Fatal("QR quiet zone not white")
				}
				if black(x, y) != black((x/scale)*scale, (y/scale)*scale) {
					t.Fatal("QR module stretched unevenly")
				}
			}
		}
	}
	if _, err := qrImage(domain.NewQR(strings.Repeat("x", 5000)), 320); err != domain.Invalid {
		t.Fatal("encoder failure not safely mapped")
	}
}
func TestQRCacheLifecycleAndExpiry(t *testing.T) {
	h := harness(domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 1, Busy: true})
	if h.view.bitmap == nil {
		t.Fatal("waiting QR absent")
	}
	first := h.view.bitmap
	h.ui.Frame()
	if h.view.bitmap != first {
		t.Fatal("unchanged QR reencoded")
	}
	h.controller.state.Phase = domain.Scanned
	h.ui.Frame()
	if h.view.bitmap != first {
		t.Fatal("scanned QR changed")
	}
	safeView(t, h)
	h.controller.state.Generation = 2
	h.ui.Frame()
	if h.view.bitmap == first || h.view.bitmap == nil {
		t.Fatal("new attempt did not retire bitmap")
	}
	h.controller.state.ExpiresAt = h.ui.Now().Add(time.Second)
	h.ui.Frame()
	h.ui.Advance(2 * time.Second)
	if h.view.bitmap != nil || !h.view.qr.Empty() || !h.ui.HasText("二维码已过期") {
		t.Fatal("expired QR remained visible/cached")
	}
	for _, phase := range []domain.Phase{domain.Saving, domain.SaveFailed, domain.SignedIn, domain.Expired, domain.Failed, domain.Canceling, domain.Closing, domain.Done} {
		h.controller.state = domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 3}
		h.ui.Frame()
		h.controller.state.Phase = phase
		h.ui.Frame()
		if h.view.bitmap != nil || !h.view.qr.Empty() {
			t.Fatal("terminal or non-QR phase retained challenge")
		}
		safeView(t, h)
	}
}
func TestNormalActionsAndSafeErrors(t *testing.T) {
	cases := []struct {
		s           domain.State
		label, call string
	}{
		{domain.State{Phase: domain.SignedOut}, "开始扫码登录", "begin"},
		{domain.State{Phase: domain.Expired, Stored: true}, "重新扫码登录", "begin"},
		{domain.State{Phase: domain.Failed, Error: domain.Storage}, "重试恢复", "restore"},
		{domain.State{Phase: domain.Failed, Error: domain.Network}, "重新生成二维码", "begin"},
		{domain.State{Phase: domain.SaveFailed, Unsaved: true}, "重试保存", "save"},
		{domain.State{Phase: domain.VerifyFailed, Stored: true}, "重新检查登录", "verify"},
		{domain.State{Phase: domain.DeleteFailed, Stored: true}, "重试退出", "logout"},
		{domain.State{Phase: domain.Verifying, Stored: true, Busy: true}, "取消检查", "cancel"},
	}
	for _, tc := range cases {
		t.Run(tc.call+string(tc.s.Phase), func(t *testing.T) {
			h := harness(tc.s)
			click(t, h, tc.label)
			if !reflect.DeepEqual(h.controller.calls, []string{tc.call}) {
				t.Fatal("wrong action dispatched")
			}
			safeView(t, h)
		})
	}
	h := harness(domain.State{Phase: domain.SignedOut})
	h.controller.err = errors.New("PRIVATE_CONTROLLER_ERROR")
	click(t, h, "开始扫码登录")
	safeView(t, h)
	if !h.ui.HasText("操作未完成，请重试。") {
		t.Fatal("unknown error has no safe feedback")
	}
	h.controller.err = domain.Busy
	click(t, h, "开始扫码登录")
	if !h.ui.HasText("当前操作尚未完成，请稍候。") {
		t.Fatal("known safe error missing")
	}
}
func TestCancelClearsQRCodeWithoutNewBegin(t *testing.T) {
	h := harness(domain.State{Phase: domain.Waiting, QR: domain.NewQR(privateQR), Generation: 1, Busy: true})
	click(t, h, "取消本次登录")
	if h.view.bitmap != nil || !reflect.DeepEqual(h.controller.calls, []string{"cancel"}) {
		t.Fatal("cancel retained QR or regenerated challenge")
	}
}
func TestLogoutRequiresConfirmationAndSavedClosePreserves(t *testing.T) {
	h := harness(domain.State{Phase: domain.SignedIn, Stored: true, Generation: 7})
	click(t, h, "退出登录…")
	h.ui.Frame()
	if len(h.controller.calls) != 0 || !h.ui.HasText("确认退出登录") {
		t.Fatal("logout skipped confirmation or confirmation disappeared")
	}
	click(t, h, "取消退出")
	if len(h.controller.calls) != 0 || h.view.confirmLogout {
		t.Fatal("cancel logout changed account")
	}
	click(t, h, "完成")
	if !reflect.DeepEqual(h.closes, []bool{false}) || len(h.controller.calls) != 0 {
		t.Fatal("ordinary saved close logged out")
	}
	click(t, h, "退出登录…")
	click(t, h, "确认退出登录")
	if !reflect.DeepEqual(h.controller.calls, []string{"logout"}) {
		t.Fatal("confirmed logout not dispatched")
	}
}
func TestUnsavedAndFailedDeleteExplicitClose(t *testing.T) {
	for _, tc := range []struct {
		s     domain.State
		label string
	}{
		{domain.State{Phase: domain.SaveFailed, Unsaved: true, CloseRequested: true}, "放弃并关闭"},
		{domain.State{Phase: domain.DeleteFailed, Stored: true, CloseRequested: true}, "保留登录并关闭"},
	} {
		h := harness(tc.s)
		if len(h.closes) != 0 {
			t.Fatal("view silently discarded state")
		}
		click(t, h, tc.label)
		if !reflect.DeepEqual(h.closes, []bool{true}) || len(h.controller.calls) != 0 {
			t.Fatal("explicit close altered stored credentials")
		}
	}
	h := harness(domain.State{Phase: domain.Saving, Busy: true, Unsaved: true})
	click(t, h, "关闭窗口")
	if !reflect.DeepEqual(h.closes, []bool{false}) || len(h.controller.calls) != 0 {
		t.Fatal("busy close bypassed root lifecycle gate")
	}
}
func TestThemeSizeAccessibilityAndKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		h := harness(domain.State{Phase: domain.SignedOut})
		h.ui.SetDark(dark)
		for _, size := range [][2]int{{400, 560}, {440, 640}, {700, 700}} {
			h.ui.SetSize(size[0], size[1])
			a, ok := h.ui.Access("开始扫码登录")
			if !ok || !a.Focusable || a.Bounds.H < 28 || a.Bounds.W < 28 || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > float32(size[0]) {
				t.Fatal("login action is not accessible or fits poorly")
			}
			safeView(t, h)
		}
		h.ui.Key(0, gui.KeyTab)
		h.ui.Key(0, gui.KeySpace)
		if len(h.controller.calls) != 1 || h.controller.calls[0] != "begin" {
			t.Fatal("keyboard could not activate login")
		}
	}
}
func TestAllSafePhasesAndBlockedActions(t *testing.T) {
	phases := []domain.Phase{domain.Initial, domain.Restoring, domain.SignedOut, domain.Generating, domain.Waiting, domain.Scanned, domain.Saving, domain.SaveFailed, domain.Verifying, domain.VerifyFailed, domain.SignedIn, domain.Expired, domain.Failed, domain.Canceling, domain.SigningOut, domain.DeleteFailed, domain.Closing, domain.Done, domain.Phase("PRIVATE_CONTROLLER_ERROR")}
	for _, p := range phases {
		h := harness(domain.State{Phase: p, Error: domain.Code("PRIVATE_CONTROLLER_ERROR"), Busy: true})
		safeView(t, h)
		if len(h.controller.calls) != 0 || len(h.closes) != 0 {
			t.Fatal("render made account action")
		}
	}
	h := harness(domain.State{Phase: domain.SignedIn, Stored: true, Restored: true})
	if !h.ui.HasText("已恢复登录") {
		t.Fatal("restore success missing")
	}
	h.controller.state = domain.State{Phase: domain.SaveFailed, Unsaved: true, Busy: true}
	h.ui.Frame()
	if err := h.ui.Click("重试保存"); err != nil {
		t.Fatal(err)
	}
	if len(h.controller.calls) != 0 {
		t.Fatal("busy retry was dispatched")
	}
}
