# Phase 1 structured run telemetry, R4 guardrails, and architecture boundaries

Spec ID: `0031-dev-structured-telemetry-guardrails`
Status: proposed for independent architecture review (Round 2); authorized by owner on 2026-11-25 in isolated worktree branch `codex/phase1-telemetry-guardrails` from `origin/master` (`66f00fc`). Because this spec modifies `internal/runlog`, `internal/spec`, `cmd/dev`, `internal/architecture`, and workflow docs, separate human policy approval on the Draft PR remains required before merge.

## User scenario and goal

Before introducing a Go DAG controller (Phase 2) or RSI reflection/eval loop (Phase 3), the repository needs `cmd/dev` and its supporting packages to turn procedural workflow rules into deterministic, machine-readable Go invariants:

- Structured telemetry events in `.agent/runs/` (task ID, role, review round, fix iteration, failure signature, duration, and R1/R8 findings) while remaining read-compatible with existing Schema v1 runs.
- Code-enforced R4 convergence limits (max 3 review rounds, max 3 fix iterations, 3rd identical failure signature circuit breaker, and explicit human-override unlock without erasing history).
- Runtime `git diff` write-scope verification against `tasks.json` `write_paths` (`dev verify-scope`).
- Complete cross-OS and test-file import boundary enforcement across all domain packages (`catalog`, `app`, `account`, `playback`, `danmaku`, `history`) and `experiments/` isolation.

## Scope and non-goals

Deliver three cohesive tasks in series:

- `TEL-001`: Pure write-scope matcher in `internal/spec` (`CheckWriteScope`), Schema v2 structured telemetry & R4 budget enforcement in `internal/runlog` (backward-compatible with v1 runs on disk), and full unit tests.
- `TEL-002`: `cmd/dev` CLI wiring for structured `dev run note` telemetry flags + `--findings-file`, `dev run override`, `dev verify-scope`, enriched `dev run status` budget output, and CLI tests.
- `TEL-003`: Expanded cross-OS and `-test` architecture import guards in `internal/architecture`, plus updated operational documentation (`docs/architecture.md`, `docs/workflow.md`, `docs/testing.md`, `STATUS.md`) and non-circular handoff report (`reports/0044-phase1-telemetry-guardrails.md`).

Non-goals:
- No changes to `.github/workflows/check.yml` or `tasks.json` schema (`schema_version` remains `1`).
- No Go DAG orchestrator, worktree scheduler, LLM runner, or `dev reflect` / `evals/` harness (deferred to Phases 2–3).
- No product runtime code changes (`internal/app`, `internal/catalog`, `internal/account`, `internal/playback`, `internal/danmaku`, `internal/history`, `internal/desktop`, `cmd/my-bilibili`).
- No edits to the primary checkout containing uncommitted `0028`/`0029` work.

## Boundaries and shared contracts

### 1. `internal/spec` write-scope verification contract

- Export `CheckWriteScope(def Definition, taskID string, changedPaths []string) ([]string, error)`.
- Validates `taskID` exists in `def.Tasks`; rejects unknown task IDs with an error.
- Validates the matched task has non-empty `WritePaths` where every entry passes `ValidPath` and uses ASCII-only characters (failing closed even if called with an unvalidated `Definition`).
- For each path in `changedPaths`, rejects empty or unsafe paths (`!ValidPath(path)`) as an error.
- A valid relative path `p` is permitted iff some `w` in `task.WritePaths` satisfies `p == w || strings.HasPrefix(p, w+"/")` (exact forward-slash case-sensitive matching, fail-closed).
- Collects all unauthorized paths, deduplicates and sorts them deterministically, and returns `(violations, fmt.Errorf(...))` when `len(violations) > 0`, or `(nil, nil)` when all paths are permitted.
- Pure, synchronous, zero filesystem or Git I/O.

### 2. `internal/runlog` Schema v2 and R4 convergence contract

