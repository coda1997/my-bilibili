# Native Chinese danmaku over fixture playback

Completed 2026-11-17; independently approved for the native fixture scope.
Run20261117T131827Z-1118dd75d447; full lane; spec0026-p0-native-danmaku.
Exact code/native candidate: cebd4170d34cb5d0f77cbc2a27e3482d97d23686.
Remote CI not run. No remote publication or release.

## Delivered scope

The fixture player now displays Chinese scrolling and fixed comments over its
fitted video area. Visibility, opacity, size and speed controls work in the
normal layout and native fullscreen. Comments follow sampled video position,
remain still while paused and return deterministically after seeking. Accepted
part/native generation owns its comment window; a pending or failed selection
does not replace the current part's comments. Close retires the view without
needing another paint.

The pure model validates/copies normalized input, caps a window at40000 entries
and8MiB, uses deterministic non-overlapping lane lifetimes and keeps at most16
active entries. The view retains current text layouts and paints an immutable
prepared frame. Two original64-entry fixtures and strict text-free observations
support the native tests; they do not ingest live Bilibili comments yet.

## Evidence

All checks below are local Go1.27.3 on macOS arm64. Root's artifacts and logs
are under .agent/p0-overlay; controller run evidence retains the scoped results.
Native artifact hashes are in native-assignment.md; both normal and e2e bundles
were rebuilt from the exact candidate. Native stages execute the e2e fixture
bundle, not a real account.

| Claim | Evidence | Result | Scope and not tested |
| --- | --- | --- | --- |
| Complete repository checks | combined-check-formatted.log; recorded check --race at the exact candidate | PASS | All six checks; initial gofmt-only failure retained separately |
| Tagged app, driver and observation checks | combined-e2e-race.log | PASS | Uncached race checks with e2e tag |
| Validation, schedule, ownership and UI mechanisms discriminate | Component results/matrices:13 timeline,26 renderer,13 parent,29 observation,10 composition,42 driver controls | PASS,133 compiling controls | Expected assertion failures from isolated/restored mutations; author validation, not independent approval |
| Native player behavior with overlay installed | native-cebd417-full/report.json | PASS,30 checks | Playback, controls, resize/fullscreen, parts/replay and disposal; cancel85.634166ms playing /65.555167ms loading |
| Pending/failed source work and minimized close | native-cebd417-source/report.json | PASS,25 checks | Existing source ownership and no-paint minimized-close assertions retained |
| Native danmaku time, settings, geometry and cleanup | native-cebd417-danmaku/report.json | PASS,30 checks | Paused pixels identical after708.742375ms; hide removes18796 text pixels with stable background; rate1/2, seek, all settings, full/minimum viewport, pointer pass-through, overflow and P2 |
| Overlay/native disposal | All three native reports | PASS | Actual zero current layouts/entries/resources,200ms cancellation and8s still-live survival; danmaku cancel75.052166ms; every process exits gracefully, no force |
| Chinese glyph pixels | Original P1, hidden, P2, fullscreen and minimum-size PNGs; independent result | PASS | Independent reviewer recognizes 中文弹幕测试 and 第二分P弹幕, verifies hidden/old-part text absence and fitted clipping; no tofu or label-only claim |
| Independent combined code/native review | .agent/p0-overlay-code-review/result.md | APPROVE | Round2/global19, no BLOCKING or ADVISORY findings; independent ordinary/tagged race checks,72 strict native observations and pixel/timebase calculations pass |

Observed versus inferred: controlled tests and native receipts establish these
fixture behaviors for the exact built candidate. Native screenshots show the
original Chinese glyphs; current checks do not establish live segment ingestion,
discovery, quality selection or history/resume in one client. The contract relied
on is frozen0026 plus accepted0025's Source/lifetime semantics. Those established
ports remain unchanged.

## Review and retained counts

Architecture round1/global15 was approved with no findings. Fresh independent
reviewer /root/p0_overlay_code_review approved round2/global19 against the exact
candidate, with no findings or required evidence gaps. The original architecture
context had no BLOCKING finding to reassign. Counts: two test-only corrections and one
mechanical formatting correction, conservatively three total; zero production
corrections, zero native failures, no repeated underlying failure. All initial
failed/masked results remain in their evidence directories. Shared remote issue
budget0/5, no filing. No policy or CI check was changed.

## Remaining P0 work

Full P0 is still active. Real QR login and same-binary restoration are complete
in report0036; retain the Keychain session. Approved real-client revision2 now
has its required native prerequisite for publication/freeze. Its
scope is fixed read-only transport/WBI, search/link/details/covers, supported
DASH choices and generation-bound resume, segmented danmaku, exclusive local
history and one native client with actual two-launch service/resume evidence.

No new out-of-scope defect was found. The remaining real-client work is already
assigned in the P0 plan, so no remote issue or systemic follow-up is warranted.
Windows native remains P2/not-tested. Physical audibility/calibration, remote CI,
runtime distribution, signing/notarization and public release are not claimed.

Next: freeze the exact approved0027 revision and continue the already authorized
real-client implementation. No new implementation permission or QR scan is
needed for that continuation. Local runs/captures are not backed up by
Git; retain needed evidence before deleting this checkout.
