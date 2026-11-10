# Repository instructions

## Read order

1. Read `docs/stage-zero.md` and the relevant handoff in `reports/`.
2. Read `docs/architecture.md`, `docs/workflow.md`, and the assigned spec.
3. Use `go run ./cmd/dev run list` and `run status --id <id>` for local run state.

## Work rules

- Keep product code and maintained development tools in Go. System media APIs
  are permitted; a JavaScript application layer is not the default.
- Work only inside the assigned task's write paths. Report contract or scope
  problems; do not silently weaken acceptance criteria or checks.
- Do not spawn sub-agents unless the user or a task's explicit authorization
  asks for delegation. Describing the future workflow is not authorization.
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

## Boundaries

- Keep UI framework types and Bilibili transport DTOs out of business modules.
- Add abstractions at real external boundaries, not for hypothetical reuse.
- Keep the initial application a single Go module and modular monolith.
- Do not add a generic agent framework, database, event bus, updater, or release
  credentials before a stage-zero acceptance criterion requires it.
