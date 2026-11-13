# Current assignment context / TGT-001 through TGT-003

Updated 2026-11-13. Controller: `/root`; independent reviewers must remain
separate from implementation authors. Owner objective: M1 evaluation with the
selected public test video. The owner has now explicitly removed origin policy
and allowed all hosts; [ADR 0007](../../docs/adr/0007-target-host-policy.md) is the
current decision. [HOST-001](../0009-target-host-removal/spec.md) owns that amendment.

## Identity and status

- Checkout: `/Users/dadachen/.codex/worktrees/8f0c/my-bilibili`.
- Base HEAD: `d846e667c537c06a1bcf100e6aac9486c29aa3d2`, dirty, not candidate identity.
- Prior TGT-001/TGT-002 review and frozen candidate identities are in
  [report 0021](../../reports/0021-target-access-method.md) and run
  `20261112T144351Z-ac60cbfe2590`. Historical assignments remain in that run.
- Current host-removal run: `20261113T005629Z-2911e43f08da`, with the separately
  reviewed/frozen 0009 contract. Its exact source/check/review identity and next
  action are in [report 0022](../../reports/0022-target-host-removal.md).
- TGT-003 native target playback remains not-tested. Earlier fixed observations
  and the diagnostic are exhausted; this document creates no new request allowance.
- Standing native delegation applies. No commit, remote publication, managed
  worktree creation, runtime change or M1 acceptance is authorized here.

## Current boundaries

Read README, stage-zero, architecture, development, testing, workflow, 0008 spec,
access-method, ADR 0007, 0009 and the current handoff. Preserve source/part,
rendition, grant, public-only headers, TLS certificate verification, expiry,
private stdin transport and complete ownership cleanup. All syntactically valid
HTTPS hosts and valid ports are allowed; no hostname/IP/localhost restrictions,
role-origin list, historical-origin comparison, or per-host review remains.

Descriptor version 2 omits origins; the updated helper and player share
`targetprobe.ValidateMediaURL`. Build matching helper/app/driver versions and
freeze their identities before any later target session. Old helper paths and
binaries are historical artifacts, not the current continuation. The new helper
is `.agent/worker-scratch/target-refresh-unrestricted/`; its default is offline.
Keep all media locators and hostnames out of public evidence.

## Review and continuation

Independent HOST-001 review assesses exact code, helper integration, unchanged
TLS/source/grant/lifecycle rules, tests and documentation. Current maintained
write paths are in the 0009 task; old TGT assignments do not broaden them.
Reviewers use assigned scratch and return sanitized evidence to the controller,
who alone writes its run. Follow the independent-review skill and review template.

After this amendment, TGT-003 still needs a separately recorded bounded execution
assignment, fresh input, relevant native fixture regression, one target session
and independent evidence disposition. Then final adapter assessment and owner
acceptance can address M1. Windows is optional P2; calibrated physical sync/rate
measurements are outside scope. No host amendment is needed for a new CDN.
