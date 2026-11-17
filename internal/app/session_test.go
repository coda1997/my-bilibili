package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type fakeSource struct {
	video     catalog.Video
	detailErr error
	inputs    map[catalog.PartID]playback.Input
	errors    map[catalog.PartID]error
	resolved  []catalog.PartID
	expires   map[catalog.PartID]time.Time
	contexts  []context.Context
}

func (f *fakeSource) Detail(ctx context.Context) (catalog.Video, error) {
	f.contexts = append(f.contexts, ctx)
	return f.video, f.detailErr
}
func (f *fakeSource) Resolve(ctx context.Context, id catalog.PartID) (Resolution, error) {
	f.contexts = append(f.contexts, ctx)
	f.resolved = append(f.resolved, id)
	return Resolution{Input: f.inputs[id], ExpiresAt: f.expires[id]}, f.errors[id]
}

type fakeEngine struct {
	deferCommands bool
	pending       []playback.Command
	snapshot      playback.Snapshot
	commands      []playback.Command
	closed        int
	failKind      playback.CommandKind
	failErr       error
}

func (e *fakeEngine) Snapshot() playback.Snapshot { return e.snapshot }
func (e *fakeEngine) Command(c playback.Command) error {
	e.commands = append(e.commands, c)
	if c.Kind == e.failKind {
		return e.failErr
	}
	if e.deferCommands {
		e.pending = append(e.pending, c)
		return nil
	}
	e.apply(c)
	return nil
}

func (e *fakeEngine) apply(c playback.Command) {
	switch c.Kind {
	case playback.Play:
		e.snapshot.State = playback.Playing
	case playback.Pause:
		e.snapshot.State = playback.Paused
	case playback.Seek:
		e.snapshot.Position = c.Position
	case playback.Volume:
		e.snapshot.Volume = c.Value
	case playback.Rate:
		e.snapshot.Rate = c.Value
	}
}
func (e *fakeEngine) Close() error { e.closed++; return nil }

type fakeOpener struct {
	inputs   []playback.Input
	engines  []*fakeEngine
	contexts []context.Context
	err      error
}

func (o *fakeOpener) Open(ctx context.Context, in playback.Input) (playback.Engine, error) {
	o.inputs = append(o.inputs, in)
	o.contexts = append(o.contexts, ctx)
	if o.err != nil {
		return nil, o.err
	}
	e := &fakeEngine{snapshot: playback.Snapshot{State: playback.Loading, Duration: 30 * time.Second, Rate: 1, Volume: 1}}
	o.engines = append(o.engines, e)
	return e, nil
}

