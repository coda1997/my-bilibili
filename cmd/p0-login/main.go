// p0-login presents the user-assisted native login. It never writes secrets to
// task output; only explicit safe phase receipts cross that boundary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	accountview "my-bilibili/internal/desktop/account"
)

const normalTitle = "My Bilibili — 登录"
const syntheticTitle = "My Bilibili — 登录（测试）"

type options struct{ begin, restoreCheck, synthetic bool }

func parseOptions(args []string, e2e bool) (options, error) {
	flags := flag.NewFlagSet("p0-login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var o options
	flags.BoolVar(&o.begin, "begin-login", false, "begin one login after restore")
	flags.BoolVar(&o.restoreCheck, "restore-check", false, "verify stored login without a new challenge")
	flags.BoolVar(&o.synthetic, "synthetic", false, "synthetic e2e build only")
	if flags.Parse(args) != nil || flags.NArg() != 0 || o.begin && o.restoreCheck {
		return options{}, account.Invalid
	}
	if e2e {
		if !o.synthetic || o.begin || o.restoreCheck {
			return options{}, account.Invalid
		}
	} else if o.synthetic {
		return options{}, account.Invalid
	}
	return o, nil
}

type backend struct {
	gateway account.Gateway
	store   account.Store
	close   func()
	before  func(*gui.Context)
	after   func(*gui.Context, *application)
}
type receipt struct {
	Event    string        `json:"event"`
	PID      int           `json:"pid"`
	Phase    account.Phase `json:"phase,omitempty"`
	Error    account.Code  `json:"error,omitempty"`
	Stored   bool          `json:"stored"`
	Verified bool          `json:"verified"`
	Restored bool          `json:"restored"`
	Busy     bool          `json:"busy"`
}

func safeReceipt(event string, s account.State) receipt {
	phase := s.Phase
	switch phase {
	case account.Initial, account.Restoring, account.SignedOut, account.Generating, account.Waiting, account.Scanned, account.Saving, account.SaveFailed, account.Verifying, account.VerifyFailed, account.SignedIn, account.Expired, account.Failed, account.Canceling, account.SigningOut, account.DeleteFailed, account.Closing, account.Done:
	default:
		phase = account.Failed
	}
	code := s.Error
	switch code {
	case account.None, account.Invalid, account.Network, account.ServiceRejected, account.ExpiredSession, account.Storage, account.Missing, account.Busy, account.Canceled, account.Timeout, account.Closed:
	default:
		code = account.Invalid
	}
	return receipt{Event: event, PID: os.Getpid(), Phase: phase, Error: code, Stored: s.Stored, Verified: phase == account.SignedIn, Restored: s.Restored, Busy: s.Busy}
}

type application struct {
	options                                          options
	backend                                          backend
	manager                                          *account.Manager
	view                                             *accountview.View
	window                                           *mygo.Window
	out                                              io.Writer
	last                                             receipt
	haveReceipt, began, closeQueued, restoreFinished bool
	exitCode                                         int
}

func (a *application) emit(event string, s account.State) {
	r := safeReceipt(event, s)
	if event == "state" && a.haveReceipt && r == a.last {
		return
	}
	if json.NewEncoder(a.out).Encode(r) != nil {
		a.exitCode = 1
	}
	if event == "state" {
		a.last = r
		a.haveReceipt = true
	}
}
func (a *application) requestClose(discard bool) {
	if a.manager.RequestClose(discard) && a.window != nil {
		a.queueClose()
	} else if a.window != nil {
		a.window.Invalidate()
	}
}
func (a *application) queueClose() {
	if a.window != nil && !a.closeQueued {
		a.closeQueued = true
		a.window.Update(func() { a.window.Close() })
	}
}
func (a *application) observe() {
	s := a.manager.Snapshot()
	a.emit("state", s)
	if a.options.begin && !a.began && !s.Busy && (s.Phase == account.SignedOut || s.Phase == account.Expired) {
		a.began = true
		if a.manager.Begin() != nil {
			a.exitCode = 1
		}
	}
	if a.options.restoreCheck && !a.restoreFinished && !s.Busy {
		switch s.Phase {
		case account.SignedIn:
			a.restoreFinished = true
			a.emit("restore-verified", s)
			a.requestClose(false)
		case account.SignedOut, account.Expired, account.Failed, account.VerifyFailed:
			a.restoreFinished = true
			a.exitCode = 1
			a.emit("restore-failed", s)
			a.requestClose(false)
		}
	}
	if a.manager.Snapshot().Phase == account.Done {
		a.queueClose()
	}
}
func (a *application) render(c *gui.Context) {
	if a.backend.before != nil {
		a.backend.before(c)
	}
	s := a.manager.Snapshot()
	a.view.Render(c)
	if a.backend.after != nil {
		a.backend.after(c, a)
	}
	if s.Phase == account.Done {
		a.queueClose()
	}
	c.After(100 * time.Millisecond)
}
func run(args []string, out, diagnostics io.Writer) int {
	o, err := parseOptions(args, syntheticBuild)
	if err != nil {
		fmt.Fprintln(diagnostics, "invalid login options")
		return 2
	}
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(diagnostics, "native login requires macOS")
		return 2
	}
	b, err := compose(o)
	if err != nil {
		fmt.Fprintln(diagnostics, "login setup unavailable")
		return 1
	}
	defer b.close()
	a := &application{options: o, backend: b, out: out}
	var stopSignals, stopState func()
	mygo.App.WhenReady(func() {
		a.manager = account.New(b.gateway, b.store)
		a.view = accountview.New(a.manager, a.requestClose)
		title := normalTitle
		if syntheticBuild {
			title = syntheticTitle
		}
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: title, Width: 440, Height: 640, MinWidth: 400, MinHeight: 560, Content: gui.View(a.render)})
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if !a.manager.RequestClose(false) {
				e.PreventDefault()
				a.window.Invalidate()
			}
		})
		a.window.OnClosed(func() {
			if stopState != nil {
				stopState()
			}
			a.emit("window-closed", a.manager.Snapshot())
			mygo.App.Quit()
		})
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			if !a.manager.RequestClose(false) {
				e.PreventDefault()
				a.window.Show()
				a.window.Invalidate()
			}
		})
		stopState = watchState(a.manager.Snapshot, func() { a.window.Update(a.observe) }, 100*time.Millisecond)
		stopSignals = watchSignals(func() { a.window.Update(func() { a.requestClose(false) }) })
		if a.manager.Restore() != nil {
			a.exitCode = 1
			a.requestClose(false)
		}
		a.window.Show()
		mygo.App.Focus()
		a.window.Focus()
		a.emit("window-ready", a.manager.Snapshot())
	})
	if mygo.App.Run() != nil {
		fmt.Fprintln(diagnostics, "native login application failed")
		a.exitCode = 1
	}
	if stopSignals != nil {
		stopSignals()
	}
	if stopState != nil {
		stopState()
	}
	return a.resultCode()
}
func (a *application) resultCode() int {
	if a.options.restoreCheck && !a.restoreFinished {
		return 1
	}
	return a.exitCode
}

// watchState publishes background completion through the UI dispatcher even
// when the window is not drawing. Lifecycle progress must not depend on paint.
func watchState(snapshot func() account.State, dispatch func(), every time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		prior := snapshot()
		dispatch()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				current := snapshot()
				if current != prior {
					prior = current
					dispatch()
				}
			}
		}
	}()
	return func() { once.Do(func() { close(stop); <-done }) }
}

func watchSignals(closeWindow func()) func() {
	signals := make(chan os.Signal, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(done)
		for {
			select {
			case <-signals:
				closeWindow()
			case <-stop:
				return
			}
		}
	}()
	return func() { signal.Stop(signals); close(stop); <-done }
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
