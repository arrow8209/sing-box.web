#!/bin/sh
set -eu

usage() {
  cat <<'EOF'
用法：
  scripts/test-proxy-ip.sh [代理 URL] [IP 查询 URL]

默认值：
  代理 URL     http://127.0.0.1:20122
  IP 查询 URL  https://api.ipify.org

也可以通过 PROXY_URL 和 IP_LOOKUP_URL 环境变量覆盖默认值。
EOF
}

case "${1:-}" in
  -h|--help)
    usage
    exit 0
    ;;
esac

proxy_url="${1:-${PROXY_URL:-http://127.0.0.1:20122}}"
ip_lookup_url="${2:-${IP_LOOKUP_URL:-https://api.ipify.org}}"

case "$proxy_url" in
  http://*|https://*) ;;
  *)
    printf '代理 URL 必须以 http:// 或 https:// 开头：%s\n' "$proxy_url" >&2
    exit 2
    ;;
esac

case "$ip_lookup_url" in
  http://*|https://*) ;;
  *)
    printf 'IP 查询 URL 必须以 http:// 或 https:// 开头：%s\n' "$ip_lookup_url" >&2
    exit 2
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  printf '未找到 curl，请先安装 curl。\n' >&2
  exit 127
fi

HTTP_PROXY="$proxy_url"
HTTPS_PROXY="$proxy_url"
http_proxy="$proxy_url"
https_proxy="$proxy_url"
export HTTP_PROXY HTTPS_PROXY http_proxy https_proxy
unset NO_PROXY no_proxy

ip_address="$(curl \
  --fail \
  --silent \
  --show-error \
  --location \
  --connect-timeout "${CURL_CONNECT_TIMEOUT:-10}" \
  --max-time "${CURL_MAX_TIME:-30}" \
  "$ip_lookup_url")"

if [ -z "$ip_address" ]; then
  printf 'IP 查询接口返回了空结果。\n' >&2
  exit 1
fi

printf '代理出口 IP：%s\n' "$ip_address"
