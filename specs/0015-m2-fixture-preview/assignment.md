# M2 roles and execution boundaries

- Owner: accepted the M2 product plan, hardcoded development runtime, and
  three-round clarification, then requested implementation on 2026-11-15.
- Controller/planner/M2-001/integration: `/root` in the primary checkout
  `/Users/dadachen/workspace/my-bilibili`, starting from clean
  `37306aa4f476dff3a67b39d99de527fcc0989794`. Exact packet/candidate manifests and
  the created run ID are recorded before freeze/dispatch in controller evidence.
- Architecture reviewer: `/root/m2_plan_review`, independent of candidate
  authorship. Read-only review of the actual serialized spec, DAG, companion
  contracts, workflow/template diff and base source before freeze. Prior inline
  review is context, not approval of these bytes.
- M2-001 code review: a separate native reviewer context assigned with its exact
  candidate and checks. Worker A/M2-002 and Worker B/M2-003 identities and separate
  authorized worktrees are recorded in dispatch packets after prerequisites.
- Delegation: standing native-agent authority. Managed worktree creation needs
  explicit human instruction; no current creation authorization is inferred.
  No commit, push, remote publication, release or app orchestration is requested.

## Preparation and freeze

The controller's authorized serial preparation paths are this new spec directory,
docs/workflow.md and docs/templates/assignment.md. This is the owner-requested
contract/policy preparation preceding the implementation DAG; workers never edit
them. Record the owner's policy confirmation separately from technical review.
Validate the actual contract via the repository CLI, independently review, then
freeze spec/tasks and archive/hash companions, assignments and policy baseline.
The owner-requested implementation permits a dirty local development baseline;
use an exact manifest/archive rather than treating HEAD plus dirty as identity.

## Minimum inputs and write limits

Read README/current handoff, docs/stage-zero.md, architecture.md, workflow.md,
development.md, testing.md, this spec/tasks and all three companion contracts.
Each implementation task owns only its declared write_paths. Shared declarations
remain read-only after M2-001. Workers return diff/source hashes, AC evidence,
test results and blockers; only the controller writes its run. Reviewer scratch
is outside the tracked candidate and controller evidence; reviewers do not edit
source, frozen inputs or historical reports.

Go1.27.3 darwin/arm64 is present. Native runtime is absent in the primary checkout
at preparation start. Read-only recovery discovery is assigned to
`/root/m2_playback_feasibility`; no native launch or global install is part of
that assignment. Runtime/artifact recovery and native invocations require their
own bounded execution packet before use.

## Review return and stop conditions

Return structured candidate-bound findings, severity, reproduction, required
evidence and proceed/request-changes/needs-human recommendation. Authors cannot
approve themselves. Scope/contracts, missing authority/prerequisites or review
limits stop the affected action and return to the controller.

Initial implementation repair counts are zero for M2-001 through M2-005. Inline
planning amendments are not implementation rounds. Follow the canonical workflow
policy; per-dispatch packets carry cumulative prior findings/rounds and any
human resumption scope/additional allowance. No reset through new agents/runs.

Human escalation packet: task/spec/candidate, finding IDs and expected/observed
behavior, attempts and evidence per round, competing conclusions, alternatives
with trade-offs, recommendation and exact decision requested. Stop the affected
repair/review dispatch until explicit human direction; independent closure and
normal checks still apply.
