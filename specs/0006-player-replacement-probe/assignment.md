# Assignment: architecture review / proposed replacement probe

## Identity and authority

- Planner/controller: current `/root` context. Reviewer: **unassigned**; this
  packet is prepared for a fresh independent context, not a self-review.
- Owner request on 2026-11-11: agreed to planning a bounded replacement-player
  probe behind the existing adapter after the current preview handoff.
- Current authority: source investigation, proposed spec/task DAG, validation
  and review-packet preparation. No implementation, runtime acquisition/global
  installation, contract freeze or reviewer dispatch is included.
- Delegation: not granted for this new planning request. Prior historical
  reviewer assignments do not assign a reviewer to this new contract.
- Commit/push/PR/merge/release: not granted. No policy/check/CI changes proposed.
- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
  Planning base: `3ecf7622725033d04aff0135c2c9ac02ca128b08`, clean before these
  four new planning files. No new branch/worktree/session is requested.
- Exact planning candidate: `.agent/review/replacement-plan-r0/candidate.tar`
  and `manifest.json`; their hashes are in adjacent `identity.json`, generated
  after this packet. The manifest covers every tracked and nonignored untracked
  source file with mode/SHA-256; it excludes `.git/`, ignored local tools/runs,
  research caches, binaries and the archive itself. Validate it against the
  archive and review that snapshot, not a possibly later working tree.
- Spec: `specs/0006-player-replacement-probe/spec.md`, SHA-256
  `2dfe8b96838dbae6f99c7b621b84d5cd605a5840bc5e846156a33c2d209a8721`.
- Tasks: `specs/0006-player-replacement-probe/tasks.json`, SHA-256
  `ab0e20d71bd66ee76c807e1df8295968db19d98de7767598486e9469b53d5fae`.
- Research: `specs/0006-player-replacement-probe/research.md`, SHA-256
  `e93964e094ee47821091d9e1fad07348c5877b86258cc2e970cef851dae3a449`.
- Review status: proposed, not reviewed, not frozen. No separate human policy
  approval is needed for writing this plan; no new trusted automation policy
  is proposed. Future implementation authority remains a separate requirement.
- Run: none created/frozen for planning. Prior run
  `20261111T002803Z-3eb554cdb1c1` is closed needs-human; never append to it.
  Validation log/result: `.agent/planning/0006-player-replacement-probe/`.
- Future tasks RPL-001 through RPL-003 are serial, inactive proposed work.
  RPL-002 additionally requires an independent exact-candidate gate-one review;
  a failed RPL-001 can record its rejection without starting dependent tasks.

## Minimum context

Read README and the current `reports/0005-provisional-playback.md`,
`docs/stage-zero.md`, `docs/testing.md`, `docs/architecture.md`,
`docs/development.md`, `docs/workflow.md`, ADR 0004 and the playback known-issue
register. Follow the independent-review skill and `docs/templates/review.md`.
Historical reviews describe historical candidates only.

Review all four files in this spec directory and their exact-candidate diff.
Relevant maintained code is `experiments/playback/{contract,player,ui}`,
`cmd/native`, `cmd/smoke`, `cmd/fixturehttp`, fixture provenance and module pins.
The source rationale links pinned upstream client/render/options/license files.
Verify source claims directly; a documented API is not runtime evidence.
No new implementation, native build or fixture result exists in this packet.

## Boundaries

- Reviewer candidate write paths: none. Future implementation paths are exactly
  those in tasks.json and are not writable under this review assignment.
- Proposed reviewer scratch: `.agent/reviewer-scratch/replacement-plan-r0/`,
  only after separately authorized assignment. Return a sanitized review report;
  the controller alone owns its run records. Do not edit frozen/published inputs.
- Read-only: original player/contract/fixtures/smoke, existing reports/runs,
  P0 planning, module pins and workflow/check/CI policy. This is planning, so
  even the future task paths do not grant present code-change authority.
- Environment inspected: Homebrew Go 1.27.3, macOS arm64; local ignored module
  cache present. Use `GOTOOLCHAIN=local GOWORK=off`, and the verified local cache
  if available; do not silently install tooling. Native libmpv, Windows runtime,
  target-service access and calibrated audio/sync evidence are unavailable or
  unverified. No account data, signed URL or private media is needed.

## Work and return contract

Review specifically:

1. Whether the narrow software-frame/public-bitmap path can preserve the Go UI
   and framework-free boundary without private native access or hidden hosts.
2. Synchronous Close versus libmpv destruction, callback/render quiescence,
   partial initialization, one-core ownership and the first-gate stop rule.
   Reject any suggestion that queued quit or early closed state proves disposal.
3. ABI/version checks, copied native memory/events, callback/library lifetime,
   queue bounds, asynchronous ordering and generation isolation.
4. Header list encoding, both-stream application, cross-session isolation,
   missing-audio failure and separation of fetched tracks from audible sync.
5. Evidence strength: unchanged timing/oracle bounds, explicit stages,
   preserved failures/occlusion, native versus headless results, and unchanged
   M1/Windows/target-access requirements. Review the early-rejection path as
   well as the successful path through the DAG.
6. Concrete future runtime provenance/acquisition blockers, unchanged legacy
   defaults and strictly bounded write paths. This review cannot authorize
   dependency installation, implementation, delegation or promotion.

Run the ordinary repository validation if appropriate after checking prerequisites:
`go run ./cmd/dev check --race`. It validates the task graph and current Go
baseline, not proposed native behavior or the truth of acceptance criteria.

Return `docs/templates/review.md` fields: reviewer independence, exact candidate
and spec/task identities, actual checks, findings with stable IDs/severity/
location/reproduction/AC/required verification, and approve/request-changes/
needs-human. Do not pre-fill approval or use author validation as review.

Stop and return needs-human if authority, runtime supply or Close semantics
require a scope change. Required contract changes go back to planning/review
and a new run before implementation. The smallest next step after this packet
is separately authorized independent architecture review, not code execution.
