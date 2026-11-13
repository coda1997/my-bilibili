# M1 target-access interpretation method

> Supersession, 2026-11-13: the owner removed all target host restrictions in
> [ADR 0007](../docs/adr/0007-target-host-policy.md). Earlier exact-origin approvals,
> historical-origin comparison requirements and any proposed two-host amendment
> are superseded for current work. All valid HTTPS hosts/ports are allowed. The
> dated evidence and stopped outcomes below retain their original meaning;
> see [the host-removal implementation](0022-target-host-removal.md). Current
> target results and remaining M1 decisions are in the dated continuation below.


Current handoff: [post-repair target PASS and adapter recommendation](#post-repair-target-continuation-2026-11-13).

## Historical record before the post-repair target continuation

Updated 2026-11-13. TGT-001 and the TGT-002 implementation have independent
scoped approval. Current native fixture regression passes. The first TGT-003
refresh stopped with UNKNOWN before native launch because the selected audio ID
or initial origins differed from the frozen observation; target playback remains
not-tested and M1 is incomplete. The two UNKNOWN observations in [report 0019](0019-authorized-target-playback.md)
remain unchanged. The current M1 scope is [report 0020](0020-m1-measurement-scope.md):
calibrated physical measurements are outside scope, but authorized target-media
validation and final independent/owner decisions remain required.

## Why the earlier method stopped

The earlier extractor observed selected candidate fields but did not classify
access. Its offline tests demonstrated redaction and field handling; they did
not establish an actual service schema. Missing guessed flags therefore prove
neither denial nor permission. Repeating the same observation would not resolve
that methodological gap.

Read-only research inspected pinned public client implementation and community
schema observations. They support investigating ordinary source classification
and returned rendition together, rather than demanding a universal positive
grant flag. These sources are not official service permission guarantees. The
[reviewed method](../specs/0008-authorized-target-playback/access-method.md)
records exact provenance, field/type rules and that limitation. No service
restriction is waived by adopting a better interpretation method.

## Bounded continuation

After independent review and a new freeze, one fixed anonymous view metadata
request may establish exact source identity and ordinary nonpay/nonexclusive
classification. Only an eligible result permits one request to the previously
selected playback endpoint. Restriction, preview, malformed, missing or conflicting
evidence stops the attempt. No signing, cookies, alternate identities/endpoints,
automatic retries or native media consumption are part of this allowance.

Offline classifier/redaction tests preceded both requests. Independent TGT-001
review of actual normalized results and exact initial origins still precedes
implementation. A preflight result cannot provide a lasting native descriptor:
a later TGT-003 refresh requires a separate explicit reviewed assignment.

## Review and evidence

Independent reviewer `/root/target_architecture_review` approved the exact method
with no actionable findings. Review SHA256:
`14aa49b389e44cdcf406de2a31bf0edee721186a9fc0d3297a4fd6a82ae29779`;
review inventory SHA256:
`303d7fb70d8bee95063f0c9a2de458d7f7f7d419dba72c6f945639bff5a01d43`.
The reviewer verified all 157 candidate files, the four-file amendment, actual
spec validation and three pinned public sources, and inspected the completed
six-stage local race check. No target request ran during review.

New run `20261112T144351Z-ac60cbfe2590` freezes spec/task inputs and retains the
review, candidate and checks. Its execution assignment also binds the copied
access method to SHA256
`5b4e7a428ad3a781541a0afb5bcd63f7fa7ed610809f7dd2dc6a8788e90fc335`.
Reviewed candidate manifest/archive SHA256:
`4bc8603a7845b769940af812b6c39b801e55af935fb919c0413319eda887228d` /
`7ee87aa0799df1cb613c851aad2d8c111b760bd9822c20fe368afd9f542c4edb`.
This status update is a subsequent documentation delta, not part of that reviewed
candidate. Historical reports, failed outcomes and frozen
inputs remain untouched. The method amendment itself changed no maintained Go,
runtime or media. It establishes no target compatibility result, remote CI
result or M1 acceptance; subsequent implementation is recorded separately below.

## Offline classifier and actual observation

Independent review found two scratch-classifier defects: locator validation
missed encoded controls/localhost names, and duplicate selected audio IDs could
evade detection across codecs. The first repair closed the audio finding; a
second completed Unicode-control and decoded UTF-8 validation. Original sources,
failing regressions and partial closure remain preserved. The original reviewer
closed both findings on r2 after 123 independent tests/subtests passed with race
detection; vet and default invocation with zero requests also passed. Final review
SHA256: `6a275c42f9d48925e05bb906aca38d6f7c76c623f58c749715440b52e9a2ab57`.
Exact scratch r2 manifest SHA256:
`8d7d81020235644cb777cead89c84f3e8e35b6b6bd05f1f0520a5353ac18f7bc`.

The controller then recorded and executed the sole bounded observation release
on 2026-11-12 at 14:56:11 UTC. Both fixed requests returned HTTP200/code0.

| Observed fact | Result |
| --- | --- |
| Public source and selected part | `BV1Zs411o7Ee`, avid899574, cid1445374, page1, `vupload`; exact match |
| Source classification | State0; pay/ugc_pay/movie/arc_pay/ugc_pay_preview zero; exclusive/preview/chargeable flags false |
| Restriction consistency | No source redirect/forward/paid season; no trial marker; view_info null; guessed permission flags absent |
| Selected rendition | Quality32, H.264/AVC video/mp4; audio30216, AAC-LC audio/mp4; from2video/3audio representations |
| Duration consistency | Selected part596s; playback596624ms, within the reviewed2s tolerance |
| Exact initial video origin | `https://upos-sz-mirrorcos.bilivideo.com:443` |
| Exact initial audio origin | `https://upos-sz-estgcos.bilivideo.com:443` |
| Method outcome | Ordinary anonymous full-rendition candidate under the reviewed trusted-source inference; independently accepted for scoped TGT-001 |
| Native target playback | Not-tested |

The method uses the combined facts; absent flags, null state, duration or HTTP
success alone are not grants. Third-party schema observations remain a trust
limitation, not an official service guarantee. The preflight does not establish
native redirect destinations or playback. The pinned runtime's known ability to
follow HTTPS-to-HTTP redirects remains explicitly documented in report0019.

Normalized result SHA256:
`a19a4247289b7f40d13302cca0a1a95c646c06d9f9f6c4e53804a634b4822435`.
The run retains it as `evidence/observation-first.json`, alongside the exact
execution release and classifier/review versions. Response bodies and media
locators were transient and discarded; no locator hash or target recording was
retained. The observation allowance is exhausted.

## Independent gate and next action

The independent reviewer accepted the actual source/part/classification/rendition
facts and the two exact origins for their respective video/audio roles. TGT-AC-01
is satisfied for scoped TGT-002 implementation. Review SHA256:
`325e9b5f06af39dc13b099f5dbe641f75bdd4e8405e95586a4324d5f3724e297`;
inventory SHA256:
`44e0b451ce12895f1d47c0f0d9cecf976cc447445b90e769b897b6d2ed052dc6`.
All 30 pinned runtime artifact hashes remain unchanged. The reviewer performed
no additional service/native request and retained the known redirect limitation.

The run records a TGT-002 assignment for the opt-in immutable input policy,
dedicated native composition and bounded driver. Implementation and its bounded TLS repair now have independent scoped approval
under the frozen contract; existing fixture-only defaults remain unchanged.
Pre-implementation manifest/archive SHA256:
`478c89ca0e1d47e6f6f4150b91d2b7b096d3c1a812ebfccef16d4dae468b5210` /
`a5ff02092ba3424667a8fec00b6105f9466e6ed9b03c55c74701478c814b843b`.
Exact-candidate code/resource review and affected fixture regressions are complete.
The separately reviewed first TGT-003 refresh stopped before native playback;
its actual outcome and remaining work are recorded below. Final independent adapter assessment and owner M1 acceptance
remain pending. Windows stays P2/nonblocking; calibrated physical sync/rate
measurements stay never performed and outside scope with no mandatory later check.


## Refresh preparation and cleanup review

A separate scratch helper now prepares the same fixed metadata pair for a future
single native session. Its first refresh has now stopped before native launch. The
helper requires exact app/driver/runtime identities, the previously accepted
source/rendition and role origins, and a fresh evidence destination. It passes
private input only through stdin; no body or locator is retained.

Independent review approved the acquisition/privacy logic but found that its
original five-second shutdown allowance could interrupt driver-owned app cleanup
(TRF-R0-01). The repair gives cleanup fifteen seconds after the ninety-second
session deadline, starts an owned process group inherited by the app, and fails
on forced cleanup or remaining descendants. The original reviewer closed this
helper finding after 137 independent race tests/subtests, vet and zero-request
default/invalid-pin runs passed. Controlled subprocesses established child
termination, private-pipe completion and isolation from an unrelated process.
The original candidate and finding remain preserved. Exact helper manifest:
`a3bc2ed416038e0bb2957438b2c5dc6fd133b6094e768f3b8f177e3513385333`;
review: `ebf4715eb0ed5618c3f580438aa9ea57b263ca19548f7a053dd4364666e1547d`.
The final maintained driver's process ownership and descriptor handoff subsequently
passed integrated review before the separately recorded execution release.


## Implementation, current fixture regression, and first target attempt

The exact 172-file implementation candidate has manifest SHA256
`e3f5f0d6889bd89e0c64d1186b976c0b39b080bd7ea2df7986f499e5b5d13801`
and archive SHA256
`5c4a8604f2c655b59211a23a79c7600588ab315d3f9f3b2575599456d6690b82`.
The six-stage local check with race detection passes. Independent review found
TGT-R0-01: an uppercase/mixed HTTPS scheme could bypass explicit TLS verification
in the opener. The bounded repair uses the parsed scheme; regressions fail before
and pass after the repair. The original reviewer closed the finding and approved
the exact code, transport, resource ownership and helper/driver integration.
Review SHA256: `b505cf18b709d29d9facbb23216c19ed6528e5641bdeabb2124c9fcf40e3269b`.

The unchanged pinned runtime passed all six native cleanup scenarios, including
survival beyond the old delayed-crash window. After the owner exposed the desktop,
the complete fixture capability stage on 2026-11-13 passed all 41 implemented
assertions, with loading cancellation at 81.072625ms and playing cancellation at
73.527292ms against the 200ms bound. Cleanup was graceful with zero active handlers;
the actual app and 30 runtime images matched their pins. Preserve the earlier
wrong-working-directory cleanup failure and two occluded capability attempts
alongside the passing results. Fixture inventory SHA256:
`0579fe7951a3e55c2ed9c6a43ac1f9879c7dac51903ed36352ddc6038d2f7aa8`.
The two coarse not-tested rows contain several capabilities and are not blanket
waivers: owner-attested fixture audibility is separate evidence; calibrated
physical measurements are removed by ADR 0006; Windows is P2; real IME, target
playback, packaging and changed-candidate CI retain their individual status.

The independent execution-readiness review approved one fixed metadata pair and
one conditional native session, with exact source/binary/runtime pins and no retry.
At **2026-11-13 00:40:23 UTC**, that sole invocation made two requests, both returning
HTTP200/service-code0, then stopped with `frozen-rendition-or-origin-changed`.
The result is **UNKNOWN**, not a playback pass. Successful classification and
retained duration/quality/codec facts narrow the mismatch to the selected audio
ID and/or initial video/audio origins; the retained evidence cannot identify
which changed. No descriptor was released, no native player launched, and no
media locator was retained. Helper exit0 records successful result emission only.

| Current target capability | Result |
| --- | --- |
| Exact source/part and ordinary-source classification | Observed; prior TGT-001 interpretation remains scoped to its exact rendition/origins |
| Refresh matches the frozen rendition/origins | UNKNOWN; stopped |
| Target picture/progression and selected native tracks | Not-tested |
| Target pause/seek/rate/volume controls | Not-tested |
| Target cancellation and graceful close | Not-tested |
| Current controlled fixture regression | PASS; does not establish target playback |

First result SHA256:
`0bba33ca44ebb6c41cc182b258a32781392f5c8bc89a556fd82402f4967ea680`;
invocation SHA256:
`6af7862e8e4f9272b0417f9cf644bbc600ecf2712a8b40e1bdc4722d7f55c145`.
The independent reviewer accepted the correctly stopped UNKNOWN outcome:
`e4b2990a36c0e5c12754e721e55a78b1cafd31707bd1b5b6e9c8d781bf0d0fa1`.
TGT-AC-04 remains unmet. TGT-AC-05's code/check/review and fixture components are
satisfied; this handoff preserves the negative actual outcome without accepting M1.

The next bounded step is an independently reviewed diagnostic-only observation
that can retain the already defined normalized rendition/origin facts but cannot
create a descriptor or launch a player, even if they match. The old allowance is
exhausted. Any subsequent origin-policy change needs concrete observed evidence,
independent review and newly frozen inputs; no automatic repinning or wildcard
hosts. Final independent adapter assessment and owner acceptance remain pending.

All detailed evidence is local under run `20261112T144351Z-ac60cbfe2590`, including
`tgt002-candidate-r1`, `tgt002-author-r1`, `tgt002-review-r1`,
`native-regression-r1`, `tgt003-readiness-review` and the `tgt003-first-*` files.
These ignored artifacts are not backed up by Git. This documentation update is a
later source delta; it does not change the recorded code/test candidate identity.


## Diagnostic-only observation and proposed origin amendment

A separately assigned and independently approved diagnostic ran once at
**2026-11-13 00:46:30 UTC**. Its two fixed requests returned HTTP200, and the
unchanged classifier accepted the same source, part, quality, codecs, duration
and audio ID. In this fresh response, the video origin was
`https://upos-sz-estgcos.bilivideo.com:443`, matching the unchanged audio origin;
only the video-origin comparison differed. This does not identify which field
changed in the earlier discarded mismatch. The diagnostic has no descriptor or
native-launch path, and retained no media locator.

Diagnostic result SHA256:
`3abc0aef3819f202ef6fb60a8984a7eb40397933b91d386ab3e38b032e907763`.
The original reviewer accepted these fresh normalized facts; review SHA256:
`df1cd5cae18462faca95dcedc0271d80035ad9da8169732076a8cb20122ba338`.
Exact scratch code and observation assignment had prior independent approval,
with 127 independent race tests/subtests passing. All diagnostic evidence is
retained in the same run under `target-diagnostic-*`; its allowance is exhausted.

A narrow amendment is being prepared for independent review: permit the two
actually observed exact video origins, while retaining the single exact audio
origin and unchanged source/rendition/classification rules. The descriptor must
match its actual locator origin as well as an approved origin for that role.
No wildcard, backup-locator selection, URL rewriting, automatic repinning or
new endpoint is proposed. Native target evidence still requires the approved
amendment, updated paired policy/helper checks and a separate one-session release.


## Post-repair target continuation (2026-11-13)

The separately assigned post-repair macOS target invocation passed at
**01:58:00 UTC / 09:58:00 Asia/Shanghai on November 13, 2026**. It used the
independently approved diagnostic driver with the unchanged target app, refresh
helper and pinned runtime. All valid HTTPS media hosts and ports are allowed
under [ADR 0007](../docs/adr/0007-target-host-policy.md); no origin comparison
or repinning is involved. The previous UNKNOWN outcomes and failed version-2
attempt remain preserved in their original runs and [report 0023](0023-target-failure-evidence.md).

The fixed anonymous metadata pair returned HTTP 200 and admitted the same
BV1Zs411o7Ee / avid899574 / cid1445374 / page1 source, H264 quality32 and
AAC-LC audio30216, with the reviewed full 0..596624ms grant. Service expiry
remained unknown; local freshness was 90 seconds. This was a bounded 3.189-second
probe, not a full-duration viewing test or a claim about other renditions/accounts.

| Actual target capability | Result |
| --- | --- |
| Visible rendered picture and selected video/external audio | PASS; ready in 701.018167ms; no media capture |
| Native playhead progression | PASS; advanced at least 300ms within 2s |
| Pause stability | PASS; drift within 150ms across 500ms after settling |
| Seek within grant | PASS; paused position 3s within 133.977125ms |
| Volume and rate configuration | PASS; native volume 0.25 and rate 1.5 |
| Playing cancellation and disposal | PASS; 87.7575ms against 200ms; no ownership, all native counters zero |
| Graceful process/socket exit | PASS; zero exit, socket removed, no forced cleanup |
| Target loading cancellation | Not-tested; one open, no retry; separate current fixture coverage remains |

There are eight passing check rows and one not-tested row. The 103 sanitized
automation calls had no errors or dropped calls. The supervisor observed no
remaining owned process, and released its temporary idle-sleep assertion.
The helper's zero exit is corroborated by the actual driver result. No target
descriptor, signed locator or locator hash, native raw output, screenshot or
media recording was retained. This pass does not explain the earlier failure;
no causal claim about desktop locking, service, TLS, host or runtime is made.

### Candidate, checks and local evidence

Continuation run `20261113T014429Z-67ce12fdd21b` freezes spec 0008 and records
this single attempt. The tested maintained candidate has 184 files, manifest
SHA256 `decc6f9b0662cc757416b6100727ec75c31bbf3958a06995c97d5ecf8a8fd0b7`
and archive `a8107a65fc30b77af9c04982acece0454ee0a6431715b6f50533b20c761de477`.
Base HEAD `d846e667c537c06a1bcf100e6aac9486c29aa3d2` is dirty and is not the
candidate identity. All file hashes/modes, four binary/manifest pins and 30
runtime artifacts were checked before execution. Exact code approval, binary
pins, the six-stage full local race PASS and independent 40+57 driver tests
remain in report 0023. No maintained code changed in this continuation.

The current native fixture regression retains its separate 41-assertion PASS
with loading/playing cancellation at 99.368625ms/79.460375ms and graceful cleanup.
Earlier six-scenario cleanup closures and independently reviewed human
combined/separate-fixture audibility retain their own attribution. The local
synthetic canary at 01:48 UTC intentionally failed media loading and passed its
visible/disposed/graceful-exit readiness predicate; it is not playback evidence.

| Evidence under this run | SHA256 |
| --- | --- |
| `evidence/target-first/inventory.json` | `61ad8a4b9f018dc39926d997e8853a3adabd558148d5819535a03a970af3a9fd` |
| `evidence/target-first/result.json` | `4b453e30651166ecbf76047e101d79861efc8887a5a4c9976f11fe2d8e1e0ff2` |
| `evidence/target-first/target-first-invocation.json` | `1a30cdeefcd136ec621bd3c0696c7ab192076df0cedc4d3c1a4da4d1dcc1cb57` |
| `evidence/target-readiness-r1-review/target-readiness-r1-review.md` | `c1510926c1f2c357d2309361f827119daac8124b78d04128a18c45ab5ed6696f` |
| `evidence/pre-effect-verification/pre-effect-verification.md` | `76d7bc3b3f11de4de69c3f0d49186564850ed8ddddc872298c1248e9c0d95369` |

The local canary's activation-persistence failure and the later rejected
unsupported-phase activation are recorded honestly. The first canary result was
not repeated. Before the actual target request, supported validation-phase
activation event 7 was independently verified; POSTREPAIR-REC-001 was
prospectively closed. No earlier event was backdated or edited.

The original independent finding author approves the scoped target outcome and
technically closes **TGT-V2-001** as the current capability/graceful-cleanup
acceptance blocker. The original FAIL and unknown cause remain unchanged;
no diagnosed functional repair is claimed. Review
`evidence/target-outcome-review/target-outcome-review.md` has SHA256
`cc1cb1d1765247095c107afdcb7263df434bf0084e0d0c5ad73a46feb475e97a`,
with inventory `65f66f6565b3b112bb5d8d092ab4881ebb81cf7c9e406e0eb04acf600721822d`.
TGT-V2-002 retains its independent diagnostic-repair closure.

The separate independent **PLAY-AC-04 final technical assessment recommends
libmpv behind the existing plain playback boundary, retaining mygo for the UI**.
It finds the required macOS M1 technical gates satisfied under ADR 0005/0006,
with no additional required native test or unresolved technical finding.
The assessment audited exact historical-to-current source applicability, the
current fixture result, prior cleanup/selection reviews and owner audibility
statements rather than combining them into a fabricated single test run.
`evidence/final-adapter-assessment/final-adapter-assessment.md` has SHA256
`a933ae0b53285f3477962b332c8c5da1092ff92dd1af73eed62678ba9839ef28`,
with inventory `2830df0de366718669223221762354bb432f2f4e0131e6b718e8b39e6662e34d`.
Target human audibility and actual target loaded-image capture were not performed;
fixture audibility and loaded-image evidence keep their separate attribution.

Explicit owner M1 acceptance remains required. The provisional mygo preview
remains the default, with no M2/P0/full-shell work started. Calibrated physical
sync/output-rate measurements were never performed and remain outside M1;
Windows is optional P2. Real IME, packaging/signing and CI for this dirty
candidate remain not-tested. A later source manifest/archive identifies the
final descriptive-only documentation delta separately from the tested code.

All detailed `.agent/` evidence is ignored, local-only and not backed up by Git.
Preserve the run archives and reviews before deleting the checkout. No commit,
push, publication, runtime/dependency change or default-player switch occurred.
