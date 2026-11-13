# Playing-close shutdown repair

Date: 2026-11-11. Spec: [0006-player-replacement-probe](../specs/0006-player-replacement-probe/spec.md).
Controller run: `20261111T153610Z-04f30db12855`.
Owner request: “sure, do the repair,” following the
[playing-close diagnosis](0009-playing-close-diagnosis.md).

The Go adapter now keeps its render owner active until native video-output
teardown acknowledges completion. Three native playing Close measurements are
**82.791167 / 61.301208 / 61.393792ms**, within the unchanged **200ms** bound.
The expanded six-scenario native cleanup suite passes, including playing context
cancellation, complete per-session unload and eight-second survival. The full
local race check also passes.

These are author verification results. **CA-CLEANUP-002 remains open pending
independent verification**; this implementation does not close the reviewer’s
finding. The full capabilities invocation still fails at separate-track readiness
and requires forced app cleanup. RPL-002, RPL-003 and M1 remain unaccepted.

## Change and shutdown ordering

The [diagnosis](0009-playing-close-diagnosis.md) localized the old overrun to a
pending frame: render-context free waited on a native video thread that could
only escape through its 200ms timeout after the adapter stopped rendering.

In [`engine.go`](../experiments/playback/mpvprobe/engine.go) and
[`shutdown.go`](../experiments/playback/mpvprobe/shutdown.go), shutdown closes
command admission and sends `set vid no` on the control owner while the render
owner continues servicing frames. `force-window=no` is explicitly pinned at
initialization. The matching command reply, using reserved request ID 0, forms
the video-output teardown barrier. Unrelated playback replies and late state
events cannot satisfy it or revive the closing session. Shutdown then sends
quit, stops and frees the renderer, destroys the core, joins workers and unloads
the session library before Close returns.

This uses the existing public async command and render APIs. In the verified
pinned mpv source, setting `vid` updates track selection through
`player/command.c`’s property/option callbacks. With active video,
`mp_switch_track_n` in `player/loadfile.c` uninitializes the video chain and calls
`handle_force_window`; with forced output disabled, this destroys and joins the
video-output worker. The ordinary non-coalesced command reply follows that
work. During loading, disabling video prevents subsequent video-track selection.
The control owner waits independently of the render owner, which can consume
the frame needed for video destruction to finish.

[`surface.go`](../experiments/playback/mpvprobe/surface.go) also preserves render
servicing after a target-render failure: it reports the failure and uses public
`MPV_RENDER_PARAM_SKIP_RENDERING` without target pixels or target-time blocking
until control-side teardown stops the worker. Rendering remains serialized on
its original owner, with the existing 30Hz limit and bounded frame buffers.

The reply wait is bounded to two existing 50ms poll intervals and at most 256
nonempty events, leaving time for remaining destruction. Rejection, missing or
failed reply, event loss, shutdown event and quit rejection record failure while
still performing full synchronous disposal. This timeout does not return an
early Closed state or turn a failed native shutdown into a timing pass; the
native regression additionally rejects any shutdown fallback. Native destructor
blocking remains a platform risk, rather than a hard real-time guarantee.

The runtime, CoreAudio patch, dependency pins, plain contract, UI, smoke driver,
fixtures, shared spec/tasks and 200ms oracle are unchanged. No native rebuild,
deadline relaxation, background disposal or process-lifetime library retention
was introduced.

## Exact candidate and evidence

Base commit: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, with the prior review
and diagnosis documentation preserved. All 118 baseline files matched the
previous handoff before implementation. This repair remains uncommitted.

The exact tested 120-file candidate is `evidence/candidate-r1/` in this run:

| Artifact | SHA-256 |
| --- | --- |
| Tested full source manifest | `b640cb7339850da8d2e2742cdda2434235ab2df6268cca43f762d5efd4d1760a` |
| Tested full source archive | `d270fa3235380db4f7e3f6e1f9ea5b2345ce9eb900b7ca7d531013d08a6def6d` |
| Native cleanup test binary | `466be9452699aa08539233dfc5c07cc512e8612dc6a6a8563651f2f060b5f6cd` |
| E2E application | `518d29d4cdd93c15db7b6a66b6bcf120d6202c6870d009706faac6a7e26d71e5` |
| Runtime manifest | `a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc` |
| libmpv | `8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd` |

