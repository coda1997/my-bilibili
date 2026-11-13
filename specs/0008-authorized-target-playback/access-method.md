# Bounded ordinary-public access classification method

Original method dated 2026-11-12; planner `/root/platform_scope_review`, integrated
by controller `/root`. Its bounded observation and interpretation were independently
approved as recorded in report 0021. On 2026-11-13 the owner removed host restrictions
under [ADR 0007](../../docs/adr/0007-target-host-policy.md); that supersedes the former
exact-origin gate. This document is not a service grant or a fresh request allowance.
The original research was read-only; execution evidence is recorded separately.
All three pinned source byte hashes below were independently fetched and verified;
raw source bytes/examples were not retained. The prior UNKNOWN outcomes remain.

## Evidence and limits

[S1] yt-dlp's own implementation, lines837–845, checks
`video_data.is_upower_exclusive` and the exact `试看` entry in
`play_info.accept_description` to identify a supporter preview. It does not
require a universal positive full-grant boolean. This is primary evidence of
that client's behavior, not an official service permission contract. Its other
request/signing/retry/download behavior is not proposed for adoption.

[S2] Community stream documentation describes quality, millisecond `timelength`,
`accept_description`, DASH tracks and rendition fields (lines142–159,499–540).
Its ordinary example has `view_info:null` and
`play_conf:{is_new_description:false}` (lines1111–1113, also1861–1863).
This refutes requiring the earlier guessed flags as universal schema. It does
not establish that every null/absent field grants access. Unexplained
`can_watch_qn_reason`/`limit_watch_reason` values are not authorization predicates.

[S3] Community view documentation names GET `/x/web-interface/view`, documents
public identity, paid/supporter classifications and part metadata. It says
visitor-restricted videos need login; its authentication heading also names
SESSDATA (lines5–29). Anonymous success is therefore a hypothesis to test, not
assumed endpoint availability. Relevant fields are `rights.pay`, `rights.ugc_pay`,
`rights.movie`, `is_upower_exclusive`, `is_upower_preview`, `pages`, and optional
`ugc_season.is_pay_season` (lines38–75,102–120,167–175,229–241).
Do not infer permission from `rights.download`, `copyright`, or undocumented
`rights.free_watch`, `arc_pay` and `ugc_pay_preview` semantics.

These are third-party implementation/community observations. The proposed rule
below is a deliberately narrow trusted-public-source technical policy, not a
universal legal/service grant guarantee. It requires architecture review and
fresh observation. It is not a reinterpretation of retained report0019 as PASS.

## Proposed two-observation assignment

After independent amendment approval and a new freeze, allow at most:

1. One GET `https://api.bilibili.com/x/web-interface/view` with only
   `bvid=BV1Zs411o7Ee`. This is a new, fixed metadata endpoint requiring explicit
   assignment; no endpoint discovery or fallback is included.
2. Only if step1 meets the ordinary-public branch below, one GET to the already
   reviewed `/x/player/wbi/playurl`, with exactly the existing seven parameters:
   `bvid=BV1Zs411o7Ee`, `avid=899574`, `cid=1445374`, `qn=0`, `fnval=4048`,
   `fnver=0`, `fourk=1`. Any identity mismatch stops; do not adapt parameters.

Both use the existing fixed public Referer/UA, verified TLS, no cookie jar,
credentials, signatures, session/device values, proxy credentials, redirects,
automatic retry or endpoint/identity switching. Each has the existing10s/1MiB
bounds. On failure, challenge, denial, schema ambiguity or changed source stop.
No Bilibili observation is authorized by this memo. The exhausted two historical
observations are not an allowance for these new requests.

## Exact classification proposal

The following type/domain requirements are local conservative policy choices;
source descriptions do not promise every field will always be present.

**View response.** Require HTTP200, integer `code=0`, object `data`;
`data.bvid` string exactly the selected BVID; `data.aid` integer899574;
`data.cid` integer1445374; a bounded `data.pages` array with exactly one selected
entry whose integer `page=1`, integer `cid=1445374`, string `from="vupload"` and
positive finite integer `duration` seconds match that first part. Require object
`data.rights` with numeric integer `pay=0`, `ugc_pay=0`, `movie=0`, and boolean
`data.is_upower_exclusive=false`. Missing/wrong kinds are UNKNOWN, never false.
Require integer `data.state=0`. Reject nonempty string `data.redirect_url` or nonzero integer `data.forward`;
wrong types are UNKNOWN. Do not follow source redirection. If `data.ugc_season` is absent, continue;
if present require an object with boolean `is_pay_season=false`. A paid value,
exclusive=true, or boolean `is_upower_preview=true` is outside the ordinary-public
branch; do not perform step2 under this assignment. Optional preview false is
not a positive grant, and mistyped optional classification facts are UNKNOWN.
If present, `data.is_chargeable_season` and `data.is_upower_exclusive_with_qa`
must be boolean false. If present, `rights.arc_pay` and `rights.ugc_pay_preview`
must be integer zero; their semantics are not used to grant access, and any
other value is unsupported for this narrow branch. Do not follow redirects or
change the selected part. Bound pages to 1–64 entries; duplicate selected parts
or malformed part entries are UNKNOWN.

**Playback response.** Require HTTP200, integer `code=0`, object `data`;
integer `quality=32`, integer `video_codecid=7`, positive finite numeric
`timelength` milliseconds, and an array `accept_description` of1–64 strings
of at most128 UTF-8 bytes each. Inspect strings transiently; retain only a boolean
trial-marker result and structural validity, not raw labels. Exact `试看` is the
source-supported marker; conservatively any string containing it vetoes the
ordinary-public branch. Missing/wrong description schema is UNKNOWN. A nonnull
`data.view_info` is unsupported restriction/interaction state for this narrow
method and returns UNKNOWN; absent/null is not positive permission.
`play_conf` and support-format guessed booleans are not required grant fields.
The existing allowlisted preview/login/VIP/can-play fields, if present in
`view_info`/`play_conf` or the selected support-format entry, remain vetoes when
they explicitly indicate preview, login/VIP required or playback denied;
unsupported types are UNKNOWN. Missing guessed fields have no positive or
negative permission meaning. No field is used to override a restriction.

