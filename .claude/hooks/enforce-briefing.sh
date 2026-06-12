#!/usr/bin/env bash
# PreToolUse hook for Edit / Write / MultiEdit.
# Blocks any edit targeting a file under a high-risk directory (CLAUDE.md
# hard rule #7) unless mcp__awg__awareness_briefing has already been
# called for that exact file in the current session.
#
# High-risk prefixes are listed below — keep in sync with CLAUDE.md and
# docs/awareness/activation_rules.yaml.

set -euo pipefail

input="$(cat)"
sid="$(printf '%s' "$input" | jq -r '.session_id // "unknown"')"
file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"

[ -z "$file" ] && exit 0

PROJECT_ROOT="/home/dave/Documents/github.com/globulario/services"
case "$file" in
  "$PROJECT_ROOT"/*) rel="${file#$PROJECT_ROOT/}" ;;
  *) exit 0 ;;
esac

high_risk=0
for prefix in \
  "golang/node_agent/" \
  "golang/cluster_controller/" \
  "golang/repository/" \
  "golang/rbac/" \
  "golang/security/" \
  "golang/cluster_doctor/" \
  "golang/mcp/" \
  "golang/ai_executor/" \
  "golang/services_manager/" \
  "docs/awareness/" \
  "docs/intent/"; do
  case "$rel" in
    "$prefix"*) high_risk=1; break ;;
  esac
done

# Narrative carve-out: reports/ and decisions/ subfolders under
# docs/awareness/ and docs/intent/ are post-incident narratives and design
# notes. yaml2nt does not load them as graph anchors, and awareness_briefing
# returns status=empty for them every time — so requiring a briefing call
# adds friction without signal. The load-bearing YAML files in the parent
# dirs (failure_modes.yaml, invariants.yaml, required_tests.yaml, knowledge/,
# generated/, etc.) still trigger the hook.
for narrative in \
  "docs/awareness/reports/" \
  "docs/awareness/decisions/" \
  "docs/intent/reports/" \
  "docs/intent/decisions/"; do
  case "$rel" in
    "$narrative"*) high_risk=0; break ;;
  esac
done

[ "$high_risk" = "0" ] && exit 0

hash="$(printf '%s' "$file" | sha256sum | awk '{print $1}')"
marker="/tmp/claude-awareness-briefings/$sid/$hash"

if [ -e "$marker" ]; then
  exit 0
fi

cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "CLAUDE.md hard rule #7: call mcp__awg__awareness_briefing with file=\"$rel\" BEFORE editing this high-risk path. No 'simple fix' exemption. After the briefing returns, retry this edit."
  }
}
EOF
exit 0
