# ADR 0006: Remove calibrated physical measurements from M1

> Acceptance update, 2026-11-13: the owner accepted M1 as completed with libmpv
> playback and mygo UI. See [the acceptance handoff](../../reports/0025-m1-acceptance.md).
> Pending-gate wording below describes this earlier decision's original scope;
> historical evidence and untested limitations retain their meaning.

- Date: 2026-11-12
- Status: owner-selected scope amendment
- Scope: M1 physical A/V synchronization and output-rate measurements

## Decision

The owner considers external calibrated physical measurements excessive for
M1 and explicitly requested their removal. M1 no longer requires external
display/speaker capture, independent capture calibration, measured physical
A/V synchronization, measured display/audio playback rates, or numerical
acceptance tolerances for those measurements. These are not deferred mandatory
gates for M2, P0 or another milestone. There is no scheduled follow-up measurement.

**These physical measurements were never performed.** Their evidence cells
remain `not-tested`, with the reason `outside M1 scope by owner decision`.
Removing a requirement does not turn its missing evidence into PASS. No
calibrated sync, measured physical speed or timing-accuracy claim is supported.

## Requirements retained

Keep native picture and progression, pause/seek, rate and volume controls and
configuration readback, required request metadata, selected separate tracks,
audible sound, missing-audio failure, switching/cancellation and complete
resource disposal. Existing control deadlines and the replacement adapter's
200ms cancellation bound are unchanged. Existing owner audibility statements
remain limited to the combined/separate fixture sessions under prompted 1×;
they do not become measurements of physical speed or synchronization.

Authorized target-media validation, independent final adapter assessment and
owner M1 acceptance remain required. This decision does not accept M1, select
the production player, change the default preview or start M2/P0 implementation.
Windows remains optional P2 under ADR 0005.

## Contracts and evidence

This amendment supersedes the physical sync/rate acceptance requirements in
earlier milestones, specifications, ADRs and reports, including RPL-AC-05 and
ADR 0005's retained-measurements wording. Active specs and current navigation
reflect this decision. Closed runs, frozen inputs, original reviews, failed
outcomes and historical reports remain unchanged and retain their original scope.

The existing fixture generator, analysis helpers, tests and optional capture
procedure remain available. Their validity requirements are unchanged: future
voluntary use cannot claim calibrated results without calibration and valid
evidence. Keeping those tools is not a requirement to use them. No maintained
code, runtime, media, dependencies, checks or CI policy changes are included.

The [scope handoff](../../reports/0020-m1-measurement-scope.md) records validation,
independent consistency review and the remaining M1 work.
