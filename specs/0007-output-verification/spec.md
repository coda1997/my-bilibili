# macOS physical output verification preparation

> Owner platform amendment, 2026-11-12: [ADR 0005](../../docs/adr/0005-platform-priority.md).
> macOS is required; Windows is P2 / nice-to-have and does not block M1, M2
> or P0. Windows remains `not-tested` until exercised. Historical reviews
> describe their frozen candidates; this amendment changes only platform priority.

> Owner measurement amendment, 2026-11-12: [ADR 0006](../../docs/adr/0006-m1-measurement-scope.md).
> Calibrated physical sync/rate measurements were never performed and are outside
> M1 scope, with no mandatory later check. The retained tooling/method below
> defines validity if voluntarily used; it does not require physical capture.

Spec ID: `0007-output-verification`
Status: architecture proposal; exact review dispositions are recorded in the
current README-linked handoff. This spec alone grants no implementation authority.
Date: 2026-11-12. Continuation of RPL-AC-05, not a replacement M1 contract.

## User scenario and goal

The owner asked to read M1 status and continue, then selected **Mac only** when
asked which verification setup is available. The independently reviewed
replacement probe plays combined/separate fixtures, but the static picture and
continuous tone provide no paired events for measuring physical synchronization.
Prepare a reproducible synthetic event fixture and an opt-in observation session
so a later authorized observer can measure display/speaker output.

This proposal can make offline preparation executable on the current Mac. It
cannot supply the missing physical capture device, observer, calibration or
Windows environment (now optional P2). Completing preparation does not
complete RPL-AC-05 or M1.

## Scope and non-goals

- Add a Go fixture preparation command and deterministic timing/measurement
  helpers under `experiments/playback/outputprobe` and `cmd/output-fixtures`.
- Generate a separate, explicitly chosen output directory, never overwrite
  `experiments/playback/testdata` or silently regenerate existing fixtures.
- Add a bounded Go observation launcher under `cmd/output-observe`, reusing the
  existing `fixturehttp.StartProbe` and an explicitly supplied replacement-native
  app. Its inputs remain generated files and numeric-loopback HTTP only.
- Document human observation and calibrated external capture procedures. The
  launcher must not activate a microphone, camera, screen or system-audio recorder.
- Preserve the adapter, runtime, UI, original fixtures, all smoke suites, transport
  contract, check policy and CI. No default-engine change, target-service access,
  Windows loader work, dependency installation or distribution is included.

Prerequisites: the verified Go pin; separately available FFmpeg/ffprobe with
recorded identities for explicit generation; the current exact macOS native app,
runtime and exposed display for an authorized observation session. Ordinary Go
checks do not require these media tools, a display or native library loading.
External physical capture and calibration are unavailable in this continuation.

## Boundaries and shared contracts

### Fixture and timing oracle

Use the existing three filenames (`combined.mp4`, `video.mp4`, `audio.m4a`)
inside a distinct output directory. Emit the compatible hash/size manifest consumed
by `fixturehttp`, plus a separate versioned timing manifest. Retain the existing
640x360, 30fps, 48kHz, eight-second and 2MiB-per-media-file limits. Neither the old
manifest nor the old picture oracle is applicable to the new media.

Proposed source events start at media seconds 1 through 6. Each event has a unique
visible numeric/binary marker and a three-frame luminance pulse paired with a
100ms windowed acoustic ID. Each ID mixes a 440Hz reference with its identifier
tone: IDs 1–6 use 770, 990, 1210, 1430, 1650 and 1870Hz respectively. Their
identifier/reference ratios are 1.75, 2.25, 2.75, 3.25, 3.75 and 4.25.
Use moderate fixed total digital amplitude; document that this is
not a guarantee about speaker loudness. No full-volume instruction or automatic
system volume change. Record rational frame/sample indices for every onset and
offset, color/marker geometry, both tone frequencies, amplitude and sample recipe.
Generate raw picture
and PCM data in Go, then invoke FFmpeg as a subprocess with an argument list.

