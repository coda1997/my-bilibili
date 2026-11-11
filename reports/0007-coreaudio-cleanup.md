# CoreAudio cleanup repair and replacement capability handoff

Date: 2026-11-11. Spec: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md).
Task: RPL-002, including the owner-requested repair of **CA-CLEANUP-001**.
Outcome: the repaired runtime passes all five isolated native cleanup
scenarios; the complete capability invocation still **fails at separate-track
readiness**. Independent repair review and finding closure remain pending.
RPL-002, the final RPL-003 recommendation and M1 are not accepted.

Repair run: `20261111T144422Z-afb986d84d83`, closed `needs-human`.
Publication continuation: `20261111T145736Z-b287382a9c18`. The owner explicitly
requested updating files, committing and pushing the existing
`codex/m1-macos-playback` branch. The public `coda1997/my-bilibili` repository
and matching remote base were verified before publication. This continuation
adds a durable report and current navigation; historical reports and closed
runs retain their original conclusions.

## Delivered and exact identity

The [cleanup patch](../experiments/playback/mpvprobe/runtime/mpv-0.42.0-coreaudio-cleanup.patch)
changes only pinned upstream `audio/out/ao_coreaudio.c`. A delayed dispatch
block could retain a descriptor in libmpv after core destruction and access it
after the Go adapter unloaded the library. The patch replaces that block with
one owned timer thread using mpv's existing mutex/condition/thread helpers.
Reset, start and idle-stop share a mutex; the seven-second idle delay is
preserved and resume cancels it. Teardown signals and joins the worker before
releasing synchronization/audio state. Thread-creation failure does not join
an unstarted worker. Per-session `Dlclose` remains in place.

The [opt-in native regression](../experiments/playback/mpvprobe/cleanup_native_test.go)
runs bounded child processes for paused close, playing close, pause/resume,
idle expiry/resume and repeated sessions. Every child survives eight seconds
after its final close and checks zero adapter ownership and idempotent Close.
Ordinary Go tests do not load a native library. The
[runtime recipe](../experiments/playback/mpvprobe/runtime/README.md) records
the two exact patches, strict build settings and regression command.

The published source also includes the related RPL-002 transport/header and
track-validation work, opt-in protected fixture routes, capability smoke
stage, and bounded sanitized RPC diagnostics. The original player, plain
contract, UI package, fixtures and legacy smoke remain unchanged. The owner
previously amended only replacement loading/playing cancellation from 100ms
to 200ms; other smoke bounds and disposal requirements remain in effect.
No dependency pin, default player, check policy or CI configuration changes.

The repair was tested as an uncommitted candidate based on
`f22a136dd1b93f98103b998bf883ca053d59d9dd`. Its final 115-file source snapshot
is in the repair run's `evidence/candidate-final/`:

- Manifest SHA-256: `5c1500c6e451a14c0a9f5b2f95531bd0e91c71b8eea880626ef41b005bb34436`.
- Archive SHA-256: `ea34b955a8bbf8b5098c0101b05f51a2ef637ed02a807a0054c8a450711be211`.
- Tested `candidate-native/` manifest: `591c92287ad68055f86ec34fd5327c64d17c457f3e1d4232f8a260b1ba6143f3`.
  Only the command README changed afterward; all tested code, patch, module,
  fixture, spec and task bytes match the final snapshot.
- Frozen spec SHA-256: `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef`.
- Frozen tasks SHA-256: `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3`.

Publication baseline verification matched all 115 files to this snapshot.
Implementation commit `8f697d229e9d4cd97ac7ad89092e680fdc7ebc20` preserves all
115 file contents and executable bits; Git normalizes non-executable local
file permissions to its regular-file mode.
README, stage-zero navigation and this new report are the only subsequent
source changes. The native evidence applies to the matching implementation
bytes, not to an independently approved publication candidate.

Runtime identity is separate from Git source:

