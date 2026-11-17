# Keep rapid seeking bounded

Spec ID: `0018-seek-queue`
Status: proposed for independent architecture review. Owner requested completion
of the active issue, PR publication, passing CI and merge on 2026-11-17.

## User scenario and goal

Continuous progress-slider input must reach its latest requested position without
destroying playback merely because Seek requests arrive faster than native
command replies. Existing issue #11 / M2-EXT-002 is the only assigned bug.
Base: a11ca09a082306f6bf0b27691c6039452d922c0d.

## Scope and non-goals

Own the bounded pending command queue inside experiments/playback/mpvprobe.
Change engine.go and focused engine regression tests only, plus this contract
and the handoff/navigation. No UI, Session, Controller, exported interface,
poll interval, native API, dependencies, checks or workflow policy changes.
No live-service experiment, package release or full-client milestone work.

## Boundaries and shared contracts

Desktop slider changes call Session.Seek, which calls the plain Controller,
which forwards to Engine.Command. Source currently queues 32 native argument
lists and sends at most one per 50ms poll after the prior reply. Queue overflow
returns Media; Controller then disposes its engine. A controlled reproduction
must establish this complete path before implementation. This is the existing
M2-AC-03 interaction and resource-lifetime contract, not a new public API.

Replace only the adapter-local queue with a bounded FIFO protected by the
existing engine mutex. Consecutive unsent Seek requests at the tail represent
superseding absolute positions: replace that tail with the latest position,
including when the queue is full. Never change an already-sent command. A
non-Seek command is an ordering barrier: retain all such commands and do not
coalesce seeks across it. Keep the 32-entry limit and Media failure for genuinely
full queues that cannot coalesce. This solves continuous seek bursts; arbitrarily
fast alternating unrelated controls retain their existing overload behavior.

The native owner pops only when its pending reply is zero. Keep command IDs,
load/reply ordering, sanitized native errors, validation, cancellation and
out-of-band Close. Do not hold the mutex during native calls. No new goroutines
or timers; UI ownership and render/native cleanup order stay unchanged.
Close marks the engine closed before shutdown; later commands return IsClosed.
Pending commands belong only to their engine and cannot leak into a new one.

## Acceptance criteria

### SEEK-AC-01

With a real Controller and adapter over controlled native replies, more than
32 consecutive different Seek targets while consumption is held do not return
Media or dispose the engine. Releasing replies sends the latest target, updates
the sampled position, and retains the same live engine/generation. The original
engine must fail this regression with recorded overflow/disposal evidence.

### SEEK-AC-02

Seek coalescing is bounded and preserves intervening Play/Pause/Volume/Rate
commands in FIFO order and the latest target on each side of those barriers.
Replacement at a full queue with a Seek tail succeeds; a full queue with a
non-Seek tail retains explicit Media failure. Validation, post-close rejection,
pending-reply ordering, native error propagation, cancellation and complete
resource cleanup remain covered. New mechanisms need discriminating tests.

### SEEK-AC-03

Targeted race tests and the complete local repository race check pass for the
exact candidate. Independent architecture and code review supply dispositions
and a fresh ruling on M2-EXT-002. Publish a Draft PR, verify Linux/macOS/Windows
CI for its final head/tested merge candidate, then merge using the owner's
explicit instruction, without bypassing protection or changing settings.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| SEEK-AC-01 | Held replies, real Controller/adapter, final property sample; original-source failing control | Fake native API, local macOS | Before/after logs, identity, disposal |
| SEEK-AC-02 | Barrier order, bounded/full cases, existing adapter/controller/app lifecycle tests | Offline fixtures, race detector | Targeted results and discriminating controls |
| SEEK-AC-03 | Full dev check --race, independent exact-candidate review, three-OS PR CI | Go 1.27.3 | Run, review, PR/CI receipts |

No installed native library is needed to verify the changed Go command queue.
Actual native UI stress, physical playback and packages remain not-tested;
the fake API covers the actual adapter ownership loop without platform claims.

## Risks, rollback, and open decisions

Intermediate absolute seek positions are intentionally superseded only before
dispatch; controls retain order. The queue stays small and bounded. Revert the
scoped source/test change to roll back; there is no data migration. No policy
adoption or unresolved milestone decision is required.

## Review and evidence

Controller /root owns local runs. Independent reviewer /root/seek_reviewer_readable receives
the exact contract and candidate, not implementation authority. R1-R9 apply;
at most 3 formal rounds per phase/task and 3 repair iterations; preserve all
counts. Inherited issue inventory is #10 closed / #11 assigned, budget 2/5,
no new issues planned. Do not attribute fresh verification to Copilot or claim
closure of its original remote review thread. Durable report:
reports/0030-seek-queue.md. Owner authorization covers this issue's publication
and merge once gates pass; no unrelated remote work or release is authorized.
