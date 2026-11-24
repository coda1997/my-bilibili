# Phase 0 workflow hygiene and gate parity

Spec ID: `0030-phase0-workflow-hygiene`
Status: proposed for independent architecture review; authorized by owner on 2026-11-24 in isolated worktree branch `codex/phase0-workflow-hygiene` from `master` (`a3e8fe0`). Policy/gate promotion still requires separate human approval on the PR.

## User scenario and goal

Contributors and agents working across Codex, Antigravity, and Claude Code need repository skill entrypoints, spec identifiers, agent read-order boundaries, and `STATUS.md` / branch-protection documentation to stay consistent and fail closed when workflow metadata drifts—without disturbing historical ADRs, retired probes, or uncommitted feature work in the primary checkout.

## Scope and non-goals

Deliver a narrow baseline-hardening slice before Phase 1 structured telemetry:

- Add the missing `.claude/skills/fix` symlink pointing to `../../.agents/skills/fix`.
- Extend `internal/check.Workflow` to validate `.claude/skills/` mirror parity against `.agents/skills/` across macOS, Linux, and Windows checkouts (handling Git `core.symlinks=false` plain-file pointers on Windows while rejecting non-symlink copies on Unix and rejecting wrong targets, missing skills, extra entries, or symlinked `.claude`/`.claude/skills` roots everywhere).
- Extend `internal/check.Specs` to reject duplicate leading numeric spec prefixes and duplicate cross-spec acceptance-criterion prefixes, while explicitly grandfathering existing historical pairs (`0004`, `0025`, and `M2`) and failing closed on any third spec or new collision.
- Update `AGENTS.md` to document `.claude/skills/` parity and instruct agents not to read `experiments/` by default unless assigned an experiment task (retaining `experiments/` so historical ADR and README links stay valid).
- Update `STATUS.md` to retire the obsolete Ubuntu `TestFullscreenCyclesRecheckBeforeNextEntry` next-action (already fixed in `c6a9152` and passing on `master` CI).
- Update `docs/pull-requests.md` to match the live, `gh api`-verified `master` branch-protection state. Prerequisite for documenting required status checks (`Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)`): the separately owner-authorized remote branch-protection update must be applied and confirmed via `gh api` readback before `HYG-002` edits `docs/pull-requests.md`. Fail closed: never claim a remote status-check gate that is not confirmed active by live `gh api` output.

Non-goals:
- No product runtime code changes (`internal/app`, `internal/catalog`, `internal/playback`, `internal/danmaku`, `internal/history`, `internal/desktop`, `cmd/my-bilibili`).
- No deletion or refactoring of `experiments/` or historical `specs/` / `reports/`.
- No Phase 1 `internal/runlog` schema v2 changes, DAG orchestrator, or CI workflow YAML edits.
- No modification of the user's primary working tree containing uncommitted `0028` / `0029` work.

## Boundaries and shared contracts

