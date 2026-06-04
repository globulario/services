#!/usr/bin/env bash
# PreToolUse hook for Edit / Write / MultiEdit.
# Blocks writes targeting the project-specific flat-file memory directory.
#
# CLAUDE.md says: for project "globular-services", use ai-memory
# (mcp__globular__memory_*) — NOT flat-file memory. This hook enforces it.
#
# The session-default memory framework defaults to flat files; that
# default leaks through CLAUDE.md unless this hook hard-blocks it.

set -euo pipefail

input="$(cat)"
file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"
[ -z "$file" ] && exit 0

# Resolve to absolute (file_path is already absolute per Write/Edit contract,
# but normalize just in case).
case "$file" in
  /*) abs="$file" ;;
  *)  abs="$PWD/$file" ;;
esac

# The project-specific flat-file memory dir for globular-services.
BLOCKED_PREFIX="/home/dave/.claude/projects/-home-dave-Documents-github-com-globulario-services/memory/"

# MEMORY.md is the index — allow edits so obsolete pointers can be
# cleaned up during the migration to ai-memory.
case "$abs" in
  "$BLOCKED_PREFIX"MEMORY.md)
    exit 0
    ;;
  "$BLOCKED_PREFIX"*)
    cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "CLAUDE.md says: for project 'globular-services', use ai-memory (mcp__globular__memory_store / memory_query / memory_update), NOT flat-file memory entry files. Use mcp__globular__memory_store with project='globular-services'. MEMORY.md (the index) is still editable for migration cleanup; new entries must go to ai-memory."
  }
}
EOF
    exit 0
    ;;
esac

exit 0