Encode combined media once and derive video/audio streams without re-encoding.
Decode all three outputs to verify actual event onsets against presentation
timestamps, including codec delay/edit-list effects. Container metadata or the
intended generation recipe alone is insufficient. Report per-event decoded A/V
offsets and duration/stream identities. Require event ordering, unique matching,
separate-versus-combined equivalence and decoded offsets within one 30fps frame
of the intended pair before the fixture is eligible for observation. This is a
proposed **fixture preparation bound**, not a player-sync acceptance tolerance.
Preserve actual errors; do not zero measured offsets to make the fixture pass.

Acoustic ID recovery must be independent of visual timestamps and presumed
small output skew. In each detected pulse, consider reference-frequency scales
from 0.5 through 2.0 and match a two-component ratio to the table above within
0.02 absolute ratio. Both components must be sustained together for at least
20ms and each exceed the local silent-baseline spectral power by 20dB; if a
usable adjacent silent baseline is absent, return no ID. All qualified pairs
must yield exactly one ID; zero or multiple possible IDs means unknown/ambiguous,
never nearest-time matching. Preserve the observed peaks, baseline, uncertainty
and rejected alternatives. Tests must include missing reference/identifier,
extra tones and reference-only harmonics; unsupported distortions yield no result.
These are proposed conservative detector rules, not a claim of physical accuracy.

Anchor the reference to the **lowest sustained qualifying frequency component**
across the analyzed positive-frequency band, using the same duration/power rules.
Do not search only the 220–880Hz reference range and discard lower components.
No higher component may substitute for a detected lower fundamental. An ambiguous
or out-of-range lowest component produces no ID. In particular, a reference-only
record containing 220, 440, 660, 880, 1100, 1320 and 1540Hz must anchor at 220Hz
and yield no ID; it must not select 880/1540Hz as ID1. Retain this exact regression.

Supported identification assumes the reference and identifier fundamentals are
observable with a common pitch scale. A spectral match cannot prove that premise:
a degraded reference-only record containing only its fourth/seventh harmonics
at 880/1540Hz is indistinguishable from an intact ID1 at scale2. Do not claim
universal missing-fundamental/harmonic rejection. Actual decoded generated media
must establish the intact-signal premise by comparison with the known recipe.
Physical capture needs documented setup qualification, including reference-only
and known-ID controls in the same mode, or separately documented independent
event identification. Without that support, a plausible spectral label remains
unqualified and cannot supply an accepted audio ID for physical measurements.
Known unsupported or ambiguous distortion yields no result. Qualification itself
has uncertainty and cannot prove immunity to every unobserved distortion.

Validate the actual decoded combined and separate fixtures with this same ID
decoder. Exercise 0.5x/1x/2x duration changes with both preserved and uniformly
scaled pitch in deterministic records. Cropped/post-seek sequences, missing edge
or interior pulses, duplicated/unknown IDs, and offsets exceeding one full event
period must recover the true IDs or reject the segment. Never accept caller-
supplied audio IDs without retained decoder evidence or a separately documented
independent identification procedure. If real fixture generation fails these
rules, preserve that failure and return the method for review rather than tuning
the detector against a desired physical result.

Generate in a new staging directory and publish only after validation. Reject
existing/nonempty destinations, symlink destinations, incomplete events, extra
streams, oversized files and failed decodes. Bound subprocess runtime and output;
cancel and reap owned tools before returning. Do not fetch media or tools.

### Observation and lifecycle

The launcher verifies both manifests and explicit binary/runtime identities,
starts one owned loopback fixture server, and launches one supplied app with the
existing `-fixtures`, `-http-base` and `-library` arguments. It adds no UI types to
business code and no new native callback or worker ownership to the adapter.
The existing controller retains generation, queued-command, UI-thread and
complete-Close semantics. Do not run the old pixel-color smoke oracle against
these new fixtures or label this session a complete capabilities-stage pass.

Record combined and separate runs distinctly, the selected rate, observation
segment, seek target, relevant stream counts, and observer statements with their
provenance. At minimum collect steady 1x, 0.5x and 2x event intervals and a 1x
post-seek segment, with at least four correctly identified event pairs per
measurement segment. Preserve startup/transient observations separately rather
than silently dropping them. Playback state/readbacks remain configuration
evidence; they cannot fill physical-output cells.

