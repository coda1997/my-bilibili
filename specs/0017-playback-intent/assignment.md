# Assignment: issue #10 controller and independent reviewer

Date: 2026-11-17. Controller/implementer: `/root`. Reviewer: to be assigned in
the controller's immutable review packet before dispatch. User selected issue
#10, received the reproduced two-direction failure, and explicitly requested
continuation. This authorizes scoped local implementation and verification.
Standing repository authority permits independent native sub-agent reviews.
Commit, push, remote comments/PR/issue closure, merge and release are not granted.

Base: ff27f1db5946eb9ebccbee8c3548a971dd112e85, branch
`codex/issue-10-playback-intent`, worktree `eager-hertz`. Preexisting untracked
`.claude/` is excluded and preserved. No local runs existed at selection.
Controller freezes spec/tasks only after architecture approval and records
exact hashes/snapshot, reviewer identity and run ID in its evidence packet.
The proposed spec status remains historical; the review/run records establish
later approval/freeze without editing frozen inputs.

## Roles and boundaries

- Planner/controller: new `specs/0017-playback-intent/` packet and run evidence.
- Implementer: INTENT-001 source/test paths only after architecture approval.
- Handoff controller: INTENT-002 paths after evidence is available.
- Reviewer: read-only candidate; isolated scratch allowed; returns conclusions
  and logs to controller, never edits its run or implementation.

Read README/current handoff, docs/agent-convergence.md, docs/testing.md,
docs/architecture.md, docs/workflow.md, the selected spec/tasks,
specs/0016-m2-policy-continuation/shared-contract.md, internal/app/session.go
and tests, and experiments/playback/contract/contract.go. Existing source at
base is the reproduction candidate. Prior scratch probe proved both directions
fail under a held command consumer; original app race suite passed.

Review correctness, async intent/observation separation, deduplication, failure
propagation, existing lifecycle preservation, test discrimination and scope.
Use docs/templates/review.md. Independently rule on M2-EXT-001's external high
claim for the original/fixed candidate; this is fresh verification, not reviewer
succession or closure of the external review thread. No substitute closure or
M2/M3 acceptance is authorized. The owner selects milestone acceptance later.

## Counts, inventory and stop conditions

Logical work: issue #10 only. Initial architecture reviews 0, code reviews 0,
automatic fix iterations 0, failed repair verifications 0. Prior reproduction
failures are expected controls, not repairs. Each architecture/code task retains
its own three-round cap; never reset counts by replacing candidate/run/agent.
Existing registration filed two issues (#10/#11), budget 2/5 retained; this
continuation plans zero new issues. #11 stays deferred; any new advisory is
recorded in the run inventory, not automatically filed or implemented.

Run the spec's targeted and full race checks on pinned Go 1.27.3. No live
service, native app, credentials or global installation is needed. Stop for
scope/contract changes, failed required gates or exhausted budgets. Return
candidate/spec identity, checks, independent ruling, tiered findings, counts,
remaining gaps and smallest next action. Remote publication remains pending
explicit authority and candidate CI. No automated issue-trigger loop exists.
