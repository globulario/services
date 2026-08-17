#!/usr/bin/env bash
# Hard assertion: the PRODUCED release contains no Globular-supplied
# libnss-resolve deb or package.
#
# This inspects artifacts, never source configuration. That distinction is the
# whole point: this repository has already shipped a release whose bundled deb
# existed in nobody's source tree — an untracked file in a sibling worktree — so
# "the declaration no longer mentions it" is not evidence that the bytes are
# gone. Declarative absence and artifact absence are two different claims, and
# only the second one ships.
#
# Usage:  check-no-bundled-libnss.sh <release-dir> [<release-dir> ...]
# Exit:   0 clean, 1 contamination found, 2 nothing inspected

set -uo pipefail

if [ "$#" -eq 0 ]; then
    echo "usage: $0 <release-dir> [...]" >&2
    exit 2
fi

found=0
inspected=0

for root in "$@"; do
    if [ ! -d "${root}" ]; then
        echo "  SKIP  ${root} (not a directory)"
        continue
    fi
    echo "== inspecting ${root} =="

    # 1. A whole libnss-resolve package tarball must not be present.
    while IFS= read -r p; do
        [ -n "${p}" ] || continue
        echo "  FOUND package artifact: ${p}"
        found=$((found+1))
    done < <(find "${root}" -name 'libnss-resolve*' -type f 2>/dev/null)

    # 2. No .deb named libnss-resolve may be bundled inside ANY package tgz.
    #    A package can carry debs for a different package entirely, so the
    #    tarballs are opened rather than trusted by filename.
    while IFS= read -r tgz; do
        [ -n "${tgz}" ] || continue
        inspected=$((inspected+1))
        if tar -tzf "${tgz}" 2>/dev/null | grep -qi 'libnss[-_]resolve.*\.deb'; then
            echo "  FOUND bundled deb inside: ${tgz}"
            tar -tzf "${tgz}" 2>/dev/null | grep -i 'libnss[-_]resolve.*\.deb' | sed 's/^/          /'
            found=$((found+1))
        fi
    done < <(find "${root}" -name '*.tgz' -type f 2>/dev/null)
done

echo
echo "== result =="
echo "  tarballs inspected: ${inspected}"
if [ "${inspected}" -eq 0 ] && [ "${found}" -eq 0 ]; then
    # Zero findings from zero inspection is not a pass. A scanner that never
    # ran reports exactly like a clean tree.
    echo "  INCONCLUSIVE: no package tarballs were inspected — nothing was proven"
    exit 2
fi
if [ "${found}" -ne 0 ]; then
    echo "  FAIL: ${found} libnss-resolve artifact(s) present in the produced release"
    exit 1
fi
echo "  PASS: produced release contains no Globular-supplied libnss-resolve"
exit 0
