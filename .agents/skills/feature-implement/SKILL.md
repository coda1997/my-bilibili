---
name: feature-implement
description: Implement an assigned my-bilibili task or repair within a reviewed frozen contract, with regression tests and evidence. Use for authorized scoped implementation, not creating scope or approving the result.
---

# Implement the assigned slice

Read the current handoff from [README](../../../README.md), the role packet,
frozen spec/tasks, and relevant code. Use [development](../../../docs/development.md)
and the canonical [workflow](../../../docs/workflow.md) for the execution rules.

Inputs: explicit implementation authority, task IDs and write paths, satisfied
dependencies, reviewed shared contracts, source identity, and required ACs.
Missing or contradictory boundaries return to the controller; do not broaden
scope, edit frozen inputs, or relax a check to make the task pass.

Confirm [tooling and tests](../../../docs/testing.md) before running commands.
Keep implementation and tests inside the assigned paths. For a reproduced bug,
preserve a failing regression before repair. Run targeted checks while working,
then the full assigned check; state actual OS results and anything not tested.

Return changed paths and exact source identity, AC-to-test results, sanitized
evidence, limitations, and the next action. Workers use their own scratch and
return logs; only the controller publishes to its run. Follow the workflow's
bounded repair policy when an independent reviewer requests changes.

Self-checking is not independent approval. Use the repository's standing
delegation authority for scoped sub-agent work. This skill does not grant
commit/push/PR authority, policy adoption, or permission to start another task.
