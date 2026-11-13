# M1 macOS continuation: delayed-switch diagnosis

- Date: 2026-11-11, Asia/Shanghai. Current run:
  `20261111T000541Z-d757e87e7d8b`; handoff disposition `needs-human`, not M1 acceptance.
- Source: uncommitted `codex/m1-macos-playback`, base
  `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600`. Base HEAD is not the candidate.
- Current implementation aggregate:
  `92c93e1f60f960893edb259ebe96d1c342a256e8a64d345ac488ce10c4a5b9fa`.
  Same path/mode/hash algorithm as the preserved initial handoff below;
  `evidence/implementation-switch-final-r0.json` lists the exact files. Full
  code review snapshot is `.agent/review/m1-switch-code-r0/`; the final report-only
  snapshot is `.agent/review/m1-switch-handoff-final/`. Exact manifest/archive
  hashes are attached to this run, not inferred from dirty HEAD.
- Independent `/root/m1_code_review` approves the narrow generation-oracle repair
  and honest partial handoff with no new actionable findings; runtime/M1 remains
  `needs-human`. The final report-only delta receives separate confirmation.
  Prior approvals below remain bound to their earlier candidates.
- Current environment: global Homebrew Go1.27.3, darwin/arm64; macOS 27.2.1
  build `26B131`. The earlier OS and occlusion observations below are historical.

## Current conclusion

**Window occlusion is no longer the observed blocker.** An unchanged-candidate
retry in closed run `20261111T000315Z-0f75e42c571a` and this run both report
`WindowHidden=false`, capture the distinctive synthetic video picture and pass
native position progression, pause stability and paused seek for local/HTTP
media. Both full smoke runs still fail during delayed loading.

The stall is now localized, not unexplained: the pinned mygo
`ui/video/avf_darwin.go` `Interval` synchronously calls
`-[AVAsset tracksWithMediaType:]`. `ui/engine_video.go` `advance` evaluates that
call even while the asset is Loading. An owned-process stack sample records
the tracks/AVURLAsset/dispatch wait chain, while RPC timings show a prompt delayed
action and first tree followed by a blocked tree. See current-run
`evidence/owned-native-sample.txt`, `diagnostic-driver.log` and
`diagnostic-native/result.json`. These are causal diagnostic evidence, not a
passing media run or proof that every historical timeout had the same cause.

No contract-preserving runtime fix is retained. The next step is a reviewed
architecture evaluation of upstream nonblocking scheduling and independently
observable status, not another longer timeout or an occlusion workaround.

## Attempted repair, rejected

A view-only gate waited for polled positive Duration before creating the video
surface, rather than waiting for Ready (which itself needs a first surface).
Its failing-before/passing-after unit regression and exact source are preserved.
Native switching and loading/playing cancellation then passed, but protected
HTTP failure did not publish the required native Media error within10s.

The gate suppresses the only refresh path for public `VideoPlayer.Err`, which
reads cached `videoEngine.err`: the cache is updated by `advance` during video
surface construction. A zero-duration failure never reaches that path behind
the gate. This is a new failure, not a permissible replacement of Media by
Timeout. The author restored both UI files to the previously reviewed baseline.
No module pin/cache, upstream code, unsafe internals, background native call,
decoder, proxy or acceptance deadline changed.

Rejected-gate implementation:
`335a29c97aab72b4ade498a2d39c2e9b3d719527b5682fecd239ded017dfea11`;
archive/manifest: `.agent/review/m1-metadata-gate-diagnostic/`;
patch/identity: current-run `evidence/metadata-gate-rejected.patch` and
`metadata-gate-rejected-identity.json`. This source is **diagnostic only**, not
the final maintained implementation or an accepted repair.

## Retained change and checks

Only four smoke-driver files differ from the previously reviewed implementation:
`client.go`, `client_test.go`, `media.go` and `hidden.go` in
`experiments/playback/cmd/smoke/`. Native player/UI/module/contracts are unchanged.
Action replies can precede their UI frame; old state-only predicates could reuse
an earlier session's Ready or identical failure. One actual attempted-gate run
recorded headers and separate both at generation7. A deterministic failing
regression also reproduced old Ready generation7 being accepted for replacement8.

Every source-open predicate now requires a newer generation. Cancellation starts
only after observing a fresh Loading/Playing generation and must retain that
generation when the owned engine disappears. Tests cover stale Ready, Loading,
Unsupported and Media observations, including unchanged-generation timeout.
Failed native stages now have explicit labels and sanitized state diagnostics.
The original assertion bounds, Media-error oracle and safety limits are unchanged.