type harness struct {
	s      *Session
	source *fakeSource
	opener *fakeOpener
	now    time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{now: time.Unix(100, 0), opener: &fakeOpener{}, source: &fakeSource{
		video:  catalog.Video{ID: "video", Title: "Title", Creator: "Creator", Description: "Description", CoverID: "cover", Parts: []catalog.Part{{ID: "p1", Title: "One", Duration: 30 * time.Second}, {ID: "p2", Title: "Two", Duration: 30 * time.Second}}},
		inputs: map[catalog.PartID]playback.Input{"p1": {FilePath: "/fixture-one.mp4"}, "p2": {FilePath: "/fixture-two.mp4"}}, errors: map[catalog.PartID]error{}, expires: map[catalog.PartID]time.Time{},
	}}
	var err error
	h.s, err = New(context.Background(), h.source, playback.New(h.opener, func() time.Time { return h.now }))
	if err != nil {
		t.Fatal(err)
	}
	h.s.now = func() time.Time { return h.now }
	return h
}
func (h *harness) start(t *testing.T) {
	t.Helper()
	if err := h.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func (h *harness) engine() *fakeEngine { return h.opener.engines[len(h.opener.engines)-1] }
func (h *harness) tick(state playback.State) {
	h.engine().snapshot.State = state
	h.now = h.now.Add(playback.PollInterval)
	h.s.Tick()
}
func requireError(t *testing.T, err error, want playback.ErrorCode) {
	t.Helper()
	if err != want {
		t.Fatalf("error=%v want %v", err, want)
	}
}
func countKind(e *fakeEngine, kind playback.CommandKind) int {
	n := 0
	for _, c := range e.commands {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

func TestConstructionAndCopyIsolation(t *testing.T) {
	h := newHarness(t)
	if len(h.opener.inputs) != 0 || h.s.Engine() != nil {
		t.Fatal("construction acquired playback")
	}
	h.source.video.Parts[0].Title = "source mutation"
	state := h.s.State()
	state.Video.Parts[0].Title = "state mutation"
	if h.s.State().Video.Parts[0].Title != "One" {
		t.Fatal("mutable catalog alias")
	}
	for _, change := range []func(*catalog.Video){
		func(v *catalog.Video) { v.ID = "" }, func(v *catalog.Video) { v.Title = " " }, func(v *catalog.Video) { v.Creator = "" }, func(v *catalog.Video) { v.Description = "" }, func(v *catalog.Video) { v.CoverID = "" }, func(v *catalog.Video) { v.Parts = nil }, func(v *catalog.Video) { v.Parts[1].ID = v.Parts[0].ID }, func(v *catalog.Video) { v.Parts[0].Title = "" }, func(v *catalog.Video) { v.Parts[0].Duration = 0 },
	} {
		x := newHarness(t)
		change(&x.source.video)
		_, err := New(context.Background(), x.source, x.s.controller)
		requireError(t, err, playback.Invalid)
	}
	_, err := New(context.Background(), nil, h.s.controller)
	requireError(t, err, playback.Invalid)
	_, err = New(context.Background(), h.source, nil)
	requireError(t, err, playback.Invalid)
	h.source.detailErr = errors.New("private detail path")
	_, err = New(context.Background(), h.source, h.s.controller)
	requireError(t, err, playback.Media)
	h.source.detailErr = fmt.Errorf("private: %w", playback.Unsupported)
	_, err = New(context.Background(), h.source, h.s.controller)
	requireError(t, err, playback.Unsupported)
}

func TestStartupPausedOnceAndReadOnlyState(t *testing.T) {
	h := newHarness(t)
	requireError(t, h.s.Start(nil), playback.Invalid)
	h.start(t)
	h.start(t)
	if len(h.opener.inputs) != 1 || h.s.Engine() != h.engine() {
		t.Fatal("start reopened or wrapped engine")
	}
	if h.s.State().SelectedPart != "p1" || h.s.State().PlaybackPart != "p1" || h.s.State().DesiredPlaying {
		t.Fatal(h.s.State())
	}
	before := len(h.engine().commands)
	_ = h.s.State()
	if len(h.engine().commands) != before {
		t.Fatal("State issued commands")
	}
	h.tick(playback.Ready)
	want := []playback.Command{{Kind: playback.Volume, Value: 1}, {Kind: playback.Rate, Value: 1}}
	if !reflect.DeepEqual(h.engine().commands, want) {
		t.Fatal(h.engine().commands)
	}
	h.tick(playback.Paused)
	h.tick(playback.Paused)
	if !reflect.DeepEqual(h.engine().commands, want) {
		t.Fatal("poll repeated initialization")
	}
}

func TestLoadingIntentSettingsAndReplacement(t *testing.T) {
	for _, finishPlaying := range []bool{false, true} {
		t.Run(fmt.Sprint(finishPlaying), func(t *testing.T) {
			h := newHarness(t)
			h.start(t)
			if err := h.s.SelectPart("p1"); err != nil {
				t.Fatal(err)
			}
			if err := h.s.SelectPart("p1"); err != nil {
				t.Fatal(err)
			}
			if !finishPlaying {
				if err := h.s.Pause(); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.s.SetRate(1.5); err != nil {
				t.Fatal(err)
			}
			if err := h.s.SetVolume(.4); err != nil {
				t.Fatal(err)
			}
			if err := h.s.RetryPlayback(); err != nil {
				t.Fatal(err)
			}
			if len(h.opener.inputs) != 1 || len(h.engine().commands) != 0 {
				t.Fatal("loading reopened or commanded")
			}
			h.tick(playback.Ready)
			h.tick(h.engine().snapshot.State)
			wantPlay := 0
			if finishPlaying {
				wantPlay = 1
			}
			if countKind(h.engine(), playback.Play) != wantPlay || countKind(h.engine(), playback.Rate) != 1 || countKind(h.engine(), playback.Volume) != 1 {
				t.Fatal(h.engine().commands)
			}
			if h.engine().snapshot.Volume != .4 || h.engine().snapshot.Rate != 1.5 {
				t.Fatal(h.engine().snapshot)
			}
		})
	}
	h := newHarness(t)
	h.start(t)
	old := h.engine()
	_ = h.s.Play()
	_ = h.s.SetRate(2)
	_ = h.s.SetVolume(.3)
	if err := h.s.SelectPart("p2"); err != nil {
		t.Fatal(err)
	}
	if old.closed != 1 || len(old.commands) != 0 || h.s.State().Playback.Generation != 2 || h.s.State().PlaybackPart != "p2" {
		t.Fatal("replacement lifetime/identity")
	}
	old.snapshot.State = playback.Ready
	h.tick(playback.Ready)
	if len(old.commands) != 0 || countKind(h.engine(), playback.Play) != 1 || h.engine().snapshot.Rate != 2 || h.engine().snapshot.Volume != .3 {
		t.Fatal("stale intent or lost settings")
	}
}

func TestCurrentPartAndEndedReplay(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	h.tick(playback.Paused)
	if err := h.s.SelectPart("p1"); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Playing)
	if err := h.s.SelectPart("p1"); err != nil {
		t.Fatal(err)
	}
	if len(h.opener.inputs) != 1 || countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("current part reopened or duplicate play")
	}
	for _, action := range []string{"play", "toggle", "select"} {
		t.Run(action, func(t *testing.T) {
			x := newHarness(t)
			x.start(t)
			_ = x.s.Play()
			x.tick(playback.Ready)
			x.engine().snapshot.Position = 30 * time.Second
			x.tick(playback.Ended)
			if x.s.State().DesiredPlaying || x.s.State().PlaybackPart != "p1" || len(x.opener.inputs) != 1 {
				t.Fatal("completion advanced or kept intent")
			}
			var err error
			switch action {
			case "play":
				err = x.s.Play()
			case "toggle":
				err = x.s.Toggle()
			case "select":
				err = x.s.SelectPart("p1")
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(x.opener.inputs) != 2 || x.s.State().Playback.Position != 0 || !x.s.State().DesiredPlaying {
				t.Fatal("ended replay did not reopen at zero")
			}
		})
	}
	if err := h.s.NextPart(); err != nil {
		t.Fatal(err)
	}
	if err := h.s.NextPart(); err != nil {
		t.Fatal(err)
	}
	if len(h.opener.inputs) != 2 || h.s.State().SelectedPart != "p2" {
		t.Fatal("last-part next not no-op")
	}
}

func TestSelectionFailureAndSeparateRetries(t *testing.T) {
	h := newHarness(t)
	h.source.inputs["p1"] = playback.Input{URL: "http://127.0.0.1:8080/a", Headers: map[string]string{"X-Test": "original"}}
	h.start(t)
	_ = h.s.Play()
	h.tick(playback.Ready)
	h.tick(playback.Playing)
	old := h.engine()
	h.source.errors["p2"] = errors.New("secret source URL")
	requireError(t, h.s.SelectPart("p2"), playback.Media)
	state := h.s.State()
	if state.SelectedPart != "p2" || state.PlaybackPart != "p1" || !state.DesiredPlaying || old.closed != 0 {
		t.Fatal(state)
	}
	requireError(t, h.s.SelectPart("unknown"), playback.Invalid)
	if h.s.State().SelectedPart != "p2" {
		t.Fatal("unknown selection changed identity")
	}
	h.source.inputs["p1"].Headers["X-Test"] = "changed"
	if err := h.s.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	if h.s.State().SelectedPart != "p2" || h.s.State().PlaybackPart != "p1" || h.s.State().SelectionError != playback.Media || h.opener.inputs[1].Headers["X-Test"] != "original" {
		t.Fatal("retry targets or input aliased")
	}
	if h.s.State().DesiredPlaying {
		t.Fatal("playback retry lost its latest pause intent")
	}
	delete(h.source.errors, "p2")
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	if h.s.State().SelectedPart != "p2" || h.s.State().PlaybackPart != "p2" || h.s.State().SelectionError != playback.NoError || !h.s.State().DesiredPlaying {
		t.Fatal(h.s.State())
	}
	requireError(t, h.s.RetrySelection(), playback.Invalid)
}

func TestInvalidResolvedInputPreservesEngineAndFailedOpenAdvancesIdentity(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	old := h.engine()
	h.source.inputs["p2"] = playback.Input{URL: "https://unapproved.invalid/video"}
	requireError(t, h.s.SelectPart("p2"), playback.Invalid)
	if old.closed != 0 || h.s.State().PlaybackPart != "p1" || h.s.State().Playback.Generation != 1 {
		t.Fatal("validation replaced playback")
	}
	h.source.inputs["p2"] = playback.Input{FilePath: "/fixture-two.mp4"}
	h.opener.err = errors.New("private decoder details")
	requireError(t, h.s.RetrySelection(), playback.Media)
	if old.closed != 1 || h.s.State().PlaybackPart != "p2" || h.s.State().Playback.Generation != 2 || h.s.State().DesiredPlaying || h.s.State().SelectionError != playback.NoError {
		t.Fatal(h.s.State())
	}
	h.opener.err = nil
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Ready)
	if countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("failed playback lost retry intent")
	}
}

func TestStartupSelectionRetryKeepsPausedIntent(t *testing.T) {
	h := newHarness(t)
	h.source.errors["p1"] = playback.Media
	requireError(t, h.s.Start(context.Background()), playback.Media)
	if len(h.opener.inputs) != 0 || h.s.State().PlaybackPart != "" {
		t.Fatal("failed resolve acquired playback")
	}
	delete(h.source.errors, "p1")
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Ready)
	if countKind(h.engine(), playback.Play) != 0 || h.s.State().DesiredPlaying {
		t.Fatal("startup retry autoplays")
	}
}

