package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/internal/catalog"
)

type fakeSource struct {
	video     catalog.Video
	detailErr error
	inputs    map[catalog.PartID]contract.Input
	errors    map[catalog.PartID]error
	resolved  []catalog.PartID
}

func (f *fakeSource) Detail() (catalog.Video, error) { return f.video, f.detailErr }
func (f *fakeSource) Resolve(id catalog.PartID) (contract.Input, error) {
	f.resolved = append(f.resolved, id)
	return f.inputs[id], f.errors[id]
}

type fakeEngine struct {
	deferCommands bool
	pending       []contract.Command
	snapshot      contract.Snapshot
	commands      []contract.Command
	closed        int
	failKind      contract.CommandKind
	failErr       error
}

func (e *fakeEngine) Snapshot() contract.Snapshot { return e.snapshot }
func (e *fakeEngine) Command(c contract.Command) error {
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

func (e *fakeEngine) apply(c contract.Command) {
	switch c.Kind {
	case contract.Play:
		e.snapshot.State = contract.Playing
	case contract.Pause:
		e.snapshot.State = contract.Paused
	case contract.Seek:
		e.snapshot.Position = c.Position
	case contract.Volume:
		e.snapshot.Volume = c.Value
	case contract.Rate:
		e.snapshot.Rate = c.Value
	}
}
func (e *fakeEngine) Close() error { e.closed++; return nil }

type fakeOpener struct {
	inputs   []contract.Input
	engines  []*fakeEngine
	contexts []context.Context
	err      error
}

func (o *fakeOpener) Open(ctx context.Context, in contract.Input) (contract.Engine, error) {
	o.inputs = append(o.inputs, in)
	o.contexts = append(o.contexts, ctx)
	if o.err != nil {
		return nil, o.err
	}
	e := &fakeEngine{snapshot: contract.Snapshot{State: contract.Loading, Duration: 30 * time.Second, Rate: 1, Volume: 1}}
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
		inputs: map[catalog.PartID]contract.Input{"p1": {FilePath: "/fixture-one.mp4"}, "p2": {FilePath: "/fixture-two.mp4"}}, errors: map[catalog.PartID]error{},
	}}
	var err error
	h.s, err = New(h.source, contract.New(h.opener, func() time.Time { return h.now }))
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func (h *harness) start(t *testing.T) {
	t.Helper()
	if err := h.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func (h *harness) engine() *fakeEngine { return h.opener.engines[len(h.opener.engines)-1] }
func (h *harness) tick(state contract.State) {
	h.engine().snapshot.State = state
	h.now = h.now.Add(contract.PollInterval)
	h.s.Tick()
}
func requireError(t *testing.T, err error, want contract.ErrorCode) {
	t.Helper()
	if err != want {
		t.Fatalf("error=%v want %v", err, want)
	}
}
func countKind(e *fakeEngine, kind contract.CommandKind) int {
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
		_, err := New(x.source, x.s.controller)
		requireError(t, err, contract.Invalid)
	}
	_, err := New(nil, h.s.controller)
	requireError(t, err, contract.Invalid)
	_, err = New(h.source, nil)
	requireError(t, err, contract.Invalid)
	h.source.detailErr = errors.New("private detail path")
	_, err = New(h.source, h.s.controller)
	requireError(t, err, contract.Media)
	h.source.detailErr = fmt.Errorf("private: %w", contract.Unsupported)
	_, err = New(h.source, h.s.controller)
	requireError(t, err, contract.Unsupported)
}

