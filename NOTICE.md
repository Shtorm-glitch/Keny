# Source lineage and licensing

Keny is an unofficial derivative distribution, not an official SagerNet or sing-box-extended release.

- Core: https://github.com/shtorm-7/sing-box-extended at commit `55faa763f986f4ca8a492d9b2719bc6330d2bef5` (`v1.14.1-extended-2.7.2`), with the modifications included in this repository.
- Original core: https://github.com/SagerNet/sing-box, copyright nekohasekai and contributors.
- Android application: derived from https://github.com/SagerNet/sing-box-for-android, with the Keny modifications included here.
- Call transport incorporates work from https://github.com/kulikov0/whitelist-bypass. Transitive dependencies of the current core are recorded in its go.mod/go.sum and retain their respective copyright and license notices.

The corresponding modified core and Android source are included in `sing-box-extended/` and `sing-box-for-android/` at the `v1.0.0` tag. Build instructions are in BUILDING.md. Keny 1.0 includes the VK fixes, Android update integration and the independent Android application ID `keny.sfa`.

Preserve all upstream copyright notices, GPLv3-or-later terms and the naming/association condition in the component LICENSE files. Third-party dependencies retain their respective licenses. Keny does not imply upstream affiliation or endorsement.

The APK uses an Android Debug certificate. It is a non-debuggable release build; no signing private key is distributed.

Upstream SSH example JSON files containing demonstration private keys have been omitted. This does not change executable source. Component copyright and license notices are preserved; this root notice and README describe Keny 1.0.
