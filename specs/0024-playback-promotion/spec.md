# Promote playback out of experiments, add an expiry-aware Source, simplify process records

Spec ID: `0024-playback-promotion`
Status: implemented on branch `refactor/playback-promotion` from `master`
`84b7435`, at the owner's request on 2026-11-17. Lane: full (shared contract,
checks and workflow policy change). The owner approved the policy changes and
local commit authority on 2026-11-17. Independent code review is required
before merge.

## User scenario and goal

The shipping app imported `experiments/playback/contract` and
`experiments/playback/mpvprobe`, so the accepted playback engine lived in a
directory reserved for probes. `app.Source` was synchronous, took no context
and could not express that a signed media URL expires, so a cached input could
be reopened after expiry. Process records repeated current state in README,
architecture and stage-zero, with no lightweight path for small fixes.

## Scope and non-goals

In scope:

- Move `experiments/playback/contract` to `internal/playback` (package
  `playback`) and `experiments/playback/mpvprobe` to `internal/adapters/mpv`
  (package `mpv`), with no behavior change. Update every importer.
- Add an architecture test: product packages never import `experiments/`.
- Change `app.Source` to `Detail(ctx)` / `Resolve(ctx, part) (Resolution, error)`
  and `app.New(ctx, source, controller)`, with expiry-aware reuse in Session.
- Documentation: `STATUS.md`, read order, risk lanes, evidence identity and
  tables, owner action queue, adopting the two proposed clauses, a shorter
  README, architecture and stage-zero.

Non-goals: moving `experiments/playback/testdata` (its manifest records its
paths), asynchronous resolution, a network `Source`, changes to UI, the
Controller or the adapter's behavior, editing historical reports or specs.

## Boundaries and shared contracts

```go
type Source interface {
	Detail(context.Context) (catalog.Video, error)
	Resolve(context.Context, catalog.PartID) (Resolution, error)
}
type Resolution struct {
	Input     playback.Input
	ExpiresAt time.Time // zero: does not expire
}
const ResolutionMargin = 30 * time.Second
```

Session passes the construction context to `Detail` and the `Start` context to
`Resolve`. An input is usable while `ExpiresAt` is zero or `now` is before
`ExpiresAt - ResolutionMargin`. A resolution that is not usable when returned
fails as `Media` without opening. Reopening the accepted part (retry, replay
after end, reselecting a failed or ended current part) reuses the cached input
when usable and otherwise resolves again. A failed re-resolution keeps the
current engine. When the accepted part is also the selected part, the failure
becomes a selection error that `RetrySelection` repeats; when another part is
selected, its pending selection error and retry are left unchanged and the
failure is only returned.
Calls remain synchronous on the UI thread; a network-backed source must wait
for asynchronous resolution with stale-result handling.

## Acceptance criteria

### PRO-AC-01

`go list -deps` of `cmd/my-bilibili` and `internal/...`, with and without the
`e2e` tag, contains no `my-bilibili/experiments/` package. A test enforces it
and fails when a product package imports an experiment.

### PRO-AC-02

The moved packages keep their tests and behavior; `go run ./cmd/dev check
--race` passes, and the `e2e` and `nativecleanup` tagged builds vet cleanly.

### PRO-AC-03

Session tests show: context propagation to `Detail` and `Resolve`; reuse of an
unexpired input without resolving; resolving again at exactly
`ExpiresAt - ResolutionMargin`; rejection of an already-expired resolution
without opening; a failed re-resolution keeping the engine and recovering
through `RetrySelection`; and a playback retry that re-resolves (successfully
or not) leaving a pending selection of another part intact. Fixture resolutions do not expire.

### PRO-AC-04

`STATUS.md` exists and is the first item in the AGENTS.md read order. Workflow
documents define fast and full lanes, commit-SHA evidence identity with CI run
URLs, the evidence table, the owner action queue, and mark read-only requests
and reviewer succession as adopted; links to the old anchors are updated.

## Verification

Run `go run ./cmd/dev check --race`, `go vet -tags e2e ./...`,
`go vet -tags nativecleanup ./internal/adapters/mpv`, and the import guard with
a temporary experiment import (it must fail). Independent code review of the
candidate commit. No native or platform run is claimed.

## Risks

- `codex/p0-login-foundation` conflicts with the moves and signatures; rebase.
- Policy text changes need the owner's review before use as trusted policy.
