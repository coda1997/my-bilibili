# Issue #10: preserve the last playback intent

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
