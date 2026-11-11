# Architecture baseline

Status: development foundation (M0) accepted by the owner on 2026-11-10.
M1 UI/playback feasibility and its final architecture decision remain outstanding.
The owner chose provisional mygo playback on2026-11-11, limited to the existing
fixture preview while considering replacement later; this does not accept M1.
Current review dispositions: [readiness publication](../reports/0003-publication.md#independent-review).
Historical findings: `reports/0001-independent-review.md`.

## Product scope

Build a Bilibili desktop client, not a replacement Bilibili service. Target
macOS and Windows. The application and UI code we maintain should be Go;
operating-system media facilities are allowed. mygo's native Go UI is the
preferred candidate, contingent on the playback spike.

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

- `docs/adr/0001-foundation-and-evidence.md`
- `docs/adr/0002-go-ui-and-playback-boundary.md`
- `docs/adr/0003-ascii-task-scopes.md` (adopted by the owner on 2026-11-10;
  M0 was separately accepted on the same date)
- [ADR0004: provisional mygo playback](adr/0004-provisional-mygo-playback.md)
  (temporary use, not final player/M1 approval); [open playback issues](known-issues/playback.md)
