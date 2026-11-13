# Portable playback regression tests

Spec ID: `0013-target-test-portability`
Status: bounded CI repair for the owner's authorized M1 PR publication.

## Problem and source

PR #6 head `d05caa25b4a3a3067a8164c2cc159993371b3e99`, tree
`b01f03b66b5428319d4b596a054294702809e4ca`, was independently approved and pushed.
Initial CI run `43379068124` tests merge `f133cc69fd3c2cc3f433d72cea5b692c882007ff`.
Windows fails six target-smoke tests: environment/CLI fixtures assume POSIX
absolute paths; two forced-cleanup cases observe an early synthetic-child exit;
and the stale-sample test assumes separately observed timestamps are unequal.
The graceful Unix-socket child case passes on Windows. The synthetic hang uses
`select {}`, which can let Go report deadlock rather than stay alive until kill.
Preserve the failing log and distinguish this diagnosis from native playback.

macOS also fails the displayed-buffer test: the fake renderer signals before
returning, while the production render worker still owns the frame mutex and
has not normalized alpha. Draw deliberately uses TryLock and may skip that
frame; the test incorrectly treats a render-entry signal as completed display
promotion. Preserve this failed job as well. Linux passes the initial candidate.

## Bounded repair

Only test fixtures/assertions change. Use platform-native absolute temporary
paths for the environment and CLI path tests, retaining relative-path rejection,
output-before-input validation and redaction checks. Keep the synthetic hanging
child alive using timed blocking, so the existing supervisor timeout/forced-kill
path is exercised rather than runtime deadlock exit. Do not change production
shutdown or test deadlines, skip platforms/tests, or suppress failure outcomes.

Allow equal observation/failure timestamps where the platform clock resolution
makes that possible. Add a deterministic seeded-sample test that requires exact
snapshot/resource value, phase and elapsed-time preservation after malformed
later observations and primary/cleanup failure recording. This retains and
strengthens the no-redating requirement without depending on clock granularity.

Make the displayed-buffer test wait for actual bitmap promotion within its
existing one-second helper bound. Use a test-only renderer barrier to exercise
Draw while the first render is still held; verify it returns without publishing
an unfinished buffer, then release and assert opaque alpha/retained display
pixels. Retain the existing 100ms nonblocking-Draw bound and subsequent frame
ownership checks. No change to production render synchronization or oracles.

Production Go files, runtime, dependencies, fixtures, CI/check configuration,
playback contracts, authorization and native/service behavior are unchanged.
No native/service invocation is needed for test-only edits; verify byte equality
for production files. Existing M1 acceptance is unchanged. Record the CI failure
and repair in a new report and update current navigation; preserve earlier
reports, specs and frozen runs. No new trusted automation policy is adopted.

## Acceptance criteria

### PORT-AC-01

All target-smoke tests run on the configured Windows/macOS/Linux checks without
OS skips or weakened cleanup, privacy or input-validation assertions. Absolute
paths use the current OS; forced cleanup actually kills/reaps the synthetic child;
seeded stale samples keep their exact prior provenance despite malformed data.
The displayed-buffer test explicitly exercises incomplete and promoted frames,
retaining opaque-alpha, buffer isolation and nonblocking Draw assertions.

### PORT-AC-02

Production/runtime/dependency/fixture and CI bytes match the approved M1 source.
Targeted repeated race tests and the full local race check pass; independent
exact test-only code/documentation review approves. Preserve initial failing CI,
source identities and limitations in the durable continuation.

### PORT-AC-03

Push the reviewed repair to the existing PR without force. Verify new exact
head/tested-merge CI on all three OSes, update the PR with both failed and passing
runs, and mark ready only when applicable checks pass and state reads back.
No merge, release, remote-policy change or new product milestone is authorized.
