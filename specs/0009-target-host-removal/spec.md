# Target playback without host restrictions

Spec ID: `0009-target-host-removal`
Status: owner-authorized amendment, 2026-11-13; independent contract review
precedes implementation. The user explicitly requested removal of origin policy,
allowing all hosts, and consistent documentation updates.

## User scenario and goal

The selected public rendition may move between media hosts. A valid fresh
response must not stop because its video or audio host differs from a prior
observation. Remove host and port pinning from the opt-in target path and its
refresh helper, rather than adding another CDN to a list.

## Scope and boundaries

All syntactically valid HTTPS destinations are eligible, including previously
unseen domains, single-label/localhost names, IP literals, and explicit ports
1–65535. Neither video nor audio has a host allowlist, host-class exclusion,
role-specific origin, or comparison with an earlier observation. Syntax checking
does not perform DNS resolution. Native transport still verifies TLS certificates.

Keep the selected source/part, rendition, classification/grant, fixed public
headers, expiry, private stdin handoff, descriptor limits, and immutable input
matching. Local files, non-HTTPS URLs, userinfo, fragments, invalid URL/port syntax,
control characters, malformed escapes and oversized input still fail. Preserve
locator bytes exactly. The fixture-only default and dual controller/opener
validation remain. Cancellation, thread ownership, timeouts, retry, and teardown
are unchanged; this amendment does not alter their lifecycle contract.

`targetprobe.ValidateMediaURL` provides the shared syntax-only validation used
by the target policy and the new scratch refresh helper. It returns sanitized
errors and never resolves or fetches a destination. Remove origin fields and
origin constants from descriptors/public metadata. Increment the strict
descriptor version to 2 and policy ID; reject old version-1/`origins` input.
The helper emits version 2 and uses the same validator, so acceptance cannot
diverge at the two entry points. Origin changes do not count as rendition changes.

Copy the reviewed refresh sources to new ignored scratch before updating them.
Retain old helpers, evidence, frozen inputs and failed results. New helper tests
must cross the actual `targetprobe.Parse` boundary with synthetic responses.
No live metadata request, native target session, runtime/dependency change,
default-player switch, M1 acceptance, commit or remote action is part of this task.

Update current specs, assignments, task descriptions, README, architecture,
milestones, testing/command guidance, known issues and the current handoff.
Historical reports that discuss the old host gate receive a dated supersession
notice; their original evidence and conclusions remain identifiable.

## Acceptance criteria

### HOST-AC-01

Fresh descriptors for previously unseen video/audio HTTPS hosts and ports pass
the target policy, including IP literals and localhost. Input URLs are copied
unchanged. No origin declaration or historical origin match is required.

### HOST-AC-02

The new refresh helper accepts host changes without retries or new observations
in its offline tests and emits version-2 input accepted by the maintained parser.
Source/rendition/grant changes, malformed input, secrets in headers, expiry and
post-validation locator mutation still fail. Public metadata contains no locator
or host copied from input. Old descriptor versions/fields are rejected.

### HOST-AC-03

Current documentation consistently states that all HTTPS hosts are allowed.
The old origin mismatch remains a historical UNKNOWN, never a playback pass.
The owner decision, exact code/check/review identity and remaining M1 gates are
recorded in ADR 0007 and report 0022. Prior run records remain unchanged.

## Verification matrix

| AC | Verification | Environment | Evidence |
| --- | --- | --- | --- |
| HOST-AC-01 | Target policy host/port acceptance table and immutable input regressions | Offline Go/race on macOS | Failure before repair, passing post-repair output |
| HOST-AC-02 | Refresh response-to-parser integration, negative/private input cases, default zero-request invocation | Offline synthetic inputs; no live service/native launch | New helper source identity and test/build logs |
| HOST-AC-03 | Repository-wide documentation audit; independent contract and exact-candidate code/doc review | Current checkout plus preserved before snapshot | Review findings/disposition, relative links and diff check |
| All | `go run ./cmd/dev check --race`; e2e target app/driver build | macOS arm64, Go 1.27.3 | Full check and compilation evidence; no native playback claim |

## Risks and rollback

Host-based filtering is intentionally removed by the owner. URL syntax and TLS
verification do not impose destination restrictions or a network firewall.
Native redirect/DNS behavior and the recorded downgrade limitation remain.
Version 2 requires rebuilding the helper and both target binaries together;
historical binaries/descriptors cannot serve as new-candidate evidence.
Rollback would restore only this amendment from its exact before snapshot,
preserving all pre-existing work. No automatic rollback or further permission
decision is required by this contract.

## Review and evidence

Use the repository's standing native delegation authority for an independent
architecture review before freeze and exact code/resource/documentation review
after checks. The user supplies the host-policy decision; reviewers assess its
faithful implementation and retained requirements. Controller-owned evidence
and candidate manifests live in a new run. Durable handoff:
`reports/0022-target-host-removal.md`. Target playback, final adapter assessment
and owner M1 acceptance remain separate work.
