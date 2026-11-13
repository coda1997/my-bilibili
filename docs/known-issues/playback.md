# Known Playback Issues

The four PLAY-KNOWN issues below remain **open for the legacy mygo adapter**.
On 2026-11-13 the owner selected libmpv as the default under
[ADR 0008](../adr/0008-default-libmpv.md). The old player is explicitly available
as `cmd/mygo-native`; its defects are preserved, not repaired by the switch.
The original temporary selection is recorded in
[ADR 0004](../adr/0004-provisional-mygo-playback.md).
The new fixture preview does not waive M1 requirements. The owner separately
changed platform priority on 2026-11-12 in
[ADR 0005](../adr/0005-platform-priority.md): replacement exits below require
macOS evidence; Windows is deferred P2 and nonblocking. This closes no issue.

## Affected Candidate And Evidence

- mygo pin: `v0.8.4-0.20261110145816-daf6341a2fe0`, commit
  `daf6341a2fe05f62dc8519e7cc215a90c757e8e0`.
- Original run: `20261111T000541Z-d757e87e7d8b`, closed `needs-human`.
- Retained implementation aggregate:
  `92c93e1f60f960893edb259ebe96d1c342a256e8a64d345ac488ce10c4a5b9fa`.
- Historical [M1 report](../../reports/0002-playback-macos.md) binds native
  binaries, failed strict runs, rejected diagnostic and independent reviews.
  Its local run evidence includes `native-final-r0/result.json` and the
  `architecture-status-consultation.md` API analysis. Local evidence is ignored,
  not backed up by Git; preserve it before removing this checkout.
- Current maintained source remains at
  `experiments/playback/{contract,player,ui}`. Only synthetic local/loopback
  fixtures were exercised; no live account, private media or Bilibili URLs.

## PLAY-KNOWN-001: Synchronous Delayed-Loading UI Stall

- Impact: opening slow media can block rendering, automation and user input;
  cancellation and the 10s controller deadline cannot execute while the UI
  thread is inside the native call. No responsive-cancel guarantee for this pin.
- Reproduce: build the e2e `cmd/mygo-native` app and run `cmd/smoke` with
  its default full suite. After
  local/HTTP playback, open the controlled 3s delayed fixture. The retained
  strict run fails `delayed loading before switch: automation response absent,
  oversized or timed out`. The app needs forced cleanup on this failure.
- Source: upstream `ui/engine_video.go` `advance` calls `Interval` in an `if`
  initializer before its Loading guards; the AVFoundation implementation
  synchronously calls `tracksWithMediaType`. Owned process samples and RPC
  timing are recorded in the historical report/consultation.
- Workaround limits: use the plain/local fixture for a bounded preview. Retry,
  a timer or a loading label does not unblock that synchronous call. Do not
  move public native calls to a worker or relax the full smoke deadlines.
- Replacement exit: delayed-load input remains responsive, switching prevents
  late restoration, cancel is externally observed within the original bound,
  and active app/engine/socket/server cleanup passes on macOS.

## PLAY-KNOWN-002: Native Error Cache Depends On Surface Advance

- Impact: a superficial stall workaround can hide actual native failures;
  public status accessors alone are not a safe independent refresh path.
- Reproduce/source: upstream `Err`/`Ready`/`Ended`/`Buffering` read cached
  wrapper state refreshed by the video surface's `advance`. The rejected
  positive-duration surface gate lets delayed-switch probes progress, but
  protected HTTP generation9 remains Loading/owned instead of producing Media
  failure within10s. No qualifying public status-only refresh was found by
  the independent architecture consultation.
- Evidence: rejected gate aggregate
  `335a29c97aab72b4ade498a2d39c2e9b3d719527b5682fecd239ded017dfea11`;
  app `cd599e865f155a89c3f9839d7ee3f12e1f6acbcb0f0e46cdd903508815370851`.
  This diagnostic is not the maintained candidate and its positives are not
  merged with the retained candidate's negative-route results.
- Workaround limits: keep the actual surface mounted while an engine exists;
  the duration gate is reverted. No hidden test host/status pump or Timeout to
  Media relabeling. This preserves current behavior, not a fix for issue001.
- Replacement exit: timely sanitized native Media errors with disposal on
  controlled protected/failure routes, independently of picture availability,
  alongside responsive loading and normal rendered playback on macOS.

## PLAY-KNOWN-003: Required Request Headers Unsupported

