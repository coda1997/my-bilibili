# Explicit M1 commands

Run from the repository root with the pinned Go version and normal testing
prerequisites. These commands do not access a real account or target service.
Ordinary `go test` uses fakes, headless UI and loopback fixtures, not a GUI,
native decoder, FFmpeg or external network. Initial module download requires
network access; fixtures are checked into this experiment with their hashes.

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
