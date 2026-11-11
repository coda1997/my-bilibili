# Provisional mygo playback preview

Spec ID: `0005-provisional-playback`
Status: selected by the owner on 2026-11-11; proposed contract awaiting
independent architecture review before freeze and implementation.

## User scenario and goal

The owner chooses to continue using the pinned mygo player temporarily, record
its known limitations, and consider replacing that component later. Give the
existing macOS fixture playback window useful transport controls without
pretending the blocked M1 media contract has passed.

This is a bounded continuation of the playback experiment, not the full desktop
shell, an M2/P0 start, or final approval of the mygo playback stack. Preserve the
old M1 specification, failures, reports and closed runs unchanged. The owner's
temporary-use decision does not waive Windows, target access, headers, separate
tracks, responsive loading or native failure evidence for final M1/P0 acceptance.

## Scope and non-goals

- Record a provisional-use ADR and issue register with stable issue IDs,
  reproduction, impact, evidence, current workaround limits and replacement
  acceptance. mygo remains behind the existing plain player boundary.
- Extend the existing native experiment UI with a sampled position/duration,
  seek slider, continuous volume slider, speed presets and retry of the last
  valid input. Preserve existing source/command keys and diagnostics.
- Keep the existing strict native smoke as the default. Add an explicitly
  named preview suite covering only this increment, with its suite name in
  results. A passing preview must not be reported as a passing full/M1 smoke.
- No dependency/pin changes, upstream patch, unsafe native access, hidden-host
  refresh, background native call, alternative engine, media proxy/remux,
  application JavaScript or extra installation. No live service/account input,
  file picker, full shell, login, danmaku, quality selection or fullscreen here.
- Keep generated local/loopback fixtures and their limits. Windows, actual
  audible output/speed/A-V sync, real IME, packaging/signing and candidate CI
  remain not-tested unless independently exercised, never inferred from build.

## Boundaries and shared contracts

`experiments/playback/contract` stays framework-free and unchanged. Its Input,
Command, Snapshot, Engine, Opener, controller generation, validation, safe errors,
10s ready deadline, 50ms polling and cancellation/disposal rules are frozen
read-only inputs. `player` and module files are also unchanged. Native public
calls remain on the UI/main thread; no new worker or subscription is introduced.

The UI reads sampled snapshots, never publishes a command echo as native
position. Slider changes issue the existing validated Seek/Volume commands;
frame rebuilds and sampled updates must not replay commands. Seek uses seconds
in the UI, clamps to a finite known positive duration, and is unavailable while
Loading/Idle/Failed/Closed or no owned engine. Volume/speed controls are available
only for an owned non-Loading session; all disabled controls emit no commands.
Speed presets are 0.5x, 1x and 2x, within the existing allowed range. Existing
diagnostic buttons retain their behavior and are not the new controls' model.

Retry stores a defensive copy of the last validated input, including synthetic
metadata. Invalid input cannot replace that saved input or cancel an active
session. A valid but Unsupported input remains unchanged on retry; never strip
headers/audio to make it work. Retry is disabled before any valid input and
during Loading. A retry is a normal replacement: close/cancel old engine, bump
generation, clear prior view control error, no automatic retry loop. Close does
not erase the saved input. Fake/headless tests demonstrate recovery from an
opening failure; native preview demonstrates a new generation after explicit
close/retry, not real transient-network recovery.

The known delay bug can still stall the UI before it can process a tick or
cancel; the banner/documentation must say so. No main-thread timer can repair
that stalled native call. Do not retain the rejected positive-duration surface
gate or map Timeout to Media. Maintain safe errors and sanitize snapshots/logs:
no raw path, URL, native body or account information in status text.

The native view must display a visible temporary-player warning identifying
the delayed-loading and unsupported-header/separate-track limitations. Keep
controls usable in the existing desktop window (scroll content if necessary),
with accessible labels and no new full-client navigation. Preserve the existing
app title and test-only automation build tag. Capture only the owned synthetic
app window, never the desktop.

