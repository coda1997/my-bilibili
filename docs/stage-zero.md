# Stage zero

This document defines milestones. It is not a live task-state database.
Execution events live in `.agent/runs/`; accepted conclusions live in reports.

Platform priority, owner decision on 2026-11-12: **macOS is required; Windows
is P2 / nice-to-have and does not block M1, M2 or P0**. Unavailable Windows
runtime results remain `not-tested`. Existing three-OS CI stays unchanged.
See [ADR 0005](adr/0005-platform-priority.md) and the
[current handoff](../reports/0025-m1-acceptance.md).

Owner measurement-scope decision on 2026-11-12: calibrated physical A/V sync
and display/speaker rate measurements are removed from M1, with no mandatory
later check. They were never performed and remain not-tested; see
[ADR 0006](adr/0006-m1-measurement-scope.md). Native controls, audibility, target
validation and final independent/owner decisions remain required.

## M0. Development foundation

Specification: `specs/0001-foundation/spec.md`.

Status: completed and accepted by the owner on 2026-11-10.

Exit criteria:

- A new contributor can find decisions, task contracts, checks, and handoffs.
- A run freezes its specification and task graph, records source provenance,
  persists checkpoints, and detects corrupt input or evidence.
- Checks validate the task graph, formatting, vet, tests, and build.
- CI is configured for Linux, macOS, and Windows with read-only permissions
  and evidence artifacts. A remote CI run is required before claiming that CI
  has passed; local cross-compilation is not an OS runtime test.
- An independent review is recorded. Self-review does not satisfy this item.

Current handoff and finding dispositions:
[readiness publication](../reports/0003-publication.md).
Readiness implementation, independent technical review, and PR #1 publication
are complete. Candidate and merged-master CI passed on all three OSes; COR-001,
COR-002, and ARC-001 are technically closed by their original reviewers.
The owner adopted the five task-scope/check/review policies on 2026-11-10;
see the [current policy handoff](../reports/0003-policy-adoption.md).
The owner separately accepted M0 on the same date.
The `0001` and local-phase `0003` reports remain unchanged historical snapshots.

## M1. mygo and playback feasibility

Specifications: [original playback spike](../specs/0002-playback-spike/spec.md)
and [bounded replacement probe](../specs/0006-player-replacement-probe/spec.md).

Host-policy decision on 2026-11-13: all valid HTTPS media hosts and ports are
allowed under [ADR 0007](adr/0007-target-host-policy.md). No origin list,
historical host comparison or repinning remains. Source/rendition, TLS,
expiry and native cleanup requirements still apply.

Status: **completed and accepted by the owner on 2026-11-13**; required macOS
technical gates satisfied. On 2026-11-13 the owner
selected libmpv as the default behind the existing boundary, retaining mygo UI;
see [ADR 0008](adr/0008-default-libmpv.md). The owner subsequently accepted M1
on the same date. This supersedes the owner’s 2026-11-11 provisional mygo choice.
The independently reviewed default launcher passes 41 macOS fixture assertions,
with loading/playing cancellation at 103.205542ms/82.0935ms and graceful cleanup;
see [report 0024](../reports/0024-default-libmpv.md). The following earlier
evaluations retain their original scope. The
[current independent selection review](../reports/0014-separate-track-review.md)
approves the separate-track repair and recommends continuing evaluation, while
retaining the remaining M1 gates. The
[native-rate handoff](../reports/0013-m1-rate-measurement.md)
records playhead observations and their limits. The
[separate-track repair](../reports/0012-separate-track-repair.md)
records the first passing complete macOS fixture capability invocation. The
[independent playing-close review](../reports/0011-playing-close-review.md)
records scoped cleanup approval and technical closure of CA-CLEANUP-002.
The [repair report](../reports/0010-playing-close-repair.md) and
[original cleanup review](../reports/0008-coreaudio-cleanup-review.md)
preserve the implementation evidence and earlier failed candidate.

