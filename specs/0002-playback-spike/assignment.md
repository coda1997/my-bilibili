# Assignment: architecture review / macOS-first M1

## Identity and authority

- Controller/planner and future implementation author: `/root`.
- Reviewer: a separate read-only agent assigned with this packet. The owner
  explicitly authorized independent review and delegated responsibility for
  today's macOS basic-playback and controlled-media experiment on 2026-11-10.
- Windows execution: deferred by the owner to 2026-11-11; final dual-platform
  M1 requirements are not waived. No claim about that future run is made.
- Authorized: review this candidate, report findings, and independently run
  safe read-only checks in assigned scratch. Do not edit the candidate.
- Not authorized: commits, push, PR, publication, policy/check/CI changes,
  live Bilibili/account access, additional global installations or full UI.
- Run ID: none before approval/freeze. Local `run list` is empty; durable M0
  evidence remains in the current handoff.
- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
- Base: `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600`, detached; prior untracked
  P0 planning files are preserved, not part of today's implementation scope.
- Exact review candidate: archive and per-file hash/mode manifest under
  `.agent/review/m1-architecture/`, supplied with their SHA-256s at dispatch.
  The manifest identifies source/spec/task inputs separately from HEAD.
- Contract status: proposed, not frozen and not independently approved.
- Task IDs: `PLAY-001` through `PLAY-004`, sequential; implementation waits for
  this review. `PLAY-004` can record an interim review but cannot complete M1.
- Human policy approval: no automation/check/workflow policy change proposed.

## Minimum context

Read `README.md`, `reports/0003-policy-adoption.md`, `docs/stage-zero.md`,
`docs/testing.md`, `docs/architecture.md`, `docs/development.md`,
`docs/workflow.md`, `docs/adr/0002-go-ui-and-playback-boundary.md`, the revised
`spec.md` and `tasks.json` beside this packet, and the exact candidate manifest.
Use the `independent-review` skill and `docs/templates/review.md`.

Source research in `.agent/research/mygo-module-retry.json` records the exact
mygo module/commit and checksums; `.agent/research/mygo-source/` is a read-only
upstream source snapshot for verifying APIs/license. It is not vendored source
or part of the future build. No experiment code or native evidence exists yet.

## Boundaries

- Candidate write paths for the reviewer: none. Future implementation scopes
  are exactly those in `tasks.json`; do not broaden them through this packet.
- Reviewer scratch: `.agent/reviewer-scratch/m1-architecture/`, outside the
  controller's run directory. Return sanitized findings/evidence paths; the
  controller attaches them after contract approval and run creation.
- Available: Homebrew Go 1.27.3, Xcode/clang, existing FFmpeg/ffprobe, macOS
  arm64. Do not assume graphical automation or audio audibility has passed.
- Unavailable: Windows runtime, live authorized Bilibili fixtures and final
  platform/service evidence. These remain not-tested, not inferred from builds.
- Preserve unrelated P0 planning, historical reports, tool/run records and
  workflow/check/CI policy. No author self-approval or hidden SDK/proxy/decoder.

## Work and return contract

Review whether the refined contract/task DAG is executable and bounded:
macOS staging versus final ACs; exact dependency/input provenance; plain ports;
main-thread/polled lifetime semantics; cancellation, limits and failure cleanup;
loopback server safety; test-only native automation; and honest negative
capability evidence versus actual target playback. Check ordered overlapping
scopes and the distinction between synthetic tests, native runs and IME/audio.

Verify spec/task hashes against the manifest and run appropriate safe checks.
Return candidate/spec identities, checks actually run, prioritized findings
with location/reproduction/required fix, and approve/request-changes/needs-human.
No findings or passing checks alone establish native compatibility. Stop for
unclear authority, changed scope, unsafe inputs or unexpected candidate changes.
