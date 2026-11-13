# Bounded libmpv replacement-player probe

> Owner platform amendment, 2026-11-12: [ADR 0005](../../docs/adr/0005-platform-priority.md).
> macOS is required; Windows is P2 / nice-to-have and does not block M1, M2
> or P0. Windows remains `not-tested` until exercised. Historical reviews
> describe their frozen candidates; this amendment changes only platform priority.

Spec ID: `0006-player-replacement-probe`
Status: owner-authorized CoreAudio cleanup repair, 2026-11-11.
The original contract and runtime-build correction were independently approved;
RPL-001 passed independent review in run `20261111T131708Z-4c1d81985a69`.
RPL-002 implementation is present; its independent review remains pending.

## Owner-authorized CoreAudio cleanup repair

On 2026-11-11 the owner requested fixing the cleanup failure after review
CA-CLEANUP-001 reproduced SIGSEGV after Close with zero adapter counters.
This authorizes a bounded local runtime repair in RPL-002 and a fresh frozen
run. Preserve the existing lifetime contract: fully quiesce native work before
per-session library unload, keep CoreAudio enabled, and retain the external
200ms cancellation bound. No process-lifetime mapping or early Close is added.

In addition to the existing build-only helper patch, permit a separately
maintained `mpv-0.42.0-coreaudio-cleanup.patch` changing only
`audio/out/ao_coreaudio.c` at the pinned source. Replace the unjoinable delayed
dispatch block with one owned timer worker using mpv's existing thread/mutex/
condition primitives. Preserve the seven-second idle-stop delay, serialize
reset/start/idle-stop, cancel the pending timer on start, and signal/join the
worker during teardown before releasing its state or unloading libmpv.
Handle initialization failure without joining an unstarted worker. No other
audio behavior, binding/API, dependency pin, plain contract or UI change is
included. This is owner implementation authority, not independent review.

Keep the old runtime and every failure. Apply both patches to a fresh verified
source extraction and use a new build/install prefix with the existing strict
build options. Retain source/patch/build/dependency provenance and loader/API
preflight. Add explicit opt-in Go native regressions that keep each subprocess
alive beyond the original delayed cleanup after close, including paused,
playing, timer-expiry/resume and repeated pause/resume/session lifetimes.
Ordinary Go checks must remain independent of native runtime availability.
After these pass, run one complete capabilities stage with unchanged oracles
and bounds, preserving its outcome. Independent review of the final repair and
finding closure remains separate; no delegation or remote action is authorized.

## Owner-authorized cancellation amendment

On 2026-11-11, after run `20261111T133906Z-38eafa873059` failed external
loading-cancel observation at 100.455083ms against 100ms, the owner explicitly
requested increasing the limit to 200ms. This supersedes only the replacement
probe's external loading and playing cancellation bound. It includes action
receipt through observed Closed/Canceled with no owned engine. Complete disposal,
zero-resource assertions and every other deadline remain unchanged. Preserve
the earlier failure under its original contract; it is not retroactively a pass.
Freeze this amendment in a new run before validating the amended candidate.
The owner authorizes that continuation; this is not independent review or M1
acceptance. Historical architecture-review and assignment packets remain records
of their own candidates. The original controller/player/legacy smoke stay unchanged.

## User scenario and goal

The owner can use the generated-fixture mygo preview, but loading slow media
can freeze it, native failure refresh depends on drawing, and required headers
and separate audio are unsupported. Determine whether **libmpv behind the
existing plain playback boundary** merits a further M1 evaluation while keeping
mygo as the Go UI. This experiment may conclude that the candidate is unsuitable.

First establish native rendering and loading teardown on macOS, then exercise
the existing controls and the missing transport capabilities. A source-level
capability is a hypothesis; it does not count as a native result.

## Scope, authority and non-goals

