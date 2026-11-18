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
| Real Bilibili content and local history/resume | Implemented with navigation-verifier repair at `7e24796`; full and tagged race pass. Current native verification is paused because the execution Mac is locked and the owner cannot currently unlock it. Real-service loop and final independent review remain pending | [spec 0027](specs/0027-p0-real-client/spec.md), [assignment](specs/0027-p0-real-client/assignment.md) |
| libmpv distribution (bundling, licence, signing) | Not designed; a development library path is hardcoded | — |
| Windows | P2, not tested | [ADR 0005](docs/adr/0005-platform-priority.md) |

## Milestones

| Milestone | State |
| --- | --- |
| M0 development foundation | Accepted 2026-11-10 |
| M1 macOS playback feasibility | Accepted 2026-11-13 |
| M2 one complete development loop | Merged (PR #9). Owner functional feedback recorded; formal milestone acceptance not recorded |
| M3 one issue-to-PR loop | Accepted 2026-11-17 |
| P0 client | In progress; navigation repair implemented and checked, native/live verification paused until the execution Mac is unlocked |

Exit criteria are in [stage zero](docs/stage-zero.md).

## Next actions

1. Wait for the owner to confirm that Dada's execution MacBook Air is awake
   and unlocked. The host reported `screen_locked`; on 2026-11-18 the owner
   answered "暂时无法解锁". Do not retry native window operations or private
   handoff before that confirmation. No repair-allowance question is pending.
2. After confirmation, start a continuation run with the same frozen spec and
   counters, recheck actual accessibility and keep the Mac awake. Reuse the
   exact `7e24796` artifacts for a fresh native full attempt, then source/
   danmaku/client stages. Preserve the first failure and all strict deadlines.
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

Batched per [owner actions](docs/workflow.md#owner-actions). The Mac was
previously unlocked; a new locked-screen result was observed on 2026-11-18,
and the owner cannot currently unlock it.

| Action | Prepared | Time | Unblocks | Report back |
| --- | --- | --- | --- | --- |
| Authorize a PR for `codex/p0-login-foundation` when ready | Rebased branch and scoped evidence in the login handoff | 2 min | PR review and candidate CI | "Open the login PR" |
| Wake and unlock Dada's MacBook Air, then keep it unlocked during verification | Navigation repair, full/tagged race and exact normal/e2e/driver builds ready at `7e24796`; login retained | 1 min, then during native checks | Remaining native checks, real-service loop and final review | "已解锁，继续 P0" |

Done 2026-11-17: the owner approved the spec 0024 policy changes (lanes,
evidence identity, adopted clauses, owner queue) and local commit authority.
PR #15 is merged; the owner requested this rebase and previously authorized
publication of the P0 branch. No P0 PR, merge or release has been performed.

Done 2026-11-18: the owner added one scoped history-notice repair/verification
iteration. `1082c36` implements it; regression/race checks and actual native
history-notice/recovery verification pass. That additional iteration is used.
The owner then authorized at most two more navigation-readiness repair/
verification iterations. `7e24796` implements the shared selected-frame wait
and passes the offline gates; one of those two iterations remains available
within the approved navigation-only scope.

## Follow-ups

- Import guard covers darwin/arm64 non-test imports only; extend it to the
  other CI targets and test files (review advisory, spec 0024).
- Add expiry tests for `Toggle` from Ended and reselecting the current part
  (review advisory, spec 0024).
- LOG-ADV-001 was addressed by the account-pane integration at `cc039a8`: an
  unconfirmed save no longer claims no credentials were stored. Disposition
  closed without another review round or remote issue.

## Verification checkpoint and blockers

Exact implementation candidate: `7e24796ea40459a2071dd3bfbd0c5d2d1e1c1bd8`.
Recorded full race, relevant e2e-tagged race and targeted navigation/notice
tests pass. Two compiling controls discriminate the navigation wait and its
shared two-second budget. Normal/e2e apps and the driver were built from a
clean exact checkout; all prior assertions and tests remain intact.

The first current-candidate native full invocation passes 11 checks, then
times out on fullscreen-entry correspondence: one native request, no entry
completion callback. Cleanup is graceful without force. A subsequent AX-only
host observation explicitly reports `screen_locked`. The current lock is
observed; its causal relation to the earlier timeout is inferred. Fullscreen
code is unchanged from the prior passing candidate. No new native retry or
repair has been attempted. Source/danmaku/client are not yet run at `7e24796`.

Prior evidence at `1082c36` remains 30/25/30 native prerequisite passes plus
44 client passes, including P0R-OBS-002's repaired notice, in-use recovery,
minimized close, synthetic restart/resume and confirmed deletion. Its later
quality-popup timeout is P0R-OBS-003 (R1 class 5). The new repair observes the
actual selected tab before using remounted player controls, addressing the
source-backed missing-frame wait; its native resolution remains unverified.
Deletion surviving quality replacement and later new-visit/Clear checks are
still pending, as are the real two-launch loop and final independent review.

Runs `20261117T152122Z-27437f77126e` (event 23),
`20261118T013405Z-20371a25d352` (event 7), and
`20261118T020849Z-a1ae44538d65` (event 7) are closed `needs-human` and must
not be appended to. P0R-010 corrections are 5 used / 6 authorized; one
navigation-only repair iteration remains. Retained failures: slider 2,
history notice 1, later quality popup 1, current fullscreen entry 1. Formal
reviews are 2/3 used, leaving final combined round 3/global 20; issues stay
0/5. Original reviewer remains available. Preserve counters on continuation.

Current local-only artifacts/evidence are in `.agent/p0-real-navigation-final/`,
including `artifacts.json`, first native report and `locked-desktop-checkpoint.md`;
the run's `evidence/pause.md` records the owner's unavailable-unlock answer.
Earlier `.agent/p0-real-continued-final/` and `.agent/p0-real-final/` evidence
remain unchanged. All owned test processes exited; temporary keep-awake was
stopped for the pause. Actual login remains retained and no real-service plan
request has been made. Logs are not backed up by this status file. No current
candidate CI, independent approval or full-P0 completion is claimed.
