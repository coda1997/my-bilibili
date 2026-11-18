// Package history owns device-local progress and checkpoint intent.
package history

import (
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

type Code string

const (
	None               Code = ""
	Invalid            Code = "invalid-history"
	Storage            Code = "history-storage"
	InUse              Code = "history-in-use"
	LockUnsupported    Code = "history-lock-unsupported"
	Corrupt            Code = "history-corrupt"
	UnsupportedVersion Code = "history-version"
	Uncertain          Code = "history-durability-uncertain"
	Closed             Code = "history-closed"
)

func (c Code) Error() string { return string(c) }

type Record struct {
	Ref                catalog.PartRef
	Title, PartTitle   string
	Position, Duration time.Duration
	UpdatedAt          time.Time
	Completed          bool
}

// Store owns one exclusive filesystem lifetime. Acquire precedes Load and
// mutation; Close releases ownership only after the final save and cleanup.
type Store interface {
	Acquire() error
	Load() ([]Record, error)
	Save([]Record) error
	Close() error
}

type Sample struct {
	Visit    uint64
	Record   Record
	Playback playback.State
}

type Phase string

const (
	Initial Phase = "initial"
	Loading Phase = "loading"
	Ready   Phase = "ready"
	Failed  Phase = "failed"
	Closing Phase = "closing"
	Done    Phase = "closed"
)

type State struct {
	Phase                   Phase
	Records                 []Record
	Revision, SavedRevision uint64
	Busy, Dirty             bool
	Error                   Code
}
