# M1 target host restrictions removed

2026-11-13. The owner explicitly requested removal of origin policy and acceptance
of all hosts. [ADR 0007](../docs/adr/0007-target-host-policy.md) records that decision.
The target policy and copied refresh helper now accept all syntactically valid
HTTPS hosts and valid ports. There is no host allowlist, role-specific origin,
host-class exclusion, historical-origin comparison, or repinning step.

M1 remains incomplete. The previous refresh stopped before native launch, and
the later diagnostic observed a changed video CDN host. Those outcomes remain
historical evidence in [report 0021](0021-target-access-method.md). This amendment
does not convert them to a playback pass. Native target playback is still not-tested.

## Changes and retained boundaries

- Removed hardcoded video/audio origins and origin fields from descriptors and
  public metadata. Descriptor version 2 and policy ID
  `0008-source1-all-https-hosts-v2` distinguish the new behavior; version 1 and
  the removed `origins` field are rejected.
- Added shared `targetprobe.ValidateMediaURL` syntax validation, used by the
  player and new refresh helper. New DNS hosts, IP literals, localhost names,
  single-label hosts and ports 1–65535 are accepted without network lookup.
- Kept exact source/part, rendition, grant, fixed public headers, TLS certificate
  verification, expiry, private stdin and byte-for-byte input matching. Invalid
  URLs, userinfo, fragments, controls, malformed escapes and expired input still
  fail. Native redirect/DNS behavior and its recorded downgrade limitation remain.
- Updated current contracts, assignment, README, architecture, milestones,
  testing/command guidance and known issues. Reports 0019–0021 have dated
  supersession notices; their original bodies and evidence remain preserved.

The fixture-only default and native lifecycle code are unchanged. Windows stays
optional P2; calibrated physical sync/rate measurements stay outside M1 scope.
The earlier 41-assertion native capability pass belongs to its recorded candidate,
not to a new native execution of this change.

## Contract and source identity

Task [HOST-001](../specs/0009-target-host-removal/spec.md), controller run
`20261113T005629Z-2911e43f08da`. Base HEAD is
`d846e667c537c06a1bcf100e6aac9486c29aa3d2`, intentionally dirty; it does not identify
the candidate. Existing user work was retained, and no commit or remote action ran.

Independent reviewer `/root/host_policy_review` approved the exact architecture
candidate with no actionable findings. Review SHA256:
`c71758d18a85add8f490ad8fd6185420e61387cad7fc98ea419a8ea45b10e581`.
Architecture manifest/archive SHA256:
`71e81a2682097401bace825e2ea3e643ee4ba145ac56da2c5a8deb6325fec31a` /
`11220f0d7e08d4def3258b84b3557cfda53dafc9fac70c05e0615ae509a0497a`.
The frozen 0009 spec/task hashes are
`707269ef41e29cd57b130d28401c8f1af7ee5ece4d8a7c15141d21367eb21790` /
`b27341ca03fc8ccddbe3132080399f032a478dbd1a5c48770a33f8f6d609f4c9`.

The same independent reviewer approved the exact code, helper and documentation
candidate with no actionable findings. Review SHA256:
`8cc7b19c90542d6d7dee9f0b51e294685ce2a0a930416d8f5ccd3c6a7bfe10ef`.
Its evidence inventory SHA256 is
`e27f28094fe26cd2a2f5ea493fbba608b5c5acceecebfb10b271b60d66663777`.
Both are retained in the run's `evidence/independent-code-r0/` directory.

| Reviewed artifact | SHA256 |
| --- | --- |
| Maintained candidate manifest, 178 files | `b293988e91a5710cdd067595d4f89a40c92a1ff0f57f4ba4ef7a59f5bf30e5b0` |
| Maintained candidate archive | `bb5409dcd41646a4db3efdc1be4cefd365201056f6ed76d86cf760ea4a4386e3` |
| Copied helper manifest, 11 Go files | `5a30e014a846a4c8fa7a584f3a83ec8faaa9212f50a527fc70554f3b702ef5ed` |
| Copied helper archive | `94077bc43f26f789636a8ba765483f593b1eaa002a177e0ad5ccc5fb0de24f29` |

The new helper is `.agent/worker-scratch/target-refresh-unrestricted/`, copied
from the previously reviewed ten-file helper without changing the old sources.
It uses the maintained target validator and adds response-to-parser integration
tests. It remains ignored local evidence, absent from fresh clones. The run
retains a source archive/manifest and the prior helper inventory for recovery.

## Verification

The before-change host regression failed on the old policy. An initial helper
integration test selected the fixture's unselected audio entry; that test setup
was corrected to the reviewed audio ID 30216, with its failed result retained.
The actual helper's selection algorithm was unchanged.

| Check | Result |
| --- | --- |
| Full `go run ./cmd/dev check --run 20261113T005629Z-2911e43f08da --race` | PASS, all six stages on macOS arm64 / Go 1.27.3 |
| Target policy regressions | PASS, including new hosts/ports, legacy schema rejection, copied input and private metadata |
| Copied helper offline race suite | PASS, 146 tests/subtests, including fresh response-to-parser host changes |
| Copied helper vet | PASS |
| e2e target-native, target-smoke and updated helper builds | PASS; compilation only |
| Helper default / invalid execution pins | PASS, zero requests in both invocations |
| Current documentation relative links / whitespace | PASS, 131 relative links; `git diff --check` clean |
| Historical helper source inventory | All ten source files unchanged |
| Independent focused maintained-package race suite | PASS, 135 tests/subtests |
| Independent copied-helper race suite | PASS, 146 tests/subtests |
| Independent additional URL/parser race probe | PASS, 56 tests/subtests |
| Independent helper rebuild and two offline invocations | PASS; both invocations made zero requests |
| Independent exact code/helper/documentation review | APPROVE, no actionable findings |

The reviewer independently verified source hashes/modes, all three controller
binary hashes, all 131 relative links and preservation of historical report
bodies. The reviewer inspected the controller's full six-stage check and app/driver
build evidence; those checks were not independently repeated in full.

This report's review outcome and the wording of the notices in reports 0019–0021
are later documentation-only updates to the reviewed candidate. They do not change
code, helper sources, frozen contracts or test outcomes. The run retains their
separate final diff and source manifest/archive; they are not part of `code-r0`.
No live metadata request, native target playback, new native fixture run,
changed-candidate CI or packaging result is claimed by this amendment.

## Next action

Use matching rebuilt version-2 helper, target-smoke and target-native binaries.
Preserve their exact identities and the unchanged runtime
pins, perform relevant native fixture regression, and record a bounded TGT-003
execution assignment before a fresh target session. A new media host requires
no additional origin-policy amendment or approval. Then obtain independent target
evidence disposition, final adapter assessment and owner M1 acceptance.

All detailed checks and exact source artifacts are local to the new run's
`evidence/`. They are ignored and not backed up by Git. The old run, helper,
frozen inputs, failed observations and previous reviews remain unchanged.
