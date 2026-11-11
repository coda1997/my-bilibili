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

## Replacement probe (RPL-001 and RPL-002)

This separate macOS arm64 experiment uses pinned mpv v0.42.0 client API 2.6,
the existing purego v0.11.1 pin and mygo's public bitmap API. It polls native
events on a control owner and software rendering on a render owner, using two
640x360 buffers at up to 30Hz. Ordinary tests and the provisional app never
load this library. Other native targets return Unsupported.

Supply an authorized, provenance-verified local runtime before running. There
is no automatic download, global install or fallback.
The approved CoreAudio-without-Cocoa build correction, owner-requested cleanup
repair, and manual provenance recipe are in
[mpvprobe/runtime](../mpvprobe/runtime/README.md). Preserve its
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
within 200ms (owner amendment, 2026-11-11). App cleanup is bounded to 3s; RPC/readiness/overall limits remain
3s/10s/90s. Captures and failures are retained; forced reaping fails the gate.
An occluded window is not-tested and exits nonzero. Unknown stages reject before app/runtime/fixture access. The default stage
remains `surface`; `capabilities` is an explicit, broader probe requiring
recorded independent gate-one approval and an RPL-002 implementation assignment.

Preserve the first native gate outcome. Failure ends this probe with a
rejection/needs-human report; do not retry into a passing result or relax
teardown semantics. Passing gate-one evidence needs separately authorized
independent review before RPL-002. Controls, required headers, separate audio,
physical audio, measured sync, Windows and real service access are not accepted
by this surface stage. The default preview and historical M1 gaps remain.


### Explicit capability stage (RPL-002)

Implementation status on 2026-11-11: code and deterministic checks are present,
but native validation is **incomplete / needs-human**. Historical run
`20261111T133906Z-38eafa873059` retains its slider-oracle failure/repair,
occluded not-tested invocation and final 100.455083ms loading-cancel failure
against the original 100ms bound. That outcome remains unchanged.

The owner then authorized a 200ms external loading/playing cancellation limit.
New frozen run `20261111T140848Z-c0ee85eaae70` passes full race checks and its
loading-cancel observation at **67.428959ms**, with Closed/Canceled, no owned
engine and zero native/worker counters. Native local/HTTP picture, delayed
switching/generation stability, local progression/pause/seek, actual seek-slider
interaction and 0.5x/1x configuration pass. The stage then fails with
`automation write failed` before the 2x preset is recorded. Its cause was
undiagnosed at that handoff; the later diagnosis below identifies a native
crash. No full capability pass is claimed. The driver attempted forced app
cleanup and the server finished with Active 0. HTTP controls, retry,
protected headers, separate tracks, negative media routes, playing cancellation
and graceful final window close were not reached in this invocation.
Independent review of RPL-002 and the amended bound remains pending.
Exact source archives, checks, result JSON, captures, loaded images and the
continuation handoff are local under the new run's `evidence/`, not backed up
by Git. Prior frozen inputs/events and historical reviews are preserved.

The subsequent blocker investigation (controller run
`20261111T141629Z-f512d285fa7e`) found matching macOS SIGSEGV reports for the
original invocation and diagnostic candidates. A minimal native regression
opens one local fixture, reaches Ready, closes with zero adapter counters, and
then crashes about seven seconds later. LLDB stops in `_Block_release` on
`io.mpv.coreaudio_stop_during_idle`: its unmapped address is exactly the former
libmpv load base plus the pinned CoreAudio idle-block descriptor offset. The
pinned runtime cancels its seven-second `dispatch_after` block during core
destruction, but dispatch still retains the block until delayed cleanup. The
adapter's per-session `Dlclose` unmaps the descriptor before that cleanup.
Consequently zero adapter counters do not establish native deferred-work
quiescence. This is a library-lifetime failure, not a 2x-speed or RPC-limit
failure. The runtime and lifetime contract were not changed by this diagnosis;
native capability acceptance remains blocked pending a separately scoped repair.

Bounded driver diagnostics now retain at most 2048 RPC metadata records with
phase, error category, timing and byte counts, never request/response bodies,
raw socket errors, locators or header values. Early app exits are distinguished
from an attempted forced reap, and the native app logs explicit window/quit
lifecycle events. Evidence, exact diagnostic candidates, retained failures and
the isolated regression are local under
`.agent/worker-scratch/repair-automation-blocker/`; the controller owns run
publication. Diagnostic executions are not a combined capability pass or an
independent review.

