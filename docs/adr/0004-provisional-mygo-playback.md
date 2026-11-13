# ADR 0004: Provisional mygo playback, replaceable at the adapter

> Superseded default selection, 2026-11-13: [ADR 0008](0008-default-libmpv.md)
> selects libmpv as the default. The former mygo launcher is now explicitly
> `cmd/mygo-native`; this dated temporary-use decision and its failures remain
> historical evidence. M1 was accepted by the owner on 2026-11-13.

> Subsequent measurement-scope decision: [ADR 0006](0006-m1-measurement-scope.md)
> removes calibrated physical sync/rate measurements from M1 with no mandatory
> later check. They were never performed; the dated evidence below is unchanged.

> Platform amendment, 2026-11-12: [ADR 0005](0005-platform-priority.md)
> supersedes the Windows acceptance requirement below. macOS is required;
> Windows is P2 / nice-to-have, nonblocking for M1, M2 and P0. The dated
> decision and evidence below remain historical; other requirements still apply.

- Date: 2026-11-11
- Status: owner-selected temporary use; not final M1/player acceptance
- Scope: `specs/0005-provisional-playback/`, existing synthetic macOS preview

## Context

Exposed local and loopback HTTP playback works with the pinned mygo player,
including native picture, progression, pause and seek. The strict M1 smoke
still fails on delayed loading: the player synchronously queries native tracks
on the UI thread. A view-only duration gate was rejected because it suppresses
native error refresh. Headers and separate audio are unsupported. See the
[historical M1 evidence](../../reports/0002-playback-macos.md) and
[known issues](../known-issues/playback.md).

## Decision

The owner asked to record these problems, continue using mygo temporarily for
playback, and consider replacing that player later. Keep the existing pinned
component behind the framework-free `experiments/playback/contract` boundary.
Add only transport controls and explicit retry to the controlled fixture
preview. Display its temporary status and limitations in the window.

Do not patch upstream, change dependencies, add hidden render/status hosts,
move native calls off the UI thread, introduce another engine or silently drop
unsupported metadata/audio. No automatic retry. Keep the strict full smoke as
the default; an explicit `preview` suite measures only this bounded increment.

This is not approval of a full client shell, M2/P0 start, production release,
final adapter choice or a waiver of M1's platform/target acceptance criteria.
Independent review of this increment cannot close the known upstream defects.

## Consequences And Replacement Gate

The preview may freeze while loading slow media, including before cancellation
or a controller timeout can run. Limit use to generated fixtures; explicit retry
is not a responsiveness repair. UI errors remain safe typed codes. Keep all
four known issues open with their distinct evidence and workaround limits.

A future replacement stays behind the plain input/command/snapshot/lifetime
boundary and needs a separately reviewed contract and authorization. Acceptance
requires exposed native picture/controls, responsive loading/switch/cancel,
timely native failure cleanup, required headers and separate streams, resource
cleanup and authorized target-media evidence on macOS and Windows. Preserve
the original failed full runs and do not combine successes from different
candidates into a passing stack. Signing, packaging and actual audible output,
rate and A/V-sync measurement need their own evidence.
