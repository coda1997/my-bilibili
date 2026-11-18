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
| Real Bilibili content and local history/resume | Real restored-login search, playback, quality change, visible danmaku, progress and same-part restart passed on macOS at `250c5d7`. Alternate-part live playback and final independent review remain open | [spec 0027](specs/0027-p0-real-client/spec.md), [assignment](specs/0027-p0-real-client/assignment.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | Real play and restart/resume verified; supplementary alternate-part evidence and final independent review pending |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Both approved real launches passed. The target offered two parts, but the
   frozen plan selected part 1. Ask the owner for a supplementary pair of
   launches to exercise the other available part. Fresh public plans have
   been parsed offline; they keep the same target, binary and Keychain session.
2. If authorized, verify real part 1 → part 2 with native picture, A/V,
   danmaku and saved progress, then restart into the saved part. Stop after a
   failed launch. Leave the consumed original plan unchanged.
3. Attach the missing evidence and send the packet to the original independent
   reviewer for final round 3. Complete report 0039, README and architecture
   after review. Local developer build available now; no push, PR or release.

## Owner action queue

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Authorize one extra bounded live run for the second part | Same normal app and sanitized collector; parser-checked inputs and hashes in `.agent/p0-real-checked-final/proposed-second-part/` | Up to two launches of four minutes; no new scan planned | Live alternate part, then final review | “补测第二分集，继续 P0” |
| Authorize a PR for `codex/p0-login-foundation` when ready | Rebased branch and scoped evidence in the login handoff | 2 min | PR review and candidate CI | “Open the login PR” |

Done 2026-11-18: the owner unlocked the Mac. All authorized verifier repairs
are used. The history notice and target-tab readiness now pass their original
native checks. Both normal launches reused the stored session. Earlier policy
approval, local commit authority and PR #15 rebase retain their existing scope.

## Follow-ups

- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).
- LOG-ADV-001 was addressed by the account-pane integration at `cc039a8`: an
  unconfirmed save no longer claims no credentials were stored. Disposition
  closed without another review round or remote issue.

## Verification checkpoint and blockers

Exact source: `250c5d71cac58712084bc54d9247c4aeee8548d5`. Normal/e2e/driver were built from a clean checkout;
identities in `.agent/p0-real-checked-final/artifacts.json`. No CI for this
SHA or final independent approval is claimed.

| Claim | Actual evidence | Result and scope |
| --- | --- | --- |
| Offline verification | Current run `evidence/check-2504189964.log`, tagged race and checked-button red/green logs | Full and tagged race pass. The new regression first failed on all four native button states |
| Native playback / source / danmaku / full client | Each `native-*/report.json` in the final artifact directory | 30/25/30/46 pass. Includes three fullscreen cycles, search, synthetic part changes, deletion surviving quality change, a new visit and confirmed Clear. Clean exit throughout |
| Actual session, search, native quality and danmaku | Final `live-1/{result.json,receipts.jsonl}` | 20 search results; 2 parts, 3 quality choices; current-engine picture and A/V on two generations; visible danmaku. Saved 2125 ms before orderly exit |
| Actual restart/Continue | Final `live-2/{result.json,receipts.jsonl}` | Same part restored; picture and advancing time at 2208 ms; 2250 ms saved. Both processes exited 0 with no forced cleanup or remaining owners |
| Missing live evidence | Current run `evidence/real-loop.md` | Two permitted launches consumed. Cumulative 14/24 API, 1/2 bootstrap, 0/4 cover and 3/4 media opens. Actual alternate part has not been exercised |

Run `20261118T120915Z-30716ecf14b2` continues closed runs ending 27437f77126e, 20371a25d352
and a1ae44538d65. P0R-010 repairs 6/6; reviews 2/3 (final combined round
3/global 20 reserved); issues 0/5. Retained previous failures: slider 2,
history notice 1, later quality popup 1 and fullscreen callback 1. At 7e24796
the full client also failed once on search after resize, and its separately
isolated history run timed out on the checked tab. The four real navigation
buttons exposed checked, and the verifier now recognizes that signal. A
separate fixture observation confirmed different control identities before
and after resize, but attributing the single search failure to that timing
is an inference. Search passed unchanged on the final sweep. Original
findings await the assigned reviewer's formal disposition.

Local delivery: `.agent/p0-real-checked-final/my-bilibili.app` and the verified
`my-bilibili-macOS-arm64-dev-250c5d7.zip` contain the normal developer build.
The installed development libmpv is still required. Evidence, the staged review
packet and public supplement are ignored local artifacts, not remote attachments.
Both app processes exited; temporary keep-awake was stopped. No real logout,
history deletion, live-content screenshots, push or release was performed.