| Artifact | SHA-256 |
| --- | --- |
| Cleanup patch | `a19e753cfb85223caf7a6ce15373e7aa96aecb1e52be22e974d70ad7f2568c4a` |
| `coreaudio-cleanup-r1/runtime-manifest.json` | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| `coreaudio-cleanup-r1/prefix/lib/libmpv.2.dylib` | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |
| `.agent/m1/replacement-coreaudio-cleanup-r1` | `112f555c566806b5c0c53323893a0d522f11cc021c03c0e473e98f04c6b7bff9` |

The runtime root is ignored `.agent/runtime/mpv-0.42.0/`. Fresh extraction and
build retain mpv v0.42.0 source commit
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6, and all 29 dependency
hashes. Only `meson.build` from the previously approved helper patch and
`audio/out/ao_coreaudio.c` differ from the original 901-file source tree.
CoreAudio/software rendering and strict undefined-symbol linking remain
enabled; Cocoa/Swift/Vulkan/GL remain disabled. The e2e app is byte-identical
to the previous diagnostic app. The old runtime and failing evidence remain.

## Verification

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands use
`GOTOOLCHAIN=local GOWORK=off GOPATH=$PWD/.agent/gopath` and generated fixtures.
Evidence locators below are relative to the repair run's `evidence/`.

| Check / acceptance coverage | Observed result | Evidence |
| --- | --- | --- |
| Old-runtime cleanup regression, RPL-AC-09 | **FAIL as expected**: Close returned in 3.266542ms with zero counters, then SIGSEGV | `native-regression-before.log` |
| Fresh strict configure/compile/install, RPL-AC-01/08/09 | **PASS**; both exact patches applied with zero fuzz | `runtime-build/` |
| Immediate loader/API/symbol preflight, RPL-AC-01/09 | **PASS**; all 15 binding symbols and API 2.6, without a core/media session | `runtime-preflight/` |
| Paused close, playing close, four pause/resume cycles, idle expiry/resume, three sessions; RPL-AC-09 | **PASS**, all five; eight-second post-close survival, zero ownership, repeat Close and direct 200ms Close check | `native-regression-after.log` |
| Native Close timings | 1.755792–21.229459ms across the cleanup scenarios | same |
| `go run ./cmd/dev check --run 20261111T144422Z-afb986d84d83 --race` | **PASS**, all six gates | recorder check log/event |
| e2e app build | **PASS**; identical to previous diagnostic app | `native-build.json` |
| Complete `replacement-smoke -stage capabilities` invocation | **FAIL** at separate-track readiness; first outcome preserved, no retry | `native-capabilities/result.json` and invocation log/JSON |
| Actual capability-process image identities, RPL-AC-01 | **PASS**; all 31 non-system images match the app and 30 runtime artifacts | `native-capabilities/loaded-images.json` |

The publication continuation reran
`go run ./cmd/dev check --run 20261111T145736Z-b287382a9c18 --race`:
**PASS**, all six gates, including uncached race tests. Its log is
`evidence/check-134617586.log`, SHA-256
`31cc83d3c10fceb8ab25dc7bbb5044d0d2f537f84126f48759afd274235729c3`.
All 46 local Markdown links in the changed guides/report resolve. Native
tests were not repeated during publication because implementation/runtime
bytes are unchanged; no changed-candidate CI result is claimed.

Staged whitespace checking reports 14 single-space blank context lines in
the unified-diff patch. Those are patch markers, not added source whitespace.
All other staged files pass that check. The unchanged patch applies with zero
fuzz to the verified source archive and produces the exact tested CoreAudio
source. `evidence/patch-verification.json` and the retained initial check log
record this distinction; no Git configuration or check policy was changed.

The native suite was compiled with `go test -c -tags nativecleanup` and run
once with `-test.run=^TestNativeCoreAudioCleanup$ -test.timeout=120s` from its
package directory, with the verified absolute runtime path. The equivalent
direct command is documented in the runtime recipe. Test executable SHA-256:
`459e27b1e51cf1ec7c05798f6b0866ea2f999f58d5982c783453d28e5d1e2c0a`.

