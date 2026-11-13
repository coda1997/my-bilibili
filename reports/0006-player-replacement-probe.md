# RPL-001 replacement-player continuation

Latest continuation: **independent RPL-001 implementation and gate-one review
approved, with no actionable findings**. See
[independent implementation and gate-one review](#independent-implementation-and-gate-one-review).
The runtime blocker is fixed. The original and independent native surface runs
both pass on macOS arm64; cancellation was observed in **72.477291ms** and
**76.1305ms** respectively, within the unchanged 100ms bound, with zero owned
resources. Recommendation: continue evaluation through a separately authorized
RPL-002 assignment. RPL-002/003 remain unstarted. Earlier sections preserve
the original failed execution, diagnosis, repair and their review states.

## Original runtime-blocked execution

Date: 2026-11-11. Outcome: **needs-human; runtime preflight failed before the
native surface gate**. RPL-001 is incomplete. RPL-002 and RPL-003 have not
started. No replacement playback, timely native disposal or M1 acceptance is
claimed. The provisional mygo preview remains the default.

Run: `20261111T122014Z-7452a6b4a989`. The owner continued implementation after
independent architecture approval and authorized isolated local runtime
preparation. The run's `evidence/implementation-assignment.json` records that
later authority; the frozen planning packet's earlier proposed/unassigned
wording describes its historical state. No global installation, commit, push,
remote publication or independent implementation-review dispatch occurred.

The uncommitted candidate is based on
`3ecf7622725033d04aff0135c2c9ac02ca128b08`. Its exact source manifest, archive,
diff and hashes are under this run's `evidence/candidate-r0/identity.json`.
That manifest covers tracked and nonignored untracked source, including this
handoff. Ignored runtimes, tools, binaries, evidence and Git metadata are
excluded from the source archive and identified separately. HEAD plus a dirty
flag is not the candidate identity.

## Delivered and checked

- An opt-in `experiments/playback/mpvprobe` adapter with a private Darwin arm64
  client API 2.6 mapping, explicit absolute-library loading and symbol/version
  rejection. Ordinary tests and the provisional app do not load libmpv.
- Separate control/render owners, asynchronous command replies, cached state,
  a 32-command queue, bounded event draining and out-of-band shutdown. Close
  waits for disposal; terminal failures are published after owned cleanup.
- Software rendering into two aligned 640x360 buffers, capped at 30Hz, through
  the public mygo bitmap API. Draw uses a nonblocking lock and preserves the
  displayed buffer. No native callbacks are registered.
- Separate `replacement-native` and `replacement-smoke` commands. Only the
  `surface` stage is implemented; broader stages reject before artifact access.
  The driver retains original picture/timing bounds, runtime hashes, window
  captures, external cancellation timing, resource counters and forced-cleanup
  status. These native assertions have **not been executed**.
- An optional UI warning retaining the existing warning by default, with a
  headless regression. The existing purego v0.11.1 pin becomes direct; mygo
  remains `v0.8.4-0.20261110145816-daf6341a2fe0`. The plain contract, original
  player, fixtures and legacy smoke implementations are unchanged.
- Command documentation in `experiments/playback/cmd/README.md`.

On macOS arm64 with Go 1.27.3, the recorded command
`go run ./cmd/dev check --run 20261111T122014Z-7452a6b4a989 --race`
**passed** all six gates: spec validation, formatting, tidy diff, vet, uncached
race tests and build. Environment: `GOTOOLCHAIN=local GOWORK=off`, with the
existing ignored local module cache. Log: `evidence/check-3332894304.log`,
SHA-256 `7a348a78b3c5557e1bd7f8d5bf0a77175d23edcc917445ebbc20cf037f7e559c`.
The final source freeze verifies the Go and module inputs against those used
for this check; only this handoff was added afterward.

Deterministic tests cover ABI struct layout, safe missing-library/unsupported
input rejection, asynchronous opening, command reply ordering, queue saturation,
Close waiting for real fake-native disposal, render-before-core teardown,
partial initialization/render failures, terminal-state ordering, pixel buffer
ownership, nonblocking Draw, warning defaults and changed runtime dependencies.
They use fakes and do not establish native ABI or media behavior.

`go build -tags e2e -o .agent/m1/replacement-surface-r0
./experiments/playback/cmd/replacement-native` also passed. Its binary hash is
recorded in the candidate identity. Building it is not native execution.
No CI ran for this candidate.

## Runtime preflight failure

Pinned source: mpv v0.42.0, commit
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, expected client API 2.6.
The downloaded source archive SHA-256 is
`bcfebbfa2b65a6d88dc97ea18117831072373e38b5785a2afb874ed4df5070dd`.
Acquisition, build tools and all outputs stay under ignored
`.agent/runtime/mpv-0.42.0/`. libplacebo 7.387.0 was built locally with GPU
backends disabled, using verified compile-time Vulkan headers. Existing
Homebrew FFmpeg 9.0.1 and libass 0.17.5 supply linked dependencies; they were
not installed or upgraded by this task.

The minimal library compiled and installed locally, but a loader-only preflight
failed with unresolved `_cfstr_from_cstr`. It created no mpv core, render
context, media session or UI. Static source inspection shows the function's
implementation in `osdep/utils-mac.c` is included with Cocoa, while CoreAudio
uses it. The failed binary and its exact configuration are retained.

Enabling Cocoa on the same unmodified source failed compilation because
`osdep/mac/swift.h` was absent. Enabling the documented `swift-build` option
then failed at `video/out/mac/view.swift:32`: `MetalLayer` is referenced, while
its implementation is included only with Vulkan enabled. A rejected configure
attempt using the wrong option spelling `swift` is also retained. None of
these preparation attempts ran the surface gate.

No upstream patch, binding shim, GPU build, suppressed audio output or altered
teardown contract was introduced to bypass this blocker. The installed failed
library is not a usable approved runtime and must not be passed to the native
gate. No successful runtime manifest was issued.

Evidence: `evidence/runtime-preflight/result.json`, with hashes of retained
logs, build options, source acquisition records, license files, Homebrew
receipts/SBOMs and static link dependency inventory. Runtime binary/dependency
file hashes describe the failed artifact, not a loaded playback runtime.
System libraries are identified by their paths and recorded macOS build;
standalone file hashes are not invented for dyld-cache entries. A successful
loaded-dependency inventory remains unavailable because loading failed.

## Capability and review state

| Criterion or capability | macOS arm64 result | Evidence or remaining gate |
| --- | --- | --- |
| RPL-AC-01 Go pins, deterministic loader checks and provenance preparation | Partial; runtime preflight **fail** | Full race check passes; failed artifact/build records retained; compatible runtime and actual loaded-dependency inventory still required |
| RPL-AC-02 embedded local/HTTP picture | **not-tested** | No native gate invocation |
| RPL-AC-02 delayed switch, external cancel within 100ms, graceful close and zero native ownership | **not-tested** | Fake-native disposal tests pass; no native timing evidence |
| RPL-AC-03 controls and retry | **not-tested** | RPL-002 unstarted |
| RPL-AC-04 required headers and session isolation | **not-tested** | Gate-one adapter explicitly rejects headers |
| RPL-AC-05 separate audio, speaker output and measured A/V sync | **not-tested** | Gate-one adapter explicitly rejects separate audio |
| RPL-AC-06 native failure and transport scenarios | **not-tested** | Deterministic failure cleanup only; broader task unstarted |
| RPL-AC-07 exact candidate and interim conclusion | Partial; **needs-human** | Source frozen and author checks pass; implementation/native-evidence review pending |
| Windows, real service, IME, packaging/signing and production suitability | **not-tested** | No corresponding runtime or acceptance evidence |

The independent architecture reviewer
`/root/replacement_architecture_review` approved the planning contract with no
actionable findings. The verbatim report is
`specs/0006-player-replacement-probe/architecture-review.md`, SHA-256
`db0a3531240d04d54b84920996086f5cc1de27328829b931bd01aea7fe365245`.
That review covers the archived planning candidate, not this implementation or
the failed runtime build. This author has not independently approved the code.

Frozen spec SHA-256:
`2dfe8b96838dbae6f99c7b621b84d5cd605a5840bc5e846156a33c2d209a8721`.
Frozen tasks SHA-256:
`ab0e20d71bd66ee76c807e1df8295968db19d98de7767598486e9469b53d5fae`.
Neither was changed during implementation. No policy/check/CI change is
proposed or adopted. Historical mygo findings and M1 requirements remain open.

## Next action and retention

Return the runtime supply/build problem to the controller and owner. Supply a
compatible, explicitly authorized pinned build with its full provenance, or
plan and independently review a bounded build/scope correction before native
execution. Revalidate any changed candidate and record a superseding assignment
and new run where the frozen contract changes. Do not retry the failed artifact
as though it were a usable runtime.

Once runtime prerequisites are satisfied, run the first surface gate and
preserve its first outcome. A native gate failure ends the probe. Successful
gate-one evidence requires separately authorized independent exact-candidate
review before RPL-002 can start. This handoff grants no dispatch or later-task
authority and does not select a production player.

All large evidence, runtime artifacts and candidate archives are local ignored
files, not remote attachments or backups. Preserve them in authorized durable
storage before removing this checkout. README/stage-zero navigation remains
unchanged because it is outside RPL-001's write scope; this report is the
continuation entry for the unfinished replacement probe.

## Runtime repair diagnosis

Date: 2026-11-11. Run: `20261111T124853Z-d5b225b44839`, continuing the closed
run above. Outcome: **repair proposed; independent amendment review pending**.
The owner asked to investigate and fix this report's issue. No reviewer has
been dispatched, and no patch has been applied to the runtime source.

The blocker is a build-source omission. With CoreAudio enabled and Cocoa
disabled, `audio/out/ao_coreaudio_utils.c` calls `cfstr_from_cstr` and
`audio/out/ao_coreaudio_properties.c` calls `cfstr_get_cstr`. Both functions
already exist in `osdep/utils-mac.c`, but pinned `meson.build` includes that
file only in its Cocoa block. Enabling Cocoa leads into the separately
recorded Swift/Metal build failures and is unnecessary for these helpers.

The pinned build defaults to `b_lundef=false`, permitting this unresolved-symbol
artifact to link. A new diagnostic build with `-Db_lundef=true`, CoreAudio on,
and Cocoa/Swift/Vulkan off fails at link time on **exactly those two helpers**.
This gives a direct before-repair regression without starting playback.

Proposed correction inside the existing CoreAudio source block:

```meson
    # CoreAudio uses these CoreFoundation helpers even without Cocoa.
    if not features['cocoa']
        sources += files('osdep/utils-mac.c')
    endif
```

This four-line addition reuses the existing CoreFoundation implementation and
avoids adding it twice when Cocoa already includes it. It preserves CoreAudio
and the software-rendering path. The repaired build should retain
`-Db_lundef=true`; the changed library has **not been built or loaded**, so this
is a source-supported proposed fix, not a successful runtime result.

Evidence under the new run's `evidence/`:

- `baseline-comparison.json`: every source file matches the original exact
  candidate before this report-only continuation; existing user code preserved.
- `diagnosis-r0/result.json`: archive checksum verified; all 901 extracted mpv
  source files match the pinned archive; the retained failed binary's checksum
  matches the prior record. A loader-only reproduction fails on
  `_cfstr_from_cstr`; symbol inspection also exposes `_cfstr_get_cstr`.
- `diagnosis-r0/strict-build-command.json`, `strict-setup.log`,
  `strict-compile.log`, and `strict-result.json`: clean, unmodified-source
  configure PASS and expected strict-link FAIL. Separate ignored build directory;
  no installation, core, render context, media session or UI created.
- `proposal/mpv-0.42.0-coreaudio-helpers.patch` and
  `diagnosis-r0/patch-dry-run.log`: exact patch and successful dry-run application;
  the extracted upstream source remains unchanged.
- `proposal/specs/0006-player-replacement-probe/{spec.md,tasks.json}`:
  unapplied amendment permitting only this build correction, with RPL-AC-08
  covering before/after strict linking, feature settings, symbol/API preflight
  and provenance. The real repository spec validator passes this draft; active
  specs and existing frozen inputs remain unchanged.
- `proposal/assignment.md` and `proposal-identity.json`: bounded independent
  review packet and exact proposed-file/archive hashes. The packet references
  the current source identity in `candidate-r1/identity.json`.

`go run ./cmd/dev check --run 20261111T124853Z-d5b225b44839 --race`
passes all six gates on macOS arm64 with Go 1.27.3. Log:
`evidence/check-1029554763.log`, SHA-256
`4a26e558cbeb4e685f362c87024dbcb55e725820deb945a21c6d674e87dfd6a9`.
`evidence/validation.json` also binds the separate proposed-contract validation.
No maintained Go or module file changed in this continuation; these checks
do not establish repaired native linking or playback. No candidate CI ran.

The feature implementation procedure requires returning contract changes for
review; the active spec explicitly excludes upstream patches. The draft above
is the narrow exception needed here. It adds no trusted automation/check/CI
policy, binding shim, graphics backend or broader playback task. The author
has not independently approved this amendment.

Next action: authorize a fresh independent review of the exact amendment and
patch. If approved, adopt the amended contract in a new frozen run, retain the
patch/manual recipe within RPL-001's existing `mpvprobe` scope, and apply it to
a fresh verified extraction under the ignored runtime directory. Build with
strict symbol resolution, complete runtime/license/dependency provenance, and
verify absolute-path loading, all required symbols and API 2.6. Then execute
the original first surface gate and preserve its first outcome. Its original
100ms cancellation bound, complete teardown requirements and failure stop rule
still apply. RPL-002/003 remain unstarted; native playback, timely disposal and
physical audio remain not-tested. Local evidence remains unbacked-up.

## Approved runtime repair and first surface result

Date: 2026-11-11. Run: `20261111T130513Z-75ca590f3868`.
Outcome: **runtime repair and original surface stage pass; needs-human for
independent implementation/native-evidence review**. RPL-002/003 remain
unstarted. The owner authorized one independent amendment reviewer, adoption
and application if approved, and the original first surface gate. No further
review dispatch, global installation, commit, push or publication occurred.

### Amendment approval and exact repair

Independent reviewer `/root/runtime_build_review` approved the exact bounded
amendment with no actionable findings. They independently reproduced the
unpatched strict-link failure, verified the existing helper's dependencies,
dry-ran the patch and checked exact-once source inclusion with either Cocoa
setting. That review did not execute the repaired runtime or approve this
implementation/native result. Its verbatim report and all submitted evidence
are retained under this run's `evidence/independent-amendment-review/`.
Review SHA-256:
`42158c0c30f049b531ecf306192f6cac3272cf9ac34c72290417ec2b90932679`.

The exact reviewed spec/tasks were adopted and frozen before patch application.
Their SHA-256 values are respectively
`92d8ba98b6dd0540e4b14d0da7c8b36f6dc59926bc102d270e716c89711a1695` and
`6b4be2504238682ff655c1d46a79d86d9109b88748b4c9c86564c64222016722`.
Their historical draft-status wording is superseded by this run's
`evidence/authority.json` and `evidence/assignment.md`; the reviewed/frozen
bytes were not rewritten to change that wording. No check/workflow/CI policy
was changed or approved.

The maintained [exact patch](../experiments/playback/mpvprobe/runtime/mpv-0.42.0-coreaudio-helpers.patch)
and [manual build/provenance recipe](../experiments/playback/mpvprobe/runtime/README.md)
now live within RPL-001's existing write scope. Patch SHA-256:
`53750ee31165d0e188f9f9a6374d8b141b0c08cfc8938d0dcd72c7500aa18e1a`.
Only the four-line conditional source-list addition changes upstream source.
All preexisting Go/module files, UI, binding, plain contract, native driver,
fixtures and original failed runtime artifacts are preserved.

### Build, loader and provenance evidence

The pinned archive was reverified and freshly extracted into ignored
`.agent/runtime/mpv-0.42.0/coreaudio-fix-r1/source/`. Before/after manifests
cover all 901 source files and identify only `meson.build` as changed. Fresh
build and local install directories retain `b_lundef=true`, CoreAudio enabled,
and Cocoa/Swift/Vulkan/GL disabled. Strict configure, compile and local install
all pass; the actual compile and link commands contain the existing helper
object exactly once. Software rendering remains compiled in.

The evidence script initially inspected exported symbols only and therefore
missed mpv's private helper definitions. A separate full-symbol-table inspection
confirms both local text definitions; the initial assertion and correction
remain recorded in `evidence/runtime-build/result.json`. No runtime/source
change or rebuild was needed for that evidence correction.

A bounded loader-only subprocess loaded the new resolved absolute dylib with
`RTLD_NOW | RTLD_LOCAL`, resolved all 12 symbols from the existing Go loader,
and returned API 2.6. It created no core, render context, media or UI. All
30 non-system runtime artifacts were loaded and hashed, including the new
libmpv and 29 unchanged dependencies. Those dependency hashes match the
original runtime inventory. Provenance includes 22 Homebrew package
receipts/SBOMs, source/dependency/tool licenses, acquisition checksums, actual
build commands/options, compiler/SDK/tool identities, before/after source
manifests and dyld image records. System cache images retain paths and exact
OS build without fabricated standalone hashes.

Runtime identity:

- Library: `.agent/runtime/mpv-0.42.0/coreaudio-fix-r1/prefix/lib/libmpv.2.dylib`,
  SHA-256 `9f1b82f810904062da94813fad74361778093fd07fa49882b25041659bf26db4`.
- Runtime manifest: sibling `coreaudio-fix-r1/runtime-manifest.json`, SHA-256
  `b62b8a05ee6aa77eeb71ceb198c0676ade8ab7143a2641e638f29af468028071`.
- Native app: `.agent/m1/replacement-surface-coreaudio-r1`, SHA-256
  `9d11032d11a5e520f78285d19aeb11a4a32efd8dd51633918c5ed6b1091aeb66`.
- Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3, Apple clang 21.0.0,
  SDK 27.2, Meson 1.9.1 and Ninja 1.13.0. Existing libplacebo 7.387.0,
  FFmpeg 9.0.1 and libass 0.17.5 dependencies were retained.

### First native result and current checks

After preflight, the original `replacement-smoke -stage surface` ran **once**
against the fresh e2e app and identified runtime. Its first outcome is preserved
unchanged in `evidence/surface-first/result.json`, SHA-256
`addf3a7f6abf967c0c0170d6cd9b4472262bf4608004999ad48cd79b592f6949`.
The driver exited zero; no retry or deadline/oracle change occurred.

| Original surface assertion | Observed result |
| --- | --- |
| Local synthetic picture inside mygo | **pass**; retained `local.png`, exposed window |
| Loopback HTTP synthetic picture inside mygo | **pass**; retained `http.png`, exposed window |
| Switch during an observed delayed HTTP request | **pass**; replacement Ready in 210.604834ms, new generation still current after 3.1s |
| Cancel during an observed delayed HTTP request | **pass**; externally Closed/Canceled with no owned engine in 72.477291ms, below 100ms; native/worker counters zero |
| Close window during a delayed request | **pass**; graceful exit, zero native/worker counters, automation socket removed |
| Controlled server cleanup | **pass**; zero active handlers; three delayed requests observed |
| Forced cleanup or runtime occlusion | **none** |

Both retained captures were also visually inspected by the author. Native
`DYLD_PRINT_LIBRARIES` evidence is separate from loader preflight:
`evidence/native-loaded-images.json` verifies every non-system image reported
by the actual app against the exact app plus all 30 runtime artifact hashes.
The original log also retains system image paths/UUIDs and delayed-load/unload
diagnostics. This is one native observation on this environment, not a
stress/reliability result or independent review.

`go run ./cmd/dev check --run 20261111T130513Z-75ca590f3868 --race` passes all
six gates. Log: `evidence/check-4015783526.log`, SHA-256
`fac679f2fa59f891c6311c2cce07bbc44f2bbda1a014201688656090119d70ef`.
The e2e native build passes too. No changed-candidate CI ran.
`evidence/validation.json` binds those checks, native inputs, first invocation,
captures, result and loaded-image inventory.

### Exact candidate and remaining gates

Base remains `3ecf7622725033d04aff0135c2c9ac02ca128b08`, uncommitted.
`evidence/candidate-r0/identity.json` binds the 108-file source snapshot used
for the checks and native binary. Its source manifest SHA-256 is
`bbf7eaf8be0e5967b01b390743b62ae5f46b4483642b09994a70a348c0385e60`; source
archive SHA-256 is
`00e02af1824bd68d4ff4f7e92026cc5f0322ad850ee8b45f4647794a1eaa3fd8`.
The final report-inclusive snapshot is `evidence/candidate-r1/identity.json`;
`evidence/handoff.json` binds its final hashes and verifies that only this
report changed after validation. All existing user source work is preserved.
Ignored runtimes, binaries, tools and evidence are identified separately from
source. Exact source archives and large evidence remain local and unbacked-up.

RPL-AC-08 has passing author build/preflight/provenance evidence, and RPL-AC-01
now has a usable identified macOS runtime. RPL-AC-02 has passing first-surface
evidence. RPL-AC-07 remains partial pending independent implementation/resource/
native-evidence review. Amendment architecture approval does not supply that
review or acceptance. Next action is a separately authorized review of the
exact candidate and retained gate-one evidence before RPL-002 may start.

RPL-AC-03 through RPL-AC-06 broader native controls, headers, separate audio and
failure scenarios remain not-tested under RPL-002. Physical speaker output,
actual playback rate, measured A/V sync, Windows, real service access, packaging/
signing and final M1 acceptance remain unestablished. The default preview and
historical mygo findings are unchanged. This successful repair supports further
evaluation; it does not select a production player.

## Independent implementation and gate-one review

Date: 2026-11-11. Review run: `20261111T131708Z-4c1d81985a69`.
The owner requested “please continue, independent review”. A fresh independent
reviewer, `/root/replacement_gate_review`, reviewed the complete RPL-001
implementation and native evidence. Decision: **approve the exact candidate
and macOS arm64 gate one; continue evaluation**. No actionable finding or
unresolved review blocker was identified; no finding ID was opened or closed.

The review covers the ABI/loader, control/render ownership and teardown,
replacement app and smoke assertions, UI warning, module metadata, runtime
patch/provenance and report. It is broader than the preceding build-amendment
review. The reviewer made no candidate changes and did not rebuild the runtime.

### Reviewed identity and retained disposition

Reviewed candidate: implementation run `20261111T130513Z-75ca590f3868`,
`evidence/candidate-r1/identity.json`, SHA-256
`0c81afa5fe0bc87e0b3ade823af07e68756bce0bde44e5c6df9042f3f1651688`.
Its 108-file source manifest SHA-256 is
`98cf877f1afd9ef534dca2b16d7fb8533f2e1d90781cffa5e77cb94949028ff3`;
archive SHA-256 is
`3b1bb901245b05b88172544e84902b3dfb846cb91662f759112820ea47c68358`.
The base commit, approved spec/tasks and runtime identities remain those
recorded in the preceding repair section.

The controller copied the verbatim review and all 28 submitted evidence files
into this review run's `evidence/independent-review/`, verifying every digest.
The structured review is `review.md`, SHA-256
`b9ddb539b3dc025e1d06be1d757208c799c3869850d542b1adf0a13bdb109532`.
Its `evidence-manifest.json` SHA-256 is
`f4b44618a572bc54419a50accf08487487a663e8077d3a65f7cbab9fcc76be0c`.
`evidence/review-disposition.json` records the controller's receipt without
changing the review or its scope.

### Checks actually performed independently

| Check | Result and evidence under `evidence/independent-review/` |
| --- | --- |
| Exact source, frozen inputs and prior evidence | **pass**; all 108 source files/archive members, all 220 retained evidence files and all 30 runtime artifact hashes verified; protected legacy code/fixtures match base |
| Full `go run ./cmd/dev check --race` | **pass**, all six gates; `check-race.log`, SHA-256 `5da49c304919fc73ebd2ba142cbe82205bd62c224de3cd68646fe81542f1137c` |
| Reviewer-authored scratch Go probes | **pass**, race-enabled, count 3; event loss/budget overflow, payload reuse, async reply ordering, destruction/unload completion, sanitized failure, repeat Close and frame alignment; `risk-probes.log` |
| Pinned-header C ABI assertions | **pass**; widths, sizes, offsets, used enums and API 2.6; `abi-check.json`; no native library loaded by this check |
| Source patch/build/provenance audit | **pass**; all 901 original/patched source payloads, exact four-line-only change, strict flags/helper once, symbols and actual licenses/dependencies; `runtime-verification.json` |
| Fresh e2e app build | **pass**; byte-identical to author app SHA-256 `9d11032d11a5e520f78285d19aeb11a4a32efd8dd51633918c5ed6b1091aeb66` |
| Single unchanged native surface invocation | **pass**; exposed local/HTTP captures; delayed switch Ready in 197.899208ms and generation stable after 3.1s; cancel Closed/Canceled/no owned engine in **76.1305ms**; zero native/worker counters; graceful delayed-load window close/socket removal; server Active 0/Delayed 3; no forced cleanup or occlusion |
| Capture and actual loaded-image inspection | **pass**; both author and independent captures inspected; each native log's non-system images match its exact app plus all 30 runtime artifacts; system image paths/OS identity retained separately |

First independent native result: `surface-first/result.json`, SHA-256
`4033d4ef2f63dc190c5cdb68963a617fcf90065bd8756cb3175854419d110a4c`.
It remains separate from the author's first result and was not retried.
The reviewer inspected the author's strict runtime build and loader-only
preflight records; those particular operations were not independently repeated.
The native surface run and Go/ABI/ownership checks above were independently
executed. All results are on macOS 27.2.1 build 26B131, arm64, Go 1.27.3.

### Current gate and next action

RPL-001's exact implementation and gate-one evidence are independently approved.
This supplies the gate-one disposition required by RPL-AC-07; the later RPL-003
final review is still unstarted. RPL-002's broader controls, transport/header
isolation, separate audio and failure scenarios require a later authorized
assignment. The owner/controller's next action is to assign that bounded task.

No product or runtime source changed during this review continuation. Only
this report was updated after the reviewed candidate was verified. The final
report-inclusive source identity and archive are recorded under this review
run's `evidence/candidate-final/identity.json`; `evidence/handoff.json` verifies
the report-only difference and binds the independent disposition. The approval
applies to the exact reviewed implementation; the final report is a controller
summary, not additional independently reviewed code.

Physical speaker output, actual playback speed, measured A/V sync, Windows,
real-service access, IME, packaging/signing, changed-candidate CI and final M1
acceptance remain unestablished. In particular, disabled render target-time
blocking makes synchronization a separate later measurement. Owned-resource
counters describe this adapter's objects/workers, not every OS allocation or
an independent leak detector. Two bounded surface passes do not establish
stress reliability or production suitability. The default preview and
historical mygo issue IDs remain unchanged. No policy/check/CI change, commit,
push or publication occurred. Detailed evidence remains local and unbacked-up.
