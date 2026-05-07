# Claude Code Awareness Hooks

The `awareness hook` command integrates with Claude Code's **PreToolUse hooks**
to surface architecture constraints before Claude edits any file in the Globular
codebase.

---

## How it works

When Claude is about to call `Edit` or `Write` on a Go file, the hook:

1. Validates all `//globular:` annotations in that file for syntax errors.
2. Queries the awareness graph for invariants, forbidden fixes, and risks
   attached to symbols defined in that file.
3. Outputs a markdown summary to stdout that Claude reads before proceeding.

Default hook mode is warning-only (`exit 0`). Strict mode can block high-risk
edits (`exit 2`) when awareness safety gates fail.

---

## Configuration

Two hook types work together:

- **PreToolUse** — runs `awareness hook` before an edit, surfaces invariants and forbidden fixes
- **PostToolUse** — runs `check-edit` after an edit, surfaces code smells and pattern violations

Add both to `~/.claude/settings.json` (global) or `.claude/settings.local.json` (project):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "globular awareness hook --file \"${CLAUDE_TOOL_INPUT_FILE_PATH:-${CLAUDE_TOOL_INPUT_FILE:-}}\" --task \"$CLAUDE_TASK\""
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "globular awareness check-edit --file \"${CLAUDE_TOOL_INPUT_FILE_PATH:-${CLAUDE_TOOL_INPUT_FILE:-}}\" --format agent 2>/dev/null || true"
          }
        ]
      }
    ]
  }
}
```

The file-path variable uses a fallback chain (`${CLAUDE_TOOL_INPUT_FILE_PATH:-${CLAUDE_TOOL_INPUT_FILE:-}}`) to support both current and older Claude Code versions. If the variable is empty, both hooks skip silently.

See `.claude/settings.example.json` for a copy-paste ready config with strict mode options.

Or for a project-scoped hook, add to `.claude/settings.json` at the repo root.

---

## Manual invocation

```bash
# Single file (warning mode)
globular awareness hook \
  --file golang/node_agent/node_agent_server/heartbeat.go \
  --task "fix heartbeat interval logic"

# Multiple files (warning mode)
globular awareness hook \
  --file golang/cluster_controller/cluster_controller_server/release_hash.go \
  --file golang/node_agent/node_agent_server/installed_services.go \
  --task "update hash computation"

# Strict mode (high-risk watchlist, can block with exit 2)
globular awareness hook \
  --strict \
  --watchlist docs/awareness/high_risk_files.yaml \
  --file "$CLAUDE_TOOL_INPUT_FILE_PATH" \
  --task "$CLAUDE_TASK"
```

---

## PreToolUse hook output

```
## Awareness hook: PASS

**Task**: fix heartbeat interval logic

### Architecture constraints for edited files

- INVARIANT: infra.heartbeat_not_desired_authority
- RISK: infra.heartbeat_sets_desired_state
- STATE TRANSITION: INSTALLED -> REPORTED

**Files checked**: golang/node_agent/node_agent_server/heartbeat.go
```

If annotations are malformed, strict mode blocks with exit 2:

```
## Awareness hook: BLOCKED

### Annotation findings (1 errors)

✗ [ERROR] golang/node_agent/node_agent_server/heartbeat.go:42: //globular:state_transition must
  have format 'FROM -> TO', got: INSTALLED REPORTED
```

---

## PostToolUse hook output

After each edit, `check-edit` scans the file for forbidden fixes and code smells linked to its invariants.

When the file is clean:

```
AWARENESS POST-EDIT CHECK
file: golang/node_agent/node_agent_server/heartbeat.go

PASS
```

When issues are detected:

```
AWARENESS POST-EDIT CHECK
file: golang/cluster_controller/convergence.go

Forbidden fixes nearby:
- create_infra_release_from_heartbeat_only

Code smells to watch:
- raw_artifact_digest_as_desired_hash
- calling Restart() without exponential backoff
```

The hook exits 1 when issues are found — `|| true` in the hook command ensures it never blocks the edit. Claude reads the output and takes it into account before the next action.

If the file is not in the graph (no `globular awareness build` run yet):

```
AWARENESS POST-EDIT CHECK
file: golang/node_agent/node_agent_server/heartbeat.go

Warnings:
- no graph node for this file — run 'globular awareness build' to index it
```

---

## Relation to `awareness audit`

| Command | Hook type | When | Blocks |
|---------|-----------|------|--------|
| `awareness hook` | PreToolUse | Before each edit | Default: no. Strict mode: yes (high-risk only) |
| `awareness check-edit` | PostToolUse | After each edit | No — always exits 0 via `\|\| true` in hook |
| `awareness audit` | CI / pre-commit | All source files | Yes (exit 1 on ERROR) |
| `awareness pr-report` | CI pull request | Changed files | Yes (exit 1 on ERROR) |

---

## Recommended CLAUDE.md rule

Add this to your `CLAUDE.md` so every AI session is aware of the hook:

```markdown
## Awareness hook

Before editing any file under `golang/`, run:

```bash
globular awareness hook --file <file> --task "<task>"
```

Read the output carefully. If the hook reports an INVARIANT or FORBIDDEN FIX,
respect it — do not work around it without understanding the architectural
constraint it encodes.
```
