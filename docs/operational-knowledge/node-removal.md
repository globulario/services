# Node Removal

Removing a node from a Globular cluster requires two phases: **cluster-side deregistration** and **local cleanup on the node itself**. Doing only one phase leaves stale state.

---

## Why two phases?

The controller `RemoveNode` RPC deregisters the node from the cluster registry, prunes its xDS endpoints, and evicts it from the MinIO pool. But it does **not**:

- Remove the node's etcd membership (quorum is still counted)
- Decommission ScyllaDB (data stays under-replicated)
- Stop the node-agent (it will re-register itself within seconds)
- Wipe `/var/lib/globular` (certs, state, packages remain)

The clean script runs **on the node** and handles all of that before the local disk is wiped.

---

## Full clean removal (recommended)

### Step 1 — Run the clean script on the node

```bash
# SSH to the target node, then:
curl -sfL https://globular.internal:8443/clean -k | sudo bash -s -- --force
```

This script (Phase 0 before local wipe):
- Calls `globular cluster nodes remove <node_id> --force --drain=false` via the CLI
- Runs `nodetool decommission` to stream ScyllaDB data to peers
- Removes the node's etcd member so quorum is not broken

Then locally (Phases 1–5):
- Stops all `globular-*` systemd units
- Removes unit files and drops state from `/var/lib/globular`, `/var/lib/scylla`, `/var/lib/etcd`
- Removes MinIO data from `/mnt/data/minio`
- Removes ScyllaDB packages (so node-agent owns reinstall on rejoin)
- Cleans PKI from system trust store
- Removes the `globular` user/group

After the script completes, the node has no trace of Globular and is ready for a fresh Day-1 join.

### Step 2 — Verify via MCP

```
cluster_list_nodes        → node should be gone
cluster_get_health        → remaining nodes should be healthy
```

---

## Forced removal (node is dead / unreachable)

When the node is powered off or unreachable and you cannot SSH to it:

**Via MCP tool:**
```
cluster_remove_node { "node_id": "<uuid>", "force": true, "drain": false }
```

**Via CLI:**
```bash
globular cluster nodes remove <node_id> --force --drain=false
```

This removes the node from the controller registry only. The etcd member and ScyllaDB token will remain as orphans until the node is cleaned and rejoined. Before rejoining, always run the clean script with `--force`.

---

## CLI reference

```
globular cluster nodes remove <node_id> [flags]

Flags:
  --force          Force removal even if node is unreachable (default: false)
  --drain          Stop services gracefully before removal (default: true)
                   Set --drain=false only when the node is already dead.
```

---

## MCP tool reference

**`cluster_remove_node`**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `node_id` | string | yes | — | Node UUID or hostname |
| `force` | bool | no | false | Force removal even if unreachable |
| `drain` | bool | no | true | Stop services before removal |

Returns: `{ "operation_id": "...", "message": "..." }`

---

## Why the node re-registers after `RemoveNode`

If the node-agent is still running when `RemoveNode` is called, it reconnects to the controller on its next heartbeat (every ~5 seconds) and re-registers itself. This is by design — the agent is resilient. To actually remove the node you must stop the agent first, which is what the clean script does.

---

## `remove_node` workflow (MCP)

The `globular_cli.workflow` MCP tool exposes a `remove_node` workflow that walks through the correct sequence:

1. `cluster_list_nodes` — confirm node ID
2. Run clean script on the node (SSH required)
3. `cluster_remove_node` with `force=true, drain=false`
4. `cluster_list_nodes` — verify node is gone
5. `cluster_get_health` — confirm remaining nodes healthy

---

## Related

- Clean script source: `Globular/internal/gateway/handlers/cluster/clean-node.sh`
- Canonical script copy: `services/scripts/clean-node.sh`
- CLI implementation: `golang/globularcli/cluster_cmds.go` — `nodeRemoveCmd`
- MCP tool: `golang/mcp/tools_cluster.go` — `cluster_remove_node`
- Proto RPC: `proto/cluster_controller.proto` — `RemoveNode` (line ~501)
