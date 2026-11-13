# Provisional mygo Playback Preview

- Date: 2026-11-11
- Run: `20261111T002803Z-3eb554cdb1c1` (local execution evidence).
- Base: `f9998c6bb771ebe0e3b470d0a7087fee5cf8e600`, branch
  `codex/m1-macos-playback`, uncommitted. Base/dirty status is not a snapshot.
- Implementation aggregate:
  `1ee2c1296a484c7cddee8a0f1582aaa9d2f84aceabba1985d798cc92adc77589`.
  Algorithm/file coverage is recorded in run `evidence/implementation-repair-r1.json`:
  compact sorted JSON of path/mode/SHA-256 entries for modules, experiment,
  old/new selected specs and new ADR/issues; report/navigation excluded to avoid
  self-hash cycles. Full review archive still includes every source file.
- Exact final candidate: `.agent/review/provisional-handoff-final/manifest.json` and
  `candidate.tar`; hashes are attached to the run and independent review, not
  embedded recursively in this report. Ignored tools/runs/research/binaries
  excluded. Existing P0 planning is preserved context, not implementation.
- Execution: bounded macOS preview PASS; strict full native smoke FAIL.
  M1/M2/P0 acceptance and final replacement decision remain pending.
- Independent architecture review: approved before freeze. Original code
  reviewer independently closed PREV-CODE-001 on exact r1 and approved the
  bounded preview code/interim handoff. Final documentation-only snapshot is
  identified separately in the run; runtime/M1 disposition remains needs-human.

## Decision And Delivered Scope

The owner asked to continue temporarily with mygo, record its issues and
consider replacement later. [ADR 0004](../docs/adr/0004-provisional-mygo-playback.md)
records that decision separately from final M1 acceptance.
[Known issues](../docs/known-issues/playback.md) retain four open IDs:
`PLAY-KNOWN-001` synchronous UI-thread delayed-loading stall, `002` render-coupled
native error cache/rejected duration gate, `003` unsupported request headers,
and `004` unsupported separate audio.

The existing synthetic-fixture window now has sampled position/duration,
seek/continuous-volume sliders,0.5x/1x/2x presets and explicit last-valid-input
retry. Controls disable for unusable/loading sessions; seek also needs known
positive duration. Pointer interaction state is separate from sampled native
state, avoiding duplicate commands when the sampled seek lags a held slider.
Repair r1 tracks public primary-button capture, not bounds-limited Pressed,
including departure/re-entry while held. After release, sliders resume showing
native samples. Retry defensively copies
headers, preserves unsupported audio/metadata, does not overwrite on invalid
input, and uses normal close/cancel/new-generation ownership. No automatic retry.

The lab has a visible temporary-player warning, current volume/rate readouts
and scrolling. Existing source/diagnostic keys, title and e2e-only automation
are preserved. This is not a file-picker/full client shell or live Bilibili
playback feature. Contract/player/module/fixture files and historical M1
inputs/report/closed runs are unchanged by this continuation.

The driver adds explicit `-suite preview`, records its suite in JSON, and rejects
unknown names before app/fixture access. **Full remains default** and its
strict assertions/bounds remain unchanged. No upstream patch, hidden status
host, extra engine, decoder, proxy/remux, SDK or dependency change is included.

## Verification

Environment: macOS27.2.1 build26B131 arm64; global Homebrew Go1.27.3. Generated
8s640x360 H264/AAC fixtures only. No account/service/private media involved.

The tested e2e app is `.agent/m1/native-provisional-repair-r1`, SHA-256
`3878ddb917794abab537b335bb60c81441d3c1799626a5a25e61896641ef0eed`.
Both final author suites below use that same binary and maintained source.

