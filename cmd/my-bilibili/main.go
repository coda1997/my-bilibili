package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	"my-bilibili/internal/app"
	"my-bilibili/internal/desktop"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/previewbundle"
)

type application struct {
	session    *app.Session
	cancel     context.CancelFunc
	window     *mygo.Window
	view       *desktop.View
	closeError error
	source     app.Source
	fullscreen fullscreenTelemetry
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	if err := previewbundle.VerifyRuntime(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return contract.Media
	}
	resources, err := previewbundle.Resources(executable)
	if err != nil {
		return err
	}
	source, err := fixture.New(resources)
	if err != nil {
		return err
	}
	source, cleanup, err := automationSource(source)
	if err != nil {
		return err
	}
	defer cleanup()
	session, err := app.New(source, contract.New(mpvprobe.Opener{Library: previewbundle.Library}, nil))
	if err != nil {
		return err
	}
	coverData, err := os.ReadFile(filepath.Join(resources, "cover.png"))
	if err != nil {
		return contract.Media
	}
	cover, err := gui.DecodeBitmap(coverData)
	if err != nil {
		return contract.Media
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &application{session: session, cancel: cancel, source: source}
	mygo.App.WhenReady(func() {
		interval, err := doubleClickInterval()
		if err != nil {
			a.closeError = err
			mygo.App.Quit()
			return
		}
		a.view = &desktop.View{Player: session, DoubleClickInterval: interval,
			Surface: func(c *gui.Context, w, h float32) { mpvprobe.DrawSized(c, session.Engine(), w, h) },
			Cover: func(c *gui.Context, id string) {
				if id == "preview-cover" {
					gui.Image(c, cover).Size(80, 45)
				}
			},
			Fullscreen: func() bool { return a.window != nil && a.window.IsFullScreen() },
			SetFullscreen: func(enabled bool) {
				a.recordFullscreen("request", &enabled)
				a.window.SetFullScreen(enabled)
			},
		}
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: "My Bilibili Preview", Width: 1120, Height: 760, MinWidth: 800, MinHeight: 600, Content: gui.View(func(c *gui.Context) {
			a.view.Render(c)
			a.automationRender(c)
		})})
		a.window.OnEnterFullScreen(func() {
			a.recordFullscreen("did-enter", nil)
			a.window.Invalidate()
		})
		a.window.OnLeaveFullScreen(func() {
			a.recordFullscreen("did-exit", nil)
			a.window.Invalidate()
		})
		a.window.OnClose(func(*mygo.CloseEvent) { a.onClose() })
		a.window.OnClosed(func() { mygo.App.Quit() })
		if err := session.Start(ctx); err != nil {
			a.closeError = err
			a.window.Close()
			return
		}
		a.window.Show()
		mygo.App.Focus()
		a.window.Focus()
	})
	if err := mygo.App.Run(); err != nil {
		return contract.Media
	}
	if err := a.dispose(); err != nil {
		return err
	}
	return a.closeError
}

func (a *application) dispose() error {
	if err := a.session.Close(); err != nil {
		return err
	}
	if mpvprobe.Owned() != (mpvprobe.Resources{}) {
		return contract.Media
	}
	return nil
}

func (a *application) onClose() {
	err := a.dispose()
	if a.closeError == nil {
		a.closeError = err
	}
}
