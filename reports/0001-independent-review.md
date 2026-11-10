# Foundation independent review record

- Date: 2026-11-10.
- Decision: **request-changes**. M0 is not accepted; stage zero is not complete.
- Review run: `20261110T122552Z-d8b466835b9f`.
- Prior implementation run: `20261110T121240Z-4bca5187b819`, unchanged and closed.
- Controller: `/root`; this document consolidates independent reports, not a
  self-review or an authenticated approval.
- Implementation repairs: none in this review round.

## Review identity and independence

The controller dispatched two separate agents with `fork_turns=none`. Neither
inherited the implementation conversation or authored the reviewed code. Both
read the repository contracts and inspected the candidate directly, with their
own test execution. They shared a filesystem, not isolated security sandboxes.

| Reviewer | Dimensions | Final decision |
| --- | --- | --- |
| `/root/review_correctness` | Go correctness, task validation, persistence, concurrency, CLI recording, portability, test gaps | `request-changes` |
| `/root/review_architecture` | Architecture/contracts, CI, permissions, acceptance evidence, handoff | `request-changes` |

The architecture reviewer subsequently inspected the correctness reviewer's
reproduction logs and test sources, but did not claim to have rerun those
probes or create duplicate findings. Findings retain their original owner.

There is no candidate commit: the repository is unborn and has no remote. The
review therefore identifies an exact 26-file content snapshot, not a release
baseline or fictional revision:

- Candidate ID: `sha256:cb12f7f3eae3b66085ac3c411284e210f1f43480c8151e379c10553a6a61979a`.
- Spec SHA-256: `269e0864be9d22236d99960765cf6f1c97cb62b65175eeadd1938cafa24582d5`.
- Task graph SHA-256: `a7772438e3805920aa0e302c1f819fb423e0f93015a9294708aaf3f2f38913e1`.
- Candidate archive SHA-256: `6505787a278690555c2a0c3e251de195ce7674d451b089920434e8876d5516b3`.

Both reviewers and the controller verified the original candidate files,
manifest, and archive unchanged after review. Subsequent controller changes
only record these conclusions in reports and update documentation status
pointers; those informational edits are not part of the reviewed snapshot.
Code, specs, checks, CI, and gate-policy rules remain unchanged.

## Open implementation findings

### COR-001 - P2: display failure loses the recorded check result

- Candidate location: `cmd/dev/main.go:199`; related writer construction at
  `cmd/dev/main.go:183` and skipped publication at `cmd/dev/main.go:207`.
- Criterion: FND-AC-04.
- Observed: `io.MultiWriter(out, file)` stops on the display writer's error.
  The final display-write error is then treated as a file-persistence error,
  and `recordedCheck` returns before `store.Note`, even when the evidence file
  syncs and closes successfully. The caller receives an error, but a completed
  check has no recorded outcome or evidence reference.
- Reproduction: a display writer accepts the header and runner output, then
  fails on the final result. Both passing and failing runners complete; both
  runs retain only their started event despite a readable evidence file.
- Expected: preserve the durable outcome and log reference when the evidence
  store is usable; separately surface the display error to the caller.
- Required verification: add pass/fail regression cases with display errors,
  assert a valid recorded outcome and referenced log, and retain propagation
  of real persistence errors. Run affected tests and the full race check on a
  newly identified candidate.
- Disposition: **open**; repair owner not yet assigned. Verifying reviewer:
  `/root/review_correctness`. The author cannot close this finding.

### COR-002 - P2: equivalent Unicode paths bypass scope overlap detection

- Candidate location: `internal/spec/spec.go:212`.
- Criterion: FND-AC-02.
- Observed: path validation accepts Unicode spellings, but overlap comparison
  only lowercases strings. Unordered scopes `internal/caf\u00e9/file.go` and
  `internal/cafe\u0301/file.go` pass validation. The native macOS reproduction
  confirms with `os.SameFile` that they identify the same file.
- Expected: reject overlapping scopes on supported platforms, or explicitly
  reject ambiguous spellings that the portable path grammar cannot support.
- Reproduction: create the file with the first spelling, verify both paths
  identify it, and validate an otherwise valid two-task graph without a
  dependency between those tasks. Both paths are accepted and validation
  returns no error. This is a declared-scope bug, not a request for a sandbox.
- Required verification: decide and review a portable spelling rule under
  the Go/standard-library constraint; test exact-file and directory-subtree
  aliases plus ordered overlap. A contract restriction requires separate
  policy review, not a silent weakening or change of the acceptance gate.
