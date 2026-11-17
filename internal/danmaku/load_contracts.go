package danmaku

import (
	"context"
	"time"

	"my-bilibili/internal/catalog"
)

// These bounds belong to the plain segment port, independently of HTTP body
// sizes. Two retained segments fit the established timeline window.
const (
	SegmentDuration     = 6 * time.Minute
	MaxSegmentEntries   = 20000
	MaxSegmentTextBytes = 4 << 20
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

// SegmentSource is called serially on the loader's worker. Methods honor their
// context, transfer owned results and do no native/UI work. IDs are stable
// presentation identities, never sender identities or raw transport objects.
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
