# Status

Updated 2026-11-17. Base: `master` at `84b7435` (PR #14).

This is the only file that carries current state. Update it in the same change
that moves the state. History lives in `reports/` and `git log`; agents do not
read reports by default.

## Product

| Area | State | Where |
| --- | --- | --- |
| Playback engine (libmpv, macOS) | Accepted (M1, 2026-11-13) | [ADR 0008](docs/adr/0008-default-libmpv.md), [report 0025](reports/0025-m1-acceptance.md) |
| Two-part fixture preview app, controls, fullscreen, developer bundle | Merged (PR #9). Owner reported all functions working on 2026-11-16 | [report 0028](reports/0028-m2-fixture-preview.md) |
| Rapid play/pause and seek fixes | Merged (PRs #12, #14) | [0029](reports/0029-playback-intent.md), [0030](reports/0030-seek-queue.md) |
| Playback code promoted out of `experiments/`; expiry-aware `app.Source` | In review on branch `refactor/playback-promotion` | [spec 0024](specs/0024-playback-promotion/spec.md) |
| QR login and Keychain session (P0) | Implemented and independently reviewed on branch `codex/p0-login-foundation`. Not on `master`, no PR. Real sign-in not done | `reports/0035-p0-persistent-login.md` on that branch |
| Real Bilibili content (feed, search, real video) | Not started | — |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | In progress: login on an unmerged branch |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Independently review and merge `refactor/playback-promotion` (full lane:
   shared contract change).
2. Rebase `codex/p0-login-foundation` onto the new `master` and open a PR.
   Expect conflicts: package moves to `internal/playback` and
   `internal/adapters/mpv`, the `app.New`/`Source` signatures, and the README.
3. Move `Source` resolution off the UI thread, with stale-result handling. This
   is required before any network-backed `Source`
   (see `internal/app/contracts.go`).
4. First real-content slice: signed-in feed or search → play one real video.
   This covers request signing, the play-URL endpoint, anti-bot errors and cookie
   refresh.
5. Write an ADR on libmpv distribution.

## Owner action queue

Batched per [owner actions](docs/workflow.md#owner-actions). Agents continue
unblocked work meanwhile.

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Authorize push and PR for `refactor/playback-promotion`, then merge after CI | Local commit with passing checks and independent review | 5 min | Rebasing the login branch | "Push and open the PR" |
| Real QR sign-in and restart restore on an unlocked Mac (LOG-AC-06) | Developer build and checklist on `codex/p0-login-foundation` | 10 min | P0 login acceptance | "Signed in and restored" or what failed |

Done 2026-11-17: the owner approved the spec 0024 policy changes (lanes,
evidence identity, adopted clauses, owner queue) and local commit authority.

## Follow-ups

- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).

## Blockers

- Real sign-in needs the owner's Mac awake and unlocked. Do not retry until the
  owner confirms.