func TestStartupPausedOnceAndReadOnlyState(t *testing.T) {
	h := newHarness(t)
	requireError(t, h.s.Start(nil), contract.Invalid)
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
	h.tick(contract.Ready)
	want := []contract.Command{{Kind: contract.Volume, Value: 1}, {Kind: contract.Rate, Value: 1}}
	if !reflect.DeepEqual(h.engine().commands, want) {
		t.Fatal(h.engine().commands)
	}
	h.tick(contract.Paused)
	h.tick(contract.Paused)
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
			h.tick(contract.Ready)
			h.tick(h.engine().snapshot.State)
			wantPlay := 0
			if finishPlaying {
				wantPlay = 1
			}
			if countKind(h.engine(), contract.Play) != wantPlay || countKind(h.engine(), contract.Rate) != 1 || countKind(h.engine(), contract.Volume) != 1 {
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
	old.snapshot.State = contract.Ready
	h.tick(contract.Ready)
	if len(old.commands) != 0 || countKind(h.engine(), contract.Play) != 1 || h.engine().snapshot.Rate != 2 || h.engine().snapshot.Volume != .3 {
		t.Fatal("stale intent or lost settings")
	}
}

func TestCurrentPartAndEndedReplay(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	h.tick(contract.Paused)
	if err := h.s.SelectPart("p1"); err != nil {
		t.Fatal(err)
	}
	h.tick(contract.Playing)
	if err := h.s.SelectPart("p1"); err != nil {
		t.Fatal(err)
	}
	if len(h.opener.inputs) != 1 || countKind(h.engine(), contract.Play) != 1 {
		t.Fatal("current part reopened or duplicate play")
	}
	for _, action := range []string{"play", "toggle", "select"} {
		t.Run(action, func(t *testing.T) {
			x := newHarness(t)
			x.start(t)
			_ = x.s.Play()
			x.tick(contract.Ready)
			x.engine().snapshot.Position = 30 * time.Second
			x.tick(contract.Ended)
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
	h.source.inputs["p1"] = contract.Input{URL: "http://127.0.0.1:8080/a", Headers: map[string]string{"X-Test": "original"}}
	h.start(t)
	_ = h.s.Play()
	h.tick(contract.Ready)
	h.tick(contract.Playing)
	old := h.engine()
	h.source.errors["p2"] = errors.New("secret source URL")
	requireError(t, h.s.SelectPart("p2"), contract.Media)
	state := h.s.State()
	if state.SelectedPart != "p2" || state.PlaybackPart != "p1" || !state.DesiredPlaying || old.closed != 0 {
		t.Fatal(state)
	}
	requireError(t, h.s.SelectPart("unknown"), contract.Invalid)
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
	if h.s.State().SelectedPart != "p2" || h.s.State().PlaybackPart != "p1" || h.s.State().SelectionError != contract.Media || h.opener.inputs[1].Headers["X-Test"] != "original" {
		t.Fatal("retry targets or input aliased")
	}
	if h.s.State().DesiredPlaying {
		t.Fatal("playback retry lost its latest pause intent")
	}
	delete(h.source.errors, "p2")
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	if h.s.State().SelectedPart != "p2" || h.s.State().PlaybackPart != "p2" || h.s.State().SelectionError != contract.NoError || !h.s.State().DesiredPlaying {
		t.Fatal(h.s.State())
	}
	requireError(t, h.s.RetrySelection(), contract.Invalid)
}

func TestInvalidResolvedInputPreservesEngineAndFailedOpenAdvancesIdentity(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	old := h.engine()
	h.source.inputs["p2"] = contract.Input{URL: "https://unapproved.invalid/video"}
	requireError(t, h.s.SelectPart("p2"), contract.Invalid)
	if old.closed != 0 || h.s.State().PlaybackPart != "p1" || h.s.State().Playback.Generation != 1 {
		t.Fatal("validation replaced playback")
	}
	h.source.inputs["p2"] = contract.Input{FilePath: "/fixture-two.mp4"}
	h.opener.err = errors.New("private decoder details")
	requireError(t, h.s.RetrySelection(), contract.Media)
	if old.closed != 1 || h.s.State().PlaybackPart != "p2" || h.s.State().Playback.Generation != 2 || h.s.State().DesiredPlaying || h.s.State().SelectionError != contract.NoError {
		t.Fatal(h.s.State())
	}
	h.opener.err = nil
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	h.tick(contract.Ready)
	if countKind(h.engine(), contract.Play) != 1 {
		t.Fatal("failed playback lost retry intent")
	}
}

func TestStartupSelectionRetryKeepsPausedIntent(t *testing.T) {
	h := newHarness(t)
	h.source.errors["p1"] = contract.Media
	requireError(t, h.s.Start(context.Background()), contract.Media)
	if len(h.opener.inputs) != 0 || h.s.State().PlaybackPart != "" {
		t.Fatal("failed resolve acquired playback")
	}
	delete(h.source.errors, "p1")
	if err := h.s.RetrySelection(); err != nil {
		t.Fatal(err)
	}
	h.tick(contract.Ready)
	if countKind(h.engine(), contract.Play) != 0 || h.s.State().DesiredPlaying {
		t.Fatal("startup retry autoplays")
	}
}

func TestSeekValidationAndSettings(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	requireError(t, h.s.Seek(time.Second), contract.Invalid)
	if len(h.engine().commands) != 0 || h.s.State().Playback.State == contract.Failed {
		t.Fatal("unusable seek harmed loading")
	}
	h.tick(contract.Paused)
	requireError(t, h.s.Seek(-time.Second), contract.Invalid)
	if err := h.s.Seek(time.Minute); err != nil {
		t.Fatal(err)
	}
	if h.engine().snapshot.Position != 30*time.Second {
		t.Fatal("seek not clamped")
	}
	h.engine().snapshot.Duration = 0
	h.tick(contract.Paused)
	requireError(t, h.s.Seek(0), contract.Invalid)
	if h.s.State().Playback.State == contract.Failed {
		t.Fatal("invalid seek failed healthy session")
	}
	before := len(h.engine().commands)
	for _, v := range []float64{-1, 1.1, math.NaN(), math.Inf(1)} {
		requireError(t, h.s.SetVolume(v), contract.Invalid)
	}
	for _, v := range []float64{0, 3.1, math.NaN(), math.Inf(-1)} {
		requireError(t, h.s.SetRate(v), contract.Invalid)
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
	for _, kind := range []contract.CommandKind{contract.Volume, contract.Rate, contract.Play} {
		t.Run(string(kind), func(t *testing.T) {
			h := newHarness(t)
			h.start(t)
			_ = h.s.Play()
			e := h.engine()
			e.failKind = kind
			e.failErr = errors.New("private native error")
			h.tick(contract.Ready)
			if h.s.State().Playback.State != contract.Failed || h.s.State().Playback.Error != contract.Media || h.s.State().DesiredPlaying || e.closed != 1 {
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
			h.tick(contract.Ready)
			if countKind(h.engine(), contract.Play) != 1 {
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
	if h.s.State().Playback.Error != contract.Timeout || old.closed != 1 {
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
	if x.s.State().Playback.Error != contract.Canceled || x.engine().closed != 1 || x.s.Engine() != nil {
		t.Fatal("cancel disposal")
	}
	requireError(t, x.s.Play(), contract.IsClosed)
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
		t.Run(name, func(t *testing.T) { requireError(t, action(), contract.IsClosed) })
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
			h.engine().snapshot.State = contract.Ready
			h.now = h.now.Add(contract.PollInterval)
			var err error
			if action == "play" {
				err = h.s.Play()
			} else {
				err = h.s.SelectPart("p1")
			}
			if err != nil {
				t.Fatal(err)
			}
			if countKind(h.engine(), contract.Play) != 1 {
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
	h.tick(contract.Ready)
	h.tick(contract.Ended)
	if err := h.s.RetryPlayback(); err != nil {
		t.Fatal(err)
	}
	h.tick(contract.Ready)
	if h.s.State().DesiredPlaying || countKind(h.engine(), contract.Play) != 0 {
		t.Fatal("playback retry restored stale pre-completion intent")
	}
	if err := h.s.Play(); err != nil {
		t.Fatal(err)
	}
	if countKind(h.engine(), contract.Play) != 1 {
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
	h.tick(contract.Paused)
	if playing {
		if err := h.s.Play(); err != nil {
			t.Fatal(err)
		}
		h.tick(contract.Playing)
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
					h.now = h.now.Add(contract.PollInterval)
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
				want := []contract.Command{{Kind: contract.Play}, {Kind: contract.Pause}}
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
			failKind := contract.Pause
			if playing {
				first, second, failKind = h.s.Pause, h.s.Play, contract.Play
			}
			if err := first(); err != nil {
				t.Fatal(err)
			}
			e.failKind, e.failErr = failKind, errors.New("private native failure")
			h.now = h.now.Add(contract.PollInterval)
			requireError(t, second(), contract.Media)
			state := h.s.State()
			if state.Playback.State != contract.Failed || state.Playback.Error != contract.Media || state.DesiredPlaying || h.s.Engine() != nil || e.closed != 1 {
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
