# Task: Recover a Node

## Overview

Restore a failed, unreachable, or degraded node to healthy operation. This covers both soft recovery (restart services) and hard recovery (replace the node entirely).

Use this when:
- A node shows `unreachable` or `unhealthy` in cluster health
- Services on a node have stopped and aren't auto-recovering
- A node had hardware failure and needs replacement

## Prerequisites

- Access to the Globular CLI from a healthy node
- Admin permissions
- For hard recovery: a replacement machine with Globular installed

## Steps

### Scenario A: Node is reachable but services are down

#### Step 1: Diagnose the problem

```bash
globular cluster health
# Identify the unhealthy node

globular doctor report --fresh
# Get detailed findings for the node
```

#### Step 2: Check the Node Agent

```bash
# Try to reach the node agent
globular node logs --node <node>:11000 --unit globular-node-agent --lines 50
```

If the node agent responds, it's running. The problem is with individual services.

If the node agent doesn't respond:
```bash
# SSH to the node and restart the agent
ssh <node>
sudo systemctl restart globular-node-agent
sudo systemctl status globular-node-agent
```

#### Step 3: Restart failed services

```bash
# Option 1: Let the Cluster Doctor auto-heal (if enforce mode is enabled)
globular doctor set-mode enforce
# Wait for the healer to restart stopped services

# Option 2: Manually restart specific services
globular node control --node <node>:11000 --unit <service> --action restart
```

#### Step 4: Verify recovery

```bash
globular cluster health
# Node should return to healthy

globular services repair --dry-run
# No drift
```

### Scenario B: Node is unreachable (but may recover)

#### Step 1: Assess the situation

```bash
globular cluster health
# Shows: <node> unreachable, last seen X minutes ago
```

#### Step 2: Check physical access

```bash
# Can you reach the machine?
ping <node-ip>
ssh <node-ip>
```

If the machine is reachable via SSH:
```bash
# Check system health
sudo systemctl status globular-node-agent
sudo journalctl -u globular-node-agent --no-pager -n 50

# Restart the node agent
sudo systemctl restart globular-node-agent
```

If the machine is not reachable:
- Check power, network cables, switch ports
- Check if the machine rebooted (check uptime after access restored)
- Check hypervisor/cloud console if virtual

#### Step 3: Wait for automatic recovery

Once the node agent restarts:
1. It sends a heartbeat to the controller
2. The controller detects it's back online
3. The controller checks for drift
4. Workflows are dispatched for any misaligned services
5. The node converges to desired state

```bash
# Monitor recovery
globular cluster health
# Watch for the node to transition from "unreachable" to "converging" to "healthy"
```

### Scenario C: Node is permanently lost (hardware failure)

#### Step 1: Remove the failed node

```bash
globular cluster nodes remove <node-id>
```

This:
- Removes the node from the controller's registry
- Removes it from etcd cluster membership (if it was a member)
- Updates MinIO pool configuration
- Stops dispatching workflows to the node

#### Step 2: Provision replacement hardware

On the new machine:
```bash
# Install Globular (same version as the cluster)
sudo ./install.sh

# Start the Node Agent
sudo systemctl start globular-node-agent
```

#### Step 3: Join the new node

```bash
# Create a join token
globular cluster token create --expires 2h

# Join
globular cluster join \
  --node <new-node-ip>:11000 \
  --controller <controller-ip>:12000 \
  --join-token <token>

# Approve with the same profiles as the failed node
globular cluster requests approve <request-id> \
  --profile <profile1> \
  --profile <profile2>
```

#### Step 4: Wait for convergence

```bash
# Monitor installation
globular services desired list
globular cluster health

# etcd replicates to the new node (if core profile)
# MinIO rebuilds erasure shards
# All profile services install via workflows
```

#### Step 5: Verify

```bash
globular cluster health
# Shows: N/N nodes healthy (same count as before failure)

globular services repair --dry-run
# All INSTALLED
```

## Verification

For all scenarios:

```bash
# Node is healthy
globular cluster health

# Services are running
globular services desired list
# All INSTALLED

# No outstanding issues
globular doctor report --fresh
# HEALTHY, no critical findings

# No drift
globular services repair --dry-run
```

## Troubleshooting

### Node keeps going unhealthy

Check for recurring issues:
```bash
# Check system resources
ssh <node> free -h          # Memory
ssh <node> df -h            # Disk
ssh <node> dmesg | tail -50 # Kernel messages (OOM killer, hardware errors)
```

### etcd quorum lost after node removal

If you removed an etcd member and the cluster now has fewer than quorum:
```bash
# This is critical — you need to restore quorum
# If 2 of 3 nodes are healthy, quorum is maintained (2/3)
# If only 1 of 3 is healthy, you need to restore from backup

# Option 1: Add a new node with core profile quickly
# Option 2: Restore from backup (see Backup and Restore)
```

### Services installing slowly on replacement node

Check if MinIO is healthy (packages are downloaded from MinIO):
```bash
globular cluster health
# Check MinIO status

# Check workflow FETCH phase timing
globular workflow list --node <new-node>
globular workflow get <run-id>
# If FETCH is slow, MinIO may be rebuilding erasure shards
```

### Node agent won't start after reboot

```bash
ssh <node>
sudo journalctl -u globular-node-agent --no-pager -n 100

# Common causes:
# - Port 11000 already in use: ss -tlnp | grep 11000
# - Certificate expired: check /var/lib/globular/pki/
# - etcd unreachable: check network to etcd peers
```
