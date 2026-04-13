# Task: Add a Node

## Overview

Expand the Globular cluster by adding a new Linux machine. The new node will receive services based on its assigned profiles and participate in the cluster's convergence model.

Use this when:
- You need more compute capacity
- You want high availability (3+ nodes for etcd quorum)
- You're replacing a failed node

## Prerequisites

- An existing Globular cluster (`globular cluster health` shows HEALTHY)
- A new Linux machine (amd64) with Globular binaries installed
- Network connectivity between the new machine and the existing cluster (ports 11000, 12000, 2379, 2380)
- Authenticated CLI session with admin permissions

## Steps

### Step 1: Install Globular on the new node

On the new machine:
```bash
# Install Globular binaries (same method as the first node)
sudo ./install.sh

# Start the Node Agent
sudo systemctl start globular-node-agent
sudo systemctl status globular-node-agent
# Active: active (running)
```

### Step 2: Create a join token

On any machine with access to the controller:
```bash
globular cluster token create --expires 4h
```

Output:
```
Token: eyJhbGciOiJFZERTQSIs...
Expires: 2025-04-12T14:30:00Z
```

Copy the token — you'll need it in the next step.

### Step 3: Request to join

```bash
globular cluster join \
  --node <new-node-ip>:11000 \
  --controller <controller-ip>:12000 \
  --join-token <token>
```

Example:
```bash
globular cluster join \
  --node 192.168.1.50:11000 \
  --controller 192.168.1.10:12000 \
  --join-token eyJhbGciOiJFZERTQSIs...
```

Output:
```
request_id: req_xyz123
status: pending
```

### Step 4: Approve the join request

```bash
# List pending requests
globular cluster requests list

# Approve with profiles
globular cluster requests approve req_xyz123 \
  --profile worker \
  --profile monitoring
```

Add `--meta` for node metadata:
```bash
globular cluster requests approve req_xyz123 \
  --profile worker \
  --meta zone=us-east-1 \
  --meta rack=rack-3
```

### Step 5: Wait for service installation

The convergence model automatically installs all services for the assigned profiles. Monitor:

```bash
# Watch cluster health
globular cluster health
# The new node should appear and progress from "converging" to "healthy"

# Watch service installation
globular services desired list
# Services will show APPLYING as they install on the new node

# Watch workflows
globular workflow list --node <new-node-id>
```

Service installation typically takes 5-15 minutes depending on the number of services.

## Verification

```bash
# New node is healthy
globular cluster health
# Shows: N+1/N+1 nodes healthy

# All services installed
globular services desired list
# All services show INSTALLED with correct node count

# No drift
globular services repair --dry-run
# All INSTALLED
```

## Troubleshooting

### "connection refused" on join

The new node's Node Agent is not running or the port is blocked:
```bash
# On the new node:
sudo systemctl status globular-node-agent
ss -tlnp | grep 11000
```

### "invalid token"

The join token expired or was created for a different cluster:
```bash
# Create a new token
globular cluster token create --expires 4h
# Use the new token
```

### Join request stuck in "pending"

An admin hasn't approved it yet:
```bash
globular cluster requests list
globular cluster requests approve <request-id> --profile <profiles>
```

### Services not installing on new node

Check if workflows are being created:
```bash
globular workflow list --node <new-node-id>
```

If no workflows exist, the node's profiles may not match any desired services. Check profile assignment:
```bash
globular cluster nodes list
# Verify the profiles column
```

### etcd/MinIO expansion failed

For infrastructure services, check the controller logs:
```bash
globular node logs --node <controller-node>:11000 --unit controller --lines 100
```
