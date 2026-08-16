#!/usr/bin/env bash
# bump-packages-pin.sh — adopt a new packages revision, deliberately.
#
# Usage:
#   scripts/release/bump-packages-pin.sh              # adopt packages origin/main
#   scripts/release/bump-packages-pin.sh <sha|ref>    # adopt a specific revision
#
# Refuses to pin a revision that is not reachable from a remote branch of the
# packages repo. That is the whole point: on 2026-08-15 the packages half of a
# cross-repo change lived only on an unpushed local branch, so services was
# validated against a corpus nobody else could see. A pin that names a commit
# only your machine has is not a pin — it recreates the bug in a file whose
# purpose is to prevent it.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SOURCES="${ROOT}/scripts/release/package-sources.json"
PACKAGES="${GLOBULAR_PACKAGES_ROOT:-${ROOT}/../packages}"

die() { echo "bump-packages-pin: $*" >&2; exit 1; }

[[ -f "${SOURCES}" ]]       || die "missing ${SOURCES}"
[[ -d "${PACKAGES}/.git" ]] || die "no packages checkout at ${PACKAGES} (set GLOBULAR_PACKAGES_ROOT)"

target="${1:-origin/main}"

echo "→ fetching ${PACKAGES}"
git -C "${PACKAGES}" fetch --quiet origin || die "could not fetch packages"

sha="$(git -C "${PACKAGES}" rev-parse --verify "${target}^{commit}" 2>/dev/null)" \
    || die "'${target}' does not resolve to a commit in ${PACKAGES}"

# The guard. A revision must be published before this repo may depend on it.
remotes="$(git -C "${PACKAGES}" branch -r --contains "${sha}" 2>/dev/null | sed 's/^[* ]*//' | grep -v '^origin/HEAD' || true)"
if [[ -z "${remotes}" ]]; then
    die "refusing to pin ${sha:0:12} — it is not reachable from any remote branch of packages.
       Push it first. A pin naming a commit only this machine has is exactly the
       failure package-sources.json exists to prevent."
fi

current="$("${ROOT}/scripts/release/packages-revision.sh")"
if [[ "${current}" == "${sha}" ]]; then
    echo "already pinned at ${sha:0:12} — nothing to do"
    exit 0
fi

python3 - "${SOURCES}" "${sha}" <<'PY'
import json, sys

path, sha = sys.argv[1], sys.argv[2]
with open(path) as fh:
    doc = json.load(fh)
doc["sources"]["packages"]["revision"] = sha
with open(path, "w") as fh:
    json.dump(doc, fh, indent=2)
    fh.write("\n")
PY

echo
echo "  packages pin: ${current:0:12} → ${sha:0:12}"
echo "  reachable from:$(echo " ${remotes}" | tr '\n' ' ')"
echo "  adopting:"
git -C "${PACKAGES}" log --oneline "${current}..${sha}" 2>/dev/null | sed 's/^/    /' || true
echo
echo "Review, then commit scripts/release/package-sources.json on its own."
