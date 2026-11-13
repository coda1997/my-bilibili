# Separate-track repair and remaining M1 gates

Date: 2026-11-12. Owner goal: **resolve M1 blockers**.
Controller/implementer: `/root`; run `20261112T011042Z-6545b719c09b`.
Contract: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md),
RPL-002, unchanged AC04/05/06 and cleanup requirements.

**The separate-track readiness blocker is repaired and the first complete
macOS fixture capability invocation passes.** It records 41 passed assertions,
zero failures and two explicitly not-tested categories. External playing
cancellation takes **71.583667ms**, within the unchanged **200ms** bound.
Missing-audio failure and graceful window/socket/server cleanup are now reached
and pass; no forced cleanup was required. Targeted and full local race checks
also pass.

These are author verification results. The exact repair awaits independent
review. Windows, physical speaker output, calibrated A/V sync/rate and
authorized target-media evidence remain missing, so the goal and M1 remain
open. No default-player or final adapter decision is implied.

## Cause and repair

The failed stage was not caused by missing headers or an unfetched audio file.
A sanitized native diagnostic observed both protected requests succeed and
found two tracks: selected video and an external audio track with
`selected=false`. The adapter then correctly failed the presentation instead
of accepting silent video-only playback.

The pinned mpv `player/loadfile.c:710` excludes all external tracks from default
selection when `autoload-files=no`, including explicitly supplied `audio-files`.
Its manual documents that interaction. The adapter had used that option to
disable automatic companion discovery. A bounded diagnostic changed only that
option and reached Ready with both selected tracks; the original failing
diagnostics remain retained.

The change in [`transport.go`](../experiments/playback/mpvprobe/transport.go)
permits external track selection only when the caller supplied `AudioURL`.
`audio-file-auto`, `sub-auto` and `cover-art-auto` remain `no`; the pinned
`autoload_external_files` function returns before discovery when all three are
disabled. Sessions without explicit audio retain `autoload-files=no`.
Typed audio/header encoding and required selected-audio checks are unchanged.
Missing or later-lost required audio still produces Media and complete disposal.

The regression models the verified native selection rule and asserts readiness,
selected external audio, disabled discovery and complete cleanup. It failed
on the old implementation before repair. Configuration-failure coverage now
checks header lists, audio lists and explicit-selection setup independently.
The native full stage additionally verifies real track selection, picture,
progression, pause, seek and both protected routes.

Only `transport.go` and `transport_test.go` changed during implementation.
The plain contract, UI, smoke driver/oracles, fixtures, library/dependencies
and shutdown repair remain unchanged. No native rebuild, deadline relaxation,
new media proxy or automatic discovery was introduced. Both prior cleanup
findings retain their [independent technical closures](0011-playing-close-review.md)
on the reviewed cleanup implementation/runtime.

## Candidate and evidence

Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.
All 122 files of the previous handoff matched before work. The exact tested
candidate is in this run's `evidence/candidate-r1/`:

| Artifact | SHA-256 |
| --- | --- |
| Tested source manifest, 122 files | `0ae80c2a86a9377894c1b4096f2a146157b108df2b75e73bff7c143cebcf7cf4` |
| Tested source archive | `3cd8329eb693aa1b71f696c3a8fab57f11220ea48a71928ee616b7e21042301c` |
| Tested tracked diff | `affda642a5701cf9a52c473889a0ebf8eae0cd23d2e90e5f51f89e9cf6f5c819` |
| Capability app | `786356f9971e57a6794852e75119b3456a9fa4d28795f88fa1ab8bc364834c70` |
| Runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| libmpv | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |
| Frozen spec | `94f843a6c06822e56307ad11aac293d5bc6c5985ae3511d03b38ec91c6ad42ef` |
| Frozen tasks | `5c5e9be9be9307e6d57159e8329a79ec5510675e336c6fefd2ba05222b48aed3` |

Runtime remains `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`, mpv source
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6. All 30 runtime artifact
hashes were checked; the complete stage's actual dyld images match those
artifacts plus the app, with no unexpected non-system image.

Final `evidence/handoff-candidate/` identifies this report and current navigation
and command-guide updates separately from the tested candidate. The final
verification confirms tested implementation bytes, historical reports,
contract, fixture and runtime identities remain unchanged. HEAD alone is not
the repair identity.

