#!/usr/bin/env bash
# PostToolUse hook for mcp__awg__awareness_briefing.
# Records that a briefing was obtained for a specific file path, so
# enforce-briefing.sh can authorize subsequent Edit/Write/MultiEdit calls
# against that file in the same session.
#
# Marker path: /tmp/claude-awareness-briefings/<session_id>/<sha256(abs_path)>
# Briefings called with task=... (no file) are ignored.

set -euo pipefail

input="$(cat)"
sid="$(printf '%s' "$input" | jq -r '.session_id // "unknown"')"
file="$(printf '%s' "$input" | jq -r '.tool_input.file // empty')"

[ -z "$file" ] && exit 0

PROJECT_ROOT="/home/dave/Documents/github.com/globulario/services"
case "$file" in
  /*) abs="$file" ;;
  *)  abs="$PROJECT_ROOT/$file" ;;
esac

dir="/tmp/claude-awareness-briefings/$sid"
mkdir -p "$dir"
hash="$(printf '%s' "$abs" | sha256sum | awk '{print $1}')"
touch "$dir/$hash"

exit 0
