//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"sync"

	_ "github.com/egoist/mygo/plugins/automation"
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/account"
)

const syntheticBuild = true
const syntheticPayload = "https://example.invalid/synthetic-login-qr?fixture=public-test-only"

type fixture struct {
	mu                               sync.Mutex
	mode                             account.PollStatus
	material                         account.Material
	failSave, failVerify, failDelete bool
	dark, large                      bool // UI-thread-only presentation switches
}

func compose(o options) (backend, error) {
	if !o.synthetic || o.begin || o.restoreCheck {
		return backend{}, account.Invalid
	}
	f := &fixture{mode: account.PollWaiting}
	return backend{gateway: f, store: f, close: func() {}, before: f.theme, after: f.render}, nil
}
func (f *fixture) Begin(context.Context) (account.Attempt, error) {
	f.mu.Lock()
	f.mode = account.PollWaiting
	f.mu.Unlock()
	return &fixtureAttempt{owner: f}, nil
}
func (f *fixture) Verify(ctx context.Context, _ account.Material) error {
	if ctx.Err() != nil {
		return account.Canceled
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failVerify {
		f.failVerify = false
		return account.Network
	}
	return nil
}
func (f *fixture) Load() (account.Material, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.material.Empty() {
		return account.Material{}, account.Missing
	}
	return account.NewMaterial(f.material.Bytes()), nil
}
func (f *fixture) Save(m account.Material) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSave {
		f.failSave = false
		return account.Storage
	}
	f.material = account.NewMaterial(m.Bytes())
	return nil
}
func (f *fixture) Delete() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDelete {
		f.failDelete = false
		return account.Storage
	}
	f.material = account.Material{}
	return nil
}

type fixtureAttempt struct{ owner *fixture }

func (*fixtureAttempt) QR() account.QR { return account.NewQR(syntheticPayload) }
func (*fixtureAttempt) Close()         {}
func (a *fixtureAttempt) Poll(ctx context.Context) (account.PollResult, error) {
	if ctx.Err() != nil {
		return account.PollResult{}, account.Canceled
	}
	a.owner.mu.Lock()
	mode := a.owner.mode
	a.owner.mu.Unlock()
	result := account.PollResult{Status: mode}
	if mode == account.PollComplete {
		result.Material = account.NewMaterial([]byte("synthetic-login-material"))
	}
	return result, nil
}
func (f *fixture) theme(c *gui.Context) {
	theme := *gui.LightTheme()
	if f.dark {
		theme = *gui.DarkTheme()
	}
	if f.large {
		theme.FontSize *= 2
	}
	c.SetTheme(&theme)
}
func (f *fixture) render(c *gui.Context, a *application) {
	gui.Column(c.Key("synthetic-controls")).Absolute().Left(-20000).Top(-20000).Children(func() {
		for _, control := range []struct {
			key string
			fn  func()
		}{
			{"test-scanned", func() { f.mu.Lock(); f.mode = account.PollScanned; f.mu.Unlock() }},
			{"test-expire", func() { f.mu.Lock(); f.mode = account.PollExpired; f.mu.Unlock() }},
			{"test-complete", func() { f.mu.Lock(); f.mode = account.PollComplete; f.mu.Unlock() }},
			{"test-save-failure", func() { f.mu.Lock(); f.failSave = true; f.mu.Unlock() }},
			{"test-verify-failure", func() { f.mu.Lock(); f.failVerify = true; f.mu.Unlock() }},
			{"test-delete-failure", func() { f.mu.Lock(); f.failDelete = true; f.mu.Unlock() }},
			{"test-dark", func() { f.dark = true }},
			{"test-light", func() { f.dark = false }},
			{"test-large", func() { f.large = true }},
			{"test-normal-size", func() { f.large = false }},
			{"test-small-window", func() { a.window.SetContentSize(400, 560) }},
			{"test-close", func() { a.window.Close() }},
		} {
			if gui.Button(c.Key(control.key), control.key).Clicked() {
				control.fn()
				c.Invalidate()
			}
		}
	})
	themeName := "light"
	if f.dark {
		themeName = "dark"
	}
	gui.Text(c.Key("synthetic-theme"), themeName).Absolute().Left(-20000).Top(-20000).Size(1, 1)
	encoded, _ := json.Marshal(safeReceipt("synthetic-state", a.manager.Snapshot()))
	gui.Text(c.Key("synthetic-state"), string(encoded)).Absolute().Left(-20000).Top(-20000).Size(1, 1)
	gui.Text(c.Key("synthetic-only-banner"), "仅用于界面验证 · 示例二维码").Absolute().Right(8).Top(4).FontSize(11)
}
