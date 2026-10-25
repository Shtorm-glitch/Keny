# Building Keny 1.0.3

Use the source snapshot tagged `v1.0.3`, Go 1.26.10, Android SDK/NDK and Android Studio JBR (the Android project requires Java 17 or newer). All dependency versions are pinned by the supplied Go and Gradle files. Android's Gradle wrapper is included. Configure ANDROID_HOME/ANDROID_SDK_ROOT for your SDK and JAVA_HOME for JBR. On Windows invoke `gradlew.bat`.

## Linux x86-64 and Linux/OpenWrt ARM64 with Naive

The Linux assets in release 1.0.3 were rebuilt on 25 October 2026 with Naive
outbound and statically linked Cronet. No separate `libcronet.so` is needed.
The core and Android version numbers remain unchanged. Use the build script
from the commit recorded in the updated release notes; the original `v1.0.3`
tag predates this build-system change.

On Linux, prepare the Chromium/musl toolchain at the exact cronet-go revision
pinned by `sing-box-extended/go.mod` (currently `0d28acc44093`):

```sh
git clone https://github.com/sagernet/cronet-go.git /path/to/cronet-go
cd /path/to/cronet-go
git checkout 0d28acc44093
git submodule update --init --depth 1 naiveproxy
go run ./cmd/build-naive --target=linux/arm64 --libc=musl download-toolchain
eval "$(go run ./cmd/build-naive --target=linux/arm64 --libc=musl env --export)"
cd /path/to/Keny
bash scripts/build-naive-linux.sh arm64
python3 scripts/test-naive-linux.py dist/keny.linux-arm64
```

The transport build tag is `with_call`, not `with_olcrtc`.

Use `amd64` instead of `arm64` for x86-64. When running the ARM64 test on x86-64,
install qemu-user and set `KENY_TEST_RUNNER=qemu-aarch64` for the Python command.
The workflow `.github/workflows/naive-linux.yml` builds and checks both targets.
It verifies static linkage, then transfers 1 MiB through the actual Naive outbound
with TLS certificate verification. It uploads artifacts without publishing them.
Call regression tests also run for amd64. ARM64 is a standalone static musl binary,
not an ipk/apk package. Build tags include `with_naive_outbound,with_musl` in addition
to the existing Call/QUIC/gVisor/uTLS/Clash API/WireGuard features.

## Android ARM64

Keep both source directories as siblings. In `sing-box-extended`, build the release AAR with the supported builder:

```sh
KENY_VERSION=1.14.4-extended-2.7.3-keny.5 go run ./cmd/internal/build_libbox -target android -platform android/arm64
```

Create the sibling Android project's app/libs directory before running the builder. The builder uses the Go mobile toolchain and copies the generated libraries there when that directory exists. Generated AARs are intentionally omitted from Git; rebuild them from the included sources.

Then run from `sing-box-for-android`, with JAVA_HOME explicitly pointing to Android Studio JBR:

```sh
./gradlew :app:testOtherDebugUnitTest :app:assembleOtherRelease
```

Version properties are set to code 4 / `1.0.3`; application ID is `keny.sfa`. Release ARM64 output is `keny.android-arm64-v8a.apk`. If release signing properties are absent, the project falls back to the local Android debug keystore. Your own key will produce a different certificate and cannot update the published package in place. The published signing key is not included. See app/build.gradle.kts for supported release signing properties.

Verify APK certificate and manifest with Android SDK apksigner/aapt. Build success does not establish live Telemost or WB Stream compatibility; those require your own authorized configuration and separate end-to-end checks.
