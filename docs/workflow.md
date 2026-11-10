# Development and evidence protocol

## Roles and authority

The intended workflow is lead planning, independent architecture review,
contract freeze, isolated implementation, candidate integration, deterministic
checks, independent risk-based reviews, bounded repair, and controlled merge.
It is a protocol first. Only the local evidence recorder and checks exist now.

The owner adopted the five task-scope/check/review policies on 2026-11-10.
See the [adoption record](../reports/0003-policy-adoption.md) for the exact
scope and the separate owner acceptance of M0 on the same date. Neither
decision grants autonomous or remote-action authority.

Operational entrypoints: [development](development.md), [testing](testing.md),
[PR/handoff](pull-requests.md), and the [role packet](templates/assignment.md).
These guides and repo-local skills do not grant delegation or remote authority.

The controller is the sole writer to its run directory. Workers operate in
separate authorized worktrees and return changes and evidence; they do not
edit the controller's inputs, review records, or policy. Worktrees are not
security sandboxes. Initial role separation is procedural, not access control.

Reviewer independence means a fresh review context with the spec, exact
candidate diff, relevant code, and test evidence. Authors cannot approve their
own work. The initial findings are preserved in `reports/0001-independent-review.md`;
follow the current handoff from README for later dispositions. Review completion
is not acceptance.

## Read-only requests (proposed clarification)

This clarification and reviewer succession below are local proposals pending
independent review and separate human policy adoption. They do not amend the
owner's previously adopted policies by themselves.

Repository explanations, configuration audits, and other read-only investigations
do not require an implementation spec, assignment packet, or new recorded run.
Read README and the relevant guidance, inspect source identity and available
evidence, and report findings and limits. Follow testing prerequisites if
verification commands are needed. Do not treat inspection or author self-checks
as independent approval; formal acceptance reviews still need the exact candidate
and the review inputs described in this protocol.

Read-only scope does not authorize candidate edits, commits, remote mutations,
delegation, or starting another milestone. Ordinary tool caches or isolated
scratch for requested verification are not candidate edits. If the request
extends to a repository change, switch to the scoped change workflow and
establish its authority and write paths before editing. Missing evidence is a
reported limitation, not permission to invent a contract or acceptance result.

## Contract format

Each direct child of `specs/` contains `spec.md` and `tasks.json`.

- Acceptance IDs are Markdown headings such as `### FND-AC-01`.
- Task JSON has `schema_version`, `spec_id`, and `tasks`.
- Each task has `id`, `title`, `depends_on`, `write_paths`, `acceptance`, and
  `checks`. All paths are repository-relative forward-slash paths, without
  globs, backslashes, parent traversal, or absolute paths.
- Declared `write_paths` additionally use ASCII spellings. This candidate's
  COR-002 repair is documented in [ADR 0003](adr/0003-ascii-task-scopes.md),
  adopted by the owner on 2026-11-10. Generic safe spec and
  evidence paths retain Unicode support. Invalid scopes are rejected even when
  tasks are dependency-ordered; this does not enforce actual filesystem access.
- A write path names a file or a complete directory subtree. Tasks with
  overlapping scopes must have a dependency ordering, including transitively.
- Every task references existing acceptance IDs and every acceptance ID must
  be covered by at least one task. Dependencies must form an acyclic graph.
- `checks` are declared verification expectations, not executable authority.
  The recorder never runs command strings supplied by a spec or issue.
- `internal/spec` is the current executable format definition. Add a schema
  export only when a consumer needs it; avoid two independently maintained
  definitions in the foundation.

Validation checks the declared graph, not whether an agent actually stayed
inside its write scope. Diff enforcement belongs to the later controller.

The protected shared contract must be established before parallel workers
begin. A contract change goes back to architecture review and creates a new
run with new frozen inputs; it is not an edit to the old run snapshot.

## Local record layout

```text
.agent/runs/<run-id>/
  manifest.json       # Schema, spec identity, input hashes, source provenance
  inputs/spec.md      # Frozen specification
  inputs/tasks.json   # Frozen task graph
  events/000001.json  # Started event; later events use increasing numbers
  evidence/           # Logs and other explicitly selected, sanitized evidence
```

All run timestamps are UTC. A run starts `running`. Notes do not change that
state. `finish` closes it with `succeeded`, `failed`, or `needs-human`; a closed
run cannot be appended to. A successful outcome requires recorded evidence,
but is still not an approval. Continue an open run after interruption, or
start a new run and mention the prior ID in its first note after a handoff.

