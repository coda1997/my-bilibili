# Stage zero

This document defines milestones. It is not a live task-state database.
Execution events live in `.agent/runs/`; accepted conclusions live in reports.

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

Status: authorized macOS-first evaluation, in progress; no M1 acceptance or
final adapter decision. The owner chose provisional mygo use on 2026-11-11
and subsequently authorized the bounded libmpv replacement probe. The
[current cleanup handoff](../reports/0007-coreaudio-cleanup.md) records the
latest source/runtime identity, capability results and remaining gates.

The replacement's RPL-001 surface and loading-teardown gate passed
[independent review](../reports/0006-player-replacement-probe.md#independent-implementation-and-gate-one-review).
RPL-002 then exposed a CoreAudio delayed-cleanup crash after library unload.
The owner-requested runtime repair passes five isolated native cleanup
scenarios and the full local race check. Independent repair review and
CA-CLEANUP-001 closure remain pending. The complete capability invocation
passes combined local/HTTP playback, controls, protected headers and header
isolation, then fails separate-track readiness and requires forced app cleanup.
Its 67.470292ms loading cancellation passes the owner-amended 200ms bound;
the earlier 100.455083ms failure against 100ms remains a historical failure.
Playing cancellation and final graceful window close were not reached in
the latest invocation. Physical audio, measured sync, Windows and real-service
access remain not-tested.

The default [bounded mygo fixture preview](../reports/0005-provisional-playback.md)
remains independently reviewed, with sliders/presets/retry and a warning. Its
explicit preview PASS remains separate from its strict full suite's
delayed-loading FAIL and unsupported header/separate-track capabilities.
See [ADR 0004](adr/0004-provisional-mygo-playback.md) and
[open mygo issues](known-issues/playback.md). Replacement progress does not
close those historical issues or waive an M1 gate.

Exit criteria:

- Pin and record an evaluated mygo version or commit.
- Run an actual Go UI on both target operating systems.
- Validate authorized target media flows, including separate streams if
  present, request metadata, seek, cancellation, and resource cleanup.
- Produce a capability matrix with pass/fail/not-tested and evidence for each
  OS. Missing Windows access remains not-tested, never inferred from a build.
- Accept a playback adapter decision, or explicitly return to architecture.

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
- A native package runs on each target OS; distinguish preview artifacts from
  signed public releases.

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
[policy handoff](../reports/0003-policy-adoption.md) and its linked publication
evidence. Continue M1 from the
[CoreAudio cleanup handoff](../reports/0007-coreaudio-cleanup.md). The owner
authorized documentation, commits and a feature-branch push for this repair;
publication does not supply independent approval. The next technical gates
are a separately authorized independent review of the exact cleanup repair
and a bounded diagnosis of the retained separate-track readiness failure.
Preserve the 200ms cancellation bound, complete teardown contract and every
failed outcome. Arrange Windows runtime access and authorized physical-audio,
sync and target-service verification before final M1 acceptance. The default
provisional preview remains limited to its controlled fixtures.
Do not record stage zero as complete until M0 through M3 have actual evidence.
