# M1 target failure and bounded diagnostic repair

2026-11-13. M1 remains incomplete. The current native fixture regression passes,
but the first version-2 real-target invocation failed before confirming its
ready-picture capability and required forced cleanup. Independent review accepts
that recorded FAIL and leaves TGT-V2-001 open with unknown cause. All valid HTTPS
hosts and ports remain allowed under [ADR 0007](../docs/adr/0007-target-host-policy.md).
No origin list, host approval or repinning is restored.

The driver-only repair under [spec 0010](../specs/0010-target-failure-evidence/spec.md)
has independent technical approval and closes TGT-V2-002: missing safe failure
state and phase. It does not repair or
explain the original native failure. The original host-removal implementation and
approvals remain in [report 0022](0022-target-host-removal.md); prior UNKNOWN
outcomes and cleanup findings retain their exact historical dispositions.

## Current fixture and target evidence

Continuation run `20261113T011610Z-2abfeb37a20c` froze current spec 0008 with a
bounded assignment and independent readiness approval. Its source was the exact
178-file HOST001 maintained candidate, manifest SHA256
`6c265aa620efd888edc634906665c0a313a5cb283c6d7cc232d47536372fd379`.
The 11-file refresh-helper manifest remains
`5a30e014a846a4c8fa7a584f3a83ec8faaa9212f50a527fc70554f3b702ef5ed`.

| Observation | Actual result |
| --- | --- |
| Current native fixture capability invocation, 01:17 UTC | PASS, 41 assertions; 15.818s, exit 0 |
| Fixture loading / playing cancellation | 99.368625ms / 79.460375ms; unchanged 200ms bound |
| Fixture disposal and runtime | No forced cleanup, zero active HTTP handlers; 31 actual non-system images equal the fixture executable plus 30 pinned runtime artifacts |
| Fresh fixed anonymous metadata pair, 01:21 UTC | Both HTTP 200; exact reviewed source/part, full 0..596624ms grant, quality32 H264 and AAC30216 admitted |
| First version-2 target ready-picture-selected-tracks gate | FAIL by shared readiness/picture timeout; no capability PASS |
| Actual target progression, controls and cancellation | Not reached; not-tested |
| Target owned-process exit | FAIL; Quit RPC acknowledged but process required forced kill/reap |
| Terminal containment | Forced reap recorded; no residual owned native process observed afterward |

The target invocation took 16.269s. The helper exited 0 while explicitly reporting
`driver-failed; inspect-sanitized-driver-evidence`; helper exit is not playback
success. Its 375 sanitized automation calls contain 371 tree polls, two window
queries and two actions, with no RPC errors or dropped calls. The old result has
no normalized last snapshot/resource sample. It cannot distinguish never reaching
Ready from Ready without visible picture or selected-track evidence. Successful
RPC acknowledgement does not establish that the media or Quit action completed.
No causal attribution to service denial, TLS, host identity, codec or runtime is
supported. No further target request was made under the consumed allowance.

Original run evidence:

- `evidence/native-fixture/`: inventory SHA256
  `5a2450a6032f86f9d19bf0aea5dbc11a1d605a7c76d1a278e98be96089206573`;
  synthetic fixture captures only.
- `evidence/readiness-review/readiness-review.md`: independent readiness approval
  SHA256 `077b72768a651d8fb1670f82bfa4395012098ec4a38a1d91120f9203abd7cb98`.
- `evidence/target-first/inventory.json`: SHA256
  `ee55ed709b5d09bf530a87ef96bb134b6107874731ee708dd89b88dfbd84b0f3`;
  immutable failed result, normalized helper facts and invocation records.
- `evidence/target-first-review/target-first-review.md`: independent outcome review
  SHA256 `dcd81b8cdd556551d7d8755b06fe0a6112b16c721199821829f9823e9060dd72`.
- `evidence/local-diagnosis/loopback-first.json`: SHA256
  `bb6cd0fdb6bd68fa110a32139736da2e90c1e32e54ceb4c149d464aaf2f01502`.
  Synthetic input to an owned loopback TLS-closing endpoint established Open
  dispatch, generation change, disposed media failure and graceful Quit. It does
  not reproduce or explain the original target failure.

At **01:30:37 UTC / 09:30:37 Asia/Shanghai**, a read-only normalized macOS console
query reported a locked desktop. This describes that later observation only;
it does not prove the desktop state or cause during the target invocation.
No UI, lock setting, trust store or runtime was changed.

## Diagnostic repair and verification

Run `20261113T013138Z-2186fc680235` freezes spec/task hashes
`7f5545ca32ec410300c7b842099d834aa0818bb970f1907750d2e9efd7d4195f` /
`6b3079a3922551dc9fff996ae83de8f8b67d8590493248592060d651299c367c`.
Independent architecture review SHA256
`65bd3173d45e1faa7070e7f7f26e8bc40c26f664f2920f9270bb622ce676a1ca`
approved the bounded driver/doc scope. Base HEAD
`d846e667c537c06a1bcf100e6aac9486c29aa3d2` is intentionally dirty and is not the
candidate identity. Existing user work remains; no commit or remote action ran.

