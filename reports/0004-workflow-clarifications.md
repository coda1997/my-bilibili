# Workflow clarification proposal handoff

- Date: 2026-11-10 (Asia/Shanghai).
- Run: `20261110T145216Z-19a83178d97e` in the new worktree.
- Branch: `codex/workflow-clarifications`.
- Base: `3a3405206cf9fdb6e9f084afc071fe8e6a1b2547`; changes are uncommitted.
- Contract: `specs/0004-workflow-clarifications`, task WFC-001.
- Authority: the user requested these local documentation edits in a new
  worktree; no delegation or remote action was authorized.
- Candidate: all Git-visible source files, including this report, are preserved
  in the run's `evidence/candidate.tar` and `evidence/candidate.json` hash/mode
  manifest. The base SHA alone does not identify this dirty candidate.
- Outcome: local draft and author checks complete; independent contract/code
  review and separate human policy adoption remain pending (`needs-human`).

## Delivered

- Canonical proposed read-only route in docs/workflow.md: no implementation
  packet or new run required for consultation; edits return to the scoped
  workflow. Formal review inputs and authority boundaries remain intact.
- Proposed reviewer succession: explicit human designation, independent new
  verification, attributed closure, original finding/report preservation,
  evidence gaps left pending, and no reviewer shopping or repair-count reset.
- Matching AGENTS/development/PR routing, assignment/review packet fields,
  and independent-review skill reference. No new agent role or runtime tool.

## Verification actually performed

On macOS arm64 with the existing Go 1.27.3 toolchain; paths below are relative
to the run's evidence directory. No tool or dependency was installed.

| Check | Result and limits | Evidence |
| --- | --- | --- |
| `git diff --check` | Pass | `documentation-validation.log` |
| `go run ./cmd/dev check --run 20261110T145216Z-19a83178d97e --race` | Pass; specs, formatting, tidy diff, vet, uncached race tests, build | Recorded `check-*.log` |
| Changed skill via installed skill-creator `quick_validate.py` | Pass with existing Python 3.14.6 / PyYAML 6.0.3 environment; default Python lacks yaml | `skill-validation.log` |
| Local Markdown file targets and changed workflow anchors | Pass; no external-link validation | `documentation-validation.log` |
| Contract templates in isolated scratch through real `check` and `run new` | Pass; syntactic validation, not prose acceptance | `template-check.log`, `template-run-new.log` |
| Seven documentation scenarios and populated assignment/review templates | Author walkthrough supports WFC-AC-01/02/03; not an agent behavior test or independent review | `scenario-walkthrough.log`, `synthetic-assignment.md`, `synthetic-review.md` |

The synthetic successor designation and disposition in fixtures are explicitly
fictional; they grant no authority and close no actual finding.

## Remaining gates and next action

The new policy sections are proposals, not self-adopted trusted policy. A
separately authorized independent context should review this exact candidate,
then the human should decide adoption before relying on changed policy. No
independent approval is claimed. Commit/push/PR/merge require separate authority;
no new remote CI or Windows/Linux native test is claimed.

M0 remains accepted under its existing record; M1 is not started. The README
milestone handoff stays unchanged because this is an unaccepted local proposal.
The original checkout, its untracked Copilot instructions, historical reports,
and old frozen runs were not edited by this task. Local evidence is ignored,
not backed up by Git, and not a remote attachment; preserve it before removing
this worktree. Resume with this spec/report and the run's status.