The owner-requested cleanup repair is recorded in run
`20261111T144422Z-afb986d84d83`. Its bounded upstream patch replaces the delayed
block with an owned timer thread that is signaled and joined during teardown,
preserving the seven-second idle stop and per-session library unload. The
maintained opt-in `nativecleanup` regression first fails with SIGSEGV on the
old runtime. All five scenarios pass on the new runtime: paused/playing close,
pause/resume, idle expiry followed by resumed progression, and repeated sessions.
Each child survives eight seconds after its final close; observed Close times
are 1.755792–21.229459ms, with zero adapter counters and idempotent Close.
Full `go run ./cmd/dev check --race`, strict runtime build, all 15 loader symbols
and API 2.6 pass. These are author checks; independent repair review and
CA-CLEANUP-001 closure remain pending.

Use the fresh ignored runtime manifest at
`.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/runtime-manifest.json`, SHA-256
`a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`.
The repaired libmpv SHA-256 is
`8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd`.
All 29 dependency hashes are unchanged. The e2e app at
`.agent/m1/replacement-coreaudio-cleanup-r1` is byte-identical to the previous
diagnostic app; native loaded-image verification matches that app and all
30 runtime artifacts. The old runtime and failed evidence are retained.

The first complete capabilities invocation with the repaired runtime still
exits 1. It passes exposed local/HTTP picture and controls, all rate preset
readbacks, explicit close/retry, native negative routes, protected-header
playback and header isolation. Loading cancellation passes at 67.470292ms
against the unchanged 200ms bound. It subsequently fails separate-track
readiness with `state=failed error=media-failure duration=0s owned=false`;
both protected stream requests were allowed with the required metadata.
That readiness failure is not diagnosed by this cleanup repair. The driver
forcibly reaps its owned app and server Active returns to zero. Separate-track
playback, playing cancellation and final graceful window-close checks are
unreached; physical audio/sync, Windows, target service, packaging and candidate
CI remain not-tested. This is not a full capability or M1 pass.

Exact source snapshots, native regression logs, runtime provenance, captures,
the failed capabilities result and continuation instructions are under the
new run's `evidence/`, with `handoff.md` as its entry point. The final source
snapshot changes only this guide from the tested candidate. Next: independent
review of the cleanup repair and a bounded investigation of separate-track
readiness. No native retry, deadline relaxation, default-player change or
remote publication accompanies this result.

RPL-001's independent approval is recorded in
[the replacement continuation](../../../reports/0006-player-replacement-probe.md#independent-implementation-and-gate-one-review).
That approval describes its historical candidate and gate observations;
independent closure of the later lifetime finding remains pending. The diagnostic
investigation retained the old runtime; the cleanup repair above supplies a new
runtime with unchanged dependency pins and default player. The extended adapter binds
`mpv_set_option`, `mpv_get_property` and `mpv_free_node_contents` in addition
to the original symbols. Node/list layout and union alignment are tested;
typed string arrays preserve commas in custom headers and audio URLs. Referer
uses the documented `referrer` option. Every new core receives copied input
before load. Selected-track nodes are copied on the control owner and native
property memory is freed before the next event; later loss of required external
audio fails and disposes the presentation.

```sh
go build -tags e2e -o .agent/m1/replacement-capabilities-r0 ./experiments/playback/cmd/replacement-native
go run ./experiments/playback/cmd/replacement-smoke \
  -stage capabilities -app .agent/m1/replacement-capabilities-r0 \
  -runtime /absolute/runtime-manifest.json -out /absolute/fresh-evidence-directory
```

This invocation includes the surface assertions with the owner-authorized 200ms cancellation bound, then local/HTTP
progression, pause, seek, actual slider keyboard interaction, speed/volume
configuration, close/closed-command rejection and explicit retry. Its opt-in
fixture-server mode requires both a comma-containing synthetic custom field
and Referer. It checks missing/wrong metadata, protected combined and separate
streams, later headerless-session isolation, missing-audio failure, playing
cancel and zero owned resources. It closes the window during an observed
delayed request. Deadlines remain 90s overall, 3s RPC, 10s readiness/media failure,
2s progression/seek and externally observed 200ms cancel, with bounded cleanup.
Original fixture-server defaults, player, contract, fixtures and legacy smoke
suites remain unchanged.

Server evidence contains only fixed per-file match/request/allow/deny counts;
no request metadata, native error body or media locator is recorded. Keep the
first capability outcome and failures. Exit zero means the implemented probe
assertions passed, not final M1 or full separate-track acceptance: physical
speaker output, measured A/V sync and actual playback speed have separate
not-tested cells. Selected/fetched audio alone never passes RPL-AC-05. Windows,
real-service access, IME, packaging/signing and candidate CI still need their
own evidence. Final exact-candidate independent review and RPL-003 remain
separate work.
