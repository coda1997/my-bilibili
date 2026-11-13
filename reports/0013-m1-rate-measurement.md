# Native-rate observations and remaining M1 gates

Date: 2026-11-12. Active owner goal: **resolve M1 blockers**.
Controller run: `20261112T012133Z-1383adb07f00`.
Prior implementation and full capability evidence:
[separate-track repair](0012-separate-track-repair.md).

Supplemental measurements now record native playhead progression against
monotonic elapsed time for separate video/audio at the three speed presets.
The longer half-speed sample measures **0.4997×**, while initial short samples
measure **0.4531× / 0.9960× / 2.0000×** for 0.5× / 1× / 2×. The early half-speed
shortfall remains visible; it is not replaced by the later observation.

No maintained implementation, check, fixture, runtime or acceptance threshold
changed. This adds native-clock evidence beyond configuration readback. It does
not observe physical speaker/display timing or calibrated A/V synchronization.
The M1 goal remains active and incomplete.

## Procedure and actual results

The experiment is a Go test overlay in this run's ignored evidence, using the
unchanged adapter and the same protected, generated video/audio fixtures. Each
preset gets a new session. It waits for Ready and selected video/external audio,
sets the requested rate, observes that configuration and starts playback.
Snapshots are sampled every 50ms against monotonic time, with native position
data copied before analysis. All raw samples are retained.

The initial measurements begin after 250ms of media progression and last two
wall-clock seconds. A separate diagnostic tests later half-speed behavior:
it waits for two seconds of media progression and measures for six seconds.
The changed window addresses the initial discrepancy; it is not an unchanged
retry seeking a passing result. No new numerical speed tolerance is asserted.

| Preset and window | Native media progression / elapsed time | Fitted rate | Complete Close |
| --- | --- | --- | --- |
| 0.5×, initial 2s window | 0.900s / 2.002055s | 0.453090× | 49.888583ms |
| 1×, initial 2s window | 1.966667s / 2.000364s | 0.995974× | 50.142916ms |
| 2×, initial 2s window | 4.000s / 2.000162s | 1.999982× | 80.147209ms |
| 0.5×, later 6s window | 3.000s / 6.002037s | 0.499670× | 90.821041ms |

Fitted rates are least-squares slopes through 41 samples in each initial
window and 121 in the longer window. Endpoint ratios and raw samples are
preserved alongside the fit. Time-position updates are frame/poll quantized;
this short experiment is not a calibrated physical-rate measurement.

The later observation is consistent with the half-speed setting. The two
different windows/runs do not establish the cause of the earlier shortfall,
so this report does not claim that startup settling is proven or waive it as
acceptable. A future independent assessment can decide what additional timing
evidence or product requirement is appropriate.

Both probe invocations pass their measurement prerequisites and unchanged
lifecycle assertions: complete Close within 200ms, zero owned cores/renderers/
workers, no recorded shutdown fallback, zero active fixture handlers, and
eight-second survival after the final unload. The initial probe takes 16.148s
and the later probe 18.906s under external supervision. A zero test exit here
means those assertions and data collection passed; it does not mean every
numerical rate or full RPL-AC-05 passed an acceptance oracle.

## Exact source and evidence

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands retain
`GOTOOLCHAIN=local GOWORK=off GOPATH=$PWD/.agent/gopath GOPROXY=off GOSUMDB=off`.
No live service/account input, microphone/system-audio capture, global
installation or remote action occurred.

Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.
The 123-file source candidate matches the preceding handoff:

- Manifest: `3a1425dc34f5c26df1ebc6edf6d20efc7f1c487ad594fed6dffe70abb1d3b3f2`.
- Archive: `49b808e7cf169e4b6466c2ec40016ad92bfe53649aef07d2aa83c34eb5c8e7a4`.
- Runtime manifest: `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`.
- libmpv: `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd`.
- Frozen spec/tasks remain `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef`
  / `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3`.

The two probe sources, overlay maps, binaries and command metadata are
identified in `evidence/evidence-manifest.json`. Both were compiled with
`go test -c -overlay <map> -tags nativecleanup`, then executed with
`-test.run=^TestNativeRateObservation$ -test.v -test.timeout=40s` under a 45s
supervisor. The verified absolute library path was supplied explicitly.
There was no full capability-stage or ordinary race-suite rerun: maintained
code is unchanged from the preceding successful checks.

All 123 source files still matched after measurement, as did 30 runtime
artifacts and six fixture/provenance files. Final `handoff-candidate/` separately
captures this report and README/stage-zero navigation; tested implementation
and all historical reports remain unchanged. Current source identity never
relies on dirty HEAD alone.

## Remaining work

The previous turn made implementation progress; this continuation adds actual
timing observations. It does not complete the unresolved prerequisites:

| Gate | State and next requirement |
| --- | --- |
| macOS controlled fixture capability stage | Previous exact implementation passes 41 assertions, including separate tracks and external cancel; result retained unchanged |
| Independent selection-repair review | Exact packet remains in run `20261112T011042Z-6545b719c09b`; dispatch still needs explicit delegation or a separate human reviewer |
| Windows actual UI/playback | Local VM/Windows tooling recheck found none; needs environment and reviewed loader/ABI/runtime/automation scope |
| Physical audio, physical rate and calibrated A/V sync | Not observed; needs an approved observation/capture setup and calibrated method |
| Authorized target-media compatibility | Not tested; current input scope is synthetic local/loopback only; needs authorized media/access and reviewed extension |
| Final adapter decision and M1 acceptance | Pending required evidence, independent assessment and owner decision |

Both historical cleanup findings retain their independent technical closures.
The pending selection repair has not been self-approved or independently
reviewed by the implementation author. No new agent was dispatched. The
provisional mygo preview remains the default; no M2/full-shell work began.

The run ends `needs-human` with all evidence retained and the broader goal
active. `rate-samples.json`, `rate-summary.json`, `rate-steady-samples.json` and
`rate-steady-summary.json` contain the observations. `m1-completion-audit.json`
records the remaining requirements; the prior concrete review packet remains
ready. Evidence under `.agent/runs/20261112T012133Z-1383adb07f00/evidence/` is
ignored local storage without guaranteed backup. No commit, push, PR or merge
occurred.
