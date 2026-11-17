// Package account presents the native login without owning transport or storage.
package account

import (
	"errors"
	"image"
	"time"

	gui "github.com/egoist/mygo/ui"
	"github.com/skip2/go-qrcode"
	domain "my-bilibili/internal/account"
)

// View and all bitmap work belong to the UI thread. The controller publishes
// copied state; composition owns window close and the application's quit gate.
type View struct {
	stateGeneration uint64
	controller      domain.Controller
	requestClose    func(bool)
	bitmap          *gui.Bitmap
	qr              domain.QR
	generation      uint64
	qrLimit         int
	qrError         domain.Code
	phase           domain.Phase
	actionError     domain.Code
	confirmLogout   bool
}

func New(controller domain.Controller, requestClose func(bool)) *View {
	return &View{controller: controller, requestClose: requestClose}
}
func (v *View) clearQR() { v.bitmap = nil; v.qr = domain.QR{}; v.qrLimit = 0; v.qrError = domain.None }

func qrImage(q domain.QR, limit int) (image.Image, error) {
	code, err := qrcode.New(q.Text(), qrcode.Medium)
	if err != nil {
		return nil, domain.Invalid
	}
	modules := len(code.Bitmap()) // includes the required quiet zone; border stays on.
	scale := limit / modules
	if scale < 1 {
		return nil, domain.Invalid
	}
	return code.Image(-scale), nil
}
func (v *View) syncQR(s domain.State, now time.Time, limit int) bool {
	expired := !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt)
	visible := (s.Phase == domain.Waiting || s.Phase == domain.Scanned) && !s.QR.Empty() && !expired
	if !visible {
		v.clearQR()
		return expired && (s.Phase == domain.Waiting || s.Phase == domain.Scanned)
	}
	if v.qr == s.QR && v.generation == s.Generation && v.qrLimit == limit {
		return false
	}
	v.bitmap = nil
	v.qr = s.QR
	v.generation = s.Generation
	v.qrLimit = limit
	v.qrError = domain.None
	img, err := qrImage(s.QR, limit)
	if err != nil {
		v.qrError = domain.Invalid
		return false
	}
	v.bitmap = gui.NewBitmap(img).Pixelated()
	return false
}
func (v *View) action(c *gui.Context, fn func() error) {
	v.actionError = domain.None
	if err := fn(); err != nil {
		var code domain.Code
		if errors.As(err, &code) {
			v.actionError = code
		} else {
			v.actionError = domain.Invalid
		}
	}
	c.Invalidate()
}
func status(s domain.State) (string, string) {
	switch s.Phase {
	case domain.Initial, domain.Restoring:
		return "正在恢复登录", "正在检查这台 Mac 钥匙串中的登录。"
	case domain.SignedOut:
		return "登录哔哩哔哩", "使用哔哩哔哩手机客户端扫码，并在手机上确认。"
	case domain.Generating:
		return "正在生成二维码", "正在连接登录服务，请稍候。"
	case domain.Waiting:
		return "等待扫码", "打开哔哩哔哩手机客户端，扫描下方二维码。"
	case domain.Scanned:
		return "已扫描，请在手机确认", "请在手机上确认这次登录。"
	case domain.Saving:
		return "正在保存登录", "登录已确认，正在保存到钥匙串。如有系统提示，请完成确认。"
	case domain.SaveFailed:
		return "登录尚未保存", "手机已确认登录。请重试保存，无需再次扫码；放弃关闭会丢失本次尚未保存的登录。"
	case domain.Verifying:
		return "正在检查登录", "正在确认已保存的登录是否可用。"
	case domain.VerifyFailed:
		return "暂时无法确认登录", "登录仍保存在钥匙串中，可以重新检查，不需要立即重新扫码。"
	case domain.SignedIn:
		if s.Restored {
			return "已恢复登录", "已从这台 Mac 的钥匙串恢复登录。关闭窗口会保留登录。"
		}
		return "登录已保存", "登录已保存到这台 Mac 的钥匙串。关闭窗口会保留登录，供后续使用。"
	case domain.Expired:
		return "登录已失效", "可以重新扫码登录。已有保存数据不会被自动删除。"
	case domain.Failed:
		if s.Error == domain.Storage {
			return "暂时无法恢复登录", "请检查系统钥匙串提示，然后重试恢复。"
		}
		return "暂时无法开始登录", "请检查网络后重试生成二维码。"
	case domain.Canceling:
		return "正在取消登录", "正在结束本次请求，请稍候。"
	case domain.SigningOut:
		return "正在退出登录", "正在移除这台 Mac 钥匙串中的登录，请稍候。"
	case domain.DeleteFailed:
		return "退出登录未完成", "登录可能仍保存在钥匙串中。可以重试退出，或确认保留登录并关闭。"
	case domain.Closing:
		return "正在关闭窗口", "正在等待当前操作完成。已保存的登录会保留。"
	case domain.Done:
		return "窗口即将关闭", "已保存的登录会保留。"
	default:
		return "登录状态暂不可用", "请关闭窗口后重试。"
	}
}
func safeError(code domain.Code) string {
	switch code {
	case domain.None:
		return ""
	case domain.Storage:
		return "钥匙串操作未完成，请检查系统提示后重试。"
	case domain.Network:
		return "网络暂时不可用，请稍后重试。"
	case domain.Timeout:
		return "请求超时，请重试。"
	case domain.ServiceRejected:
		return "服务暂时无法完成请求，请稍后重试。"
	case domain.ExpiredSession:
		return "登录已失效，请重新扫码。"
	case domain.Missing:
		return "尚未保存登录。"
	case domain.Busy:
		return "当前操作尚未完成，请稍候。"
	case domain.Canceled:
		return "操作已取消。"
	case domain.Closed:
		return "窗口已关闭。"
	default:
		return "操作未完成，请重试。"
	}
}
func (v *View) Render(c *gui.Context) {
	s := v.controller.Snapshot()
	if v.phase != s.Phase || v.stateGeneration != s.Generation {
		v.confirmLogout = false
		v.actionError = domain.None
		v.phase = s.Phase
		v.stateGeneration = s.Generation
	}
	w, _ := c.Size()
	width := max(float32(1), w-48)
	expired := v.syncQR(s, c.Now(), int(min(float32(320), width)))
	heading, body := status(s)
	if expired {
		heading = "二维码已过期"
		body = "请取消本次登录，再重新生成二维码。"
	}
	if s.CloseRequested && s.Busy {
		body += " 操作结束后会关闭窗口。"
	}
	theme := c.Theme()
	gui.Scroll(c.Key("login-scroll")).Fill().Padding(24).Gap(16).Background(theme.Background).Children(func() {
		gui.Column(c).Width(width).Gap(8).Children(func() {
			gui.Text(c, "My Bilibili").FontSize(20).TextColor(theme.Text)
			gui.Text(c.Key("login-status"), heading).Width(width).FontSize(18).TextColor(theme.Text)
			gui.Text(c.Key("login-explanation"), body).Width(width).FontSize(13).TextColor(theme.TextMuted)
		})
		if v.bitmap != nil {
			size, _ := v.bitmap.Size()
			gui.Row(c).Width(width).Justify(gui.Center).Children(func() {
				gui.Box(c.Key("login-qr")).Size(float32(size), float32(size)).Background(gui.RGB(255, 255, 255)).Role(gui.RoleImage).Label("扫码登录二维码").Children(func() { gui.Image(c, v.bitmap).Size(float32(size), float32(size)) })
			})
		}
		if v.qrError != domain.None {
			gui.Text(c.Key("login-qr-error"), "二维码暂时无法显示，请取消后重试。").Width(width).FontSize(13).TextColor(theme.Danger)
		}
		if text := safeError(s.Error); text != "" {
			gui.Text(c.Key("login-error"), text).Width(width).FontSize(13).TextColor(theme.Danger)
		}
		if text := safeError(v.actionError); text != "" {
			gui.Text(c.Key("login-action-error"), text).Width(width).FontSize(13).TextColor(theme.Danger)
		}
		gui.Column(c.Key("login-actions")).Width(width).Gap(10).Children(func() { v.actions(c, s) })
		gui.Text(c, "登录仅保存在这台 Mac 的钥匙串中。会话失效时需要重新登录。").Width(width).FontSize(13).TextColor(theme.TextMuted)
	})
	if s.Phase != domain.Done {
		c.After(100 * time.Millisecond)
	}
}
func (v *View) button(c *gui.Context, key, label string, disabled bool, fn func()) {
	if gui.Button(c.Key(key), label).Height(36).MinWidth(180).Disabled(disabled).Clicked() {
		fn()
	}
}
func (v *View) actions(c *gui.Context, s domain.State) {
	act := func(key, label string, fn func() error) { v.button(c, key, label, s.Busy, func() { v.action(c, fn) }) }
	if v.confirmLogout {
		gui.Text(c, "退出会移除已保存的登录，下次需要重新扫码。是否继续？").Width(max(float32(1), cWidth(c)-48)).FontSize(13).TextColor(c.Theme().Text)
		v.button(c, "cancel-logout", "取消退出", false, func() { v.confirmLogout = false; c.Invalidate() })
		v.button(c, "confirm-logout", "确认退出登录", s.Busy, func() { v.confirmLogout = false; v.action(c, v.controller.Logout) })
		return
	}
	switch s.Phase {
	case domain.SignedOut:
		act("begin-login", "开始扫码登录", v.controller.Begin)
	case domain.Expired:
		act("begin-login", "重新扫码登录", v.controller.Begin)
	case domain.Failed:
		if s.Error == domain.Storage {
			act("retry-restore", "重试恢复", v.controller.Restore)
		} else if !s.Stored && !s.Unsaved {
			act("begin-login", "重新生成二维码", v.controller.Begin)
		}
	case domain.Generating, domain.Waiting, domain.Scanned:
		v.button(c, "cancel-login", "取消本次登录", false, func() { v.clearQR(); v.action(c, v.controller.Cancel) })
	case domain.Verifying:
		if !s.Unsaved {
			v.button(c, "cancel-verify", "取消检查", false, func() { v.action(c, v.controller.Cancel) })
		}
	case domain.SaveFailed:
		act("retry-save", "重试保存", v.controller.RetrySave)
		v.button(c, "discard-close", "放弃并关闭", s.Busy, func() { v.clearQR(); v.requestClose(true) })
		return
	case domain.VerifyFailed:
		act("retry-verify", "重新检查登录", v.controller.RetryVerify)
	case domain.DeleteFailed:
		act("retry-logout", "重试退出", v.controller.Logout)
		v.button(c, "retained-close", "保留登录并关闭", s.Busy, func() { v.clearQR(); v.requestClose(true) })
		return
	}
	if s.Stored && !s.Busy && (s.Phase == domain.SignedIn || s.Phase == domain.VerifyFailed || s.Phase == domain.Expired) {
		v.button(c, "logout", "退出登录…", false, func() { v.confirmLogout = true; c.Invalidate() })
	}
	if s.Phase != domain.Done {
		label := "关闭窗口"
		if s.Phase == domain.SignedIn {
			label = "完成"
		}
		v.button(c, "close-login", label, false, func() { v.clearQR(); v.requestClose(false) })
	}
}
func cWidth(c *gui.Context) float32 { w, _ := c.Size(); return w }
