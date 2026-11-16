# M2 fixture desktop preview and bounded development loop

Spec ID: `0015-m2-fixture-preview`
Status: owner-selected plan; implementation requested on 2026-11-15. Serialized
contract review and freeze must precede implementation. Current acceptance is
recorded in the run and handoff, never inferred from this status.

## User scenario and goal

The developer opens a native Apple Silicon macOS application, sees one sample
video's details and two parts, and operates a Bilibili-web-style player using
the accepted mygo/libmpv boundary. The same change demonstrates the M2 workflow:
frozen contracts, two isolated workers, integration, independent review,
bounded repair, human escalation, and an actually running developer package.

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

Independent reviewers produce candidate-bound findings. The controller follows
the canonical three-round policy in docs/workflow.md, including early escalation,
no automatic fourth round and no counter reset through reassignment. M2 requires
actual bounded repair and human-escalation evidence; planning exchanges or
invented findings cannot satisfy it. Missing evidence leaves M2 pending.

## Verification matrix

| AC | Checks | Fixture/platform and evidence |
| --- | --- | --- |
| 01 | fixture hash/size/duration/ID tests; missing and invalid resource tests; packaged launch from a temporary cwd | Original local assets; portable Go tests and actual macOS package |
| 02 | app fake source/engine/clock tests: initial pause, switch while loading/playing, stale readiness, loading play then pause, settings, replay, separate retries, timeout, idempotent close | Headless deterministic tests; actual part switch/replay on macOS |
| 03 | desktop headless interaction tests and native UI review against ui-contract.md | Synthetic media; keyboard/pointer/layout/fullscreen evidence |
| 04 | product native smoke plus existing 41-assertion libmpv fixture suite; loading/playing close and delayed-survival checks | Exact candidate, runtime manifest/dependency hashes and actual darwin/arm64 |
| 05 | spec validation; `go test -count=1 ./internal/architecture`; worker return packets and integration identity; `go run ./cmd/dev check --race` | Exact source manifests, role and worktree identities, full local check |
| 06 | independent review packets, original findings, round ledger, human decision packet and response | Actual task events; no fabricated repair/escalation PASS |

## Risks, rollback and open decisions

Native runtime availability and explicit managed-worktree authorization are
execution prerequisites, not alternative product designs. No product choice is
left open. Package/resource failures must not weaken native cleanup or existing
checks. Keep the laboratory launchers functional; the new app has its own entry
point. Reverting this scoped addition restores the preceding experiment without
rewriting historical evidence. Do not claim CI, packaging or platform checks
that were not actually run against the candidate.

## Review and evidence

Controller `/root` owns policy preparation, freezing, M2-001, integration and
run records. Independent architecture reviewer `/root/m2_plan_review` reviewed
the inline direction only; the serialized files need their own exact review.
The owner explicitly accepted the three-round clarification and requested
implementation. No commit, push, remote change or managed worktree creation is
authorized by that request alone. Durable continuation goes to
`reports/0028-m2-fixture-preview.md`; milestone acceptance remains separate.
