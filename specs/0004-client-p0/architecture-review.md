# Assignment: architecture-review / desktop client P0

> Current scope amendment, 2026-11-12: [ADR 0005](../../docs/adr/0005-platform-priority.md)
> supersedes Windows requirements in this unissued 2026-11-10 packet: macOS
> runtime/preview evidence is required; Windows is optional P2 and nonblocking.
> Standing native delegation authority is now recorded in `AGENTS.md`. Before
> dispatch, replace the historical identity/environment below with an exact
> current candidate and assignment. This packet remains unissued; the detailed
> P0 architecture is not approved by the platform amendment.

## Identity and authority

- Role: proposed independent architecture/security/native-UI reviewer.
- Controller: current planning context; reviewer: unassigned. The planner is
  not an independent reviewer of this proposal.
- Owner request: define and prioritize core client features, split future
  parallel work, include login and danmaku in P0, and prepare an execution plan,
  concrete code architecture and GitHub API references.
- Owner integration decision on 2026-11-10: maintain thin P0 Go adapters, with
  no third-party Bilibili SDK dependency. Other reviewed dependencies remain
  possible; this is not approval of the remaining architecture or access route.
- Authorized now: local planning documents and owner-requested Go installation
  (subsequently specified as Homebrew/global). No product implementation or M1
  runtime spike is started by the request to discuss M1.
- Delegation, commit, push, PR, merge and release authority: not granted.
- Run ID: none. `run list` returned an empty list after Go installation; there
  is no local run ID to inspect with `run status`. This does not change durable
  M0. Repository checks and their source identities remain separate from runs.
- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
- Base: clean detached `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600` before
  creating this draft. That base is not the modified planning candidate.
- Candidate: uncommitted draft, not frozen or ready for dispatch. Before
  dispatch, record an exact archive and per-file hash/mode manifest of the
  relevant candidate, plus spec/task hashes, following `docs/development.md`.
- Inputs: `specs/0004-client-p0/spec.md` and `tasks.json`; hashes must be
  recorded from the exact selected review candidate before dispatch.
- Contract review: not performed. Product-priority agreement does not approve
  concrete contracts or change workflow/check policy.
- Task IDs: review all `P0-001` through `P0-010`; none is authorized to start.
- Dependencies: M1, sign-in/danmaku feasibility, and native access remain
  unresolved. M2 evidence is additionally required before the expansion wave.

## Minimum context

- `README.md`, `reports/0003-policy-adoption.md`, `docs/stage-zero.md`.
- `docs/architecture.md`, `docs/workflow.md`, `docs/development.md`,
  `docs/testing.md`, `docs/adr/0002-go-ui-and-playback-boundary.md`.
- `specs/0002-playback-spike/spec.md` and `tasks.json`, then this proposed
  P0 spec/task graph, `plan.md`, `architecture.md`, `api-references.md`, and the
  exact candidate manifest. Source revisions/statuses in the reference notes
  are research evidence, not native/service runtime evidence or permission.
- Concrete shared interfaces, dependency revision, async limits and safe
  fixture provenance must be supplied before implementation-contract approval.
  There is no product implementation or native runtime evidence to review yet.
- Finding-closure inputs: not applicable; this is a new proposal, not closure
  of an existing finding. Historical findings remain tied to their candidates.

## Boundaries

- Reviewer candidate write paths: none; review is read-only. Proposed future
  implementation scopes are exactly those in `tasks.json`.
- Do not rewrite existing contracts, stage-zero policy, run records, checks,
  or reports to make this proposal appear approved.
- Scratch/evidence location: an explicitly assigned reviewer-owned scratch
  directory, not the controller's `.agent/runs/`. The controller attaches only
  sanitized returned evidence to its own selected run.
- Required tools: Git and the pinned Go version for real repository checks.
  The owner-authorized `brew install go` installed Go 1.27.3 at
  `/opt/homebrew/bin/go`; other tools/dependencies have no implicit installation
  authority. Native feasibility requires actual macOS and Windows runtime
  access plus authorized sessions/media.
- Current unavailability: Windows runtime access and safe service/media
  fixtures are not established. Go itself is no longer a blocker.

## Work and return contract

- Check the login/danmaku-inclusive product boundary and AC-to-test mapping.
- Check milestone gates: M1 is not silently started or widened, the M2 fixture
  slice remains small, and partial demos cannot satisfy full P0 acceptance.
- Check consuming-module port ownership, UI/DTO separation, dependency pins,
  cancellation/timeouts, thread affinity, stale-result rejection and cleanup.
- Check protected credential storage, sign-out/expiry invalidation, redaction,
  permission handling, ephemeral QR presentation, history privacy and absence
  of live default fixtures. Check the owner-selected API/access route and
  source-reuse decision given archived references and unresolved licensing.
- Check the owner-confirmed SDK-free boundary: no direct, wrapped or vendored
  Bilibili SDK; transport DTOs/signing stay in adapters. Necessary non-SDK
  dependencies still need separate selection, pinning and license review.
- Check whether danmaku can truly use a separate renderer/timebase seam on
  each OS, including fullscreen. Require ordered scopes if shared edits remain.
- Check non-overlapping parallel scopes, shared-file ownership, fake-to-real
  integration evidence, data/resource bounds and honest native packaging scope.
- Validation expectations: `git diff --check`, real repository spec validation
  and `go run ./cmd/dev check` once prerequisites exist. Future implementation
  additionally needs race checks where supported and the spec's native matrix.
- Return using `docs/templates/review.md`: exact candidate/spec identity,
  findings, tested versus untested assumptions, limitations, and a
  proceed/request-changes/needs-human recommendation. No self-approval.
- Stop/escalate unresolved authority, missing prerequisites, unknown service
  feasibility, unsafe fixtures, changed boundaries or unexpected user changes.

## Current planning verification and next action

No independent review or runtime/check success is claimed here. An initial
source inspection, concrete architecture proposal and execution plan now
accompany the product draft. Next, discuss the defaults/access findings and
separately select M1/review work. Before dispatch, replace this not-ready
identity section with a bounded
assignment containing actual reviewer authority, exact candidate hashes,
available environments and a reviewer-owned evidence location. Do not treat
this draft packet as permission to spawn an agent.
