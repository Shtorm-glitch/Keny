# Keny 1.0.3

- Интегрированы исправления sing-box **1.14.4** поверх Extended **2.7.3**, с сохранением доработок Keny.
- Core **1.14.4-extended-2.7.3-keny.5**, Go **1.26.10**.
- Исправления DNS, TUN/TCP-сессий, HTTP/2/gRPC, восстановления сети, Vision и WireGuard.
- Сохранены async DNS/TUN fix, Call/VK/Телемост, ротация/PSK, CAPTCHA, простой интерфейс и обновлятор.
- Naive/Cronet: Android и статические Linux/OpenWrt binaries, без внешней libcronet.so.
- Android **keny.sfa**, **1.0.3 / code 4**, ARM64 release, прежняя подпись.

Файлы: keny.android-arm64-v8a.apk, keny.linux-amd64, keny.linux-arm64, SFA-version-metadata.json, SHA256SUMS.
ARM64 binary подходит Linux/OpenWrt ARM64, это не пакет ipk/apk.

Проверки: Naive/TLS/1MiB на amd64 и ARM64/QEMU, DNS/route/Call/gRPC, Vision и WireGuard tests, Android unit tests и signed release build.
Происхождение: [CORE_UPGRADE_1.0.3.md](https://github.com/Shtorm-glitch/Keny/blob/v1.0.3/CORE_UPGRADE_1.0.3.md).

Проверяйте SHA256SUMS. Конфиги, пароли и комнаты не входят в релиз; маршрутизация автоматически не меняется.
