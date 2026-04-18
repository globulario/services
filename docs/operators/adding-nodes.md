# Adding Nodes (Day-1)

This page covers expanding a Globular cluster by adding new nodes. The process involves creating a join token, requesting to join from the new node, approving the request, and watching the convergence model install services automatically.

## Why Add Nodes

Adding nodes to a Globular cluster provides:

- **High availability**: etcd, the Cluster Controller, and MinIO can run across multiple nodes with quorum-based failover. A 3-node cluster tolerates the loss of one node.
- **Capacity**: More nodes means more compute and storage capacity for running services.
- **Isolation**: Different nodes can run different profiles — for example, one node runs the control plane while another runs compute-intensive workloads.
- **Geographic distribution**: Nodes can be in different network segments or locations for fault isolation.

## Join Workflow

Adding a node is a multi-step workflow that involves the existing controller, the new node's agent, and an administrator for approval.

### Step 1: Create a Join Token

On any machine with access to the controller, create a time-limited join token:

```bash
globular cluster token create --expires 72h
```

Output:
```
Token: eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...
Expires: 2025-04-15T10:30:00Z (72h from now)
```

The join token is a JWT signed by the controller. It contains:
- The cluster domain
- An expiration timestamp
- A unique token ID (for revocation)

The token does not grant access to the cluster — it only authorizes the join request. The actual admission decision is made by an administrator.

**Security considerations**:
- Set the shortest practical expiration. A 72-hour token is suitable for planned expansions. For urgent additions, use `--expires 1h`.
- A token can be used multiple times (one per node). If you only want one node, revoke the token after use.
- Share tokens through secure channels — anyone with the token can submit a join request.

### Step 2: Prepare the New Node

On the new machine:

1. Install the Globular binaries (same version as the existing cluster)
2. Start the Node Agent:

```bash
sudo systemctl start globular-node-agent
# Or manually:
node_agent_server --port 11000
```

3. Ensure the new node can reach the controller node on port 12000 (gRPC) and port 2380 (etcd peer).

### Step 3: Request to Join

From the new node (or any machine that can reach both the new node and the controller):

```bash
globular cluster join \
  --node newnode.example.com:11000 \
  --controller controller.mycluster.local:12000 \
  --join-token eyJhbGciOiJFZERTQSIs...
```

**Flags explained**:
- `--node`: The Node Agent endpoint on the new machine
- `--controller`: The Cluster Controller endpoint on the existing cluster
- `--join-token`: The token created in Step 1

What happens internally:

1. The CLI connects to the controller and calls `RequestJoin`
2. The controller validates the join token (signature, expiration, cluster domain)
3. The controller queries the new node's agent for its identity:
   - Hostname, IP addresses, MAC addresses
   - Hardware capabilities (CPU, RAM, disk)
   - Current software inventory
4. The controller creates a `JoinRequestRecord` in etcd with status `pending`
5. The CLI returns the request ID and status

```
request_id: req_xyz123
status: pending
message: "Join request submitted, awaiting admin approval"
```

### Step 4: Approve the Join Request

An administrator reviews and approves the request:

```bash
# List pending requests
globular cluster requests list
# Output:
# REQUEST ID   HOSTNAME         IP              STATUS   SUBMITTED
# req_xyz123   newnode          192.168.1.50    pending  2m ago

# Approve with profile assignment
globular cluster requests approve req_xyz123 \
  --profile worker \
  --profile monitoring \
  --meta zone=us-east-1 \
  --meta rack=rack-3
```

**Flags explained**:
- `--profile`: Assign profiles that determine which services this node will run. Multiple profiles can be specified.
- `--meta`: Arbitrary key-value metadata for the node (zone, rack, team, etc.)

What happens when approved:

1. Controller generates a unique `node_id` (e.g., `node_abc456`)
2. Controller creates a node identity token (JWT with `node_<uuid>` principal)
3. Controller creates a `NodeRecord` in etcd with the node's identity, profiles, and metadata
4. Controller sends the node token and cluster configuration to the Node Agent
5. Node Agent stores the token and begins cluster participation

### Step 5: Automatic Service Installation

Once approved, the convergence model takes over. The controller:

1. Evaluates the node's profiles to determine which services it should run
2. For each service in the node's profiles, checks if a `DesiredService` entry exists
3. Creates workflows to install each service on the new node

The standard installation workflow executes:
```
DECISION → FETCH → INSTALL → CONFIGURE → START → VERIFY → COMPLETE
```

For each service:
- **FETCH**: Download the package from the Repository service (MinIO)
- **INSTALL**: Verify checksum, extract binary, write systemd unit
- **CONFIGURE**: Write service configuration to etcd
- **START**: `systemctl start <service>`
- **VERIFY**: Check gRPC health endpoint

Monitor the progress:

```bash
# Watch service installation
globular services desired list
# Shows: APPLYING for services being installed, INSTALLED for completed

# Check node-specific status
globular cluster nodes list
# Shows node status, profile assignment, service count

# View workflow progress
globular workflow list --node node_abc456
```

### Step 6: Infrastructure Expansion

When a new node joins with certain profiles, the controller also expands infrastructure:

**etcd cluster expansion**: If the cluster has fewer than the target etcd members (typically 3 or 5), the controller adds the new node to the etcd cluster:
1. Calls `etcdctl member add` with the new node's peer URL
2. Starts etcd on the new node configured to join the existing cluster
3. Waits for the new member to sync

