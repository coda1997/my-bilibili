package desktop

import (
	"errors"
	"fmt"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	"my-bilibili/internal/catalog"
	accountview "my-bilibili/internal/desktop/account"
	catalogview "my-bilibili/internal/desktop/catalog"
	historyview "my-bilibili/internal/desktop/history"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

// ClientStatus contains presentation state only. Composition owns all account,
// content, persistence and close decisions, including rechecking button actions.
type ClientStatus struct {
	AccountPhase        account.Phase
	AccountError        account.Code
	AccountBusy, Stored bool
	HistoryPhase        history.Phase
	HistoryError        history.Code
	HistoryDirty        bool
	Switching, Closing  bool
	Error               catalog.Code
	CanRetry            bool
}

type clientTab uint8

const (
	catalogTab clientTab = iota
	playerTab
	historyTab
	accountTab
)

// Client mounts the feature views on the UI thread. It never starts or closes
// their controllers, performs I/O, or owns the native window. Show methods are
// explicit transitions; their caller invalidates the window when used externally.
type Client struct {
	Player              *View
	Catalog             *catalogview.View
	History             *historyview.View
	Account             *accountview.View
	KeepAccountAndClose func()
	RetryNavigation     func() error

	tab, sidebar            clientTab
	wide, mountedFullscreen bool
	mounted, playerSeen     *View
	playerEpoch             uint64
	paneNotice              bool
	navigationError         catalog.Code
}

func (v *Client) ShowCatalog() { v.show(catalogTab) }
func (v *Client) ShowPlayer()  { v.show(playerTab) }
func (v *Client) ShowHistory() { v.show(historyTab) }
func (v *Client) ShowAccount() { v.show(accountTab) }

func (v *Client) show(tab clientTab) {
	if v.tab != tab {
		// These are presentation retirements, not controller close operations.
		if player := v.Player; player != nil {
			player.retireInput()
		}
		v.playerEpoch++
		if pane := v.Catalog; pane != nil {
			pane.Hide()
		}
		if pane := v.History; pane != nil {
			pane.Hide()
		}
		if pane := v.Account; pane != nil && tab != accountTab {
			if err := pane.Hide(); err != nil {
				v.paneNotice = true
			}
		}
		v.mounted = nil
		v.tab = tab
	}
	if tab == catalogTab || tab == historyTab {
		v.sidebar = tab
	}
	if tab == accountTab {
		v.paneNotice = false
		if pane := v.Account; pane != nil {
			pane.Show()
		}
	}
}

// PlayerVisible reports current mount intent, not a painted picture or native
// window visibility. Composition must also check its window and current engine.
func (v *Client) PlayerVisible() bool {
	if v.Player == nil || v.mounted != v.Player || v.Player.Fullscreen() != v.mountedFullscreen {
		return false
	}
	return v.mountedFullscreen || v.tab != accountTab && (v.wide || v.tab == playerTab)
}

func (v *Client) Render(c *gui.Context, s ClientStatus) {
	v.mounted = nil
	// An action may synchronously retire or replace a pane during this pass.
	// Never dereference a cleared field later, or mount a retired player.
	player, discovery, recent, login := v.Player, v.Catalog, v.History, v.Account
	if v.playerSeen != player {
		v.playerSeen = player
		v.playerEpoch++
	}
	w, h := c.Size()
	v.wide = w >= 1100
	if player != nil && player.Fullscreen() {
		v.paintPlayer(c, player, w, h, true, true)
		return
	}
	gui.Column(c.Key("client-shell")).Size(w, h).Background(c.Theme().Background).Children(func() {
		gui.Row(c.Key("client-tabs")).Width(w).Height(52).Padding(10).Gap(8).AlignItems(gui.Center).Children(func() {
			gui.Text(c, "My Bilibili").FontSize(17).TextColor(c.Theme().Text)
			gui.Box(c).Grow(1)
			for _, item := range []struct {
				key, text, label string
				tab              clientTab
			}{
				{"client-catalog", "发现", "发现", catalogTab},
				{"client-player", "正在播放", "正在播放", playerTab},
				{"client-history", "历史", "历史", historyTab},
				{"client-account", "账号 · " + accountLabel(s), "账号", accountTab},
			} {
				if gui.Button(c.Key(item.key), item.text).Label(item.label).Height(32).MinWidth(58).AccessSelected(v.tab == item.tab).Clicked() {
					v.show(item.tab)
					c.Invalidate()
				}
			}
		})
		banner := v.banner(c, s, w)
		bodyHeight := max(h-52-banner, 1)
		gui.Box(c.Key("client-body")).Size(w, bodyHeight).Children(func() {
			if v.tab == accountTab {
				// The embedded account view owns its 24-DIP padding and reads the
				// whole window width. Do not wrap it in a padded sidebar.
				if login != nil && login == v.Account {
					login.Render(c)
				} else {
					clientPlaceholder(c, w, bodyHeight, "账号面板暂不可用。")
				}
				return
			}
			feature := func(tab clientTab, width float32) {
				switch tab {
				case historyTab:
					if recent != nil && recent == v.History {
						recent.Render(c, width, bodyHeight, s.Closing)
					} else {
						clientPlaceholder(c, width, bodyHeight, "历史记录面板暂不可用。")
					}
				default:
					if discovery != nil && discovery == v.Catalog {
						discovery.Render(c, width, bodyHeight)
					} else {
						clientPlaceholder(c, width, bodyHeight, "发现页面正在准备…")
					}
				}
			}
			if v.wide {
				gui.Row(c.Key("client-wide")).Size(w, bodyHeight).Gap(12).AlignItems(gui.Start).Children(func() {
					// Build focused feature inputs/popovers before player shortcuts.
					pane := gui.Box(c.Key("client-sidebar")).Size(320, bodyHeight)
					pane.Children(func() { feature(v.sidebar, 320) })
					if v.tab != accountTab {
						// Single-line native inputs leave Up/Down unconsumed. A
						// focused feature owns those keys too, rather than changing
						// the volume of the player beside it.
						v.paintPlayer(c, player, max(w-332, 1), bodyHeight, false, !pane.FocusWithin())
					}
				})
			} else if v.tab == playerTab {
				v.paintPlayer(c, player, w, bodyHeight, false, true)
			} else {
				feature(v.tab, w)
			}
		})
	})
}

func (v *Client) paintPlayer(c *gui.Context, player *View, width, height float32, full, shortcuts bool) {
	if player == nil || player != v.Player {
		clientPlaceholder(c, width, height, "选择视频后在这里播放。")
		return
	}
	tab := v.tab
	// A new video may restart native generation numbering. Retire GUI capture
	// by view identity as well, while retaining the public child control keys.
	gui.Box(c.Key(fmt.Sprintf("client-player-mount-%d", v.playerEpoch))).Size(width, height).Children(func() {
		player.render(c, max(width, 1), max(height, 1), shortcuts)
	})
	if player == v.Player && tab == v.tab && player.Fullscreen() == full {
		v.mounted, v.mountedFullscreen = player, full
	}
}

func clientPlaceholder(c *gui.Context, width, height float32, text string) {
	gui.Column(c).Size(width, height).Padding(24).Children(func() {
		gui.Text(c, text).Width(max(width-48, 1)).FontSize(15).TextColor(c.Theme().TextMuted)
	})
}

func (v *Client) banner(c *gui.Context, s ClientStatus, width float32) float32 {
	messages := []string{}
	if s.Closing {
		messages = append(messages, "正在关闭应用，请等待当前操作完成。")
	} else if s.Switching {
		messages = append(messages, "正在切换视频，请稍候。")
	}
	if s.Error != catalog.None {
		messages = append(messages, contentMessage(s.Error))
	}
	if v.navigationError != catalog.None {
		messages = append(messages, contentMessage(v.navigationError))
	}
	if s.AccountError != account.None {
		messages = append(messages, accountMessage(s.AccountError))
	}
	if text := historyMessage(s); text != "" {
		messages = append(messages, text)
	}
	if v.paneNotice {
		messages = append(messages, "账号面板已收起，但取消登录未完成，请在账号页检查。")
	}
	retry := s.CanRetry && v.RetryNavigation != nil
	keep := s.Closing && s.AccountPhase == account.DeleteFailed && !s.AccountBusy && v.KeepAccountAndClose != nil
	rows := len(messages)
	if retry || keep {
		rows++
	}
	if rows == 0 {
		return 0
	}
	height := float32(rows*32 + (rows-1)*6 + 8)
	gui.Column(c.Key("client-notices")).Width(width).Height(height).PaddingX(12).Gap(6).Children(func() {
		for i, message := range messages {
			gui.Text(c.Key(fmt.Sprintf("client-notice-%d", i)), message).Width(max(width-24, 1)).Height(32).MaxLines(2).FontSize(12).TextColor(c.Theme().Text)
		}
		if retry || keep {
			gui.Row(c).Width(max(width-24, 1)).Gap(8).Children(func() {
				if retry && gui.Button(c.Key("client-retry-navigation"), "重试打开视频").Height(32).Disabled(s.Switching || s.Closing).Clicked() {
					v.navigationError = contentCode(v.RetryNavigation())
					c.Invalidate()
				}
				if keep && gui.Button(c.Key("client-keep-account-close"), "保留登录并关闭").Height(32).Clicked() {
					v.KeepAccountAndClose()
					c.Invalidate()
				}
			})
		}
	})
	return height
}

func accountLabel(s ClientStatus) string {
	switch s.AccountPhase {
	case account.SignedIn:
		return "已登录"
	case account.SignedOut:
		return "未登录"
	case account.Expired:
		return "登录失效"
	case account.SaveFailed, account.DeleteFailed, account.VerifyFailed, account.Failed:
		return "需要处理"
	default:
		if s.AccountBusy {
			return "处理中"
		}
		if s.Stored {
			return "已保存登录"
		}
		return "待确认"
	}
}

func contentCode(err error) catalog.Code {
	if err == nil {
		return catalog.None
	}
	var code catalog.Code
	if errors.As(err, &code) && code != catalog.None {
		return code
	}
	return catalog.Unavailable
}

func contentMessage(code catalog.Code) string {
	switch code {
	case catalog.None:
		return ""
	case catalog.Invalid:
		return "输入内容不符合要求，请检查后重试。"
	case catalog.Unsupported:
		return "暂不支持此类内容或链接。"
	case catalog.Canceled:
		return "请求已取消。"
	case catalog.Timeout:
		return "请求超时，请重试。"
	case catalog.Network:
		return "网络暂时不可用，请重试。"
	case catalog.Restricted:
		return "当前内容受限，暂时无法播放。"
	case catalog.LoginRequired:
		return "需要登录后才能查看此内容。"
	case catalog.SessionExpired:
		return "登录已失效，请重新登录。"
	case catalog.VerificationRequired:
		return "服务需要进一步验证。"
	case catalog.RateLimited:
		return "请求过于频繁，请稍后重试。"
	case catalog.ServiceUnavailable:
		return "服务暂时不可用，请稍后重试。"
	case catalog.Closed:
		return "当前内容会话已关闭。"
	case catalog.QualityUnavailable:
		return "所选画质暂不可用。"
	default:
		return "当前内容暂不可用，请重试。"
	}
}

func playbackMessage(code playback.ErrorCode) string {
	switch code {
	case playback.Invalid:
		return "当前无法执行此播放操作。"
	case playback.Unsupported:
		return "暂不支持当前媒体格式。"
	case playback.Canceled:
		return "播放操作已取消。"
	case playback.Timeout:
		return "播放加载超时，请重试。"
	case playback.IsClosed:
		return "播放已关闭。"
	default:
		return "播放暂时不可用，请重试。"
	}
}

func accountMessage(code account.Code) string {
	switch code {
	case account.Storage:
		return "登录保存操作未完成，请在账号页处理。"
	case account.Network:
		return "账号暂时无法连接网络，请在账号页重试。"
	case account.Timeout:
		return "账号请求超时，请在账号页重试。"
	case account.ExpiredSession:
		return "登录已失效，请在账号页重新登录。"
	case account.Missing:
		return "尚未保存登录，可以在账号页扫码登录。"
	case account.Busy:
		return "账号操作尚未完成，请稍候。"
	case account.Canceled:
		return "账号操作已取消。"
	case account.Closed:
		return "账号会话已关闭。"
	case account.ServiceRejected:
		return "登录服务暂时无法完成请求，请在账号页重试。"
	default:
		return "账号操作未完成，请在账号页处理。"
	}
}

func historyMessage(s ClientStatus) string {
	if s.HistoryPhase == history.Failed && s.Closing && !s.HistoryDirty {
		return "历史记录清理未完成，请在历史页重试。"
	}
	if s.HistoryError == history.Uncertain || s.HistoryPhase == history.Failed && s.HistoryDirty {
		return "最新播放进度尚未确认保存，请在历史页处理。"
	}
	switch s.HistoryError {
	case history.None:
		return ""
	case history.InUse:
		return "另一个应用正在使用历史记录，当前播放进度暂不保存。"
	case history.LockUnsupported:
		return "当前系统无法安全访问历史记录，当前播放进度暂不保存。"
	case history.Corrupt, history.UnsupportedVersion:
		return "历史记录无法读取，原文件已保留，请在历史页处理。"
	case history.Closed:
		return "历史记录已关闭。"
	default:
		return "历史记录暂不可用，请在历史页重试。"
	}
}