The launcher has explicit bounded session duration (default 180s, maximum 600s),
handles cancellation, and retains app/server exits. Allow the existing 3s app
cleanup and 3s server-shutdown supervision bounds. A forced reap of its owned
child is a failed session, recorded separately from graceful completion. Those
supervision bounds do not replace the unchanged 200ms adapter cancellation bound;
only an actual instrumented cancel observation can assert that bound. Failure or
interruption leaves a partial result with all unobserved cells not-tested.

### Physical measurement and uncertainty

A human may explicitly attest that the Mac speakers produced the expected tones.
That supplies only a scoped audibility observation, not calibrated timing or a
claim by the agent that it heard output. An external camera/recorder must capture
the display and acoustic output together for physical measurements. Screen
recording, loopback audio and native playhead sampling are supplementary paths;
they do not observe both physical outputs.

Before a capture can yield calibrated sync results, identify the capture device,
recording mode, actual timestamps/frame cadence, acoustic placement and a
documented calibration method/reference for its audio/video skew and uncertainty.
The tested player cannot calibrate its own capture path. Unknown calibration
leaves the measurement uncalibrated; a camera advertised frame rate is not proof
of a common timebase or zero skew. Include uncertainty from event localization,
fixture decoding, capture timing and reference/placement. Do not silently subtract
display or speaker latency: those are part of the observed playback output path.

Define positive offset as audio arriving after its paired visual event. Retain
raw observed offsets, capture calibration correction, encoded-fixture offsets
and derived residuals separately with units and sign. Report intervals/rates for
display and audio events separately, matched by event ID, with uncertainty and
the precise sample window. Refuse mismatched, missing, duplicate, nonmonotonic or
insufficient events; preserve the rejected input and reason in sanitized evidence.

Use these time-domain definitions, all in seconds:

- `O_i = captured_audio_i - captured_video_i` is raw capture-time offset.
- `C_i` is capture audio-minus-video skew measured with an independent reference;
  a positive value means the capture path itself makes audio appear late.
- `S_i = O_i - C_i` is the observed physical output offset after capture correction.
- `D_i = decoded_audio_PTS_i - decoded_video_PTS_i` is encoded fixture offset in
  media time, not capture time.
- Establish a local mapping `g(media_time)` into capture time from independently
  identified visual events in that continuous segment. Then
  `F_i = g(decoded_audio_PTS_i) - g(decoded_video_PTS_i)` and `R_i = S_i - F_i`.
  `R_i` is the player residual relative to the fixture; keep `S_i` visible because
  subtracting the fixture offset is not a measurement of total observed sync.

For a justified local constant slope `b` in capture seconds per media second,
`F_i = b * D_i = D_i / r`, where `r = 1/b` is the measured local playback rate.
Fit the visual mapping from at least four identified events and retain the data,
slope interval and fit residuals; do not use selected/native rate as the mapping.
Do not fit away audio/video skew with independently adjustable audio intercepts.
Report audio rate separately so unequal stream rates and drift remain observable.
With `C=0`, a +20ms encoded offset yields +40ms observed offset at 0.5x and +10ms
at 2x for a perfect player; both give zero residual, not +20ms/-10ms.

Propagate bounded uncertainty conservatively through `O-C` and `b*D` using
interval arithmetic (including slope, calibration, decoded onsets and event
localization), not an assumption that errors are independent. If the visual
events cannot be explained by one slope within their declared uncertainty,
the mapping is unidentified. Never span a seek, rate change or discontinuity
with one fit. Split only at recorded segment boundaries; retain transients and
all rejected points, without selecting points to improve the result. Unsupported
nonlinear/transient mappings return an inconclusive residual with raw values.
Without valid calibration, retain raw observations but no calibrated `S_i` or
`R_i`; without a valid local map, no `F_i`/`R_i`. Record uncertainty rather than
inventing a player acceptance tolerance.

No numerical player-sync or physical-rate acceptance tolerance exists in the
current contract. This scope delivers valid measurements or explicit not-tested/
uncalibrated outcomes. It does not invent a passing threshold from collected
results. If future voluntary work asserts physical sync/rate PASS, it needs
independently assessed measurements and owner-approved criteria. Under ADR 0006,
no such claim, measurement or tolerance is required for the M1 adapter decision.

