# High Availability

This page covers how Globular achieves high availability: leader election for the Cluster Controller, etcd quorum, MinIO erasure coding, service redundancy, and what happens during failover.

## Why High Availability Matters

A single-node Globular cluster is a single point of failure. If that node goes down, the entire cluster is unavailable. For production environments, Globular supports multi-node deployments where critical components run across multiple nodes with automatic failover.

High availability in Globular operates at three levels:
1. **Control plane HA**: The Cluster Controller uses leader election — one active leader, one or more standby instances that take over on failure
2. **Data store HA**: etcd runs as a replicated cluster with quorum-based writes, MinIO uses erasure coding
3. **Service HA**: Application services can run on multiple nodes with the Envoy gateway load-balancing between them

## Controller Leader Election

### How It Works

The Cluster Controller uses etcd lease-based leader election. In a multi-node cluster with the `core` profile on multiple nodes, multiple controller instances start but only one becomes the active leader.

**Election process**:
1. Each controller instance creates a lease in etcd and attempts to acquire a leadership key
2. The first instance to acquire the key becomes the leader
3. Other instances become standby — they monitor the lease and wait
4. The leader periodically renews its lease (heartbeat)
5. If the leader fails to renew (crash, network partition, freeze), the lease expires
6. A standby instance acquires the key and becomes the new leader

**Timing**:
- Lease TTL: configurable (typically 15-30 seconds)
- Lease renewal: every TTL/3 (typically 5-10 seconds)
- Failover time: at most one lease TTL period after leader failure

### Liveness Watchdog

The controller implements a **liveness watchdog** to detect zombie leaders — instances that hold the lease but are not actually processing requests (deadlocked thread, GC pause, I/O stall).

The watchdog runs as a goroutine that monitors internal processing:
1. If the leader processes no operations for a configurable threshold
2. The watchdog triggers a leadership resignation
3. The controller calls `ResignLeadership` which releases the etcd lease
4. A healthy standby instance acquires the lease

This prevents scenarios where a leader holds the lease for the full TTL duration while being effectively dead.

### What the Leader Does

Only the leader instance processes these operations:
- Desired-state management (UpsertDesiredService, RemoveDesiredService)
- Workflow dispatch (creating and tracking workflows)
- Node management (processing join requests, approving/rejecting nodes)
- Release reconciliation (periodic drift check and workflow creation)
- Health monitoring (evaluating node heartbeats)
- Infrastructure expansion (etcd member add, MinIO pool management)

Standby instances:
- Accept gRPC requests but forward them to the leader (or reject with a redirect)
- Monitor the leadership lease
- Are ready to take over within seconds

### Failover Behavior

When the leader fails and a standby takes over:

1. **etcd lease expires** (5-30 seconds after last renewal)
2. **Standby acquires lease** (typically < 1 second after expiry)
3. **New leader loads state from etcd** — all persistent state is in etcd, not in-memory only
4. **New leader rebuilds in-memory caches** — node registry, desired state, active operations
5. **Reconciliation loop resumes** — the new leader evaluates all desired-state entries
6. **In-flight workflows continue** — the Workflow Service runs independently and is unaffected by controller failover
7. **Node heartbeats route to new leader** — nodes discover the new leader via etcd service registration

