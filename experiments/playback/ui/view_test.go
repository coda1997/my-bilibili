package ui

import (
	"context"
	"reflect"
	"testing"
	"time"

	gui "github.com/egoist/mygo/ui"
	"my-bilibili/experiments/playback/contract"
)

type testEngine struct {
	state    contract.State
	commands []contract.Command
	closed   bool
	closes   int
	sample   *contract.Snapshot
}

func (e *testEngine) Snapshot() contract.Snapshot {
	if e.sample != nil {
		return *e.sample
	}
	return contract.Snapshot{State: e.state, Duration: 8 * time.Second, Rate: 1, Volume: 1}
}
func (e *testEngine) Command(c contract.Command) error {
	e.commands = append(e.commands, c)
	return nil
}
func (e *testEngine) Close() error { e.closed = true; e.closes++; return nil }

type testOpener struct{ engine *testEngine }

func (o testOpener) Open(context.Context, contract.Input) (contract.Engine, error) {
	return o.engine, nil
}

func TestHeadlessChineseCommandsAndStates(t *testing.T) {
	now := time.Unix(1, 0)
	e := &testEngine{state: contract.Loading}
	v := &View{Controller: contract.New(testOpener{e}, func() time.Time { return now }), Sources: []Source{{"local", "Local fixture", contract.Input{FilePath: "synthetic.mp4"}}}}
	defer v.Close()
	tt := gui.NewTester(v.Render, 720, 780)
	if err := tt.Click("Chinese input (injected, not IME)"); err != nil {
		t.Fatal(err)
	}
	tt.Type("哔哩哔哩 测试")
	if v.Text != "哔哩哔哩 测试" {
		t.Fatalf("Chinese insertion failed: %q", v.Text)
	}
	if err := tt.Click("Local fixture"); err != nil {
		t.Fatal(err)
	}
	if v.Controller.Snapshot().State != contract.Loading {
		t.Fatal("loading not observable")
	}
	v.open(contract.Input{})
	v.Controller.Tick()
	if !v.Controller.Active() || e.closed {
		t.Fatal("invalid view input canceled current session")
	}
	for _, label := range []string{"Play", "Pause", "Seek 3s", "Volume 25%", "Rate 1.5x"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.commands) != 5 || e.commands[0].Kind != contract.Play || e.commands[2].Position != 3*time.Second {
		t.Fatal("control command delivery")
	}
	e.state = contract.Failed
	now = now.Add(time.Second)
	tt.Frame()
	if v.Controller.Snapshot().Error != contract.Media || !e.closed {
		t.Fatal("error not presented/cleaned")
	}
	if err := tt.Click("Play"); err != nil {
		t.Fatal(err)
	}
	if v.lastError != contract.IsClosed {
		t.Fatal("closed command not visible")
	}
}

