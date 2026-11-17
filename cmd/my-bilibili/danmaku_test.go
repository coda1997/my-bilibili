package main

import (
	"context"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/danmaku"
	danmakuview "my-bilibili/internal/desktop/danmaku"
	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
)

func renderBoundDanmaku(a *application, state app.State) danmakuview.State {
	a.bindDanmaku(state)
	gui.NewTester(func(c *gui.Context) {
		a.danmaku.Render(c, state.Playback, 640, 360)
	}, 640, 360)
	return a.danmaku.State()
}

func requireFixturePhrase(t *testing.T, state danmakuview.State, phrase string) {
	t.Helper()
	width, _ := gui.ShapeText(phrase, gui.Font{Size: float32(state.Settings.FontSize), Weight: 600, Lang: "zh-Hans", LineHeight: 1.25}, 0).Size()
	for _, item := range state.Items {
		if item.Index == 0 && item.Mode == danmaku.FixedTop && item.Width == float64(width) {
			return
		}
	}
	t.Fatalf("accepted fixture phrase was not shaped: %+v", state)
}

func TestDanmakuBindsAcceptedPartAndRebindsSamePartGeneration(t *testing.T) {
	a := &application{danmaku: danmakuview.New()}
	defer a.danmaku.Close()
	settings := danmaku.DefaultSettings()
	settings.Opacity, settings.FontSize, settings.Speed = .55, 29, 1.2
	if err := a.danmaku.Configure(settings); err != nil {
		t.Fatal(err)
	}
	state := app.State{SelectedPart: "part-2", PlaybackPart: "part-1", ResolvingPart: "part-2", Playback: playback.Snapshot{Generation: 1, State: playback.Paused, Position: 2 * time.Second}}
	first := renderBoundDanmaku(a, state)
	requireFixturePhrase(t, first, "中文弹幕测试")
	if first.Stats.Input != 64 || first.Settings != settings || first.Error != danmaku.NoError {
		t.Fatal("fixture data or chosen settings missing", first)
	}
	if again := renderBoundDanmaku(a, state); !reflect.DeepEqual(first, again) {
		t.Fatal("a rebuild reinstalled or accumulated the accepted window")
	}
	// Playback replay can change generation without changing the accepted part.
	state.Playback.Generation++
	replay := renderBoundDanmaku(a, state)
	if replay.Generation != 2 || replay.Revision <= first.Revision || replay.Stats.Input != 64 || replay.Settings != settings {
		t.Fatal("same-part native generation did not receive a fresh window", replay)
	}
	requireFixturePhrase(t, replay, "中文弹幕测试")
	state.Playback.Generation++
	state.PlaybackPart, state.ResolvingPart = "part-2", ""
	second := renderBoundDanmaku(a, state)
	requireFixturePhrase(t, second, "第二分P弹幕")
	if second.Generation != 3 || second.Settings != settings || second.Stats.Input != 64 {
		t.Fatal("part replacement lost settings or accumulated comments", second)
	}
}

func TestDanmakuBindingHidesMismatchAndPreservesSafeErrors(t *testing.T) {
	a := &application{danmaku: danmakuview.New()}
	state := app.State{PlaybackPart: "part-1", Playback: playback.Snapshot{Generation: 1, State: playback.Paused, Position: 2 * time.Second}}
	first := renderBoundDanmaku(a, state)
	if first.Active == 0 {
		t.Fatal("initial fixture is absent")
	}
	// A Tick after binding may accept a new attempt. The outgoing frame cannot
	// display A's comments under B, even before the next composition rebuild.
	state.Playback.Generation = 2
	gui.NewTester(func(c *gui.Context) { a.danmaku.Render(c, state.Playback, 640, 360) }, 640, 360)
	if got := a.danmaku.State(); got.Active != 0 || len(got.Items) != 0 {
		t.Fatal("old comments survived a newer native generation", got)
	}
	state.PlaybackPart = "missing-fixture"
	missing := renderBoundDanmaku(a, state)
	if missing.Error != danmaku.Unavailable || missing.Active != 0 || missing.Stats.Input != 0 {
		t.Fatal("lookup failure retained old data or was hidden", missing)
	}
	state.Playback.Generation, state.PlaybackPart = 1, "part-1"
	stale := renderBoundDanmaku(a, state)
	if stale.Generation != 2 || stale.Error != danmaku.Unavailable || stale.Stats.Input != 0 {
		t.Fatal("stale composition work replaced current safe state", stale)
	}
	state.Playback.Generation, state.PlaybackPart = 3, "part-2"
	recovered := renderBoundDanmaku(a, state)
	requireFixturePhrase(t, recovered, "第二分P弹幕")
	if recovered.Error != danmaku.NoError {
		t.Fatal("new accepted fixture failed to clear the old lookup error")
	}
	state.Playback.State = playback.Closed
	a.bindDanmaku(state) // Deliberately no subsequent paint.
	if got := a.danmaku.State(); !got.Closed || got.Active != 0 || got.Layouts != 0 || got.Stats.Input != 0 {
		t.Fatal("closed binding requires a future paint to release ownership", got)
	}
	state.Playback.Generation++
	state.Playback.State = playback.Paused
	a.bindDanmaku(state)
	if !a.danmaku.State().Closed {
		t.Fatal("late binding reopened a closed overlay")
	}
}

