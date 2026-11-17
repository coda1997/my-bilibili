# P0 login foundation: reviewed, native-tested, awaiting the owner's scan

Date: 2026-11-17. Run: 20261117T053339Z-bd821b155f6c.
Status: account/login implementation and synthetic native readiness are approved.
Real sign-in and same-binary session restoration remain pending: the execution
Mac was locked, the private handoff read/typed nothing, and the owner requested
pausing until they can unlock it. The expired attempt was closed gracefully;
no actual credential was acquired, saved or deleted. Do not retry sign-in until
the owner confirms the execution Mac is awake and unlocked.

## Delivered and validated

- Framework-free account lifecycle and opaque/redacted credential/QR types.
- Fixed web QR generation/polling/session-verification adapters, with bounded
  requests, exact cookie scope and a versioned protected-session format.
- Stable Mac Keychain service/key, confirmed save/readback and restart reuse;
  save/network failures retain material instead of forcing another scan.
- Native Go login window with in-memory QR rendering, retry/cancel/close gates,
  explicit logout confirmation and no production automation endpoint.
- Separate synthetic-only UI build and isolated Keychain smoke command. Test
  builds cannot reach real account adapters/storage; normal builds reject test
  flags. The only new dependency is pinned MIT go-qrcode with its license notice.

The preceding service work is in [0031](0031-p0-start.md),
[0032](0032-p0-qr-diagnostic.md), [0033](0033-p0-qr-host-fix.md) and
[0034](0034-p0-empty-danmaku.md). The latest anonymous five-step probe passes:
QR generation/pending poll, details and danmaku; 13 nonempty entries and one
skipped empty entry. This does not establish real account sign-in or full P0.

## Review and exact evidence

Independent reviewer /root/p0_native_research is distinct from authors /root,
/root/p0_implement and /root/p0_login_research. Login architecture round1 found
LOG-REV-001: host-only/path-restricted cookies must not be widened to API nav.
Round2 closes it after requiring explicit parent-domain/root-path grants and
fixed-scope restoration. Integrated code/evidence round3 approves with no
BLOCKING findings. Global formal review count is12 with earlier history retained.

Nonblocking LOG-ADV-001 (P3): save-failure wording should distinguish uncertain
readback from definitely absent persistence. Deferred under R2 to the next
account UI copy adjustment; RetrySave and no implicit credential deletion already
work. No remote issue filed for this wording-only advisory. No additional review
is required solely to preserve this disposition.

Reviewed code manifest .agent/p0-login/code-final-manifest.json SHA-256:
6e07d964ffcdcfa4854d782804304c9d4e36bde24f0b1e71801b582389eb0522.
Archive SHA-256:
e9705a3e2f1d8a69ca78b5959c2ced6f937a98c26f1773cab40250435736dea6.
Frozen spec SHA-256:
8928f186d65135486eba6880a87f9bfbbd6feb6ce63ae6d8516aa16f04cf9aff.
Task graph SHA-256:
9efe5d1349192ec561febd30703038d13f9b2f141b53edfff071ce4626707f70.
Shared contract SHA-256:
1ab2f609bc122cd539ca69678a20985be8ad162e68785f9fb2ae2e05119d7109.

Final controller full check, Go1.27.3 darwin/arm64:
go run ./cmd/dev check --run 20261117T053339Z-bd821b155f6c --race: PASS.
Event000004 log SHA-256:
ac38980aca4b68efa2f0b6c4565964f1ac12adb3118b1f7c7fc581beae0954e6.
Independent targeted and tagged offline race checks pass. Selected privacy,
cookie-scope, cancel/save/close, dependency and mode-isolation mutation controls
compile and fail their intended assertions. No changed-candidate CI is claimed.

| Criterion | Evidence / status |
| --- | --- |
| LOG-AC-01 | Plain ports, redaction/copy tests, import boundaries, exact QR pin/license: PASS |
| LOG-AC-02 | Synthetic protocol/cookie/scope/schema/isolation/bounds: PASS |
| LOG-AC-03 | Cancellation, retained material, serialized store work, retry and close/logout rules: PASS |
| LOG-AC-04 | Synthetic same-helper two-process Keychain save/read/replace/restore/delete: PASS; test item cleanup complete |
| LOG-AC-05 | Final synthetic native UI suite: PASS; keyboard, light/dark/minimum-size QR pixels, cancel/expiry/retries, logout confirmation/failure and graceful close |
| LOG-AC-06 | Code/check/review readiness complete; real scan/save/nav and same-binary restoration pending owner unlock |

Native history is preserved: attempt1 failed because the test driver read the
wrong focus field; attempt2 reached all preceding flows but close completion
waited for another paint. The corrected driver uses the actual focus node ID,
and composition dispatches state completion independently of rendering. Final
attempt3 passes without forced cleanup. Synthetic storage passed in two processes.
Implementation input-boundary, API/label, compilation and native integration
corrections remain disclosed in local author/controller evidence; none is erased
or used to reset review/repair history. No new outside defect is claimed.

Reviewed normal developer executable SHA-256:
5204b54dc6f6b34f843bfd419d0ce9e1c0af74c8d98a43cd672bed6407b12b27.
Synthetic e2e executable:
fde31a75f9c29069b617feb6c1d30df9e2ef80b6b81ca3a63f788a3450c7bdf8.
Storage smoke executable:
cb40bdcb28b91d8f47b10aeed8c6c2a40643b00290b1c680185eb1c3447cbeaa.
These are local ad-hoc developer artifacts, not signed public releases.

## Publication and continuation

The owner subsequently requested a new branch and remote push as a checkpoint,
while feature work remains paused. Publication branch: codex/p0-login-foundation.
Remote confirmed as public https://github.com/coda1997/my-bilibili. It starts
from master84b74351a177c4e2a66f2d108f9a3765ef3eb896; its tree equals the original
review base e35022c1218588b2ebacc4c5e69b5f4cd431a9fe (tree
4add49ca570816d3f1925646c913cad4c25f10c7), avoiding duplicate prior PR history.
Publication adds this sanitized handoff/navigation; executable source, frozen
contracts and dependencies remain byte-identical to the reviewed candidate.
Old reports retain their original uncommitted/source identities. The actual
publication commit and remote ref are verified separately, not pre-filled here.
No PR, merge, repository settings change or release is requested by this push.

Local runs, archives, native test images, private-handoff metadata and developer
bundles remain under ignored .agent/; they are not remote attachments or Git
backups. Preexisting .claude/ is excluded and untouched. No real QR image,
credential, session payload or raw authenticated response is included in Git.

Resume only after the owner confirms the execution Mac is unlocked. Verify the
reviewed normal binary, create a fresh challenge and new owned process/window,
then use the private native handoff. Require safe SignedIn/Stored evidence,
close without Logout, and run the identical binary --restore-check. Keep the
real Keychain session for subsequent development; do not test real logout by
deleting it. Then continue the owner-authorized Chinese overlay/timebase and
full client integration. Full P0 remains incomplete; Windows native remains P2
and not-tested. No background scan retry or automatic evening schedule is active.
