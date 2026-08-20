#!/bin/sh
set -eu


repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
runtime_root=$(mktemp -d)
backend_pid=
caddy_pid=

cleanup() {
    if [ -n "$caddy_pid" ]; then
        kill "$caddy_pid" 2>/dev/null || true
        wait "$caddy_pid" 2>/dev/null || true
    fi
    if [ -n "$backend_pid" ]; then
        kill "$backend_pid" 2>/dev/null || true
        wait "$backend_pid" 2>/dev/null || true
    fi
    if [ -f "$runtime_root/.singbox-web-e2e-root" ]; then
        rm -rf -- "$runtime_root"
    fi
}
trap cleanup EXIT
trap 'exit 0' HUP INT TERM
touch "$runtime_root/.singbox-web-e2e-root"

for required_command in caddy curl grep id jq ss; do
    command -v "$required_command" >/dev/null
done
for required_port in 18080 18443; do
    if ss -H -ltn "sport = :$required_port" | grep -q .; then
        echo "E2E port already occupied: $required_port" >&2
        exit 1
    fi
done

data_dir=$runtime_root/data
config_root=$runtime_root/config
caddy_data=$runtime_root/caddy-data
caddy_config=$runtime_root/caddy-config
args_log=$runtime_root/fake-sing-box.args
backend_log=$runtime_root/backend.log
caddy_log=$runtime_root/caddy.log
mkdir -p "$data_dir" "$config_root" "$caddy_data" "$caddy_config"
chmod 0700 "$data_dir"
chmod 0750 "$config_root"

fake_sing_box=$runtime_root/fake-sing-box
cat >"$fake_sing_box" <<'FAKE_SING_BOX'
#!/bin/sh
set -eu

: "${FAKE_SING_BOX_ARGS_LOG:?}"
printf '%s\n' "$@" >>"$FAKE_SING_BOX_ARGS_LOG"
case "${1:-}" in
    version)
        test "$#" -eq 1
        printf '%s\n' 'sing-box version e2e-latest'
        ;;
    check)
        test "$#" -eq 3
        test "$2" = '-C'
        test -d "$3"
        test ! -e "$3/config.json"
        module_count=0
        for module_path in "$3"/*.json; do
            test -f "$module_path" || continue
            jq -e 'type == "object" and length == 1' "$module_path" >/dev/null
            module_count=$((module_count + 1))
        done
        test "$module_count" -gt 0
        ;;
    run)
        echo 'sing-box run is forbidden in phase-one E2E' >&2
        exit 90
        ;;
    *)
        echo 'unsupported fake sing-box command' >&2
        exit 64
        ;;
esac
FAKE_SING_BOX
chmod 0755 "$fake_sing_box"
export FAKE_SING_BOX_ARGS_LOG=$args_log

printf '%s\n%s\n' 'phase1-e2e-password' 'phase1-e2e-password' \
    | "$repo_dir/dist/singbox-web" admin set-password --data-dir "$data_dir"

config_group=$(id -gn)
"$repo_dir/dist/singbox-web" serve \
    --listen 127.0.0.1:18080 \
    --data-dir "$data_dir" \
    --config-root "$config_root" \
    --sing-box "$fake_sing_box" \
    --config-group "$config_group" \
    --public-origin https://127.0.0.1:18443 \
    >"$backend_log" 2>&1 &
backend_pid=$!

attempt=0
until curl --noproxy '*' -fsS http://127.0.0.1:18080/healthz >/dev/null; do
    if ! kill -0 "$backend_pid" 2>/dev/null; then
        sed -n '1,160p' "$backend_log" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        echo 'backend health timeout' >&2
        exit 1
    fi
    sleep 0.1
done

caddyfile=$runtime_root/Caddyfile
cat >"$caddyfile" <<'CADDYFILE'
{
    admin off
    skip_install_trust
}

https://127.0.0.1:18443 {
    tls internal
    request_body {
        max_size 2MB
    }
    header {
        Strict-Transport-Security "max-age=31536000"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "no-referrer"
    }
    reverse_proxy 127.0.0.1:18080
}
CADDYFILE
XDG_DATA_HOME="$caddy_data" XDG_CONFIG_HOME="$caddy_config" \
    caddy run --config "$caddyfile" --adapter caddyfile >"$caddy_log" 2>&1 &
caddy_pid=$!

attempt=0
until curl --noproxy '*' -kfsS https://127.0.0.1:18443/healthz >/dev/null; do
    if ! kill -0 "$caddy_pid" 2>/dev/null; then
        sed -n '1,160p' "$caddy_log" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        echo 'Caddy health timeout' >&2
        exit 1
    fi
    sleep 0.1
done

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$caddy_pid" 2>/dev/null; do
    if grep -qx 'run' "$args_log" 2>/dev/null; then
        echo 'forbidden sing-box run invocation detected' >&2
        exit 1
    fi
    sleep 1
done
echo 'E2E backend or Caddy exited unexpectedly' >&2
sed -n '1,160p' "$backend_log" >&2
sed -n '1,160p' "$caddy_log" >&2
exit 1
