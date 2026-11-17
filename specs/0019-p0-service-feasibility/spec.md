# P0 service feasibility: bounded anonymous access probe

Spec ID: `0019-p0-service-feasibility`
Status: proposed for independent architecture review, 2026-11-17.

## User scenario and goal

The owner requested starting P0 and explicitly selected ordinary web QR login
and read-only video/danmaku interfaces, with credentials eventually stored only
in the Mac Keychain on the client machine. Before implementing the full client,
establish which historical service contracts still respond as expected.
This first executable slice produces sanitized observations, not a login UI
or a claim that full P0 or its login prerequisite is complete.

M1 is owner-accepted; M2 PR #9 merged as ff27f1db5946eb9ebccbee8c3548a971dd112e85;
M3 is owner-accepted. PR #14 merged as 84b74351a177c4e2a66f2d108f9a3765ef3eb896.
The current checkout is the reviewed PR #14 source e35022c1218588b2ebacc4c5e69b5f4cd431a9fe,
not the squash commit; its preexisting untracked .claude/ is excluded.

## Scope and non-goals

Add an opt-in Go experiment and command for at most five sequential HTTPS GETs:
anonymous session status, QR generation, one pending QR poll, fixed public video
details, and the first recorded-video danmaku segment. Use the existing M1
target BV1Zs411o7Ee / CID 1445374. No scan is requested or QR presented by this
probe; no live account, browser cookies, saved credentials, playback URLs or
media are accessed. QR challenge/key remain memory-only and never enter output.

Keep the accepted fixture application, playback contracts, checks, CI, module
dependencies and historical P0 draft unchanged. No SDK, copied implementation
or protobuf schema, signature bypass, challenge workaround or endpoint fallback.
No retries. No product search, real sign-in, Keychain write, UI, native launch,
history, quality selection, packaging or remote mutation in this slice.

## Boundaries and shared contracts

Owned paths: experiments/p0/serviceprobe and experiments/p0/cmd/service-probe.
The package owns protocol conversion, injected HTTP transport for offline tests,
bounded observations and sanitization; the CLI owns flags, process deadline,
JSON output and exit status. Business modules do not import this experiment.
The package exposes Run(context.Context, http.RoundTripper) Report and plain
report/observation structs. Production transport is created privately by the
CLI or package; a nil injected transport selects a dedicated standard transport
with Proxy=nil and normal TLS verification. It never uses http.DefaultClient,
ambient cookie jars, browser profiles or credentials. Close idle connections
when the invocation ends. No goroutines or queues beyond net/http internals.

Fixed requests, in order (only actual requests count against five):

1. api.bilibili.com/x/web-interface/nav: expect explicit envelope code -101
   or code 0 with explicit isLogin=false. A logged-in result is unexpected and
   stops the probe; never report account identity.
2. passport.bilibili.com/x/passport-login/web/qrcode/generate: require explicit
   code 0, nonempty bounded qrcode_key and an HTTPS URL on passport.bilibili.com
   with no userinfo. Do not fetch, print or persist that URL.
3. passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=...:
   one poll only; expect pending code 86101. Scanned (86090), expired (86038),
   success (0) or any other code is an unexpected observation and stops the
   probe. Ignore/discard Set-Cookie without storing or printing it.
4. api.bilibili.com/x/web-interface/view?bvid=BV1Zs411o7Ee: require explicit
   success, matching BVID, positive AID and matching CID 1445374 in pages.
   Display data stays out of the report; no image requests.
5. api.bilibili.com/x/v2/dm/web/seg.so?type=1&oid=1445374&segment_index=1:
   assess the historical unsigned segment route; do not silently try WBI or
   XML routes if denied. Decode only the needed protobuf wire fields using an
   original bounded parser: repeated top-level field 1, entry field 2 progress
   in milliseconds, field 3 mode and field 7 UTF-8 content. Skip well-formed
   unknown fields of wire types 0/1/2/5; reject groups and malformed/truncated
   or overflowing varints. Never emit text, author identifiers or raw bytes.
   Ordinary text modes 1/4/5/6 are counted; other modes are counted as unsupported,
   not executed. Entries need positive mode and nonempty valid UTF-8 text.
   Omitted progress defaults to zero under protobuf scalar semantics; progress
   must be in [0,360000] for this first segment. Top-level state field 2 defaults
   to zero; nonzero state is unavailable/inconclusive even with entries present.
   An empty well-formed segment is inconclusive, not proof of working danmaku.

