# Foundation specification

Spec ID: `0001-foundation`
Status: implementation contract; independent acceptance is still required.

## Goal

Make work discoverable, recordable, resumable at explicit checkpoints, and
locally verifiable before adding a UI or autonomous orchestration.

## Non-goals

No client features, media playback, agent scheduling, security sandbox,
automatic PR creation, merge approval, or public release. The evidence tool
records claims and files; it does not authenticate reviewers or certify claims.

## Acceptance criteria

### FND-AC-01

A fresh reader can locate the architecture, stage-zero milestones, development
protocol, task boundaries, current handoff, and next action without chat history.

### FND-AC-02

The task validator rejects unknown fields, missing acceptance coverage,
invalid or cyclic dependencies, unsafe paths, and unordered overlapping scopes.
It never executes command strings from a spec.

### FND-AC-03

A run freezes spec/task inputs and their hashes, records source provenance,
and persists numbered events and evidence references. Reading detects modified
inputs/evidence, invalid event sequences, and unsafe paths. Concurrent writers
fail safely; a closed run cannot be appended to.

### FND-AC-04

One command checks task contracts, formatting, module consistency, vet, tests,
and build. Race checking is optional. When associated with a run, both successful
and failed check results are recorded with their logs.

### FND-AC-05

CI is configured to test foundation behavior on Linux, macOS, and Windows and
retain evidence artifacts with read-only repository permissions. Configuration
alone is not evidence that remote CI passed.

### FND-AC-06

A sanitized handoff names actual verification, outstanding independent review,
untested platforms, and the next step. It must not claim stage zero is complete.

## Operational constraints

- Use Go and its standard library for the foundation.
- Use atomic publication into new event filenames and a per-run writer lock;
  do not keep an independently mutable current-state file.
- Restrict evidence references to regular files beneath the run's evidence
  directory; do not follow symlinks.
- Never overwrite a frozen input, event, or referenced evidence file.
- Dirty/unborn source runs need explicit opt-in and are not release evidence.
- A successful local run still cannot approve itself or an implementation.

## Acceptance authority

An independent reviewer plus actual CI results are required to accept M0.
The implementation author may report local check results but not manufacture
that independent approval. Spec or gate changes require separate review.
