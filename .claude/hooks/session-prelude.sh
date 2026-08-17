#!/usr/bin/env bash
# SessionStart hook for the globular-services project.
# Injects a short rule reminder so the highest-priority rules surface
# in Claude's attention before the first user message.
#
# This is the prosthetic that replaces "Claude will remember next time."
# Claude has no continuous memory across sessions; this hook makes the
# session-start rules visible at the moment they're needed.

set -euo pipefail

cat <<'EOF'
{
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "GLOBULAR-SERVICES SESSION PRELUDE — read before any tool call.\n\n1. MEMORY: This project uses ai-memory. Call mcp__globular__memory_store / memory_query / memory_update with project='globular-services'. Do NOT write to /home/dave/.claude/projects/.../memory/ (a PreToolUse hook will deny it). MEMORY.md is still readable as the index but new entries go to ai-memory.\n\n2. AWARENESS-FIRST: Before editing any file under golang/node_agent/, golang/cluster_controller/, golang/repository/, golang/rbac/, golang/security/, golang/cluster_doctor/, golang/mcp/, golang/ai_executor/, golang/services_manager/, docs/awareness/, or docs/intent/ — call mcp__awg__awareness_briefing(file=<path>) FIRST. A PreToolUse hook will deny Edit/Write/MultiEdit without it. No 'simple fix' exemption.\n\n3. ASK THE GRAPH, DON'T GREP: When you need to know what intent/invariant/failure-mode applies, use mcp__awg__awareness_query / awareness_resolve / awareness_briefing — NOT grep over docs/intent/ or docs/awareness/. The graph has the relationships; the YAML files are inputs.\n\n4. BEHAVIORAL LOOP: For non-trivial software work, use behavioral memory with project='globular-services', domain='programming'. Resolve governed context for the goal and, before a risky or semantically classified action, call mcp__globular__behavioral_check_action with the applicable programming conditions/action type. Treat allowed=true + governed=false as a COVERAGE GAP, never as a safety endorsement. After a checked action has a verified result, record the outcome against its action_check_id so experience can accumulate. Read cluster_learning_report / behavioral_list_promotion_candidates when a gap repeats. NEVER auto-promote a candidate; promotion remains an explicit governed human decision.\n\n5. END NON-TRIVIAL TASKS with the awareness template from CLAUDE.md (briefing used, invariants touched, tests run, remaining uncertainty).\n\nIf you find yourself defaulting to flat-file memory, grep over awareness sources, editing high-risk code without calling briefing, or treating an ungoverned behavioral allow as approval — STOP. Those are exactly the drift modes these controls exist to expose."
  }
}
EOF