After successful native compile/link/install, the build evidence script
initially missed private helper definitions because `nm -gU` excludes local
symbols. The original failure remains recorded. Supplemental `nm -U` verifies
both helpers and the idle worker as defined text symbols, each exactly once;
no source/build change or rebuild was used to resolve that reporting error.

## Capability outcome and remaining work

The repaired-runtime capabilities invocation ran for 22.96s and exited 1.
It reached the following outcomes within that single invocation:

| Capability | macOS arm64 result |
| --- | --- |
| Combined local/HTTP picture, progression, pause, seek, slider interaction, close/retry; RPL-AC-03 | **PASS** for the reached assertions |
| Speed/volume presets | Configuration/readback **PASS**; actual playback-speed measurement and speaker output **not-tested** |
| Delayed switch/generation stability and loading cancellation; RPL-AC-02/06 | **PASS**; external cancel 67.470292ms against 200ms, no owned engine and zero adapter counters |
| Negative routes, protected combined playback, header isolation; RPL-AC-04/06 | **PASS** for reached assertions; both separate-stream requests also received required metadata |
| Separate-track readiness/playback; RPL-AC-05 | **FAIL** at readiness; no separate-track playback success |
| Playing cancellation, missing-audio failure, final graceful window close | **Not reached** after separate-track failure |
| Physical audio, measured A/V sync, Windows, real service, packaging and candidate CI | **Not-tested** |

Failure: `separate ready: context deadline exceeded (state=failed error=media-failure duration=0s owned=false)`.
The adapter had already published Media failure with no ownership while the
driver waited for Ready. Both protected video/audio requests were allowed.
The cause of this readiness failure is not established by the cleanup repair.
The driver forcibly reaped its owned app; the PID was then absent and the
server had zero active handlers. This is not a graceful full-stage exit.

Historical failures remain failures: run `20261111T133906Z-38eafa873059`
observed loading cancellation at 100.455083ms against the original 100ms;
run `20261111T140848Z-c0ee85eaae70` passed the owner-amended cancellation
bound but failed later with an automation write error. Diagnostic run
`20261111T141629Z-f512d285fa7e` traced the native delayed-cleanup crash and
improved diagnostics. Those results are not combined into a passing stage.

## Review disposition and next action

RPL-001's independent architecture/build/surface approvals are recorded in
the [historical replacement report](0006-player-replacement-probe.md).
They cover their own candidates. The current Codex context independently
reproduced the diagnostic candidate's crash and opened **CA-CLEANUP-001, P1**,
then became the implementation author when the owner requested the repair.
It therefore cannot independently approve this repair or close that finding.
The original review remains at
`.agent/reviewer-scratch/coreaudio-cleanup-review-r0/review.md`; its reviewed
source manifest is `5fb24ca8d982134cffa5d1e67687d414d8cdfb563bde576ce87fd19c2850938c`.
The repair evidence is author verification. No new reviewer was dispatched,
and no workflow/check/CI policy approval is claimed.

The owner/controller's next technical action is to assign a separate
authorized review context the exact cleanup patch, locking/initialization/join
paths, regression evidence and failed capability result. Independent closure
of CA-CLEANUP-001 remains required. Then scope the first separate-track
readiness failure for diagnosis, preserving the 200ms bound and complete
teardown contract. Required Windows, physical-audio/sync and target-service
evidence still precede final M1 acceptance. The default provisional preview
and historical mygo issue dispositions remain unchanged.

Large logs, captures, binaries, runtime builds and source archives are ignored
local artifacts, not remote attachments or Git backups. Their entry point is
the repair run's `evidence/handoff.md`; publication evidence is in the new
continuation run. They have no guaranteed retention beyond this checkout.
Preserve required evidence in authorized durable storage before removing it;
a reviewer without those artifacts must keep the evidence gate pending.
