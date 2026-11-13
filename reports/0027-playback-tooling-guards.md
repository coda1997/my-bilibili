# PR #6: playback tooling guard repairs

- Date: 2026-11-13; controller `/root`, implementer `/root/playback_guards_fix`.
- Contract: [0014-playback-tooling-guards](../specs/0014-playback-tooling-guards/spec.md), GUARD-001/002.
- Baseline: `52ecd96b01c4a4df80ebccf2a287bb1d893bd4de`.
- Run: `20261113T071234Z-501346f8b044`.

## Findings and scope

Copilot comments 4560405995 and 4560406047 on PR #6 were independently reproduced
by `/root/copilot_findings_review`. Empty/null/partial resource counters could
satisfy replacement-smoke cleanup checks. The output tool runner accepted a real
synthetic child's 1MiB stdout despite a 4KiB cap, and separately 1MiB stderr
despite a 64KiB cap, both without error. The original findings and evidence are
preserved in this run. Existing scoped tests passed despite these defects.

The resource observation flaw does not establish a native leak or a false
historical playback result: the current native producer emits complete counters.
The already-strict target-smoke parser is separate and unchanged. M1 owner
acceptance and prior native evidence remain recorded in reports 0024/0025.

## Repair

Replacement-smoke now requires one unambiguous resource node and all three exact
counter names with non-null nonnegative int64 values. Missing/partial objects,
nulls, duplicate keys/nodes, malformed/trailing data and invalid numeric values
fail observation. Unknown fields cannot supply or override exact counters.
Complete positive counters still parse and fail zero-resource cleanup; complete
zero counters pass. Transport failures retain their original safe diagnosis.

The subprocess capture wrapper now holds a named buffer, removing the promoted
`ReadFrom` entry point that bypassed its guarded `Write`. Stdout and stderr retain
their existing caps. Overflow cancels the owned child and returns the specific
output-limit error after process/copy cleanup. Production tool timeout and
WaitDelay remain 60 seconds and one second; native playback/RPC deadlines are
unchanged.

## Verification and independent review

- Desired regressions were added and run before production changes. The baseline
  failed on missing/null/duplicate/ambiguous resource observations, a case-alias
  override, stdout 4097 bytes with a 4096-byte cap, stderr 65537 bytes with a
  65536-byte cap, and a 10,250,000-byte flood accepted without an error.
- Both full affected packages pass with the race detector on Go1.27.3,
  macOS arm64. Tests cover below/exact/over-limit output on each stream, retained
  bytes/content, the precise overflow error, caller cancellation/deadline and
  nonzero exit. The existing two-second cleanup assertion remains in place;
  generous caller deadlines separate overflow from timeout.
- Independent scope review: `/root/copilot_findings_review`, PROCEED, SHA256
  `d577290a0cdb8db6ec75541100e5cd79cc6ed32192c666c9c55fb833ee90e3ab`.
- Full local `go run ./cmd/dev check --race` PASS on Go1.27.3, macOS arm64:
  spec contracts, formatting, tidy-diff, vet, all-package race tests and build.
  The log is retained in this run's `evidence/full-check-console.txt`.
  Exact-candidate independent review is required before commit/push; the source
  manifest/archive and reviewer decision are retained in the run and referenced
  by the PR, followed by new head/tested-merge CI.

The independent reviewer verifies the repair against Copilot's original findings;
this does not impersonate Copilot or resolve its remote review threads. Preserve
both original comment identities and their reproductions. Earlier reviews/CI
remain evidence for their recorded candidates. The final PR/run records the new
commit, all three CI results and publication readback, not a baseline green run.

## Continuation and limits

Complete independent verification and the existing authorized PR update. No merge,
release, policy change, native/service invocation, or new milestone implementation
is included. Only the two tooling implementations and regression tests change;
player/runtime/dependencies/media fixtures/CI and historical reports are preserved.
No observed native leak or false historical41PASS result is inferred. Windows
native playback remains optional P2/not-tested; M2/P0 remains unstarted.

Evidence is local under this run's `evidence/worker/`, `original-findings/` and
review/candidate artifacts. Ignored `.agent/` evidence is not backed up by Git.
Remote CI check artifacts expire after 14 days and are not app packages.
