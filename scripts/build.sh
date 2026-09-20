#!/bin/sh
set -eu

build_version="${VERSION-$(node -p 'require("./web/package.json").version')}"
build_version="${build_version#v}"
case "$build_version" in
  ""|*[!0-9A-Za-z.+-]*)
    echo "版本号不能为空，且只能包含字母、数字、点、加号和连字符" >&2
    exit 1
    ;;
esac
output_file="dist/singbox-web-v${build_version}"

assets_dir="internal/webui/assets"
[ -d "$assets_dir" ] || { echo "缺少前端资源目录: $assets_dir" >&2; exit 1; }
find "$assets_dir" -mindepth 1 ! -name _placeholder -delete
npm --prefix web run build
[ -f "$assets_dir/index.html" ] || { echo "前端构建未生成 index.html" >&2; exit 1; }
mkdir -p dist
go build -o "$output_file" ./cmd/singbox-web
printf '构建完成：%s\n' "$output_file"
