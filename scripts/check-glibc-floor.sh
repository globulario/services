#!/usr/bin/env bash
set -euo pipefail

# check-glibc-floor.sh — release gate: every shipped binary must run on the
# oldest platform we advertise support for.
#
# WHY THIS EXISTS
#
# Release 1.2.288 shipped file_server linked against GLIBC_2.38 while the other
# 54 binaries needed at most GLIBC_2.34. docs/operators/building-from-source.md
# advertises "Ubuntu 22.04+" (glibc 2.35) and Debian 12 (2.36), so file_server
# could not execute on either:
#
#   file_server: /lib/x86_64-linux-gnu/libm.so.6: version `GLIBC_2.38' not found
#
# Day-0 aborted at "Workload Services" with `step start-services did not
# converge`. The cause is build provenance: the release is produced on a
# glibc-2.39 workstation instead of a container pinned to the support floor.
# Most binaries never reference newer symbols and survive by luck; one did not.
#
# Luck is not a release gate. This is.
#
# USAGE
#   scripts/check-glibc-floor.sh dist/globular-1.2.288-linux-amd64
#   MAX_GLIBC=2.35 scripts/check-glibc-floor.sh <bundle-dir>
#
# Exit codes: 0 all binaries within floor | 1 violation | 2 usage/tooling error

BUNDLE="${1:-}"
# 2.35 = Ubuntu 22.04, the oldest platform docs/operators/building-from-source.md
# advertises. Raise this ONLY together with that document — the whole point is
# that the code and the promise move as one.
MAX_GLIBC="${MAX_GLIBC:-2.35}"

[ -n "$BUNDLE" ] || { echo "usage: $0 <release-bundle-dir> [MAX_GLIBC=x.yz]" >&2; exit 2; }
[ -d "$BUNDLE/packages" ] || { echo "no packages/ under $BUNDLE" >&2; exit 2; }
command -v objdump >/dev/null || { echo "objdump required (apt install binutils)" >&2; exit 2; }

# Compare dotted versions: returns 0 if $1 > $2
ver_gt() { [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -1)" = "$1" ] && [ "$1" != "$2" ]; }

work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
for pkg in "$BUNDLE"/packages/*.tgz; do
    [ -f "$pkg" ] || continue
    tar xzf "$pkg" -C "$work" --wildcards '*bin/*' 2>/dev/null || true
done

violations=0 checked=0
while IFS= read -r bin; do
    # Skip anything that is not an ELF executable (scripts, data blobs).
    head -c4 "$bin" 2>/dev/null | grep -q $'\x7fELF' || continue
    checked=$((checked+1))
    # `|| true` is required: statically linked Go binaries (prometheus, minio…)
    # have no GLIBC_ symbols at all, so grep exits 1, the assignment inherits
    # that status, and `set -e` would abort the whole scan silently — reporting
    # a clean release because it stopped at the first static binary.
    need=$(objdump -T "$bin" 2>/dev/null \
             | grep -oE 'GLIBC_[0-9]+\.[0-9]+' \
             | sed 's/GLIBC_//' | sort -V | tail -1 || true)
    [ -n "$need" ] || continue   # static binary — no glibc floor to violate
    if ver_gt "$need" "$MAX_GLIBC"; then
        echo "  ✗ $(basename "$bin"): requires GLIBC_${need} > floor ${MAX_GLIBC}"
        violations=$((violations+1))
    fi
done < <(find "$work" -type f -perm -u+x)

if [ "$checked" -eq 0 ]; then
    echo "check-glibc-floor: no ELF binaries found under $BUNDLE/packages" >&2
    exit 2
fi

if [ "$violations" -gt 0 ]; then
    cat >&2 <<EOF

check-glibc-floor: FAIL — $violations of $checked binaries exceed the GLIBC_${MAX_GLIBC} floor.

These cannot execute on the oldest supported platform. Day-0 will abort when the
affected service's unit fails to start.

Fix: build the release inside a container pinned to the floor (Ubuntu 22.04 for
glibc 2.35), or set CGO_ENABLED=0 where the service permits. Do NOT raise
MAX_GLIBC without also updating docs/operators/building-from-source.md — the
gate exists to keep the binaries and the support promise in agreement.
EOF
    exit 1
fi

echo "check-glibc-floor: OK — $checked binaries, all within GLIBC_${MAX_GLIBC}"
exit 0
