# Task-scope/check-policy adoption and M0 acceptance

- Date: 2026-11-10 (Asia/Shanghai).
- Run ID: `20261110T143714Z-7312425c7fe8`.
- Source baseline: clean `20005fb9cc1ea2ca37d69472879d34e25ac066e7`.
- Authority: after receiving the enumerated five-policy explanation, the owner
  explicitly confirmed adoption of items 1 through 5 in this chat.
- Separate M0 decision: the owner subsequently directed marking M0 completed.
  M0 is therefore accepted and completed on 2026-11-10, based on the linked
  completed technical review and publication evidence, not on a run outcome.
- Scope: record those human decisions and update current documentation only.
  No product code, task-contract body, check implementation, CI, or remote settings
  are changed. At decision recording, this documentation continuation was
  uncommitted. The owner subsequently authorized creating and merging a PR
  for these records; this is separate from adoption and M0 acceptance.

## Adopted policies

1. **ASCII task write scopes (ADR 0003).** Only `tasks.json` `write_paths`
   reject non-ASCII spellings, in addition to the existing safe-path grammar
   and case-insensitive comparison. Unicode source/UI text, checkout paths,
   spec paths, and evidence names are not generally prohibited. A deliberate,
   assigned and reviewed ASCII parent scope can cover a Unicode filename.
2. **Task scope and dependency validation.** Overlapping valid scopes require
   dependency ordering, dependencies must be acyclic, and every acceptance
   criterion must have task coverage. Ordering does not legalize an invalid
   path. Validation is not a filesystem sandbox or actual diff enforcement.
3. **Check authority and trusted policy.** Task `checks` are verification
   expectations, not command strings the recorder executes. A future controller
   must load gate policy from a trusted baseline, not the candidate's editable
   copy. Later policy changes require separate human review and cannot approve
   themselves.
4. **Durable check results.** While the evidence log remains writable, display
   failure must not stop checks or discard their actual durable result; the
   CLI still returns an error. Evidence persistence failure cannot be reported
   as successfully recorded evidence.
5. **Independent review and bounded repair.** Authors cannot approve their own
   implementation. P0/P1 findings and unmet acceptance criteria block passing;
   lower-severity findings retain ownership and disposition. Re-run affected
   checks and review after candidate changes. Start with at most three repair
   rounds, escalating recurring failure, ambiguous requirements or reviewer
   disagreement to a human. Review/run completion is not human acceptance.

The canonical operational rules remain in [workflow](../docs/workflow.md);
the narrow path decision is [ADR 0003](../docs/adr/0003-ascii-task-scopes.md).
Adoption does not claim that procedural gates are machine-enforced.

## Evidence and boundaries

The [publication handoff](0003-publication.md) retains the exact PR #1 source,
independent technical dispositions and three-OS CI evidence. Its pending-policy
statements, and those in older reports and frozen contracts, describe the
earlier decision state; this record supersedes policy/M0 status only. Current
spec status headers reflect the later decisions without changing contract
bodies. No historical reports or frozen inputs are rewritten, and no new independent
review or CI result is claimed for this documentation continuation.

Local verification on macOS arm64 with Go 1.27.3: `git diff --check` and
`go run ./cmd/dev check --run 20261110T143714Z-7312425c7fe8 --race` passed.
Go checks do not approve prose. The initial uncommitted candidate is preserved
as `evidence/policy-adoption-candidate.tar` with a per-file hash/mode manifest
at `evidence/policy-adoption-candidate.json` in that local run. These identify
the candidate separately from its base commit; neither is a remote attachment.

PR #1 CI artifacts expire on 2026-11-24 UTC. Local run evidence is ignored and
not backed up by Git; preserve needed raw evidence separately before expiry.

## Next milestone and authority

M0 is accepted and completed. M1 has not started and stage zero is not complete.
Neither policy adoption nor M0 acceptance starts M1, authorizes delegation,
commit/push/merge, remote setting changes, or unattended automation. The owner's
separate PR/merge instruction authorizes publication of this documentation,
not those other actions. The next milestone is M1, requiring separate start
authorization, independent contract review and actual macOS/Windows runtime
access. Publication status, exact candidate checks and review are recorded in
the PR and its controller run rather than inferred from the earlier evidence.
