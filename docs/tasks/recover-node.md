# Task: Recover a Node

## Overview

Restore a failed, unreachable, or degraded node to healthy operation.

Globular has four recovery modes, ordered from least to most disruptive. Always try the lightest option first.

| Mode | When | Command |
|------|------|---------|
| **Auto-heal** | Services crashed, node agent alive | `globular doctor heal` |
| **Targeted repair** | Specific artifacts corrupt, OS intact | `globular node repair` |
| **Replace node** | Hardware dead, join a new machine | `globular cluster nodes remove` + rejoin |
| **Full-reseed** | OS untrustworthy, disk corrupt, need clean rebuild | `globular node recover full-reseed` |

This task covers Scenarios A–C (auto-heal through replacement). For the full-reseed workflow — complete wipe and rebuild from a captured snapshot — see the dedicated guide: **[Node Full-Reseed Recovery](../operators/node-recovery.md)**.

## Prerequisites

- Access to the Globular CLI from a healthy node
- Admin permissions
- For hard recovery or full-reseed: physical or SSH access to the affected node

---

## Scenario A: Node is reachable, services are down

### Step 1: Diagnose

```bash
globular cluster health
# Identify the unhealthy node

globular cluster get-doctor-report
# Get detailed findings for the node
```

### Step 2: Check the Node Agent

```bash
# Try to reach the node agent
globular node logs --node <node>:11000 --unit globular-node-agent --lines 50
```

If the node agent doesn't respond:
```bash
ssh <node>
sudo systemctl restart globular-node-agent
sudo systemctl status globular-node-agent
```

### Step 3: Auto-heal or manually restart

```bash
# Option 1: Let the Cluster Doctor auto-heal
globular doctor set-mode enforce
# Wait for the healer to restart stopped services

# Option 2: Manually restart a specific service
globular node control --node <node>:11000 --unit <service> --action restart
```

### Step 4: Verify

```bash
globular cluster health
# Node returns to healthy

globular cluster get-doctor-report
# No critical findings
```

---

## Scenario B: Node is unreachable (power loss, network issue)

### Step 1: Assess

```bash
globular cluster health
# Shows: <node> unreachable, last seen X minutes ago

# Can you reach the machine?
ping <node-ip>
ssh <node-ip>
```

### Step 2: Restore access

If reachable via SSH:
```bash
sudo systemctl status globular-node-agent
sudo journalctl -u globular-node-agent --no-pager -n 50
sudo systemctl restart globular-node-agent
```

If not reachable: check power, network cables, switch ports, or hypervisor/cloud console.

### Step 3: Wait for automatic convergence

Once the node agent restarts, it:
1. Sends a heartbeat to the controller
2. The controller checks for drift
3. Workflows are dispatched for misaligned services
4. The node converges to desired state automatically

```bash
# Monitor recovery
watch -n 10 globular cluster health
```

---

## Scenario C: Node hardware is permanently lost

Use this when the physical machine is dead and you are replacing it.

### Step 1: Remove the failed node

```bash
globular cluster nodes remove <node-id>
```

This removes the node from the controller registry, etcd membership, and MinIO pool.

> **Before removing a storage node**: confirm at least 2 other nodes have the `storage` profile. Dropping below 3 storage nodes breaks MinIO erasure coding and ScyllaDB replication.

### Step 2: Provision replacement hardware

On the new machine:
```bash
sudo ./install.sh
sudo systemctl start globular-node-agent
```

### Step 3: Join the new node

```bash
# Create a join token
globular cluster token create --expires 2h

# Join
globular cluster join \
  --node <new-node-ip>:11000 \
  --controller globular.internal:12000 \
  --join-token <token>

# Approve with the same profiles as the failed node
globular cluster requests approve <request-id> \
  --profile <profile1> \
  --profile <profile2>
```

### Step 4: Wait for convergence

```bash
globular cluster health
# etcd replicates, MinIO rebuilds erasure shards,
# all profile services install via workflows
```

### Step 5: Verify

