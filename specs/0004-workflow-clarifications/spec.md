# Read-only routing and reviewer succession

Spec ID: `0004-workflow-clarifications`
Status: user-authorized documentation proposal. Independent contract review
and separate human adoption of changed workflow policy remain pending.

## Goal and scope

Clarify the two gaps identified in the configuration review: read-only requests
need no implementation packet, and an unavailable reviewer needs an explicit,
independent successor. The user authorized a new worktree and these local edits.

Write only the paths in tasks.json. Do not change product/tool code, CI,
historical reports, existing frozen inputs, or the user's untracked Copilot
instructions. Do not delegate, commit, push, create a PR, or start M1.

## Shared contract

Keep detailed rules in docs/workflow.md and route guides/skills to them.
Read-only routing must not authorize edits or grant independent approval.
Succession requires explicit human designation, fresh independent verification,
preserved original findings, and escalation of missing evidence or disagreement.
It must not reset repair limits or let an author approve their own work.
These are documentation proposals, not self-adopted trusted automation policy.
Async, resource-lifetime, and platform interfaces are unchanged.

## Acceptance criteria

### WFC-AC-01

A read-only consultation can proceed without an assigned spec, role packet,
or new run. Navigation explains when the request must return to the scoped
change workflow; mutation, delegation, and approval boundaries remain intact.

### WFC-AC-02

When an original reviewer is unavailable, the human can explicitly designate
an independent successor. Assignment/review packets capture the original
finding, designation, exact candidate, new evidence, and attributed disposition
without overwriting the original report or implicitly closing the finding.

### WFC-AC-03

An author, undesignated replacement, or reviewer missing required evidence
cannot claim closure. Disagreement escalates to a human, and reassignment does
not reset repair rounds. The handoff states actual checks and pending review
and policy adoption, without changing historical acceptance decisions.

## Verification

| AC | Procedure | Evidence |
| --- | --- | --- |
| WFC-AC-01 | Walk through consultation and consultation-to-edit scenarios | Local scenario log and entrypoint inspection |
| WFC-AC-02 | Populate assignment/review templates for a designated successor | Scratch packets and scenario log |
| WFC-AC-03 | Walk through author, undesignated, missing-evidence, and disagreement cases | Local scenario log; pending gates in handoff |

Run the real Go check with race support on available macOS, validate the changed
skill's metadata and local links, and instantiate contract templates in scratch.
Scenario walkthroughs are author checks, not independent review or proof of
agent behavior. No new native Windows/Linux, remote CI, or playback claim.

## Risks and rollback

Avoid reviewer shopping and accidental authority expansion. Leave all new
workflow-policy clarifications pending separate review/adoption. Roll back only
these proposed edits if rejected; preserve user work and historical evidence.
