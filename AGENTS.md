# Repository instructions

## Read order

1. Follow `README.md` to the current handoff and `docs/stage-zero.md`. Historical
   reports describe their own candidates, not necessarily the current checkout.
2. Verify prerequisites in `docs/testing.md` before invoking Go. Inspect Git
   status and source identity; do not assume ignored tools/runs exist in a clone.
3. Read `docs/architecture.md`, `docs/workflow.md`, the assigned spec/tasks, and
   the role assignment. Use `docs/development.md` for feature startup and routing.
4. Use `go run ./cmd/dev run list` and `run status --id <id>` for local run state.
   An empty local run list does not erase the durable handoff.

## Task routing

- Planning: `feature-plan`, `docs/templates/spec.md`, and `docs/templates/tasks.json`.
- Implementation: `feature-implement`, the assigned task, and `docs/testing.md`.
- Independent architecture/code review: `independent-review` and
  `docs/templates/review.md`, in a separate authorized agent/context.
- PR or handoff preparation: `pr-handoff` and `docs/pull-requests.md`.

These repo-local skills live in `.agents/skills/`. If skill discovery is not
available, read the matching `SKILL.md` directly. Skills are procedures, not
additional authority. Keep detailed policy in the linked protocol, not copies
inside every skill. Assign roles with `docs/templates/assignment.md`.

## Work rules

- Keep product code and maintained development tools in Go. System media APIs
  are permitted; a JavaScript application layer is not the default.
- Work only inside the assigned task's write paths. Report contract or scope
  problems; do not silently weaken acceptance criteria or checks.
- Freeze shared contracts before splitting implementation. Record the base
  revision and exact candidate identity; a dirty flag plus HEAD is not a snapshot.
- Use native sub-agents at your discretion for work within the user's task.
  The owner grants standing delegation authority; no per-dispatch permission
  is needed. Keep assignments scoped and preserve reviewer independence.
- Do not claim independent review when the implementation author performed it.
- Do not claim a playback, platform, packaging, or CI check that was not run.
- Preserve existing user work. Do not commit, push, publish, or change remote
  settings without a request to do so.
- Keep secrets, live account cookies, signed media URLs, and private media out
  of specifications, fixtures, logs, screenshots, and reports.

## Evidence and checks

- Run `go run ./cmd/dev check` for a code change; use `--race` where supported.
- The controller owns `.agent/runs/`. Workers submit evidence to the controller;
  a worktree is not a security sandbox or an approval mechanism.
- Checkpoint significant progress with `run note`; write the next action and
  remaining blockers before handing off. Only sanitized final conclusions go
  in `reports/`. Do not manually edit published run events or frozen inputs.
- The local recorder is an observability tool, not a trusted merge gate.
- Run records never substitute for independent review. Changes to checks,
  workflow policy, task scope, or CI need separate human review before use as
  trusted automation policy.

## Code review rules

- Prioritize behavioral defects, unmet acceptance criteria, platform/resource
  risks, and missing regression tests. Formatting belongs to deterministic checks.
- Bind findings and closure evidence to the exact candidate and spec. A passing
  baseline CI run is not CI evidence for a changed working tree.

## Boundaries

- Keep UI framework types and Bilibili transport DTOs out of business modules.
- Add abstractions at real external boundaries, not for hypothetical reuse.
- Keep the initial application a single Go module and modular monolith.
- Do not add a generic agent framework, database, event bus, updater, or release
  credentials before a stage-zero acceptance criterion requires it.
