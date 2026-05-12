#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="/var/lib/globular"
CONTRACT_FILE="${STATE_DIR}/objectstore/minio.json"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SOURCE_WEBROOT_DIR=""
for _wr in \
  "${RELEASE_ROOT}/webroot" \
  "${SCRIPT_DIR}/../webroot" \
  "${SCRIPT_DIR}/../../webroot" \
  "/usr/lib/globular/webroot" \
  "/opt/globular/webroot"; do
  if [[ -f "${_wr}/index.html" ]]; then
    SOURCE_WEBROOT_DIR="${_wr}"
    break
  fi
done

[[ -f "${CONTRACT_FILE}" ]] || {
  echo "[setup-minio] missing contract: ${CONTRACT_FILE}" >&2
  exit 1
}

if [[ -z "${SOURCE_WEBROOT_DIR}" ]] || [[ ! -d "${SOURCE_WEBROOT_DIR}" ]]; then
  echo "[setup-minio] missing bundled webroot (checked installer and system paths)" >&2
  exit 1
fi

if [[ -z "$(find "${SOURCE_WEBROOT_DIR}" -type f -print -quit 2>/dev/null)" ]]; then
  echo "[setup-minio] bundled webroot is empty: ${SOURCE_WEBROOT_DIR}" >&2
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
DEST_WEBROOT_DIR="webroot"

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

if command -v etcdctl >/dev/null 2>&1 && [[ -f "/var/lib/globular/pki/ca.crt" && -f "/var/lib/globular/pki/issued/services/service.crt" && -f "/var/lib/globular/pki/issued/services/service.key" ]]; then
  ETCD_WEBROOT="$(etcdctl \
    --endpoints="https://127.0.0.1:2379" \
    --cacert="/var/lib/globular/pki/ca.crt" \
    --cert="/var/lib/globular/pki/issued/services/service.crt" \
    --key="/var/lib/globular/pki/issued/services/service.key" \
    get "/globular/cluster/minio/config" --print-value-only 2>/dev/null | \
    python3 -c 'import json,sys; d=(sys.stdin.read() or "").strip(); print((json.loads(d).get("webroot_dir","") if d else "").strip())' 2>/dev/null || true)"
  if [[ -n "${ETCD_WEBROOT}" ]]; then
    DEST_WEBROOT_DIR="${ETCD_WEBROOT#/}"
    DEST_WEBROOT_DIR="${DEST_WEBROOT_DIR%/}"
  fi
fi

upload_tree() {
  local key_prefix="$1"
  mc mirror --overwrite "${SOURCE_WEBROOT_DIR}/" "${ALIAS}/${BUCKET}/${key_prefix}/" >/dev/null
  printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/${key_prefix}/.keep" >/dev/null
}

# Publish at bucket root for legacy gateway welcome-page lookups.
mc mirror --overwrite "${SOURCE_WEBROOT_DIR}/" "${ALIAS}/${BUCKET}/" >/dev/null
upload_tree "${DEST_WEBROOT_DIR}"
printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/users/.keep" >/dev/null

if [[ -n "${PREFIX}" ]]; then
  mc mirror --overwrite "${SOURCE_WEBROOT_DIR}/" "${ALIAS}/${BUCKET}/${PREFIX}/" >/dev/null
  upload_tree "${PREFIX}/${DEST_WEBROOT_DIR}"
  printf 'keep\n' | mc pipe "${ALIAS}/${BUCKET}/${PREFIX}/users/.keep" >/dev/null
fi

mc stat "${ALIAS}/${BUCKET}/index.html" >/dev/null
mc stat "${ALIAS}/${BUCKET}/logo.png" >/dev/null
mc stat "${ALIAS}/${BUCKET}/${DEST_WEBROOT_DIR}/index.html" >/dev/null
mc stat "${ALIAS}/${BUCKET}/${DEST_WEBROOT_DIR}/logo.png" >/dev/null

echo "[setup-minio] seeded ${BUCKET}/ and ${BUCKET}/${DEST_WEBROOT_DIR} from ${SOURCE_WEBROOT_DIR}"
