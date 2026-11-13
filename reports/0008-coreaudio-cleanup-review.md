# Independent CoreAudio cleanup review and remaining blockers

Date: 2026-11-11. Spec: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md).
Controller run: `20261111T150612Z-804b080ab456`.
Scope: owner-authorized independent review of the exact RPL-002 cleanup repair
and a superseding disposition of **CA-CLEANUP-001**.

The independent reviewer requests changes. **CA-CLEANUP-001 is technically
closed for the exact reviewed source/runtime**, but a new **CA-CLEANUP-002, P1**
remains open: playing Close took **284.933708ms** against the preserved **200ms**
bound. The first independent native cleanup suite failed. Cleanup repair
approval, full RPL-002 acceptance, the RPL-003 recommendation and M1 remain
pending. The separate-track readiness failure also remains unresolved.

## Independent decision and exact identity

Reviewer: fresh native Codex context `/root/coreaudio_cleanup_review`, separate
from the implementation author. The owner selected step 1, independent cleanup
review and finding disposition. The original reviewer had become the repair
author, so the fresh reviewer was assigned superseding independent verification.
The original finding and all closed runs remain unchanged. The controller
prepared this handoff from the returned independent report and evidence.

The immutable reviewer report is the new run's
`evidence/independent-review/review.md`, SHA-256
`884d5e86855b2acfe879ef2ea5da0257703966c3a00d9ccff719e3f3f2452432`.
Its 23-file evidence inventory, `evidence/independent-review/evidence-manifest.json`,
has SHA-256 `164e41f8f456b0e51f8ca178d68e92b0a5bd24f226630246c3b0c97b62e5305f`.
The controller verified every copied file against that inventory.

- Reviewed clean commit: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`.
- Implementation commit: `8f697d229e9d4cd97ac7ad89092e680fdc7ebc20`.
- Reviewed 116-file manifest SHA-256:
  `6acddd26d80dd6afb2424dec32fa9a4e00e20e790db5f424adede0fb2838bc69`.
- Reviewed source archive SHA-256:
  `24add4b376a0ddad350125e106827a1bd039864a332d30606078e911b194aec3`.
- Frozen spec SHA-256:
  `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef`.
- Frozen tasks SHA-256:
  `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3`.
- Runtime manifest SHA-256:
  `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`.
- Repaired libmpv SHA-256:
  `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd`.

Runtime location: ignored `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`.
The reviewer verified the exact two-patch replay, all 901 upstream source files,
30 runtime artifacts and their dependencies. The post-check identity confirms
all 116 candidate source files and 30 runtime artifacts remained unchanged.
This handoff and README/stage-zero navigation are subsequent documentation-only
changes; they do not extend the review to a different implementation or runtime.

## Finding dispositions

**CA-CLEANUP-001 — P1 — technically closed.** The owned idle timer is signaled
and joined before teardown returns and the library is unloaded. The reviewer
inspected initialization, locking, condition-wait and teardown ordering, audited
the retained old-runtime SIGSEGV, and independently observed every repaired
scenario survive eight seconds after close. This closes the identified delayed
unload crash on the exact candidate; it does not prove every cleanup requirement.

**CA-CLEANUP-002 — P1 — open.** In the first independent playing-close execution,
complete synchronous Close took 284.933708ms. It returned zero owned resources
and the process survived the old crash window, but exceeded 200ms. This is a
direct Close timing failure, not a new externally observed UI cancellation
measurement. Its cause is not established by this review. The contract still
requires complete native disposal before return and per-session library unload;
zero counters, early return, library retention or a relaxed deadline do not
satisfy that contract. A bounded diagnosis and repair must preserve the failed
result and receive independent verification on the changed candidate.
The measured call/assertion is in
[`cleanup_native_test.go`](../experiments/playback/mpvprobe/cleanup_native_test.go),
lines 94-100; complete disposal waits in
[`engine.go`](../experiments/playback/mpvprobe/engine.go), line 113. The assigned
verifying reviewer for this new finding is `/root/coreaudio_cleanup_review`.

## Verification

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. The reviewer used the
existing verified runtime and generated fixtures, with `GOTOOLCHAIN=local`,
`GOWORK=off`, the local module cache, and module network access disabled.

| Independent check | Result |
| --- | --- |
| Source, frozen contract, runtime/dependency identity and exact patch replay | PASS |
| Immediate loader/API/symbol preflight | PASS |
| `go run ./cmd/dev check --race` | PASS, all six gates |
| Paused close | PASS; 2.456042ms and eight-second survival |
| Playing close | FAIL; 284.933708ms exceeds 200ms; zero ownership and survival PASS |
| Repeated pause/resume | PASS; Close 45.125583ms and survival |
| Idle expiry/resume | PASS; Close 45.489458ms and survival |
| Three sessions | PASS; Close 3.038666 / 1.583334 / 1.764416ms and survival |
| Complete native cleanup suite | FAIL, exit 1, no timeout; first result preserved, no retry |
| Prior complete capabilities invocation | Audited only; retained FAIL at separate-track readiness with forced cleanup |

The reviewer independently compiled the native test with
`go test -c -tags nativecleanup`, then executed it once with
`-test.run=^TestNativeCoreAudioCleanup$ -test.v -test.timeout=120s`.
Test binary SHA-256:
`459e27b1e51cf1ec7c05798f6b0866ea2f999f58d5982c783453d28e5d1e2c0a`.
Native log SHA-256:
`ae4fe32a5a4a53109943eeb5f9c9ae2fa3aba6f31ab46a349f6151323a8a8149`.

The [previous author verification](0007-coreaudio-cleanup.md) remains historical:
all five cleanup scenarios passed in that execution. The later independent
deadline failure is preserved alongside it. No result is retried or rewritten
into a pass, and the full capabilities suite was not repeated for this review.

The controller verified all 23 imported reviewer artifacts, checked 38 local
Markdown targets/anchors in the changed documents, and checked whitespace. All
reviewed implementation and frozen-contract bytes remain unchanged; only this
report, README and stage-zero navigation changed afterward. The final uncommitted
documentation snapshot is identified in the new run's
`evidence/handoff-candidate/identity.json`, with its full manifest and archive.
No additional native or Go test invocation was needed for these documentation
changes, and no changed-candidate CI result is claimed.

## Remaining work and next action

First scope a bounded diagnosis of CA-CLEANUP-002, identifying where complete
playing Close spends time while preserving the 200ms bound and complete teardown.
Any repair needs exact source/runtime identity, fresh relevant native regression
evidence and review by the assigned independent reviewer. Then continue the
separate-track readiness diagnosis from the retained failed capability invocation.
This review request does not start either implementation task.

The prior capability run still leaves playing cancellation, missing-audio failure
and final graceful window close unreached. Windows, physical speaker output,
measured playback rate/A/V sync, authorized target-service playback, packaging
and candidate CI remain unverified. The default mygo fixture preview remains
unchanged. No M1 acceptance, M2/P0/full-shell work, policy approval or remote
publication is implied.

Initialization/thread-creation failures were inspected, not induced. The native
runtime was verified but not rebuilt in this review, and Go's race detector
does not instrument its C code. The regression observes expiry/resume behavior;
physical device sleep/stop timing, device changes and system suspend were not
instrumented or tested.

The new run retains the assignment, exact candidate archive/manifest, returned
review and validation evidence. Original reviewer scratch is
`.agent/reviewer-scratch/coreaudio-cleanup-closure-r1/`. Large logs, native test
binaries, runtime builds and archives remain local ignored artifacts with no
guaranteed retention or remote backup. Preserve required evidence before removing
this checkout. No commit, push, PR or merge was performed for this review.
