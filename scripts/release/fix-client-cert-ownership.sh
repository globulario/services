#!/usr/bin/env bash
set -euo pipefail

echo ""
echo "━━━ Fixing Client Certificate Ownership ━━━"
echo ""

ACTUAL_USER="${1:-${SUDO_USER:-${USER}}}"
if [[ "${ACTUAL_USER}" == "root" ]]; then
    echo "ERROR: This script must be run with sudo from a regular user account" >&2
    echo "       Usage: sudo $0" >&2
    exit 1
fi

ACTUAL_HOME="$(eval echo ~${ACTUAL_USER})"
ACTUAL_GROUP="$(id -gn "${ACTUAL_USER}")"
CERT_SEARCH_ROOT="${GLOBULAR_CERT_HOME_OVERRIDE:-${ACTUAL_HOME}/.config/globular/tls}"
DOMAIN="${DOMAIN:-}"
declare -a CANDIDATE_DIRS=()
declare -a TRIED_DIRS=()

add_candidate_dir() {
    local candidate="${1:-}"
    [[ -n "$candidate" ]] || return 0
    local existing
    for existing in "${CANDIDATE_DIRS[@]:-}"; do
        [[ "$existing" == "$candidate" ]] && return 0
    done
    CANDIDATE_DIRS+=("$candidate")
}

if [[ -z "$DOMAIN" ]] && [[ -f /var/lib/globular/config.json ]]; then
    DOMAIN="$(jq -r '.Domain // ""' /var/lib/globular/config.json 2>/dev/null || true)"
fi
add_candidate_dir "$DOMAIN"
add_candidate_dir "globular.internal"
add_candidate_dir "localhost"

if [[ -d "$CERT_SEARCH_ROOT" ]]; then
    shopt -s nullglob
    for _path in "$CERT_SEARCH_ROOT"/*; do
        [[ -d "$_path" ]] || continue
        add_candidate_dir "$(basename "$_path")"
    done
    shopt -u nullglob
fi

CERT_DIR=""
for _d in "${CANDIDATE_DIRS[@]}"; do
    TRIED_DIRS+=("$_d")
    _candidate="${CERT_SEARCH_ROOT}/${_d}"
    if [[ -d "$_candidate" ]]; then
        CERT_DIR="$_candidate"
        break
    fi
done

if [[ -z "${CERT_DIR}" ]]; then
    echo "WARNING: Certificate directory not found under ${CERT_SEARCH_ROOT}" >&2
    echo "         Tried: ${TRIED_DIRS[*]:-<none>}" >&2
    echo "         Ownership fix skipped because no client certificate directory exists yet." >&2
    exit 0
fi

echo "User: ${ACTUAL_USER}"
echo "Cert Directory: ${CERT_DIR}"
echo ""

echo "→ Fixing ownership..."
chown -R "${ACTUAL_USER}:${ACTUAL_GROUP}" "${CERT_DIR}"
echo "  ✓ Ownership fixed"

echo "→ Setting correct permissions..."
chmod 700 "${CERT_DIR}"
chmod 644 "${CERT_DIR}/ca.crt" 2>/dev/null || true
chmod 644 "${CERT_DIR}/client.crt" 2>/dev/null || true
chmod 600 "${CERT_DIR}/client.key" 2>/dev/null || true
chmod 600 "${CERT_DIR}/client.pem" 2>/dev/null || true
echo "  ✓ Permissions set"

echo "→ Cleaning up temp files..."
rm -f "${CERT_DIR}/client.csr" "${CERT_DIR}/client.conf"
echo "  ✓ Cleanup done"

echo ""
echo "━━━ Client Certificates Fixed ━━━"
echo ""
echo "Certificate files:"
ls -la "${CERT_DIR}"
echo ""
echo "Verifying certificate..."
openssl verify -CAfile "${CERT_DIR}/ca.crt" "${CERT_DIR}/client.crt"
echo ""
echo "✓ Client certificates are ready for use"
echo ""
