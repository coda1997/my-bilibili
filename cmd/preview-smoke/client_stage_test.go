package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"my-bilibili/internal/account"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
)

func TestClientProgressCannotUseAnotherGenerationNavigationOrMissingPaint(t *testing.T) {
	before := completeClientObservation()
	after := before
	after.Playback.State, after.DesiredPlaying = playback.Playing, true
	after.Playback.Position += 300 * time.Millisecond
	if !clientProgressed(before, after) {
		t.Fatal("complete progress rejected")
	}
	for _, change := range []func(*clientObserved){
		func(o *clientObserved) { o.Navigation++ },
		func(o *clientObserved) { o.Playback.Generation++ },
		func(o *clientObserved) { o.Playback.Position-- },
		func(o *clientObserved) { o.PicturePresented = false },
		func(o *clientObserved) { o.DesiredPlaying = false },
		func(o *clientObserved) { o.Resuming = true },
		func(o *clientObserved) { o.Playback.State = playback.Paused },
	} {
		o := after
		change(&o)
		if clientProgressed(before, o) {
			t.Fatal("stale, insufficient or no-picture progress accepted")
		}
	}
}

func TestClientHistoryEvidenceNeedsTheCurrentSuccessfulSave(t *testing.T) {
	o := completeClientObservation()
	o.HistoryCount, o.HistoryRevision, o.HistorySavedRevision = 1, 3, 3
	if !clientHistorySaved(o, 1) {
		t.Fatal("complete saved state rejected")
	}
	for _, change := range []func(*clientObserved){
		func(o *clientObserved) { o.HistoryCount = 0 },
		func(o *clientObserved) { o.HistorySavedRevision-- },
		func(o *clientObserved) { o.HistoryBusy = true },
		func(o *clientObserved) { o.HistoryDirty = true },
		func(o *clientObserved) { o.HistoryError = history.Uncertain },
		func(o *clientObserved) { o.HistoryPhase = history.Failed },
	} {
		bad := o
		change(&bad)
		if clientHistorySaved(bad, 1) {
			t.Fatal("unconfirmed history became restart evidence")
		}
	}
}

func TestClientRevocationRetainsPositiveOwnershipAsFailure(t *testing.T) {
	o := closedClientSample()
	o.Closing, o.Closed = false, false
	o.AccountPhase = account.Expired
	o.PlaybackPart, o.SelectedPart, o.Parts, o.Qualities, o.SelectedQuality, o.PlaybackQuality = 0, 0, 0, 0, 0, 0
	o.Playback.State = playback.Idle
	if !clientRetired(o) {
		t.Fatal("complete retirement rejected")
	}
	for name, change := range map[string]func(*clientObserved){
		"auth":        func(o *clientObserved) { o.Authenticated = true },
		"epoch":       func(o *clientObserved) { o.AccountEpoch = 1 },
		"part":        func(o *clientObserved) { o.PlaybackPart = 1 },
		"state":       func(o *clientObserved) { o.Playback.State = playback.Playing },
		"core":        func(o *clientObserved) { o.Resources.Cores = 1 },
		"renderer":    func(o *clientObserved) { o.Resources.Renderers = 1 },
		"worker":      func(o *clientObserved) { o.Resources.Workers = 1 },
		"video":       func(o *clientObserved) { o.VideoTrack = true },
		"audio":       func(o *clientObserved) { o.AudioTrack = true },
		"picture":     func(o *clientObserved) { o.PicturePresented = true },
		"source":      func(o *clientObserved) { o.SourceActive = 1 },
		"search":      func(o *clientObserved) { o.SearchActive = 1 },
		"api":         func(o *clientObserved) { o.APIActive = 1 },
		"cover":       func(o *clientObserved) { o.CoverActive = 1 },
		"cover-cache": func(o *clientObserved) { o.CoverRetained = 1 },
		"segments":    func(o *clientObserved) { o.CachedSegments = 1 },
		"entries":     func(o *clientObserved) { o.CachedEntries = 1 },
		"text":        func(o *clientObserved) { o.CachedTextBytes = 1 },
		"danmaku":     func(o *clientObserved) { o.DanmakuActiveRequests = 1 },
		"visible":     func(o *clientObserved) { o.VisibleComments = 1 },
		"layouts":     func(o *clientObserved) { o.TextLayouts = 1 },
		"resolving":   func(o *clientObserved) { o.Resolving = true },
		"resuming":    func(o *clientObserved) { o.Resuming = true },
		"intent":      func(o *clientObserved) { o.DesiredPlaying = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := o
			change(&bad)
			if clientRetired(bad) {
				t.Fatal("retired state concealed a live owner/reference")
			}
		})
	}
}

