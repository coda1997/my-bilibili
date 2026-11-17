# Preserve the last playback intent

Spec ID: `0017-playback-intent`
Status: proposed for independent architecture review; implementation requested
by the owner on 2026-11-17 after selecting and reproducing issue #10.

## User scenario and goal

Rapid Play/Pause or Toggle actions must leave playback matching the last user
intent even while the engine reports an older sampled state. Track the existing
[issue #10](https://github.com/coda1997/my-bilibili/issues/10), M2-EXT-001.

## Scope and non-goals

Repair `Session.play` and its application-layer regression tests. Preserve the
existing M2 public interface and lifetime contract. No engine, UI, transport,
queue, timer, dependency, check-policy or CI changes. Seek overload (#11),
SelectPart behavior and broader control redesign are separate work.

M2 PR #9 is merged at base ff27f1db5946eb9ebccbee8c3548a971dd112e85; its three
platform checks passed. This bounded follow-up implements an existing control
contract; it does not assert new M2 or M3 milestone acceptance. The M3 trial is
manually invoked by the owner. No event listener, bot trigger or scheduler is
created. One controller owns issue #10; no competing local run was listed.
Remote publication, issue closure and merge require separate explicit authority.

## Boundaries and shared contracts

The owning package is `internal/app`; desktop callers use Player.Play, Pause
and Toggle. Session consumes the plain `contract.Controller`/Engine boundary.
Engine commands may enqueue without immediately changing Snapshot. The
controller samples on its existing clock; sampled state is an observation,
not confirmation that an opposite pending intent can be discarded.

Retain validation of usable states, terminal replay, Loading intent storage,
same-intent deduplication and one-time readiness initialization. Once ready,
each changed Play/Pause intent is sent through Controller.Command even when
the sampled state already matches it. Keep chosen intent and observed state
separate. No new concurrency, queue or exported types are needed.

All calls remain on the owning UI thread. Existing command failures propagate
as sanitized errors and dispose via Controller; cancellation, generation
replacement, terminal Close, 50ms polling and 200ms cancellation bound remain
unchanged. No network, credentials or native media are needed for this repair.

## Acceptance criteria

### INTENT-AC-01

Given a paused or playing engine with deferred command consumption/sampling,
two opposite actions through explicit Play/Pause or Toggle enqueue both changed
intents in order. Consuming them and sampling leaves playback matching the last
intent, with the same live engine and no unintended reopen. The regression
fails on the original code for both starting states.

### INTENT-AC-02

Repeated identical requests do not enqueue duplicates while samples lag.
Loading stores the latest intent and applies it once at readiness. Existing
startup, replay, replacement, cancellation, close and failure behavior remains
covered. A failure from the newly forwarded opposite command is propagated,
sanitized and disposes once rather than being hidden as success.

### INTENT-AC-03

The exact local candidate passes targeted race tests and the full repository
race check, and receives independent correctness/regression review. Preserve
old-code failing evidence and a candidate snapshot. Prepare a sanitized handoff
and PR draft. No baseline CI, native playback or public release claim is made.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| INTENT-AC-01 | Deferred-engine regression for direct actions and Toggle; restore original Session with Go overlay | Synthetic engine, real Session/Controller, local macOS | Before/after command order and final state; old-code FAIL |
| INTENT-AC-02 | Same-intent deferred regression, opposite-command failure; existing app lifecycle/readiness suite | Fake clock and engine, local macOS | Targeted race PASS |
| INTENT-AC-03 | `go run ./cmd/dev check --race`; independent review of exact snapshot | Local pinned Go; no native library loaded | Full check and review receipt; handoff/PR draft |

Native UI/adapter code is unchanged. Deterministic Engine-boundary tests prove
the application command contract; they do not establish native stress behavior.
Native playback and new Linux/Windows runtime/remote CI remain not-tested here.
Remote CI becomes a publication gate if publication is subsequently authorized.

## Risks, rollback, and open decisions

Forwarding previously dropped opposite commands increases correct command
traffic. Queue saturation remains separately tracked as #11, and is not solved
by this repair. Preserve same-intent suppression; do not broaden into queue
coalescing. Reverting the scoped source/test patch restores the prior behavior
without data migration. There are no public contract or policy changes.

## Review and evidence

Use separate architecture and code review passes by an independent context.
The existing high-severity external claim needs a candidate-specific independent
ruling; do not claim to close Copilot's original thread or the remote issue.
R1–R9 apply: at most three formal reviews per phase/task, three automatic fixes,
stop on three repeated failures; preserve all counts across continuations.
Original defect/control failures are reproduction evidence, not failed repairs.
No new issues are planned. Use #10 for the scoped inventory and #11 for its
existing deferred concern. Evidence belongs to the controller's local run.
Durable report: `reports/0029-playback-intent.md`.