- **Manifest versions and frozen task validation:**
  - `Store.Start` writes `SchemaVersion: 2`. Before creating a run, `Store.Start` strictly decodes `snapshot.TaskJSON` (`DisallowUnknownFields`, single JSON object) and verifies it matches `snapshot.Definition` and passes `spec.Validate`, preventing unreadable runs.
  - `Store.Read` accepts `SchemaVersion == 1` and `SchemaVersion == 2` so historical v1 runs remain readable without migration.
  - After verifying `inputs/spec.md` and `inputs/tasks.json` SHA-256 digests, `Store.Read` strictly decodes `inputs/tasks.json` into `spec.Definition`, checks `def.SpecID == Manifest.SpecID`, and validates it so `TaskID` references are checked against the immutable frozen task graph.
  - **Schema v1 immutability rule:** On `SchemaVersion == 1` runs, all v2-only event fields (`TaskID`, `Role`, `ReviewRound`, `FixIteration`, `FailureSignature`, `DurationMS`, `Findings`, `Override`) and `Kind == "human-override"` are rejected on both `append()` and `Read()`.
- **Event kinds and allowed v2 fields (Schema v2, strict `DisallowUnknownFields`):**
  - `ReviewRound >= 0`, `FixIteration >= 0`, and `DurationMS >= 0` unconditionally on all events (negative values are always rejected).
  - `Kind == "started"` (`Sequence == 1`): forbids `Phase`, `Outcome`, `Evidence`, and ALL v2 fields (`TaskID`, `Role`, `ReviewRound`, `FixIteration`, `FailureSignature`, `DurationMS`, `Findings`, `Override`).
  - `Kind == "finished"`: requires valid `Outcome` (`succeeded`, `failed`, `needs-human`), forbids `Phase` and ALL v2 fields (`TaskID`, `Role`, `ReviewRound`, `FixIteration`, `FailureSignature`, `DurationMS`, `Findings`, `Override`). Additionally, `Outcome == "succeeded"` is rejected while any scope in the run has `FailureHalted == true` (`failed` and `needs-human` remain permitted for escalation).
  - `Kind == "human-override"`: forbids `Phase`, `Outcome`, `Role`, `ReviewRound`, `FixIteration`, `FailureSignature`, `DurationMS`, `Findings`. Requires non-empty `Message` (explicit human direction), valid `Source`, optional `Evidence`, optional `TaskID` (validated against frozen `tasks.json` when non-empty), and non-nil `Override *HumanOverride` (`json:"override,omitempty"`):
    ```json
    {
      "extra_review_rounds": 0,
      "extra_fix_iterations": 0,
      "reset_failure_signatures": false
    }
    ```
    Bounds: `0 <= ExtraReviewRounds <= 100`, `0 <= ExtraFixIterations <= 100`, total cumulative allowed rounds/iterations per scope capped at `1000` (preventing signed integer overflow), and at least one of `ExtraReviewRounds > 0 || ExtraFixIterations > 0 || ResetFailureSignatures` must be set. Run-level overrides (`TaskID == ""`) may only set `ExtraReviewRounds > 0` (`ExtraFixIterations` must be `0` and `ResetFailureSignatures` must be `false`). Overrides never erase or decrement historical events or `review_rounds`/`fix_iterations` counters.
  - `Kind == "checkpoint"`: requires valid `Phase` (`planning`, `implementation`, `validation`, `review`, `handoff`), forbids `Outcome` and `Override`, and allows optional v2 fields:
    - `TaskID string` (`json:"task_id,omitempty"`): when non-empty, must exist in frozen `inputs/tasks.json`. Scope key is `TaskID` when non-empty, or `"_run"` when empty.
    - `Role string` (`json:"role,omitempty"`): allowlisted to `lead`, `architecture-review`, `implementation`, `code-review`, `controller`, `handoff`.
    - `ReviewRound int` (`json:"review_round,omitempty"`): when `> 0`, valid only on `Phase == "review"`. Must be non-decreasing per scope key (`>= prior MaxReviewRound`) and `<= AllowedReviewRounds` (default `3`, extended only by `human-override`).
    - `FixIteration int` (`json:"fix_iteration,omitempty"`): when `> 0`, requires `TaskID != ""`, valid only on `Phase == "implementation"` or `Phase == "validation"`. Must be non-decreasing per task (`>= prior MaxFixIteration`) and `<= AllowedFixIterations` (default `3`, extended only by `human-override`).
    - `FailureSignature string` (`json:"failure_signature,omitempty"`): when non-empty, requires `TaskID != ""`, `len(sig) <= 200` UTF-8 bytes, no control characters, and `strings.TrimSpace(sig) == sig`. Tracks active-window occurrences per `(task, signature)` since run start or the most recent `ResetFailureSignatures` override for that task. On reaching count `3`, that 3rd failure event is recorded and sets `FailureHalted = true` for that task. While `FailureHalted == true`, any subsequent `Phase == "implementation"` checkpoint, `FixIteration > 0`, or further `FailureSignature != ""` event for that task is rejected until an explicit `human-override` with `ResetFailureSignatures: true` is appended (`handoff`, `review`, and `finish --outcome needs-human` / `failed` remain permitted).
    - `DurationMS int64` (`json:"duration_ms,omitempty"`): `>= 0`.
    - `Findings []Finding` (`json:"findings,omitempty"`): exact JSON schema:
      ```json
      {
        "id": "CR-01",
        "tier": "BLOCKING",
        "blocking_class": 1,
        "severity": "P1",
        "file": "internal/runlog/store.go",
        "recurring_key": "class1:internal/runlog",
        "disposition": "open"
      }
      ```
      Validation rules:
      - `ID` (`json:"id"`): required, `1..64` UTF-8 bytes, matching `^[A-Za-z0-9][A-Za-z0-9_.-]*$`. Duplicate `ID` within the same event is rejected.
      - `Tier` (`json:"tier"`): required, `"BLOCKING"` or `"ADVISORY"`.
      - `BlockingClass` (`json:"blocking_class,omitempty"`): required `1..7` when `Tier == "BLOCKING"`; must be `0` when `Tier == "ADVISORY"`.
      - `Severity` (`json:"severity,omitempty"`): optional, allowlisted to `P0`, `P1`, `P2`, `P3`, `critical`, `high`, `medium`, `low`.
      - `File` (`json:"file,omitempty"`): optional, must pass `spec.ValidPath`.
      - `RecurringKey` (`json:"recurring_key,omitempty"`): optional, `<= 128` UTF-8 bytes, no whitespace or control characters.
      - `Disposition` (`json:"disposition,omitempty"`): optional (`""` allowed). When `Tier == "BLOCKING"`, allowlisted to `""`, `"open"`, or `"closed"` (`"addressed"`, `"deferred"`, and `"not-filed"` are rejected on `BLOCKING` findings per R1/R2). When `Tier == "ADVISORY"`, allowlisted to `""`, `"open"`, `"closed"`, `"addressed"`, `"deferred"`, `"not-filed"`.
      - `open_blocking` transitions per scope key: a `BLOCKING` finding with `Disposition == ""` or `"open"` marks `ID` open in that scope; ONLY a subsequent `BLOCKING` finding with the same `ID` and `Disposition == "closed"` in the same scope removes it from `open_blocking`. Re-emitting an already-open `BLOCKING` `ID` in the same scope with `Tier == "ADVISORY"` is rejected (cannot silently downgrade/close a blocker).
