# Tests, evidence, and CI

## Prerequisites

Install Git and the exact Go version in `go.mod` (currently 1.27.3), and make
both available on PATH. Installation/global shell changes require the user's
permission. Do not silently download a different toolchain to make a check pass.
The foundation has no third-party Go dependencies.

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

The template's package command must be replaced with a package that exists.
Default tests must use controlled fixtures, not a live Bilibili account,
cookies, authenticated media URLs, or external network access.

| Layer | What it proves | Additional evidence needed |
| --- | --- | --- |
| Unit/package tests | Deterministic behavior and regression cases | Run command, candidate identity and result |
| CLI/integration tests | Real process/filesystem interactions where exercised | Runtime OS, fixtures and observed behavior |
| Cross-compilation | Source compiles for a target | Does not establish native execution |
| Native UI/media smoke | Target OS behavior, input, playback and cleanup | Authorized fixtures and actual OS evidence for the exact candidate; current M1 technical gates support owner acceptance, which remains pending |
| Packaging/release | Installable artifact and promotion behavior | Separate packaging/smoke/signing policy; not implemented yet |

Under [ADR 0005](adr/0005-platform-priority.md), macOS is the required product
runtime for M1, M2 and P0. Windows is optional P2: missing Windows native or
package evidence does not block those milestones and stays `not-tested`, never
PASS or a support claim. Existing Linux/macOS/Windows Go CI checks still apply;
tooling compatibility does not establish native product support.

Map ACs to tests or named manual checks in the spec. Record `pass`, `fail`, or
`not-tested` per required OS/capability. Coverage percentages describe tests;
they do not prove an AC or independent approval. Go checks do not validate
Markdown links, skill behavior, or the completeness of PR descriptions.

For template/skill changes, instantiate templates in an isolated scratch
checkout, validate through the real CLI, and exercise a realistic workflow.
When the skill-creator tooling is available, also run its bundled
`scripts/quick_validate.py <skill-directory>` on each changed skill. Record the
tool path/version and any unavailable validation; this external authoring
check is not a Python runtime requirement for the Go project.

## Default playback verification

The default playback launcher now selects libmpv under
[ADR 0008](adr/0008-default-libmpv.md). Its offline setup tests validate explicit
runtime paths, local-only startup and loopback-origin inputs without loading a
library. Native default-app validation builds `cmd/native` with `-tags e2e` and
uses `cmd/replacement-smoke -stage capabilities` with the verified runtime
manifest. The old `cmd/smoke` full/preview suites apply only to `cmd/mygo-native`.
See [the command guide](../experiments/playback/cmd/README.md#default-libmpv-playback)
and [default-switch evidence](../reports/0024-default-libmpv.md). Default selection
does not make the fixture launcher a target URL player.

## Target host-policy regression

Under [ADR 0007](adr/0007-target-host-policy.md), all valid HTTPS media hosts and
ports are allowed in the opt-in target path. Offline regressions cover new hosts,
localhost/IP literals, different video/audio hosts, port and URL syntax, version-2
descriptor compatibility, private metadata and immutable source input. Default
constructors remain fixture-only. The copied refresh helper uses the same URL
validator and tests its synthetic output against the real target parser.
See [report 0022](../reports/0022-target-host-removal.md) for its source location,
exact evidence and rebuild requirements. These tests make no target requests
and do not establish native Bilibili playback. Prior 41-assertion native results
remain evidence for their recorded candidate; a changed candidate needs its own
relevant native checks before a later target session.

## Target failure evidence

The first version-2 target invocation failed its readiness/picture gate and
required forced cleanup; its original result is preserved. The driver now records
separate primary and cleanup failures with fixed phases and optional complete,
validated state/resource samples from existing RPC trees. Samples retain their
original phase and relative time; unavailable/malformed state is omitted, never
reported as Idle or zero ownership. No extra RPC/action, native output, locator,
media capture or deadline change is introduced.

Offline regressions exercise loading, hidden-ready and picture-pending timeouts,
missing/partial/null/duplicate/hostile data, stale sample provenance and a primary
failure followed by forced reap of an owned synthetic child. These tests make no
service request and cannot close the native target failure. See
[spec 0010](../specs/0010-target-failure-evidence/spec.md) and
[the current handoff](../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13). Further native
validation requires an unlocked desktop, exact rebuilt driver pins and a newly
recorded bounded assignment; the earlier invocation cannot be retried under its
consumed allowance. The separately reviewed 2026-11-13 01:58 UTC attempt passes
eight target capability/cleanup checks, including visible picture/selected tracks,
progression, pause, seek, native volume/rate configuration, 87.7575ms playing
cancellation and graceful exit without force. Target loading cancellation remains
not-tested in that one-open session; current fixture coverage is separate. This
result has independent evidence approval and supports the final scoped adapter
recommendation; it does not imply full-duration viewing, target audibility,
physical calibration, changed-candidate CI or owner M1 acceptance.

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

## CI today

The canonical definition is [check.yml](../.github/workflows/check.yml): PRs,
pushes to `main`/`master`, and manual dispatch run the complete Go checks on
Ubuntu 24.04, macOS 15, and Windows 2025. Linux/macOS jobs use `--race`; Windows
currently uses ordinary checks. Action revisions are pinned, repository token
permissions are read-only, and checkout does not retain Git credentials.

Important limits:

- Every Go package and active spec is checked, but the CI run recorder is
  hardcoded to `specs/0001-foundation`. Feature-specific evidence selection is
  not implemented. Record feature AC evidence separately; do not claim that
  the uploaded foundation record proves the feature's acceptance.
- Match the CI run's actual head SHA to the intended candidate. A green run for
  an earlier revision does not validate later changes. The [current handoff](../reports/0003-publication.md)
  records the completed readiness PR and merged-master results. On a PR,
  distinguish its source head from GitHub's tested merge candidate.
- Artifacts are run logs/records, not app binaries or installers. Retention is
  14 days; retain durable conclusions and required evidence before expiry.
- Remote status, protected branches, and required reviews are not established
  by this YAML. Check remote policy separately; never infer enforced gates.

## CD later

There is no current deployment, package, signing, notarization, updater, or
release workflow. M2 calls for native preview packages and smoke evidence;
public promotion needs a separately approved exact revision and credential
boundary. Do not expose release secrets to PR tests or execute untrusted
artifacts in privileged jobs. See [release policy](workflow.md#ci-storage-and-release).
