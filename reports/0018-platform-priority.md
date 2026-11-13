# M1 platform priority: macOS required, Windows P2

Date: 2026-11-12. Run: `20261112T132021Z-81fe26701fa5`.
Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.
This base is not the identity of the changed candidate.

The owner made **Windows P2 / nice-to-have**. macOS is the required platform
for M1, M2 and the current P0 roadmap. Missing Windows runtime or package
evidence no longer blocks those milestones; Windows stays `not-tested` and
has no promised delivery date. [ADR 0005](../docs/adr/0005-platform-priority.md)
records the decision and supersedes earlier Windows acceptance requirements,
including those in historical handoffs 0014 and 0017.

## Delivered

Updated the README, milestone/architecture/testing guidance, active playback
issue replacement criteria, experiment guide, playback specs 0002/0005/0006/0007,
and P0 spec/task graph/plan. The unissued P0 review packet now identifies the
scope amendment; earlier ADRs retain their dated decisions with supersession
notes. P0 login, protected sessions, discovery, playback, danmaku, local history
and macOS preview packaging remain required. The detailed P0 contract remains
proposed, not approved for implementation.

Historical reports, completed review packets, frozen run inputs and failures
are preserved. Existing Linux/macOS/Windows Go CI remains unchanged. This
continuation changes documents and task-check wording only; playback code,
native libraries, dependencies, fixtures and measurement thresholds are unchanged.
The prior standing native sub-agent authorization remains in effect.

## Verification and identity

Controller `go run ./cmd/dev check --run 20261112T132021Z-81fe26701fa5 --race`
passes all six stages on macOS arm64 with Go 1.27.3 and the existing offline
module cache. `git diff --check` and local Markdown target checks also pass.
Independent native reviewer `/root/platform_scope_review` **approved** the exact
platform amendment with no actionable findings. The reviewer verified all 132
candidate files, all seven specs through the actual Go validator, 104 relative
Markdown targets, whitespace and unchanged task dependencies/write paths/AC
bindings. The full controller race log was inspected, not rerun by the reviewer.
The owner's direct instruction supplies platform-priority authority; independent
review supplies the separate technical consistency decision.

| Reviewed artifact | SHA-256 |
| --- | --- |
| Source manifest | `784ab924f762eff6317b1a9c535dbf34ca4bd6fb1e1adfd02ed3a45c1ed47913` |
| Source archive | `b465a411fc139791ac2a7124d6f91da56e4e39de0d5eaf1deb242b2c3bf3a184` |
| Amended 0007 spec | `3fef448a2fba2a96d8d4da5815a38c7a58cb2587c3b9ee504c2c68ad269767ac` |
| Unchanged 0007 tasks | `341415c79cac6f6b93f9aaa4795284c743f26b3093a98cc7946f55864e766323` |
| Independent review | `c6156bb82710ffec01e992e3479955e0143d757a194114adbde65b628cb9fbf2` |

The manifest also identifies all other amended spec/task files. Only this report
receives the review result after that candidate; the final `handoff-candidate/`
archive/manifest records that documentation-only delta. Returned review artifacts
were imported after hash verification. No new native playback, physical capture,
CI or packaging result is claimed. Execution outcome: documentation amendment
complete; M1 remains unaccepted.

The run's ignored `evidence/` directory retains the pre-amendment baseline,
exact review candidate, scoped diff, checks and returned independent review.
These artifacts are local-only and must be preserved before deleting the
checkout; this report is the sanitized durable conclusion, not an uploaded
artifact or release.

## M1 status and next action

The [independent selection review](0014-separate-track-review.md) still records
41 passing macOS controlled-fixture assertions, the approved separate-track
repair, and technical closure of both cleanup findings. The
[output-verification architecture review](0017-output-plan-review.md) records
approval of the measurement preparation contract and closure of its two findings.
The current amendment changes platform priority only.

M1 remains in progress. Required macOS physical audibility, calibrated A/V
sync/output-rate evidence, authorized target-media validation, final independent
adapter assessment and owner acceptance remain open. No Windows runtime or
loader work is needed to close those macOS gates. Physical capture/calibration
is still unavailable in the owner's Mac-only setup. The provisional mygo
fixture preview remains the default; no production player is selected.

Next development slice: the controller assigns OUT-001 under the amended
[0007 contract](../specs/0007-output-verification/spec.md), freezes a new
implementation run and implements the reviewed fixture generator/analysis
helpers. OUT-002 prepares the observation launcher afterward. Actual decoded
fixture checks and independent review remain necessary before using the method;
preparation alone cannot establish physical-output acceptance. M2/P0 expansion
remains gated on M1. No commit, push, PR or remote action occurred here.
