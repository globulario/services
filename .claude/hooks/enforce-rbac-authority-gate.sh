#!/usr/bin/env bash
# PreToolUse hook for Edit / Write / MultiEdit.
#
# Behavioral-memory action brake for the RBAC built-in-superadmin authority
# surface. Edits to the files that recognize/authorize the built-in "sa"
# superadmin must pass through the behavioral_check_action gate before they land.
#
# This is the runtime "tooth" for the principle
#   cluster_operator: "Built-in superadmin sa recognition must use the one
#   canonical predicate"  (proposed; promote-gated).
# AWG (briefing) + the three regression tests remain the PRIMARY enforcement;
# this hook is only the brake — it does NOT prove semantics, it detects the
# authority surface and obeys the recorded gate verdict.
#
# MECHANISM (mirrors enforce-briefing.sh): a PreToolUse bash hook cannot call an
# MCP tool itself. The model calls mcp__globular__behavioral_check_action; a
# companion PostToolUse hook (record-rbac-gate.sh) writes an "allowed" marker for
# that file+session; this hook checks for the marker.
#
# MODE (so the hook can be installed BEFORE the principle is promoted):
#   warn   (default) — no marker => ALLOW, emit a systemMessage nudge. Safe to
#                       land and exercise without freezing edits.
#   strict          — no allowed-marker => DENY with the exact gate call to make.
# Set via .claude/hooks/rbac-gate.mode (single word "strict" or "warn"), or the
# RBAC_GATE_MODE env var. Flip to strict only AFTER the principle is promoted.

set -euo pipefail

input="$(cat)"
sid="$(printf '%s' "$input" | jq -r '.session_id // "unknown"')"
file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"

[ -z "$file" ] && exit 0

PROJECT_ROOT="${CLAUDE_PROJECT_DIR:-/home/dave/Documents/github.com/globulario/services}"
case "$file" in
  "$PROJECT_ROOT"/*) rel="${file#"$PROJECT_ROOT"/}" ;;
  *) exit 0 ;;
esac

# Tightly scoped: only the files that recognize/authorize the built-in sa
# superadmin. NOT the whole rbac/ tree — keep this a narrow tripwire.
authority_surface=0
for f in \
  "golang/rbac/rbac_server/rbac_access.go" \
  "golang/rbac/rbac_server/rbac_role_bindings.go" \
  "golang/rbac/rbac_server/rbac_ownership.go" \
  "golang/rbac/rbac_server/server.go" \
  "golang/security/path.go"; do
  [ "$rel" = "$f" ] && { authority_surface=1; break; }
done

[ "$authority_surface" = "0" ] && exit 0

# Resolve mode: env var wins, else mode file, else default warn.
mode="${RBAC_GATE_MODE:-}"
if [ -z "$mode" ] && [ -f "$PROJECT_ROOT/.claude/hooks/rbac-gate.mode" ]; then
  mode="$(tr -d '[:space:]' < "$PROJECT_ROOT/.claude/hooks/rbac-gate.mode")"
fi
[ -z "$mode" ] && mode="warn"

# Allowed-marker written by record-rbac-gate.sh after the model receives an
# "allowed" verdict from behavioral_check_action for this file. Keyed by the
# REPO-RELATIVE path so the recorder (which only sees the relative target_ref)
# and this enforcer agree on the marker name.
hash="$(printf '%s' "$rel" | sha256sum | awk '{print $1}')"
marker="/tmp/claude-rbac-authority-gate/$sid/$hash"

# TTL: a stale "allowed" verdict from earlier must not unlock a later edit.
marker_ttl=2700 # 45 minutes
if [ -e "$marker" ]; then
  now="$(date +%s)"
  mtime="$(stat -c %Y "$marker" 2>/dev/null || echo 0)"
  [ "$((now - mtime))" -lt "$marker_ttl" ] && exit 0
  rm -f "$marker" # stale — require a fresh verdict
fi

# No allowed verdict on record for this file this session.
gate_hint="Call mcp__globular__behavioral_check_action FIRST: project=globular-services domain=cluster_operator action_type=code_change.security_authority.rbac_superadmin_recognition target_ref=\"$rel\" conditions=\"change.touches_rbac_sa_or_superadmin_recognition\" provided_evidence_refs=\"test:TestIsBuiltinSuperadmin,test:TestCallerIsAdmin_BuiltinSA,test:TestGetRoleBinding_SAReadsOtherSubject_NoFallback\". Proceed only on verdict=allowed; on needs_evidence add/run the listed tests; on blocked you matched a forbidden move (inline sa check / no @-strip / sa-admin-via-stored-binding-only / weakened deny-overrides-allow) — revise. Do NOT bypass the canonical isBuiltinSuperadmin predicate."

# Build output via jq so $rel / $gate_hint are JSON-escaped (gate_hint contains
# quotes; raw heredoc interpolation would emit invalid JSON).
if [ "$mode" = "strict" ]; then
  reason="RBAC superadmin-authority surface ($rel): no allowed behavioral gate verdict on record this session. $gate_hint"
  jq -n --arg r "$reason" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}}'
  exit 0
fi

# warn mode: allow the edit, but surface the gate nudge.
msg="RBAC superadmin-authority surface ($rel) edited without a behavioral gate verdict (mode=warn). $gate_hint"
jq -n --arg m "$msg" '{systemMessage: $m}'
exit 0
