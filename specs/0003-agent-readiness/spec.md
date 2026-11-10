# Agent development readiness and foundation repairs

Spec ID: `0003-agent-readiness`
Status: implementation, independent technical review, three-OS CI, and
user-authorized publication complete through PR #1 (`daca629`). Separate human
adoption of trusted automation policy and M0 acceptance remain pending.

Current status and evidence: [publication handoff](../../reports/0003-publication.md).
The original local-only scope below preceded the user's separate authorization
to publish and merge PR #1. Its contract body, acceptance criteria, task graph,
and historical frozen inputs are unchanged; this status update does not grant
authority for new remote actions or accept the proposed policy.

## Goal

Make the existing development protocol usable from a fresh checkout/context,
and repair COR-001/COR-002 from `reports/0001-independent-review.md`. This is a
bounded foundation follow-up, not permission to start the playback spike.

## Scope and non-goals

Deliver concise repository instructions, role assignments, spec/task/PR
templates, testing and PR guides, four repo-local workflow skills, regression
tests, independent re-review, and an accurate continuation report.

Do not add an orchestrator, change CI policy, install global configuration,
introduce dependencies, commit, push, create a remote PR, change repository
settings, implement playback, or claim M0/entire stage-zero acceptance.

## Contracts and decisions

- `docs/workflow.md` remains the protocol source; skills route to it rather
  than duplicating approval policy. A skill invocation grants no extra rights.
- Role assignments are separate from the strict version-1 task JSON schema.
  The controller owns run records; reviewers do not edit the implementation.
- COR-001: the evidence log is the required output sink. Display failures are
  returned to the caller but must not stop checks or discard a durable result
  when the log remains writable. Persistence failures must remain errors and
  must not publish a success backed by an incomplete log.
- COR-002: proposed ADR `docs/adr/0003-ascii-task-scopes.md` restricts declared
  `write_paths` to ASCII, in addition to existing safe-path checks. Do not
  restrict the shared `ValidPath` helper, spec locations, evidence names,
  absolute checkout paths, source text, or UI text. Existing closed records
  must remain readable. This is a contract restriction needing human review
  before adoption as trusted automation policy, not a Unicode normalizer.
- Existing remote CI for `04213b6` does not validate this changed candidate.
  The unchanged workflow records foundation evidence, not feature acceptance.

## Acceptance criteria

### RDY-AC-01

A fresh reader can find the current handoff, prerequisites, task boundaries,
test entrypoints, and next authorized action using only repository files.
Guidance works without the original chat, personal skills, or ignored local
toolchain; unavailable tooling is reported rather than silently installed.

### RDY-AC-02

Repo-local planning, implementation, independent-review, and PR-handoff skills
have valid metadata, precise triggers, explicit inputs/outputs, and working
references to the canonical protocol. They preserve scope, delegation, and
remote-action authorization; self-review is never independent approval.

### RDY-AC-03

Reusable spec/task, role-assignment, and PR templates cover acceptance-to-test
mapping, source identity, actual verification, untested assumptions, findings,
and next actions. Instantiate the contract templates in scratch space and
validate them with the real validator. Test/CI documentation distinguishes
unit checks, native runtime tests, evidence artifacts, packages, and releases.

### RDY-AC-04

COR-001 has passing and failing check regressions with display errors during
the header, command output, and final result. Each completed check retains its
actual outcome and a complete referenced log when persistence is healthy.
Display errors/short writes are surfaced separately. Real evidence write or
finalization failures remain errors without a misleading published result.

### RDY-AC-05

COR-002 has platform-independent tests for composed/decomposed Unicode file
and subtree scopes, including dependency-ordered cases. Both spellings are
rejected by the proposed ASCII task-scope rule; ASCII ordered overlap remains
allowed, unordered case-insensitive overlap remains rejected. Unicode paths
outside `write_paths` retain their existing safe-path behavior. A macOS
filesystem reproduction is additional evidence, not a portable test dependency.

### RDY-AC-06

Independent reviewers inspect an exact identified candidate, run relevant
checks, disposition their findings, and exercise a realistic fresh-context
workflow. An uncommitted candidate uses a hashed file manifest/archive, not
its base commit as a fictional candidate SHA. A new handoff distinguishes old
remote CI, new local checks, review decisions, pending human policy adoption,
and missing candidate CI. Historical reports and closed runs remain unchanged.

## Verification and delivery

Run targeted regressions, `go run ./cmd/dev check --race` on the macOS host,
skill metadata validation, template instantiation, and independent correctness
and workflow reviews. Document other OS execution as not tested for this
candidate until actual remote results exist. Keep raw sanitized evidence in
the selected run; publish durable conclusions in `reports/0003-agent-readiness.md`.

## Risks and rollback

The ASCII scope restriction is deliberately conservative and may reject a
future task naming a non-ASCII file. Use a valid ASCII ancestor scope only if
the broader ownership is intentional and reviewed; otherwise redesign the
contract separately. Do not silently relax validation. Reverting a candidate
requires an explicit reviewed change; never rewrite frozen runs or historical
findings. No release or data migration occurs in this task.
