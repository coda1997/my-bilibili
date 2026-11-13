# my-bilibili

A Go desktop Bilibili client with macOS as the required platform and Windows
as P2 / nice-to-have, and an experiment in
spec-driven, independently reviewed agent development.

The repository is in stage zero. M0 and M1 are completed and accepted. The owner accepted M1 on
2026-11-13 after completion of the required macOS technical evaluation. On 2026-11-13 the owner selected **libmpv as the default
playback backend**, retaining the mygo UI and existing playback boundary.
`cmd/native` now uses libmpv; `cmd/mygo-native` preserves the old player for
explicit diagnostics. See [the default switch handoff](reports/0024-default-libmpv.md)
and [launch commands](experiments/playback/cmd/README.md#default-libmpv-playback).
See [M1 acceptance and publication](reports/0025-m1-acceptance.md) for the
owner decision and current continuation. Independent
review of the libmpv adapter has now technically closed both the delayed
CoreAudio unload crash and the playing-close delay. The shutdown-order repair
passed independent native cleanup checks, with playing Close at 44–78ms against
200ms, and received scoped technical approval. A subsequent explicit-audio
selection repair is also independently approved. The independently reviewed
default libmpv app passes 41 macOS fixture assertions, with external loading/playing
cancellation at 103.2ms/82.1ms and graceful cleanup. The prior six-scenario cleanup
suite is separate evidence. The scoped anonymous target invocation now passes
picture, separate tracks, controls and graceful cleanup; playing cancellation
takes 87.8ms against 200ms. Independent target and final adapter reviews approve
the scoped technical evidence; the owner accepted M1 on 2026-11-13. Calibrated
physical sync/rate measurements were never performed and are outside M1 scope
under [ADR 0006](docs/adr/0006-m1-measurement-scope.md), with no mandatory later
measurement gate. Windows is
deferred P2 and does not block M1, M2 or P0; see the
[platform decision](docs/adr/0005-platform-priority.md). This is not yet a working
Bilibili client.

Paired-event fixture generation, measurement helpers and the bounded observation
launcher now have independent technical approval after both code findings were
closed. Full local race checks and independent actual fixture generation pass.
The first public-target preflight stopped with permission unknown. A separately
reviewed method and bounded observation now establish an independently accepted
ordinary anonymous rendition for scoped implementation. The previous opt-in target player
and driver have independent scoped approval and passing fixture regression.
The first target refresh stopped before native launch; a later diagnostic
observed a changed video CDN host. On 2026-11-13 the owner removed target host
restrictions: all valid HTTPS hosts and ports are allowed, with no origin list
or repinning. The descriptor and refresh helper are updated together under
[ADR 0007](docs/adr/0007-target-host-policy.md). The first version-2 target attempt
failed readiness/picture and required forced cleanup. An independently approved
driver repair added safe failure evidence. A separately reviewed later invocation
passes eight target capability/cleanup checks; target loading cancellation remains
not-tested in that single-open session, with passing fixture coverage recorded
separately. The original reviewer closes the current target acceptance blocker;
the earlier failure and its unknown cause remain recorded. See the
[current target continuation](reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13) and
[M1 measurement scope](reports/0020-m1-measurement-scope.md). A subsequent normal-app
observation now has positive owner statements of advancing playback and six
audible tones for combined and separate fixtures under the prompted 1× setting
after explicit Play. Independent evidence review approves these scoped human
attestations. The first inconclusive attempt and partial session outcomes are
retained. This does not measure physical sync/rate.

## Start here

- [Current handoff: M1 completed and accepted](reports/0025-m1-acceptance.md)
- [Default libmpv implementation and evidence](reports/0024-default-libmpv.md)
- [Default playback decision](docs/adr/0008-default-libmpv.md)
- [Previous M1 handoff: target PASS and adapter recommendation](reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13)
- [Historical target failure and diagnostic repair](reports/0023-target-failure-evidence.md)
- [Reviewed host-restriction removal](reports/0022-target-host-removal.md)
- [Host policy: all HTTPS hosts allowed](docs/adr/0007-target-host-policy.md)
- [Historical target implementation, stopped refresh and diagnostic](reports/0021-target-access-method.md)
- [M1 scope: measurements removed and remaining gates](reports/0020-m1-measurement-scope.md)
- [Reviewed output preparation and human audibility evidence](reports/0016-output-verification.md)
- [Public-target preflight: independent needs-input disposition](reports/0019-authorized-target-playback.md)
- [Platform decision: macOS required, Windows P2](reports/0018-platform-priority.md)
- [Approved output-verification architecture and standing delegation](reports/0017-output-plan-review.md)
- [Original output-verification proposal and Mac-only setup](reports/0015-m1-output-plan.md)
- [Current independent separate-track review and remaining M1 gates](reports/0014-separate-track-review.md)
- [Native-rate observations and their limits](reports/0013-m1-rate-measurement.md)
- [Separate-track repair and complete macOS capability results](reports/0012-separate-track-repair.md)
- [Independent playing-close verification and closure](reports/0011-playing-close-review.md)
- [Playing-close repair and author verification](reports/0010-playing-close-repair.md)
- [Playing-close diagnosis and original measurements](reports/0009-playing-close-diagnosis.md)
- [Independent cleanup review and finding dispositions](reports/0008-coreaudio-cleanup-review.md)
- [Historical CoreAudio cleanup repair and capability evidence](reports/0007-coreaudio-cleanup.md)
- [Historical independent replacement surface-gate review](reports/0006-player-replacement-probe.md#independent-implementation-and-gate-one-review)
- [Historical provisional mygo preview and verification](reports/0005-provisional-playback.md)
- [Open mygo playback issues and replacement criteria](docs/known-issues/playback.md)
- [Historical macOS M1 results and limitations](reports/0002-playback-macos.md)
- [Current policy adoption, M0 acceptance, and next milestone](reports/0003-policy-adoption.md)
- [Independent review dispositions](reports/0003-publication.md#independent-review)
- [Stage-zero milestones and next action](docs/stage-zero.md)
- [Application boundaries](docs/architecture.md)
- [Start a feature and assign roles](docs/development.md)
- [Tests, prerequisites, and CI limits](docs/testing.md)
- [Write a PR or handoff](docs/pull-requests.md)
- [Development protocol and evidence ownership](docs/workflow.md)
- [Current readiness contract](specs/0003-agent-readiness/spec.md)

Readiness implementation and independent technical review are complete. PR #1
merged as `daca629`; its PR and merged-master CI passed on all three OSes. The
owner confirmed public visibility and protected, PR-based squash merging.
The owner adopted the five task-scope/check/review policies on 2026-11-10;
see the [policy handoff](reports/0003-policy-adoption.md) for their scope. The
owner separately accepted M0 on the same date. Those checks and decisions
describe their recorded revisions, not later playback changes.

The explicit legacy mygo fixture preview has sliders,
speed presets, copied-input retry and a temporary-player warning. Its explicit
preview suite passes; its strict full suite still fails delayed loading because
the pinned player queries tracks on the UI thread. Required headers and
separate tracks remain unsupported by that adapter. See
[ADR 0004](docs/adr/0004-provisional-mygo-playback.md) and the preview report.

The replacement probe's original surface gate passed independent review.
The subsequent RPL-002 work adds transport/header handling and a broader smoke
stage. Its CoreAudio runtime repair joins the idle timer worker before library
unload. The independent reviewer closed CA-CLEANUP-001 after surviving the old
crash window, then opened CA-CLEANUP-002 when playing Close took 284.933708ms
against 200ms. That failure and the diagnosis remain preserved. The repair
keeps rendering active until native video teardown acknowledges completion.
The original finding author independently approved this exact repair and
technically closed CA-CLEANUP-002: the first six-scenario native suite passes,
with playing Close at 78.182125 / 43.789417 / 59.687792ms and direct adapter
playing cancellation at 75.140416ms. Complete disposal, per-session unload,
zero ownership, no shutdown fallback and eight-second survival pass, as does
the full independent local race check. Both cleanup findings were technically
closed at that stage; the later final assessment and owner default selection
are recorded in the current handoff. The owner accepted M1 on 2026-11-13.
The separate-track diagnosis found that disabling mpv's `autoload-files` also
prevented selection of explicitly supplied audio. The repaired adapter permits
that selection while audio/subtitle/cover-art discovery remain disabled. Its
first complete macOS capability invocation passes all 41 implemented assertions:
combined and separate picture/controls, required headers/isolation, external
loading and playing cancellation, missing-audio failure and graceful cleanup.
No forced cleanup was needed. The earlier failed stage remains preserved.
The subsequent independent review approves the exact selection repair and
recommends continuing evaluation, with no new actionable findings. Its full
local race check passes; its first complete native capability invocation also
passes 41 assertions, with loading cancellation at 103.35175ms and separate
playing cancellation at 45.361125ms. Missing-audio failure and graceful cleanup
pass without forced cleanup. Physical audio, measured A/V sync, Windows and
real-service access were not tested in that invocation. Later audibility/target
evidence and the default selection have their own records; M2/P0/the full shell
have not started.

Supplemental native playhead measurements observe rates of 0.453×, 0.996× and
2.000× in short initial windows for the 0.5×, 1× and 2× presets. A later,
longer half-speed sample measures 0.4997×; the early discrepancy remains
recorded. These are native-clock observations, not physical output or calibrated
A/V synchronization measurements. No maintained code changed for this probe.

The [foundation handoff](reports/0001-foundation.md),
[foundation review](reports/0001-independent-review.md), and local-phase
[readiness handoff](reports/0003-agent-readiness.md) and
[review](reports/0003-independent-review.md) remain historical snapshots.

## Prerequisites

Install Git and Go 1.27.3, the version pinned in `go.mod`, and put both on PATH.
The module name remains local (`my-bilibili`); it is not a published library
import path. Foundation tools use the standard library; the M1 experiment
pins mygo and its module graph in `go.mod`/`go.sum`. Initial module download
needs network access. The current macOS checkout uses the owner's global
Homebrew Go at `/opt/homebrew/bin/go`; its local toolchain is only a backup.
See [testing](docs/testing.md#prerequisites) for Windows and fresh-clone setup.
Explicit native experiment commands are in
[the experiment guide](experiments/playback/cmd/README.md); ordinary Go tests
do not establish native media or platform support.

```sh
export GOTOOLCHAIN=local GOWORK=off
go run ./cmd/dev check
go run ./cmd/dev check --race
```

The second command needs a supported race-detector platform and C toolchain;
this is a testing requirement, not a production cgo dependency.

The initial macOS bootstrap also preserves a checksum-verified local toolchain
under the ignored `.agent/toolchains/go1.27.3/` directory. It is not included in
fresh clones and is not portable to other OSes. If it exists on this checkout,
enable it for the current shell without changing system configuration:

```sh
export PATH="$PWD/.agent/toolchains/go1.27.3/bin:$PATH"
```

Local toolchain provenance is recorded in `.agent/environment.json`.

## Start or resume recorded work

```sh
go run ./cmd/dev run new --spec specs/<selected-spec>
go run ./cmd/dev run list
go run ./cmd/dev run status --id <run-id>
go run ./cmd/dev run note --id <run-id> --phase implementation --message "Describe the checkpoint and next action"
go run ./cmd/dev check --run <run-id>
go run ./cmd/dev run finish --id <run-id> --outcome needs-human --message "Implementation checked; independent review still required"
```

Replace `<selected-spec>` with the assigned spec directory and use the printed
run ID in place of `<run-id>`. An uncommitted development checkout
requires `run new ... --allow-dirty`; its manifest explicitly records the dirty
or unborn source baseline. Such a run is not release evidence.

`run finish --outcome succeeded` records an execution outcome only. It never
approves a specification, closes review findings, authorizes a merge, or marks
stage zero complete.

## Where work is saved

| Location | Purpose | Versioned? |
| --- | --- | --- |
| `docs/`, `specs/` | Decisions, protocol, acceptance criteria, task definitions | Yes |
| `.agent/runs/<run-id>/` | Frozen inputs, immutable event records, local evidence | No |
| `reports/` | Sanitized durable handoffs, conclusions, links to evidence | Yes |
| `.agents/skills/` | Planning, implementation, review, and handoff procedures | Yes |
| GitHub Actions artifacts | Copies of CI run evidence, not app packages; retained for 14 days | Public remote, not Git history |

Chat history is context, not the source of truth. Local runs are persistent
across process restarts but are not a backup. Before deleting a checkout,
preserve important sanitized conclusions in `reports/` and copy required
evidence to access-controlled durable storage. CI artifacts expire.