func TestSeekValidationAndSettings(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	requireError(t, h.s.Seek(time.Second), playback.Invalid)
	if len(h.engine().commands) != 0 || h.s.State().Playback.State == playback.Failed {
		t.Fatal("unusable seek harmed loading")
	}
	h.tick(playback.Paused)
	requireError(t, h.s.Seek(-time.Second), playback.Invalid)
	if err := h.s.Seek(time.Minute); err != nil {
		t.Fatal(err)
	}
	if h.engine().snapshot.Position != 30*time.Second {
		t.Fatal("seek not clamped")
	}
	h.engine().snapshot.Duration = 0
	h.tick(playback.Paused)
	requireError(t, h.s.Seek(0), playback.Invalid)
	if h.s.State().Playback.State == playback.Failed {
		t.Fatal("invalid seek failed healthy session")
	}
	before := len(h.engine().commands)
	for _, v := range []float64{-1, 1.1, math.NaN(), math.Inf(1)} {
		requireError(t, h.s.SetVolume(v), playback.Invalid)
	}
	for _, v := range []float64{0, 3.1, math.NaN(), math.Inf(-1)} {
		requireError(t, h.s.SetRate(v), playback.Invalid)
	}
	if len(h.engine().commands) != before || h.s.State().Volume != 1 || h.s.State().Rate != 1 {
		t.Fatal("invalid settings mutated state")
	}
	_ = h.s.SetVolume(.35)
	_ = h.s.ToggleMute()
	if h.s.State().Volume != 0 {
		t.Fatal("mute")
	}
	_ = h.s.ToggleMute()
	if h.s.State().Volume != .35 {
		t.Fatal("mute restore")
	}
	_ = h.s.SetVolume(0)
	_ = h.s.ToggleMute()
	if h.s.State().Volume != .35 {
		t.Fatal("zero overwrote restore volume")
	}
}

