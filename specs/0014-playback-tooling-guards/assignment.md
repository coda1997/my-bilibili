# Assignment: GUARD-001 / GUARD-002

- Owner request: use a subagent to fix both independently confirmed Copilot
  comments on PR #6. Controller `/root` owns planning, integration and GUARD-002.
- GUARD-001 implementer: `/root/playback_guards_fix`; independent scope/code
  reviewer: `/root/copilot_findings_review`, who reproduced both original issues
  and has not authored this repair. Standing native delegation plus this request.
- Source: clean `52ecd96b01c4a4df80ebccf2a287bb1d893bd4de`; packet hashes and
  exact reviewed repair manifest/archive are computed at dispatch/review.
- Read README, report0026, this spec/tasks, docs/testing.md, architecture and
  workflow. Original findings/reproduction evidence are in
  `.agent/reviewer-scratch/copilot-20261113/`; contracts 0006/0007 provide context.
- Scope review precedes the controller's new frozen run and implementation.
  Worker may edit only the four GUARD-001 paths; no frozen packet/run/doc edits.
  Controller alone writes its run, README and new report. Preserve prior reports.
- Worker scratch: `.agent/worker-scratch/playback-guards/`; reviewer scratch:
  `.agent/reviewer-scratch/playback-guards/`. Same checkout, disjoint ownership;
  no new managed worktree/session or app orchestration is requested.
- Worker returns baseline failing regressions, repaired targeted race checks,
  file hashes, diff, actual limits and remaining work. Controller runs the full
  race check and freezes source before independent review. Maximum three repair
  rounds; scope/contract changes return to review before a new freeze.
- Reviewer is read-only on candidate and controller run. Verify original issue
  behavior, new edge cases, deadline/reap preservation and test validity. Return
  actual checks and candidate-bound decision; this is not Copilot impersonation
  or authority to resolve its GitHub threads. Succession is not applicable: this
  is independent repair verification, not reassignment of original closure.
- Controller retains earlier user authority to commit/push/update this existing
  PR after checks/review; workers/reviewer have no Git or remote mutation role.
  No merge, release, message/review reply, policy adoption, live media or native
  session. Local Go1.27.3 prerequisites and controlled fixtures only.
