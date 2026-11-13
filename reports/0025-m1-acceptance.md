# M1 completed and accepted

- Date: 2026-11-13 (Asia/Shanghai).
- Owner decision: “mark m1 milestone is completed, polish the PR, push and publish it”.
- M1 status: **completed and accepted**. macOS is the required platform; libmpv
  is the default playback backend and mygo remains the UI.
- Publication contract: [0012-m1-publication](../specs/0012-m1-publication/spec.md),
  task PUB-001; controller run `20261113T062323Z-35d00be61e9e`.
- Current product continuation: separately scoped M2 planning. M2/P0 and the
  full client shell have not started; stage zero as a whole is not complete.

## Accepted delivery

The default `cmd/native` launcher uses the reviewed libmpv adapter behind the
existing plain Go playback boundary. `cmd/mygo-native` retains the earlier
player for explicit diagnostics; its known failures remain documented.
Launch requires an explicit absolute `-library` path to the verified runtime.
There is no automatic runtime installation, discovery or fallback. See
[commands](../experiments/playback/cmd/README.md#default-libmpv-playback) and
[ADR 0008](../docs/adr/0008-default-libmpv.md).

The opt-in target path accepts every valid HTTPS host and port. Descriptor v2
and the copied refresh helper share syntax validation; no host allowlist,
origin pin or repinning remains. TLS, source/rendition identity, grant bounds,
request-metadata isolation, expiry and lifecycle constraints remain. The
default fixture launcher is not a general target-URL player. See
[ADR 0007](../docs/adr/0007-target-host-policy.md).

## Acceptance evidence

| M1 criterion | Actual evidence and disposition |
| --- | --- |
| Pinned Go UI and native macOS execution | Evaluated mygo pin and native UI evidence retained in [the original M1 report](0002-playback-macos.md); subsequent default app uses the same UI boundary |
| Fixture capability and cancellation | Default libmpv app: **41 PASS**, two explicit not-tested categories; loading **103.205542ms**, playing **82.0935ms**, both within **200ms**; graceful app/socket/server cleanup, zero native ownership, no forced reap; [report 0024](0024-default-libmpv.md) |
| Runtime identity and automation | All 30 runtime artifact hashes and loaded-image paths match; 300 automation calls with zero errors/drops in that default fixture stage; report 0024 |
| Authorized target playback | Separately bounded anonymous rendition: **8 PASS, 1 not-tested**; picture, selected separate tracks, controls, **87.7575ms** playing cancellation and graceful exit; [target continuation](0021-target-access-method.md#post-repair-target-continuation-2026-11-13) |
| Fixture audibility | Owner attestations for combined/separate fixtures after explicit Play under the prompted 1× setting; independently reviewed as human evidence, not independent hearing or physical-rate measurement; [report 0016](0016-output-verification.md) |
| Adapter decision and cleanup findings | Independent scoped closure of CA-CLEANUP-001/002, approved explicit-audio selection repair, final adapter recommendation and reviewed default switch; [cleanup review](0011-playing-close-review.md), [selection review](0014-separate-track-review.md), target continuation and report 0024 |
| Owner acceptance | Explicit acceptance on 2026-11-13, recorded by this report and current milestone/navigation docs |

The earlier failed target invocation and unknown cause remain preserved. The
later pass closes the current blocker without explaining that earlier failure.
Target loading cancellation was not tested in the single-open target session;
fixture loading-cancellation evidence is separate. No native/service session or
new audibility observation was performed during publication.

## Publication source and verification

The approved 192-file final default-switch source has manifest SHA256
`7422e2757bce7fb043fdf0f20aae614e670ffa50e6b53c236fda762658a26c97` and archive
`3e9381fef65a5e4e454863e8def0dc892b2343a26d94562f0d7698708a443d28`, retained
under run `20261113T023232Z-ffe109b8bd77`, `evidence/final-r1/`.
It was committed as `0ef3ce9`; master documentation commit `c66bdf4` was then
integrated on the feature branch as `0c58024`. The final publication candidate
adds descriptive acceptance/status updates and this publication packet; all
code, fixtures and dependency bytes remain equal to that approved source.
Historical reports and frozen inputs are unchanged.

The base integration preserves both the owner's standing native delegation and
master's **proposed**, unadopted read-only/reviewer-succession clarifications.
Publication does not adopt new workflow/check/CI policy. P0 prerequisite status
is updated without changing its feature acceptance criteria or task graph.

Publication scope review by independent `/root/default_libmpv_review` is
PROCEED with no blocking finding (SHA256
`086a9304fcd2494809fd429ff00eec18fae7b5428277b595b1a8bc0567cf7886`). The
reviewer is separate from controller/author `/root`. Final publication review,
local race-check result and exact manifest/archive are retained in this run;
the PR and run record the final commit identity, exact-head/tested-merge CI
results and publication state. Those remote results are not inferred from prior
baseline CI or from native playback evidence.

The full local `go run ./cmd/dev check --race` passes on macOS arm64 with
Go 1.27.3. All six stages pass; log `evidence/check-1801942342.log` has SHA256
`a72d916397f97de85d2676765e9f33195fa9cdd2b8de6501efc9a08aa8d2ff80`.
The final report and descriptive link corrections followed that check; no code,
fixtures, dependency or task-contract changes followed it. Link/content/scope
checks and independent publication review cover the final documentation.
The optional external skill validator could not run because Python's PyYAML
module is unavailable; no installation was attempted. This is separate from
the passing required Go checks, and no new skill behavior is introduced here.

The earlier default-switch independent code/evidence approval is SHA256
`296febf6abd8d2b3ed89f68cffbade0ad9bf1b7743dcb6f81ad40f651c05bad0`; its final
descriptive confirmation is
`de04667a3467f6fa0e10d76817af92b46c9f8053eb087f59f4ec14b336fb491d`.
See report 0024 for tested-source, native-result and runtime identities. Detailed
`.agent/` evidence and runtime binaries remain ignored, local-only and not backed
up by Git. GitHub Actions artifacts contain check records, not native packages,
and expire after 14 days. Sanitized durable conclusions are in the linked reports.

## Limits and next action

Windows native playback is optional P2 and remains **not-tested**. Windows Go CI
does not establish native support. Calibrated physical synchronization and
speaker/display output-rate measurements were never performed and are outside
M1 scope under ADR 0006, with no mandatory later measurement gate. Real IME,
packaging, signing, full-duration target viewing and target audibility remain
untested. Acceptance does not turn these cells into passes.

The authorized publication action is to push the reviewed feature commits and
publish a PR against the verified target `master`, keeping it draft until its
required checks pass. Exact remote outcome belongs in that PR and this run.
No merge into master or release is authorized. The next product step is an
owner-scoped M2 plan, with independent architecture review and frozen contracts
before implementation; P0 login/danmaku/access feasibility remains separate work.
