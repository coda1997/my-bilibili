---
name: feature-plan
description: Plan a scoped my-bilibili feature or repair by producing a spec, task DAG, and architecture-review packet. Use for feature planning or task decomposition, not implementation or approval.
---

# Plan a bounded change

Start with [README](../../../README.md), the current handoff, and the selected
milestone. Follow [development](../../../docs/development.md) and the canonical
[workflow](../../../docs/workflow.md); this skill grants no extra authority.

Inputs: the user's observable goal, constraints, relevant architecture/code,
and explicit scope. Use the repository's standing delegation authority.
If a milestone or contract
decision blocks the request, surface that decision rather than assuming it.

Use the [spec](../../../docs/templates/spec.md) and
[task](../../../docs/templates/tasks.json) templates. Define successful and
failing behavior, real module boundaries, shared async contracts where needed,
and AC-to-test/platform evidence. Replace all template placeholders. Keep role
metadata outside the strict task JSON. Order overlapping writes in the DAG.

Validate the instantiated contract using the real repository checks after
verifying [prerequisites](../../../docs/testing.md#prerequisites). Do not claim
that schema validation establishes the prose acceptance criteria.

Return the proposed spec/tasks, unresolved decisions, validation actually run,
and an [architecture-review packet](../../../docs/templates/assignment.md).
Dispatch independent review under the repository's standing delegation authority.
Review must come from a separate agent/context; planning does not authorize implementation, a contract
freeze, a commit, or remote changes.
