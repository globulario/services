# MCP Setup — Connecting AI to Your Cluster

This page explains how to configure the MCP (Model Context Protocol) server so that AI assistants like Claude Code can help manage, diagnose, and configure your Globular cluster. Once connected, Claude can inspect cluster health, read service logs, query metrics, manage DNS records, review workflows, and assist with troubleshooting — all through structured, audited tools.

## What MCP Does

MCP is the structured interface between AI and your cluster. Instead of AI parsing logs or guessing at CLI commands, MCP provides **122+ typed tools** that return structured data:

```
Claude Code ←→ MCP Server ←→ Globular Services
                  │
                  ├── Cluster health, node status, desired state
                  ├── Doctor reports, drift detection, findings
                  ├── Service logs, certificate status
                  ├── Workflow runs, diagnostics
                  ├── Prometheus metrics, alerts
                  ├── Backup status, recovery posture
                  ├── RBAC permissions, role bindings
                  ├── DNS records, domain management
                  ├── AI memory (persistent knowledge)
                  ├── etcd key inspection
                  └── gRPC service introspection
```

Every tool invocation is audited. The MCP server enforces read-only access by default and requires explicit opt-in for write operations.

## Prerequisites

- A running Globular cluster (single node or multi-node)
- The MCP server package installed and running (`globular-mcp` systemd unit)
- [Claude Code](https://claude.ai/code) installed on your development machine
- Network access from your machine to the MCP server port (10260)

## Step 1: Verify MCP Server is Running

```bash
# Check the MCP service
sudo systemctl status globular-mcp

# Check the health endpoint
curl -sk https://<node-ip>:10260/health
# Expected: {"status":"ok","tools":122,"read_only":false}
```

If the MCP server is not installed, deploy it:

```bash
globular services desired set mcp 0.0.1
```

The MCP server runs on port **10260** (HTTPS with the cluster's internal TLS certificate).

## Step 2: Configure Claude Code

Claude Code discovers MCP servers through a `.mcp.json` configuration file. You can set this up globally (for all projects) or per-project.

### Global Setup (Recommended)

This connects Claude Code to your cluster from any project:

```bash
# Create or edit ~/.claude/.mcp.json
cat > ~/.claude/.mcp.json << 'EOF'
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://<your-node-ip>:10260/mcp"
    }
  }
}
EOF
```

Replace `<your-node-ip>` with:
- Your node's IP (e.g., `10.0.0.63`) if working from the local network
- Your VIP address (e.g., `10.0.0.100`) if you have keepalived configured
- Your public domain (e.g., `globular.io`) if MCP is exposed externally (not recommended)

### Per-Project Setup

To connect only when working in a specific project directory:

```bash
# Create .mcp.json in your project root
cat > /path/to/your/project/.mcp.json << 'EOF'
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://<your-node-ip>:10260/mcp"
    }
  }
}
EOF
```

### TLS Certificate Trust

The MCP server uses the cluster's internal CA certificate. Claude Code needs to trust it:

```bash
# Add to ~/.claude/settings.json
cat > ~/.claude/settings.json << 'EOF'
{
  "env": {
    "NODE_EXTRA_CA_CERTS": "/var/lib/globular/pki/ca.crt"
  }
}
EOF
```

If your development machine is not a cluster node, copy the CA certificate:

```bash
# Copy CA from a cluster node to your dev machine
scp <cluster-node>:/var/lib/globular/pki/ca.crt ~/.globular-ca.crt

# Update settings.json
cat > ~/.claude/settings.json << 'EOF'
{
  "env": {
    "NODE_EXTRA_CA_CERTS": "/home/<your-user>/.globular-ca.crt"
  }
}
EOF
```

### Alternative: HTTP (Non-TLS)

If you're on the same machine as the MCP server and want to skip TLS:

```json
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "http://<node-ip>:10260/mcp"
    }
  }
}
```

This only works if the MCP server is configured with `"http_use_tls": false`. The default is TLS enabled.

## Step 3: Verify the Connection

Start (or restart) Claude Code. The MCP tools should appear as deferred tools in the conversation:

```
You should see tools like:
  mcp__globular__cluster_get_health
  mcp__globular__cluster_list_nodes
  mcp__globular__cluster_get_operational_snapshot
  mcp__globular__memory_store
  mcp__globular__nodeagent_get_service_logs
  ... (122+ total)
```

Test by asking Claude: **"What's the cluster health?"**

Claude will call `mcp__globular__cluster_get_health` and show you the result.

## Step 4: Add CLAUDE.md to Your Project

For AI to understand your cluster's architecture and rules, add a `CLAUDE.md` file to your project root. The Globular services repository already has one — use it as a reference or copy it:

```bash
# If your project is the services repo, CLAUDE.md is already there
# For other projects, create one with at least:
cat > CLAUDE.md << 'EOF'
# CLAUDE.md

This project uses a Globular cluster. The MCP server provides diagnostic tools.

## Cluster Info
- Domain: globular.internal
- Controller: localhost:12000
- VIP: 10.0.0.100

## Rules
- etcd is the single source of truth
- No environment variables for configuration
- No hardcoded addresses — resolve from etcd
- All state changes through workflows
EOF
```

## What Claude Can Do With MCP

### Cluster Diagnostics

Ask Claude to check your cluster:
- "What's the cluster health?"
- "Show me any failed workflows"
- "Are there any doctor findings?"
- "What services are drifted?"
- "Check the backup status"

### Service Debugging

Ask Claude to investigate problems:
- "Show me the logs for the authentication service"
- "Why did the last workflow for postgresql fail?"
- "Check if the certificates are expiring"
- "What's the error rate for the RBAC service?"

### DNS and Domain Management

Ask Claude to manage DNS:
- "Set an A record for app.globular.io pointing to 96.20.133.54"
- "What DNS records exist for globular.io?"
- "Check the domain certificate status"

### Configuration Inspection

Ask Claude to review settings:
- "What's the etcd key for the authentication service config?"
- "Show me the RBAC bindings for user admin"
- "What ports is each service using?"

### AI Memory

Claude can remember knowledge across sessions:
- "Remember that the DNS zones need re-registering after restart"
- "What did we fix in the last debugging session?"
- "Save this debugging finding for next time"

## MCP Server Configuration

### Config File Location

The MCP server reads its config from `/var/lib/globular/mcp/config.json`.

### Tool Groups

Each category of tools can be enabled or disabled independently:

| Group | Default | Tools | What Claude Can Do |
|-------|---------|-------|-------------------|
| `cluster` | Enabled | 6 | Cluster health, nodes, desired state, convergence |
| `doctor` | Enabled | 3 | Health analysis, drift reports, finding explanations |
| `nodeagent` | Enabled | 4 | Node inventory, installed packages, service logs |
| `repository` | Enabled | 4 | Artifact catalog, search, manifests |
| `backup` | Enabled | 12 | Backup jobs, validation, retention, recovery status |
| `rbac` | Enabled | 8 | Permission checks, role bindings |
| `resource` | Enabled | 3 | Account/group/org identity |
| `composed` | Enabled | 5 | Aggregated diagnostic views |
| `monitoring` | Enabled | 6 | Prometheus metrics, alerts, rules |
| `workflow` | Enabled | 3 | Workflow runs, diagnostics |
| `memory` | Enabled | 10 | AI memory store/query/sessions |
| `ai_executor` | Enabled | 4 | AI executor status, peer collaboration |
| `etcd` | Enabled | 2 | etcd key read/write |
| `proto` | Enabled | 4 | gRPC reflection, service inspection |
| `cli` | Enabled | 1 | CLI validation, execution |
| `title` | Enabled | 3 | Search index stats, rebuild |
| `frontend` | Enabled | 2 | gRPC service map, web probes |
| `http_diag` | Enabled | 1 | HTTP endpoint diagnostics |
| `browser` | Enabled | 3 | Chrome DevTools bridge |
| `skills` | Enabled | 3 | Operational skill playbooks |
| `governor` | Enabled | 2 | Command validation, approval gates |
| `file` | **Disabled** | 10 | File inspection (requires allowlist) |
| `persistence` | **Disabled** | 5 | Database queries (requires allowlist) |
| `storage` | **Disabled** | 5 | Key-value queries (requires allowlist) |

### Enabling Additional Tool Groups

To enable file inspection:

```json
{
  "tool_groups": { "file": true },
  "file_allowed_roots": [
    "/var/lib/globular/webroot",
    "/var/lib/globular/data/files"
  ]
}
```

To enable database queries:

```json
{
  "tool_groups": { "persistence": true },
  "persistence_allowed_connections": ["local_resource"],
  "persistence_allowed_databases": ["local_resource"],
  "persistence_allowed_collections": ["accounts", "roles"]
}
```

### Read-Only vs Read-Write

By default, MCP is **read-only** (`"read_only": true`). This means Claude can inspect but not modify cluster state.

To enable write operations (CLI execution, etcd writes, package operations):

```json
{
  "read_only": false
}
```

**Caution**: With `read_only: false`, Claude can execute CLI commands and modify etcd keys. The governor system provides validation and approval gates for dangerous operations, but this increases the scope of what AI can do.

### Safety Controls

| Control | Purpose | Config Key |
|---------|---------|-----------|
| Read-only mode | Prevent all writes | `read_only` |
| Tool groups | Enable/disable categories | `tool_groups` |
| File allowlists | Restrict file access paths | `file_allowed_roots` |
| DB allowlists | Restrict database access | `persistence_allowed_*` |
| Max response size | Prevent huge responses | `max_response_size` (default 1MB) |
| Max results | Limit query results | `max_result_count` (default 100) |
| Concurrency limit | Prevent overload | `concurrency_limit` (default 10) |
| Audit logging | Record all tool calls | `audit_log` (default true) |
| Sensitive field redaction | Hide passwords/tokens in logs | Automatic |

### Audit Logging

Every tool invocation is logged with:
- Timestamp, tool name, tool group
- Arguments (with sensitive fields redacted)
- Duration, success/failure
- Error class (if failed)

Logs go to stderr by default, or to a file if `audit_log_path` is set.

## Multi-Node Access

The MCP server runs on the node where it's deployed. For multi-node clusters, you can either:

**Option A: Connect to a specific node**
```json
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://10.0.0.63:10260/mcp"
    }
  }
}
```

**Option B: Connect through the VIP** (if MCP is on a gateway node with keepalived)
```json
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://10.0.0.100:10260/mcp"
    }
  }
}
```

Option B provides automatic failover — if the active gateway goes down, keepalived moves the VIP and MCP stays reachable.

## Exposing MCP Externally

To allow AI access from outside your network (e.g., a developer's laptop not on the LAN):

**Option 1: VPN** (recommended)
Connect to your network via VPN, then use the internal IP.

**Option 2: Port forward through gateway**
Add port 10260 to the router's DMZ or port forwarding. The MCP server uses TLS with the cluster CA, so connections are encrypted. However, this exposes an admin interface to the internet — use with caution.

**Option 3: SSH tunnel** (secure, no router changes)
```bash
# On your dev machine, create an SSH tunnel to the cluster
ssh -L 10260:localhost:10260 user@<cluster-public-ip>

# Then configure Claude Code to use localhost
# .mcp.json:
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://localhost:10260/mcp"
    }
  }
}
```

## Troubleshooting

### Claude doesn't see MCP tools

1. Check `.mcp.json` exists in `~/.claude/` or project root
2. Restart Claude Code (close and reopen terminal)
3. Verify MCP health: `curl -sk https://<ip>:10260/health`

### "Connection refused"

- MCP service not running: `sudo systemctl start globular-mcp`
- Wrong port: MCP default is 10260 (check `http_listen_addr` in config)
- Firewall blocking: `sudo ufw allow from <your-ip> to any port 10260`

### "Certificate error" or TLS failure

- Missing CA trust: Set `NODE_EXTRA_CA_CERTS` in `~/.claude/settings.json`
- Wrong CA file: Verify with `openssl x509 -in /path/to/ca.crt -noout -subject`
- Use HTTP instead of HTTPS if on localhost (set `http_use_tls: false` in MCP config)

### Tools return errors

- "Unavailable": Target service is down. Check `globular cluster health`
- "PermissionDenied": MCP service account lacks permissions. Check RBAC bindings.
- "Timeout": Increase `default_timeout` in MCP config or the specific tool's timeout

### "read_only" prevents needed operation

If Claude needs to make changes (DNS records, etcd keys, CLI commands):
```json
{
  "read_only": false
}
```
Restart the MCP service after changing config:
```bash
sudo systemctl restart globular-mcp
```

## Complete Setup Example

Here's a complete setup for a developer on the local network:

```bash
# 1. Create MCP connection config
mkdir -p ~/.claude
cat > ~/.claude/.mcp.json << 'EOF'
{
  "mcpServers": {
    "globular": {
      "type": "http",
      "url": "https://10.0.0.100:10260/mcp"
    }
  }
}
EOF

# 2. Trust the cluster CA
cat > ~/.claude/settings.json << 'EOF'
{
  "env": {
    "NODE_EXTRA_CA_CERTS": "/var/lib/globular/pki/ca.crt"
  }
}
EOF

# 3. If not on a cluster node, copy the CA cert
scp globule-ryzen:/var/lib/globular/pki/ca.crt ~/.globular-ca.crt
# Then update settings.json to point to ~/.globular-ca.crt

# 4. Start Claude Code
claude

# 5. Test: ask "What's the cluster health?"
```

After this, Claude can help you operate your entire Globular cluster — inspect services, diagnose failures, manage DNS, check backups, review workflows, and remember context across sessions.

## What's Next

- [AI Overview](../ai/ai-overview.md) — How the AI layer works
- [AI Rules](../ai/ai-rules.md) — What AI can and cannot do
- [AI Operator Guide](../ai/ai-operator-guide.md) — Monitor and control AI behavior
- [Observability](operators/observability.md) — Prometheus, logs, and monitoring