- Failing regression: `evidence/generation-before.txt`.
- Passing targeted race tests: `evidence/generation-after.txt`.
- Full recorded `go run ./cmd/dev check --run
  20261111T000541Z-d757e87e7d8b --race`: PASS (specs/format/tidy/vet/race/build).
- Maintained native run: `evidence/native-final-r0/result.json`; overall FAIL,
  specifically `delayed loading before switch: automation response absent,
  oversized or timed out`. App SHA-256:
  `e9935dd432e1b4cdc06c7b9c63bc5da0f70edf8c17660288ae434d416b741b98`.
  The binary matches the baseline because only the separate driver changed.
- Rejected-gate repeat with the repaired driver:
  `evidence/native-gate-strict-r1/result.json`; overall FAIL at protected
  generation9 still Loading/owned within the10s bound. It separately observes
  headers7/separate8, loading cancel52.205417ms and playing cancel52.605333ms.
  App SHA-256:
  `cd599e865f155a89c3f9839d7ee3f12e1f6acbcb0f0e46cdd903508815370851`.
  These diagnostic positives do not validate the restored baseline's switching
  or cancel capabilities. The first attempted-gate failure is also preserved
  in `evidence/native-fixed-r0/`, never overwritten by the repeat.

## Current capability matrix

| Capability | macOS current maintained candidate | Windows |
| --- | --- | --- |
| Native window and injected Chinese | pass; real IME not-tested | not-tested |
| Local/HTTP native synthetic picture and progression | pass; captured colors, at least300ms progress/2s | not-tested |
| Pause stability and paused local/HTTP3s seek | pass; zero drift/500ms, seek within500ms | not-tested |
| Native HTTP Range; volume/rate configuration | pass; configuration is not actual speed/audio proof | not-tested |
| Delayed Loading responsiveness and switch | fail; synchronous native tracks query blocks UI | not-tested |
| Loading/playing cancellation, later negative probes and graceful full-smoke exit | not reached in final native run; earlier results below remain historical, diagnostic gate results are not final-source proof | not-tested |
| Headers and separate-track playback | fail capability at unchanged pin; typed safe rejection unit-tested, final native run stops before these probes | not-tested |
| Owned server cleanup on failed native run | pass; Active0; app forcibly reaped by driver, not a graceful full-smoke pass | not-tested |
| Deterministic contract/UI/server/driver checks | pass; full local race gate | not-tested |
| Live target access, real IME/audio/speed/sync, packaging/signing/candidate CI | not-tested | not-tested |

## Review and next action

Independent `/root/m1_architecture_review` confirms no qualifying public status
refresh at this pin under the contract, recommends restoring the pre-gate UI and
returning to architecture. Its bounded consultation is attached as
`evidence/architecture-status-consultation.md`, SHA-256
`b205d2948bfb5874a3df16677f2a9f8cd6681f87a4c93ddcf00bc834df0753e1`.
This is not final code or adapter approval.

Independent `/root/m1_code_review` verifies the exact79-file source/archive,
seven-path delta and restored UI bytes; the lower initial handoff is unchanged.
The original old-Ready regression fails independently, and corrected generation
and deadline scenarios pass20 race repetitions. Full independent race checks pass.
A fresh independent owned native replay rebuilds the same `e993...` app and
reproduces exposed picture/controls followed by the labelled delayed-loading
failure, serverActive0 and owned-process reaping. Review report:
`evidence/switch-code-review-r0.md`, SHA-256
`dc9928007a96658d65e825b56288aaa42cf9ee45885e2b70086b404de3d8da0b`.
The reviewer approves only the retained oracle correction and honest partial
handoff, with no new actionable finding; M1-CODE-001 remains independently closed.
The known runtime failure is not repaired or waived by this verdict.

No author self-approval, final player decision or PLAY-AC-04/M1 acceptance exists.
Controller records the final report-only confirmation and closes this local run
`needs-human`; that is an execution disposition, not a merge or milestone gate.
The smallest next runtime step needs a separately reviewed bounded contract for
upstream scheduling/status repair, or a separately authorized adapter evaluation.
It must preserve prompt loading,100ms cancellation,
native Media failure, rendered picture/control oracles and exact-source review.
Do not install the rejected gate, fake a hidden test host in runtime, relax
deadlines, patch the module cache or silently switch engines. Windows and
authorized target/header/separate-track evidence remain required before M1.

