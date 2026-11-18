package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestClientPumpBoundsQueuedUpdatesAndRetiresWithoutPaint(t *testing.T) {
	queued := make(chan func(), 16)
	var ticks atomic.Int32
	pump := startClientPump(func(fn func()) { queued <- fn }, func() { ticks.Add(1) }, time.Millisecond)
	defer pump.Stop()
	var first func()
	select {
	case first = <-queued:
	case <-time.After(time.Second):
		t.Fatal("no UI update")
	}
	time.Sleep(15 * time.Millisecond)
	if len(queued) != 0 || ticks.Load() != 0 {
		t.Fatal("unacknowledged UI work grew or ran off UI")
	}
	first()
	if ticks.Load() != 1 {
		t.Fatal("update failed to pump without rendering")
	}
	var stale func()
	select {
	case stale = <-queued:
	case <-time.After(time.Second):
		t.Fatal("no next UI update")
	}
	pump.Stop()
	select {
	case <-pump.Done():
	case <-time.After(time.Second):
		t.Fatal("stop waited on a paint or queued callback")
	}
	stale()
	if ticks.Load() != 1 {
		t.Fatal("retained callback ran domain work after pump stop")
	}
	if len(queued) != 0 {
		t.Fatal("another update survived stop")
	}
}
