# Inspected protocol facts

Inspected 2026-11-17 through GitHub contents API at
pskdje/bilibili-API-collect commit cfc5fddcc8a94b74d91970bb5b4eaeb349addc47.
This archived CC-BY-NC-4.0 documentation is a research reference only. No
implementation, fixture, schema or runtime dependency is copied. The probe
uses original Go protocol parsing and wholly synthetic fixtures.

- docs/login/login_action/QR.md: web generate and poll use passport.bilibili.com;
  generate returns URL/qrcode_key, challenge lifetime described as 180 seconds;
  pending/scanned/expired/success poll states are 86101/86090/86038/0.
- docs/danmaku/danmaku_proto.md: the legacy unsigned web/seg.so and newer WBI
  route are distinct; six-minute segments are partly available anonymously.
  Partial anonymous visibility must not be reported as a complete danmaku corpus.
- grpc_api/bilibili/community/service/dm/v1/dm.proto: top-level elems field 1,
  state field 2; entry progress 2 (milliseconds), mode 3, content 7. Default
  scalar progress/state can be absent at zero. Modes 1/2/3 are scrolling, 4/5
  fixed, 6 reverse, 7 advanced, 8 code, 9 BAS. The probe counts only 1/4/5/6
  as its proposed initial rendering subset; other modes are unsupported.
- specs/0008-authorized-target-playback/access-method.md binds the existing
  anonymous target to BV1Zs411o7Ee / AID 899574 / CID 1445374. No media resolver
  or signed URL is used in this new probe.

These facts do not establish current service availability. The single later
explicit live invocation supplies that evidence, after code review. No account
secrets, actual QR material or raw service response is retained here.
