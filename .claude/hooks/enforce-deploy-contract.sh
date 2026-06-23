#!/usr/bin/env bash
# PreToolUse hook for Bash — enforce-deploy-contract.sh
#
# Closes the runtime-op governance seam. enforce-briefing*.sh push-gate CODE
# EDITS in high-risk dirs, but RUNTIME operations were discipline-only — which
# let an agent propose `cp <binary> /usr/lib/globular/bin && systemctl restart
# globular-<svc>` from generic sysadmin instinct, the EXPLICITLY FORBIDDEN
# pattern in docs/operational-knowledge/deploy-package-via-mcp.md.
#
# This gate hard-denies the two domain-forbidden runtime operations:
#   1. Writing a binary INTO /usr/lib/globular/bin/ (cp/mv/install/rsync/ln/tee/
#      dd/redirect). Those bytes are package-pipeline outputs ONLY
#      (ai-memory ops.artifacts.root-layout). A hand-placed binary has a
#      different sha256 than the published artifact → the verifier fires
#      package.installed_binary_hash_mismatch and the cluster splits.
#   2. Manual systemd lifecycle of globular-* units (restart/stop/start/reload).
#      Convergence (re)starts a unit through node-agent AFTER verifying the
#      published artifact — manual control bypasses that proof.
#
# The correct path is the 4-layer pipeline:
#   go build -ldflags -> package_build -> package_publish ->
#   `globular services desired set <svc> <v>` -> `globular services repair`
#   (or the wrapper `globular deploy <svc> --bump patch`).
#
# Scope: this governs the AGENT's Bash. A human operator intentionally
# overriding runs the command themselves via the `!` prefix — same human-
# authorizes-destructive-state pattern as the AWG store reseed. Read-only
# systemctl (is-active/status/show/list-units/cat) and reads/backups OUT of the
# bin dir are allowed.

set -euo pipefail

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty')"
[ -z "$cmd" ] && exit 0

# Carve-out: git and go commands never write binaries into the protected bin
# dir or run systemctl. Their arguments (commit messages, file lists) routinely
# mention forbidden strings in PROSE — `git commit -m "... systemctl restart
# globular-x ..."` is not an execution. Skip, mirroring enforce-briefing-bash.sh.
case "$cmd" in
  "git "*|"git"$'\t'*) exit 0 ;;
  "go "*|"go"$'\t'*) exit 0 ;;
  "gh "*|"gh"$'\t'*) exit 0 ;;
esac

BIN_DIR="/usr/lib/globular/bin"
PLAYBOOK="docs/operational-knowledge/deploy-package-via-mcp.md"

deny() {
  # $1 = reason (already escaped-safe: no embedded double quotes)
  cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "$1"
  }
}
EOF
  exit 0
}

# under_bindir TOKEN -> exit 0 if the token resolves under the protected bin dir.
under_bindir() {
  case "$1" in
    "$BIN_DIR"/*|"$BIN_DIR") return 0 ;;
    *) return 1 ;;
  esac
}

REASON_BIN="FORBIDDEN runtime op: writing into ${BIN_DIR} bypasses the 4-layer package deploy contract (Repository->Desired->Installed->Runtime). Binaries there are package-pipeline outputs ONLY; a hand-placed binary fails the verifier (sha256 != manifest entrypoint_checksum) and splits the cluster. Use the pipeline in ${PLAYBOOK}: build -ldflags -> package_build -> package_publish -> 'globular services desired set' -> 'globular services repair' (or 'globular deploy <svc> --bump patch'). A human operator overriding intentionally must run it via the ! prefix."

REASON_UNIT="FORBIDDEN runtime op: manual systemctl control of a globular-* unit bypasses the package deploy/convergence contract. Service (re)starts happen through node-agent during 'globular services repair' AFTER the published artifact is verified. See ${PLAYBOOK}. Read-only systemctl (is-active/status/show/list-units) is fine. A human operator overriding intentionally must run it via the ! prefix."

# ── Check 1: binary placement into the protected bin dir ──────────────────────

# Redirect into the bin dir:  > /usr/lib/globular/bin/...   >> ...   &> ...
if printf '%s' "$cmd" | grep -qE "(>{1,2}|&>)[[:space:]]*${BIN_DIR}/"; then
  deny "$REASON_BIN"
fi

# dd of=/usr/lib/globular/bin/...
if printf '%s' "$cmd" | grep -qE "\bdd\b[^;&|]*of=${BIN_DIR}/"; then
  deny "$REASON_BIN"
fi

# tee [flags] /usr/lib/globular/bin/...  (tee writes to its path args)
if printf '%s' "$cmd" | grep -qE "\btee\b[^;&|]*${BIN_DIR}/"; then
  deny "$REASON_BIN"
fi

# cp / mv / install / rsync / ln — the WRITE TARGET is the last positional arg.
# This allows reads/backups OUT of the bin dir (last arg elsewhere) while
# blocking writes INTO it. Scan each ;/&&/|-separated segment.
while IFS= read -r seg; do
  case "$seg" in
    *cp*|*mv*|*install*|*rsync*|*ln*) : ;;
    *) continue ;;
  esac
  # Only act when the segment's command word is one of the placement verbs
  # (strip a leading sudo / env).
  verb="$(printf '%s' "$seg" | sed -E 's/^[[:space:]]*(sudo[[:space:]]+)?(env[[:space:]]+[^[:space:]]+[[:space:]]+)?//' | awk '{print $1}')"
  case "$verb" in
    cp|mv|install|rsync|ln) ;;
    *) continue ;;
  esac
  last="$(printf '%s' "$seg" | awk '{print $NF}')"
  # Trim quotes/punctuation.
  last="$(printf '%s' "$last" | sed -E 's/^["'"'"']+//; s/["'"'"';)]+$//')"
  if under_bindir "$last"; then
    deny "$REASON_BIN"
  fi
done < <(printf '%s\n' "$cmd" | tr ';&|' '\n')

# ── Check 2: manual systemd lifecycle of globular-* units ─────────────────────
# Block restart/stop/start/reload/reload-or-restart/try-restart of globular-*.
# Read-only verbs are not matched.
if printf '%s' "$cmd" | grep -qE '\bsystemctl\b([[:space:]]+--[^[:space:]]+)*[[:space:]]+(restart|stop|start|reload|reload-or-restart|try-restart|try-reload-or-restart)[[:space:]]+[^;&|]*globular-'; then
  deny "$REASON_UNIT"
fi

exit 0
