#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SERVICE_LIST="$SCRIPT_DIR/services.list"

if [[ ! -f "$SERVICE_LIST" ]]; then
  echo "Service manifest not found at $SERVICE_LIST"
  exit 1
fi

echo "Running Go builds for services listed in $SERVICE_LIST"

while IFS= read -r line || [[ -n "$line" ]]; do
  line="${line%%#*}"
  line="${line//	/ }"
  # Trim leading/trailing whitespace.
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%"${line##*[![:space:]]}"}"
  if [[ -z "$line" ]]; then
    continue
  fi
  read -r module output <<<"$line"
  if [[ -z "$module" || -z "$output" ]]; then
    continue
  fi

  module_path="$ROOT_DIR/$module"
  output_path="$ROOT_DIR/$output"
  mkdir -p "$(dirname "$output_path")"
  echo "=> go build -buildvcs=false -o $output_path $module_path"
  (cd "$ROOT_DIR" && go build -buildvcs=false -o "$output_path" "$module_path")
done < "$SERVICE_LIST"
