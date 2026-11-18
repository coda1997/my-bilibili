# Agent workflow integrity

Spec ID: `0025-agent-workflow-integrity`
Status: proposed for independent architecture review; local implementation
requested on branch `codex/agent-workflow-integrity`. Remote publication and
policy adoption remain separate owner decisions.

## User scenario and goal

A contributor or agent needs the repository's workflow instructions to route
reliably, fail closed when their executable references drift, and preserve
independent technical review without copying Aether-specific infrastructure.

## Scope and non-goals

Deliver a narrow hardening of the existing agent workflow:

- validate the repository's skill metadata, routing, and local references from
  the canonical development check;
- add a fast-lane fix procedure for reproduced defects;
- make independent-review scope explicit and risk-selected;
- stop `STATUS.md` from carrying a volatile base-commit claim.

Do not add autonomous merging, remote issue filing, policy adoption, release
automation, a generic agent framework, or Aether-specific telemetry, PR, and
deployment controls. No Go product behavior changes and no new product-runtime
dependency is added.

## Boundaries and shared contracts

The canonical `go run ./cmd/dev check` command is the deterministic workflow
gate. It validates tracked workflow files on disk without changing them or
requiring network access after module setup. The development-only validator
uses Goldmark for Markdown AST extraction and YAML v3 for skill frontmatter.
These dependencies are imported by `internal/check`, not by product packages.

The validator owns three repository-local contracts:

- every skill under `.agents/skills/<name>/` has `SKILL.md` frontmatter with a
  non-empty `description` and a unique `name` exactly equal to `<name>`;
- every relative Markdown link in checked workflow documents resolves from the
  linking document, with existing local fragments where supplied;
- every routed skill named by `AGENTS.md` exists and the routing table names
  each skill entrypoint exactly once.

The checked workflow inventory is discovered deterministically:

- `README.md`, `AGENTS.md`, `STATUS.md`, `.github/copilot-instructions.md`, and
  `.github/pull_request_template.md`;
- all immediate skill entrypoints at `.agents/skills/<name>/SKILL.md`;
- `docs/templates/*.md`;
- the canonical workflow documents `docs/agent-convergence.md`,
  `docs/architecture.md`, `docs/development.md`, `docs/pull-requests.md`,
  `docs/testing.md`, and `docs/workflow.md`.

The gate rejects symlinked roots, missing expected files, and relative link
targets whose evaluated path escapes the repository root. Link fragments must
match a heading in the target Markdown document. External URLs are not fetched
or validated. This inventory is intentionally narrower than product specs and
historical reports.

Workflow instructions and skills remain the source of procedure. The
validator does not choose agents, authorize remote actions, or replace
independent review. Skill links to external URLs are outside its scope.

## Acceptance criteria

### AWF-AC-01

Given an inventory document or skill with missing frontmatter, a broken local
link, a missing local fragment, a missing routed skill, an outside-root link, or
a symlink escape, when the workflow validator runs, then it returns a concrete
path-specific failure.

### AWF-AC-02

Given the repository's current valid workflow corpus, when the canonical
development check runs, then the workflow-integrity step executes and the
full check passes without mutating workflow files or requiring network access.

### AWF-AC-03

Given a reproduced, bounded defect, when an agent follows the task router, then
it can choose a dedicated `fix` skill that requires a failing regression,
smallest repair, canonical checks, and independent review before handoff.

### AWF-AC-04

Given an implementation candidate, when its review packet is prepared, then
review dimensions are selected from the candidate's actual correctness,
architecture, security/privacy, native lifecycle/resource, or evidence risks;
the workflow does not claim that author self-review or a fixed fan-out supplies
independent closure.

### AWF-AC-05

Given the current state file, when a reader opens `STATUS.md`, then it describes
durable milestones and next actions without claiming a volatile base commit
as an evergreen source identity.

## Verification matrix

| AC | Test/command or manual procedure | Fixture and OS | Required evidence |
| --- | --- | --- | --- |
| AWF-AC-01 | `go test -count=1 ./internal/check` with invalid metadata/link/routing/traversal/symlink fixtures and an invalid corpus passed through `Run` | Go temporary repository, all CI OSes | Named negative controls fail before fixed fixtures pass; later Go commands are not reached |
| AWF-AC-02 | `go run ./cmd/dev check --race` and the discovered inventory count | Local macOS; CI Linux/macOS/Windows matrix | A non-empty workflow step executes and the complete canonical check passes |
| AWF-AC-03 | `go test -count=1 ./internal/check` plus router/document inspection | Current repository, no network | `fix` skill resolves and is named by task routing |
| AWF-AC-04 | independent review of workflow diff and review-routing text | Exact candidate, policy-aware reviewer | Review packet records selected dimensions and independence |
| AWF-AC-05 | workflow integrity check and diff inspection | Current repository | No `Base: <branch> at <sha>` identity claim remains |

## Risks, rollback, and open decisions

The main risk is a false-confidence gate: a validator that never sees a damaged
workflow input could report success. AWF-AC-01 addresses this with negative
controls, including an invalid inventory passed through `Run` and a failing
fixture for the registered step. The Markdown parser is used for structural
extraction, not as a CommonMark conformance gate: the workflow checker accepts
ordinary Markdown prose and validates only repository-owned links, headings
used as local anchors, skill routes, and frontmatter.

Policy and CI changes require separate human approval before they are treated
as trusted automation. Independent review is still required for this change.
Rollback is the removal of the workflow step and its documentation changes;
because no product behavior or persisted data format changes, rollback does
not require migration.

## Review and evidence

Required review dimensions: workflow correctness, gate anti-vacuity, path and
link safety, permission boundaries, and candidate-specific evidence. A separate
agent/context must provide the architecture review and later code review; the
implementation author cannot claim independent closure.

The controller should record the source snapshot and check results. A proposed
spec grants no remote authority, policy approval, merge, or release decision.
