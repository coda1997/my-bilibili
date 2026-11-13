# Architecture review context / OUT-001 through OUT-003

## Identity and authority

- Planner/controller: `/root`; independent reviewer: `/root/output_plan_review`.
- User requested M1 status and continuation, then selected **Mac only**, whose
  stated next step was preparation of the output-verification contract.
- The owner subsequently requested removal of per-dispatch permission and granted
  standing native-subagent authority on 2026-11-12. Review dispatch uses that
  authority; no further delegation prompt is needed. Implementation, physical
  capture, commit and remote scope are not expanded by this packet.
- Planning run: `20261112T125901Z-88d36e6b471d`. Prior independent selection review:
  `20261112T124520Z-e12a2e27addd`, closed successfully.
- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
- Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, dirty, not candidate identity.
- Original proposal source: planning run `evidence/proposal-candidate/manifest.json`,
  `candidate.tar`, `tracked.diff`, and `identity.json`. The initial independent
  review is retained in run `20261112T130353Z-b0291eddf22b`. Each subsequent review
  assignment supplies new candidate archive/manifest and spec/task hashes in its
  controller run. Use the current README-linked handoff for that exact identity.
- Contract status: proposed; initial review requested OUT-ARCH-001/002 corrections.
  This revised proposal defines acoustic IDs and time-domain correction. Review
  closure remains the original reviewer's decision on the revised candidate.
  Existing `0006` spec/tasks and adopted workflow/check policies are unchanged.
- Task dependencies: OUT-001 -> OUT-002 -> OUT-003; none has started.

## Minimum context

Read `reports/0015-m1-output-plan.md`, `reports/0014-separate-track-review.md`,
`reports/0013-m1-rate-measurement.md`, the proposed spec/tasks here,
`docs/architecture.md`, `docs/workflow.md` and `docs/testing.md`.

Read the current generator/manifest in `experiments/playback/testdata`,
`experiments/playback/cmd/fixturehttp/server.go`, replacement-native composition,
the plain input/controller contract, and RPL-AC-05/07 in spec `0006`.
The old fixture has no paired changing picture/tone timing oracle. Existing
native-rate observations do not observe physical output. Both cleanup findings
and selection repair retain their existing independent dispositions.

## Boundaries

Reviewer candidate write paths: none. Future implementation paths are exactly
those in `tasks.json`; this packet does not grant their use. Original fixtures,
adapter/runtime, UI, server, original smoke stages, policy and CI are read-only.

Once explicitly assigned, a native reviewer may return artifacts in
`.agent/reviewer-scratch/output-plan-r0/`; the controller alone imports evidence
into its run. No worktree creation, recording, installation or remote action is
needed for architecture review. Available environment is macOS arm64 with the
verified Go pin. Physical capture/calibration hardware and Windows are unavailable.

## Work and return contract

Assess source/decoded timing distinction, event matching and unique labels,
offset sign/correction, calibration independence, measurement uncertainty and
which outcomes can actually be asserted with only a Mac. Check the distinct
fixture directory, unchanged legacy oracles, manifest compatibility, bounded
subprocess/server lifecycle and proposed task write scopes.

Verify the exact candidate before reviewing and record checks actually run.
`go run ./cmd/dev check --race` validates repository/schema/code, not the physical
method or future implementation. Use `docs/templates/review.md` to return
structured findings and approve/request-changes/needs-human with exact identity.
No finding needs closure in this new proposal; do not reopen or self-close old
findings without new evidence. Stop for unresolved measurement validity or a
required scope change. Return the smallest actionable revision or next step.
