package danmaku

import (
	"context"

	"my-bilibili/internal/catalog"
)

type SegmentEntry struct {
	ID    string
	Entry Entry
}

type Metadata struct {
	Known, Available bool
}

type Segment struct {
	Index                              int
	Entries                            []SegmentEntry
	Encoded, SkippedEmpty, Unsupported int
}

type SegmentSource interface {
	Metadata(context.Context, catalog.PartRef) (Metadata, error)
	Segment(context.Context, catalog.PartRef, int) (Segment, error)
}

type LoadPhase string

const (
	LoadInitial LoadPhase = "initial"
	LoadLoading LoadPhase = "loading"
	LoadReady   LoadPhase = "ready"
	LoadFailed  LoadPhase = "failed"
	LoadClosed  LoadPhase = "closed"
)

type LoadState struct {
	Phase                                                       LoadPhase
	Ref                                                         catalog.PartRef
	Generation, Revision                                        uint64
	Segment, CachedSegments, Entries, TextBytes, ActiveRequests int
	Error, PrefetchError                                        catalog.Code
}

type Window struct {
	Generation, Revision uint64
	Entries              []Entry
}