**MinIO pool expansion**: If MinIO is assigned to the new node's profiles, the controller adds a new erasure pool:
1. Configures the new MinIO instance with the existing cluster credentials
2. Adds the new node to the MinIO pool configuration
3. MinIO rebalances data across the expanded pool

**ScyllaDB expansion**: If ScyllaDB is assigned:
1. Configures the new ScyllaDB instance with gossip seeds pointing to existing nodes
2. Starts ScyllaDB, which joins the cluster via gossip protocol
3. Data begins streaming to the new node

### Step 7: Verify

After all services are installed (typically 5-15 minutes depending on the number of services):

```bash
# Check cluster health
globular cluster health
# Output:
# CLUSTER STATUS: HEALTHY
# NODES: 2/2 healthy
#
# NODE         STATUS   LAST SEEN    SERVICES
# node-abc123  healthy  3s ago       12/12 running
# node-abc456  healthy  1s ago       8/8 running

# Verify no drift
globular cluster get-drift-report
# All services should show INSTALLED
```

## Rejecting a Join Request

If a join request should not be approved:

```bash
globular cluster requests reject req_xyz123 --reason "Unknown machine, not authorized"
```

The request is marked as rejected in etcd. The new node can re-request with a valid token if this was a mistake.

## Removing a Node

To remove a node from the cluster:

```bash
globular cluster nodes remove <node-id>
```

This:
1. Marks the node as `removing` in the controller's node registry
2. Stops dispatching workflows to the node
3. Removes the node from etcd cluster membership (if it was an etcd member)
4. Removes the node from MinIO pools (if applicable)
5. Updates desired-state computations to exclude the node
6. Removes the node record from etcd

Services on the removed node continue running but are no longer managed by the cluster. The node agent should be stopped manually.

## Profile Changes

After a node has joined, its profiles can be updated:

```bash
# Add a new profile
globular cluster nodes set-profiles <node-id> \
  --profile worker \
  --profile monitoring \
  --profile compute

# This triggers:
# - Workflows to install services in the new profile (compute)
# - Services already installed from existing profiles are unaffected
```

Removing a profile from a node stops and uninstalls services that were exclusively from that profile (services shared with other assigned profiles remain).

## Multi-Node Topologies

### 3-Node Production Cluster

A typical production setup:

| Node | Profiles | Services |
|------|----------|----------|
| node-1 | core, gateway | controller, auth, rbac, event, discovery, gateway, etcd, minio |
| node-2 | core, database | controller (standby), auth, etcd, minio, postgresql, scylladb |
| node-3 | worker, monitoring | compute, monitoring, prometheus, alertmanager, etcd, minio |

This provides:
- etcd quorum across 3 nodes (tolerates 1 failure)
- MinIO erasure coding across 3 nodes
- Controller HA (leader on node-1, standby on node-2)
- Service redundancy for authentication and RBAC

### Adding Nodes to Existing 3-Node Cluster

```bash
# Create a token
globular cluster token create --expires 24h

# For each new node:
globular cluster join --node newnode:11000 --controller controller:12000 --join-token <token>
globular cluster requests approve <request-id> --profile worker
```

Additional worker nodes don't need to join the etcd or MinIO clusters — they only run application services and report status to the controller.

## Practical Scenarios

### Scenario 1: Adding a Worker Node

You have a 3-node cluster and need more compute capacity:

```bash
# On the existing cluster
globular cluster token create --expires 4h
# Token: abc123...

# On the new node
sudo systemctl start globular-node-agent
globular cluster join --node worker-4:11000 --controller controller:12000 --join-token abc123...

# On the existing cluster
globular cluster requests list
# req_001  worker-4  192.168.1.54  pending  1m ago

globular cluster requests approve req_001 --profile worker --meta role=compute
# Node node_def789 added, 5 services queued for installation

# Wait for convergence
globular services desired list
# Shows APPLYING... then INSTALLED for each service on worker-4
```

### Scenario 2: Replacing a Failed Node

Node-2 had a hardware failure. Replace it:

```bash
# Remove the failed node
globular cluster nodes remove <node-2-id>

# Provision new hardware, install Globular binaries, start node agent
# Then join with the same profiles as the failed node:
globular cluster token create --expires 2h
globular cluster join --node new-node-2:11000 --controller controller:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core --profile database

# The convergence model installs all services that were on the failed node
# etcd automatically rebalances (if the failed node was an etcd member)
# MinIO rebuilds erasure-coded data on the new node
```

### Scenario 3: Expanding etcd Cluster

Your single-node cluster needs HA. Add two more nodes:

```bash
# Node 2
globular cluster join --node node-2:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core

# Node 3
globular cluster join --node node-3:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core

# The controller automatically:
# 1. Adds node-2 and node-3 to the etcd cluster
# 2. Waits for etcd to reach quorum (3 members)
# 3. Starts the controller standby on nodes with core profile
# 4. Installs all core-profile services
```

After expansion, the cluster tolerates the loss of any one node without data loss or service interruption.

## What's Next

- [Deploying Applications](operators/deploying-applications.md): Deploy and manage services
- [Updating the Cluster](operators/updating-the-cluster.md): Upgrade services and infrastructure
