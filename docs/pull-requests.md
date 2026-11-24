# Pull requests and handoffs

Follow [agent convergence](agent-convergence.md) R1–R9 for review readiness, issue filing, fix evidence and completion.

## Authority and source identity

Preparing text locally is not permission to commit, push, create a PR, change
branch protection, or merge. Check the explicit task authorization first.
Without remote authority, return a draft body and [handoff](templates/handoff.md).
Creating a PR never implies permission to merge it or publish a release.

For authorized feature work use a local `codex/<short-task-name>` branch. Check
the base branch from the remote rather than assuming `main`; this repository's
configured default is currently `master`. Preserve existing branches/work.
Before a push or PR, verify the intended repository and visibility match the
user's authorization. The owner confirmed public visibility on 2026-11-10.
Do not change visibility to work around an account-plan limitation.

Record the lane, spec/task IDs if any, and the candidate commit SHA; see
[evidence identity](workflow.md#evidence-identity-and-tables). Do not call the
base commit the candidate. PR commits created later need checks/reviews matched to
their content. Do not claim CI ran for a revision it did not test.

## Write the PR

Use [the PR template](../.github/pull_request_template.md). A useful title is
`[<spec-id>] <observable change>`, not a list of implementation steps.

Explain why the change is needed, its boundaries, and which ACs it satisfies.
Give the evidence as a table (claim, evidence, result, scope and not tested),
binding CI to the run URL for the PR head SHA. Include failures, not-tested items, open questions, and
findings with their tier/class, severity, owners and dispositions. Do not paste raw local
logs, cookies, signed URLs, or private media into the PR.

List reviewer identities, independence, dimensions, and the exact candidate
they reviewed. A test pass, author self-review, or local `succeeded` outcome
does not substitute for independent review. Separate technical recommendation
from human approval of checks, policy, task-scope, or CI changes.

## Review, repair, and merge

Start as a draft when implementation, candidate CI, review, or policy approval
is outstanding. Run [checks](testing.md), obtain authorized independent review,
revalidate responses and changed behavior within the remaining review budget.
The assigned independent reviewer technically closes BLOCKING findings; an
author's belief that a fix works does not establish that closure.
If that reviewer cannot continue, use the
[succession procedure](workflow.md#reviewer-succession).
Keep closure pending until designation and independent verification are recorded;
do not attribute a successor's decision to the original reviewer.

Before requesting merge, establish the agreed AC evidence, required platform
results, BLOCKING closures and high-severity rulings for the candidate. Confirm actual remote required
checks/review enforcement, not merely the existence of workflow files. After
the owner-authorized 2026-11-24 branch-protection update, `master` requires
PR-based linear history, administrator enforcement (`enforce_admins: true`),
squash merging, and strict required status checks for all three CI platforms
(`Check (ubuntu-24.04)`, `Check (macos-15)`, `Check (windows-2025)`). The
observed configuration still has zero required approving reviews, so protection
enforces CI checks and linear history but does not enforce the
independent-review gate described here. Recheck the live rules before merging.
Continue using an explicit human merge decision; never bypass protection or
enable unattended merging as a fallback. See the
[publication handoff](../reports/0003-publication.md) for the completed
supervised merge and its actual checks.

## Durable handoff

For full-lane work, write a new sanitized report rather than altering a
historical review or closed run. Use [handoff.md](templates/handoff.md), update
[STATUS](../STATUS.md), and retain source identity, actual
checks, review/finding status, missing evidence, and the smallest next action.

Keep large logs in the controller run or access-controlled artifact storage.
Record artifact locators/expiry and what is not backed up. A local absolute
path is useful on this machine but is not a remotely accessible PR attachment.
If required evidence is unavailable to the reviewer, mark the gate pending.
