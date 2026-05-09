# Agent Awareness Workflow

Every agent (Claude, GPT, or any code-editing AI) that works on this codebase must follow this workflow. Skipping steps is not faster — it creates blind spots that cause incomplete changes and regressions.

---

## Required Workflow (4 steps)

### Step 1: Session Start

Run at the beginning of every development session.

```bash
# MCP (preferred)
awareness.session_start {}

# CLI fallback
globular awareness session-start --output json
```

Read:
- `graph.stale` — if true, rebuild before editing
- `live_overlay.status` — if absent or stale, note the blind spot
- `proposal_queue.status` — if stale, drain before new work
- `blind_spots` — address each before proceeding

---

### Step 2: Before Editing Any File

For each file you plan to edit:

```bash
# MCP (preferred)
awareness.pre_edit_context { "file": "golang/cluster_controller/server.go" }

# Alternative
awareness.file_invariant_context { "file": "golang/cluster_controller/server.go" }

# CLI
globular awareness impact --file golang/cluster_controller/server.go --output json
```

Read:
- `invariants` — what invariants this file implements or enforces
- `edit_warnings` — forbidden actions derived from those invariants
- `required_tests` — tests you MUST run after editing

**Do not proceed if `edit_warnings` contains `FORBIDDEN_FIX` patterns that match your intended change.**

---

### Step 3: Before Committing

Scan for violations across all changed files.

```bash
# MCP
awareness.scan_violations { "paths": ["golang/cluster_controller/server.go"] }

# CLI
globular awareness scan-violations --paths golang/cluster_controller/server.go --output json
```

A clean scan with no findings AND no blind spots means the change is safe to commit.  
A clean scan with blind spots means **the scan could not check everything** — do not treat as safe.

---

### Step 4: After a Verified Fix

When a bug was fixed and the fix is verified by tests:

```bash
# MCP
awareness.learn_from_fix { "incident": "INC-2026-xxxx", "summary": "..." }

# CLI
globular awareness learn-from-fix --incident INC-2026-xxxx --output json
```

This creates a DRAFT proposal. Run `awareness.pending_proposals` and review it before the next session.

---

## Skip-Rate Monitoring

Every call to `awareness.pre_edit_context`, `awareness.agent_context`, and `awareness.preflight` is recorded. Session starts without a preflight call increase the skip rate.

```bash
awareness.agent_usage_report { "window_days": 7 }
```

If `preflight_skip_rate` > 20%, agents are bypassing awareness. Investigate why.

---

## Hook Configuration

To automatically record session starts and pre-commit checks, configure Claude Code hooks:

```json
// .claude/settings.json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [{ "type": "command", "command": "globular awareness hook --file \"$CLAUDE_TOOL_INPUT_PATH\"" }]
      }
    ]
  }
}
```

See `docs/awareness/claude_hooks.md` for full hook configuration.

---

## What Happens When You Skip

| Skipped Step | Risk |
|-------------|------|
| session_start | May work on stale graph — missed invariants |
| pre_edit_context | May violate a forbidden fix pattern unknowingly |
| scan_violations | Commits with undetected violations enter the codebase |
| learn_from_fix | Fix is not remembered — same pattern recurs in next incident |

None of these risks are hypothetical. All four have caused regressions in this codebase.
