# Playing-close delay diagnosis

Date: 2026-11-11. Spec: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md).
Controller run: `20261111T151954Z-64b1f354e8c1`.
Scope: owner-authorized diagnosis of RPL-002 finding **CA-CLEANUP-002**.

The dominant delay is in render shutdown. The adapter sends asynchronous quit
and immediately stops servicing frames. Native render-context destruction then
waits for the video-output thread, which is waiting for a pending frame to be
rendered and falls back to its **200ms timeout**. That wait consumes the complete
Close budget before the remaining teardown finishes. Three original-option
playing reproductions took **274.728375–287.707875ms**; render-context free alone
took **232.138875–238.379959ms**.

Diagnosis is complete; no repair was implemented. **CA-CLEANUP-002 remains P1
open**, and the [independent review](0008-coreaudio-cleanup-review.md) still
requests changes. CA-CLEANUP-001 remains technically closed for the reviewed
delayed-unload crash. RPL-002, RPL-003 and M1 remain unaccepted. Separate-track
readiness is still a separate unresolved failure.

## Exact source and runtime

The diagnostic baseline is commit
`d846e667c537c06a1bcf100e6aac9486c29aa3d2` plus the preceding review handoff's
uncommitted README, stage-zero and report 0008 changes. All 117 baseline files
matched that handoff before execution. SHA-256 identities:

| Artifact | SHA-256 |
| --- | --- |
| Baseline full source manifest | `fa05798eb570ff581f940d9c547529493b0f1873d08e012e1c7e95ad378d88ce` |
| Baseline full source archive | `dcc83854242e314d9c9f98bcade7ce6aac814fdec630ba79d532d1763baf4cb8` |
| Frozen spec | `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef` |
| Frozen tasks | `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3` |
| Runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| Repaired libmpv | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |
| Generated combined fixture | `5d9dd937b18d5b350aab2d17f040a67a0ef49857a22f2d21d4dc3d233f82f943` |