The author has not committed, pushed, published, changed CI/check/workflow policy,
accessed Bilibili/accounts or started the full client. Local `.agent/` evidence,
archives and reviewer scratch are ignored and not a backup or remote attachment;
preserve needed artifacts before deleting this checkout. The initial handoff
below is retained verbatim as a historical snapshot, not the current blocker.

---

# M1 macOS interim handoff

- Date: started 2026-11-10; continuation 2026-11-11, Asia/Shanghai.
- Run: `20261110T154903Z-e3a64b13ab67` (local development evidence).
- Base: `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600`; uncommitted changes on
  local branch `codex/m1-macos-playback`. The base is not the changed candidate.
- Implementation identity: `466353703d1de00f6ca3c4a6a542bdf8abf3c849bece8b6767ee1c94dd82983e`.
  This is SHA-256 of compact, key-sorted JSON of path-sorted
  `{path, mode, sha256}` entries for `go.mod`, `go.sum`, all files under
  `experiments/playback/` and `specs/0002-playback-spike/`. The retained file
  list is `evidence/code-identity-final-r1.json` under the run. Handoff/navigation
  documents are outside that implementation set to avoid a self-hash cycle.
  The full tracked/nonignored-untracked review snapshot is separately archived
  under `.agent/review/m1-code-r1/`; the final status-only handoff snapshot is
  `.agent/review/m1-handoff-final/`, with hashes in the controller checkpoints.
- Execution: partial macOS harness and controlled negative/lifetime probes;
  handoff disposition `needs-human`. Visible playback acceptance is blocked,
  not passed. M1 remains incomplete.
- Architecture review: independent `/root/m1_architecture_review` approved
  repaired round 1 before freeze; M1-ARC-001/002/003 closed by that reviewer.
- Exact-candidate code/native-evidence review: independent `/root/m1_code_review`
  closed M1-CODE-001 after round-1 verification; no open code findings remain.
  The narrow repair and honest partial handoff are technically approved, not
  final native/M1 acceptance. No author self-approval, final adapter decision,
  policy adoption or merge claim.

## Authority and frozen contract

The owner authorized serial macOS steps 1/2 and independent review, deferred
Windows execution to 2026-11-11, and delegated bounded implementation. This
did not waive Windows or authorize commits, push, publication, workflow/CI
changes, live accounts, full client implementation or another playback engine.
The separate P0 planning in `specs/0004-client-p0/` is preserved, not implemented.

Frozen `0002-playback-spike` inputs:

- Spec SHA-256: `601d826af3494ec05b6220335d7488e05955047d01cca9120ec52b8b453e271c`.
- Tasks SHA-256: `2f61ed0f28ebb2efb8f50e5c7c5c9844281f351fdb2bcc9fe7978700a288c595`.
- Architecture manifest SHA-256: `379737bbeec22279fce4cae98de4d3347ac7b9ff9413fd44950fd9559f86f5a3`.
- Architecture archive SHA-256: `b8e789c7915a9b89a582e622e1d1f7118d3ee80183e09ce74d95bd6deb752ffb`.

The original review requested fixes to module ownership/tidy sequencing,
experiment-versus-upstream thread/tick guarantees, and positive native-playback
oracles. All were independently closed before the run began. Their reports
are retained unchanged in reviewer scratch and the approved report is attached
as `evidence/architecture-r1.md`.

## Delivered

- PLAY-001: framework-free inputs/commands/snapshots and polled controller;
  deterministic limits, stale-generation, cancel, timeout, errors and disposal
  tests; original synthetic combined H.264/AAC MP4, video MP4 and audio M4A,
  with explicit Go-driven generation and codec/duration/size/hash manifest.
- PLAY-002: pinned mygo with the first real UI import, minimal native Go
  window, headless Chinese insertion/controls/error tests and test-build-only
  automation. Real macOS native window creation, injected Chinese, command/error
  observation, app-only capture and graceful exit were exercised separately
  before media integration. Insertion is not real IME composition.
- PLAY-003: mygo system-media adapter, 20 Hz experiment-owned polling,
  loopback-only hash-verified fixture server, Range/protected/delayed/failure
  routes, bounded concurrency/timeouts/shutdown and Go native smoke driver.
  Positive render/progress/pause/seek assertions are implemented, but not passed
  in this runtime. Independent asset/negative/lifetime probes did execute.
- PLAY-004: this partial matrix and continuation instructions. No final
  `0004-playback-decision` ADR or final M1 acceptance is issued.

The code remains a single Go module and an experiment, not the desktop shell.
There is no Bilibili SDK, account/API access, application JavaScript, media
proxy, runtime FFmpeg, remux, custom decoder or silent second player engine.

