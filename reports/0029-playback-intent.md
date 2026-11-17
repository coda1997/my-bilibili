# Issue #10: preserve the last playback intent

## Owner acceptance, merge and closure — 2026-11-17

Following the proposed next step to accept PR #12, merge it, close #10 and
record M3 acceptance, the owner replied: “Yes, I approve this PR. and I think
you can merge it, close the issue, and update the status as well.” This is
recorded as acceptance of the scoped manual M3 issue-to-PR loop and authority
for the requested merge, closure and status publication. It grants no authority
to start #11, merge unrelated work, change policy or release a package.

The controller rechecked the live PR head, all three successful checks, merge
eligibility, public repository identity and protected-branch settings. No
review findings were open. Copilot's earlier static-only review reported no
open issues and asked for final CI/candidate confirmation; those checks had
subsequently completed and their identities were verified. The exact approved
head `a8be7810a87eb2a1e0345e21982b5d68e25d5240` was marked ready and squash
merged without bypassing protection or changing settings.

- [PR #12](https://github.com/coda1997/my-bilibili/pull/12): MERGED at
  `2026-11-17T02:33:47Z`, commit `15e353089408d369ee740382f41f8e129ada5707`.
- [Issue #10](https://github.com/coda1997/my-bilibili/issues/10): CLOSED,
  reason COMPLETED, at `2026-11-17T02:34:16Z`.
- [Final PR CI run 44002412720](https://github.com/coda1997/my-bilibili/actions/runs/44002412720):
  Linux/macOS race and Windows ordinary checks PASS. Tested merge
  `167ba9c3f9e4697adbcf0a67b814d0b4a0e54077` has the same tree as the approved
  head; downloaded manifests, input/log hashes and successful outcomes were
  verified for all three platforms. This is PR-candidate evidence, not a claim
  about later status-only commits or a new native playback experiment.

Independent architecture/code review and the old-source failing controls remain
the technical basis recorded below. Counts stay architecture 1/code 1/product
fix 1, failed repairs 0, issue filing budget 2/5 with no new issues. The original
implementation run `20261117T021320Z-8c1485126a67` is finished and unchanged.
Status-only continuation run `20261117T023419Z-2ca8c00d02e9` records the owner
decision, remote receipts and documentation publication separately. Its write
scope is README, stage-zero and this report under the existing INTENT-002
handoff task; no product source, tests, frozen inputs or policies change.

M3 is completed and accepted for the demonstrated manual loop. No automated
event handler or scheduler was introduced; issue ownership was serialized and
bot-generated events did not initiate new work. This does not claim automatic
orchestration, native playback revalidation, public release, or acceptance of
unrelated outstanding work. Next candidate: existing #11, still open and not
started here. No systemic follow-up warranted by this status-only continuation.

The sections below preserve earlier publication and prepublication states.
Their then-pending permissions/results are historical, superseded by this
acceptance record; the original reviews and closed-run events are not rewritten.

## Publication continuation — 2026-11-17

After the local handoff below, the owner explicitly authorized commit, push,
Draft PR creation and new CI verification. Initial publication commit
`271b4fd68cc40636acb4d18b5090c093b8464119` matches the final local manifest
`05c0376d1171e4398eddae5ac7174bc8d6b81c98a686a3c5a40e09e34e802441`.
Draft [PR #12](https://github.com/coda1997/my-bilibili/pull/12) targets `master`
in the verified public repository `coda1997/my-bilibili`. Receipt confirmed
the intended branch/head, draft/open state and still-open issue #10.

This publication follow-up changes documentation only; the independently
reviewed implementation, tests and frozen spec/tasks remain byte-identical.
See the PR for the latest head, tested merge revision and three-platform CI
results; the controller run retains their receipts. There is no claim of CI
success before its completion. Remote merge, issue closure and M3 milestone
acceptance are not authorized. Counts stay architecture 1/code 1/fix 1, with
no new product fix, review round or issue caused by publication.

## Preserved local handoff before publication

Date: 2026-11-17. Run: `20261117T021320Z-8c1485126a67`.
Branch: `codex/issue-10-playback-intent`. Base:
`ff27f1db5946eb9ebccbee8c3548a971dd112e85`. The repair is local and uncommitted;
preexisting `.claude/` is preserved and excluded.

## Result and observed cause

[Issue #10](https://github.com/coda1997/my-bilibili/issues/10), M2-EXT-001:
rapid opposite Play/Pause or Toggle actions now forward both changed intents
even while sampled playback state still matches the second request. Unchanged
intent and Loading suppression remain. Only `internal/app/session.go` and its
tests change product behavior; no native, UI, queue, timer or interface changes.
The owner selected the reproduced issue and requested continuation.

Observed vs inferred: deferred command consumption reproduces the lost second
command in both directions, through explicit controls and Toggle. The independent
reviewer separately reproduces Controller sample lag using an immediate engine.
The native adapter source establishes asynchronous enqueue and sampled state;
native stress behavior was not measured. Contract relied on: the existing M2
application intent/readiness/lifetime contract and the independently approved
[spec 0017](../specs/0017-playback-intent/spec.md). Samples are observations,
not acknowledgements that superseding intent can be discarded.

## Exact identity and independent review

Reviewed code candidate: manifest SHA-256
`7394a91c3e6be02ee8342eb96288eebc2d66ea05f4af04e2b4edda1d08177def`;
archive SHA-256
`e481e93d8fac2d71df29e9288dca64d5ccf16be12927b55e1ea237253d360a00`.
Both are in the run's `evidence/code-candidate/`, covering 277 files.
Spec SHA-256: `e7347235cc52682fe8522e155bb89dfbc387cf3874e3d76257a1970ef23a479b`.
Tasks SHA-256: `a8d35210428cdf93a82f7c80fb43e39fb0441e44938cfe55a091e0126a3fa499`.

Reviewer `/root/intent_review_readable` is independent of author `/root`.
Architecture round 1 approved the scope and independently confirmed the original
M2-EXT-001 as BLOCKING class 1. The first attempted dispatch received unreadable
messages and performed no review; it produced no finding or disposition.

Code review round 1 **approved** the exact candidate, with no new findings.
M2-EXT-001: **not BLOCKING** on that candidate: the independent four-case
sample-lag probe passes, and restoring original Session makes all six maintained
regression cases fail. The baseline class-1 ruling remains preserved. The reviewer
independently ran app/controller race tests and inspected the controller's full
six-step check; no second full check is claimed. This fresh verification does
not close Copilot's original thread or the remote issue.

Final handoff/navigation documentation is added after code review. The final
packaging manifest/archive in `evidence/final-candidate/` identifies those
documentation-only additions; all reviewed source, tests and frozen spec/task
bytes are checked unchanged. Code approval does not imply review of later prose.

## Acceptance and verification

| Criterion | Local macOS evidence, pinned Go 1.27.3 | Result |
| --- | --- | --- |
| INTENT-AC-01 | Four initial-state × explicit/Toggle cases, deferred consumption, final state and same live engine/generation | PASS |
| INTENT-AC-01 discrimination | Six new cases fail before repair and when only original Session is restored by overlay | Expected FAIL |
| INTENT-AC-02 | Identical intents do not duplicate commands; both opposite-command errors propagate sanitized failure and dispose once | PASS |
| INTENT-AC-02 preservation | Existing loading/readiness/replay/replacement/cancellation/close suite | PASS |
| INTENT-AC-03 | Full app race suite; full repository `check --run 20261117T021320Z-8c1485126a67 --race`, all six steps | PASS |
| INTENT-AC-03 review/handoff | Independent candidate disposition above; this report and linked PR draft | Recorded |

Controller evidence: `before-fix.log`, `app-race.log`, `original-control.log`,
`full-race.log` in the run's `evidence/`. Architecture report/probes are in
`evidence/planning/`; code review and independent logs are in `evidence/review/`.
These are local-only, not backed up by Git or remotely accessible attachments.
The maintained Go regression tests make the behavior reproducible.

No new native UI/playback, package, Linux/Windows runtime or remote CI result is
claimed. PR #9's green three-platform CI and merge were verified during selection
and apply to the baseline only. No check, CI or workflow policy is changed; no
new human policy approval or public release is inferred.

## Counts, inventory and remaining work

R1–R9: architecture 1 review/0 fixes; code 1 review; product fix iteration 1,
failed repairs 0. Old-code failures are expected controls. Retain issue filing
budget 2/5 from #10/#11 registration, 0 new in this continuation. No automatic
repair/review loop remains active.

Found, not done: [#11](https://github.com/coda1997/my-bilibili/issues/11), seek
queue saturation, stays at its existing destination outside this repair. No
new unrelated defect or systemic chain was established. No systemic follow-up
warranted. No new issue, comment or remote-thread closure was performed.

## Next action

Obtain explicit owner authority to commit, push and create a Draft PR using
[the prepared body](0029-playback-intent-pr.md). Then bind required remote CI
to the new PR head/tested merge candidate before proposing acceptance or merge.
Merge and remote issue closure remain separate decisions. Revert the scoped
repair to roll back; there is no data migration.

This owner-invoked M3 trial has one controller/run for #10; no competing local
run existed at creation. There is no bot/event scheduler or recursive trigger
path. Full M3 issue-to-PR completion and milestone acceptance remain pending
publication and owner disposition; this repair does not waive M2 gates.
