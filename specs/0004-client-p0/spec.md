# Desktop client P0

> Owner platform amendment, 2026-11-12: [ADR 0005](../../docs/adr/0005-platform-priority.md).
> macOS is required; Windows is P2 / nice-to-have and does not block M1, M2
> or P0. Windows remains `not-tested` until exercised. Historical reviews
> describe their frozen candidates; this amendment changes only platform priority.

Spec ID: `0004-client-p0`
Status: product-priority draft, updated with owner direction on 2026-11-10.
Login and danmaku are P0, not follow-up features. The detailed behavior and
delivery contract below are proposed, not independently reviewed or frozen.
This planning request does not start M1, authorize implementation/delegation,
accept a milestone, or authorize commits or remote actions.

Planning companions: [execution plan](plan.md),
[concrete architecture proposal](architecture.md), and
[inspected GitHub/API references](api-references.md). They remain proposals
and research inputs, not frozen contracts or runtime evidence.

## User scenario and goal

A desktop user signs in, finds a video, selects a part, watches with danmaku,
and later resumes from local history. Deliver this complete loop on macOS
with maintained application/UI code in Go. Windows is optional P2.

P0 is complete only when login, discovery, details, playback, read-only danmaku,
and local resume work together. An intermediate fixture demo is not P0 delivery.

## Scope and non-goals

| P0 feature | Proposed first-version boundary |
| --- | --- |
| Login/session | Prefer QR sign-in, session restoration, expiry handling, sign-out, protected credential storage; feasibility remains to be demonstrated |
| Discovery | Keyword search, pagination, supported video link/identifier entry, loading/empty/error/retry states |
| Details | Title, cover, creator, description, parts, selection of the part to play |
| Playback | Play/pause, seek, volume, fullscreen, speed, switching parts, available-quality selection, loading/failure/retry |
| Danmaku | Read-only display, visibility, opacity, text size, speed, synchronization with pause/seek/speed/part changes |
| Local history | Stable video/part identity and progress, continue watching, delete/clear; no account-cloud synchronization |

Metadata or playback may be used signed out where permitted; login is a
required capability, not a requirement to gate every screen. Respect account
and content permissions. Unsupported/restricted content must have an explicit
state, never an entitlement bypass or a silently substituted source.

Proposed exclusions: danmaku sending, comments and other write interactions,
personalized feeds, subscriptions, favorites/cloud history, downloads, live
streaming, posting, private messages, public signed releases, and an updater.
Login does not implicitly include the personal-content features above.

### Milestone gates and parallel delivery

M0 and M1 are accepted. The owner accepted M1 on 2026-11-13 with libmpv
playback and mygo UI; M2/P0 remain proposed and unstarted. The
[M1 contract](../0002-playback-spike/spec.md) and
[stage-zero gates](../../docs/stage-zero.md) apply with the owner platform
amendment above. See the [current handoff](../../reports/0025-m1-acceptance.md).

1. M1 prerequisite satisfied: independently reviewed macOS fixture/target
   evidence and the owner-accepted UI/player decision are recorded in the
   current handoff. Windows remains optional P2/not-tested. This does not
   establish sign-in, danmaku or the remaining product prerequisites.
2. Before freezing this contract, resolve sign-in/session and danmaku access
   feasibility, and validate a native overlay/timebase seam with the selected
   player. These are additional P0 prerequisites, not claims that M1 already
   covers or has validated them. If blocked, return to the owner rather than
   removing login or danmaku from P0.
3. Review concrete boundaries, pin dependencies and fixtures, then freeze an
   implementation contract. `P0-001` implements the reviewed boundary types;
   it does not invent or approve contracts inside a frozen run.
4. `P0-002` (fixture details) and `P0-003` (fixture player) can then run in
   parallel. `P0-004` integrates the small M2 slice and collects the independent
   reviews, native preview evidence, repair/escalation evidence, and milestone
   disposition required by stage zero. The fixture slice is deliberately not
   the full desktop shell or an assertion that the full P0 works.
5. After M2 exit evidence and owner authorization to expand, `P0-005` through
   `P0-009` can run in parallel using the frozen ports and controlled fakes.
   Real cross-feature behavior is verified at `P0-010`, not inferred from fakes.
6. Final integration, independent review, and required macOS runtime checks
   are required before proposing P0 acceptance. No task/run outcome grants it.

The graph expresses prerequisites, not automatic scheduling or delegation
authority. If the chosen native renderer requires overlapping player/danmaku
writes, revise the graph and obtain review before freezing; do not retain a
fictional parallel boundary.

## Boundaries and shared contracts

Use the existing single-module modular-monolith architecture. Paths in
`tasks.json` are proposed ownership boundaries for future implementation;
they do not require creating empty packages now.

- Account owns session lifecycle and the interfaces it consumes for sign-in
  and protected credential storage. No password-login fallback is assumed.
