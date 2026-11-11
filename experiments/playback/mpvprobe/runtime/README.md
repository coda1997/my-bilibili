# Pinned macOS runtime corrections

This is a manual, local-only recipe for the approved RPL-001 build amendment
and the owner-requested RPL-002 CoreAudio cleanup repair of 2026-11-11.
The first patch supplies the existing CoreFoundation helpers when CoreAudio is
enabled and Cocoa is disabled. The second replaces the delayed dispatch block
with an owned timer thread that teardown cancels and joins before library
unload. It preserves the seven-second idle stop and cancels that timer on resume.
The cleanup repair awaits separate independent review. Keep the original failed
sources, builds and evidence. This recipe
does not download dependencies or authorize installation, native execution or
later probe stages by itself.

The exact inputs are:

| Input | Identity |
| --- | --- |
| mpv | v0.42.0, commit `b0841e8c4d156b8403dc1876c4cbd9011eb583f3`, client API 2.6 |
| mpv source archive | SHA-256 `bcfebbfa2b65a6d88dc97ea18117831072373e38b5785a2afb874ed4df5070dd` |
| [Exact patch](mpv-0.42.0-coreaudio-helpers.patch) | SHA-256 `53750ee31165d0e188f9f9a6374d8b141b0c08cfc8938d0dcd72c7500aa18e1a` |
| [Cleanup patch](mpv-0.42.0-coreaudio-cleanup.patch) | SHA-256 `a19e753cfb85223caf7a6ce15373e7aa96aecb1e52be22e974d70ad7f2568c4a` |
| Meson 1.9.1 wheel | SHA-256 `f824ab770c041a202f532f69e114c971918ed2daff7ea56583d80642564598d0` |
| Ninja 1.13.0 wheel | SHA-256 `fa2a8bfc62e31b08f83127d1613d10821775a0eb334197154c4d6067b7068ff1` |
| libplacebo v7.387.0 archive | SHA-256 `14ceeeb14d1c08fcdb1a782a06129da11f842c6fddbf3a43c1c4061406ad03d2` |
| Vulkan compile-only headers archive | SHA-256 `4a078be12bef21cfebc09d878b77a63cff9d68f899254a0b00d0e37ef73e7f7e` |

The authorized environment uses the existing local libplacebo build with GPU
backends disabled, FFmpeg 9.0.1 and libass 0.17.5. Record the actual transitive
artifact hashes and Homebrew receipts/SBOMs; version labels alone do not
identify them. No GPU backend is enabled by providing compile-only headers.

## Fresh source and strict build

Verify the archive and patch hashes above. Safely extract the archive into a
fresh ignored directory; reject absolute/traversal paths and links. Record a
manifest of every source file's path, mode, size and SHA-256. Before applying
the patch, retain the unpatched strict-link failure on `cfstr_from_cstr` and
`cfstr_get_cstr`. Apply the exact patch with zero fuzz:

```sh
patch --batch --fuzz=0 -p1 -d "$mpv_source" < "$mpv_patch"
patch --batch --fuzz=0 -p1 -d "$mpv_source" < "$mpv_cleanup_patch"
```

Compare before/after source manifests. Only `meson.build` and
`audio/out/ao_coreaudio.c` may change, by these exact patches. The helper patch
adds exactly four lines. The actual build must compile and link
`osdep/utils-mac.c` exactly once. With Cocoa enabled, the existing Cocoa block
owns that source; the new conditional must not add another copy.

Set these absolute paths to already verified inputs and fresh outputs:

