# Explicit M1 commands

Run from the repository root with the pinned Go version and normal testing
prerequisites. These commands do not access a real account or target service.
Ordinary `go test` uses fakes, headless UI and loopback fixtures, not a GUI,
native decoder, FFmpeg or external network. Initial module download requires
network access; fixtures are checked into this experiment with their hashes.

The opt-in libmpv replacement surface gate uses separate commands below. The
commands in this first section continue to select the provisional mygo player.

```sh
export GOTOOLCHAIN=local GOWORK=off
go run ./cmd/dev check --race
mkdir -p .agent/m1
go build -tags e2e -o .agent/m1/native ./experiments/playback/cmd/native
# Default is still the strict full suite (currently fails delayed loading).
go run ./experiments/playback/cmd/smoke -app .agent/m1/native
# Explicit bounded fixture preview; NOT a full/M1 acceptance result.
go run ./experiments/playback/cmd/smoke -suite preview -app .agent/m1/native
```

The smoke prints a fresh evidence directory; `-out` may name a new directory
but cannot overwrite an existing one. Keep the owned native window exposed in
an interactive macOS desktop for the whole run, with no manual control input.
The automation socket lives in a private temporary directory and is removed
on exit. Only the owned app window is captured, never the desktop. The process
has a 90-second total deadline plus bounded teardown. Do not pass an untrusted
binary: the caller explicitly supplies the owned experiment app to execute.

Every result records `suite: full` or `suite: preview`; unknown names are rejected
before launching the app. Preview verifies the visible temporary-use warning,
local synthetic colors/progression/pause, slider seek4s, continuous volume,
speed preset configuration, close/retry generation and graceful active-session
exit. It does not exercise delayed loading, required headers or separate audio.
A preview pass does not pass full/M1. Keep both runs, including full's failure.

The existing lab is now scrollable. New sliders/presets are disabled without a
usable owned non-Loading session; seek additionally needs a positive sampled
duration. Retry reopens the last validated input, including unsupported metadata,
and stays disabled while Loading. It is manual, not transient-network recovery
or a remedy for a native UI-thread freeze. Legacy diagnostic buttons remain.
See [temporary-use ADR](../../../docs/adr/0004-provisional-mygo-playback.md) and
[known issues](../../../docs/known-issues/playback.md). No upstream workaround
or dependency change is included.

If `WindowHidden` is true, the pinned framework skips native frame delivery.
The full driver records the rendered-playback/control cells as `not-tested`, still
exercises independent asset/HTTP/negative/lifetime probes, and exits nonzero.
Preview records its native playback as not-tested, quits and checks cleanup,
then exits nonzero without attempting a picture/control pass.
This is not a successful playback smoke and does not prove a decoder defect.
Other probe failures remain `fail`, not masked by the occlusion diagnosis.

An exposed-window run asserts the synthetic rendered colors, positive duration,
playhead advance, paused stability, seek tolerance, generation isolation and
owned close/teardown. Volume/rate readback is configuration only. Real IME,
physical audio, actual playback speed, A/V sync and real Bilibili compatibility
need separate evidence. Windows must be run on Windows; cross-builds do not
count. Missing header/separate-track support is a failed capability even when
its safe unsupported rejection passes.

The adapter permits mygo's initial play to obtain a first native picture, then
pauses/resets before publishing Ready unless Play was requested. Loading
Pause/Seek intent is queued until that picture. This startup behavior is unit
tested; exposed local/HTTP evidence is in the historical M1 report and the
current provisional report. It does not repair slow-load responsiveness.

For a manual local-file window, without the test-only automation plugin:

```sh
go run ./experiments/playback/cmd/native
```

Normal builds do not import the automation plugin. The smoke itself serves only
the three hash-verified generated media files from `../testdata`, with no
forwarding, proxy, remux, custom decoder or Bilibili SDK. Existing FFmpeg is an
explicit offline fixture generator only; see `../testdata/README.md`.

## Replacement surface gate (RPL-001 only)

This separate macOS arm64 experiment uses pinned mpv v0.42.0 client API 2.6,
the existing purego v0.11.1 pin and mygo's public bitmap API. It polls native
events on a control owner and software rendering on a render owner, using two
640x360 buffers at up to 30Hz. Ordinary tests and the provisional app never
load this library. Other native targets return Unsupported.

Supply an authorized, provenance-verified local runtime before running. There
is no automatic download, global install or fallback.
The approved CoreAudio-without-Cocoa build correction and its manual provenance
recipe are in [mpvprobe/runtime](../mpvprobe/runtime/README.md). Preserve its
strict linker and loader preflight before invoking the native surface gate.

The runtime manifest uses the following JSON fields, plus separate
source/build/license/OS records:

```json
{
  "SourceCommit": "b0841e8c4d156b8403dc1876c4cbd9011eb583f3",
  "API": "2.6",
  "Library": {"Path": "/absolute/resolved/libmpv.2.dylib", "SHA256": "64 lowercase hex characters"},
  "Dependencies": [
    {"Path": "/absolute/resolved/dependency.dylib", "SHA256": "64 lowercase hex characters"}
  ]
}
```

Record and hash all non-system runtime dependencies, actual build options,
licenses, acquisition checksums, fixture identity and OS/toolchain. System
frameworks in the macOS dyld cache are identified by their paths and OS build;
do not fabricate standalone hashes for unavailable files. The explicit smoke
also records the owned app's loaded images in `native.log`.

```sh
go build -tags e2e -o .agent/m1/replacement-surface-r0 ./experiments/playback/cmd/replacement-native
go run ./experiments/playback/cmd/replacement-smoke \
  -stage surface -app .agent/m1/replacement-surface-r0 \
  -runtime /absolute/runtime-manifest.json -out /absolute/fresh-evidence-directory
```

Keep the owned window exposed and avoid manual input. The first gate checks
local/HTTP synthetic picture, delayed-request switching and cancellation,
zero owned native/worker counters and graceful delayed-load window close.
Cancellation includes action receipt and external closed/no-owned observation
within 100ms. App cleanup is bounded to 3s; RPC/readiness/overall limits remain
3s/10s/90s. Captures and failures are retained; forced reaping fails the gate.
An occluded window is not-tested and exits nonzero. Unknown stages, including
`capabilities`, reject before app/runtime/fixture access.

Preserve the first native gate outcome. Failure ends this probe with a
rejection/needs-human report; do not retry into a passing result or relax
teardown semantics. Passing gate-one evidence needs separately authorized
independent review before RPL-002. Controls, required headers, separate audio,
physical audio, measured sync, Windows and real service access are not accepted
by this surface stage. The default preview and historical M1 gaps remain.