```bash
globular cluster health
# Shows N/N nodes healthy

globular cluster get-doctor-report
# HEALTHY, no critical findings
```

---

## Scenario D: Node needs a clean rebuild (full-reseed)

Use this when the node's OS or disk cannot be trusted — hardware failure that corrupted the filesystem, disk encryption key loss, security incident, or any situation where you cannot rely on what is installed.

This triggers the `node.recover.full_reseed` workflow, which:
1. Captures a snapshot of the node's current installed artifact inventory
2. Fences the reconciler (no parallel state changes while recovery is in progress)
3. Pauses and waits for you to physically wipe and reprovision the node
4. Reinstalls every artifact in deterministic bootstrap order (etcd first, then auth/rbac/discovery, then applications)
5. Verifies each artifact's `build_id` and checksums
6. Unfences the reconciler

```bash
# Dry-run first — shows the planned install order
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "disk corruption, I/O errors on /var" \
  --dry-run

# Dispatch the workflow
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "disk corruption, I/O errors on /var"

# Monitor progress (workflow pauses at AWAIT_REPROVISION)
globular node recover status --node-id <node-id>

# Wipe the node and install a fresh OS (your action)

# Acknowledge reprovision — this unblocks the workflow
globular node recover ack-reprovision \
  --node-id <node-id> \
  --workflow-id <workflow-id> \
  --note "Reinstalled Ubuntu 22.04"

# Watch reseed complete
watch -n 5 globular node recover status --node-id <node-id>
```

**Full guide with all flags, safety rules, failure handling, and FAQs:**
→ [Node Full-Reseed Recovery](../operators/node-recovery.md)

---

## Verification (all scenarios)

```bash
# Node is healthy
globular cluster health

# No outstanding issues
globular cluster get-doctor-report

# All services converged
globular cluster get-node-full-status --node-id <node-id>
```

---

## Troubleshooting

### Node keeps going unhealthy after recovery

Check for recurring resource exhaustion:
```bash
ssh <node> free -h          # Memory
ssh <node> df -h            # Disk (especially /var/lib/globular)
ssh <node> dmesg | tail -50 # Kernel messages — OOM killer, hardware errors
```

### etcd quorum lost after node removal

If you removed an etcd member and the cluster lost quorum (fewer than 2 of 3 members healthy):
```bash
# Add a new node with core profile as quickly as possible
globular cluster token create --expires 2h
# ... join and approve with core profile

# If quorum cannot be restored, restore etcd from backup
globular backup restore <backup-id> --provider etcd
# See: docs/operators/backup-and-restore.md
```

### Services installing slowly after replacement

MinIO may be rebuilding erasure shards. Monitor:
```bash
globular cluster health
# Check MinIO status

globular workflow list
# Check FETCH phase timing on in-progress workflows
```

### Node agent won't start after reboot

```bash
ssh <node>
sudo journalctl -u globular-node-agent --no-pager -n 100

# Common causes:
# - Port 11000 in use:     ss -tlnp | grep 11000
# - Certificate expired:   ls -la /var/lib/globular/pki/issued/
# - etcd unreachable:      check network to etcd peers
```

### Full-reseed workflow stuck at AWAIT_REPROVISION

The workflow waits up to 24 hours. If you need to check the status:
```bash
globular node recover status --node-id <node-id>
# phase: AWAIT_REPROVISION means it is waiting for your ACK

# Once the node OS is reinstalled and the node agent has started:
globular node recover ack-reprovision \
  --node-id <node-id> \
  --workflow-id <workflow-id>
```

### Full-reseed failed mid-reseed

If the workflow reaches FAILED after the destructive boundary:
```bash
globular node recover status --node-id <node-id>
# Shows: phase=FAILED, fenced=true
# Check which artifacts failed in the results list

# Restart recovery using the same snapshot (skips already-verified artifacts)
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "resume after partial failure" \
  --snapshot-id <original-snapshot-id>
```

See [Node Full-Reseed Recovery — what happens if the workflow fails](../operators/node-recovery.md#what-happens-if-the-workflow-fails) for full details.
