# Standing delegation authority and M1 output-plan review

Date: 2026-11-12. Final review run: `20261112T131251Z-93a05da06aba`.
Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty.

The owner requested removal of “Dispatch only if delegation is explicitly
authorized” and stated, “It's free for you to call subagent.” The repository now
records standing authority for native sub-agents within the requested task.
No per-dispatch permission is needed. This change is the owner's direct policy
instruction, not authority inferred from an automated check or reviewer approval.

Updated guidance: `AGENTS.md`, the `feature-plan`, `feature-implement` and
`independent-review` skills, `docs/workflow.md`, `docs/development.md` and the
assignment template. Task boundaries, separate reviewer context, controller
evidence ownership and separate commit/remote/managed-state authority remain.
Historical statements in [0015](0015-m1-output-plan.md) and older frozen packets
describe the authority at that time; they are not current permission gates.

Under this standing authority, native reviewer `/root/output_plan_review`
independently reviewed the delegation changes and the
[0007 output-verification proposal](../specs/0007-output-verification/spec.md).
The guidance change received scoped technical approval. The proposal required
two bounded repairs and now has **independent architecture approval** on the
exact r2 candidate. This is preparation architecture approval, not implemented
tooling, physical output evidence or M1 acceptance.

## Findings and closure

| Finding | Original defect | Final independent disposition |
| --- | --- | --- |
| OUT-ARCH-001, P1 | Uniform tones cannot independently identify events in cropped/post-seek captures; the first ID repair also admitted a harmonic false match | Closed on r2: frozen acoustic ID table, lowest-component anchor, retained counterexample, decoded-media evidence and explicit physical identification assumptions/no-result rules |
| OUT-ARCH-002, P2 | Encoded-media offsets were not explicitly mapped into observed time at non-1x rates | Closed on r1, preserved on r2: observed local mapping, signed equations, interval uncertainty and inconclusive outcomes |

Original reviews, counterexamples and failed closure outcomes remain retained:

- Initial review: `20261112T130353Z-b0291eddf22b`, guidance approved, architecture
  request-changes; review SHA-256
  `a7356853d5cc84b0bedc4ea6b3f8c2844bb46923ffdb57af1e914e9cfa34d798`.
- First closure: `20261112T130934Z-347692293960`, OUT-ARCH-002 closed,
  OUT-ARCH-001 open; review SHA-256
  `6e1d0bd7dd8ec7cc749b5a663dcca33d2bbbdf1d92f81c746e5e793005463155`.
- Final closure: `20261112T131251Z-93a05da06aba`, architecture approved,
  both findings technically closed; review SHA-256
  `e5168489cc2ec3c99e7c66cc5897614b812854665de00e001a5203e85cbd383b`.

The original reviewer made each closure decision. The controller imported the
returned artifacts only after verifying every inventoried hash and size.

## Actual validation and exact identity

Independent full `go run ./cmd/dev check --race` passes all six stages on the
initial and r1 candidates, using Go 1.27.3 on macOS arm64 with the existing
offline module cache. The controller additionally exported the r1 candidate
into a temporary test repository, instantiated the assignment template, and
passed the same real CLI check there. That temporary validation repository was
removed; no app-managed workspace/worktree or commit was created.

The r2 delta adds only 21 spec-prose lines. Independent real spec loading,
`git diff --check`, 18 clean acoustic ID/scale cases and five rejection cases
pass. The full race suite was not repeated for r2; its prior r1 result is not
represented as a new invocation. These synthetic cases test selection semantics,
not waveform decoding, encoded fixture quality or physical detectability.

All 129 reviewed source/archive files match before and after the final review.

| Reviewed artifact | SHA-256 |
| --- | --- |
| r2 source manifest | `09b1e479309bd39fb4a1b462a2276e458d1ee8fdfb9262a82ad73c36af5db451` |
| r2 source archive | `17a4cbf1afda5c0a75363fcb992022ca22141653cf314bc219f2d6aaafbfe764` |
| Revised spec | `6d83d9afd0d8cebe49c28c2eae2cdadfc6823f012c11e104bda7843eb101293b` |
| Task graph | `341415c79cac6f6b93f9aaa4795284c743f26b3093a98cc7946f55864e766323` |

The three skills retain unchanged frontmatter and valid local references. The
bundled `skill-creator/scripts/quick_validate.py` could not start because the
available Python lacks PyYAML. Both controller and reviewer recorded that
unavailable check; no dependency was installed. Actual scoped dispatch, read-only
review, returned evidence and the isolated template exercise validate the revised
workflow behavior without pretending to replace the unavailable YAML validator.

Current evidence is under each listed run's `evidence/`. The final run contains
`candidate/`, `independent-review/`, and a later `handoff-candidate/` identifying
this report/navigation update. Final verification confirms the approved spec,
tasks, guidance, code and historical reports remain unchanged. These ignored
artifacts are local-only and must be preserved before deleting the checkout.

## M1 status and next action

The [0014 implementation review](0014-separate-track-review.md) remains the
playback disposition: 41 passing macOS fixture assertions, separate-track repair
approved, and both cleanup findings technically closed. This continuation changes
guidance and planning documents only; playback code/runtime/fixtures are unchanged.

The next development slice is OUT-001: implement the reviewed fixture generator
and analysis helpers in a new implementation run, with its bounded assignment
and applicable owner task authority. Native sub-agents may be used without a
new delegation prompt. OUT-002 prepares the observation launcher afterward.
Actual decoded fixture behavior must be validated before relying on the method;
the proposal explicitly preserves failures and requires review of method changes.

Physical capture/calibration remains unavailable with the owner's Mac-only setup.
Physical audibility/sync/rate, Windows execution, authorized target-media access,
the final independent adapter assessment and owner M1 acceptance remain open.
No native playback, fixture generation, physical recording, CI or packaging run
occurred in this continuation. No commit, push, PR or remote action occurred.
