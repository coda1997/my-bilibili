package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func (c *client) clientStart(ctx context.Context, historyReady bool) (clientObserved, error) {
	return c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.AccountPhase == account.SignedIn && o.AccountError == account.None && o.Authenticated &&
			o.Stored && o.Restored && o.AccountEpoch > 0 && o.ScopeRevision > 0 &&
			(!historyReady || o.HistoryPhase == history.Ready && !o.HistoryBusy && o.HistoryError == history.None)
	})
}

func (c *client) clientOpen(ctx context.Context, entry string, part int) (clientObserved, error) {
	before, err := c.clientSnapshot(ctx)
	if err != nil {
		return before, err
	}
	if err = c.clientAction(ctx, "client-catalog"); err != nil {
		return before, err
	}
	if err = c.clientText(ctx, "client-open-text", entry); err != nil {
		return before, err
	}
	if err = c.clientAction(ctx, "client-open-submit"); err != nil {
		return before, err
	}
	return c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return clientReady(o) && o.Navigation > before.Navigation && o.Visit > before.Visit &&
			o.Parts == 2 && o.PlaybackPart == part && o.SelectedPart == part &&
			o.Qualities == 2 && o.PlaybackQuality == 1 && o.SelectedQuality == 1 &&
			!o.Resolving && !o.DesiredPlaying && o.SourceActive == 0 && o.SourceCalls >= before.SourceCalls+2
	})
}

func clientPlaying(o clientObserved) bool {
	paused := o
	paused.Playback.State = playback.Paused
	return o.Playback.State == playback.Playing && o.DesiredPlaying && clientReady(paused)
}

func (c *client) clientProgress(ctx context.Context, before clientObserved) (clientObserved, error) {
	return c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool {
		return clientProgressed(before, o)
	})
}

func clientProgressed(before, after clientObserved) bool {
	return clientPlaying(after) && after.Navigation == before.Navigation && after.Playback.Generation == before.Playback.Generation &&
		after.Playback.Position >= before.Playback.Position+300*time.Millisecond
}

func (c *client) clientPlay(ctx context.Context) (clientObserved, error) {
	before, err := c.clientUntil(ctx, 10*time.Second, clientReady)
	if err != nil {
		return before, err
	}
	if err = c.clientAction(ctx, "play-toggle"); err != nil {
		return before, err
	}
	return c.clientProgress(ctx, before)
}

func (c *client) clientPause(ctx context.Context) (clientObserved, error) {
	o, err := c.clientSnapshot(ctx)
	if err != nil {
		return o, err
	}
	if o.Playback.State == playback.Playing {
		if err = c.clientAction(ctx, "play-toggle"); err != nil {
			return o, err
		}
	}
	return c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool { return clientReady(o) && !o.DesiredPlaying })
}

func (c *client) clientSeek(ctx context.Context, position time.Duration) (clientObserved, error) {
	before, err := c.clientPause(ctx)
	if err != nil {
		return before, err
	}
	if err = c.slider(ctx, "seek-slider", fmt.Sprint(position.Seconds())); err != nil {
		return before, err
	}
	settled := func(o clientObserved) bool {
		return clientReady(o) && !o.DesiredPlaying && o.Navigation == before.Navigation &&
			o.Playback.Generation == before.Playback.Generation && absDuration(o.Playback.Position-position) <= 100*time.Millisecond
	}
	o, err := c.clientUntil(ctx, 3*time.Second, settled)
	if err != nil {
		return o, err
	}
	if err = wait(ctx, 100*time.Millisecond); err != nil {
		return o, err
	}
	o, err = c.clientSnapshot(ctx)
	if err != nil {
		return o, err
	}
	if !settled(o) {
		return o, errors.New("client native seek did not remain settled")
	}
	return o, nil
}

func (c *client) clientQuality(ctx context.Context, before clientObserved) (clientObserved, error) {
	if err := c.clientPopup(ctx, "quality-menu", "quality-1"); err != nil {
		return before, err
	}
	return c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return clientReady(o) && !o.DesiredPlaying && !o.Resolving && o.Navigation == before.Navigation &&
			o.Visit == before.Visit && o.PlaybackPart == before.PlaybackPart && o.Playback.Generation > before.Playback.Generation &&
			o.PlaybackQuality == 2 && o.SelectedQuality == 2 && o.SourceCalls == before.SourceCalls+1 && o.SourceActive == 0 &&
			o.ResumeError == catalog.None && absDuration(o.Playback.Position-before.Playback.Position) <= 250*time.Millisecond
	})
}

