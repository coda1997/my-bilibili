# QR diagnosis: API host and confirmation-page host differ

Date: 2026-11-17. Run: 20261117T034241Z-8a75700a3e76.
Status: authorized research and single QR diagnostic completed. The current
reply is rejected solely by the old exact-host condition. Validator repair and
actual login are not performed by this diagnostic.

## Goal and authorized continuation

The owner approved one additional QR-only diagnostic and the proposed two
reviews, requiring existing GitHub implementations to be researched first.
This continues [the first probe](0031-p0-start.md), FEAS-OBS-001 and closed run
20261117T031441Z-f84114441569. All prior failure/review records remain intact.
No scan, poll, credential storage, full five-step retry or remote mutation was
authorized or performed. Cumulative budgets were extended, not reset.

## Research before the request

Pinned research and source limitations are recorded in
[0020 references](../specs/0020-p0-qr-diagnostic/references.md).

- [BiliPai QR handling](https://github.com/jay3-yy/BiliPai/blob/1637adcbeaa70660183c4979e7c9f541e6186bbc/app/src/main/java/com/android/purebilibili/feature/login/BilibiliLoginQr.kt)
  recognizes both account.bilibili.com and the older passport.bilibili.com
  confirmation pages. Its web login view model encodes data.url locally and
  keeps qrcode_key separately for web polling.
- [biligo login](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/login.go)
  and its endpoint declarations likewise separate the generated URL from the
  key/poll operation on the passport API host.
- FluxDown at 68ff687915f6ccbafaff8b52703bfccc0e7e05ed uses an account-origin
  synthetic QR rendering fixture, corroborating existing usage of that origin.
- Inspected PiliPlus uses a TV/HD auth_code flow; it was not adopted for the
  owner's chosen web qrcode_key route. No SDK, code, fixture, schema, app key
  or dependency was copied from any reference. License/source status remains
  in the research notes; source availability is not live capability evidence.

This research established the account-origin hypothesis before implementation.
The original reviewer independently corroborated the primary BiliPai facts in
architecture round 4, also before the extra request.

## Actual observation and conclusion

The sole extra invocation started 2026-11-17T03:49:35Z UTC (11:49:35 Asia/Shanghai),
after code approval and identity verification. Command:
.agent/p0-qr/qr-diagnostic --live. Exactly one fixed QR-generation request; no
poll or returned-URL visit. Exit0; stderr0 bytes. The diagnostic returned
pass/observed-format, meaning fact collection completed, not successful login.

| Observation | Actual result |
| --- | --- |
| HTTP / service code / response size | 200 / 0 / 245 bytes |
| data / qrcode_key / url types | object / string / string |
| Decoded key / URL byte lengths | 32 / 131 |
| URL parsed / HTTPS | true / true |
| Exact passport.bilibili.com host match | false |
| Exact account.bilibili.com host match | true |
| Userinfo present | false |
| Unchanged old validator accepts | false |

Observed vs inferred: all shape, size, HTTPS, parse and userinfo conditions in
the old validator pass for this reply; its exact passport-host condition fails.
The diagnostic calls the old validator directly on the same response data.
Thus the current rejection is established as the host condition, rather than
missing QR data, service refusal or malformed URL. The generation API remains
on passport.bilibili.com while this returned confirmation-page URL is on
account.bilibili.com. Those origins have different roles.

The original 03:30:47Z response body was deliberately not retained, so its exact
fields cannot be retroactively inspected. This fresh observation reproduces
old-validator rejection and establishes the current cause; it does not invent
facts about the earlier raw response. Actual URL path/query, challenge/key,
headers/cookies and body are not retained or emitted. Named hosts above derive
from fixed match booleans, not stored response-derived strings. No login,
restoration, video/danmaku service or native rendering success is claimed.

## Delivered code and exact candidate

Four new Go files implement a separate explicit-live diagnostic and tests:
experiments/p0/serviceprobe/qr_diagnostic.go, qr_diagnostic_test.go and
experiments/p0/cmd/qr-diagnostic/main.go, main_test.go. Existing fetch/transport
bounds and old validation are reused; all eight prior source/test files remain
byte-identical. The original CLI, parser, specs, report, checks and dependencies
are unchanged. The new report consists only of fixed enums, lengths, optional
booleans and old-validator acceptance, with absent versus false preserved.

Base e35022c1218588b2ebacc4c5e69b5f4cd431a9fe; local branch
codex/p0-service-feasibility. Work is uncommitted. Preexisting .claude/ is
excluded and untouched. No commit/push/PR/merge/release operation occurred.

- Spec SHA-256: 76af13ddf744321481bee2e5f5b65ab5af3714484e3f52258c8528e785edb630.
- Tasks SHA-256: a6ff3c07204d37fd89f1eea786bd7d262e23f6eb2a4bc12a97d9136d9b4cad2f.
- Code manifest .agent/p0-qr/code-manifest.json: 8ce96f2f05b758f1019fc3e8c5b8339c66bed42eab66dfadd321ab74981a6aae.
- Code archive .agent/p0-qr/code-candidate.tar: a4c1e005fb5418adfc42aca7e86a951f7f0dfd05f69c910e3e9326c587f6da6a.
- Actually invoked binary SHA-256: b0f579d565b687e00adb80a299e47380c017502e53570c78d63a765489119c12.

## Verification and independent disposition

Implementer /root/p0_implement; original independent reviewer
/root/p0_native_research; controller /root. Architecture round4 and code round5
both approve with no findings. The reviewer independently verifies all305
code-candidate files and confirms exactly four new Go files since architecture.
This later report/live observation is controller evidence, not a sixth review.

| Criterion | Exact evidence | Disposition |
| --- | --- | --- |
| QRD-AC-01 | Pinned GitHub research before implementation/request; primary source facts independently corroborated | Complete |
| QRD-AC-02 | Synthetic field/type/length/URL facts, raw old-validator parity, exact one request, privacy, cancellation/cleanup and CLI cases | PASS |
| QRD-AC-03 | Targeted race, full recorded race, five independently repeated failing controls, architecture/code approval | PASS |
| QRD-AC-04 | One authorized real observation, typed facts, exact binary/source/time/exit receipt and this report | Complete; current host mismatch identified |

Go1.27.3 darwin/arm64, GOTOOLCHAIN=local GOWORK=off. Actual commands:

- go test -count=1 -race ./experiments/p0/... PASS, including unchanged prior
  tests; independently repeated by reviewer.
- Five compiled source-removal/mutation controls fail the intended tests:
  explicit-live, account-host match, prior-validator parity, secret omission,
  single-request constraint. All five independently repeated under race.
- go run ./cmd/dev check --run 20261117T034241Z-8a75700a3e76 --race PASS at
  event000003, evidence/check-2601455444.log SHA-256
  95d6969a0b2b35c4f906ebc469fb484359f961cc715c5eaaa57baf79f0d11c7e.
- Independent code review evidence/code-round5.md SHA-256
  744f64ea317e02aaa60c7c6f7e7d4c62d38e44d14c769bd59a12a34118affcbc.

Logs, manifests, sanitized live-report.json/live-receipt.json and independent
reviews live under the controller run. Full archives/diffs and worker/reviewer
scratch are in .agent/p0-qr, p0-qr-implementation and p0-qr-review. These are
machine-local, not remote attachments or Git backups. No changed-candidate CI,
Keychain, actual login, native playback/UI, package or release check was run.
Windows remains optional P2/not-tested. No policy/check/CI changes.

## Retained scope, inventory and next action

Cumulative formal5/5, repairs2/3 (prior fixture plus signed-state repair), total
live invocations2: initial five-step probe stopped after two HTTP responses,
then this single QR request. Extra allowance1/1 consumed; issues0/5. No new
implementation failure, fix iteration or review finding. P0F-REV-001 remains
closed; FEAS-OBS-001 is diagnosed for the current response, not fixed in the old
validator. The common inventory .agent/p0-start/inventory.md retains its history.
No remote issue search/filing is claimed; local continuation is the destination.
No systemic follow-up warranted by this diagnostic-only addition.

The authorized research/diagnostic task is complete. Proposed next scoped work
is to revise the QR confirmation-URL contract using the source/observation above,
then implement and independently verify that reviewed acceptance policy. Keep
generation API, confirmation URL and poll key as distinct concepts. Do not
infer permission to accept arbitrary hosts or to scan/auto-confirm. Additional
review scope/allowance must preserve the cumulative history; this run grants
no further invocation or validator edit.

Found, not done: the old validator still rejects the observed account origin;
real web login, protected session restoration/Keychain, native Chinese overlay,
and full P0 integration remain the planned work in report0031. Full P0 is not
completed or accepted by this successful diagnostic. The original requested
one-extra-observation scope has no remaining action.
