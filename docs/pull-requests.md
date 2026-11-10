# Pull requests and handoffs

## Authority and source identity

Preparing text locally is not permission to commit, push, create a PR, change
branch protection, or merge. Check the explicit task authorization first.
Without remote authority, return a draft body and [handoff](templates/handoff.md).
Creating a PR never implies permission to merge it or publish a release.

For authorized feature work use a local `codex/<short-task-name>` branch. Check
the base branch from the remote rather than assuming `main`; this repository's
configured default is currently `master`. Preserve existing branches/work.
Before a push or PR, verify the intended remote is still the user's private
repository. Do not change visibility to work around an account-plan limitation.

Record the spec/task IDs and exact source identity. A draft for an uncommitted
candidate must say so and refer to its manifest/archive; do not call the base
commit the candidate. PR commits created later need checks/reviews matched to
their content. Do not claim CI ran for a revision it did not test.

## Write the PR

Use [the PR template](../.github/pull_request_template.md). A useful title is
`[<spec-id>] <observable change>`, not a list of implementation steps.

Explain why the change is needed, its boundaries, and which ACs it satisfies.
For each claim link a concrete check/evidence locator and state the actual
result and platform. Include failures, not-tested items, open questions, and
lower-severity findings with their owners/dispositions. Do not paste raw local
logs, cookies, signed URLs, or private media into the PR.

List reviewer identities, independence, dimensions, and the exact candidate
they reviewed. A test pass, author self-review, or local `succeeded` outcome
does not substitute for independent review. Separate technical recommendation
from human approval of checks, policy, task-scope, or CI changes.

## Review, repair, and merge

Start as a draft when implementation, candidate CI, review, or policy approval
is outstanding. Run [checks](testing.md), obtain authorized independent review,
repair within the [bounded loop](workflow.md#review-and-repair-policy), and
revalidate the changed candidate. Reviewers close their own findings; do not
resolve a review comment solely because the author believes the fix works.

Before requesting merge, establish every AC, required platform result, and
blocking review disposition for the candidate. Confirm actual remote required
checks/review enforcement, not merely the existence of workflow files. The
2026-11-10 audit found `master` unprotected and private-repository protection
APIs unavailable under the account plan. Recheck before relying on enforcement.
Until protection and approval policy are explicitly arranged, use a human
merge decision; never enable unattended merging as a fallback.

## Durable handoff

Write a new sanitized report for this continuation rather than altering a
historical review or closed run. Use [handoff.md](templates/handoff.md), update
the current pointer in README/stage-zero, and retain source identity, actual
checks, review/finding status, missing evidence, and the smallest next action.

Keep large logs in the controller run or access-controlled artifact storage.
Record artifact locators/expiry and what is not backed up. A local absolute
path is useful on this machine but is not a remotely accessible PR attachment.
If required evidence is unavailable to the reviewer, mark the gate pending.
