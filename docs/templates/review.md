# Review: <spec-id>

Follow [agent convergence](../agent-convergence.md) R1/R2/R3/R4/R5/R6/R7/R9 for classification, disposition, scope, budgets, triage and rulings.

- Reviewer/context identity:
- Independent of implementation author: yes / no
- Spec SHA-256:
- Task-graph SHA-256:
- Base commit and exact candidate commit or manifest/archive identity:
- Diff scope and any excluded/unavailable inputs:
- Review dimensions:
- Original goal, locked scope and done criteria:
- Policy version; review round; cumulative fix iterations / same-failure counts:
- Checks actually performed:
- Decision: request-changes / approve / needs-human
- Separate human policy-approval status (not granted by this review):
- Reviewer succession, if applicable: original reviewer/report/finding IDs,
  human designation reference, reason, successor identity/independence, and
  prior review/fix counts; otherwise not applicable:

## Findings

For each finding record:

- ID and severity:
- Tier: BLOCKING [class N] / ADVISORY; evidence for the class:
- Candidate/spec identity to which the finding or closure applies:
- File and line:
- Observed and expected behavior:
- Reproduction/evidence; observed facts versus inference:
- Contract relied on:
- Acceptance criterion affected:
- In scope / out of scope; required in-scope fix and verification (BLOCKING),
  or advisory response/destination:
- Disposition, owner and verifying reviewer where required:
- Prior response / changed cited lines if revisiting an advisory:

## High-severity rulings

For each P0/P1 or high/critical claim, record the independent reviewer/human
identity, candidate and one-line ruling; otherwise state none:

`<finding id>: BLOCKING | not BLOCKING: <one-line reason>`

## Consolidated triage and completion

- Complete finding inventory and one-pass triage result:
- Existing issue search/results; single advisory follow-up destination:
- Cumulative new issue count; not-filed/deferred items and reasons:
- Original done criteria with evidence or gaps; remaining BLOCKING IDs:
- Found, not done: out-of-scope IDs and recorded destinations:
- Next action and remaining review/fix budget:

For successor closure, follow the proposed succession procedure in
docs/workflow.md#reviewer-succession-proposed-clarification. Attribute the new
verification to the successor, retain the original finding ID and report, and
reference new candidate-specific evidence. Missing designation/evidence leaves
closure pending; do not rewrite the original decision or reset repair rounds.

## Remaining uncertainty

State untested platforms, missing credentials or fixtures, and residual risks.
An empty finding list is not proof of bug absence or a substitute for tests.
Do not identify a dirty candidate by its unchanged HEAD commit. A technical
approval does not establish missing platform CI, human policy approval, merge
authority, or release readiness.
