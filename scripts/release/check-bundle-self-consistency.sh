#!/usr/bin/env bash
# check-bundle-self-consistency.sh — refuse a bundle whose carried-forward
# binaries disagree with the package set it actually ships.
#
# WHY THIS EXISTS
#
# build-local-release.sh carries unchanged packages forward from a base bundle.
# That is the right default — rebuilding 57 packages to change two is waste —
# but it means a binary in the produced release can be older than the release
# itself, and can therefore encode assumptions the release no longer satisfies.
#
# Observed 2026-08-17. The 1.2.318 bundle correctly excluded libnss-resolve
# (forbidden by check-no-bundled-libnss.sh, with a resolver-witness receipt
# proving cluster FQDN resolution works without it). The gateway binary, carried
# forward from the 1.2.312 base, still embedded a Day-1 join script whose step
# 4.7 installed that package — a step deleted from source on 2026-08-14
# precisely because bundling it had broken Day-1 joins twice before.
#
# Result: every joining node failed
#
#     FAIL: package libnss-resolve not found in /var/lib/globular/packages
#     [bootstrap] FATAL: join failed
#
# and because the founding node bootstrapped normally, the cluster reported
# HEALTHY at one node while four could never join. That reads as "still
# converging", not "broken", which is the expensive part.
#
# The class is general: a binary that embeds a PROCEDURE (a join script, an
# installer, a bootstrap sequence) is coupled to the package set that procedure
# expects. Carrying it forward across a change to that set is a silent
# mismatch. This gate makes the mismatch loud, at build time, before anything
# is installed anywhere.
#
# It deliberately checks the PRODUCED bundle rather than source: source being
# correct is exactly what made the shipped bundle look fine.
#
# Usage: check-bundle-self-consistency.sh <bundle-dir>
# Exit:  0 consistent, 1 mismatch or nothing inspected.

set -euo pipefail

BUNDLE_DIR="${1:-}"
if [[ -z "$BUNDLE_DIR" || ! -d "$BUNDLE_DIR" ]]; then
    echo "usage: $0 <bundle-dir>" >&2
    exit 1
fi

PKG_DIR="${BUNDLE_DIR}/packages"
if [[ ! -d "$PKG_DIR" ]]; then
    echo "  ✗ REJECTED — no packages/ directory in ${BUNDLE_DIR}; nothing inspected." >&2
    exit 1
fi

echo "== bundle self-consistency =="
echo "   bundle: ${BUNDLE_DIR}"

# shipped_package <name> — is a package with this name present in the bundle?
shipped_package() {
    local want="$1" f base
    for f in "${PKG_DIR}"/*.tgz; do
        [[ -f "$f" ]] || continue
        base="$(basename "$f")"
        # Strip _<version>_<platform>.tgz; versions start with a digit.
        if [[ "$base" =~ ^(.+)_[0-9][^_]*_linux_amd64\.tgz$ ]]; then
            [[ "${BASH_REMATCH[1]}" == "$want" ]] && return 0
        fi
    done
    return 1
}

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

inspected=0
violations=0

# Each rule pairs a MARKER string that a binary may embed with the package that
# marker implies the bundle must ship. Add a row whenever a binary starts
# depending on a package by name.
#
#   <package-that-embeds>|<marker string>|<package the marker requires>
RULES=(
    "gateway|Installing libnss-resolve|libnss-resolve"
)

for rule in "${RULES[@]}"; do
    IFS='|' read -r carrier marker required <<< "$rule"

    carrier_tgz="$(find "$PKG_DIR" -maxdepth 1 -name "${carrier}_*.tgz" -print -quit 2>/dev/null || true)"
    if [[ -z "$carrier_tgz" ]]; then
        echo "   - ${carrier}: not shipped in this bundle; rule not applicable"
        continue
    fi

    extract="${WORK}/${carrier}"
    mkdir -p "$extract"
    tar -xzf "$carrier_tgz" -C "$extract" 2>/dev/null || {
        echo "  ✗ REJECTED — could not extract ${carrier_tgz}" >&2
        exit 1
    }

    # Inspect every executable the package ships, not a guessed filename.
    #
    # NOTE: counted, not `grep -q`. Under `set -o pipefail`, `grep -q` exits as
    # soon as it matches, `strings` then dies of SIGPIPE (141), and pipefail
    # reports the PIPELINE as failed — so a successful match was read as "no
    # match" and this gate could never fire. It passed the known-broken 1.2.312
    # gateway on its first run. Counting consumes the whole stream, so no
    # SIGPIPE and no false negative.
    found_marker=0
    while IFS= read -r bin; do
        [[ -f "$bin" ]] || continue
        hits=""
        hits="$(strings "$bin" 2>/dev/null | grep -cF "$marker" || true)"
        if [[ "${hits:-0}" -gt 0 ]]; then
            found_marker=1
            break
        fi
    done < <(find "$extract" -type f -perm -u+x 2>/dev/null)

    inspected=$(( inspected + 1 ))

    if [[ $found_marker -eq 1 ]] && ! shipped_package "$required"; then
        violations=$(( violations + 1 ))
        echo "  ✗ ${carrier} ($(basename "$carrier_tgz")) embeds \"${marker}\"," >&2
        echo "    which requires the package '${required}', but this bundle does not ship it." >&2
        echo "    The binary was almost certainly carried forward from an older base bundle" >&2
        echo "    whose package set still included it." >&2
    elif [[ $found_marker -eq 1 ]]; then
        echo "   - ${carrier}: embeds \"${marker}\" and '${required}' IS shipped — consistent"
    else
        echo "   - ${carrier}: no stale marker \"${marker}\" — consistent"
    fi
done

echo "== result =="
echo "   rules inspected: ${inspected}"

if [[ $inspected -eq 0 ]]; then
    # Zero inspected is not a pass. A rule set that matched nothing reports
    # clean exactly like a bundle that is clean, which is the failure this whole
    # gate family exists to avoid.
    echo "  ✗ REJECTED — no rule matched any shipped package, so nothing was verified." >&2
    echo "    Either the bundle is missing packages it should have, or the rules are stale." >&2
    exit 1
fi

if [[ $violations -gt 0 ]]; then
    echo "  ✗ RELEASE REJECTED — ${violations} carried-forward binary/binaries disagree with the shipped package set." >&2
    echo "    Rebuild the offending package(s) from current source, e.g.:" >&2
    echo "      bash scripts/build-local-release.sh --version <v> --rebuild gateway --prev <base>" >&2
    exit 1
fi

echo "  ✓ carried-forward binaries agree with the shipped package set"