## Environment and inputs

Homebrew global Go: `/opt/homebrew/bin/go`, `go1.27.3 darwin/arm64`.
The ignored older project-local toolchain is not selected. No global shell
configuration was edited. Runtime: macOS 27.2 arm64, build `26B123`, Xcode/clang.
The existing FFmpeg 9.0.1 was used only for explicit offline fixture generation.
All three generated files are eight seconds, at most 640x360 and below 2 MiB.

mygo pin: `v0.8.4-0.20261110145816-daf6341a2fe0`, commit
`daf6341a2fe05f62dc8519e7cc215a90c757e8e0`.
Module sum: `h1:TK69mv6glYKEjaq3LqeRhwMbbqZPXfwx0iz5SejgS2M=`;
go.mod sum: `h1:Hw9sxdSQ4tHuehHw5Q6alR1qTS8kgv3oSFxEejg+7qo=`.
Root license is MIT; this does not approve every transitive license or a
production dependency choice. Exact resolved imports/sums live in module files.

## Verification and capability matrix

Author checks: `go run ./cmd/dev check --run 20261110T154903Z-e3a64b13ab67 --race`
passes on macOS, including all specs, formatting, tidy-diff, vet, uncached race
tests and build. Each serial implementation checkpoint was checked. A self-found
late-Ready deadline bypass has a preserved failing regression
(`evidence/late-ready-before.txt`), a repaired controller and passing strict
deadline/20 Hz regressions; downstream checks/native probes were rerun. Native
commands and constraints are in `experiments/playback/cmd/README.md`.
These are local results, not changed-candidate CI or signed package evidence.

Independent round-0 review reproduced M1-CODE-001: the native driver's `until`
could accept an RPC reply after its assertion deadline. The original report is
`evidence/code-review-r0.md`. Round 1 now bounds RPCs and waits by a child deadline
no later than the parent/assertion end and rechecks before accepting success.
Delayed-positive, already-buffered-but-late, canceled-positive, on-time and
shorter-parent regressions have preserved failing-before/passing-after logs
(`evidence/assertion-deadline-before-r1.txt` and `assertion-deadline-after-r1.txt`).
Full race checks pass; no acceptance deadline or frozen contract was weakened.
The original reviewer independently verified the old-source failures, original
reproduction and 20 race repetitions on the repair, then closed M1-CODE-001 in
`evidence/code-review-r1.md` (SHA-256
`272def13ce5323c2e2e65c845f0376f9514a4e1b003c37a16dfc19f8e68d4f69`).

The stage-2 native result is `evidence/ui-stage2/result.json` and `ui.png`.
Its app SHA-256 is `3f3bb781032992220a4234aeaa210953dd54d24e017ee4f0b3145bb68a93697a`;
its exact source archive/manifest are under `.agent/review/m1-ui-stage2/`.

Media attempts `evidence/media-r0/` through `media-r3/` retained their failures:
positive duration could be read, but native Ready/picture assertions timed out.
A reduced same-framework diagnostic also reported `WindowHidden=true`,
zero picture size and an advancing clock. Explicit Show/Focus did not establish
an exposed window. Do not infer the screen is locked, or that the decoder is
broken: the session/occlusion cause is not established.

The pre-review occlusion-aware `evidence/media-review-r1/result.json` recorded
independent probes and exited nonzero, not as a successful playback smoke.
The reviewer independently rebuilt the identical app and reproduced that result
in `.agent/reviewer-scratch/m1-code-r0/native-smoke/`. The app SHA-256 is
`e9935dd432e1b4cdc06c7b9c63bc5da0f70edf8c17660288ae434d416b741b98`.
The pinned framework skips native Frame while occluded. The harness therefore
does not convert duration/clock/status labels or an offscreen UI capture into
rendered-playback evidence.

After the strict-deadline repair, `evidence/media-code-r1/result.json` failed with
`context deadline exceeded` after loading-switch evidence and before any recorded
loading-cancel success. The exact failing assertion is not established by that
run's generic error. The owned process was forcibly reaped on this error; server
shutdown completed at Active=0. One diagnostic repeat added only sanitized phase
labels, not longer deadlines: `evidence/media-code-r1-diagnostic/result.json`
completed negative/lifetime probes, observed loading cancellation in 1.211292 ms,
and graceful process/socket/server cleanup. Its final smoke still exited 1 as
not-tested for occlusion. Preserve both outcomes: the repeat does not erase the
first failure or establish consistently bounded native lifetime behavior.
The reviewer also independently completed the repaired-driver probes in
`.agent/reviewer-scratch/m1-code-r1/native-smoke/`, observed cancellation in
1.320167 ms and graceful cleanup, then exited 1/not-tested for occlusion.
This additional repeat does not close the unexplained timeout.

