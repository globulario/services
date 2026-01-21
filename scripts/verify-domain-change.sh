#!/usr/bin/env bash
set -euo pipefail

DOMAIN=""
PROTOCOL="https"
ACME_FLAG=""
DNS_ADDR="${GLOBULAR_DNS_UDP_ADDR:-127.0.0.1:53}"
ENVOY_URL="${GLOBULAR_HEALTH_ENVOY_URL:-http://127.0.0.1:9901/ready}"
GATEWAY_URL_HTTP="http://127.0.0.1"
GATEWAY_URL_HTTPS="https://127.0.0.1"

usage() {
  cat <<'EOF'
Usage: verify-domain-change.sh --domain <domain> [--protocol http|https] [--acme] [--https-port PORT] [--dns-addr host:port]

Steps:
 1) Runs: globular cluster network set --domain DOMAIN --protocol PROTOCOL [--acme]
 2) Waits briefly, then performs health checks:
    - DNS UDP A lookup for gateway.DOMAIN (using dns-addr)
    - Envoy admin /ready
    - Gateway /health with Host: DOMAIN (http or https)
    - etcd TCP 2379
    - MinIO TCP 9000
    - Scylla TCP 9042

Environment overrides:
  GLOBULAR_HEALTH_ENVOY_URL, GLOBULAR_HEALTH_GATEWAY_URL, GLOBULAR_ETCD_ADDR, GLOBULAR_MINIO_ADDR, GLOBULAR_SCYLLA_ADDR, GLOBULAR_DNS_UDP_ADDR
EOF
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain) DOMAIN="$2"; shift 2;;
    --protocol) PROTOCOL="$2"; shift 2;;
    --acme) ACME_FLAG="--acme"; shift 1;;
    --https-port) HTTPS_PORT="$2"; shift 2;;
    --dns-addr) DNS_ADDR="$2"; shift 2;;
    *) usage;;
  esac
done

[[ -z "$DOMAIN" ]] && usage

wait_for_http() {
  local name="$1"; shift
  local url="$1"; shift
  local extra=("$@")
  for i in {1..30}; do
    if curl -fsSL --max-time 5 "${extra[@]}" "$url" >/dev/null 2>&1; then
      echo "[verify] $name ready at $url"
      return 0
    fi
    sleep 2
  done
  echo "[verify] $name not ready at $url"
  return 1
}

wait_for_https() {
  local name="$1"; shift
  local url="$1"; shift
  local extra=("$@")
  for i in {1..30}; do
    if curl -ksSf --max-time 5 "${extra[@]}" "$url" >/dev/null 2>&1; then
      echo "[verify] $name ready at $url"
      return 0
    fi
    sleep 2
  done
  echo "[verify] $name not ready at $url"
  return 1
}

wait_for_tcp() {
  local name="$1"; shift
  local addr="$1"
  local host="${addr%:*}"; local port="${addr##*:}"
  for i in {1..15}; do
    if nc -z "$host" "$port" >/dev/null 2>&1; then
      echo "[verify] $name TCP $addr ok"
      return 0
    fi
    sleep 2
  done
  echo "[verify] $name TCP $addr failed"
  return 1
}

echo "[verify] applying network change..."
globular cluster network set --domain "$DOMAIN" --protocol "$PROTOCOL" $ACME_FLAG || {
  echo "[verify] network set failed"; exit 1; }

echo "[verify] waiting for control plane..."
wait_for_http "envoy" "$ENVOY_URL"

echo "[verify] DNS check gateway.${DOMAIN} via ${DNS_ADDR}"
if ! dig +short @"${DNS_ADDR%:*}" -p "${DNS_ADDR##*:}" "gateway.${DOMAIN}" | head -n1 | grep -qE '^[0-9]'; then
  echo "[verify] DNS lookup failed"; exit 1
fi

ENVOY_URL="${GLOBULAR_HEALTH_ENVOY_URL:-$ENVOY_URL}"
echo "[verify] Envoy: ${ENVOY_URL}"
wait_for_http "envoy" "$ENVOY_URL"

if [[ "${PROTOCOL}" == "https" ]]; then
  URL="${GLOBULAR_HEALTH_GATEWAY_URL:-${GATEWAY_URL_HTTPS}}"
  PORT="${HTTPS_PORT:-443}"
  echo "[verify] Gateway HTTPS: ${URL}:${PORT}/health Host:${DOMAIN}"
  wait_for_https "gateway" "${URL}:${PORT}/health" "-H" "Host: ${DOMAIN}"
else
  URL="${GLOBULAR_HEALTH_GATEWAY_URL:-${GATEWAY_URL_HTTP}}"
  echo "[verify] Gateway HTTP: ${URL}:${HTTP_PORT:-80}/health Host:${DOMAIN}"
  wait_for_http "gateway" "${URL}:${HTTP_PORT:-80}/health" "-H" "Host: ${DOMAIN}"
fi

ETCD_ADDR="${GLOBULAR_ETCD_ADDR:-127.0.0.1:2379}"
MINIO_ADDR="${GLOBULAR_MINIO_ADDR:-127.0.0.1:9000}"
SCYLLA_ADDR="${GLOBULAR_SCYLLA_ADDR:-127.0.0.1:9042}"

wait_for_tcp "etcd" "$ETCD_ADDR"
wait_for_tcp "minio" "$MINIO_ADDR"
wait_for_tcp "scylla" "$SCYLLA_ADDR"

echo "[verify] SUCCESS: domain change checks passed for ${DOMAIN}"
