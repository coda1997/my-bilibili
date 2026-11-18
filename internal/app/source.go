package app

import (
	"context"
	"maps"
	"time"

	"my-bilibili/internal/catalog"
)

const sourceTimeout = 10 * time.Second

type sourcePurpose uint8

const (
	detailRequest sourcePurpose = iota
	selectionRequest
	playbackRequest
	qualityRequest
)

// Requests are immutable after submission. The UI-owned attempt holds mutable
// intent separately, so a worker never reads application state.
type sourceRequest struct {
	generation uint64
	purpose    sourcePurpose
	part       catalog.PartID
	quality    QualityID
	ctx        context.Context
}

type sourceAttempt struct {
	request  sourceRequest
	cancel   context.CancelFunc
	intent   bool
	startAt  time.Duration
	newWatch bool
}

type sourceResult struct {
	request    sourceRequest
	video      catalog.Video
	resolution Resolution
	err        error
	consumed   chan struct{}
}

func cloneVideo(video catalog.Video) catalog.Video {
	// An empty catalog is represented by an empty array, never a nil slice.
	parts := make([]catalog.Part, len(video.Parts))
	copy(parts, video.Parts)
	video.Parts = parts
	return video
}

func cloneQualities(qualities []Quality) []Quality {
	owned := make([]Quality, len(qualities))
	copy(owned, qualities)
	return owned
}

func validateQualities(resolution Resolution) error {
	if len(resolution.Qualities) > 16 {
		return catalog.InvalidResponse
	}
	seen := make(map[QualityID]bool, len(resolution.Qualities))
	chosen := false
	for _, quality := range resolution.Qualities {
		if quality.ID == "" || seen[quality.ID] {
			return catalog.InvalidResponse
		}
		seen[quality.ID] = true
		chosen = chosen || quality == resolution.Quality
	}
	if !chosen {
		return catalog.InvalidResponse
	}
	return nil
}

// Deadline expiry must not depend on when the context's timer goroutine gets
// scheduled. This clock is separate from Session.now's media-expiry clock.
func sourceContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func runSource(ctx context.Context, source Source, requests <-chan sourceRequest, results chan<- sourceResult, done chan<- struct{}) {
	defer close(done)
	for {
		var request sourceRequest
		select {
		case <-ctx.Done():
			return
		case request = <-requests:
		}
		if sourceContextError(ctx) != nil {
			return
		}
		if sourceContextError(request.ctx) != nil {
			continue
		}
		result := sourceResult{request: request, consumed: make(chan struct{})}
		if request.purpose == detailRequest {
			result.video, result.err = source.Detail(request.ctx)
			result.video = cloneVideo(result.video)
		} else {
			result.resolution, result.err = source.Resolve(request.ctx, ResolveRequest{Part: request.part, Quality: request.quality})
			result.resolution.Input.Headers = maps.Clone(result.resolution.Input.Headers)
			result.resolution.Qualities = cloneQualities(result.resolution.Qualities)
		}
		select {
		case <-ctx.Done():
			return
		case results <- result:
		}
		// Do not finish another call while its predecessor's completed result
		// is still buffered. This bounds completed results as well as work.
		select {
		case <-ctx.Done():
			return
		case <-result.consumed:
		}
	}
}
