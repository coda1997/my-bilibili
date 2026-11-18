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
| QR login and Keychain session (P0) | Real scan, Keychain save/readback, service verification and same-binary restart restoration passed on 2026-11-17. Session retained; included in [the P0 client PR](https://github.com/coda1997/my-bilibili/pull/16) | [real login and restore](reports/0036-p0-login-restored.md) |
| Asynchronous detail/media loading | Implemented and independently approved at `371428b`; full race and fresh macOS full/source native suites pass (30 + 25 checks) | [0037](reports/0037-p0-async-source.md) |
| Native Chinese danmaku | Implemented and independently approved at cebd417; full race, native full/source/danmaku stages (30+25+30 checks) and independent Chinese pixel/timebase verification pass | [0038](reports/0038-p0-native-danmaku.md) |
| Real Bilibili content and local history/resume | Both real parts, native A/V, a quality change, visible danmaku, local history and restart into each saved part verified on macOS at `250c5d7`. Its three review findings were fixed and independently closed at `4443b69` using offline tests and 46 fixture-native checks | [Real-account evidence](reports/0039-p0-real-client.md), [repair and closure](reports/0040-p0-focused-repair.md), [spec 0027](specs/0027-p0-real-client/spec.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 macOS client | **Accepted 2026-11-18** by the owner at `4443b69`. [Acceptance record](reports/0041-p0-acceptance.md) · [integration PR #16](https://github.com/coda1997/my-bilibili/pull/16) |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

The accepted P0 [developer build](reports/0040-p0-focused-repair.md) is available
on the execution Mac. No further account check is pending for this milestone.
Portable libmpv, signing/notarization, Windows playback and release are separate work.

## Owner action queue

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| None for P0. | The owner authorized one PR and merge for the accepted client, including the login foundation. | — | — | — |

Done 2026-11-18: the owner unlocked the execution Mac and later authorized
one new bounded plan for the available second part. The original verification
and supplement are now fully used; both pairs reused the saved Keychain login.
The original reviewer closed OBS-002/003/004 at round 3/global 20 and requested changes. The owner then authorized one offline response and original-reviewer closure; all three new findings closed at round 4/global 21.

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

Corrected offline candidate: `4443b69ad2fa1f7daf2feef03b51a0ef68c3732a`. Its full and tagged race gates passed, along with 46 synthetic native client checks. The [repair report](reports/0040-p0-focused-repair.md) lists the controlled regressions and package identity. No real account actions were performed for this revision.

Earlier real-account candidate: `250c5d71cac58712084bc54d9247c4aeee8548d5`.
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
The preceding plan used repairs 6/6 and reviews 3/3 (global 20). Its request for changes led to one owner-authorized additional repair/verification and one closure, cumulative limits 7 and 4 (global 21). Completed offline run: 20261118T125821Z-84ff4504ca07, with the original decision in evidence/original-closure.md; repairs 7/7 and reviews 4/4 used. Remote issues 0/5. See [PR #16](https://github.com/coda1997/my-bilibili/pull/16) for the CI results, tested revisions and merge status.

Corrected developer ZIP: `.agent/p0-three-finding-repair/my-bilibili-macOS-arm64-dev-4443b69.zip`. Its normal executable matches the fresh build; only its fixture-only counterpart was run. The earlier real-tested ZIP remains under `.agent/p0-real-checked-final/`. The installed development libmpv
is required. Evidence and the binary are local artifacts, not part of a clone.
No real logout, history deletion or live-content screenshots were part of verification. The local development bundle is not a public release.
