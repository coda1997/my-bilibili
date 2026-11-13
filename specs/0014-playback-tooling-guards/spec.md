# Playback tooling observation and output guards

Spec ID: `0014-playback-tooling-guards`
Status: proposed bounded repair of two independently reproduced PR #6 findings.

## Goal and authority

The owner requested using a subagent to fix both confirmed Copilot comments.
Baseline: `52ecd96b01c4a4df80ebccf2a287bb1d893bd4de`, clean before this packet.
Copilot comments 4560405995 and 4560406047 concern incomplete resource observations
and subprocess output bypassing configured byte limits. Independent verification
is retained in `.agent/reviewer-scratch/copilot-20261113/review.md`.
Continue the existing authorized PR publication after scoped implementation,
independent verification and checks. No merge, release or review-thread resolution
is included. Verification by our reviewer is not closure attributed to Copilot.

## Scope and boundaries

The replacement smoke driver must distinguish an observed zero counter from a
missing observation. Require exactly one resource observation with all three
exact field names `Cores`, `Renderers`, `Workers`, each a non-null nonnegative
int64 JSON integer. Reject malformed/non-object/trailing JSON, missing fields,
duplicate keys, ambiguous multiple resource nodes, and invalid counter values.
Unknown fields may be ignored but may not supply or override required fields.
Complete zero counters pass; complete positive counters parse successfully but
fail the existing zero-resource cleanup consumer. Preserve transport failures,
RPC/cancellation deadlines and resource equality checks. Keep implementation
local to replacement-smoke; the already-strict target-smoke parser is unchanged.

The media-tool runner must enforce its existing stdout limit and 64KiB stderr
limit through every subprocess copy entry point. Remove unintended promoted
buffer methods or implement guarded copying without recursion. Retained output
must never exceed its limit. Exact-limit output may succeed; any extra byte must
produce the existing `tool output limit exceeded` error and cancel/reap the
owned child. Preserve the 60-second tool deadline, one-second WaitDelay, ordinary
tool errors and caller cancellation. No whole-stream buffering before checking.

No native player, runtime, media fixture, dependency, business interface, CI,
policy, historical report, M1 acceptance or milestone scope changes. Synthetic
offline subprocesses and automation responses suffice; no native/service session
or physical/platform support claim is authorized. UI thread and playback state
contracts are unaffected. Process stdout/stderr copy owners remain independent;
inspect results only after `cmd.Run` joins them as today.

## Acceptance criteria

### GUARD-AC-01

Resource parsing and the actual `zeroResources` consumer reject missing, null,
partial, negative, fractional, overflowing, duplicate and ambiguous observations.
Complete-zero and positive-counter controls preserve valid behavior. Regression
tests fail on baseline and pass after repair without relaxing existing checks.

### GUARD-AC-02

Real finite synthetic subprocess tests cover stdout and stderr separately, with
below/exact/over-limit output, retained byte counts and the specific overflow
error. A generous separate test deadline must not masquerade as overflow.
Caller cancellation/nonzero exit behavior remains covered. Regressions fail on
baseline and pass after repair; no platform skips or timeout weakening.

### GUARD-AC-03

Targeted race tests and `go run ./cmd/dev check --race` pass. An independent
reviewer verifies the exact source candidate against both original reproductions.
Preserve failures and sanitized evidence in a new run and report, with current
navigation updated. Update the existing PR under prior publication authority;
new CI must identify the actual head/tested merge and pass all three OS jobs.
Do not claim Copilot thread closure, native retesting or new platform support.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| GUARD-AC-01 | Table tests through parsing and cleanup consumer | Synthetic automation trees, local race; CI OS matrix | Baseline failure and repaired output |
| GUARD-AC-02 | Actual child stdout/stderr boundaries and failure modes | Offline Go test executable, local race; CI OS matrix | Counts, overflow identity, bounded completion |
| GUARD-AC-03 | Full check, exact source review, remote CI/readback | macOS local, Linux/macOS/Windows Go CI | Manifest/archive, logs, review and PR identities |

## Risks and rollback

Strict parsing intentionally rejects previously accepted incomplete diagnostics;
the current native producer already emits all required counters. Avoid accidental
copy-method recursion and excessive subprocess output in tests. Preserve bounded
capture prefixes without requiring an exact prefix length after overflow. A
reviewed follow-up may revert the scoped repair; do not erase original findings.
No open product decision or new human policy approval is needed for these guards.
