package main

import (
	"testing"
	"time"
)

func TestSourceCloseCompletionDoesNotNeedPaint(t *testing.T) {
	done := make(chan struct{})
	posted := make(chan struct{}, 2)
	var observer sourceCloseObserver
	post := func() { posted <- struct{}{} }
	if observer.ready(done, post) {
		t.Fatal("window disposed before source ownership ended")
	}
	first := observer.finished
	if observer.ready(done, post) || observer.finished != first {
		t.Fatal("repeated close replaced the owned completion observer")
	}
	select {
	case <-posted:
		t.Fatal("close dispatched before source completion")
	default:
	}
	close(done)
	// No render, timer tick or window invalidation is supplied to the observer.
	select {
	case <-observer.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("source completion did not dispatch an owned close")
	}
	observer.wait()
	if len(posted) != 1 {
		t.Fatal("repeated close created duplicate completion observers")
	}
	if !observer.ready(done, post) || len(posted) != 1 {
		t.Fatal("finished source did not allow idempotent native close")
	}
}

func TestSourceCloseAlreadyFinishedNeedsNoObserver(t *testing.T) {
	done := make(chan struct{})
	close(done)
	var observer sourceCloseObserver
	if !observer.ready(done, func() { t.Error("unneeded close dispatch") }) || observer.finished != nil {
		t.Fatal("already-finished ownership created a worker")
	}
	observer.wait()
}