The existing runtime remains `.agent/runtime/mpv-0.42.0/coreaudio-cleanup-r1/`,
mpv source `b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, API 2.6. All 30 runtime
artifacts were verified before native execution and after validation. The
frozen spec/tasks and generated combined fixture retain the hashes listed in
report 0009. Maintained changes are confined to `engine.go`, `surface.go`, new
`shutdown.go`, their tests, and `cleanup_native_test.go` under `mpvprobe`.

This report and README/stage-zero navigation were added after execution. Final
`evidence/handoff-candidate/identity.json`, manifest and archive identify the
complete documentation candidate, with every tested implementation byte verified
unchanged. The full diff and separately identified runtime are available to the
reviewer; HEAD alone is not the changed candidate identity.

## Verification

Environment: macOS 27.2.1 build 26B131, arm64, Go 1.27.3. Commands used
`GOTOOLCHAIN=local`, `GOWORK=off`, the existing local GOPATH, `GOPROXY=off` and
`GOSUMDB=off`. No live service or account media was used.

| Check | Result |
| --- | --- |
| New pending-frame regression on old implementation | Expected FAIL: premature render free; Close 211.338125ms; preserved before repair |
| `go test -count=1 -race ./experiments/playback/mpvprobe` | PASS |
| `go run ./cmd/dev check --run 20261111T153610Z-04f30db12855 --race` | PASS, all six gates |
| Existing runtime loader/API/symbol preflight | PASS |
| Native paused Close | PASS, 2.457708ms |
| Native playing Close, three sessions | PASS, 82.791167 / 61.301208 / 61.393792ms |
| Native playing context cancellation through complete join and Close | PASS, 82.325458ms |
| Native repeated pause/resume | PASS, Close 49.067750ms |
| Native idle expiry/resume | PASS, Close 68.220250ms |
| Native repeated paused sessions | PASS, Close 2.790000 / 1.583792 / 1.721917ms |
| Complete six-scenario native cleanup suite | PASS, first invocation; zero ownership, no fallback and eight-second final survival in every scenario |
| Complete capabilities invocation | FAIL at separate-track readiness; forced cleanup retained |

The deterministic regression models video-output destruction requiring its last
frame to be consumed, and inserts an unrelated reply before the matching one.
Additional cases cover target-render failure with skip servicing, context
cancellation, rejected commands/replies, missing reply, event overflow, shutdown
event, missing event storage, bounded event consumption and quit rejection.
Existing partial initialization, queue saturation, terminal-state ordering,
pixel ownership and repeated-Close checks remain in the full race suite.

The native binary was built with `go test -c -tags nativecleanup`, then executed
once with `-test.run=^TestNativeCoreAudioCleanup$ -test.v -test.timeout=120s`.
The suite adds three playing sessions and a context-cancel scenario; it preserves
the 200ms and eight-second assertions. Native timing tests use the uninstrumented
native build; Go race checks neither load libmpv nor instrument its C code.
Context-cancel timing is a direct adapter measurement, distinct from an external
UI action-to-snapshot observation.

The unchanged full `replacement-smoke -stage capabilities` invocation passes
exposed local/HTTP picture, generation isolation, combined controls, protected
headers, header isolation, negative routes and retry. External loading cancel
passes at **60.994958ms** with zero ownership. The stage then fails:
`separate ready: context deadline exceeded (state=failed error=media-failure duration=0s owned=false)`.
The driver records forced app cleanup and zero active fixture handlers. External
playing cancellation, missing-audio failure and final graceful window close
remain unreached in this stage. No selective rerun or second capability
invocation replaced this failure.

## Review status and next action

CA-CLEANUP-001 retains its preceding independent technical closure.
CA-CLEANUP-002 retains its P1 open disposition and original 284.933708ms
independent failure. The assigned independent reviewer is
`/root/coreaudio_cleanup_review`; no reviewer was dispatched for this repair.
The run's `evidence/review-packet.md` identifies the exact candidate, original
finding, regression evidence and checks needed for closure. The next step is
independent verification of this repair; author checks do not approve it.

The separate-track readiness failure still needs its own diagnosis. Windows,
physical speaker output, measured playback rate/A/V sync, authorized service
playback, packaging and candidate CI remain unverified. The provisional mygo
preview stays the default. No M1 acceptance or broader application work follows
from this repair.

Local evidence is under
`.agent/runs/20261111T153610Z-04f30db12855/evidence/`: assignment and baseline,
`regression-before.*`, `targeted-race-r1.*`, `full-race-r1.*`, loader/build records,
`native-cleanup-r1.*`, `capabilities-r1/`, final identity checks and
`evidence-manifest.json`. The run ends `needs-human` for pending independent
verification and remaining capability gates. All earlier failures and historical
reports remain intact. Logs, binaries, captures, builds and archives are local
ignored evidence without guaranteed backup or retention; preserve them before
removing the checkout. No commit, push, PR, merge or remote publication occurred.