All requests use only fixed public User-Agent/Referer headers, no cookies or
Authorization. Redirects fail closed, including same-host redirects; there is
no alternate-host retry. Service host restrictions here protect account/API
requests and do not change ADR 0007 media-host policy. Every request has a 10s
deadline including body read; the entire command has a 60s context and honors
caller cancellation. Close every response body on success and all error paths.
Decoded response cap is 1 MiB for JSON and 4 MiB for danmaku; read cap+1 and
reject overflow. Bound QR key to 256 bytes and URL to 2048 bytes. Bound decoded
entries to 20000, individual text to 1024 UTF-8 bytes and entry messages to
4096 bytes; oversized/malformed data is a visible failed observation.

Reports are an allowlisted format: fixed step name, pass/fail/inconclusive/
not-tested, fixed safe category, HTTP status, service/poll integer code where
present, response byte count, total/supported/unsupported entry counts. Never
include arbitrary errors, headers, response strings, request URLs, QR material,
body snippets, text or native output. Report absent integer codes distinctly
from zero. JSON envelopes require explicit code fields; JSON null/missing data
cannot become success. Non-200 HTTP, transport/cancellation/deadline, service
rejection and parse errors remain distinct safe categories. Stop on the first
failed/inconclusive observation, append not-tested for later steps, return a
nonzero CLI status. A passing run establishes only these five observed contracts.

The CLI requires --live explicitly, accepts no endpoint/account/cookie flags,
rejects unknown/trailing input before network work, and writes the sanitized
report to stdout. Default go test never contacts external services.

## Acceptance criteria

### P0F-AC-01

Offline fixtures exercise all five protocol conversions and the exact fixed
request sequence; no implicit network call occurs without --live. Success,
empty segment, denied service, missing/null code/data, invalid identifiers and
malformed/oversized protobuf produce the specified truthful observations.

### P0F-AC-02

Synthetic hostile responses, transport errors, redirects and secret-bearing
headers/QR/body fields never appear in serialized output or CLI diagnostics.
Tests prove finite cancellation/body limits, body closure, no cookies or
Authorization on any request, and stopping without retries or fallbacks. Each
new guard/branch/limit has a discriminating regression or mechanism-removal
control; ordinary positive tests alone do not prove failure handling.

### P0F-AC-03

After architecture review, implementation checks and independent code approval,
execute one bounded live invocation on the owner-authorized route. Retain only
the sanitized report, exact candidate identities, time, command and exit status.
PASS, service denial and inconclusive results are all valid feasibility evidence;
only actual positive observations establish the corresponding service ability.
An unsuccessful observation stops this slice for a concrete next-step decision,
not an automatic alternative endpoint or additional invocation.

### P0F-AC-04

The exact code candidate passes go run ./cmd/dev check --race and independent
correctness/privacy review. A durable startup handoff maps current existing
code to the P0 roadmap and preserves remaining real-login/Keychain, native
Chinese-overlay/timebase, complete service integration and macOS end-to-end gaps.
No P0 delivery, new CI or native capability is claimed from this experiment.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| P0F-AC-01 | package/CLI tests with injected transports and synthetic JSON/protobuf | local Go, no external service | targeted test log |
| P0F-AC-02 | hostile sentinel, redirect, limit, cancellation, closure and guard-discrimination tests | controlled transport/loopback only | race log and named controls |
| P0F-AC-03 | one --live command, maximum 60s/five requests | macOS anonymous service session | sanitized JSON/exit code, no payload files |
| P0F-AC-04 | full check --race and independent review | exact source candidate | manifest, archive, review and report |

## Risks, rollback, and open decisions

Historical community API descriptions are research inputs, not service promises.
Before coding the parser, verify field/mode/route semantics against pinned
references in specs/0004-client-p0/api-references.md, recording source identities
without copying implementations or schemas. Service denial must remain visible.
The owner selected the ordinary web route on 2026-11-17; no policy bypass is
authorized. No new dependencies or source reuse are needed for this probe.

This experiment does not yet verify successful QR sign-in, session restoration
or protected storage; those need a subsequent reviewed interactive assignment.
It also cannot prove native overlay/fullscreen compatibility. Those prerequisites
remain explicit before freezing the full P0 implementation contract. Rollback
removes only the new experiment and its command; no user data is created.

## Review and evidence

Controller /root owns local runs. Native sub-agent review uses standing owner
delegation. R1-R9 apply: at most three formal reviews per task and three repair
iterations, stop on the third recurrence. No remote issues are authorized; all
findings stay in one local inventory, filing budget used 0/5. Architecture and
code decisions, exact counters and observations go in reports/0031-p0-start.md.
No commit, push, PR, merge, release or change to trusted automation policy.
