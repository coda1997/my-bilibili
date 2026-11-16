# M2 desktop interaction contract

- Native window starts at 1120x760, minimum 800x600. At content width 1000 or
  greater, parts appear beside video; below 1000 they appear beneath it. Cover,
  title, creator and description remain available in the normal page. Video is
  16:9 with proportional fit/black letterboxing; no native session re-creation
  on layout changes.
- Use a restrained Bilibili-inspired palette and native system typography.
  Keep the video dominant. The bottom overlay has a timeline above a controls
  row: play/pause, next part and elapsed/total time on the left; parts, speed,
  mute/volume and fullscreen on the right. Distinguish selected/requested part
  from actually playing part when selection failed.
- Parts have visible titles/current state. Speed options are 0.5, 0.75, 1, 1.25,
  1.5 and 2.0. Seek/volume use drag state separate from native sampled state;
  redraws and old samples cannot emit duplicate commands. Release outside a
  slider completes capture correctly. Part replacement resets drag state.
- Single video click toggles playback; double click toggles fullscreen without
  an additional playback toggle. Delay single-click dispatch through the
  platform double-click recognition window; inject timing for headless tests.
  Control/menu clicks cannot propagate into video-click actions.
- Space toggles play; left/right seek -/+5s clamped to the timeline; up/down
  adjust volume +/-0.05 clamped to [0,1]; M toggles mute; F toggles fullscreen.
  Esc dismisses a popup first and otherwise leaves fullscreen. Do not steal
  keyboard input from focused widgets; their own keyboard behavior takes priority.
- Hide the overlay after three seconds of inactivity only while Playing. Keep
  it visible while paused/loading/failed/ended, or when hovered/focused/dragging
  or a popup is open. Invalidate/schedule using the injected clock; no timer
  goroutine. Expose a meaningful loading/error/retry presentation.
- Fullscreen uses the native window API and its actual-state callbacks. Show
  video and overlay with a part menu; preserve session, position, intent,
  settings and errors through repeated entry/exit. Respect native exit actions.
- All interactive controls have readable labels/tooltips, visible keyboard
  focus and >=28pt hit targets. Cover uses an accessible description. Use clear
  enabled/disabled states. No diagnostic JSON or library paths in normal UI.

Headless tests cover event routing and state transitions; actual macOS review
establishes visible layout, picture/audio, native fullscreen and keyboard behavior.