The runtime remains `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`, mpv
source `b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6. All 30 runtime
artifacts and 901 native source files were verified against their recorded
manifests. No maintained implementation, spec, task, native source or runtime
artifact changed. This report and current navigation are documentation changes;
the final uncommitted candidate is identified by the run's
`evidence/handoff-candidate/identity.json`, manifest and archive.

## Measurements and method

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Temporary Go tests in
the ignored run evidence use `go -overlay` and the existing injected loader
seam to time quit dispatch, render-context free, core destruction and library
unload. Production call ordering is preserved. Each case uses the existing
generated fixture, a fresh process, the unchanged 200ms assertion, complete
per-session unload, zero-ownership and repeated-Close checks, and eight seconds
of survival after Close. Module networking is disabled. These are author
diagnostic observations, not independent acceptance evidence.

The first fixed batch used original options; the actual gapless setting read
back as `weak`. All durations below are milliseconds. Complete Close includes
scheduling and wrapper overhead in addition to the listed native calls.

| Case | Complete Close | Render free | Core destroy | Unload | 200ms result |
| --- | ---: | ---: | ---: | ---: | --- |
| Paused control | 2.516083 | 0.391458 | 1.756208 | 0.325958 | PASS |
| Playing 1 | 287.707875 | 236.725250 | 48.387875 | 0.886791 | FAIL |
| Playing 2 | 274.728375 | 232.138875 | 40.823084 | 1.513792 | FAIL |
| Playing 3 | 285.815041 | 238.379959 | 46.311959 | 0.856333 | FAIL |

A second fixed batch requested diagnostic-only `gapless-audio=no`. All three
cases still failed: **234.904917 / 235.703584 / 235.922583ms**, with
**234.308083 / 235.098750 / 234.739292ms** in render free. Core destruction
fell to about 0.15–0.22ms, but the dominant wait remained. The option setter
accepted the value; the diagnostic's string-only property readback returned
`unread`, a retained instrumentation limit. The initial audio-drain hypothesis
does not explain or remove the dominant overrun, and this option change is not
a proposed fix. Pinned `player/loadfile.c:2124` also clears audio output buffers
on non-EOF termination, so the remaining core time cannot simply be labeled
buffered-audio drain.

The profiling round retained both invocations. Its first sample missed shutdown
and captured post-teardown threads only; Close was 250.272833ms, with
241.141292ms in render free. One synchronized follow-up began sampling after
playing readiness and released Close after a two-second hold. It caught the
wait, with Close 308.805250ms and render free 227.564209ms. The hold and sampling
overhead make these timings localization evidence only. They are not acceptance
measurements or a sample of the original reviewer's 284.933708ms invocation.

All nine diagnostic cases survived eight seconds with zero owned resources and
successful repeated Close. All eight playing cases exceeded 200ms. Every
failure and the missed sample remain recorded; no result was retried into
acceptance. All owned subprocesses were reaped, with no execution timeout.

## Causal evidence

The adapter's shutdown defer in
[`engine.go`](../experiments/playback/mpvprobe/engine.go), lines 188–197, sends
async `quit`, immediately closes `renderStop`, and waits for `renderDone`.
The render loop in
[`surface.go`](../experiments/playback/mpvprobe/surface.go), lines 35–48,
returns on stop and calls `mpv_render_context_free` from its defer. It no longer
consumes pending frames.

The synchronized native sample captured the resulting wait chain:

| Thread | Captured native stack | Supporting samples |
| --- | --- | ---: |
| Render owner | `mpv_render_context_free` → `mp_dispatch_queue_process` → condition wait | 165 |
| Playback core | `uninit_video_out` → `vo_destroy` → `mp_dispatch_run` → wait | 165 |
| Video output | `flip_page +244` → condition wait | 152 |

In the verified pinned native source, `video/out/vo_libmpv.c:252–286` requests
video destruction and waits while the render context remains in use.
`player/video.c:130` destroys the video output. In `vo_libmpv.c:504–550`,
`flip_page` establishes a 200ms deadline and first waits while `next_frame`
remains unconsumed. Normal rendering at lines 384–398 consumes that frame and
signals the waiter. The file's opening comment describes this render/VO circular
dependency and timeout fallback.

Disassembly of the exact loaded library maps sampled `flip_page +244` to that
**first pending-frame wait**, rather than the later optional swap-report wait:
load base `0x10e398000`, sampled PC `0x10e48c2c0`, function image offset
`0xf41cc`, return PC offset `0xf42c0`. Together with phase timings and the source
ordering, this localizes the dominant reproduced overrun to stopping frame
servicing before native video shutdown is quiescent.

The repaired CoreAudio idle worker is present in ordinary condition waits in
the sample; no dominant idle-worker join was observed. Some later core
destruction samples include `AudioComponentInstanceDispose` and system IPC.
Those observations do not account for every millisecond or prove a worst-case
bound for the remaining cleanup. The diagnosis does not reopen or independently
re-close CA-CLEANUP-001.

## Proposed bounded repair and required verification

The smallest proposed repair is a two-phase shutdown in the Go adapter:

1. Initiate native playback shutdown on the control owner while the render owner
   continues servicing update requests and pending frames. The pinned public
   render API's `MPV_RENDER_PARAM_SKIP_RENDERING` can acknowledge a frame without
   drawing it; evaluate that for shutdown while preserving render-thread rules.
2. Establish and test a native quiescence handshake before stopping the render
   owner. Do not equate dispatching async quit with native shutdown. Evaluate
   shutdown events and final pending-frame handling together: pinned
   `mp_play_files` can retain the global video output until later destruction,
   so receipt of `MPV_EVENT_SHUTDOWN` alone is not proof of disposal. The exact
   handshake remains an implementation question, not a validated solution here.
3. Free the render context, destroy the core, join owned workers, and unload the
   session library before Close returns. Preserve the 200ms complete-disposal
   bound and terminal-state semantics. Arbitrary sleeps, returning Closed early,
   retaining libmpv, or shortening its internal timeout do not satisfy the task.

An authorized implementer should add a deterministic regression in which native
shutdown needs a pending frame to be serviced, plus coverage for loading,
renderer failure, quit failure, repeated Close and cancellation. Then run the
full race check and relevant native cleanup scenarios, including playing-close
timing and eight-second survival, on the exact candidate. Keep every failure.
The assigned independent reviewer `/root/coreaudio_cleanup_review` must verify
the repair before closing CA-CLEANUP-002. This diagnosis does not authorize or
perform that repair or a new reviewer dispatch.

## Verification, evidence and remaining gates

The existing runtime's loader/API/symbol preflight passed. The recorded command
`go run ./cmd/dev check --run 20261111T151954Z-64b1f354e8c1 --race` passed all
six gates with `GOTOOLCHAIN=local`, `GOWORK=off`, the existing local GOPATH,
`GOPROXY=off` and `GOSUMDB=off`. Ordinary checks do not execute the ignored
diagnostic overlay or instrument native C code. The maintained Go bytes were
unchanged through the check and handoff. Documentation targets, whitespace,
source/runtime identities and final evidence hashes were checked separately.

Local evidence root:
`.agent/runs/20261111T151954Z-64b1f354e8c1/evidence/`.
It retains the immutable assignment, baseline manifest/archive, runtime/source
verification, diagnostic sources and binaries, exact invocation/environment
records, per-case logs and phase JSON, both samples and disassembly, full check
log, and final handoff inventory. Key locators and SHA-256 identities:

- `diagnostics/original/summary.json`:
  `460c03f0f0fbc2dfb71967326a43ad966c9fa2fe7a124dedf77b1f94b2d59e46`.
- `diagnostics/gapless-no/summary.json`:
  `0af9ea7ee1c9ba61d275869f48ce985a48cb2732846746406cd2f2609dc37240`.
- `diagnostics/sample-synchronized/sample.txt`:
  `59a9dca6221b8be7e261fab6dace7edbd63e2e2366c9947b539ac70dd8b9460a`.
- `diagnostics/flip-page-disassembly.txt`:
  `a70964799265f58f1fe6432fd1f88cb8af1449420bd323dddc10ef44eef1e511`.
- `diagnosis.json`:
  `2cb082bf488539364e3ce0a717b2be59f282dae2f207c2935f45bb23af06e25b`.
- Diagnostic source/binary: `diagnostics/build.json`; synchronized variant:
  `diagnostics/build-sample.json`; final inventory: `evidence-manifest.json`.

The original independent failure remains in run
`20261111T150612Z-804b080ab456`; its native log SHA-256 is
`ae4fe32a5a4a53109943eeb5f9c9ae2fa3aba6f31ab46a349f6151323a8a8149`.
Historical reports and closed runs are unchanged. This diagnostic run ends
`needs-human` with a concrete repair proposal and the finding still open.

The next step is owner-authorized implementation of that bounded shutdown
repair, followed by independent verification. Separate-track readiness remains
pending; the retained capability run still did not reach playing cancellation,
missing-audio failure or final graceful window close. Windows, physical speaker
output, measured rate/A/V sync, authorized service playback, packaging and
candidate CI remain unverified. No acceptance criterion or policy changed.

Large logs, binaries, native builds and archives remain local ignored artifacts
without guaranteed retention or remote backup. Preserve needed evidence before
removing this checkout. Only this report and current navigation were added or
updated for the diagnosis; no implementation repair, new delegation, commit,
push, PR, merge or remote publication occurred.
