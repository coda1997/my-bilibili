# M1 output verification preparation and remaining gates

2026-11-12. macOS remains the required platform; Windows is P2 / nice-to-have
and does not block M1, M2 or P0. **M1 remains in progress and unaccepted.**
The default provisional mygo preview is unchanged. Reviewed synthetic fixtures
and an observation launcher are prepared. A subsequent normal-app retry now has
positive owner audibility statements for combined and separate fixtures under
the prompted 1× setting; independent evidence review approves these scoped
attestations. Calibrated physical sync/rate
and real target-media validation remain incomplete.

## Delivered

[Spec 0007](../specs/0007-output-verification/spec.md) now has executable
OUT-001/002 tooling. The Go generator creates isolated eight-second paired-event
fixtures, encodes combined media once, derives separate streams, and decodes all
outputs to retain actual timestamps, acoustic IDs and fixture offsets. It binds
media, recipe, commands and tool hashes in separate timing evidence. The six
original fixture/provenance files remain unchanged.

The analysis helpers match acoustic IDs independently of visual time and retain
raw offsets, independent audio/video rates, capture correction, mapped fixture
offsets, residuals and bounded uncertainty. Missing calibration, ambiguous IDs
or unsupported timing maps yield uncalibrated/inconclusive results. A measured
arithmetic result does not establish physical acceptance.

The opt-in observation launcher verifies supplied app/runtime/timing identities,
starts the unchanged loopback fixture server and bounds app/server lifetime.
It preserves failure/interruption/forced-cleanup outcomes, never starts a recorder
and leaves all unobserved cells not-tested. Configuration labels are intent,
not proof that the app selected a stream, rate or seek position. The
[experiment guide](../experiments/playback/cmd/README.md) describes invocation,
manual observation and the separate calibrated-capture prerequisites.

## Exact candidate and verification

