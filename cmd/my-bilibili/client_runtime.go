package main

import (
	"context"
	"os"
	"slices"
	"time"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/desktop"
	accountview "my-bilibili/internal/desktop/account"
	historyview "my-bilibili/internal/desktop/history"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

type clientBackend struct {
	config  clientConfig
	verify  *verificationRun
	setup   func(*clientApplication)
	render  func(*gui.Context, *clientApplication)
	tick    func(*clientApplication, time.Time)
	closed  func(*clientApplication)
	pending func() int
	cleanup func() error
}

type clientApplication struct {
	core                           *clientCore
	backend                        clientBackend
	shell                          *desktop.Client
	window                         *mygo.Window
	pump                           *clientPump
	interval                       time.Duration
	mounted                        *clientVideo
	navigation                     uint64
	finalJoin                      <-chan struct{}
	closeQueued, closed, minimized bool
	closeError                     error
	verifier                       *verificationController
	report                         verificationReport
	reported                       bool
	verifyCovers                   []string
	detailCover                    string
}

func runClient(resources string, options clientOptions) error {
	backend, err := composeClient(resources, options)
	if err != nil {
		return err
	}
	if backend.verify != nil {
		defer backend.verify.Stop()
	}
	core, err := newClientCore(backend.config)
	if err != nil {
		return err
	}
	a := &clientApplication{core: core, backend: backend}
	mygo.App.WhenReady(func() {
		a.interval, err = doubleClickInterval()
		if err != nil {
			a.closeError = err
			core.RequestClose(false)
			mygo.App.Quit()
			return
		}
		a.shell = &desktop.Client{KeepAccountAndClose: core.KeepAccountAndClose, RetryNavigation: core.RetryNavigation}
		var accountController accountview.PaneController = clientAccountController{core}
		if backend.verify != nil {
			accountController = backend.verify.AccountController(accountController)
		}
		a.shell.Account = accountview.NewPane(accountController, a.shell.ShowCatalog)
		a.shell.History, err = historyview.New(core.history, historyview.Actions{
			Continue: core.Continue, Replay: core.Replay, RequestClose: a.requestClose,
		})
		if err != nil {
			a.closeError = err
			core.RequestClose(false)
			mygo.App.Quit()
			return
		}
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: "My Bilibili", Width: 1120, Height: 760, MinWidth: 800, MinHeight: 600, Content: gui.View(a.render)})
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if !a.closeQueued {
				e.PreventDefault()
				a.requestClose(false)
			}
		})
		a.window.OnClosed(func() {
			a.closed = true
			if a.verifier != nil {
				a.report = a.verifier.Report(a.verificationSnapshot())
				a.reported = true
			}
			if backend.closed != nil {
				backend.closed(a)
			}
			mygo.App.Quit()
		})
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if !a.closeQueued {
				e.PreventDefault()
				a.requestClose(false)
			}
		})
		if backend.verify != nil {
			a.verifier, err = newVerificationController(backend.verify, a.verificationActions())
			if err != nil {
				a.closeError = err
				a.requestClose(false)
			}
		}
		if backend.setup != nil {
			backend.setup(a)
		}
		a.pump = startClientPump(a.window.Update, a.tick, 40*time.Millisecond)
		if !core.closing {
			if err := core.Start(context.Background()); err != nil {
				a.closeError = err
				a.requestClose(false)
			}
		}
		a.syncViews()
		a.window.Show()
		mygo.App.Focus()
		a.window.Focus()
	})
	runErr := mygo.App.Run()
	// Normal close reaches here only after the actual domain and dispatcher joins.
	// If native setup/loop failed, keep driving accepted cleanup without drawing.
	if !core.closed {
		if a.pump != nil {
			a.pump.Stop()
			<-a.pump.Done()
		}
		core.RequestClose(false)
		for !core.closed || backend.pending != nil && backend.pending() != 0 {
			if backend.tick != nil {
				backend.tick(a, time.Now())
			}
			core.Tick()
			if s := core.history.State(); s.Phase == history.Failed {
				return history.Storage
			}
			if s := core.account.Snapshot(); !s.Busy && (s.Unsaved || s.Phase == account.DeleteFailed) {
				return account.Storage
			}
			if !core.closed || backend.pending != nil && backend.pending() != 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}
	if a.finalJoin != nil {
		<-a.finalJoin
	}
	if backend.cleanup != nil {
		if err := backend.cleanup(); err != nil && a.closeError == nil {
			a.closeError = err
		}
	}
	if backend.verify != nil {
		if !a.reported {
			return catalog.Unavailable
		}
		if err := backend.verify.Finish(a.report, os.Stdout); err != nil {
			return err
		}
	}
	if runErr != nil || mpv.Owned() != (mpv.Resources{}) {
		return playback.Media
	}
	if core.closeError != nil {
		return core.closeError
	}
	return a.closeError
}

func (a *clientApplication) requestClose(discard bool) {
	if a.closed {
		return
	}
	a.core.RequestClose(discard)
	if a.window != nil {
		a.window.Invalidate()
	}
}

func (a *clientApplication) tick() {
	if a.closed {
		return
	}
	now := time.Now()
	if a.backend.tick != nil {
		a.backend.tick(a, now)
	}
	a.core.Tick()
	a.syncViews()
	if a.verifier != nil {
		a.verifier.Tick(now)
	}
	if a.core.closing && !a.core.closed {
		accountState, historyState := a.core.account.Snapshot(), a.core.history.State()
		if !accountState.Busy && (accountState.Unsaved || accountState.Phase == account.DeleteFailed) {
			a.shell.ShowAccount()
			a.window.Show()
		} else if historyState.Phase == history.Failed {
			a.shell.ShowHistory()
			a.window.Show()
		}
	}
	if a.core.closed && a.finalJoin == nil && (a.backend.pending == nil || a.backend.pending() == 0) {
		a.shell.History.Close()
		_ = a.shell.Account.Hide()
		a.pump.Stop()
		done := make(chan struct{})
		a.finalJoin = done
		go func() {
			defer close(done)
			<-a.pump.Done()
			a.window.Update(a.finishClose)
		}()
	}
}

