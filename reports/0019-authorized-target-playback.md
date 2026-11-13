# M1 public-target preflight

> Supersession, 2026-11-13: the owner removed all target host restrictions in
> [ADR 0007](../docs/adr/0007-target-host-policy.md). Earlier exact-origin approvals,
> historical-origin comparison requirements and any proposed two-host amendment
> are superseded for current work. All valid HTTPS hosts/ports are allowed. The
> dated evidence and stopped outcomes below retain their original meaning;
> see [the current handoff](0022-target-host-removal.md) for implementation and
> remaining M1 gates.


2026-11-12. **Native target playback is not-tested; TGT-001 is incomplete.**
The selected public page and two bounded anonymous metadata responses do not
settle full-versus-preview permission under the reviewed contract. TGT-002 and
TGT-003 remain unstarted. No target URLs, cookies, response bodies, locators or
third-party recordings are retained. Windows remains optional P2; macOS M1 is
not accepted.

## Scope and review

The owner asked to complete M1 and selected a public test video. Normal signed-out
browsing selected `BV1Zs411o7Ee` (Big Buck Bunny). A visible preview item, browser
duration597s and position18.6s were discovery observations, not proof of full
permission. The public page is identifiable without retaining signed media URLs.

[Spec0008](../specs/0008-authorized-target-playback/spec.md) defines a separate,
bounded anonymous probe with fixture-only defaults preserved. Its original
architecture was independently approved by `/root/target_architecture_review`
in run `20261112T133510Z-9ff99e95b049`, review SHA256
`a36904f4d1b3c2052a52fb3f7ae38e950b724af9fc372c4522493eb19e9eef38`.
That approval permitted preflight only. No opt-in remote-input policy, target
UI, resolver or target-native command has been implemented.

The first request's safe parser omitted nested permission fields. A narrow
observation-correction amendment was independently approved, review SHA256
`8d8a4866d645fde097113eb9ae490eb19a7b0890f10d7d45824eaa78c2b3bebc`.
Current spec SHA256 is
`64e1f42d55b7a70fa8eed904cb860b7dbf17a35371089406273df2492a8b900b`;
task SHA256 is
`c6cba3e5e731ad09a67c5f5f74da6c751943b25e2ff4718fd3b518340d344a91`.
The original run `20261112T133658Z-5a8178e6b420` preserves all first outcomes;
new freeze `20261112T134725Z-c3dd09a44f5b` owns the sole correction and gate
disposition. There is no third-request allowance.

## Actual anonymous observations

The browser's documented network interface exposes request/status metadata,
not response bodies. It identified the normal player's metadata endpoint.
Two independently scoped requests then used that same HTTPS endpoint and only
the observed public video/part/rendition parameters. They used fixed public
Referer/User-Agent, verified TLS, a10s deadline,1MiB body cap and no cookies,
credentials, signatures, session/device values, redirects or automatic retries.
Only normalized nonsecret facts survived the process. The second extractor's
offline race tests passed before its sole request.

| Observation | First request | Sole correction |
| --- | --- | --- |
| HTTP / service code | 200 / 0 | 200 / 0 |
| Selected quality / codec identifier | 32 / 7 | 32 / 7 |
| Reported duration | 596624ms | 596624ms |
| DASH representation counts | 2 video,3 audio | 2 video,3 audio |
| Nested permission facts | Not retained by initial parser | `view_info` null; all allowlisted direct permission fields absent in `play_conf`; rendition login/VIP fields absent |
| Full-versus-preview grant | Unknown | Unknown |
| Native target playback | Not-tested | Not-tested |

The correction preserves the first incomplete observation. Successful metadata,
duration, representation counts and absent permission fields cannot fill the
missing grant. No usable descriptor, exact permitted origin policy or full/preview
interval was established. No native target attempt followed. The reviewed stop
condition applies; do not make further requests or broaden acquisition implicitly.

## Controlled native feasibility

