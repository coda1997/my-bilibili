# Status

Updated 2026-11-18. Branch: `codex/p0-real-client`. Base: `master` at
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
| Native Chinese danmaku | Implemented and independently approved at cebd417; full race, native full/source/danmaku stages (30+25+30 checks) and independent Chinese pixel/timebase verification pass | [0038](reports/0038-p0-native-danmaku.md) |
| Real Bilibili content and local history/resume | Both real parts, native A/V, a quality change, visible danmaku, local history and restart into each saved part verified on macOS at `250c5d7`. Independent review requested changes: early quality switching, history title persistence and missing CID regression | [P0 evidence](reports/0039-p0-real-client.md), [spec 0027](specs/0027-p0-real-client/spec.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | Real playback and both restarts verified; three open findings in the completed independent review. P0 has not been accepted |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Owner: decide on the [scoped repair proposal](reports/0039-p0-real-client.md#proposed-decision). One repair/verification iteration and one independent closure review would cover P0R-CODE-001 (pending seek), P0R-HISTORY-001 (blank truncated title) and P0R-AT-001 (CID regression). The existing allowance is exhausted. No further repair or real launch has started.
2. The normal developer bundle is available on the execution Mac. Portable
   libmpv, signing/notarization, Windows playback and release are separate work.

## Owner action queue

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Decide on the three P0 review findings | [Reproductions and proposed scope](reports/0039-p0-real-client.md#three-open-findings) | 2 min | Scoped offline corrections and independent closure; no new login needed | “修这三项，追加一轮修复验证和一次定向复核” |
| Authorize a PR for `codex/p0-login-foundation` when ready | Rebased branch and scoped evidence in the login handoff | 2 min | PR review and candidate CI | “Open the login PR” |

Done 2026-11-18: the owner unlocked the execution Mac and later authorized
one new bounded plan for the available second part. The original verification
and supplement are now fully used; both pairs reused the saved Keychain login.
The original reviewer closed OBS-002/003/004 at round 3/global 20 and requested changes for the three new findings.

## Follow-ups

- P0R-OBS-001/resize-search and the unrelated slider setup failure in the optional mutated run remain local reliability observations. For P0R-AT-001 and the earlier ASY-CODE-001, the local inventory proposes deriving control counts from actual outcomes (no change to trusted checks).
- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).
- LOG-ADV-001 was addressed by the account-pane integration at `cc039a8`: an
  unconfirmed save no longer claims no credentials were stored. Disposition
  closed without another review round or remote issue.

## Verification and availability

Exact runtime candidate: `250c5d71cac58712084bc54d9247c4aeee8548d5`.
The [P0 evidence report](reports/0039-p0-real-client.md) maps the ten acceptance
criteria to their checks and original files. The full and tagged race gates
passed. Four Mac native reports contain 30/25/30/46 passing checks. In the
original normal pair, a quality change kept part 1, which resumed at 2.208 s.
In the supplement, part 2 showed a comment at 11.708 s, saved at 11.750 s and
resumed in another process at 11.833 s. Both pairs exited normally. Counts per
plan (API/bootstrap/covers/media opens): 14/1/0/3 and 15/1/0/3, respectively.

Run `20261118T122807Z-2217126d5dfb` holds the supplement, independent artifact audit and original final review (`evidence/final-code-native-review.md`).
Its closed predecessor `20261118T120915Z-30716ecf14b2` holds the final repair,
full/tagged race evidence and first pair summary. Earlier records preserve
slider, notice, popup, fullscreen, checked-tab and resize/search failures.
P0R-010 repairs 6/6, formal review 3/3 (global 20), remote issues 0/5. The final technical decision is `request changes`; continuation needs an owner decision. No current-candidate CI run or release.

Developer ZIP: `.agent/p0-real-checked-final/my-bilibili-macOS-arm64-dev-250c5d7.zip`.
Its normal executable matches the tested one. The installed development libmpv
is required. Evidence and the binary are local artifacts, not part of a clone.
No real logout, history deletion, live-content screenshots, push or release.
