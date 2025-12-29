#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICE_LIST="${SERVICE_LIST:-$SCRIPT_DIR/services.list}"
ARCH="${ARCH:-$(dpkg-architecture -qDEB_HOST_ARCH 2>/dev/null || uname -m)}"
STAGE_ROOT="${STAGE_ROOT:-$ROOT_DIR/tools/stage/linux-$ARCH}"

last_line=""
on_err() {
  echo "Build failed." >&2
  if [[ -n "$last_line" ]]; then
    echo "Last manifest entry: $last_line" >&2
  fi
}
trap on_err ERR

if [[ ! -f "$SERVICE_LIST" ]]; then
  echo "Service manifest not found at $SERVICE_LIST" >&2
  exit 1
fi

echo "Running Go builds listed in: $SERVICE_LIST"
echo "ARCH=$ARCH"
echo "STAGE_ROOT=$STAGE_ROOT"

mkdir -p "$STAGE_ROOT"

while IFS= read -r raw || [[ -n "$raw" ]]; do
  last_line="$raw"
  line="${raw%%#*}"
  line="${line//$'\t'/ }"
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  [[ -z "$line" ]] && continue
  if [[ "$line" != *"|"* ]]; then
    echo "Invalid manifest line (missing '|'): $raw" >&2
    exit 1
  fi
  IFS='|' read -r target out_rel <<<"$line"
  target="$(xargs <<<"$target")"
  out_rel="$(xargs <<<"$out_rel")"
  [[ -z "$target" || -z "$out_rel" ]] && continue
  output_path="$STAGE_ROOT/$out_rel"
  mkdir -p "$(dirname "$output_path")"
  echo "=> go build -buildvcs=false -o $output_path $target"
  (cd "$ROOT_DIR" && go build -buildvcs=false -o "$output_path" "$target")
done < "$SERVICE_LIST"

echo "Build complete."
