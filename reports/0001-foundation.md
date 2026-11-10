# Foundation handoff

- Date: 2026-11-10
- Status: independent review requests changes; M0 acceptance remains blocked.
- Implementation run ID: `20261110T121240Z-4bca5187b819`
- Latest review run ID: `20261110T122552Z-d8b466835b9f`
- Handoff outcome: `needs-human`, not a failed build or a release approval.
- Source baseline: unborn repository; no commits or remote configured.
- Independent review: performed by two fresh-context agents; both return
  `request-changes`. Findings and evidence: `reports/0001-independent-review.md`.

The independent review found two P2 implementation defects (COR-001 and
COR-002) and tracked missing remote CI separately as acceptance blocker
ARC-001. No implementation repair or finding closure has occurred. The prior
closed implementation run remains unchanged.

## Delivered

- Architecture and evidence decisions, stage-zero milestones, and agent rules.
- Foundation and playback-spike specs with validated task graphs. The spike
  stays serial until UI/media integration is understood; M2 tests parallelism.
- A standard-library-only Go CLI to start, inspect, checkpoint, and finish runs.
- Frozen spec inputs, hashed evidence, append-only event publication, writer
  exclusion, and corruption/path checks. This is not a security sandbox.
- Read-only local checks and a pinned-action, read-only three-OS CI workflow.
- A checksum-verified local Go toolchain; no global installation or shell
  configuration changes. There are no product features or release workflows.

## Original implementation verification

Host execution: macOS, arm64, Go 1.27.3. All evidence paths below are relative
to `.agent/runs/20261110T121240Z-4bca5187b819/`.

| Check actually performed | Result | Evidence |
| --- | --- | --- |
| `go run ./cmd/dev check --run 20261110T121240Z-4bca5187b819 --race` | Pass: task contracts, formatting, tidy diff, vet, tests with race, build | `evidence/check-4171857470.log` |
| `go test -race -count=50 -run TestConcurrentWriters ./internal/runlog` | Pass, 50 repetitions | `evidence/final-build-and-stress.log` |
| actionlint v1.7.12 on the CI workflow | Pass; ShellCheck and pyflakes explicitly disabled | `evidence/actionlint.log` |
| `CGO_ENABLED=0 go build -trimpath` of the dev CLI for macOS arm64/amd64, Windows amd64/arm64, Linux amd64 | Pass, compilation only | `evidence/final-build-and-stress.log` |
| Execute the built macOS arm64 CLI and inspect the run | Pass | `evidence/native-cli-status.json` |
| Inspect local Go download provenance | Official archive checksum verified before extraction | `evidence/environment.json` |

Tests cover invalid and cyclic task graphs, unordered overlapping write paths,
acceptance coverage, dirty-baseline opt-in, failed-check logging, source changes,
run continuation, closed-run writes, evidence/input mutation, malformed events,
stale locks, concurrent writers, symlinks, and portable path rejection.

The initial test and build evidence is retained too. The final validation
references above include the subsequent portability and provenance fixes.

No Windows or Linux binaries were executed. No remote Actions job was run.
No mygo UI, video playback, installer, signing, or update behavior was tested.

## Independent review

Both `/root/review_correctness` and `/root/review_architecture` independently
passed the existing race-enabled check suite on the exact frozen candidate.
The correctness reviewer also exercised the native CLI lifecycle in separate
processes and reproduced two missing regression cases in a scratch copy:

- COR-001: display-output failure can prevent publication of a completed
  check's outcome and evidence reference (FND-AC-04).
- COR-002: Unicode-normalization-equivalent macOS paths can pass as disjoint,
  unordered task write scopes (FND-AC-02).

Both findings remain open under the correctness reviewer's ownership. The
architecture reviewer independently passed workflow lint, inspected those
reproductions without claiming to rerun them, and keeps missing actual CI
evidence open as ARC-001. See `reports/0001-independent-review.md` for exact
candidate identity, locations, required verification, and preserved evidence.

## Not completed

- M0 acceptance still requires repaired findings, independent re-review, and
  actual remote CI results. Review completion is not acceptance.
- M1 native UI/playback feasibility has not started.
- M2 multi-agent implementation/review/repair and native packages do not exist.
- M3 issue-to-PR execution is not connected.
- Gate-policy isolation, enforceable worker scopes, authenticated approvals,
  automatic dispatch, signing, notarization, and release are not implemented.
- These files are uncommitted. Neither Git history nor a remote backs up the
  local run. Dirty/unborn provenance is explicitly not release evidence.

## Continue on this machine

The bootstrap toolchain is ignored local state, not part of a clone. Its
provenance is in `.agent/environment.json`. From the repository root:

```sh
export PATH="$PWD/.agent/toolchains/go1.27.3/bin:$PATH"
go run ./cmd/dev run list
go run ./cmd/dev run status --id 20261110T121240Z-4bca5187b819 --events
go run ./cmd/dev run status --id 20261110T122552Z-d8b466835b9f --events
go run ./cmd/dev check
```

The closed implementation and review runs remain immutable. Start a
continuation run with the selected spec and reference the review run ID in its
first note. A fresh clone needs
Go installed separately and will not contain ignored runs or the toolchain.

## Next action

Authorize and assign scoped repairs for COR-001/COR-002, identify the changed
candidate, rerun checks, and return findings to the independent reviewer for
closure. Any path-contract or check-policy change needs the repository's
separate review; no repair is authorized merely by this handoff. When
authorized, create a reviewed baseline commit, configure the intended remote,
push, and inspect all three CI jobs to address ARC-001. Do not equate this
local report with those remote results or trusted-policy acceptance.

Next, review and select `specs/0002-playback-spike/spec.md`, arrange authorized
fixtures and Windows runtime access, and execute M1. Preserve important evidence
before removing this checkout; configured CI artifacts have a 14-day retention.
