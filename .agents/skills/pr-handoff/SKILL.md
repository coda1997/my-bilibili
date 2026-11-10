---
name: pr-handoff
description: Prepare a my-bilibili PR description or durable continuation handoff from actual checks and independent reviews. Use when handing off scoped work or drafting a PR; remote actions require separate explicit authorization.
---

# Publish an honest continuation

Read the current handoff from [README](../../../README.md) and follow
[PR/handoff guidance](../../../docs/pull-requests.md) and the canonical
[workflow](../../../docs/workflow.md). Check the requested authority before any
commit, push, PR creation, remote setting change, merge, or release.

Inputs: selected spec/tasks, exact candidate identity, actual verification and
OS results, original independent findings/dispositions, evidence locators and
retention, and unresolved acceptance or human-policy decisions.

Use the [PR template](../../../.github/pull_request_template.md) for a draft
body and the [handoff template](../../../docs/templates/handoff.md) for a new
durable report. Map ACs to observed evidence; distinguish local checks, CI for
the actual tested revision, native runtime tests, packages, and releases.

Preserve historical reports and closed runs. Update current navigation only
within assigned paths. Identify uncommitted changes by their manifest/archive,
not the base SHA. State local-only evidence and expiry honestly; a local path
is not an attachment available to remote reviewers.

Return the draft/report paths, actual checks and independent decisions,
remaining gates, and smallest next action with its owner/required authority.
Keep the PR a draft while required gates are pending. A handoff or green run
does not close someone else's finding or authorize remote publication.