| Scope / AC | Actual result | Evidence in current run |
| --- | --- | --- |
| PREV-AC-01 decision/four open issues | Implemented; r0 independent review found no additional issue here | ADR0004, issue register, architecture/code review |
| PREV-AC-02 sampled controls/disabled states/no feedback | r0 FAIL PREV-CODE-001; r1 independently CLOSED; headless race PASS and author native controls PASS | `outside-held-before-r1.txt`, `outside-held-after-r1.txt`, independent closure, preview JSON/captures |
| PREV-AC-03 copy/retry/invalid/unsupported/failure recovery/ownership | PASS headless; native explicit close/retry generation1 to2 and cleared control error PASS | UI tests; preview JSON (not native network recovery) |
| PREV-AC-04 explicit preview | Repair r1 PASS twice, exit0, active-session graceful app/socket/server teardown | `native-preview-repair-r1/result.json`, `native-preview-repeat-repair-r1/result.json`, `ui.png`, `preview-local.png` |
| PREV-AC-04 default full retained/run | Repair r1 FAIL, exit1 at known delayed-loading switch; native local/HTTP picture/control/Range stages PASS | `native-full-repair-r1/result.json`, `local.png`, `http.png` |
| PREV-AC-05 deterministic checks and independent review | Author/independent full race PASS; independent r0 request-changes, r1 bounded code approval/closure | run check logs, architecture review, `code-review-r0.md`, `code-review-r1.md` |
| Windows/target service/real IME/physical audio/actual speed/sync/packaging/signing/candidate CI | NOT-TESTED | No inference from compilation, injected Chinese or readbacks |

Preview JSON SHA-256:
`0b79e709d56aba39f33c35a5f2ec0b43d286228ef6e04a651a83a60efd87996e`;
repeat `e9e6a7e9cfe6f78a2ae4bd9a9a4516d3e91dc03799457f1212542b5ad5d95052`.
It observes actual synthetic colors, positive8s duration, at least300ms native
progress within2s, zero pause drift across500ms after200ms settling, paused
slider seek exactly4s within the original2s/500ms bound, volume.5 and all three
speed preset configuration readbacks. Config readback does not measure speaker
output or actual speed/A-V sync. Native retry is close/reopen, not transient
network recovery. App exits gracefully with an owned playing session; socket
is removed and server Active0. Only the owned app window is captured.

Full JSON SHA-256:
`5425a7e73513f1b1f597e2729b932986c2a66f7e09806bae0d9fd2c2a4712cfd`.
Its final error is `delayed loading before switch: automation response absent,
oversized or timed out`. Stats: Plain2/Delayed1/Range2/Active0. The failed run
forcibly reaps its owned app; it is not a graceful full-smoke success. Later
loading/playing-cancel/negative-route probes are not reached in this run.
Header/separate playback remains unsupported at the unchanged pin, not a
capability proven by the passing preview. No workaround is claimed.

Commands (from repo root, standard prerequisites/environment):

```sh
go run ./cmd/dev check --run 20261111T002803Z-3eb554cdb1c1 --race
go build -tags e2e -o .agent/m1/native-provisional-repair-r1 ./experiments/playback/cmd/native
go run ./experiments/playback/cmd/smoke -suite preview -app .agent/m1/native-provisional-repair-r1
go run ./experiments/playback/cmd/smoke -app .agent/m1/native-provisional-repair-r1
```

Both suites retain90s total,3s RPC and10s ready bounds; original full2s
progress and100ms externally observed cancel assertions are not relaxed.
See [experiment commands](../experiments/playback/cmd/README.md).

Author iteration failures `native-preview-r0` through `r3` are retained, not
overwritten: native geometry used `width/height` (decoder regression preserved
before repair); upstream `setValue` is text-only; batched relative keys raced
sampled seeks, while down/up in a single input RPC did not reliably span a UI
frame. An absolute held-pointer iteration passed once (`native-preview-r4`),
but its later repeat (`native-preview-final-r0`) failed strict seek observation;
that pointer injection is not retained as reliable native evidence. The final
driver focuses the actual slider and sends one supported key per observed
native update, under a single2s seek/1s volume deadline. It does not substitute
legacy command buttons or repeat a failing assertion until success. Both final
keyboard-suite runs pass. A headless regression caught repeated held
seek commands before retaining interaction state (`held-slider-before.txt`).
Final acceptance evidence is from the final keyboard suites, not a composite
of failed or earlier pointer attempts. All attempts remain in the run.
The original candidate's two author preview passes, independent preview pass
and full failures are preserved as r0 evidence. They did not cover captured
pointer departure and are not the repaired candidate's native evidence.

## Independent Review

Frozen spec SHA-256:
`490ba61ae01caab8952ad2999bd84aec1fc2265aced3441ef0d9382c38ab74b4`;
tasks SHA-256:
`e97ce75b882d47d354945ef1c7a4894fa5e6a4d6af47f8267d8607b942d187b7`.
The files retain their proposed-status wording as frozen inputs; the reviewed
approval and run freeze, not a post-freeze edit, record their actual transition.

