# M1 scope: physical sync/rate measurements removed

> Supersession, 2026-11-13: the owner removed all target host restrictions in
> [ADR 0007](../docs/adr/0007-target-host-policy.md). Earlier exact-origin approvals,
> historical-origin comparison requirements and any proposed two-host amendment
> are superseded for current work. All valid HTTPS hosts/ports are allowed. The
> dated evidence and stopped outcomes below retain their original meaning;
> see [the current handoff](0022-target-host-removal.md) for implementation and
> remaining M1 gates.


2026-11-12. The owner explicitly removed calibrated physical A/V synchronization
and display/speaker playback-rate measurements from M1 as excessive for this
feasibility milestone. [ADR 0006](../docs/adr/0006-m1-measurement-scope.md) records
the decision. It creates no mandatory later measurement gate or scheduled check.

**These measurements were never performed.** They remain `not-tested` and are
now outside M1 scope. Native configuration/playhead observations and human
audibility attestations do not establish physical timing accuracy.

## Current evidence and remaining work

| Requirement | Evidence / disposition |
| --- | --- |
| Native macOS fixture playback, controls, headers, separate tracks, cancellation and cleanup | Independently reviewed 41-assertion capability stage in [report 0014](0014-separate-track-review.md); both cleanup findings closed |
| Combined/separate fixture audibility | Two positive owner attestations independently reviewed in [report 0016](0016-output-verification.md); limited to prompted 1× sessions, all partial outcomes preserved |
| Calibrated physical sync and actual display/speaker rates | Never measured; not-tested, outside M1 scope; no mandatory later check |
| Windows runtime | Not-tested, optional P2 and nonblocking |
| Authorized target-media validation | Incomplete: [report 0019](0019-authorized-target-playback.md) records unknown permission and the reviewed request stop condition |
| Final independent adapter assessment and owner M1 acceptance | Pending; this scope amendment is not either decision |

All native control/resource requirements remain. The provisional mygo preview
remains the default. M2/P0 and the full shell remain gated on M1. Historical
reports and frozen inputs still describe their original candidates and criteria;
their former physical-measurement blockers are superseded only by ADR 0006.

## Amendment and verification

Updated current milestone/navigation, active replacement/output/target contracts,
task check descriptions, known-issue replacement criteria and the experiment
guide. ADR 0005 retains its dated decision with a supersession note. Existing
measurement helpers and their validity tests remain unchanged; retaining them
does not require performing measurements. No source/runtime/media change or
new native playback result is claimed.

Independent reviewer `/root/measurement_scope_review` **approved** the exact
scope amendment with no blocking findings. All 155 candidate files and the full
15-file amendment were verified. All 66 Go/module files are unchanged; task IDs,
dependencies, write paths and acceptance mappings are unchanged. The controller's
`go run ./cmd/dev check --race` passed all six stages on macOS arm64 with Go
1.27.3. The reviewer inspected that completed log rather than rerunning it.
All 105 relative Markdown links and whitespace checks passed.

New run `20261112T142947Z-583ce1b082f8` freezes the reviewed 0007 amendment.
Base HEAD is `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty;
the base alone does not identify this candidate.

| Reviewed artifact | SHA-256 |
| --- | --- |
| Candidate manifest | `48d24b21917c1296613f06f17bd47bdc6004dd526392b48d5e800c1d43b61039` |
| Candidate archive | `d1d0c89713ae76cfb4bc9787d5701a3f90182264106289ace7128493ae4c3e9b` |
| Full amendment diff | `8d2c4973bcb1ecba2d23088cc94860b1ead0b1cadeb4538a862d6a02a9e82d26` |
| Independent review | `5eee3bda66e3d1fd71c491603688d206334fe02424ea552488f3c5344b47f5b2` |
| Review inventory | `765ab3c9f4f2f252a13c09fcdf26d380d26b9431b737430b5c198029f494fe05` |

The run retains `evidence/reviewed-candidate/`, the complete check log, affected
contract identities and hash-verified `evidence/independent-scope-r0/` artifacts.
Only this report records the disposition after review; `evidence/handoff-candidate/`
identifies that documentation-only delta. These ignored artifacts are local-only;
this report is the sanitized durable conclusion, not a remote attachment.
The owner's explicit instruction supplies scope authority; the independent
review supplies technical consistency approval. Neither is M1 acceptance.

## Next action

The scope amendment is complete. Resolve the target-media preflight with an
interpretable ordinary authorized grant or a separately reviewed access-validation
method. No further metadata request is allowed under the existing packet.
After the required target evidence exists, obtain final independent adapter
assessment and owner acceptance against the amended M1 scope. No external
calibration equipment or numerical physical sync/rate tolerance is required.
