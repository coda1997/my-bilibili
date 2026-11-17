# Architecture baseline

Current state is in [STATUS](../STATUS.md); this document describes stable
boundaries only. Decisions are listed at the end.

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
`internal/architecture` enforces two rules in tests: `internal/catalog` and
`internal/app` depend only on each other, `internal/playback` and the standard
library; product packages (`cmd/my-bilibili`, `internal/...`) never import
`experiments/`. Other boundaries here are documented, not yet enforced.

## Packages

| Package | Owns |
| --- | --- |
| `internal/catalog` | Framework-free video and part model |
| `internal/app` | Session: application intent, part selection, `Source` contract, observable state |
| `internal/playback` | Plain playback boundary: `Input`, `Engine`, `Controller`, snapshots, safe error codes |
| `internal/adapters/mpv` | libmpv adapter implementing `playback.Opener`/`Engine`; runtime build notes |
| `internal/desktop` | mygo view and rendering of `app.State` |
| `internal/fixture` | Hash-verified bundled fixture `Source` |
| `internal/previewbundle` | macOS developer bundle layout and runtime checks |
| `cmd/my-bilibili` | Composition root |
| `experiments/` | Probes and harnesses kept as evidence tools; product code must not import them |

`app.Source` returns a `Resolution`: a playback input plus an optional expiry.
Session reuses a cached input only before `ExpiresAt - ResolutionMargin` and
otherwise resolves again. Source detail/resolution run on one owned worker,
with one latest pending request and one acknowledged result. Only UI-thread
Tick accepts a current, live, unexpired result and opens native playback.
Construction does no source I/O; submission deadlines include queue wait.
Close cancels work without waiting on the UI thread, and composition joins
Done through the native dispatcher before window disposal. Done alone does
not request window close. The default input policy remains fixture-only;
real service adapters require their separately reviewed integration contract.

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
