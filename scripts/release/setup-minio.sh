#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="/var/lib/globular"
CONTRACT_FILE="${STATE_DIR}/objectstore/minio.json"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WEBROOT_DIR="${RELEASE_ROOT}/webroot"

[[ -f "${CONTRACT_FILE}" ]] || {
  echo "[setup-minio] missing contract: ${CONTRACT_FILE}" >&2
  exit 1
}

if [[ ! -d "${WEBROOT_DIR}" ]]; then
  echo "[setup-minio] missing bundled webroot: ${WEBROOT_DIR}" >&2
  exit 1
fi

if [[ -z "$(find "${WEBROOT_DIR}" -type f -print -quit 2>/dev/null)" ]]; then
  echo "[setup-minio] bundled webroot is empty: ${WEBROOT_DIR}" >&2
  exit 1
fi

if ! command -v mc >/dev/null 2>&1; then
  echo "[setup-minio] mc is required to seed MinIO webroot assets" >&2
  exit 1
fi

readarray -t CONTRACT_FIELDS < <(python3 - <<'PY' "${CONTRACT_FILE}"
import json, sys
path = sys.argv[1]
with open(path, "r", encoding="utf-8") as fh:
    data = json.load(fh)
endpoint = str(data.get("endpoint", "")).strip()
bucket = str(data.get("bucket", "globular")).strip() or "globular"
prefix = str(data.get("prefix", "")).strip().strip("/")
secure = "true" if bool(data.get("secure", True)) else "false"
cred_file = str(((data.get("auth") or {}).get("credFile", "/var/lib/globular/minio/credentials"))).strip()
print(endpoint)
print(bucket)
print(prefix)
print(secure)
print(cred_file)
PY
)

ENDPOINT="${CONTRACT_FIELDS[0]:-}"
BUCKET="${CONTRACT_FIELDS[1]:-globular}"
PREFIX="${CONTRACT_FIELDS[2]:-}"
SECURE="${CONTRACT_FIELDS[3]:-true}"
CRED_FILE="${CONTRACT_FIELDS[4]:-/var/lib/globular/minio/credentials}"

if [[ -z "${ENDPOINT}" ]]; then
  echo "[setup-minio] contract endpoint is empty" >&2
  exit 1
fi
if [[ "${ENDPOINT}" == *127.0.0.1* ]] || [[ "${ENDPOINT}" == *localhost* ]]; then
  echo "[setup-minio] refusing loopback endpoint in contract: ${ENDPOINT}" >&2
  exit 1
fi
[[ -f "${CRED_FILE}" ]] || {
  echo "[setup-minio] missing credentials: ${CRED_FILE}" >&2
  exit 1
}

if ! IFS=":" read -r ACCESS_KEY SECRET_KEY < "${CRED_FILE}"; then
  echo "[setup-minio] invalid credentials file: ${CRED_FILE}" >&2
  exit 1
fi
if [[ -z "${ACCESS_KEY}" || -z "${SECRET_KEY}" ]]; then
  echo "[setup-minio] empty MinIO credentials in ${CRED_FILE}" >&2
  exit 1
fi

SCHEME="https"
if [[ "${SECURE}" != "true" ]]; then
  SCHEME="http"
fi

ALIAS="globular-bootstrap"
mc alias rm "${ALIAS}" >/dev/null 2>&1 || true
mc alias set "${ALIAS}" "${SCHEME}://${ENDPOINT}" "${ACCESS_KEY}" "${SECRET_KEY}" --api s3v4 >/dev/null
mc mb --ignore-existing "${ALIAS}/${BUCKET}" >/dev/null

upload_tree() {
  local key_prefix="$1"
  mc mirror --overwrite "${WEBROOT_DIR}/" "${ALIAS}/${BUCKET}/${key_prefix}/" >/dev/null
  printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/${key_prefix}/.keep" >/dev/null
}

upload_tree "webroot"
printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/users/.keep" >/dev/null

if [[ -n "${PREFIX}" ]]; then
  upload_tree "${PREFIX}/webroot"
  printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/${PREFIX}/users/.keep" >/dev/null
fi

mc stat "${ALIAS}/${BUCKET}/webroot/index.html" >/dev/null
mc stat "${ALIAS}/${BUCKET}/webroot/logo.png" >/dev/null

echo "[setup-minio] seeded ${BUCKET}/webroot and bundled assets from ${WEBROOT_DIR}"