func TestHeadlessCancelAndQuit(t *testing.T) {
	e := &testEngine{state: contract.Loading}
	quit := false
	v := &View{Controller: contract.New(testOpener{e}, nil), Sources: []Source{{"local", "Local fixture", contract.Input{FilePath: "a"}}}, Quit: func() { quit = true }}
	tt := gui.NewTester(v.Render, 720, 780)
	if err := tt.Click("Local fixture"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if v.Controller.Snapshot().Error != contract.Canceled || !e.closed {
		t.Fatal("cancel failed")
	}
	if err := tt.Click("Quit lab"); err != nil {
		t.Fatal(err)
	}
	if !quit || v.Controller.Active() {
		t.Fatal("quit failed")
	}
}

func access(t *testing.T, tt *gui.Tester, label string, disabled bool) gui.AccessInfo {
	t.Helper()
	a, ok := tt.Access(label)
	if !ok || a.Disabled != disabled {
		t.Fatalf("%q accessible=%t disabled=%t want=%t", label, ok, a.Disabled, disabled)
	}
	return a
}

func click(t *testing.T, tt *gui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitProbeWarningPreservesDefault(t *testing.T) {
	for _, warning := range []string{"", "EXPERIMENTAL replacement fixture probe"} {
		v := &View{Controller: contract.New(testOpener{&testEngine{}}, nil), Warning: warning}
		tt := gui.NewTester(v.Render, 760, 1200)
		label := warning
		if label == "" {
			label = ProvisionalWarning
		}
		access(t, tt, label, false)
		v.Close()
	}
}

func TestSampledTransportInteractionsAndNoFeedback(t *testing.T) {
	now := time.Unix(1, 0)
	sample := contract.Snapshot{State: contract.Ready, Duration: 8 * time.Second, Position: time.Second, Volume: .5, Rate: 1}
	e := &testEngine{sample: &sample}
	v := &View{Controller: contract.New(testOpener{e}, func() time.Time { return now })}
	v.open(contract.Input{FilePath: "synthetic.mp4"})
	defer v.Close()
	tt := gui.NewTester(v.Render, 760, 1200)
	access(t, tt, ProvisionalWarning, false)
	a := access(t, tt, "Seek position (seconds)", false)
	if a.Now != 1 || a.Max != 8 || a.Min != 0 {
		t.Fatalf("seek is not sampled in seconds: %+v", a)
	}
	click(t, tt, "Seek position (seconds)")
	if len(e.commands) != 1 || e.commands[0] != (contract.Command{Kind: contract.Seek, Position: 4 * time.Second}) {
		t.Fatalf("midpoint click did not seek4s: %+v", e.commands)
	}
	tt.Key(0, gui.KeyEnd)
	if len(e.commands) == 0 || e.commands[len(e.commands)-1] != (contract.Command{Kind: contract.Seek, Position: 8 * time.Second}) {
		t.Fatalf("seek did not clamp at duration: %+v", e.commands)
	}
	if v.Controller.Snapshot().Position != time.Second {
		t.Fatal("command echoed into sampled position")
	}
	click(t, tt, "Playback volume")
	tt.Key(0, gui.KeyHome)
	tt.Key(0, gui.KeyRight)
	last := e.commands[len(e.commands)-1]
	if last.Kind != contract.Volume || last.Value <= 0 || last.Value >= 1 {
		t.Fatalf("continuous volume interaction missing: %+v", last)
	}
	for i, label := range []string{"0.5x", "1x", "2x"} {
		click(t, tt, label)
		last = e.commands[len(e.commands)-1]
		if last.Kind != contract.Rate || last.Value != []float64{.5, 1, 2}[i] {
			t.Fatalf("preset %s: %+v", label, last)
		}
	}
	count := len(e.commands)
	sample.Position, sample.Volume, sample.Rate = 2500*time.Millisecond, .75, .5
	now = now.Add(time.Second)
	tt.Frame()
	tt.Frame()
	if len(e.commands) != count || access(t, tt, "Seek position (seconds)", false).Now != 2.5 || access(t, tt, "Playback volume", false).Now != .75 {
		t.Fatal("sample update did not update controls or replayed a command")
	}
}

func TestTransportDisabledStates(t *testing.T) {
	for _, state := range []contract.State{contract.Idle, contract.Loading, contract.Ready, contract.Paused, contract.Playing, contract.Ended, contract.Failed, contract.Closed} {
		for _, duration := range []time.Duration{0, -time.Second, 8 * time.Second} {
			t.Run(string(state)+"/"+duration.String(), func(t *testing.T) {
				e := &testEngine{sample: &contract.Snapshot{State: state, Duration: duration, Volume: 1, Rate: 1}}
				v := &View{Controller: contract.New(testOpener{e}, nil)}
				if state != contract.Idle && state != contract.Closed {
					v.open(contract.Input{FilePath: "a"})
				}
				defer v.Close()
				tt := gui.NewTester(v.Render, 760, 1200)
				usable := state == contract.Ready || state == contract.Paused || state == contract.Playing || state == contract.Ended
				seekDisabled := !usable || duration <= 0
				access(t, tt, "Seek position (seconds)", seekDisabled)
				access(t, tt, "Playback volume", !usable)
				for _, label := range []string{"0.5x", "1x", "2x"} {
					access(t, tt, label, !usable)
					if !usable {
						_ = tt.Click(label)
					}
				}
				if seekDisabled {
					_ = tt.Click("Seek position (seconds)")
					tt.Key(0, gui.KeyEnd)
				}
				if !usable {
					_ = tt.Click("Playback volume")
					tt.Key(0, gui.KeyHome)
				}
				if len(e.commands) != 0 {
					t.Fatalf("disabled controls emitted %+v", e.commands)
				}
			})
		}
	}
}

func TestHeldSlidersDoNotReplayOnNativeSampleChanges(t *testing.T) {
	for _, label := range []string{"Seek position (seconds)", "Playback volume"} {
		t.Run(label, func(t *testing.T) {
			now := time.Unix(1, 0)
			sample := contract.Snapshot{State: contract.Paused, Duration: 8 * time.Second, Position: time.Second, Volume: .2, Rate: 1}
			e := &testEngine{sample: &sample}
			v := &View{Controller: contract.New(testOpener{e}, func() time.Time { return now })}
			v.open(contract.Input{FilePath: "synthetic.mp4"})
			defer v.Close()
			tt := gui.NewTester(v.Render, 760, 1200)
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal("slider missing")
			}
			x, y := r.X+r.W/2, r.Y+r.H/2
			tt.Press(x, y)
			if len(e.commands) != 1 {
				t.Fatalf("held gesture emitted %d commands", len(e.commands))
			}
			sample.Position, sample.Volume = 2*time.Second, .8
			now = now.Add(time.Second)
			tt.Frame()
			tt.Frame()
			if len(e.commands) != 1 || v.Controller.Snapshot().Position != sample.Position {
				t.Fatal("held sample update replayed a command or hid native state")
			}
			tt.Release(x, y)
			tt.Frame()
			want := 2.0
			if label == "Playback volume" {
				want = .8
			}
			if a := access(t, tt, label, false); a.Now != want || len(e.commands) != 1 {
				t.Fatalf("release did not restore sampled control: %+v commands=%+v", a, e.commands)
			}
		})
	}
}

func TestCapturedSlidersOutsideBoundsDoNotReplay(t *testing.T) {
	for _, label := range []string{"Seek position (seconds)", "Playback volume"} {
		for _, releaseOutside := range []bool{false, true} {
			name := label + "/release-inside"
			if releaseOutside {
				name = label + "/release-outside"
			}
			t.Run(name, func(t *testing.T) {
				now := time.Unix(1, 0)
				sample := contract.Snapshot{State: contract.Paused, Duration: 8 * time.Second, Position: time.Second, Volume: .2, Rate: 1}
				e := &testEngine{sample: &sample}
				v := &View{Controller: contract.New(testOpener{e}, func() time.Time { return now })}
				v.open(contract.Input{FilePath: "synthetic.mp4"})
				defer v.Close()
				tt := gui.NewTester(v.Render, 760, 1200)
				r, ok := tt.Find(label)
				if !ok {
					t.Fatal("slider missing")
				}
				x, y, outside := r.X+r.W/2, r.Y+r.H/2, r.Y+r.H+25
				tt.Press(x, y)
				tt.Move(x, outside)
				if len(e.commands) != 1 {
					t.Fatalf("vertical departure replayed commands: %d", len(e.commands))
				}
				sample.Position, sample.Volume = 2*time.Second, .8
				now = now.Add(time.Second)
				tt.Frame()
				tt.Frame()
				held := 4.0
				if label == "Playback volume" {
					held = .5
				}
				if a := access(t, tt, label, false); a.Now != held || len(e.commands) != 1 || v.Controller.Snapshot().Position != sample.Position {
					t.Fatalf("outside held sample replayed or hid native state: %+v commands=%d", a, len(e.commands))
				}
				tt.Move(x, y)
				tt.Frame()
				if len(e.commands) != 1 {
					t.Fatal("re-entry replayed stationary gesture")
				}
				if releaseOutside {
					tt.Move(x, outside)
					y = outside
				}
				tt.Release(x, y)
				tt.Frame()
				want := 2.0
				if label == "Playback volume" {
					want = .8
				}
				if a := access(t, tt, label, false); a.Now != want || len(e.commands) != 1 {
					t.Fatalf("release did not restore native sample: %+v commands=%d", a, len(e.commands))
				}
			})
		}
	}
}

func TestCapturedSlidersDisableAndReplacement(t *testing.T) {
	for _, label := range []string{"Seek position (seconds)", "Playback volume"} {
		t.Run(label, func(t *testing.T) {
			now := time.Unix(1, 0)
			sample := contract.Snapshot{State: contract.Paused, Duration: 8 * time.Second, Position: time.Second, Volume: .2, Rate: 1}
			e := &testEngine{sample: &sample}
			v := &View{Controller: contract.New(testOpener{e}, func() time.Time { return now })}
			v.open(contract.Input{FilePath: "synthetic.mp4"})
			defer v.Close()
			tt := gui.NewTester(v.Render, 760, 1200)
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal("slider missing")
			}
			x, y := r.X+r.W/2, r.Y+r.H+25
			tt.Press(x, r.Y+r.H/2)
			tt.Move(x, y)
			count := len(e.commands)
			generation := v.Controller.Snapshot().Generation
			v.Close()
			tt.Frame()
			tt.Frame()
			access(t, tt, label, true)
			tt.Release(x, y)
			if len(e.commands) != count || v.seekUI.pressed || v.volumeUI.pressed {
				t.Fatal("disabled captured slider retained interaction or emitted command")
			}
			sample.Position, sample.Volume = 3*time.Second, .7
			v.open(contract.Input{FilePath: "replacement.mp4"})
			now = now.Add(time.Second)
			tt.Frame()
			want := 3.0
			if label == "Playback volume" {
				want = .7
			}
			if a := access(t, tt, label, false); a.Now != want || len(e.commands) != count || v.Controller.Snapshot().Generation <= generation {
				t.Fatalf("replacement did not show new generation sample: %+v commands=%d", a, len(e.commands))
			}
		})
	}
}

