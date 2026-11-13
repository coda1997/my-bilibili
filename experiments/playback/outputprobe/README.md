# Paired-event preparation and analysis

This package creates synthetic eight-second fixtures and performs deterministic
measurement arithmetic. It does not record a device, play media, qualify a
physical capture setup, or declare M1/physical synchronization PASS.

Generate only into a new directory outside the original fixture tree:

```sh
go run ./experiments/playback/cmd/output-fixtures -out .agent/output-events-r1
```

The parent directory must exist. FFmpeg and ffprobe must already be installed;
the command records resolved binary hashes and versions and never downloads
tools. It creates source grayscale frames and signed-16-bit PCM in Go, encodes
combined media once, stream-copies separate media, and decodes all three outputs.
A compatible `manifest.json` binds media hashes/sizes. `timing.json` retains the
recipe, tool identities and commands, stream/frame timestamps, acoustic detector
evidence, combined/separate equivalence and decoded A/V offsets. Publication
occurs only after validation; a failure keeps its staging directory and partial
timing evidence. A failed stage is never an eligible observation fixture.

The visible marker is binary, least-significant bit on the left. Events 1–6
start at seconds 1–6, each lasting three video frames and 100ms of audio. The
pulse rectangle and three marker rectangles are recorded in the recipe. Each
audio event mixes a 440Hz reference and its unique identifier tone with 5ms
raised-cosine ramps, at 0.16 digital amplitude per component. Digital amplitude
is not speaker loudness; the tool never changes system volume.

Acoustic labels are recovered from audio alone. The detector retains actual
frequency components, frequency-localization bounds, adjacent baseline evidence
and supported time intervals; it never matches against nearest visual time.
Candidate discovery also examines overlapping local residual spectra, so a
whole-pulse taper cannot silently suppress a sustained component near an edge.
Components whose duration uncertainty overlaps the 20ms qualification threshold
remain explicitly unresolved and prevent an ID; they are not discarded in favor
of a higher reference tone. Unresolved intervals include both the removed edge
block and the potentially partial neighboring block in their uncertainty.
Digital resolution and unsupported distortion can yield no ID. A plausible
spectral label alone is never qualified physical identification: the prescribed
same-mode reference-only/known-ID controls or a separately documented independent
identification procedure remain required. In particular, 880/1540Hz can be intact
ID1 at pitch scale 2 or degraded fourth/seventh harmonics of a missing 220Hz
fundamental. Signal matching cannot distinguish those histories.

```sh
go run ./experiments/playback/cmd/output-fixtures -verify .agent/output-events-r1
```

Verification loads bounded regular files, checks both manifests and hashes, and
validates retained decoded evidence. It does not re-run the decoder. Retain the
exact trusted generator/candidate and timing-manifest hash when using that
evidence; editable local manifests are not authenticated attestations.

`AnalyzeSegment` consumes four to six matched, continuously ordered event pairs
with uncertainty intervals and retained identity references. It preserves raw
observations, derives the visual media-to-capture slope from the actual events,
and reports audio rate separately. Capture correction, physical output offset,
mapped fixture offset and player residual stay distinct. It rejects missing,
duplicate, unqualified and discontinuous events and never substitutes configured
rate for measured mapping. Transients, unavailable calibration and unidentified
mappings remain inconclusive or uncalibrated. Evidence hashes identify artifacts;
callers/reviewers must resolve and assess those artifacts independently. Even a
`measured` arithmetic result is not physical acceptance or final adapter approval.
