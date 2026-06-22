#!/usr/bin/env bash
# PostToolUse hook for mcp__globular__behavioral_check_action.
#
# The "keyhole" for enforce-rbac-authority-gate.sh. When the model calls the
# behavioral gate for the RBAC superadmin-authority surface AND the verdict is a
# clean "allowed", this records an auditable marker that the PreToolUse enforcer
# checks before permitting an Edit/Write to one of the 5 authority files.
#
# Conservative / fail-closed: writes a marker ONLY when every condition below
# holds. Anything ambiguous (needs_evidence, blocked, no principle, malformed or
# unrecognized tool_response, missing/foreign target_ref, missing evidence) =>
# NO marker. In strict mode that means the edit stays denied — the safe
# direction for a brake.
#
# Call inputs (project/domain/action_type/target_ref/provided_evidence_refs) are
# read from tool_input, whose schema is known. The verdict is read from
# tool_response; its exact field name was not observable when this was authored
# (gRPC backend offline), so verdict parsing is defensive across likely shapes
# and requires a POSITIVE structured "allowed" — never a substring guess.

set -euo pipefail

input="$(cat)"

tool="$(printf '%s' "$input" | jq -r '.tool_name // empty')"
case "$tool" in
  *behavioral_check_action) ;;
  *) exit 0 ;;
esac

sid="$(printf '%s' "$input" | jq -r '.session_id // "unknown"')"

# --- call inputs (known schema: tool_input == the gate call args) ------------
proj="$(printf '%s'   "$input" | jq -r '.tool_input.project // empty')"
domain="$(printf '%s' "$input" | jq -r '.tool_input.domain // empty')"
action="$(printf '%s' "$input" | jq -r '.tool_input.action_type // empty')"
target="$(printf '%s' "$input" | jq -r '.tool_input.target_ref // empty')"
evidence="$(printf '%s' "$input" | jq -r '.tool_input.provided_evidence_refs // empty')"

# Identity gate — must be exactly the RBAC superadmin-authority action.
[ "$proj" = "globular-services" ] || exit 0
[ "$domain" = "cluster_operator" ] || exit 0
[ "$action" = "code_change.security_authority.rbac_superadmin_recognition" ] || exit 0

# target_ref must be one of the 5 exact authority files (the same allowlist the
# enforcer scopes to). Anything else => no marker.
case "$target" in
  golang/rbac/rbac_server/rbac_access.go|\
  golang/rbac/rbac_server/rbac_role_bindings.go|\
  golang/rbac/rbac_server/rbac_ownership.go|\
  golang/rbac/rbac_server/server.go|\
  golang/security/path.go) ;;
  *) exit 0 ;;
esac

# Required evidence: all three regression tests must be cited in the call.
for t in \
  "test:TestIsBuiltinSuperadmin" \
  "test:TestCallerIsAdmin_BuiltinSA" \
  "test:TestGetRoleBinding_SAReadsOtherSubject_NoFallback"; do
  case ",$evidence," in
    *",$t,"*) ;;
    *) exit 0 ;;
  esac
done

# --- verdict (tool_response; defensive, fail-closed) -------------------------
# Try common structured fields; lowercase; require an exact "allowed".
verdict="$(printf '%s' "$input" | jq -r '
  (.tool_response // {}) as $r
  | ( ($r | objects | (.status // .verdict // .decision // .result))
      // ($r | strings)
      // ($r.content? | if type=="array"
           then (map(select(.type=="text") | .text) | join(" "))
           else . end)
      // "" )
  | tostring | ascii_downcase' 2>/dev/null || echo "")"

# Positive match only: the verdict value is exactly "allowed". A response that
# merely contains the word (e.g. "not allowed", "needs_evidence ... allowed if")
# must NOT pass — so require the trimmed value to equal allowed.
verdict_trimmed="$(printf '%s' "$verdict" | tr -d '[:space:]')"
[ "$verdict_trimmed" = "allowed" ] || exit 0

# --- write the auditable marker ----------------------------------------------
hash="$(printf '%s' "$target" | sha256sum | awk '{print $1}')"
dir="/tmp/claude-rbac-authority-gate/$sid"
mkdir -p "$dir"
recorded_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

jq -n \
  --arg project "$proj" \
  --arg domain "$domain" \
  --arg action_type "$action" \
  --arg target_ref "$target" \
  --arg recorded_at "$recorded_at" \
  '{
    project: $project,
    domain: $domain,
    action_type: $action_type,
    target_ref: $target_ref,
    verdict: "allowed",
    evidence: [
      "test:TestIsBuiltinSuperadmin",
      "test:TestCallerIsAdmin_BuiltinSA",
      "test:TestGetRoleBinding_SAReadsOtherSubject_NoFallback"
    ],
    recorded_at: $recorded_at
  }' > "$dir/$hash"

exit 0