The replacement's RPL-001 surface and loading-teardown gate passed
[independent review](../reports/0006-player-replacement-probe.md#independent-implementation-and-gate-one-review).
RPL-002 then exposed a CoreAudio delayed-cleanup crash after library unload.
The independent reviewer technically closed CA-CLEANUP-001 on the exact runtime
repair after all five native scenarios survived the old crash window. The first
independent suite then failed playing Close at 284.933708ms against 200ms,
opening P1 finding CA-CLEANUP-002. Diagnosis traced that delay to an unserviced
frame and mpv's 200ms timeout; those failures remain preserved. The subsequent
repair keeps frame servicing active until native video teardown acknowledges
completion. Its original finding author has independently approved the bounded
repair and technically closed CA-CLEANUP-002 on the exact candidate/runtime.
The first independent six-scenario suite passes: playing Close takes
78.182125 / 43.789417 / 59.687792ms, and direct adapter playing cancellation takes
75.140416ms. Complete disposal, per-session unload, zero ownership, no fallback
and eight-second survival pass. The full independent local race check passes;
the native runtime is unchanged. This scoped closure does not accept full
RPL-002/RPL-003 or M1.
The subsequent separate-track diagnosis found that `autoload-files=no` also
suppressed selection of explicitly supplied external audio. The adapter now
permits that selection only when audio is supplied, with audio/subtitle/cover-art
discovery still disabled. The new regression failed before repair; targeted and
full local race checks pass afterward. The first complete macOS fixture
capability invocation on this repair passes all 41 implemented assertions,
including separate picture/controls and required metadata on both streams.
External loading cancellation takes 74.229125ms and playing cancellation takes
71.583667ms, both within 200ms. Missing-audio Media failure with zero ownership
takes 106.128917ms, and final graceful window-close/socket/server cleanup passes
without forced cleanup. A separate repaired-code probe survives eight seconds
after complete Close/unload. Prior failed stages and the 100.455083ms failure
against the former 100ms bound remain unchanged historical failures.
The selection repair now has independent scoped approval with no new actionable
findings. The reviewer's full local race check and first fresh complete native
stage pass, with 41 passing assertions and two explicit not-tested categories.
Independent external loading cancellation takes 103.35175ms and separate-playing
cancellation takes 45.361125ms, within 200ms. Missing-audio disposed Media failure
takes 105.062833ms; final app/socket/server cleanup is graceful, with no forced
cleanup. The exact source/runtime and actual loaded images match. Physical audio,
measured sync/rate, Windows and real-service access remain not-tested; the
passing fixture stage does not establish full RPL-AC-05 or M1 acceptance.
Supplemental native-clock sampling observes initial rates of 0.453×/0.996×/2.000×
for the 0.5×/1×/2× presets, and 0.4997× in a later six-second half-speed window.
The initial half-speed discrepancy is preserved. These samples add playhead
timing evidence; physical display/acoustic rate and calibrated A/V sync remain
unmeasured. Their lifecycle checks pass without changing maintained code.

The legacy [bounded mygo fixture preview](../reports/0005-provisional-playback.md)
is now available explicitly through `cmd/mygo-native`, with its existing
sliders/presets/retry and warning. Its
explicit preview PASS remains separate from its strict full suite's
delayed-loading FAIL and unsupported header/separate-track capabilities.
See [ADR 0004](adr/0004-provisional-mygo-playback.md) and
[open mygo issues](known-issues/playback.md). Replacement progress does not
close those historical issues. ADR 0005 changed platform priority; ADR 0006 subsequently removed calibrated
physical sync/rate measurements. All other macOS capability and acceptance
gates still apply.

The [output-preparation handoff](../reports/0016-output-verification.md) now
records independently approved fixture generation, analysis and observation
launcher tooling, with both code findings closed and actual decoded fixture
checks passing. A later normal-app attempt now has positive owner audibility
statements for combined/separate fixtures under the prompted 1× setting after
explicit Play; independent evidence review approves these scoped attestations.
Initial failure and partial session outcomes remain retained.
No calibrated sync/rate measurement is claimed. The first
[public-target preflight](../reports/0019-authorized-target-playback.md) stopped
with permission unknown. A separately reviewed access method and bounded
observation now have independent TGT-001 approval for one ordinary anonymous
rendition; see [the current continuation](../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13).
TGT-002 opt-in target implementation and its TLS repair have independent scoped
approval; that reviewed candidate's native fixture regression passed all 41
assertions. The first TGT-003 refresh on 2026-11-13 stopped UNKNOWN before native
launch. A subsequent diagnostic observed a changed video host. The owner then
removed host restrictions, implemented under 0009 with descriptor version 2.
That target-continuation fixture regression passed 41 assertions, including loading and
playing cancellation at 99.368625ms/79.460375ms, with graceful cleanup. The first
version-2 target invocation failed readiness/picture and required forced reap.
TGT-V2-002, missing failure state/phase, is independently closed by the driver-only
0010 repair. The separately reviewed post-repair invocation at 01:58 UTC on
2026-11-13 passes eight target capability/cleanup checks, with playing cancellation
at 87.7575ms and graceful exit without force. Target loading cancellation remains
not-tested in that single-open session. Independent review closes TGT-V2-001 as a
current acceptance blocker; the original failure and unknown cause are preserved.
Final independent assessment recommended libmpv; the owner subsequently selected
it as the default. The owner subsequently accepted M1 on 2026-11-13.

Exit criteria:

- Pin and record an evaluated mygo version or commit.
- Run an actual Go UI on macOS, the required target operating system.
- Validate authorized target media flows, including separate streams if
  present, request metadata, seek, cancellation, and resource cleanup.
- Produce a capability matrix with pass/fail/not-tested and evidence for each
  OS attempted. Required macOS results must pass; Windows is deferred P2 and
  remains not-tested until exercised, never inferred from a build.
- Accept a playback adapter decision, or explicitly return to architecture.

Calibrated physical A/V sync and actual display/speaker playback-rate measurements
are outside M1 scope under ADR 0006. They were never performed and remain
not-tested; their absence does not prevent satisfying these exit criteria.

The M1 feasibility gate is satisfied. M2 subsequently received its own reviewed
contract and implementation authorization; its current acceptance status is
recorded below. The full desktop shell remains outside this fixture slice.

## M2. One complete development loop

Choose a small user-visible slice only after M1: for example, fixture-backed
video details followed by a playback command through the approved adapter.

Latest acceptance continuation — 2026-11-16: the original independent reviewer
approved the scoped fullscreen causal repair and automated native response in
formal round 5. REV-002 is technically CLOSED; REV-001 remains CLOSED. Independent
targeted/tagged race tests pass and removing the callback guard makes the
regressions fail. Four complete native suites each pass 30 checks; successful
fixed runs total 51 fullscreen cycles. No new scoped defect or code repair was
identified. Cumulative M2-004 history is five reviews/eight fixes, not a budget
reset; no further automatic repair/review loop is active.

Owner functional feedback — 2026-11-16: after the pinned normal package and
manual picture/audio/interaction checklist were supplied, the owner replied
“功能全部正常”. This is recorded as aggregate owner-reported functional acceptance,
including the prompted ordinary playback/audio and controls. No detailed
per-item observations or independent system-level accessibility audit were
provided or inferred. The normal executable and preceding 274-file source
snapshot remain unchanged. Run `20261116T145910Z-31c75327708a` preserves this
attestation separately from the prior frozen checklist and review evidence.

Publication and CI continuation — 2026-11-16: Draft
[PR #9](https://github.com/coda1997/my-bilibili/pull/9) publishes the candidate.
Initial CI passed Linux/Windows but macOS race tests exhausted artificial
whole-test4s/1.2s deadlines. The owner authorized repair9/review6. The controller
reproduced the exact phases with controlled picture-processing delay and repaired
only the test contexts/fixtures, retaining production per-transition limits and
all state/order assertions. Repaired targeted/full local race checks pass;
restoring old test limits and removing the native callback guard both fail their
discriminating controls. The original reviewer and new exact-candidate CI supply
the final disposition within the same authorized round. See current PR checks
and the latest section of [report0028](../reports/0028-m2-fixture-preview.md).
M2 milestone acceptance and merge remain separate owner decisions; there is no
automatic additional repair/review loop.

Historical additional repair completion — 2026-11-16: the proven
fullscreen callback guard was already in maintained source. The controller
rebuilt normal/e2e developer bundles and the driver, then passed one complete
native suite (30 checks, 38.382s, five gracefully closed processes) and the
recorded full race gate. No new implementation change was needed or counted;
prior four reviews/eight fixes remain. This is packaging and verification of
the existing repair, not independent closure or owner M2 acceptance. Fresh
normal bundle build is established; its separate interactive launch is not.
Artifacts and exact identities are in the current M2 handoff.

Historical investigation — 2026-11-16: the owner authorized code/log investigation,
causal repair and repeated native experiments without intermediate review/fix/
human loops. The fullscreen driver had treated geometry and rendered-state
restoration as transition completion. Native callback traces show that the
next entry request reached the setter before the preceding DidExit delivery.
Waiting for real DidEnter/DidExit counts alongside the original checks repairs
the reproduced race. Five pre-fix reproductions and three guard-removal controls
fail; thirteen fixed diagnostic prefixes and three complete native suites pass,
covering 48 successful fixed fullscreen cycles. Each full suite passes 30 checks,
including parts/replay, cancellation below 200ms, zero ownership, eight-second
live survival and actual close. All 36 owned processes exit gracefully. Final
local full race and relevant e2e race gates pass. The scoped investigation is
complete; no formal review or independent finding closure was performed.
REV-001 remains CLOSED and REV-002 remains OPEN pending separate disposition.
Physical audibility, remaining manual inputs, exact-commit integration/CI and
M2 acceptance remain pending. The latest owner instruction supersedes the
per-attempt stop proposals in the historical sequence below for this completed
investigation only; it does not change repository policy or product criteria.

Historical execution before this investigation: [spec 0016](../specs/0016-m2-policy-continuation/spec.md)
continues the original fixture preview with owner-approved R1–R9 budgets and
the labeled handoff-drill alternative. Two authorized workers completed isolated
application/fixture and desktop work. The integrated developer package builds;
local checks and the separate 41-assertion laboratory regression pass. The first
product native smoke failed at seek and exposed aliased report observations.
The owner then authorized one additional M2-004 repair: snapshots are
independently closed as repaired, and actual native seek passes. A later
invocation passes controls, layouts and one fullscreen cycle, then fails in the
second fullscreen cycle. After the owner reported accidental UI interaction,
one unchanged retry was authorized. It passes seek again, then fails at a
speed-menu action before reaching fullscreen. A separately authorized 2.8-second
diagnostic establishes that the driver selects 2× before the popup node exists;
the node appears later without another action. The owner authorized a fifth
driver repair and one subsequent acceptance invocation. The bounded popup wait
passes local race checks and the native 2× selection. That invocation again
times out during the second fullscreen exit, with eleven product checks and
graceful cleanup passing. The original reviewer completed the final third
M2-004 review with REV-002 still OPEN; its independent race checks pass.
The owner then authorized one fullscreen diagnostic, which ran once for
10.437 seconds and reproduced the second-exit timeout with graceful cleanup.
The trace establishes premature cached-state acceptance and an intended exit
toggle sent after ordinary window state had returned, causing fullscreen entry.
The first-cycle cached-state PASS does not prove completed native exit. Exact
native callback timing remains unknown. The owner subsequently authorized one
sixth driver repair with offline verification only. The driver now checks
native geometry and rendered layout alongside cached state, requires ordinary
restoration and rechecks action preconditions around capture and reversal.
Targeted/four-package e2e/full race gates pass; fourteen mechanism-removal
controls fail as intended. The owner then authorized one full native invocation
and one fourth response review. That invocation fails after 9.514 seconds:
controls, layouts and the first corroborated fullscreen cycle pass, then the
second entry sequence times out before its capture completes. Cleanup is
graceful without force; later native scenarios remain unexecuted. The exact
failed RPC or predicate and current cause remain unknown. The original reviewer
completes round four with independent targeted/e2e/full race gates passing and
all fourteen negative controls failing as intended. REV-001 stays CLOSED;
REV-002 stays OPEN, BLOCKING class 5. All six fixes and four formal reviews are
consumed; repeated fullscreen acceptance has four observed failures, without
claiming the same cause. The proposed next decision is one read-only second-entry
diagnostic lasting at most 60 seconds. No further repair, review or native launch
is authorized. This is a real escalation, not a drill. M2 is not completed or
accepted; exact-commit integration and remaining product/native evidence are
still pending.
See [the current M2 handoff](../reports/0028-m2-fixture-preview.md).

Exit criteria:

- Lead produces a spec and task DAG; an independent architect reviews them.
- Freeze shared contracts before splitting non-overlapping tasks.
- At least two authorized workers can develop in separate worktrees.
- The controller integrates an exact candidate commit and runs checks.
- Independent reviewers return structured findings on that commit.
- Exercise a bounded repair cycle and a human-escalation path.
- A native package runs on macOS; Windows packaging is optional P2 and does
  not block M2. Distinguish preview artifacts from signed public releases.

Initially coordinate this protocol manually. Automate only the demonstrated
transitions; do not start by implementing a general-purpose agent platform.

## M3. One issue-to-PR loop

Status: **completed and owner-accepted for the scoped manual issue-to-PR loop
on 2026-11-17**. The owner approved PR #12 after the proposed acceptance step,
authorized merge and issue closure, and requested status updates.

Existing issue #10 was triaged and independently reproduced. The minimal
play/pause-intent repair passed six discriminating regression cases, local race
checks, independent architecture/code review, and all three CI platforms for
the exact PR candidate. [PR #12](https://github.com/coda1997/my-bilibili/pull/12)
is merged as `15e353089408d369ee740382f41f8e129ada5707`; issue #10 is closed as
completed. See [the acceptance handoff](../reports/0029-playback-intent.md).

The execution/publication/merge steps were explicitly owner-authorized and
manually invoked. One controller owned the issue; its implementation run was
finished before this status-only continuation. No bot/event scheduler exists
to recursively trigger new work. This records the demonstrated manual loop,
not an automated controller or a new interpretation of historical M2 evidence.

After a remote and execution policy exist, use one authorized reproducible
issue to demonstrate triage, a failing regression test, a minimal fix, review,
and a PR. Do not bypass the M2 gates. Deduplicate active runs per issue and do
not allow bot-generated events to recursively trigger new work.

## Not included in this bootstrap

Automatic agent scheduling, security sandboxes, enforceable worker write
scopes, trusted review approvals, PR creation, automatic merges, application
signing, notarization, and public releases are deliberately not implemented.

## Next action

The issue #10 / M3 loop is completed, accepted and merged. The owner subsequently
authorized repair, PR publication, passing CI and merge for
[issue #11](https://github.com/coda1997/my-bilibili/issues/11). That bounded seek
queue repair has independent architecture/code approval and passing local race
checks; [PR #14](https://github.com/coda1997/my-bilibili/pull/14) holds its live
CI/merge receipt. Read [the current repair handoff](../reports/0030-seek-queue.md)
and [M3 acceptance](../reports/0029-playback-intent.md). P0 and the full shell
remain pending and are not started by this repair. Historical M2 investigation
and review evidence remains in [report 0028](../reports/0028-m2-fixture-preview.md).

Preserve the 200ms cancellation bound, complete teardown contract, evidence and
all historical failures. Any future service/native invocation needs its own
bounded assignment. Windows remains optional P2/not-tested; calibrated physical
sync/rate measurements remain unperformed and outside M1, with no mandatory
later check. Standing native delegation applies within authorized task scope.
Do not record stage zero as complete until M0 through M3 have actual evidence.
