# Bounded anonymous target-media playback probe

Spec ID: `0008-authorized-target-playback`
Status: amended by the owner on 2026-11-13 to remove target host restrictions
under [ADR 0007](../../docs/adr/0007-target-host-policy.md). Earlier architecture,
TGT-001 and TGT-002 approvals describe their recorded candidates. The current
host-removal implementation is tracked by [0009](../0009-target-host-removal/spec.md)
and [report 0022](../../reports/0022-target-host-removal.md). TGT-003 target playback,
final adapter selection and M1 acceptance remain incomplete.

## User scenario and goal

The owner requested completion of M1 and selected **Public test video** for
actual target-service validation. The controller inspected the normal signed-out
Bilibili page for `BV1Zs411o7Ee` (Big Buck Bunny 720P). The page displayed video
controls, login/register and a `试看30秒` item; later observation reported duration
597s, paused position 18.6s, and a 640x360 source. These observations establish
neither a currently effective 30-second limit nor permission to play the full
resource. No API/cookie extraction or challenge bypass is established by them. Subsequent
read-only network observation retained only hosts/statuses: 206 from
`upos-sz-mirrorhw.bilivideo.com` and `upos-sz-mirrorcos.bilivideo.com`, and 200
from `bvc.bilivideo.com`. No query, cookie, body or media locator was retained.
These historical host observations identify research candidates, not a
permission mode or a restriction on current media hosts; `__INITIAL_STATE__.videoData` and `__playinfo__` were unavailable.

Determine whether one currently site-granted anonymous rendition can play through
the evaluated macOS libmpv adapter, with required public metadata, separate tracks
where supplied, seek/cancel and complete cleanup. Inspect the permitted player
response before accepting a runnable descriptor. Honor actual preview, login,
entitlement and expiry restrictions. A preview result is labeled preview and
cannot establish full target access. Missing or restricted access is an honest
stop/needs-input outcome, not authority to find another access path.

macOS is required; Windows remains optional P2 under ADR 0005. The separate
0007 tooling remains unchanged. Physical audibility and final independent/owner
decisions remain required for M1; streaming media alone supplies none of them.
[ADR 0006](../../docs/adr/0006-m1-measurement-scope.md) removes calibrated physical
sync/output-rate measurements from M1, with no mandatory later check. These
measurements were never performed and remain not-tested.

## Scope and non-goals

- One selected public video/part, one explicitly granted rendition, and one
  bounded anonymous session at a time. No authenticated session or private media.
- Direct HTTPS media files from the permitted player response, with one optional
  external audio stream. Reuse the current pinned runtime if its actual build
  supports those formats/TLS capabilities; do not infer this from synthetic MP4s.
- Only the fixed public Referer
  `https://www.bilibili.com/video/BV1Zs411o7Ee/` and User-Agent
  `my-bilibili-m1-probe/0008` when required. An empty profile is also allowed;
  changing these values requires reviewed policy revision, not automatic retry. No Cookie,
  Authorization, Proxy-Authorization, arbitrary custom header, browser credential
  import, client secret, signed header or per-user identifier in header values.
  A service-generated expiring query locator remains private transient input;
  its presence does not authorize modifying signatures, duration or entitlement.
- A dedicated opt-in target-native command and bounded observation driver. The
  existing fixture commands, UI, server, fixtures, default validators and smoke
  stages remain unchanged. No forward proxy, remux, downloader or recording.
- No media resolution SDK, login, account, search, danmaku, quality switching,
  full shell, packaging, Windows loader, runtime rebuild, dependency change,
  global installation, public release, CI/check/workflow-policy change or remote
  repository action. Unexpected codec/transport requirements return to planning.

## Preflight: access and actual runtime feasibility

TGT-001 is serial and precedes implementation. Read the actual granted player
response through an authorized ordinary browser session, without obtaining
account cookies or defeating restrictions. If necessary, inspect one bounded
anonymous public metadata request used by the normal player for this selected
content; its endpoint/method/parameters and no-credential behavior must first be
identified and included in the independent preflight assignment. Do not invent
endpoint/signing behavior or expand into a generic resolver. Keep response
bodies/transient locators out of retained logs; return only normalized nonsecret
grant/rendition facts. No separate arbitrary endpoint discovery is authorized. Record only nonsecret source identity,
permission classification, time and the reason it is considered preview/full.
A duration, page label, playable cache or working direct URL alone is insufficient
proof of permission. If the response is ambiguous, restricted or lacks a usable
direct rendition, stop with needs-input. No endpoint signing/reverse engineering,
alternate account, client identity or repeated restriction retry is authorized.

