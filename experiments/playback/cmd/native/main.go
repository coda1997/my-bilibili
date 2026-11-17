package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/cmd/fixturehttp"
	lab "my-bilibili/experiments/playback/ui"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/playback"
)

func main() {
	dir := flag.String("fixtures", "experiments/playback/testdata", "generated fixture directory")
	base := flag.String("http-base", "", "owned loopback fixture server base")
	library := flag.String("library", "", "absolute path to the provenance-verified libmpv API 2.6 library (required)")
	flag.Parse()
	v, err := newView(*dir, *base, *library)
	if err != nil {
		log.Fatal(err)
	}
	mygo.App.WhenReady(func() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "M1 Playback Lab / libmpv", Width: 760, Height: 840, Content: gui.View(func(c *gui.Context) {
			v.Render(c)
			b, _ := json.Marshal(mpv.Owned())
			gui.Text(c, "resources "+string(b))
			b, _ = json.Marshal(mpv.Inspect(v.Controller.Engine()))
			gui.Text(c, "tracks "+string(b))
		})})
		v.Quit = w.Close
		w.OnClose(func(*mygo.CloseEvent) { v.Close() })
		w.OnClosed(func() {
			if mpv.Owned() != (mpv.Resources{}) {
				log.Fatal("libmpv resources remain after window close")
			}
			log.Print("libmpv window closed with zero resources")
			mygo.App.Quit()
		})
		w.Show()
		mygo.App.Focus()
		w.Focus()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal("native experiment failed")
	}
}

func newView(dir, base, library string) (*lab.View, error) {
	info, err := os.Lstat(library)
	if !filepath.IsAbs(library) || err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("libmpv requires -library with an absolute path to the verified library; see experiments/playback/cmd/README.md")
	}
	sources := []lab.Source{{Key: "local", Label: "Local fixture", Input: playback.Input{FilePath: filepath.Join(dir, "combined.mp4")}}}
	if base != "" {
		u, err := url.Parse(base)
		if err != nil || (playback.Input{URL: base}).Validate() != nil || u.Path != "" && u.Path != "/" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery {
			return nil, errors.New("invalid loopback fixture server base: use an HTTP origin without a path or query")
		}
		base = u.Scheme + "://" + u.Host
		sources = append(sources,
			lab.Source{Key: "http", Label: "HTTP fixture", Input: playback.Input{URL: base + "/plain/combined.mp4"}},
			lab.Source{Key: "delayed", Label: "Delayed fixture", Input: playback.Input{URL: base + "/delayed/combined.mp4"}},
			lab.Source{Key: "protected", Label: "Required headers", Input: playback.Input{URL: base + "/protected/combined.mp4", Headers: map[string]string{fixturehttp.Header: fixturehttp.ProbeSentinel, "Referer": fixturehttp.ProbeReferer}}},
			lab.Source{Key: "no-headers", Label: "Missing headers", Input: playback.Input{URL: base + "/protected/combined.mp4"}},
			lab.Source{Key: "wrong-headers", Label: "Wrong headers", Input: playback.Input{URL: base + "/protected/combined.mp4", Headers: map[string]string{fixturehttp.Header: "wrong", "Referer": fixturehttp.ProbeReferer}}},
			lab.Source{Key: "separate", Label: "Separate tracks", Input: playback.Input{URL: base + "/protected/video.mp4", AudioURL: base + "/protected/audio.m4a", Headers: map[string]string{fixturehttp.Header: fixturehttp.ProbeSentinel, "Referer": fixturehttp.ProbeReferer}}},
			lab.Source{Key: "missing-audio", Label: "Failed audio", Input: playback.Input{URL: base + "/plain/video.mp4", AudioURL: base + "/failure/audio.m4a"}},
			lab.Source{Key: "failure", Label: "HTTP failure", Input: playback.Input{URL: base + "/failure/combined.mp4"}})
	}
	for _, source := range sources {
		if source.Input.Validate() != nil {
			return nil, errors.New("invalid fixture input")
		}
	}
	return &lab.View{Controller: playback.New(mpv.Opener{Library: library}, nil), Sources: sources, Surface: mpv.Draw,
		Warning: "LIBMPV PLAYER / synthetic fixture playback"}, nil
}
