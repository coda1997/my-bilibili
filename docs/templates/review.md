# Review: <spec-id>

- Reviewer/context identity:
- Independent of implementation author: yes / no
- Spec SHA-256:
- Task-graph SHA-256:
- Base commit and exact candidate commit or manifest/archive identity:
- Diff scope and any excluded/unavailable inputs:
- Review dimensions:
- Checks actually performed:
- Decision: request-changes / approve / needs-human
- Separate human policy-approval status (not granted by this review):
- Reviewer succession, if applicable: original reviewer/report/finding IDs,
  human designation reference, reason, successor identity/independence, and
  prior repair-round count; otherwise not applicable:

## Findings

For each finding record:

- ID and severity:
- Candidate/spec identity to which the finding or closure applies:
- File and line:
- Observed and expected behavior:
- Reproduction or concrete reasoning:
- Acceptance criterion affected:
- Required fix and verification:
- Disposition and verifying reviewer:

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
