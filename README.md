# my-bilibili

A Go desktop Bilibili client for macOS and Windows, and an experiment in
spec-driven, independently reviewed agent development.

The repository is in stage zero. The current deliverable is the development
foundation, not a working Bilibili client or an autonomous agent orchestrator.

## Start here

- [Stage-zero milestones and next action](docs/stage-zero.md)
- [Application boundaries](docs/architecture.md)
- [Development protocol and evidence ownership](docs/workflow.md)
- [Foundation specification](specs/0001-foundation/spec.md)
- [Latest foundation handoff](reports/0001-foundation.md)

## Prerequisites

Install Git and Go 1.27.3, the version pinned in `go.mod`, and put both on PATH.
The module name is intentionally local until a remote repository is selected.
No third-party Go dependencies are needed for the foundation.

```sh
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
go run ./cmd/dev run new --spec specs/0001-foundation
go run ./cmd/dev run list
go run ./cmd/dev run status --id <run-id>
go run ./cmd/dev run note --id <run-id> --phase implementation --message "Describe the checkpoint and next action"
go run ./cmd/dev check --run <run-id>
go run ./cmd/dev run finish --id <run-id> --outcome needs-human --message "Implementation checked; independent review still required"
```

Use the printed run ID in place of `<run-id>`. An uncommitted bootstrap checkout
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
| GitHub Actions artifacts | Copies of CI run evidence, retained for 14 days | Remote, after a remote is configured |

Chat history is context, not the source of truth. Local runs are persistent
across process restarts but are not a backup. Before deleting a checkout,
preserve important sanitized conclusions in `reports/` and copy required
evidence to access-controlled durable storage. CI artifacts expire.
