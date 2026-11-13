# Default libmpv playback

- Date: 2026-11-13
- Run: `20261113T023232Z-ffe109b8bd77`
- Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, dirty development
  checkout; source identity is recorded by the run's manifest/archive.
- Owner decision: set libmpv as the default after the independent adapter
  recommendation in [the previous handoff](0021-target-access-method.md#post-repair-target-continuation-2026-11-13).
- Contract: [0011-default-libmpv](../specs/0011-default-libmpv/spec.md), task
  DMP-001; decision [ADR 0008](../docs/adr/0008-default-libmpv.md).
- Architecture review: independent `/root/default_libmpv_review`, PROCEED;
  exact report retained under `evidence/architecture-review/` in this run.
- Execution: implementation and scoped macOS verification complete.
- Independent code/native-evidence review: APPROVE, no actionable findings;
  `/root/default_libmpv_review`, separate from the implementation author.

## Delivered

`cmd/native` now composes the existing libmpv adapter, retaining mygo UI and
the plain playback boundary. It names libmpv in the window/banner and checks
native ownership at window close. It accepts a required absolute `-library`
path and optional owned loopback `-http-base`; a local fixture needs no server.
Invalid library paths or fixture origins fail before window creation. Runtime
provenance remains an explicit prerequisite, with no download or fallback.

`cmd/mygo-native` preserves the old default main and e2e import byte-for-byte.
The legacy `cmd/smoke` still exercises that player's original full/preview suites.
The libmpv `replacement-smoke` recognizes the fixed default title as well as
the replacement probe title; all capability assertions/deadlines are unchanged.
Current docs and [commands](../experiments/playback/cmd/README.md#default-libmpv-playback)
route to libmpv. Historical reports, failed outcomes, adapters, runtime, fixtures,
target policy and dependencies are preserved.

## Verification

| Criterion | Actual result on macOS arm64 / Go 1.27.3 |
| --- | --- |
| DMP-AC-01: default composition and safe startup | PASS; libmpv opener/surface, nonempty backend banner, local-only setup and origin/path validation; author and independent offline race tests pass without loading native code |
| DMP-AC-02: native default behavior | PASS; sole fresh `replacement-smoke -stage capabilities` invocation on rebuilt default app passes all 41 implemented assertions |
| DMP-AC-02: preserved legacy diagnostic | PASS; main/e2e import byte-identical to prior default and e2e build succeeds; no new legacy runtime pass claimed |
| DMP-AC-03: checks/docs/review | PASS; full six-stage `go run ./cmd/dev check --run 20261113T023232Z-ffe109b8bd77 --race`, scoped documentation audit and independent candidate/native-evidence approval |

Native assertions include exposed local/HTTP picture, progression/pause/seek,
slider/rate/volume configuration, retry, required headers and their isolation,
selected separate audio/video, negative routes and graceful disposal. Externally
observed loading cancellation takes **103.205542ms**, playing cancellation
**82.0935ms**, both within the unchanged **200ms** bound. App close reports zero
native/worker ownership; the socket is removed and server Active returns to zero.
No force was used. The 300 recorded automation calls have no errors/drops. All
30 expected runtime artifact hashes and actual loaded-image paths match. No
owned build process remains and the temporary display/idle assertion ended with
the invocation.

The result retains two `not-tested` categories for physical output and other
platform/target/release evidence. This fixture run does not repeat the prior
target session or establish new human audibility, physical synchronization or
actual output-rate measurements. Earlier audibility/target evidence remains
separately attributed; calibrated measurements stay outside M1 under ADR 0006.

The default normal/e2e builds, legacy e2e build and fixture driver build all pass.
Normal builds exclude the automation plugin. The planning review separately
verified all 184 previous maintained files and passed the baseline full race
check; that baseline result is not used as proof for the changed code.

## Exact source and evidence

The tested/reviewed r0 maintained candidate has 192 files, with 18 scoped changes
relative to the previous final handoff. Its manifest SHA256 is
`266ae08b58a248b211c684af7b7e6fa1d16f9ffdda0cf3fce932555508fdc083`, archive
`c80ca346182b9e57ea21be7bb3de05abef73255a58f5579d03e5054ad2d7efb9`, under
`evidence/candidate-r0/` in this run. Frozen spec SHA256 is
`000f3fbbb42bf186fedda3a3d7341cd516e65368494f0e79c52513f56ae041a8`, task graph
`e60287170d0a69984a8188f7ff7757d2126368a65448c2d7db96be1eaa7c93e8`.
The base commit alone does not identify this dirty candidate.

The default e2e executable SHA256 is
`a3a6ae15aa7d8334457bc72821a9d0cd52de0aa2f525d38bdd59c62fbd401d35`;
the normal build is
`1e60be5cb5f4042ab19ed35e00f5dacbe0aeee779b6cfb641c5cc657b8b5a8e5`.
The runtime manifest remains
`a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`, with
libmpv `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd`.

| Local run evidence | SHA256 |
| --- | --- |
| `evidence/check-2097200974.log` | `ddd83750de1ef18f06d42f0a7faca66c8573a81a8290c9f8b60ad39d9f2c733b` |
| `evidence/builds-r0.json` | `953b3629f72d3ae770ed2796a69cc772c33ad0ffa15a6bd55220a91a0aef590e` |
| `evidence/native-first/result.json` | `aa467ab717d5ba7e62057787e16e5b9e0c6ae51fd01414e4bd7b85f6fe035f35` |
| `evidence/native-first/inventory.json` | `823ff32471940aa29966e753d52d13e7d02ed0ab8939030327a25880321b0fa2` |
| `evidence/code-review-r0/code-review.md` | `296febf6abd8d2b3ed89f68cffbade0ad9bf1b7743dcb6f81ad40f651c05bad0` |
| `evidence/code-review-r0/inventory.json` | `9836f2237307452fdd2492718b5035358ca661dc0be3ba2a065225e1d074dbe2` |

The independent reviewer verified exact working/archive hashes and modes, frozen
contracts, unchanged adapter/runtime/input-policy groups, byte-identical legacy
files and all 137 then-current relative documentation links/anchors. It ran
targeted race tests and audited the controller's full check, builds and sole
native invocation; it did not claim a second native run or independent hearing.
No additional code/native test was indicated for the final descriptive-only
handoff update. A separate `evidence/final-r1/` manifest/archive identifies that
documentation delta; the run records retain the corresponding final disposition.

## Remaining scope and next action

The owner selected the default adapter. Formal M1 milestone acceptance remains
pending; M2/P0 and the full shell have not started. Windows remains optional
P2/not-tested. Packaging/signing, real IME and CI for this dirty candidate remain
not-tested. Calibrated physical measurements remain outside M1 under ADR 0006.
No target service request is part of this switch; the previous scoped target and
fixture audibility results retain their own attribution and limitations.

The requested default switch is complete. The next milestone action is the
owner's formal M1 acceptance; no further implementation is started by this
handoff. No commit, push or publication occurred.
Detailed `.agent/` evidence is ignored, local-only and not backed up by Git.
