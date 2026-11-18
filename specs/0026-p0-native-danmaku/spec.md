# Native Chinese danmaku over fixture playback

Spec ID: `0026-p0-native-danmaku`
Status: complete proposal for independent architecture review, 2026-11-17.
Full lane. The owner requested completing P0. This document proposes one
bounded prerequisite; it does not approve itself or claim full P0 delivery.
Planning reference: `6e291e24b6fd9b13debccf452e0a6adfa1c468e7`, with spec 0025
integration still in progress. Before freeze, the controller records the final
accepted 0025 candidate as this slice's implementation base.

## User scenario and scope

In the existing two-part fixture app, Chinese comments appear over the actual
video. The user can hide them and change opacity, text size and scroll speed.
Pause freezes their positions; seek, playback-rate changes, part changes and
native fullscreen keep them tied to the video. A bad comment set leaves video
playback usable and presents a safe, non-blocking danmaku error.

Implement the pure timeline, a mygo retained-text view, original fixture data,
desktop settings, and candidate-bound macOS evidence. The completed 0025
implementation/review/native evidence is a prerequisite for edits to its shared
desktop, composition, fixture and smoke-driver paths. Do not overlap its writers.

No live request, service/protobuf parser, fetch worker, account/Keychain access,
history, media-host policy, playback adapter/lifecycle redesign, new dependency,
comment sending, CI/check-policy change or public release belongs to this slice.
The saved real account session is preserved. Real-service danmaku ingestion and
the complete discovery/playback/history flow require a later reviewed contract.

## Owning packages and signatures

`internal/danmaku` depends only on the standard library and the existing plain
`internal/playback.Snapshot`. It owns validation, copied normalized text, settings
and deterministic admission. `internal/desktop/danmaku` (package `danmakuview`)
owns mygo layouts and settings controls. Parent desktop owns mounting, popup
routing and z-order. Composition owns fixture selection and lifetime. No UI,
native handle, transport DTO, account ID or location enters the pure model.

The following is the complete shared public model contract; implementations may
add private helpers, not invent new exported ports to split work:

```go
type Mode uint8
const (
    ScrollRTL Mode = 1
    FixedBottom Mode = 4
    FixedTop Mode = 5
    ScrollLTR Mode = 6
)
const (
    MaxEntries = 40000
    MaxTextBytes = 1024
    MaxWindowBytes = 8 << 20
    MaxLanes = 16
)
type ErrorCode string
const (
    NoError ErrorCode = ""
    Invalid ErrorCode = "invalid-danmaku"
    Stale ErrorCode = "stale-generation"
    IsClosed ErrorCode = "danmaku-closed"
    Unavailable ErrorCode = "danmaku-unavailable"
)
func (e ErrorCode) Error() string
type Entry struct { At time.Duration; Mode Mode; Text string; Color uint32 }
type Settings struct { Visible bool; Opacity, FontSize, Speed float64 }
func DefaultSettings() Settings
type Stats struct { Input, Loaded, SkippedEmpty, Overflow int }
type Item struct {
    Index int
    Mode Mode
    Text string
    Color uint32
    Lane int
    Top, Progress float64
}
type Frame struct {
    Generation, Revision uint64
    Position time.Duration
    Width, Height, LineHeight float64
    Lanes int
    Settings Settings
    Stats Stats
    Items []Item
    Closed bool
}
type Timeline struct { /* private state */ }
func New() *Timeline
func (t *Timeline) Reset(generation uint64) error
func (t *Timeline) SetEntries(generation uint64, entries []Entry) error
func (t *Timeline) Configure(settings Settings) error
func (t *Timeline) Settings() Settings
func (t *Timeline) Frame(snapshot playback.Snapshot, width, height float64) (Frame, error)
func (t *Timeline) Close()
```

The renderer's public contract is:

