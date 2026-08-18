#!/usr/bin/env bash
# packages-revision.sh — print the packages revision this repo is pinned to.
#
# Single parse point for scripts/release/package-sources.json. CI checks the
# packages repo out at exactly this revision; local tooling compares its sibling
# checkout against it. Everything that needs the pin reads it through here, so
# the JSON shape is described in one place rather than re-parsed per caller.
#
# Usage:
#   scripts/release/packages-revision.sh            # print the pinned SHA
#   scripts/release/packages-revision.sh --repo     # print owner/name
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCES="${ROOT}/scripts/release/package-sources.json"

if [[ ! -f "${SOURCES}" ]]; then
    echo "packages-revision: missing ${SOURCES}" >&2
    exit 1
fi

field="revision"
case "${1:-}" in
    --repo) field="repository" ;;
    "") ;;
    *) echo "packages-revision: unknown argument '$1'" >&2; exit 2 ;;
esac

python3 - "${SOURCES}" "${field}" <<'PY'
import json, sys

path, field = sys.argv[1], sys.argv[2]
with open(path) as fh:
    doc = json.load(fh)

try:
    value = doc["sources"]["packages"][field]
except (KeyError, TypeError):
    print(f"packages-revision: sources.packages.{field} missing from {path}", file=sys.stderr)
    raise SystemExit(1)

value = str(value).strip()
if not value:
    print(f"packages-revision: sources.packages.{field} is empty in {path}", file=sys.stderr)
    raise SystemExit(1)

# A pin must be a full 40-hex commit SHA. A branch name or short SHA is not a
# pin — it is a moving target wearing a pin's clothes, which is the exact
# failure this file exists to prevent.
if field == "revision" and (len(value) != 40 or not all(c in "0123456789abcdef" for c in value.lower())):
    print(f"packages-revision: revision '{value}' is not a full 40-hex commit SHA", file=sys.stderr)
    raise SystemExit(1)

print(value)
PY