**Impact during failover**:
- New desired-state commands fail during the election window (5-30 seconds)
- Running workflows are unaffected (they're managed by the Workflow Service, not the controller)
- Node heartbeats may be delayed (nodes retry with backoff)
- No data is lost (all state is in etcd)

## etcd Quorum

### How etcd HA Works

etcd uses the Raft consensus algorithm. In a cluster of N members:
- Writes require a **quorum** (majority): (N/2) + 1 members must agree
- Reads can be served by any member (for stale reads) or require quorum (for linearizable reads)

| Cluster Size | Quorum | Fault Tolerance |
|-------------|--------|----------------|
| 1 node | 1 | 0 (no HA) |
| 3 nodes | 2 | 1 node failure |
| 5 nodes | 3 | 2 node failures |
| 7 nodes | 4 | 3 node failures |

**Recommendation**: 3 or 5 nodes for production. Larger clusters have higher write latency.

### etcd Membership Management

The Cluster Controller manages etcd cluster membership:

**Adding a member** (during node join):
1. Controller calls `etcdctl member add` with the new node's peer URL
2. The new node's etcd instance starts configured to join the existing cluster
3. etcd replicates data to the new member
4. New member becomes a full voting member after sync

**Removing a member** (during node removal):
1. Controller calls `etcdctl member remove` with the member's ID
2. The remaining cluster continues with one fewer member
3. If this reduces the cluster below quorum, the operation is rejected

### etcd Health Monitoring

The Node Agent probes etcd health on each node:

```bash
# Health check uses the node's routable IP, never 127.0.0.1
etcdctl endpoint health --endpoints=https://<routable-ip>:2379
# Expected: {"health": true}
```

The controller tracks etcd member health as part of the overall node health assessment. An unhealthy etcd member triggers a WARN finding in the Cluster Doctor.

## MinIO Erasure Coding

### How MinIO HA Works

MinIO uses erasure coding to distribute data across nodes. In a multi-node setup:
- Data is split into data and parity shards
- Shards are distributed across nodes
- The cluster can reconstruct data even if some nodes are unavailable

| Nodes | Data Shards | Parity Shards | Fault Tolerance |
|-------|-------------|---------------|----------------|
| 4 | 2 | 2 | 2 node failures |
| 6 | 3 | 3 | 3 node failures |
| 8 | 4 | 4 | 4 node failures |

### MinIO Pool Expansion

When a new node joins with a profile that includes MinIO:
1. Controller configures the new MinIO instance with existing cluster credentials
2. New instance is added to the MinIO pool configuration
3. MinIO automatically rebalances data to include the new node
4. Erasure coding is recalculated with the expanded pool

## Service Redundancy

### Multi-Instance Services

Services assigned to multiple nodes (via profile assignment) run as independent instances. The Envoy gateway routes traffic between them:

```
Client → Envoy Gateway
              │
              ├── authentication@node-1 (healthy)  ← receives traffic
              ├── authentication@node-2 (healthy)  ← receives traffic
              └── authentication@node-3 (unhealthy) ← removed from rotation
```

Envoy performs active health checking and removes unhealthy backends from the rotation. When a service is restarted during an upgrade, traffic flows to the remaining healthy instances.

### Service-Level Failover

If a service crashes on one node:
1. systemd detects the crash and attempts to restart (on-failure policy)
2. The Node Agent's next heartbeat reports the changed unit state
3. The controller detects drift (unit not running despite being installed)
4. If systemd restart succeeds → heartbeat confirms recovery
5. If systemd restart fails → controller creates a repair workflow
6. Meanwhile, Envoy routes traffic to healthy instances on other nodes

### Infrastructure Service HA

Some infrastructure services have additional HA mechanisms:

**ScyllaDB**: Uses gossip-based replication. Data is replicated across nodes with configurable replication factors. A node failure doesn't lose data — reads and writes continue on remaining replicas.

**Prometheus**: Can be deployed in HA mode with multiple instances scraping the same targets. Alertmanager deduplicates alerts from multiple Prometheus instances.

## Failure Modes and Recovery

### Single Node Failure

**Impact**: Services on that node are unavailable. Services with instances on other nodes continue.

**Recovery**:
1. etcd: Quorum maintained if 2+ nodes remain (in a 3-node cluster)
2. MinIO: Erasure coding handles the missing node
3. Controller: If the leader was on the failed node, a standby takes over
4. Services: Gateway routes traffic to remaining healthy instances
5. Node agent: When the node recovers, it sends a heartbeat and the controller reconciles

### Network Partition

**Impact**: Nodes in the minority partition lose etcd quorum and cannot write. Nodes in the majority partition continue normally.

**Recovery**:
1. Nodes in the minority partition detect they cannot reach quorum
2. Controllers in the minority partition resign leadership (cannot write to etcd)
3. Services in the minority partition continue serving cached state (reads may work)
4. When the partition heals, etcd re-synchronizes automatically
5. The controller reconciles any drift that accumulated during the partition

### Controller Crash

**Impact**: No new desired-state operations. Running workflows continue.

**Recovery**:
1. etcd lease expires (5-30 seconds)
2. Standby controller acquires leadership (< 1 second)
3. New leader loads state from etcd
4. Normal operations resume
5. Total downtime: 5-30 seconds for control plane operations

### etcd Member Failure

**Impact**: Depends on cluster size. 3-node cluster with 1 failure → quorum maintained.

**Recovery**:
1. Remaining members continue serving reads and writes
2. The Cluster Doctor detects the missing member
3. When the node recovers, etcd re-syncs automatically
4. If the node is permanently lost, remove the member and add a replacement

## Practical Scenarios

### Scenario 1: Setting Up a 3-Node HA Cluster

```bash
# Node 1: Bootstrap
globular cluster bootstrap \
  --node node-1:11000 \
  --domain prod.example.com \
  --profile core --profile gateway

# Node 2: Join with core profile
globular cluster token create --expires 4h
globular cluster join --node node-2:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core

# Node 3: Join with core profile
globular cluster join --node node-3:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core

# Verify HA
globular cluster health
# 3/3 nodes healthy
# etcd: 3 members, quorum=2
# Controller: leader on node-1, standby on node-2 and node-3
```

### Scenario 2: Controller Failover Test

```bash
# Identify current leader
globular cluster health
# Controller leader: node-1

# Simulate leader failure (on node-1)
sudo systemctl stop globular-controller

# Wait for failover (5-30 seconds)
# New leader elected on node-2 or node-3

# Verify
globular cluster health
# Controller leader: node-2
# node-1: controller stopped (will be repaired when node-1 agent reports)

# Restart controller on node-1 (becomes standby)
sudo systemctl start globular-controller
```

### Scenario 3: Recovering from Node Loss

Node-2 hardware failure in a 3-node cluster:

```bash
# Cluster detects node-2 unreachable
globular cluster health
# node-2: unreachable (last seen 5m ago)
# Cluster: DEGRADED (etcd quorum maintained: 2/3)

# Remove the failed node
globular cluster nodes remove <node-2-id>

# Provision replacement hardware, install Globular
# Join the new node
globular cluster token create --expires 2h
globular cluster join --node new-node-2:11000 --controller node-1:12000 --join-token <token>
globular cluster requests approve <req-id> --profile core

# etcd re-replicates to new node
# MinIO rebuilds erasure shards
# All services install via convergence model

globular cluster health
# 3/3 nodes healthy, cluster fully restored
```

## What's Next

- [Failure Scenarios and Recovery](operators/failure-scenarios.md): Comprehensive failure pattern guide
- [Network and Routing](operators/network-and-routing.md): Envoy gateway, xDS, and service mesh
