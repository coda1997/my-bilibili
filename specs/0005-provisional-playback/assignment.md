# Assignment: provisional playback continuation

## Identity and authority

- Controller/planner/implementer: `/root`; independent reviewers are the
  existing authorized `/root/m1_architecture_review` and `/root/m1_code_review`.
  No new worker/delegation is inferred. Reviewers cannot edit the candidate.
- Owner on2026-11-11: record known mygo issues, continue using that component
  temporarily for playback, consider replacing it later. The bounded increment
  is existing fixture-preview transport/retry, not full-client/P0 work.
- Prior independent review authority remains for this continuation; no commit,
  push, publication, policy/check/CI change, global install, service/account
  access, dependency change or replacement-engine implementation authorized.
- Base: `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600`, branch
  `codex/m1-macos-playback`, uncommitted. Prior full79-file manifest/archive:
  `.agent/review/m1-switch-handoff-final/`; source verified unchanged before
  new planning files. Dirty HEAD alone is not the candidate identity.
- Prior run `20261111T000541Z-d757e87e7d8b` closed needs-human; never append to it.
  New run only after this spec/task review. Current contract proposed/not frozen.
- Exact planning candidate manifest/archive and spec/task hashes are supplied
  at dispatch under `.agent/review/provisional-architecture-r0/`.
- Tasks PREV-001 through004 are sequential; implement only after independent
  review/freeze, within their exact task write paths. Specs are planning inputs,
  not post-freeze writable implementation paths. No change to old M1 contract.

## Minimum context

Read README/current M1 handoff, stage-zero/testing, architecture/development/
workflow, prior architecture status consultation and delayed-switch results,
this spec/tasks, contract/player/UI/native/smoke source, existing generated
fixture provenance. Use independent-review skill and docs/templates/review.md.
The new ADR/known issue documents do not exist yet; review their specified
decision and scope, not nonexistent implementation success.

## Boundaries

- Review candidate write paths: none. Reviewer scratch:
  `.agent/reviewer-scratch/provisional-architecture-r0/` or later explicitly
  assigned code-review scratch, never controller `.agent/runs/`.
- Read-only: old specs/reports/runs, contract/player/module/fixtures, existing
  P0 planning and workflow/check/CI policy. Controller alone records evidence.
- Runtime: Homebrew Go1.27.3 macOS arm64; actual Windows unavailable. Native
  preview may need an exposed desktop. Controlled generated fixtures only.
- No upstream patch, hidden test-host status pump, unsafe private native access,
  cross-thread native calls or deadline relaxation. UI shows the known stall;
  a successful new preview suite must not pass the default full/M1 gate.

## Review and return

Review temporary-use authority versus unchanged M1 requirements, UI snapshot/
command feedback, disabled states, saved-input copying and retry lifecycle,
suite selection/labels and native-oracle/cleanup honesty. Report findings with
identity/severity/reproduction/required evidence and proceed/request-changes/
needs-human. An author self-check or preview success is not independent review
or final acceptance. Escalate contract/scope/policy ambiguity rather than
silently weakening acceptance. Return actual checks and next action; no new
agents or remote actions.
