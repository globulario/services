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
mkdir -p "${SPECS_DIR}"

for exe_path in "${BIN_DIR}"/*_server; do
  [ -x "${exe_path}" ] || continue
  exe="$(basename "${exe_path}")"
  svc="$(svc_name_from_exe "${exe}")"

  echo "==> ${exe} -> ${svc}"

  unit="globular-${svc}.service"
  spec="${SPECS_DIR}/${svc}_service.yaml"

  # Write spec - services store their own config as <uuid>.json in services directory
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
      - path: "{{.StateDir}}/services"
        owner: globular
        group: globular
        mode: 0750

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
          WorkingDirectory={{.StateDir}}/services
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
          ExecStart={{.Prefix}}/bin/${exe}
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
    binaries:
      ${unit}: ${exe}

  - id: health-${svc}
    type: health_checks
    services:
      - ${unit}
EOF

done

echo "Done. Output in: ${OUT_ROOT}"