- Catalog owns search/details behavior and its external-data interfaces.
- Playback owns media resolution/player interfaces, session commands, events,
  and resource lifetime. Native handles stay inside the player adapter.
- Danmaku owns loading, normalized timed entries, scheduling and settings;
  its view consumes the reviewed playback timebase, not native media handles.
- History owns local progress and persistence interfaces. Use a simple local
  representation, not a new database or generic storage framework.
- Service adapters own request metadata and transport DTO conversion. Shared
  transport plumbing is fixed before parallel work; feature-specific adapters
  live in separate subdirectories. Business modules never import those DTOs.
- Owner-confirmed on 2026-11-10: do not depend on a third-party Bilibili SDK.
  Maintain thin, P0-specific Go adapters behind module-owned ports, using Go's
  HTTP facilities. Reference projects are research inputs, not dependencies
  hidden behind a wrapper or vendored SDK code. This does not exclude reviewed
  UI, media, credential-storage or protobuf dependencies; it does not resolve
  service access, source reuse or feasibility decisions.
- Feature views remain separate; the composition root owns navigation, wiring,
  account propagation, overlay mounting, and cross-feature orchestration.
  No generic event bus or agent framework is introduced.

The reviewed contract must specify concrete video/part identity, summary/detail
data, token-free session state, playback commands/events/timebase, danmaku
entries/settings, history records, and safe error categories before dispatch.
Ports belong to consuming business modules; do not create a universal shared
domain merely to increase the worker count. UI framework types do not cross
into business modules. Feature tests accompany their owned implementation.

### Async, account, and resource behavior

- Every asynchronous operation has cancellation, an explicit finite timeout,
  and an owner. Exact timeout/buffering limits must be set in the reviewed
  contract before implementation; unbounded requests/queues are not allowed.
- UI changes run on the chosen UI framework's required thread. Tests can
  control callbacks, time, network, player events, and persistence.
- Search/video/part switches invalidate older generations. Canceled or late
  results cannot replace current content, revive a closed view, or start audio.
- Login has observable pending, success, expired, canceled and failure states.
  Sign-out clears session credentials and stops/cancels authenticated playback
  and pending account-bound work. Late responses cannot restore a signed-out
  session. Expiry is distinct from a network failure and does not retry forever.
- Session secrets are confined to account/credential/transport boundaries. UI
  state and history contain no session cookies or raw poll keys. A live QR
  challenge is an unavoidable sensitive presentation exception: display it
  only for the active attempt, clear it on completion/expiry/dismissal, and
  exclude it from logs, screenshots and recorded fixtures. Transient media
  locations stay in the resolver/player path, never UI/history/logs.
- Playback serializes open/close/switch and exposes actual media position and
  speed. Close/switch releases player resources, subscriptions and goroutines.
- Danmaku follows media time, not an independent wall clock. Pause freezes
  motion; seek resets scheduling; speed changes stay synchronized; switching
  parts clears old entries. Retries do not duplicate entries in the same pass.
  Manual replay after seeking is allowed. Malformed or unavailable danmaku
  produces a non-blocking state without stopping otherwise valid playback.
- Bound decoded data, active overlays, pending fetches and subscriptions. Set
  concrete caps and overflow behavior during review and verify cleanup.
- Proposed history is device-local and survives sign-out, with explicit delete
  and clear controls. Resume uses a stable identity to resolve fresh permitted
  media, never persisted signed URLs. A small versioned local format must fail
  safely on corruption rather than crashing or silently overwriting the file.

## Acceptance criteria

### P0-AC-01

Before implementation dispatch, M1 has an independently reviewed disposition
and actual macOS evidence; login/danmaku feasibility and concrete shared
contracts are reviewed. Before expansion beyond the fixture slice, M2 has its
required evidence and disposition. Missing prerequisites remain blockers.

### P0-AC-02

Given fixture video details, opening a video shows its metadata and parts;
selecting a part emits the correct playback request. A late prior result cannot
overwrite the selected video; loading, missing content and failure are visible.

### P0-AC-03

Given approved media fixtures on macOS, the player supports play/pause,
seek, volume, fullscreen, speed and part switching. Switching/closing cancels
obsolete work and releases resources without stale playback or callbacks.

### P0-AC-04

Given the reviewed sign-in flow, success creates a session restorable on app
restart using protected storage. Expired/canceled/failed attempts are visible
and retryable. Sign-out removes stored credentials and invalidates pending
account-bound work; storage failure cannot silently fall back to plaintext.

### P0-AC-05

Given controlled success/empty/error/permission fixtures, search, pagination and
supported link/identifier entry reach the correct real-adapter-backed details
and parts without duplicate pages or stale-query results. Invalid input makes
no unintended request. Restricted content has an explicit safe outcome.

### P0-AC-06

Given a permitted selected video/part and current session, fresh media resolves
through the approved adapter with required request metadata and track handling.
Available qualities can be selected; switching safely replaces the session.
Permission/expiry/unsupported-source failures are distinct and never bypassed.

