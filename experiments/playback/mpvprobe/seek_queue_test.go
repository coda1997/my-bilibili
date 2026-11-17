package mpvprobe

import (
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"my-bilibili/experiments/playback/contract"
)

type queuedNativeCommand struct {
	id   uint64
	args []string
}

// The real adapter owns the queue and native loop. Only native replies and
// property observations are controlled, so a held reply models a slow consumer.
func seekController(t *testing.T) (*contract.Controller, *engine, *fakeNative, <-chan queuedNativeCommand) {
	t.Helper()
	f := fake()
	tracks := trackNode(true, false, false)
	f.a.getProperty = func(_ uintptr, _ string, _ int32, p unsafe.Pointer) int32 {
		*(*nodeValue)(p) = tracks.root
		return 0
	}
	sent := make(chan queuedNativeCommand, 40)
	command := f.a.command
	f.a.command = func(core uintptr, id uint64, p unsafe.Pointer) int32 {
		args := commandArgs(p)
		result := command(core, id, p)
		if len(args) == 3 && (args[0] == "seek" || args[0] == "set" && args[1] != "vid") {
			sent <- queuedNativeCommand{id, args}
		}
		return result
	}
	c := contract.New(Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}, nil)
	if err := c.Open(context.Background(), contract.Input{FilePath: "synthetic.mp4"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		runtime.KeepAlive(tracks)
	})
	e := c.Engine().(*engine)
	select {
	case <-f.sent:
	case <-time.After(time.Second):
		t.Fatal("load was not sent")
	}
	duration := float64(600)
	p := property{Format: 5, Data: unsafe.Pointer(&duration)}
	f.events <- &event{ID: 5, Reply: 1}
	f.events <- &event{ID: 8}
	f.events <- &event{ID: 22, Reply: 2, Data: unsafe.Pointer(&p)}
	await(t, func() bool { c.Tick(); return c.Snapshot().State == contract.Ready })
	return c, e, f, sent
}

func receiveQueuedCommand(t *testing.T, sent <-chan queuedNativeCommand, want []string) queuedNativeCommand {
	t.Helper()
	select {
	case got := <-sent:
		if !reflect.DeepEqual(got.args, want) {
			t.Fatalf("native command = %v, want %v", got.args, want)
		}
		return got
	case <-time.After(time.Second):
		t.Fatalf("native command not sent: %v", want)
		return queuedNativeCommand{}
	}
}

func TestSeekBurstKeepsControllerAndLatestPosition(t *testing.T) {
	c, e, f, sent := seekController(t)
	generation := c.Snapshot().Generation
	if err := c.Command(contract.Command{Kind: contract.Seek, Position: time.Second}); err != nil {
		t.Fatal(err)
	}
	pending := receiveQueuedCommand(t, sent, []string{"seek", "1.000000000", "absolute+exact"})
	// Each burst arrives while an actual native command reply is held. The
	// first already-sent target must remain intact while unsent targets merge.
	for _, burst := range []struct {
		start, end int
		want       string
	}{{2, 101, "101.000000000"}, {102, 202, "202.000000000"}} {
		for i := burst.start; i <= burst.end; i++ {
			if err := c.Command(contract.Command{Kind: contract.Seek, Position: time.Duration(i) * time.Second}); err != nil {
				t.Fatalf("seek %d: %v; controller=%+v active=%v owned=%+v", i, err, c.Snapshot(), c.Active(), Owned())
			}
		}
		if c.Engine() != e || c.Snapshot().Generation != generation || !c.Active() {
			t.Fatal("seek burst replaced or disposed the engine", c.Snapshot())
		}
		select {
		case got := <-sent:
			t.Fatal("command overtook pending reply", got)
		default:
		}
		f.events <- &event{ID: 5, Reply: pending.id}
		pending = receiveQueuedCommand(t, sent, []string{"seek", burst.want, "absolute+exact"})
	}
	position := float64(202)
	p := property{Format: 5, Data: unsafe.Pointer(&position)}
	f.events <- &event{ID: 5, Reply: pending.id}
	f.events <- &event{ID: 22, Reply: 1, Data: unsafe.Pointer(&p)}
	await(t, func() bool { c.Tick(); return c.Snapshot().Position == 202*time.Second })
	if c.Engine() != e || c.Snapshot().Generation != generation || c.Snapshot().Error != contract.NoError {
		t.Fatal("final target did not preserve the live engine", c.Snapshot())
	}
	if err := c.Close(); err != nil || Owned() != (Resources{}) {
		t.Fatal("close retained ownership", err, Owned())
	}
	if e.Command(contract.Command{Kind: contract.Seek, Position: time.Second}) != contract.IsClosed {
		t.Fatal("closed engine accepted Seek")
	}
}

