//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

// controlledSource delays or fails only the hash-verified fixture Source. Its
// gate and counters never exist in a normal build and accept no media input.
type controlledSource struct {
	app.Source
	mode            string
	gate            chan struct{}
	releaseOnce     sync.Once
	mu              sync.Mutex
	observed        previewbundle.SourceObservation
	part2Attempts   int64
	closeOnMinimize bool // UI-thread-only test action
}

func (s *controlledSource) begin(detail bool, part catalog.PartID) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	if detail {
		s.observed.DetailRequests++
		count = s.observed.DetailRequests
	} else {
		s.observed.ResolveRequests++
		if part == "part-2" {
			s.part2Attempts++
		}
		count = s.part2Attempts
	}
	s.observed.Active++
	s.observed.PeakActive = max(s.observed.PeakActive, s.observed.Active)
	return count
}

func (s *controlledSource) finish() {
	s.mu.Lock()
	s.observed.Active--
	s.mu.Unlock()
}

func (s *controlledSource) snapshot() previewbundle.SourceObservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.observed
}

func (s *controlledSource) release() { s.releaseOnce.Do(func() { close(s.gate) }) }

func (s *controlledSource) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Keep cancellation observably asynchronous so native close must await
		// real worker completion, including while the window cannot paint.
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		<-timer.C
		return ctx.Err()
	case <-s.gate:
		return ctx.Err()
	}
}

func (s *controlledSource) Detail(ctx context.Context) (catalog.Video, error) {
	call := s.begin(true, "")
	defer s.finish()
	video, err := s.Source.Detail(ctx)
	if err != nil {
		return catalog.Video{}, err
	}
	if s.mode == "source-detail-failure" && call == 1 {
		return catalog.Video{}, playback.Media
	}
	if s.mode == "source-detail" {
		if err := s.wait(ctx); err != nil {
			return catalog.Video{}, err
		}
	}
	return video, nil
}

func (s *controlledSource) Resolve(ctx context.Context, request app.ResolveRequest) (app.Resolution, error) {
	call := s.begin(false, request.Part)
	defer s.finish()
	resolution, err := s.Source.Resolve(ctx, request)
	if err != nil {
		return app.Resolution{}, err
	}
	if request.Part == "part-2" {
		if s.mode == "source-resolve-failure" && call == 1 {
			return app.Resolution{}, playback.Media
		}
		if s.mode == "source-resolve" || s.mode == "source-timeout" {
			if err := s.wait(ctx); err != nil {
				return app.Resolution{}, err
			}
		}
	}
	return resolution, nil
}

func (a *application) automationSetup() {
	controlled, ok := a.source.(*controlledSource)
	if !ok {
		return
	}
	a.window.OnMinimize(func() {
		if controlled.closeOnMinimize && a.window.IsMinimized() {
			controlled.closeOnMinimize = false
			a.sourceReceipt(os.Stdout, previewbundle.SourceWindowMinimized, a.automationObservation(a.window.IsFullScreen(), true))
			a.window.Close()
		}
	})
}

func (a *application) automationClosed() {
	if _, ok := a.source.(*controlledSource); ok {
		// The native window is already destroyed. Read only owned Go state and
		// counters here; do not query native window properties after disposal.
		a.sourceReceipt(os.Stdout, previewbundle.SourceWindowClosed, a.automationObservation(false, false))
	}
}

func (a *application) sourceReceipt(out io.Writer, event string, observation previewbundle.Observation) {
	err := json.NewEncoder(out).Encode(previewbundle.SourceReceipt{Event: event, Observation: observation})
	if err != nil && a.closeError == nil {
		a.closeError = playback.Media
	}
}
