# M2 fixture preview: macOS CI test deadline repair

## macOS CI test deadline repair — 2026-11-16

M2 is published as Draft [PR #9](https://github.com/coda1997/my-bilibili/pull/9),
initial head `6b35cdf91d85b10938dad115a65c9597a2932df2`. The first required CI run
`43892470266` tested merge `3611dcdbe3554cbaecaebc26bdf47295b650e66e`, whose tree
matches that head: Linux race and Windows non-race PASS, macOS race FAIL. The
controller downloaded all three run artifacts and verified their source/log
identities. This is the new required-verification finding **M2-CI-001**, separate
from the closed original native smoke REV-002. Earlier native passes and owner
functional confirmation remain valid within their recorded scope.

The owner explicitly authorized one additional test-timing diagnosis/repair,
original-reviewer response and push/recheck CI. Run:
`20261116T151908Z-a7869129e308`. Retain five previous reviews/eight previous fixes;
this is repair iteration **9** and original-reviewer response **6**, with no
further automatic iteration. Runtime code, native deadlines, CI policy and
frozen contracts are unchanged. Only `cmd/preview-smoke/fullscreen_test.go` and
the three handoff documents change.

### Observed failure and local reproduction

The failing tests use fake RPC fixtures, not a native app. Successful three-cycle
tests imposed a 4s parent context around three 500ms entry waits plus three large
PNG decode/pixel-hash operations. The missing-callback test imposed 1.2s around
entry, capture and the later exit-callback wait. CI exhausted these parent
contexts at `entry-post-capture`: successful cases had five actions/three captures;
the missing-callback case had only one action and never reached its exit phase.

The unchanged tests pass naturally on the local Mac. Independent timing samples
of the 1710x1073 decode/hash operation are 304.643/304.858/304.703ms; successful
cases take about2.9–3.2s. A scratch-only 500ms delay in picture processing reproduces
all CI failure locations/action counts locally. A separate 700ms fake capture-RPC
delay also reproduces parent-budget exhaustion, at the earlier capture phase.
These are controlled local reproductions, not spontaneous local failures or
measurements of CI host scheduling. They establish the test's sensitivity to
valid capture cost; the actual CI machine's cost breakdown was not measured.

### Test-only repair and validation

Success scenarios now use the test context with the driver's existing 5s
per-transition bounds. The missing-callback scenario reaches the actual exit
phase and must expire under that phase's unchanged 5s bound, with two actions,
one capture, no premature transition and zero PASS records. A 900ms fake capture
case deliberately exceeds the old whole-test budget while each transition stays
within its real contract. Existing geometry, session, ordering, picture and
shared-entry-deadline assertions remain. No production timeout or predicate is
relaxed; no runtime app change or native relaunch is needed for this test repair.

| Verification | Actual local result |
| --- | --- |
| Original race tests, normal timing | PASS; no spontaneous local reproduction claimed |
| Original tests with 500ms extra picture work | Expected FAIL at the same CI phases/counters |
| Repaired targeted race tests, including slow captures and shared5s deadline | PASS |
| Repaired tests with the same500ms picture work | PASS |
| Restore only old parent contexts | Slow-capture and missing-callback regressions FAIL as expected |
| Remove only production callback-completion guard in scratch overlay | All delayed-callback/slow-capture cases detect premature actions; missing-callback case rejects wrong PASS |
| `go run ./cmd/dev check --run 20261116T151908Z-a7869129e308 --race` | PASS, all six steps |

The first parent-context negative-control selector selected only the missing
callback test; a corrected selector actually ran and failed both intended
regressions. Both command records are retained. The repair candidate has274files,
manifest `0203517c81d062b6634b33cb2700ca878d1f9f917f3e9bfe939304c0d41b5359`,
archive `a3b9a57ba781b3177319043e5620375eee55dfe4de51b5366e8df4c2e84c042e`;
only the test file differs from the published base before this prose update.

Original reviewer `/root/m2_feature_review` evaluates this exact repair and
subsequent CI within one authorized round6. The current run retains its final
review and actual publication/CI identities when available. This section records
the local prepublication evidence; the PR carries the latest remote checks and
independent disposition. Local passing tests alone do not close M2-CI-001 or
substitute for the new macOS CI result. No second repair/review is implicit.

M2 final milestone acceptance and merge remain separate owner decisions. The
single follow-up inventory continues M2-CI-001 and those remaining gates; no
unrelated defect or systemic follow-up is established, and issues remain0/5.
GitHub CI artifacts expire after14days; downloaded logs, profiles, snapshots,
normal/e2e packages and reviewer evidence under `.agent/` remain local-only.
The following sections preserve their historical state, including prepublication
status that is superseded by PR9 and this continuation.

## Publication preparation — 2026-11-16

The owner requested continued M2 completion after confirming all functions
normal. The controller verified the previous 274-file handoff without drift,
and validated the recorded completion, independent-review and human-feedback
runs. Read-only remote checks show public `coda1997/my-bilibili`, default branch
`master`, no open PR, and remote `master` still at the local base
`da8804b58f8c3e987c3ac2a8e350371d57f8bfe2`. No merge or rebase is needed against
that observed base.

Run `20261116T150055Z-18e08483856a` prepares `evidence/pr-draft.md`,
`evidence/publication-plan.json`, a complete changed-file inventory and final
source snapshot. Proposed branch: `codex/m2-fixture-preview`; target: `master`.
Proposed title: `[M2] Add native two-part fixture preview and developer packaging`.
The prepared publication includes only the approved M2 source/tests/assets,
original/continuation specs and three handoff documents. Ignored runtimes,
app bundles, raw logs and run archives stay local. The draft carries the
existing native/local/independent evidence and owner's aggregate functional
attestation, while CI for the future commit remains pending.

No new implementation, review round or repair was performed; five/eight
review/fix counts and REV-001/002 closures remain unchanged. Documentation-only
scope, file links and whitespace are checked. Candidate checks are inherited
only for byte-identical implementation; remote CI will be bound to the actual
published commit/tested revision after publication. No CI success is inferred.

The next required action is owner authorization for the concrete commit, push
and draft-PR publication, as required by repository instructions and
[PR authority rules](../docs/pull-requests.md#authority-and-source-identity).
No commit, push, PR creation or merge has occurred in this preparation. Once
authorized, publish the prepared candidate, run/inspect its CI and record exact
revision evidence; keep the PR draft while required gates are pending. Final
merge and M2 milestone acceptance remain separate owner decisions. There is
no new defect or automatic repair/review loop, and no new issue (0/5). This
section continues the existing remaining-work inventory; all local-only
evidence must be preserved with this checkout.

## Owner functional confirmation — 2026-11-16

Following the supplied normal-package launch command and manual picture/audio/
interaction checklist, the owner replied **“功能全部正常”**. This is recorded as
**owner-reported aggregate functional acceptance** of the prompted ordinary
package, including P1/P2 playback/audio and ordinary controls. No new defect is
reported. The reply is an overall statement; no individual observations,
measured values or independently witnessed input sequence are invented.

Run: `20261116T145910Z-31c75327708a`, evidence
`owner-functional-attestation.md`. The previous run's checklist and review are
frozen and unchanged; this new attestation supersedes the earlier lack of owner
functional feedback without rewriting that history. The supplied package is
`.agent/m2-repair-final/MyBilibili.app`; its executable SHA256 remains
`f1a63e6db297e40110a9ef44bede07906f2af8ddda5385793ba77d7ea60113b1`.
The controller verified all 274 source files against the preceding handoff
manifest `e871b44d449c08ef5b5c79f6f965d25c125b78a23d8ebd13e11da3a96ac655a1`
before this documentation-only update. Artifact attribution follows the launch
and checklist context; the controller did not launch or operate the app again.

| Area | Latest disposition |
| --- | --- |
| Ordinary package picture/audio/controls and prompted keyboard/focus/layout | Owner reports all functions normal; aggregate human evidence, without separate per-item observation records |
| Independent causal repair and automated native suite | Scoped APPROVE; REV-001/002 CLOSED; four full suites each30PASS, unchanged |
| System-level accessibility, calibrated physical sync/rate, Windows | Not established by this reply; existing scope and platform limits unchanged |
| Exact integrated commit and changed-candidate CI | Pending; no commit/push authorization or remote action |
| Final M2/M2-005 milestone acceptance | Pending remaining integration gates and final owner decision |

Cumulative history remains five formal reviews/eight actual fixes. No new
review or repair loop is started. The next development step is authorized
commit/publication and exact-candidate CI; functional feedback alone does not
authorize those Git operations. No additional functional confirmation is
requested here. Only README, stage-zero and this report change; local file-link,
whitespace and exact scope checks are recorded, with no repeat Go/native test
for these prose changes. Local ignored evidence remains local-only. There are
no new findings or remote issues (0/5); the remaining gates above continue the
single follow-up inventory. The following sections preserve their prior state.

## Independent acceptance continuation — 2026-11-16

The owner requested a status update and continuation. The original independent
reviewer `/root/m2_feature_review` has now issued **scoped APPROVE**, technically
closing **M2-004-REV-002** on the exact candidate below. REV-001 remains CLOSED;
no new scoped finding or code repair is identified. This is the fifth formal
M2-004 review, with eight actual fixes retained. No further automatic review or
repair loop is active. M2 as a milestone remains incomplete and unaccepted.

Controller run: `20261116T143015Z-3af9394caace`. The reviewed 274-file candidate is
the preceding completion run's `evidence/final-handoff-candidate/`:

- Manifest SHA256: `b7113e6485a1e9ec5f3e2108c426e8f3323c567152669e0962d12d3dff3fa688`.
- Archive SHA256: `8ac33e043761a53a6b0c99d1435d3988b9a4b7067660d8ec5113f9e868b61be1`.
- Frozen review: current run `evidence/m2-product-review-r5/review.md`, SHA256
  `2a84c22fde26e8a2ec0e91987a3bf82dedbee13c0d9142ad067714d924dabc11`.
- Reviewer evidence index: same directory `evidence-sha256.json`, SHA256
  `ecfcf2502d78dd9f2d673a0ecb2210d1a80975984cb76aaabb9e07416bf6833b`.

The controller verified and archived all 13 frozen reviewer deliverables. The
reviewer independently checked the candidate before/after, ran targeted and
e2e-tagged race tests (PASS), and removed only the completion guard in a scratch
overlay (expected regression FAIL). No reviewer native launch occurred. The
reviewer audited the four existing full native suites, each with 30 PASS, and
51 complete fixed fullscreen cycles across all successful fixed invocations.
The fresh normal/e2e package identities and final full local race gate are
recorded in the preceding completion section below. The automated failure and
previously unexecuted smoke scenarios in original REV-002 are resolved; ordinary
human inputs and physical audio remain separate acceptance gaps.

| Acceptance area | Current evidence and remaining work |
| --- | --- |
| AC01 catalog/resources, AC02 ownership | Existing deterministic approvals plus complete native part/replay evidence; unchanged contracts retain their original scope |
| AC03 player UI | Automated controls/layout/fullscreen pass; ordinary pointer/keyboard, focus and accessibility still not-tested |
| AC04 package/lifecycle | Fresh developer bundles, full native cancellation/disposal/close pass; ordinary-package human verification and physical audibility still not-tested |
| AC05 integration | Exact snapshot, integration and local checks retained; exact integrated commit and changed-candidate CI still absent |
| AC06 development loop | Actual failures, bounded stops/resumptions and independent closure retained; owner acceptance and completed M2-005 milestone handoff pending |

The current native computer-control provider failed with
`Sky Computer Use native pipe startup failed`; browser access also reported its
host unavailable. No ordinary-app launch or input occurred in this continuation.
The controller verified 15 bundle/driver artifact files and 30 runtime files,
and prepared `evidence/normal-app-manual-checklist.md` in the current run. All
checklist items remain **未执行 / not-tested**. To perform the next human check:

```sh
open /Users/dadachen/workspace/my-bilibili/.agent/m2-repair-final/MyBilibili.app
```

Normal executable SHA256:
`f1a63e6db297e40110a9ef44bede07906f2af8ddda5385793ba77d7ea60113b1`.
At 1×, explicitly play P1 and verify advancing picture and an audible continuous
synthetic tone; select P2 and verify its distinct picture and higher tone. Then
record actual results for click/double-click, shortcuts, Escape menu priority,
slider release outside the control, overlay visibility, visible Tab focus and
responsive layout using the checklist. Untested items remain untested; selected
audio tracks and rendered captures do not establish physical audibility or
system-level accessibility. Windows remains optional P2/not-tested; calibrated
physical sync/rate measurement remains outside the accepted M1 scope.

Next action belongs to the owner: supply those ordinary-package observations.
Any newly observed defect needs a scoped decision preserving the five/eight
ledger; this handoff requests no speculative repair. Exact-commit/CI and owner
milestone acceptance follow separately; no commit, push or remote action is
authorized or performed here. Logical issues remain 0/5; no remote filing or
dedupe is claimed. No unrelated discovery or systemic follow-up is established.
Only README, stage-zero and this report change after the reviewed candidate;
the current run records their final snapshot and scope/Markdown checks. No Go
or native tests are repeated for this documentation-only update. All ignored
run evidence and developer packages are local-only and require preserving this
checkout. The sections below retain their historical dispositions and budgets;
this section supersedes their OPEN/pending-review status for the current candidate.

## Additional repair completion — 2026-11-16

The owner authorized one additional repair round after the cause was identified.
Source verification showed the demonstrated callback guard was already in the
maintained app/driver source; all 274 files still matched the preceding final
handoff manifest `71d83ca50c2dd466bfd8d2abfb2e55a769df7c4f79f964e163df0c661c6d5c71`.
No outstanding reproduced code defect justified a further patch. This round
therefore completes fresh developer packaging and verifies the existing repair.
No new production/test code, formal review, commit or remote action is included.

Run: `20261116T142531Z-bc46c4a6fdac`, continuing the completed investigation below.
Both bundles were built through `cmd/package-preview`; the driver was built with
`go build -trimpath`. Local development artifacts:

- Normal package: `.agent/m2-repair-final/MyBilibili.app`.
- E2E package: `.agent/m2-repair-final/MyBilibiliE2E.app`.
- Driver: `.agent/m2-repair-final/preview-smoke`.

| Artifact/check | Exact identity or result |
| --- | --- |
| Normal executable | `f1a63e6db297e40110a9ef44bede07906f2af8ddda5385793ba77d7ea60113b1` |
| E2E executable | `d031407f788e432cf70d414f799e3a59a5b2a571a7207207456fb5907d9f6cb2`, identical to the successful investigation app |
| Fresh driver | `e6db7b4de7d34bc48da09d1219fd17db5f63b4d193fee522be05e1cefe654540` |
| `go run ./cmd/dev check --run 20261116T142531Z-bc46c4a6fdac --race` | PASS, all six steps recorded |
| Fresh e2e bundle and driver, full native suite | PASS, 30 checks in 38.382s; one invocation, no retry |
| Playing/loading external cancellation | 65.11575ms / 45.74875ms, each below 200ms |
| Owned-process cleanup | All five processes exited gracefully, no forced cleanup, all PIDs absent afterward |

The full suite again covers three fullscreen cycles including Escape, parts,
replay, cancellation, zero ownership, eight-second live survival and actual
window close. Existing deadlines and predicates remain unchanged. The normal
bundle was built but not separately launched; native evidence belongs to the
explicit e2e bundle. libmpv remains the same verified internal hardcoded path.

Evidence is retained in this run's `evidence/`: assignment, recorded check log,
build logs/metadata, prelaunch source/artifact identities, `native-full/`,
execution result, copied `artifacts/`, integrity index and final handoff source
snapshot. The 15 bundle/driver files retain their prelaunch hashes. Only README,
stage-zero and this report change from the previous source snapshot. All local
ignored evidence and development bundles require preserving this checkout;
there is no remote artifact backup or release packaging claim.

This completes the additional repair/verification request using the already
implemented fix. Cumulative actual code history remains **four formal reviews /
eight fixes**; rebuilding and rechecking does not invent a ninth code fix.
Across this round and the previous investigation, there are now four passing
full suites and 51 complete fixed fullscreen cycles. **REV-001 remains CLOSED;
REV-002 remains OPEN pending separate independent disposition.** Manual
input/audibility evidence, exact-commit integration/CI and M2 acceptance remain
pending. No further repair or review loop is started by this completion.

## Concentrated investigation completed — 2026-11-16

The owner explicitly requested code analysis, better logs, a causal repair and
many experiments, without the intermediate review/fix/needs-human loop. This
continuation follows that instruction. It completes the scoped investigation;
it neither changes the frozen M2 acceptance contract nor claims independent
REV-002 closure or M2 acceptance. No formal review, commit, push or remote
mutation occurred. The historical stop proposals below are superseded for this
investigation by that owner instruction, not by an automatic budget reset.

### Cause and repair

The reproduced failure is an automation transition race. The driver considered
native content bounds, rendered layout and the fullscreen bit sufficient to
start the next transition. Those values can agree before AppKit delivers
`DidExitFullScreen`. In all three telemetry-only reproductions, the second
entry request actually reached the native setter **before** the first exit's
completion callback, by 3.156084 / 0.561584 / 1.304083ms. No second DidEnter
followed, and cycle 2 timed out at `entry-correspondence`.

Pinned mygo already forwards real `windowDidEnterFullScreen` and
`windowDidExitFullScreen` callbacks. Its setter compares a style bit and sends
`toggleFullScreen:`; returning from that setter does not mean the transition is
finished. The app previously used the callbacks only to invalidate rendering.
The instrumented app now records request/callback counters, monotonic relative
timestamps, style and content bounds in an e2e-only `e2e-fullscreen` node. The
existing strict playback observation is unchanged. Normal builds retain no
event history or diagnostic node, and their transition actions are unchanged.

The driver now waits for the expected request, DidEnter and DidExit counts,
together with its original state/geometry/layout/picture checks. It adds safe
phase/RPC metadata and an explicitly diagnostic `-stage fullscreen` prefix;
the default full suite remains intact. No additional sleep, retry, relaxed
predicate or increased deadline was used: fullscreen entry/exit retain 5s,
external cancellation 200ms, and post-disposal live survival 8s. The existing
500ms entry wait is unchanged. This repairs the automated sequence; arbitrary
rapid manual toggling is not established by these experiments.

This conclusion is supported by observed ordering and a controlled removal of
the callback guard. It does not distinguish whether AppKit internally ignores,
supersedes or rolls back an overlapping request. It does not prove that all
four older historical fullscreen failures had this exact cause. No libmpv
cause or user interference is asserted.

### Native experiments on darwin/arm64

Each attempt uses a fresh owned process and retains its original outcome.
Diagnostic prefixes stop after three fullscreen cycles and are not counted as
full acceptance. Fixed runs include final Escape exit.

| Experiment | Actual result |
| --- | --- |
| Old app with phase/RPC logs, before callback guard | 2/2 fail at cycle 2 entry-correspondence |
| Telemetry-only app, same transition behavior | 3/3 fail at the same predicate; all show request before prior DidExit |
| Same telemetry app, remove only the driver's callback guard | 3/3 fail again at cycle 2 entry-correspondence |
| Repaired driver, initial prefixes | 3/3 pass |
| Repaired driver, consecutive repeat prefixes | 10/10 pass |
| Final driver, complete five-scenario suites | 3/3 pass, 30 checks per suite, 37.61–38.05s each |

There are **24 suite invocations**, **36 owned app processes**, and **48 complete
fullscreen cycles in the 16 successful fixed invocations**. All owned processes
exit gracefully; there is no forced cleanup and post-run PID checks find none
remaining. Every fixed product trace has six requests alternating with the
corresponding three DidEnter and three DidExit events in the expected order.

Each full suite covers outside-repository bundle startup, synthetic picture and
progress, pause/seek/rate/volume/mute, responsive layouts, three fullscreen
cycles, P2/return-to-P1, ended replay, complete disposal, eight-second still-live
survival, loading/playing cancellation and dedicated loading/playing window
close. Externally observed playing cancellation is **44.174334 / 32.067292 /
69.937041ms**; loading cancellation is **38.710667 / 38.583542 / 38.765ms**.
All are below 200ms. Content captures are rendered rasters, not OS compositor
captures. Selected audio tracks do not prove physical audibility.

### Regression and candidate evidence

Delayed-entry/exit-callback regressions deliberately keep geometry/layout valid
while delaying the native completion. They fail before the guard because the
next action is sent prematurely. A missing-callback regression also fails
before repair because later transitions and PASS records are emitted. All pass
with the guard; a Go overlay removing only the guard makes all three regressions
fail again. The same overlay supplies the native negative-control binary.

The first full and e2e race runs failed: prepending the new fake diagnostic node
shifted existing tests' fixed node indices. Moving it after existing nodes
restores those tests without changing assertions or product behavior. Original
failures and passing reruns are retained. The worker's first telemetry compile
failure and its correction are retained too. Final checks on the final code:

- `GOTOOLCHAIN=local GOWORK=off go run ./cmd/dev check --race`: PASS, all six steps.
- `GOTOOLCHAIN=local GOWORK=off go test -tags e2e -race -count=1 ./cmd/my-bilibili ./cmd/preview-smoke ./internal/app ./internal/desktop`: PASS.
- Fresh driver build, scope comparison, frozen-input and runtime integrity: PASS.

Run: `20261116T140453Z-5d4d91e98a74`. The uncommitted final code snapshot contains
274 files and changes only 15 source/test files in `cmd/my-bilibili` and
`cmd/preview-smoke` relative to the prior handoff. README, stage-zero and this
report are updated afterward; the run separately snapshots those final docs.
Base `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` is provenance, not a candidate commit.

| Exact artifact | SHA256 |
| --- | --- |
| Final code manifest | `ac65f87d7324241f7e3dc611a73a519c870052db2b8a04b8f11e454c8b3e512b` |
| Final code archive | `8c7cdee2ef08ca2c02b918629bf27ec340b606e912543c9660b091b6f86e327b` |
| Final full-suite driver | `7970783e0122416ebb2314761d798c2431cab9e17337e3fbc3d18345719c1787` |
| Telemetry e2e app | `d031407f788e432cf70d414f799e3a59a5b2a571a7207207456fb5907d9f6cb2` |
| Runtime manifest, unchanged | `a5fb151f6c41d6c0eb12f214ec187581b0589dc57f37911887cf133eda99fb54` |
| 260-file investigation evidence index | `e13b870e600489ce0d791921c41f7113ead9ad9f730a113b37728e45a146df8a` |

The 13 fixed prefixes use driver `a5aef4155444939771280fd8feacb0ebd90f8b9cab41ad3abcd8a31cba1221b3`.
The final production source differs only by a comment correction; the fake
node ordering fix changes tests only. The prefix/control binaries were built
with `-trimpath`; the final full-suite binary was built without that flag.
Exact intermediate source snapshots, build metadata and binaries remain archived. The original trace driver's geometry-metadata copy issue was corrected
before callback experiments; that earlier failure is not used as callback proof.

Local evidence lives under this run's `evidence/final-code-candidate/`,
`final-implementation-scope.json`, `investigation/` and `investigation-index.json`.
It includes all six experiment batches, reports/captures, native event sequences,
original failed checks, passing reruns, overlay, binaries, telemetry app and
implementation worker return. The index hashes 260 files. These ignored local
artifacts have no remote backup or retention guarantee; preserve the run and
pinned runtime before deleting the checkout. The internal hardcoded development
libmpv path and packaged synthetic fixtures remain unchanged; no user library
picker/configuration was introduced.

### Disposition and remaining work

Prior M2-004 history remains **4 formal reviews / 6 fixes**. This investigation
adds one causal driver repair and one subsequent test-fixture correction, counted
conservatively as two implementation/verification iterations: **4 reviews /
8 fixes cumulatively**. Instrumentation preparation, 24 experiments and their
expected negative controls are recorded separately, not disguised review rounds.
The latest explicit owner instruction authorizes this investigation beyond the
prior single-attempt limits. No general workflow policy was amended.

The causal investigation is complete, with passing repeated native and offline
checks. Author verification is not independent closure: **REV-001 remains CLOSED;
REV-002 remains OPEN, BLOCKING class 5, pending separate reviewer disposition.**
Physical audibility, remaining manual pointer/keyboard/accessibility checks,
exact-commit integration/CI and owner M2 acceptance remain pending. Windows stays
optional P2/not-tested; calibrated physical sync/rate remains outside M1 scope.

Found, not done: retain the existing sequencing inventory without broadening
scope to a generic scheduler or product-wide transition redesign. Remote issues
remain 0/5 because no filing authority was granted. Further manual rapid-toggle
behavior is untested; no product defect is inferred from this driver experiment.
The next M2 continuation can use this exact evidence to address remaining
acceptance and independent disposition. This handoff requests no new automatic
review/fix/human loop. **M2 remains incomplete and unaccepted.** Older sections
below retain their original authorizations and outcomes.

## Historical native acceptance and fourth response review — 2026-11-16

The owner authorized one full bounded native acceptance invocation and one
fourth response review by the original reviewer. The invocation ran exactly
once at **2026-11-16 13:55:48 UTC** (21:55:48 Asia/Shanghai), failed after
**9.514 seconds**, and closed its one owned process gracefully. No code repair,
retry, extra diagnostic, commit or remote action was performed.

| Required behavior | Actual result |
| --- | --- |
| Outside-repository bundle startup, paused P1 picture and playback progress | PASS |
| Pause, seek to 5s, 2× speed, volume, mute and unmute | PASS |
| Narrow and wide layout with preserved playback/settings | PASS |
| First fullscreen entry and exit with state/geometry/layout corroboration | PASS |
| Second fullscreen entry sequence | FAIL: context deadline exceeded before the second capture completed |
| Third cycle, final Escape, P2, return to P1 and replay | Not reached |
| Explicit disposal with eight-second live survival; loading/playing cancellation and dedicated close scenarios | Not reached |
| Failure cleanup | PASS: normal window close, zero exit, socket removal and zero ownership; no force |

The report contains eleven passing product assertions and one passing failure
cleanup assertion. Five content captures are retained, including `fullscreen-1`
only. The first fullscreen capture shows the synthetic picture, player overlay
and Exit full control. Source ordering localizes the failure to the second
entry function; the current report does not identify the exact failed RPC or
predicate. Pre-entry state checking, entry action/observation and capture remain
possible locations. This is not evidence that the earlier second-exit cause
recurred, nor that OS transition completion was established. Content captures
are last-rendered rasters, not compositor screenshots. No user interference or
libmpv root cause is asserted.

Original reviewer `/root/m2_feature_review` completed **formal round four**,
independently of the implementation author. Targeted driver race, four-package
e2e race and the full six-step race gate all pass. All fourteen independently
rerun mechanism-removal controls fail actual tests as intended, without build
failures. No new scoped defect was found. **REV-001 remains CLOSED; REV-002
remains OPEN, BLOCKING class 5, P2; decision: needs-human.** The current failure
and missing required evidence prevent technical closure. The reviewer finds the
actual human escalation packet sufficient for the proposed bounded diagnostic
decision; that assessment is not permission to execute it.
The archived report is `evidence/m2-product-review-r4/review.md`, SHA256
`ebf8ccd9b79d841abbb53e254182880cf0343239137b1c47c55182228ecb1414`;
its hash index covers 53 files, with 54 standalone files retained including the
index. The controller verified the returned hashes and check outcomes.

Current continuation run: `20261116T135518Z-45ff0e769adc`. The exact tested and
reviewed 267-file source manifest is
`aa2b449e53cb0b5db723b4f326d8891a24fffee3080ecc2718a6ff338e5d0b60`;
archive `e1486186f9a52313704e6682546c8c617c29f04097b29d085cbaae40f0c44104`.
Both are retained in the previous run's `m2-handoff-candidate-r6/` evidence.
The sixth repair changes only four driver files; this continuation changes only
README, stage-zero and this report after review. Its final documentation
snapshot is separately identified in the run. Base
`da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` is provenance, not a candidate commit.

Driver SHA256 remains
`ae74f5a1477ef2f400038a33c36d70648786466d15f37cdefe0239947df76681`;
e2e app `a62bcf8242d88f4dbfda62be81d66ebc40cdb7316a5b86bae8567b9bd158877f`;
runtime manifest `a5fb151f6c41d6c0eb12f214ec187581b0589dc57f37911887cf133eda99fb54`.
Prelaunch verification rechecked every source file and mode, the archive,
68 worker evidence files, 14 bundle files, 30 runtime files and source-contract
pins. Post-invocation source and evidence hashes are unchanged; the owned
process is absent. Frozen spec0016 and all companion contracts remain unchanged.
The app still uses the internal hardcoded development libmpv path and packaged
synthetic fixtures; user-facing runtime configuration was not introduced.

The run's local-only `evidence/` contains the resumption assignment, exact command,
prelaunch verification, `product-native-r5/`, `native-r5-execution.json`, invocation
log, index/result, postnative integrity check, fourth-review assignment and
return, `human-escalation-r4.md` and the continuing finding inventory. Native
report SHA256:
`2c8a1e553f069fc369608fef02539359b223e0033f3b1d865e16f2f30f520b58`.
Ignored evidence is neither a remote attachment nor a backup; preserve it before
removing this checkout.

Cumulative budgets: architecture 3 reviews/1 fix approved; M2-001 2/1 approved;
M2-002 2/1; M2-003 2/2; M2-004 **4 formal reviews / 6 fixes**, with both explicit
extensions consumed; M2-005 incomplete. Repeated fullscreen acceptance has now
failed four times, without asserting a common cause for each failure. Seek has
two failures followed by six passes; rate selection has three passes after two
prior action failures. Remote issues remain **0/5**. No further repair, review,
diagnostic or native launch is authorized by this completed allowance.

Recommended next decision, if the owner continues: one focused read-only
second-entry diagnostic with owned native lifetime at most 60 seconds, preserving
current actions, predicates and deadlines. Record existing RPC timing, cycle/
substep and geometry comparisons using scratch instrumentation; first failure
stops the run and cleanup is bounded. Freeze the exact diagnostic diff, driver
hash and command before launching. This proposal grants no seventh repair,
retry or fifth review. Keeping M2 stopped remains an option. The explicit stop
follows [R4](../docs/agent-convergence.md#r4-bound-review-and-automatic-repair).

Observed vs inferred: current prefix passes, timeout and graceful cleanup are
observed; source order narrows the location but not the cause. The pinned
render/automation/geometry contracts support finite corroboration, not an
unavailable native callback acknowledgment. Found, not done: the same local R8
sequencing-category inventory links seek, popup and fullscreen without a common
native-cause claim or broader sweep. No remote filing authority exists. Missing
native scenarios, physical audibility and remaining input checks, exact-commit
integration, candidate CI and owner acceptance remain pending. Windows stays
optional P2/not-tested. **M2 remains incomplete and unaccepted.** Older sections
retain their historical authorizations and outcomes.

## Previous sixth repair and pending native verification — 2026-11-16

The owner authorized **one sixth scoped repair with offline verification**.
The prior proposal's full native invocation and fourth formal review were not
included. The repair is implemented, built and frozen; no native application was
launched and no independent review was dispatched in this continuation.

Only `cmd/preview-smoke/client.go`, `suite.go`, `picture.go` and new
`fullscreen_test.go` changed. The driver records ordinary native bounds and
page geometry, then corroborates render-cached fullscreen state with native
content bounds, rendered layout and control availability across two read pairs.
Entry cannot pass at ordinary dimensions; exit must restore the observed
ordinary geometry. It rechecks before capture, after capture before reversal,
and before the next entry. Legal mixed-frame transitions keep polling; missing
or malformed required geometry cannot appear as valid zero coordinates.

All three cycles, final Escape, paused-session/settings preservation, picture
checks and downstream scenarios remain. The original five-second transition
deadlines now cover each action and its observation/capture/precondition work;
there are no action retries, new fixed sleeps or timeout extensions. The initial
ordinary baseline has a bounded five-second read-only context. The existing
500ms entry wait is retained. Application/UI/backend/runtime/fixtures are
unchanged, and the development libmpv path remains internal.

| Criterion | Candidate-specific result |
| --- | --- |
| Reproduce the old sequencing defect | Old production flow fails the fake-RPC regressions; failing-before evidence retained |
| Verify added checks | Fourteen mechanism-removal controls fail actual tests as intended, with no build failures |
| Driver race tests | PASS: `go test -race -count=1 ./cmd/preview-smoke` |
| Related e2e race tests | PASS: four packages with `-tags e2e` |
| Full local gate and build | All six `go run ./cmd/dev check --race` steps PASS; driver build succeeds |
| Native product acceptance | NOT RUN for this repaired driver; prior failures and missing later scenarios remain |
| Independent disposition | No fourth review; REV-001 remains CLOSED and REV-002 remains OPEN, BLOCKING class 5, P2 |

The regression tests call the shared production three-cycle flow through fake
RPC. They cover delayed exit geometry, transient entry, cross-read and capture
contradictions, next-entry and Escape preconditions, successful cycles, mixed
exit frames, malformed observations, session preservation, cancellation/errors
and the shared deadline. During implementation, author coordination corrected
the fixture's nested part-button position and mixed-frame validation order;
these were part of the unfinished sixth repair. No unexpected post-repair
test failure occurred. Author checks do not constitute independent approval.

Current run: `20261116T133634Z-ed4a1b4db109`, handed off as **needs-human**.
Frozen spec0016/tasks and companion contracts remain unchanged. The exact
267-file repair manifest is
`f163d3f9f268743fe241754ce02350594546b62ccd1a715edbb4caa46c17ad9d`;
source archive `b9821d6b3ef2afc0bbb3353cec380e5e864104e69304a1625fcf3c476084ecaf`
contains those files plus its embedded manifest. Built driver SHA256:
`ae74f5a1477ef2f400038a33c36d70648786466d15f37cdefe0239947df76681`.
The controller verified source/archive hashes and modes, the exact four-path
delta, all 68 indexed worker evidence files, 14 unchanged bundle files and 30
unchanged runtime files. The unchanged e2e app remains
`a62bcf8242d88f4dbfda62be81d66ebc40cdb7316a5b86bae8567b9bd158877f`.
The final handoff snapshot adds only README, stage-zero and this report, and is
identified separately in the run. Base `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2`
is provenance only; there is no candidate commit or remote action.

Evidence is local-only under this run's `evidence/`: the immutable
`m2-004-repair-r6-assignment.md`, `repair-r6-worker/` packet with exact commands,
logs, controls, source/artifact manifests and design source pins;
`root-repair-verification.json`, `root-check-log-verification.json`,
`follow-ups-product-r6.md` and `next-verification-proposal.md`. Ignored evidence
is not an external attachment or remote backup and has no retention guarantee.

Cumulative budgets: architecture 3 reviews/1 fix approved; M2-001 2/1 approved;
M2-002 2/1; M2-003 2/2; M2-004 **3 formal reviews / 6 fixes of 6 authorized**;
M2-005 incomplete. Native failure history is unchanged: fullscreen three
occurrences; seek two before repair followed by five passes; action failure two
before repair, followed by passing rate selection twice. Expected old-flow and
mechanism-removal failures are regression evidence, not new native attempts.
Remote issues stay **0/5**. No seventh fix or fourth review is automatic.

Recommended next decision: authorize **one full bounded native acceptance
invocation of this exact driver, followed by one fourth formal response review
by the original reviewer**. Retain the five owned-process scenarios, 240-second
suite context, three fullscreen cycles, both parts/replay, 200ms cancellation,
eight-second live survival and bounded cleanup. Any failure consumes the one
invocation; preserve it and stop automatic repair. The review covers the scoped
repair and existing acceptance gaps; it grants no seventh fix. The exact pins,
proposed command and stop boundary are in `next-verification-proposal.md`.
This proposal is not execution authority; [R4](../docs/agent-convergence.md#r4-bound-review-and-automatic-repair)
requires the additional allowance. Keeping M2 stopped remains an option.

Observed vs inferred: the prior trace establishes the driver sequencing defect,
and deterministic checks establish the new guards' behavior. Actual native
success remains untested. Contract relied on: the pinned automation/render/
geometry sources and frozen spec0016. Finite corroboration is not an OS
transition-completion callback; capture remains last-rendered content rather
than compositor evidence. Found, not done: the same local R8 sequencing-category
inventory links seek, popup and fullscreen, without a common native-cause claim
or a broader sweep. No separate issue is filed because remote authority is
absent. Physical audibility, remaining inputs, later native scenarios,
exact-commit integration, candidate CI and owner acceptance remain pending;
Windows stays optional P2. M2 is incomplete and unaccepted. Older sections below
retain their own historical authorizations and outcomes.

## Previous fullscreen diagnostic and human decision — 2026-11-16

The owner authorized one focused fullscreen diagnostic lasting at most
60 seconds. It ran once at **2026-11-16 13:29:28 UTC** (21:29:28 Asia/Shanghai),
finished in **10.437 seconds**, reproduced the second fullscreen exit timeout
and closed its one owned process gracefully. No maintained product code changed.
Scratch Go overlays added sanitized call tracing; the app, runtime and fixtures
were unchanged. Diagnostic recording completed, but the product prefix failed.

The trace establishes a driver sequencing defect:

| Main-connection request | Observed behavior |
| --- | --- |
| 109–110 | First exit action followed by cached fullscreen `false`; the normal page still uses the enlarged 1710×1073 area. The driver accepts exit immediately. |
| 111 | Second entry is sent 0.105ms after that sample returns, 10.163ms after the first exit action response. |
| 113–115 | Second entry briefly reports cached `true`. About 502ms later, the native content bounds are back to 1120×760 and the rendered tree says `false`, with a `Fullscreen` button and ordinary page layout. |
| 116–119 | The content capture shows the normal page. The driver nevertheless sends its intended second exit toggle; the next samples show `true`, `Exit full` and fullscreen layout. |
| 299–314 | Waiting for `false` times out; subsequent samples still show fullscreen state, layout and 1710×1073 native content bounds. |

The intended exit is sent after the driver's own picture routine has already
observed ordinary window state, and it enters fullscreen. All relevant RPC
envelopes and action responses are valid successes. This is evidence for an
incorrect completion predicate and missing action precondition, not evidence
of a libmpv defect. The exact native style-mask/callback timing behind the
transient state reversal is unavailable. The trace adds timing overhead, and
the captures are last-rendered content rasters, not OS compositor screenshots.

The report records eleven passing prefix assertions and passing actual-window
cleanup. The first-cycle PASS means only that the original cached predicates
matched; this diagnostic shows why it cannot establish completed native exit.
Second/third cycle acceptance, later parts/replay, explicit disposal with live
survival, external cancellation and dedicated close scenarios remain missing.
Physical audibility, remaining input checks, exact-commit integration, candidate
CI and owner acceptance are still pending. Windows remains optional P2.

Current run: `20261116T132123Z-75d49142107c`, handed off as **needs-human**.
Frozen spec0016/tasks are unchanged. The diagnostic used the prior 266-file
handoff manifest `092fbcf12278a889967f5a02aa2a18197c5fb69807c75baf1e515d5519b34265`
and archive `458dd9fe4cb67778b14c6706fc11e87da4011b3b3fec73cf0d11507f13a4acbd`.
The final source snapshot adds only this report, README and stage-zero updates;
its manifest/archive and validation are attached to the run separately.
Base `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` is provenance, not this
uncommitted candidate's identity. No commit or remote action occurred.

Diagnostic driver SHA256:
`a858b71594fc6080fde0b36202c18e3aee9e63bfbb18493cc448957d66d5538a`;
overlay manifest `324aaa9beab79ff37e715aa7da25d23e718fd0c1d0e2d2a60d0b2099d48e1afd`;
trace `042ed85a46a9baba638bcdbe690bc7390800f53f4175716fbc0764f6fffbc78d`;
report `561f41e360bf58952688edc73967350034b3e7d65eac70484da60cd6cbcf265c`.
The unchanged app SHA256 is
`a62bcf8242d88f4dbfda62be81d66ebc40cdb7316a5b86bae8567b9bd158877f`.
Targeted diagnostic race tests and the overlay build pass. Controller checks
verified all 266 maintained source files/modes, 14 bundle files, 30 runtime
files and frozen diagnostic artifacts before launch. Product race gates from
the previous candidate remain separate; no fourth independent review ran.

Local-only evidence is under the current run's `evidence/`: `diagnostic-source/`,
`observation-contract.md`, `capture-contract-supplement.md`,
`prelaunch-verification.json`, `native-diagnostic-assignment.md`,
`fullscreen-diagnostic-r1/`, `diagnostic-metrics.json`, `diagnostic-result.md`,
`diagnostic-result-index.json`, `repair-proposal.md` and
`follow-ups-fullscreen-diagnostic-r1.md`. These ignored files are not remotely
backed up; no retention guarantee or external attachment is claimed.

**REV-001 remains independently CLOSED; REV-002 remains OPEN, BLOCKING class 5,
P2.** Cumulative budgets: architecture 3 reviews/1 fix approved; M2-001 2/1
approved; M2-002 2/1; M2-003 2/2; M2-004 **3 reviews / 5 fixes of 5 authorized**;
M2-005 incomplete. Fullscreen has now failed three times. Seek has two pre-fix
failures and five subsequent passes; the action failure occurred twice before
repair, followed by passing rate selection in acceptance and this diagnostic.
Remote issues stay **0/5**. No count resets, new finding or independent closure
is claimed. No further native launch is authorized.

Recommended next decision: authorize **one sixth M2-004 driver repair, one full
bounded acceptance invocation, and one fourth formal response review by the
original reviewer**. Limit the repair to fullscreen sequencing in
`cmd/preview-smoke`: correlate requested cached state, native geometry and
rendered layout; require ordinary geometry restoration before another entry;
revalidate before a reverse action, including after capture. Keep each existing
five-second transition bound, strict errors, single actions and all three cycles
including Escape. This is an observed-state barrier proposal, not a claim that
geometry exposes a native completion callback. If the available observations
cannot support it, stop for a scope decision. Do not silently add sleeps,
retries, timeout extensions or application/framework changes.

The proposal includes discriminating regressions and negative controls for the
recorded transient/mismatched sequence, targeted and full race checks, one
unchanged-scope 240-second acceptance suite with bounded cleanup, then one
consolidated response review. Failure consumes the added allowance and stops
automatic work. Keeping M2 stopped remains an option. No part of this proposal
is authorized by the completed diagnostic. Additional authority is required by
[R4](../docs/agent-convergence.md#r4-bound-review-and-automatic-repair).

Observed vs inferred: the call sequence, native geometry and cached/rendered
states are observed; complete OS transition timing is unknown. Contract relied
on: frozen spec0016, the pinned automation/window/render/capture sources and
unchanged native/cancellation/survival gates. Found, not done: the visible-state
sequencing category now links seek, popup and fullscreen in the same R8 local
inventory, without asserting a common native cause or expanding repair scope.
No separate instance issue is filed; remote authority is absent. M2 remains
incomplete and unaccepted. The hardcoded development libmpv path stays internal,
and the full UI scope is unchanged. Older sections retain historical outcomes.

## Previous fifth repair and bounded acceptance — 2026-11-16

The owner authorized one fifth M2-004 driver repair and one subsequent full
bounded acceptance invocation. Both are now consumed. The driver now waits for
exactly one usable popup item before sending its action, at the two assigned
speed/parts transitions. One two-second context bounds opening, observation and
selection; no failed action is retried. Safe action errors retain the key,
recognized protocol category and deadline identity without raw native payloads.
Only four `cmd/preview-smoke` files changed. The app, UI, backend, fixtures and
runtime were unchanged; the hardcoded development runtime remains internal.

The native invocation started **2026-11-16 13:13:38 UTC** and failed again in
the second fullscreen cycle. Eleven product assertions pass: bundled startup
outside the working directory, advancing picture, pause, seek to 5s, 2× selection,
volume, mute/unmute, narrow/wide layouts and the first fullscreen cycle. Actual
window-close cleanup also passes: zero exit, socket removal, zero ownership,
no cleanup error and no forced cleanup. This is partial verification, not full
product acceptance. Only one product process was started.

The repaired speed popup now passes natively. The parts popup has deterministic
coverage but was not reached. `fullscreen-2.png` again shows the normal wide
page, despite the prior fullscreen-entry predicate. The fixed sequential suite
localizes the timeout after that capture to waiting for the second exit state.
The report has no final per-call trace establishing cause or ownership. Earlier
owner-reported accidental UI interaction does not establish a cause for this
repeated symptom. No connection to popup timing, seek or libmpv is established.

| Done criterion | Evidence and remaining gap |
| --- | --- |
| Scoped popup sequencing and error handling | Failing-before and two negative controls preserved; targeted race, four-package e2e race and full six-step race gate PASS |
| Exact source and artifacts | Controller verified all 266 source/archive files and modes, exactly four changed paths, 14 unchanged bundle files and 30 runtime files |
| Native product acceptance | FAIL at second fullscreen exit; eleven product assertions and graceful cleanup PASS |
| Original review findings | Final independent round 3: REV-001 remains CLOSED; REV-002 remains OPEN, BLOCKING class 5, P2 |
| Later behavior | Fullscreen cycles 2/3, parts/replay, explicit disposal with eight-second live survival, playing/loading cancellation and dedicated close scenarios not reached |
| M2 integration/acceptance | Physical audibility and remaining input checks, exact commit, candidate CI and owner acceptance remain missing; Windows is optional P2 |

Current run: `20261116T130426Z-8c516058a55b`, handed off as **needs-human**. Frozen spec0016 and task graph
remain unchanged. Source candidate manifest SHA256:
`c93b6a0f1ada0b5d556566219d7e44761c5fd818612b91975e6772624d032638`;
archive: `214931577c951a4e48ae96b82855252262ff793e442361bd1f34dc5f3a837bf7`.
Driver: `077e1cd2740421a3ce9032653ae9b987a082108864379f028c4cf7e36babedb8`.
Unchanged e2e app: `a62bcf8242d88f4dbfda62be81d66ebc40cdb7316a5b86bae8567b9bd158877f`.
Native report: `7e2d0d7e80ea03aac6ffac3cf69b19cb38ed0703f733b593f6c45db7ed4fa1e0`.
Base `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2` is provenance only;
this remains an uncommitted candidate. The final handoff snapshot includes only
three subsequent documentation changes, separately identified in the run.

Evidence is local-only under this run's `evidence/`: `repair-r5-worker/`,
`root-repair-verification.json`, `native-acceptance-assignment.md`,
`product-native-r4/`, `native-r4-index.json`, `native-r4-result.md` and
`m2-004-review-r3-assignment.md`, `human-escalation-r3.md`,
`follow-ups-product-r5.md` and `m2-product-review-r3/`. Worker and independent
reviewer logs record the exact Go commands under Go1.27.3 darwin/arm64. The older 41-pass laboratory result remains separate
unchanged-dependency evidence; it does not fill the missing product scenarios.
The initial recorder checkpoint rejected the unsupported phase `verification`;
the controller corrected it to `validation` while the single authorized native
invocation ran. There was no second launch.

Original independent reviewer `/root/m2_feature_review` completed the
**final M2-004 formal round 3** with **needs-human**: REV-001 remains CLOSED;
REV-002 remains OPEN, BLOCKING class 5, P2. The reviewer independently verified
the exact source/artifacts, ran targeted race, four-package e2e race and the full
six-step race gate successfully, and reproduced failing negative controls for
removing the popup wait and restoring the old RPC error behavior. Two initial
negative-overlay attempts used a `/tmp` alias that Go did not apply; those PASS
results are retained as invalid controls and are not closure evidence. Corrected
canonical-path overlays fail as intended. No new finding or advisory was raised.
Independent report SHA256:
`6fcbcb468b91ad312eb7095e9a5758df4044b0cabd52dc403064366d8a84a918`.
The reviewer assessed the human packet as sufficient for a scoped decision;
this is not owner authorization or a waiver of missing native checks.

Cumulative budgets are architecture 3 reviews/1 fix approved,
M2-001 2/1 approved, M2-002 2/1, M2-003 2/2, M2-004 **3 reviews / 5 fixes of 5
authorized**, and M2-005 incomplete. No new run or task resets these counts.
Seek failed twice before repair and has since passed four times, including the
diagnostic; action error occurred twice before the popup repair and now passes;
fullscreen has failed twice. Remote issues remain **0/5**, with no filing
permission or remote dedupe claim.

Recommended human next step: **one focused fullscreen diagnostic, at most
60 seconds**, with an unchanged app/runtime and pinned synthetic fixtures,
a fresh owned process, bounded cleanup, per-action timestamps, available native
state and geometry, and captures around the first two transitions. Prepare the
exact diagnostic assignment before launch; record unavailable observations.
Do not bundle a sixth repair or another full acceptance invocation. Keeping M2
stopped is also a valid choice. The final review allowance is now exhausted. Additional repair/review
allowances require explicit human scope and counts under
[R4](../docs/agent-convergence.md#r4-bound-review-and-automatic-repair).

Observed vs inferred: current native assertions, popup regression behavior and
repeated fullscreen capture/state mismatch are observed; timeout localization
follows the unchanged sequential control flow. Fullscreen root cause is unknown.
Contract relied on: frozen spec0016, existing automation protocol, pinned mygo
frame behavior and unchanged native/cancellation/survival gates. Found, not done:
no unrelated discovery; the existing REV-002 inventory retains all acceptance
gaps. The repeated fullscreen symptom is one local investigation linked to both
failures under R8, without duplicate instance issues. The narrow popup structural
proposal is implemented; no broader framework or scope reduction is claimed.
Older sections below retain their historical authorizations and outcomes.

## Previous bounded diagnostic — 2026-11-16

The owner authorized one additional short action-response diagnostic. It ran
once at **2026-11-16 12:59:58 UTC**, for **2.812 seconds**, using the unchanged
app, runtime and fixtures. Product source was not edited. One owned process
closed gracefully, with no forced cleanup. The diagnostic harness completed
successfully while preserving the reproduced failing product prefix; its PASS
is not product acceptance.

The cause of the reproduced speed-menu failure is now observed:

1. The `speed-menu` action returns a valid matching-ID success response.
2. The driver sends `speed-2` only **0.165ms after the first request started**
   (0.005ms after its response).
3. The second response has the correct ID and reports **`no_element`**, with
   `no element has the key "speed-2"`. This was not invalid JSON or an ID mismatch.
4. The immediate post-failure tree still lacks that item. The next 100ms-spaced
   observation contains the item; later samples and a screenshot show the open
   six-option menu. No action was retried. The first observed presence is
   101.681ms after the error, not a measurement of the exact render time.

Pinned mygo source confirms that actions resolve keys against the current tree,
and visible-window `Settle` does not force the next frame. The driver sends the
item action before the popup is rendered. The defect belongs to the test-driver
sequence; this evidence does not establish a product rate/backend defect.
It supports the same explanation for the preceding rate-stage failure, whose
missing per-call trace remains missing. The earlier fullscreen mismatch has
not been causally explained or cleared.

Current run: `20261116T125833Z-07e6332e5800`, handed off as `needs-human`.
The prelaunch 265-file manifest was
`f112d6f32c82697a33f7098a5b19ef407019988062f4e2af9dec3e142a48d669`, archive
`62bf0e26895c1c545ef4b8ad774fbaa3644bb13d2989238876e3acb217a9fef6`.
Product source remains identical to the independently reviewed repair candidate
recorded below. All source files, five artifact entries and 14 bundle files
were verified unchanged before launch. The scratch diagnostic binary SHA256 is
`c24804bbf407b45163f1918b0275cf5ed9c6dbe9cfe2e3da4c09e48bc825ca0d`;
its Go overlays and binary are retained as diagnostic evidence only.

Evidence under the current run's `evidence/`: `resumption-and-diagnostic.md`,
`prelaunch-verification.json`, `diagnostic-artifacts.json`, `diagnostic-source/`,
`action-diagnostic-r1/`, `diagnostic-metrics.json`, `diagnostic-result.md`,
`diagnostic-result-index.json`, `repair-proposal.md` and
`follow-ups-product-r4.md`. The trace retains sanitized action keys, matching
IDs, recognized error codes, control availability and safe playback state;
raw native payloads are not published. Only the three handoff documents change
after diagnosis; their final snapshot is attached separately to the run.

Recommended decision: authorize **one fifth M2-004 driver repair and one
subsequent bounded acceptance invocation**. Add a bounded wait for exactly one
usable popup item before sending its one intended action; apply it to the two
explicit popup transitions in this suite: speed-menu → speed-2 and parts-menu
→ part-part-1. The latter shares the same source sequence but has no observed
native failure. Preserve strict protocol errors, deadlines, cancellation and
live-survival requirements; do not retry failed actions or add arbitrary sleeps.
Include a discriminating delayed-node regression, error/deadline cases, targeted
and full race checks, then freeze the exact rebuilt driver before the one native
invocation. The detailed path scope and checks are in `repair-proposal.md`.
This proposal has not been implemented or authorized by the diagnostic allowance.

This is the second observed visible-frame sequencing defect in the driver,
following the earlier seek input diagnosis. Under R8, retain one local
structural proposal for these explicit popup transitions; do not file separate
instance issues or speculate about fullscreen. There is no generic automation
framework or broader UI sweep in the proposed scope.

**REV-001 remains CLOSED; REV-002 remains OPEN, BLOCKING class 5, P2.**
M2-004 stays at **2 formal reviews / 4 fixes of 4 authorized**, with the final
formal review reserved for the intended complete response. M2-002/003 remain
at two reviews and one/two fixes, architecture 3/1 approved, M2-001 2/1 approved.
M2-005 is incomplete. This one diagnostic allowance is consumed; no fifth fix,
full acceptance rerun or new formal review was dispatched. Remote issues remain
0/5; the structural proposal stays local because filing authority is absent.

Failure history: the action error has occurred twice (retry plus diagnostic);
seek failed twice before repair and has since passed in two acceptance attempts
and this diagnostic; fullscreen failed once. No counter is reset. Remaining
full native checks, physical audibility and interactions, optional Windows,
candidate CI, exact commit and owner milestone acceptance retain their prior
dispositions. No unrelated discovery or product-scope reduction is claimed.

Observed vs inferred: current RPC error, missing-then-present item and pinned
visible-frame semantics establish this popup ordering defect. No repair
success is yet observed. Earlier user interaction and fullscreen causality
remain separate and unresolved. Contract relied on: frozen spec0016,
UI/automation contracts, pinned mygo action/tree behavior and R1–R9. Evidence is
local-only beneath `.agent/`; preserve it separately from Git. The earlier
sections below retain their original facts and historical next actions.

## Previous unchanged retry — 2026-11-16

The owner reported accidentally interacting with the prior test UI and explicitly
authorized one retry. The controller reran the exact existing acceptance driver
and app, without source changes, instrumentation, rebuilt artifacts or relaxed
checks. The prior failure remains preserved: reported interference is evidence
of interaction, not proof of the fullscreen failure's cause.

The retry started at **2026-11-16 12:55:10 UTC** and failed **before fullscreen**
with `product: automation act response invalid`. Startup paused picture,
progressing playback, pause and seek to 5s pass again. The only process exits
gracefully, with zero ownership, socket removal and no forced cleanup. Thus the
report records four product passes plus cleanup, not full acceptance.

Read-only source inspection localizes the failure to opening the speed menu or
selecting 2×: the seek assertion was recorded, while the rate assertion was not.
The report does not identify which of those two actions failed. The client uses
the same error text for malformed protocol responses, missing/mismatched response
IDs and explicit RPC errors. No raw response or final node tree was retained,
so the precise cause is unknown. A popup/frame timing explanation remains an
unverified hypothesis; no repair follows from it.

This retry neither reproduces nor clears the prior fullscreen mismatch. It also
does not erase the previous invocation's rate/volume/mute/layout and first-cycle
passes. The full required native checks remain incomplete; later parts/replay,
cancellation, live disposal survival, dedicated close scenarios, audibility and
remaining interaction evidence were not established by this retry.

- Current run: `20261116T125424Z-b5a66deeb31c`, handed off as `needs-human`.
- Invoked source identity: 265-file manifest
  `3bebd5a634272b21e4c222ab396fcff6cce89ade8e6d76ba2f776ae65de76ac7`, archive
  `b8d76ad5d4035398d8e8b3d248898b5225d1f4d7cf93267246cab5b99e423243`.
  This is the prior documentation publication: product source remains identical
  to reviewed repair manifest `a6445aba9109de797f42f2e59397349324efc69a3ceaa066f9c33d6467b8c6b9`.
- Driver/app/runtime/assets are the unchanged identities in the repair section
  below; all 265 source files, five artifact entries and 14 bundle files were
  reverified before launch. The driver revalidated runtime and fixture integrity.
- Native report SHA256:
  `10fe547ba9e83577b242b0e514cfd4d6d5fbbea52b5bc82e7d054bd1ad866996`.
  Evidence: `resumption-and-acceptance-r3.md`, `prelaunch-verification.json`,
  `product-native-r3/`, `acceptance-r3-index.json` and `acceptance-r3-result.md`
  under the current run's `evidence/`. The final handoff manifest is separately
  attached after only README, stage-zero and this report are updated.

The original independent dispositions remain: **REV-001 CLOSED; REV-002 OPEN,
BLOCKING class 5, P2**. This unchanged retry has not been submitted as a new
formal review response because the intended complete verification evidence is
not ready. M2-004 remains at **2 formal reviews / 4 authorized fixes consumed**;
M2-002/003 remain at two reviews each, with one/two fixes respectively. The last
M2-004 review allowance is preserved. Architecture remains 3/1 approved and
M2-001 2/1 approved. This authorized unchanged invocation is not a fifth code
fix. No further retry or repair is authorized. M2-005 remains incomplete.

Failure history: seek failed twice before repair and has now passed twice;
fullscreen failed once; the current action-response failure occurred once.
No common root cause is established. The same finding inventory continues in
`follow-ups-product-r3.md`, with remote issue count **0/5** and no filing
permission or remote dedupe claim. No new unrelated defect or systemic
follow-up is established. Windows/CI/exact-commit/owner-acceptance gaps persist.

Recommended next action: one separately scoped diagnostic of the automation
action boundary, at most 60 seconds and one owned test process, to retain the
requested key, sanitized protocol error classification and relevant rendered
node availability. Use the same synthetic inputs and bounded cleanup. It would
not authorize a fifth repair or full acceptance rerun. Keep the prior fullscreen
question unresolved rather than assuming this failure has the same cause.
A repair proposal needs an observed cause and owning scope first. Alternatively
keep M2 stopped. No milestone acceptance, commit, push or release is claimed.

Observed vs inferred: current exact-artifact passes, generic action failure and
graceful cleanup are observed; accidental interaction in the earlier invocation
is the owner's statement. Its causal role, the exact failing action and any
shared timing cause remain unknown. Contract relied on: unchanged spec0016,
UI/automation contracts and R1–R9 with the owner's explicit one-retry exception.
Local race checks retain their original exact-code scope; no redundant code
check or new native attempt was run after this failure. All `.agent/` evidence
remains local-only and must be preserved separately from Git.

The following repair continuation and first-stop sections retain their original
facts and proposed actions; this section supplies the current state.

## Previous repair continuation — 2026-11-16

The owner explicitly authorized one additional M2-004 repair iteration, one
bounded seek diagnostic and one new acceptance invocation. These allowances
are consumed. The original reviewer technically closes **M2-004-REV-001**
(report snapshot corruption) and accepts the repaired seek component of
**M2-004-REV-002**. REV-002 remains **OPEN, BLOCKING class 5, P2** because
required native verification fails later, during the second fullscreen cycle.
M2 and M2-005 remain incomplete; no fifth repair or native relaunch is authorized.
The full agreed fixture UI and lifecycle criteria remain unchanged.

Current run: `20261116T123605Z-88af396491f8`. Prior run
`20261116T115302Z-a835926ae66c` and its failed evidence remain closed and
unchanged. The contract is frozen spec 0016, based on
`da8804b58f8c3e987c3ac2a8e350371d57f8bfe2`; the candidate is uncommitted.

| Current repair identity | SHA256 |
| --- | --- |
| 265-file source manifest | `a6445aba9109de797f42f2e59397349324efc69a3ceaa066f9c33d6467b8c6b9` |
| Source archive | `fc06e44fa551a5b99f34cd5890e6f0695b717f548206b9a3f8bcc676f40ca052` |
| New `preview-smoke` driver | `97d48dc3b703db89b96f5d6c4825c824c61a776d3f33c96237da2abc0317d195` |
| Actual second product report | `6c49f14f176ffc03b0183c1340ab6d5e78381b09577f10eed4a2076844433409` |

The driver is `.agent/m2-integration-repair-r4/artifacts/preview-smoke`;
it supersedes both earlier smoke binaries. All 14 files in the original normal
and e2e app bundles rehash unchanged. Their executable identities remain those
in the historical artifact table below. The runtime and fixture assets are
unchanged. The final documentation publication manifest is retained separately
in the current run; it adds only this report and README/stage-zero updates to
the tested source, not a new product implementation.

### Observed repairs and verification

Exactly four driver paths changed: `cmd/preview-smoke/main.go`, `client.go`,
`slider_test.go`, and new `report_test.go`. Reports now copy each observation
and clone its nested `Parts` slice, retaining nil semantics. The regression
fails against the old implementation; an independent negative check also fails
when only the nested clone is removed. Historical snapshots in the new native
report are distinct and correctly retain their respective states.

The single 4.78-second seek diagnostic recorded pointer Down/tree/Up only
0.568ms apart, while the visible slider tree still held the old value. Pinned
UI source established that visible-window `Settle` does not force a frame,
whereas the headless tester builds a frame for each input. The driver now holds
the pointer until the actual numeric slider value reaches the target, with a
bounded two-second observation wait and bounded release on both success and
failure. Regression evidence distinguishes the old sequencing and malformed
observations. The second actual product invocation successfully seeks to 5s.
No app, desktop, playback adapter, suite deadline or cancellation bound changed.

Worker and original reviewer targeted, e2e-tagged and full
`go run ./cmd/dev check --race` checks pass on Go 1.27.3, darwin/arm64.
Independent source/archive/artifact checks match. The worker removed write
permissions after generating its artifact manifest: the external driver is
0555 and evidence files 0444, versus the manifest's pre-freeze 0755/0644.
This documented storage-mode change leaves content hashes and executability
intact; it is not an unrecorded source change.

### Actual second native invocation

The only new acceptance invocation started at **2026-11-16 12:43:10 UTC**.
It records 11 passing product assertions plus graceful cleanup, then exits 1
with `product: context deadline exceeded`. Actual passes cover startup paused
intent, progressing playback, pause, seek to 5s, rate 2, volume 0.35,
mute/unmute, narrow/wide layouts and the first complete fullscreen cycle.
One process was launched; it exited gracefully with no forced cleanup.

The second fullscreen cycle reached its capture, then timed out waiting for
`Fullscreen == false`. `fullscreen-1.png` shows the fullscreen video/overlay;
`fullscreen-2.png` instead shows the normal wide detail page, despite the prior
state predicate reporting fullscreen. Therefore the second visual entry is
not claimed as successful. The observed state/capture disagreement is real;
its cause is unknown. Asynchronous transition or frame timing is an
investigation hypothesis, not a basis for a repair.

| Criterion | Updated evidence and remaining gap |
| --- | --- |
| AC01 catalog/resources | Integrity/build and outside-cwd P1 launch pass; full two-part native acceptance pending |
| AC02 playback ownership | Approved deterministic use-case tests; native part switch/replay not reached |
| AC03 player UI | Approved deterministic tests; actual seek/rate/volume/mute and narrow/wide layout pass; repeated fullscreen fails, other required interaction evidence pending |
| AC04 package/lifecycle | Package starts and failure cleanup is graceful; product cancellation, eight-second live disposal survival, both parts and audibility remain unestablished |
| AC05 isolated development | Approved contracts, two worktrees, verified integration and checks retained; exact candidate commit absent |
| AC06 review/repair/escalation | Original reviewer independently verifies this bounded repair and receives the real second escalation; full milestone review/acceptance pending |

Later P2/replay, cancellation, live survival and dedicated window-close scenarios
were not executed. Failure cleanup does not establish those scenarios. Physical
audibility and unexercised keyboard/pointer interactions remain unverified.
The separate laboratory result remains 41 PASS / two not-tested categories;
it is adapter regression, not completion of product acceptance. Windows is
optional P2/not-tested. CI has not run for the changed candidate. M1 acceptance
is unchanged.

### Independent disposition and cumulative limits

Original reviewer `/root/m2_feature_review` performed M2-004 **formal round 2**,
limited to the repair response and affected evidence. This does not consume a
new M2-002/003 round or reopen architecture. Its exact report and hash index are
retained in `evidence/m2-product-review-r2/` in the current run. Report SHA256:
`17082cdefc7665899b7b7f1573776413d8a858db333416722201020f77151d80`.
No new finding
or high-severity claim was raised. REV-001 is technically closed; REV-002 stays
open with its seek component accepted and the later native gaps preserved.

| Scope | Cumulative formal reviews | Cumulative fixes |
| --- | --- | --- |
| Architecture/continuation | 3, approved | 1 |
| M2-001 | 2, approved | 1 |
| M2-002 | 2, final native dimensions pending | 1 |
| M2-003 | 2, final native dimensions pending | 2 |
| M2-004 | 2, required native verification open | **4 of 4 authorized, exhausted** |
| M2-005 | Incomplete dependencies; this is a stop handoff | No product repairs charged here |
| Runtime prerequisite | 1, approved | 1 |

The fourth fix comprises the confirmed snapshot and driver sequencing repairs.
The prior seek failure occurred twice (first acceptance and diagnostic), then
passed in acceptance r2. The fullscreen failure has occurred once. Counts and
failed evidence are preserved; changing runs or agents grants no allowance.

### Concrete next decision and retention

Recommended next action: authorize **only one focused fullscreen diagnostic,
with a maximum 60-second stage**, one owned process/window, pinned synthetic
fixtures/runtime, a temporary working directory and bounded cleanup. Record
native fullscreen state, geometry, rendered observations and callbacks to
identify the owning boundary and observed cause. The diagnostic would permit
neither a fifth product repair nor another full acceptance invocation. Produce
a concrete repair proposal only after observing the cause; alternatively keep
M2 stopped. No waiver, deadline extension or UI reduction is proposed.

This is a real human escalation, not a drill. The original reviewer assessed
`evidence/human-escalation-r2.md` as sufficient for this scoped decision within
M2-004 round 2; that receipt is not owner authorization. The current run is
handed off as `needs-human`. Commit, push, publication and owner milestone
acceptance remain separate gates.

Current local evidence root:
`.agent/runs/20261116T123605Z-88af396491f8/evidence/`.
Read `resumption.md`, `repair-r4-worker/`, `diagnostic-result.md`,
`seek-diagnostic-r1/`, `acceptance-r2-stop.md`, `product-native-r2/`,
`m2-product-review-r2/`, `follow-ups-product-r2.md`, `human-escalation-r2.md`
and the final handoff manifest/checkpoint. The prior findings inventory is
continued by an immutable supplement, not rewritten. Product source and
bundles remain frozen; only the three handoff documents changed afterward.

Observed vs inferred: the diagnostic trace, pinned frame behavior, native
control passes and later timeout/capture disagreement are observed. A shared
cause between seek and fullscreen is not established. Contract relied on:
frozen spec 0016 AC01–06 and its UI/automation contracts, pinned input/frame
semantics and owner-approved R1–R9 with the single fourth-fix exception.

Found, not done: no unrelated defect is claimed. Existing Windows, CI,
audibility, exact-commit and owner-acceptance gaps stay in the same inventory.
Logical remote issues remain **0/5**; no remote filing authority or dedupe is
claimed. No systemic follow-up warranted by current evidence; the later
fullscreen symptom alone does not establish the seek root cause elsewhere.
Ignored `.agent/` evidence and app bundles are local-only and not backed up by
Git; preserve them before removing this checkout.

## Historical first stop — preserved record

The following records the first product candidate and stop before the owner's
additional iteration. Its open finding status, budgets and proposed next action
are historical; the continuation above supplies the current disposition.


- Date: 2026-11-16; controller `/root`.
- Contract: [0016 continuation](../specs/0016-m2-policy-continuation/spec.md),
  including its shared, UI and automation contracts; original 0015 is preserved.
- Run: `20261116T115302Z-a835926ae66c`; previous run
  `20261115T133635Z-984c0955c8cb` remains closed and unchanged.
- Base: `da8804b58f8c3e987c3ac2a8e350371d57f8bfe2`; candidate is **uncommitted**.
- Exact 263-file product manifest:
  `c97da85468f3b876684473e3731e205ec31c5db17984df8db4f2fcacb02a7091`;
  source archive:
  `389f94e2485f3ad3b389a502e22ea2342b03124d7614e9721de2fafcc0ee4896`.
  These identify product code before this documentation-only handoff. The final
  handoff publication manifest is retained separately in the run.
- Outcome: automatic repair/native relaunch stopped under R4; human decision
  required. M2 and M2-005 completion are not claimed.

## Goal and delivered scope

Deliver one native macOS fixture detail page with two distinct original
30-second parts, the full agreed player interface, developer packaging and an
independently reviewed development loop. The owner approved worktree creation
and continuation. No smaller UI scope was adopted.

Application use cases, fixture validation/assets, desktop view, native
composition, packaging and a bounded smoke driver are present. Two workers used
separate worktrees from the same frozen baseline with nonoverlapping writes;
the controller verified their manifests and integrated only assigned paths.
M2-004 added 24 files without altering the approved 239-file baseline.

The normal `.app` bundles fixture resources resolved relative to its executable.
It uses the verified absolute development libmpv path without a user picker or
setting. Bundling libmpv and dependencies for user distribution remains future
work. The normal build excludes test automation. No signing, notarization,
service playback, commit, push or release was performed.

Local artifacts beneath `.agent/m2-integration-worker/artifacts/`:

| Artifact | SHA256 |
| --- | --- |
| Normal `MyBilibili.app/Contents/MacOS/my-bilibili` | `645f1d24cd9e69930fb1a906358ee7268c881c28983c7d77c9598527ec326bf6` |
| E2e `MyBilibiliE2E.app/Contents/MacOS/my-bilibili` | `a62bcf8242d88f4dbfda62be81d66ebc40cdb7316a5b86bae8567b9bd158877f` |
| `preview-smoke-final` | `6a876dac73f2a98109efb107fe9f385e4aa6ddd65073e95ed50300e5ed6d1ff8` |

The earlier `preview-smoke` binary is superseded. These artifacts are available
for inspection; this handoff does not authorize another native invocation.

## Actual checks and acceptance gaps

Go 1.27.3, darwin/arm64, `GOTOOLCHAIN=local GOWORK=off`. Worker final full
`go run ./cmd/dev check --race`, scoped package race tests, e2e-tagged race tests
and fresh normal/e2e package builds passed. The independent reviewer also ran
scoped/tagged race tests and the full race check. These do not prove native
behavior or changed-candidate CI.

The separately assigned laboratory invocation passed all 41 implemented
assertions, with two explicit not-tested categories and no forced cleanup.
External Loading/Playing cancellation took 70.651209ms / 34.115875ms. It used
the approved 239-file candidate, whose laboratory dependencies are unchanged
in the product candidate. It is adapter regression, not product acceptance.

The first product invocation started at **2026-11-16 12:27:55 UTC** from a
temporary working directory. It launched one owned process and failed with
`product: seek control: context deadline exceeded`. The driver reached its
startup picture/pause, progressing picture and pause assertions. Two synthetic
captures are retained. Cleanup closed the actual window with zero exit and
socket removal; no forced cleanup was used.

However, the driver retained pointers to a reused observation variable. Its
startup, progress and pause report entries were overwritten with the same
later paused snapshot at 458.333333ms. The original report remains untouched;
those serialized observations are not reliable historical snapshots. Successful
control flow and captures are partial evidence, not complete native approval.

| Criterion | Evidence | Current disposition |
| --- | --- | --- |
| AC01 catalog/resources | Distinct 30-second assets, integrity/resource tests, builds and actual outside-cwd P1 launch | Partial; full product verification pending |
| AC02 playback ownership | Independently approved deterministic use-case tests | Native part switch/replay not reached |
| AC03 player UI | Independently approved deterministic tests; actual wide picture/controls capture | Seek check failed; native narrow/fullscreen/keyboard and remaining controls pending |
| AC04 package/lifecycle | Actual package startup and graceful failure cleanup; separate 41-assertion laboratory PASS | Product cancellation, eight-second live disposal survival, both parts and audibility not established |
| AC05 isolated development | Architecture approval, shared declaration approval, two worktrees, verified integration and checks | Development evidence retained; milestone exact-commit integration absent |
| AC06 review/repair/escalation | Original M2-001 repair and independent closure; current real stop and reviewer-received human packet | Real handoff recorded, no drill; human direction/resumption and milestone completion pending |

No later product rate/volume/mute, responsive/fullscreen, P2/replay, cancellation,
live disposal-survival or dedicated window-close scenario ran. The failure
cleanup must not be presented as all those scenarios passing. Physical
audibility remains not-tested. Windows is optional P2/not-tested. CI has not
run for this changed candidate. M1 acceptance is unchanged.

## Independent findings and retained budgets

Original reviewer `/root/m2_feature_review` independently approved the 239-file
M2-002/003 code and deterministic UI scope in their first review. The current
263-file review consolidates M2-004 round 1 and M2-002/003 native dimensions
round 2; it does not reopen unchanged architecture or grant new repair budgets.
Its decision is **needs-human**. Report SHA256:
`b8685efd2a27bae00dab7345973753c6eef107d7cfe630d43b1e4d33bb94ec55`,
retained at `evidence/m2-product-review-r1/review.md` with independently verified
probe/check logs and their hash index.

| Finding | Tier and evidence | Disposition |
| --- | --- | --- |
| M2-004-REV-001 | BLOCKING class 3, P2: `report.pass` aliases mutable observations; independent scratch regression reproduces historical snapshot corruption | Open; M2-004 owns evidence repair |
| M2-004-REV-002 | BLOCKING class 5, P2: required product smoke fails at seek and later required checks are unexecuted | Open; seek cause/owning implementation boundary still unknown |

The reviewer exercised the same slider coordinates against the actual desktop
and fixture data in a headless probe: exactly one `Seek(5s)` occurred. That
narrows investigation but does not explain the native timeout or prove that
the driver, desktop, application or backend caused it. No speculative repair
or native retry was attempted after the failure.

| Scope | Formal review rounds | Automatic fixes |
| --- | --- | --- |
| Architecture/continuation | 3, approved | 1 |
| M2-001 shared declarations | 2, approved | 1 |
| M2-002 app/fixtures | 2, current native dimensions incomplete | 1 |
| M2-003 desktop/surface | 2, current native dimensions incomplete | 2 |
| M2-004 integration/package/smoke | 1, blocked | **3 of 3, exhausted** |
| M2-005 completed final handoff milestone | Not completed; this report records the R4 stop | No product repairs charged here |
| Runtime prerequisite | 1, approved | 1 |

M2-004 fixes were: preserve startup errors through disposal; correct a slider
test's invalid `Release` call; and replace a driver layout lookup for an
unroled row with observed geometry. The test compile failure counts too. Each
underlying prior failure occurred once; the actual seek timeout occurred once.
No counter resets, fourth architecture review or hidden fourth M2-004 repair.

## Real escalation and next decision

This is **a real blocker handoff, not a drill**. The controller sent
`evidence/m2-human-escalation-r1.md` to the independent reviewer in the current
formal round. The receiver assessed its sufficiency for a human decision and
confirmed that the proposed continuation must distinguish the established
evidence defect from the unknown seek cause. No human approval is inferred.

Recommended next action: the owner grants one bounded seek diagnostic stage
and **one additional M2-004 repair iteration**, preserving the three completed
iterations. Repair the independently established evidence defect with a
discriminating regression; any seek-related change requires observed cause and
the assigned scope first. Then run one newly pinned, bounded native invocation
and use the existing remaining independent review allowance. Retain the 200ms
cancellation bound, eight-second live survival, full UI scope and all failed
evidence. A newly evidenced cause outside M2-004 or further failed repair
requires another disposition; do not silently broaden this proposal.

Alternatively keep the candidate stopped and defer M2. No check waiver, budget
reset, synthetic blocker or drill can supply missing product acceptance.
Commit authority and owner milestone acceptance remain separate later gates;
they are not requested or implied by this repair proposal.

## Evidence and continuation

Run evidence root:
`.agent/runs/20261116T115302Z-a835926ae66c/evidence/`.
Read `m2-product-candidate-r1/`, `m2-004-worker-return/`,
`m2-004-integration.json`, `product-native-r1-assignment.md`,
`product-native-r1/report.json`, `product-native-r1-evidence-index.json`,
`product-native-r1-stop.md`, the product review and human escalation packet.
The original receiver packet is preserved alongside
`m2-human-escalation-clarified.md`; the latter names both findings and keeps
seek diagnosis separate from the confirmed evidence repair. The single finding
inventory continues through immutable `follow-ups-product-r1.md` because its
earlier published entries cannot be edited.
The native report SHA256 is
`bf1cad92ad749c1f86d5e4605729f511bc1f0b2cdb404c39de8b350fbf4ddfdd`.
The run's final checkpoint binds the independent report and publication
manifest. Preserve all earlier worker returns, before/after failures, original
M2-001 closure and frozen contracts.

Observed vs inferred: actual checks, captures, failure and independent alias
reproduction are observed. The seek cause and effectiveness of a proposed
repair remain unknown. Contracts relied on: frozen AC01–06, UI/automation and
the owner-approved [R1–R9 policy](../docs/agent-convergence.md).

Found, not done: no unrelated defect is claimed. The unselected UI reduction
and Windows/CI/audibility/commit limitations remain in the single local
follow-up inventory. Logical remote issues remain **0/5**; no remote authority,
dedupe or publication is claimed. No systemic follow-up warranted by current
evidence. Ignored `.agent/` evidence and packages are local-only, not backed up
by Git or remotely accessible attachments; preserve them before removing this
checkout. Historical CI artifacts expire after 14 days and do not validate
this candidate.