Prepare a transient descriptor only when its permission/provenance is clear. A
preview must be the actual granted preview representation. Do not fetch a full
restricted resource and pretend a client-side 30-second timer makes it permitted.
If the permitted response relies solely on an unenforceable client-side preview
boundary, this direct-stream probe cannot represent it: remain browser-only and
report the limitation. Full content may be tested only if the response actually
grants anonymous full playback.

Inspect the pinned mpv/FFmpeg options and actual runtime build for HTTPS, normal
certificate verification, MOV/MP4/container and video/audio codec support, public
headers and redirect semantics. Explicitly set `tls-verify=yes` for remote sessions before native initialization
and fail closed if that setting cannot apply. Local source inspection found
`HAVE_LIBCURL 0` in the current build, `tls-verify` default true in network.c,
and forwarding of `tls_verify` to FFmpeg in stream_lavf.c. The documented
`curl-max-redirects` option is not a usable control for this build. These source
facts are not executed TLS/redirect evidence.
Use controlled TLS/redirect/negative checks before the selected target session;
no ordinary Go test may need a public service. The assigned TGT-001 worker may author bounded ignored scratch Go/native
feasibility probes using the pinned existing purego module and controlled TLS
servers. These probes cannot change maintained source, fixture validation, the
runtime build or global trust configuration; preserve their exact source and
sanitized outcomes for independent gate review. They do not consume target URLs.
The feasibility report records what was inspected versus executed and unresolved inputs. Independent review of
this gate is required before TGT-002; a failed/unknown gate leaves it unstarted.

### One bounded observation correction

The first independently assigned anonymous metadata request returned HTTP200 /
service-code0, quality32, duration596624ms and DASH representation counts, but
the safe parser discarded nested permission facts. Its retained result SHA256 is
`4d52c68a73f3336390b587f7aa094d802157ba5292f2d8c5e2ecce187c7a3a25`;
source SHA256 is `de7127c3965baeaa21df438e648bffa2769d48fc181ed27833cc0f66b4a48e1d`.
That result remains UNKNOWN for grant; preserve it unchanged.

After independent review and a new freeze of this amendment, permit at most one
independently assigned correction request solely because that successful response
was incompletely observed. Use GET `https://api.bilibili.com/x/player/wbi/playurl`
with exactly `bvid=BV1Zs411o7Ee`, `avid=899574`, `cid=1445374`, `qn=0`,
`fnval=4048`, `fnver=0`, `fourk=1`, and the same fixed public Referer/User-Agent
defined above. Bound the total request to10s and decoded body to1MiB; verified
TLS, no proxy credentials, cookies, signatures, session/device values, redirects
or automatic retries. Do not retry service rejection, restriction, challenge,
transport failure or changed access/rendition. If this correction fails, is
restricted/unsupported or remains ambiguous, stop with needs-input. No third
metadata request, alternate endpoint/identity, credential use or native media
consumption is authorized by this correction.

Extract only the existing normalized public status/code/quality/duration/codec
and stream counts, plus the following fixed permission facts. For
`data.support_formats`, `data.view_info` and `data.play_conf`, retain container
presence/kind (absent/null/object/array/other). For at most64 support-format
entries retain finite integer `quality`, and `need_login`/`need_vip` only as
booleans or exact numeric0/1. In `view_info` and `play_conf`, inspect only the
direct fields `is_preview`, `is_trial`, `need_login`, `need_vip`, `is_paid`,
`has_paid`, `can_watch`, `can_play`; retain booleans or exact numeric0/1 with
their original kind, otherwise record missing/unrecognized without the raw value.
There is no recursive extraction or inference for other/nested fields. Do not
retain strings, arbitrary field names, identifiers, tokens, session/device
values, URLs, headers, messages, error bodies, locators or locator hashes.

Before the one correction, offline extractor tests must cover synthetic
full/preview fields, missing/wrong kinds, unknown nested keys, numeric identifiers
and URL/token/message strings. Only the fixed allowed facts may emerge; every
outcome remains UNKNOWN until independent interpretation. Rendition
`need_login=false` or `need_vip=false` alone does not establish full-length
permission. This amendment changes only the observation allowance, retaining
the serial task graph and independent grant/runtime/M1 gates, subject to the
subsequent ADR 0006 removal of calibrated physical measurements only.

