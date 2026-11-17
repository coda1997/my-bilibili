// Package danmaku converts fixed Bilibili metadata and ordinary segments.
package danmaku

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/catalog"
	model "my-bilibili/internal/danmaku"
)

type Source struct {
	scope *transport.Scope
	video catalog.VideoID
	parts map[catalog.PartID]time.Duration
}

var _ model.SegmentSource = (*Source)(nil)

// New is pure. The supplied scope and copied detail membership remain fixed
// for this source; it never binds a later account revision.
func New(scope *transport.Scope, video catalog.Video) (*Source, error) {
	if scope == nil || scope.Context() == nil || catalog.ValidateVideo(video) != nil {
		return nil, catalog.Invalid
	}
	if scope.State().Closed {
		return nil, catalog.Canceled
	}
	s := &Source{scope: scope, video: video.ID, parts: make(map[catalog.PartID]time.Duration, len(video.Parts))}
	for _, part := range video.Parts {
		value, err := strconv.ParseUint(string(part.ID), 10, 63)
		if err != nil || value == 0 || strconv.FormatUint(value, 10) != string(part.ID) {
			return nil, catalog.Invalid
		}
		s.parts[part.ID] = part.Duration
	}
	return s, nil
}

func (s *Source) check(ctx context.Context, ref catalog.PartRef) (time.Duration, error) {
	if ctx == nil {
		return 0, catalog.Invalid
	}
	if err := currentContext(ctx); err != nil {
		return 0, err
	}
	if s.scope.Context().Err() != nil || s.scope.State().Closed {
		return 0, catalog.Canceled
	}
	duration, ok := s.parts[ref.Part]
	if ref.Video != s.video || !ok {
		return 0, catalog.Invalid
	}
	return duration, nil
}

func currentContext(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return catalog.Timeout
	}
	if ctx.Err() != nil {
		return catalog.Canceled
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return catalog.Timeout
	}
	return nil
}

func (s *Source) Metadata(ctx context.Context, ref catalog.PartRef) (model.Metadata, error) {
	if _, err := s.check(ctx, ref); err != nil {
		return model.Metadata{}, err
	}
	if !s.scope.State().Authenticated {
		// Unknown is distinct from a known empty or disabled corpus.
		if _, err := s.check(ctx, ref); err != nil {
			return model.Metadata{}, err
		}
		return model.Metadata{Known: false}, nil
	}
	body, err := s.scope.Protobuf(ctx, transport.DanmakuView, url.Values{"oid": {string(ref.Part)}})
	if err != nil {
		return model.Metadata{}, err
	}
	metadata, err := decodeMetadata(body)
	if _, cause := s.check(ctx, ref); cause != nil {
		return model.Metadata{}, cause
	}
	return metadata, err
}

func (s *Source) Segment(ctx context.Context, ref catalog.PartRef, index int) (model.Segment, error) {
	duration, err := s.check(ctx, ref)
	if err != nil {
		return model.Segment{}, err
	}
	count := int((duration-1)/model.SegmentDuration) + 1
	if index < 1 || index > count {
		return model.Segment{}, catalog.Invalid
	}
	body, err := s.scope.Protobuf(ctx, transport.DanmakuSegment, url.Values{
		"oid": {string(ref.Part)}, "segment_index": {strconv.Itoa(index)},
	})
	if err != nil {
		return model.Segment{}, err
	}
	segment, err := decodeSegment(body, index, duration)
	if _, cause := s.check(ctx, ref); cause != nil {
		return model.Segment{}, cause
	}
	return segment, err
}
