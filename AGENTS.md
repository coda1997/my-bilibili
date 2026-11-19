# Repository instructions

Follow [agent convergence](docs/agent-convergence.md) R1–R9 for finding tiers, scope, bounded review, issue triage and completion.

## Read order

For read-only consultations, use the lightweight route in
`docs/workflow.md#read-only-requests`. It requires no implementation packet or
new run, and grants no edit or approval authority. Formal independent reviews
still require their assigned review inputs.

1. Read `STATUS.md`: current state, next actions, blockers and the owner action
   queue. Do not read `reports/` by default; open a report only when STATUS or
   the task links it. Reports describe their own candidates, not the checkout.
2. Verify prerequisites in `docs/testing.md` before invoking Go. Inspect Git
   status and source identity; do not assume ignored tools/runs exist in a clone.
3. Choose the risk lane in `docs/workflow.md#risk-lanes`. For the full lane,
   read `docs/architecture.md`, `docs/workflow.md`, the assigned spec/tasks and
   the role assignment, and use `docs/development.md` for startup and routing.
4. Use `go run ./cmd/dev run list` and `run status --id <id>` for local run state.
   An empty local run list does not erase the durable handoff.

## Task routing

- Reproduced fast-lane defect: [`fix`](.agents/skills/fix/SKILL.md), with a failing regression, smallest repair,
  `go run ./cmd/dev check --race`, an evidence table, and independent review.
- Planning: [`feature-plan`](.agents/skills/feature-plan/SKILL.md), `docs/templates/spec.md`, and `docs/templates/tasks.json`.
- Implementation: [`feature-implement`](.agents/skills/feature-implement/SKILL.md), the assigned task, and `docs/testing.md`.
- Independent architecture/code review: [`independent-review`](.agents/skills/independent-review/SKILL.md) and
  `docs/templates/review.md`, in a separate authorized agent/context.
- Unavailable reviewer: use the succession procedure in
  `docs/workflow.md#reviewer-succession`; do not infer reassignment or closure
  authority.
- PR or handoff preparation: [`pr-handoff`](.agents/skills/pr-handoff/SKILL.md) and `docs/pull-requests.md`.
- Owner-only steps: add them to the owner action queue in `STATUS.md` and batch
  them; see `docs/workflow.md#owner-actions`.

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
  It also validates the checked skill metadata, routes, and local Markdown links.
- The controller owns `.agent/runs/`. Workers submit evidence to the controller;
  a worktree is not a security sandbox or an approval mechanism.
- Bind evidence to a commit SHA and, for CI, the run URL for that SHA. Write
  evidence as a claim/evidence/result/scope table.
- Checkpoint significant progress with `run note`; write the next action and
  remaining blockers in `STATUS.md` before handing off. Only sanitized final
  conclusions go in `reports/`. Do not manually edit published run events or frozen inputs.
- The local recorder is an observability tool, not a trusted merge gate.
- Run records never substitute for independent review. Changes to checks,
  workflow policy, task scope, or CI need separate human review before use as
  trusted automation policy.

## Code review rules

- Prioritize behavioral defects, unmet acceptance criteria, platform/resource
  risks, and missing regression tests. Select required review dimensions from
  [risk-selected independent review](docs/workflow.md#risk-selected-independent-review);
  formatting belongs to deterministic checks.
- Bind findings and closure evidence to the exact candidate and spec. A passing
  baseline CI run is not CI evidence for a changed working tree.

## Boundaries

- Keep UI framework types and Bilibili transport DTOs out of business modules.
- Add abstractions at real external boundaries, not for hypothetical reuse.
- Keep the initial application a single Go module and modular monolith.
- Do not add a generic agent framework, database, event bus, updater, or release
  credentials before a stage-zero acceptance criterion requires it.

## Implementation discipline

Prefer explicit contracts, direct control flow, and the smallest change that
fully satisfies the assigned acceptance criteria.

### Validation and errors

- Validate external input at its owning boundary: CLI arguments, files,
  network responses, subprocess output, and native callbacks. Keep transport
  decoding and conversion in adapters; keep business rules in their owning
  domain packages.
- Reuse established internal invariants. Do not repeat the same validation
  across internal layers or add guards for states the contract excludes.
  Verify where an invariant is established before relying on it; a Go type
  alone does not prove semantic validity.
- Preserve checks for cancellation, state transitions, stale results, and
  resource ownership. These enforce runtime contracts.
- Return and propagate errors explicitly. Add context when it helps diagnosis,
  preserving error identity where callers depend on it. Never turn a failure
  into an empty result, default value, or success unless the contract defines
  that behavior.
- Do not use panic for expected input, I/O, or domain failures. Do not add
  broad recover handlers that hide programming defects. Keep required cleanup
  and the established error-sanitization boundary intact.

### Scope and architecture

- Before a non-trivial change, identify the owning package, affected callers,
  dependency direction, and relevant state or lifetime contracts.
- Search existing capability packages before adding helpers or interfaces.
  Keep helpers local until there is a concrete shared need; follow
  docs/architecture.md for boundary ownership.
- If the request conflicts with an approved contract or requires changes
  outside assigned paths, explain the concrete conflict and proposed trade-off
  before implementation. Resolve routine choices within the existing design.
- Match surrounding Go style and concurrency patterns. Avoid unrelated
  refactors, renames, speculative fallbacks, compatibility layers, and unused
  configuration.
- Do not remove existing checks merely to shorten code. Establish that a check
  is redundant against the actual contract and preserve relevant regression
  coverage.

### Role discipline

- Planning and architecture review produce contracts, trade-offs, and review
  findings; they do not authorize implementation.
- Implementation follows the reviewed contract and assigned paths, with the
  checks required by docs/testing.md.
