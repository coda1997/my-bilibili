# One user-assisted web QR login, securely reusable after restart

Spec ID: 0023-p0-persistent-login
Status: proposed scoped account capability, 2026-11-17.

## User goal and delivery

The owner authorizes real扫码login and credential persistence for subsequent
P0 development, preferring one successful scan, then overlay/full-client work.
Provide a real native Go login window on this Mac, protected save, same-binary
restart verification and reusable account/storage adapters. Leave real saved
credentials intact. Full client integration/overlay follow; this is not complete
P0 or a signed public release. No commit/push/PR/merge or remote messaging.

Use pinned mygo and its existing Secrets API, with stable Keychain service
com.coda.mybilibili.account and key web-session-v1 across initial login and later
client integration. No cookies/passwords/tokens/QR payloads in files, stdout,
reports, screenshots, automation trees, environment variables, CLI arguments or
memory notes. Native owner-only QR presentation is permitted, via Cindy's
request_input_handoff targeting the login window/pid, not a task screenshot.

## Boundaries and actual paths

- internal/account owns plain contracts/shared-contract.md, state machine,
  generation/cancellation, saved/unsaved/restore lifecycle and tests. No UI,
  platform, Bilibili DTO or HTTP imports. Add account-root dependency verification
  in internal/architecture/account_imports_test.go; preserve existing checks.
- internal/adapters/bilibili/account owns fixed request/response/cookie conversion,
  version1 opaque credential encoding, request isolation and safe errors.
- internal/adapters/credentials owns mygo Keychain operations and their safe
  error mapping. Store.Save confirms readback equality in memory; no value/hash
  is emitted. Get absent is distinct from failure. Bound all values to2560 bytes.
- internal/desktop/account owns native state presentation and in-memory QR bitmap;
  it consumes account.Controller, never cookie/HTTP DTOs.
- cmd/p0-login is the real composition root/developer window. Normal binary has
  no automation plugin or synthetic-account mode. A separately built e2e variant
  uses only fakes and may expose synthetic UI automation; it cannot reach live
  gateway or real Keychain even if supplied production launch flags.
- cmd/account-storage-smoke exercises only a generated synthetic service namespace
  and fixed synthetic material in two processes; no real service/key selector.
  Tests cannot access native storage by default. UI native test driver lives in
  tagged cmd/p0-login tests and runs only with explicit synthetic-binary path.

Select small MIT utility github.com/skip2/go-qrcode at
v0.0.0-20200617195104-da1b6568686e (commitda1b6568686e89143e94f980a98bc2dbd5537f13).
Its source/license were inspected in .agent/p0-login-research/result.md. This is
not a Bilibili SDK; no service implementation/schema is copied. Pin go.mod/go.sum
and retain its MIT notice at docs/licenses/go-qrcode.txt. New(.Medium) and
Bitmap/Image integer scaling are used only in memory, border enabled. Do not
write QR image files or introduce native FFI/custom crypto as a fallback.

## Service and credential contract

Gateway.Begin GET https://passport.bilibili.com/x/passport-login/web/qrcode/generate.
Require explicit code0, data.url HTTPS on exact passport.bilibili.com or
account.bilibili.com, no userinfo, <=2048 bytes; qrcode_key exactly32 ASCII hex
characters based on pinned web implementation. URL query must contain exactly
one qrcode_key equal to the returned key. No URL navigation/SSO. Attempt privately
owns key; UI gets opaque QR presentation only. Poll fixed passport web qrcode/poll
with encoded key; require explicit top-level and nested codes.86101waiting,
86090scanned,86038expired,0complete; unknown/errors fail safely, no endless retry.

One serial polling attempt: total180s from Begin,2s between polls, max90 polls;
every HTTP request/body10s and decoded1MiB cap. Use normal TLS, dedicated HTTP/1,
Proxy=nil, no redirects or transparent connection-reuse retries, no ambient jar.
Retain existing known public UA my-bilibili-p0-feasibility/1 and Referer
https://www.bilibili.com/. No alternate endpoint, app keys, verification bypass
or third-party-cookie acquisition. Unexpected service responses remain safe errors.

