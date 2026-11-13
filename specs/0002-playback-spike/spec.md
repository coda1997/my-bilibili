# Native UI and playback spike

> Owner platform amendment, 2026-11-12: [ADR 0005](../../docs/adr/0005-platform-priority.md).
> macOS is required; Windows is P2 / nice-to-have and does not block M1, M2
> or P0. Windows remains `not-tested` until exercised. Historical reviews
> describe their frozen candidates; this amendment changes only platform priority.

Spec ID: `0002-playback-spike`
Status: selected by the owner on 2026-11-10 for a macOS-first experiment;
this refined contract still requires independent architecture review before
freeze and implementation. Windows execution is now optional P2.

## Goal

Decide whether native mygo UI and its player can support the first desktop
playback slice on macOS, with Windows deferred P2, without building the full application.

## Non-goals

No complete login implementation, real-account CI fixtures, downloading a
media library, entitlement bypass, public release, or general player framework.

## Preconditions

Select and pin the evaluated mygo revision. Obtain authorized, sanitized media
fixtures. The owner authorized doing the macOS phase first on 2026-11-10.
On 2026-11-12 the owner made Windows optional P2, superseding the earlier
Windows-access plan and dual-platform acceptance requirement. Windows capability
cells remain not-tested; required macOS and target-access evidence still gates M1.

The owner separately authorized independent review and delegated responsibility
for this bounded experiment. No commit, push, publication, policy change, full
client implementation or additional global tool installation is included.

Keep this feasibility spike serial: validate the UI harness before integrating
media surfaces and playback. Demonstrate parallel feature development in M2,
after these contracts are understood, rather than inventing premature seams.

## Selected candidate and safe inputs

- Go 1.27.3 from the owner's Homebrew installation; macOS arm64 with Xcode.
- Evaluate mygo commit `daf6341a2fe05f62dc8519e7cc215a90c757e8e0`, resolved by
  Go to `v0.8.4-0.20261110145816-daf6341a2fe0`. Record module sums in `go.sum`.
  Upstream source has an MIT license. This is an experimental pin, not a
  production UI/player decision or proof of its transitive dependencies.
- PLAY-001 records this intended pin without an unused module requirement.
  PLAY-002 adds the pin with the first real UI import; PLAY-003 may update
  ordered module metadata for its actual video imports. Every task retains
  the full tidy-diff/check gate; no artificial dependency-holder is allowed.
- Retain one Go module. No third-party Bilibili SDK or service calls are needed.
- Generate short synthetic H.264/AAC fixtures with the already-installed
  FFmpeg, invoked by a maintained Go fixture command under `testdata`.
  Generate a combined MP4, video-only MP4 and audio-only M4A. Record the
  generation arguments, tool version, codec/duration metadata and SHA-256s.
  Only generated media is stored; no upstream example video is copied.
- Fixtures are at most 10 seconds, 640 by 360, and 2 MiB per file. Generation
  is an explicit preparation step, not a dependency of ordinary unit tests.
  FFmpeg is an offline fixture tool, not a runtime decoder, proxy or remuxer.

Inspected source references at the selected revision:
[module](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/go.mod),
[license](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/LICENSE),
[video API](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/ui/video/video.go),
[UI testing](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/docs/ui/testing.md),
and [native automation](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/docs/plugins/automation.md).
Source inspection is not native runtime evidence.

## Boundaries and execution contract

This is a small `experiments/playback` harness, not the desktop shell. The
`contract` package owns plain input, command, state and error types. `player`
is the native adapter; `ui` owns controls and presentation; `cmd` owns fixture
serving, process orchestration and the native smoke driver. No framework type
appears in `contract`; UI-facing drawing methods may exist in the adapter.
No product packages, database, event bus or generic playback framework appear.

The plain boundary has these concrete shapes (named state/error/command enums
are implemented with the cases listed below):

```go
type Input struct {
    FilePath string
    URL      string
    AudioURL string
    Headers  map[string]string
}
type Command struct {
    Kind     CommandKind
    Position time.Duration
    Value    float64
}
type Snapshot struct {
    Generation uint64
    State      State
    Position   time.Duration
    Duration   time.Duration
    Rate       float64
    Volume     float64
    Error      ErrorCode
}
type Engine interface {
    Snapshot() Snapshot
    Command(Command) error
    Close() error
}
type Opener interface {
    Open(context.Context, Input) (Engine, error)
}
```