func TestSeekQueuePreservesControlBarriers(t *testing.T) {
	c, _, f, sent := seekController(t)
	if err := c.Command(contract.Command{Kind: contract.Seek, Position: time.Second}); err != nil {
		t.Fatal(err)
	}
	pending := receiveQueuedCommand(t, sent, []string{"seek", "1.000000000", "absolute+exact"})
	controls := []contract.Command{
		{Kind: contract.Pause}, {Kind: contract.Volume, Value: .4},
		{Kind: contract.Rate, Value: 1.5}, {Kind: contract.Play},
	}
	wantControls := [][]string{
		{"set", "pause", "yes"}, {"set", "volume", "40"},
		{"set", "speed", "1.5"}, {"set", "pause", "no"},
	}
	var want [][]string
	for _, control := range controls {
		for i := 2; i <= 50; i++ {
			if err := c.Command(contract.Command{Kind: contract.Seek, Position: time.Duration(i) * time.Second}); err != nil {
				t.Fatal(err)
			}
		}
		if err := c.Command(control); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(controls); i++ {
		want = append(want, []string{"seek", "50.000000000", "absolute+exact"}, wantControls[i])
	}
	if err := c.Command(contract.Command{Kind: contract.Seek, Position: 55 * time.Second}); err != nil {
		t.Fatal(err)
	}
	want = append(want, []string{"seek", "55.000000000", "absolute+exact"})
	for _, args := range want {
		f.events <- &event{ID: 5, Reply: pending.id}
		pending = receiveQueuedCommand(t, sent, args)
	}
	// A native error from the last coalesced path still disposes instead of
	// being swallowed as successful input handling.
	f.events <- &event{ID: 5, Reply: pending.id, Error: -1}
	await(t, func() bool { c.Tick(); return !c.Active() })
	if c.Snapshot().State != contract.Failed || c.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
		t.Fatal("native failure was hidden or retained resources", c.Snapshot(), Owned())
	}
}

func TestSeekQueueFullTailReplacementAndBound(t *testing.T) {
	for _, seekTail := range []bool{false, true} {
		t.Run(map[bool]string{false: "control-tail", true: "seek-tail"}[seekTail], func(t *testing.T) {
			f := fake()
			e := openFake(t, f) // Hold load reply so all 32 entries remain unsent.
			for i := 0; i < 31; i++ {
				if err := e.Command(contract.Command{Kind: contract.Play}); err != nil {
					t.Fatal(err)
				}
			}
			tail := contract.Command{Kind: contract.Pause}
			if seekTail {
				tail = contract.Command{Kind: contract.Seek, Position: time.Second}
			}
			if err := e.Command(tail); err != nil {
				t.Fatal(err)
			}
			want := error(contract.Media)
			if seekTail {
				want = nil
			}
			if err := e.Command(contract.Command{Kind: contract.Seek, Position: 2 * time.Second}); err != want {
				t.Fatalf("full queue Seek = %v, want %v", err, want)
			}
			if err := e.Command(contract.Command{Kind: contract.Seek, Position: -time.Second}); err != contract.Invalid {
				t.Fatal("invalid target accepted", err)
			}
			if err := e.Command(contract.Command{Kind: contract.Volume, Value: .5}); err != contract.Media {
				t.Fatal("queue bound changed or a control was silently dropped", err)
			}
			if err := e.Close(); err != nil || Owned() != (Resources{}) {
				t.Fatal("full queue suppressed cleanup", err, Owned())
			}
		})
	}
}
