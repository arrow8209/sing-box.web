#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -P -- "$(dirname -- "$0")/.." && pwd)
binary="$repo_root/dist/singbox-web"

listen="${SINGBOX_WEB_LISTEN:-127.0.0.1:50010}"
data_dir="${SINGBOX_WEB_DATA_DIR:-$repo_root/.singbox-web/data}"
config_root="${SINGBOX_WEB_CONFIG_ROOT:-$repo_root/.singbox-web/config}"
config_group="${SINGBOX_WEB_CONFIG_GROUP:-$(id -gn)}"

fail() {
	echo "$1" >&2
	exit 1
}

if [ ! -x "$binary" ]; then
	fail "未找到可执行文件: $binary"
fi

if [ "${listen#\[*}" != "$listen" ]; then
	host="${listen#\[}"
	host="${host%\]:*}"
	port="${listen##*:}"
else
	host="${listen%:*}"
	port="${listen##*:}"
fi

if [ "$listen" = "$port" ] || [ -z "$host" ] || [ -z "$port" ]; then
	fail "SINGBOX_WEB_LISTEN 必须是 host:port 格式，例如 127.0.0.1:50010"
fi

if [ "$host" = "0.0.0.0" ] || [ "$host" = "::" ]; then
	public_listen="--allow-public-listen"
else
	public_listen=""
fi

mkdir -p "$data_dir" "$config_root"
chmod 700 "$data_dir"
chmod 750 "$config_root"

if [ ! -f "$data_dir/auth.json" ]; then
	echo "首次启动未检测到管理员文件，开始设置密码"
	"$binary" admin set-password --data-dir "$data_dir"
fi

printf '启动服务: %s\n' "$listen"
echo "访问地址: http://$host:$port"

set -- "$binary" serve --dev \
	--listen "$listen" \
	--data-dir "$data_dir" \
	--config-root "$config_root" \
	--config-group "$config_group"

if [ -n "$public_listen" ]; then
	set -- "$@" "$public_listen"
fi

exec "$@"
