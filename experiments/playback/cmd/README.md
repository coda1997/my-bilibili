# Explicit M1 commands

> Current milestone status, 2026-11-13: M1 completed and accepted by the owner.
> See [the acceptance handoff](../../../reports/0025-m1-acceptance.md).
> M2/P0 remain unstarted; this does not approve their contracts or start new sessions.

## Default libmpv playback

On 2026-11-13 the owner selected libmpv as the default playback backend, retaining
mygo for UI. `cmd/native` now composes the reviewed libmpv adapter; the previous
system-media launcher is explicitly `cmd/mygo-native`. See
[ADR 0008](../../../docs/adr/0008-default-libmpv.md) and
[the current handoff](../../../reports/0024-default-libmpv.md).

Run from the repository root with the pinned Go toolchain. Supply the verified
mpv v0.42.0 / API 2.6 runtime with the reviewed CoreAudio cleanup patch; its
provenance and installation prerequisites are in
[the runtime guide](../mpvprobe/runtime/README.md). The runtime is ignored and
absent from fresh clones. There is no automatic download, discovery or fallback.
For this checkout's reviewed runtime, the local-fixture command is:

```sh
export GOTOOLCHAIN=local GOWORK=off
go run ./experiments/playback/cmd/native \
  -library "$PWD/.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/prefix/lib/libmpv.2.dylib"
```

Use the absolute verified library path for your checkout. `-library` is required;
missing/relative/nonregular paths fail before a window is created. The loader
reports Unsupported for an incompatible library, with no switch to mygo media.
No HTTP server is needed for the local fixture. Select **Local fixture**, then
**Play**; inputs initially load paused. An optional `-http-base` must be an owned
loopback HTTP origin, without credentials, path or query; one trailing slash is
accepted. It enables the existing combined/header/separate-track fixture controls.
This command does not accept target descriptors or request Bilibili metadata.

For automated default-app verification, use the libmpv capability driver:

```sh
mkdir -p .agent/m1
go build -tags e2e -o .agent/m1/native ./experiments/playback/cmd/native
go run ./experiments/playback/cmd/replacement-smoke \
  -stage capabilities -app .agent/m1/native \
  -runtime "$PWD/.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/runtime-manifest.json" \
  -out "$PWD/.agent/m1/default-capabilities-r1"
```

Choose a fresh `-out` directory. The driver validates runtime artifact hashes,
starts its owned fixture server, supplies `-library`/`-http-base`, checks the
default libmpv window and retains the existing controls, header/separate-track,
200ms cancellation and graceful-cleanup assertions. Keep the desktop exposed
and do not operate controls during the run. `cmd/smoke` is the legacy mygo
oracle; use it only with `cmd/mygo-native`. The normal build has no automation
plugin. The separate `replacement-native` and `target-native` commands remain
available for their explicitly assigned diagnostics.

## Target playback host-policy amendment

[ADR 0007](../../../docs/adr/0007-target-host-policy.md) removes the target media
origin policy. All valid HTTPS hosts and ports are allowed, including IP literals
and localhost. Source/rendition, grant, public headers, TLS verification, expiry
and teardown requirements remain. Descriptor version 2 omits `origins`; old
version-1 input and old helper/app/driver binaries are not interchangeable.

The updated helper source is in ignored
`.agent/worker-scratch/target-refresh-unrestricted/`; its default makes no request.
It shares `targetprobe.ValidateMediaURL` with both target binaries. Rebuild all
three from the exact reviewed candidate, for example into a fresh ignored directory
whose parent already exists:

```sh
go build -tags e2e -o .agent/m1/target-native-v2 ./experiments/playback/cmd/target-native
go build -o .agent/m1/target-smoke-v2 ./experiments/playback/cmd/target-smoke
go build -o .agent/m1/target-refresh-v2 ./.agent/worker-scratch/target-refresh-unrestricted
```

