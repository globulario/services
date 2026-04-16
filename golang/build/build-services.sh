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

VERSION="${VERSION:-0.0.1}"
BUILD_TIME="${BUILD_TIME:-$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
GIT_COMMIT="${GIT_COMMIT:-$(cd "$ROOT_DIR" && git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_NUMBER="${BUILD_NUMBER:-0}"

LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -X main.BuildNumberStr=${BUILD_NUMBER}"

echo "Running Go builds listed in: $SERVICE_LIST"
echo "ARCH=$ARCH  VERSION=$VERSION  COMMIT=$GIT_COMMIT"
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
  echo "=> go build -ldflags \"$LDFLAGS\" -buildvcs=false -o $output_path $target"
  (cd "$ROOT_DIR" && go build -ldflags "$LDFLAGS" -buildvcs=false -o "$output_path" "$target")
done < "$SERVICE_LIST"

echo "Build complete."
