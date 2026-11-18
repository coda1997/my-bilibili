// Package historyview presents copied history and explicit recovery actions.
package historyview

import (
	"errors"
	"fmt"
	"slices"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
)

type Controller interface {
	State() history.State
	Delete(catalog.PartRef) error
	Clear() error
	ResetCorrupt() error
	Retry() error
}

type Actions struct {
	Continue     func(history.Record) error
	Replay       func(history.Record) error
	RequestClose func(bool)
}

type confirmation uint8

const (
	noConfirmation confirmation = iota
	deleteRecord
	clearRecords
	resetRecords
)

type View struct {
	controller  Controller
	actions     Actions
	list        *gui.ListState
	confirm     confirmation
	ref         catalog.PartRef
	actionError string
	closed      bool
}

func New(controller Controller, actions Actions) (*View, error) {
	if controller == nil || actions.Continue == nil || actions.Replay == nil || actions.RequestClose == nil {
		return nil, history.Invalid
	}
	return &View{controller: controller, actions: actions, list: &gui.ListState{}}, nil
}

func storageText(s history.State, closing bool) string {
	if closing && s.Phase == history.Failed && !s.Dirty {
		return "历史记录清理未完成，请重试。"
	}
	if s.Error == history.Uncertain || s.Dirty && s.Phase == history.Failed {
		return "最新播放进度尚未确认保存，请重试。"
	}
	switch s.Error {
	case history.InUse:
		return "另一个应用正在使用本地历史记录。当前播放进度暂不保存。"
	case history.LockUnsupported:
		return "当前系统无法安全访问本地历史记录。当前播放进度暂不保存。"
	case history.Corrupt:
		return "本地历史记录无法读取，原文件已保留。"
	case history.UnsupportedVersion:
		return "历史记录来自不支持的版本，原文件已保留。"
	case history.Storage:
		return "本地历史记录暂时不可用，请重试。"
	case history.Invalid:
		return "当前无法执行此历史记录操作。"
	case history.Closed:
		return "历史记录已关闭。"
	case history.None:
		return ""
	default:
		return "历史记录操作未完成，请重试。"
	}
}

func safeAction(err error) string {
	var storage history.Code
	if errors.As(err, &storage) {
		return storageText(history.State{Error: storage}, false)
	}
	var content catalog.Code
	if errors.As(err, &content) {
		switch content {
		case catalog.LoginRequired:
			return "需要登录后才能继续播放。"
		case catalog.SessionExpired:
			return "登录已失效，请重新登录。"
		case catalog.Network, catalog.Timeout:
			return "暂时无法连接播放服务，请重试。"
		case catalog.Restricted, catalog.Unsupported:
			return "此内容暂时无法播放。"
		}
	}
	return "操作未完成，请重试。"
}

func (v *View) action(c *gui.Context, fn func() error) {
	if v.closed {
		return
	}
	v.actionError = ""
	if err := fn(); err != nil {
		v.actionError = safeAction(err)
	}
	c.Invalidate()
}

func (v *View) ask(c *gui.Context, anchor gui.Element, kind confirmation, question string, disabled bool, fn func() error) {
	wasOpen := v.confirm == kind
	open := wasOpen
	key := map[confirmation]string{deleteRecord: "history-delete-confirmation", clearRecords: "history-clear-confirmation", resetRecords: "history-reset-confirmation"}[kind]
	gui.Popover(c.Key(key), anchor, &open, func() {
		gui.Column(c).Width(240).Gap(8).Children(func() {
			gui.Text(c, question).Width(240).FontSize(13)
			if gui.Button(c.Key("confirm"), "确认").Width(240).Height(32).Disabled(disabled).Clicked() {
				v.confirm = noConfirmation
				v.action(c, fn)
			}
			if gui.Button(c.Key("cancel"), "取消").Width(240).Height(32).Clicked() {
				v.confirm = noConfirmation
				c.Invalidate()
			}
		})
	})
	if wasOpen && !open {
		v.confirm = noConfirmation
	}
}

func progress(position, duration time.Duration) string {
	clock := func(value time.Duration) string {
		seconds := int(max(value, 0) / time.Second)
		return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
	}
	return clock(position) + " / " + clock(duration)
}