### Ordinary-public interpretation amendment

The two prior observations remain UNKNOWN and consumed their original allowances.
Read-only source research found no basis for requiring the guessed positive
permission fields in every ordinary response. The proposed
[access method](access-method.md) replaces that requirement with a narrow
combined inference from exact source identity, explicit nonpay/nonexclusive
classification, anonymous returned rendition and absence of conflicting
restriction evidence. The method records its third-party-source limitation;
it is not an official service permission guarantee or a general resolver.

After independent amendment approval and a new frozen assignment under the
owner's selected public-video M1 task, allow exactly the two bounded observations
in that method: one fixed anonymous view metadata request, followed only on an
eligible result by one request to the previously selected playback endpoint.
This explicitly replaces the no-third-request rule only for that named pair;
it grants no retries, endpoint alternatives or native media consumption.
Unknown, restricted, preview or conflicting results stop before dependent work.
No old observation is reclassified. Keep the public headers, bounds, private
locator handling and controlled-runtime evidence described above.

The controller may assign a bounded scratch Go classifier/extractor with offline
synthetic tests before observation. The method's field/type/veto rules govern it;
tests prove those local rules, not service permission. TGT-001 independent gate
review must accept the observed source/rendition facts and interpretation
before TGT-002. Host identity is unrestricted under ADR 0007 and requires no
per-origin review. Discard preflight locators after interpretation; the report retains
only normalized nonsecret evidence. A later fresh TGT-003 descriptor needs its
own explicit reviewed refresh assignment; this pair does not authorize it.
Record this continuation in `reports/0021-target-access-method.md`; report 0019
and all frozen inputs/outcomes remain unchanged historical evidence.

### URL and redirect scope

The target composition retains its reviewed source/part, rendition and grant
policy independently of stdin. Descriptor-declared source/grant cannot expand
that policy. All syntactically valid HTTPS media hosts and ports 1–65535 are
allowed: no allowlist, host-class exclusion, role-specific origin or comparison
with previous observations. This includes domains, single-label/localhost names
and IP literals. Host changes need no repinning or separate approval.

`targetprobe.ValidateMediaURL` validates syntax at the maintained policy and
refresh-helper boundaries without DNS resolution or network access. Reject
userinfo, fragments, non-HTTPS initial URLs, empty/invalid URL syntax, invalid
ports, control characters, malformed escapes and oversized locators. Keep raw
signed query bytes unchanged; do not canonicalize or reconstruct the locator.
Descriptor version 2 carries no `origins` field. Public results omit media hosts.

The probe uses the pinned native transport's standard redirect/DNS behavior.
It does not impose per-hop hostname, IP, DNS-rebinding or private-network
restrictions. Only fixed public Referer/User-Agent may travel through native
redirect handling. TLS verification stays enabled, and invalid HTTPS certificates
must be rejected. Do not claim every redirect hop is HTTPS: the prior controlled
runtime probe recorded a possible HTTPS-to-HTTP downgrade. A known downgrade,
unexpected credential demand or restriction/challenge stops a target attempt.
A changed host alone is accepted and does not trigger a stop or review.

## Boundaries and frozen interface direction

`experiments/playback/targetprobe` owns descriptor parsing, immutable permission/
expiry policy and sanitized result types; it imports only plain playback
contract types, never UI or Bilibili transport DTOs. The target-native command
composes that policy with the existing controller and libmpv opener and owns its
minimal native view. The driver owns bounded process/input/evidence lifetime.
Do not import the existing `ui.View`: its `open` method intentionally validates
fixture-only input and its retry storage is not the remote-input policy.

Introduce the explicit plain seam `contract.InputPolicy` with
`Validate(Input) error` and `contract.NewWithPolicy(opener, now, policy)`; add an
optional `Policy contract.InputPolicy` field to `mpvprobe.Opener`. Existing
`contract.New`, nil policy, `Input.Validate` and default opener remain exactly
fixture-only. The target composition passes the same immutable policy to both
controller and opener. Both validate before disposal/native access respectively;
one layer must not rely on the other. The policy enforces existing mutual
exclusion, string/header count/size, duplicate/control/injection restrictions,
plus this descriptor's source/permission/expiry rules and shared HTTPS URL
validation. A remote policy rejects local paths and post-validation locator
replacement, while allowing all valid HTTPS hosts. A caller cannot swap locators or
headers after validation; clone inputs and descriptor state before async work.
No exported mutable global policy or permissive fallback is allowed.

