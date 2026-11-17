# Real login saved and restored

Date: 2026-11-17. Source: `2c0fd1656810e5556b7ac7e885b3043d23ea5d39`.
Run: `20261117T053339Z-bd821b155f6c`.

The owner unlocked the execution Mac and completed ordinary Bilibili web QR
sign-in. The reviewed normal application saved the session in the Mac Keychain,
confirmed readback, verified it with the service and exited normally. A fresh
process of the identical executable restored and verified that session without
generating another QR, then exited normally. The real Keychain item remains
intact for subsequent P0 development.

| Claim | Evidence | Result | Scope and not tested |
| --- | --- | --- | --- |
| Real scan, protected save/readback and service verification | Run event 13; allowlisted `SignedIn`, `Stored=true`, `Verified=true` receipt | PASS | Actual authorized session on macOS; no QR/cookie/account identity retained |
| Normal close preserves stored session | Same process emitted closed with Stored=true and exited 0 | PASS | No Logout/Delete or forced process termination |
| Same-binary restart restoration | Run event 14; `restore-verified`, Stored/Verified/Restored all true; exit 0 | PASS | `--restore-check` cannot generate a QR; credentials remain saved |
| Rebased source checks | Run event 10, full `check --race` at `2c0fd16` | PASS | Local Go1.27.3 darwin/arm64; no new candidate CI claim |
| Implementation/native readiness review | [Prior handoff](0035-p0-persistent-login.md) | Previously approved | Historical exact candidate; rebase preserved login source/dependencies |

The exact developer executable and its one recorded SHA-256 are in the prior
handoff. It was reverified before both real invocations. Local artifacts remain
under `.agent/p0-login/live-resume-g1apxy/` and `restore-FM1Lvj/`; sanitized
receipts are attached to the run. They are local evidence, not remote uploads.
The earlier screen-locked attempt remains recorded. The later private-handoff
ended receipt was unavailable in the controller context; application receipts
and owned process exits establish the results above. No further interaction
with the retired handoff window occurred.

LOG-AC-06 is now observed complete. Login formal review remains round 3/3,
global prior formal count 12; these runtime observations are not a new review
or a reset of earlier failures. LOG-REV-001 remains independently closed.
LOG-ADV-001 remains nonblocking and deferred to the next account UI copy edit.
No new finding or remote issue was created.

Continue the owner's P0 objective: native Chinese danmaku/timebase verification,
reviewed asynchronous detail/media loading, real search/link-to-play integration
and device-local history/resume. Actual account expiry, full-client sign-out
coordination and those cross-feature flows still require their own scoped work.
Do not delete the live session for testing. Use synthetic account/storage
fixtures for logout, deletion, expiry and failure cases. Windows native remains
P2/not-tested. This is login completion, not full P0 acceptance or a release.
