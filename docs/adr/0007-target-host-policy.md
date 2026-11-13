# ADR 0007: Remove target host restrictions

> Acceptance update, 2026-11-13: the owner accepted M1 as completed with libmpv
> playback and mygo UI. See [the acceptance handoff](../../reports/0025-m1-acceptance.md).
> Pending-gate wording below describes this earlier decision's original scope;
> historical evidence and untested limitations retain their meaning.

- Date: 2026-11-13
- Status: owner-selected policy amendment
- Scope: opt-in M1 target playback and its refresh helper

## Decision

The owner explicitly requested removal of origin policy and acceptance of all
hosts. Target media URLs no longer require an allowlist, role-specific origin,
port pin, or match with a previous observation. All syntactically valid HTTPS
hosts are allowed, including new CDN domains, single-label/localhost names and
IP literals; explicit ports 1–65535 are allowed. There is no host-class or
DNS-destination restriction. A CDN host change is not a rendition change.

This supersedes every earlier exact-origin approval, frozen-origin comparison,
no-wildcard requirement, and proposed two-origin amendment for current target
playback. Future host changes need no per-host approval or repinning.

## Retained requirements

Keep HTTPS URL syntax and certificate verification, selected source/part and
rendition, access classification/grant, fixed public headers, expiry, private
input handling and exact post-validation locator matching. Reject userinfo,
fragments, local files, non-HTTPS URLs, malformed encodings/ports, control
characters and oversized input. Preserve locator bytes without rewriting them.
The fixture-only default, native control/cleanup deadlines, runtime and native
redirect/DNS behavior remain unchanged. No network-firewall claim is made.

## Descriptor and refresh compatibility

Descriptor version 2 removes `origins`; old version-1 or origin-bearing input
is rejected. The target policy and updated refresh helper share
`targetprobe.ValidateMediaURL`, a syntax-only check with no DNS/network action.
Public metadata contains no media host, URL, locator hash or header values.
Rebuild the helper, target-smoke driver and target-native app together before
use. Old binaries retain their historical behavior and evidence identity.

## Evidence and continuation

The previous origin mismatch remains a historical UNKNOWN before native launch.
This amendment does not convert it to a playback pass or authorize a new live
request/session by itself. Target playback, final independent adapter assessment
and owner M1 acceptance remain pending. Windows stays P2; calibrated physical
measurements remain outside M1 scope under ADR 0006.

Current specs and navigation reflect this decision. Historical reports receive
supersession notices; their evidence, reviews, frozen inputs and published run
events remain preserved. See [the implementation contract](../../specs/0009-target-host-removal/spec.md)
and [current handoff](../../reports/0022-target-host-removal.md) for exact
checks, independent technical review and remaining work.