A small main-thread controller owns the active engine, context, generation,
clock and ready deadline. It exposes open/command/tick/snapshot/close plus the
current plain engine for the UI's adapter-local drawing assertion. Opening is
not a blocking wait for readiness. Inject the clock and opener for lifetime
tests; no goroutine or event channel is needed for this immediate-open/polled
experiment. A late state change in a disposed engine cannot affect the current
snapshot. Invalid inputs are rejected before replacing the current engine;
a valid but unsupported replacement closes the old engine and exposes failure.

The experiment contract to implement is:

- Input identifies exactly one local fixture path or loopback HTTP URL, with
  optional request metadata and a separate audio URL to probe capability.
  Reject ambiguous, malformed, non-loopback remote inputs and excessive
  metadata before native access. Fixture headers contain synthetic sentinels
  only. URL/header values are never included in public error strings.
  Limit paths/URLs to 4096 bytes each and metadata to 8 entries/4096 total
  bytes; reject control characters, URL credentials/fragments, non-HTTP URLs,
  non-numeric loopback hosts, and a separate audio input without a primary URL.
- Session commands cover play, pause, seek, volume and rate. A snapshot exposes
  idle/loading/ready/playing/paused/ended/failed/closed, actual position and
  duration, configured volume/rate and a safe error category. The UI can
  present the native surface without exposing it through the plain contract.
  Reject unknown commands, non-finite numbers, volume outside 0..1, rate
  outside 0.25..3, and negative seeks. Clamp seeks to a known positive duration.
- Experiment-owned public native calls, polling and disposal run on the
  UI/main thread.
  Opening returns a loading session promptly; readiness has a 10-second
  deadline. During loading/playing, experiment-owned polling requests UI ticks
  no faster than 20 Hz. No experiment background callback calls native methods
  or mutates a view directly. These guarantees do not constrain framework
  internals: the inspected mygo AVFoundation engine has a 25 ms background
  watcher and independent video-frame scheduling. Record that distinction;
  native teardown evidence covers owned handles, not all OS allocations.
- Each open gets a monotonically increasing generation and an owned context.
  Switch cancels and closes the old session before publishing the replacement.
  Canceled or obsolete completions cannot revive it. At most one native
  session is active. Close is idempotent and removes native handles, pending
  ticks and subscriptions owned by the experiment. Closed commands fail safely.
  Native source errors and ready timeouts close the active engine before
  publishing failed state. Cancellation closes it and publishes closed state
  with a canceled code. Explicit close publishes closed with no error code.
- Context cancellation is observed on the next UI tick while loading or
  playing, within 100 ms while the UI event loop is responsive. Window close
  performs synchronous owned teardown. This is not a promise about a stalled
  OS event loop or every internal system-media allocation.
- The selected public open API takes a single URL/file/data input and exposes
  no header or separate-audio argument. Do not invent support: unsupported
  requests return a stable unsupported error without dropping metadata or
  audio. Include a protected-HTTP negative probe to observe native failure.
  A successful safe rejection is not a passing playback-capability cell.
- Errors distinguish invalid input, unsupported capability, canceled, timeout,
  media failure and closed. Native error bodies/locators stay out of logs.
  An error is visible and retryable; retries replace rather than accumulate
  sessions. Limits and errors have deterministic regression tests.

### Controlled HTTP and native smoke

The Go fixture server binds only `127.0.0.1:0`, serves an explicit allowlist of
generated files, supports normal Range requests, and has plain, protected,
delayed and failure routes. A protected route requires a synthetic header;
delayed responses wait at most 3 seconds and stop on request cancellation.
Use finite HTTP header/read/write/idle timeouts, an 8-request concurrency cap,
and a 3-second server shutdown deadline. It never forwards or fetches media.
Requests are counted, but credentials and arbitrary request/response bodies
are not logged. Default tests use temporary directories and local fixtures.

