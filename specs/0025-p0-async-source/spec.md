# Responsive asynchronous video loading

Spec ID: `0025-p0-async-source`
Status: proposed for independent architecture review, 2026-11-17. Full lane.
Baseline: `2c0fd1656810e5556b7ac7e885b3043d23ea5d39`, including merged PR #15.
The owner accepted preparing this bounded plan, resumed P0 verification and
then explicitly requested completing P0. Scoped implementation may proceed
after independent architecture review and contract freeze; this proposal is
not itself a frozen contract or technical approval.

## User scenario and goal

Opening a video or selecting another part must leave the window responsive
while details or a playback address load. Rapid selection, pause, cancellation
and closing must retain the user's latest intent. A late response cannot open
an obsolete part, overwrite current metadata or revive a closed window.

Observed baseline: app.New calls Source.Detail before returning; selection,
retry and expired-input refresh call Source.Resolve synchronously. Source's
documented contract prohibits wiring in network I/O until this changes.
The existing playback Controller is UI-thread confined and already owns native
open/control/close. This slice changes source loading, not that ownership.

## Scope and non-goals

Move both Detail and Resolve off the UI thread, expose their loading/error/retry
states, adapt the fixture composition and its controlled e2e scenarios, and
verify cancellation, expiry and resource ownership. Use controlled Source
implementations and existing local media only.

No live Bilibili adapter, login/Keychain change, network media policy change,
danmaku implementation, search/navigation, history, new dependency, native
adapter redesign, check/CI policy change or public release. The separately
reviewed real-login flow can continue using its unchanged executable. Full P0
still requires real login, native Chinese overlays and complete integration.

## Boundaries and shared contracts

### Application and source interface

- Source retains Detail(context.Context) (catalog.Video, error) and
  Resolve(context.Context, catalog.PartID) (Resolution, error). A Source is
  bound to one video's identity. Cross-video navigation is a later slice.
- New(source Source, controller *playback.Controller) (*Session, error) validates
  dependencies and performs no Source call. Start(ctx) starts asynchronous
  detail loading; its returned error describes submission/precondition failure,
  not the eventual Source result. Start is idempotent and cannot restart a
  closed Session. Its context owns subsequent source work and playback.
- Add DetailPhase with Initial, Loading, Ready and Failed states (distinctly
  named DetailInitial, DetailLoading, DetailReady, DetailFailed). State exposes
  DetailPhase and DetailError, plus ResolvingPart (empty when no resolution is
  pending), PlaybackRetryError and Closing. Existing SelectedPart, PlaybackPart, SelectionError, Playback,
  DesiredPlaying, Volume and Rate retain their meanings, clarified below.
- Player gains RetryDetail() error. After a successful Detail, validate the
  existing catalog invariants at their current owning boundary, publish an
  immutable copy, select its first part, and resolve it with initial play intent
  false, preserving the existing explicit-Play fixture experience.
- Detail failure leaves no invented metadata or playable selection. RetryDetail
  is explicit and available only for a failed detail attempt while the session
  context remains live. Part actions before detail readiness return Invalid.
- Session exposes Done() <-chan struct{} to composition. It signals that owned
  source work has stopped after Close or parent cancellation. Composition still
  calls Close on the UI thread to dispose native playback. Closing is true
  after Close is requested while source shutdown is pending. No UI, HTTP
  DTO or native handle crosses into app/catalog business state.

### Bounded worker, deadlines and result acceptance

- One source worker per started Session; at most one Source call in flight,
  one latest pending request and one completed result. Superseding a request
  cancels it and replaces the pending request, never starts an unbounded set
  of goroutines or lets queue size grow with clicks.
- Every Detail/Resolve attempt has a ten-second deadline starting at request
  submission, bounded further by the session context. Explicit retry starts a
  new attempt; there is no automatic retry loop. Source implementations must
  honor cancellation promptly and return ownership of their work. A canceled
  or expired queued request is never dispatched as useful work.
- Tick also retires a logically expired attempt and exposes Timeout even if
  its canceled worker has not returned yet. Keep ownership of that worker and
  never dispatch another concurrent call. The single queued retry has its own
  deadline. Use a request-deadline seam independent of the existing media-expiry
  test clock; epoch-based fixture clock values must not become real deadlines.
- Source implementations transfer result ownership on return; they do not
  mutate returned slices/maps afterward. Copy mutable data before acceptance
  or forwarding, including Video.Parts and Input.Headers. State returns copies
  and never contains playback locations or request headers.
