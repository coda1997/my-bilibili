# Native UI and playback spike

Spec ID: `0002-playback-spike`
Status: proposed; do not start until explicitly selected and independently reviewed.

## Goal

Decide whether native mygo UI and its player can support the first desktop
playback slice on macOS and Windows, without building the full application.

## Non-goals

No complete login implementation, real-account CI fixtures, downloading a
media library, entitlement bypass, public release, or general player framework.

## Preconditions

Select and pin the evaluated mygo revision. Obtain authorized, sanitized media
fixtures and both OS runtime environments. Document unavailable resources as
blocked or not-tested rather than assuming success.

Keep this feasibility spike serial: validate the UI harness before integrating
media surfaces and playback. Demonstrate parallel feature development in M2,
after these contracts are understood, rather than inventing premature seams.

## Acceptance criteria

### PLAY-AC-01

Record the exact dependency revision, Go version, OS/architecture, fixture
provenance, and supported test cases. Do not store authenticated URLs or cookies.

### PLAY-AC-02

A minimal native Go UI starts on both target OSes. Verify Chinese text and
input, loading/error states, and a test-observable playback command.

### PLAY-AC-03

Test target stream access, required request metadata, separate tracks when
applicable, seek, switching, cancellation, late results, and cleanup. Record
pass/fail/not-tested for each capability on each OS, with evidence.

### PLAY-AC-04

An independently reviewed decision accepts the player adapter, selects a
different implementation behind the same boundary, or rejects the proposed
stack with a specific reason. Do not infer platform support from compilation.

## Result location

Raw sanitized evidence belongs to the selected run. The durable capability
matrix and decision belong in `reports/0002-playback-spike.md` and a follow-up
ADR. Those results do not exist yet.
