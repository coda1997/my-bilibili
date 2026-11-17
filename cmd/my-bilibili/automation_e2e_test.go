//go:build e2e

package main

import (
	"context"
	"my-bilibili/internal/fixture"
	"net/http"
	"testing"
	"time"
)

func TestControlledDelayedRequestIsObservedAndCanceled(t *testing.T) {
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", "delayed")
	source, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	got, cleanup, err := automationSource(source)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	delayed := got.(*delayedSource)
	r, err := got.Resolve(context.Background(), "part-1")
	in := r.Input
	if err != nil || in.Validate() != nil {
		t.Fatal(err)
	}
	if _, err := got.Resolve(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown ID accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		res, err := http.DefaultClient.Do(req)
		if res != nil {
			res.Body.Close()
		}
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for delayed.active.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if delayed.requests.Load() != 1 || delayed.active.Load() != 1 {
		t.Fatal("real request not observed")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("delayed response unexpectedly completed")
	}
	for delayed.active.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if delayed.active.Load() != 0 {
		t.Fatal("delayed handler survived request cancellation")
	}
}
