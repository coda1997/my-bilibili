# Default libmpv playback

Spec ID: `0011-default-libmpv`
Status: owner requested on 2026-11-13; independent architecture review required
before implementation and freeze.

## User scenario and goal

After the reviewed M1 adapter recommendation, the owner requested "ok set it tp
default" in response to the explanation that libmpv was available only through
the replacement/target commands. Make the ordinary `cmd/native` playback lab
use libmpv, retaining the existing mygo UI and plain playback boundary.

## Scope and non-goals

Switch the launcher composition to the already reviewed `mpvprobe.Opener` and
`mpvprobe.Draw`. Preserve the previous launcher verbatim as `cmd/mygo-native`
with its opt-in e2e import for explicit legacy diagnostics. The existing
replacement and target commands, adapter implementations, runtime/dependencies,
fixtures, input policy and playback contracts remain unchanged.

The default launcher accepts the existing `-fixtures` and optional `-http-base`
flags plus required `-library`, an absolute path to the provenance-verified
libmpv API 2.6 library. No automatic discovery/download, runtime installation or
fallback is introduced. Without a server only the local fixture is shown; with
a valid loopback base use the replacement launcher's existing fixture inputs,
including required headers and separate tracks. Missing/relative library paths,
nonregular/missing library files and invalid fixture bases fail before creating
a window. Loading a valid-path incompatible library retains the adapter's typed
Unsupported behavior. Runtime provenance remains an explicit prerequisite.

The default UI names libmpv and does not display the old mygo limitation banner.
It retains resource/track diagnostics and the replacement launcher's graceful
window-close/zero-resource check. The replacement smoke accepts the fixed
default libmpv window title in addition to its existing fixed title, without
changing any assertion, deadline, stage default or acceptance criterion.

Update current documentation, command examples and a dated ADR. Preserve dated
historical reports and frozen inputs. Record owner selection of the default
adapter separately from formal M1 milestone acceptance, which remains pending.
No M2/P0/full shell, target request/retry, workflow/CI policy change, remote
action, commit or release is included.

## Boundaries and shared contracts

Only launcher composition and its offline setup validation change. All engine
state transitions, copied inputs, threading, timeouts, cancellation, generation
handling and resource disposal remain those of the reviewed plain contract and
libmpv adapter. No UI/native types enter business modules. Existing automated
capability checks run unchanged on the newly built default executable, with
the title match extended only for that named launcher. No external service
requests or account data are permitted in this task's verification.

## Acceptance criteria

### DMP-AC-01

The default native command composes libmpv and the mygo UI, supports local-only
launch and optional controlled HTTP fixtures, advertises libmpv, rejects invalid
startup inputs, and has no implicit fallback to the legacy player.

### DMP-AC-02

The exact rebuilt default e2e executable passes the existing macOS fixture
capability stage, including visible picture, controls, required metadata,
separate audio, responsive loading/playing cancellation within 200ms and graceful
cleanup. Keep the first outcome and any failure; do not retry an unexplained
native failure or relax the oracle. The explicit old mygo launcher preserves
the baseline source and builds; its known failures remain documented.

### DMP-AC-03

Current docs identify libmpv as the selected default, show the required runtime
and correct smoke/legacy commands, and retain the previous evidence with honest
milestone/platform/packaging limits. Independent review is bound to the exact
candidate and actual checks, and a durable handoff routes future work.

## Verification matrix

| AC | Verification | Environment and evidence |
| --- | --- | --- |
| DMP-AC-01 | Offline launcher setup tests; inspect composition and flags; manual-local configuration branch tested without native load | Go race tests, synthetic paths/loopback input, no GUI/network |
| DMP-AC-02 | Build default normal/e2e and legacy e2e; compare legacy files byte-for-byte; one `replacement-smoke -stage capabilities` with default app and reviewed runtime | macOS arm64, unchanged generated fixtures and runtime manifest, exact app/source hashes, result and cleanup |
| DMP-AC-03 | `go run ./cmd/dev check --race`; scoped docs/link audit; independent candidate/code/evidence review | Full local checks, source manifest/archive/diff, review and report |

## Risks, rollback, and open decisions

The verified runtime is ignored and absent from fresh clones; document the
explicit prerequisite and library argument. Native execution requires an exposed
interactive macOS desktop. Windows remains optional P2/not-tested. The previous
player's known issues are preserved, not repaired by changing the default.
Rollback is an explicit choice of `mygo-native`; no automatic fallback hides
libmpv failures. Formal M1 acceptance and future product work remain owner
decisions; neither is needed to implement this explicitly requested switch.

## Review and evidence

Controller/planner/implementer: `/root`; separate reviewer per `assignment.md`.
Review architecture before freezing a new run, then exact code and native
evidence. The new fixture invocation is authorized as verification of this
request; it does not reuse any earlier consumed target/native allowance.
Controller owns `.agent/runs/`; workers return evidence from isolated scratch.
Durable handoff: `reports/0024-default-libmpv.md`. Base HEAD is
`d846e667c537c06a1bcf100e6aac9486c29aa3d2`, not the dirty candidate identity.
