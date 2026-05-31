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

---

## Discovering tools

### Via CLI

List all tools registered in the running MCP service:

```bash
globular mcp tools
```

Filter to awareness tools only:

```bash
globular mcp tools --group awareness
```

Sample output:

```
TOOL                              DESCRIPTION
awareness.agent_context           Invariants + forbidden fixes for a coding task
awareness.approve_proposal        Approve a validated proposal (sets APPROVED; does n...
awareness.did_we_fix              Look up the fix-ledger for a task or symptom
awareness.fix_status              Retrieve a fix case by ID or keyword
awareness.impact_file             Downstream graph impact for a specific source file
awareness.package_context         Package architectural context from the awareness gr...
awareness.pattern_status          All fix cases that match a keyword pattern
awareness.preflight               Full awareness preflight — primary entry point for ...
awareness.propose_from_incident   Generate a draft proposal from an incident (DRAFT s...
awareness.runtime_snapshot        Read-only snapshot of live cluster runtime state
awareness.validate_package        Validate a package against awareness graph rules
awareness.validate_proposal       Validate a proposal file against all 12 rules

12 tool(s) in group "awareness".
```

The command connects to the MCP service resolved from etcd, or falls back to `https://globular.internal:10260/mcp`. Use `--url` to override:

```bash
globular mcp tools --group awareness --url https://10.0.0.63:10260/mcp
```

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
