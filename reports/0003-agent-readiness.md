# Agent readiness handoff

- Date: 2026-11-10.
- Run: `20261110T130200Z-03d009b52421`.
- Branch: `codex/agent-readiness`.
- Base: `04213b6d4da9fdb44ef37618f47656e02a33d891`; working tree is uncommitted.
- Contract: `specs/0003-agent-readiness`; frozen spec/tasks are in the run.
- Status: bounded local implementation, checks, independent technical review,
  and fresh-context workflow exercise complete. No further repair is requested.
- Closing outcome: `needs-human`, for policy adoption and changed-candidate CI;
  this does not mean the local repair loop is still running. M0 is not accepted.

## Delivered

- Fresh-context navigation in README/AGENTS and development, testing, and
  PR/handoff guides; local state and durable handoffs have distinct roles.
- Four repo-local skills for planning, scoped implementation, independent
  review, and PR/handoff preparation. They do not grant extra permissions.
- Spec/task, role-assignment, review, handoff, and PR templates.
- COR-001 independently closed: evidence is written before display; returned
  display errors and native broken pipes preserve the actual result and
  referenced log. Persistence/finalization failures remain blocking. The
  final correction is entrypoint signal handling plus a subprocess regression,
  not new agent infrastructure.
- COR-002 independently closed technically: only task write scopes require
  ASCII. Generic Unicode paths and old closed records retain support. ADR 0003
  remains proposed until separate human policy review.

No CI redesign, project dependency, agent orchestrator, remote mutation,
commit, push, application feature, packaging, or release was added.

## Exact reviewed source

Implementation candidate B is the 43-file manifest SHA-256
`9cb1ef6cee27847a70d49e0ce91d580c0b1c040c2a3faffbb86cbe5b7f847775`,
stored with its archive/diff as `evidence/review-candidate-repair.*`.
It differs from initial candidate A only in `cmd/dev/main.go` and its test.
The original correctness reviewer independently approved B and closed both
technical findings; the initial reports still describe A and remain unchanged.

Only this handoff and `reports/0003-independent-review.md` are later publication
outputs. Their exact delta from B is identified in
`evidence/handoff-publication.json`; source, contracts, skills, and guides do
not change. The final publication verdict is in
`evidence/final-architecture/report.md`. See the
[independent record](0003-independent-review.md) for attribution and limits.

## Verification actually performed

Evidence paths below are relative to this run's `evidence/`. Execution used
Go 1.27.3 on macOS arm64 unless explicitly marked compilation-only.

| Check actually performed | Result | Evidence |
| --- | --- | --- |
| New COR regressions against unchanged production code | Expected failures, both defects reproduced | `regression-red.log` |
| Native broken-pipe regression before/after the small follow-up | Expected failure before; pass after for real passing/failing checks | `native-regression-red.log`, `native-repair-regressions.log` |
| Full `go run ./cmd/dev check --run <id> --race` on B | Pass: specs, formatting, tidy diff, vet, uncached race tests, build | `check-4059813728.log` |
| Original reviewer: full race check, affected/original regressions, six real early/late/healthy pipe scenarios | Pass; COR-001 closed, COR-002 technical closure preserved | `correctness-repair/report.md`, `correctness-repair/native-pipe.log` |
| Original reviewer: macOS Unicode alias and old-to-current Unicode record compatibility | Pass; old record stays unchanged | `initial-correctness/legacy-unicode.log`, `correctness-repair/original-regressions.log` |
| Four skill metadata checks, 56 local Markdown references, populated templates through real `check` and `run new` | Author and independent architecture checks pass | `initial-architecture/report.md` and indexed logs |
| Fresh-context planning/handoff exercise using repo-local skills and templates | Pass for this bounded scenario; generated spec/DAG, reviewer packet, handoff and PR draft in scratch only | `forward-workflow/exercise-report.md`, `forward-workflow/delivery-manifest.json` |
| Production and CLI-test compilation: darwin arm64/amd64, windows amd64/arm64, linux amd64 | Pass; not native runtime or remote CI evidence | `native-repair-cross-compile.log` |
| Three old closed runs, 83 historical run files, original reports and frozen inputs | Readable/unchanged; no migration | `legacy-file-manifest.json`, `initial-architecture/history-after.log` |

The fresh-context exercise tested planning and handoff, not autonomous execution
or every skill mode. Its proposed `run list --spec` feature was not implemented.
A minor `proceed`/`approve` template wording difference was nonblocking and is
deferred; it does not justify another implementation round or more process.

## Remaining gates and next action

The bounded readiness task is ready for human review, not unattended merge or
release. There are no outstanding technical repairs requested by its reviewers.

- Human owner: review the changed check/task-scope policy and proposed ADR 0003.
- Then separately authorize commit/push of the reviewed tree and its three-OS
  CI. ARC-001 remains open for that candidate-specific acceptance evidence,
  not for another local architecture redesign.
- Saved baseline Actions run `42818199313` passed for `04213b6`, not B. Windows
  and Linux runtime tests for B are not run. The unchanged workflow records the
  foundation spec, not feature-specific ACs. Evidence artifacts are not packages
  and expire after 14 days; saved baseline expiry is 2026-11-24 UTC.
- Decide M0 acceptance only after the remaining evidence and human decisions.
  M1/mygo remains a separately authorized task and has not started. M2/M3,
  unattended scheduling, deployment, signing, and releases are not implemented.

Resume from this report, `reports/0003-independent-review.md`, and
`go run ./cmd/dev run status --id 20261110T130200Z-03d009b52421 --events`.
Local `.agent/` evidence is ignored and not backed up by Git. These reports are
Git-visible but still uncommitted. Preserve needed evidence before removing the
checkout; a fresh clone will not contain local runs or the optional toolchain.
