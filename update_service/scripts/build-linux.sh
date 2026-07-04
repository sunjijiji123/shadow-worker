#!/usr/bin/env bash
# 交叉编译 Linux 版 update_service
# 用法：bash scripts/build-linux.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist/linux-amd64"

rm -rf "$DIST"
mkdir -p "$DIST"

cd "$ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o "$DIST/server" ./cmd/server

cp -r "$ROOT/web" "$DIST/"
cp "$ROOT/config.example.yaml" "$DIST/config.yaml.example"

echo "Build OK: $DIST/server"
