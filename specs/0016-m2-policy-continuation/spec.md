# M2 fixture desktop preview — approved policy continuation

Spec ID: `0016-m2-policy-continuation`
Status: continuation authorized on 2026-11-16 for two policy changes only:
apply the current R1–R9 review/fix budgets without resetting history, and allow
a clearly labeled human-handoff drill when no real blocker exists. The original
product implementation request was made on 2026-11-15. This continuation needs
independent contract review and a new freeze before use. It does not resume
product implementation during the owner's current M2 reconsideration.

The original `0015` packet and run `20261115T133635Z-984c0955c8cb` remain historical
and unchanged. This continuation retains their task IDs, product scope, shared
interfaces, native checks and worker write paths. The proposal to reduce UI
scope was not approved and is not included. `continuation.md` records approval,
prior findings, cumulative budgets and the limited supersession.

## User scenario and goal

The developer opens a native Apple Silicon macOS application, sees one sample
video's details and two parts, and operates a Bilibili-web-style player using
the accepted mygo/libmpv boundary. The same change demonstrates the M2 workflow:
frozen contracts, two isolated workers, integration, independent review,
bounded repair, real human escalation or a labeled handoff drill, and an
actually running developer package.

## Scope, defaults, and prerequisites

- One original synthetic video detail with cover, title, creator and description;
  two distinct 30-second parts, packaged locally. No network/service requests.
- Go modular monolith, pinned mygo and accepted CoreAudio-cleanup libmpv. Required
  runtime is darwin/arm64; Windows remains optional P2/not-tested.
- A developer `.app` uses one hardcoded verified absolute library path in its
  composition root. No user library picker, CLI requirement or runtime setting.
  Use the verified M2 reconstruction at the primary checkout's
  `.agent/runtime/mpv-0.42.0/m2-rebuild-r1/prefix/lib/libmpv.2.dylib`.
  Rebuild from recovered pinned inputs without modifying historical evidence;
  record fresh artifact hashes and repeat loader/cleanup/capability gates.
  Missing/incompatible runtime is a clear developer-setup failure. There is no
  discovery, download, global install, alternative backend or silent fallback.
- Future user distribution bundles libmpv and its dependencies. This milestone
  bundles fixture resources but still depends on the development runtime.
- No login/search/history, real-service playback, danmaku, quality switching,
  updater, signing/notarization, release credentials or generic agent scheduler.
- Recover and verify the accepted runtime before any native invocation. Missing
  runtime does not prevent pure-Go contract work; it blocks native acceptance.
- Managed worktree creation requires an explicit human instruction under the
  host application's rules. Standing native delegation does not grant it.

## Boundaries and shared contracts

`shared-contract.md`, `ui-contract.md` and `automation.md` are normative inputs
to the architecture review and frozen source packet. The recorder freezes only
spec.md/tasks.json automatically; the controller separately hashes and archives
all these companion inputs and assignments before dispatch.

`internal/catalog` owns plain detail data; `internal/app` owns use cases and the
synchronous source boundary. `internal/fixture` maps packaged assets to those
types. `internal/desktop` owns mygo widgets and view state. Composition owns
runtime validation, windows, menus, resource paths and lifecycle. Business
packages cannot import UI/native/fixture adapters or transport DTOs, directly
or transitively. Keep the existing playback contract and adapter identities.

The shared app and catalog declarations land in M2-001 before independent
workers start. Overlapping write scopes are ordered. Any change to these
contracts returns to architecture review and a new freeze; no worker silently
edits a frozen input or relaxes acceptance.

## Acceptance criteria

### M2-AC-01

**Catalog and portable resources**

Opening the developer package outside the repository displays the sample detail
and two distinguishable original 30-second parts. Part IDs resolve to different
verified media. Assets are located relative to the executable/bundle, never cwd.
Unknown IDs, missing files and malformed resources return sanitized errors.

### M2-AC-02

**Playback and selection ownership**

Startup loads P1 paused. Selecting another part opens it at zero with playback
intent true, preserving chosen volume/rate. Current paused part resumes;
current playing part is a no-op. Clicking the current loading part sets play
intent without reopening; it is a no-op when that intent is already true.
Ended replay reopens the same input at
zero. There is no automatic next-part advance. Loading Play/Pause changes only
generation-bound intent; first readiness applies settings and plays at most
once if still requested. Failure, replacement and close discard obsolete intent.
Failed source resolution leaves prior playback alive and correctly identified.
Selection and playback retries target their distinct identities.

### M2-AC-03

**Player interface**

The UI satisfies ui-contract.md: details and responsive parts list, overlay
transport, seek, six speed presets, volume/mute, fullscreen, pointer/keyboard
behavior, focus and accessible controls. Controls expose functional actions;
native snapshot sampling does not echo seek/volume commands. Fullscreen and
resize preserve the session and its playback state.

### M2-AC-04

**Native lifecycle and developer package**

An actual darwin/arm64 developer `.app` launches outside the repo and plays both
parts with visible picture and audible output. Loading/playing cancellation is
externally observed at at most 200ms. Close joins complete disposal, leaves zero
ownership and survives eight seconds afterward in a still-live test process.
A separate actual window-close scenario verifies graceful process exit. Preserve the
accepted 50ms poll and 10-second readiness timeout. Rendering remains 640x360
scaled with letterboxing; no increased output resolution is claimed.

