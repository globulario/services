# Preflight Audit Log

The awareness system writes a durable record to the graph database every time `preflight` is run with `--write-audit`. This gives you an audit trail of what invariants, forbidden fixes, and code smells the agent was told about before each edit session.

## How it works

When `preflight.Run` is called with `WriteAudit: true` (or the CLI flag `--write-audit`), it appends a row to the `preflight_audits` table in `graph.db` after the report is assembled. Each row captures:

| Field | Description |
|-------|-------------|
| `id` | Auto-generated UUID |
| `task` | The task description passed to preflight |
| `git_sha` | Git SHA at the time of the run (optional) |
| `files` | Files included in the impact analysis |
| `invariants` | Invariant IDs surfaced to the agent |
| `forbidden_fixes` | Forbidden fix names surfaced to the agent |
| `code_smells` | Code smell strings surfaced to the agent |
| `timestamp` | Unix timestamp of the preflight run |

## CLI usage

```bash
# Query all audit records
globular awareness preflight-audit

# Show only records from the last 24 hours
globular awareness preflight-audit --since 24h

# Filter by git SHA
globular awareness preflight-audit --git-sha abc123def

# JSON output
globular awareness preflight-audit --format json

# Write an audit record during preflight
globular awareness preflight --task "fix desired_hash drift" --write-audit --git-sha $(git rev-parse HEAD)
```

## PostToolUse hook

The `.claude/settings.json` in the repo root configures a `PostToolUse` hook that runs `check-edit` after every `Edit` or `Write` tool call:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "globular awareness check-edit --file \"$CLAUDE_TOOL_INPUT_FILE_PATH\" --format agent 2>/dev/null || true"
          }
        ]
      }
    ]
  }
}
```

The hook uses `|| true` so it never blocks an edit — it only surfaces a warning to the agent when forbidden fixes or code smells are detected for the file.

### What the hook does

For each file edited:
1. Looks up the file in the awareness graph
2. Follows `enforces`/`protects` edges to find linked invariants
3. Follows `forbids` edges to find forbidden fixes
4. Queries `CodeSmellsForInvariants` to find anti-patterns linked to those invariants
5. Prints a `CHECK-EDIT ALERT` block if issues are found, or `CHECK-EDIT CLEAR` otherwise

### Exit code

`check-edit` exits with code 1 when `HasIssues=true`. The `|| true` in the hook prevents this from blocking the edit. If you want the hook to block edits with known issues, remove `|| true`.

## Post-commit hook

To automatically write an audit record after every commit:

```bash
# .git/hooks/post-commit (make executable: chmod +x .git/hooks/post-commit)
#!/bin/sh
SHA=$(git rev-parse HEAD)
globular awareness preflight \
  --task "$(git log -1 --pretty=%s)" \
  --write-audit \
  --git-sha "$SHA" \
  --format agent > /dev/null 2>&1 || true
```

This ensures every commit has an associated preflight record in the audit log.

## Schema

```sql
CREATE TABLE IF NOT EXISTS preflight_audits (
    id                   TEXT PRIMARY KEY,
    task                 TEXT NOT NULL DEFAULT '',
    timestamp            INTEGER NOT NULL DEFAULT 0,
    git_sha              TEXT NOT NULL DEFAULT '',
    files_json           TEXT NOT NULL DEFAULT '[]',
    forbidden_fixes_json TEXT NOT NULL DEFAULT '[]',
    invariants_json      TEXT NOT NULL DEFAULT '[]',
    code_smells_json     TEXT NOT NULL DEFAULT '[]',
    created_at           INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_preflight_audits_ts ON preflight_audits(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_preflight_audits_sha ON preflight_audits(git_sha);
```