- **Replay validation and derived budget summary:**
  - All R4 and v2 rules execute both on `append()` (preventing invalid publication) and on `Read()` (failing closed on tampered event files).
  - Preserve existing `Store.Note(...)` and `Store.Finish(...)` signatures; add `Store.NoteWithOptions(id string, opts NoteOptions, source Source) error` and `Store.HumanOverride(id, taskID, message string, override HumanOverride, paths []string, source Source) error`.
  - `View` and `Summary` expose `Budgets map[string]BudgetStatus` (`json:"budgets,omitempty"`), populated only for scopes that have recorded v2 telemetry or overrides (omitted when empty). Exact `BudgetStatus` JSON fields:
    - `review_rounds` (`int`) — highest recorded `review_round` in this scope
    - `allowed_review_rounds` (`int`) — `3` + cumulative `extra_review_rounds`
    - `fix_iterations` (`int`) — highest recorded `fix_iteration` in this scope
    - `allowed_fix_iterations` (`int`) — `3` + cumulative `extra_fix_iterations`
    - `failure_counts` (`map[string]int,omitempty`) — active-window counts since run start or last `reset_failure_signatures` override (cleared on reset; immutable historical events in `events/` are never modified)
    - `failure_halted` (`bool`) — true while any active-window `failure_signature` count is `>= 3`
    - `open_blocking` (`[]string,omitempty`) — lexicographically sorted open `BLOCKING` finding IDs in this scope
