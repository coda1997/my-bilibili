//go:build e2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/danmaku"
	danmakuview "my-bilibili/internal/desktop/danmaku"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func TestDanmakuObservationCopiesThePreparedFrameAndExcludesText(t *testing.T) {
	view := danmakuview.New()
	defer view.Close()
	if err := view.Reset(1); err != nil {
		t.Fatal(err)
	}
	if err := view.SetEntries(1, []danmaku.Entry{{Text: "do-not-export-comment-sentinel", Mode: danmaku.FixedTop}}); err != nil {
		t.Fatal(err)
	}
	snapshot := playback.Snapshot{Generation: 1, State: playback.Paused, Position: 2 * time.Second, Duration: 30 * time.Second, Rate: 1, Volume: 1}
	gui.NewTester(func(c *gui.Context) { view.Render(c, snapshot, 640, 360) }, 640, 360)
	state := view.State()
	got := observeDanmaku(state)
	wantJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil || !bytes.Equal(gotJSON, wantJSON) || bytes.Contains(gotJSON, []byte("do-not-export")) {
		t.Fatal("prepared observation lost safe fields or disclosed text", err)
	}
	if got.Active != 1 || got.Layouts != 1 || len(got.Items) != 1 {
		t.Fatal("test did not prepare an active layout", got)
	}
	state.Items[0].X = -9999
	if got.Items[0].X == -9999 {
		t.Fatal("evidence retained the supplied item slice")
	}
	got.Items[0].X = -8888
	if view.State().Items[0].X == -8888 {
		t.Fatal("evidence retained the renderer's item slice")
	}

	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	video, err := source.Detail(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o := previewbundle.Observation{State: app.State{DetailPhase: app.DetailReady, Video: video, SelectedPart: "part-1", PlaybackPart: "part-1", Playback: snapshot, Rate: 1, Volume: 1}, Danmaku: observeDanmaku(view.State())}
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := previewbundle.ParseObservation(raw); err != nil || !reflect.DeepEqual(parsed.Danmaku, o.Danmaku) {
		t.Fatal("actual prepared observation rejected by strict consumer", err)
	}
	view.Close()
	o.Danmaku = observeDanmaku(view.State())
	o.State.Playback.State = playback.Closed
	if o.Danmaku.Items == nil || !o.Danmaku.Closed || o.Danmaku.Layouts != 0 || o.Danmaku.Active != 0 {
		t.Fatal("closed observation is not complete", o.Danmaku)
	}
	raw, _ = json.Marshal(o)
	if _, err := previewbundle.ParseObservation(raw); err != nil {
		t.Fatal("actual closed observation rejected", err)
	}
}

func TestDanmakuObservationDoesNotCoerceLeaksToZero(t *testing.T) {
	state := danmakuview.State{Closed: true, Active: 2, Layouts: 3, Stats: danmaku.Stats{Input: 4, Loaded: 4}, Items: []danmakuview.ItemState{{Index: 1}, {Index: 2}}, Error: danmaku.Unavailable}
	got := observeDanmaku(state)
	if !got.Closed || got.Active != 2 || got.Layouts != 3 || got.Stats != state.Stats || len(got.Items) != 2 || got.Error != danmaku.Unavailable {
		t.Fatal("observation hid positive ownership or the original safe error", got)
	}
}

func TestApplicationObservationIncludesDanmakuBeforeDetailAndAfterClose(t *testing.T) {
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	session, err := app.New(source, playback.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	a := &application{session: session, danmaku: danmakuview.New()}
	defer a.dispose()
	for _, closed := range []bool{false, true} {
		if closed {
			if err := a.dispose(); err != nil {
				t.Fatal(err)
			}
		}
		got := a.automationObservation(false, false)
		if got.Danmaku.Items == nil || got.Danmaku.Settings != danmaku.DefaultSettings() || got.Danmaku.Closed != closed {
			t.Fatal("application did not publish the actual complete overlay state", got.Danmaku)
		}
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := previewbundle.ParseObservation(raw); err != nil {
			t.Fatal("application lifecycle observation is not parseable", err)
		}
	}
}