Independent `/root/m1_architecture_review` approved the exact planning contract
before freeze with no actionable findings. Report `evidence/architecture-r0.md`,
SHA-256 `1f83404ec3be71b7d637e313e0f220803cfc8c48e874984aafb5d6d88df91260`;
planning manifest `37bf4063b03115436eaefab5ed2ee9f6375977a10e28474fe13a6266edda9f6f`.
That review did not approve unimplemented controls or native acceptance.

Independent `/root/m1_code_review` requested changes on r0 for PREV-CODE-001
(P2), despite independently passing the full race gate and keyboard preview.
Report `evidence/code-review-r0.md`, SHA-256
`dfe2a6c7c72bc4485f899aa8428212360da601f580010a679e9bf1ef2d695c15`,
binds full manifest `49b2745ec1d6006e8e38eae4903dc363c85327fb2a95eddc14439b7272125cea`
and the original implementation aggregate. Its overlay reproduction preserved
120 duplicate commands over two stationary frames after leaving slider bounds.
The bounds-limited Pressed query ended our retention before upstream capture
ended, violating PREV-AC-02. No other actionable finding was identified.

Repair r1 changes only the UI/test implementation plus this handoff. Maintained
tests first reproduced58 commands on vertical departure, then passed with the
public primary PressedButton capture query. Both controls cover held departure,
stationary/sample-updated frames, re-entry, inside/outside release, disabled
close/replacement and generation sample reset. Full recorded race and two fresh
native previews pass on the repaired binary; full retains the known failure.
The original independent reviewer CLOSED PREV-CODE-001 on exact r1, with no new
actionable findings, and approved only the bounded code/interim handoff.
Report `evidence/code-review-r1.md`, SHA-256
`f4b1f4273827e81e7231e1fffc080e91f186b4bfe96fcc25acc044e7e7009ac7`,
binds manifest `b048c59242e77f87e1c1ef6ab991824871fb39c798aa69b4700e2a09e7da4e52`
and archive `7d80c75b16f615bba0ddc491d383ac8ad50b01c560de7869b563806e0cd45935`.
Its original overlay and maintained regressions each pass with race/count3;
its independent full race gate passes and native rebuild matches the author
binary. The final snapshot changes only report/navigation facts, not reviewed
implementation, modules, frozen inputs or smoke/oracle logic.

Reviewer r1 native launches were occluded: preview and one separately named
unchanged exposure repeat exited1/not-tested with WindowHidden=true; no picture
or control pass is claimed for them. The reviewer's default full also ran hidden,
recording rendered cells not-tested, independent asset/Range/loading-lifetime/
native-negative probes and cleanup, and unsupported playback capabilities fail.
Scratch evidence is under `.agent/reviewer-scratch/provisional-code-r1/`:
`native-preview/`, `native-preview-exposure-repeat/` (both JSON SHA-256
`adceb4376738fd25abd10e80de306713783628ea907aee7906f4310897404f3b`),
and `native-full/` (JSON SHA-256
`f6985a2e05c24c0c74e12f30e2d1fcdd0afe716794d505a83d5e878575d94ba3`).
Those outcomes are separate from the author's source-bound exposed preview
passes and delayed-loading full failure, which the reviewer independently
audited. They are never combined into a passing full stack. Exposure varies;
do not infer a decoder defect or loop until a pass. Temporary use remains
owner-selected; no workflow/check/CI policy change or publication is authorized.

## Not Completed And Next Action

The preview increment does not complete M1, authorize M2/P0, or accept mygo as
the final player. Slow-loading UI responsiveness and the error-refresh design
gap remain open; headers and separate audio remain unsupported. Windows and
authorized target media still need runtime access and their original criteria.
Keep the old [M1 handoff](0002-playback-macos.md) as historical evidence.

The bounded preview code is independently reviewed. The owner can use the
generated-fixture lab (subject to exposed-window availability) and select a
separately scoped/reviewed
replacement probe when ready, keeping the plain adapter boundary. Arrange
Windows runtime separately; do not infer Windows success or quietly start a
full client shell. No commit/push/PR/publication was performed.

Raw evidence, binaries and candidate archives are local ignored files, not
remote attachments or a backup. Preserve required sanitized evidence in
access-controlled durable storage before deleting this checkout; no new
remote CI/artifact retention is established by this work.
