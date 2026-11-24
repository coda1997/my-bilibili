# Status

Updated 2026-11-24. The accepted macOS P0 client is on `master`:
[PR #16](https://github.com/coda1997/my-bilibili/pull/16) was squash-merged as
[`ec4e275`](https://github.com/coda1997/my-bilibili/commit/ec4e27514e9021658205b1c0dc1ddb97b0992bac).
The earlier Ubuntu fullscreen fixture timeout on `ec4e275` was stabilized using
virtual time in [PR #17](https://github.com/coda1997/my-bilibili/commit/c6a9152363c29dc80cce881987e7786ac7f4e762)
(`c6a9152`), and `master` CI passed on all three platforms on `c6a9152` and
[`a3e8fe0`](https://github.com/coda1997/my-bilibili/commit/a3e8fe00d958e7025662e2570887eb7c1baa49d9).
See [CI evidence](#verification-and-availability).

This is the only file that carries current state. Update it in the same change
that moves the state. History lives in `reports/` and `git log`; agents do not
read reports by default. Exact source identity belongs to the assigned run,
review packet, or PR candidate; do not maintain an evergreen base SHA here.

## Product

| Area | State | Where |
| --- | --- | --- |
| Playback engine (libmpv, macOS) | Accepted (M1, 2026-11-13) | [ADR 0008](docs/adr/0008-default-libmpv.md), [report 0025](reports/0025-m1-acceptance.md) |
| Two-part fixture preview app, controls, fullscreen, developer bundle | Merged (PR #9). Owner reported all functions working on 2026-11-16 | [report 0028](reports/0028-m2-fixture-preview.md) |
| Rapid play/pause and seek fixes | Merged (PRs #12, #14) | [0029](reports/0029-playback-intent.md), [0030](reports/0030-seek-queue.md) |
| Playback code promoted out of `experiments/`; expiry-aware `app.Source` | Merged (PR #15, `34e6faf`) | [spec 0024](specs/0024-playback-promotion/spec.md) |
| P0 service probes, QR-host compatibility and empty-text danmaku handling | Merged with P0; the anonymous five-step probe passed | [0033](reports/0033-p0-qr-host-fix.md), [0034](reports/0034-p0-empty-danmaku.md) |
| QR login and Keychain session (P0) | Real scan, Keychain save/readback, service verification and same-binary restart restoration passed on 2026-11-17. The saved session was reused in the real playback checks | [real login and restore](reports/0036-p0-login-restored.md) |
| Asynchronous detail/media loading | Implemented and independently approved at `371428b`; full race and fresh macOS full/source native suites pass (30 + 25 checks) | [0037](reports/0037-p0-async-source.md) |
| Native Chinese danmaku | Implemented and independently approved at `cebd417`; full race, native full/source/danmaku stages (30+25+30 checks) and independent Chinese pixel/timebase verification pass | [0038](reports/0038-p0-native-danmaku.md) |
| Real Bilibili content and local history/resume | Both real parts, native A/V, a quality change, visible danmaku, local history and restart into each saved part verified on macOS at `250c5d7`. Its three review findings were fixed and independently closed at `4443b69` using offline tests and 46 fixture-native checks | [Real-account evidence](reports/0039-p0-real-client.md), [repair and closure](reports/0040-p0-focused-repair.md), [spec 0027](specs/0027-p0-real-client/spec.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows native playback and packaging | P2; not tested. The Windows Go check passed on the merge. | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 macOS client | **Accepted and merged.** The owner [accepted `4443b69`](reports/0041-p0-acceptance.md) on 2026-11-18; [PR #16](https://github.com/coda1997/my-bilibili/pull/16) landed on `master` at `ec4e275`. |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

- Review and approve the Phase 0 workflow-hygiene Draft PR (`specs/0030-phase0-workflow-hygiene`)
  after three-OS CI passes (modifies `internal/check/`, `AGENTS.md`, and `docs/pull-requests.md`,
  requiring separate human policy approval).
- Continue post-P0 desktop client slices and Phase 1 structured `cmd/dev` telemetry / write-scope enforcement.

The earlier Ubuntu `TestFullscreenCyclesRecheckBeforeNextEntry` fixture timeout on `ec4e275`
was resolved by virtual-time `testing/synctest` stabilization in `c6a9152` and verified green
on `master` across all three CI platforms (`44427348642`, `44429566237`).

## Owner action queue

No owner action or further account verification is pending for P0. Portable
libmpv distribution, signing/notarization, Windows playback and a public
release are separate work.

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

| Revision / scope | Result | Evidence |
| --- | --- | --- |
| `master` push at `a3e8fe0` | macOS and Ubuntu race checks and the Windows check all passed (including stabilized fullscreen fixtures from `c6a9152`). | [Master CI (`a3e8fe0`)](https://github.com/coda1997/my-bilibili/actions/runs/44429566237) · [Master CI (`c6a9152`)](https://github.com/coda1997/my-bilibili/actions/runs/44427348642) |
| PR #16 head `2d3fcd5` (CI tested merge `8e24962`) | macOS and Ubuntu race checks passed; the standard Windows check passed. | [PR CI](https://github.com/coda1997/my-bilibili/actions/runs/44318195874) |
| `master` push at `ec4e275` | macOS and Windows passed; Ubuntu failed in `TestFullscreenCyclesRecheckBeforeNextEntry` (later stabilized in `c6a9152`). | [Post-merge CI](https://github.com/coda1997/my-bilibili/actions/runs/44319474570) |
| Accepted product `4443b69` | Full and tagged Go race checks and 46 macOS fixture checks passed. The original reviewer closed all three findings. | [Repairs](reports/0040-p0-focused-repair.md) · [Owner acceptance](reports/0041-p0-acceptance.md) |
| Real account and playback | QR login and Keychain restore passed. At `250c5d7` both parts played with the saved session, quality switching and visible danmaku were observed, and each part resumed in a fresh process. | [Login](reports/0036-p0-login-restored.md) · [Playback](reports/0039-p0-real-client.md) |

The corrected developer ZIP, `.agent/p0-three-finding-repair/my-bilibili-macOS-arm64-dev-4443b69.zip`,
is local to the execution Mac and requires its installed libmpv. The [build
instructions](README.md#run-the-macos-developer-app) also apply to a fresh
bundle on that Mac. No public installer was released.