```go
type ItemState struct {
    Index int
    Mode danmaku.Mode
    Lane int
    Progress, X, Y, Width, Height float64
}
type State struct {
    Generation, Revision uint64
    Position time.Duration
    Width, Height float64
    Lanes int
    Settings danmaku.Settings
    Stats danmaku.Stats
    Active, Layouts int
    Items []ItemState
    Error danmaku.ErrorCode
    Closed bool
}
type View struct { /* private timeline, current layouts and state */ }
func New() *View
func (v *View) Reset(generation uint64) error
func (v *View) SetEntries(generation uint64, entries []danmaku.Entry) error
func (v *View) SetUnavailable(generation uint64) error
func (v *View) Settings() danmaku.Settings
func (v *View) Configure(settings danmaku.Settings) error
func (v *View) Render(c *gui.Context, snapshot playback.Snapshot, width, height float32)
func (v *View) Controls(c *gui.Context, disabled bool) bool
func (v *View) State() State
func (v *View) Close()
```

`Controls` renders the contents of the parent's settings popover and returns
whether a setting changed. It does not own the popover or player commands.
`State` returns a copy including a nonnil copied `Items` slice, never text or
layout pointers. It describes the latest prepared frame, not an actual-paint
receipt. All model/view methods run on their single owner, the UI thread in
production. The pure package itself has no UI dependency.

## Validation, replacement and state

- `New` is pure, unbound generation 0, with visible=true, opacity=0.9, font
  size=24 DIPs and speed=1. Settings ranges are opacity [0,1], font size [16,40]
  and speed [0.5,2], all finite. Invalid settings preserve the previous values.
- `Reset` requires a positive generation. Older generations return `Stale`; an
  equal generation is an idempotent no-op. A newer generation clears entries,
  schedules and statistics, preserves settings, and advances content Revision.
  `SetEntries` requires the exact positive installed generation. It replaces a
  whole window transactionally, never appends; success advances Revision even
  if equal contents are reinstalled. Invalid/stale calls preserve prior data.
- Validate at this model input boundary: input count <=40000, each raw string
  <=1024 UTF-8 bytes, aggregate raw text <=8 MiB, valid UTF-8, supported mode,
  color <=0xffffff, and At in [0, MaxInt64 nanoseconds minus 16 seconds]. This
  upper bound solely prevents lifetime addition overflow. Check caps before
  copying. No field, even on an empty-text entry, bypasses validation.
- Collapse runs of `unicode.IsSpace` characters to one ASCII space and trim
  leading/trailing whitespace. This handles CR/LF/tab and Unicode line/paragraph
  separators. Reject remaining `unicode.IsControl` runes. Other valid Unicode,
  including combining characters and joiners, remains text. Do not interpret
  markup, URLs, escapes or commands. Empty normalized text is skipped and
  counted. An empty/nil input is a valid installed no-comments window.
- Own copies of the slice and normalized strings (do not retain a substring of
  a large transport buffer). Sort by At, then original input index. Index is
  the original zero-based index, including skipped positions. Repeated content
  at separate input positions remains separate entries. Reinstalling a window
  cannot accumulate duplicates. Only input/loaded/skipped counts are stored at
  load time; overflow is a schedule result, never a growing per-frame counter.
- `Frame` accepts finite nonnegative geometry and nonnegative media position.
  Invalid values return `Invalid` without changing entries/settings. Zero width
  or height is valid and produces no items. The returned Items is always nonnil
  and copied; mutating a returned slice cannot change a later frame.
- Only Ready, Paused and Playing snapshots with matching generation may show
  entries. Idle, Loading, Failed, Ended, Closed or a generation mismatch shows
  none. No native Rate multiplier is applied: actual Snapshot.Position is the
  only timebase. Identical position/settings/geometry/data yields identical
  items regardless of wall time, callback count or forward/backward seeking.
- `Close` is terminal/idempotent and drops owned entries/schedules. Later writes
  return `IsClosed`; later Frame returns a closed empty frame with zero counts
  and geometry, retaining its last generation/revision/settings. The renderer
  closes on a Closed snapshot as well as an explicit composition Close.