- Impact: sources needing authorization/referrer/custom request metadata cannot
  be played through this adapter. No real-service compatibility is claimed.
- Reproduce: `With required header` uses the synthetic protected route. The
  adapter returns typed Unsupported and owns no engine; the header is never
  silently discarded. Public mygo source options expose file/URL/data only.
- Evidence: native safe-rejection passes and failed header-playback capability
  are separate cells in the historical M1 report and runs; source
  `experiments/playback/player/native.go` plus pinned upstream public API.
- Workaround limits: unprotected fixture only; retry retains all metadata and
  remains Unsupported. No credential logging, forwarding proxy or header loss.
- Replacement exit: required synthetic headers actually reach the owned server
  and native playback/seek/cancel/cleanup pass; separately authorized target
  access is then verified on macOS without exposing secrets.

## PLAY-KNOWN-004: Separate Audio Track Unsupported

- Impact: separate video/audio media cannot be consumed as a synchronized
  presentation by the current public player API. Combined fixtures do not
  establish separate-track or Bilibili DASH compatibility.
- Reproduce: `Separate video/audio` passes controlled video and audio URLs;
  adapter returns typed Unsupported without opening a native engine. The
  public source options lack a separate audio input.
- Evidence: safe rejection and failed separate-track playback are distinct
  historical M1 cells; current adapter and pinned upstream public API.
- Workaround limits: combined generated fixture only; retry preserves the
  audio URL. No silent audio drop, remux/proxy or new engine in this increment.
- Replacement exit: both streams consumed with native picture and audible audio,
  seek, rate controls, cancellation and cleanup verified on macOS with authorized
  target media. Audibility requires evidence beyond config readback. Calibrated
  physical sync/rate measurements are outside M1 scope under
  [ADR 0006](../adr/0006-m1-measurement-scope.md); no synchronized-output claim
  follows from native state alone.

## Other Unverified Gates

Windows remains not-tested as optional P2, not an M1/M2/P0 blocker.
Real IME, packaging/signing and changed-candidate CI remain not-tested. Actual
native Bilibili compatibility now has a passing, independently reviewed bounded
result for the selected anonymous source/rendition. The first target refresh
stopped before native playback under its historical origin policy. A later diagnostic observed
a changed video CDN host. The owner removed all target host/port restrictions on
2026-11-13 under [ADR 0007](../adr/0007-target-host-policy.md); this is no longer
a current gate. See the [current target handoff](../../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13). Combined/separate fixture audibility has scoped,
independently reviewed owner attestations in [report 0016](../../reports/0016-output-verification.md).
Calibrated physical playback-rate/A-V-sync measurements were never performed;
ADR 0006 removes them from M1 without a mandatory later check. The original mygo
strict suite remains failed despite its passing preview suite. Current M1
technical evaluation is complete; the owner selected libmpv as the default.
The switch is recorded in [the default-switch handoff](../../reports/0024-default-libmpv.md).
The owner accepted M1 as completed on 2026-11-13.


## TGT-V2-001: Scoped target blocker closed; original failure preserved

The first version-2 invocation on 2026-11-13 passed scoped metadata admission,
but the shared readiness/picture deadline expired before any capability PASS.
Quit was acknowledged by the RPC transport, then required forced kill/reap.
The saved trace cannot distinguish Loading from Ready without visible picture;
root cause is unknown. Further controls/cancellation were not reached in that
invocation. A later locked-desktop observation does not prove its cause.
No host restriction or runtime change is inferred.

The independently reviewed post-repair invocation at 01:58 UTC on 2026-11-13
passes picture, selected tracks, controls, 87.7575ms playing cancellation and
graceful exit without force. The original reviewer closes TGT-V2-001 as the
current scoped capability/cleanup acceptance blocker. This does not reclassify
the original FAIL or establish a causal repair. Target loading cancellation
remains not-tested in this single-open session, with fixture coverage separate.

TGT-V2-002, missing failure state/resources/phase, is independently closed. The
bounded driver-only repair under spec 0010 retains validated samples and separates
primary from cleanup failures; passing offline evidence and exact review are linked in
[report 0023](../../reports/0023-target-failure-evidence.md). That diagnostic repair
alone did not close TGT-V2-001. The later scoped target result and final independent
adapter recommendation are in the target handoff. The owner subsequently
selected libmpv as the default; M1 was accepted as completed on 2026-11-13. Any future native attempt needs a distinct bounded
assignment and independent readiness disposition.
