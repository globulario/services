# Globular MCP Server

An AI-operator interface for Globular clusters. Exposes read-only diagnostic tools over the Model Context Protocol (MCP) for use with Claude Code and other MCP-compatible AI assistants.

## Purpose

Replaces fragile log parsing, CLI scraping, and REST endpoint guessing with structured, safe, operator-grade tools that AI assistants can use to inspect and diagnose Globular cluster state.

## Phase 1 Scope

Phase 1 is **read-only only**. No mutations, no state changes, no destructive operations.

## Recent updates (April 2026)
- Transport defaults to HTTPS on port 10250 with service certificates (`/var/lib/globular/pki/issued/services/service.crt/key`). Ensure your config has `http_use_tls: true` and advertise host 0.0.0.0.
- Health endpoint: `https://<node>:10250/health` returns `{status:"ok", tools:<count>}` for readiness checks.
- Tool count expanded to ~129; command/runtime checks relaxed for command-only packages to reduce false failures in diagnostics.
- Prometheus: if you scrape MCP, use the TLS endpoint; disable plain HTTP unless explicitly required.

## Security Model

- **Read-only by default** (`read_only: true`)
- **Tool groups** can be independently enabled/disabled
- **File/persistence/storage** disabled by default — require explicit allowlists
- **Automatic redaction** of sensitive fields (passwords, tokens, keys)
- **Path traversal prevention** for file tools
- **Connection/database/collection allowlists** for persistence tools
- **Key prefix allowlists** for storage tools
- **Structured audit logging** of all tool invocations
- **Result size limits** prevent unbounded responses
- **SA token auth** — uses the local service account, not broad admin

## Tool Groups (65 tools)

| Group | Default | Tools | Description |
|-------|---------|-------|-------------|
| cluster | enabled | 6 | Cluster health, nodes, plans, desired state |
| doctor | enabled | 3 | Invariant checks, drift detection, finding explanations |
| nodeagent | enabled | 4 | Node inventory, installed packages, plan status |
| repository | enabled | 4 | Artifact catalog, search, manifests, versions |
| backup | enabled | 12 | Jobs, backups, validation, retention, recovery |
| rbac | enabled | 8 | Permission validation, role bindings, resource permissions |
| resource | enabled | 3 | Account/group/org identity context |
| composed | enabled | 5 | High-value aggregated diagnostic views |
| file | **disabled** | 6+4 | File inspection + deployment diagnostics (requires allowlist) |
| persistence | **disabled** | 5 | Database queries (requires allowlist) |
| storage | **disabled** | 5 | Key-value inspection (requires allowlist) |
| auth | deferred | 0 | Token validation (phase 2) |
| dns | deferred | 0 | DNS inspection (phase 2) |

## Configuration

Config is loaded from (in order):
1. `$GLOBULAR_MCP_CONFIG` environment variable
2. `/var/lib/globular/mcp/config.json`
3. `~/.config/globular/mcp.json`
4. Built-in defaults

See `config.example.json` for the full schema.

### Enabling file tools

```json
{
  "tool_groups": { "file": true },
  "file_allowed_roots": [
    "/var/lib/globular/webroot",
    "/var/lib/globular/config"
  ]
}
```

### Enabling persistence tools

```json
{
  "tool_groups": { "persistence": true },
  "persistence_allowed_connections": ["local_resource"],
  "persistence_allowed_databases": ["local_resource"],
  "persistence_allowed_collections": ["accounts", "roles", "groups"]
}
```

### Enabling storage tools

```json
{
  "tool_groups": { "storage": true },
  "storage_allowed_connections": ["default"],
  "storage_allowed_key_prefixes": ["/globular/nodes/", "/globular/services/"]
}
```

## Claude Code Integration

Add to your project's `.claude/settings.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "globular": {
      "command": "/usr/lib/globular/bin/globular-mcp-server",
      "env": {
        "GLOBULAR_MCP_CONFIG": "/var/lib/globular/mcp/config.json"
      }
    }
  }
}
```

## Building

```bash
cd golang
go build -o globular-mcp-server ./mcp/...
```

## Service Identity & RBAC

The MCP server authenticates using the local SA token (`security.GetLocalToken`). It requires read permissions on:

| Service | Required Permission |
|---------|-------------------|
| ClusterController | cluster-read |
| ClusterDoctor | cluster-read |
| NodeAgent | node-read |
| Repository | repository-read |
| BackupManager | backup-read |
| RBAC | rbac-read |
| Resource | resource-read |
| File | file-read (if enabled) |
| Persistence | persistence-read (if enabled) |
| Storage | storage-read (if enabled) |

Recommend creating a dedicated `mcp-operator` role with least-privilege read access rather than using the broad `sa` identity in production.

## Deployment

The MCP server is an **optional admin/ops component**:
- Not required for Day-0 bootstrap
- Not required for core cluster operation
- Recommended for admin/ops profiles
- Can run standalone or as a Globular package

## Phase 2 Roadmap

- Auth diagnostic tools (token validation)
- DNS inspection tools
- Preview/planning tools (profile changes, upgrade plans)
- Guarded operational tools (with confirmation gates)
- HTTP/SSE transport for remote access