func TestCommandFailureClearsIntentAndInitialCommandsStop(t *testing.T) {
	for _, kind := range []playback.CommandKind{playback.Volume, playback.Rate, playback.Play} {
		t.Run(string(kind), func(t *testing.T) {
			h := newHarness(t)
			h.start(t)
			_ = h.s.Play()
			e := h.engine()
			e.failKind = kind
			e.failErr = errors.New("private native error")
			h.tick(playback.Ready)
			if h.s.State().Playback.State != playback.Failed || h.s.State().Playback.Error != playback.Media || h.s.State().DesiredPlaying || e.closed != 1 {
				t.Fatal(h.s.State())
			}
			n := len(e.commands)
			h.s.Tick()
			h.s.Tick()
			if len(e.commands) != n {
				t.Fatal("retried initialization after failure")
			}
			if err := h.s.RetryPlayback(); err != nil {
				t.Fatal(err)
			}
			h.tick(playback.Ready)
			if countKind(h.engine(), playback.Play) != 1 {
				t.Fatal("retry lost requested play intent")
			}
		})
	}
}

func TestTimeoutCancellationAndTerminalClose(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	old := h.engine()
	h.now = h.now.Add(10 * time.Second)
	h.s.Tick()
	if h.s.State().Playback.Error != playback.Timeout || old.closed != 1 {
		t.Fatal("readiness timeout")
	}
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	if len(h.opener.inputs) != 2 || h.s.State().DesiredPlaying {
		t.Fatal("timeout retry startup intent")
	}
	x := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	if err := x.s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	x.s.Tick()
	if x.s.State().Playback.Error != playback.Canceled || x.engine().closed != 1 || x.s.Engine() != nil {
		t.Fatal("cancel disposal")
	}
	requireError(t, x.s.Play(), playback.IsClosed)
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	if h.engine().closed != 1 || h.s.Engine() != nil || h.s.State().DesiredPlaying {
		t.Fatal("close lifetime")
	}
	for name, action := range map[string]func() error{
		"start": func() error { return h.s.Start(context.Background()) }, "select": func() error { return h.s.SelectPart("p1") }, "next": h.s.NextPart, "selection retry": h.s.RetrySelection, "playback retry": h.s.RetryPlayback, "play": h.s.Play, "pause": h.s.Pause, "toggle": h.s.Toggle, "seek": func() error { return h.s.Seek(0) }, "volume": func() error { return h.s.SetVolume(.5) }, "rate": func() error { return h.s.SetRate(1) }, "mute": h.s.ToggleMute,
	} {
		t.Run(name, func(t *testing.T) { requireError(t, action(), playback.IsClosed) })
	}
	h.s.Tick()
}