- Errors use only the fixed ErrorCode values, without input text, URLs or raw
  errors. Invalid fixture installation shows a safe non-blocking error; it never
  stops playback. A new accepted generation starts empty, so corrupt new data
  cannot retain the previous part's comments. Valid replacement clears the error.
- In the view, stale/closed method calls return their code without overwriting
  visible state. Current-generation invalid installation/configuration keeps a
  safe Invalid state until the corresponding successful operation or newer
  Reset. SetUnavailable accepts only the current positive generation, installs
  an empty window and publishes Unavailable; it handles fixture lookup failure
  without stopping the player. A layout failure omits only that frame's bad
  item, keeps the remaining items and sets Unavailable unless a persistent input
  error already takes precedence. The next valid frame clears only that transient
  layout error. No raw error is retained or displayed.

## Scheduling and exact small-viewport rules

Scroll lifetime is `time.Duration(float64(8*time.Second)/Speed)`; fixed lifetime
is the same expression with 4 seconds. Truncation to positive nanoseconds is
intentional. Intervals are [At, At+lifetime); at the end an entry is absent.
Progress is `(Position-At)/lifetime` in [0,1). Changing speed rebuilds admission
at the same media position; there is no incremental animation state to reset.

LineHeight is `ceil(FontSize*1.25)+4` DIPs. For positive width, L is
`min(16, floor(height/LineHeight))`, clamped before converting to int. Width 0
sets L=0. Every entry reserves exactly one eligible lane for its entire life.
Process sorted entries once, choose the first free eligible lane whose prior
end <= At, or drop/count this entry. No catch-up burst, collision prediction,
pending queue, fading or adaptive density algorithm is introduced.

| Lane count | Top fixed | Bottom fixed | Either scrolling direction | Lane top coordinates |
| --- | --- | --- | --- | --- |
| 0 | none | none | none | no items; all loaded entries overflow |
| 1 | [0] | [0] | [0] | `(height-LineHeight)/2` |
| 2 | [0] | [1] | [0,1] | 0 and `height-LineHeight` |
| 3..16 | first T lanes ascending | last T lanes descending | middle lanes ascending | top/middle: lane*LineHeight; bottom: height-(L-lane)*LineHeight |

For L>=3, T=max(1,floor(L/4)). The pools are disjoint. L=1 and L=2 deliberately
share lanes; input order resolves contention without overlap. Bottom lane IDs
are still ascending top-to-bottom. Free space left by the 16-lane cap lies
between middle and bottom groups. The model supplies each Item.Top.

Store non-overlapping admitted intervals per lane. Build the schedule only
after entry replacement, speed change or L change; font changes affect L and
the returned geometry. At most 40000 scheduled references are stored. Lookup
is a binary search per lane, with at most 16 active results ordered by (At,
original index). Frame must not rescan all 40000 entries on every paint/tick.
Visibility and opacity do not change admission: hidden or zero-opacity frames
have no Items, and re-enabling immediately uses the current media position.

## Native text and desktop integration

- Shape single lines with existing `gui.ShapeText(text, font, 0)`, using
  `gui.Font{Size: float32(FontSize), Weight: 600, Lang: "zh-Hans", LineHeight: 1.25}`
  and the default system family/fallback. No bundled/downloaded font or new
  dependency. `TextLayout.Size` supplies actual width/height; a nil, zero or
  nonfinite layout result is `Unavailable` for that frame, never player failure.
- Layout preparation happens outside Draw callbacks. Retain layouts for active
  items only, keyed by generation/revision/index/font size; remove inactive
  keys after preparation. Steady retained layouts <=16; an outgoing and incoming
  prepared frame can temporarily reference <=32 distinct layouts, below the
  hard 64-layout ceiling. Do not shape all 40000 entries or add an LRU subsystem.
  Reset, valid replacement, font-size change and Close retire affected references.
  mygo owns its transient paint scene/font cache; this is not a claim that its
  global font resources are destroyed by application Close.
