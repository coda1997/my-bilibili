# Accept and publish M1

Spec ID: `0012-m1-publication`
Status: owner authorized 2026-11-13; scope review before freeze.

## Goal and authority

The owner explicitly requested: "mark m1 milestone is completed, polish the PR,
push and publish it". Record M1 acceptance, preserve the reviewed implementation,
commit the accumulated M1 work, synchronize necessary base documentation, and
publish a polished PR from `codex/m1-macos-playback` into the live app-selected
base `master` in the existing public `coda1997/my-bilibili` repository.
No merge into master, release, account/media session, dependency/runtime change,
new worktree, remote-policy change or M2/P0 implementation is authorized.

## Boundaries

The starting 192-file candidate is the approved final snapshot from run
`20261113T023232Z-ffe109b8bd77`: manifest
`7422e2757bce7fb043fdf0f20aae614e670ffa50e6b53c236fda762658a26c97`, archive
`3e9381fef65a5e4e454863e8def0dc892b2343a26d94562f0d7698708a443d28`.
Every file was reverified unchanged before this planning packet. Initial HEAD is
`d846e667c537c06a1bcf100e6aac9486c29aa3d2`, equal to the remote feature branch.
Fetched master is `c66bdf41e127a4c73a705e2fafd326ab24cb46d6`; its additional
workflow-clarification commit is documentation only. Retain its proposed-policy
status and the owner's standing native delegation authority when reconciling
shared documents. No new trusted automation policy is adopted by publication.

Only descriptive acceptance/navigation/status edits are permitted beyond that
reviewed candidate and the unchanged base additions. Preserve prior reports and
frozen inputs; add report 0025 as current acceptance/publication handoff. The P0
draft may update its prerequisite status, not its feature ACs or task graph.
No shared interface, async behavior, cancellation, timeout or resource contract
changes. Existing native evidence remains applicable only after verifying all
code, fixture and dependency bytes match the approved source. Run the full local
race check on the publication candidate. No new native/service invocation.

The controller may commit the exact reviewed M1 source first, then merge the
current base into this feature branch without force-pushing, preserving both
sides. Resolve only documentation conflicts within the listed scope and review
the final combined candidate before pushing. If runtime/code conflicts appear,
stop that integration for a revised scoped assessment. Use explicit staged
paths; exclude ignored runtime, raw evidence, scratch and any private data.

## Acceptance criteria

### PUB-AC-01

Current README, milestone/architecture/testing/command docs and P0 prerequisite
status record explicit owner M1 acceptance on 2026-11-13, retaining exact evidence
and untested limitations. Historical reports remain historical. The next product
step is separately scoped M2 planning, not automatic implementation.

### PUB-AC-02

A complete, independently reviewed publication candidate preserves the approved
M1 code and both sides of base documentation. Full local checks, source/scope
identity, link checks and publication-content audit pass; no secret/live locator
or ignored runtime/evidence is staged. Independent review does not grant human
policy approval or close another reviewer's findings.

### PUB-AC-03

Push the reviewed commits to the existing feature branch without force. Create
or update its PR against the verified target with an accurate summary, AC/test
matrix, review attribution, evidence limits and next action. Use draft while CI
is pending; inspect the exact head/merge candidate CI and publish ready for review
when applicable checks pass. Preserve failures and repair actual defects within
the existing runtime/contract scope only after review; never weaken CI/checks.
Remote merge, release and new product implementation remain outside this task.

## Verification and evidence

PUB-AC-01: independent documentation consistency/link audit.
PUB-AC-02: exact manifest/archive and final commit tree; baseline/code comparison;
`go run ./cmd/dev check --race`; scoped independent publication review.
PUB-AC-03: read-back of remote branch SHA, PR base/head/body/draft state and CI
run/head/merge SHA, with Windows CI distinguished from Windows native support.
Detailed evidence stays local in a new controller run; sanitized conclusions
and references go in `reports/0025-m1-acceptance.md` and the PR description.
GitHub Actions evidence expires after 14 days. Local evidence is not backed up.
