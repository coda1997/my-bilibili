# Issue #11: keep rapid seeking from interrupting playback

Date: 2026-11-17. Controller run: 20261117T025128Z-5568fd01dc4f.
[PR #14](https://github.com/coda1997/my-bilibili/pull/14) is the live publication,
CI and merge record; it closes [issue #11](https://github.com/coda1997/my-bilibili/issues/11).
This report records the independently approved local candidate before final PR CI.
The PR description retains final-head/tested-merge CI identities and merge is
conditional on those checks. No CI/merge success is pre-filled here.

## Authority, scope and result

The owner requested completion of the active issue, PR publication, passing CI
and merge if no problems. Remote identity was verified as public
coda1997/my-bilibili, default master. Existing branch protection remains unchanged:
PR-based linear history, administrator enforcement and squash-only merging.
No protection bypass, unattended merge setting or package release is authorized.

The only open issue was #11 / M2-EXT-002. Base is
a11ca09a082306f6bf0b27691c6039452d922c0d; branch codex/issue-11-seek-queue.
Preexisting untracked .claude/ is preserved and excluded. No prior local runs
existed in this worktree; the durable M3 acceptance remains intact.

Only the adapter command queue and its focused tests change behavior. Consecutive
unsent Seek requests now replace the pending tail target. Other controls retain
FIFO order as barriers, already-sent commands retain reply ownership, and the
32-entry bound remains. Native calls run outside the existing mutex. UI,
Session/Controller interfaces, polling, error handling and cleanup order are
unchanged. Arbitrarily fast alternating unrelated controls retain their existing
non-coalescible overload behavior; this is explicitly outside the assigned scope.

Observed vs inferred: independent and author reproductions use the actual
Controller and adapter over controlled native replies. One Seek is sent, 32
more queue behind it, and the next request returns Media, leaves Controller
Failed/inactive and fully disposes the engine. With repair, 100- and 101-target
bursts dispatch their final targets after reply release; the final property
sample is 202 seconds with the same engine/generation. The physical UI cadence
and actual installed-libmpv stress behavior were not measured.

Contract relied on: M2-AC-03 and the reviewed
[spec 0018](../specs/0018-seek-queue/spec.md), preserving absolute-seek,
pending-reply, FIFO control, cancellation and ownership semantics.

## Exact candidate and independent review

Reviewed implementation commit: f7649ae28dabad2272f548d3f3a202a5f332e714.

- Spec SHA-256: c4b022ce9355813332f75be159f29024f127d3b4c62eaf83b484c4aa6f19c7f9.
- Tasks SHA-256: fdcd73b4c4f1b08d78d6c84e56c734634e95d21d1306473b6659b1bafe74d8f9.
- engine.go SHA-256: bd537f572e131b73feaf8719a395defba03a4535f85f12f1b826c7c74813a423.
- seek_queue_test.go SHA-256: 84d141802a436687b358d82d63f4c6417269b7d3c3d3e3b9de74d57608598b24.

Reviewer /root/seek_reviewer_readable is independent of implementer
/root/seek_implement and controller /root. Architecture round 1 approves the
bounded design and independently reproduces the original defect. A reviewer-name
advisory was corrected before freeze as expressly permitted by that review;
no behavioral contract changed. The first attempted reviewer dispatch returned
unreadable transport messages and no usable evidence/disposition; it performed
no recorded review and introduced no finding.

Code round 1 approves the exact implementation above, with no open findings.
The reviewer independently runs adapter/controller/app race tests and the
original-source overlay controls, and verifies the controller's stable full
check and recorder event. The reviewer did not repeat the full repository check.

M2-EXT-002: **not BLOCKING** on f7649ae: independently passing real
Controller/adapter regressions retain the live engine and final target; restoring
the original adapter independently reproduces overflow/disposal. This technically
closes the reviewer's fresh baseline class-1 finding. It does not claim to close
or rewrite Copilot's original remote thread.

Handoff/navigation additions after code review are documentation only. Reviewed
product, test and frozen contract bytes are verified unchanged before publication.
The review does not purport to approve later prose or pre-approve remote CI.

## Acceptance and evidence

| Criterion | Candidate-specific verification | Local result |
| --- | --- | --- |
| SEEK-AC-01 | Held replies, two large bursts, final target/property sample, same live engine/generation | PASS |
| SEEK-AC-01 discrimination | Original source before repair and Go overlay after repair | Expected FAIL |
| SEEK-AC-02 | Play/Pause/Volume/Rate barriers; full Seek-tail replacement; non-coalescible bound; invalid/closed input; native error disposal | PASS |
| SEEK-AC-02 preservation | Adapter, Controller and app lifecycle/cancellation tests under race detector | PASS |
| SEEK-AC-03 local | Complete repository check --race on stable f7649ae; independent review | PASS |
| SEEK-AC-03 publication | Final PR head and tested merge revision; Linux/macOS/Windows CI; protected squash merge | See live PR receipt |

Commands use pinned Go 1.27.3 on darwin/arm64 with GOTOOLCHAIN=local and GOWORK=off.
The stable full check is recorded in event 000004 and evidence/check-2233809919.log,
SHA-256 256b080a9d463e77fc3654f1fa9317159b56cf5ae09c64c41ff1d4a6bb2a49f1.
All six steps pass. The first full-check attempt passed individual steps but was
correctly rejected because the controller committed during execution; this
failed evidence remains recorded, and is not reported as a successful check.
The stable rerun resolves that execution gap without a product-code change.

Sanitized local evidence lives under the controller run's evidence/planning,
evidence/implementation and evidence/review directories. It includes baseline,
before/after, original-source controls, targeted/full checks and independent
reports. These local files are not remotely accessible attachments or backed up
by Git. Maintained regression tests reproduce the scoped behavior. CI artifacts
are separate, expire after 14 days, and record all-package checks rather than
native playback or feature-specific recorder inputs.

No new native UI/library execution, physical playback, package or release claim
is made. Three-platform Go CI does not establish native Windows/Linux playback.

## Counts, inventory and continuation

R1–R9: architecture 1, code 1, product fix 1, failed product repairs 0. One
controller provenance-check execution failed and was rerun on stable source.
Expected old-code failures are discrimination evidence, not failed repairs.
Inherited filing budget 2/5 (#10 and #11), new issues 0. Identity advisory addressed;
no open BLOCKING or ADVISORY findings. No policy/check/CI changes.

Found, not done: no new outside defects. The explicit mixed-control overload
non-goal remains unchanged and is not a new backlog claim. No systemic follow-up
warranted. Rollback is a scoped source/test revert with no data migration.

The owner's instruction supplies authority to finish this PR's CI and merge
when gates pass. Consult PR #14 for that final receipt rather than treating
this pre-CI report as a merge claim. M3 remains owner-accepted; P0/full client
work remains outside this repair. No further milestone or release is started.