On poll complete parse Set-Cookie with net/http. Require nonempty valid SESSDATA;
retain only SESSDATA, bili_jct, DedeUserID, DedeUserID__ckMd5 if present. Reject
ambiguous duplicate known cookies, control/invalid cookie values, invalid known
cookie deletion/expired values and values >1024bytes. Each retained known cookie must explicitly have Domain=bilibili.com or
.bilibili.com (ASCII case-insensitive, one optional leading dot) and Path=/
exactly. Reject absent/host-only domain, passport-only domain, trailing-dot/
other domain, absent path or non-root path before publishing Material or saving.
Do not widen server-issued scope to reach nav. No copying to media/confirmation/
cover origins.
Retain values exactly (no query-unescape/re-encode). Unknown cookie names and
refresh_token/cross-domain URLs are discarded, not logged or persisted. Automatic
cookie refresh is not implemented or promised; stored sessions are verified.

Credential blob is version1 JSON owned by the account adapter: explicit
version=1, scope="bilibili.com/", and ordered known-name/value/expiry records
(zero expiry means server session lifetime unspecified), whole encoded size
<=2560bytes. All records inherit that validated fixed parent-domain/root-path
scope; unsupported scope is never serialized by the converter. Require exact
version/scope and allowed schema on restore; missing/unknown scope, extra
scope-changing fields, bad shape/duplicate/name/value/bounds are rejected before use; no silent empty result or overwriting
unknown data. HTTP header attachment is only to the fixed nav request on
api.bilibili.com, never a caller-selected URL. Verify GET /x/web-interface/nav:
explicit code0 and isLogin=true required. code-101 or explicit false means
ExpiredSession; network error is distinct and never deletes stored material.
No profile/name/id is exposed in reports; the UI may simply say logged in.

## Async and native lifetime

Follow exact shared-contract.md. One Manager operation worker; no lock held
across gateway/Keychain work; UI polls copies of safe State and renders only on
UI thread. Generations invalidate late results before Save/publication. No
new Begin while old worker is active. Keychain methods have no cancellation
parameter; Saving/Restoring/SigningOut may wait for OS consent, with responsive
window and explicit status. Normal Close/Quit is gated on completion and an
explicit unsaved/delete-failed disposition. No claim that a Go deadline cancels
native writes. Serial Save cannot race with a newer logout/login.

Successful poll material survives save failure in memory, offering RetrySave
without QR regeneration. Save/readback happens before nav, so a transient nav
failure still leaves the newly scanned material securely restorable. Existing
valid saved sessions restore before any attempt to generate QR. Expiry is shown
as such; no auto-reset/auto-delete to get a new challenge. Failed delete is
prominent; no claim that storage was cleared. Explicit close acknowledgment may
leave saved data; user must never be told failed logout succeeded.

## Native UI and user handoff

One standard resizable macOS window (title My Bilibili — 登录; initial440x640,
minimum400x560) with scrollable content when needed. System theme/text/controls,
body>=13pt, action hit area>=28pt (target36pt), explicit Chinese status/action
labels and keyboard navigation. Quiet white QR border and black modules in both
themes; integer module scale fitting<=320DIP, no distortion. Cache only current
in-memory bitmap; clear on scan completion/expiry/cancel/failure/close. Never
include URL/key in accessibility label or automation metadata. No moving QR.

State copy: 开始扫码登录; 等待扫码; 已扫描，请在手机确认; 正在保存登录;
登录已保存; 正在恢复登录; 已恢复登录. Distinguish storage retry, network recheck,
expiry/new scan and explicit logout. Native close during unsaved failure offers
重试保存 or 放弃并关闭; failed logout offers 重试退出 or 保留登录并关闭.
Saved success closes without logout. Login text explains storage in this Mac's
Keychain, no invented permanent session guarantee. App menu/keyboard Close/Quit
must follow the same lifecycle gate; destructive logout needs explicit confirm.

Guidance consulted via apple-design: accessibility/layout/typography/color/macOS,
accounts/privacy/buttons/windows/loading. Apply key principles: native window
controls, readable/theme-aware states, clear authentication method, visible loading,
and Keychain storage; no standalone HIG compliance claim from source alone.

Production flags: --begin-login starts one challenge only after restore finds
none/expired and owner is ready; otherwise restore existing session. --restore-check
only restores/verifies and exits with safe boolean/status receipt, never generates
QR. Flags mutually exclusive; no secret/endpoint/store-path overrides. Default
normal launch restores then offers user Begin. Status logs use allowlisted phases,
stored/verified booleans and fixed errors only, never raw State/credentials/QR.

