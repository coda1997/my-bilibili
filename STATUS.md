# Status

Updated 2026-11-17. Branch: `codex/p0-async-source`. Base: `master` at
`34e6faf` (merged PR #15).

This is the only file that carries current state. Update it in the same change
that moves the state. History lives in `reports/` and `git log`; agents do not
read reports by default.

## Product

| Area | State | Where |
| --- | --- | --- |
| Playback engine (libmpv, macOS) | Accepted (M1, 2026-11-13) | [ADR 0008](docs/adr/0008-default-libmpv.md), [report 0025](reports/0025-m1-acceptance.md) |
| Two-part fixture preview app, controls, fullscreen, developer bundle | Merged (PR #9). Owner reported all functions working on 2026-11-16 | [report 0028](reports/0028-m2-fixture-preview.md) |
| Rapid play/pause and seek fixes | Merged (PRs #12, #14) | [0029](reports/0029-playback-intent.md), [0030](reports/0030-seek-queue.md) |
| Playback code promoted out of `experiments/`; expiry-aware `app.Source` | Merged (PR #15, `34e6faf`) | [spec 0024](specs/0024-playback-promotion/spec.md) |
| P0 service probes, QR-host compatibility and empty-text danmaku handling | Scoped implementation and independent review complete before rebase; anonymous five-step probe passed | [0033](reports/0033-p0-qr-host-fix.md), [0034](reports/0034-p0-empty-danmaku.md) |
| QR login and Keychain session (P0) | Real scan, Keychain save/readback, service verification and same-binary restart restoration passed on 2026-11-17. Session retained; branch not merged, no PR | [real login and restore](reports/0036-p0-login-restored.md) |
| Asynchronous detail/media loading | Implemented and independently approved at `371428b`; full race and fresh macOS full/source native suites pass (30 + 25 checks) | [0037](reports/0037-p0-async-source.md) |
| Native Chinese danmaku | Architecture proposal independently approved; implementation and glyph/timebase evidence next | [P0 scope draft](specs/0004-client-p0/spec.md) |
| Real Bilibili content and local history/resume | Complete integration proposal prepared; awaits native overlay prerequisite and independent architecture review. Personalized feed remains outside P0 | [P0 scope draft](specs/0004-client-p0/spec.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | In progress; real login/restoration complete, overlay and integrated client pending |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Continue native Chinese danmaku overlay/timebase and full client integration.
   Real sign-in and same-binary restart restoration now pass; preserve the
   Keychain session without real Logout/Delete. See the
   [login completion](reports/0036-p0-login-restored.md).
2. Open a PR for `codex/p0-login-foundation` when authorized. This branch is
   rebased onto merged PR #15; only README conflicted, and its P0 checkpoint
   now lives here with the upstream STATUS-first navigation retained.
3. Freeze and implement the independently approved native Chinese overlay
   proposal. Async detail/resolution and its exact-candidate review/native
   checks are complete; see [0037](reports/0037-p0-async-source.md).
4. Review the prepared real-client contract, then connect search/link/details,
   available-quality playback, segmented danmaku and local history/resume in
   one normal app, reusing the preserved login.
5. Write an ADR on libmpv distribution.

## Owner action queue

Batched per [owner actions](docs/workflow.md#owner-actions). The owner resumed
P0 verification and confirmed the execution Mac unlocked on 2026-11-17.

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Authorize a PR for `codex/p0-login-foundation` when ready | Rebased branch and scoped evidence in the login handoff | 2 min | PR review and candidate CI | "Open the login PR" |
| Keep the execution Mac unlocked for native verification | Login and restart restore complete; overlay/fixture scenarios will follow reviewed contracts | During native checks | Native P0 verification | Report any native authorization prompt or failure |

Done 2026-11-17: the owner approved the spec 0024 policy changes (lanes,
evidence identity, adopted clauses, owner queue) and local commit authority.
PR #15 is merged; the owner requested this rebase and previously authorized
publication of the P0 branch. No P0 PR, merge or release has been performed.

## Follow-ups

- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).
- LOG-ADV-001: clarify uncertain persistence in the save-failure wording during
  the next account UI copy adjustment. Nonblocking; retained in the login
  handoff, with no remote issue for this wording-only advisory.

## Blockers

- No remaining real-login or async-loading blocker. Native overlay evidence
  and reviewed real-content/history integration remain prerequisites for full P0.
