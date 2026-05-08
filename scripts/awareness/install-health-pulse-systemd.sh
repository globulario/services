#!/usr/bin/env bash
# Installs the awareness-health-pulse systemd service and timer on a Linux cluster node.
# Idempotent: safe to run multiple times.
#
# Usage: sudo bash scripts/awareness/install-health-pulse-systemd.sh
#
# Prerequisites:
#   - globular binary in PATH
#   - globular-mcp.service running and accessible
#   - Running as root (required for systemd unit installation)
#
# Activation status: prepared_not_installed
#   This script is ready to run but has not been executed on a live cluster.
#   It requires systemd + the globular binary installed on the target node.

set -euo pipefail

GLOBULAR_BIN="${GLOBULAR_BIN:-/usr/local/bin/globular}"
SERVICE_USER="${SERVICE_USER:-globular}"
LOG_DIR="${LOG_DIR:-/var/log/globular}"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_FILE="${SYSTEMD_DIR}/awareness-health-pulse.service"
TIMER_FILE="${SYSTEMD_DIR}/awareness-health-pulse.timer"

# ── Guards ──────────────────────────────────────────────────────────────────

if [[ "${EUID}" -ne 0 ]]; then
  echo "ERROR: must run as root (sudo bash $0)" >&2
  exit 1
fi

if ! command -v "${GLOBULAR_BIN}" &>/dev/null; then
  echo "ERROR: globular binary not found at ${GLOBULAR_BIN}" >&2
  echo "  Set GLOBULAR_BIN=/path/to/globular or install the binary first." >&2
  exit 1
fi

if ! id "${SERVICE_USER}" &>/dev/null; then
  echo "ERROR: service user '${SERVICE_USER}' does not exist" >&2
  echo "  Create the user or set SERVICE_USER to an existing user." >&2
  exit 1
fi

# ── Log directory ────────────────────────────────────────────────────────────

mkdir -p "${LOG_DIR}"
chown "${SERVICE_USER}:${SERVICE_USER}" "${LOG_DIR}"
chmod 0755 "${LOG_DIR}"

# ── Write service unit ───────────────────────────────────────────────────────

cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Awareness health pulse check
After=network.target

[Service]
Type=oneshot
User=${SERVICE_USER}
# Exit codes: 0=healthy 1=warning 2=critical 3=check_failed
ExecStart=${GLOBULAR_BIN} awareness mcp-call awareness.health_pulse \\
  --arg stale_proposal_hours=24 \\
  --arg include_graph_check=true
StandardOutput=journal
StandardError=journal
# Do not restart on failure — the timer handles re-scheduling.
EOF

echo "Wrote ${SERVICE_FILE}"

# ── Write timer unit ─────────────────────────────────────────────────────────

cat > "${TIMER_FILE}" <<EOF
[Unit]
Description=Run awareness health pulse every 30 minutes
Requires=awareness-health-pulse.service

[Timer]
OnBootSec=5min
OnUnitActiveSec=30min
AccuracySec=1min
Persistent=true

[Install]
WantedBy=timers.target
EOF

echo "Wrote ${TIMER_FILE}"

# ── Enable and start ─────────────────────────────────────────────────────────

systemctl daemon-reload
systemctl enable --now awareness-health-pulse.timer

echo ""
echo "Installation complete. Run the following to verify:"
echo ""
echo "  # Timer is listed and shows NEXT/LAST timestamps:"
echo "  systemctl list-timers awareness-health-pulse.timer --no-pager"
echo ""
echo "  # Service ran without error (after first firing, ~5 min after boot):"
echo "  journalctl -u awareness-health-pulse.service -n 20 --no-pager"
echo ""
echo "  # Most recent exit code is 0 or 1:"
echo "  journalctl -u awareness-health-pulse.service -n 1 --no-pager | grep -E 'exit_code.: [01]'"
echo ""
echo "  # Manual one-shot invocation to test immediately:"
echo "  systemctl start awareness-health-pulse.service"
echo "  journalctl -u awareness-health-pulse.service -n 5 --no-pager"
echo ""

# Print current timer state
echo "Current timer state:"
systemctl list-timers awareness-health-pulse.timer --no-pager || true
