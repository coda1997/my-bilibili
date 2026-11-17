# Additional bounded reference checks

Planner read these four public GitHub documents on 2026-11-17 at revision
cfc5fddcc8a94b74d91970bb5b4eaeb349addc47 of pskdje/bilibili-API-collect.
They were fetched as documentation, not invoked as Bilibili endpoints. The
temporary complete inspection copies are removed; no live/example media URL,
cookie, comment text, protobuf schema or implementation is copied into the
proposal or fixtures. Existing archive/license cautions in the service research
remain; the implementation is independently authored narrow Go conversion.

- [Detail fields](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/video/info.md): ordinary metadata includes rights flags and explicit redirection/upower markers. Paid/interactive markers are excluded conservatively. Some flags have uncertain meanings; the plan treats those nonzero values as Unsupported, not an invented permission grant.
- [Media format and quality tables](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/video/videostream_url.md): fnval16 is standard DASH; qn does not by itself prove a returned DASH choice. Video and audio arrays are distinct; null audio denotes no soundtrack; aliases exist for base URL/MIME fields. The plan selects actual AVC/AAC entries and a conservative client reuse cutoff, not the document's generic two-hour example as a service guarantee.
- [Danmaku metadata](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/danmaku/danmaku_view_proto.md): metadata is cookie-authenticated; state0/1 describes permitted/disabled comments. Segment configuration/count fields are marked uncertain. The plan skips this metadata operation in anonymous mode and never treats absent counts as empty comments.
- [Ordinary segment protocol](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/danmaku/danmaku_proto.md): both the original unsigned web and newer WBI routes are documented. Six-minute intervals use absolute progress and the first interval includes its 360000ms boundary. Modes1/2/3 are ordinary scrolling; sender hashes and advanced/command packages are separate and are excluded. This proposal deliberately chooses the already observed unsigned web route, with no silent fallback chain.

Other research references, signing/browser setup and ownership observations
remain in .agent/p0-full-research/result.md and were read by this planner.
No live availability, account entitlement or actual protocol response is
established by these public documentation reads.

## Local API basis for the bounded revision

Continuation author /root/p0_real_contract_revision inspected the following
existing local sources on2026-11-17. These are observed APIs, not runtime proof
of the future Store or verification controller. No public-reference refetch or
live Bilibili/account/history operation occurred for this response.

- Go1.27.3 darwin/arm64 standard library: syscall.Flock in
  src/syscall/zsyscall_darwin_arm64.go directly calls libSystem flock;
  zerrors_darwin_arm64.go declares LOCK_EX/LOCK_NB/LOCK_UN. The corresponding
  Linux syscall implementation exists. The implementation can use these with
  os.File and platform build tags, without a new dependency. Optional other
  platforms can compile an explicit LockUnsupported implementation rather than
  infer that a process-local mutex protects a shared file.
- Existing pinned mygo single_instance.go: RequestSingleInstanceLock is keyed
  by application identity, uses a socket rather than the history location, and
  explicitly returns true on some listen errors and after retry exhaustion. It
  is not an authoritative lock for history persistence.
- Fixed product reference14ce0ce6d374e5a655169348d5f7dfd76cf87efa:
  internal/adapters/mpv/surface.go sets pending only after successful native
  software rendering, consumes it under TryLock, and creates/updates a bitmap.
  DrawSized currently labels fixtures and returns no presentation observation.
  engine.go's DrawTarget has target labeling but fixed640x360 sizing; Close
  waits for native owners before dropping frames/bitmap on the UI thread. The
  proposed port observes this existing path, not a new decoder or frame buffer.
- Pinned mygo v0.8.4-0.20261110145816-daf6341a2fe0: ui/bitmap.go exposes
  NewBitmap, Update and Size with main-thread bitmap ownership; ui/api_gen.go
  exposes Image and Element.DrawOver. ui/paint.go executes its image drawing
  branch before the visible element's DrawOver callback. That supports the
  proposed idempotent scene-submission boolean and the separate gui.Tester
  synthetic raster assertion. It is not a GPU/compositor completion callback
  or physical display/AV calibration.

The final prepared response narrows its native changes to the explicitly named
P0R-010 mpv files and both strict observation consumers. It keeps buffer sizes,
threading, polling, cancellation, disposal and existing fixture pixel gates.
Standard lock primitives must still be exercised by the assigned independent
owners/helper-process product tests; source inspection alone cannot close those
future verification requirements.
