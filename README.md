# my-bilibili

A Go desktop Bilibili client with macOS as the required platform and Windows
as P2 / nice-to-have, and an experiment in
spec-driven, independently reviewed agent development.

The macOS developer app restores a saved Bilibili account from Keychain. Search
for an ordinary user-uploaded video or open its BV/av identifier or canonical
link, choose a part and an available quality, and watch with Chinese danmaku.
Local history lets you continue the same part after restarting the app. The
[verification report](reports/0039-p0-real-client.md) distinguishes actual
service playback from fixture tests and lists the remaining limitations.

## Start here

- [STATUS](STATUS.md): current state, next actions, blockers and the owner
  action queue. Read this first.
- [Architecture](docs/architecture.md) and [decisions](docs/adr/)
- [Development protocol](docs/workflow.md), including risk lanes and evidence
- [Start a feature](docs/development.md), [tests and prerequisites](docs/testing.md),
  [PRs and handoffs](docs/pull-requests.md)
- [Milestone exit criteria](docs/stage-zero.md)
- [Native playback commands](experiments/playback/cmd/README.md#default-libmpv-playback)
- `reports/`: dated handoffs and reviews. They are history and describe their
  own candidates.

## Run the macOS developer app

On the development Mac, unzip the verified developer bundle and open
`my-bilibili.app`. The normal app uses the real service. If it already has a
saved session it restores it automatically; otherwise sign in from the account
pane. Use **发现** to search or open a video and **历史** to continue watching.
History lives on this Mac; searches and playback do not post anything to Bilibili.
If you change quality while a saved position is being restored, the new
quality resumes from that same position.

The bundle needs the existing development libmpv installation described in
[the runtime boundary](internal/previewbundle/bundle.go). It is not a portable
installer. To build a fresh bundle on that Mac from the repository root:

```sh
export GOTOOLCHAIN=local GOWORK=off
go run ./cmd/package-preview -output "$PWD/my-bilibili.app"
open ./my-bilibili.app
```

The output path must not already exist. For the offline preview, quit the app
and run `./my-bilibili.app/Contents/MacOS/my-bilibili --fixture`.

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
