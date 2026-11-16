// Package app owns the preview's application actions and observable state.
package app

import (
	"context"
	"time"

	"my-bilibili/experiments/playback/contract"
	"my-bilibili/internal/catalog"
)

// Source is the synchronous catalog/media boundary consumed by the application.
// Implementations own resource validation and conversion to playback inputs.
type Source interface {
	Detail() (catalog.Video, error)
	Resolve(catalog.PartID) (contract.Input, error)
}

// State separates the requested part from the accepted playback attempt.
// Volume and Rate are chosen settings; Playback contains native observations.
type State struct {
	Video                      catalog.Video
	SelectedPart, PlaybackPart catalog.PartID
	SelectionError             contract.ErrorCode
	Playback                   contract.Snapshot
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
