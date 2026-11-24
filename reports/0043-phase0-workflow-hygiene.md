# Handoff: 0030-phase0-workflow-hygiene

Follow [agent convergence](../docs/agent-convergence.md) R5/R6/R7/R9 for triage, rulings, evidence and completion/context-reset re-orientation.

- Date: 2026-11-24
- Run ID: `20261124T151032Z-51528510fddb` (local to worktree `.agent/worktrees/phase0-workflow-hygiene`)
- Lane: full
- Reviewed code & docs candidate commit SHA: `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` (on frozen spec commit `d6c28eaf6785feb48257dd863f181c821ebf866c`, base `master` `a3e8fe00d958e7025662e2570887eb7c1baa49d9`)
- CI run URL for PR head SHA: pending Draft PR push (recorded in the Draft PR body; not circularly embedded here)
- Execution outcome: `needs-human` (local checks and independent technical reviews passed; three-OS PR CI and separate human policy approval on the Draft PR remain pending)
- Independent review status:
  - Architecture review (`00c1835e-65e1-4e24-935d-ed839e6a01f1`, independent read-only context): Round 1 raised `ARCH-001` (`BLOCKING [class 7]`) + `ARCH-002`–`005` (`ADVISORY`); Round 2 closed `ARCH-001` on `spec.md` SHA-256 `147b6b0d4cd179494b611aaa264acf1d95f019302e02f9c93164e4b39eab105d` and issued `proceed`.
  - Code review (`8c4df163-6aea-4d69-9a67-ad371b0ca850`, independent read-only context): Round 1 independently ran targeted and full `--race` checks on clean commit `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` and issued `approve` (0 `BLOCKING`, 2 low-severity `ADVISORY`: `HYG-ADV-001`, `HYG-ADV-002`).
- Findings and separate human policy-approval status: 0 open `BLOCKING` findings; 2 answered `ADVISORY` findings closed locally under R2 (see Found, not done). Separate human policy approval for `internal/check/`, `AGENTS.md`, and `docs/pull-requests.md` is **pending**.
- Policy version; cumulative review/fix/failure counts and issue budget: R1–R9 (2026-11-16); architecture review rounds: 2/3; code review rounds: 1/3; implementation fix iterations: 0/3; repeated-failure count: 0/3; new remote issues: 0/5.

## Original goal and done criteria

- Original goal and locked scope: Phase 0 baseline workflow hygiene ([spec 0030](../specs/0030-phase0-workflow-hygiene/spec.md), [tasks.json](../specs/0030-phase0-workflow-hygiene/tasks.json)) executed in an isolated worktree without modifying the primary checkout's uncommitted `0028`/`0029` work.

| Done criterion | Candidate-specific evidence | Result or remaining gap |
| --- | --- | --- |
| `HYG-AC-01` (`.claude/skills` parity + `fix` mirror) | `.claude/skills/fix` -> `../../.agents/skills/fix`; `validateClaudeSkillMirrors` + `TestWorkflowClaudeSkillMirrors` + `cmd/dev/main_test.go` at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` | Local macOS PASS; `ubuntu-24.04` and `windows-2025` PR CI pending |
| `HYG-AC-02` (cross-spec numeric & AC prefix uniqueness) | `Specs()` + `validateUniqueSpecGroups` + `TestSpecsCrossSpecUniqueness` at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` | Local macOS PASS; three-OS PR CI pending |
| `HYG-AC-03` (`AGENTS.md`, `STATUS.md`, `docs/pull-requests.md` accuracy) | Diff verified at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` against live `gh api` `master` protection readback (`strict: true`, 3 checks) and green `master` CI runs `44427348642` / `44429566237` | PASS |
| `HYG-AC-04` (canonical checks, non-circular report, independent review) | Recorded `go run ./cmd/dev check --run 20261124T151032Z-51528510fddb --race` and independent code review at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f`; final `--race` check on report commit | Local PASS; Draft PR three-OS CI and separate human policy approval pending |

## Delivered

