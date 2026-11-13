// Target-native is the explicit, transient-stdin composition for one reviewed source.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	"my-bilibili/experiments/playback/targetprobe"
)

func main() {
	fs := flag.NewFlagSet("target-native", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	library := fs.String("library", "", "verified absolute libmpv path")
	if fs.Parse(os.Args[1:]) != nil || fs.NArg() != 0 || !filepath.IsAbs(*library) {
		fmt.Fprintln(os.Stderr, "invalid target configuration")
		os.Exit(2)
	}
	// Direct launches also drop proxy inheritance; this affects only this process.
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		_ = os.Unsetenv(k)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	b, e := targetprobe.ReadDescriptor(ctx, os.Stdin)
	cancel()
	if e != nil {
		fmt.Fprintln(os.Stderr, "target input unavailable")
		os.Exit(2)
	}
	p, e := targetprobe.Parse(b, nil)
	b = nil
	if e != nil {
		fmt.Fprintln(os.Stderr, "target input rejected")
		os.Exit(2)
	}
	session := targetprobe.NewSession(mpvprobe.Opener{Library: *library, Policy: p}, p, nil)
	mygo.App.WhenReady(func() {
		var w *mygo.Window
		quit := func() {
			session.Close()
			if w != nil {
				w.Close()
			} else {
				mygo.App.Quit()
			}
		}
		w = mygo.NewWindow(mygo.WindowOptions{Title: "Authorized Target Playback Probe", Width: 760, Height: 650, Content: gui.View(func(c *gui.Context) {
			session.Tick()
			c.After(contract.PollInterval)
			if session.Expired() {
				quit()
				return
			}
			gui.Column(c).Padding(18).Gap(8).Children(func() {
				gui.Text(c, "Authorized public test video").FontSize(23)
				gui.Text(c, "One reviewed rendition · transient session").FontSize(12)
				gui.Box(c).Width(640).Height(360).Children(func() { mpvprobe.DrawTarget(c, session.Engine()) })
				gui.Row(c).Gap(8).Children(func() {
					if gui.Button(c.Key("open"), "Open").Disabled(session.Snapshot().Owned).Clicked() {
						_ = session.Open(context.Background())
					}
					for _, b := range []struct {
						k, label string
						cmd      contract.Command
					}{{"play", "Play", contract.Command{Kind: contract.Play}}, {"pause", "Pause", contract.Command{Kind: contract.Pause}}, {"seek", "Seek 3s", contract.Command{Kind: contract.Seek, Position: 3 * time.Second}}, {"volume", "Volume 25%", contract.Command{Kind: contract.Volume, Value: .25}}, {"rate", "Rate 1.5x", contract.Command{Kind: contract.Rate, Value: 1.5}}} {
						if gui.Button(c.Key(b.k), b.label).Clicked() {
							_ = session.Command(b.cmd)
						}
					}
				})
				gui.Row(c).Gap(8).Children(func() {
					if gui.Button(c.Key("cancel"), "Cancel").Clicked() {
						session.Cancel()
					}
					if gui.Button(c.Key("close"), "Close media").Clicked() {
						session.Close()
					}
					if gui.Button(c.Key("retry"), "Retry same input").Clicked() {
						_ = session.Retry(context.Background())
					}
					if gui.Button(c.Key("quit"), "Quit").Clicked() {
						quit()
					}
				})
				status := struct {
					targetprobe.Status
					WindowHidden bool `json:"window_hidden"`
				}{session.Snapshot(), c.WindowHidden()}
				data, _ := json.Marshal(status)
				gui.Text(c, "snapshot "+string(data)).FontSize(9)
				data, _ = json.Marshal(mpvprobe.Owned())
				gui.Text(c, "resources "+string(data)).FontSize(9)
				data, _ = json.Marshal(mpvprobe.Inspect(session.Engine()))
				gui.Text(c, "tracks "+string(data)).FontSize(9)
			})
		})})
		w.OnClose(func(*mygo.CloseEvent) { session.Close() })
		w.OnClosed(func() {
			if mpvprobe.Owned() != (mpvprobe.Resources{}) {
				fmt.Fprintln(os.Stderr, "target cleanup failed")
				os.Exit(1)
			}
			mygo.App.Quit()
		})
		w.Show()
		mygo.App.Focus()
		w.Focus()
	})
	if mygo.App.Run() != nil {
		fmt.Fprintln(os.Stderr, "target application failed")
		os.Exit(1)
	}
}
