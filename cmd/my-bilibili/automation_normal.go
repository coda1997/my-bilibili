//go:build !e2e

package main

import (
	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
)

func automationSource(source app.Source) (app.Source, func(), error) { return source, func() {}, nil }
func (a *application) automationRender(*gui.Context)                 {}
func (*application) automationSetup()                                {}
func (*application) automationClosed()                               {}

// Normal builds retain neither fullscreen diagnostic events nor an e2e node.
type fullscreenTelemetry struct{}

func (*application) recordFullscreen(string, *bool) {}
