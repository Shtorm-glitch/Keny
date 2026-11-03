# Keny 1.0.3 with Admin Panel and Manager

Base: release v1.0.3, commit 40c86ab5f98a58935fae30a947e8ff39a2ae2722.
Binary version: 1.14.4-extended-2.7.3-keny.5-admin.1.

The build preserves Call, QUIC, gVisor, uTLS, Clash API, WireGuard,
Naive/Cronet and static musl linkage. It adds with_admin_panel and
with_manager. The embedded SPA is built before compiling Go.

Workflow: .github/workflows/naive-linux.yml (manual dispatch).
Targets: Linux ARM64 and AMD64; ARM64 executes through QEMU during checks.
Checks include static ELF linkage, real Naive TLS transfer, version tags,
isolated loopback SPA assets and SQLite Manager startup, and native Go
regressions for the panel, Manager, APIs, Call and existing core features.

No device installation, production configuration changes or release
publication are part of this build. Enabling services requires a separate
configuration change. A successful build does not prove compatibility
with production profiles or live Call platforms.