- Disposition: **open**; repair owner not yet assigned. Verifying reviewer:
  `/root/review_correctness`. The filesystem reproduction ran on macOS only.

No P0/P1 implementation defect was established. These P2 findings still block
acceptance because they leave explicit acceptance criteria unmet.

## Separate acceptance blocker

### ARC-001 - P1 acceptance blocker: remote CI evidence is missing

- Candidate location: `specs/0001-foundation/spec.md:67`; matrix definition at
  `.github/workflows/check.yml:24`.
- This is **not a code defect**. The baseline has no commit or remote, and no
  actual Linux/macOS/Windows Actions results or uploaded artifacts exist for
  it. Passing local checks and workflow lint do not replace this evidence.
- Required action: obtain authorization for the intended baseline and remote;
  resolve the code findings; run the reviewed revision through all three CI
  jobs and inspect their results and evidence. Do not weaken the gate.
- Disposition: **open**; human/controller owns remote authorization and
  execution. Verifying reviewer: `/root/review_architecture`.

## Independent verification

Host execution was macOS arm64 with Go 1.27.3. Evidence paths in this section
are relative to `.agent/runs/20261110T122552Z-d8b466835b9f/evidence/`.

| Check actually performed | Result | Preserved evidence |
| --- | --- | --- |
| Each reviewer independently ran `go run ./cmd/dev check --race` without `--run` | Pass on the original candidate | `correctness/check-race.log`, `architecture/check-race.log` |
| Correctness reviewer repeated recorder concurrency, stale-lock, pending-file, and resume tests 50 times with race | Pass | `correctness/runlog-stress.log` |
| Correctness reviewer ran race-enabled package tests with coverage | Pass; coverage is descriptive, not a gate | `correctness/coverage.log`, `correctness/coverage.out` |
| Correctness reviewer built and exercised the native CLI in separate processes in a scratch fixture repository | Pass: actual passing/failing checks recorded, resume, finish, and post-close rejection | `correctness/cli-integration.log` |
| Reviewer-only regression probes on a verified archive copy | Fail as expected, reproducing COR-001 and COR-002 | `correctness/repro.log`, `correctness/source/cmd/dev/reviewer_regression_test.go`, `correctness/source/internal/spec/reviewer_regression_test.go` |
| Architecture reviewer ran actionlint v1.7.12 | Pass; ShellCheck and pyflakes explicitly disabled | `architecture/actionlint.log` |
| Both reviewers and controller rechecked candidate identity | Pass: all 26 original files, manifest, and archive unchanged during review | `correctness/manifest-after.log`, `architecture/identity-final.log`, `candidate-after-review.json` |

The new regression probes remain evidence, not changes to the candidate test
suite. Existing tests passing does not negate their failing assertions.
No Windows/Linux executable, remote CI, UI, playback, native package, signing,
or release was tested in this review.

## Evidence and continuation

The controller copied the original reviewer reports and selected logs/tests
into its run, recorded their hashes with `run note`, and did not edit either
reviewer's report. Important evidence under the run's `evidence/` directory:

- `review-candidate.json` and `review-candidate.tar.gz`: exact reviewed input.
- `review-assignment.json`: separate reviewer identities and assigned scopes.
- `correctness/report.md`: original report, SHA-256
  `20a7a0369167144f90fea2457cd89a93bcb35bcb5bcb406ba78dc86033269cf4`.
- `architecture/report.md`: original report, SHA-256
  `3f7c873598ed670ddf3d30f0d3909c294df34ea1fbfb0bd806b5ba6fca842deb`.
- `review-evidence-index.json`: copied artifact paths, sizes, and digests.

The review run closes `needs-human`: review execution completed, but repair
authorization, contract-policy decisions, and remote CI remain unresolved.
Read it with `go run ./cmd/dev run status --id 20261110T122552Z-d8b466835b9f --events`.

Next, authorize and assign scoped repairs, preserve the two regressions,
identify the changed candidate, rerun checks, and return findings to their
independent reviewer for closure. Any scope/contract/check-policy change must
receive the separate review required by `AGENTS.md`. Keep the three-round
repair limit and human escalation from the existing workflow; this record
does not implement or override those gates.

Local run data and reviewer scratch are Git-ignored and not remotely backed
up. This sanitized report is intended for Git but is currently uncommitted.
Preserve required evidence before deleting the checkout. No commit, push,
remote configuration, policy relaxation, or implementation repair occurred.
