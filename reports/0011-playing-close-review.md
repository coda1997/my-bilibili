# Independent playing-close verification

Handoff date: 2026-11-12, Asia/Shanghai. Verification ran on 2026-11-11 UTC;
run timestamps and the independent review use UTC.
Spec: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md).
Controller run: `20261111T155210Z-7e16351c96f2`.
Owner request: “ok, verify,” following the [repair handoff](0010-playing-close-repair.md).

**The original independent finding author approved the bounded playing-close
repair and technically closed CA-CLEANUP-002.** The first independent native
suite passes all six scenarios. Playing Close measured
**78.182125 / 43.789417 / 59.687792ms** and direct adapter playing cancellation
measured **75.140416ms**, within the unchanged **200ms** bound. The full local
race check passes. No new actionable findings were identified in this scope.

CA-CLEANUP-001 retains its earlier technical closure for the delayed-unload
crash. The full capabilities stage still fails at separate-track readiness.
Full RPL-002, RPL-003 and M1 remain unaccepted; scoped repair approval supplies
no production-player, platform, CI, packaging, merge or release approval.

## Independent decision and scope

Reviewer: `/root/coreaudio_cleanup_review`, the original CA-CLEANUP-002 finding
author. Controller `/root` implemented the repair and did not independently
approve it. The reviewer worked with the candidate read-only, writing only new
ignored scratch and tool caches. The controller imported the finalized review
and all evidence with hashes and sizes verified before updating this handoff.

The review covered the six implementation/test files under
`experiments/playback/mpvprobe/`: `engine.go`, `surface.go`, `shutdown.go`,
`engine_test.go`, `shutdown_test.go` and `cleanup_native_test.go`. It inspected
the pinned native command/reply path, render ownership, late replies, loading,
failure fallback, complete synchronous destruction and per-session unload.
The existing runtime was verified without a rebuild.

The reviewer confirmed that the non-coalesced `set vid no` command, with
`force-window=no`, reaches video-output destruction and worker join before its
reply. The render owner remains active during that work. Skip rendering after a
target-render failure still consumes pending frames on the same render owner.
Only then does shutdown stop/free rendering, destroy the core and unload the
library. Missing/failed replies and other fallback paths still execute complete
disposal; native tests reject a recorded fallback even if Close returns.

The [original review](0008-coreaudio-cleanup-review.md), its **284.933708ms**
failure, the [diagnosis](0009-playing-close-diagnosis.md), and the old-code
regression failure at **211.338125ms** remain unchanged. The new closure does
not reclassify those historical failures as passes.

## Exact reviewed candidate

Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, with intentional
uncommitted repair and documentation changes. HEAD alone is not the candidate.
The frozen 121-file candidate is `evidence/review-candidate/` in this run.

| Artifact | SHA-256 |
| --- | --- |
| Reviewed source manifest | `a4cf5096970c87b18ff9cec7df587a3a45c8df973edebd358f6077a9b039fa0b` |
| Reviewed source archive | `eec6c8b3e0b22e7f2735a5de5cef2bb556b2d8f4926c4381a11928c975535797` |
| Tracked diff | `32c44a92cbbf4643bf438b008f57450c23dab0fc9f2f73150b9aeb34f1aad71e` |
| Frozen spec | `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef` |
| Frozen tasks | `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3` |
| Runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| libmpv | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |
| Fresh native test binary | `466be9452699aa08539233dfc5c07cc512e8612dc6a6a8563651f2f060b5f6cd` |
| Independent review | `22d0ae11579ecd5320d8530c257587f8fd8c8150e0ae6338349d1d82fb93477a` |
| Reviewer evidence inventory | `d5d83cdd8d8056e55e7ee2aada762dfb57a77ff831544df14ebec9c9bcfb6625` |

Runtime: `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`, mpv source pin
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6. Before and after testing,
all 121 source files, source archive/modes/diff, frozen contract, 30 runtime
artifacts, recursive dependency closure, 901 native source files and generated
fixtures matched. No implementation or runtime bytes changed during review.

This report and README/stage-zero navigation were added after review. The
separate final `evidence/handoff-candidate/` snapshot identifies those
documentation updates; `handoff-verification.json` verifies that the reviewed
implementation, contract and historical reports remain unchanged. No new
implementation acceptance is inferred for later edits.

