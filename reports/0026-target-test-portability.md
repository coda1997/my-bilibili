# M1 publication: portable regression tests

- Date: 2026-11-13; controller/author `/root`.
- M1 remains **completed and accepted by the owner**, with libmpv playback and
  mygo UI; [acceptance record](0025-m1-acceptance.md).
- PR: #6, `codex/m1-macos-playback` into `master`.
- Repair contract: [0013-target-test-portability](../specs/0013-target-test-portability/spec.md),
  PORT-001; run `20261113T063534Z-43d639773510`.
- Starting source: `d05caa25b4a3a3067a8164c2cc159993371b3e99`, published tree
  `b01f03b66b5428319d4b596a054294702809e4ca`. The repair candidate is identified
  separately by this run's source manifest/archive and final PR commit.

## Initial CI and bounded repair

The initially reviewed publication candidate passed the full local macOS race
check and was pushed as draft PR #6. GitHub run `43379068124` tested synthetic
merge `f133cc69fd3c2cc3f433d72cea5b692c882007ff`; all three downloaded artifact
manifests confirm that exact clean source, whose tree equals the reviewed head.
Ubuntu passed, Windows and macOS failed. Publication run
`20261113T062323Z-35d00be61e9e` retains both failed job logs, all three CI artifacts
and the initial result; it closed `failed` for the publication gate. This does
not reverse the owner's M1 acceptance or turn failed CI into a passing result.

Windows exposed POSIX-only absolute-path fixtures, unexpected early exit of the
synthetic hanging child, and an assertion requiring separate clock observations
to have strictly different timestamps. The early exit is observed; Go deadlock
detection from `select {}` is a plausible cause, not a confirmed native diagnosis,
because the synthetic child's output was intentionally discarded.

macOS exposed a displayed-buffer test race. The fake renderer signaled before
returning, while alpha normalization and publication were still pending. Draw
correctly uses a nonblocking lock and can skip that frame; the test read the
unpromoted front buffer. The original test reproduced **38 failures in 50** local
race-enabled repetitions with `GOMAXPROCS=2` before repair.

Only three test files change:

- Environment/CLI fixtures use platform-native absolute temporary paths. Relative
  path rejection, fresh-output-before-input behavior and redaction remain tested.
- The synthetic hanging child uses a timed wait loop so it remains alive for the
  supervisor to kill and reap. Shutdown and RPC deadlines remain unchanged.
- Adjacent observation/failure timestamps may be equal. An additional deterministic
  seeded-sample regression requires exact preservation of snapshot/resource values,
  phases and elapsed times through malformed later observations and both failures.
- The frame test holds the first fake render behind a barrier and requires Draw
  to return within the existing 100ms bound without publishing it. After release
  it waits for actual promotion within the existing one-second helper bound,
  checks opaque alpha, then verifies a subsequent render preserves retained pixels
  and Draw still does not wait on a held render mutex.

Production Go, native runtime, dependencies, media fixtures, CI configuration,
playback contracts and cleanup deadlines are unchanged. No platform skip, native
session, service request or new playback/support claim is introduced. Earlier
reports and frozen inputs remain unchanged. The source delta is tests, current
navigation and this new scoped publication continuation only.

## Verification and review

- Repaired displayed-buffer race test: **100/100 PASS**, `GOMAXPROCS=2`, race
  detector; `evidence/display-repaired-r1-stress.log` for the final correction.
- Target-smoke package: **three repeated race runs PASS**;
  `evidence/target-tests-race.log`.
- Full local six-stage race checks: **PASS**, macOS arm64 / Go 1.27.3.
  Final test refinements also passed; exact-candidate logs and source identity
  are retained in this run.
- Independent scope review: `/root/default_libmpv_review`, PROCEED, SHA256
  `c70a319dce0cfba6a8cce451bfcdbb1bbb7dbcee821d4cfd12289567d01e5a17`.
  Exact test/documentation review and final source identity are retained in the
  same run before commit/push; the reviewer is separate from the author.

Independent test review found PORT-REVIEW-001 (P2): the UI tester eagerly draws
in its constructor, outside the intended timeout boundary. The correction
constructs it with drawing disabled until the timed test begins, and releases
test barriers/mutexes plus joins drawing work before failure cleanup. The
100ms assertion is unchanged; cleanup time does not extend it. The original
finding and the reviewer's exact-candidate verification/closure remain in this
run, including isolated mutation probes rather than changes to production code.

The PR and run record final source head, tested merge revision, all three new CI
results and ready-state readback. Initial failing CI remains linked. No green
baseline or macOS-local test substitutes for repaired Windows/macOS/Linux CI.
Local `.agent/` evidence is ignored and not backed up by Git; remote CI artifacts
expire after 14 days. They are check records, not native app packages.

## Continuation and limits

Finish authorized publication by pushing the reviewed repair, verifying all
three exact-candidate CI jobs, and marking PR #6 ready for review. No merge or
release is authorized. M1 acceptance and its native evidence remain as recorded
in reports 0024/0025. Windows native playback stays optional P2/not-tested;
Windows Go tests do not establish native support. Physical calibration remains
unperformed and outside M1, with no mandatory later measurement gate.
M2/P0/full-shell implementation remains unstarted; the next product step is
separately scoped M2 planning and review.