- Workers return plain results. Only Tick on the UI thread may publish them,
  mutate Session or invoke Controller methods. All public Session/Player calls
  except receiving from Done remain on the UI thread.
- Use a monotonically increasing source-request identity separate from the
  native playback generation. Accept only the current request, its intended
  part/purpose, a live session and an unexpired request context. Check these
  conditions again before Open. Superseded success and failure are discarded.
- Treat context deadline/cancellation as safe Timeout/Canceled categories for
  the owning current attempt. Preserve the existing safe mapping for other
  source errors. A superseded cancellation must not replace the new request's
  state with an error. Parent cancellation retires all pending source work and
  prevents subsequent native opens.
- Validate Resolution.Input and ExpiresAt when accepting the result on the UI
  thread, using the existing thirty-second ResolutionMargin. A result which
  expires while waiting for acceptance is unusable, even if valid at return.
  Non-expiring fixture inputs remain reusable. No stale-address fallback.

### Selection, controls and retry semantics

- Selecting B sets SelectedPart and ResolvingPart to B immediately. Existing
  accepted playback A remains owned and keeps its current transport state
  until B has a valid accepted resolution; a failed resolution does not close A.
  PlaybackPart changes only when Controller accepts a new playback attempt.
- Re-selecting the same pending target coalesces the request and updates its
  intended autoplay without another call. Invalid IDs leave valid work intact.
  Selecting accepted A while B resolves invalidates B even when A takes a
  cached/resume/no-op path which does not advance native playback generation.
- Initial resolution requests paused playback; explicit part selection and
  replay use the same initial intent as today. A pending resolution has its
  own intended play state, separate from the accepted engine's intent.
- Play/Pause/Toggle during pending resolution update that pending intent and,
  when an accepted engine exists, its transport intent. Result delivery never
  overwrites newer explicit user intent. DesiredPlaying describes the pending
  target's intent while it is resolving, otherwise the accepted player's intent.
- A terminal native event from accepted A must not clear pending B's intent.
  Play/Pause while B resolves does not start a competing replay of ended/failed
  A. Conflicting RetrySelection/RetryPlayback while another target resolves is
  disabled and returns Invalid without mutation; a new SelectPart may supersede
  any pending resolution.
- Volume and rate changes during a resolvable pending selection are retained
  and applied to the accepted new attempt through the existing initialization
  path. Do not issue native commands to an absent engine. Seek continues to act
  only on an accepted seekable engine, never an unresolved target.
- Resolution failure for selected B sets its SelectionError and explicit
  RetrySelection target, retaining A and A's latest transport intent. A new
  selection retires that selected-part failure and starts the new request.
- Retrying or replaying accepted A after its input expires resolves A again.
  If a different B is selected with a failure, keep B's selection/error/retry
  identity unchanged. This existing separation must survive the async change.
  Resolve/open failure for A is exposed through PlaybackRetryError, not
  attributed to B. Explicit retry or a new accepted playback attempt clears
  that error. The desktop offers RetryPlayback for that safe error state.
  If accepted A is also selected, retain the existing selection-error/retry
  semantics for its failed refresh instead of assigning the separate error.

### Closing and native composition

- Close is idempotent: retire request identities, cancel source ownership and
  close the playback Controller on the UI thread. No wait for network/source
  completion is performed on that thread. Close before Start completes Done
  without launching a worker. Source results after Close cannot affect state
  or call Open.
- The composition waits for Done off the UI thread before allowing window
  disposal/application exit. Keep the closing state visible and prevent new
  actions. Dispatch completion through Window.Update, independently of paint
  callbacks; no minimized/idle-window paint dependency. The observer has one
  owner and terminates after completion.
- A source which ignores cancellation violates the port. Do not silently drop
  ownership or report full cleanup while it remains active. All implementations
  used in this slice must prove prompt cancellation.
- Preserve existing native cleanup and fullscreen protocols and required
  deadlines. No libmpv, Controller teardown or media-host policy changes.
- The desktop presents distinct detail loading, detail failure/retry and part
  resolving states. Existing accepted playback remains controllable while a
  different part resolves. Normal builds gain no automation endpoint; new
  source-delay/failure controls exist only in the synthetic e2e composition.
- The existing fixture Source must cooperate with cancellation while validating
  its files, with context checks around bounded reads and owned file cleanup.
  Preserve integrity checks, fixed fixture IDs and zero expiry. No claim is made
  that Go can interrupt an arbitrary stalled operating-system file syscall.
