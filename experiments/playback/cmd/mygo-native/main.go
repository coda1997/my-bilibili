package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/cmd/fixturehttp"
	"my-bilibili/experiments/playback/contract"
	"my-bilibili/experiments/playback/player"
	lab "my-bilibili/experiments/playback/ui"
)

func main() {
	dir := flag.String("fixtures", "experiments/playback/testdata", "generated fixture directory")
	base := flag.String("http-base", "", "owned loopback fixture server base")
	flag.Parse()
	sources := []lab.Source{{Key: "local", Label: "Local fixture", Input: contract.Input{FilePath: filepath.Join(*dir, "combined.mp4")}}}
	if *base != "" {
		if (contract.Input{URL: *base + "/plain/combined.mp4"}).Validate() != nil {
			log.Fatal("invalid loopback base")
		}
		for _, p := range []struct{ key, label, route string }{{"http", "HTTP fixture", "plain"}, {"delayed", "Delayed fixture", "delayed"}, {"protected", "Protected (no header)", "protected"}, {"failure", "HTTP failure", "failure"}} {
			sources = append(sources, lab.Source{Key: p.key, Label: p.label, Input: contract.Input{URL: *base + "/" + p.route + "/combined.mp4"}})
		}
		sources = append(sources, lab.Source{Key: "headers", Label: "With required header", Input: contract.Input{URL: *base + "/protected/combined.mp4", Headers: map[string]string{fixturehttp.Header: fixturehttp.Sentinel}}}, lab.Source{Key: "separate", Label: "Separate video/audio", Input: contract.Input{URL: *base + "/plain/video.mp4", AudioURL: *base + "/plain/audio.m4a"}})
	}
	v := &lab.View{Controller: contract.New(player.Opener{}, nil), Sources: sources, Surface: player.Draw}
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "M1 Playback Lab", Width: 760, Height: 840, Content: gui.View(v.Render)})
		v.Quit = w.Close
		w.OnClose(func(*mygo.CloseEvent) { v.Close() })
		w.OnClosed(func() { mygo.App.Quit() })
		w.Show()
		mygo.App.Focus()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal("native experiment failed")
	}
}