Native automation is test-build-only (`e2e` tag), using a Unix socket in a
fresh private temporary directory. The Go driver uses bounded reads and
request deadlines, starts only the owned experiment process and tears it down
on failure. A complete smoke has a 90-second deadline. Capture only that
synthetic app window, never the desktop or other apps. Screenshots, decoded
native states and test actions are evidence; a headless tester is not a native
runtime check. Physical speaker audibility and real IME composition remain
not-tested unless actually observed; injected Chinese text is labeled as such.

The native driver must assert positive duration and ready state within 10
seconds, then verify the rendered synthetic picture in an app-window capture
(a distinctive fixture color region, not merely a status label). After Play,
position must advance at least 300 ms within 2 seconds. After Pause settles
for 200 ms, position may drift no more than 150 ms across 500 ms. A paused
seek to 3 seconds must reach within 500 ms of its target within 2 seconds.
Replacement must increase generation, reset its state/position and never
restore the old generation during subsequent ticks. Close/cancel must clear
the owned engine, keep the generation stable, reject further controls and
allow the owned app process/socket/server to terminate within bounded cleanup.
Failed assertions are failed capabilities, not passing command delivery.
Rate/volume readback proves configuration only; actual speed and physical
audibility require separate observations and otherwise remain not-tested.

Validate the UI harness before adding media probes. Full login, live Bilibili
access, full danmaku/overlay work, additional player libraries, custom decoding,
transcoding, local media proxies and application JavaScript are excluded.
If the built-in player is unsuitable, record the gap and stop at a replacement
recommendation; do not quietly change engines or bypass access requirements.

## Acceptance criteria

### PLAY-AC-01

Record the exact dependency revision, Go version, OS/architecture, fixture
provenance, and supported test cases. Do not store authenticated URLs or cookies.

### PLAY-AC-02

A minimal native Go UI starts on macOS, the required target OS. Verify Chinese text and
input, loading/error states, and a test-observable playback command. For the
selected macOS phase, preserve native-window evidence plus headless control
tests; explicitly distinguish injected text from real IME coverage.

### PLAY-AC-03

Test target stream access, required request metadata, separate tracks when
applicable, seek, switching, cancellation, late results, and cleanup. Record
pass/fail/not-tested for each capability on macOS, with evidence; retain
Windows as optional P2 / not-tested until separately exercised.
The macOS phase covers generated local/HTTP media, protected-route failure,
unsupported header/separate-track handling, controls, cancellation, stale
generations and teardown. It does not establish real Bilibili compatibility.

### PLAY-AC-04

An independently reviewed decision accepts the player adapter, selects a
different implementation behind the same boundary, or rejects the proposed
stack with a specific reason. Do not infer platform support from compilation.
The intermediate macOS report may recommend the next probe or identify a
blocking gap, but cannot accept the final adapter or complete M1 while
required macOS capability and target-access evidence are missing. Missing
Windows evidence does not block acceptance.

## Verification matrix

| Criterion | Automated and native evidence | macOS-first boundary |
| --- | --- | --- |
| PLAY-AC-01 | Pinned module/sums, fixture manifest, generation/metadata checks | Record exact Go, OS/architecture, source and fixture hashes |
| PLAY-AC-02 | Headless control/Chinese input/state tests; actual native window and isolated capture | Native text insertion is not real IME validation |
| PLAY-AC-03 | Input/lifetime/error regression tests; Range/header/delay/server tests; native media/control/switch/close smoke | Unsupported track/header support is fail with reason; target service and Windows remain not-tested |
| PLAY-AC-04 | Independent architecture and exact-candidate code/native evidence review | Interim recommendation only, not final M1 acceptance |

Run `go run ./cmd/dev check --race` on macOS and preserve sanitized failures as
well as successes. Unit tests must not require a live account, external network,
FFmpeg, a graphical session or native playback. Explicit preparation/native
commands are documented separately. Missing runtime permissions or unavailable
dependencies are blockers, never fabricated passing evidence.

## Result location

Raw sanitized evidence belongs to the selected run. The durable capability
matrix and decision belong in `reports/0002-playback-spike.md` and a follow-up
ADR. The partial macOS result belongs in `reports/0002-playback-macos.md`.
Before code review, archive the exact tracked/untracked candidate and record
file hashes/modes separately from HEAD. No completed runtime result exists at
contract preparation. A changed contract requires review and a new frozen run.
