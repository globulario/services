#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="/home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin"
OUT_ROOT="$(pwd)/generated"

# Normalize: strips "_server" suffix and replaces underscores with dashes
svc_name_from_exe() {
  local exe="$1"
  local base="${exe%_server}"

  case "${base}" in
    clustercontroller) echo "cluster-controller" ;;
    nodeagent) echo "node-agent" ;;
    *) echo "${base//_/-}" ;;
  esac
}

usage() {
  cat <<EOF
Usage: $0 <BIN_DIR> <OUT_ROOT>

Generates:
  <OUT_ROOT>/specs/<svc>_service.yaml
  <OUT_ROOT>/config/<svc>/config.json

Arguments:
  BIN_DIR   Directory containing *_server binaries
  OUT_ROOT  Output directory (default recommended: ./generated)

Example:
  $0 /path/to/stage/bin ./generated
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -lt 2 ]]; then
  usage >&2
  exit 2
fi

BIN_DIR="$1"
OUT_ROOT="$2"
SPECS_DIR="${OUT_ROOT}/specs"
CONFIG_DIR="${OUT_ROOT}/config"
mkdir -p "${SPECS_DIR}" "${CONFIG_DIR}"

yaml_indent_json_block() {
  # Reads JSON from stdin; prints YAML block content indented by 10 spaces.
  python3 - <<'PY'
import sys, json
j = json.load(sys.stdin)
s = json.dumps(j, indent=2)
pad = " " * 10
print("\n".join(pad + line for line in s.splitlines()))
PY
}

for exe_path in "${BIN_DIR}"/*_server; do
  [ -x "${exe_path}" ] || continue
  exe="$(basename "${exe_path}")"
  svc="$(svc_name_from_exe "${exe}")"

  echo "==> ${exe} -> ${svc}"

  # 1) default config from --describe
  desc_json="$("${exe_path}" --describe 2>/dev/null || true)"
  if [ -z "${desc_json}" ]; then
    echo "WARN: ${exe} --describe returned empty; skipping"
    continue
  fi
  if ! printf '%s' "${desc_json}" | python3 -c 'import sys,json; json.load(sys.stdin)' >/dev/null 2>&1; then
    echo "WARN: ${exe} --describe did not return valid JSON; skipping"
    continue
  fi

  mkdir -p "${CONFIG_DIR}/${svc}"
  echo "${desc_json}" > "${CONFIG_DIR}/${svc}/config.json"

  # 2) discover whether --config exists
  has_config="false"
  if "${exe_path}" --help 2>/dev/null | grep -Eq -- '(^|[[:space:]])-(-)?config([[:space:]]|=|$)'; then
    has_config="true"
  fi

  # 3) derive listen address/port (best-effort)
  # Prefer Address if present, else fallback to localhost:0
  addr="$(echo "${desc_json}" | python3 -c 'import sys,json; j=json.load(sys.stdin); print(j.get("Address","localhost:0"))')"
  port="$(echo "${desc_json}" | python3 -c 'import sys,json; j=json.load(sys.stdin); print(j.get("Port",0))')"
  # For naming only; spec itself doesn't need port unless you want it

  unit="globular-${svc}.service"
  spec="${SPECS_DIR}/${svc}_service.yaml"

  # 4) write spec (package spec: no staging, no install_package_payload)
  cat > "${spec}" <<EOF
version: 1

metadata:
  name: ${svc}

service:
  name: ${svc}
  exec: ${exe}

steps:
  - id: ensure-user-group
    type: ensure_user_group
    user: globular
    group: globular
    home: "{{.StateDir}}"
    shell: /usr/sbin/nologin
    system: true

  - id: ensure-dirs
    type: ensure_dirs
    dirs:
      - path: "{{.Prefix}}"
        owner: root
        group: root
        mode: 0755
      - path: "{{.Prefix}}/bin"
        owner: root
        group: root
        mode: 0755

      - path: "{{.StateDir}}"
        owner: globular
        group: globular
        mode: 0750
      - path: "{{.StateDir}}/${svc}"
        owner: globular
        group: globular
        mode: 0750

      - path: "{{.ConfigDir}}"
        owner: root
        group: root
        mode: 0755
      - path: "{{.ConfigDir}}/${svc}"
        owner: root
        group: root
        mode: 0755

  - id: install-${svc}-config
    type: install_files
    files:
      - path: "{{.ConfigDir}}/${svc}/config.json"
        owner: root
        group: root
        mode: 0644
        atomic: true
        # NOTE: this is intended to be replaced by node-agent/controller later.
        # It is seeded from '${exe} --describe'.
        content: |
$(printf '%s' "${desc_json}" | yaml_indent_json_block)

  - id: install-${svc}-service
    type: install_services
    units:
      - name: ${unit}
        owner: root
        group: root
        mode: 0644
        atomic: true
        content: |
          [Unit]
          Description=Globular ${svc}
          After=network-online.target
          Wants=network-online.target

          [Service]
          Type=simple
          User=globular
          Group=globular
          WorkingDirectory={{.StateDir}}/${svc}
EOF

  if [ "${has_config}" = "true" ]; then
    cat >> "${spec}" <<EOF
          ExecStart={{.Prefix}}/bin/${exe} --config {{.ConfigDir}}/${svc}/config.json
EOF
  else
    cat >> "${spec}" <<EOF
          ExecStart={{.Prefix}}/bin/${exe}
EOF
  fi

  cat >> "${spec}" <<EOF
          Restart=on-failure
          RestartSec=2
          LimitNOFILE=524288

          [Install]
          WantedBy=multi-user.target

  - id: enable-${svc}
    type: enable_services
    services:
      - ${unit}

  - id: start-${svc}
    type: start_services
    services:
      - ${unit}
    restart_on_files:
      ${unit}:
        - "{{.ConfigDir}}/${svc}/config.json"
    binaries:
      ${unit}: ${exe}

  - id: health-${svc}
    type: health_checks
    services:
      - ${unit}
EOF

done

echo "Done. Output in: ${OUT_ROOT}"
