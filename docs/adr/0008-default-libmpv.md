# ADR 0008: libmpv is the default playback backend

- Date: 2026-11-13
- Status: owner selected; implements the explicit request to set libmpv as default
- Scope: default playback launcher, existing mygo UI and plain playback contract

## Decision

Use the reviewed libmpv adapter for `experiments/playback/cmd/native`.
Keep mygo for the UI, controls and bitmap presentation. The earlier provisional
mygo media adapter is available only through the explicit `cmd/mygo-native`
diagnostic launcher. This supersedes ADR 0004's temporary default selection.
There is no automatic fallback between players.

The normal launcher requires `-library` with an absolute path to the verified
libmpv API 2.6 library. The current evaluated runtime is mpv v0.42.0 with the
reviewed CoreAudio cleanup patch; its dependencies and hashes do not change.
Runtime provenance is a prerequisite; the launcher does not download, install
or discover libraries. The existing loader rejects incompatible libraries.
Use local generated fixtures by default, or an explicitly supplied owned
loopback server for the existing header/separate-track fixture controls.

The dedicated target path retains descriptor version 2 and all-valid-HTTPS-host
policy under ADR 0007. Selecting the default engine does not broaden the
fixture launcher's inputs or initiate service requests.

## Evidence and consequences

The prior independent M1 assessment recommended this adapter after reviewed
macOS fixture, cleanup, human fixture-audibility and bounded target evidence;
see [the previous handoff](../../reports/0021-target-access-method.md#post-repair-target-continuation-2026-11-13).
The default launcher change has its own contract and verification in
[spec 0011](../../specs/0011-default-libmpv/spec.md) and
[the default-switch handoff](../../reports/0024-default-libmpv.md).

The old mygo playback defects remain documented for the explicit legacy path.
Historical failures and reviews retain their original scope. The owner has
selected the default adapter; M1 was accepted as completed on 2026-11-13.
This change does not start M2/P0 or a full client shell. Windows remains optional
P2/not-tested; packaging/signing, real IME and changed-candidate CI remain
separate work. Calibrated physical measurements remain outside M1 under ADR 0006.

Launch and verification commands are in the
[playback command guide](../../experiments/playback/cmd/README.md#default-libmpv-playback).
