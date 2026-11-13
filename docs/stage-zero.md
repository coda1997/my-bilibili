# Stage zero

This document defines milestones. It is not a live task-state database.
Execution events live in `.agent/runs/`; accepted conclusions live in reports.

Platform priority, owner decision on 2026-11-12: **macOS is required; Windows
is P2 / nice-to-have and does not block M1, M2 or P0**. Unavailable Windows
runtime results remain `not-tested`. Existing three-OS CI stays unchanged.
See [ADR 0005](adr/0005-platform-priority.md) and the
[current handoff](../reports/0024-default-libmpv.md).

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

Status: required macOS technical gates satisfied. On 2026-11-13 the owner
selected libmpv as the default behind the existing boundary, retaining mygo UI;
see [ADR 0008](adr/0008-default-libmpv.md). Formal owner M1 milestone acceptance
remains pending. This supersedes the owner’s 2026-11-11 provisional mygo choice.
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
it as the default. Formal M1 milestone acceptance remains pending.

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

Do not build the full desktop shell before this uncertainty is resolved.

## M2. One complete development loop

Choose a small user-visible slice only after M1: for example, fixture-backed
video details followed by a playback command through the approved adapter.

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

After a remote and execution policy exist, use one authorized reproducible
issue to demonstrate triage, a failing regression test, a minimal fix, review,
and a PR. Do not bypass the M2 gates. Deduplicate active runs per issue and do
not allow bot-generated events to recursively trigger new work.

## Not included in this bootstrap

Automatic agent scheduling, security sandboxes, enforceable worker write
scopes, trusted review approvals, PR creation, automatic merges, application
signing, notarization, and public releases are deliberately not implemented.

## Next action

M0 acceptance and policy adoption are complete; see the
[policy handoff](../reports/0003-policy-adoption.md). Continue M1 from the
[current target continuation](../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13)
and [measurement scope](../reports/0020-m1-measurement-scope.md).
The owner confirmed only a Mac is available and granted standing native subagent
authority; no per-dispatch permission is needed. Windows is deferred P2 and does
not block M1, M2 or P0; future Windows support still requires actual evidence.

OUT-001/002 preparation now has independent technical approval after original
reviewer closure of both code findings. OUT-003 records the current matrix and
remaining inputs. Local race checks, actual fixture generation/decoded audit and
controlled launcher lifecycle tests pass. Three normal-app observation sessions
now retain one inconclusive attempt and two positive owner audibility statements;
all ended partially with graceful cleanup. No physical capture ran. Preserve the
200ms adapter cancellation bound, complete
teardown contract, original fixtures and every failed outcome.

The positive combined/separate audibility statements now have scoped independent
evidence approval; this is not independent hearing or a measured 1× output rate.
Calibrated sync and actual display/audio rates were never measured. ADR 0006
removes those requirements and their numerical criteria from M1; no equipment
or mandatory later measurement is needed.
The original target-media packet stopped with permission unknown; its results
remain unchanged. A separately reviewed interpretation method and new bounded
observation now satisfy TGT-001 for the selected ordinary anonymous rendition.
The earlier TGT-002 code/resource review and native fixture regressions are
complete for their recorded candidate. HOST-001 removes host restrictions in
both descriptor validation and the copied refresh helper. The independently
reviewed post-repair target invocation now passes picture, tracks, controls,
87.7575ms playing cancellation and graceful cleanup. No target retry is needed
for this result. Any future invocation needs its own bounded assignment; preserve
all UNKNOWN and failed outcomes. No per-host review or origin-policy amendment
remains. The completed observation pairs permit no further requests by themselves.

Both cleanup findings and the separate-track repair retain their prior
independent closure/approval, including the 41-assertion macOS fixture stage.
The owner selected libmpv as the default behind the existing plain playback
boundary with mygo UI retained; the switch and its checks are in
[report 0024](../reports/0024-default-libmpv.md). The next milestone action is
formal owner M1 acceptance. M2/P0 and the full shell remain gated on M1. Do not
record stage zero as complete until M0 through M3 have actual evidence.
