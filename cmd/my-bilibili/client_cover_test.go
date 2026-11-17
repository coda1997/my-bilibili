package main

import (
	"context"
	"slices"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/desktop"
	catalogview "my-bilibili/internal/desktop/catalog"
)

type coverTestSource struct{ app.Source }

func (s coverTestSource) Detail(ctx context.Context) (catalog.Video, error) {
	v, err := s.Source.Detail(ctx)
	v.CoverID = "synthetic-detail-cover"
	return v, err
}

type coverTestBrowser struct{}

func (coverTestBrowser) State() catalog.SearchState {
	return catalog.SearchState{Phase: catalog.SearchReady, Page: 1, TotalPages: 1, Results: []catalog.Summary{{ID: "BV0000000002", Title: "Original result", Creator: "Original creator", CoverID: "synthetic-search-cover"}}}
}
func (coverTestBrowser) Search(string) error { return nil }
func (coverTestBrowser) NextPage() error     { return nil }
func (coverTestBrowser) Retry() error        { return nil }

func TestClientCoversFollowActuallyMountedPanesAcrossLayoutChanges(t *testing.T) {
	h := newCoreHarness(t, true)
	original := h.core.config.newSource
	h.core.config.newSource = func(scope *transport.Scope, details *catalogapi.Client, entry catalog.Entry) (app.Source, error) {
		source, err := original(scope, details, entry)
		if err != nil {
			return nil, err
		}
		return coverTestSource{source}, nil
	}
	startCore(t, h)
	openCore(t, h, "BV0000000001")
	g := h.core.group
	g.view.Close()
	var err error
	g.view, err = catalogview.New(coverTestBrowser{}, h.core.OpenEntry, scopeCoverView{g})
	if err != nil {
		t.Fatal(err)
	}
	a := &clientApplication{core: h.core, shell: &desktop.Client{}, interval: 500 * time.Millisecond}
	a.syncViews()
	full := false
	g.video.player.Fullscreen = func() bool { return full }
	g.video.player.SetFullscreen = func(value bool) { full = value }
	ui := gui.NewTester(a.render, 1120, 760)
	ui.Frame()
	if !slices.Equal(h.core.coverDemand, []string{"synthetic-detail-cover", "synthetic-search-cover"}) {
		t.Fatal("wide mounted cover demand missing or unordered", h.core.coverDemand)
	}
	full = true
	ui.Frame()
	if len(h.core.coverDemand) != 0 {
		t.Fatal("fullscreen retained unmounted detail or catalog covers")
	}
	full = false
	ui.Frame()
	ui.SetSize(800, 600)
	ui.Frame()
	if !slices.Equal(h.core.coverDemand, []string{"synthetic-detail-cover"}) {
		t.Fatal("compact player retained stale sidebar demand", h.core.coverDemand)
	}
	a.shell.ShowCatalog()
	ui.Frame()
	ui.Frame()
	if !slices.Equal(h.core.coverDemand, []string{"synthetic-search-cover"}) {
		t.Fatal("compact catalog retained hidden detail demand", h.core.coverDemand)
	}
	a.shell.ShowHistory()
	ui.Frame()
	if len(h.core.coverDemand) != 0 {
		t.Fatal("hidden content retained cover requests")
	}
	if h.core.config.metrics.cover.snapshot().calls != 0 {
		t.Fatal("painting initiated cover I/O")
	}
}