func TestActionAtFirstReadinessDoesNotDuplicatePendingPlay(t *testing.T) {
	for _, action := range []string{"play", "select-current"} {
		t.Run(action, func(t *testing.T) {
			h := newHarness(t)
			h.start(t)
			if err := h.s.Play(); err != nil {
				t.Fatal(err)
			}
			h.engine().snapshot.State = playback.Ready
			h.now = h.now.Add(playback.PollInterval)
			var err error
			if action == "play" {
				err = h.s.Play()
			} else {
				err = h.s.SelectPart("p1")
			}
			if err != nil {
				t.Fatal(err)
			}
			if countKind(h.engine(), playback.Play) != 1 {
				t.Fatalf("first-ready action sent duplicate play: %v", h.engine().commands)
			}
		})
	}
}

func TestCompletionClearsRetryIntentButExplicitReplayPlays(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	if err := h.s.Play(); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Ready)
	h.tick(playback.Ended)
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Ready)
	if h.s.State().DesiredPlaying || countKind(h.engine(), playback.Play) != 0 {
		t.Fatal("playback retry restored stale pre-completion intent")
	}
	if err := h.s.Play(); err != nil {
		t.Fatal(err)
	}
	if countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("explicit play did not request playback")
	}
}

// Keep command acceptance separate from engine consumption, as in the native
// adapter. Advancing the controller clock still sees the old engine snapshot.
func deferredPlaybackHarness(t *testing.T, playing bool) *harness {
	t.Helper()
	h := newHarness(t)
	h.start(t)
	t.Cleanup(func() { _ = h.s.Close() })
	h.tick(playback.Paused)
	if playing {
		if err := h.s.Play(); err != nil {
			t.Fatal(err)
		}
		h.tick(playback.Playing)
	}
	h.engine().commands = nil
	h.engine().deferCommands = true
	return h
}

