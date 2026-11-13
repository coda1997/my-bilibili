# Architecture baseline

Status: development foundation (M0) accepted by the owner on 2026-11-10.
Required macOS M1 technical evaluation is complete. On 2026-11-13 the owner
selected libmpv as the default behind the existing plain playback boundary,
retaining mygo UI; see [ADR 0008](adr/0008-default-libmpv.md) and
[the current handoff](../reports/0025-m1-acceptance.md). The owner accepted
M1 as completed on 2026-11-13; M2/P0 remain unstarted.
The [previous target handoff](../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13) records the
owner-requested removal of target host restrictions. All valid HTTPS media hosts
and ports are accepted; source/rendition, TLS, expiry and lifecycle checks remain.
The first version-2 native target attempt failed readiness/picture and graceful
exit; its cause remains unknown. After the safe-diagnostic repair and separate
readiness review, a later target invocation passes picture, selected separate
tracks, controls, 87.7575ms playing cancellation and graceful exit. Independent
review accepts this scoped evidence and closes the current target blocker;
the previous stopped refresh and failed invocation remain preserved.
The owner’s 2026-11-11 provisional mygo selection is superseded. The old player
remains available as `cmd/mygo-native` for diagnostics, with its known failures.
Current review dispositions: [readiness publication](../reports/0003-publication.md#independent-review).
Historical findings: `reports/0001-independent-review.md`.

## Product scope

Build a Bilibili desktop client, not a replacement Bilibili service. Target
macOS as the required platform; Windows is P2 / nice-to-have, nonblocking for
M1, M2 and P0 under the owner
[platform-priority decision](adr/0005-platform-priority.md) of 2026-11-12.
The application and UI code we maintain should be Go;
operating-system media facilities are allowed. The accepted M1 decision retains mygo's native Go UI with libmpv playback
behind the existing plain boundary.

## Application shape

Use one Go module and a modular monolith. Introduce capability packages only
when implementing a real slice, such as account, catalog, and playback.
Do not create empty packages for every imagined future feature.

- UI and presenters invoke application use cases and expose observable state.
- Business modules own the interfaces they consume at external boundaries.
- Bilibili adapters own request details and external DTO conversion.
- Playback adapters own media handles, controls, events, and cleanup.
- Platform adapters own credential storage and operating-system facilities.
- One composition root wires implementations into the application.

Business modules must not import UI types, platform handles, database drivers,
or Bilibili transport DTOs. Cross-module dependencies must be explicit.
The foundation currently validates task contracts, not these future product
import rules. Add executable import checks when the first product packages
exist; do not describe documentation-only boundaries as enforced.

## Async contract requirements

Specify cancellation, timeout, state transitions, goroutine ownership, callback
thread affinity, late-result handling, and resource disposal. For example,
opening video B must prevent a late result for video A from replacing B.
Go interface signatures alone do not describe this behavior.

## Test seams

Keep network access, playback, persistence, and time controllable in tests.
Default tests must not need a real account or live network access. A headless
use-case adapter and UI automation belong to the first product slice, not to
an imaginary testing layer that has no executable behavior.

## Delivery control plane

Separate the product from development tooling. The initial Go `dev` command
records evidence and runs deterministic checks. It does not choose agents,
approve changes, or implement an autonomous workflow.

A later controller should own scheduling, budgets, bounded retries, trusted
policy versions, execution isolation, and promotion decisions. Agent outputs
are inputs to those decisions, not authority to change the rules.

## Decisions

- [ADR 0007: target host policy](adr/0007-target-host-policy.md): all valid HTTPS
  hosts/ports are allowed; version-2 descriptors carry no origins and share URL
  syntax validation with the refresh helper.

- `docs/adr/0001-foundation-and-evidence.md`
- `docs/adr/0002-go-ui-and-playback-boundary.md`
- `docs/adr/0003-ascii-task-scopes.md` (adopted by the owner on 2026-11-10;
  M0 was separately accepted on the same date)
- [ADR 0005: platform priority](adr/0005-platform-priority.md): macOS is required;
  Windows is P2 / nice-to-have and does not block M1, M2 or P0.
- [ADR 0006: M1 measurement scope](adr/0006-m1-measurement-scope.md): calibrated
  physical sync/rate measurements were never performed and are not required
  for M1 or scheduled as a mandatory later check.
- [ADR0004: provisional mygo playback](adr/0004-provisional-mygo-playback.md)
  (historical temporary use, superseded by ADR 0008); [legacy playback issues](known-issues/playback.md)
- [ADR 0008: default libmpv](adr/0008-default-libmpv.md): owner-selected default
  backend with mygo UI retained; M1 accepted on 2026-11-13.
