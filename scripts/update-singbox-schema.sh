#!/bin/sh
set -eu

singbox_bin=${SING_BOX_BIN:-sing-box}
target_version=1.13.19
schema_commit=583a690e888cdf6e8b024bf017ebf6297e08338d
schema_url=https://raw.githubusercontent.com/SagerNet/sing-box/$schema_commit/schema.json
expected_sha256=ee03dcaffe9282100418de280c61d3f0c8022712af50bc79c072994247133c41
asset_dir=internal/assets
schema_temp=$(mktemp "$asset_dir/.sing-box.schema.json.XXXXXX")
metadata_temp=$(mktemp "$asset_dir/.schema.version.json.XXXXXX")
trap 'rm -f "$schema_temp" "$metadata_temp"' EXIT

version=$($singbox_bin version | sed -n '1s/.* //p')
test -n "$version"
if [ "$version" != "$target_version" ]; then
  echo "installed sing-box version $version does not match target $target_version" >&2
  exit 1
fi
curl -fsSL "$schema_url" -o "$schema_temp"
jq -e . "$schema_temp" >/dev/null
sha256=$(sha256sum "$schema_temp" | sed 's/ .*//')
if [ "$sha256" != "$expected_sha256" ]; then
  echo "schema SHA-256 mismatch" >&2
  exit 1
fi
jq -e '
  . as $schema
  | ($schema["$defs"].DNS.properties.independent_cache.type == "boolean")
  and (
    $schema["$defs"].RuleSet.oneOf
    | any(.properties.type.const == "remote" and .properties.download_detour.type == "string")
  )
' "$schema_temp" >/dev/null
fetched_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
jq -n --arg targetVersion "$target_version" --arg schemaURL "$schema_url" --arg schemaCommit "$schema_commit" --arg sha256 "$sha256" --arg fetchedAt "$fetched_at" \
  '{targetVersion:$targetVersion,schemaURL:$schemaURL,schemaCommit:$schemaCommit,sha256:$sha256,fetchedAt:$fetchedAt}' \
  > "$metadata_temp"
mv "$schema_temp" "$asset_dir/sing-box.schema.json"
mv "$metadata_temp" "$asset_dir/schema.version.json"
