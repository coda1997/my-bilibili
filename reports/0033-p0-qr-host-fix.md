# QR confirmation-host repair verified; downstream danmaku observation pending

Date: 2026-11-17. Run: 20261117T035620Z-a9aeab71536f.
Result: the requested QR host repair is complete, independently approved and
verified through fresh QR generation plus pending polling. The overall five-step
probe exits1 because its final danmaku response fails parsing; that separate
observation is recorded below and is not reported as a successful full probe.

## Original goal, scope and delivered change

After [report0032's diagnosis](0032-p0-qr-diagnostic.md), the owner explicitly
requested fixing the cause. The selected [0021 contract](../specs/0021-p0-qr-host-fix/spec.md)
adds exact account.bilibili.com alongside exact passport.bilibili.com for returned
QR confirmation URLs. Generation/poll API endpoints remain on passport.
HTTPS, no-userinfo, JSON/type/parse behavior, key/URL limits, explicit-port and
lookalike-host rejection, safe errors and network/privacy policy are retained.
No path/query rule was invented from unretained response data.

A local decodeQR helper serves two real consumers: current conversion enables
the account host, while the existing diagnostic disables it so its serialized
PriorValidatorAccepts field retains the historical passport-only meaning.
The diagnostic's fixed expectations, schema and raw duplicate-field behavior
remain intact. Only probe.go, qr_diagnostic.go, two parity-reference lines in
qr_diagnostic_test.go and new qr_hosts_test.go change in the implementation.
CLIs, transport, danmaku parser, other tests, dependencies and historical records
are unchanged. No SDK/source/schema copy, policy/CI change or native UI work.

## Actual one-time service verification

Started 2026-11-17T04:02:26Z UTC (12:02:26 Asia/Shanghai), after code approval and
source/binary verification. Executed .agent/p0-qr-fix/service-probe --live once.
Exit1; stderr0 bytes. Exactly five fixed HTTP responses, no retry/fallback,
scan, real login, credential storage, returned-URL visit or media playback.

| Step | Actual result | Safe evidence |
| --- | --- | --- |
| Anonymous session | PASS | HTTP200, code-101, 259bytes |
| QR generation | PASS | HTTP200, code0, 245bytes |
| One pending QR poll | PASS | HTTP200, code0, poll86101, 119bytes |
| Fixed public video detail | PASS | HTTP200, code0, 2804bytes |
| First danmaku segment | FAIL: invalid-response | HTTP200, 1501bytes |

The required QR generation and pending-poll checks pass. No scan or sign-in
success is inferred from pending code86101. The report does not emit the chosen
confirmation host; account-host compatibility is established by the earlier
bounded diagnostic plus the causal synthetic regression, while this invocation
establishes fresh flow progression. No QR/key/body/header/cookie/text payload
is persisted or printed.

FEAS-OBS-002 records the danmaku parser rejection. Its specific failed predicate
and response format are unknown because raw data was not retained. No service
refusal, protobuf defect or root cause is guessed. This was the first reached
live segment step; it is not shown to have been introduced by this host repair.
It remains a separate investigation, with no automatic edit or extra request.

## Exact candidate and independent review

Base e35022c1218588b2ebacc4c5e69b5f4cd431a9fe; local branch
codex/p0-service-feasibility. Work remains uncommitted. Existing .claude/ is
excluded and untouched. No commit/push/PR/merge/release or remote issue action.

- Spec SHA-256: c0fc585f14eb4d28dd90da4b65006d21dfc9754210895873a44e85e870da8d6b.
- Tasks SHA-256: 048e084544a1a95330c228ac39d33d05ba87ef3de63f13d8fa9b3548665c6f33.
- Code manifest .agent/p0-qr-fix/code-manifest.json: ea62f59cc7a257284afab4e080fe95595ef74a7d907be4b2cc4c59d43286a3af.
- Code archive .agent/p0-qr-fix/code-candidate.tar: fd2db37f35233145ca7307eeb003d433774c289f256a200160e3fedb84974c57.
- Actually invoked binary SHA-256: 9f235d4c66354bbc851909bf7572779c11d3f2c9c3190407245353ec55359dec.

Implementer /root/p0_implement; original independent reviewer
/root/p0_native_research; controller /root. Architecture round6 and final code
round7 approve without findings. The reviewer independently verifies all310
snapshot files, four-path delta, guard/consumer preservation and actual tests.
This later live outcome/handoff is controller evidence, not an extra formal
review or independently performed service invocation.

## Done criteria and verification

| Criterion | Candidate-specific evidence | Result |
| --- | --- | --- |
| QRH-AC-01 | Account full scripted flow compiles and fails before repair, passes afterward; passport flow passes; disable-account control fails | PASS |
| QRH-AC-02 | Both-host protocol/userinfo/port/lookalike/type/size/parse/error tests and existing privacy/lifetime suites; unrestricted-host control fails | PASS |
| QRH-AC-03 | Fixed current/history expected outcomes, diagnostic schema/duplicate facts preserved; historical-current-policy control fails | PASS |
| QRH-AC-04 | Full race and independent review; actual QR generation/poll pass; later segment failure faithfully recorded | QR repair scope complete; overall live probe FAIL at downstream step |

Actual Go1.27.3 darwin/arm64 commands use GOTOOLCHAIN=local GOWORK=off:

- go test -count=1 -race ./experiments/p0/... PASS, independently repeated.
- Compiled before-regression failure and all three compiled negative controls
  retained. Reviewer independently repeats the three controls under race; these
  expected failures are causal evidence, not failed additional repair attempts.
- go run ./cmd/dev check --run 20261117T035620Z-a9aeab71536f --race PASS at
  event000003. evidence/check-468024196.log SHA-256:
  5e37da1edd196738cb64fa156825a81f05a6bc5d7c3879fd82e1b9284dab7f99.
- Code review evidence/code-round7.md SHA-256:
  5ff0487803d6ad539e14eabe79a4954744eb09bb2d43cbf4fad1364a89526549.

Observed vs inferred: current exact-host rejection was observed in the prior
safe diagnostic. A compiled synthetic before/after regression discriminates the
host addition, and fresh QR generation/pending-poll now pass. The entire five-step
probe does not pass; no cause or causal repair is claimed for its segment failure.
Contract relied on: frozen0021 two-host policy and historical diagnostic consumer,
with all unchanged 0019 transport/parser/stop constraints and 0020 output semantics.

Logs, manifests, sanitized live-report.json/live-receipt.json and reviewer
conclusions live in this controller run. Full archive, scoped diff and worker/
reviewer scratch are .agent/p0-qr-fix, p0-qr-fix-implementation and p0-qr-fix-review.
These are machine-local, not remote attachments or Git backups. No new remote
CI, actual sign-in/Keychain, native playback/UI, packaging or release check ran.
Windows remains optional P2/not-tested. Earlier passing CI is not applied here.

## Inventory, retained counters and next action

FEAS-OBS-001's host mismatch has an independently approved repair and successful
controller QR verification. Prior P0F-REV-001 remains independently closed.
No new scoped code finding/advisory. FEAS-OBS-002 is an out-of-scope investigation
note: HTTP200/1501byte danmaku response rejected, cause unknown. Destination is
the single .agent/p0-start/inventory.md and this handoff. No remote authority or
dedupe/search/filing is claimed; local follow-up remains pending selection.
No systemic follow-up warranted by the isolated host repair; no evidence makes
the newly reached segment response a regression caused by that repair.

Cumulative formal reviews7, repairs3 (prior fixture correction, signed-state fix,
this host repair), service invocations3. The repair's one verification allowance
is consumed; issues0/5. User-authorized continuations and original failed reports
remain preserved; no task/run resets these counts. No automatic further review,
repair or service retry is active under this completed scope.

The requested QR repair has no remaining action. Next proposed P0 investigation
is to classify the segment response safely and compare current recorded-video
implementations before selecting a parser/access change. Real interactive login,
protected restoration/Keychain, Chinese overlay/timebase and full P0 integration
remain planned work. This repair does not complete or accept full P0.