- Concurrency and durability: preserves the existing `.write-lock` directory lock, atomic `.pending-*` temp-file sync + rename, and fail-closed symlink/digest verification.

### 3. `cmd/dev` CLI contract

- `dev run note`: preserves existing flags and adds optional `--task`, `--role`, `--review-round`, `--fix-iteration`, `--failure-signature`, `--duration-ms`, `--findings-file <path>` (strict JSON array `[]runlog.Finding`, max 1 MiB regular file, `DisallowUnknownFields`, no trailing JSON).
- `dev run override`: `--id <run-id> [--task <task-id>] --message <human-direction> [--extra-review-rounds N] [--extra-fix-iterations N] [--reset-failure-signatures] --confirm-human-override [--evidence evidence/<file>]`. Fails closed unless `--confirm-human-override` is explicitly passed. Usage and documentation explicitly note that `--confirm-human-override` records explicit human direction to extend local R4 budgets, is not cryptographic authentication, and never grants independent review, policy approval, or merge authority.
- `dev verify-scope`: `--spec specs/<id> --task <task-id> [--base <rev>] [--head <rev>]`.
  - Default `--base` is `HEAD`.
  - Input hardening: reject any `--base` or `--head` value starting with `-` or containing whitespace/control characters.
  - Revision resolution:
    - When `--head` is set: resolve both `--base` and `--head` via `git rev-parse --verify --quiet --end-of-options <rev>^{commit}` (fail closed if unborn or invalid), then inspect exact committed range `git diff --name-only --no-renames -z <baseSHA> <headSHA> --`.
    - When `--head` is empty: attempt `git rev-parse --verify --quiet --end-of-options <base>^{commit}`. If `--base == "HEAD"` and the repository has an unborn `HEAD` (exit code 1 on `rev-parse --verify --quiet HEAD`), collect the union of `git diff --cached --name-only --no-renames -z --` and `git ls-files --others --exclude-standard -z`. Otherwise (resolved `<baseSHA>`), collect the union of:
      1. `git diff --cached --name-only --no-renames -z <baseSHA> --` (staged changes relative to `<baseSHA>`),
      2. `git diff --name-only --no-renames -z <baseSHA> --` (working-tree changes relative to `<baseSHA>`), and
      3. `git ls-files --others --exclude-standard -z` (untracked non-ignored files).
  - Deduplicate and sort collected paths, evaluate via `spec.CheckWriteScope`, and emit structured JSON on stdout for every scope evaluation (both pass and violation):
    ```json
    {
      "spec_id": "0031-dev-structured-telemetry-guardrails",
      "task_id": "TEL-001",
      "base": "<resolved SHA or unborn>",
      "head": "<resolved SHA or empty>",
      "ok": true,
      "checked_paths": ["internal/spec/spec.go"],
      "violations": []
    }
    ```
    Flag, spec, or Git errors fail before evaluation via stderr and non-zero exit without printing a scope JSON payload. When `ok == false`, stdout receives the JSON payload (with sorted `violations`) and the command exits non-zero.

### 4. `internal/architecture` import boundary contract

