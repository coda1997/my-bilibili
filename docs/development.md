# Starting a scoped development task

This guide makes the [workflow](workflow.md) usable by a human-operated lead.
It does not implement a scheduler, grant permissions, or replace its gates.

## Start from a fresh context

For a read-only explanation, audit, or investigation, use the
[proposed lightweight route](workflow.md#read-only-requests-proposed-clarification)
instead of requiring an implementation assignment or creating a run. It does
not bypass formal review inputs or authorize edits. The steps below apply to
scoped changes; a request to edit leaves the read-only route.

1. Read [README](../README.md) for the current handoff. Check the milestone in
   [stage zero](stage-zero.md); do not start a full client slice before the M1
   UI/playback decision. A proposed spec is not authorization to implement it.
2. Follow [testing prerequisites](testing.md#prerequisites). Record the checkout,
   branch, base commit, and dirty state. Preserve unrelated work and ask about
   unexpected changes rather than resetting it.
3. Identify the selected spec, tasks, role, write paths, and explicit authority.
   If boundaries or a blocking decision are missing, return a planning question
   or proposed contract; do not invent permission to implement. Native sub-agent
   dispatch within the requested task uses the owner's standing authorization.
4. Inspect the controller's local run if available. A clone/worktree need not
   contain `.agent/`; recover conclusions from the handoff and request missing
   evidence when necessary. Do not copy private account data between worktrees.

## Write a feature contract

Copy [spec.md](templates/spec.md) and [tasks.json](templates/tasks.json) into a
new direct child `specs/<id>/`. Replace every placeholder, keep `spec_id` equal
to the directory name, and choose a unique acceptance prefix. Templates stay
under `docs/templates/`, never an incomplete directory under `specs/`.

Specify observable behavior before implementation details: user scenario,
goals/non-goals, boundaries and shared contracts, errors/cancellation/lifetime,
acceptance criteria, test/OS evidence, risks, and unresolved decisions. Each AC
needs a verification method; marking an AC as covered by a task is not a test.

The strict task schema is defined by `internal/spec`, not a second schema here.
Keep role metadata in an assignment rather than adding unknown JSON fields.
Task `checks` describe verification expectations; they are not shell programs
the recorder will execute. Run `go run ./cmd/dev check` to validate all active
specs along with the Go checks. It does not evaluate the truth of the prose ACs.

Split only at real independently testable boundaries. Put implementation and
its tests under the same ownership where appropriate. Order shared-file or
directory-subtree writes in the DAG. Freeze the shared interface and async
semantics before parallel workers start. Do not invent interfaces merely to
increase the number of agents.

## Assign roles with bounded context

Use [assignment.md](templates/assignment.md). The packet identifies the role,
task IDs, source/spec identity, inputs, checks, writable paths, return format,
and the user's authorization. Include relevant code/tests and decisions, not
the entire author's chat transcript.

| Role | Required input | Return to controller |
| --- | --- | --- |
| Lead/planner | User scenario, current milestone, architecture, constraints | Proposed spec/task DAG, unresolved decisions, reviewer packet |
| Architecture reviewer | Spec/tasks, affected interfaces, risks, relevant baseline | Findings and proceed/request-changes/needs-human recommendation |
| Implementer | Reviewed frozen contract, satisfied dependencies, assigned paths | Changes, AC/test evidence, limitations and next action |
| Code reviewer | Exact candidate/diff, frozen contract, relevant tests and evidence | Structured findings and independent disposition |
| Controller/handoff | Worker outputs, candidate checks, independent reviews | Integration identity, evidence records, draft PR and durable handoff |

A reviewer must be a separate authorized agent/context, not the implementation
author wearing a different role. For finding closure, provide the original
finding/reproduction and the changed candidate. If the original reviewer cannot
continue, follow the [proposed succession procedure](workflow.md#reviewer-succession-proposed-clarification),
including explicit human designation; do not silently substitute a reviewer.
Select review dimensions from actual risk (correctness, architecture, security,
native platform/UI), not an automatic fixed number of agents. Prepare the packet
and dispatch native sub-agents under the owner's standing authorization; no
per-dispatch permission is needed. Do not fabricate an independent result.

The controller may save a sanitized assignment under its run's `evidence/`
and attach it with `run note` before dispatch. Published packets are immutable;
record a superseding packet for a changed assignment. A spec, shared-contract,
or task-scope change still requires architecture review and a new frozen run.
Workers return evidence from their own scratch/worktree; they do not append to
the controller's run or change review/policy records.

## Execute and finish

1. The controller freezes the selected spec with `run new`. Dirty development
   baselines need explicit `--allow-dirty`; this is not a substitute for source
   identity. See [recorded checks](testing.md#recorded-checks).
2. Implement only the assigned slice. Add a regression for a reproduced bug,
   execute targeted checks, then the full check. Record failures honestly.
3. Integrate and identify a candidate. Prefer an authorized clean commit; if
   committing is not authorized, preserve a manifest and archive of all relevant
   tracked/untracked candidate files, their hashes/modes, and the base SHA. Do
   not identify modified code solely by the unchanged HEAD commit. Keep a diff
   and enumerate any excluded local files. The recorder does not do this for you.
4. Dispatch authorized independent review of that exact candidate. Apply the
   [review/repair policy](workflow.md#review-and-repair-policy), including the
   three-round limit and human escalation. Changes invalidate affected checks
   and reviews; identify and re-review the new candidate.
5. Prepare a [PR/handoff](pull-requests.md). Publish no remote change without
   the requested authority. Only the independent reviewer closes their findings;
   a `succeeded` run or green test suite cannot approve itself.

## Repo-local skills

The entrypoints are `feature-plan`, `feature-implement`, `independent-review`,
and `pr-handoff` under `.agents/skills/`. They route to this guide and the
canonical protocol. Personal skills/configuration are optional conveniences,
not prerequisites or copied repository secrets. No model/provider or permission
profile is forced on contributors by this repository.

Codex configuration references checked on 2026-11-10:
[AGENTS.md](https://learn.chatgpt.com/docs/agent-configuration/agents-md) and
[Build skills](https://learn.chatgpt.com/docs/build-skills). Repo-local skills use
`SKILL.md` name/description metadata; none of these skills installs a plugin,
grants remote permissions, or automatically creates an independent reviewer.