- Extend the strict synthetic observation validator only for consistent
  pre-detail, pre-first-open, detail-failure and early-close states. Return an
  empty nonnil Parts slice before detail. Keep missing/null/duplicate/unknown
  fields, contradictory IDs/generations and invalid safe errors rejected. An
  early Idle observation is valid pending evidence, never playback readiness;
  retain actual track/resource requirements in the native smoke driver.
- Tick is an explicit UI-thread event pump which can accept a completed source
  result and Open once. State, construction and layout rebuilding alone remain
  side-effect free. Update the old Render comment rather than claiming the new
  event pump cannot open media.

## Acceptance criteria

### ASY-AC-01

Construction does no I/O. With Detail held by a controlled source, Start returns,
the UI shows loading and remains responsive. Success publishes validated copied
metadata and resolves the first part; failure is visible and explicitly retryable.

### ASY-AC-02

Rapid A/B/C selection uses bounded work; only the latest requested result may
open. Canceled, timed-out, queued and out-of-order results cannot overwrite the
current state or open after Close. Existing playback survives resolution failure.

### ASY-AC-03

Delayed resolution preserves newer Play/Pause/Toggle, volume and rate intent.
Selected and accepted part identities remain distinct. Retrying expired accepted
playback does not clear another selected part's failure or retry target.

### ASY-AC-04

Resolution expiry is checked at acceptance and reuse, including the margin
boundary. Request deadlines include queue wait. Close cancels pending/in-flight
work; Done establishes worker/observer cleanup without UI-thread source waits
or paint-dependent completion. No source result exposes URL/header/error text.

### ASY-AC-05

The normal fixture app and its explicit e2e composition adopt the new interface.
Desktop loading/retry controls and synthetic delayed-source native actions work
on macOS, including closing while detail/resolve is pending and while minimized.
Existing native controls/fullscreen/cleanup smoke continues to pass.

### ASY-AC-06

The exact implementation commit passes full check --race, relevant normal/e2e
offline regressions and independent review. Required native evidence uses rebuilt
matching artifacts. Report source, checks and review in an evidence table; keep
Windows native not-tested/P2 and retain real service/P0 gaps.

## Verification matrix

| AC | Verification | Fixture and OS | Required evidence |
| --- | --- | --- | --- |
| ASY-AC-01 | Latch-controlled Detail, invalid metadata, ownership copy and retry tests; view tester | Offline Go/macOS UI tester | Constructor/start return and actual loading/error/ready transitions |
| ASY-AC-02 | Controlled completion/cancellation order, burst selection, finite concurrent-call/queue counts | Offline race tests | Latest-only Open, preserved accepted engine, canceled calls terminate |
| ASY-AC-03 | Pending-intent and existing selected-vs-playing retry regressions | Fake source/player/clock | Exact command/part sequence after delayed completion |
| ASY-AC-04 | Controlled deadline/clock, close before/during work, result acceptance after expiry, hostile error sentinel | Offline race tests | Timeout, stale rejection, Done/observer cleanup and sanitized state |
| ASY-AC-05 | Normal/e2e compile and native fixture suite plus explicit delayed Detail/Resolve scenarios | Rebuilt macOS app and driver; synthetic media | Responsive controls, loading/retry UI, minimized close, existing cleanup/fullscreen assertions |
| ASY-AC-06 | go run ./cmd/dev check --race; independent architecture/code review | Exact commit; local Go1.27.3 | Candidate-bound check/review/native table and STATUS update |

Regressions for new generation, cancellation, deadline and intent mechanisms
must discriminate removal of those mechanisms. Use controlled latches and clocks,
not success assertions based solely on short sleeps. Default tests stay offline.

## Risks, rollback, and open decisions

Changing app.New and Player affects known fixture callers/tests and both normal
and e2e entrypoints; update them together. Async source mocks must stop mutating
shared maps without synchronization. This is a new bounded contract, not an
attempt to repeat or reset prior login review/repair history.

Before freeze, architecture review must verify the proposed playback-retry error
state and cancellation/Done composition against current lifecycle requirements.
No product implementation begins before that contract review and freeze.
Rollback reverts only this slice, preserving real Keychain credentials/history
and earlier evidence. No remote action or policy change is part of this proposal.

## Review and evidence

Use independent architecture and exact-candidate code/native-lifetime review.
Planner/researcher/implementers cannot approve their own candidate. R1-R9 apply:
at most three formal review rounds and three repair iterations for this new
task, with no reset of existing findings. New remote issue budget used 0; no
issue filing authorized. Keep findings in .agent/p0-async-source/inventory.md.
The controller owns frozen runs; workers return evidence outside those runs.
Durable implementation handoff: reports/0037-p0-async-source.md plus STATUS.md.