The driver adds separate `session_failure` and `cleanup_failure` objects. Each
records a fixed execution phase and safe error code. Optional last snapshot and
resource samples retain their own phase and elapsed time from a common driver
clock. Already received trees supply the samples; there is no new RPC/action,
open, retry, target request or deadline change. Complete fields, enum values,
numeric bounds, duplicates and nulls are validated before retention. Unknown
fields and case aliases cannot override retained values. Missing/malformed samples
remain absent or preserve the earlier valid sample with its original timestamp.
Raw trees/labels, native output, descriptors, locators, headers/bodies and target
frames are excluded. Primary failure survives later forced cleanup.

The new regressions first failed on the previous driver, reproducing missing
failure evidence and partial/null snapshot acceptance. The initial result and
exact test are retained as `evidence/regression-before.log` and
`evidence/regression-before-test.go`. The repaired targeted race suite passes
40 tests/subtests, including real synthetic-child forced reap, loading/hidden-ready/
picture-pending timeouts, redaction and stale sample provenance. Existing success
and cancellation checks remain. No real-service request or native media execution
is part of this repair.

The full recorded six-stage `go run ./cmd/dev check --race` passes, including
spec validation, formatting, tidy, vet, uncached race tests and builds. Its log
SHA256 is `f4b60136f17760e6d9bfb516a4f89786dbd257466498539c983dbf61369f9da8`.
The original independent finding author approves the exact repair and technically
closes **TGT-V2-002**, with no new actionable code findings. Review SHA256:
`4c1e3e08cea4da506167411a7e2d918848b09871283bcb299bcdd2622fc6c31e`;
review inventory SHA256:
`33ef55fe8fbb30bf81f1ddb8fc65af569f688a5e921d326e7469b7d402010040`.
These and all inventory members are retained in `evidence/independent-code-r0/`.

The reviewer independently passed the exact driver suite's 40 tests/subtests and
57 additional malformed-input/redaction/provenance/cleanup tests, and reproduced
selected expected failures against the preserved earlier driver. All 184
candidate files and modes, 15 scoped changed paths, 107 relative documentation
links, unchanged helper sources and four build/runtime pins were verified.
The reviewer inspected the complete controller check and independently ran the
affected focused suites; no reviewer native target or service request occurred.

| Exact reviewed artifact | SHA256 |
| --- | --- |
| 184-file candidate-r0 manifest | `c5532c9148072beff8667e0651fca2234d1bd6b09015e253524069471b1cf19d` |
| Candidate-r0 archive | `968fa0125e89944ed192727be5c0d14e9b2bf52962e7e9e25c7be7acffd0c188` |
| Rebuilt target-smoke | `348a063719bca3bbe3a333d232f29194111000151c5d8ccdb7931ab29b15013a` |
| Unchanged target-native | `8a9c542e57d9580d0991bf2825ead3a6a6eefaf79309235be6588fe70587cdf2` |
| Unchanged target-refresh | `7f0c6039c9da6c64a46bff4453c6b1560112aed07f8bb747005cac0abdd68574` |
| Unchanged runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |

The rebuilt driver is `.agent/controller-scratch/target-failure-evidence/target-smoke`;
the unchanged app/helper remain in
`.agent/controller-scratch/target-host-removal/builds-final/`. Runtime remains
`.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`, with all 30 artifact hashes
verified for the original invocation. Recheck actual pins before future use.
The final descriptive-only handoff delta, manifest/archive and reviewer identity
confirmation are retained separately in this run's evidence; code/test bytes
remain those of candidate-r0. Neither base HEAD nor the earlier manifest silently
identifies later documentation edits.

## Remaining M1 work

TGT-V2-001 remains open. On an unlocked desktop, the controller must prepare a new
bounded assignment/readiness disposition using the reviewed rebuilt driver and
unchanged pinned app/helper/runtime before another target invocation. Preserve
its first result; do not retry under the exhausted previous allowance. Use the
new phase/state evidence to diagnose any repeat failure before changing behavior.

After required target evidence passes, independent PLAY-AC-04/final adapter
assessment and explicit owner M1 acceptance remain. Prior independently reviewed
combined/separate fixture audibility and cleanup closures remain valid for their
recorded candidates. Calibrated physical sync/rate measurements were never run
and remain outside M1; Windows is optional P2. Real IME, packaging/signing and CI
for this dirty candidate remain not-tested. The provisional mygo preview remains
the default; no M2/P0/full-shell work is implied.

All referenced `.agent/` evidence is ignored, local-only and not backed up by Git.
No target descriptor, signed locator or media recording is retained. Preserve the
run archives/reviews before deleting the checkout; remote publication or artifact
retention elsewhere requires separate authorization.
