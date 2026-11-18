// Package catalogview presents discovery without owning source or window work.
package catalogview

import (
	"errors"
	"fmt"
	"slices"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/catalog"
)

type Controller interface {
	State() catalog.SearchState
	Search(string) error
	NextPage() error
	Retry() error
}

type CoverRenderer interface {
	Render(*gui.Context, string, float32, float32)
}

type View struct {
	controller     Controller
	open           func(catalog.Entry) error
	covers         CoverRenderer
	query, entry   string
	listQuery      string
	list           *gui.ListState
	rows           []catalog.Summary
	phase          catalog.SearchPhase
	actionError    string
	hidden, closed bool
}

func New(controller Controller, open func(catalog.Entry) error, covers CoverRenderer) (*View, error) {
	if controller == nil || open == nil || covers == nil {
		return nil, catalog.Invalid
	}
	return &View{controller: controller, open: open, covers: covers, list: &gui.ListState{}}, nil
}

func contentError(code catalog.Code) string {
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
		return "当前内容受限，暂时无法查看。"
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

func (v *View) action(c *gui.Context, fn func() error) {
	if v.closed {
		return
	}
	v.actionError = ""
	if err := fn(); err != nil {
		var code catalog.Code
		if errors.As(err, &code) {
			v.actionError = contentError(code)
		} else {
			v.actionError = contentError(catalog.Unavailable)
		}
		if v.actionError == "" {
			v.actionError = contentError(catalog.Unavailable)
		}
	}
	c.Invalidate()
}

func (v *View) submitEntry() error {
	entry, err := catalog.ParseEntry(v.entry)
	if err != nil {
		return err
	}
	return v.open(entry)
}

// Render consumes widget actions only. Ordinary construction reads copied
// controller state and presents covers; root pumps the asynchronous owners.
func (v *View) Render(c *gui.Context, width, height float32) {
	if v.closed {
		return
	}
	v.hidden = false
	s := v.controller.State()
	if s.Query != v.listQuery || s.Phase == catalog.SearchLoading && v.phase != catalog.SearchLoading {
		v.list = &gui.ListState{}
	}
	v.listQuery, v.phase = s.Query, s.Phase
	v.rows = slices.Clone(s.Results)
	rows := v.rows
	v.list.Key = func(i int) any { return rows[i].ID }
	v.list.Label = func(i int) string { return rows[i].Title }
	width, height = max(width, 1), max(height, 1)
	content := max(width-24, 1)
	theme := c.Theme()
	gui.Column(c.Key("client-discovery")).Size(width, height).Padding(12).Gap(10).Children(func() {
		gui.Text(c, "发现视频").FontSize(18).TextColor(theme.Text)
		search := gui.TextInput(c.Key("client-search-text"), &v.query).Width(content).Height(34).
			Label("搜索关键词").Placeholder("搜索视频").MaxLength(256).Disabled(s.Phase == catalog.SearchClosed)
		searchButton := gui.Button(c.Key("client-search-submit"), "搜索").Width(content).Height(32).Disabled(s.Phase == catalog.SearchClosed)
		if search.Submitted() || searchButton.Clicked() {
			v.action(c, func() error { return v.controller.Search(v.query) })
		}
		entry := gui.TextInput(c.Key("client-open-text"), &v.entry).Width(content).Height(34).
			Label("链接或视频编号").Placeholder("BV、av 或视频链接").MaxLength(2048).Disabled(s.Phase == catalog.SearchClosed)
		openButton := gui.Button(c.Key("client-open-submit"), "打开链接或编号").Width(content).Height(32).Disabled(s.Phase == catalog.SearchClosed)
		if entry.Submitted() || openButton.Clicked() {
			v.action(c, v.submitEntry)
		}
		if text := v.actionError; text != "" {
			gui.Text(c.Key("client-action-error"), text).Width(content).FontSize(12).TextColor(theme.Danger)
		}
		switch s.Phase {
		case catalog.SearchInitial:
			gui.Text(c.Key("client-search-state"), "输入关键词搜索，或直接打开视频链接。").Width(content).FontSize(12).TextColor(theme.TextMuted)
		case catalog.SearchLoading:
			gui.Text(c.Key("client-search-state"), "正在搜索…").Width(content).FontSize(12).TextColor(theme.TextMuted)
		case catalog.SearchReady:
			if len(rows) == 0 {
				gui.Text(c.Key("client-search-state"), "没有找到相关视频。").Width(content).FontSize(12).TextColor(theme.TextMuted)
			}
		case catalog.SearchClosed:
			gui.Text(c.Key("client-search-state"), "当前内容会话已关闭。").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if text := contentError(s.Error); text != "" {
			gui.Text(c.Key("client-search-error"), text).Width(content).FontSize(12).TextColor(theme.Danger)
			if gui.Button(c.Key("client-retry-search"), "重试搜索").Width(content).Height(32).
				Disabled(s.LoadingMore || s.Phase == catalog.SearchLoading || s.Phase == catalog.SearchClosed).Clicked() {
				v.action(c, v.controller.Retry)
			}
		}
		// A navigation/controller callback may retire this view synchronously.
		if v.closed {
			return
		}
		gui.List(c.Key("client-results"), v.list, len(rows)).Width(content).Grow(1).Rows(func(row gui.ListRow) {
			if v.closed {
				return
			}
			rc, value := row.Context, rows[row.Index]
			gui.Column(rc).Width(content).PaddingY(8).Gap(6).Children(func() {
				gui.Row(rc).Width(content).Gap(8).Children(func() {
					v.covers.Render(rc, value.CoverID, 80, 45)
					gui.Column(rc).Width(max(content-88, 1)).Gap(3).Children(func() {
						gui.Text(rc, value.Title).Width(max(content-88, 1)).MaxLines(2).FontSize(13).TextColor(theme.Text)
						gui.Text(rc, value.Creator).Width(max(content-88, 1)).SingleLine().FontSize(11).TextColor(theme.TextMuted)
					})
				})
				if gui.Button(rc.Key(fmt.Sprintf("client-result-%d", row.Index)), "播放视频").Width(content).Height(30).Clicked() {
					v.action(c, func() error { return v.open(catalog.Entry{Kind: catalog.BVEntry, ID: string(value.ID)}) })
				}
			})
		})
		if v.closed {
			return
		}
		if s.LoadingMore {
			gui.Text(c, "正在加载下一页…").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if s.AtLimit {
			gui.Text(c.Key("client-search-limit"), "已达到本次搜索的显示上限。").Width(content).FontSize(12).TextColor(theme.TextMuted)
		}
		if len(rows) > 0 {
			disabled := s.Phase != catalog.SearchReady || s.LoadingMore || s.Error != catalog.None || s.AtLimit || s.Page >= s.TotalPages
			if gui.Button(c.Key("client-next-page"), "加载下一页").Width(content).Height(32).Disabled(disabled).Clicked() {
				v.action(c, v.controller.NextPage)
			}
		}
	})
}

// VisibleCoverIDs reports the last actual viewport, excluding list overscan.
// Root combines this copy with detail demand before updating the one presenter.
func (v *View) VisibleCoverIDs() []string {
	ids := []string{}
	if v.closed || v.hidden {
		return ids
	}
	first, last := v.list.Visible()
	for i := max(first, 0); i <= last && i < len(v.rows) && len(ids) < 20; i++ {
		id := v.rows[i].CoverID
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func (v *View) Hide() { v.hidden = true }
func (v *View) Close() {
	v.closed, v.hidden = true, true
	v.controller, v.open, v.covers = nil, nil, nil
	v.query, v.entry, v.listQuery = "", "", ""
	// The frame may still own the old ListState while a row callback closes.
	// Drop our pointer without mutating the list currently being laid out.
	v.rows, v.list, v.actionError = nil, nil, ""
}