`/root/platform_scope_review` separately ran ignored Go/purego probes against
generated H.264/AAC fixtures and local HTTP/TLS servers. No target URL was used.
The unchanged runtime manifest SHA256 is
`a633103c161ee4b648722ba041c695825c5532170bde7311f6ceb917fc05e2bc`;
libmpv SHA256 is
`8f7c25d37eab7245731fa46734d2003601b272d2dcade2fbb925157e55246bbd`.
All30 runtime library/dependency identities matched before and after the probes.

| Controlled scenario | Result and limit |
| --- | --- |
| Local generated MOV/MP4 H.264/AAC | Native decode/progression passed with null outputs |
| Valid HTTPS with scratch-only private CA | Passed; no global trust configuration changed |
| Untrusted certificate / wrong hostname / invalid certificate after redirect | Rejected before destination HTTP handler access |
| Cross-origin HTTPS redirect | Followed; fixed public headers forwarded |
| HTTPS→HTTP redirect | Followed and played: explicit transport limitation, not an HTTPS-only security pass |
| Missing required public headers | Controlled denial, no playback |
| Separate TLS streams, initial scratch r0 | Failed: string audio-list option supplied only video |
| Separate TLS streams, corrected scratch r1 | Passed using the maintained adapter's typed-node list; both streams and public-header matches observed |
| Core destruction, library unload, post-unload survival, server cleanup | All10 invocations passed; eight-second survival retained |

Only the separate-stream case was rerun on scratch r1. The first failure and
both source/binary identities remain preserved. Native-core destruction took
0.12–31.01ms with null outputs; these are neither rendered-player cancellation
nor physical sound/sync measurements. The private test CA was scoped to the
probe process; it does not establish a real-service certificate-chain result.

The native build has no curl backend. `tls-verify=yes` validates certificates
on HTTPS requests but does not forbid a redirect to HTTP. Initial-origin checks
in the proposed policy would be input validation, not a network firewall.
A known downgrade in a future target attempt must stop. No selected target
redirect chain, codec/rendition compatibility or actual output was established.

## Evidence and next action

The original run's `evidence/transport/` retains the sanitized report,31-artifact
inventory and controlled sources/results. Report SHA256:
`edbd555b185c1de1676bd1f46139e4fe84c8d7b620e338b1f058801810cf21e4`;
inventory SHA256:
`5ee9db2c1cfc57819d5f9e31664aa31c0b50f707dbe40b28083980e7a2e7ccb7`.
Original and corrected metadata source/tests/results are separately retained in
their corresponding controller runs. Ignored local evidence is not backed up
by Git; retain it before deleting this checkout. No remote CI result is claimed.

Independent reviewer `/root/target_architecture_review` approved the exact report
and evidence as a truthful bounded stop, with no actionable method/report finding.
The gate disposition is **needs-input, not passed**; TGT-002/003 remain unstarted.
The reviewer independently passed the extractor race tests and ran six controlled
native cases, reproducing valid TLS, certificate rejection, the downgrade
limitation, the original separate-stream failure and typed-list repair. All six
completed destruction, unload, eight-second survival and server cleanup; all30
runtime artifacts remained unchanged. No further external request ran.

The reviewed report candidate SHA256 is
`767467bc251dc2da4c2672a7886be8822b003f877ec9f8199f5f2dbcc004cb4c`.
The review and15-artifact inventory are retained in the correction run's
`evidence/independent-gate-r0/`, with review SHA256
`29d7a3a18c745883d679c12a2a7c4ec3e10848ec45853236257592c8a868cd70`
and inventory SHA256
`e12247d05840108de891a0c4848e75d59dbf115bc7da44f145f7f406e5e372e5`.
This disposition was appended after review; it does not change the observed facts.
Continuation
needs an ordinary authorized source response whose permission can actually be
interpreted, or a separately reviewed change to the access-validation method.
Do not silently treat successful metadata or a browser preview as full grant.

Separately, [0007 output preparation](../specs/0007-output-verification/spec.md)
does not supply the unavailable human audibility, externally calibrated physical
sync/rate evidence or approved numerical acceptance criteria. A final independent
adapter assessment and owner M1 acceptance remain required. No M2, P0, full shell,
commit, publication or production-player decision follows from this preflight.