func (v *View) Render(c *gui.Context, width, height float32, closing bool) {
	if v.closed {
		return
	}
	s := v.controller.State()
	rows := slices.Clone(s.Records)
	mutable := !closing && (s.Phase == history.Ready || s.Phase == history.Failed && s.Dirty)
	if !mutable && v.confirm != resetRecords || v.confirm == deleteRecord && !slices.ContainsFunc(rows, func(record history.Record) bool { return record.Ref == v.ref }) {
		v.confirm = noConfirmation
	}
	if v.confirm == resetRecords && (closing || s.Error != history.Corrupt && s.Error != history.UnsupportedVersion) {
		v.confirm = noConfirmation
	}
	v.list.Key = func(i int) any { return rows[i].Ref }
	v.list.Label = func(i int) string { return rows[i].Title + " · " + rows[i].PartTitle }
	width, height = max(width, 1), max(height, 1)
	content := max(width-24, 1)
	theme := c.Theme()
	gui.Column(c.Key("history-pane")).Size(width, height).Padding(12).Gap(10).Children(func() {
		gui.Text(c, "本地历史记录").FontSize(18).TextColor(theme.Text)
		clearButton := gui.Button(c.Key("history-clear"), "清空历史记录…").Width(content).Height(32).Disabled(!mutable || len(rows) == 0)
		if clearButton.Clicked() {
			v.confirm = clearRecords
			c.Invalidate()
		}
		v.ask(c, clearButton, clearRecords, "确认清空这台设备上的全部历史记录？", !mutable, v.controller.Clear)
		if text := storageText(s, closing); text != "" {
			gui.Text(c.Key("history-error"), text).Width(content).FontSize(12).TextColor(theme.Danger)
		}
		if v.actionError != "" {
			gui.Text(c.Key("history-action-error"), v.actionError).Width(content).FontSize(12).TextColor(theme.Danger)
		}
		if s.Phase == history.Initial || s.Phase == history.Loading {
			gui.Text(c, "正在读取本地历史记录…").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if s.Phase == history.Closing {
			gui.Text(c, "正在结束历史记录操作…").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if s.Phase == history.Ready && len(rows) == 0 {
			gui.Text(c.Key("history-empty"), "还没有播放记录。").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if s.Phase == history.Failed {
			if gui.Button(c.Key("history-retry"), "重试").Width(content).Height(32).Disabled(s.Busy).Clicked() {
				v.action(c, v.controller.Retry)
			}
			if !closing && (s.Error == history.Corrupt || s.Error == history.UnsupportedVersion) {
				resetButton := gui.Button(c.Key("history-reset"), "重置历史记录…").Width(content).Height(32).Disabled(s.Busy)
				if resetButton.Clicked() {
					v.confirm = resetRecords
					c.Invalidate()
				}
				v.ask(c, resetButton, resetRecords, "重置会删除这台设备上的历史记录，是否继续？", s.Busy, v.controller.ResetCorrupt)
			}
			if s.Dirty {
				if gui.Button(c.Key("history-discard-close"), "不确认保存并关闭").Width(content).Height(32).Disabled(s.Busy).Clicked() {
					v.actions.RequestClose(true)
				}
			} else if !closing {
				if gui.Button(c.Key("history-close"), "关闭应用").Width(content).Height(32).Clicked() {
					v.actions.RequestClose(false)
				}
			}
		}
		if v.closed {
			return
		}
		gui.List(c.Key("history-records"), v.list, len(rows)).Width(content).Grow(1).Rows(func(row gui.ListRow) {
			if v.closed {
				return
			}
			rc, record := row.Context, rows[row.Index]
			gui.Column(rc).Width(content).PaddingY(8).Gap(5).Children(func() {
				gui.Text(rc, record.Title).Width(content).MaxLines(2).FontSize(13).TextColor(theme.Text)
				gui.Text(rc, record.PartTitle).Width(content).SingleLine().FontSize(12).TextColor(theme.TextMuted)
				text := progress(record.Position, record.Duration)
				if record.Completed {
					text += " · 已看完"
				}
				gui.Text(rc, text).Width(content).FontSize(12).TextColor(theme.TextMuted)
				if !record.Completed {
					if gui.Button(rc.Key(fmt.Sprintf("history-continue-%d", row.Index)), "继续播放").Width(content).Height(30).Disabled(!mutable).Clicked() {
						v.action(c, func() error { return v.actions.Continue(record) })
					}
				}
				if gui.Button(rc.Key(fmt.Sprintf("history-replay-%d", row.Index)), "从头播放").Width(content).Height(30).Disabled(!mutable).Clicked() {
					v.action(c, func() error { return v.actions.Replay(record) })
				}
				remove := gui.Button(rc.Key(fmt.Sprintf("history-delete-%d", row.Index)), "删除记录…").Width(content).Height(30).Disabled(!mutable)
				if remove.Clicked() {
					v.confirm, v.ref = deleteRecord, record.Ref
					c.Invalidate()
				}
				if v.ref == record.Ref {
					v.ask(rc, remove, deleteRecord, "确认删除这条本地播放记录？", !mutable, func() error { return v.controller.Delete(record.Ref) })
				}
			})
		})
	})
}

func (v *View) Hide() { v.confirm, v.ref, v.actionError = noConfirmation, catalog.PartRef{}, "" }
func (v *View) Close() {
	v.Hide()
	v.closed = true
	v.controller, v.actions, v.list = nil, Actions{}, nil
}
