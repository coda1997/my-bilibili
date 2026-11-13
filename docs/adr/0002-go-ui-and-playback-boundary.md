# ADR 0002: Prefer native Go UI; validate playback before commitment

> Default adapter selection, 2026-11-13: [ADR 0008](0008-default-libmpv.md)
> selects libmpv for playback and retains mygo for UI. The original proposal
> and research below are historical; formal M1 acceptance remains pending.

> Platform amendment, 2026-11-12: [ADR 0005](0005-platform-priority.md)
> supersedes the Windows acceptance requirement below. macOS is required;
> Windows is P2 / nice-to-have, nonblocking for M1, M2 and P0. The dated
> decision and evidence below remain historical; other requirements still apply.

Date: 2026-11-10
Status: proposed; playback capability has not been validated locally.

## Decision

Prefer mygo's native Go UI, with no application JavaScript layer. Allow Go
bindings to system media facilities. Keep the player behind an application-
owned interface; do not expose mygo or platform media types in business code.

This is conditional on `specs/0002-playback-spike/spec.md`. Do not claim that
generic URL playback demonstrates compatibility with target Bilibili flows.

## Observed upstream baseline

Inspected on 2026-11-10:

- Repository: https://github.com/egoist/mygo
- Observed HEAD: `e98410d0490bcaad7c81be2fc7283bfc1279e815`
- Its `go.mod` declares `go 1.27.3`.
- Native UI docs: https://mygo.egoist.dev/docs/ui/
- Video package: https://github.com/egoist/mygo/tree/e98410d0490bcaad7c81be2fc7283bfc1279e815/ui/video
- UI testing: https://mygo.egoist.dev/docs/ui/testing

These are research inputs, not evidence that the project builds or plays media.
The spike must choose and pin its own evaluated dependency revision.

## Questions to resolve

Can authorized target streams be played with the required request metadata,
separate audio/video if applicable, seek, cancellation, quality changes, and
cleanup on both target OSes? If not, compare a separate native playback adapter
before changing the UI choice or expanding the language scope.
