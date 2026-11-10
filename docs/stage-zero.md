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

Current implementation handoff: `reports/0001-foundation.md`.

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

Review the M0 implementation independently, configure the intended remote when
authorized, and run CI. In parallel with that decision, arrange macOS and
Windows runtime access and safe media fixtures for M1. Do not record stage
zero as complete until M0 through M3 have actual acceptance evidence.