- Proposed candidate: mpv `v0.42.0`, peeled source commit
  `b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, client API 2.6. Source rationale
  and evidence limits are in [research.md](research.md). This is a fixed probe
  reference, not a claim that it is the latest or a selected production engine.
- Serial, macOS, synthetic local/loopback fixtures only. Windows is optional
  P2, not required for final M1, and remains not-tested until actually exercised.
- Preserve the current mygo pin, provisional preview, default/full smoke,
  historical contracts/reports and closed runs. The new executable and driver
  are explicitly named replacement probes; selecting the old app never loads
  libmpv. No silent fallback or default-player change.
- No full shell, P0/M2, service/account access, remote publication, any upstream
  patch except the CoreAudio source-list and cleanup corrections described here, custom decoder, transcoder, media proxy, JavaScript application layer,
  generic plugin/player framework, GPU integration or packaging/signing.
- Runtime preparation remains opt-in, limited to the authorized local build
  described below. Before native work, supply the library with provenance, dependency and
  license inventory, absolute path and hashes. Do not select an unpinned package
  manager version or treat the source tag as the identity of a binary build.
- The proposed Go binding uses the already pinned `purego v0.11.1` with a narrow
  private C-ABI mapping, subject to ABI validation. Promoting it to a direct
  requirement is the only proposed module metadata change. No wrapper SDK,
  additional module or binding-version change is included. ABI failure returns
  to planning rather than introducing an unreviewed cgo/C shim or new binding.

## Proposed runtime-build correction

This amendment is limited to the RPL-001 preflight failure preserved in run
`20261111T122014Z-7452a6b4a989`, reproduced in
`20261111T124853Z-d5b225b44839`. Keep mpv source commit
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, client API 2.6, existing dependency
pins, CoreAudio output and software rendering. Permit exactly the four-line
addition in the accompanying `mpv-0.42.0-coreaudio-helpers.patch`: include the
existing `osdep/utils-mac.c` inside the CoreAudio source block when Cocoa is
disabled. The existing Cocoa inclusion remains responsible when Cocoa is
present. No C function, ABI mapping, Swift source or player behavior changes.
This exception does not authorize an upstream contribution or global install.

After independent amendment approval, keep the patch and manual build recipe
under `experiments/playback/mpvprobe/runtime/`, within RPL-001's existing
write scope. Apply it only to a newly extracted, hash-verified source tree
under ignored `.agent/runtime/`; preserve previous source/builds/logs. Pin and
record the exact source archive, patch, before/after source manifests, full
build command, compiler/SDK/tool versions and output/dependency/license hashes.
Build in a fresh directory/install prefix with the existing isolated tools and
existing dependencies. Keep `coreaudio=enabled`, `cocoa=disabled`,
`swift-build=disabled`, `vulkan=disabled`, `gl=disabled`, `cplayer=false`,
`libmpv=true`, `--auto-features=disabled`, `--wrap-mode=nodownload`, and add
`-Db_lundef=true` so missing functions fail during linking. No disabled sound,
null audio output, GPU backend, shim or automatic runtime acquisition.

The baseline strict link must fail on the two missing helpers. After repair,
the same build must link, with the helper object included exactly once. A
bounded subprocess must successfully load the explicit absolute library path
with immediate symbol binding, resolve every symbol the existing Go loader
requires and observe API 2.6 without creating a core. Inventory the libraries
actually loaded at preflight, and keep subsequent native-smoke runtime identity
evidence separate. Never pass the failed original artifact as a repaired one.
If another blocker appears, preserve it and return for a further bounded plan;
do not broaden this patch.

Only after successful build/provenance/loader checks may the original surface
gate run. Preserve its first outcome and all original picture, timing, complete
Close and zero-resource assertions. A native gate failure still ends the probe;
RPL-002 still requires separately authorized independent exact-candidate review.
The build correction itself permits no contract, UI, smoke-oracle, deadline or default-engine
change (the separate owner cancellation amendment above changes only that bound), and does not establish physical audio or production playback support.

## Boundaries and lifetime contract

### Plain controller and integration

Keep `experiments/playback/contract` unchanged, including input limits, typed
errors, command validation, generation ownership, 50ms polling and 10s readiness
deadline. It must not import framework, native or transport types. The new
`experiments/playback/mpvprobe` implements its `Opener` and `Engine` interfaces;
its adapter-local `Draw` accepts the plain engine just as the current player
does. `cmd/replacement-native` composes it with the existing UI. A narrow optional
warning in that UI may label the replacement experiment, retaining the current
temporary-mygo warning as the default. No libmpv type enters the UI package.

Controller operations and bitmap presentation remain on the UI thread. Unlike
the old mygo native adapter, this proposed backend has dedicated libmpv control
and render owners, permitted only under the reviewed replacement contract.
It never calls mygo native/video APIs from a worker. `Open` validates/copies
input and queues initialization; `Snapshot` reads an adapter-owned cached plain
state; `Command` validates and queues an operation. None waits for network I/O,
decoding, property reads or command completion on the UI thread. A queued
command is not reported as successful native playback until observed state
confirms it. Serialize dependent asynchronous commands by their reply IDs;
old-generation replies, property updates and frames cannot affect the new one.

### Rendering feasibility gate

Use the public libmpv **software render API**, at the existing fixture size of
640x360, as a deliberately bounded feasibility path. The render owner serializes
render-context calls and does not issue normal client commands. Callbacks only
signal owned work; they never call UI/native APIs, render recursively or block
on the UI thread. Copy a completed frame into a bounded mailbox; the UI thread
alone updates a public mygo `Bitmap` and presents it with `Image`. No private
surface/window pointers, hidden status host, external player window or mygo
video surface is allowed. Source API availability is not proof this path works.

Bound queues to 32 pending commands, 256 copied state/event records and two
640x360 four-byte frames. Command overflow returns a sanitized media failure;
event loss/overflow fails and disposes the session rather than losing lifecycle
events silently. Frames may replace older pending frames. Copy native event
payloads before the next API poll; document ABI widths, alignment, pointer and
callback lifetimes. Native code must not retain Go-managed memory without a
reviewed lifetime strategy. Keep function pointers/library references alive
through callback quiescence. Poll at the existing controller rate; frame
production is capped at 30Hz. No per-frame goroutine or unbounded allocation
queue. No performance inference for HD, HDR, battery use or production playback.

### Teardown feasibility gate

One session owns one libmpv core, its handles, render context, queues and workers.
Switch cancels and fully disposes the old session before a replacement core
starts. Close is idempotent and cannot claim completion while native handles,
callbacks or workers are still owned. Stop command production, signal quit,
quiesce callbacks/render work, free the render context before destroying the
core, join owned workers, and release buffers/handles. No goroutine that simply
hides blocking destruction or leaked background shutdown counts as disposal.

The upstream API documents blocking destruction. **Timely teardown is an open
feasibility question**, not solved by choosing asynchronous load commands.
Before broader work, an externally supervised first-gate executable must show
picture plus switch/cancel/window-close cleanup while the controlled server is
delaying a response. Use the owner-authorized externally observed 200ms cancel
bound, including receipt of the cancel action through the closed/no-owned-engine
snapshot. A stalled close is a failed gate. The driver may reap only its owned
app after its deadline; forced cleanup is recorded as failure, never graceful
disposal. Do not return an early fake closed snapshot to meet the bound.

Gate-one failure ends the probe with rejection/needs-human evidence. Do not add
process isolation, change `Close` semantics, permit overlapping cores, loosen
deadlines beyond the explicit owner amendment or start broader controls/header/track work to evade it. Any such
design change needs a separate scope, architecture review and new frozen run.
An independent reviewer must assess the exact gate-one evidence before task
RPL-002 starts; this packet does not authorize dispatch.

### Input, errors and transport

Retain local-file or numeric-loopback HTTP validation and header limits. Treat
all input as data using typed/list API values, never shell commands or concatenated
mpv command text. Disable user configuration, scripts/autoload, ytdl hooks,
automatic external media discovery, IPC servers and unwanted playlist expansion.
Use only the allowlisted generated fixtures; controlled routes do not redirect.
Record applied settings without logging locators, header values or native error
bodies. This is a fixture-only experiment, not a network sandbox for arbitrary
untrusted media. Do not generalize its access guarantees to real services.

Header values are copied per session and applied before loading either stream.
Configure required custom/referrer headers and separate audio through the
pinned documented libmpv API. The CLI-only singular audio-file alias is not a
binding API. Do not silently drop fields, allow headers to bleed into a later
session, or report a video-only result as separate-stream playback. Missing
capability is explicit Unsupported/fail; initialization/load/output failures
become Media, readiness expiry remains Timeout, cancellation remains Canceled.
Failures dispose ownership before exposing terminal state. There is no automatic
retry; preserve the UI's explicit copied-input retry and generation rules.

## Acceptance criteria

### RPL-AC-01

Record the exact source/API reference, Go and mygo/binding pins, runtime binary
and loaded dependency hashes/build options/licenses, OS/architecture, fixture
hashes and all environment prerequisites. Reject incompatible API/symbols
before use with a safe diagnostic. Ordinary tests and the existing preview
must run without libmpv installed. Runtime availability is currently not-tested.

### RPL-AC-02

The first-gate app displays a captured synthetic local/HTTP picture inside the
mygo window through the proposed render path. During an observed delayed HTTP
request, UI actions still respond, switch cannot revive the old generation after
the 3s delay, and loading cancellation is externally observed within 200ms with
no owned engine. Close/quit releases render/core/callback/worker resources.
Failure rejects the candidate at gate one; configuration readbacks alone fail.

### RPL-AC-03

After gate-one review, combined local/HTTP fixtures reach Ready within 10s;
Play advances at least 300ms within 2s; after 200ms pause settling, drift is no
more than 150ms across 500ms; paused seek to 3s reaches within 500ms of target
within 2s. Sampled sliders, presets, explicit retry, playing cancellation and
generation/closed-command behavior retain their existing deterministic and
native expectations, with external playing cancellation observed within 200ms. Report rate/volume configuration separately from actual
speed, speaker output and synchronization.

### RPL-AC-04

The controlled protected route denies a missing/wrong synthetic header and
plays with the exact required sentinel and synthetic Referer. Observe header
match booleans/counters at the server, native picture/progression/seek and
cleanup. Use values containing commas in a list-encoding regression. A later
session without headers is denied, proving no session carryover. Protected
routes for both video and external audio must receive the required fields.
No header text is logged. The default legacy server behavior remains compatible.

### RPL-AC-05

For video-only plus audio-only generated inputs, observe both server paths,
selected external audio/video tracks and native picture/progression/seek/rate/
cancel/cleanup. Audio fetch or a selected-track flag alone is insufficient to
claim audible playback: physical audio needs an authorized observation, such as
an identified human attestation. A missing/failing audio source fails the
presentation, rather than silently continuing video. Under the owner decision in
[ADR 0006](../../docs/adr/0006-m1-measurement-scope.md), calibrated physical A/V
sync and display/speaker output-rate measurements are outside M1 scope. They
were never performed and remain not-tested, with no mandatory later check.
Native tracks, controls and audibility do not establish measured synchronization
or physical speed; those claims must not be inferred.

### RPL-AC-06

Protected/failure HTTP routes produce sanitized Media failures and dispose the
session within the existing 10s deadline, independently of picture availability.
Delayed loading stays observable before completion; loading/playing cancel,
late events, partial initialization, queue saturation, callback-after-close and
repeat Close are covered. Native outcomes and fake-backend tests are distinct.
No timeout is relabeled Media and no hidden surface pumps error state.

### RPL-AC-07

Produce an exact-candidate capability matrix and independently reviewed interim
recommendation: continue evaluation, reject, or needs-human. Preserve every
failure, stage label, forced cleanup and occluded/not-tested cell. macOS fixture
results cannot accept M1, close historical mygo issue IDs, select the production
player or imply Windows/real-service support. No default-engine change occurs.

### RPL-AC-08

The exact permitted source-list patch applies cleanly to the verified pin,
changes only `meson.build`, includes the existing helper once with CoreAudio on
and Cocoa off, and adds no second inclusion when Cocoa is on. The unpatched
strict build fails on `_cfstr_from_cstr` and `_cfstr_get_cstr`; the patched strict
build and immediate loader/API/symbol preflight pass with complete provenance.
CoreAudio stays enabled; graphics/Cocoa/Swift remain disabled. Preserve all
before/after artifacts and record native playback, physical sound and timely
disposal separately; a successful loader is not a native-surface pass.

### RPL-AC-09

The bounded CoreAudio cleanup patch preserves the seven-second idle-stop and
resume behavior and joins its native timer worker before core teardown returns.
The old runtime fails the isolated close-and-survive regression; the new exact
runtime survives beyond the old delay after paused/playing close, repeated
pause/resume/session use and expiry/resume. Preserve initialization-failure
cleanup and complete per-session unload. Keep the 200ms cancellation bound,
strict native build, loader/dependency provenance, ordinary race checks and
the complete capabilities stage. Report any subsequent stage failure honestly;
native regression success alone does not accept all RPL-002 capabilities.

## Verification matrix

Proposed future commands, not commands run during planning:

| AC | Procedure | Environment | Required evidence |
| --- | --- | --- | --- |
| RPL-AC-08 | Exact patch/source verification; unpatched/patched strict link; absolute-path loader, all required symbols and API 2.6; loaded dependency inventory | macOS arm64, no core/media in preflight | Full commands and retained failures, patch/source/build hashes, config/source list, preflight log and library manifests |
| RPL-AC-09 | Opt-in `nativecleanup` tests, fresh strict runtime build/preflight, full race check and one capabilities invocation | macOS arm64, generated fixtures, explicit runtime path | Failing old-runtime regression, delayed-survival and control assertions, exact runtime/source hashes, unchanged-bound complete-stage outcome |
| RPL-AC-01 | ABI/loader failure tests; manifest validation; ordinary full race check without native library loading | Headless macOS; Windows later | Source/API and binary/dependency manifests; safe failure and check logs |
| RPL-AC-02 | Explicit replacement driver `-stage surface`; capture owned window; delayed switch/cancel/quit probes | Exposed macOS, generated fixtures, approved runtime | Per-probe external timings, native frame captures, resource counters, exit/server/socket cleanup; exact gate-one review |
| RPL-AC-03 | Adapter/controller/UI regression tests and replacement driver `-stage capabilities` | macOS first | Native positions/timestamps and captures; slider/retry evidence; separate configuration-only cells |
| RPL-AC-04 | Header list/copy/isolation tests and protected combined/separate native routes | Loopback fixtures | Server match counts by stream and generation, no-header denial, actual native play/seek/disposal |
| RPL-AC-05 | Track selection and missing-audio tests; separate-stream native controls; authorized audibility observation | macOS; Windows optional P2 | Both route counts, selected tracks, positions/picture and audibility; calibrated physical sync/rate never measured and outside M1 scope under ADR 0006 |
| RPL-AC-06 | Fake native interleavings/cleanup regressions; delayed/negative native probes | Headless and exposed macOS | Error categories, ownership/worker quiescence, 200ms cancel timing; full failing evidence preserved |
| RPL-AC-07 | `go run ./cmd/dev check --race`; exact source freeze and independent architecture/code/evidence reviews | Current checkout; each later OS separately | Candidate-bound logs, independent dispositions and durable report |

The new driver rejects unknown stages before app/runtime/fixture access. Surface
is only gate one; capabilities is the full proposed replacement probe and
requires recorded gate-one approval. Each invocation retains the existing 90s
overall, 3s RPC and 10s readiness bounds; app cleanup is 3s and fixture-server
shutdown retains 3s. Do not split a failed stage into selectively passing
sub-runs or combine results from different candidates. Any allowed repeats
must preserve the first outcome and explain exposure/environment changes.

Only `cmd/replacement-smoke` gains replacement expectations. The existing
`cmd/smoke` full/preview suites, thresholds and oracles are read-only; their
safe-Unsupported expectations describe the old player and must not be used
as a passing replacement-capability test. Reuse fixture server semantics,
adding only opt-in sanitized per-file/header observations as needed. Do not
change ordinary checks, workflow policy or CI to depend on a native library.

## Risks, rollback and unresolved decisions

1. Blocking destruction may disqualify libmpv under the unchanged boundary.
   Gate-one evidence, not an asynchronous API name, decides this.
2. Purego ABI/callback correctness and library dependency identity need review;
   no appropriate local runtime has been verified or installed for this plan.
3. Software rendering may be too costly. This experiment caps resolution and
   frame rate; a future GPU renderer would require a separately reviewed scope.
4. Binary licensing depends on actual build inputs. Distribution is out of
   scope; runtime provenance still must identify its build/dependencies.
5. Authorized real target-media validation remains required for M1. Calibrated
   physical sync/rate measurements were never performed and are outside scope
   under ADR 0006, with no mandatory later check. Windows is optional P2 under
   ADR 0005. Record all unavailable evidence as not-tested, never inferred PASS.

Rollback is to keep using the unchanged provisional app and remove only the
new probe files/module-metadata promotion if later authorized. Preserve evidence
and user work. No automatic runtime acquisition or alternate-engine fallback.

## Review and evidence

Independent architecture review is required before freezing/implementing this
proposal. Review callback/render thread rules, Close feasibility, API encoding,
native library provenance, negative-path observability and unchanged M1 gates.
After explicit implementation authority, create a new controller run referencing
`20261111T002803Z-3eb554cdb1c1`; do not append to that closed run. Freeze the
reviewed contract, checkpoint stage boundaries, and submit exact gate-one and
final candidates to separately authorized independent review. An author check
or local recorder success is not independent approval.

Durable future outcome: `reports/0006-player-replacement-probe.md`, linked from
navigation only after actual execution. Planning evidence and the review packet
are described in [assignment.md](assignment.md). This proposal does not claim
a native result, a selected runtime build, independent approval or M1 acceptance.
