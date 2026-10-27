# Keny

**Keny 1.0.4** — самостоятельный неофициальный Android-клиент и Linux/OpenWrt-бинарник на базе Extended с интегрированными исправлениями sing-box 1.14.4.

[Скачать](https://github.com/Shtorm-glitch/Keny/releases/tag/v1.0.4) · [Описание релиза](RELEASE_NOTES.md) · [Сборка](BUILDING.md) · [Лицензии и происхождение](NOTICE.md)

В 1.0.4 исправлены переключение режимов и таймер Android. Ядро сохранено из 1.0.3. Linux/OpenWrt в этом релизе не перевыпускаются; используйте файлы [1.0.3](https://github.com/Shtorm-glitch/Keny/releases/tag/v1.0.3).

## Файлы

| Файл | Платформа |
| --- | --- |
| `keny.android-arm64-v8a.apk` | Android 7.0+, ARM64 |
| `keny.linux-amd64` | Linux x86-64 |
| `keny.linux-arm64` | Linux и OpenWrt ARM64, самостоятельный статический бинарник |
| `SFA-version-metadata.json` | Метаданные обновлятора Android |
| `SHA256SUMS` | Контрольные суммы |

Android: application ID `keny.sfa`, версия `1.0.3`, versionCode `4`. Core: `1.14.4-extended-2.7.3-keny.5`. Происхождение интеграции: [CORE_UPGRADE_1.0.3.md](CORE_UPGRADE_1.0.3.md).

## Возможности

- Транспорт Call с Телемостом и VK; согласованный профиль WebRTC creator/joiner.
- NaiveProxy: клиент в Android; Linux/OpenWrt-файлы релиза 1.0.2 пересобраны 25.10.2026 с Naive outbound и статически встроенным Cronet, без отдельной libcronet.so.
- Ротация двух комнат Телемоста, независимый PSK и восстановление соединений.
- Уведомление для ручного прохождения CAPTCHA VK.
- Встроенное обновление Android из Releases этого репозитория с выбором APK по архитектуре.
- Диагностика соединения; необязательные Xposed-функции доступны при активации модуля для `keny.sfa`.

## Установка

Сверьте SHA-256, затем установите Android APK. Профили, токены, cookie и приватные комнаты в релиз не входят. APK использует имеющийся сертификат Android Debug, но собран как release без отладки; приватный ключ не публикуется. Для последующих обновлений сохраняйте пакет и подпись.

Linux и OpenWrt: выдайте бинарнику право исполнения, проверьте свою конфигурацию командой `check -c /path/to/config.json`, затем запускайте с `run -c /path/to/config.json`. Файл OpenWrt не является пакетом `.ipk`/`.apk`. Готовые конфиги и системные службы не устанавливаются автоматически.

Базовые примеры: [examples](sing-box-extended/examples). Транспорт собирается с `with_call`. Старый тип `olcrtc` не поддерживается новым Call-ядром.

## Обновления

Включите проверку обновлений в настройках Android. Для новой публикации загружайте `keny.android-<ABI>.apk` и `SFA-version-metadata.json` с `version_code` и `version_name`, совпадающими с APK. Увеличивайте versionCode в каждом следующем релизе.

## Проверки

Выполняются целевые Go-тесты Call, Android unit tests, полный lint и сборки debug/release. Точный итог опубликован в описании релиза. Успешная сборка не гарантирует работу любого оператора или внешнего сервиса; live-проверки имеют отдельные границы.

## Исходные проекты

- [SagerNet/sing-box](https://github.com/SagerNet/sing-box) — оригинальное ядро.
- [shtorm-7/sing-box-extended](https://github.com/shtorm-7/sing-box-extended) — непосредственная база ядра.
- [SagerNet/sing-box-for-android](https://github.com/SagerNet/sing-box-for-android) — исходное Android-приложение.

Keny не заявляет поддержку или одобрение исходными проектами. Авторские уведомления, GPLv3-or-later и условия использования названия сохранены.
