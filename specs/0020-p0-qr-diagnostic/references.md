# Existing QR implementations inspected before the extra request

Inspected 2026-11-17 through read-only GitHub source/API calls. No Bilibili
endpoint was called during research. No third-party implementation, fixture,
schema or dependency is copied; below are independently summarized observations.
Repository activity/license metadata describes the inspected snapshot only.

## BiliPai: current web login and both confirmation-page origins

Repository jay3-yy/BiliPai, not archived, GPL-3.0. Inspected commit
1637adcbeaa70660183c4979e7c9f541e6186bbc (2026-11-16T18:25:57Z).

- [QR parsing](https://github.com/jay3-yy/BiliPai/blob/1637adcbeaa70660183c4979e7c9f541e6186bbc/app/src/main/java/com/android/purebilibili/feature/login/BilibiliLoginQr.kt)
  explicitly recognizes account.bilibili.com/h5/account-h5/auth/scan-web as
  well as the older passport.bilibili.com/h5-app/passport/login/scan. This is
  scanned-QR/confirmation parsing, not proof of our generation response.
- [API declarations](https://github.com/jay3-yy/BiliPai/blob/1637adcbeaa70660183c4979e7c9f541e6186bbc/core-data/src/main/java/com/android/purebilibili/core/network/ApiClient.kt)
  use web qrcode/generate and qrcode/poll, passing qrcode_key for polling.
- [Login view model](https://github.com/jay3-yy/BiliPai/blob/1637adcbeaa70660183c4979e7c9f541e6186bbc/app/src/main/java/com/android/purebilibili/feature/login/LoginViewModel.kt)
  takes response data.url for local QR bitmap generation and retains
  data.qrcode_key separately for polling; states 86101/86090/86038 represent
  waiting/scanned/expired. We do not adopt its logging, default-zero response
  models, other authentication methods or credential-storage choices.

## biligo: focused Go web protocol comparison

Repository guohuiyuan/biligo, not archived at inspection, no recognized license.
Inspected commit 34abdd487479d1f72e7cf65238f70f55464a8c74 (2026-03-06).

[login.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/login.go)
models URL and QRCodeKey separately; QRCodePoll sends qrcode_key. Its
[endpoint.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/endpoint.go)
uses passport web generate/poll GET endpoints and decodes the data field.
The inspected model does not constrain the returned QR page to the API host.
No SDK adoption, source copying or license permission is inferred.

## FluxDown: account-origin QR presentation fixture

Repository zerx-lab/FluxDown, not archived, AGPL-3.0. Inspected commit
68ff687915f6ccbafaff8b52703bfccc0e7e05ed (2026-11-17T02:58:27Z).
[qr.rs](https://github.com/zerx-lab/FluxDown/blob/68ff687915f6ccbafaff8b52703bfccc0e7e05ed/crates/components/src/qr.rs)
locally encodes supplied QR text; its synthetic rendering test uses an
account.bilibili.com/h5/account-h5/auth/scan-web URL. This corroborates existing
usage of that origin, not live API availability or our application's renderer.

## Routes deliberately not adopted

PiliPlus at 217fb54f51f6764176319bbdedab2a7522d75401 (2026-11-14), GPL-3.0,
not archived, uses TV/HD auth_code generation/polling in the inspected
[login controller](https://github.com/bggRGjQaUbCoE/PiliPlus/blob/217fb54f51f6764176319bbdedab2a7522d75401/lib/pages/login/controller.dart)
and [HTTP login](https://github.com/bggRGjQaUbCoE/PiliPlus/blob/217fb54f51f6764176319bbdedab2a7522d75401/lib/http/login.dart).
This differs from the owner's chosen ordinary web qrcode_key flow. No app
keys/secrets, TV route, auto-confirmation, challenge bypass or alternate login
route is adopted. BBDown is archived with default branch DELETED; it was not
used as a current implementation reference. PiliPala is also archived.

## Consequence for our diagnostic

Our 0019 validator accepts only a returned URL whose exact Host is
passport.bilibili.com. The API endpoint host and returned confirmation-page
host need not be the same in the inspected implementations. The account-origin
hypothesis is now source-backed but not verified for the failed live reply.
The extra observation therefore emits two fixed host-match booleans, for the
old passport origin and the source-evidenced account origin, without printing
any live hostname, URL, query, key or body. It does not relax the old validator
or assert an actual cause before observing the authorized response.
