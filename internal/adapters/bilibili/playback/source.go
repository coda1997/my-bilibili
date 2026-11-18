// Package playback resolves one validated catalog video through scoped web DASH.
package playback

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	catalogapi "my-bilibili/internal/adapters/bilibili/catalog"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/app"
	"my-bilibili/internal/catalog"
)

const operationLimit = 10 * time.Second
const reuseLimit = 5 * time.Minute

// Source's methods are serialized by Session's existing source worker. It owns
// only copied catalog membership; callers own requests and returned resolutions.
// It retains no media URL or quality cache and performs no native/UI operation.
type Source struct {
	scope   *transport.Scope
	details *catalogapi.Client
	entry   catalog.Entry
	video   catalog.Video
	loaded  bool
	now     func() time.Time
}

var _ app.Source = (*Source)(nil)

func NewSource(scope *transport.Scope, details *catalogapi.Client, entry catalog.Entry) (*Source, error) {
	if !details.MatchesScope(scope) {
		return nil, catalog.Invalid
	}
	text := entry.ID
	switch entry.Kind {
	case catalog.BVEntry:
	case catalog.AVEntry:
		text = "av" + text
	default:
		return nil, catalog.Invalid
	}
	parsed, err := catalog.ParseEntry(text)
	if err != nil || parsed.Kind != entry.Kind || parsed.ID != entry.ID || entry.Page < 0 || entry.Page > 1000 {
		return nil, catalog.Invalid
	}
	return &Source{scope: scope, details: details, entry: entry, now: time.Now}, nil
}
func (Source) Format(s fmt.State, _ rune)   { _, _ = io.WriteString(s, "[private media source]") }
func (Source) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

func (s *Source) check(ctx context.Context) error {
	if ctx == nil {
		return catalog.Invalid
	}
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return catalog.Timeout
		}
		return catalog.Canceled
	}
	if end, ok := ctx.Deadline(); ok && !time.Now().Before(end) {
		return catalog.Timeout
	}
	if !s.details.MatchesScope(s.scope) {
		return catalog.Canceled
	}
	return nil
}
func copyVideo(video catalog.Video) catalog.Video {
	video.Parts = append([]catalog.Part(nil), video.Parts...)
	return video
}

func (s *Source) Detail(parent context.Context) (catalog.Video, error) {
	if err := s.check(parent); err != nil {
		return catalog.Video{}, err
	}
	ctx, cancel := context.WithTimeout(parent, operationLimit)
	defer cancel()
	if s.loaded {
		return copyVideo(s.video), nil
	}
	video, err := s.details.Detail(ctx, s.entry)
	if err != nil {
		return catalog.Video{}, err
	}
	if err := s.check(ctx); err != nil {
		return catalog.Video{}, err
	}
	// The concrete catalog adapter establishes canonical identity, supported
	// content and ValidateVideo invariants before transferring this owned value.
	s.video, s.loaded = video, true
	return copyVideo(video), nil
}

func (s *Source) Resolve(parent context.Context, request app.ResolveRequest) (app.Resolution, error) {
	if err := s.check(parent); err != nil {
		return app.Resolution{}, err
	}
	ctx, cancel := context.WithTimeout(parent, operationLimit)
	defer cancel()
	member := false
	if s.loaded {
		for _, part := range s.video.Parts {
			if part.ID == request.Part {
				member = true
				break
			}
		}
	}
	if !member {
		return app.Resolution{}, catalog.Invalid
	}
	quality := int64(64)
	if request.Quality != "" {
		var ok bool
		quality, ok = qualityCode(request.Quality)
		if !ok {
			return app.Resolution{}, catalog.QualityUnavailable
		}
	}
	body, err := s.scope.JSON(ctx, transport.PlayURL, url.Values{
		"bvid": {string(s.video.ID)}, "cid": {string(request.Part)},
		"fnver": {"0"}, "fnval": {"16"}, "qn": {strconv.FormatInt(quality, 10)},
	})
	if err != nil {
		return app.Resolution{}, err
	}
	received := s.now()
	resolution, err := decodeMedia(body, request.Quality)
	if err != nil {
		return app.Resolution{}, err
	}
	if err := s.check(ctx); err != nil {
		return app.Resolution{}, err
	}
	resolution.ExpiresAt = received.Add(reuseLimit)
	resolution.RefreshOnRetry = true
	return resolution, nil
}