func (c *client) clientPartTwo(ctx context.Context, before clientObserved) (clientObserved, error) {
	if err := c.clientPopup(ctx, "parts-menu", "part-102"); err != nil {
		return before, err
	}
	return c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return clientPlaying(o) && o.Navigation == before.Navigation && o.Visit > before.Visit && o.PlaybackPart == 2 &&
			o.SelectedPart == 2 && o.Playback.Generation > before.Playback.Generation && o.Playback.Position < 2*time.Second
	})
}

func (c *client) clientTextState(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := c.clientControl(ctx, key); err != nil {
		return err
	}
	nodes, err := c.tree(ctx)
	if err != nil {
		return err
	}
	n, err := uniqueNode(nodes, key)
	if err != nil || strings.TrimSpace(n.Label) == "" {
		return errors.New("client loading, empty or error UI unavailable")
	}
	return nil
}

func (c *client) clientSearchScene(ctx context.Context, _ *ownedProcess, dir string, r *report) error {
	before, err := c.clientStart(ctx, true)
	if err != nil {
		return err
	}
	if err = c.act(ctx, "e2e-narrow"); err != nil {
		return err
	}
	if err = c.clientText(ctx, "client-search-text", "测试视频"); err != nil {
		return err
	}
	typed, err := c.clientSnapshot(ctx)
	if err != nil {
		return err
	}
	if typed.SearchCalls != before.SearchCalls {
		return errors.New("typing triggered a search request")
	}
	if err = c.clientAction(ctx, "client-search-submit"); err != nil {
		return err
	}
	o, err := c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.SearchPhase == catalog.SearchReady && o.SearchError == catalog.None && o.Results == 2 && o.Page == 1 &&
			o.SearchCalls == before.SearchCalls+1 && o.SearchActive == 0 && o.CoverRequests > 0 && o.CoverRetained > 0 && o.CoverActive == 0
	})
	if err != nil {
		return err
	}
	w, err := c.identify(ctx)
	if err != nil || w.ContentBounds.Width != 800 || w.ContentBounds.Height != 600 {
		return errors.New("client minimum-size window was not observed")
	}
	r.passClient("client-search-explicit-page-and-cover", "actual application text input and submit; two results/page1 with bounded original cover retention at800x600", o)
	if err = c.clientAction(ctx, "client-next-page"); err != nil {
		return err
	}
	o, err = c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.SearchError == catalog.ServiceUnavailable && o.Results == 2 && o.Page == 1 && !o.LoadingMore &&
			o.SearchCalls == before.SearchCalls+2 && o.SearchActive == 0
	})
	if err != nil {
		return err
	}
	if err = c.clientTextState(ctx, "client-search-error"); err != nil {
		return err
	}
	r.passClient("client-append-failure-preserves-page", "fixed HTTP503 leaves accepted two rows/page1 with explicit error and Retry", o)
	if err = c.clientAction(ctx, "client-retry-search"); err != nil {
		return err
	}
	o, err = c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.SearchPhase == catalog.SearchReady && o.SearchError == catalog.None && o.Results == 3 && o.Page == 2 &&
			o.SearchCalls == before.SearchCalls+3 && o.SearchActive == 0 && !o.LoadingMore
	})
	if err != nil {
		return err
	}
	r.passClient("client-append-retry-deduplicates", "explicit failed-page retry retains first-seen rows and deduplicates the overlapping second page", o)
	if err = c.clientText(ctx, "client-search-text", "空结果"); err != nil {
		return err
	}
	if err = c.clientAction(ctx, "client-search-submit"); err != nil {
		return err
	}
	o, err = c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.SearchPhase == catalog.SearchReady && o.SearchError == catalog.None && o.Results == 0 && o.Page == 1 && o.SearchCalls == before.SearchCalls+4 && o.SearchActive == 0
	})
	if err != nil {
		return err
	}
	if err = c.clientTextState(ctx, "client-search-state"); err != nil {
		return err
	}
	r.passClient("client-empty-search", "valid empty first page remains distinct from the prior service failure", o)
	o, err = c.clientOpen(ctx, "https://www.bilibili.com/video/BV0000000001?p=2", 2)
	if err != nil {
		return err
	}
	if _, err = c.picture(ctx, dir, "client-direct-part2.png"); err != nil {
		return err
	}
	r.passClient("client-direct-multipart-link", "canonical original link selects its second part; strict accepted indices and actual synthetic picture", o)
	return nil
}

