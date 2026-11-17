//go:build e2e

package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func syntheticCore(t *testing.T, scene, directory string) (*clientCore, clientBackend, *coreOpener) {
	t.Helper()
	if directory == "" {
		directory = t.TempDir()
	}
	if os.Chmod(directory, 0700) != nil {
		t.Fatal("private synthetic directory unavailable")
	}
	t.Setenv("MY_BILIBILI_E2E_HISTORY_DIR", directory)
	backend, err := composeClient("../../internal/fixture/assets", clientOptions{scenario: scene})
	if err != nil {
		t.Fatal(err)
	}
	opener := &coreOpener{}
	backend.config.opener = opener
	core, err := newClientCore(backend.config)
	if err != nil {
		t.Fatal(err)
	}
	if err = core.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		core.RequestClose(false)
		app := &clientApplication{core: core}
		deadline := time.Now().Add(3 * time.Second)
		for !core.closed || backend.pending() != 0 {
			backend.tick(app, time.Now())
			core.Tick()
			if time.Now().After(deadline) {
				t.Error("synthetic ownership did not close")
				break
			}
			time.Sleep(time.Millisecond)
		}
		if err := backend.cleanup(); err != nil {
			t.Error(err)
		}
	})
	waitCore(t, core, func() bool {
		return core.account.Snapshot().Phase == account.SignedIn && core.group != nil && core.group.scope.State().Authenticated && !core.group.retiring
	})
	return core, backend, opener
}

func TestSyntheticClientUsesActualSearchDetailMediaAndSegmentAdapters(t *testing.T) {
	c, _, engines := syntheticCore(t, "search", "")
	waitCore(t, c, func() bool { return c.history.State().Phase == history.Ready })
	if err := c.group.browser.Search("测试视频"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool { return c.group.browser.State().Phase == catalog.SearchReady })
	if s := c.group.browser.State(); len(s.Results) != 2 || s.Page != 1 {
		t.Fatal("first synthetic page invalid")
	}
	if err := c.group.browser.NextPage(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool { return c.group.browser.State().Error == catalog.ServiceUnavailable })
	if s := c.group.browser.State(); len(s.Results) != 2 || s.Page != 1 {
		t.Fatal("failed append erased accepted rows")
	}
	if err := c.group.browser.Retry(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool { return c.group.browser.State().Page == 2 })
	if len(c.group.browser.State().Results) != 3 {
		t.Fatal("retry did not deduplicate actual adapter rows")
	}
	entry, err := catalog.ParseEntry("https://www.bilibili.com/video/BV0000000001?p=2")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.OpenEntry(entry); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool {
		return c.group.video != nil && c.group.video.session.State().PlaybackPart == "102" && c.group.video.session.State().Playback.State == playback.Ready
	})
	v := c.group.video
	if s := v.session.State(); s.PlaybackQuality != "80" || len(s.Qualities) != 2 || s.Video.ID != "BV0000000001" {
		t.Fatal("actual media conversion was bypassed")
	}
	waitCore(t, c, func() bool {
		return v.loader != nil && v.loader.State().Phase == danmaku.LoadReady && v.windowRevision > 0
	})
	if v.loader.State().Entries != 5 || !v.hasComment {
		t.Fatal("original protobuf did not reach actual loader window")
	}
	if err = v.session.Seek(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool {
		s := v.session.State()
		return s.Playback.State == playback.Paused && s.Playback.Position == 5*time.Second
	})
	if err = v.session.SelectQuality("64"); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool {
		s := v.session.State()
		return s.PlaybackQuality == "64" && !s.Resuming && s.Playback.State == playback.Paused && s.Playback.Position == 5*time.Second
	})
	if len(engines.engines) != 2 || v.session.State().Playback.Position != 5*time.Second || v.session.State().DesiredPlaying {
		t.Fatal("native replacement seam lost exact quality/position/pause")
	}
	waitCore(t, c, func() bool { return c.history.State().SavedRevision > 0 && !c.history.State().Dirty })
}

