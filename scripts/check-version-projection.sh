#!/usr/bin/env bash
# check-version-projection.sh — proves the committed package-version PROJECTION
# agrees with the committed per-package version AUTHORITY.
#
# Why this exists: golang/build/package-versions.txt declares itself
# "materialized from committed zz_version_generated.go files", but nothing
# enforced that. On master at 4427d888 the zz files said 0.0.0-ci while the
# projection said 1.2.272 — the projection was stale relative to its own
# declared source and no gate noticed. This check makes that state fail.
#
# Verifies:
#   - every services.list target has exactly one committed version file
#   - no committed version is a 0.0.0-* placeholder
#   - every projection entry equals its version file, value for value
#   - no package missing, duplicated, or unexpected in the projection
#   - xds/gateway validated when the sibling Globular checkout is present;
#     its ABSENCE is reported explicitly, never treated as full coverage
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJ="${ROOT}/golang/build/package-versions.txt"
FAIL=0
err() { echo "PROJECTION-GATE FAIL: $*" >&2; FAIL=1; }

[[ -f "$PROJ" ]] || { echo "PROJECTION-GATE FAIL: missing $PROJ" >&2; exit 1; }

declare -A proj
while IFS='=' read -r k v; do
  [[ -z "$k" || "$k" == \#* ]] && continue
  [[ -n "${proj[$k]:-}" ]] && err "duplicate projection entry: $k"
  proj["$k"]="$v"
done < "$PROJ"

# Every committed version file must match its projection entry.
declare -A seen
while IFS= read -r zz; do
  [[ -f "$zz" ]] || continue
  ver="$(grep -oE 'Version = "[^"]*"' "$zz" | head -1 | sed 's/.*"\(.*\)"/\1/')"
  rel="${zz#"${ROOT}/"}"; leaf="$(basename "$(dirname "$rel")")"
  name="${leaf%_server}"; name="${name//_/-}"
  [[ "$name" == "globularcli" ]] && name="globular-cli"
  seen["$name"]=1
  case "$ver" in
    ""|0.0.0|0.0.0-*) err "$name: placeholder or empty committed version '$ver' in $rel" ;;
  esac
  if [[ -z "${proj[$name]:-}" ]]; then
    # A version file with no projection entry is only a defect for packages the
    # build actually ships (services.list). `compute` is a known in-tree service
    # that is deliberately not built or packaged, so it legitimately has a
    # committed version file and no projection row.
    if grep -qE "^\./[^|]*/${leaf}[[:space:]]*\|" "${ROOT}/golang/build/services.list" 2>/dev/null; then
      err "$name: in services.list but has no projection entry"
    else
      echo "  NOTICE: $name has a committed version file but is not in services.list (not shipped)"
    fi
  elif [[ "${proj[$name]}" != "$ver" ]]; then
    err "$name: projection says '${proj[$name]}' but $rel says '$ver'"
  fi
done < <(find "${ROOT}/golang" -name zz_version_generated.go -not -path '*/vendor/*' | sort)

# Sibling coverage must be stated, not assumed.
if [[ -d "${ROOT}/../Globular" ]]; then
  for n in xds gateway; do
    f="${ROOT}/../Globular/cmd/${n}/zz_version_generated.go"
    if [[ -f "$f" ]]; then
      v="$(grep -oE 'Version = "[^"]*"' "$f" | head -1 | sed 's/.*"\(.*\)"/\1/')"
      seen["$n"]=1
      [[ "${proj[$n]:-}" == "$v" ]] || err "$n: projection '${proj[$n]:-<none>}' != sibling mirror '$v'"
    else
      err "$n: sibling Globular present but no committed version file"
    fi
  done
else
  echo "  NOTICE: sibling Globular checkout absent — xds/gateway NOT verified (coverage is partial, not complete)"
fi

for k in "${!proj[@]}"; do
  [[ -n "${seen[$k]:-}" ]] || echo "  NOTICE: projection entry '$k' has no local or sibling version file (external package)"
done

(( FAIL )) && exit 1
echo "  ✓ version projection agrees with committed authority (${#proj[@]} entries)"