type danmakuTestSource struct {
	app.Source
	partB chan error
}

func (s *danmakuTestSource) Resolve(ctx context.Context, id catalog.PartID) (app.Resolution, error) {
	if id == "part-2" {
		select {
		case err := <-s.partB:
			if err != nil {
				return app.Resolution{}, err
			}
		case <-ctx.Done():
			return app.Resolution{}, ctx.Err()
		}
	}
	return s.Source.Resolve(ctx, id)
}

type danmakuTestEngine struct{ state playback.Snapshot }

func (e *danmakuTestEngine) Snapshot() playback.Snapshot { return e.state }
func (e *danmakuTestEngine) Close() error                { return nil }
func (e *danmakuTestEngine) Command(c playback.Command) error {
	switch c.Kind {
	case playback.Play:
		e.state.State = playback.Playing
	case playback.Pause:
		e.state.State = playback.Paused
	case playback.Seek:
		e.state.Position = c.Position
	case playback.Rate:
		e.state.Rate = c.Value
	case playback.Volume:
		e.state.Volume = c.Value
	}
	return nil
}

type danmakuTestOpener struct{}

func (danmakuTestOpener) Open(context.Context, playback.Input) (playback.Engine, error) {
	return &danmakuTestEngine{state: playback.Snapshot{State: playback.Ready, Duration: 30 * time.Second, Rate: 1, Volume: 1}}, nil
}

func TestDanmakuActualSessionKeepsAWhileBResolvesAndFails(t *testing.T) {
	t.Run("accepted replacement", func(t *testing.T) { danmakuSessionScenario(t, false) })
	t.Run("close while source pending", func(t *testing.T) { danmakuSessionScenario(t, true) })
}

func danmakuSessionScenario(t *testing.T, closePending bool) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		base, err := fixture.New("../../internal/fixture/assets")
		if err != nil {
			t.Fatal(err)
		}
		source := &danmakuTestSource{Source: base, partB: make(chan error, 1)}
		session, err := app.New(source, playback.New(danmakuTestOpener{}, nil))
		if err != nil {
			t.Fatal(err)
		}
		a := &application{session: session, danmaku: danmakuview.New()}
		defer func() {
			if err := a.dispose(); err != nil {
				t.Error(err)
			}
			<-session.Done()
		}()
		if err := session.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		pump := func(want func(app.State) bool) app.State {
			t.Helper()
			for i := 0; i < 20; i++ {
				synctest.Wait()
				time.Sleep(playback.PollInterval)
				session.Tick()
				if state := session.State(); want(state) {
					return state
				}
			}
			t.Fatal("controlled source did not settle", session.State())
			return app.State{}
		}
		state := pump(func(s app.State) bool {
			return s.Playback.State == playback.Ready || s.Playback.State == playback.Paused
		})
		first := renderBoundDanmaku(a, state)
		requireFixturePhrase(t, first, "中文弹幕测试")
		if err := session.SelectPart("part-2"); err != nil {
			t.Fatal(err)
		}
		pending := pump(func(s app.State) bool { return s.ResolvingPart == "part-2" })
		if pending.SelectedPart != "part-2" || pending.PlaybackPart != "part-1" {
			t.Fatal("test did not establish unresolved selection", pending)
		}
		if got := renderBoundDanmaku(a, pending); !reflect.DeepEqual(got, first) {
			t.Fatal("unresolved B changed accepted A comments", got)
		}
		if closePending {
			if err := a.dispose(); err != nil {
				t.Fatal(err)
			}
			closed := a.danmaku.State()
			if !closed.Closed || closed.Active != 0 || closed.Layouts != 0 || closed.Stats.Input != 0 {
				t.Fatal("pending-source disposal retained overlay ownership without paint", closed)
			}
			<-session.Done()
			session.Tick()
			a.bindDanmaku(session.State())
			if got := a.danmaku.State(); !reflect.DeepEqual(got, closed) {
				t.Fatal("late source completion changed the disposed overlay", got)
			}
			return
		}
		source.partB <- playback.Media
		failed := pump(func(s app.State) bool { return s.SelectionError == playback.Media })
		if got := renderBoundDanmaku(a, failed); !reflect.DeepEqual(got, first) {
			t.Fatal("failed B changed accepted A comments", got)
		}
		if err := session.RetrySelection(); err != nil {
			t.Fatal(err)
		}
		source.partB <- nil
		accepted := pump(func(s app.State) bool { return s.PlaybackPart == "part-2" && s.Playback.State == playback.Playing })
		requireFixturePhrase(t, renderBoundDanmaku(a, accepted), "第二分P弹幕")
		if accepted.Playback.Generation <= first.Generation {
			t.Fatal("accepted B did not establish a fresh native attempt")
		}
		if err := a.dispose(); err != nil {
			t.Fatal(err)
		}
		if got := a.danmaku.State(); !got.Closed || got.Active != 0 || got.Layouts != 0 || got.Stats.Input != 0 {
			t.Fatal("explicit dispose retained overlay ownership without repaint", got)
		}
	})
}
