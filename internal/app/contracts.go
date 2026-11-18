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
// Session invokes at most one method at a time on its owned source worker.
// Calls receive a bounded context derived from the session lifetime and must
// return promptly once it is canceled. Implementations transfer ownership of
// returned mutable data and release resources opened by the call before return.
// No native playback or UI work belongs in a Source method.
type Source interface {
	Detail(context.Context) (catalog.Video, error)
	Resolve(context.Context, ResolveRequest) (Resolution, error)
}

// QualityID is an opaque returned choice. An empty request selects a default.
type QualityID string

type Quality struct {
	ID            QualityID
	Label         string
	Width, Height int
}

type ResolveRequest struct {
	Part    catalog.PartID
	Quality QualityID
}

// Resolution is a playable input for one part and the time it stops being
// usable. A zero ExpiresAt means the input does not expire (for example a
// local file). Session never reuses an input at or past ExpiresAt minus
// ResolutionMargin; it resolves the part again instead.
type Resolution struct {
	Input          playback.Input
	ExpiresAt      time.Time
	Quality        Quality
	Qualities      []Quality
	RefreshOnRetry bool
}

type Options struct {
	InputPolicy playback.InputPolicy
	InitialPart catalog.PartID
	InitialPage int
	StartAt     time.Duration
	Autoplay    bool
}

// ResolutionMargin keeps an input that is about to expire from being opened,
// so a connection is not started with a URL that expires during loading.
const ResolutionMargin = 30 * time.Second

// DetailPhase describes source metadata loading independently of native playback.
type DetailPhase string

const (
	DetailInitial DetailPhase = "initial"
	DetailLoading DetailPhase = "loading"
	DetailReady   DetailPhase = "ready"
	DetailFailed  DetailPhase = "failed"
)

// State separates the requested part from the accepted playback attempt.
// Volume and Rate are chosen settings; Playback contains native observations.
type State struct {
	DetailPhase                      DetailPhase
	DetailError                      catalog.Code
	Video                            catalog.Video
	SelectedPart, PlaybackPart       catalog.PartID
	ResolvingPart                    catalog.PartID
	SelectionError                   catalog.Code
	PlaybackRetryError               catalog.Code
	SelectedQuality, PlaybackQuality QualityID
	Qualities                        []Quality
	Resuming                         bool
	ResumeError                      catalog.Code
	WatchSequence                    uint64
	Playback                         playback.Snapshot
	DesiredPlaying                   bool
	Volume, Rate                     float64
	Closing                          bool
}

// Player is the UI-facing application boundary. All calls belong to the UI
// thread. State returns a copy of mutable detail data. Native engine access is
// reserved for composition and is deliberately absent from this interface.
type Player interface {
	State() State
	Start(context.Context) error
	RetryDetail() error
	SelectPart(catalog.PartID) error
	SelectQuality(QualityID) error
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
	// Close cancels source work and disposes native playback without waiting
	// for source completion on the UI thread. Composition joins Session.Done.
	Close() error
}