The ignored helper is local evidence, absent from fresh clones; recover its exact
reviewed source from the current run rather than reusing the historical helper.
No command above launches native media or requests metadata. Actual TGT-003
execution still needs a separately recorded bounded session assignment and exact
binary/runtime pins. A new CDN host requires no additional policy review.
See [the current handoff](../../../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13). Earlier
stopped observations and their origin-specific policies remain historical evidence.

The first version-2 invocation failed before its ready-picture capability and
needed forced cleanup. Its allowance is consumed. The driver-only evidence repair
adds `session_failure` and `cleanup_failure` to normalized result JSON, with fixed
phases and optional last validated snapshot/resource samples. Each sample has its
own phase and relative time; it does not describe later cleanup unless sampled
then. Missing samples remain unknown. Unknown fields, raw trees/native output,
locators and target frames are never retained. Existing actions and deadlines
are unchanged. Rebuild target-smoke for this repair and record its new hash;
the helper and target-native remain unchanged. A future target attempt needs an
unlocked desktop and a new separately reviewed bounded assignment. The later
post-repair invocation at 01:58 UTC on 2026-11-13 passes eight target capability/
cleanup checks, including playing cancellation at 87.7575ms and graceful exit
without force. Target loading cancellation remains not-tested in that single-open
session. Independent outcome and final adapter reviews support owner M1 acceptance;
the original failure remains preserved with unknown cause.

## Paired-event output preparation (OUT-001 / OUT-002)

These commands prepare synthetic physical-output verification under
[0007](../../../specs/0007-output-verification/spec.md). They do not measure
physical sound, synchronization or rate by themselves, and do not accept M1.
Use the current handoff for independent review status before a native session.
The original fixture directory and original smoke oracles remain unchanged.
Under [ADR 0006](../../../docs/adr/0006-m1-measurement-scope.md), calibrated
physical sync/rate measurements are outside M1 scope with no mandatory later
check. They were never performed; the tools below remain optional.

With separately available FFmpeg/ffprobe, generate into a **new** ignored
directory whose parent already exists:

```sh
go run ./experiments/playback/cmd/output-fixtures \
  -out .agent/m1/output-fixtures-r1 \
  -ffmpeg /opt/homebrew/bin/ffmpeg -ffprobe /opt/homebrew/bin/ffprobe
```

Generation records exact tools, media, recipe, decoded timestamps and acoustic
ID evidence in `timing.json`, plus the compatible server `manifest.json`.
Validation compares actual decoded combined and separate tracks and preserves
failed staging artifacts. Existing destinations are rejected. The generator's
one-frame pair bound validates fixtures; it is not a player-sync tolerance.
The digital amplitude is moderate, but does not guarantee speaker loudness.
Choose a comfortable system volume manually; no command changes it.

Build the unchanged replacement app and retain its exact candidate/build and
SHA-256 evidence. The observation launcher requires **absolute paths** and
expected hashes from that reviewed evidence; it checks the runtime manifest
and every listed library/dependency, both fixture manifests, and media bytes:

```sh
go build -o .agent/m1/output-native ./experiments/playback/cmd/replacement-native
go run ./experiments/playback/cmd/output-observe \
  -app /absolute/path/to/output-native -app-sha256 EXPECTED_APP_SHA256 \
  -runtime /absolute/path/to/runtime-manifest.json -runtime-sha256 EXPECTED_RUNTIME_SHA256 \
  -fixtures /absolute/path/to/output-fixtures-r1 -timing-sha256 EXPECTED_TIMING_SHA256 \
  -out .agent/m1/combined-steady-1x-r1 \
  -source combined -segment steady -rate 1 -duration 180s
```