Before generating the real challenge, complete tests/review and ask the owner's
readiness. Then open the exact normal bundle and hand its window/pid to native
Cindy request_input_handoff; complete_when text_appears=登录已保存 (also verify
safe application receipt, not tool completion alone). While pending, do not
inspect/click/type secrets in that window. Native QR is handed privately to the
remote owner; no screenshot fallback or localhost link on the wrong computer.

## Acceptance criteria

### LOG-AC-01

Ports/ownership are frozen; account transitive dependencies exclude UI/adapters/
service DTOs. QR utility version/license pinned. Fixed synthetic tests verify
secret redaction, clone ownership, state/action rules and allowed dependencies.

### LOG-AC-02

Synthetic HTTP cases prove generation/URL-key match, all poll states, successful
cookie conversion, malformed/expired/unrelated/host-only/passport-only/non-root
path cookies, exact accepted parent-root value round trip, session scope/restore,
origin isolation, no redirects/secret errors and bounded request/poll lifetime.
A scope-guard removal control must fail; unsupported scope produces no complete
Material and therefore no save. No external service or real credential needed
for go test.

### LOG-AC-03

Manager tests prove cancel/late results cannot save or sign in, store operations
serialize, save failure can retry without a new QR, network revalidation retains
saved credentials, close preserves saved state, unsaved close requires explicit
discard, logout failure remains visible, and no active operation publishes after
Done. Meaningful mechanism-removal regressions discriminate new guards/limits.

### LOG-AC-04

Synthetic native storage writes/reads/replaces through mygo Secrets, a second
process using identical binary restores, then deletes only synthetic items and
confirms absence. Fixed service namespace guard prevents smoke selecting real
storage. A system prompt/error is not a pass and never triggers security bypass.
Native calls may wait for owner OS input; do not kill a pending write and claim
rollback/cleanup. Record any unresolved exact synthetic item cleanup honestly.

### LOG-AC-05

Synthetic-only e2e build verifies normal window rendering, QR quiet zone/crisp
contrast, safe labels, actions, cancel/expiry/retry/saved states and graceful close.
Normal build has no automation import/socket and cannot use synthetic modes;
e2e cannot use real network/credentials. Native captures are synthetic only.
Default tests run no native window/Keychain. Native invocation has an owned
process, bounded RPC waits and cleanup evidence; platform onlymacOS is claimed.

### LOG-AC-06

Exact candidate passes go run ./cmd/dev check --race plus assigned tagged tests,
independent security/lifecycle/native UI review and synthetic native checks.
Only then obtain one user-assisted actual scan, securely save, verify nav and
same-binary restart restoration. Preserve real credentials; real logout/delete
is NOT a test step. Poll expiry/retry remains explicit user action, not an
automatic new challenge. No secrets in any receipt. If save fails, keep app open
and retry save, protecting the one scan. Proceed to authorized steps3/4 afterward.

## Task checks, risks and continuation

Use tasks.json DAG: root freezes shared types/dependency; independent workers
can then implement business/HTTP and native store/view on disjoint paths; root
integrates launcher and native checks. Mutable shared files are serially owned.
One module, no generic event bus/database/updater or service SDK. No signed release
or certificate install. Developer .app uses fixed bundle ID
com.coda.mybilibili.login.dev and stable Keychain service; ad-hoc rebuilds may
require another OS access prompt but must not delete credentials or demand a new
Bilibili scan unnecessarily. Public release signing is out of scope.

This is a new concrete account capability under the user's active authorization.
Retain global prior formal9/repairs5/invocations5 in the shared history. Bound
login task reviews to repository R4 (contract,code,response if needed), track
actual attempts and never reset a stopped loop. Real QR polling is a finite
protocol sequence, not an agent retry loop. Subsequent overlay/fullP0 need their
own frozen scopes; they remain authorized objectives, not done claims.
Required durable checkpoint reports/0035-p0-persistent-login.md. Controller owns
runs. Review independent of both implementing workers, original reviewer if
available. Any remaining failure is recorded with cause/evidence and next action;
keep user goal active. Rollback preserves the Keychain session and historical
reports; do not delete real saved login merely to revert source.
