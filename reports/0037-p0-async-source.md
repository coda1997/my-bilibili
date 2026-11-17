# Responsive asynchronous source loading

Date: 2026-11-17. Full lane, spec0025, ASY-001..005.
Implementation candidate: `371428b7ad45cc472520a1bfd78066ba217a3b4f`.
Run: `20261117T114618Z-010da1bb9581`. Remote CI: not run for this candidate.
Independent architecture and final code/native review: approved by
`/root/p0_async_review`; ASY-CODE-001 technically closed. This completes the
asynchronous prerequisite, not the owner's original full-P0 goal.

## Delivered and observed

Detail and playback-address resolution now run on one owned worker, with one
latest pending request and one acknowledged result. Construction performs no
source I/O. Submission-based ten-second deadlines, copied inputs, separate
source/native generations and UI-thread acceptance prevent stale responses
from opening an obsolete part. Detail failure and selected-part/accepted-player
retry failures have explicit states and actions.

Accepted playback remains usable while a different part resolves or fails.
Pause/play, volume and rate changes survive the pending request. Close retires
requests and native playback without waiting for source I/O on the UI thread;
an owned dispatcher observer waits for actual source completion before closing
the window, including while minimized and unable to paint. Parent cancellation
does not itself request window disposal. Fixture hashes still verify, now with
cooperative context checks around bounded reads.

Observed vs inferred: the table establishes controlled offline and native
fixture behavior, not real HTTP loading or full-client acceptance. Contract
relied on: frozen spec0025, plain Source/Player state, UI-thread Controller,
30-second resolution margin and existing mygo Close/Update semantics.

## Evidence

| Done criterion / claim | Candidate-specific evidence | Result | Scope and not tested |
| --- | --- | --- | --- |
| ASY-AC-01: responsive detail startup and explicit retry | app/detail, desktop and tagged source regressions; native source detail hold/release/retry | PASS | Controlled fixture Source, no live service |
| ASY-AC-02: bounded work, latest-only results and retained accepted playback | race tests, six compiled source-mechanism negative controls; native P2 failure/retry | PASS | Negative mutations were restored; no stale native open |
| ASY-AC-03: latest transport/settings intent and separate retry identities | pending/terminal/expiry regressions; native delayed P2 stays paused | PASS | Synthetic native media |
| ASY-AC-04: deadlines, expiry, ownership and paint-independent close | controlled clock/cancellation tests; actual ten-second native timeout and minimized Detail/Resolve close | PASS | No RPC/tree/capture after requesting minimized close; closed receipts have zero ownership |
| ASY-AC-05: existing controls/fullscreen/cleanup remain usable | macOS full stage 30 checks; source stage 25 checks in seven scenarios | PASS | Matching rebuilt normal/e2e bundles and driver; no forced cleanup |
| Existing cancellation deadline and live survival | Playing 65.856125ms; loading 79.632ms; all required eight-second live post-disposal checks | PASS | Original 200ms bound retained; not physical A/V calibration |
| ASY-AC-06: deterministic checks | Recorded check --race, six steps; tagged cmd/my-bilibili race | PASS | Exact implementation commit above; no current-candidate CI or Windows native claim |
| ASY-AC-06: independent review and limit regressions | Original reviewer architecture/code/closure reports; revised tests pass and each compiled limit-free overlay fails | APPROVED | ASY-CODE-001 closed in final round; no remaining finding |

Local evidence is under `.agent/p0-async-source/`,
`.agent/p0-async-source-review/` and the run's immutable `evidence/`.
Final full/source native reports are `native-full-2/report.json` and
`native-source-2/report.json`, also attached to the run. Matching artifact
hashes and synthetic PNGs remain local. These paths are not remote attachments
or a backup; preserve the checkout or copy evidence to approved storage before
removing it. Windows is P2/not-tested.

## Review history and continuation

The reviewer is independent of root and the app/UI implementers. Architecture
round1/global13 had no findings. Code/native round2/global14 found that two
working 64KiB limits lacked isolated regression inputs. Only those two test
inputs changed from candidate416c408; the original reviewer independently
verified their passing positive and failing compiled negative checks, plus
fresh matching native evidence, and closed ASY-CODE-001 in round3/global16.
Original reports and both sets of native evidence are retained.

Final async counts: reviews3/3, genuine corrections2/3. The first correction
fixed an observed deadline boundary where the logical cutoff preceded
context.Err publication; each original affected assertion failed once. The
second was the test-input repair above, passing on its first verification.
Intentional negative controls are not candidate failures. No native failure
or repair occurred. No scoped blocker, advisory or high-severity claim remains.
Shared remote issue budget0/5 and earlier login history are unchanged. No
check/CI/workflow policy change, remote publication or merge is part of this
slice. No systemic follow-up warranted.

Found, not done: native Chinese danmaku, real search/link/details/quality, service
session publication and device-local history/resume remain the planned P0
follow-ons in `.agent/p0-start/inventory.md` and the overlay/full-client plans.
They are not newly reproduced defects and no issue was filed. Existing
LOG-ADV-001 account wording disposition remains assigned to future account UI
integration. Real scan/save/restart evidence and the Keychain session are
preserved; this work neither accesses nor deletes that session.

Next: freeze independently approved spec0026 and implement native Chinese
danmaku, then review and implement the real-client/history proposal. Its
architecture reviewer inspected416c408; the only subsequent source changes
were the two test inputs, with all relied-on product seams unchanged. No
further async review is pending or authorized under its consumed round budget.