```sh
runtime_root="$PWD/.agent/runtime/mpv-0.42.0"
repair_root="$runtime_root/coreaudio-cleanup-r1"
mpv_source="$repair_root/source/mpv-0.42.0"
mpv_build="$repair_root/build"
mpv_prefix="$repair_root/prefix"
mpv_patch="$PWD/experiments/playback/mpvprobe/runtime/mpv-0.42.0-coreaudio-helpers.patch"
mpv_cleanup_patch="$PWD/experiments/playback/mpvprobe/runtime/mpv-0.42.0-coreaudio-cleanup.patch"

export PYTHONPATH="$runtime_root/build-tools"
export NINJA="$runtime_root/build-tools/ninja-1.13.0.data/scripts/ninja"
export PKG_CONFIG_PATH="$runtime_root/prefix/lib/pkgconfig:/opt/homebrew/lib/pkgconfig"

python3 -m mesonbuild.mesonmain setup "$mpv_build" "$mpv_source" \
  --prefix="$mpv_prefix" --buildtype=release \
  --auto-features=disabled --wrap-mode=nodownload \
  -Dlibmpv=true -Dcplayer=false -Dbuild-date=false \
  -Dgl=disabled -Dlua=disabled -Dcoreaudio=enabled \
  -Dcocoa=disabled -Dswift-build=disabled -Dvulkan=disabled \
  -Db_lundef=true \
  -Dc_args="-I$runtime_root/Vulkan-Headers-vulkan-sdk-1.4.363.0/include"
python3 -m mesonbuild.mesonmain compile -C "$mpv_build" -j 4
python3 -m mesonbuild.mesonmain install -C "$mpv_build" --no-rebuild
```

Do not reuse an existing `repair_root`. Preserve full command/environment
records, stdout/stderr, exit statuses, compiler/SDK/tool versions and hashes,
Meson options/configuration/compile commands, linked library inventory and
installed output hashes. Keep `b_lundef=true`, CoreAudio on, and Cocoa, Swift,
Vulkan and GL off. Another build failure returns to a bounded plan; it does
not permit broadening this patch or disabling audio.

## Loader and provenance gate

Before any media or UI work, use a bounded subprocess to load the resolved,
absolute regular dylib with `RTLD_NOW | RTLD_LOCAL`. Resolve all 15 names in
[`load_darwin_arm64.go`](../load_darwin_arm64.go), call only
`mpv_client_api_version`, and require `(2 << 16) | 6`. Do not create a core,
render context or media session. Preserve loader errors and timeout/exit status.

Record dyld images before and after the load. Recursively resolve and hash all
non-system dependencies and record every actually loaded non-system image.
Identify system dyld-cache images by path and the exact OS build; unavailable
standalone system files have no fabricated file hashes. Retain source and
dependency licenses, Homebrew receipts/SBOMs, acquisition checksums, the exact
patch, before/after manifests and complete build/tool provenance. Use the
manifest format in the [command README](../../cmd/README.md).

Only successful build, loader and provenance checks permit native regression
and playback checks. Bind the exact Go source snapshot, native
binary and runtime manifest before invocation. Keep its actual loaded-image
evidence separate from loader-only preflight. Run the required full race check.
Preserve the first native outcome and the owner-authorized 200ms external
cancel bound (2026-11-11 amendment), complete disposal, picture and zero-resource
assertions. Failure ends the probe; passing requires
separately authorized independent implementation/native-evidence review before
RPL-002. Build/loader success does not establish native playback, sound or sync.

## Explicit cleanup regression

The old `coreaudio-fix-r1` library reproduces CA-CLEANUP-001: closing a paused
fixture returns zero adapter counters, then delayed native block disposal
crashes the process. Preserve that failing runtime. The cleanup repair uses
mpv's existing mutex/condition/thread primitives; `uninit` signals the worker
and joins it, so no idle callback or block descriptor outlives core destruction.
Reset/start/idle-stop share one mutex. Initialization failure joins only a
successfully created worker. The Go adapter still unloads each library handle.

After preflight, explicitly run the tagged Go test with the repaired runtime:

```sh
MY_BILIBILI_MPV_LIBRARY="$mpv_prefix/lib/libmpv.2.dylib" \
  go test -tags nativecleanup ./experiments/playback/mpvprobe \
  -run '^TestNativeCoreAudioCleanup$' -count=1 -v -timeout=120s
```

Each scenario runs in a bounded child process and stays alive eight seconds
after its final close. It checks paused/playing close, repeated pause/resume,
idle expiry followed by resumed progression, repeated sessions, zero ownership
and idempotent Close. The direct Close timing assertion also retains 200ms.
These are native lifetime tests; they do not measure physical sound or A/V sync.
Ordinary tests exclude the `nativecleanup` tag and require no native library.

Once the cleanup regressions pass, run the complete explicit `capabilities`
stage using the new runtime manifest and preserve its first result. Loading
and playing cancellation retain the externally observed 200ms bound and all
other oracles remain. A lifetime pass cannot replace missing capability cells
or independent review of the exact repair.