Changes to this seam, permission interpretation or transport policy require
architecture review and new frozen inputs. TGT-002 owns all affected shared
contract/adapter paths serially. Existing method/measurement contracts, runtime
build and legacy tests are read-only; add regression files for the new seam.

### Transient descriptor and safe result

Use a strict version-2 JSON descriptor read from inherited stdin, not argv,
environment variables or an automation RPC containing media locators. Cap input
at 32 KiB and reject unknown fields, duplicates, trailing data and malformed
values. The descriptor contains source video/part ID, rendition label, grant
kind (preview/full), granted start/end interval, observation/provenance time,
expiry, primary/audio URLs and the fixed public header profile. There is no
origin declaration. Reject version 1 and the removed `origins` field. Each URL retains the existing 4096-byte bound; headers retain eight/
4096 aggregate bounds even though this profile allows at most two names. Missing
or ambiguous permission fields and missing local freshness deadlines fail before
native access. A local freshness deadline is not a promise of service validity.

The controller/operator assembles a truthful descriptor from the granted response;
the JSON is not independently authenticated permission evidence. Its source and
interpretation are reviewed at TGT-001. A self-declared full grant is insufficient.
No descriptor, raw URI, query, header value, native error body or media frame goes
into public logs, snapshots, screenshots, reports, CLI echo or test fixtures.
Record only source/part ID, preview/full disposition, nonsecret rendition/format,
app/runtime/policy identity, times, sanitized state/counters and outcomes. Do not
hash a signed locator as a substitute for permitted public evidence. Keep any
needed private transient material outside Git/controller report evidence and
remove owned transient copies at session end; Go memory zeroization is not claimed.

### State, expiry, cancellation and ownership

Use existing generation, copied-input, command validation, 10s readiness,
2s progression/seek and complete synchronous disposal semantics. Preserve the
replacement 200ms externally observed loading/playing cancellation bound, no
more than one active core, no stale completion, idempotent Close, callback/worker
quiescence and per-session library unload. Same UI/render/control thread ownership
as the approved adapter; no new native calls on arbitrary observer goroutines.

One session has a 90s wall-clock deadline or descriptor expiry, whichever is
sooner. Descriptor expiry is a local freshness upper bound at most 90s after acquisition,
and no later than a known service expiry. A service may expire an input sooner;
unknown server expiry is recorded, not invented or used to assert guaranteed
validity. An already expired grant is rejected. Check freshness
before open and every explicit retry. The supervisor cancels at expiry/deadline;
close remains the same disposal contract. Loading cannot bypass expiration.
Only operator-initiated retry of a still-valid, identical authorized input is
allowed; no auto-refresh, new URL resolution or repeated challenge retries.

Constrain seek and playback to the actual granted media interval; record original
service times and probe time mapping if a preview asset starts at a different
media offset. Disable seek when interval permission cannot be established. Observe
position and stop at the granted endpoint; this is a secondary playback bound,
not authority to fetch otherwise restricted bytes. Stop is bounded by the existing
poll/cancel behavior, so client timing alone never establishes entitlement safety.

The driver supervises only its owned app and caps connect/read/command waits,
overall session and cleanup. Use 3s RPC deadlines and a 5s graceful process-exit
wait after requesting close; forced reaping is recorded as failure, never graceful
cleanup. Transport 3s network timeout and existing 10s readiness failure remain;
network variability can yield an honest failure, not relaxed defaults. Return
sanitized Invalid/Unsupported/Canceled/Timeout/Media/Closed errors, never relabel
Timeout as Media. Authentication, expiry, missing audio and restriction failures
stop rather than dropping audio/metadata or selecting another rendition.

## Acceptance criteria

### TGT-AC-01

The access/runtime feasibility report identifies the one allowed source/part and
grant, exact runtime/build, TLS/codec/header/redirect behavior and limitations.
It contains no secret descriptors. Unclear permission, unsupported transport or
missing inputs produce a recorded stop, not successful target compatibility.
Independent gate review precedes implementation and any native target run.

### TGT-AC-02

