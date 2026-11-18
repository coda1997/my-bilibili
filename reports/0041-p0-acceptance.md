# P0 macOS client accepted

On 2026-11-18 (Asia/Shanghai), the owner confirmed: “OK 确认P0验收”. This accepts
the P0 macOS client milestone at corrected revision `4443b69ad2fa1f7daf2feef03b51a0ef68c3732a`
on `codex/p0-real-client`.

| Evidence | What was verified |
| --- | --- |
| [Login and session restore](0036-p0-login-restored.md) | Real QR sign-in, Keychain save/readback and reuse after restarting the client. |
| [Real playback at `250c5d7`](0039-p0-real-client.md) | Both parts played on macOS with the saved account. Quality switching and visible danmaku were observed; each part resumed from its saved progress after a restart. |
| [Repairs at `4443b69`](0040-p0-focused-repair.md) | Full and tagged Go race checks and 46 offline native client checks passed. The original reviewer closed all three findings on this revision. |

No new real-account session or test run accompanied this decision. The developer
package still requires this Mac’s installed libmpv. No remote CI, merge or
release is recorded by this acceptance.
