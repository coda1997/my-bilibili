//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	_ "github.com/egoist/mygo/plugins/automation"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

// The test seam creates its own loopback listener. No caller-supplied media or
// origin can enter the normal app or this controlled delayed-loading scenario.
type delayedSource struct {
	app.Source
	origin           string
	requests, active atomic.Int64
}

func (s *delayedSource) Resolve(ctx context.Context, id catalog.PartID) (app.Resolution, error) {
	if _, err := s.Source.Resolve(ctx, id); err != nil {
		return app.Resolution{}, err
	}
	return app.Resolution{Input: playback.Input{URL: s.origin + "/delayed"}}, nil
}
func automationSource(source app.Source) (app.Source, func(), error) {
	mode := os.Getenv("MY_BILIBILI_E2E_SCENARIO")
	if mode == "" {
		return source, func() {}, nil
	}
	if mode != "delayed" {
		return nil, nil, playback.Invalid
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, playback.Media
	}
	delayed := &delayedSource{Source: source, origin: "http://" + listener.Addr().String()}
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/delayed" {
			http.NotFound(w, r)
			return
		}
		delayed.requests.Add(1)
		delayed.active.Add(1)
		defer delayed.active.Add(-1)
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			http.Error(w, "synthetic delayed fixture", http.StatusServiceUnavailable)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	return delayed, func() { _ = server.Close() }, nil
}
func (a *application) automationRender(c *gui.Context) {
	// Absolute placement leaves the product layout intact. These controls and
	// sanitized observations are linked only into explicitly requested e2e builds.
	gui.Column(c.Key("e2e-controls")).Absolute().Left(-20000).Top(-20000).Children(func() {
		if gui.Button(c.Key("e2e-cancel"), "Cancel parent").Clicked() {
			a.cancel()
			c.Invalidate()
		}
		if gui.Button(c.Key("e2e-dispose"), "Dispose playback").Clicked() {
			a.closeError = a.dispose()
			c.Invalidate()
		}
		if gui.Button(c.Key("e2e-close-window"), "Close window").Clicked() {
			a.window.Close()
		}
		if gui.Button(c.Key("e2e-narrow"), "Narrow window").Clicked() {
			a.window.SetContentSize(800, 600)
		}
		if gui.Button(c.Key("e2e-wide"), "Wide window").Clicked() {
			a.window.SetContentSize(1120, 760)
		}
	})
	observation := previewbundle.Observation{State: a.session.State(), Fullscreen: a.window.IsFullScreen(), Resources: mpv.Owned(), Tracks: mpv.Inspect(a.session.Engine()), DisposalError: ""}
	if delayed, ok := a.source.(*delayedSource); ok {
		active := delayed.active.Load()
		observation.Delay = previewbundle.DelayObservation{Requests: delayed.requests.Load(), Active: active}
	}
	if a.closeError != nil {
		observation.DisposalError = playback.Media
	}
	b, _ := json.Marshal(observation)
	gui.Text(c.Key("e2e-observation"), string(b)).Absolute().Left(-20000).Top(-20000).Size(1, 1)
	a.renderFullscreen(c)
}