Controller run: `20261112T132454Z-0e31baa6de4d`. Base HEAD:
`d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty; the base commit
alone is not this candidate. Frozen spec SHA256:
`3fef448a2fba2a96d8d4da5815a38c7a58cb2587c3b9ee504c2c68ad269767ac`;
tasks SHA256:
`341415c79cac6f6b93f9aaa4795284c743f26b3093a98cc7946f55864e766323`.
The platform amendment in [ADR 0005](../docs/adr/0005-platform-priority.md) and
[report 0018](0018-platform-priority.md) remains in force. Frozen proposal-era
spec/assignment status text describes its original packet; this report records
subsequent implementation and review without rewriting frozen inputs.

| Repaired r1 artifact | SHA-256 |
| --- | --- |
| `evidence/candidate-r1/manifest.json` | `c45182838af108850310cee7165c125d53b43cce63ed7aa81ddc286b12bda956` |
| `evidence/candidate-r1/source.tar` | `490b6112eba5a969b8479b6d1150266d2bd023e8c81152928269ea972e2cc5a9` |
| Repair diff against r0 | `fc55fb1d5592b80b8e26f8e8bab332d381a64f1857f5b87e3d01aad7f918b2ef` |

The controller's recorded `go run ./cmd/dev check --run
20261112T132454Z-0e31baa6de4d --race` passes all six stages on macOS arm64 with
Go 1.27.3 and the existing offline module cache. The author package race tests
and actual FFmpeg/ffprobe generation plus manifest verification passed after
repair. Three media files and six decoded event pairs passed; final timing SHA256:
`27723a276189d7c28bda3f2f0e013f26063371a6b3ea21bc9707d67451a9f96e`.
The final author fixture directory is preserved under
`evidence/out001-repair-r1/artifacts/.agent/worker-scratch/output-implementation/fixtures-r2-final/`.
Its intact-media audit matches the earlier valid generation byte-for-byte, but
was actually regenerated after the source repair. The repair inventory binds
source and execution evidence, SHA256
`d7b74c1397b7f885c304961fe975cfb8f9f9577392044aad34e37428997e8d5b`.

Launcher tests cover normal/failing exit, spawn failure, expired deadlines,
interruption, forced cleanup, server shutdown and nonfinite arguments. A separate
controlled-helper invocation verified app/runtime/fixture identities and graceful
lifecycle; its session SHA256 is
`34d84bacf03db685ee5ebb4ebefe5fb1d0ebcfc8beb96535af4a628b2e8d7737`.
That helper fetched no media and rendered no output. It is not a native playback
observation; every physical and native-disposal cell remains not-tested.

Independent reviewer `/root/output_code_review` approved the exact r1 method/code
for OUT-001/002 technical preparation. The reviewer independently reran both
packages with race detection, reproduced both original failures against repaired
behavior, regenerated and verified actual media, and rechecked all152 candidate
identities plus the six original fixtures. The independent generation produced
the same timing hash. The full integrated log was inspected rather than duplicated.
Review SHA256:
`3638ce32f2cfa5e3056fb7df3d66e46d268b5eb69387366c25844133337e3bb4`;
14-artifact inventory SHA256:
`74dcf832142cee268c2320802a32f6daf54e6cf23a40318663df7fddc6462ec7`.
Both are retained in `evidence/independent-r1/`.

This report and README/stage-zero navigation were added after code review. The
run's final handoff snapshot identifies that documentation-only delta; code and
frozen inputs remain identical to reviewed r1. OUT-003 records the actual review
and unavailable observations; it makes no M1 acceptance decision.

## First failures and independent findings

The first author implementation incorrectly accepted nonoverlapping/short
components; its original failing tests and repair evidence remain retained.
Independent r0 review then requested changes despite a passing full race check
and actual fixture generation:

- **OUT-CODE-001, P1:** whole-pulse spectral discovery could hide a lower tone
  sustained near a pulse edge and accept the wrong acoustic ID. The repair adds
  local residual-spectrum discovery and explicit unresolved-duration evidence;
  ambiguous components prevent identification. The original cases and 225 new
  edge/sample-rate/playback/pitch cases are retained.
- **OUT-CODE-002, P2:** a nonfinite seek value could launch the child and then
  prevent JSON result serialization. Validation now rejects nonfinite rate and
  seek values before session side effects. Six real CLI regressions cover both
  flags with NaN and positive/negative infinity.

Both findings are **technically closed by their original independent reviewer**
on exact r1. No new blocking issue was found within the reviewed scope. This is
preparation approval, not a physical-output or production-player decision.

The first review, reproductions and r0 candidate are preserved under
`evidence/independent-r0/` and `evidence/candidate-r0/`. Initial and intermediate
fixture generations remain historical artifacts; only the final generator-bound
fixtures may be used for subsequent observation. Verification checks retained
manifest evidence and media hashes; it does not rerun FFmpeg or authenticate an
editable manifest. An observation must use the exact trusted timing hash.

## Subsequent native observation and corrected instructions

Observation run `20261112T140714Z-31f31afeb236` freezes the same 0007 spec/tasks.
All153 files matched the preceding checked handoff before the normal app and
launcher were built. App SHA256:
`cc01a4f29b6157e74c80bace150106863821b81c3893c5f612aa6458396fc863`;
launcher SHA256:
`8d05b2d4c9bffe3cfef1bf3d0443f96b1219d6e5d9f8b400cb3834d389cea5b3`.
The runtime manifest is
`a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`;
all30 library/dependency hashes matched. The final fixture timing hash above
was reverified. No maintained Go code, runtime or media changed.

The first human session was inconclusive: the owner selected “Could not verify,”
then reported a playback/audio issue and that playback did not progress. It
reached its180s deadline, then exited gracefully with zero resources reported,
no forced reap and a closed server. The first outcome remains under
`evidence/manual-combined-r0/`; it is not replaced by later success.

The controller's instructions and initial procedure omitted **Play**. Source
inspection shows that selecting a fixture loads it paused. A bounded diagnostic
confirmed Ready/position0 before Play and advancing native time afterward:
2.93s for the normal adapter with the new fixture, and2.90s for both new and old
fixtures in the instrumented UI. All probes cleaned up. The instrumented UI's
settle operation can affect frames; those probes alone do not prove the normal
UI rendered correctly. The original symptom is consistent with the missing
step, not proof that no other cause could exist. The procedure now explicitly
says to press Play and scroll if necessary; no hidden-button defect was proved.
Diagnosis source/results are retained under `evidence/diagnosis/`, inventory
SHA256 `b7776a0d873af5746677ceedba1d1ccd35116476d78113468af37c0803ac01fd`.

The owner then tried the unchanged **normal app**, following HTTP fixture →1×
→Play, and answered **“Advanced; six tones heard”** to the prompt asking whether
position advanced and all six events were audible from the Mac speakers. The
separate-stream retry used Separate tracks →1× →Play and received the same
answer. These are identified human statements, not an agent claim to have heard
sound. The separate server record also shows one allowed, header-matching request
for each video/audio stream; request counts do not prove native track selection.

| Human observation | Session SHA-256 | Statement record SHA-256 |
| --- | --- | --- |
| Combined1× | `3e441296f12888188ed6ce75a3c321eee5ce4466908bcaa254aaa580022de80e` | `5ca21b0772f6df03b62c4abe97d3b4d7a3a234c78ee886828d3f37a6fcb21540` |
| Separate1× | `52a5b6729812b95e09f0dd8b9dd295784aaf9237b9f2f738622e786fbefb903d` | `a866ce93b12b30d8f357f682fd2fc202305ad9124feffe240f7fed438253487e` |

Evidence lives in `evidence/manual-combined-r1/` and `manual-separate-r0/`.
The controller interrupted each owned supervisor after the human response;
both records correctly remain **partial**, with app exit0, no forced reap,
closed servers and zero-resource app logs. They are not normal-completion PASS
records. The original session JSON remains unchanged with unobserved cells
not-tested; each separate human statement binds its exact session hash.
No capture, calibrated timing, native rate/track readback or200ms cancellation
measurement was made.

Independent reviewer `/root/output_code_review` **approved** the observation
evidence and corrected procedure with no blocking findings. The two positive
owner statements satisfy the contract's limited human audibility attestation;
this is independent evidence review, not independent hearing, measured output
rate or M1 approval. The reviewer verified all 153 candidate files, 24 retained
session/diagnostic artifacts, all three statement-to-session bindings, the app
and launcher binaries, and 30 runtime dependencies. All 66 Go/module files match
the approved preparation candidate. The reviewer derived and read the complete
four-document delta because the scoped implementation diff covers only the
command guide.

| Observation review artifact | SHA-256 |
| --- | --- |
| `evidence/observation-candidate/manifest.json` | `e85da1a24c23347cb2dc7f7bdc7346c468ef8e44f3be545b94c1c0d97adf5156` |
| `evidence/observation-candidate/source.tar` | `23f5465af5ab1ca9f988ba2c48af8f665ea3187f7dd7d879556d9a803fff8b68` |
| Independent observation review | `a5e58978d4ab3ffcec9ecba5d0fc6533670d1cd419876ba3229bc1da44df4860` |
| Independent review inventory | `75e4ca6f2e91a113774ab5e687bb07406bde2a15cd3f3ca1717c248ecf01af42` |

Hash-verified review artifacts are retained in the observation run's
`evidence/independent-observation-r0/`. Recording this disposition in this report
and README/stage-zero is a subsequent documentation-only delta, identified by
`evidence/handoff-candidate/`. No Go checks or native sessions were rerun for
this final documentation update; the prior passing race checks apply to the
unchanged Go source, not a new execution or CI result.

## Capability and acceptance matrix

| Capability | Current evidence / disposition |
| --- | --- |
| Required platform | macOS; Windows P2 remains not-tested and nonblocking |
| Existing native synthetic capability stage | Prior independent 41-assertion PASS; [report 0014](0014-separate-track-review.md), not rerun or relabeled here |
| Fixture generation and decoded timing/ID validation | Final actual generated combined/separate fixture checks passed on macOS |
| Deterministic measurement arithmetic | Unit/race tests; no physical observation implied |
| Observation launcher lifecycle | Controlled tests plus three normal-app sessions; deadline/interruption partial outcomes retained, all graceful without forced reap |
| Human audibility | Independently reviewed positive owner statements for combined/separate fixtures under prompted 1× after explicit Play; other modes untested; no measured rate claim |
| Raw physical display/speaker capture | Not-tested; no recording performed |
| Calibrated physical sync and actual display/audio rates | Not-tested; external capture/calibration unavailable |
| Physical sync/rate numerical acceptance criteria | Not yet approved; no numerical PASS may be invented |
| Authorized real target media | [Independent preflight needs-input](0019-authorized-target-playback.md): permission remains unknown; native playback not-tested |
| Final adapter decision and owner M1 acceptance | Pending; provisional mygo remains default |
| Packaging, changed-candidate CI, other native OSes | Not-tested; no new result claimed |

## Evidence retention and next action

The run's ignored evidence retains baseline/r0/r1 source snapshots, author
fixture generations and tests, controlled launcher session, independent findings
and review artifacts. These are local-only and not backed up by Git. Preserve
them before deleting the checkout; this sanitized report is not a remote artifact
attachment. No commit, push, publication or runtime/dependency installation is
part of this continuation.

The combined/separate audibility statements have scoped independent evidence approval.
The next missing physical evidence is calibrated display/speaker synchronization
and actual rates; audibility does not establish those measurements. They require
external capture equipment, an independent calibration reference and setup
qualification. Final assessment also needs approved criteria. The Mac-only setup
still cannot supply that evidence; neither tooling nor audibility waives it.

For target-media work, read [report 0019](0019-authorized-target-playback.md).
The bounded metadata correction reached the reviewed stop condition; no further
request is allowed by that packet. An interpretable ordinary authorized grant
or a separately reviewed access-validation method is needed before TGT-002/003.
Only after the remaining macOS evidence exists can independent adapter assessment
and owner M1 acceptance unlock M2/P0. Standing native-subagent authority remains
available without a per-dispatch permission prompt.