func TestSliderSampleResetsOnNewGeneration(t *testing.T) {
	s := sliderState{value: 4, generation: 1, pressed: true}
	s.sample(1, 2)
	if s.value != 4 {
		t.Fatal("held sample overwrote interaction")
	}
	s.sample(2, 3)
	if s.value != 3 || s.generation != 2 {
		t.Fatal("new generation retained prior slider value")
	}
}

type openFunc func(context.Context, contract.Input) (contract.Engine, error)

func (f openFunc) Open(ctx context.Context, in contract.Input) (contract.Engine, error) {
	return f(ctx, in)
}

func TestRetryCopiesValidInputPreservesUnsupportedAndIgnoresInvalid(t *testing.T) {
	for _, in := range []contract.Input{
		{URL: "http://127.0.0.1:1234/protected", Headers: map[string]string{"X-Test": "original"}},
		{URL: "http://127.0.0.1:1234/video", AudioURL: "http://127.0.0.1:1234/audio"},
	} {
		t.Run(in.URL, func(t *testing.T) {
			var inputs []contract.Input
			v := &View{Controller: contract.New(openFunc(func(_ context.Context, got contract.Input) (contract.Engine, error) {
				inputs = append(inputs, got)
				return nil, contract.Unsupported
			}), nil)}
			defer v.Close()
			tt := gui.NewTester(v.Render, 760, 1200)
			access(t, tt, "Retry last input", true)
			v.retry()
			v.open(in)
			first := inputs[0]
			if in.Headers != nil {
				in.Headers["X-Test"] = "mutated"
			}
			v.open(contract.Input{})
			tt.Frame()
			generation := v.Controller.Snapshot().Generation
			access(t, tt, "Retry last input", false)
			click(t, tt, "Retry last input")
			if len(inputs) != 2 || !reflect.DeepEqual(inputs[1], first) || v.Controller.Snapshot().Generation <= generation || v.lastError != contract.Unsupported {
				t.Fatalf("retry lost valid unsupported input: %+v", inputs)
			}
		})
	}
}