- All three architecture boundary tests use `go list -test -deps -json` across all 6 CI targets (`darwin/arm64`, `linux/amd64`, `windows/amd64` × `cgo={0,1}`).
- **Synthetic `-test` package matching rule (anti-vacuity):** For any permitted package path `<p>`, allow ONLY exact `<p>`, `<p>.test`, `<p> [<p>.test]`, and `<p>_test [<p>.test]`. Never blindly skip packages with `ForTest != ""` (which would exempt forbidden non-standard dependencies rebuilt for tests). Assert that every inspected root package's `<p>.test` synthetic package was actually observed in `go list` output so `-test` coverage cannot silently become vacuous.
- `TestBusinessDependencyBoundary`: inspects `./internal/catalog`, `./internal/app`, `./internal/playback`, `./internal/danmaku`, and `./internal/history`, permitting only standard library packages and the allowed plain business packages (`my-bilibili/internal/{catalog,app,playback,danmaku,history}` and their exact synthetic test forms).
- `TestAccountDependencyBoundary`: inspects `./internal/account`, permitting only standard library packages (and rejecting `net/http`, any `net/http/...` subpackage, and any test-rebuilt variant of `net/http` or `net/http/...`) plus `my-bilibili/internal/account` and its exact synthetic test forms.
- `TestProductDoesNotImportExperiments`: inspects `./cmd/my-bilibili` and `./internal/...` across all 6 CI targets for both `tags=""` and `tags="e2e"` using `-test -deps -json` (not `strings.Fields`), rejecting any dependency whose base import path (before any ` [` test suffix) starts with `my-bilibili/experiments/`, and asserting `my-bilibili/internal/adapters/mpv` is observed on `darwin/arm64` `cgo=1` and `cmd/my-bilibili.test` is observed on every target.

## Acceptance criteria

### TEL-AC-01

Given a frozen spec and `tasks.json`, when `internal/spec.CheckWriteScope` or `dev verify-scope` runs against committed (`--base`/`--head`), staged-only, unstaged, renamed (`--no-renames`), or untracked files (including unborn `HEAD`), then in-scope paths return JSON with `"ok": true` and exit 0, while any out-of-scope path (including staged-only changes reverted in the working tree), unknown task ID, unsafe path, invalid/flag-like Git revision, or invalid spec fails closed with deterministic sorted violations and non-zero exit.

### TEL-AC-02

Given a Schema v2 run in `internal/runlog`, when structured checkpoints (`task_id`, `role`, `review_round`, `fix_iteration`, `failure_signature`, `duration_ms`, `findings`) are recorded, then valid telemetry persists and surfaces in `View.Budgets`/`Summary.Budgets` with exact JSON fields, existing Schema v1 runs remain readable while rejecting v2 events/fields on v1 manifests, `Store.Start` rejects mismatched `TaskJSON`, and invalid task IDs, roles, finding tiers/classes/dispositions, duplicate finding IDs, negative counters/durations, v2 fields on `started`/`finished`, or unknown JSON fields fail closed without publishing an event.

### TEL-AC-03

Given an open Schema v2 run subject to R4 limits, when an agent attempts a 4th review round, a 4th fix iteration, a non-monotonic round/iteration regression, a `BLOCKING` finding downgrade or non-`closed` closure, a `finish --outcome succeeded` while halted, or any repair/implementation attempt after recording the 3rd identical `failure_signature` for a task, then `internal/runlog` rejects the write without mutating history (while still permitting `handoff`, `review`, or `finish --outcome needs-human`/`failed`); only an explicit `human-override` event (requiring `--confirm-human-override` in `cmd/dev`, bounded against overflow) extends allowances or clears active-window failure halt while preserving prior events, and tampered on-disk events violating R4 fail closed on `Read()`.

### TEL-AC-04

Given the repository codebase, when `go test -count=1 -race ./internal/architecture` and `go run ./cmd/dev check --race` run, then business dependency guards cover `catalog`, `app`, `account`, `playback`, `danmaku`, and `history` (including `-test` packages with anti-vacuity `.test` assertions and `net/http/...` rejection in `account`) across all 6 CI targets, `experiments/` isolation covers all 6 CI targets with `-test -deps -json` and both `tags=""` and `tags="e2e"`, and docs (`docs/architecture.md`, `docs/workflow.md`, `docs/testing.md`, `STATUS.md`) plus `reports/0044-phase1-telemetry-guardrails.md` accurately document the verified capabilities without circular SHA claims.

