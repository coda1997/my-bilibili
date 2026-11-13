# M1 continuation: physical output verification proposal

Date: 2026-11-12. Planning run: `20261112T125901Z-88d36e6b471d`.
Source base: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.
Outcome: preparation complete; independent architecture review and execution
inputs pending. M1 remains in progress and unaccepted.

The owner asked to read M1 status and continue, then selected **Mac only** from
the available verification setups. This continuation prepares the next bounded
contract. It does not change the playback implementation or add physical-output
evidence. The [independent selection review](0014-separate-track-review.md)
remains the current implementation disposition: scoped approval, 41 fixture
assertions passing, both cleanup findings technically closed, continue evaluation.

## Delivered

- [Proposed output-verification spec](../specs/0007-output-verification/spec.md):
  paired visible/tone events in isolated generated fixtures, decoded event timing
  validation, bounded observation launcher and physical capture methodology.
- [Task DAG](../specs/0007-output-verification/tasks.json): fixture/analysis work,
  observation preparation, then independent review and honest reporting.
- [Pending architecture-review packet](../specs/0007-output-verification/assignment.md):
  exact candidate locators, scope, missing prerequisites and review questions.

The original static geometry and continuous tone cannot provide paired event
timing. The proposal keeps the original fixtures and smoke oracles unchanged.
It reuses the current app and loopback server with a separate fixture directory.
No new native player, transport relaxation, framework boundary or default change
is proposed. Maintained tools remain Go; explicit media generation uses separately
available FFmpeg/ffprobe. Both executables are present on PATH, but no generation
or version/capability qualification was performed in this planning continuation.

The method separates intended timing, decoded encoded-media timing, human
audibility, raw physical observations, calibrated sync/rate and acceptance.
Unknown capture calibration cannot produce a sync PASS. Proposed fixture timing
validation is not a new player-sync acceptance threshold. The existing contract
has no numerical physical sync/rate tolerance; final criteria require independent
assessment and owner decision. Only a Mac is available, so physical capture
remains unavailable and untested.

## Verification actually performed

The latest run was inspected through `run list` and `run status`; its successful
bounded review outcome and remaining M1 gates agree with the durable handoff.
Before edits, all **125 source files** matched its handoff manifest. All **32
runtime-manifest artifacts** matched their hashes: library/dependencies plus the
two recorded patches. No native runtime was rebuilt or loaded for this audit.

The full command passed all six checks on macOS arm64 / Go 1.27.3:

```sh
GOTOOLCHAIN=local GOWORK=off GOPATH="$PWD/.agent/gopath" \
  GOPROXY=off GOSUMDB=off \
  go run ./cmd/dev check --run 20261112T125901Z-88d36e6b471d --race
```

Spec contracts, formatting, tidy-diff, vet, uncached race tests and build pass.
This validates the new task graph and current code, not the physical method,
unimplemented tooling or independent architecture approval. No native playback,
physical recording, Windows, target-service, CI or packaging test was run.

Checked source: 128-file manifest SHA-256
`1b9ee0b1e71c16d2f33e48e8c95ff606f82d0a56df3da964d6d9eb5a43a210aa`;
archive `b5754354265bf96c4ea75b287fa83317ddb770630bad9e516b3c2d4533284994`.
Proposed spec SHA-256
`95528617ac85e7472924d6e2d517f6dfa2057fae2994bcba56b652f8f74b9eab`;
tasks `19ec832470c9bc784d283f0ee044c20cf9c39ef4c19cd0a706c930fe07c27333`.
The later publication snapshot adds this report and navigation only; the final
verification records that checked code, proposal and historical reports did not
change. No second race run is claimed for navigation-only edits.

Evidence is local under `.agent/runs/20261112T125901Z-88d36e6b471d/evidence/`:
`baseline-verification.json`, `baseline/`, the recorded check log,
`checked-candidate/`, final `proposal-candidate/`, `final-verification.json`
and the final evidence inventory. Candidate identities include manifests,
archives, diffs and explicit hashes; dirty HEAD alone is not the identity.
Ignored evidence is not backed up by Git and must be retained separately before
removing the checkout. The planning run freezes the existing `0006` inputs only;
`0007` remains a proposal and is not frozen for implementation.

## Remaining gates and next action

| Work | Current state | Next input/action |
| --- | --- | --- |
| macOS controlled fixture capability/cleanup | Previously independently approved within its scope | Preserve prior evidence and unchanged limits |
| Output verification preparation | Spec/task DAG validated; no implementation | Separately authorized independent architecture review of the prepared packet |
| Physical audibility and calibrated sync/rate | Not tested; Mac only | Observer/session authorization, suitable capture and independently assessed calibration/uncertainty method |
| Windows actual UI/playback | Not tested | Actual Windows environment and reviewed runtime/loader scope |
| Authorized target-media compatibility | Not tested | Authorized media/access and reviewed extension beyond synthetic input |
| Final adapter decision and M1 acceptance | Pending | Required evidence, independent assessment and owner decision |

No agent was dispatched: `docs/development.md` requires an authorized separate
review context and says, “Without delegation authorization, prepare the packet
and ask.” The concrete next step is authorization to dispatch one native
independent architecture reviewer for `0007`, or review in a separate human
context. After review and implementation authorization, start a new frozen run.
Physical capture can remain blocked while independently approved preparation is
implemented. No commit, push, PR, branch/worktree creation, recording, global
installation or remote action occurred. No M2/full-shell work began.