func TestPlaybackIntentWithDeferredCommands(t *testing.T) {
	for _, playing := range []bool{false, true} {
		for _, toggle := range []bool{false, true} {
			t.Run(fmt.Sprintf("playing=%v/toggle=%v", playing, toggle), func(t *testing.T) {
				h := deferredPlaybackHarness(t, playing)
				e := h.engine()
				initial := e.snapshot.State
				for i, intent := range []bool{!playing, !playing, playing, playing} {
					h.now = h.now.Add(playback.PollInterval)
					action := h.s.Pause
					if intent {
						action = h.s.Play
					}
					if toggle && i%2 == 0 {
						action = h.s.Toggle
					}
					if err := action(); err != nil {
						t.Fatal(err)
					}
					if h.s.State().DesiredPlaying != intent {
						t.Fatal("lost requested intent")
					}
				}
				want := []playback.Command{{Kind: playback.Play}, {Kind: playback.Pause}}
				if playing {
					want[0], want[1] = want[1], want[0]
				}
				if !reflect.DeepEqual(e.commands, want) {
					t.Errorf("accepted commands=%v, want %v", e.commands, want)
				}
				if e.snapshot.State != initial {
					t.Fatal("engine consumed commands before explicit drain")
				}
				for _, cmd := range e.pending {
					e.apply(cmd)
				}
				e.pending = nil
				h.tick(e.snapshot.State)
				state := h.s.State()
				if state.DesiredPlaying != playing || state.Playback.State != initial {
					t.Errorf("after consumption: desired=%v state=%v, want desired=%v state=%v", state.DesiredPlaying, state.Playback.State, playing, initial)
				}
				if e.closed != 0 || h.s.Engine() != e || len(h.opener.inputs) != 1 || state.Playback.Generation != 1 {
					t.Fatal("intent change disposed or reopened playback")
				}
			})
		}
	}
}

func TestOppositePlaybackIntentFailure(t *testing.T) {
	for _, playing := range []bool{false, true} {
		t.Run(fmt.Sprintf("playing=%v", playing), func(t *testing.T) {
			h := deferredPlaybackHarness(t, playing)
			e := h.engine()
			first, second := h.s.Play, h.s.Pause
			failKind := playback.Pause
			if playing {
				first, second, failKind = h.s.Pause, h.s.Play, playback.Play
			}
			if err := first(); err != nil {
				t.Fatal(err)
			}
			e.failKind, e.failErr = failKind, errors.New("private native failure")
			h.now = h.now.Add(playback.PollInterval)
			requireError(t, second(), playback.Media)
			state := h.s.State()
			if state.Playback.State != playback.Failed || state.Playback.Error != playback.Media || state.DesiredPlaying || h.s.Engine() != nil || e.closed != 1 {
				t.Fatal("opposite command failure did not clear intent and dispose", state)
			}
			if len(e.commands) != 2 || e.commands[1].Kind != failKind {
				t.Fatal(e.commands)
			}
			h.s.Tick()
			if err := h.s.Close(); err != nil {
				t.Fatal(err)
			}
			if e.closed != 1 || len(e.commands) != 2 {
				t.Fatal("failure cleanup repeated commands or disposal")
			}
		})
	}
}

type ctxKey struct{}

func TestExpiringResolutionIsReusedUntilMarginThenResolvedAgain(t *testing.T) {
	h := newHarness(t)
	one, refreshed := playback.Input{URL: "http://127.0.0.1:1/one"}, playback.Input{URL: "http://127.0.0.1:1/one-refreshed"}
	h.source.inputs["p1"], h.source.expires["p1"] = one, h.now.Add(10*time.Minute)
	startCtx := context.WithValue(context.Background(), ctxKey{}, "start")
	if err := h.s.Start(startCtx); err != nil {
		t.Fatal(err)
	}
	if got := h.source.contexts[len(h.source.contexts)-1]; got != startCtx {
		t.Fatal("Resolve did not receive the session context")
	}
	h.tick(playback.Failed)
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	if len(h.source.resolved) != 1 || !reflect.DeepEqual(h.opener.inputs[1], one) {
		t.Fatalf("unexpired input not reused: resolved=%v inputs=%v", h.source.resolved, h.opener.inputs)
	}

	// At exactly ExpiresAt-ResolutionMargin the cached input is no longer usable.
	h.now = h.source.expires["p1"].Add(-ResolutionMargin)
	h.source.inputs["p1"], h.source.expires["p1"] = refreshed, h.now.Add(10*time.Minute)
	h.tick(playback.Failed)
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	if len(h.source.resolved) != 2 || !reflect.DeepEqual(h.opener.inputs[2], refreshed) || h.s.State().PlaybackPart != "p1" {
		t.Fatalf("expired input not resolved again: resolved=%v inputs=%v", h.source.resolved, h.opener.inputs)
	}

	// A failed re-resolution keeps the current engine and becomes a selection retry.
	h.tick(playback.Ended)
	engines := len(h.opener.engines)
	h.now = h.source.expires["p1"]
	h.source.errors["p1"] = errors.New("private signed URL")
	requireError(t, h.s.Play(), playback.Media)
	if len(h.opener.engines) != engines || h.engine().closed != 0 || h.s.State().SelectionError != playback.Media || h.s.State().PlaybackPart != "p1" {
		t.Fatal(h.s.State())
	}
	delete(h.source.errors, "p1")
	h.source.expires["p1"] = h.now.Add(10 * time.Minute)
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	h.tick(playback.Ready)
	if len(h.opener.engines) != engines+1 || h.s.State().SelectionError != playback.NoError || countKind(h.engine(), playback.Play) != 1 {
		t.Fatal("selection retry after failed re-resolution did not replay")
	}
}