- For shaped width W, RTL X=`width-Progress*(width+W)`; LTR
  X=`-W+Progress*(width+W)`; fixed X=`(width-W)/2`. Y is
  `Item.Top+(LineHeight-shapedHeight)/2`. Clip both text and its (1,1)-DIP black
  shadow to the lane rectangle and actual video rectangle. Foreground uses the
  24-bit RGB color and settings opacity; shadow uses the same opacity. No wrap
  or ellipsis: long text is clipped, preserving its actual travel distance.
- Draw an immutable prepared frame via `Element.Draw` and `Painter.TextLayout`
  inside `Painter.Clip`. The callback only paints; no shaping, scheduling,
  time sampling, I/O, cache mutation or player commands, even if called twice.
  A read-only UI-thread lifetime guard returns without painting after Close,
  including a framework callback still holding the outgoing immutable frame.
  Use `PassThrough` and no focus/keyboard handlers on the overlay surface.
- Add optional `Danmaku *danmakuview.View` to parent desktop.View. Mount it as
  a sibling after the native image inside the existing fitted sw/sh rectangle,
  before player controls/status. Fullscreen letterboxing uses this same inner
  rectangle, not the whole window. No mpv/surface or Controller changes.
- Put a Danmaku button on a row with the existing Playing-part label, keeping
  the existing first control row's minimum-width behavior. Parent owns a third
  mutually exclusive popover flag. An open popover holds controls visible;
  opening parts/speed closes it and vice versa. Escape dismisses it before
  exiting fullscreen; outside dismissal preserves existing video-click
  suppression. Slider/checkbox keys get first refusal from global shortcuts.
- Stable automation keys: `danmaku-surface`, `danmaku-menu`, `danmaku-popup`,
  `danmaku-visible`, `danmaku-opacity`, `danmaku-font-size`, `danmaku-speed`,
  `danmaku-error`. Controls use checkbox visibility and sliders opacity step
  0.05, font size step 1 DIP and speed step 0.1. Settings are process-local,
  persist across parts/fullscreen, and reset to defaults on application restart.
- `fixture.Danmaku(part catalog.PartID) ([]danmaku.Entry, error)` is an in-memory
  copied-data function, separate from app.Source. It accepts only part-1/part-2;
  other IDs return danmaku.Invalid. It never does I/O or reads a credential.
  Composition handles a lookup error with SetUnavailable for that generation.
- Composition retains the last bound native generation. At the beginning of a
  window rebuild it samples Session.State; on a new nonzero generation it
  resets/installs fixtures for PlaybackPart, never SelectedPart. Existing desktop
  Render then performs its existing Tick. If that Tick accepts a newer part,
  the generation guard hides the old text until the next rebuild (<=one existing
  poll interval in an actively painting window). Do not add a generic callback
  or change the async source contract merely to eliminate this harmless gap.
- Explicit dispose closes the danmaku view on the UI thread before completing
  window close, without waiting for paint. Parent cancellation still leaves the
  window alive for the existing eight-second post-disposal check. No overlay
  goroutine, timer, subscription, native handle or close observer is added.
  Cancellation/deadline/late network results are not applicable here: all new
  work is bounded synchronous computation; the future transport must define
  its own async contract. Existing playback/source deadlines remain unchanged.

## Synthetic fixtures and observation contract

Each part returns 64 original entries, all within the 30-second clip. The first
eight cover 0-second top fixed Chinese text, RTL at 1s, LTR at 1.5s, bottom
fixed at 1.75s, top fixed at 4.5s, RTL at 4.6s, bottom fixed at 5s and RTL at
9s. Part 1 starts with `中文弹幕测试`; part 2 starts with `第二分P弹幕`. Include
`暂停后位置不变` and `拖动后重新出现` in the relevant original fixtures. Remaining
56 entries begin at `12s+(i%4)*1ms`, i=0..55, cycle modes 1/4/5/6 and use
original `密集弹幕` text with an index. Colors are 0xffffff, 0xffb7d5 and
0x80d4ff; no service data or copied comments.

