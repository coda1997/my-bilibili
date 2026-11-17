# P0 QR response diagnosis after implementation research

Spec ID: 0020-p0-qr-diagnostic
Status: proposed continuation for architecture review, 2026-11-17.

## User scenario and goal

The owner authorized one additional QR-only diagnostic request plus the proposed
two independent reviews, requiring GitHub implementation research first. The
research is recorded in references.md and precedes any new Bilibili request.
Explain the QR-format mismatch observed in 0019 without collecting login data.

This continues FEAS-OBS-001 and run 20261117T031441Z-f84114441569, which is closed
needs-human. Previous code remains independently approved; P0F-REV-001 stays
closed. Before this continuation: formal reviews3, repairs2 (one fixture, one
product), live invocations1, issues0/5. Owner adds exactly two formal reviews
(cumulative4/5) and one QR-generation request (cumulative invocation2); no reset.

## Scope and non-goals

Add serviceprobe.RunQRDiagnostic and a separate explicit opt-in qr-diagnostic
CLI. Reuse 0019's fetch/isolated production transport, 10s request/body deadline,
1 MiB decoded JSON cap and safe Observation fields. Do not edit the existing
five-step Run, old validator, old CLI, protobuf parser, transport policy or
historical spec/report/run. No SDK or new dependency. No QR rendering, scan,
poll, account session, cookie access/storage, navigation, detail/danmaku/media
request, alternate endpoint, redirect or retry. No native launch or remote write.

## Boundaries and shared contract

Only new files experiments/p0/serviceprobe/qr_diagnostic.go and
qr_diagnostic_test.go plus experiments/p0/cmd/qr-diagnostic are writable by the
implementer. The new function accepts context.Context and http.RoundTripper
and returns a plain QRDiagnostic report. nil transport creates the same isolated
HTTP/1 transport; body and idle connections close as in 0019. Tests inject
controlled transports. No added goroutines, callbacks or UI state.

Exactly one fixed GET to
https://passport.bilibili.com/x/passport-login/web/qrcode/generate, with existing
fixed public headers and normal TLS verification. The CLI accepts --live only;
unknown/trailing input or absent/false --live makes no request and emits only
fixed usage. Entire invocation context is 15s; per-request/body cap remains10s.
No output may include arbitrary error text, body/header values, key, URL, query,
path, hostname or user information.

The report contains Observation (Step=qr-format-diagnostic, existing safe
numeric/status/category fields) and optional facts, exposed only after a valid
JSON object envelope with explicit integer code0. Non-200, transport/cancel/
deadline/limit errors reuse existing categories. Missing/null/wrong-type code
or malformed/non-object envelope is invalid-response; nonzero explicit code is
service-rejected. Such failures have no QR facts and nonzero CLI exit.

A facts record describes expected fields using only:

- DataKind, KeyKind, URLKind: fixed enum absent/null/string/object/other. KeyKind
  and URLKind describe absent when data is not an object. Present scalar JSON
  null differs from absent. No arbitrary field names are enumerated.
- KeyBytes and URLBytes: optional integers, present even for zero length only
  when the corresponding JSON value is a string; bounds cannot exceed the
  bounded input. Strings stay local and never become report fields.
- URLParsed: optional boolean, only assessed for nonempty URL strings with
  at most2048 bytes. Absent for missing/wrong-type/empty/oversized strings;
  false for a URL parse error. No parser error string is exposed.
- HTTPS, PassportHostMatch, AccountHostMatch and UserInfoPresent: optional
  booleans only when parsing succeeded. Match u.Scheme==https, u.Host exactly
  passport.bilibili.com or account.bilibili.com, and u.User!=nil. These are
  observations, not expanded trust/validation. Never visit the returned URL.
- PriorValidatorAccepts: boolean computed by calling existing decodeJSON for
  QR generation with the same data, discarding the returned key. No duplicated
  substitute validator and no change to 0019 acceptance semantics.

For valid code0 JSON, successful fact collection returns Observation status
pass/category observed-format and CLI0, even when PriorValidatorAccepts=false
or expected data/fields are absent. This means diagnosis completed, NOT QR
acceptance or login success. Category semantics must be explicit in docs/tests.
Raw facts pointers distinguish unassessed from false. A malformed body never
becomes a successful diagnosis. Code/service fields preserve absence vs zero.

## Acceptance criteria

### QRD-AC-01

The pinned GitHub research identifies concrete web QR request/data/poll behavior,
contrasts API and confirmation-page origins, and separates TV/HD references.
Conclusions distinguish actual source facts from hypotheses about live service.
No third-party source/schema/dependency is copied.

### QRD-AC-02

Synthetic fixtures prove exact one-request behavior, explicit live opt-in,
missing/null/wrong-typed fields, string byte lengths, old/new/unrecognized hosts,
HTTP/HTTPS, userinfo, malformed/oversized URLs, and parity with the unchanged
old validator. Secret sentinels in QR/header/body/error fields never appear in
serialized report or CLI diagnostics. The two host booleans describe only
fixed known origins, never print a response-derived hostname.

### QRD-AC-03

Targeted race and full recorded go run ./cmd/dev check --race pass on the exact
candidate; independent architecture and code review complete. Meaningful
mechanism-removal controls discriminate the one-request/live gate, host/parity
facts and no-output-of-secret contract. Existing 0019 tests remain unchanged and
passing; no historical check, state guard or required gate is weakened.

### QRD-AC-04

After code approval, execute the one additional QR-only request, retain only
sanitized report, exact candidate/binary/time/exit receipt, and explain which
old conditions match or fail. If facts are inconclusive, label that explicitly.
No repeat/full five-step invocation, validation-policy repair or actual login
is authorized by this diagnostic. Record conclusions and next action in
reports/0032-p0-qr-diagnostic.md; preserve prior failure and cumulative budgets.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| QRD-AC-01 | Pinned GitHub source comparison in references.md | read-only GitHub | URLs/revisions and interpreted facts |
| QRD-AC-02 | New package/CLI tests and report sentinel checks | synthetic transport only | race logs and exact reports |
| QRD-AC-03 | Targeted/full --race, controls and independent review | pinned Go1.27.3 darwin/arm64 | candidate manifest, logs, dispositions |
| QRD-AC-04 | one --live QR-only invocation | current Mac, no account | sanitized report/receipt and durable handoff |

## Risks, rollback and review

References suggest the account origin may explain the old strict host failure;
that remains a hypothesis until observation. Other field/URL mismatches remain
possible. Richer diagnostics risk secret leakage, so output is structurally
restricted to enums/booleans/counts. Removing the newly added files rolls back
this diagnostic without touching user data or approved 0019 source.

Same independent reviewer /root/p0_native_research; controller /root owns runs.
Two additional formal rounds are authorized, cumulative4 and5. Preserve repairs2
and issue count0/5; no extra repair allowance or issue/commit/remote authority.
Any required scope/code change after the final disposition follows R4, without
resetting through a new run. New frozen inputs record continuation, not a budget
reset. Native/login/Keychain/full P0 remain unimplemented/unverified here.
