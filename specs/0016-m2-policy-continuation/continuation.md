# M2 policy transition and retained ledger

## Owner decision and limited scope

On 2026-11-16 the owner explicitly agreed to both proposed changes:

> 同意：没有阻塞时，可以用明确标注的演练验证交接流程；- 按新规则计算预算：同意

Apply the current `docs/agent-convergence.md` R1–R9 policy from commit
`da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` to this M2 continuation. The owner did
not approve the separate suggestion to reduce player UI scope. The original
product/native behavior, checks, task IDs and write paths remain unchanged.
Only M2-AC-06's policy and handoff evidence alternative change. Actual bounded
repair evidence remains required; no synthetic finding satisfies that element.

The transition becomes operational after independent contract review and a new
freeze. It supersedes the older policy for continuing these same tasks without
changing the meaning of historical packets or findings. No additional review
or fix allowance was requested or granted. Remaining allowance is not a request
to perform additional reviews of already approved, unchanged code.

## Exact historical records

Prior run: `20261115T133635Z-984c0955c8cb`, evidence beneath
`.agent/runs/20261115T133635Z-984c0955c8cb/evidence/`.

- Architecture r1: `.agent/m2-preparation/architecture-r1/` retained candidate.
  The original serialized review raised M2-SER-001/002/003. Architecture r2
  corrected loading intent, live-process disposal observation and the UI-facing
  engine boundary. Reviewer `/root/m2_plan_review` approved and closed all three;
  see `architecture-review.md`. These were two formal architecture rounds and
  one contract correction/reverification, not M2-001 implementation repairs.
- Approved architecture r2 manifest:
  `0c07c39265ed982cd7f06b93dc541c2497e2321070f530bedd8217c8f16bf1d2`.
  Original spec: `63b130dccc54c6a03d586b36a1fff2a8eaf7521db5e6ed19565f38ee5ce2f64e`.
  Original tasks: `e9cbc57558c00698b467bb215c282fc9c67df625740bb03fa97230b123f06e23`.
- M2-001 initial code review raised M2-001-IMPORT-001; one repair made cgo=0/1
  dependency checks cover all three targets. Original reviewer
  `/root/m2_shared_review` independently closed it in the second formal review.
  See `m2-001-review-r1.md`, `m2-001-review-r2.md`, targeted/negative-probe logs
  and `m2-001-r2-full-check.log`. Approved manifest:
  `852cd8447d30db1244cf7c8fcfe9eac217d7514b408376d051265757a961eebb`;
  archive: `7f14dd1da0602ddfc66fc303f7189059f2977b114d498cfa7d3edeb83eb598b8`.
- Runtime build/loader prerequisite has one approved formal review and one
  verified correction after an initial missing-header build failure. See
  `runtime-prerequisite-review.md`, `runtime-resume-assignment.md` and the
  ignored `.agent/runtime/mpv-0.42.0/m2-rebuild-r1/` build records. Its manifest
  is `a5fb151f6c41d6c0eb12f214ec187581b0589dc57f37911887cf133eda99fb54`.
  Native cleanup observations are separate controller evidence, not extra
  review rounds or product acceptance.

The former P1 architecture findings are already independently closed; no open
high-severity claim is carried into this transition. Do not retroactively label
the historical reviews with a policy they did not use. Current findings use R1
tiers/classes and R6 rulings. Author summaries never replace reviewer closure.

## Cumulative budget at submission

| Scope | Formal rounds completed | Fix iterations completed | Repeated failure history | Remaining before this submission |
| --- | --- | --- | --- | --- |
| M2 architecture/contract, including this continuation | 2 | 1 | Original three contract defects corrected once; no failed correction recorded | 1 review, 2 fixes; next review is final round 3 |
| M2-001 declarations/import gate | 2 | 1 | Cgo omission reproduced before repair, then closed; no failed repair | 1 review, 2 fixes; already approved, no new review needed here |
| M2-002 application/fixtures | 0 | 0 | None; not dispatched | 3 reviews, 3 fixes |
| M2-003 desktop/surface | 0 | 0 | None; not dispatched | 3 reviews, 3 fixes |
| M2-004 integration/package/smoke | 0 | 0 | None; not started | 3 reviews, 3 fixes |
| M2-005 final evidence/handoff | 0 | 0 | None; not started | 3 reviews, 3 fixes |
| Runtime reconstruction prerequisite | 1 | 1 | Initial header-include build failure once, corrected build and loader approved | 2 reviews, 2 fixes; already approved |

These are retained work-unit budgets, not newly allocated pools for the same
change. A review covering several tasks updates every affected task's ledger
and consolidates all dimensions. M2-005 cannot supply a fresh review budget for
exhausted M2-001–004 code or hide an extra final sweep. Known negative probes
that correctly reject forbidden input are successful regression observations,
not repeated automatic fix failures.

This contract submission consumes formal architecture round 3. If it has an
unresolved BLOCKING finding, stop automatic contract repair/review and return
the evidence and decision request to the owner; do not use the remaining fix
count to create a fourth review. Record the review outcome in new controller
evidence, not by rewriting this published packet. Existing completed repairs
remain completed. Later scope changes retain this history and require whatever
explicit additional allowance R4 calls for.

## One finding inventory and issue budget

Logical work remains the original M2 run and its continuations. New remote
issues created: 0 of the maximum 5; remote filing authority is absent. No remote
dedupe or publication is claimed. The controller's next run continues this
inventory and uses one local follow-up list; an authorized future advisory
issue must be a single qualifying batch, not one issue per comment.

| ID/item | Actual disposition | Destination / filing decision |
| --- | --- | --- |
| M2-SER-001/002/003 | Independently closed in architecture r2 | Historical architecture review; no open issue to file |
| M2-001-IMPORT-001 | Independently closed in code r2 | Historical code review; no open issue to file |
| Runtime missing-header build failure | Corrected and prerequisite approved | Runtime build records; no unresolved user-impact issue |
| Proposed smaller UI scope | Not selected by the owner | Local planning follow-up only; product preference below R5 filing threshold |
| Worktree creation and eventual commit authority | Still required before affected actions | Controller continuation; authority decision, not a product defect/remote issue |

No systemic follow-up warranted by this policy-only change. No new observed
code defect, recurring defect category or unsupported causal repair is claimed.

## Completion and remaining gaps

For this update, done means the two owner decisions are serialized accurately,
old inputs remain unchanged, current budgets are explicit, required checks pass,
and independent contract review approves the exact new candidate before freeze.
Observed evidence is the owner decision, preserved source/review records and
actual checks. Any expected benefit of simpler escalation is an inference.
The relied-on contracts are R1–R9 and this continuation's M2-AC-06.

The handoff drill itself has not run. Product implementation, two worktrees,
package/native checks, exact-commit integration and M2 acceptance remain pending.
These are the existing product task backlog, not work authorized by this
policy-only update. Next: finish this scoped continuation review/freeze, then
continue the owner's M2 discussion without silently dispatching old workers.
