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

# Check if service requires Scylla database
needs_scylla() {
  local svc="$1"
  # ScyllaDB is used for resource and rbac services
  # NOTE: ScyllaDB must be installed and configured with TLS before these services start
  case "${svc}" in
    resource|rbac) return 0 ;;
    *) return 1 ;;
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

  # Determine address_host based on service type
  if needs_scylla "${svc}"; then
    address_host="auto"
  else
    # Default to auto-select node advertise address; avoid localhost to prevent
    # Envoy/xDS picking loopback endpoints in multi-service deployments.
    address_host="auto"
  fi

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
      - path: "{{.StateDir}}/${svc}"
        owner: globular
        group: globular
        mode: 0750

      # TLS directories (certificates must be provisioned by Day-0 bootstrap)
      - path: "{{.StateDir}}/pki"
        owner: globular
        group: globular
        mode: 0750
      - path: "{{.StateDir}}/config/tls"
        owner: globular
        group: globular
        mode: 0750

  - id: install-${svc}-payload
    type: install_package_payload
    install_bins: true
    install_config: false
    install_spec: false
    install_systemd: false

  - id: ensure-${svc}-config
    type: ensure_service_config
    service_name: ${svc}
    exec: ${exe}
    address_host: ${address_host}
    rewrite_if_out_of_range: true

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
EOF

  # Add Scylla dependencies if needed
  if needs_scylla "${svc}"; then
    cat >> "${spec}" <<EOF
          After=network-online.target scylla-server.service
          Wants=network-online.target scylla-server.service

          [Service]
          Type=simple
          User=globular
          Group=globular
          WorkingDirectory={{.StateDir}}/${svc}
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
          Environment=GLOBULAR_BOOTSTRAP=1
          ExecStartPre=/bin/sh -c 'for i in \$(seq 1 90); do ss -lnt | grep -q ":9042 " && exit 0; sleep 1; done; echo "scylla 9042 not ready"; exit 1'
          ExecStart={{.Prefix}}/bin/${exe}
          Restart=on-failure
          RestartSec=2
          LimitNOFILE=524288

          [Install]
          WantedBy=multi-user.target
EOF
  else
    cat >> "${spec}" <<EOF
          After=network-online.target
          Wants=network-online.target

          [Service]
          Type=simple
          User=globular
          Group=globular
          WorkingDirectory={{.StateDir}}/${svc}
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
          Environment=GLOBULAR_BOOTSTRAP=1
EOF

    # DNS service needs CAP_NET_BIND_SERVICE to bind port 53 as non-root
    if [[ "${svc}" == "dns" ]]; then
      cat >> "${spec}" <<EOF
          AmbientCapabilities=CAP_NET_BIND_SERVICE
EOF
    fi

    cat >> "${spec}" <<EOF
          ExecStart={{.Prefix}}/bin/${exe}
          Restart=on-failure
          RestartSec=2
          LimitNOFILE=524288

          [Install]
          WantedBy=multi-user.target
EOF
  fi

  cat >> "${spec}" <<EOF

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
