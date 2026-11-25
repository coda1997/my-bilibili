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
`internal/architecture` checks the plain business boundary across `catalog`,
`app`, `playback`, `danmaku`, and `history` (including test files) on all six
CI build targets, prevents `internal/account` and its tests from depending on
`net/http/...` or nonstandard packages, and prevents product packages and tests
from depending on `experiments/`.

## Packages

| Owner | Responsibility |
| --- | --- |
| `internal/account`; `internal/adapters/credentials` | Login, restore and revocation; the platform adapter stores and verifies session material in Keychain. |
| `internal/adapters/bilibili` | Private session encoding and scoped read-only web transport. Catalog, playback and danmaku adapters turn service responses into plain feature types. |
| `internal/catalog` | Parse direct entries, search and paginate. Own video and part identities, results and the cover boundary. |
| `internal/app` | Own one video's details, selected versus playing part/quality, source requests and confirmed resume seek. |
| `internal/playback`; `internal/adapters/mpv` | Validate input under the chosen policy and own the native playback engine. The mpv adapter reports when the current engine has actually painted a picture. |
| `internal/danmaku` | Load bounded segments for the accepted part/generation and schedule visible comments against native playback time. |
| `internal/history`; `internal/adapters/localhistory` | Track visits and progress; acquire exclusive ownership before reading or mutating the local history file and release it after the final save. |
| `internal/desktop` | Native discovery, history, account and player views. Render fitted video and Chinese text without owning transport data. |
| `cmd/my-bilibili` | Compose the normal or synthetic backends, publish verified account changes, retire outgoing work and close the window after ownership ends. |
| `internal/fixture`; `internal/previewbundle` | Verified bundled media, runtime checks, developer packaging and strict evidence formats. Product code does not import the probes in `experiments/`. |

A verified account publishes a service scope that can be revoked. Switching
account or closing the app cancels outstanding work; late responses cannot
replace the active session. Credentials stay on their permitted API routes.
The normal app explicitly opts into HTTPS media playback. The media opener
accepts only playback headers under that policy; it does not receive account
cookies. The fixture player keeps its local-file policy.

`app.Source` returns a playback input with an optional expiry. A single worker
resolves the latest requested part/quality; its timeout includes queue wait.
Only a UI-thread tick can accept the current result. The player refreshes a
cached input inside its expiry margin. A quality change keeps the current
part and restores position; an explicit part change starts a new visit. To
resume after a restart, the composition root opens the stored video and part,
resolves fresh media and confirms the seek before resuming playback.

The danmaku loader keeps bounded current and adjacent segments. The renderer
shapes only the active comments, clips them to the fitted video and releases
old layouts when the active playback generation changes. Its timebase is the
sampled playback position, including after seek, speed or quality changes.
History saves progress locally under an exclusive lock; deleting the current
visit suppresses its recreation on a quality change. A later explicit visit can
create a new record. See [the P0 contract](../specs/0027-p0-real-client/spec.md)
and [verification on macOS](../reports/0039-p0-real-client.md).

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