func (a *clientApplication) finishClose() {
	if a.closeQueued {
		return
	}
	if !channelDone(a.finalJoin) {
		a.window.Update(a.finishClose)
		return
	}
	a.closeQueued = true
	a.window.Close()
}

func (a *clientApplication) syncViews() {
	a.shell.Catalog, a.shell.Player = nil, nil
	g := a.core.group
	if g == nil || g.retiring {
		a.mounted = nil
		return
	}
	a.shell.Catalog = g.view
	v := g.video
	if v == nil || v.retiring {
		a.mounted = nil
		return
	}
	if a.mounted != v {
		a.mounted = v
		v.player.RealContent, v.player.DoubleClickInterval = true, a.interval
		v.player.Surface = func(c *gui.Context, w, h float32) { mpv.DrawTargetSized(c, v.session.Engine(), w, h) }
		v.player.Cover = func(c *gui.Context, id string) {
			a.detailCover = id
			if g.covers != nil {
				g.covers.Render(c, id, 80, 45)
			}
		}
		v.player.Fullscreen = func() bool { return a.window != nil && a.window.IsFullScreen() }
		v.player.SetFullscreen = func(enabled bool) { a.window.SetFullScreen(enabled) }
		v.player.RetryDanmaku = a.core.RetryDanmaku
	}
	v.player.DanmakuError = v.loaderError
	v.player.DanmakuPrefetchError = catalog.None
	if v.loader != nil {
		v.player.DanmakuPrefetchError = v.loader.State().PrefetchError
	}
	a.shell.Player = v.player
	if a.navigation != v.navigation {
		a.navigation = v.navigation
		a.shell.ShowPlayer()
	}
}

func (a *clientApplication) render(c *gui.Context) {
	a.detailCover = ""
	if a.core.group != nil {
		a.core.group.catalogPainted = false
	}
	accountState, historyState := a.core.account.Snapshot(), a.core.history.State()
	a.shell.Render(c, desktop.ClientStatus{AccountPhase: accountState.Phase, AccountError: accountState.Error, AccountBusy: accountState.Busy, Stored: accountState.Stored,
		HistoryPhase: historyState.Phase, HistoryError: historyState.Error, HistoryDirty: historyState.Dirty, Switching: a.core.pending != nil, Closing: a.core.closing,
		Error: a.core.navigationError, CanRetry: a.core.lastRequest != nil && a.core.navigationError != catalog.None})
	a.coverDemand()
	if a.backend.render != nil {
		a.backend.render(c, a)
	}
}

func (a *clientApplication) coverDemand() {
	g := a.core.group
	if a.core.closing || g == nil || g.retiring {
		return
	}
	if a.backend.verify != nil && (g.video == nil || g.video.session.State().DetailPhase != app.DetailReady) {
		return
	}
	ids := []string{}
	if a.detailCover != "" {
		ids = append(ids, a.detailCover)
	}
	if g.catalogPainted {
		for _, id := range g.view.VisibleCoverIDs() {
			if !slices.Contains(ids, id) && len(ids) < 20 {
				ids = append(ids, id)
			}
		}
	}
	if a.backend.verify != nil {
		accepted := ids[:0]
		for _, id := range ids {
			if !slices.Contains(a.verifyCovers, id) {
				if len(a.verifyCovers) >= 2 {
					continue
				}
				a.verifyCovers = append(a.verifyCovers, id)
			}
			accepted = append(accepted, id)
		}
		ids = accepted
	}
	if err := a.core.SetCoverDemand(ids); err != nil {
		a.core.navigationError = contentCode(err)
	}
}

func (a *clientApplication) pendingOwners() int {
	if !a.core.closing {
		return 0
	}
	n := a.core.PendingOwners()
	if a.backend.pending != nil {
		n += a.backend.pending()
	}
	if a.pump != nil && !channelDone(a.pump.Done()) {
		n++
	}
	if a.finalJoin != nil && !channelDone(a.finalJoin) {
		n++
	}
	return n
}

func (a *clientApplication) verificationActions() verificationActions {
	player := func() (*app.Session, error) {
		if a.core.closing || a.core.group == nil || a.core.group.video == nil || a.core.group.video.retiring {
			return nil, catalog.Closed
		}
		return a.core.group.video.session, nil
	}
	return verificationActions{Snapshot: a.verificationSnapshot, Open: a.core.Open,
		Search: func(query string) error {
			if a.core.closing || a.core.group == nil || a.core.group.retiring {
				return catalog.Closed
			}
			return a.core.group.browser.Search(query)
		},
		SelectPart: func(id catalog.PartID) error {
			p, err := player()
			if err != nil {
				return err
			}
			return p.SelectPart(id)
		},
		SelectQuality: func(id app.QualityID) error {
			p, err := player()
			if err != nil {
				return err
			}
			return p.SelectQuality(id)
		},
		Play: func() error {
			p, err := player()
			if err != nil {
				return err
			}
			return p.Play()
		},
		Pause: func() error {
			p, err := player()
			if err != nil {
				return err
			}
			return p.Pause()
		},
		Seek: func(at time.Duration) error {
			p, err := player()
			if err != nil {
				return err
			}
			return p.Seek(at)
		},
		RequestClose: func() { a.requestClose(false) },
	}
}
