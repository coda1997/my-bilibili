//go:build e2e

package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"my-bilibili/internal/fixture"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func sourceScenario(t *testing.T, mode string) *controlledSource {
	t.Helper()
	t.Setenv("MY_BILIBILI_E2E_SCENARIO", mode)
	fixture, err := fixture.New("../../internal/fixture/assets")
	if err != nil {
		t.Fatal(err)
	}
	got, cleanup, err := automationSource(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	source, ok := got.(*controlledSource)
	if !ok {
		t.Fatal("synthetic source mode was not isolated")
	}
	return source
}

func TestControlledDetailIsHeldUntilExplicitRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := sourceScenario(t, "source-detail")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result := make(chan error, 1)
		go func() { _, err := source.Detail(ctx); result <- err }()
		synctest.Wait()
		if o := source.snapshot(); o.DetailRequests != 1 || o.Active != 1 || o.PeakActive != 1 {
			t.Fatal("detail was not held as one owned call", o)
		}
		source.release()
		source.release()
		if err := <-result; err != nil || source.snapshot().Active != 0 {
			t.Fatal("release did not complete the owned call", err)
		}
	})
}

func TestControlledSourceCancellationRemainsOwnedUntilReturn(t *testing.T) {
	for _, detail := range []bool{true, false} {
		name := "resolve"
		if detail {
			name = "detail"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				source := sourceScenario(t, "source-"+name)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result := make(chan error, 1)
				go func() {
					if detail {
						_, err := source.Detail(ctx)
						result <- err
					} else {
						_, err := source.Resolve(ctx, "part-2")
						result <- err
					}
				}()
				synctest.Wait()
				if source.snapshot().Active != 1 {
					t.Fatal("no held call")
				}
				cancel()
				synctest.Wait()
				if source.snapshot().Active != 1 {
					t.Fatal("cancellation was reported as completed before return")
				}
				if err := <-result; !errors.Is(err, context.Canceled) || source.snapshot().Active != 0 {
					t.Fatal("call did not finish cancellation", err)
				}
			})
		})
	}
}

func TestControlledSourceDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := sourceScenario(t, "source-timeout")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		result := make(chan error, 1)
		go func() { _, err := source.Resolve(ctx, "part-2"); result <- err }()
		synctest.Wait()
		if source.snapshot().Active != 1 {
			t.Fatal("timeout source did not wait")
		}
		if err := <-result; !errors.Is(err, context.DeadlineExceeded) || source.snapshot().Active != 0 {
			t.Fatal("deadline did not end source ownership", err)
		}
	})
}

func TestControlledSourceFailuresRequireExplicitRetry(t *testing.T) {
	t.Run("detail", func(t *testing.T) {
		source := sourceScenario(t, "source-detail-failure")
		if _, err := source.Detail(context.Background()); err != playback.Media {
			t.Fatal(err)
		}
		if video, err := source.Detail(context.Background()); err != nil || video.ID != "preview-video" {
			t.Fatal("detail retry did not recover", err)
		}
		if o := source.snapshot(); o.DetailRequests != 2 || o.Active != 0 || o.PeakActive != 1 {
			t.Fatal(o)
		}
	})
	t.Run("resolution", func(t *testing.T) {
		source := sourceScenario(t, "source-resolve-failure")
		if _, err := source.Resolve(context.Background(), "part-1"); err != nil {
			t.Fatal(err)
		}
		if _, err := source.Resolve(context.Background(), "part-2"); err != playback.Media {
			t.Fatal(err)
		}
		if _, err := source.Resolve(context.Background(), "part-2"); err != nil {
			t.Fatal("part retry did not recover", err)
		}
		if o := source.snapshot(); o.ResolveRequests != 3 || o.Active != 0 || o.PeakActive != 1 {
			t.Fatal(o)
		}
	})
}

type failedSourceReceiptWriter struct{}

func (failedSourceReceiptWriter) Write([]byte) (int, error) {
	return 0, errors.New("private-writer-sentinel")
}

func TestSourceReceiptFailureDoesNotBecomeSuccessfulExit(t *testing.T) {
	a := &application{}
	a.sourceReceipt(failedSourceReceiptWriter{}, previewbundle.SourceWindowClosed, previewbundle.Observation{})
	if a.closeError != playback.Media {
		t.Fatal("receipt failure was hidden")
	}
	a.closeError = playback.Unsupported
	a.sourceReceipt(failedSourceReceiptWriter{}, previewbundle.SourceWindowClosed, previewbundle.Observation{})
	if a.closeError != playback.Unsupported {
		t.Fatal("receipt failure erased an earlier failure")
	}
}