No capture begins without the user's explicit device/session authorization.
Keep raw recordings outside Git and reports; retain only authorized synthetic
display/sound evidence, hashes and sanitized conclusions. Stop recording before
unrelated applications or conversations enter the capture.

## Acceptance criteria

### OUT-AC-01

Explicit generation produces distinct combined/separate fixtures, exact tool and
media identities, a compatible server manifest and verified decoded event timing.
Bad inputs/tool failures terminate boundedly without overwriting existing files.
Existing six fixture/provenance files remain byte-identical.

### OUT-AC-02

An explicitly invoked observation session uses only the supplied candidate,
runtime and new synthetic fixtures. It records exact provenance and complete
owned-process/server cleanup, preserving failures and unavailable observations.
The old full capabilities suite remains a separate unchanged verification.

### OUT-AC-03

Deterministic analysis tests recover acoustic IDs independently of visual time,
known event ordering, audio-late sign, time-domain corrections and independent
audio/video rates from generated measurement records. Include hand-calculated
positive/negative fixture and capture corrections at 0.5x/1x/2x, unequal stream
rates/drift, uncertain or unidentified mapping and transition/post-seek windows.
Known injected offsets, bad calibration and missing/duplicate events
demonstrate both valid and rejected paths. Missing calibration/observer evidence
cannot yield physical-sync or audibility PASS. Analysis tests do not claim a
physical output measurement.

### OUT-AC-04

The durable matrix distinguishes fixture validation, native controls, human
audibility, raw physical observations, calibrated sync/rate and final acceptance.
Physical sync/rate measurements were never performed; retain not-tested cells
marked outside M1 scope under ADR 0006, with no mandatory later check. Independent review assesses
the exact candidate and method; no report accepts M1 or other platform support.

## Verification matrix

| AC | Procedure | Environment | Evidence |
| --- | --- | --- | --- |
| OUT-AC-01 | Go recipe/oracle tests; explicit generation and decoded event audit; failure/cancel/destination tests | Ordinary tests offline; actual generation on Mac with recorded tools | Recipe, commands, timestamps, tool/media hashes, original-fixture identity comparison |
| OUT-AC-02 | Launcher lifecycle tests with controlled child/server; later authorized observation | Headless tests; actual macOS display separately | Exit/cleanup records, candidate/runtime identities, first complete session outcome |
| OUT-AC-03 | Hand-calculated positive/negative offset, drift/rate and uncertainty fixtures; malformed/missing calibration cases | Offline Go tests | Raw/derived values and error disposition; no fabricated capture evidence |
| OUT-AC-04 | Full `go run ./cmd/dev check --race`; exact candidate and independent method/code review | macOS first | Check log, review, per-capability matrix and missing-input list |

## Risks, rollback and open decisions

The next independent architecture reviewer must assess event detectability,
encoded delay handling, calibration independence, uncertainty propagation,
lifecycle supervision and whether this scope stays separate from existing gates.
An eight-second fixture limits measurement windows; insufficient precision is an
honest inconclusive result, not authority to change fixture limits or tolerances.

The owner has only a Mac available. Capture device/mode, calibration reference,
observer and authorized session would be prerequisites only for future voluntary
physical work. ADR 0006 creates no requirement or schedule for that work.
Target-service access remains a separate M1 blocker. Windows is deferred P2
and does not block M1, M2 or P0. Any implementation
need to alter the adapter, original fixture/server contract or runtime returns
to architecture review with a new scope. Rollback removes only this new opt-in
tooling if authorized; it preserves old and new evidence and the default preview.

## Review and evidence

Planning run: `20261112T125901Z-88d36e6b471d`, recording against the existing
replacement contract. The proposed contract is not frozen for implementation.
See `assignment.md` for review context. The first independent review is retained
in run `20261112T130353Z-b0291eddf22b`; it approved the standing delegation changes
and requested acoustic-ID/time-domain corrections as OUT-ARCH-001/002. This
revision defines those corrections. Only the original reviewer may close them
on the exact revised candidate; the current handoff records that disposition.
After architecture review and implementation authorization, start a new run with
this spec and retain an exact source manifest/archive before checking/reviewing.
Final evidence belongs to a new controller run and a sanitized report under
`reports/0016-output-verification.md`; that report does not yet exist.