Both controller and opener enforce the same immutable opt-in policy. Default
constructors and existing fixture tests retain remote rejection. New tests reject
local paths, credentials, malformed/oversized/expired descriptors,
header mutation/injection and altered post-validation inputs before native access.
Fixed public headers remain isolated per session; no sensitive header is accepted.
Previously unseen HTTPS hosts, IP literals, localhost names and valid ports pass
without origin declarations; source/rendition and copied-input identity still apply.

### TGT-AC-03

The dedicated native composition and driver preserve existing controls, generation
and complete lifetime semantics, descriptor/grant/deadline bounds and explicit
retry rules. Controlled tests cover spawn failure, invalid input, load/playing
expiry, interruption, missing audio, stale completion and forced cleanup failure.
No raw input or native body leaks into safe errors, state or retained evidence.

### TGT-AC-04

An authorized attempt records one complete, candidate-bound invocation with actual
macOS picture/progression, required public metadata, selected audio/video, pause,
seek within the grant, rate/volume configuration, cancellation and graceful close.
Each capability is pass/fail/not-tested. A missing audio source fails the
presentation. Observation is not a frame recording; no third-party media capture
is retained. Permission denial/expiry is a valid negative result, never PASS.
A preview result remains preview-only and does not establish full target access,
all renditions, account capabilities, physical output or M1 acceptance.

### TGT-AC-05

Full repository checks/race and independent exact-candidate code/resource/transport
review cover the new policy and native composition, with relevant unchanged
fixture/native regression evidence. A durable handoff preserves first failures,
exact source/runtime identity, granted scope and unresolved M1 requirements.
No default engine, old findings or Windows status changes. Retained audibility
requirements remain; calibrated physical measurement gates are removed by ADR 0006.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| TGT-AC-01 | Inspect actual permitted player response, pinned native source/build; controlled certificate/redirect/header/format checks; independent gate review | macOS runtime; authorized anonymous browser; controlled TLS endpoints | Sanitized grant and exact runtime/option feasibility report, honest unknowns |
| TGT-AC-02 | Parser/policy and dual-boundary fake-opener/native-API regressions; unchanged fixture tests | Offline Go tests | All-host HTTPS acceptance, failure-before-access, copied-input, expiry/header isolation assertions |
| TGT-AC-03 | Controlled lifecycle, expiry/cancel/retry, redaction and subprocess tests | Offline fakes/local fixtures, macOS later | Explicit timing/outcome/cleanup and safe error assertions |
| TGT-AC-04 | One bounded native permitted-rendition invocation; independently observed controls and cleanup | Actual macOS, approved public source descriptor | Sanitized capability/grant matrix; no recorded media/locators; first failures retained |
| TGT-AC-05 | `go run ./cmd/dev check --race`; affected fixture native checks; independent candidate review | Exact source/runtime on macOS; Windows P2 not-tested | Manifest/archive, logs, review, limitations and next action |

## Risks, rollback and open decisions

No runnable descriptor or actual native target execution exists at planning time.
The selected page observations do not settle its current anonymous grant. The
normal response may expose only a browser/segmented/unsupported rendition, require
login, or expire a descriptor before use; TGT-001 must report that rather than expand
into acquisition/authentication. Exact runtime TLS/redirect behavior and controls
remain to be established. All valid HTTPS hosts are allowed by owner decision, and native redirect/DNS
behavior does not impose destination restrictions. Source/rendition and transport
limits remain explicit in the independent review and handoff.

Calibrated physical sync/rate measurements were never performed. ADR 0006 removes
them and their numerical criteria from M1 with no mandatory later check; do not
infer a measured physical result from this target probe.
Final adapter decision needs a separately reviewed complete M1 assessment after
required evidence exists. RPL-003 and this probe remain interim evidence work.

Rollback removes only explicitly authorized target-probe changes, preserving
existing default behavior, user work and evidence. No commit/remote action is
implied. Shared policy contract changes require new reviewed/frozen inputs.

## Review and evidence

Use `assignment.md` for an independent architecture reviewer distinct from the
planner `/root/platform_scope_review`. The controller owns runs and exact candidate
archives/hashes; no planner or worker writes controller evidence. Implementation
requires reviewed/frozen inputs and an explicit bounded assignment. Proposed task
order: TGT-001 -> TGT-002 -> TGT-003. Stop after failed/unknown preflight rather
than execute dependent tasks. Historical preflight: `reports/0019-authorized-target-playback.md`; current
continuation: `reports/0022-target-host-removal.md`. Host removal is assigned and
frozen separately under 0009; it does not erase the earlier stopped observations.