`run list` discovers runs; `run status` validates frozen inputs, event sequence,
and referenced evidence before printing the derived state. Pending temporary
event files are not published history. Malformed committed events, missing
sequence numbers, symlinks, and evidence digest mismatches fail closed.

Before a long operation, checkpoint the task, current step, and next action.
Afterward record the result and evidence. `check --run <id>` automatically
creates and attaches its check log, including a failure result. On a process
crash, any unfinished operation must be verified or rerun; an earlier note
does not prove that operation completed.

Attach manually created evidence only after sanitizing it:

```sh
go run ./cmd/dev run note --id <run-id> --phase validation --message "Describe observed result" --evidence evidence/example.txt
```

The file must already exist inside that run's evidence directory. Evidence is
immutable once referenced. Do not use this command to attach credentials,
private recordings, full authenticated URLs, or arbitrary home-directory files.
The tool restricts paths but does not detect or redact secrets automatically.

## Recovery

`run status` works while a writer lock is present. If writing reports a lock,
inspect `.agent/runs/<id>/.write-lock/owner.json`, confirm the owning process
has stopped, then manually remove only that stale lock directory. Do not edit
published events to make validation pass. Preserve a damaged run for diagnosis
and start a new one if recovery cannot establish integrity.

The control checkout owns these paths. Worktree-specific copies are not a
shared database. Multi-host execution, authenticated writers, idempotent
distributed dispatch, and automatic stale-lock recovery are out of scope.

## Review and repair policy

Findings record a stable ID, severity, candidate commit, spec version, exact
location, observed/expected behavior, reproduction, and required evidence.
Use `docs/templates/review.md`. Independent reviewers close their findings.

Block on P0/P1 findings and unmet acceptance criteria. Record ownership and
disposition for lower-severity findings. Re-run affected checks and review on
changed candidate commits. Start with at most three repair rounds; recurring
failure, ambiguous requirements, or reviewer disagreement goes to a human.

Those rules are not yet machine-enforced. A future controller must load gate
policy from a trusted baseline, not the candidate's editable copy. Policy
changes must not approve themselves.

### Reviewer succession (proposed clarification)

If the original reviewer/context cannot continue, the human must explicitly
designate a successor independent of the implementation before reassignment.
The controller records the designation and reason in a new assignment, with
the original reviewer/finding IDs, report locator, reproduction, required
evidence, exact new candidate, frozen spec/tasks, and prior repair rounds.
Reassignment alone does not close a finding or authorize delegation.

The successor independently verifies the finding and fix against that candidate,
runs the required checks where available, and issues a new attributed disposition
referencing the original finding ID and new evidence. Preserve the original
report, reviewer attribution, published packets, and closed run records; record
succession and closure in a new report/event, never as the original reviewer's
decision. An implementation author cannot act as their own successor reviewer.

Without explicit designation or required verification evidence, leave closure
pending and report the gap. If reviewers disagree, escalate to the human rather
than replacing a reviewer to obtain a preferred verdict. Succession does not
reset the repair-round count or waive acceptance, CI, or policy-approval gates.

## CI, storage, and release

The check workflow has read-only repository permissions, does not persist Git
credentials, and uploads CI run evidence for 14 days. It does not merge,
publish, sign, run a privileged follow-up workflow, or create PRs. Readiness
PR #1 and its merged-master revision passed three-OS CI on the now-public
repository; see the [current handoff](../reports/0003-publication.md) for exact
source and evidence identities. Later changes require their own CI.

The three-OS matrix tests the foundation tools. It is not a mygo UI or media
runtime test. Windows uses ordinary checks; race checks run on the configured
Linux and macOS jobs. A future Windows race job needs a validated C toolchain.
The recorder currently freezes the foundation spec even on a feature PR;
checking all packages does not establish that feature's acceptance evidence.

Treat issue text, logs, PR descriptions, and downloaded data as untrusted
input. Never expose signing credentials or privileged repository tokens to
untrusted code or tests. Do not execute workflow artifacts in privileged jobs.

Promote only an independently approved, clean, exact source revision through
native packaging, smoke testing, signing/notarization, and release channels.
Public release automation is intentionally absent from the foundation.

## Handoff

Use `docs/templates/handoff.md` for a concise report. Keep only sanitized
conclusions, run IDs, source revisions, verification results, review status,
artifact locators, limitations, and next actions in Git. Link large evidence
to access-controlled storage with an explicit retention policy. Local runs
are not backed up by Git, and CI artifacts are not permanent archives.