func TestResolutionAlreadyInsideMarginIsRejectedWithoutOpening(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	old := h.engine()
	h.source.expires["p2"] = h.now.Add(ResolutionMargin)
	requireError(t, h.s.SelectPart("p2"), playback.Media)
	if len(h.opener.engines) != 1 || old.closed != 0 || h.s.State().PlaybackPart != "p1" || h.s.State().SelectionError != playback.Media {
		t.Fatal(h.s.State())
	}
	h.source.expires["p2"] = h.now.Add(ResolutionMargin + time.Nanosecond)
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	if h.s.State().PlaybackPart != "p2" || old.closed != 1 {
		t.Fatal(h.s.State())
	}
}

func TestNewPassesContextToDetailAndRejectsNilContext(t *testing.T) {
	h := newHarness(t)
	ctx := context.WithValue(context.Background(), ctxKey{}, "detail")
	if _, err := New(ctx, h.source, h.s.controller); err != nil {
		t.Fatal(err)
	}
	if h.source.contexts[len(h.source.contexts)-1] != ctx {
		t.Fatal("Detail did not receive the construction context")
	}
	_, err := New(nil, h.source, h.s.controller)
	requireError(t, err, playback.Invalid)
}

func TestExpiredPlaybackRetryKeepsPendingSelectionOfAnotherPart(t *testing.T) {
	for _, reresolveFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("reresolveFails=%v", reresolveFails), func(t *testing.T) {
			h := newHarness(t)
			h.source.expires["p1"] = h.now.Add(10 * time.Minute)
			h.start(t)
			if err := h.s.Play(); err != nil {
				t.Fatal(err)
			}
			h.tick(playback.Ready)
			h.tick(playback.Playing)
			h.source.errors["p2"] = errors.New("private p2 failure")
			requireError(t, h.s.SelectPart("p2"), playback.Media)
			h.tick(playback.Failed)
			h.now = h.now.Add(20 * time.Minute)
			h.source.expires["p1"] = h.now.Add(10 * time.Minute)
			engines := len(h.opener.engines)
			if reresolveFails {
				h.source.errors["p1"] = errors.New("private p1 failure")
				requireError(t, h.s.RetryPlayback(), playback.Media)
				if len(h.opener.engines) != engines {
					t.Fatal("failed re-resolution opened media")
				}
			} else {
				if err := h.s.RetryPlayback(); err != nil {
					t.Fatal(err)
				}
				if len(h.opener.engines) != engines+1 || h.s.State().PlaybackPart != "p1" {
					t.Fatal("expired playback was not resolved again")
				}
			}
			state := h.s.State()
			if state.SelectedPart != "p2" || state.SelectionError != playback.Media || h.s.selectionRetry != "p2" {
				t.Fatalf("pending p2 selection changed: %+v retry=%q", state, h.s.selectionRetry)
			}
			delete(h.source.errors, "p2")
			if err := h.s.RetrySelection(); err != nil {
				t.Fatal(err)
			}
			if h.s.State().PlaybackPart != "p2" || h.s.State().SelectionError != playback.NoError {
				t.Fatal(h.s.State())
			}
		})
	}
}