This opens one supplied app with one owned loopback server. Use the app's
**HTTP fixture** control for combined media or **Separate tracks** for separate
media, then select the rate manually. **A selected source loads paused: press
Play to start it.** The Play row is below the source, seek, volume and speed
controls; scroll down if it is outside the window. Confirm that the native
position advances after Play. The synthetic picture has brief event flashes
with a static background between events, so continuous motion is not expected.
Launcher labels describe the intended session; they neither operate controls
nor prove which source/rate actually played. Close the app window normally
before the deadline. The default limit
is 180 seconds, maximum 600. Interruption/deadline retains partial evidence;
app cleanup is bounded to three seconds, including forced reaping if needed,
and server shutdown to three seconds. Forced cleanup is a failed session.
Process exit alone proves neither zero native ownership nor 200ms cancellation.
Do not run the old pixel-color or full-capabilities oracle on these fixtures.

Every fresh evidence directory retains `session.json` and a bounded `app.log`.
All physical and unobserved configuration cells start `not-tested`. Never edit
the original session record to turn a configuration label into an observation.
After the session, keep a separate `observations.json` (or signed human note)
bound to its exact session-file SHA-256, with this structure:

```json
{
  "session_sha256": "EXACT_SESSION_FILE_SHA256",
  "observer": "identified human or named instrument",
  "observed_at": "actual ISO-8601 time",
  "source": "combined or separate, actually selected",
  "segment": "steady or post-seek; actual start/end and seek target",
  "selected_rate": 1,
  "rate_readback": null,
  "video_streams": null,
  "audio_streams": null,
  "statement": "actual observation, or explicitly not observed",
  "evidence": "authorized local record and its hash, if any"
}
```

Fill each field only from the actual identified session; retain `null` for
unknown readbacks/counts. Record separate statements if observers differ.
A human's explicit statement that the Mac speakers produced the expected tones
is scoped audibility evidence only. The agent cannot attest that it heard them.
Do not prefill a successful observation or equate HTTP request counts with
selected native streams. A positive human statement needs its own review.

For each of combined and separate media, retain steady **1x, 0.5x and 2x**
segments plus a **1x post-seek** segment (for example seek to 2s). Each measured
segment needs at least four correctly identified visual/audio event pairs.
Restart the eight-second fixture as needed, keeping each continuous segment
distinct. Record startup/transient intervals and all failures separately;
never drop points or join across seeks/rate changes to improve a result.

The following physical capture procedure is optional reference material, not an
M1 requirement or a mandatory later check under
[ADR 0006](../../../docs/adr/0006-m1-measurement-scope.md). These measurements
were never performed. Any future voluntary physical capture needs separate
explicit device/session authorization. The
launcher never activates a microphone, camera, screen or system-audio recorder.
An external device must observe display and speakers together, with actual
capture timestamps, acoustic placement, an independent A/V calibration reference
and bounded uncertainty. Qualify acoustic IDs in the same capture mode using
reference-only and known-ID controls, or document independent identification.
An unsupported/missing-fundamental recording cannot supply accepted audio IDs
merely because its frequency ratio resembles one.

Keep raw synthetic recordings outside Git. Retain the capture/device/mode and
calibration provenance, session and fixture hashes, all event IDs/onsets with
uncertainty, segment boundaries, rejected observations and actual visual/audio
rate windows. Positive offset means audio arrives after picture. Report raw
capture offset `O`, independent capture skew `C`, physical offset `S=O-C`,
decoded fixture offset `D`, visual-time mapping `g` (at least four events),
fixture contribution `F=g(audioPTS)-g(videoPTS)` and residual `R=S-F` separately.
The mapping comes from physical visual events, not selected/native rate;
report audio rate independently. Use the interval-valued analysis helpers in
`outputprobe`; see its README and the spec for input/provenance requirements.
Missing calibration or a valid mapping leaves derived results inconclusive.
No numerical physical player-sync/rate PASS tolerance has been approved.

## Explicit legacy mygo preview and smoke stages

Run from the repository root with the pinned Go version and normal testing
prerequisites. These commands do not access a real account or target service.
Ordinary `go test` uses fakes, headless UI and loopback fixtures, not a GUI,
native decoder, FFmpeg or external network. Initial module download requires
network access; fixtures are checked into this experiment with their hashes.

