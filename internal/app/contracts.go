// Package app owns the preview's application actions and observable state.
package app

import (
	"context"
	"time"

	"my-bilibili/internal/catalog"
	"my-bilibili/internal/playback"
)

// Source is the catalog/media boundary consumed by the application.
// Implementations own resource validation, transport details and conversion to
// playback inputs, including any required request headers in Input.Headers.
//
// Calls receive the session context and must return promptly once it is
// canceled. Session currently invokes Source synchronously on the UI thread, so
// an implementation that performs network I/O must not be wired in until
// resolution moves off the UI thread with stale-result handling.
type Source interface {
	Detail(context.Context) (catalog.Video, error)
	Resolve(context.Context, catalog.PartID) (Resolution, error)
}

// Resolution is a playable input for one part and the time it stops being
// usable. A zero ExpiresAt means the input does not expire (for example a
// local file). Session never reuses an input at or past ExpiresAt minus
// ResolutionMargin; it resolves the part again instead.
type Resolution struct {
	Input     playback.Input
	ExpiresAt time.Time
}

// ResolutionMargin keeps an input that is about to expire from being opened,
// so a connection is not started with a URL that expires during loading.
const ResolutionMargin = 30 * time.Second

// State separates the requested part from the accepted playback attempt.
// Volume and Rate are chosen settings; Playback contains native observations.
type State struct {
	Video                      catalog.Video
	SelectedPart, PlaybackPart catalog.PartID
	SelectionError             playback.ErrorCode
	Playback                   playback.Snapshot
	DesiredPlaying             bool
	Volume, Rate               float64
}

// Player is the UI-facing application boundary. All calls belong to the UI
// thread. State returns a copy of mutable detail data. Native engine access is
// reserved for composition and is deliberately absent from this interface.
type Player interface {
	State() State
	Start(context.Context) error
	SelectPart(catalog.PartID) error
	NextPart() error
	RetrySelection() error
	RetryPlayback() error
	Play() error
	Pause() error
	Toggle() error
	Seek(time.Duration) error
	SetVolume(float64) error
	SetRate(float64) error
	ToggleMute() error
	Tick()
	Close() error
}
