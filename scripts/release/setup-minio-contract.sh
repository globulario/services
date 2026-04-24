#!/usr/bin/env bash
set -euo pipefail

STATE_DIR="/var/lib/globular"
CONTRACT_DIR="${STATE_DIR}/objectstore"
CONTRACT_FILE="${CONTRACT_DIR}/minio.json"
CRED_FILE="${STATE_DIR}/minio/credentials"
CA_BUNDLE="${STATE_DIR}/pki/ca.pem"

NODE_IP="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
if [[ -z "${NODE_IP}" ]]; then
  NODE_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
fi
if [[ -z "${NODE_IP}" || "${NODE_IP}" == "127.0.0.1" || "${NODE_IP}" == "localhost" ]]; then
  echo "[setup-minio-contract] could not determine a routable node IP" >&2
  exit 1
fi

mkdir -p "${CONTRACT_DIR}" "$(dirname "${CRED_FILE}")"

if [[ ! -f "${CRED_FILE}" ]]; then
  printf 'globular:globularadmin\n' > "${CRED_FILE}"
  chmod 600 "${CRED_FILE}"
fi

if ! IFS=":" read -r ACCESS_KEY SECRET_KEY < "${CRED_FILE}"; then
  echo "[setup-minio-contract] invalid credentials file: ${CRED_FILE}" >&2
  exit 1
fi
if [[ -z "${ACCESS_KEY}" || -z "${SECRET_KEY}" ]]; then
  echo "[setup-minio-contract] empty MinIO credentials in ${CRED_FILE}" >&2
  exit 1
fi

python3 - <<'PY' "${CONTRACT_FILE}" "${NODE_IP}" "${CRED_FILE}" "${CA_BUNDLE}"
import json
import sys

contract_file, node_ip, cred_file, ca_bundle = sys.argv[1:5]
contract = {
    "type": "minio",
    "endpoint": f"{node_ip}:9000",
    "bucket": "globular",
    "prefix": "",
    "secure": True,
    "caBundlePath": ca_bundle,
    "auth": {
        "mode": "file",
        "credFile": cred_file,
    },
}
with open(contract_file + ".tmp", "w", encoding="utf-8") as fh:
    json.dump(contract, fh, indent=2)
    fh.write("\n")
import os
os.replace(contract_file + ".tmp", contract_file)
PY

chown globular:globular "${CRED_FILE}" "${CONTRACT_FILE}" 2>/dev/null || true
chmod 600 "${CRED_FILE}" 2>/dev/null || true
chmod 644 "${CONTRACT_FILE}" 2>/dev/null || true

echo "[setup-minio-contract] wrote ${CONTRACT_FILE} for ${NODE_IP}:9000"
