# Assignment: controller and architecture review / M2 policy continuation

## Identity and authority

- Controller/author: `/root`; native independent reviewer identity is recorded
  in the exact dispatch packet. No product implementation is assigned here.
- Owner authorization: 2026-11-16 acceptance of applying new budgets and using
  a labeled handoff drill when no real blocker exists. See `continuation.md`.
- Planning write paths: `specs/0016-m2-policy-continuation/` and the controller's
  ignored evidence/new run only. Existing spec0015 and all published evidence
  stay unchanged. `tasks.json` preserves later product implementation paths.
- Baseline: `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` plus preserved uncommitted
  M2-001 source and spec0015. The submitted source manifest/archive and hashes
  are in the dispatch packet; HEAD alone is not the candidate.
- Prior run: `20261115T133635Z-984c0955c8cb`. Review inputs are prepared before
  the new freeze; exact new run ID is recorded after independent approval.
- Policy: R1–R9 at baseline above. `continuation.md` maps historical task
  counters. This is architecture formal round 3, with one earlier contract fix.
  No new allowance, counter reset, commit, push, remote issue, merge, worktree
  creation, app orchestration or product-dispatch authority is granted.
- Native delegation uses the owner's standing authority. Reviewer authorship
  must be independent. No previously open BLOCKING finding requires succession;
  all prior architecture findings were closed by their original reviewer.

## Minimum context and locked scope

Read AGENTS.md, README/current handoff, docs/stage-zero.md, architecture.md,
development.md, workflow.md, agent-convergence.md, testing.md, the planning/review
skills, original spec0015 and this complete continuation. Use historical reviews
and exact hashes linked in `continuation.md`, not baseline CI or chat as proof.

Review only fidelity to the two owner decisions and their affected budget,
handoff and acceptance semantics. Verify unchanged product interfaces/UI/native
checks, task dependencies and write paths. This is not a fresh product-code or
whole-repository sweep. The smaller UI proposal is not authorized.

Done criteria: correct explicit migration without reset; actual repair remains
required; drill is executed/labeled and limited to handoff evidence; real blockers
still stop affected work; no synthetic approval or hidden fourth review; old
records preserved; exact spec/task validation and required check result available.

## Verification and return

Controller runs `GOTOOLCHAIN=local GOWORK=off go run ./cmd/dev check --race` on
Go1.27.3 darwin/arm64 and compares frozen/product bytes with their prior snapshot.
No native playback, packaging, service request or new code behavior is claimed.

Reviewer uses separate scratch only; no candidate edits, published evidence
changes or controller-run writes. Return exact identity, observed versus
inferred, relied-on contract, structured BLOCKING[class]/ADVISORY findings,
any R6 rulings, recommendation and the final round's disposition. Use
docs/templates/review.md. Clarifications do not create an extra formal round.

All review dimensions share this contract's third round. Any unresolved BLOCKING
finding stops automatic work for explicit human direction and additional
allowance. Answered advisories go to the controller's single local follow-up
list; no extra approval round. The inherited issue budget is0/5 used, with no
remote authority; no dedupe/publication is implied. Worktree/commit permissions
and proposed UI reduction are found-not-done planning items in continuation.md.