func (c *client) clientPlaybackScene(ctx context.Context, _ *ownedProcess, dir string, r *report) error {
	if _, err := c.clientStart(ctx, true); err != nil {
		return err
	}
	o, err := c.clientOpen(ctx, "BV0000000001", 1)
	if err != nil {
		return err
	}
	first, err := c.picture(ctx, dir, "client-play-paused.png")
	if err != nil {
		return err
	}
	o, err = c.clientPlay(ctx)
	if err != nil {
		return err
	}
	next, err := c.picture(ctx, dir, "client-play-advancing.png")
	if err != nil {
		return err
	}
	if first == next {
		return errors.New("client synthetic video pixels did not advance")
	}
	r.passClient("client-native-play-picture-progress", "selected A/V, current-engine picture bit, actual synthetic bands and changing native pixels with advancing time", o)
	before, err := c.clientSeek(ctx, 5*time.Second)
	if err != nil {
		return err
	}
	o, err = c.clientQuality(ctx, before)
	if err != nil {
		return err
	}
	r.passClient("client-quality-confirmed-position", "second returned synthetic choice replaces native generation and preserves confirmed paused position and visit; no encoded-quality claim", o)
	o, err = c.clientPartTwo(ctx, o)
	if err != nil {
		return err
	}
	o, err = c.clientPause(ctx)
	if err != nil {
		return err
	}
	r.passClient("client-explicit-part-starts-at-zero", "explicit second-part intent creates a fresh visit/generation near zero, then pauses", o)
	previousNavigation := o.Navigation
	o, err = c.clientOpen(ctx, "BV0000000002", 1)
	if err != nil {
		return err
	}
	if o.Navigation <= previousNavigation {
		return errors.New("cross-video navigation did not advance")
	}
	if _, err = c.picture(ctx, dir, "client-second-video.png"); err != nil {
		return err
	}
	r.passClient("client-cross-video-native-replacement", "new navigation accepted only after outgoing ownership retirement; fresh current-engine picture", o)
	return nil
}

func (c *client) clientSegmentScene(ctx context.Context, _ *ownedProcess, _ string, r *report) error {
	if _, err := c.clientStart(ctx, true); err != nil {
		return err
	}
	if _, err := c.clientOpen(ctx, "BV0000000001", 1); err != nil {
		return err
	}
	if _, err := c.clientPlay(ctx); err != nil {
		return err
	}
	if _, err := c.clientSeek(ctx, 5*time.Second); err != nil {
		return err
	}
	current := func(o clientObserved) bool {
		return clientReady(o) && o.DanmakuPhase == danmaku.LoadReady && o.DanmakuError == catalog.None &&
			o.DanmakuGeneration == o.Playback.Generation && o.Segment == 1 && o.CachedSegments == 1 && o.CachedEntries == 5 &&
			o.CachedTextBytes > 0 && o.VisibleComments > 0 && o.DanmakuActiveRequests == 0 && o.APIActive == 0
	}
	before, err := c.clientUntil(ctx, 10*time.Second, current)
	if err != nil {
		return err
	}
	if before.Playback.Duration < 29*time.Second || before.Playback.Duration > 31*time.Second {
		return errors.New("synthetic catalog duration replaced actual native duration")
	}
	r.passClient("client-segment-current-window", "actual native5s, current segment1 and five original current-segment entries; visible prepared overlay", before)
	if _, err = c.clientSeek(ctx, 18*time.Second); err != nil {
		return err
	}
	o, err := c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return clientReady(o) && o.Segment == 1 && o.CachedSegments == 2 && o.CachedEntries == 6 && o.CachedTextBytes > before.CachedTextBytes &&
			o.DanmakuRevision > before.DanmakuRevision && o.DanmakuGeneration == before.Playback.Generation &&
			o.DanmakuActiveRequests == 0 && o.DanmakuError == catalog.None && o.APIActive == 0 && o.APIRequests == before.APIRequests+1 && o.VisibleComments <= 1
	})
	if err != nil {
		return err
	}
	r.passClient("client-segment-next-prefetch", "actual native18s remains segment1; next segment accepted into second slot, with no future361s text shown", o)
	prefetch := o
	if _, err = c.clientSeek(ctx, 5*time.Second); err != nil {
		return err
	}
	o, err = c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool {
		return current(o) && o.CachedTextBytes == before.CachedTextBytes && o.APIRequests == prefetch.APIRequests && o.DanmakuRevision > prefetch.DanmakuRevision
	})
	if err != nil {
		return err
	}
	r.passClient("client-segment-seek-evicts-next", "native5s restored current comments and evicted second slot without another HTTP call", o)
	before = o
	o, err = c.clientQuality(ctx, before)
	if err != nil {
		return err
	}
	o, err = c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool {
		return current(o) && o.Playback.Generation > before.Playback.Generation && o.CachedTextBytes == before.CachedTextBytes && o.APIRequests == before.APIRequests+1
	})
	if err != nil {
		return err
	}
	r.passClient("client-quality-reuses-current-segment", "same-part quality changes native presentation generation while retaining accepted segment cache", o)
	r.Checks = append(r.Checks, check{Scenario: r.Scenario, Name: "native-current-segment-1-to-2", Result: "not-tested", Detail: "existing native fixture is30s; this scene proves5s/18s/5s cache-window epochs, while controlled decoder/loader tests cover absolute times beyond360s"})
	return nil
}

