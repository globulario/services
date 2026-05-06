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

The hook **never blocks** (exits 0 always). It is purely informational — a
pre-flight context injection, not a gate.

---

## Configuration

Add this to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "globular awareness hook --file \"$CLAUDE_TOOL_INPUT_PATH\" --task \"$CLAUDE_TASK\""
          }
        ]
      }
    ]
  }
}
```

Or for a project-scoped hook, add to `.claude/settings.json` at the repo root.

---

## Manual invocation

```bash
# Single file
globular awareness hook \
  --file golang/node_agent/node_agent_server/heartbeat.go \
  --task "fix heartbeat interval logic"

# Multiple files
globular awareness hook \
  --file golang/cluster_controller/cluster_controller_server/release_hash.go \
  --file golang/node_agent/node_agent_server/installed_services.go \
  --task "update hash computation"
```

---

## Example output

```
## Awareness hook: PASS

**Task**: fix heartbeat interval logic

### Architecture constraints for edited files

- INVARIANT: infra.heartbeat_not_desired_authority
- RISK: infra.heartbeat_sets_desired_state
- STATE TRANSITION: INSTALLED -> REPORTED

**Files checked**: golang/node_agent/node_agent_server/heartbeat.go
```

If annotations are malformed:

```
## Awareness hook: BLOCKED

### Annotation findings (1 errors)

✗ [ERROR] golang/node_agent/node_agent_server/heartbeat.go:42: //globular:state_transition must
  have format 'FROM -> TO', got: INSTALLED REPORTED
```

---

## Relation to `awareness audit`

| Command | When | Scope | Blocks |
|---------|------|-------|--------|
| `awareness hook` | Before each edit | Files being edited | Never (informational) |
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
