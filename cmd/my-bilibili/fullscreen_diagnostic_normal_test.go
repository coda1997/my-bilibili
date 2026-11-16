//go:build !e2e

package main

import (
	gui "github.com/egoist/mygo/ui"
	"testing"
)

func TestNormalBuildDoesNotRecordOrExposeFullscreenTelemetry(t *testing.T) {
	a := &application{}
	desired := true
	// A nil window proves the normal hook performs no native reads or actions.
	a.recordFullscreen("request", &desired)
	a.recordFullscreen("did-enter", nil)
	a.recordFullscreen("did-exit", nil)
	ui := gui.NewTester(a.automationRender, 1120, 760)
	if texts := ui.Texts(); len(texts) != 0 {
		t.Fatal("normal build exposes diagnostic UI", texts)
	}
}