### P0-AC-07

Given controlled timed danmaku, entries render with correct Chinese text and
remain synchronized through pause, resume, seek, speed/fullscreen and part
changes on macOS. Visibility, opacity, size and speed are adjustable.
Loading/error/corrupt-data cases do not break playback; stale responses and
duplicate retry results do not leak into the current part or exceed set bounds.

### P0-AC-08

Given watched video parts, restart preserves useful local progress and continue
watching reopens the correct part/position, clamped to valid media bounds.
Delete/clear is reflected after restart. Corrupt/unwritable history reports a
recoverable state without crashing playback or silently destroying old data.

### P0-AC-09

The integrated flow signs in, searches/opens a video, selects a part, plays with
danmaku, records progress, and resumes after restart. Account expiry/sign-out,
network errors, rapid switching and application close have the defined safe
outcomes. No credentials or private media enter tests, logs or reports.

### P0-AC-10

The exact final candidate has passing repository checks and independent
risk-based review, plus native preview-package and end-to-end evidence on
macOS. Windows is optional P2 and does not block P0 or the M2 preview slice.
Each capability/OS is marked pass/fail/not-tested; Windows stays not-tested
until exercised. A missing
required native check blocks acceptance; compilation or earlier CI is not a
substitute. Preview packages are not signed public releases.

## Verification matrix

The named suites below are required future checks, not existing tests or
executed evidence. Default tests use controlled fixtures without live network
or credentials. Native smoke uses separately authorized sessions/media and
records only sanitized observations, not live login payloads or signed URLs.

| AC | Required verification | Fixture / environment | Evidence |
| --- | --- | --- | --- |
| P0-AC-01 | Inspect M1, feasibility and frozen-contract decisions; inspect M2 exit before expansion | Review packets and exact candidate identities | Reviewer dispositions and owner decisions |
| P0-AC-02 | Catalog detail/presenter tests including late completion | Controlled metadata; the native macOS UI | Tests and observed part selection |
| P0-AC-03 | Player state/lifetime tests and native control smoke | Authorized media, macOS | Controls, cancellation and cleanup matrix |
| P0-AC-04 | Account lifecycle, fake-clock/credential-store tests; real sign-in/restart/sign-out smoke | Synthetic unit flows; authorized native macOS session | Redacted state transitions, no login payloads |
| P0-AC-05 | Catalog/service-adapter parsing, pagination, input validation and stale-query tests; service smoke | Sanitized metadata plus authorized service access | Tests and observed discovery flow |
| P0-AC-06 | Media-resolution adapter tests, expired-session/permission cases, native stream/quality smoke | Sanitized responses and authorized target media | Per-source/quality/OS capability results |
| P0-AC-07 | Deterministic danmaku scheduling/limits tests; native overlay, Chinese text and fullscreen smoke | Synthetic timed entries and authorized target flow | Timebase assertions and sanitized visual results |
| P0-AC-08 | Progress/restart/clamping/delete/corruption/write-failure tests | Temporary local files and fake player events | Persistence/recovery assertions |
| P0-AC-09 | Cross-feature tests plus native scenario walkthrough | Controlled error cases; authorized native flows | Full-loop and account-switch/close outcomes |
| P0-AC-10 | `go run ./cmd/dev check`; `--race` where supported; independent review and package smoke | Exact candidate, macOS runtime | Candidate-bound logs, review and package locators |

## Risks, rollback, and open decisions

Owner product direction: login and danmaku are mandatory P0 features. Proposed
defaults needing detailed-contract confirmation: QR-first login, read-only
danmaku, device-local history that survives sign-out, and no personal feed.

Blocking technical decisions before freeze: evaluated UI/player revision and
adapter; authorized sign-in/service/media/danmaku mechanisms; protected-store
implementation and failure behavior per OS; overlay/fullscreen/timebase seam;
exact boundary signatures, supported input forms, request timeouts, resource
caps, and native runtime/fixture availability. Feasibility failure returns to
the owner/architecture review; it does not downgrade a P0 criterion silently.

Dependencies and shared files have a single serial owner. Unexpected overlap
or changed contracts require review and new frozen inputs before work resumes.
No global tool installation, database, release credentials, CI/check-policy
change or remote action is part of this proposal. Withdraw this draft without
touching existing contracts if product direction changes; later rollback must
preserve user credentials/history and never rewrite old evidence.

## Review and evidence

Prepare independent architecture/security/native-rendering review using
[the assignment packet](architecture-review.md). No review is dispatched yet.
Implementation needs a separately authorized reviewed/frozen assignment.
The controller owns run records and exact candidate archives/manifests.

Future durable conclusions: `reports/0004-client-p0-m2.md` for the intermediate
loop and `reports/0004-client-p0.md` for final P0. Those reports and successful
runtime evidence do not exist yet. Existing stage-zero records remain unchanged.