## Checks and capability results

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands use
`GOTOOLCHAIN=local GOWORK=off GOPATH=$PWD/.agent/gopath GOPROXY=off GOSUMDB=off`.
No live/account media or dependency download was used.

| Check | Result |
| --- | --- |
| Baseline native track diagnostics | FAIL, retained: fetched external audio was unselected; disposed Media failure |
| Isolated option diagnostic | PASS: explicit external audio selected, Ready, zero ownership and eight-second survival |
| New regression before repair | Expected FAIL for separate input; combined case passed |
| `go test -count=1 -race ./experiments/playback/mpvprobe` | PASS |
| `go run ./cmd/dev check --run 20261112T011042Z-6545b719c09b --race` | PASS, all six gates |
| Fresh e2e app build | PASS |
| One complete `replacement-smoke -stage capabilities` invocation | PASS, 41 assertions; 2 not-tested categories; 15.815s supervised elapsed |
| Actual repaired-code native separate probe | PASS: selected tracks and Ready; Close 2.579583ms; zero ownership; eight-second post-unload survival |

The two initial native diagnostics and the changed-option hypothesis are
diagnostic probes, not retries that replace a failed capability result. The
complete stage was executed once on the repaired candidate. The final direct
probe uses a scratch Go overlay that records only sanitized event IDs and
track booleans; it does not override the repaired transport options.

| Capability in the complete stage | Result |
| --- | --- |
| Local/HTTP picture, progress, pause, seek, sliders, configuration and explicit retry | PASS |
| Delayed switch/generation isolation | PASS |
| External loading cancellation | PASS, 74.229125ms; no owned engine/native workers |
| Required headers and Referer, comma encoding and session isolation | PASS |
| Separate protected video/audio selection, picture, progression, pause and seek | PASS |
| External playing cancellation | PASS, 71.583667ms; no owned engine/native workers |
| Missing audio | PASS, sanitized Media failure and zero ownership in 106.128917ms |
| Window close during delayed loading, socket removal, server shutdown | PASS, graceful exit; no forced cleanup; zero active handlers |
| Physical audio, actual playback-rate measurement and calibrated A/V sync | Not tested |
| Windows, authorized target service, real IME, packaging and candidate CI | Not tested |

The separate-track screenshot was inspected: the generated colored picture is
visible in the owned mygo window with Ready/8s and selected video/audio/external
flags. It is not proof of audible synchronized output. Ordinary Go race testing
does not instrument the C runtime. The previously independent six-scenario
cleanup suite was not rerun here; its implementation and runtime bytes are
unchanged, and this repair adds separate-session native disposal/survival
evidence rather than reclassifying the prior result as a new candidate check.

## Completion audit and next actions

| M1 gate | Current evidence and remaining work |
| --- | --- |
| Exact pins/provenance and macOS Go UI | Verified for this controlled candidate |
| macOS controlled transport, separate tracks, seek/cancel/cleanup | Author checks pass; independent repair review pending |
| Independent candidate/adapter assessment | Exact packet prepared in `evidence/review-packet.md`; no new reviewer dispatched |
| Actual Windows UI/playback | No local Windows/VM commands or relevant installed VM app found; `load_other.go` returns Unsupported. Needs an actual Windows environment and reviewed loader/ABI/runtime/automation scope |
| Physical audio and calibrated sync/rate | Needs an approved observation method and suitable calibration fixture; selected/fetched tracks are insufficient |
| Authorized target-media access | Existing contract allows synthetic local/loopback inputs only; needs authorized media/access and reviewed extension of that scope |
| Final adapter decision and M1 acceptance | Pending required evidence, independent assessment and owner decision |

The next step is independent review of the concrete candidate. The repository
requires explicit delegation before dispatching agents; the current goal did
not itself name agents/delegation. A separate authorized reviewer can consume
the packet and return an exact-candidate decision. Windows, audio/sync and
target-media arrangements remain required after the local code blocker is
fixed. The goal stays active; no acceptance requirement has been removed.

The run ends `needs-human` for those review/environment gates. Evidence is in
`.agent/runs/20261112T011042Z-6545b719c09b/evidence/`, including baseline and tested
snapshots, sanitized native diagnostics, before/after regression records, full
race log, capability captures/results, runtime-image audit, review packet and
`m1-completion-audit.json`. Ignored local evidence has no guaranteed backup;
preserve it before removing this checkout. Previous failed runs/reports are
unchanged. No commit, push, PR, merge or remote publication occurred.
