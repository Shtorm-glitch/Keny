# Building Keny 1.0.2

Use the source snapshot tagged `v1.0.2`, Go 1.26.4, Android SDK/NDK and Android Studio JBR (the Android project requires Java 17 or newer). All dependency versions are pinned by the supplied Go and Gradle files. Android's Gradle wrapper is included. Configure ANDROID_HOME/ANDROID_SDK_ROOT for your SDK and JAVA_HOME for JBR. On Windows invoke `gradlew.bat`.

## Linux x86-64

Run in `sing-box-extended`:

```sh
go test -tags with_call ./protocol/call ./transport/call/...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath \
  -tags with_call,with_quic,with_gvisor,with_utls,with_clash_api,with_wireguard \
  -ldflags '-s -w -X github.com/sagernet/sing-box/constant.Version=1.14.1-extended-2.7.2-keny.4' \
  -o ../keny.linux-amd64 ./cmd/sing-box
```

The transport build tag is `with_call`, not `with_olcrtc`.

For Linux and OpenWrt ARM64, use `GOARCH=arm64` and output `keny.linux-arm64`. This shared standalone CGO-disabled binary is built with these tags; it is not an ipk/apk package.

## Android ARM64

Keep both source directories as siblings. In `sing-box-extended`, build the release AAR with the supported builder:

```sh
KENY_VERSION=1.14.1-extended-2.7.2-keny.4 go run ./cmd/internal/build_libbox -target android -platform android/arm64
```

Create the sibling Android project's app/libs directory before running the builder. The builder uses the Go mobile toolchain and copies the generated libraries there when that directory exists. Generated AARs are intentionally omitted from Git; rebuild them from the included sources.

Then run from `sing-box-for-android`, with JAVA_HOME explicitly pointing to Android Studio JBR:

```sh
./gradlew :app:testOtherDebugUnitTest :app:assembleOtherRelease
```

Version properties are set to code 3 / `1.0.2`; application ID is `keny.sfa`. Release ARM64 output is `keny.android-arm64-v8a.apk`. If release signing properties are absent, the project falls back to the local Android debug keystore. Your own key will produce a different certificate and cannot update the published package in place. The published signing key is not included. See app/build.gradle.kts for supported release signing properties.

Verify APK certificate and manifest with Android SDK apksigner/aapt. Build success does not establish live Telemost or WB Stream compatibility; those require your own authorized configuration and separate end-to-end checks.
