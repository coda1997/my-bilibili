# my-bilibili

A Go desktop Bilibili client for macOS and Windows, and an experiment in
spec-driven, independently reviewed agent development.

The repository is in stage zero. M0 is accepted; macOS M1 playback evaluation
is in progress. The provisional mygo preview remains the default. An opt-in
libmpv replacement probe now has passing CoreAudio cleanup regressions and
local/HTTP playback evidence, but its full capability stage fails at
separate-track readiness. Independent repair review and M1 acceptance remain
pending. This is not yet a working Bilibili client.

## Start here

- [Current CoreAudio cleanup repair and remaining replacement gates](reports/0007-coreaudio-cleanup.md)
- [Historical independent replacement surface-gate review](reports/0006-player-replacement-probe.md#independent-implementation-and-gate-one-review)
- [Default provisional playback preview and verification](reports/0005-provisional-playback.md)
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

The independently reviewed provisional mygo fixture preview has sliders,
speed presets, copied-input retry and a temporary-player warning. Its explicit
preview suite passes; its strict full suite still fails delayed loading because
the pinned player queries tracks on the UI thread. Required headers and
separate tracks remain unsupported by that adapter. See
[ADR 0004](docs/adr/0004-provisional-mygo-playback.md) and the preview report.

The replacement probe's original surface gate passed independent review.
The subsequent RPL-002 work adds transport/header handling and a broader smoke
stage. Its CoreAudio runtime repair joins the idle timer worker before library
unload; all five isolated cleanup scenarios pass on macOS arm64. The full
stage passes combined local/HTTP controls, protected-header playback and
header isolation, then fails separate-track readiness. The current report
preserves that failure, forced cleanup, exact source/runtime identities and
the open CA-CLEANUP-001 review disposition. Physical audio, measured A/V sync,
Windows and real-service access remain not-tested. No production-player
selection, M1 acceptance or authorization for M2/P0/the full shell is implied.

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