## Verification actually performed

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands used
`GOTOOLCHAIN=local GOWORK=off GOPATH=$PWD/.agent/gopath GOPROXY=off GOSUMDB=off`.
Only generated fixtures were used. The reviewer independently executed:

| Check | Result |
| --- | --- |
| Source/runtime/dependency/fixture identities before and after | PASS |
| Loader-only API 2.6, 15 symbols and actual loaded images | PASS; all 30 expected runtime images match |
| `go run ./cmd/dev check --race` | PASS, all six gates, including uncached race tests |
| `go test -c -tags nativecleanup -o <scratch>/nativecleanup.test ./experiments/playback/mpvprobe` | PASS |
| One fresh `TestNativeCoreAudioCleanup` invocation | PASS, exit 0, 61.291898s supervised elapsed, no timeout |

The native binary ran from the package directory with the verified absolute
library in `MY_BILIBILI_MPV_LIBRARY`, using
`-test.run=^TestNativeCoreAudioCleanup$ -test.v -test.timeout=120s` and a 135s
outer supervisor. It was executed once, without retrying an unchanged failure.

| Native scenario | Complete Close/cancel duration | Result |
| --- | --- | --- |
| Paused close | 2.735083ms | PASS |
| Playing close, three sessions | 78.182125 / 43.789417 / 59.687792ms | PASS |
| Playing context cancellation through completion and Close | 75.140416ms | PASS |
| Repeated pause/resume | 51.585208ms | PASS |
| Idle expiry followed by resumed progression | 84.340333ms | PASS |
| Three paused sessions | 2.563416 / 1.530708 / 1.615458ms | PASS |

All six scenarios reached their final eight-second post-close survival marker,
with zero owned cores/renderers/workers, repeated Close and no shutdown fallback.
The direct adapter cancellation result does not establish external UI timing.
The race run exercises the new pending-frame, reply-correlation, skip-render,
cancellation and fallback regressions alongside existing tests. Native C code
is not race-instrumented; native system-failure paths were not fault-injected.

## Retained capability failure and remaining gates

The reviewer audited all 49 retained repair-run evidence files and the actual
loaded-image inventory. The complete capabilities invocation from
`20261111T153610Z-04f30db12855` was not rerun. It retains 32 passed reached
checks and a failed stage at separate-track readiness:

`separate ready: context deadline exceeded (state=failed error=media-failure duration=0s owned=false)`

Combined local/HTTP picture and controls, protected routes, header isolation
and retry retain their recorded passes. External loading cancellation passed
at 60.994958ms. Forced app cleanup remains recorded; the owned process was
reaped and fixture handlers reached zero. External playing cancellation,
missing-audio failure and final graceful window close were not reached.
Separate-stream request/header observations do not establish playback readiness,
physical audio or synchronization.

The closure supplies scoped RPL-AC-03/06/09 cleanup evidence. It does not make
the complete capability stage or RPL-AC-05 pass. Physical speaker output,
measured speed/A/V sync, Windows runtime, authorized service playback,
packaging and candidate CI remain unverified. Timing is observed feasibility
on this setup, not a hard real-time guarantee across devices and scheduling.
The provisional mygo preview remains the default.

## Handoff and next action

The verification run completes `succeeded` for this bounded review and evidence
handoff. That recorder status grants no broader acceptance. Both cleanup
findings are technically closed; the next technical work is a separately
authorized bounded diagnosis of the retained separate-track readiness failure.
Preserve the 200ms bound, complete disposal and every failed outcome.

Read `evidence/independent-review/review.md` for the full independent reasoning.
Its 23 inventoried artifacts plus the inventory itself were imported unchanged
from `.agent/reviewer-scratch/playing-close-closure-r1/`. Controller assignment,
candidate snapshots, import/evidence audits and final handoff verification are
under `.agent/runs/20261111T155210Z-7e16351c96f2/evidence/`.
These ignored local logs, binaries and archives have no guaranteed remote
backup; preserve them before removing the checkout. Historical reports and
closed runs remain intact. No commit, push, PR, merge or remote publication
occurred.
