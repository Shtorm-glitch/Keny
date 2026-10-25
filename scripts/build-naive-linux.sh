#!/usr/bin/env bash
set -euo pipefail
arch=${1:?usage: build-naive-linux.sh amd64|arm64}
case "$arch" in amd64|arm64) ;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
: "${CC:?Configure the pinned cronet-go musl toolchain first}"
: "${CXX:?Configure the pinned cronet-go musl toolchain first}"
root=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$root/dist"
cd "$root/sing-box-extended"
CGO_ENABLED=1 GOOS=linux GOARCH="$arch" go build -buildvcs=false -trimpath \
  -tags with_call,with_quic,with_gvisor,with_utls,with_clash_api,with_wireguard,with_naive_outbound,with_musl \
  -ldflags '-s -w -linkmode external -extldflags "-static" -X github.com/sagernet/sing-box/constant.Version=1.14.4-extended-2.7.3-keny.5' \
  -o "$root/dist/keny.linux-$arch" ./cmd/sing-box
