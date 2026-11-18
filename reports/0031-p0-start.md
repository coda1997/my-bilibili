# P0 startup: bounded web-QR and read-only service feasibility

Date: 2026-11-17. Controller run: 20261117T031441Z-f84114441569.
Status: probe implementation independently approved; first live observation
stopped at QR response validation. Further diagnosis needs a bounded continuation.
Selected contract: [0019](../specs/0019-p0-service-feasibility/spec.md).

## Goal, authority and delivered scope

The owner requested starting P0 and explicitly selected ordinary web QR login
and read-only video/danmaku access. Credentials are to stay in the Mac Keychain
on the machine running the client. Login and danmaku remain mandatory P0 goals.
No commit, push, PR, merge or release was requested or performed.

This first executable slice adds eight Go source/test files in
experiments/p0/serviceprobe and experiments/p0/cmd/service-probe. The opt-in
command checks five fixed historical anonymous contracts, using isolated HTTP/1,
finite request/body deadlines, response/decode limits, no redirects/retries,
no ambient credentials, and allowlisted output. Tests use synthetic data and
owned loopback servers. No SDK, schema copy or dependency was added.
The accepted fixture application/player and trusted check/CI policy are unchanged.

This probe never displays a QR, signs in, stores credentials, accesses browser
cookies or fetches playback media. Successful QR generation alone would not
establish login, protected restoration or a working P0 client.

## Actual live observation

One invocation started at 2026-11-17T03:30:47Z UTC (11:30:47 Asia/Shanghai),
after independent code approval and source hash verification. Command:
.agent/p0-start/service-probe-reviewed --live. Exit status 1; stderr empty.
The run observed two HTTP responses, stopped without retries, and did not
execute the remaining three steps. No raw response, challenge/key or cookie
was printed or retained.

| Step | Observed result | Interpretation |
| --- | --- | --- |
| Anonymous session | PASS; HTTP 200, code -101, 259 bytes | Expected signed-out response observed |
| QR generation | FAIL / invalid-response; HTTP 200, code 0, 245 bytes | The reply failed the reviewed response validator despite a successful service envelope |
| One pending QR poll | not-tested | Not reached |
| Fixed public video details | not-tested | Not reached |
| First danmaku segment | not-tested | Not reached |

Observed vs inferred: HTTP/status/size and local classification above are actual
observations. The exact failing QR field or URL condition is unknown because
this privacy-preserving report intentionally omits those values. No server
refusal, endpoint removal, credential issue or specific host change is inferred.
The result establishes neither successful login nor video/danmaku availability.
The single invocation allowance is consumed; no automatic retry/fallback ran.

## Exact candidate and independent review

Base: e35022c1218588b2ebacc4c5e69b5f4cd431a9fe, originally on the prior issue-11
branch. Current local branch: codex/p0-service-feasibility. Changes are
uncommitted. Preexisting untracked .claude/ is excluded and untouched.
Read-only receipts confirmed PR #9 merged as
ff27f1db5946eb9ebccbee8c3548a971dd112e85 and PR #14 merged as
84b74351a177c4e2a66f2d108f9a3765ef3eb896; prior issue-11 run is closed succeeded.
M3 remains owner-accepted. Those receipts do not validate changed P0 code.

- Frozen spec SHA-256: 6bf3df020d0f916549c7decd6dc739b7edd374fb90a38b0246bcc81e7007f277.
- Frozen tasks SHA-256: 286a8cdf1f5a78618657cbeaf681fcb3f03e2c4aebea46c9ecadaa4894c2187e.
- Final code manifest .agent/p0-start/code-repair-manifest.json: 7a6ecccbffcd78a6cae30c70bb57c73f529905493277a809d8b3095bb0dca108.
- Code archive .agent/p0-start/code-repair-candidate.tar: 00e705853a6178d57f5989298d4331d7e506bef40b5a8b838e3491070bfe53d4.
- Actually invoked binary SHA-256: 92adcf1d19019fe3f145b948e26fe07a566269802060305b4e7844facbdab5b4.

Implementer /root/p0_implement is distinct from original independent reviewer
/root/p0_native_research and controller /root. Architecture round 1 approved
the scope. Formal round 2 found P0F-REV-001, a P2 BLOCKING class-1 error:
valid negative int32 danmaku state was misclassified as malformed instead of
unavailable. The scoped repair preserves malformed/overflow rejection and all
other guards. Formal round 3 independently verifies before/after/old-source
and guard-removal behavior, closes P0F-REV-001 and approves the exact code.
No remaining code findings or advisories. This later handoff prose and actual
live result are controller records, not an additional independent review.

## Verification and done criteria

| Criterion | Candidate-specific evidence | Result |
| --- | --- | --- |
| P0F-AC-01 | Fixed sequence, explicit live opt-in, JSON and protobuf synthetic cases, signed-state repair | PASS |
| P0F-AC-02 | Privacy sentinels, body closure/cancel/deadline, decompressed-size caps, fresh connections/no retry, malformed/oversized input, ten guard/old-source controls across initial and repair stages | PASS; expected control failures retained |
| P0F-AC-03 | One reviewed --live invocation, safe report/time/binary/exit receipt | Executed; QR validation FAIL; later steps not-tested |
| P0F-AC-04 | Final full recorded --race check, independent code approval, this handoff | Local code/check/review complete; real service continuation unresolved |

