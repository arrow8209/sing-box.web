#!/bin/sh
set -eu

selection="internal/contracttest/testdata/selection.json"
workdir=$(mktemp -d "${TMPDIR:-/tmp}/singbox-web-contract.XXXXXX")
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT INT TERM

require_sha256() {
  value=$1
  case "$value" in
    *[!0123456789abcdef]*|????????????????????????????????????????????????????????????????) ;;
    *) echo "无效 SHA-256: $value" >&2; exit 1 ;;
  esac
}

download_and_verify() {
  asset_url=$1
  archive_sha256=$2
  binary_sha256=$3
  asset_name=$4
  archive="$workdir/$asset_name"
  curl -L --fail --silent --show-error --output "$archive" "$asset_url"
  actual_archive=$(sha256sum "$archive" | awk '{print $1}')
  [ "$actual_archive" = "$archive_sha256" ] || { echo "压缩包 SHA-256 不匹配" >&2; exit 1; }
  tar -xzf "$archive" -C "$workdir"
  binary="$workdir/${asset_name%.tar.gz}/sing-box"
  [ -f "$binary" ] && [ ! -L "$binary" ] || { echo "压缩包中缺少受管 sing-box 二进制" >&2; exit 1; }
  actual_binary=$(sha256sum "$binary" | awk '{print $1}')
  [ "$actual_binary" = "$binary_sha256" ] || { echo "二进制 SHA-256 不匹配" >&2; exit 1; }
  chmod 700 "$binary"
  SINGBOX_CONTRACT_BINARY="$binary" go test -tags contract ./internal/contracttest
}

refresh() {
  api_url="https://api.github.com/repos/SagerNet/sing-box/releases/latest"
  metadata="$workdir/release.json"
  curl -L --fail --silent --show-error --output "$metadata" "$api_url"
  tag=$(jq -r '.tag_name' "$metadata")
  version=${tag#v}
  [ "$tag" = "v$version" ] && [ -n "$version" ] || { echo "官方 latest 不是稳定版本" >&2; exit 1; }
  case "$(go env GOARCH)" in
    amd64|arm64|386) arch=$(go env GOARCH) ;;
    arm) arch=armv7 ;;
    *) echo "不支持当前契约测试架构" >&2; exit 1 ;;
  esac
  asset_name="sing-box-$version-linux-$arch.tar.gz"
  asset_url=$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .browser_download_url' "$metadata")
  asset_digest=$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .digest' "$metadata")
  [ "$(printf '%s\n' "$asset_url" | wc -l | tr -d ' ')" = "1" ] && [ "$asset_url" != "null" ] || { echo "官方 release 中没有唯一 Linux 资产" >&2; exit 1; }
  archive_sha256=${asset_digest#sha256:}
  [ "$asset_digest" = "sha256:$archive_sha256" ] || { echo "官方 release 未提供 SHA-256 资产摘要" >&2; exit 1; }
  case "$asset_url" in
    "https://github.com/SagerNet/sing-box/releases/download/$tag/"*) ;;
    *) echo "官方 release 返回了不受信任的资产地址" >&2; exit 1 ;;
  esac
  require_sha256 "$archive_sha256"
  archive="$workdir/$asset_name"
  curl -L --fail --silent --show-error --output "$archive" "$asset_url"
  actual_archive=$(sha256sum "$archive" | awk '{print $1}')
  [ "$actual_archive" = "$archive_sha256" ] || { echo "官方资产 SHA-256 不匹配" >&2; exit 1; }
  tar -xzf "$archive" -C "$workdir"
  binary="$workdir/${asset_name%.tar.gz}/sing-box"
  [ -f "$binary" ] && [ ! -L "$binary" ] || { echo "压缩包中缺少受管 sing-box 二进制" >&2; exit 1; }
  binary_sha256=$(sha256sum "$binary" | awk '{print $1}')
  require_sha256 "$binary_sha256"
  release_url="https://api.github.com/repos/SagerNet/sing-box/releases/tags/$tag"
  temporary_selection="$workdir/selection.json"
  jq -n --arg version "$version" --arg releaseURL "$release_url" --arg assetName "$asset_name" --arg assetURL "$asset_url" --arg archiveSHA256 "$archive_sha256" --arg binarySHA256 "$binary_sha256" '{version:$version,releaseURL:$releaseURL,assetName:$assetName,assetURL:$assetURL,archiveSHA256:$archiveSHA256,binarySHA256:$binarySHA256}' > "$temporary_selection"
  mv "$temporary_selection" "$selection"
  echo "已更新经官方 release SHA-256 与本地二进制 SHA-256 双重验证的选择: $version" >&2
}

if [ "${1:-}" = "--refresh" ]; then
  refresh
elif [ "$#" -ne 0 ]; then
  echo "用法: $0 [--refresh]" >&2
  exit 2
fi

[ -f "$selection" ] || { echo "缺少契约选择记录: $selection" >&2; exit 1; }
version=$(jq -r '.version' "$selection")
release_url=$(jq -r '.releaseURL' "$selection")
asset_name=$(jq -r '.assetName' "$selection")
asset_url=$(jq -r '.assetURL' "$selection")
archive_sha256=$(jq -r '.archiveSHA256' "$selection")
binary_sha256=$(jq -r '.binarySHA256' "$selection")
require_sha256 "$archive_sha256"
require_sha256 "$binary_sha256"
case "$release_url" in
  https://api.github.com/repos/SagerNet/sing-box/releases/tags/v*) ;;
  *) echo "选择记录的 release 地址不受信任" >&2; exit 1 ;;
esac
case "$asset_url" in
  https://github.com/SagerNet/sing-box/releases/download/v*/sing-box-*-linux-*.tar.gz) ;;
  *) echo "选择记录的资产地址不受信任" >&2; exit 1 ;;
esac
metadata="$workdir/release.json"
curl -L --fail --silent --show-error --output "$metadata" "$release_url"
official_digest=$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .digest' "$metadata")
official_url=$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .browser_download_url' "$metadata")
[ "$official_digest" = "sha256:$archive_sha256" ] || { echo "记录的压缩包 SHA-256 不再匹配官方 release 元数据" >&2; exit 1; }
[ "$official_url" = "$asset_url" ] || { echo "记录的资产地址不再匹配官方 release 元数据" >&2; exit 1; }
download_and_verify "$asset_url" "$archive_sha256" "$binary_sha256" "$asset_name"
