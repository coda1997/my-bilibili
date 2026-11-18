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
| Real Bilibili content and local history/resume | Implemented at `1082c36`; full and tagged race pass. Native full/source/danmaku pass; client passes history recovery and restart, then stops at a later quality-menu check. Real-service loop and final independent review remain pending | [spec 0027](specs/0027-p0-real-client/spec.md), [assignment](specs/0027-p0-real-client/assignment.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | In progress; implementation integrated, verification paused pending the scoped navigation-readiness extension |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Await the owner's pending decision on at most two additional scoped repair/
   verification iterations for P0R-OBS-003. The prepared proposal waits for the
   target tab's rendered selected state before using remounted controls. Changes
   are limited to `cmd/preview-smoke/client_controls.go` and corresponding tests.
   It preserves the native deadline, quality, history and cleanup requirements.
2. If authorized, start a continuation run with the same frozen spec and
   preserved counters, apply that repair, check the exact committed candidate,
   and complete the required native stages. Do not retry under the closed run.
3. Use the identical final normal executable for the prepared bounded
   two-launch real-service verification: restored login, search/details,
   playback, danmaku, saved progress and restart/resume. Preserve the Keychain
   session; actual login/restoration already passed in
   [0036](reports/0036-p0-login-restored.md).
4. Submit complete exact-candidate evidence to the original independent reviewer
   for the remaining final round, then deliver report 0039 and the developer
   application. Full P0 has not yet passed.
5. PR publication and libmpv distribution remain separate follow-ups. No new
   push, PR, merge, signing or release is authorized by this continuation.

## Owner action queue

Batched per [owner actions](docs/workflow.md#owner-actions). The owner resumed
P0 verification and confirmed the execution Mac unlocked on 2026-11-17.

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Allow at most two additional scoped navigation-readiness repair/verification iterations for P0R-OBS-003 | Reproduced missing navigation-completion wait and unapplied shared-tab proposal; original native failure preserved | 1 min | Complete client verification, then real-service loop and final review | Answer the pending question: "追加最多 2 轮，继续完成 P0" or pause |
| Authorize a PR for `codex/p0-login-foundation` when ready | Rebased branch and scoped evidence in the login handoff | 2 min | PR review and candidate CI | "Open the login PR" |
| Keep the execution Mac unlocked for native verification | Login/restore and 85 current-candidate fixture checks complete; remaining client and live scenarios follow after the scoped continuation | During native checks | Native P0 verification | Report any native authorization prompt or failure |

Done 2026-11-17: the owner approved the spec 0024 policy changes (lanes,
evidence identity, adopted clauses, owner queue) and local commit authority.
PR #15 is merged; the owner requested this rebase and previously authorized
publication of the P0 branch. No P0 PR, merge or release has been performed.

Done 2026-11-18: the owner added one scoped history-notice repair/verification
iteration. `1082c36` implements it; regression/race checks and actual native
history-notice/recovery verification pass. That additional iteration is used.

## Follow-ups

- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).
- LOG-ADV-001 was addressed by the account-pane integration at `cc039a8`: an
  unconfirmed save no longer claims no credentials were stored. Disposition
  closed without another review round or remote issue.

## Verification checkpoint and blockers

Exact implementation candidate: `1082c365249a2284f3823c0992da3b1ac019ffde`.
The recorded full race check and relevant e2e-tagged race suites pass. Native
full/source/danmaku stages pass 30/25/30 checks. The client stage passes 44
checks, including in-use history notice/recovery, minimized close, saved
progress, fresh-process same-part resume and confirmed deletion. P0R-OBS-002's
native notice gate now passes. All scene processes close with zero owned
resources and no forced cleanup.

P0R-OBS-003 is the new required-verification gap (R1 class 5): the final
history-restart scene times out waiting for quality-1 after switching back
to the player. Deletion surviving that quality change and a later new visit/
Clear remain unverified. The real-service plan remains unconsumed.

Observed: `clientAction` returns on the automation acknowledgment without
observing the selected tab. The pinned automation protocol reads the last
rendered frame; tab changes remount player controls with new identities.
A scratch queued-action/next-frame regression reproduces the missing wait.
Inferred: this gap explains the native popup timeout; the original raw trees
were not retained, so actual re-verification is still required. One shared
navigation-completion proposal addresses this second observed frame-readiness
gap (R8), rather than separate per-tab fixes. It has not been applied.

Original run `20261117T152122Z-27437f77126e` remains closed `needs-human` at
event 23; authorized continuation `20261118T013405Z-20371a25d352` closed
`needs-human` at event 7 after the new native failure. P0R-010
corrections are 4/4 consumed (three original plus one explicitly added).
Retained failure counts: slider 2, history notice 1, later quality popup 1.
Formal reviews remain 2/3 used, leaving final combined round 3/global 20;
issue budget remains 0/5. The original reviewer remains available. Do not
reset counters or retry the failed native stage without the scoped extension.

Local-only current evidence is in `.agent/p0-real-continued-final/`: exact
artifacts, native reports, `native-client-failure.md`, scratch diagnostic and
`navigation-readiness-proposal.patch`. Earlier `.agent/p0-real-final/` evidence
and closed run records remain unchanged. Logs and worker evidence are not
backed up by this status file. No candidate CI, final independent approval
or full-P0 completion is claimed.