Require object `data.dash` and a single selected matching video representation
from its `video` array (1–64 entries): integer `id=32`, string `codecs` with
recognized `avc1.` prefix, MIME `video/mp4` and a private direct HTTPS base locator.
From `audio` (1–64 entries), select the lowest numeric positive `id` among entries
with `codecs="mp4a.40.2"` and MIME `audio/mp4`; duplicate selected IDs or unsupported
codec suffix syntax are UNKNOWN. No request iterates quality/audio alternatives.
Validate string aliases (`baseUrl`/`base_url`, `mimeType`/`mime_type`) consistently;
conflicting values are UNKNOWN. No backup locator fallback. Keep only the selected
pair transiently. Validate HTTPS syntax with the same `targetprobe.ValidateMediaURL`
used by the player. All hosts and valid ports are allowed, including IP literals
and localhost; no exact-origin approval, host comparison or repinning is required.
Do not retain hostnames in public metadata.
Record codec/MIME only as recognized enums, not arbitrary strings. Unsupported
codec/container, absent usable track or locator is needs-input, not a resolver task.

Require playback `timelength` to agree with selected-part duration within a
fixed 2-second rounding tolerance (a conservative local consistency bound, not
an empirical service guarantee).
Duration is a consistency check after source classification and returned playback
provenance, never permission by itself. Any shorter/longer unexplained response,
trial marker, paid/exclusive fact or conflict yields UNKNOWN/preview-needs-input;
no full claim, media consumption or native descriptor release follows.

When every check passes, classify **ordinary anonymous full-rendition candidate
under the reviewed trusted-source policy**: coherent public/nonpay/nonexclusive
metadata plus successful anonymous playback data issued for the exact part and
rendition, with no observed restriction conflict. This combined inference is the
method amendment; no individual zero, null, success code or duration grants access.
Independent TGT001 review must accept the observed facts and policy interpretation
before TGT002. The policy must candidly retain the third-party-schema limitation.

## Preview, privacy and continuation

A trial marker identifies a potential preview, but these sources do not prove
that the returned asset is server-truncated or establish an enforceable interval.
Thus this smallest method does not authorize direct restricted-preview playback.
If observed, stop with preview-needs-input; a future bounded preview method needs
actual rendition/interval provenance. A client timer is not that evidence.

Before either request, reject duplicate JSON object keys at every depth, malformed
JSON, trailing data and non-finite/out-of-domain numbers. Cap nesting at 64;
ignore unrecognized fields only after structural validation. An unknown field
is never a permission fact. Response body bytes remain transient.

Offline synthetic tests must cover all eligible facts, absent/wrong fields,
identity mismatch, explicit pay/exclusive/trial, conflicting aliases/durations,
challenge/nonzero codes and URL/token/string redaction. Tests validate our rule,
not the external schema. Record normalized public identity/classification/types,
recognized rendition facts, observation time and stop reason only. No raw bodies,
labels, messages, credentials, locator or locator hash in logs/reports. Preserve
existing freshness/expiry and native transport limitations; do not invent expiry.

The finite observation may still return UNKNOWN. If it passes, the fresh locator
will likely expire during implementation/review. Do not retain it long term or
promise a runnable later test from this allowance: discard it after bounded
private review. TGT003 will need a separately reviewed and explicitly assigned one-session refresh
allowance using this same reviewed two-step method after implementation approval,
with no retry on failure. This lifecycle consequence should be specified now.

No new signing, login, SDK, endpoint guessing, generic firewall, physical
measurement gate or broader service assurance is proposed. The original observation allowances are exhausted. A new live session still needs
a fresh bounded assignment; host changes require no additional policy approval.
See [the current handoff](../../reports/0022-target-host-removal.md).

## Exact public-source provenance

- [yt-dlp] Repository `yt-dlp/yt-dlp`, file `yt_dlp/extractor/bilibili.py`, commit `e8de28e23c1ecb4a12b2c3dec188c07e998c412c`; SHA256 `b99ad995a016b11097cdadfb64bc6da55cd64129b76e7914eafbc7e45685ec33`. Source: `https://raw.githubusercontent.com/yt-dlp/yt-dlp/e8de28e23c1ecb4a12b2c3dec188c07e998c412c/yt_dlp/extractor/bilibili.py`.
- [stream-schema] Repository `bilibili-plugins/bilibili-api-collect`, file `docs/video/videostream_url.md`, commit `cfc5fddcc8a94b74d91970bb5b4eaeb349addc47`; SHA256 `1b7a476d2c6fbc89fa4cbe464541c05dc9815aa17a02e2b26b1a988b525616c9`. Source: `https://raw.githubusercontent.com/bilibili-plugins/bilibili-api-collect/cfc5fddcc8a94b74d91970bb5b4eaeb349addc47/docs/video/videostream_url.md`.
- [video-schema] Repository `bilibili-plugins/bilibili-api-collect`, file `docs/video/info.md`, commit `bae574a1a944554956e627026bd5107d2e8a3d13`; SHA256 `f217cbc92c1cdc6a55e60d7c5cca0f13f74842086581ba1e6d98040b5d25cd53`. Source: `https://raw.githubusercontent.com/bilibili-plugins/bilibili-api-collect/bae574a1a944554956e627026bd5107d2e8a3d13/docs/video/info.md`.