func TestSyntheticClientRestartLoadsSameCanonicalHistoryAndFreshInput(t *testing.T) {
	directory := t.TempDir()
	c, backend, engines := syntheticCore(t, "history", directory)
	waitCore(t, c, func() bool { return c.history.State().Phase == history.Ready })
	if err := c.OpenEntry(catalog.Entry{Kind: catalog.BVEntry, ID: syntheticBV(1)}); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool {
		return c.group.video != nil && c.group.video.session.State().PlaybackPart != "" && c.group.video.session.State().Playback.State == playback.Ready
	})
	v := c.group.video
	if err := v.session.Play(); err != nil {
		t.Fatal(err)
	}
	engines.engines[0].state.Position = 7 * time.Second
	waitCore(t, c, func() bool {
		return v.session.State().Playback.State == playback.Playing && v.session.State().Playback.Position == 7*time.Second
	})
	if err := v.session.Pause(); err != nil {
		t.Fatal(err)
	}
	waitCore(t, c, func() bool {
		return v.session.State().Playback.State == playback.Paused && v.session.State().Playback.Position == 7*time.Second
	})
	c.RequestClose(false)
	waitCore(t, c, func() bool { return c.closed })
	if err := backend.cleanup(); err != nil {
		t.Fatal(err)
	}
	second, _, secondEngines := syntheticCore(t, "history", directory)
	waitCore(t, second, func() bool { return second.history.State().Phase == history.Ready })
	rows := second.history.State().Records
	if len(rows) != 1 || rows[0].Ref.Part != "101" || rows[0].Position != 7*time.Second {
		t.Fatal("saved history did not restore")
	}
	if err := second.Continue(rows[0]); err != nil {
		t.Fatal(err)
	}
	waitCore(t, second, func() bool {
		return second.group.video != nil && second.group.video.session.State().Playback.State == playback.Playing && !second.group.video.session.State().Resuming
	})
	s := second.group.video.session.State()
	if s.PlaybackPart != "101" || s.Playback.Position != 7*time.Second || len(secondEngines.engines) != 1 || second.config.metrics.source.snapshot().calls != 2 {
		t.Fatal("Continue did not freshly resolve and confirm exact saved part/time")
	}
}

func TestSyntheticClientModeCannotSelectLiveAccountOrUnboundedInput(t *testing.T) {
	t.Setenv("MY_BILIBILI_E2E_CLIENT_SCENARIO", "search")
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", "delayed")
	if _, _, err := clientBuildMode(); err != catalog.Invalid {
		t.Fatal("mixed scenarios accepted")
	}
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", "")
	if e2e, scene, err := clientBuildMode(); err != nil || !e2e || scene != "search" {
		t.Fatal("fixed scene unavailable")
	}
	t.Setenv("MY_BILIBILI_E2E_CLIENT_SCENARIO", "real")
	if _, _, err := clientBuildMode(); err != catalog.Invalid {
		t.Fatal("unknown scene selected a backend")
	}
	if _, err := composeClient("", clientOptions{scenario: "search", verifyPath: filepath.Join(t.TempDir(), "plan.json")}); err != catalog.Invalid {
		t.Fatal("e2e admitted real verification")
	}
	t.Setenv("MY_BILIBILI_E2E_HISTORY_DIR", "")
	if _, err := composeClient("", clientOptions{scenario: "search"}); err != catalog.Invalid {
		t.Fatal("e2e defaulted to real user history")
	}
	gate := make(chan struct{})
	close(gate)
	routes, err := newSyntheticClientRoutes("search", gate)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"https://passport.bilibili.com/x/passport-login/web/qrcode/generate", "https://outside.example.test/", "https://api.bilibili.com/unassigned"} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		if response, err := routes.RoundTrip(req); err == nil || response != nil {
			t.Fatal("fixed fake transport forwarded unknown request")
		}
	}
}

func TestSyntheticSourceCannotReplaceAnUnacceptedResolution(t *testing.T) {
	source := &syntheticNativeSource{Source: &coreSource{id: "original"}, fixture: &coreSource{id: "fixture"}}
	if _, err := source.Resolve(context.Background(), app.ResolveRequest{Part: "unassigned"}); err == nil {
		t.Fatal("unknown part selected a fixture file")
	}
}
