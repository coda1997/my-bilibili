# Independent separate-track review and remaining M1 gates

Date: 2026-11-12. Controller run: `20261112T124520Z-e12a2e27addd`.
Owner authorization: “yes, go ahead” to dispatch the prepared independent review.
Reviewer: `/root/coreaudio_cleanup_review`, independent of implementation author
`/root`. The review is complete and the controller run records `succeeded` for
this bounded verification task.

**The separate-track selection repair is independently approved for the exact
candidate/runtime and controlled macOS fixture evidence below.** No new
actionable findings were identified. The interim recommendation is **continue
evaluation**. Both earlier cleanup findings retain their technical closures.
Full RPL-AC-05, RPL-002/RPL-003 and M1 remain unaccepted.

This continuation changes only this report and README/stage-zero navigation.
It does not change implementation, runtime, fixtures, check policy or acceptance
bounds. Prior [repair evidence](0012-separate-track-repair.md),
[rate observations](0013-m1-rate-measurement.md) and
[cleanup review](0011-playing-close-review.md) remain historical records.

## Independent results

The reviewer verified the 124-file source candidate, frozen spec/tasks, all 30
runtime artifacts, 901 pinned native-source files and six fixture/provenance
files before and after the checks. The fresh app is byte-identical to the
author-tested app. Its actual loaded images match the app and all 30 runtime
artifacts, with no unexpected non-system image.

The full `go run ./cmd/dev check --race` passes all six gates. Fresh app/driver
builds pass. The first and only complete native capabilities invocation passes
**41 assertions, zero failures and two explicit not-tested categories**. It
exits normally in 15.5422325 seconds, without an exposure block or forced cleanup.

| Independent macOS observation | Result |
| --- | --- |
| Combined local/HTTP and protected picture, progression, pause and seek | PASS |
| Slider interaction, rate/volume configuration, retry and generation isolation | PASS |
| Loading cancellation through Closed/Canceled with zero ownership | PASS, **103.35175ms** against 200ms |
| Required headers/Referer, comma encoding and later headerless-session isolation | PASS |
| Separate protected video and external audio selection, picture and controls | PASS; both streams receive required metadata |
| Separate-playing cancellation with zero native/worker ownership | PASS, **45.361125ms** against 200ms |
| Missing audio produces disposed, sanitized Media failure | PASS, **105.062833ms** against the unchanged 10s failure deadline |
| Window close during delayed loading, socket removal and server cleanup | PASS, graceful exit and zero active handlers |
| Physical audio, physical rate and calibrated A/V synchronization | Not tested |
| Windows actual UI/playback and authorized target-media flows | Not tested |

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands use
`GOTOOLCHAIN=local GOWORK=off GOPATH=$PWD/.agent/gopath GOPROXY=off GOSUMDB=off`.
The reviewer built replacement-native with `-tags e2e`, built replacement-smoke,
and invoked `-stage capabilities` with explicit app/runtime/fixture/output paths.
Full commands, timing, exits, logs and six native captures are retained in the
imported review. No unchanged failed-stage retry or selected passing sub-run
was substituted for the complete invocation. Ordinary race checks do not
instrument the native C runtime.

## Review assessment and evidence limits

The incremental repair changes only `transport.go` and `transport_test.go` from
the cleanup-approved implementation. The reviewer confirmed in the exact pinned
mpv source that `autoload-files=no` suppresses selection even for explicitly
supplied external audio. The repaired adapter sets the typed `audio-files` list
and enables selection only when audio is supplied. All three automatic
audio/subtitle/cover-art discovery options remain disabled; native code returns
before companion discovery under that configuration.

Selected-video/external-audio readiness requirements, later audio-loss failure,
metadata isolation and complete destruction/unload remain intact. The new
selection regression and expanded option-failure cleanup tests pass in the
independent race check. The reviewer found no unresolved incremental risk that
required an additional isolated native probe after the full stage passed.

The reviewer also verified all 105 inventoried artifacts from the prior repair
and measurement runs. Original separate-readiness failures, failed diagnostics,
old-code regression and earlier cleanup/timing failures remain unchanged. The
previous six-scenario cleanup suite was not rerun here: its implementation and
runtime are unchanged, its independent closures remain scoped to that evidence,
and the fresh capabilities stage supplies current cancellation/cleanup results.

