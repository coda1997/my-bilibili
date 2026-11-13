# Assignment: default-libmpv architecture and candidate review

2026-11-13. Owner explicitly requested setting libmpv as the default after being
told it was implemented/tested but opt-in. The controller/author is `/root`.
Standing owner authority permits native independent subagent review within this
task. No commit, remote publication, new worktree, runtime installation or formal
M1 acceptance is requested. User request supersedes earlier prohibitions on
changing the default, solely for this scoped launcher change.

Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`; base HEAD
`d846e667c537c06a1bcf100e6aac9486c29aa3d2`. Starting maintained candidate is the
184-file final snapshot in run `20261113T014429Z-67ce12fdd21b`,
`evidence/handoff-final-r1-manifest.json`, SHA256
`da260b9231e1ecd0a5a26a63fc1230e51828e259ea76bff0cbea6512c61117dc`, archive
`evidence/handoff-final-r1.tar`, SHA256
`48c1014e00f93e98d273c50dea8284eef36ac661f97347302f069bdc5d4146f0`.
All 184 file hashes were verified before adding this planning packet. Only this
new spec/task/assignment directory is added for architecture review; reviewer
must calculate its exact hashes. No frozen run exists yet for this task.

Read README/current target handoff, architecture/workflow/testing, the
independent-review skill, spec/tasks here, current `cmd/native`,
`cmd/replacement-native`, `cmd/replacement-smoke`, and the playback UI/contract
as needed. Review the small composition change, runtime prerequisites, test
compatibility, preserved historical evidence and explicit acceptance boundary.
Return proceed or concrete findings before implementation/freeze. Scope and
ACs are exactly DMP-001 in tasks.json. Shared adapter/contracts/runtime, legacy
smoke, fixtures, target path and policy are read-only.

Reviewer writes only under `.agent/reviewer-scratch/default-libmpv/`; candidate
and controller run records are read-only. No native/service execution during
architecture review. Later code review receives exact candidate manifest,
archive, baseline diff, check logs and the newly recorded fixture result.
Independent review may execute offline tests, and must truthfully distinguish
its own checks from the controller's native evidence. Findings use the repository
review template and remain owned by their author. Stop on unclear scope, unsafe
inputs or the three-round repair limit. Preserve all prior user work.
