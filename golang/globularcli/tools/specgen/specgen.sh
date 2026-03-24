#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="/home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin"
OUT_ROOT="$(pwd)/generated"

# Normalize: strips "_server" suffix; keeps underscores for the canonical service name.
# Systemd unit names use dashes (derived separately below).
svc_name_from_exe() {
  local exe="$1"
  echo "${exe%_server}"
}

# Check if service requires Scylla database
needs_scylla() {
  local svc="$1"
  # ScyllaDB is used for resource, rbac, and ai_memory services
  # NOTE: ScyllaDB must be installed and configured with TLS before these services start
  case "${svc}" in
    resource|rbac|ai_memory|workflow) return 0 ;;
    *) return 1 ;;
  esac
}

# Return the Globular service dependencies for a given service.
# Every service implicitly depends on event (via initServiceEvents interceptor hooks),
# plus explicit client dependencies discovered from import analysis.
# Output: space-separated list of systemd unit names (globular-<name>.service).
service_deps() {
  local svc="$1"
  local deps=""

  # Explicit per-service dependencies (runtime client connections).
  #
  # Startup tiers (cycles broken by removing back-edges to lower tiers):
  #   Tier 0 — event, persistence       (infrastructure, no globular deps)
  #   Tier 1 — resource, rbac           (core, depend on tier 0 only)
  #   Tier 2 — everything else          (depend on tier 0 + 1)
  #
  # Cycles removed (lazy runtime connections, retry at startup):
  #   event → resource     (event is foundational, connects lazily)
  #   resource → rbac      (resource calls rbac lazily at runtime)
  #   resource → persistence (both need scylla; resource retries)
  #   persistence → resource (persistence connects lazily)
  #   media → file         (mutual dep; media connects to file lazily)
  case "${svc}" in
    # --- Tier 0: infrastructure (no globular service deps) ---
    event)              deps="" ;;
    persistence)        deps="" ;;
    # --- Tier 1: core (depend on tier 0 only) ---
    resource)           deps="event" ;;
    rbac)               deps="event resource" ;;
    # --- Tier 2: application services ---
    authentication)     deps="ldap rbac resource" ;;
    ai_executor)        deps="ai_memory cluster_controller" ;;
    ai_memory)          deps="resource" ;;
    ai_router)          deps="resource" ;;
    ai_watcher)         deps="ai_executor event resource" ;;
    workflow)           deps="event" ;;
    backup_manager)     deps="resource" ;;
    blog)               deps="event resource" ;;
    catalog)            deps="event persistence resource" ;;
    cluster_controller) deps="event" ;;
    cluster_doctor)     deps="cluster_controller" ;;
    conversation)       deps="event rbac resource" ;;
    discovery)          deps="event rbac resource" ;;
    dns)                deps="rbac resource" ;;
    echo)               deps="resource" ;;
    file)               deps="event media rbac resource search title" ;;
    ldap)               deps="resource" ;;
    log)                deps="event resource" ;;
    mail)               deps="persistence" ;;
    media)              deps="authentication event rbac resource title" ;;
    monitoring)         deps="resource" ;;
    node_agent)         deps="cluster_controller" ;;
    repository)         deps="resource" ;;
    search)             deps="resource" ;;
    title)              deps="event rbac resource" ;;
    torrent)            deps="event rbac" ;;
    # sql, storage: no inter-service dependencies
    *) deps="" ;;
  esac

  # Implicit: all services except tier-0 (event, persistence) depend on event
  # (initServiceEvents connects every service to the event bus).
  if [[ "${svc}" != "event" && "${svc}" != "persistence" ]] && [[ ! " ${deps} " =~ " event " ]]; then
    deps="event ${deps}"
  fi

  # Convert service names to systemd unit names.
  local units=""
  for d in ${deps}; do
    units="${units} globular-${d//_/-}.service"
  done
  echo "${units}" | xargs  # trim whitespace
}

# Check if service must run as root (needs to install packages, manage systemd, create users)
needs_root() {
  local svc="$1"
  case "${svc}" in
    node_agent) return 0 ;;
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

  unit="globular-${svc//_/-}.service"
  spec="${SPECS_DIR}/${svc}_service.yaml"

  # Determine address_host based on service type
  if needs_scylla "${svc}"; then
    address_host="auto"
  else
    # Default to auto-select node advertise address; avoid localhost to prevent
    # Envoy/xDS picking loopback endpoints in multi-service deployments.
    address_host="auto"
  fi

  # node_agent must run as root to install packages, manage systemd units, create users
  if needs_root "${svc}"; then
    run_user="root"
    run_group="root"
  else
    run_user="globular"
    run_group="globular"
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
      # INV-PKI-1: Removed obsolete config/tls directory - all certs under pki/

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

  # TLS cert readiness check — all services need certs before starting.
  # On Day-0, setup-tls.sh runs after systemd units are installed but before
  # services are expected to work. This pre-check waits up to 60s for the
  # service certificate to appear (generated by setup-tls.sh).
  tls_wait='ExecStartPre=/bin/sh -c '"'"'for i in $(seq 1 60); do [ -f /var/lib/globular/pki/issued/services/service.crt ] && exit 0; sleep 1; done; echo "TLS cert not ready"; exit 1'"'"''

  # Ensure working directory exists before starting (node agent doesn't run ensure_dirs).
  # The + prefix runs ExecStartPre as root (needed for chown when User= is set).
  ensure_workdir="ExecStartPre=+/bin/sh -c 'mkdir -p {{.StateDir}}/${svc} && chown ${run_user}:${run_group} {{.StateDir}}/${svc}'"

  # Build systemd dependency lists dynamically from the service dependency graph.
  svc_units="$(service_deps "${svc}")"
  after_deps="network-online.target"
  wants_deps="network-online.target"
  if [[ -n "${svc_units}" ]]; then
    after_deps="${after_deps} ${svc_units}"
    wants_deps="${wants_deps} ${svc_units}"
  fi

  # Add Scylla dependencies if needed
  if needs_scylla "${svc}"; then
    after_deps="${after_deps} scylla-server.service"
    wants_deps="${wants_deps} scylla-server.service"
    cat >> "${spec}" <<EOF
          After=${after_deps}
          Wants=${wants_deps}

          [Service]
          Type=simple
          User=${run_user}
          Group=${run_group}
          WorkingDirectory={{.StateDir}}/${svc}
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
          Environment=GLOBULAR_BOOTSTRAP=1
          ${ensure_workdir}
          ${tls_wait}
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
          After=${after_deps}
          Wants=${wants_deps}

          [Service]
          Type=simple
          User=${run_user}
          Group=${run_group}
          WorkingDirectory={{.StateDir}}/${svc}
          Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
          Environment=GLOBULAR_BOOTSTRAP=1
          ${ensure_workdir}
          ${tls_wait}
EOF

    # DNS service needs CAP_NET_BIND_SERVICE to bind port 53 as non-root
    if [[ "${svc}" == "dns" ]]; then
      cat >> "${spec}" <<EOF
          AmbientCapabilities=CAP_NET_BIND_SERVICE
EOF
    fi

    # Backup manager needs etcdctl and restic on PATH for provider execution
    if [[ "${svc}" == "backup_manager" ]]; then
      cat >> "${spec}" <<EOF
          Environment=PATH={{.Prefix}}/bin:/usr/local/bin:/usr/bin:/bin
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
