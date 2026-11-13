# Bounded target-driver failure evidence

Spec ID: `0010-target-failure-evidence`
Status: proposed bounded repair of independently reported TGT-V2-002; independent architecture review and freeze required before implementation.

## User scenario and goal

The owner requested continued M1 evaluation after removal of all target host restrictions. The first version-2 target invocation failed its readiness/picture gate and graceful exit. Its trace omits the last validated state and phase, preventing diagnosis. Preserve that failure and add enough safe driver evidence to distinguish loading, hidden-ready, picture pending and cleanup failures on a future assigned invocation.

## Scope and non-goals

Only the target-smoke driver, its offline regressions and current descriptive documentation change. No app, adapter, helper, runtime, descriptor/host policy, source/rendition/grant, headers, TLS, timeout, acceptance threshold, action sequence or retry change. No external request or native target invocation is authorized by this repair. The consumed invocation remains failed. TGT-V2-001 remains open until separately assigned relevant evidence establishes closure. Windows remains optional P2; calibrated physical measurements remain outside M1.

## Boundaries and shared contracts

The driver owns normalized result JSON. Add a typed diagnostic failure record for primary session failure and a separate cleanup failure record. Each has a code from the existing safe ErrorCode enumeration and a fixed driver phase. Last valid snapshot/resources are optional typed samples with their own enumerated phase and relative monotonic elapsed time; sample time must never be represented as shutdown time. Preserve primary failure even if forced cleanup follows. Record no fabricated Idle/zero-owned sample when absent or malformed.

Obtain evidence solely from trees already received during existing polling. Cache a constant number of typed latest samples; no additional RPC, action, open, media request, unbounded log or deadline extension. Validate complete required snapshot/resource fields, finite/ranged values and known state/error codes before caching. Drop unknown fields rather than serializing them. Raw trees/labels/keys, arbitrary strings, locators, descriptor data, headers/bodies, native output and target images/media remain excluded. Cleanup retains its own phase/code; cached session samples retain their original phase/time even if cleanup has no snapshot. Normal successful capability records and existing exit/check semantics remain compatible.

The controller executes sequentially in the existing checkout and owns run evidence. Reviewer uses a separate native context and scratch. No shared business contract changes; Go-only maintained tooling. Cancellation, UI thread ownership and native lifetime stay unchanged. Driver samples are synchronous with the existing RPC client and require no added goroutines.

## Acceptance criteria

### EVD-AC-01

Given a failed target session, result JSON identifies the failed driver phase and safe error code, and retains only the last complete valid snapshot and resource samples with original phase/time when available. Loading, hidden-ready and picture-pending failures are distinguishable; absent/malformed data is omitted. No private/unknown data is serialized.

### EVD-AC-02

Given a primary failure followed by forced or failed cleanup, both failures remain independently identifiable. Cleanup cannot overwrite the primary phase/code/sample. No action, network request, polling deadline, threshold, retry or native lifecycle behavior is added or weakened. Successful existing driver tests continue to pass.

### EVD-AC-03

Preserve failing offline regressions before repair, then pass targeted race tests and the full repository race check. Independent exact-candidate review decides TGT-V2-002 closure. Durable current docs record the original target failure and precise evidence limits; no target success, TGT-V2-001 closure, adapter approval or M1 acceptance is inferred from this repair.

## Verification matrix

| AC | Procedure | Environment | Evidence |
| --- | --- | --- | --- |
| EVD-AC-01 | Actual driver polling over synthetic automation: loading timeout, hidden-ready timeout, picture pending, no/malformed/partial/duplicate snapshots, hostile extra fields, resource validation and stale sample provenance | Offline Go/macOS, no libmpv | Preserved initial FAIL and subsequent race PASS; normalized JSON assertions |
| EVD-AC-02 | Primary session error followed by actual owned synthetic-child forced reap; success sequence and existing bounded cancellation tests | Owned local pipe/socket/process only | Both independently preserved failures; unchanged actions/deadlines |
| EVD-AC-03 | go run ./cmd/dev check --race; independent review; link/whitespace and exact scope checks | Current dirty source manifest, Go1.27.3 | Full check, immutable manifest/archive and reviewer closure or findings |

## Risks, rollback and open decisions

Normalized JSON additions must not leak arbitrary app strings or misattribute stale samples to shutdown. Missing samples remain unknown. Rollback removes only this assigned driver/doc repair and preserves original user work and evidence. No new owner policy decision is required; a later target attempt needs a distinct bounded assignment/readiness review and cannot reuse the exhausted allowance.

## Review and evidence

Controller/author /root; independent reviewer /root/host_policy_review owns TGT-V2-001 and TGT-V2-002. Base HEAD d846e667c537c06a1bcf100e6aac9486c29aa3d2 is not a dirty candidate identity. Starting maintained candidate is the178-file HOST001 manifest6c265aa620efd888edc634906665c0a313a5cb283c6d7cc232d47536372fd379. Prior attempt run20261113T011610Z-2abfeb37a20c and outcome reviewdcd81b8cdd556551d7d8755b06fe0a6112b16c721199821829f9823e9060dd72 remain immutable. Review packet is assignment.md; controller freezes approved inputs into a new run. Durable continuation is reports/0023-target-failure-evidence.md. No commit/push/publication/default-player/M2/P0 authority.
