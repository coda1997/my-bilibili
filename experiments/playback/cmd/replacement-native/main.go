package main

import (
	"encoding/json"
	"flag"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/mpvprobe"
	lab "my-bilibili/experiments/playback/ui"
)

func main() {
	dir := flag.String("fixtures", "experiments/playback/testdata", "generated fixture directory")
	base := flag.String("http-base", "", "owned loopback fixture server base")
	library := flag.String("library", "", "absolute path to the provenance-verified libmpv 2.6 dylib")
	flag.Parse()
	if !filepath.IsAbs(*library) || (contract.Input{URL: *base + "/plain/combined.mp4"}).Validate() != nil {
		log.Fatal("replacement requires an explicit library and loopback fixture server")
	}
	sources := []lab.Source{{Key: "local", Label: "Local fixture", Input: contract.Input{FilePath: filepath.Join(*dir, "combined.mp4")}},
		{Key: "http", Label: "HTTP fixture", Input: contract.Input{URL: *base + "/plain/combined.mp4"}},
		{Key: "delayed", Label: "Delayed fixture", Input: contract.Input{URL: *base + "/delayed/combined.mp4"}}}
	v := &lab.View{Controller: contract.New(mpvprobe.Opener{Library: *library}, nil), Sources: sources, Surface: mpvprobe.Draw,
		Warning: "EXPERIMENTAL LIBMPV / surface and loading-lifetime probe only; not an accepted replacement"}
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Replacement Playback Probe", Width: 760, Height: 840, Content: gui.View(func(c *gui.Context) {
			v.Render(c)
			b, _ := json.Marshal(mpvprobe.Owned())
			gui.Text(c, "resources "+string(b))
		})})
		v.Quit = w.Close
		w.OnClose(func(*mygo.CloseEvent) { v.Close() })
		w.OnClosed(func() {
			if mpvprobe.Owned() != (mpvprobe.Resources{}) {
				log.Fatal("replacement resources remain after window close")
			}
			mygo.App.Quit()
		})
		w.Show()
		mygo.App.Focus()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal("replacement experiment failed")
	}
}
