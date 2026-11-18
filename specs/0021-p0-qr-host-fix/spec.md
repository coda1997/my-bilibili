# Accept the observed web QR confirmation host

Spec ID: 0021-p0-qr-host-fix
Status: proposed scoped repair, 2026-11-17.

## Goal and authority

The owner explicitly requested fixing the diagnosed issue after report0032.
The current web generate API uses passport.bilibili.com, but its returned HTTPS
confirmation URL was independently diagnosed on account.bilibili.com. All other
old validation conditions passed. Keep these separate roles and accept the
observed confirmation host without accepting arbitrary hosts.

This authorizes the scoped repair, necessary contract/code reviews and ordinary
verification. Retain the same logical history: formal5, repairs2, invocations2,
issues0/5 before this repair. Controller bounds this requested continuation to
one repair (cumulative3), contract/code reviews (cumulative6/7) and one existing
five-step read-only verification invocation (cumulative3). No counter reset,
remote action, actual sign-in, scan, credential storage or native UI work.

## Scope and compatibility

Change QR generation conversion in experiments/p0/serviceprobe only. Accept
exact u.Host passport.bilibili.com OR account.bilibili.com with HTTPS and no
userinfo. Keep existing JSON code/data/type handling, nonempty key<=256 bytes,
nonempty URL<=2048 bytes, parse validation, duplicate-field behavior and error
categories. Explicit ports, subdomains/suffixes and unrelated hosts remain
rejected exactly as before. No new path/query rule: the actual path/query was
not retained by the diagnostic and this repair does not invent an observed one.
Never fetch/render/scan the returned URL; polling continues against the fixed
passport endpoint with encoded qrcode_key. Requests, deadlines, caps, cookies,
transport isolation, retries, media/danmaku parsing and CLIs stay unchanged.

Known consumer: 0020 QRDiagnosticFacts.PriorValidatorAccepts promises the old
passport-only rule. Preserve its JSON name and historical meaning, including
raw duplicate-field semantics; it must stay false for account-origin input
while current conversion succeeds. Introduce one local QR decoder shared by
current conversion and the diagnostic, with an explicit allow-account-host
policy argument. Move the existing case's validation into it without changing
other predicates or failure-key preservation. Current decodeJSON QR case uses
allow-account=true; diagnostic calls it with false. This is a real two-consumer
boundary, not a generic validation framework. Keep generated key local.

Tests which formerly compared diagnostic output to current decodeJSON should
compare to the shared old-policy path, retaining their original expected facts.
Add independent expected-outcome tests so parity is not only same-helper equals
same-helper. No rewrite of frozen0019/0020 specs, historical reports or runs.

## Acceptance criteria

### QRH-AC-01

A synthetic account-origin QR generation response passes current conversion and
the entire existing five-step scripted flow with the correct poll key. Old
passport responses still pass. The account regression fails before the repair
and when account acceptance is removed; the repaired code passes.

### QRH-AC-02

Both hosts still reject HTTP, userinfo, explicit ports, malformed/oversized URLs,
invalid key/data types and key bounds. Lookalike/suffix/subdomain/unrelated hosts
remain rejected. Existing privacy/request/lifetime suites pass. Removing the
host restriction entirely fails a discriminating rejection test.

### QRH-AC-03

The diagnostic keeps its exact existing output schema and historical old-policy
outcomes: account false, passport true when otherwise valid, malformed/duplicate
input unchanged. A regression discriminates accidentally using the current
expanded rule for PriorValidatorAccepts. Code and report emit no QR/key/raw data.

### QRH-AC-04

Exact repaired candidate passes targeted race and go run ./cmd/dev check --race,
and independent contract/code review. After approval, run the existing
service-probe --live once, bounded by its original five requests/60s and stop
rules. Record actual QR generation/pending-poll outcomes; later details/danmaku
results are additional observations, not permission to repair another issue.
Do not retry or silently change endpoints if a downstream step fails. A failing
required QR check remains an explicit gap. No login success can be inferred.

## Verification and ownership

| AC | Verification | Environment |
| --- | --- | --- |
| QRH-AC-01 | Account/legacy scripted flow; before/after and old-policy control | synthetic only |
| QRH-AC-02 | Rejection boundary/privacy/lifetime tests and unrestricted-host control | controlled transport only |
| QRH-AC-03 | Fixed diagnostic expectations, duplicate-field regression, current-policy mutation | synthetic only |
| QRH-AC-04 | Targeted/full race, independent review, one existing --live probe | Go1.27.3 macOS; one anonymous invocation |

Write paths are exactly in tasks.json. Controller /root owns runs and snapshots;
same independent reviewer /root/p0_native_research. Required review dimensions:
correctness, boundary preservation, diagnostic compatibility, privacy and causal
regression. No trusted check/workflow/CI policy change or dependency change.
Source/diagnosis references: report0032 and0020/references.md. New durable result:
reports/0033-p0-qr-host-fix.md. Use a new frozen continuation run; retain prior
closed runs and cumulative history. Revert only scoped decoder/test changes if
needed, without user-data migration. Full P0/login/Keychain/native overlay are
outside this repair. No new outside defects are preclaimed or auto-dispatched.
