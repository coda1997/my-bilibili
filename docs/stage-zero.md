# Stage zero

This document defines milestones. It is not a live task-state database.
Execution events live in `.agent/runs/`; accepted conclusions live in reports.

## M0. Development foundation

Specification: `specs/0001-foundation/spec.md`.

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
Human policy adoption and M0 acceptance remain separate pending decisions.
The `0001` and local-phase `0003` reports remain unchanged historical snapshots.

## M1. mygo and playback feasibility

Specification: `specs/0002-playback-spike/spec.md`.

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

The human owner should decide adoption of the proposed task-scope/check policy
and M0 acceptance using the current handoff. The repairs, independent review,
publication, and candidate CI are complete; the historical pending-CI notes do
not request another repair or publication round. M1 remains a separately
authorized feasibility task requiring independent contract review, real
macOS/Windows runtime access, and safe media fixtures.
Do not record stage zero as complete until M0 through M3 have actual evidence.