func TestClientWindowIdentityRemainsSeparateFromFixture(t *testing.T) {
	for _, mode := range []bool{false, true} {
		title := "My Bilibili Preview"
		if mode {
			title = "My Bilibili"
		}
		c := pipeClient(t, func(int, string) any {
			return windowResult{Windows: []window{{ID: 1, Title: title, Kind: "content", Visible: true}}}
		})
		c.clientMode = mode
		if _, err := c.identify(context.Background()); err != nil {
			t.Fatal("owned window not identified", err)
		}
		c.clientMode = !mode
		if _, err := c.identify(context.Background()); err == nil {
			t.Fatal("wrong application mode's window accepted")
		}
	}
}

func historyNoticeNode() map[string]any {
	n := popupNode("history-error")
	n["label"] = "另一个应用正在使用本地历史记录。"
	return n
}

// An acknowledged navigation action does not guarantee a newly rendered tree.
func TestClientTextStateWaitsForRenderedNotice(t *testing.T) {
	frames := 0
	c, trace := popupClient(t, func(request popupRequest) map[string]any {
		if request.Method != "tree" {
			t.Error("notice observation issued an action")
			return popupOK()
		}
		frames++
		if frames == 1 {
			return popupTree(popupNode("client-history"))
		}
		return popupTree(historyNoticeNode())
	})
	if err := c.clientTextState(context.Background(), "history-error"); err != nil {
		t.Fatalf("read the pre-action frame instead of awaiting the history notice: %v", err)
	}
	if trace.count("tree", "") < 2 || trace.count("act", "") != 0 {
		t.Fatal("notice did not wait for a fresh rendered tree without replaying an action")
	}
}

func TestClientTextStateRejectsInvalidNotice(t *testing.T) {
	for _, name := range []string{"empty", "whitespace", "duplicate", "missing-geometry", "malformed-label", "disappeared"} {
		t.Run(name, func(t *testing.T) {
			frames := 0
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				frames++
				n := historyNoticeNode()
				switch name {
				case "empty":
					n["label"] = ""
				case "whitespace":
					n["label"] = " \t\n"
				case "duplicate":
					return popupTree(n, historyNoticeNode())
				case "missing-geometry":
					delete(n["bounds"].(map[string]any), "width")
				case "malformed-label":
					n["label"] = map[string]any{"payload": "private sentinel"}
				case "disappeared":
					if frames > 1 {
						return popupTree(popupNode("client-history"))
					}
				}
				return popupTree(n)
			})
			err := c.clientTextState(context.Background(), "history-error")
			if err == nil || strings.Contains(err.Error(), "private sentinel") {
				t.Fatalf("invalid notice was accepted or its content leaked: %v", err)
			}
			expectedReads := 2
			if name == "duplicate" || name == "missing-geometry" {
				expectedReads = 1
			}
			if trace.count("tree", "") != expectedReads || trace.count("act", "") != 0 {
				t.Fatal("invalid notice was retried or caused an action")
			}
		})
	}
}

func TestClientTextStateHonorsCancellationAndParentDeadline(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "parent-deadline"
		if canceled {
			name = "cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			c, trace := popupClient(t, func(request popupRequest) map[string]any {
				if canceled {
					cancel()
				}
				return popupTree(popupNode("client-history"))
			})
			want := context.DeadlineExceeded
			if canceled {
				want = context.Canceled
			}
			start := time.Now()
			if err := c.clientTextState(ctx, "history-error"); !errors.Is(err, want) {
				t.Fatalf("lost parent context failure: %v", err)
			}
			if time.Since(start) > 500*time.Millisecond || trace.count("act", "") != 0 {
				t.Fatal("notice wait escaped its parent budget or replayed an action")
			}
		})
	}
}

type noticeDeadlineConn struct {
	net.Conn
	deadlines []time.Time
}

func (c *noticeDeadlineConn) SetDeadline(deadline time.Time) error {
	c.deadlines = append(c.deadlines, deadline)
	return c.Conn.SetDeadline(deadline)
}

func TestClientTextStateSharesTwoSecondBudgetWithLabelRead(t *testing.T) {
	frames := 0
	c, _ := popupClient(t, func(request popupRequest) map[string]any {
		frames++
		if frames == 1 {
			return popupTree(popupNode("client-history"))
		}
		return popupTree(historyNoticeNode())
	})
	conn := &noticeDeadlineConn{Conn: c.conn}
	c.conn = conn
	start := time.Now()
	if err := c.clientTextState(context.Background(), "history-error"); err != nil {
		t.Fatal(err)
	}
	if len(conn.deadlines) != 3 {
		t.Fatal("did not observe prior frame, rendered notice and its label")
	}
	budget := conn.deadlines[0].Sub(start)
	if budget < 1900*time.Millisecond || budget > 2100*time.Millisecond {
		t.Fatalf("notice does not own its two-second budget: %v", budget)
	}
	for _, deadline := range conn.deadlines[1:] {
		if !deadline.Equal(conn.deadlines[0]) {
			t.Fatal("readiness or label observation renewed the notice budget")
		}
	}
}
