---
name: fix
description: Fix a reproduced my-bilibili defect through the fast lane with a failing regression, minimal repair, canonical checks, and independent handoff. Use for a bounded reproduced bug, not feature planning, architecture changes, or approval.
---

# Fix one reproduced defect

Follow [agent convergence](../../../docs/agent-convergence.md) R3/R4/R5/R7/R9
for scope, repair limits, observed contracts, triage and re-orientation.

Read [STATUS](../../../STATUS.md), the
[fast lane](../../../docs/workflow.md#risk-lanes), and
[testing guidance](../../../docs/testing.md). Inputs are explicit fix
authority, the reproduced behavior, the assigned write paths, and acceptance
or expected behavior. This skill does not grant a commit, push, PR, merge, or
remote issue action.

Before changing code:

1. Record the checkout, base revision, dirty state, reproduction, and expected
   behavior.
2. Identify the narrow bug class, owner package, callers, and existing tests.
3. If the cause or relied-on contract is not observable, return a bounded
   investigation; do not guess a causal repair.
4. Add a regression that fails on the unfixed behavior and preserves the
   failing evidence.

Implement the smallest change that fixes that exact behavior and covers its
real boundary conditions. Do not widen the task into a similar-bug sweep,
shared-contract change, policy change, or speculative refactor. If the fix
requires any of those, stop and route to `feature-plan`.

Run the targeted regression, then
`go run ./cmd/dev check --race` where supported. State failures and unavailable
checks honestly; never weaken an existing check to make the task pass.

Return the exact changed paths and candidate identity, reproduction and
regression result, observed cause, contract relied on, check evidence, remaining
risk, and smallest next action. Hand the exact candidate to `independent-review`
in a separate context, then use `pr-handoff`. Self-checking, a green run, or
the PR body does not provide independent approval.