| Capability | macOS observed result | Windows |
| --- | --- | --- |
| Native Go window starts; injected Chinese round-trip | pass; OS window and automation, not exposed-video/IME proof | not-tested |
| Deterministic contract, headless UI and server regressions | pass; full local race checks | not-tested |
| Local generated asset access | pass; native duration read, no picture claim | not-tested |
| Loopback HTTP asset access and native Range request | pass; server counts and native duration | not-tested |
| Native rendered local/HTTP picture and positive playback oracle | not-tested/blocked; `WindowHidden=true`, attempts never reached Ready | not-tested |
| Native pause stability, seek and playing cancellation | not-tested; visible native playback prerequisite missing | not-tested |
| Volume/rate configuration readback | pass; 0.25/1.5, not actual speed/audio proof | not-tested |
| Required custom request headers | fail capability; safe unsupported rejection passes | not-tested |
| Separate video/audio tracks | fail capability; safe unsupported rejection passes | not-tested |
| Unadorned protected HTTP / failure route | pass negative probe; native media-failure and disposed engine | not-tested |
| Loading replacement / stale generation | pass; current session stays current beyond delayed response window | not-tested |
| Loading cancellation | pass in diagnostic repeat below 100 ms; earlier strict run timed out before this success, so repeatability is unresolved | not-tested |
| Close, closed controls, owned process/socket/server cleanup | pass in completed negative/lifetime runs; first strict failure required forced process cleanup, not graceful exit | not-tested |
| Real IME, physical audio, actual playback speed and A/V sync | not-tested | not-tested |
| Real Bilibili access, packaging/signing, changed-candidate CI | not-tested | not-tested |

In media-review-r1 and media-code-r1-diagnostic the native server observed plain,
delayed, denied-protected and failure requests, including Range. Both ended with
zero active handlers. Default
Go tests separately verify successful synthetic-header serving, Range contents,
rejection/limits, request cancellation, eight-handler bound and listener shutdown.
Those route tests do not establish native header support.

Thread guarantees cover experiment-owned public calls and polling only. The
pinned upstream AVFoundation implementation has its own 25 ms watcher and
video-frame scheduling; it is not main-thread-only or globally capped at 20 Hz.
The adapter's first-picture priming/pause/reset is unit-tested, not validated
in an exposed native window. No all-allocations/leak-free claim is made.

## Review and remaining gates

Architecture contract approval is not native acceptance. Independent code review
round 0 verified all 79 source/archive paths and full race checks, rebuilt the
same native binary and reproduced the honest occlusion-aware result. Round 1
independently verified the changed candidate, repeated the full race check and
native probes, and closed the one P2 timing finding. The original review and
failures are preserved; technical closure is by the reviewer, not the author.
All final PLAY ACs still require their missing OS/runtime/target-access evidence.
The intermediate report cannot accept the final player or mark M1 complete.

The current built-in player is not demonstrated as a P0 playback solution:
header and separate-track input have no supported public path at this pin,
and positive native-picture behavior still lacks evidence. Keep mygo as a UI
candidate and the plain playback boundary, not a confirmed final stack.
Return to a separately reviewed playback-adapter evaluation before a full
client slice; do not hide these gaps with a proxy, remux or different engine.

## Continue

1. Keep this macOS experiment window genuinely exposed in an interactive
   desktop and rerun the documented smoke into a fresh evidence directory.
   It must pass real picture/progress/pause/seek assertions, not just status
   labels. This may require owner/display access; no unlock was attempted.
   Investigate the preserved strict native timeout and establish repeatable
   bounded loading/cancellation as well; do not extend deadlines to hide it.
2. Obtain the Windows runtime the owner planned for 2026-11-11; pin the same
   source and record Windows results independently. No Windows claim exists.
3. Review the header/separate-track gaps and authorized target-media needs,
   then freeze the next bounded adapter probe. No alternative engine is
   authorized or implemented by this handoff. M2/full P0 implementation waits.

Run state: `go run ./cmd/dev run list`, then `run status --id
20261110T154903Z-e3a64b13ab67`. Raw evidence, archives, binaries and reviewer
scratch are ignored local state, not Git backups or remote attachments.
Preserve the needed sanitized artifacts in access-controlled storage before
removing this checkout. Historical M0 reports remain unchanged. No commit,
push, PR, publication, CI or release operation was performed.
