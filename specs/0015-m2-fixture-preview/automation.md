# M2 evidence observation contract

Use the repository's existing e2e-build pattern for product automation. The
normal build exposes no automation plugin or diagnostic UI. Test keys remain
stable: `preview-video`, `play-toggle`, `next-part`, `seek-slider`, `parts-menu`,
`part-part-1`, `part-part-2`, `speed-menu`, `mute-toggle`, `volume-slider`,
`fullscreen-toggle`, `retry-selection`, and `retry-playback`. Prefix speed keys
with `speed-` and use `0.5`, `0.75`, `1`, `1.25`, `1.5`, `2` as suffixes.

An e2e-only observation includes app State, actual fullscreen state and native
ownership counters, plus the existing accepted track/picture observation where
needed. It contains no media URL, file path, headers, cookies or source payload.
The driver validates required fields and types; absent/malformed state is never
treated as idle, zero ownership or success. Use sampled native state and actual
frame evidence, not command echoes, to establish progress and picture.

M2-004 supplies a bounded local-fixture invocation assignment identifying the
exact app/driver/runtime/assets, owned process/window, timeout and cleanup. The
product smoke exercises startup pause, both parts, seek/rate/volume/mute, ended
replay, repeated fullscreen and graceful close; loading cancellation requires
a controlled delayed fixture seam, never a real service request. Native
cancellation times span initiating the action through observed disposed state.
Provide an e2e-only dispose-playback action that calls Player.Close without
quitting the app; observe zero ownership and survival in the still-live process
for eight seconds to expose delayed native cleanup failures. A separate fresh
invocation closes the actual window and checks graceful process exit. Waiting
after an already-exited process is not post-close survival evidence. Repeated
fullscreen validation performs at least three complete entry/exit cycles.
Capture sanitized findings and relevant artifacts in the controller's evidence.

Keep the existing 41-assertion laboratory fixture suite as adapter regression;
it is not a substitute for the new product UI/package tests. Never label
unavailable picture/audio/UI/platform evidence as passing based on Go tests or
cross-compilation. Human audibility observations and their limits are explicit.
