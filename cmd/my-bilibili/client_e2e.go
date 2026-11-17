//go:build e2e

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
	accountapi "my-bilibili/internal/adapters/bilibili/account"
	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	mediaapi "my-bilibili/internal/adapters/bilibili/playback"
	"my-bilibili/internal/adapters/bilibili/sessiondata"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/adapters/localhistory"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/history"
	"my-bilibili/internal/previewbundle"
)

func validClientScene(scene string) bool {
	switch scene {
	case "search", "playback", "segments", "history", "account-expiry", "account-signout", "history-corrupt", "history-in-use", "close-held":
		return true
	}
	return false
}
func clientBuildMode() (bool, string, error) {
	scene := os.Getenv("MY_BILIBILI_E2E_CLIENT_SCENARIO")
	if scene != "" && (!validClientScene(scene) || os.Getenv("MY_BILIBILI_E2E_SCENARIO") != "") {
		return true, "", catalog.Invalid
	}
	return true, scene, nil
}

type syntheticAccountStore struct{ material account.Material }

func (s *syntheticAccountStore) Load() (account.Material, error) {
	if len(s.material.Bytes()) == 0 {
		return account.Material{}, account.Missing
	}
	return account.NewMaterial(s.material.Bytes()), nil
}
func (s *syntheticAccountStore) Save(m account.Material) error {
	s.material = account.NewMaterial(m.Bytes())
	return nil
}
func (s *syntheticAccountStore) Delete() error { s.material = account.Material{}; return nil }

type heldHistory struct {
	history.Store
	gate <-chan struct{}
}

func (s heldHistory) Load() ([]history.Record, error) { <-s.gate; return s.Store.Load() }

type syntheticClient struct {
	scene           string
	routes          *syntheticClientRoutes
	gate            chan struct{}
	once            sync.Once
	holder          history.Store
	holderDone      chan struct{}
	holderError     error
	releaseAt       time.Time
	closeOnMinimize bool
}

func (s *syntheticClient) release() { s.once.Do(func() { close(s.gate) }) }

func composeClient(resources string, options clientOptions) (clientBackend, error) {
	if options.fixture || options.verifyPath != "" || !validClientScene(options.scenario) {
		return clientBackend{}, catalog.Invalid
	}
	directory := os.Getenv("MY_BILIBILI_E2E_HISTORY_DIR")
	if directory == "" || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return clientBackend{}, catalog.Invalid
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return clientBackend{}, catalog.Invalid
	}
	known, err := fixture.New(resources)
	if err != nil {
		return clientBackend{}, err
	}
	material, err := sessiondata.Parse([]string{"SESSDATA=SYNTHETIC_CLIENT_ONLY; Domain=bilibili.com; Path=/; Secure; Max-Age=3600"}, time.Now())
	if err != nil {
		return clientBackend{}, err
	}
	local, err := localhistory.New(directory)
	if err != nil {
		return clientBackend{}, err
	}
	s := &syntheticClient{scene: options.scenario, gate: make(chan struct{})}
	s.routes, err = newSyntheticClientRoutes(options.scenario, s.gate)
	if err != nil {
		return clientBackend{}, err
	}
	if s.scene == "history-corrupt" {
		f, err := os.OpenFile(filepath.Join(directory, "history.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return clientBackend{}, history.Storage
		}
		_, writeErr := f.WriteString(`{"original_synthetic_corruption":true}`)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			return clientBackend{}, history.Storage
		}
	}
	if s.scene == "history-in-use" {
		s.holder, err = localhistory.New(directory)
		if err != nil {
			return clientBackend{}, err
		}
		if err = s.holder.Acquire(); err != nil {
			return clientBackend{}, err
		}
	}
	var store history.Store = local
	if s.scene == "close-held" {
		store = heldHistory{Store: local, gate: s.gate}
	} else {
		s.release()
	}
	metrics := &clientMetrics{}
	api := countedTransport{delegate: s.routes, activity: &metrics.api}
	cover := countedTransport{delegate: s.routes, activity: &metrics.cover}
	gateway := accountapi.New(api)
	service := transport.New(api)
	return clientBackend{config: clientConfig{gateway: gateway, store: &syntheticAccountStore{material: material}, closeGateway: gateway.Close, service: service,
		coverTransport: cover, historyStore: store, opener: mpv.Opener{Library: previewbundle.Library}, fixtureAudio: true, metrics: metrics,
		newSource: func(scope *transport.Scope, details *catalogapi.Client, entry catalog.Entry) (app.Source, error) {
			source, err := mediaapi.NewSource(scope, details, entry)
			if err != nil {
				return nil, err
			}
			return &syntheticNativeSource{Source: source, fixture: known}, nil
		}}, setup: s.setup, render: s.render, tick: s.tick, closed: s.closed, pending: s.pending, cleanup: s.cleanup}, nil
}