Only e2e builds expose observations. Add `Danmaku DanmakuObservation` to the
existing strict preview Observation. DanmakuObservation has exactly the fields
of danmakuview.State above (same names/types, copied into plain evidence types),
including no Text, Color, location, user identity or layout pointer. Its Items
contains only ItemState fields. Missing/null/duplicate/case-alias/unknown fields
remain errors. Validate allowed codes/modes, finite geometry/progress/settings,
0<=Active<=Lanes<=16, Layouts<=16, len(Items)==Active, unique indices and lanes,
0<=Input<=64, Loaded+SkippedEmpty==Input, 0<=Overflow<=Loaded, and each index
within Input. Inactive/mismatched/closed/hidden/zero-opacity state must have
Active=0 and empty Items; Closed also has zero layouts/counts/geometry. When
active, generation matches playback, position equals the sampled playback
position, and bounds/progress follow the mode formulas. Counts never coerce a
positive leak to zero. Retain the current 64 KiB observation envelope bound.

The new native `-stage danmaku` uses the ordinary approved fixture Source.
No new environment-selected media/network path is required. Invalid-data and
stale-install cases are controlled unit/UI tests; no diagnostic endpoint for
arbitrary comments enters the normal app.

## Acceptance criteria

### DMK-AC-01

Validation, normalization, transactional replacement and generation guards have
deterministic regressions. Empty text is counted/skipped; corrupt or oversized
input cannot mutate the valid current window or create unbounded data.

### DMK-AC-02

The bounded schedule and exact 0/1/2/3/16-lane rules are correct. Pause, seek
back/forward, rate changes and time boundaries follow sampled media position.
Reinstall/resize/settings operations do not accumulate duplicates or frames.

### DMK-AC-03

The retained renderer paints Chinese text in the actual video area with correct
travel distance/clipping/settings, stays within the stated layout caps, does
not mutate state during painting, and releases its owned references on close.

### DMK-AC-04

Settings/popover input remains usable at the current 800x600 minimum size and
fullscreen. Overlay painting does not steal transport, click, keyboard or
fullscreen behavior, and error/empty states do not break playback.

### DMK-AC-05

The normal and synthetic e2e fixture app mount comments by accepted part/native
generation. Selected-but-unresolved parts preserve accepted comments; accepted
replacement/close cannot show the old generation. Native full/source checks
retain their original disposal, minimized-close and fullscreen assertions.

### DMK-AC-06

An exact combined commit passes full check --race, targeted normal/e2e tests,
independent review, and rebuilt macOS full/source/danmaku native stages. The
Chinese pixel proof and media-time behavior below are actually observed. This
establishes the native P0 seam, not real-service/full-client acceptance.

## Verification matrix and discriminating checks

| AC | Required verification | Evidence / platform |
| --- | --- | --- |
| 01 | Valid/invalid UTF-8, all caps/overflow-safe At, whitespace/control handling; mutable input/output copies; stale/reset/replace/close | Offline pure tests; race |
| 02 | Equal-time order, admission and interval ends; all small lane counts; repeated fixed snapshots despite advanced test clock; backward seek identity; Rate ignored; size/speed/visibility changes; largest supported window | Offline pure tests; race; allocation/schedule-build counters local to tests |
| 03 | gui.Tester image comparisons for paint/no-paint, clips and actual shaped width; active-set reuse/cap/release; repeat paint yields same output/state; invalid layout seam | Offline UI tests and native captures; native Chinese recognition is separate |
| 04 | Existing desktop suite plus popup exclusivity, outside dismissal, Escape ordering, focused slider keys, min-size bounds, no input through overlay | gui.Tester; native danmaku and full stages |
| 05 | Accepted A while B resolves/fails, then B accepted; same-part new generation/replay; close without paint and during pending source | Offline composition tests, existing source/native stages |
| 06 | Full checker, exact-candidate code/native review, three rebuilt native stages and glyph inspection | Local Go1.27.3/macOS; Windows native not-tested/P2; remote CI only if actually run for candidate |

