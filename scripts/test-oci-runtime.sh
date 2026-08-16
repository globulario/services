#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${OCI_SMOKE_WORKDIR:-$(mktemp -d)}"
REGISTRY_PORT="${OCI_SMOKE_REGISTRY_PORT:-15005}"
SERVICE_PORT="${OCI_SMOKE_SERVICE_PORT:-18081}"
REGISTRY_NAME="globular-oci-registry-$$"
IMAGE="localhost:${REGISTRY_PORT}/globular-oci-smoke:ci"
RUNNER="${WORK}/globular-oci-runner"
SPEC="${WORK}/service.json"
STATE_ROOT="${WORK}/state"
RUNNER_PID=""

cleanup() {
  set +e
  if [[ -n "${RUNNER_PID}" ]] && kill -0 "${RUNNER_PID}" 2>/dev/null; then
    kill -TERM "${RUNNER_PID}"
    wait "${RUNNER_PID}" || true
  fi
  docker rm -f globular-oci-smoke >/dev/null 2>&1 || true
  docker rm -f "${REGISTRY_NAME}" >/dev/null 2>&1 || true
  if [[ -z "${OCI_SMOKE_WORKDIR:-}" ]]; then
    rm -rf "${WORK}"
  fi
}
trap cleanup EXIT

command -v docker >/dev/null || { echo "Docker is required" >&2; exit 1; }
command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
docker info >/dev/null

"${ROOT}/scripts/build-oci-runner.sh" "${RUNNER}"
docker run -d --name "${REGISTRY_NAME}" -p "127.0.0.1:${REGISTRY_PORT}:5000" registry:2 >/dev/null

docker build -t "${IMAGE}" "${ROOT}/golang/oci/testdata/smoke" >/dev/null
docker push "${IMAGE}" >/dev/null
REPO_DIGEST="$(docker inspect --format '{{index .RepoDigests 0}}' "${IMAGE}")"
DIGEST="${REPO_DIGEST##*@}"
if [[ ! "${DIGEST}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "registry did not produce an immutable digest: ${REPO_DIGEST}" >&2
  exit 1
fi

cat >"${SPEC}" <<JSON
{
  "api_version": "globular.io/oci/v1alpha1",
  "kind": "OCIService",
  "metadata": { "name": "oci-smoke" },
  "spec": {
    "image": {
      "repository": "localhost:${REGISTRY_PORT}/globular-oci-smoke",
      "tag": "ci",
      "digest": "${DIGEST}",
      "pull_policy": "always"
    },
    "network": {
      "mode": "bridge",
      "ports": [
        { "container_port": 8080, "host_port": ${SERVICE_PORT}, "protocol": "tcp" }
      ]
    },
    "security": {
      "read_only_root_filesystem": true,
      "no_new_privileges": true,
      "drop_capabilities": ["ALL"]
    },
    "health": {
      "startup": {
        "type": "http",
        "address": "127.0.0.1",
        "port": ${SERVICE_PORT},
        "path": "/ready",
        "interval_millis": 200,
        "timeout_millis": 500,
        "failure_threshold": 50
      },
      "readiness": {
        "type": "http",
        "address": "127.0.0.1",
        "port": ${SERVICE_PORT},
        "path": "/ready",
        "interval_millis": 200,
        "timeout_millis": 500,
        "failure_threshold": 20
      },
      "liveness": {
        "type": "http",
        "address": "127.0.0.1",
        "port": ${SERVICE_PORT},
        "path": "/ready",
        "interval_millis": 1000,
        "timeout_millis": 500,
        "failure_threshold": 3
      }
    },
    "lifecycle": {
      "stop_timeout_seconds": 10,
      "remove_on_stop": true
    }
  }
}
JSON

"${RUNNER}" validate --spec "${SPEC}" >/dev/null
"${RUNNER}" run --spec "${SPEC}" --state-root "${STATE_ROOT}" >"${WORK}/runner.out" 2>"${WORK}/runner.err" &
RUNNER_PID=$!

for _ in $(seq 1 60); do
  if curl --fail --silent "http://127.0.0.1:${SERVICE_PORT}/ready" | grep -q ready; then
    break
  fi
  if ! kill -0 "${RUNNER_PID}" 2>/dev/null; then
    cat "${WORK}/runner.err" >&2
    echo "OCI runner exited before readiness" >&2
    exit 1
  fi
  sleep 0.25
done
curl --fail --silent "http://127.0.0.1:${SERVICE_PORT}/ready" | grep -q ready

python3 - "${STATE_ROOT}/oci-smoke/default/observed.json" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as fh:
    state = json.load(fh)
assert state["phase"] == "READY", state
assert state["ready"] is True, state
assert state["image"].split("@", 1)[1].startswith("sha256:"), state
PY

kill -TERM "${RUNNER_PID}"
wait "${RUNNER_PID}"
RUNNER_PID=""

if docker inspect globular-oci-smoke >/dev/null 2>&1; then
  echo "managed container remained after remove_on_stop" >&2
  exit 1
fi

echo "OCI runtime smoke test passed (${REPO_DIGEST})"
