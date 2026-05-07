# Globular Awareness MCP Tools

Awareness tools are exposed by the **main Globular MCP service** (`golang/mcp`).
This is the canonical way for AI agents to query the awareness graph.

The standalone `globular awareness mcp-server` command is **deprecated** — use it only for local development or testing when the cluster MCP service is unavailable.

---

## Architecture

```
golang/mcp            ← MCP transport + tool exposure (owner)
  └─ imports ──►  golang/awareness/*  ← awareness business logic (owner)

golang/awareness/mcp  ← DEPRECATED standalone server (dev-only)
```

Dependency direction: `golang/mcp` imports `golang/awareness` packages.
`golang/awareness` packages never import `golang/mcp`.

---

## Accessing awareness tools via the main MCP service

The main Globular MCP service runs on port `10260` and exposes awareness tools under the `awareness.*` namespace when `tool_groups.awareness` is `true` (the default).

### Claude Code (`~/.claude/mcp_config.json`)

```json
{
  "mcpServers": {
    "globular": {
      "url": "https://<node>:10260/mcp",
      "name": "globular"
    }
  }
}
```

Awareness tools will appear automatically in `tools/list` alongside all other Globular MCP tools.

### Disabling awareness tools

Set `tool_groups.awareness = false` in `/var/lib/globular/mcp/config.json` to exclude awareness tools from the main service.

### Awareness graph configuration

Add an `"awareness"` block to `/var/lib/globular/mcp/config.json` to override path auto-detection:

```json
{
  "tool_groups": { "awareness": true },
  "awareness": {
    "db_path":   "/path/to/.globular/awareness/graph.db",
    "repo_path": "/path/to/globulario/services",
    "docs_dir":  "/path/to/docs/awareness",
    "node_id":   "globule-ryzen"
  }
}
```

All fields are optional. When empty, the server auto-detects the repo root via `git rev-parse` and uses the standard `.globular/awareness/graph.db` location. If the graph DB is missing the tools degrade gracefully (return structured warnings instead of erroring).

---

## Tool list

| Tool | Description | Required args |
|------|-------------|---------------|
| `awareness.preflight` | Full architecture preflight — primary entry point | `task` |
| `awareness.agent_context` | Invariants + forbidden fixes for a task | `task` |
| `awareness.impact_file` | Downstream graph impact for a specific file | `file` |
| `awareness.did_we_fix` | Fix-ledger lookup for a task or symptom | `task` |
| `awareness.pattern_status` | All fix cases matching a keyword pattern | `pattern` |
| `awareness.fix_status` | Fix case by ID or keyword | (none required) |
| `awareness.runtime_snapshot` | Read-only live cluster snapshot | (none required) |
| `awareness.validate_package` | Package admission check against graph rules | `path` |
| `awareness.package_context` | Package architectural context from graph | `path` |
| `awareness.propose_from_incident` | Generate a draft proposal (DRAFT status only) | `incident_id` |
| `awareness.validate_proposal` | Validate a proposal file (12 rules) | `file` |
| `awareness.approve_proposal` | Approve a validated proposal (no code change) | `file` |

`awareness.promote_proposal` is **intentionally not exposed** over MCP. Promotion is a CLI-only operation:

```bash
globular awareness promote-proposal proposals/my-proposal.yaml
```

---

## Safety model

| Property | Guarantee |
|----------|-----------|
| **Read-only by default** | `preflight`, `agent_context`, `impact_file`, `did_we_fix`, `pattern_status`, `fix_status`, `runtime_snapshot`, `validate_package`, `package_context` never write to disk or etcd |
| **Propose = DRAFT only** | `propose_from_incident` writes a `.yaml` file under `proposals/` with status `DRAFT`. No graph mutation. |
| **Approve ≠ Promote** | `approve_proposal` sets status to `APPROVED` in the file only. Graph and fix-ledger are NOT updated. |
| **Path traversal blocked** | `output_name` in `propose_from_incident` is hard-rejected if it is an absolute path, contains `/` or `\`, or resolves to `..`. |
| **Graceful degradation** | If the graph DB is missing, all tools return a structured result with a `"no graph DB"` warning instead of erroring. |
| **No promotion gate bypass** | The tool registry is tested on every CI run: `awareness.promote_proposal` must not be registered. |

---

## Standalone awareness MCP server (DEPRECATED)

The `globular awareness mcp-server` command remains available for local development, but is no longer the recommended integration point:

```bash
# Dev-only: start as a separate stdio process
globular awareness mcp-server

# Explicit paths
globular awareness mcp-server \
  --repo    /path/to/globulario/services \
  --db      /path/to/.globular/awareness/graph.db \
  --docs    /path/to/docs/awareness \
  --node-id globule-ryzen
```

The standalone server will be removed in a future release once all clients have migrated to the main Globular MCP service.

---

## Example: Claude preflight call

```json
{
  "jsonrpc": "2.0",
  "id":      1,
  "method":  "tools/call",
  "params": {
    "name": "awareness.preflight",
    "arguments": {
      "task":            "desired_hash mismatch after deploy",
      "files":           ["golang/node_agent/node_agent_server/heartbeat.go"],
      "include_runtime": true,
      "runtime_window":  "10m"
    }
  }
}
```

---

## Recommended CLAUDE.md rule

```markdown
## Mandatory awareness preflight

Before editing any file under `golang/`, `proto/`, or `docs/`:

1. Call `awareness.preflight` with the task description and affected files.
2. Read `classification`, `invariants`, `forbidden_fixes`, and `did_we_fix`.
3. If classification contains `ARCHITECTURE_SENSITIVE` or `CONVERGENCE_RISK`,
   also call `awareness.runtime_snapshot` and `awareness.agent_context`.
```
