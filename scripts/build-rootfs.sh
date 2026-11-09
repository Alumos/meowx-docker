#!/bin/bash
# 备好镜像里的程序：scripts/build-rootfs.sh <版本>
# 产物在 docker/rootfs/：控制服务、内核（CORE_BIN 可指向已编好的）、官方 tailscale、geo 数据。只做 amd64。
# 平时由 GitHub Actions 跑（.github/workflows/build.yml），本机不用跑。
set -euo pipefail
VERSION="${1:?用法：$0 <版本>}"
GOARCH=amd64
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ROOTFS="$ROOT/docker/rootfs"
rm -rf "$ROOTFS"
mkdir -p "$ROOTFS/usr/bin" "$ROOTFS/data"

(cd "$ROOT/agent" && CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$ROOTFS/usr/bin/meowx-agent" .)
if [ -n "${CORE_BIN:-}" ]; then
    cp "$CORE_BIN" "$ROOTFS/usr/bin/meowx-core"
else
    "$ROOT/scripts/build-core.sh" "$GOARCH" "$ROOTFS/usr/bin/meowx-core"
fi
"$ROOT/scripts/fetch-tailscale.sh" "$GOARCH" "$ROOTFS/usr/bin" >/dev/null
"$ROOT/scripts/fetch-geo.sh" "$ROOTFS/usr/share/meowx/geo" >/dev/null
ls -la "$ROOTFS/usr/bin"
