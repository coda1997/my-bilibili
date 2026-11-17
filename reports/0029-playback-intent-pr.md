# Draft PR: preserve the last rapid play/pause action

Published as Draft [PR #12](https://github.com/coda1997/my-bilibili/pull/12)
after explicit owner authority. The PR contains current publication/CI identity;
the text below preserves the local prepublication draft. Related issue: #10
(M2-EXT-001). No merge or remote issue closure is authorized.

## Why and scope

Rapid Play → Pause while paused could send only Play, leaving playback running
despite the last request to pause. The reverse sequence could remain paused.
The sampled state lags accepted commands, so it cannot safely suppress a changed
intent. Session now suppresses only Loading or unchanged intent; the existing
initialization, errors, replay and lifecycle behavior remains intact.

- Spec/tasks: 0017-playback-intent, INTENT-001/002.
- Product changes: `internal/app/session.go` and its tests only. No UI, native
  engine, queue, timeout, dependency, CI or policy change; #11 is deferred.
- Base: `ff27f1db5946eb9ebccbee8c3548a971dd112e85`. Uncommitted code candidate
  manifest: `7394a91c3e6be02ee8342eb96288eebc2d66ea05f4af04e2b4edda1d08177def`.
- Observed vs inferred: controlled delayed consumption and an independent
  Controller-sampling probe reproduce both directions. Native stress behavior
  was not measured. Contract relied on: M2 application intent/readiness/lifetime
  semantics and the reviewed spec 0017.

## Acceptance and verification

| AC | Check | Actual result |
| --- | --- | --- |
| INTENT-AC-01 | Four paused/playing × explicit/Toggle delayed-consumer cases, final state and stable engine/generation | PASS |
| INTENT-AC-01 | Original Session restored with Go overlay | All six new cases fail as expected |
| INTENT-AC-02 | Duplicate intent, opposite-command failure disposal, existing loading/readiness and lifetime tests | PASS |
| INTENT-AC-03 | Full app race suite; full repository `go run ./cmd/dev check --race` | PASS, Go 1.27.3 darwin/arm64 |
| INTENT-AC-03 | Independent exact-candidate review | APPROVE |

Independent app/controller race tests and a separate four-case sample-lag probe
also pass. New remote CI/head/tested merge SHA: **pending publication**. Native
playback, new packaging and Linux/Windows runtime: not-tested. Baseline PR #9 CI
does not validate this changed code. No public release is produced.

Evidence: [durable handoff](0029-playback-intent.md). Full local artifacts live
under run `20261117T021320Z-8c1485126a67`, not in remote attachments or Git.
Retain them before removing this checkout. The maintained regressions can be
rerun using `go test -count=1 -race ./internal/app`. Final documentation-only
packaging is identified separately from the reviewed code in the handoff.

## Independent review

Reviewer `/root/intent_review_readable` is independent of author `/root`.
Architecture round 1 and code round 1 approved; product fix iteration 1,
failed repairs 0. Review used spec SHA
`e7347235cc52682fe8522e155bb89dfbc387cf3874e3d76257a1970ef23a479b` and the
exact code manifest above, covering correctness, async intent, errors and tests.

M2-EXT-001: **not BLOCKING** on the repaired candidate; independent final-state
probes pass and old-source controls fail. No new findings. The baseline BLOCKING
class-1 ruling is preserved; no external review-thread closure is claimed.
No human policy approval or milestone acceptance is inferred.

## Risks and continuation

Correct forwarding increases previously dropped command traffic. Existing seek
queue overload #11 remains out of scope and tracked there; no new issue or
speculative follow-up is filed. Retain registration budget 2/5, new issues 0.
No systemic follow-up warranted. Rollback is the scoped patch; no data migration.

Commit/push/Draft PR creation requires owner authority. Keep the PR draft while
new candidate CI is pending. Merge, remote issue closure and full M3 milestone
acceptance remain separate decisions.
