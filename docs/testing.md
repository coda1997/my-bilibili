# Tests, evidence, and CI

## Prerequisites

Install Git and the exact Go version in `go.mod` (currently 1.27.3), and make
both available on PATH. Installation/global shell changes require the user's
permission. Do not silently download a different toolchain to make a check pass.

The project pins its dependencies in [go.mod](../go.mod) and [go.sum](../go.sum).
For offline checks, populate the Go module cache with the pinned versions first.
The product foundation has no parser dependency. The development checker uses
Goldmark and YAML v3 only to validate agent Markdown and skill frontmatter;
product packages do not import those parsers.

From the repository root, with Bash/zsh:

```sh
export GOTOOLCHAIN=local GOWORK=off
git --version
go version
git status --short --branch
git rev-parse HEAD
```

For PowerShell on Windows:

```powershell
$env:GOTOOLCHAIN = "local"
$env:GOWORK = "off"
git --version
go version
git status --short --branch
git rev-parse HEAD
```

The original macOS checkout has an optional, ignored, checksum-verified
`.agent/toolchains/go1.27.3/bin/go`. If it exists on that machine, a temporary
`export PATH="$PWD/.agent/toolchains/go1.27.3/bin:$PATH"` enables it. A fresh
clone or worktree does not contain it; do not copy this macOS binary to Windows.
If tooling is absent, report the prerequisite and stop rather than inventing
test results. `gh` is needed only for GitHub operations, not local Go checks.

## Local checks

```sh
go run ./cmd/dev check
go run ./cmd/dev check --race
```

The runner validates every spec, checks formatting without changing files,
runs `go mod tidy -diff`, `go vet ./...`, uncached tests (`-count=1`), and
`go build ./...`. `--race` adds the race detector to the tests. It needs a
supported platform and C toolchain; a missing requirement is not a passing
race result. Do not remove an assigned gate to accommodate your environment.

Use targeted tests while iterating, then run the full assigned check:

```sh
go test -count=1 ./cmd/dev ./internal/spec
go test -count=1 -race -run '^TestRecordedCheckDisplayFailures$' ./cmd/dev
go test -count=1 -race -run '^TestUnicodeWriteScopes$' ./internal/spec
```

Default tests must use controlled fixtures, not a live Bilibili account,
cookies, authenticated media URLs, or external network access.

| Layer | What it proves | Additional evidence needed |
| --- | --- | --- |
| Unit/package tests | Deterministic behavior and regression cases | Run command, candidate identity and result |
| CLI/integration tests | Real process/filesystem interactions where exercised | Runtime OS, fixtures and observed behavior |
| Cross-compilation | Source compiles for a target | Does not establish native execution |
| Native UI/media smoke | Target OS behavior, input, playback and cleanup | Authorized fixtures and actual OS evidence for the exact candidate |
| Developer packaging | Contents of the built bundle and whether it launches on the tested host | Verify the exact artifact and its runtime dependencies; signing and public distribution need separate evidence |

Under [ADR 0005](adr/0005-platform-priority.md), macOS is the required product
runtime for M1, M2 and P0. Windows is optional P2: missing Windows native or
package evidence does not block those milestones and stays `not-tested`, never
PASS or a support claim. Existing Linux/macOS/Windows Go CI checks still apply;
tooling compatibility does not establish native product support.

Map ACs to tests or named manual checks in the spec. Record `pass`, `fail`, or
`not-tested` per required OS/capability. Coverage percentages describe tests;
they do not prove an AC or independent approval. The canonical Go check
validates the checked workflow inventory's skill frontmatter, routing, local
Markdown links and fragments, and repository containment. It does not execute
the semantic behavior of a skill or validate the completeness of a PR
description.

For template/skill changes, instantiate templates in an isolated scratch
checkout, validate through the real CLI, and exercise a realistic workflow.
When the skill-creator tooling is available, also run its bundled
`scripts/quick_validate.py <skill-directory>` on each changed skill. Record the
tool path/version and any unavailable validation; this external authoring
check is not a Python runtime requirement for the Go project.

## Native macOS client checks

Use an unlocked Mac with the development libmpv installed. Build the fixture
bundle and the [preview driver](../cmd/preview-smoke) from the same commit:

```sh
fixture_dir=$(mktemp -d)
go run ./cmd/package-preview -e2e -output "$fixture_dir/my-bilibili-e2e.app"
go build -o "$fixture_dir/preview-smoke" ./cmd/preview-smoke
```

Pass the driver an absolute `-app` path to the bundle’s `Contents/MacOS/my-bilibili`,
the app and driver digests (`-app-sha256`, `-driver-sha256`), a fresh absolute
`-output` directory, and the assigned `-stage` (`full`, `fullscreen`, `source`,
`danmaku`, or `client`).

The fixture bundle uses synthetic data. Real account or playback verification
uses the [normal developer app](../README.md#run-the-macos-developer-app) and
a separately authorized plan. Keep session material and signed media URLs out
of evidence.

## Recorded checks

The controller runs these from its checkout, substituting the selected spec
and returned run ID; workers run ordinary checks and return sanitized logs:

```sh
go run ./cmd/dev run new --spec specs/<id>
go run ./cmd/dev check --run <run-id> --race
go run ./cmd/dev run status --id <run-id> --events
```

For an explicitly allowed dirty development baseline, add `--allow-dirty` to
`run new`. The run freezes spec/task inputs, not the entire source tree. Freeze
a separately identified candidate for review as described in
[development](development.md#execute-and-finish). Do not commit just to bypass
the dirty-tree safeguard when the user has not authorized a commit.

Preserve failing evidence too. A display/terminal failure must not discard a
completed check's durable outcome, but still makes the CLI return an error.
An evidence persistence failure must not be reported as recorded success.
After interruption inspect the run; rerun operations whose completion is not
established. Only the controller attaches logs beneath its own run directory.

## GitHub Actions

The canonical definition is [check.yml](../.github/workflows/check.yml): PRs,
pushes to `main`/`master`, and manual dispatch run the complete Go checks on
Ubuntu 24.04, macOS 15, and Windows 2025. Linux/macOS jobs use `--race`; Windows
currently uses ordinary checks. Action revisions are pinned, repository token
permissions are read-only, and checkout does not retain Git credentials.

- Every Go package and active spec is checked, but the CI run recorder is
  hardcoded to `specs/0001-foundation`. Feature-specific evidence selection is
  not implemented. Record feature AC evidence separately; do not claim that
  the uploaded foundation record proves the feature's acceptance.
- Record the run URL and the revision checked out by each job. For a PR, record
  both its head and the tested merge commit. A push to `master` is a separate
  run at the merged SHA. [STATUS](../STATUS.md) links the current results.
- Artifacts are run logs/records, not app binaries or installers. Retention is
  14 days; retain durable conclusions and required evidence before expiry.
- Remote status, protected branches, and required reviews are not established
  by this YAML. Check remote policy separately; never infer enforced gates.

GitHub Actions does not package, sign or publish the app. A release needs a
separately approved revision and credentials outside the PR workflow; see
[release policy](workflow.md#ci-storage-and-release).