- Added `.claude/skills/fix` symlink targeting `../../.agents/skills/fix`.
- Added `.claude/skills/` mirror parity validation in `internal/check/workflow.go` (`validateClaudeSkillMirrors`) enforcing strict symlinks on Unix, symlink (`\` -> `/`) or exact Git plain-file pointer on Windows, regular-file-only `.DS_Store` exemption, and rejection of missing, stale, mistargeted, directory-copy, or symlink-root entries.
- Added cross-spec numeric-prefix and AC-prefix uniqueness validation in `internal/check/check.go` (`Specs`, `specNumericPrefix`, `validateUniqueSpecGroups`) enforcing exact membership on grandfathered historical sets (`0004`, `0025`, `M2`) with deterministic sorted diagnostics.
- Added negative/positive unit coverage in `internal/check/check_test.go` and updated `cmd/dev/main_test.go` fixture to create `.claude/skills/sample`.
- Updated `AGENTS.md` (`.claude/skills/` parity and default exclusion of `experiments/`), `STATUS.md` (retired resolved Ubuntu fullscreen timeout and cited passing `master` CI runs `44427348642` / `44429566237`), and `docs/pull-requests.md` (recorded live `gh api`-verified `master` required status checks while keeping explicit human merge review).
- Separately updated GitHub `master` branch protection under explicit owner authorization (`strict: true`, required contexts `Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)`).

- Observed vs inferred:
  - Observed: `c6a9152` stabilized `cmd/preview-smoke/fullscreen_test.go` via `testing/synctest`; `master` CI runs `44427348642` and `44429566237` succeeded; `gh api` readback confirmed `master` branch protection requires all three CI contexts with `strict: true`, `enforce_admins: true`, `required_linear_history: true`, and `required_approving_review_count: 0`; local macOS Go 1.27.3 race checks and independent code review passed on `06cf427c117f02b96e9bc51f4a5afacb4033dc1f`.
  - Inferred / not yet executed: Real `ubuntu-24.04` and `windows-2025` runner checkouts (including Windows `core.symlinks=false` checkout of `.claude/skills/*`) are inferred from unit tests (`isWindows=true`) until PR CI completes on the Draft PR head SHA.
- Contract relied on: [spec 0030](../specs/0030-phase0-workflow-hygiene/spec.md) (`147b6b0d...`), [tasks.json](../specs/0030-phase0-workflow-hygiene/tasks.json) (`b24b15d2...`), [workflow.md](../docs/workflow.md), and [agent-convergence.md](../docs/agent-convergence.md) R1–R9.

## Verification

| Claim | Evidence | Result | Scope and not tested |
| --- | --- | --- | --- |
| Targeted check & dev race tests pass | `go test -count=1 -race ./internal/check ./cmd/dev` at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` | PASS | macOS arm64 Go 1.27.3; not Linux/Windows |
| Complete canonical recorded check passes | `go run ./cmd/dev check --run 20261124T151032Z-51528510fddb --race` at `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` (`dirty: false`) | PASS | macOS arm64 Go 1.27.3; not independent approval or PR CI |
| Independent architecture review | Subagent `00c1835e-65e1-4e24-935d-ed839e6a01f1`, Round 2/3 on `spec.md` `147b6b0d...` | PROCEED | Read-only static review + live `gh api` check; no policy approval |
| Independent code review | Subagent `8c4df163-6aea-4d69-9a67-ad371b0ca850`, Round 1/3 on `06cf427c117f02b96e9bc51f4a5afacb4033dc1f` (reran targeted & full `--race` checks read-only) | APPROVE | 0 BLOCKING, 2 ADVISORY; macOS only; no policy approval |
| Live `master` branch protection | `gh api repos/coda1997/my-bilibili/branches/master/protection` readback on 2026-11-24 | PASS | `strict: true`, `Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)`, `enforce_admins: true`, `required_approving_review_count: 0` |
| Three-OS PR CI | Pending Draft PR creation | NOT-TESTED | Must pass on PR head SHA before merge |

## Not completed

- Three-OS GitHub Actions PR CI (`ubuntu-24.04`, `macos-15`, `windows-2025`) on the final PR head commit.
- Separate human policy approval and merge decision on the Draft PR (required because `internal/check/`, `AGENTS.md`, and `docs/pull-requests.md` changed).

## Found, not done

- **`HYG-ADV-001` (ADVISORY, Low, closed under R2 — not filed per R5 cosmetic threshold):** In `STATUS.md:7`, `[PR #17]` links to commit `c6a9152` rather than `/pull/17`. Retained in this local follow-up list without reopening code review; can be polished on the next `STATUS.md` update.
- **`HYG-ADV-002` (ADVISORY, Low, closed under R2 — not filed per R5 threshold):** `internal/check/check_test.go` could additionally test a multi-AC single-spec fixture for `seenACPrefixes` and a symlinked `.claude` parent directory (both paths are already exercised by real-repo `dev check` and `paths.resolve(..., false)`). Deferred to Phase 1 `internal/check` work.
- Single advisory follow-up destination: this local follow-up list (`0/5` remote issues filed). No systemic follow-up warranted.

## Next action

1. Push `codex/phase0-workflow-hygiene` and open a Draft PR against `master`, attaching this report's evidence table and verifying three-OS CI (`Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)`).
2. Owner reviews and explicitly grants separate human policy approval and merge decision on the Draft PR.
3. Local run evidence under `.agent/worktrees/phase0-workflow-hygiene/.agent/runs/20261124T151032Z-51528510fddb` is local-only and not backed up by Git.
