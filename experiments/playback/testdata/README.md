# Synthetic media

Intended framework pin: `github.com/egoist/mygo` at
`v0.8.4-0.20261110145816-daf6341a2fe0` (MIT root license). The real UI import
will add this requirement in PLAY-002, not an unused pin in PLAY-001.

These are original generated geometric pictures and a synthetic 440 Hz tone,
not Bilibili content or copied upstream example media. Regeneration is explicit:

```sh
go run ./experiments/playback/testdata/generate.go
```

Requires the separately installed FFmpeg/ffprobe. Ordinary tests and playback
do not invoke either. `manifest.json` records exact commands, tool versions,
metadata and hashes. Regeneration may change encoded bytes between versions.
The green field/red rectangle/blue rectangle is the native rendering oracle;
the UI outside the video must not use these saturated fixture colors.