The four retained rate fits were independently recomputed from raw samples:
initial **0.453089618× / 0.995974122× / 1.999981916×**, and later half-speed
**0.499669821×**. The initial half-speed shortfall is preserved. Different
windows/runs do not establish its cause; no speed tolerance was invented.
Native-clock observations do not measure physical output or calibrated A/V sync.
Their retained lifecycle/survival assertions were audited, not freshly rerun.

RPL-AC-01/03/04/06 receive current provenance, control, transport, negative-path
and cleanup evidence. AC-05 receives selected-track and fixture control evidence
but remains incomplete without physical audio and synchronization. AC-07 receives
this independent scoped disposition and interim recommendation. AC-09 retains
the previous independent cleanup closures plus current full-stage cleanup.
These are bounded conclusions, not full task or milestone acceptance.

## Exact identities and retained artifacts

Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.
The reviewed source is identified by its manifest/archive, not by HEAD alone.

| Artifact | SHA-256 |
| --- | --- |
| Reviewed 124-file manifest | `df0ccb082c46ab66cf571106658d2f28d17e6de268ed6bf73bfd9fe0104ea7bf` |
| Reviewed source archive | `863d4bd21e42ccc52e0cef2032642fa88430e8c51a91b83e64e3ead55bfc612f` |
| Frozen spec | `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef` |
| Frozen tasks | `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3` |
| Runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| libmpv | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |
| Fresh e2e app | `786356f9971e57a6794852e75119b3456a9fa4d28795f88fa1ab8bc364834c70` |
| Independent structured review | `0b55182b89c529a69bd84b902e9e8a9f60289df4968a32f017be396545b58869` |
| Independent evidence inventory | `904fe71d88192da583ac3f6b8104e708ce933e711afb2fdf8219d5e9f0271250` |

Runtime location: `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`, mpv source
pin `b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6. Runtime and fixture
identities are fully enumerated in `independent-review/identity-after.json`.

All current evidence is under
`.agent/runs/20261112T124520Z-e12a2e27addd/evidence/`:

- `candidate/` preserves the exact reviewed manifest, archive and diff.
- `independent-review/review.md` contains the complete technical disposition.
- `independent-review/` contains 34 sealed artifacts totaling 28,411,968 bytes,
  plus their inventory; the controller reverified every size/hash on import.
- `review-import.json` and `retained-evidence-audit.json` record controller
  integrity checks without substituting for independent review.
- `handoff-candidate/` identifies this subsequent documentation-only snapshot;
  `handoff-verification.json` verifies unchanged implementation and old reports.
- `m1-completion-audit.json` records remaining gates; `evidence-manifest.json`
  inventories the completed controller evidence.

Ignored evidence is local storage without guaranteed backup or remote access.
Preserve required artifacts before removing the checkout. No commit, push, PR,
merge, runtime rebuild, global installation or remote action occurred. Candidate
CI, packaging and real IME checks were not run; packaging belongs to later scope.

## Remaining M1 gates and next action

The current selection-repair independent-review gate is complete. The owner goal
to resolve M1 blockers remains incomplete:

| Gate | Next requirement |
| --- | --- |
| Windows actual Go UI/playback | An available Windows environment and reviewed loader/ABI/runtime/automation scope; prior local discovery found no runtime or VM |
| Physical audio, physical rate and calibrated A/V sync | An authorized observation/capture setup and calibrated fixture/method |
| Authorized target-media compatibility | Authorized media/access and a reviewed extension beyond synthetic local/loopback input |
| Final adapter decision and M1 acceptance | Independent final assessment and owner decision after required evidence exists |

The next step is to arrange one of these missing execution inputs and define its
bounded verification scope. A Windows cross-build or selected audio flag cannot
supply the missing runtime/output evidence. The provisional mygo preview remains
the default; no M2/full-shell work or production-player selection is authorized
by this review. Read this report before continuing; historical handoffs retain
the authority and outcomes applicable to their own candidates.