For each new guard/limit, at least one regression must fail if that mechanism
is locally reverted. Required negative controls specifically remove stale
generation rejection, entry/byte/active-layout caps, empty-text accounting,
replacement rather than append, sampled-position use, exclusive lane lifetime,
clipping, pointer pass-through, and close cache release. Use compiling local
mutations, restore them individually, and keep the result matrix. This is
regression evidence, not an author independent review.

Native danmaku stage, within the existing 240-second invocation bound:

1. Pause at a settled media position near 2s. Capture the fitted video region
   with popovers closed. Verify original Chinese glyph pixels and recognize
   `中文弹幕测试` in the native PNG, not in an accessibility string. A second
   hidden-overlay capture at the same paused position must remove the text
   pixels; blank rectangles or tofu boxes do not satisfy the visual review.
2. Compare two stable paused observations/crops separated by >=500ms, then
   play and verify the same scroll item moves in the expected direction using
   observed media-position deltas. Native Rate=2 must follow position once,
   not double it; no calibrated physical-rate claim is required.
3. Seek back to the same settled position with the same settings. The same
   entries/lanes/geometry return within native position tolerance (100ms);
   deterministic unit tests require exact equality. Exercise visibility,
   zero/partial opacity, size and danmaku speed via their actual UI controls.
4. Enter/leave native fullscreen and resize at the 800x600 minimum. Verify
   the fitted clip/letterbox geometry and unchanged native generation/ownership;
   exercise popup Escape and ordinary video clicks with the overlay present.
5. Seek near 12.5s to observe overflow with <=16 active items and <=16 current
   layouts. Switch to part 2, pause, and inspect its original Chinese phrase;
   old generation text is absent. Source stage separately exercises unresolved
   selection and minimized close with this integrated candidate.
6. External cancellation/dispose still reaches zero native resources within
   the existing 200ms bound, has zero active/layout/entry counts, and keeps the
   window alive for the existing eight-second survival check before graceful
   close. Do not relax readiness, picture, source ownership or cleanup checks.

Build matching normal/e2e bundles with the existing package-preview flags and
an exact smoke-driver binary. The controller records hashes privately and runs
the existing driver flags (`-app`, `-app-sha256`, `-driver-sha256`, `-output`)
with `-stage full`, `-stage source` and new `-stage danmaku`, each into a fresh
directory. Screenshots contain only these original fixtures. No screenshots of
the login/account window, live danmaku, service responses or signed media.

## Risks, review, rollback and continuation

The system font fallback and drawing order must be demonstrated on the native
Mac, not inferred from nonempty text/layout data. Whole-lifetime lane reservation
intentionally trades density for deterministic non-overlap. At maximum input,
sorting/one-time schedule rebuild is bounded synchronous CPU work; measure the
targeted maximum-input test, without inventing a network timeout or an animation
worker. The cap allows transient old/new input copies (<=16 MiB text combined),
not unlimited history or segment accumulation.

Independent architecture review resolves this proposal before freeze. Pure
timeline implementation precedes renderer/fixture workers; overlapping parent
desktop/composition/driver writes are serial after both and completed 0025. No
other task may edit these shared files concurrently. Code/native review binds
to the final combined commit and actual artifacts. No check/policy relaxation
is proposed; any such need returns for separately authorized review.

New slice review/fix counts start at 0/0 (maximum 3 each); prior login/async
findings and their counters are retained, never reset. The shared remote issue
budget remains 0/5 used, with no filing authority. Controller finding inventory:
`.agent/p0-start/inventory.md`; overlay-local evidence under `.agent/p0-overlay/`.
Controller alone creates/updates frozen run records. Durable result:
`reports/0038-p0-native-danmaku.md` and STATUS.md, claim/evidence/result/scope
table, including found-not-done live ingestion and full-client integration.

Rollback reverts only this slice; retain account credentials, source/playback
evidence and user work. No open product preference needs a new owner question.
Remaining pre-freeze prerequisites are final 0025 identity and independent
architecture disposition. Native glyph/timebase/cleanup proof is additionally
required before freezing a contract that claims the complete real P0 loop.