func clientHistorySaved(o clientObserved, count int) bool {
	return o.HistoryPhase == history.Ready && o.HistoryError == history.None && o.HistoryCount == count &&
		!o.HistoryBusy && !o.HistoryDirty && o.HistoryRevision == o.HistorySavedRevision
}

func clientRetired(o clientObserved) bool {
	return !o.Authenticated && o.AccountEpoch == 0 && o.PlaybackPart == 0 &&
		(o.Playback.State == playback.Idle || o.Playback.State == playback.Closed) &&
		o.Resources == (mpv.Resources{}) && !o.VideoTrack && !o.AudioTrack && !o.PicturePresented &&
		o.SourceActive == 0 && o.SearchActive == 0 && o.APIActive == 0 && o.CoverActive == 0 && o.CoverRetained == 0 &&
		o.CachedSegments == 0 && o.CachedEntries == 0 && o.CachedTextBytes == 0 &&
		o.DanmakuActiveRequests == 0 && o.VisibleComments == 0 && o.TextLayouts == 0 &&
		!o.Resolving && !o.Resuming && !o.DesiredPlaying
}

func (c *client) clientAccountScene(ctx context.Context, _ *ownedProcess, expiry bool, r *report) error {
	if _, err := c.clientStart(ctx, true); err != nil {
		return err
	}
	if _, err := c.clientOpen(ctx, "BV0000000001", 1); err != nil {
		return err
	}
	if _, err := c.clientPlay(ctx); err != nil {
		return err
	}
	before, err := c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool { return o.HistoryCount > 0 })
	if err != nil {
		return err
	}
	want := account.Expired
	if expiry {
		err = c.act(ctx, "e2e-expire-account")
	} else {
		want = account.SignedOut
		if err = c.clientAction(ctx, "client-account"); err != nil {
			return err
		}
		if err = c.clientAction(ctx, "logout"); err != nil {
			return err
		}
		err = c.clientAction(ctx, "confirm-logout")
	}
	if err != nil {
		return err
	}
	o, err := c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool {
		return o.AccountPhase == want && o.ScopeRevision > before.ScopeRevision && clientRetired(o) && o.HistoryCount >= before.HistoryCount
	})
	if err != nil {
		return err
	}
	if err = wait(ctx, 200*time.Millisecond); err != nil {
		return err
	}
	still, err := c.clientSnapshot(ctx)
	if err != nil {
		return err
	}
	if !clientRetired(still) || still.SourceCalls != o.SourceCalls || still.SearchCalls != o.SearchCalls {
		return errors.New("retired account implicitly reopened content")
	}
	r.passClient("client-account-retires-current-content", "synthetic account transition changes scope and retires native/cache/overlay ownership without anonymous reopen; local history retained", still)
	return nil
}

