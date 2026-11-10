# ADR 0001: Separate durable decisions from execution evidence

Date: 2026-11-10
Status: proposed; implementation author has not supplied independent approval.

## Decision

Use Git for specifications, task definitions, architecture decisions, and
sanitized handoffs. Use controller-owned, ignored `.agent/runs/<id>/` directories
for execution evidence. Do not introduce a database or hosted orchestrator in
the foundation.

Each run freezes the spec and task JSON and records their SHA-256 digests,
the starting Git commit (or `unborn`), and dirty-tree status. Immutable numbered
events are the sole authoritative execution history. `run status` derives its
view from that history; there is no independently editable status file.

Evidence is copied or written under that run's `evidence/` directory and
referenced by relative path, byte size, and SHA-256. Changing a referenced
file or frozen input causes validation to fail. Digests detect accidental
mutation; they are not signatures or authorization boundaries.

## Consequences

- A process restart does not lose published checkpoints.
- Events use a per-run writer lock and temporary-file publication. Readers
  reject invalid or incomplete published history instead of guessing state.
- A crash can leave a writer lock. Operators inspect it before removing it;
  the tool never silently steals a lock.
- Local storage is not replicated, tamper-proof, or safe from disk loss.
- A dirty-tree run does not snapshot all source code and is not reproducible
  release evidence. Commit a reviewed baseline before production automation.
- CI copies evidence to expiring artifacts. Durable conclusions and evidence
  locators belong in reports; preserve important large artifacts separately.
- Only a future trusted controller may enforce approvals. Local run outcomes
  cannot satisfy independent review or release gates.

## Revisit when

Multiple machines need to coordinate writes, evidence retention exceeds CI
limits, or authoritative approvals must survive an untrusted worker. At that
point add a durable control-plane store and access-controlled artifact store,
without changing the meanings of specs, events, or review evidence.
