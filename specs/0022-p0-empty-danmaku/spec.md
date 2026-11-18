# Preserve valid danmaku when an entry has no text

Spec ID: 0022-p0-empty-danmaku
Status: proposed scoped repair, 2026-11-17.

## Goal and scope

The owner authorizes resolving danmaku parsing, then real QR sign-in with saved
credentials (ideally one scan), then native overlay and full P0. This first
contract addresses the observed empty-text rejection only; subsequent slices
get concrete reviewed contracts without another routine permission question.
Observed read-only investigation: HTTP200 binary1501 bytes, transport already
decompressed, one valid entry then text-missing-or-empty rejection. See
.agent/danmaku-investigation/summary.md and retained safe live-report.json.
BiliPai's pinned DanmakuParser skips empty-content display entries rather than
invalidating the segment. No payload or third-party code is copied.

Existing decoder correctly rejects malformed wire/field/type/range/UTF-8/size
conditions. Retain all of those. Change only structurally valid entries with
positive valid mode and absent/zero-byte content: count and skip them. This is
not a whitespace-trimming policy. Unknown or invalid mode handling remains.
Current QR host repair and historical diagnostics remain unchanged.

## Contracts and boundaries

Owned package experiments/p0/serviceprobe. decodeEntry may return an explicit
empty classification in addition to supported/valid; it still validates the
entire entry before deciding eligibility. An empty field before a later malformed
field is not a reason to bypass that malformed-data failure.

segmentCounts.total/supported/unsupported continue to describe nonempty accepted
entries. Add skippedEmpty for valid entries without text. Before processing any
entry enforce entryLimit20000 against total+skippedEmpty, so skips cannot evade
the bound. Keep entryBytes4096, textBytes1024, response caps, signed-state repair,
proto3 defaults, cancellation/transport/privacy and request sequence unchanged.

Observation adds only SkippedEmpty int with JSON skipped_empty,omitempty; zero
is omitted and all old fields retain meaning. Existing QR diagnostic serializes
its zero field as omitted, so its established output remains unchanged. This
additive numeric result records skips without content, IDs or arbitrary strings.
If total==0 (including all-empty input), report existing inconclusive/
danmaku-unavailable, never a successful displayable segment. Nonzero state and
malformed/oversized input retain their existing unavailable/failure outcomes.

## Acceptance criteria

### DME-AC-01

A normal+empty+normal segment retains/counts both nonempty entries and counts one
skip. Explicit-empty and omitted-text variants both work; empty-only input is
inconclusive. Before repair the mixed regression compiles and fails; restoring
the old eligibility behavior makes it fail after repair.

### DME-AC-02

All examined entries including skipped ones consume the20000 cap; exact limit
passes and limit+1 fails, including all-empty and mixed data. Malformed wires,
wrong field types, invalid UTF-8, range/size failures and mode0 remain failures
even for empty text. Nonzero signed segment state behavior is retained. A control
that ignores skipped entries in the cap must compile and fail the limit test.

### DME-AC-03

The probe safely reports skipped_empty only when nonzero, retains old schema
semantics and privacy, and does not treat all-empty segments as successful.
Targeted race, full go run ./cmd/dev check --race and independent contract/code
review pass on the exact candidate. Existing QR/lifecycle/privacy tests pass.

### DME-AC-04

After code approval, run the existing five-step read-only probe once with its
unchanged60s/five-request/stop bounds. Record actual segment parse/count/skip
results; no raw comments or credential material. If another failure occurs,
record its exact remaining gap before selecting a causal further change under
the owner's broader instruction; do not guess or silently widen this contract.

## Evidence, ownership and retained history

| AC | Check | Environment |
| --- | --- | --- |
| DME-AC-01 | Mixed and empty variants; compiled before/after/control | synthetic protobuf |
| DME-AC-02 | Entire-entry validation and exact/over-cap controls | deterministic synthetic data |
| DME-AC-03 | Report privacy/inconclusive, targeted/full race, independent review | Go1.27.3 darwin/arm64 |
| DME-AC-04 | One unchanged service-probe --live, sanitized receipt | current Mac, anonymous |

New continuation run freezes this contract. Same controller /root and original
independent reviewer /root/p0_native_research; worker write paths in tasks.json.
Prior formal7, repairs3, service invocations4 and issues0/5 stay in the shared
ledger. Owner now explicitly requests further repair and progression: bound
this repair to architecture/code rounds8/9, repair4 and one verification(total5).
No reset, commit/remote action or workflow-policy change. Required record:
reports/0034-p0-empty-danmaku.md. Removing only this delta rolls back without
user-data migration. This repair does not log in or finish P0; continue to the
owner-authorized login preparation after it, preserving the one-scan preference.