func TestRetryLifecycleFailureRecoveryCloseAndEnded(t *testing.T) {
	now := time.Unix(1, 0)
	var engines []*testEngine
	var contexts []context.Context
	v := &View{Controller: contract.New(openFunc(func(ctx context.Context, _ contract.Input) (contract.Engine, error) {
		contexts = append(contexts, ctx)
		e := &testEngine{state: contract.Ready}
		engines = append(engines, e)
		if len(engines) == 1 {
			return e, contract.Media
		}
		return e, nil
	}), func() time.Time { return now })}
	defer v.Close()
	tt := gui.NewTester(v.Render, 760, 1200)
	v.open(contract.Input{FilePath: "synthetic.mp4"})
	tt.Frame()
	if !engines[0].closed || contexts[0].Err() == nil {
		t.Fatal("failed opener left owned engine/context")
	}
	generation := v.Controller.Snapshot().Generation
	click(t, tt, "Retry last input")
	if v.lastError != contract.NoError || v.Controller.Snapshot().Generation <= generation || !v.Controller.Active() {
		t.Fatal("explicit retry did not recover fake failure")
	}
	v.open(contract.Input{})
	if contexts[1].Err() != nil || engines[1].closed {
		t.Fatal("invalid input canceled active retry")
	}
	v.Controller.Tick()
	click(t, tt, "Retry last input")
	if !engines[1].closed || contexts[1].Err() == nil || engines[1].closes != 1 || v.lastError != contract.NoError {
		t.Fatal("replacement retry did not dispose/reset control error")
	}
	v.Close()
	generation = v.Controller.Snapshot().Generation
	tt.Frame()
	click(t, tt, "Retry last input")
	if v.Controller.Snapshot().Generation <= generation || !v.Controller.Active() || v.lastError != contract.NoError {
		t.Fatal("close erased retry input")
	}
	engines[len(engines)-1].state = contract.Ended
	now = now.Add(time.Second)
	tt.Frame()
	if v.Controller.Snapshot().State != contract.Ended {
		t.Fatal("ended state not observed before retry")
	}
	generation = v.Controller.Snapshot().Generation
	click(t, tt, "Retry last input")
	if v.Controller.Snapshot().Generation <= generation {
		t.Fatal("ended retry did not start a new generation")
	}
}

func TestRetryDisabledDuringLoadingAndDesktopScroll(t *testing.T) {
	e := &testEngine{state: contract.Loading}
	v := &View{Controller: contract.New(testOpener{e}, nil)}
	v.open(contract.Input{FilePath: "a"})
	defer v.Close()
	tt := gui.NewTester(v.Render, 760, 840)
	access(t, tt, "Retry last input", true)
	generation := v.Controller.Snapshot().Generation
	v.retry()
	if e.closed || v.Controller.Snapshot().Generation != generation {
		t.Fatal("loading retry replaced ownership")
	}
	// The warning is visible initially; the lower controls remain reachable.
	if a := access(t, tt, ProvisionalWarning, false); a.Offscreen || a.Bounds.Y < 0 || a.Bounds.Y >= 840 {
		t.Fatal("provisional warning is not initially visible")
	}
	click(t, tt, "Close media")
	if v.Controller.Active() {
		t.Fatal("close control not usable in desktop scroll view")
	}
}
