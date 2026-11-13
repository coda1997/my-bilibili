# Assignment: <role> / <task-id>

## Identity and authority

- Role: lead / architecture-review / implementation / code-review / handoff
- Controller and assigned agent/context identity:
- User request and explicitly authorized actions:
- Delegation authority: owner's standing authorization for scoped native sub-agent
  work; record any task-specific restriction.
- Commit/push/PR/merge/release authority: not granted unless explicitly stated.
- Run ID and prior run/finding references:
- Checkout/worktree and base revision:
- Exact candidate commit, or uncommitted manifest/archive and hashes:
- Spec path/version/hash and task-graph hash:
- Contract review status; separate human policy approval status:
- Task IDs and satisfied dependency evidence:

## Minimum context

- Current handoff and relevant architecture/ADR sections:
- Frozen spec/tasks and shared interfaces/async behavior:
- Relevant source, diff, tests, and sanitized fixtures/evidence:
- For closure: original finding, reproduction, and required verification:
- For reviewer succession (otherwise not applicable): original reviewer and
  report/finding IDs, unavailability reason, explicit human designation and
  successor independence, prior repair-round count; follow the proposed
  procedure in docs/workflow.md#reviewer-succession-proposed-clarification:

Do not substitute the author's chat or desired verdict for review inputs.

## Boundaries

- Candidate write paths, exactly as assigned in tasks.json:
- Read-only/shared contracts and prohibited changes:
- Scratch/evidence output location (reviewers do not edit the candidate):
- Required environments/tooling and unavailable prerequisites:

The controller owns its run records. Workers/reviewers return evidence; they
do not modify frozen inputs, policy, or each other's reports. This packet and
worktree isolation are not technical access controls.

## Work and return contract

- Required ACs and verification commands/procedures:
- Review dimensions, if applicable:
- Return: source/candidate identity, changed paths, AC-to-test results,
  evidence locators, findings/limitations, and smallest next action.
- Review return: use docs/templates/review.md; state independence truthfully.
- Stop/escalate: unclear authority, boundary/contract changes, unsafe inputs,
  missing prerequisites, unexpected changes, or the workflow's repair limit.

Fill all relevant fields before dispatch. Mark irrelevant fields explicitly;
do not turn placeholders or checked boxes into authorization. A changed scope
needs contract review/new frozen run, not an informal exception in this packet.
