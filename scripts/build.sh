#!/bin/sh
set -eu

assets_dir="internal/webui/assets"
[ -d "$assets_dir" ] || { echo "缺少前端资源目录: $assets_dir" >&2; exit 1; }
find "$assets_dir" -mindepth 1 ! -name _placeholder -delete
npm --prefix web run build
[ -f "$assets_dir/index.html" ] || { echo "前端构建未生成 index.html" >&2; exit 1; }
mkdir -p dist
go build -o dist/singbox-web ./cmd/singbox-web
