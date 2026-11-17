// Package danmaku owns bounded, deterministic presentation of timed comments.
// Its methods belong to one owner; playback position is its only timebase.
package danmaku

import "time"

type Mode uint8

const (
	ScrollRTL   Mode = 1
	FixedBottom Mode = 4
	FixedTop    Mode = 5
	ScrollLTR   Mode = 6
)

const (
	MaxEntries     = 40000
	MaxTextBytes   = 1024
	MaxWindowBytes = 8 << 20
	MaxLanes       = 16
)

type ErrorCode string

const (
	NoError     ErrorCode = ""
	Invalid     ErrorCode = "invalid-danmaku"
	Stale       ErrorCode = "stale-generation"
	IsClosed    ErrorCode = "danmaku-closed"
	Unavailable ErrorCode = "danmaku-unavailable"
)

func (e ErrorCode) Error() string { return string(e) }

type Entry struct {
	At    time.Duration
	Mode  Mode
	Text  string
	Color uint32
}

type Settings struct {
	Visible                  bool
	Opacity, FontSize, Speed float64
}

func DefaultSettings() Settings {
	return Settings{Visible: true, Opacity: .9, FontSize: 24, Speed: 1}
}

type Stats struct{ Input, Loaded, SkippedEmpty, Overflow int }

type Item struct {
	Index         int
	Mode          Mode
	Text          string
	Color         uint32
	Lane          int
	Top, Progress float64
}

type Frame struct {
	Generation, Revision      uint64
	Position                  time.Duration
	Width, Height, LineHeight float64
	Lanes                     int
	Settings                  Settings
	Stats                     Stats
	Items                     []Item
	Closed                    bool
}