func (s *syntheticClient) setup(a *clientApplication) {
	a.window.OnMinimize(func() {
		a.minimized = true
		if s.closeOnMinimize {
			a.requestClose(false)
			s.emit(a, previewbundle.ClientWindowMinimized)
			s.releaseAt = time.Now().Add(150 * time.Millisecond)
		}
	})
}
func (s *syntheticClient) render(c *gui.Context, a *clientApplication) {
	gui.Column(c.Key("e2e-client-controls")).Absolute().Left(-20000).Top(-20000).Children(func() {
		if gui.Button(c.Key("e2e-close-window"), "Close synthetic window").Clicked() {
			a.window.Close()
		}
		if gui.Button(c.Key("e2e-narrow"), "Narrow window").Clicked() {
			a.window.SetContentSize(800, 600)
		}
		if gui.Button(c.Key("e2e-wide"), "Wide window").Clicked() {
			a.window.SetContentSize(1120, 760)
		}
		if s.scene == "account-expiry" {
			if gui.Button(c.Key("e2e-expire-account"), "Synthetic account expiration").Clicked() {
				a.core.account.SessionExpired(a.core.account.Snapshot().SessionEpoch)
			}
		}
		if s.scene == "history-in-use" {
			if gui.Button(c.Key("e2e-release-history"), "Release synthetic history holder").Clicked() && s.holderDone == nil {
				s.releaseHolder()
			}
		}
		if s.scene == "close-held" {
			if gui.Button(c.Key("e2e-minimized-close"), "Close after native minimize").Clicked() {
				s.closeOnMinimize = true
				a.window.Minimize()
			}
		}
	})
	data, err := json.Marshal(a.clientObservation())
	if err != nil {
		a.closeError = catalog.Invalid
		a.requestClose(false)
		return
	}
	gui.Text(c.Key("e2e-client-observation"), string(data)).Absolute().Left(-20000).Top(-20000).Size(1, 1)
}
func (s *syntheticClient) tick(a *clientApplication, now time.Time) {
	if !s.releaseAt.IsZero() && !now.Before(s.releaseAt) {
		s.release()
		s.releaseAt = time.Time{}
	}
	if a.core.closing && s.scene == "close-held" && !s.closeOnMinimize {
		s.release()
	}
	if a.core.closing && s.holder != nil && s.holderDone == nil {
		s.releaseHolder()
	}
	if s.holderDone != nil && channelDone(s.holderDone) && s.holderError != nil {
		a.closeError = history.Storage
	}
}
func (s *syntheticClient) releaseHolder() {
	s.holderDone = make(chan struct{})
	go func() { defer close(s.holderDone); s.holderError = s.holder.Close() }()
}
func (s *syntheticClient) pending() int {
	if s.holder != nil && (s.holderDone == nil || !channelDone(s.holderDone) || s.holderError != nil) {
		return 1
	}
	return 0
}
func (s *syntheticClient) emit(a *clientApplication, event string) {
	if json.NewEncoder(os.Stdout).Encode(previewbundle.ClientReceipt{Event: event, Observation: a.clientObservation()}) != nil {
		a.closeError = catalog.Unavailable
	}
}
func (s *syntheticClient) closed(a *clientApplication) { s.emit(a, previewbundle.ClientWindowClosed) }
func (s *syntheticClient) cleanup() error {
	s.release()
	if s.holder != nil {
		if s.holderDone != nil {
			<-s.holderDone
			return s.holderError
		}
		return s.holder.Close()
	}
	return nil
}