The commands in this section explicitly select the legacy mygo player, whose
limitations and historical failures remain unchanged. Default libmpv commands
are at the top of this guide.

```sh
export GOTOOLCHAIN=local GOWORK=off
go run ./cmd/dev check --race
mkdir -p .agent/m1
go build -tags e2e -o .agent/m1/mygo-native ./experiments/playback/cmd/mygo-native
# This legacy driver defaults to the strict full suite (fails delayed loading).
go run ./experiments/playback/cmd/smoke -app .agent/m1/mygo-native
# Explicit bounded fixture preview; NOT a full/M1 acceptance result.
go run ./experiments/playback/cmd/smoke -suite preview -app .agent/m1/mygo-native
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
need separate evidence. Windows is optional P2 under
[ADR 0005](../../../docs/adr/0005-platform-priority.md); any future Windows
support claim needs actual Windows execution, not cross-builds. Missing header/separate-track support is a failed capability even when
its safe unsupported rejection passes.

The adapter permits mygo's initial play to obtain a first native picture, then
pauses/resets before publishing Ready unless Play was requested. Loading
Pause/Seek intent is queued until that picture. This startup behavior is unit
tested; exposed local/HTTP evidence is in the historical M1 report and the
current provisional report. It does not repair slow-load responsiveness.

For a manual local-file window, without the test-only automation plugin:

```sh
go run ./experiments/playback/cmd/mygo-native
```

Normal builds do not import the automation plugin. The smoke itself serves only
the three hash-verified generated media files from `../testdata`, with no
forwarding, proxy, remux, custom decoder or Bilibili SDK. Existing FFmpeg is an
explicit offline fixture generator only; see `../testdata/README.md`.

## Replacement probe (RPL-001 and RPL-002)

Current result, 2026-11-12: the explicit external-audio selection repair passes
the first complete macOS fixture capability invocation with 41 passing
assertions and two explicitly not-tested categories. Separate-track playback,
external playing cancellation (71.583667ms), missing-audio failure and graceful
window/socket/server cleanup now pass, with no forced cleanup. The repair
enables `autoload-files` only for explicit `AudioURL`; `audio-file-auto`,
`sub-auto` and `cover-art-auto` remain disabled, preventing companion discovery.
Independent selection-repair review now passes with 41 fresh fixture assertions.
A subsequent scoped target-media invocation and final independent adapter assessment
pass; the owner then selected libmpv as the default. The owner accepted M1 as completed on 2026-11-13.
Combined/separate fixture audibility has independently reviewed owner attestations.
Calibrated physical sync/rate measurements were never performed and are outside
M1 scope with no mandatory later check under ADR 0006. Windows remains optional
P2. See the [current handoff](../../../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13).
The build/cleanup failures and pending decisions described in older execution
sections below remain historical records of their own candidates.

This macOS arm64 adapter, now also used by the default command, uses pinned
mpv v0.42.0 client API 2.6,
the existing purego v0.11.1 pin and mygo's public bitmap API. It polls native
events on a control owner and software rendering on a render owner, using two
640x360 buffers at up to 30Hz. Ordinary tests and the explicit legacy app never
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
by this surface stage. Later capability evidence and default selection have
their own records; the original mygo failures remain.


### Historical capability-stage execution (RPL-002)

The dated outcomes below describe their original candidates. For current default
selection and checks, start at the top of this guide and report 0024. The retained
`replacement-native` command still supports this diagnostic capability stage.

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
assertions passed, not final M1 or full separate-track acceptance. Audibility
needs its own evidence; scoped owner attestations are recorded in report 0016.
Measured physical A/V sync and display/speaker playback speed remain not-tested,
outside M1 scope under ADR 0006, with no mandatory later check.
Selected/fetched audio alone never passes RPL-AC-05. Windows
is deferred P2; real-service access, IME, packaging/signing and candidate CI need their
own evidence. Final exact-candidate independent review and RPL-003 remain
separate work.