- `internal/check` owns deterministic read-only workflow and spec validation executed by `go run ./cmd/dev check`.
  - `.claude/skills` parity contract:
    - `.claude` and `.claude/skills` must exist as non-symlink directories inside the repository root.
    - Every canonical skill `<name>` discovered in `.agents/skills/<name>/SKILL.md` must have an entry `.claude/skills/<name>` targeting `../../.agents/skills/<name>`.
    - On non-Windows platforms, `.claude/skills/<name>` must be a symlink (`fs.ModeSymlink`) whose `Readlink` value equals `../../.agents/skills/<name>`.
    - On Windows, `.claude/skills/<name>` may be either a symlink whose backslash-normalized (`\` -> `/`) `Readlink` value equals `../../.agents/skills/<name>`, or a regular file (Git `core.symlinks=false` checkout) whose exact raw bytes equal `../../.agents/skills/<name>` (no trailing newline or extra bytes). Directories and mismatched targets are rejected on all platforms.
    - Unexpected entries inside `.claude/skills/` are rejected so stale skill links cannot accumulate; `.DS_Store` is ignored only when it is a regular file (a symlink or directory named `.DS_Store` is rejected).
  - Cross-spec uniqueness contract in `internal/check.Specs`:
    - Numeric prefix extraction: leading ASCII digits followed by `-` at the start of a spec directory name (e.g., `0030-phase0-workflow-hygiene` -> `0030`). Spec directories without a leading `<digits>-` prefix (such as unit-test fixture `specs/valid`) remain allowed.
    - Grandfathered historical numeric-prefix sets: `0004` (`0004-client-p0`, `0004-workflow-clarifications`) and `0025` (`0025-agent-workflow-integrity`, `0025-p0-async-source`). Exact set membership is enforced on every colliding spec (so any unauthorized pair such as `0004-client-p0` + `0004-new`, a third spec under `0004`/`0025`, or any other duplicate numeric prefix fails closed with deterministic sorted colliding directory names).
    - Acceptance-criterion prefix extraction: each distinct `<PREFIX>` from `### <PREFIX>-AC-<digits>` covered by `snapshot.Definition.Tasks` (guaranteed complete by `spec.Validate`).
    - Grandfathered historical AC-prefix set: `M2` (`0015-m2-fixture-preview`, `0016-m2-policy-continuation`). Exact set membership is enforced on every colliding spec (so any unauthorized pair, a third spec using `M2`, or any other cross-spec AC prefix collision fails closed with deterministic sorted colliding spec IDs).
- Concurrency, cancellation, and external I/O: `internal/check` performs bounded synchronous filesystem reads on local repository files only; no network calls, background goroutines, or file mutations.
- `cmd/dev/main_test.go` test repository fixture must include `.claude/skills/sample` matching `.agents/skills/sample` so `TestRecordedCheckBrokenPipe` exercises the updated `Workflow` contract honestly.

## Acceptance criteria

### HYG-AC-01

Given the repository contains `.agents/skills/<name>/SKILL.md` entries, when `internal/check.Workflow` runs, then `.claude/skills/fix` resolves to `../../.agents/skills/fix`, all canonical skills have matching `.claude/skills/<name>` entries, and any missing, stale, mistargeted, symlink-root, or (on Unix) regular-file copy entry under `.claude/skills/` fails closed with a concrete path diagnostic.

### HYG-AC-02

Given a repository `specs/` directory, when `internal/check.Specs` runs, then the existing historical repository passes while any newly introduced duplicate numeric spec prefix (including a third `0004` or `0025` spec) or duplicate cross-spec AC prefix (including a third `M2` spec) fails closed with a concrete diagnostic identifying the colliding specs before later Go checks run.

### HYG-AC-03

Given a contributor or agent reads `AGENTS.md`, `STATUS.md`, and `docs/pull-requests.md`, then `AGENTS.md` states `.claude/skills/` mirror parity and default exclusion of `experiments/`, `STATUS.md` no longer lists the already-resolved Ubuntu fullscreen timeout as an open next action and accurately reflects `master` CI, and `docs/pull-requests.md` matches the live `gh api`-confirmed `master` branch-protection state (documenting required status checks `Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)` only after confirming them active via readback, and never weakening explicit human merge approval).

### HYG-AC-04

Given the complete Phase 0 change set on `codex/phase0-workflow-hygiene`, when targeted negative-control tests and `go run ./cmd/dev check --race` run on a clean committed candidate, then all negative controls fail on damaged fixtures, all canonical checks pass without mutating repository files, and `reports/0043-phase0-workflow-hygiene.md` records non-circular evidence (binding code checks and independent review to the exact reviewed commit SHA; PR CI run URLs belong in the Draft PR body rather than circularly inside the commit they test).

## Verification matrix

| AC | Test/command or manual procedure | Fixture and OS | Required evidence |
| --- | --- | --- | --- |
| HYG-AC-01 | `go test -count=1 -race ./internal/check ./cmd/dev` covering missing/stale/mistargeted/non-symlink/Windows-pointer `.claude/skills` fixtures | Temporary Go repositories, local macOS + CI Linux/macOS/Windows | Negative controls reject damaged `.claude/skills` inputs; positive fixtures and real repo pass |
| HYG-AC-02 | `go test -count=1 -race ./internal/check` covering duplicate numeric prefix, unauthorized `0004`/`0025` pair and third spec, duplicate AC prefix, unauthorized `M2` pair and third spec, and `Run` fail-fast ordering | Temporary Go repositories, local macOS + CI Linux/macOS/Windows | Collisions fail before `go vet`/`go test`; historical grandfathered pairs pass |
| HYG-AC-03 | `go run ./cmd/dev check --race` + diff inspection + `gh api` branch-protection readback | Worktree candidate + GitHub `master` settings | Valid Markdown links/anchors, accurate `STATUS.md` and `docs/pull-requests.md`, verified `gh api` output |
| HYG-AC-04 | `go run ./cmd/dev check --run <run-id> --race` on committed candidate + independent code review | Clean commit on `codex/phase0-workflow-hygiene`, macOS arm64 + PR CI | Recorded run log, non-circular handoff report, independent review closure, Draft PR CI URLs in PR body |

## Risks, rollback, and open decisions

- **Remote branch-protection sequencing prerequisite**: `HYG-002` must verify the live `gh api repos/coda1997/my-bilibili/branches/master/protection` state before editing `docs/pull-requests.md`. If the separately owner-authorized branch-protection update has not been applied or fails, `docs/pull-requests.md` must fail closed and keep reporting no required status checks rather than asserting unenabled gates.
- **Windows symlink portability risk**: Git on Windows CI commonly checks out symlinks as plain-text pointer files (`core.symlinks=false`). The contract explicitly tests both symlink and Windows text-pointer validation helpers (using explicit `\` -> `/` normalization in Windows mode so Unix tests exercise backslash normalization) so Windows CI neither false-fails nor weakens Unix symlink checks.
- **Policy/gate trust boundary**: Because this spec modifies `internal/check/`, independent technical review does not grant trusted-policy approval; the PR remains a Draft until CI passes and the owner explicitly reviews and merges it.
- **Rollback**: Revert the single squash commit; no product data or persisted schema is affected.

## Review and evidence

Required independent review dimensions (per `docs/workflow.md#risk-selected-independent-review`):
- Correctness and regression discrimination (anti-vacuity of new gates and cross-platform symlink handling);
- Gate/workflow integrity plus separate human policy approval.

Durable handoff location: `reports/0043-phase0-workflow-hygiene.md`. Local run evidence stays under `.agent/runs/` inside the worktree until summarized in the report and Draft PR.
