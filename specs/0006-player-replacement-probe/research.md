# Replacement probe rationale and source record

Date: 2026-11-11. Source research and proposed design only; no native libmpv
runtime, installation, binding, playback, audio or Windows result is established.

## Existing repository evidence

Read [the current handoff](../../reports/0005-provisional-playback.md),
[ADR 0004](../../docs/adr/0004-provisional-mygo-playback.md) and
[the four open issues](../../docs/known-issues/playback.md). The temporary preview
is a useful baseline, with an exposed strict delayed-loading failure. Do not
rewrite its history as a replacement result. Current clean planning base is
`3ecf7622725033d04aff0135c2c9ac02ca128b08`.

Inspected local maintained source:

- `experiments/playback/contract/contract.go`: plain input/engine/controller;
  synchronous disposal before switching, generation control, 50ms polling,
  10s ready deadline and safe categories.
- `experiments/playback/player/native.go`: the mygo-specific surface lives in
  the adapter; headers and AudioURL are explicitly Unsupported.
- `experiments/playback/ui/view.go`: `Surface` is already injected. Keep this
  boundary; only an optional probe warning is proposed for shared UI code.
- `experiments/playback/cmd/smoke/media.go`: strict external cancel measurement,
  state-based playback assertions and negative routes. Its Unsupported cells
  describe the old player, so a separate replacement driver is required.
- `experiments/playback/cmd/fixturehttp/server.go`: generated allowlisted media,
  3s maximum delay, protected sentinel route and bounded server lifecycle.

At pinned mygo commit `daf6341a2fe05f62dc8519e7cc215a90c757e8e0`, inspected public
`ui/bitmap.go` (`NewBitmap`, `Bitmap.Update`) and `ui/widgets.go` / `ui/api_gen.go`
(`Image`). These provide the proposed bitmap presentation seam. The local
source was found in the ignored `.agent/gopath/pkg/mod/` cache after the default
module-cache path proved absent. Future clones must use normal module setup;
the ignored cache is not a repository prerequisite or native proof.

## Why libmpv is the first proposed candidate

This is a narrow first-candidate recommendation, not an exhaustive engine
comparison. Retaining mygo UI and testing one replacement reduces the number of
variables. A platform-specific AVFoundation/Windows backend pair and libVLC are
deferred alternatives, not evaluated or claimed incapable here. No alternate
engine should be added automatically if this probe fails.

The exact mpv source reference is `v0.42.0`, peeled commit
`b0841e8c4d156b8403dc1876c4cbd9011eb583f3`. Read-only `git ls-remote` returned
tag object `b985c3634ef056b185e3f13c7ac597b3611f5774` and that peeled commit.
Do not confuse the tag object, commit, client API version or native binary hash.

Primary sources, all read at that commit:

1. [client API](https://github.com/mpv-player/mpv/blob/b0841e8c4d156b8403dc1876c4cbd9011eb583f3/include/mpv/client.h):
   API version 2.6, asynchronous commands/property observation, event polling
   and reply identifiers. Synchronous calls can wait on slow I/O. Destruction
   can block; asynchronous quit is not proof that resources have been released.
   This motivates the independent loading-teardown gate, not a guaranteed fix.
2. [render API](https://github.com/mpv-player/mpv/blob/b0841e8c4d156b8403dc1876c4cbd9011eb583f3/include/mpv/render.h):
   software rendering writes into a memory surface, with thread/callback
   restrictions and context-before-core teardown. Upstream describes this path
   as slow. Propose it only at 640x360 to avoid introducing a GPU/window bridge
   before compatibility is known.
3. [options](https://github.com/mpv-player/mpv/blob/b0841e8c4d156b8403dc1876c4cbd9011eb583f3/DOCS/man/options.rst):
   `http-header-fields`, `referrer` and `audio-files` describe the needed input
   capabilities. `audio-file` is a CLI/config-only alias. API list encoding,
   both-stream propagation and failure semantics still need actual tests.
4. [copyright/build licensing](https://github.com/mpv-player/mpv/blob/b0841e8c4d156b8403dc1876c4cbd9011eb583f3/Copyright):
   licensing depends on build selections and linked libraries; the build flag
   alone is insufficient evidence about the resulting binary. Require actual
   provenance before acquisition/distribution decisions; no legal clearance is
   claimed by this plan.

## Proposed approach and stop conditions

Map only the C functions needed for the probe through already pinned purego;
validate ABI and native-memory/callback ownership before trusting it. Use a
dedicated control owner, separate software-render owner and bounded copied
frames. The UI sees only plain snapshots and adapter-local drawing. State
refresh does not depend on mounting a native video surface.

This design deliberately exposes the hardest compatibility question early:
the existing controller requires complete synchronous disposal before switch.
If native teardown prevents the existing 100ms externally observed cancel
criterion, the candidate fails this contract. Moving cleanup to a goroutine,
marking it closed early or leaving a prior core alive would violate the goal.
An independent reviewer must inspect gate one before further implementation.

Before implementation there are three required decisions/evidence gaps:

- Obtain independent architecture review of this exact proposal in a separately
  authorized context; no reviewer was dispatched for this planning request.
- Authorize implementation and a specific runtime acquisition/build path with
  dependency/license provenance. No candidate binary or installation source is
  selected here, and global installation is outside the present request.
- Arrange the later Windows and audio/sync evidence. Their absence need not
  prevent a bounded macOS fixture feasibility result, but prevents final M1 or
  full separate-track playback acceptance.

No benchmarking, runtime source audit, binary license audit or cross-platform
compatibility conclusion has been completed by this source survey.