## Acceptance criteria

### PREV-AC-01

The ADR records the owner's temporary-use choice, distinct from final M1
acceptance. The issue register names the synchronous loading stall, render-coupled
error cache, unsupported headers and unsupported separate audio, with original
run/source evidence and explicit replacement exit criteria. No issue is marked
fixed merely because preview development continues.

### PREV-AC-02

Given a controlled sampled Ready/Paused/Playing session, position/duration are
shown, slider interaction emits bounded Seek, volume interaction emits Volume,
and speed presets emit Rate. Rebuilds/sample changes emit no commands. Inactive,
Loading and invalid-duration seek cannot issue a seek. Headless UI tests cover
these states and accessible disabled controls.

### PREV-AC-03

Given a valid input that failed, closed or ended, Retry reopens the same copied
input with a newer generation and clears prior control error. Invalid input
does not overwrite it; retries never drop unsupported metadata/audio. No saved
input or a Loading session disables retry. Headless tests verify old ownership
is disposed and a fake failure can recover without a leaked prior engine.

### PREV-AC-04

The explicit `preview` native suite verifies the warning, synthetic rendered
local picture, native progress/pause, slider seek to4s within500ms/2s, volume
configuration, speed preset readback, and close/retry with a newer generation.
It closes the owned app with an active session and verifies bounded process,
socket and server cleanup. Existing 10s ready/2s progress/100ms cancel and other
strict suite bounds remain unchanged. The full suite remains default and its
known delayed-loading failure is run and recorded, never converted to success.
Missing exposure yields not-tested/nonzero, not fake playback acceptance.

### PREV-AC-05

Full repository checks/race and independent exact-candidate code/native-evidence
review cover the new UI/retry/suite distinction. The durable handoff maps each
claim to its source/suite, labels known failures and not-tested platforms, and
leaves M1/M2/P0 acceptance pending. No commit/push/publication or workflow/check
policy change is inferred from this user instruction or preview's success.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| PREV-AC-01 | Independent ADR/issue review against prior strict/native evidence | Local documents | Exact candidate, issue IDs, prior run/source locators |
| PREV-AC-02 | Headless slider/commands/state/accessibility tests | Fakes, no native decoder | Go test output, sampled-change/no-feedback cases |
| PREV-AC-03 | Headless retry/invalid/unsupported/copy/cleanup tests | Controlled opener/engines | Generation, owned handles, unchanged metadata assertions |
| PREV-AC-04 | Explicit preview suite plus unchanged-default full suite | Actual macOS, generated fixtures | Suite-labelled JSON, app hash, app-only captures, cleanup |
| PREV-AC-05 | Full recorded check --race; independent review | macOS; Windows not-tested | Frozen inputs, complete candidate archive/manifest, review/handoff |

## Risks, rollback and decisions

The provisional player may freeze on slow loading; prominently disclose this,
do not expose a false cancellation guarantee, and preserve the stricter M1
failure. Restrict this increment to the controlled fixture preview. Later
replacement must satisfy the unchanged plain contract and required target
headers/separate streams, native errors, responsiveness, seek and cleanup on
both OSes. Selection/implementation of that replacement remains future work.

Rolling back this increment restores only its owned UI/driver/docs changes,
never preexisting user work or closed evidence. Single-module Go boundaries
remain; do not extract speculative product modules or create a generic player
framework. No existing workflow/check/CI policy is changed or bypassed.

## Review and evidence

Use the owner's previously authorized independent architecture/code contexts;
the author cannot approve this change. Freeze this new spec/task graph only
after review, create a new run, archive all tracked/nonignored untracked source
with hashes/modes before exact-candidate review. Max3 repair rounds per protocol.
Keep raw sanitized evidence in the controller-owned run and the durable result
in `reports/0005-provisional-playback.md`. Ignored local evidence is not a backup.
