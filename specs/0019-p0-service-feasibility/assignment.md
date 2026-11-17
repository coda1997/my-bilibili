# Assignment: P0 service-feasibility architecture and implementation

## Identity and authority

Controller: /root. User requested starting P0 on 2026-11-17 and then explicitly
selected ordinary web QR login/read-only service access with Mac Keychain
storage. This authorizes this bounded startup work after its contract review,
not commits, remote writes, releases or full-P0 acceptance. Native delegation
uses the owner's standing AGENTS.md authority. No workflow-policy change.

Base: e35022c1218588b2ebacc4c5e69b5f4cd431a9fe on
codex/issue-11-seek-queue, checkout
/Users/dadachen/workspace/my-bilibili/.cindy-worktrees/brave-newton.
Preexisting .claude/ is excluded and untouched. Source identity for dispatch is
.agent/p0-start/architecture-manifest.json plus architecture-candidate.tar;
manifest includes all tracked source and new spec files, with per-file hashes,
modes and archive SHA-256. No run frozen until architecture review approves.
No prior P0 review finding or implementation; architecture round 1, code round
0, repairs 0, repeated failures 0. Shared inventory .agent/p0-start/inventory.md;
issues filed 0/5, no remote filing authority. Maximum 3 formal reviews per task,
3 automatic repairs, stop on third same-behavior failure. Architecture counts
are retained separately and never used to hide a repair/code review round.

## Minimum context and role scopes

Read AGENTS.md, README/current reports 0029/0030, docs/agent-convergence.md,
architecture.md, development.md, workflow.md and testing.md. Read this spec,
tasks, references and existing full P0 draft 0004. Current source evidence is
in internal/catalog, internal/app, internal/desktop and the existing playback
experiment. Historical 0004 proposal is not the selected implementation task.

Independent architecture reviewer: use independent-review skill and review
only 0019's bounded probe contract, API facts, privacy, timeouts and test mapping.
Candidate is read-only. Write review/logs only in .agent/p0-review/. Return an
attributed review using docs/templates/review.md. Assess a proceed disposition
for the bounded probe, never waive full login/Keychain/native P0 prerequisites.
No network calls to Bilibili, native launches, code changes or controller run
writes. This reviewer may later review the code because it does not author it.

Implementer (controller after approval): use feature-implement, task P0F-001;
only experiments/p0/serviceprobe and experiments/p0/cmd/service-probe writable.
Frozen spec/task hashes and review reference will be supplied by run status.
Use Go 1.27.3, GOTOOLCHAIN=local GOWORK=off; wholly synthetic tests. No Bilibili
calls until code approval. No new module dependency. Required command: go run
./cmd/dev check --race plus targeted tests and meaningful negative controls.

Controller task P0F-002: one bounded live invocation only after independent
code approval, then sanitized report in reports/0031-p0-start.md. Follow
pr-handoff for durable conclusions, with no remote write. Only the controller
writes .agent/runs. User scan/sign-in, native overlays and Keychain are later
reviewed slices, not waived or reported successful here.

## Completion and return

P0F-AC-01/02 need executable synthetic protocol/privacy/lifetime evidence;
P0F-AC-03 needs one actual sanitized observation, even if unavailable;
P0F-AC-04 needs full checks, independent code disposition and honest roadmap.
Review return includes exact candidate/spec/tasks hashes, actually performed
checks, findings by R1 class, observed versus inferred, all remaining gaps and
updated counts. Preserve prior reports. No reviewer succession needed at start.
Found-not-done matters go to the common inventory with no-filing reasons.
Missing required evidence remains explicit; no full P0/runtime/CI success claim.