func (c *client) clientHistoryRecoveryScene(ctx context.Context, _ *ownedProcess, inUse bool, r *report) error {
	if _, err := c.clientStart(ctx, false); err != nil {
		return err
	}
	if err := c.clientAction(ctx, "client-history"); err != nil {
		return err
	}
	want := history.Corrupt
	if inUse {
		want = history.InUse
	}
	o, err := c.clientUntil(ctx, 3*time.Second, func(o clientObserved) bool {
		return o.HistoryPhase == history.Failed && o.HistoryError == want && !o.HistoryBusy
	})
	if err != nil {
		return err
	}
	if err = c.clientTextState(ctx, "history-error"); err != nil {
		return err
	}
	r.passClient("client-history-recovery-visible", "isolated synthetic history failure is visible and not reported as an empty successful load", o)
	if inUse {
		if err = c.act(ctx, "e2e-release-history"); err != nil {
			return err
		}
		if err = c.clientAction(ctx, "history-retry"); err != nil {
			return err
		}
	} else {
		if err = c.clientConfirm(ctx, "history-reset", "history-reset-confirmation", true); err != nil {
			return err
		}
	}
	o, err = c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool { return clientHistorySaved(o, 0) })
	if err != nil {
		return err
	}
	r.passClient("client-history-explicit-recovery", "explicit fresh acquisition Retry or scoped Reset confirmation restores usable isolated history", o)
	return nil
}

func (c *client) clientHeldCloseScene(ctx context.Context, p *ownedProcess, r *report) error {
	if _, err := c.clientStart(ctx, false); err != nil {
		return err
	}
	if err := c.clientText(ctx, "client-search-text", "hold"); err != nil {
		return err
	}
	if err := c.clientAction(ctx, "client-search-submit"); err != nil {
		return err
	}
	before, err := c.clientUntil(ctx, 10*time.Second, func(o clientObserved) bool { return o.HistoryBusy && o.SearchActive == 1 && o.APIActive >= 1 })
	if err != nil {
		return err
	}
	r.passClient("client-held-http-store-before-close", "HTTP and isolated Store.Load are both physically pending before native minimize", before)
	p.closeRequested = true
	if err = c.act(ctx, "e2e-minimized-close"); err != nil {
		return err
	}
	// Do not issue another RPC, tree or capture: cleanup must not require paint.
	if !p.await(5*time.Second) || p.exit != nil {
		return errors.New("minimized client did not finish owned close")
	}
	minimized, err := p.clientReceipts.get(previewbundle.ClientWindowMinimized)
	if err != nil {
		return err
	}
	if !minimized.Minimized || !minimized.Closing || minimized.PendingOwners == 0 || !minimized.HistoryBusy {
		return errors.New("native minimize did not retain actual held ownership")
	}
	if _, err := clientClosed(p); err != nil {
		return err
	}
	r.passClient("client-minimized-pending-close", "native minimized receipt with pending owners precedes strict joined close and graceful exit; no post-minimize RPC", minimized)
	return nil
}

func runClient(ctx context.Context, executable, dir string, r *report) error {
	scratch, err := os.MkdirTemp("", "p0-client-history-")
	if err != nil {
		return errors.New("private client history scratch unavailable")
	}
	defer os.RemoveAll(scratch)
	scenes := []struct {
		name     string
		exercise func(context.Context, *client, *ownedProcess) error
	}{
		{"search", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientSearchScene(ctx, p, dir, r)
		}},
		{"playback", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientPlaybackScene(ctx, p, dir, r)
		}},
		{"segments", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientSegmentScene(ctx, p, dir, r)
		}},
		{"account-expiry", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientAccountScene(ctx, p, true, r)
		}},
		{"account-signout", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientAccountScene(ctx, p, false, r)
		}},
		{"history-corrupt", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientHistoryRecoveryScene(ctx, p, false, r)
		}},
		{"history-in-use", func(ctx context.Context, c *client, p *ownedProcess) error {
			return c.clientHistoryRecoveryScene(ctx, p, true, r)
		}},
		{"close-held", func(ctx context.Context, c *client, p *ownedProcess) error { return c.clientHeldCloseScene(ctx, p, r) }},
	}
	for _, scene := range scenes {
		historyDir := filepath.Join(scratch, scene.name)
		if err := os.Mkdir(historyDir, 0700); err != nil {
			return errors.New("isolated client history directory unavailable")
		}
		if err := scenarioInvocation(ctx, executable, scene.name, "", scene.name, historyDir, dir, r, scene.exercise); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	historyDir := filepath.Join(scratch, "restart")
	if err := os.Mkdir(historyDir, 0700); err != nil {
		return errors.New("isolated restart directory unavailable")
	}
	return runClientHistory(ctx, executable, historyDir, dir, r)
}
