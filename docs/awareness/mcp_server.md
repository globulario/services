# Globular Awareness MCP Server

The awareness MCP server exposes the Globular awareness graph as 12 tools over the Model Context Protocol (JSON-RPC 2.0 / stdio). AI agents use it to load architecture context before editing code.

---

## Running the server

```bash
# Start with auto-detected repo root and default graph.db location
globular awareness mcp-server

# Explicit paths
globular awareness mcp-server \
  --repo    /path/to/globulario/services \
  --db      /path/to/.globular/awareness/graph.db \
  --docs    /path/to/docs/awareness \
  --node-id globule-ryzen
```

**Flags**

| Flag | Default | Purpose |
|------|---------|---------|
| `--db` | `<repo>/.globular/awareness/graph.db` | Path to the SQLite graph DB |
| `--repo` | auto-detected via `git rev-parse` | Repo root directory |
| `--docs` | `<repo>/docs/awareness` | docs/awareness directory |
| `--node-id` | (empty) | Optional node ID for runtime bridge labelling |

All flags are optional. The server degrades gracefully if the graph DB is missing.

The server writes a single line to stderr when ready:

```
globular-awareness-mcp: ready (12 tools)
```

All MCP traffic flows on **stdin/stdout** with Content-Length framing (same as Language Server Protocol). The process exits cleanly on `SIGINT` / `SIGTERM` or when the client closes stdin.

---

## Client configuration

### Claude Code (`~/.claude/mcp_config.json`)

```json
{
  "mcpServers": {
    "globular-awareness": {
      "command": "globular",
      "args": ["awareness", "mcp-server"],
      "name":  "globular-awareness"
    }
  }
}
```

### Generic MCP client

```json
{
  "command": "globular",
  "args":    ["awareness", "mcp-server"],
  "name":    "globular-awareness"
}
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
| `awareness.validate_proposal` | Validate a proposal file (12 rules); optional `strict` bool makes missing graph a FAIL | `file` |
| `awareness.approve_proposal` | Approve a validated proposal (no code change) | `file` |

`awareness.promote_proposal` is **intentionally not exposed** over MCP. Promotion (writing to fix-ledger, moving to PROMOTED status) is a CLI-only operation that requires explicit human intent:

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
| **Path traversal blocked** | `output_name` in `propose_from_incident` is hard-rejected (not sanitised) if it is an absolute path, contains `/` or `\`, or resolves to `..`. Only plain filenames are accepted. |
| **Graceful degradation** | If the graph DB is missing or corrupted, all tools return a structured result with a `"no graph DB"` warning instead of erroring. Preflight still returns classification, docs-sourced invariants, and failure modes. |
| **No promotion gate bypass** | The server's tool registry is tested on every CI run: `HasTool("awareness.promote_proposal")` must be `false`. |

---

## Example: Claude preflight call

When Claude opens a Globular file, AGENTS.md instructs it to call `awareness.preflight` first:

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

The response is a JSON document matching the preflight schema:

```json
{
  "task":            "desired_hash mismatch after deploy",
  "classification":  ["STATE_MISMATCH", "CONVERGENCE_RISK"],
  "invariants":      ["desired_hash_immutable", "state_layer_order"],
  "failure_modes":   ["hash_drift_on_partial_rollout"],
  "forbidden_fixes": ["Do not patch desired_hash at the node-agent layer"],
  "did_we_fix": {
    "status":           "PARTIAL",
    "matched_patterns": ["desired_hash"],
    "fix_cases":        ["FC-0042"],
    "remaining_gaps":   ["hash propagation on restart"]
  },
  "runtime": {
    "included":       true,
    "state_deltas":   [{"node": "globule-nuc", "field": "desired_hash", "desired": "abc", "installed": "def"}],
    "warnings":       []
  },
  "agent_instruction": "Walk layers: Repository → Desired → Installed → Runtime. Do not patch node-agent hashes directly."
}
```

---

## Recommended AGENTS.md rule

Add this block to your project's `AGENTS.md` (or `CLAUDE.md`) so every AI agent loads context before editing Globular code:

```markdown
## Mandatory awareness preflight

Before editing any file under `golang/`, `proto/`, or `docs/`:

1. Call `awareness.preflight` with the task description and affected files.
2. Read `classification`, `invariants`, `forbidden_fixes`, and `did_we_fix`.
3. If classification contains `ARCHITECTURE_SENSITIVE` or `CONVERGENCE_RISK`,
   also call `awareness.runtime_snapshot` and `awareness.agent_context`.
4. End your response with:

   **Awareness used:** preflight · [matched aliases] · did_we_fix=[STATUS]
```

---

## Protocol details

The server implements MCP 2025-03-26 over stdin/stdout.

Supported methods:

| Method | Notes |
|--------|-------|
| `initialize` | Returns `protocolVersion: "2025-03-26"` and server capabilities |
| `initialized` / `notifications/initialized` | Acknowledged, no response |
| `tools/list` | Returns all 12 tool definitions with JSON Schema |
| `tools/call` | Dispatches to the named tool handler |
| `resources/list` | Returns empty list (no resources exposed) |
| `prompts/list` | Returns empty list (no prompts exposed) |
| `ping` | Returns `{}` |

Transport supports both framing modes:

- **Content-Length framing** (preferred): `Content-Length: N\r\n\r\n<N bytes of JSON>`
- **Newline-delimited JSON** (fallback): raw `{...}\n` lines
