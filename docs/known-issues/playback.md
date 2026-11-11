# Known Playback Issues

All issues below are **open** as of 2026-11-11. The owner selected temporary
mygo use, not a repair or final acceptance; see [ADR 0004](../adr/0004-provisional-mygo-playback.md).
The new fixture preview does not waive the original M1 requirements.

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
- Reproduce: build the e2e native app and run the default full smoke. After
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
  and active app/engine/socket/server cleanup passes on both target OSes.

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
  alongside responsive loading and normal rendered playback on both OSes.

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
  access is then verified on both OSes without exposing secrets.

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
- Replacement exit: both streams consumed, native synchronized picture/audio,
  seek, rate, cancellation and cleanup verified on both OSes with authorized
  target media; physical audio/A-V sync require evidence beyond config readback.

## Other Unverified Gates

Windows, real IME, physical audio, actual playback-rate/A-V-sync measurements,
authorized Bilibili access, packaging/signing and changed-candidate CI remain
not-tested. The explicit preview suite may pass while full/M1 remains failed.
Replacing the playback component is future owner-selected work, not authorized
by documenting these issues or by a preview result.
