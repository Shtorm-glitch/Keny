# Keny 1.0.3 core integration

Core: `1.14.4-extended-2.7.3-keny.5`, Go 1.26.10.
This is a Keny integration, not an official Extended 1.14.4 release.

Source inputs:

- Previous Keny: Extended v1.14.1-extended-2.7.2,
  `55faa763f986f4ca8a492d9b2719bc6330d2bef5`, plus published Keny 1.0.2 patches.
- Updated Extended v1.14.3-extended-2.7.3,
  `d9442d14b19ddd02605bf3ee893a171947038e19`.
- sing-box upstream tag v1.14.4, commit prefix `c005f35300`.
- `third_party/sing`: Extended v0.9.7-extended-1.2.1 `fe045639087fe73d1412b99f09486bbef96433db`
  merged with SagerNet `5dc452e537bf75c5c51c54c8c4a4be1d19d85c4f`.
- `third_party/sing-vmess`: Extended v0.2.9-extended-1.0.0 `567e5235b0dec975efb8d324e60a178eb2be52a7`
  merged with SagerNet `a0c58542bd9d99b00ea0c3cfacce6f03cf8a4017`.
- `third_party/wireguard-go`: Extended v0.0.8-extended-1.6.2 `046031366eeecef762d47cf66919cd6d05a9c37c`
  merged with SagerNet `e4d85ef2ed387cc07233b5324d4a40f3df94566e`.

The local module replacements preserve Extended APIs/configuration extensions.
Module licenses and copyright notices remain alongside the sources.

Preserved Keny behavior: async DNS dispatch preventing stalled DNS from blocking
TUN; Telemost rotation/durable sessions/PSK/HTTP-session fixes; VK CAPTCHA and
matching WebRTC profiles; gRPC-lite path fix; Android ID/signing/UI/updater;
Naive/Cronet on Android and static Linux/OpenWrt binaries.

Integration resolutions:

- Call Scope cleanup retains Keny's cancellation and bridge-publication locks.
- Extended provider flags, DNS loop detection and configurable WireGuard timers
  remain; expiry calculations now account for device sleep.
- Vision retains encryption/splice handling and gains atomic direct-write state
  and raw-connection close after direct writes.
- Fallback groups retain their retrying dialer under the new group resolver.
- WireGuard tests use current Extended APIs; short unpadded transport packet
  recognition with configured padding is fixed.

Compatibility review:

- No automatic DNS, route, IPv6, strict_route or authentication changes.
- Latvia2 remains stock Caddy -> loopback h2c -> Naive inbound -> direct;
  backend has no TUN, WireGuard or rule sets.
- Android profiles require validation against the rebuilt libbox.
- Existing 1.14 complex-rule-set matching remains; check DNS/route choices with
  multi-rule sets. DNS address filters now apply to cache hits as fixed upstream.
- Removal of unimplemented hot-reload declarations does not remove working
  full-configuration hot reload.

Validation: native amd64 and ARM64/QEMU Naive/TLS/1MiB, targeted DNS/route/Call/gRPC,
Vision close and WireGuard tests, Android unit tests and signed release build.
Live Call service availability is separate from these regression checks.
