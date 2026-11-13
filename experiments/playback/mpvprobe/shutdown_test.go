package mpvprobe

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"my-bilibili/experiments/playback/contract"
)

// Model the native dependency from CA-CLEANUP-002: video-output destruction
// cannot acknowledge completion until the render owner consumes its last frame.
// An unrelated command reply must not release that ownership barrier.
func TestCloseServicesPendingFrameBeforeRenderFree(t *testing.T) {
	f := fake()
	var disabling, consumed, acknowledged atomic.Bool
	command, render, wait, free := f.a.command, f.a.render, f.a.wait, f.a.renderFree
	f.a.command = func(core uintptr, id uint64, p unsafe.Pointer) int32 {
		args := commandArgs(p)
		if len(args) == 3 && args[0] == "set" && args[1] == "vid" && args[2] == "no" {
			if id != 0 {
				t.Errorf("shutdown must reserve a reply ID distinct from playback commands: %d", id)
			}
			f.events <- &event{ID: 5, Reply: 999}
			disabling.Store(true)
			return 0
		}
		return command(core, id, p)
	}
	f.a.renderUpdate = func(uintptr) uint64 {
		if disabling.Load() && !consumed.Load() {
			return 1
		}
		return 0
	}
	f.a.render = func(handle uintptr, p unsafe.Pointer) int32 {
		result := render(handle, p)
		if disabling.Load() && consumed.CompareAndSwap(false, true) {
			f.events <- &event{ID: 5, Reply: 0}
		}
		return result
	}
	f.a.wait = func(core uintptr, timeout float64) unsafe.Pointer {
		p := wait(core, timeout)
		if ev := (*event)(p); ev.ID == 5 && ev.Reply == 0 {
			acknowledged.Store(true)
		}
		return p
	}
	f.a.renderFree = func(handle uintptr) {
		if !acknowledged.Load() || !consumed.Load() {
			t.Error("render context freed before pending-frame consumption and video-output acknowledgement")
			// The native fallback consumes the entire Close budget before the
			// remaining destructors. Keep the failing path finite for the test.
			time.Sleep(210 * time.Millisecond)
		}
		free(handle)
	}
	e := openFake(t, f)
	started := time.Now()
	_ = e.Close()
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Errorf("complete Close exceeded 200ms: %s", elapsed)
	}
	if Owned() != (Resources{}) || e.Snapshot().State != contract.Closed {
		t.Fatal("Close returned with ownership", Owned(), e.Snapshot())
	}
}

func commandArgs(p unsafe.Pointer) []string {
	var args []string
	for i := 0; i < 16; i++ {
		value := *(**byte)(unsafe.Add(p, uintptr(i)*unsafe.Sizeof(p)))
		if value == nil {
			break
		}
		text, _ := nodeString(value)
		args = append(args, text)
	}
	return args
}

func TestShutdownFailureStillDisposesInOrder(t *testing.T) {
	for _, scenario := range []string{"rejected", "reply-error", "missing-reply", "shutdown-event", "overflow", "nil-event", "event-budget", "quit-rejected"} {
		t.Run(scenario, func(t *testing.T) {
			f := fake()
			command, wait := f.a.command, f.a.wait
			stopping := false // command and wait have the same control owner
			f.a.command = func(core uintptr, id uint64, p unsafe.Pointer) int32 {
				args := commandArgs(p)
				if args[0] == "quit" && scenario == "quit-rejected" {
					return -1
				}
				if len(args) == 3 && args[1] == "vid" {
					stopping = true
					switch scenario {
					case "rejected":
						return -1
					case "reply-error":
						f.events <- &event{ID: 5, Reply: id, Error: -1}
					case "missing-reply":
						f.events <- &event{ID: 5, Reply: id + 1}
					case "shutdown-event":
						f.events <- &event{ID: 1}
					case "overflow":
						f.events <- &event{ID: 24}
					case "quit-rejected":
						return command(core, id, p)
					}
					return 0
				}
				return command(core, id, p)
			}
			f.a.wait = func(core uintptr, timeout float64) unsafe.Pointer {
				if stopping {
					if scenario == "nil-event" {
						return nil
					}
					if scenario == "event-budget" {
						return unsafe.Pointer(&event{ID: 5, Reply: 999})
					}
				}
				return wait(core, timeout)
			}
			e := openFake(t, f)
			_ = e.Close()
			_ = e.Close()
			if e.failure != contract.Media || Owned() != (Resources{}) {
				t.Fatal("shutdown failure lost or resources remain", e.failure, Owned())
			}
			if !reflect.DeepEqual(f.order, []string{"render-free", "destroy", "unload"}) {
				t.Fatal("destructors skipped, repeated or reordered", f.order)
			}
		})
	}
}

func TestRenderFailureServicesShutdownWithoutTargetPixels(t *testing.T) {
	f := fake()
	var disabling, skipped atomic.Bool
	command := f.a.command
	f.a.command = func(core uintptr, id uint64, p unsafe.Pointer) int32 {
		args := commandArgs(p)
		if len(args) == 3 && args[1] == "vid" {
			disabling.Store(true)
			return 0 // Completion requires the render owner to consume a frame.
		}
		return command(core, id, p)
	}
	f.a.render = func(_ uintptr, p unsafe.Pointer) int32 {
		if (*renderParam)(p).Kind != 13 {
			return -1 // A failed target leaves the native frame unconsumed.
		}
		params := unsafe.Slice((*renderParam)(p), 3)
		if *(*int32)(params[0].Data) != 1 || params[1].Kind != 12 || *(*int32)(params[1].Data) != 0 || params[2].Kind != 0 {
			t.Error("shutdown skip must neither use a surface nor wait for presentation")
		}
		if disabling.Load() && skipped.CompareAndSwap(false, true) {
			f.events <- &event{ID: 5, Reply: 0}
		}
		return 0
	}
	free := f.a.renderFree
	f.a.renderFree = func(handle uintptr) {
		if !skipped.Load() {
			t.Error("failed renderer abandoned pending native frame")
		}
		free(handle)
	}
	e := openFake(t, f)
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("render failure did not finish disposal")
	}
	if e.Snapshot().State != contract.Failed || e.Snapshot().Error != contract.Media || Owned() != (Resources{}) {
		t.Fatal(e.Snapshot(), Owned())
	}
}

func TestContextCancellationJoinsVideoShutdown(t *testing.T) {
	f := fake()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plain, err := (Opener{loader: func(string) (*nativeAPI, error) { return f.a, nil }}).Open(ctx, contract.Input{FilePath: "synthetic.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	e := plain.(*engine)
	t.Cleanup(func() { _ = e.Close() })
	select {
	case <-f.sent:
	case <-time.After(time.Second):
		t.Fatal("load not queued")
	}
	cancel()
	select {
	case <-e.done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not join shutdown")
	}
	if e.failure != contract.NoError || Owned() != (Resources{}) || !reflect.DeepEqual(f.order, []string{"render-free", "destroy", "unload"}) {
		t.Fatal(e.failure, Owned(), f.order)
	}
}
