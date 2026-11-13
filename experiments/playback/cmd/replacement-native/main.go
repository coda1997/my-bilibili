package main

import (
	"encoding/json"
	"flag"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/cmd/fixturehttp"
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
		{Key: "delayed", Label: "Delayed fixture", Input: contract.Input{URL: *base + "/delayed/combined.mp4"}},
		{Key: "protected", Label: "Required headers", Input: contract.Input{URL: *base + "/protected/combined.mp4", Headers: map[string]string{fixturehttp.Header: fixturehttp.ProbeSentinel, "Referer": fixturehttp.ProbeReferer}}},
		{Key: "no-headers", Label: "Missing headers", Input: contract.Input{URL: *base + "/protected/combined.mp4"}},
		{Key: "wrong-headers", Label: "Wrong headers", Input: contract.Input{URL: *base + "/protected/combined.mp4", Headers: map[string]string{fixturehttp.Header: "wrong", "Referer": fixturehttp.ProbeReferer}}},
		{Key: "separate", Label: "Separate tracks", Input: contract.Input{URL: *base + "/protected/video.mp4", AudioURL: *base + "/protected/audio.m4a", Headers: map[string]string{fixturehttp.Header: fixturehttp.ProbeSentinel, "Referer": fixturehttp.ProbeReferer}}},
		{Key: "missing-audio", Label: "Failed audio", Input: contract.Input{URL: *base + "/plain/video.mp4", AudioURL: *base + "/failure/audio.m4a"}},
		{Key: "failure", Label: "HTTP failure", Input: contract.Input{URL: *base + "/failure/combined.mp4"}}}
	v := &lab.View{Controller: contract.New(mpvprobe.Opener{Library: *library}, nil), Sources: sources, Surface: mpvprobe.Draw,
		Warning: "EXPERIMENTAL LIBMPV / fixture capability probe; audio/sync and final M1 unverified"}
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Replacement Playback Probe", Width: 760, Height: 840, Content: gui.View(func(c *gui.Context) {
			v.Render(c)
			b, _ := json.Marshal(mpvprobe.Owned())
			gui.Text(c, "resources "+string(b))
			b, _ = json.Marshal(mpvprobe.Inspect(v.Controller.Engine()))
			gui.Text(c, "tracks "+string(b))
		})})
		v.Quit = func() {
			log.Print("replacement explicit quit action")
			w.Close()
		}
		w.OnClose(func(*mygo.CloseEvent) {
			log.Print("replacement window closing")
			v.Close()
		})
		w.OnClosed(func() {
			if mpvprobe.Owned() != (mpvprobe.Resources{}) {
				log.Fatal("replacement resources remain after window close")
			}
			log.Print("replacement window closed with zero resources")
			mygo.App.Quit()
		})
		w.Show()
		mygo.App.Focus()
		w.Focus()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal("replacement experiment failed")
	}
}
