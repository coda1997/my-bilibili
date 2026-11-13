# Bilibili API and implementation references

Inspected on 2026-11-10 (Asia/Shanghai). Research only: no Bilibili endpoint,
real-account login or target media flow was exercised. A public source file
does not establish access permission, current endpoint behavior or production
suitability. No third-party code/protobuf schema has been copied into this
repository and no dependency has been added.

## Source selection and current status

### Original community API collection: historical status, not a live contract

[SocialSisterYi/bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
is archived as of 2026-01-30. Its default `deprecated` README states that
maintenance ended and the documentation/source were removed following a
2026-01-28 warning received by the maintainer. Record the maintainer's statement
as source status, not an independent legal conclusion. Do not describe the
project as an actively maintained documentation provider.

The similarly named Go SDK
[CuteReimu/bilibili](https://github.com/CuteReimu/bilibili) is also deprecated
and archived as of 2026-03-18. It is not a recommended new runtime dependency.
[Nemo2011/bilibili-api](https://github.com/Nemo2011/bilibili-api) is archived as
of 2026-07-06; do not rely on old recommendations calling it actively maintained,
or introduce a Python application layer here.

### Archived contributor snapshot: documentation reference only

[pskdje/bilibili-API-collect](https://github.com/pskdje/bilibili-API-collect)
identifies `master` as a contributor snapshot synchronized around
2026-01-24/25. The fork itself is archived as of 2026-01-28, not an actively
maintained successor. Its observed `master` reference was:

`cfc5fddcc8a94b74d91970bb5b4eaeb349addc47` (commit date 2026-01-22 UTC).

Use its source links below to locate historical behavior for validation, not
to promise current availability. The pinned
[license](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/LICENSE)
is Creative Commons Attribution-NonCommercial 4.0. No permission to copy code,
fixtures or protobuf definitions into this product is inferred. Reuse and
distribution need a separately resolved source/license/access decision.

### Go implementation reference: inspect, do not adopt automatically

[guohuiyuan/biligo](https://github.com/guohuiyuan/biligo) describes an unofficial
Go client. At inspection it was not archived; observed `main` was
`34abdd487479d1f72e7cf65238f70f55464a8c74` (2026-03-06 UTC). That is source
identity, not evidence of recent maintenance, endpoint success or security.

Useful focused files at that revision:

- [login.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/login.go): QR generate/poll and session-info calls.
- [video.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/video.go): details/parts and media-response shapes, including separate DASH tracks.
- [search.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/search.go): typed search entry points.
- [endpoint.go](https://github.com/guohuiyuan/biligo/blob/34abdd487479d1f72e7cf65238f70f55464a8c74/endpoint.go): endpoint responsibilities and request flags.

The inspected root tree contained no LICENSE file and the GitHub metadata
reported no recognized license. Treat licensing as unresolved: no vendoring,
source copying or SDK dependency adoption. A live-room `DanmuInfo` method is
not an implementation of recorded-video timed danmaku. Read source critically;
do not equate feature names or examples with this P0's full lifecycle contract.

### Additional Go references: recorded-video danmaku and older SDK shapes

[tamnd/bilibili-cli](https://github.com/tamnd/bilibili-cli) was not archived at
inspection. Observed `main`: `877663c6998eda2712025201ddbf8816c12d1c0c`
(2026-08-19 UTC), with an
[Apache-2.0 license](https://github.com/tamnd/bilibili-cli/blob/877663c6998eda2712025201ddbf8816c12d1c0c/LICENSE).
Its [danmaku.go](https://github.com/tamnd/bilibili-cli/blob/877663c6998eda2712025201ddbf8816c12d1c0c/bili/danmaku.go)
is a useful source-level comparison for CID lookup, segment requests, binary
response classification and normalized timestamps. The inspected function
collects segments before sorting/yielding; do not adopt that whole-video
accumulation as the bounded on-demand scheduler for our player. No live success
or reuse approval is inferred. Its download/crawl features are outside P0.

[iyear/biligo](https://github.com/iyear/biligo) is another historical Go SDK
reference: observed `v0` was `2b51a7f5ef332f20dbb0b04107ee6c1d5dc21f9d`
(2023-04-30 UTC), not archived at inspection. Its README warns about unstable
v0 interfaces, and the repository identifies GPL-3.0 licensing. Use it for
comparison only until current behavior and any reuse decision are reviewed;
its debug examples are not our credential/logging policy.

### Official platform reference: different scope

[bilibili-openplatform/demo](https://github.com/bilibili-openplatform/demo)
is official but archived as of 2024-11-06, and its README advises against using
that legacy repository. Observed `main`:
`331ed2d8918cbb8a3b069dec488129f19383e4b9`. Its examples primarily demonstrate
OAuth and video submission, not a demonstrated general desktop viewing API.

Start official access/capability inquiries at the
[official platform](https://openhome.bilibili.com/), rather than treating
community endpoint names as an official grant. This inspection does not prove
that official access can satisfy all P0 capabilities. An OAuth example is not
interchangeable with a personal web-cookie session. Do not embed an application
client secret in a distributed desktop executable or add a new auth backend
without an explicit architecture decision.

## P0 API candidate map

All paths below are historical candidates to revalidate through authorized,
bounded feasibility work. Hosts and authentication differ; do not send one
cookie/header bundle indiscriminately to all hosts. No signed example URLs,
live QR payloads or account credentials are included here.

| P0 use | Candidate host/path | Evidence / owned adapter |
| --- | --- | --- |
| QR challenge | `passport.bilibili.com` GET `/x/passport-login/web/qrcode/generate` | [QR reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/login/login_action/QR.md); account |
| QR status | `passport.bilibili.com` GET `/x/passport-login/web/qrcode/poll` | Same QR reference; account maps pending/scanned/success/expiry, owns poll key |
| Session check | `api.bilibili.com` GET `/x/web-interface/nav` | [Session info](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/login/login_info.md); account, signing metadata remains transport-private |
| Optional session renewal research | `passport.bilibili.com` GET `/x/passport-login/web/cookie/info` and related renewal flow | [Refresh reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/login/cookie_refresh.md); not a promise of permanent auto-renewal |
| Optional remote sign-out | `passport.bilibili.com` POST `/login/exit/v2` | [Exit reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/login/exit.md); local invalidation must still work offline |
| Video-only search | `api.bilibili.com` GET `/x/web-interface/wbi/search/type` | [Search reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/search/search_request.md); catalog; validate cookies/signing and safe challenge errors |
| Video details/parts | `api.bilibili.com` GET `/x/web-interface/view` or `/x/web-interface/wbi/view` | [Details reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/video/info.md); catalog normalizes BVID/AID and part CID |
| Media resolution | `api.bilibili.com` GET `/x/player/wbi/playurl` | [Stream reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/video/videostream_url.md); playback; preserve permissions and track/quality distinctions |
| Danmaku metadata | `api.bilibili.com` GET `/x/v2/dm/web/view` | [Metadata reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/danmaku/danmaku_view_proto.md); danmaku; protobuf metadata keyed by CID |
| Timed recorded-video danmaku | `api.bilibili.com` GET `/x/v2/dm/wbi/web/seg.so` or documented older `/x/v2/dm/web/seg.so` | [Segment reference](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/danmaku/danmaku_proto.md); danmaku; validate segment size, progress units and required session/signing |

Cross-cutting reference:
[WBI documentation](https://github.com/pskdje/bilibili-API-collect/blob/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/misc/sign/wbi.md).
Any permitted request signing belongs in the transport adapter, not UI or
business packages. A risk/verification response is a stop/error state, not
authorization to defeat a challenge, evade limits, spoof paid access or reuse
another application's secrets.

No endpoint is needed for device-local history. Metadata cover URLs and search
highlight markup are untrusted presentation inputs; bound image fetching and
normalize display text rather than executing arbitrary markup.

## Native UI and player reference

[egoist/mygo](https://github.com/egoist/mygo) remains a candidate, not an adopted
product dependency. Observed `main` during inspection:
`daf6341a2fe05f62dc8519e7cc215a90c757e8e0` (2026-11-10 UTC).
The older revision recorded in ADR 0002 remains historical and unchanged.

- [Video documentation](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/docs/ui/video.md) documents controls, media position/rate and main-thread use.
- [Public video open API](https://github.com/egoist/mygo/blob/daf6341a2fe05f62dc8519e7cc215a90c757e8e0/ui/video/video.go) takes a file, one URL or bytes, without header/separate-track/context parameters in those entry points.

These inspected interfaces explain why M1 cannot be replaced by an MP4 demo.
Headers, separate tracks, cancellation, fullscreen/danmaku composition and
actual target-OS behavior remain not-tested. Do not update the dependency pin
or declare support based on this documentation inspection.

## Recommended use of the references

Owner-confirmed direction on 2026-11-10: do not depend on a third-party Bilibili
SDK. Implement only the P0-required service adapters with Go's HTTP facilities
behind module-owned ports, subject to the access/license and native feasibility
decisions. Do not hide an SDK behind a wrapper or vendor it into the product.
Keep the chosen transport replaceable, make parsing/error cases deterministic
with synthetic fixtures, and preserve the exact inspected source revisions
when implementation contracts are reviewed. Reference code remains research
input; source/schema copying needs its own reuse decision.

This choice does not prohibit separately reviewed UI, media, protected-storage
or protobuf dependencies. It is an engineering direction, not implementation
authorization, a legal opinion or a claim that an independent implementation
avoids service terms, licensing or access risks.
Before service integration/distribution, obtain an explicit owner decision on
the authorized route and any needed permission/reuse review. If no permitted
route meets P0, return to product scope rather than working around controls.