Actual checks used Go 1.27.3 darwin/arm64, GOTOOLCHAIN=local and GOWORK=off:

- go test -count=1 -race ./experiments/p0/... passes after repair, independently
  repeated by reviewer. Eight initial source-removal controls and two repair
  controls compile and fail the intended assertions. Reviewer independently
  repeats selected initial controls and both final repair controls.
- go run ./cmd/dev check --run 20261117T031441Z-f84114441569 --race passes on
  the final stable code (event 000006), including all six check steps.
  evidence/check-4143037066.log SHA-256:
  876569ef634d49dbbfb076ce54cbe769850d6743767ad201615a6b8647975800.
- Final review evidence/code-review-round3.md SHA-256:
  0b5dc4733bfba096780e5f18b2c302b15e12d6b783d2fa10868a3f37018193bb.
- Initial synthetic padding-fixture failure, original parser reproduction,
  pre-repair full PASS and all review dispositions remain recorded separately.
  Earlier passing checks are not substituted for the repaired candidate.

The controller run contains sanitized implementation/repair/review logs,
code manifests and live-report.json/live-receipt.json. Full source archives,
diffs and reviewer/worker scratch are under .agent/p0-start, p0-review and
p0-implementation. These are machine-local, not remotely accessible attachments
or Git-backed storage. No new remote CI, native playback, Keychain, real login,
package or release check was run. Windows remains optional P2/not-tested.

## Concrete next diagnostic proposal

FEAS-OBS-001 records the unclassified QR response mismatch. The recommended
continuation is one additional QR-generation request, without scanning or
polling, after a scoped diagnostic-code review. Add only fixed, safe diagnostic
facts: expected-field presence/types, key/URL byte lengths, URL parse success,
HTTPS match, exact expected-host match and userinfo-presence boolean. Do not
emit the URL, host text, query, challenge/key, headers or body. Retain the same
10s request/body deadline, 1 MiB decoded limit, no cookies/redirects/retries.
Use synthetic cases to prove each diagnostic and output sanitization.

This would identify which reviewed assumption failed without guessing a fix
or collecting login material. Proposed authorization: one focused diagnostic
change, its tests and two additional formal reviews (diagnostic contract and
code), plus one such live request. Preserve cumulative counters; no new run/task should reset this
logical change's budget. A changed access/response contract would then need
explicit review before implementation, based on the observed facts. This
proposal is not already executed or approved.

## P0 roadmap and found-not-done

1. Resolve the observed QR response mismatch through the bounded diagnostic
   above. Successful interactive QR login, session check, restart and sign-out
   remain unimplemented/unverified.
2. Verify protected storage with synthetic cross-process save/read/delete,
   then a reviewed live-session format. Pinned mygo already exposes Mac
   Keychain Secrets, with a 2560-byte single-item limit and potentially blocking
   consent calls without cancellation. Reuse the adapter; preserve errors,
   no plaintext fallback and stale-result rejection. Native behavior untested.
3. Verify Chinese danmaku in the actual player scene with synthetic entries.
   libmpv already uploads RGBA to a mygo scene; mygo provides Chinese text
   layout/clipping. Source inspection does not prove actual overlay/fullscreen
   behavior. Use sampled native Position (50ms controller cadence) for initial
   pause/seek/rate/generation/clipping/cleanup checks; no physical-sync claim.
4. Review/freeze concrete full-client contracts. Current app.Source is
   synchronous and fixture-only; blocking HTTP cannot be placed on its UI call
   path. Define asynchronous ownership, safe errors and account/video generation
   invalidation. The historical proposed internal/ui tree is not the current
   internal/desktop code or an implementation write assignment.
5. Implement account, discovery/details, resolver/quality, bounded danmaku and
   local history; freeze seams before parallel work and order shared UI writes.
6. Integrate and independently verify the complete macOS loop and preview,
   including failure/restart/logout. Full P0 owner acceptance remains separate.

Contract relied on: frozen 0019, existing 0004 product goals, ADR 0005/0008,
current R1-R9 and accepted playback ownership/cleanup constraints. The roadmap
prerequisites are planned work, not new bugs; no issues filed for them.

## Retained counters and continuation

Formal reviews 3/3 (architecture1 + code2); repairs 2/3 (one conservative test-
fixture correction plus one product repair); the one original parser failure
is independently closed. Guard-removal failures are expected evidence.
Live invocation 1/1; remote issues 0/5. No automatic review rounds remain.
P0F-REV-001 CLOSED; FEAS-OBS-001 is a bounded investigation note with unknown
cause. No remote dedupe search or filing is claimed; local inventory is
.agent/p0-start/inventory.md. No systemic follow-up warranted by the isolated
parser repair; no recurrence elsewhere established. No policy/check/CI changes.

Next action: obtain the owner's explicit bounded diagnostic continuation above.
Controller records needs-human rather than silently retrying or claiming P0
completion. Existing source and evidence remain available for that continuation.