## Verification matrix

| AC | Test/command or manual procedure | Fixture and OS | Required evidence |
| --- | --- | --- | --- |
| TEL-AC-01 | `go test -count=1 -race ./internal/spec ./cmd/dev` (`TestCheckWriteScope`, `TestVerifyScopeCLI` covering in-scope, out-of-scope, staged-only reverted in worktree, untracked, renamed-out, `--base`/`--head`, unborn HEAD, flag-injection rejection, unknown task) | Temporary Git repositories, local macOS + 3-OS PR CI | All negative controls fail non-zero with exact sorted violations; JSON output matches frozen schema |
| TEL-AC-02 | `go test -count=1 -race ./internal/runlog ./cmd/dev` (Schema v1 read-compat & v1-rejects-v2-fields fixtures, `Store.Start` mismatched `TaskJSON`, Schema v2 structured telemetry, invalid task/role/finding/duplicate-ID/negative-counter/started-finished-v2/JSON fixtures) | Temporary stores & CLI tests, local macOS + 3-OS PR CI | v1 runs read cleanly; v2 telemetry round-trips; invalid writes fail closed leaving event count unchanged |
| TEL-AC-03 | `go test -count=1 -race ./internal/runlog ./cmd/dev` (4th review round, 4th fix iteration, regression, `BLOCKING` advisory-disposition/downgrade rejection, 3rd identical failure halt, `succeeded` rejected while halted, `needs-human` allowed, `dev run override` requiring `--confirm-human-override` + overflow bounds, tampered on-disk R4 violation failing `Read()`) | Temporary stores & CLI tests, local macOS + 3-OS PR CI | Every R4 boundary fails closed; explicit override extends budget without erasing history |
| TEL-AC-04 | `go test -count=1 -race ./internal/architecture` and `go run ./cmd/dev check --run <run-id> --race` on clean committed candidate + independent code review | Clean commits on `codex/phase1-telemetry-guardrails`, macOS arm64 + 3-OS PR CI | All 7 canonical steps pass across all 6 cross-OS `go list -test` targets; independent review closed; Draft PR CI green |

## Risks, rollback, and open decisions

- **Backward compatibility risk**: Existing `.agent/runs/` directories on developer machines use Schema v1. `Store.Read` explicitly accepts both v1 and v2 (while rejecting v2 fields on v1 manifests) and is tested against a frozen v1 fixture so no existing local run breaks.
- **Cross-OS `go list -test` risk**: Live probe on macOS with `CC=/nonexistent-cc` verified all 6 targets succeed without invoking a C compiler; real Ubuntu 24.04 and Windows 2025 execution is verified by PR CI before merge.
- **Policy/gate trust boundary**: Because this spec modifies `internal/runlog`, `internal/spec`, `cmd/dev`, `internal/architecture`, and workflow docs, independent technical review does not grant trusted-policy approval; `dev run override --confirm-human-override` is a local observability record, not cryptographic auth or merge approval; the Draft PR requires three-OS CI and explicit owner review/merge.
- **Rollback**: Revert the single squash commit; no product runtime code or persisted user data is affected.

## Review and evidence

Required independent review dimensions (per `docs/workflow.md#risk-selected-independent-review`):
- Correctness and regression discrimination (R4 state-machine edge cases, v1 read compatibility, Git `-z` / staged-only / rename scope matching, anti-vacuity of architecture `-test` guards);
- Architecture and contract compatibility (`internal/spec`, `internal/runlog`, `cmd/dev` boundaries);
- Gate/workflow integrity plus separate human policy approval.

Durable handoff location: `reports/0044-phase1-telemetry-guardrails.md` (written after independent code review referencing the exact reviewed code commit SHA, with PR CI URLs recorded in the Draft PR body).
