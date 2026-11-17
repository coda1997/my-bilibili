Closes #11.

Continuous seek input could fill the 32-entry adapter queue while a native reply was pending. The resulting Media error destroyed the playback engine. Consecutive unsent absolute Seek requests now coalesce to the latest target; Play/Pause/Volume/Rate remain ordering barriers and the queue stays bounded.

Spec: 0018-seek-queue, SEEK-001/002. Reviewed implementation: f7649ae28dabad2272f548d3f3a202a5f332e714, base a11ca09a082306f6bf0b27691c6039452d922c0d. No UI, public interface, timer, native lifetime, dependency, workflow or CI changes.

Observed vs inferred: real Controller and adapter over held fake-native replies reproduce the old overflow, Failed state and complete disposal. The repaired burst reaches the final target with the same engine. Physical/native UI stress was not measured.
Contract relied on: M2-AC-03 and spec 0018 absolute-seek, pending-reply, FIFO control and resource-lifetime contracts.

| Acceptance | Verification | Result |
| --- | --- | --- |
| SEEK-AC-01 | Two held-reply bursts, final sampled position, same engine/generation; original-source overlay | PASS; old source expected FAIL |
| SEEK-AC-02 | Control barriers, full-tail replacement/bound, invalid/closed requests, native failure, adapter/controller/app race suites | PASS on macOS, Go 1.27.3 |
| SEEK-AC-03 local | Complete repository check --race on stable implementation candidate | PASS |
| SEEK-AC-03 remote | Final PR head and tested merge candidate, Linux/macOS/Windows CI | Pending |

Independent reviewer /root/seek_reviewer_readable, independent of implementer /root/seek_implement and controller /root, approved architecture round 1 and exact code candidate round 1. Independently reran targeted race and original-source controls; inspected controller full-check evidence. No open findings. M2-EXT-002: not BLOCKING on reviewed candidate; baseline class-1 finding independently reproduced and repaired. This does not claim closure of Copilot's original remote thread.

R1–R9 counts: architecture 1, code 1, product fix 1, failed product repairs 0. One full-check attempt was rejected because controller committed during execution; a stable-candidate rerun passed, with original evidence retained. Inherited issue budget 2/5, new issues 0. No policy/check/CI changes needing adoption.

No new native library/UI, physical playback, package or release claim. Arbitrary alternating non-Seek control storms retain existing bounded overload behavior. Rollback is a scoped source/test revert, with no data migration. No systemic follow-up warranted. Found, not done: no new outside defects.

Controller run: 20261117T025128Z-5568fd01dc4f. Local logs are not remote attachments; maintained tests reproduce the evidence. CI artifacts expire after 14 days. See [the durable handoff](0030-seek-queue.md) and [PR #14](https://github.com/coda1997/my-bilibili/pull/14) for current publication/CI status. This saved pre-CI body records the local review; the live PR description receives final CI identities. Owner explicitly authorized PR publication, CI and merge if no problems.
