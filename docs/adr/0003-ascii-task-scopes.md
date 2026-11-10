# ADR 0003: Restrict declared task write scopes to ASCII

Date: 2026-11-10
Status: implemented and independently technically reviewed in PR #1; adopted
as task-scope policy by the owner on 2026-11-10. See the
[adoption record](../../reports/0003-policy-adoption.md) and the
[publication evidence](../../reports/0003-publication.md).

## Problem

COR-002 demonstrates that NFC/NFD spellings can name the same file on macOS
while the lexical overlap check treats them as different scopes. Tasks can
name future files, so comparing only existing filesystem entries cannot fix
the contract. The foundation must stay Go-standard-library-only.

## Decision

For `tasks.json` `write_paths` only, reject any non-ASCII rune before comparing
scopes. Keep the existing safe-path grammar and case-insensitive comparison.
This applies even when tasks have a dependency ordering: an invalid spelling
does not become valid because tasks are serial. Retain ordered overlap for
valid ASCII paths.

Leave `ValidPath` and its spec/evidence callers unchanged. This restriction is
not a blanket ban on Unicode files, checkout paths, Chinese text, or UI input.
It does not normalize or rename files and does not invalidate old run records.

## Alternatives and consequences

- Unicode normalization would require an additional normalization dependency
  or maintaining Unicode tables, neither justified for this foundation.
- Rejecting only combining marks misses precomposed or other equivalent
  spellings. A small ad hoc normalizer would be a misleading guarantee.
- The restriction is intentionally narrower than future product needs. For a
  Unicode source filename, an ASCII parent directory scope may be used only
  when that broader ownership is deliberate, assigned, and reviewed.
- Lexical validation still does not enforce actual diffs, filesystem aliases
  such as symlinks/hardlinks, or worker isolation. Do not claim a sandbox.
- Existing task manifests use ASCII scopes. Test Unicode evidence/spec paths
  separately so this fix cannot accidentally narrow shared path validation.

## Verification and adoption

Cover NFC/NFD exact-file and subtree scopes, both unordered and ordered tasks;
cover valid ordered ASCII overlap and invalid unordered case aliases. Retain
the native macOS reproduction as independent supporting evidence.

Implementation and an independent technical recommendation do not constitute
human policy approval. The owner separately supplied that approval on
2026-11-10, adopting all five enumerated policies in the adoption record.
This does not implement autonomous enforcement or permit later policy changes
to approve themselves. The owner separately accepted M0 on the same date;
that decision is also recorded in the adoption record.
