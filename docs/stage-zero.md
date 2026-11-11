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

Specification: `specs/0002-playback-spike/spec.md`.

Status: authorized macOS-first experiment started 2026-11-10; partial handoff
continued on 2026-11-11. [Current macOS evidence](../reports/0002-playback-macos.md)
records exposed local/HTTP native picture, progression, pause and seek passes.
The strict smoke still fails in delayed loading: mygo's synchronous tracks query
blocks the UI thread. A metadata-gated view workaround was rejected because it
prevents native error refresh; it is not retained as a repair. The smoke's
stale-generation oracle is separately repaired. Header/separate-track capabilities
remain unsupported; Windows and real target access are not-tested. Return the
runtime scheduling/status gap to architecture; no final adapter decision or M1
acceptance is issued. On2026-11-11 the owner separately chose to continue with
mygo temporarily, document the issues and consider replacement later. The
[bounded fixture preview](../reports/0005-provisional-playback.md) adds transport
sliders/presets/retry and a warning; explicit preview PASS is separate from the
default full suite's retained delayed-loading FAIL. Independent review approved
the bounded preview code after closing its slider-capture feedback finding;
reviewer occluded reruns remain separate/not-tested, not a new exposed pass. See
[ADR0004](adr/0004-provisional-mygo-playback.md) and
[open issues](known-issues/playback.md). This continuation is not an M1 gate waiver.

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
[current handoff](../reports/0003-policy-adoption.md) and its linked publication
evidence. Historical pending-policy/M0/CI notes do not request another repair
or publication round. M1's macOS-first start and independent contract review
are now authorized; the repaired contract was reviewed before freeze. Continue
from the [provisional playback handoff](../reports/0005-provisional-playback.md):
use the independently reviewed bounded preview only with its controlled
fixtures, and preserve the strict failure/open issues. The owner will consider
a replacement later; that requires a separately scoped reviewed adapter probe.
Arrange Windows runtime access and retain header/separate-track/target-access
gates. The current exposed preview does not fix slow-load responsiveness or
complete M1/authorize the full client. Historical occlusion is not the latest
strict run's blocker; its delayed-loading failure remains.
Do not record stage zero as complete until M0 through M3 have actual evidence.
