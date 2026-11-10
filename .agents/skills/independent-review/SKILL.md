---
name: independent-review
description: Review a my-bilibili architecture contract or exact code candidate in an authorized reviewer context, or independently re-verify a finding. Use for requested independent review, not author self-approval.
---

# Review without authoring the candidate

Use the canonical [workflow](../../../docs/workflow.md),
[role packet](../../../docs/templates/assignment.md), and
[review format](../../../docs/templates/review.md). Start from the current
handoff linked by [README](../../../README.md), not the author's desired verdict.

Confirm reviewer identity and independence from implementation. If you authored
the change, provide a packet for a separate authorized reviewer; do not call
self-review independent or dispatch an agent without delegation authority.

Choose the assigned mode:

- Architecture: inspect the proposed spec/task DAG, boundaries, shared async
  contracts, dependency ordering, AC evidence, and permission/policy changes.
  Return a proceed/request-changes/needs-human recommendation before freeze.
- Code or closure: verify the exact candidate identity and diff, frozen
  spec/tasks, relevant code/tests, and original findings for closure. Prioritize
  behavioral defects, unmet ACs, resource/platform risks, and regression gaps.
  Run relevant [checks](../../../docs/testing.md) independently where available.

Keep the candidate read-only. Use isolated scratch for probes and return
sanitized logs; do not write the controller's run or change historical reports.
Do not bind dirty code solely to HEAD or accept baseline CI as changed-candidate
evidence. Report unavailable inputs and checks rather than inventing results.

Return candidate/spec identities, checks actually run, findings with severity,
reproduction and required evidence, and a technical decision. Only the assigned
independent reviewer closes their findings. If the original reviewer cannot
continue, follow the [proposed succession procedure](../../../docs/workflow.md#reviewer-succession-proposed-clarification);
do not assume reassignment or closure authority. Distinguish that decision from
human policy approval, missing CI/platform acceptance, and merge authority.