### M2-AC-05

**Contracts and isolated development**

The actual spec/DAG/companion contracts receive independent architecture review
before freeze. M2-001 supplies compilable shared declarations and executable
business-import checks. At least two authorized workers develop M2-002/003 in
separate worktrees against identical frozen inputs, with nonoverlapping writes.
The controller records exact candidate identities, integration and real checks.

### M2-AC-06

**Independent review and bounded escalation**

Independent reviewers produce candidate-bound findings under
`docs/agent-convergence.md` R1–R9. At most three formal review rounds include
the initial review; at most three automatic fix iterations include unsuccessful
attempts; recurrence of the same failure three times also stops automatic work.
The first reached limit applies. All dimensions share their task's budget.
Historical counts and dispositions carry forward through runs, agents and
candidates as recorded in `continuation.md`; there is no automatic fourth review.

M2 still needs evidence of an actual bounded repair. The recorded M2-001 repair
and independent closure can supply that workflow evidence; it does not prove
new code or product acceptance. Do not invent findings or extra repair cycles.

When a real BLOCKING finding or other required human decision exists, follow
R4's actual escalation, stop the affected work, record `needs-human`, and obtain
explicit direction and any needed additional allowance before resuming. A drill
cannot close that finding or authorize resumption. When there is no real blocker,
a clearly labeled drill may satisfy the human-handoff-path evidence only:

- Mark the packet, scenario and outcome `DRILL — simulated, no real blocker`.
  Use separate `DRILL-*` identifiers; keep them out of real finding/budget counts.
- Include goal/task/spec/candidate, hypothetical open issue, assumed prior rounds,
  simulated attempts/evidence, competing options, recommendation and the exact
  decision requested. Mark invented inputs as synthetic; actual checks keep
  their real provenance.
- Exercise the handoff by sending that packet to an independent receiving
  reviewer. The receiver checks whether it is sufficient for a human decision
  and confirms the simulated stop/resume boundary. Record the actual exchange
  and its limitations, not just an unexecuted template. This is part of the
  assigned task's existing formal review round, not an extra uncounted review.
- Do not claim a human made a decision, fabricate approval, change real task
  status to blocked, or execute a simulated permission. Any example response
  is explicitly synthetic. A real blocker found during the drill uses the real
  inventory and R4 counts. The drill never satisfies missing product/native
  checks or independent technical closure.

Missing required real repair evidence or an executed real/drill handoff leaves
this criterion pending. Planning discussions alone are not a completed drill.

## Verification matrix

| AC | Checks | Fixture/platform and evidence |
| --- | --- | --- |
| 01 | fixture hash/size/duration/ID tests; missing and invalid resource tests; packaged launch from a temporary cwd | Original local assets; portable Go tests and actual macOS package |
| 02 | app fake source/engine/clock tests: initial pause, switch while loading/playing, stale readiness, loading play then pause, settings, replay, separate retries, timeout, idempotent close | Headless deterministic tests; actual part switch/replay on macOS |
| 03 | desktop headless interaction tests and native UI review against ui-contract.md | Synthetic media; keyboard/pointer/layout/fullscreen evidence |
| 04 | product native smoke plus existing 41-assertion libmpv fixture suite; loading/playing close and delayed-survival checks | Exact candidate, runtime manifest/dependency hashes and actual darwin/arm64 |
| 05 | spec validation; `go test -count=1 ./internal/architecture`; worker return packets and integration identity; `go run ./cmd/dev check --race` | Exact source manifests, role and worktree identities, full local check |
| 06 | independent review packets, actual repair/closure, cumulative ledger, real escalation exchange or executed labeled drill/receiver response | Actual events distinguished from synthetic scenario; no invented blocker, human approval or product PASS |

## Risks, rollback and open decisions

Native runtime availability and explicit managed-worktree authorization are
execution prerequisites, not alternative product designs. Product scope is retained here; any later product-scope decision requires
its own reviewed continuation. Package/resource failures must not weaken native cleanup or existing
checks. Keep the laboratory launchers functional; the new app has its own entry
point. Reverting this scoped addition restores the preceding experiment without
rewriting historical evidence. Do not claim CI, packaging or platform checks
that were not actually run against the candidate.

## Review and evidence

Controller `/root` owns the current policy-continuation preparation, freeze and
records. Historical architecture and M2-001 approvals retain their exact
candidate identities. The owner explicitly approved these two policy changes
on 2026-11-16; independent review assesses this continuation's fidelity and
consistency, not authority to change the owner's decisions. Review/fix counts
and original finding closure are retained in `continuation.md`.

This policy update does not authorize commits, push, remote changes, managed
worktree creation or renewed implementation dispatch. The existing product plan
still requires two authorized worktrees and eventual exact-commit integration;
those gates are not waived. The eventual product handoff remains
`reports/0028-m2-fixture-preview.md`, owned by M2-005 after its dependencies.
Milestone acceptance is separate.
