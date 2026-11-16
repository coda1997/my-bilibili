# Original M2 fixture resources

`fixture.New(resourceDir)` expects these five files directly in the directory:
`part-1.mp4`, `part-2.mp4`, `cover.png`, `metadata.json`, `manifest.json`.
The composition/package layer copies `assets/` to the application's resource
directory and supplies that directory; the adapter does not search the cwd.
The opaque IDs are `preview-video`, `part-1`, `part-2`, and `preview-cover`.
Composition loads the known cover filename when drawing that cover ID.

These files contain original synthetic patterns and tones created locally on
2026-11-16 with FFmpeg 9.0.1 (Apple clang 21.0.0). No downloaded media, personal
recordings or service requests were used. Part 1 has a pink rectangle and a
440 Hz tone; part 2 has shifted colors, a blue rectangle and a 660 Hz tone.
Both clips have 640×360 H.264 video at 24 fps and 48 kHz AAC audio. FFprobe
reported exactly 30.000000 seconds for each container. This is asset inspection,
not native player playback, perceived audibility or physical calibration.

Generation commands, run from the repository root:

```sh
ffmpeg -hide_banner -loglevel error \
  -f lavfi -i 'testsrc2=size=640x360:rate=24:duration=30' \
  -f lavfi -i 'sine=frequency=440:sample_rate=48000:duration=30' \
  -vf 'drawbox=x=24:y=24:w=110:h=44:color=0xFB7299:t=fill' \
  -c:v libx264 -preset veryfast -crf 30 -pix_fmt yuv420p \
  -c:a aac -b:a 64k -movflags +faststart -shortest \
  internal/fixture/assets/part-1.mp4
ffmpeg -hide_banner -loglevel error \
  -f lavfi -i 'testsrc2=size=640x360:rate=24:duration=30' \
  -f lavfi -i 'sine=frequency=660:sample_rate=48000:duration=30' \
  -vf 'hue=h=120,drawbox=x=506:y=24:w=110:h=44:color=0x00AEEC:t=fill' \
  -c:v libx264 -preset veryfast -crf 30 -pix_fmt yuv420p \
  -c:a aac -b:a 64k -movflags +faststart -shortest \
  internal/fixture/assets/part-2.mp4
ffmpeg -hide_banner -loglevel error \
  -f lavfi -i 'color=c=0x1D2233:s=640x360,drawbox=x=0:y=255:w=640:h=105:color=0xFB7299:t=fill,drawbox=x=80:y=70:w=160:h=130:color=0x00AEEC:t=fill,drawbox=x=320:y=90:w=220:h=90:color=0xFFD166:t=fill' \
  -frames:v 1 internal/fixture/assets/cover.png
```

`metadata.json` records the synthetic detail and part durations. `manifest.json`
records the exact file sizes and SHA-256 hashes. The adapter embeds that manifest
as its trust anchor, checks the adjacent manifest matches, and verifies each
regular file before construction. Resolve rechecks the requested part before
returning a fixed absolute local path. Unknown IDs cannot become filenames.
Missing, replaced, malformed or symlinked resources return safe playback errors
without exposing resource paths. A deliberate future fixture replacement must
update the assets and embedded manifest together as a reviewed source change.

FFmpeg/FFprobe are asset-authoring/inspection tools only. They are not required
by `fixture.New`, the packaged app, or default Go tests. Encoder output can vary
with toolchain builds; new output needs a new reviewed integrity manifest.
