# ADR 0005: macOS required; Windows P2 / nice-to-have

> Subsequent owner decision, 2026-11-12: [ADR 0006](0006-m1-measurement-scope.md)
> removes the calibrated physical sync/rate requirements retained below. Those
> measurements were never performed and are not a mandatory later gate. This
> dated platform decision and its original review remain historical evidence.

- Date: 2026-11-12
- Status: owner-selected platform priority
- Scope: current product roadmap, M1, M2 and the P0 draft

## Decision

The owner asked to make Windows a P2 / nice-to-have platform and update M1
and related documents. macOS is the required platform for current delivery.
Windows is deferred P2 and does not block M1 acceptance, M2 native preview
packaging, or the complete P0 product loop. P2 here describes product priority,
not review-finding severity. No Windows delivery date is committed.

Windows remains a possible later platform. Its unavailable native capabilities
stay `not-tested`; this decision establishes neither a passing result nor
Windows support. Future Windows work needs an explicitly scoped contract,
actual runtime access, loader/dependency validation, native UI/playback and
resource checks, and package smoke evidence before any support claim.

## Requirements retained

All functional and resource requirements still apply on macOS, including
authorized target media, required request metadata and separate streams,
responsive loading/cancellation, complete teardown, physical audio, calibrated
A/V synchronization and actual output rate. Final independent adapter review
and owner M1 acceptance remain required. This priority decision does not accept
M1, select a production player, approve the detailed P0 architecture or start
M2/P0 implementation. Login and read-only danmaku remain P0 features.

Keep the configured Linux/macOS/Windows Go CI checks and completed M0 evidence.
Those checks validate development-tool/source compatibility; they do not prove
native Windows playback or installable product support. No CI, check policy,
runtime code, dependency, smoke deadline or measurement oracle changes here.

## Contract and evidence precedence

This decision supersedes only the Windows acceptance requirement in ADR 0002,
ADR 0004, and earlier milestone/specification/handoff/assignment wording.
Current stage-zero criteria, playback specs 0002/0005/0006/0007, the P0 draft
0004 and task graphs reflect that priority. Earlier reports, completed review
packets, frozen run inputs and failures retain their original meaning and
candidate identities. Historical Windows blockers are no longer current
acceptance blockers; they are not retroactively passing results.

The [current handoff](../../reports/0018-platform-priority.md) records the
scope review and validation for this amendment. The earlier
[output architecture review](../../reports/0017-output-plan-review.md) remains
the evidence for its original measurement contract; this amendment does not
change that method or imply new physical measurements.
