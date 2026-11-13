# Assignment: architecture review / HOST-001

- Controller/planner/implementer: `/root`; independent reviewer is a separate
  native agent, assigned in the review packet. No candidate author may approve it.
- Authority: on 2026-11-13 the owner said: "origin policy is no need, we allow all
  of the hosts, remove it, and update across all of the docs". Standing native
  delegation applies. No commit, remote, live-request or native-playback action.
- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
- Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, dirty. The base is not
  the candidate. A per-dispatch manifest/archive binds the actual candidate.
- Before snapshot: `.agent/controller-scratch/target-host-removal/before/`;
  manifest SHA256 `dfe87100592185c3215da200f8cb25293a0f8b447343588b247db9f6df34e61c`;
  archive SHA256 `03707bc61b392c15df1a793ba191c15f6ce085c75514f1acfd4581cdc72ee2ea`.
- Prior run: `20261112T144351Z-ac60cbfe2590`; review its latest checkpoint and
  report 0021. Historical origin mismatch and diagnostic remain unchanged.
- Contract: this spec/tasks plus the documented source/descriptor changes.
  No new run until independent architecture approval; controller then freezes
  the contract and attaches the exact assignment before implementation.

Read README, reports/0021 and 0020, docs/stage-zero, architecture, development,
testing, workflow, and the independent-review skill. Inspect
`experiments/playback/targetprobe/policy.go`, its tests, both target commands,
`specs/0008-authorized-target-playback`, and the reviewed helper sources in
`.agent/worker-scratch/target-refresh/`. Verify Go prerequisites before checks.

Candidate writes are exactly HOST-001's paths. Controller additionally owns the
new spec/assignment and run. Helper implementation uses fresh ignored scratch
`.agent/worker-scratch/target-refresh-unrestricted/`; copy Go sources and change
only host validation/schema/output and affected tests. Do not edit old scratch
or frozen/published evidence. No runtime, dependencies, fixtures, CI or workflow
policy changes. The implementation is serial; no shared-file worker split.

Reviewer candidate writes: none. Reviewer scratch:
`.agent/reviewer-scratch/target-host-removal/`. Return report and hashes to the
controller; do not write controller runs. Review host/port interpretation,
shared validator and descriptor compatibility, private metadata, unchanged
source/grant/TLS/lifecycle boundaries, docs and verification coverage.
Use `docs/templates/review.md`; distinguish owner policy authority from technical
approval. Record actual checks and unavailable/native evidence without claiming
M1 acceptance. After implementation, reuse this independent context for the
exact candidate review, supplied with new hashes/diff/check evidence.
