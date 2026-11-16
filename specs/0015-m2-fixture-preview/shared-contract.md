# Frozen M2 application boundary

These declarations are implemented by M2-001 before workers split. Catalog and
app imports remain standard library plus the existing plain playback contract.
No exported type carries a mygo value, native handle, resource locator or DTO.

```go
// package catalog
type VideoID string
type PartID string
type Part struct {
    ID PartID
    Title string
    Duration time.Duration
}
type Video struct {
    ID VideoID
    Title, Creator, Description string
    CoverID string
    Parts []Part
}

// package app
type Source interface {
    Detail() (catalog.Video, error)
    Resolve(catalog.PartID) (contract.Input, error)
}
type State struct {
    Video catalog.Video
    SelectedPart, PlaybackPart catalog.PartID
    SelectionError contract.ErrorCode
    Playback contract.Snapshot
    DesiredPlaying bool
    Volume, Rate float64
}
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
```

## Construction and source ownership

M2-002 implements `app.New(source Source, controller *contract.Controller)
(*Session, error)`, with `*Session` satisfying Player. Source/detail validation
occurs once during construction: nonempty unique IDs, nonempty human labels,
positive duration and at least one part. The fixture adapter owns filesystem
validation and contract input conversion. Construction does not acquire a
playback session. State returns independent copies of mutable detail slices.

`Start` accepts a nonnil parent context, stores it for later opens, and opens P1
paused exactly once. Repeated Start is a no-op while the session remains open;
nil context is Invalid. Close is terminal and idempotent; subsequent actions
return IsClosed. Context cancellation stops playback through Controller.Tick.
No app goroutines or asynchronous fixture resolver are introduced.

Source methods return safe contract errors where possible; unknown errors are
mapped to Media before reaching view/state/log output. Unknown part IDs are
Invalid. The fixture adapter uses fixed IDs `preview-video`, `part-1`, `part-2`
and cover ID `preview-cover`; it never derives filesystem paths from user IDs.
`fixture.New(resourceDir string) (app.Source, error)` validates the known assets.
The fixture package owns original media, cover, metadata and integrity manifest.

## Identity and playback intent

SelectedPart records the most recently requested known part. Before replacing
playback, synchronously Resolve and validate the input. On failure record only
SelectionError and preserve existing playback/input/identity. An unknown ID
does not replace SelectedPart. Compare Controller generation before/after Open:
PlaybackPart changes exactly when generation advances, including accepted opens
that fail immediately. Failed validation before generation advance retains the
previous PlaybackPart. The UI uses PlaybackPart for the actual-player label.

Store distinct selection-retry target and last accepted playback input/identity.
RetrySelection repeats the failed selection with its original playback intent;
RetryPlayback reopens the last accepted playback input at zero, preserving its
latest requested play/pause intent. They must not exchange targets after a failed
selection. RetrySelection without a pending failure returns Invalid. Retrying
current loading playback is a no-op. Current paused part resumes; current
playing part is a no-op. Clicking the current loading part sets its current
generation's play intent to true without reopening; when already pending play,
the click is a no-op. Current Ended part and Play/Toggle at Ended
reopen the current accepted input at zero with play intent. Current Failed part
selection invokes playback retry with play intent. NextPart selects the next
catalog part; it is a no-op on the last part, where the UI disables the action.

Default desired rate/volume are 1/1. They persist across opens; mute restores
the last positive chosen volume (initially 1). Zero volume is muted. Source
snapshots reflect native observations separately from chosen Volume/Rate.
SetVolume/SetRate validate with the existing Command contract. During Loading,
store settings/intent without issuing transport commands. On first readiness
for the current generation, apply volume/rate and optionally Play once. Do not
send those initial commands on every poll. Later controls use Controller.Command;
any failure clears obsolete intent and preserves sanitized playback failure.
Seek is allowed only in Ready/Paused/Playing with known positive duration; clamp
to duration via Controller. Invalid or unusable seek returns Invalid without
turning a healthy session into Failed. Completion clears playback intent; no
automatic next-part advance. Replacement, failure and close clear old-generation
initialization/intent. UI sampling never writes commands back to the engine.

## Native presentation

M2-003 adds `mpvprobe.DrawSized(c *gui.Context, plain contract.Engine,
displayWidth, displayHeight float32)`. The existing Draw delegates using 640x360.
Retain the private concrete Engine and current TryLock, bitmap and frame ownership.
Only display dimensions change; render buffer size and native teardown do not.
`Session.Engine()` returns the original Controller engine to composition only
for drawing/inspection. It is not part of the UI-facing Player interface.
App state, lifetime and controller belong outside layout reconstruction.

## Worker integration

M2-003 implements `desktop.View` with Player, a
`Surface func(*gui.Context, float32, float32)` drawing callback,
a `Cover func(*gui.Context, string)` callback, and injected fullscreen getter/
setter callbacks owned by composition. `Render(*gui.Context)` handles view-only
state and delegates use cases to Player. A supplied clock makes overlay/double
click behavior deterministic in tests. The view does not load media/runtime or
access the engine. Composition captures Session.Engine in the Surface closure.

M2-004 wires a Session, native drawing, cover/resource loading and native window
callbacks. Startup errors are developer-readable but do not print media inputs.
Window close calls Player.Close synchronously, then verifies mpvprobe.Owned is
zero before quitting. An e2e-only action disposes playback while keeping the
process alive for eight-second post-close observation; test that separately
from actual window-close/process-exit. Preserve Controller.PollInterval (50ms), its 10-second
readiness limit, complete native disposal and the 200ms external cancellation
bound. Runtime recovery itself is an ignored developer prerequisite.
