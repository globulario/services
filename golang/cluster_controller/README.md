# Globular Cluster Setup Guide

This guide covers the complete process of creating and configuring a Globular cluster from initial bootstrap (Day 0) through production operation (Day 1+).

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Prerequisites](#prerequisites)
3. [Day 0: Cluster Bootstrap](#day-0-cluster-bootstrap)
4. [Day 1: Growing the Cluster](#day-1-growing-the-cluster)
5. [Network Configuration](#network-configuration)
6. [Node Profiles](#node-profiles)
7. [Service Configuration](#service-configuration)
8. [Plan System](#plan-system)
9. [Operations & Monitoring](#operations--monitoring)
10. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

A Globular cluster consists of three main components working together:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Globular Cluster                                │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │              Cluster Controller (port 12000)                    │   │
│  │                                                                  │   │
│  │  - Central control plane for the cluster                        │   │
│  │  - Manages node membership and join requests                    │   │
│  │  - Generates and dispatches configuration plans                 │   │
│  │  - Tracks cluster network configuration                         │   │
│  │  - Reconciles node state every 15 seconds                       │   │
│  └──────────────────────────┬──────────────────────────────────────┘   │
│                              │                                          │
│              ┌───────────────┼───────────────┐                          │
│              │               │               │                          │
│              ▼               ▼               ▼                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐               │
│  │  Node Agent   │  │  Node Agent   │  │  Node Agent   │               │
│  │  (port 11000) │  │  (port 11000) │  │  (port 11000) │               │
│  │               │  │               │  │               │               │
│  │  - Receives   │  │  - Receives   │  │  - Receives   │               │
│  │    plans      │  │    plans      │  │    plans      │               │
│  │  - Executes   │  │  - Executes   │  │  - Executes   │               │
│  │    actions    │  │    actions    │  │    actions    │               │
│  │  - Reports    │  │  - Reports    │  │  - Reports    │               │
│  │    status     │  │    status     │  │    status     │               │
│  └───────────────┘  └───────────────┘  └───────────────┘               │
│        Node 1            Node 2             Node 3                      │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Shared Services                               │   │
│  │                                                                  │   │
│  │   etcd (2379/2380)  │  MinIO (9000)  │  XDS/Envoy (gateway)     │   │
│  │                                                                  │   │
│  │   - Configuration   │  - Object      │  - Service mesh          │   │
│  │     storage         │    storage     │  - TLS termination       │   │
│  │   - Plan store      │  - Artifacts   │  - Load balancing        │   │
│  │   - Distributed     │  - Backups     │  - Dynamic routing       │   │
│  │     locking         │                │                          │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### How It Works

1. **Cluster Controller** maintains the desired state of the cluster
2. **Node Agents** run on each node and execute configuration plans
3. **Plans** are declarative specifications that describe what services should run
4. **Reconciliation Loop** continuously ensures nodes match their desired state
5. **Service Configurations** are automatically generated based on cluster membership

---

## Prerequisites

Before setting up a cluster, ensure each node has:

- Linux operating system (Ubuntu 20.04+ or similar)
- Network connectivity between all nodes
- Ports open: 11000 (agent), 12000 (controller), 2379-2380 (etcd), 9000 (MinIO)
- Globular binary installed at `/usr/local/bin/globular`
- systemd for service management

### Required Environment Variables

```bash
# Optional: Override default ports
export NODE_AGENT_PORT=11000
export CLUSTER_PORT=12000

# Optional: TLS configuration
export NODE_AGENT_TLS_CERT=/path/to/cert.pem
export NODE_AGENT_TLS_KEY=/path/to/key.pem
export NODE_AGENT_TLS_CA=/path/to/ca.pem
```

---

## Day 0: Cluster Bootstrap

Day 0 is the initial cluster creation. This involves bootstrapping the first node which becomes the initial control plane.

### Step 1: Start the Node Agent

On the first node, start the node agent service:

```bash
# Start the node agent (if using systemd)
sudo systemctl start globular-nodeagent

# Or start manually for testing
globular nodeagent --port 11000
```

### Step 2: Bootstrap the Cluster

Bootstrap creates the cluster with the first node:

```bash
globular cluster bootstrap \
  --node=localhost:11000 \
  --domain=mycluster.example.com \
  --profile=core
```

**What happens during bootstrap:**

1. Node agent initializes local etcd in managed mode
2. Cluster controller starts on the node
3. First node automatically joins with specified profiles
4. A join token is generated for adding more nodes
5. Initial network configuration is set

**Output:**
```
Cluster bootstrapped successfully
  Cluster ID:    a1b2c3d4-e5f6-7890-abcd-ef1234567890
  Domain:        mycluster.example.com
  Join Token:    eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  Controller:    192.168.1.10:12000

Save the join token to add more nodes to the cluster.
```

### Step 3: Verify Bootstrap

Check that the cluster is running:

```bash
# List nodes in the cluster
globular cluster nodes list

# Expected output:
# NODE ID                               HOSTNAME    STATUS    PROFILES    LAST SEEN
# a1b2c3d4-e5f6-7890-abcd-ef1234567890  host1       ready     core        2024-01-15T10:30:00Z
```

### Step 4: Configure Network Settings

Set the cluster network configuration:

```bash
# For HTTP-only (development)
globular cluster network set \
  --domain=mycluster.example.com \
  --protocol=http \
  --http-port=80

# For HTTPS with Let's Encrypt (production)
globular cluster network set \
  --domain=mycluster.example.com \
  --protocol=https \
  --https-port=443 \
  --acme \
  --email=admin@example.com
```

**What happens:**

1. Controller updates `ClusterNetworkSpec` in state
2. `NetworkingGeneration` is incremented
3. New plans are generated for all nodes
4. Plans include updated `/var/lib/globular/network.json`
5. Affected services are restarted

---

## Day 1: Growing the Cluster

Day 1 operations involve adding nodes, changing profiles, and scaling the cluster.

### Adding a New Node

#### Step 1: Create a Join Token

On a machine with access to the controller:

```bash
# Create a token that expires in 24 hours
globular cluster token create --expires=24h

# Output:
# Join Token: abc123-def456-ghi789
# Expires: 2024-01-16T10:30:00Z
```

#### Step 2: Start Node Agent on New Node

On the new node:

```bash
sudo systemctl start globular-nodeagent
```

#### Step 3: Request to Join

On the new node:

```bash
globular cluster join \
  --controller=192.168.1.10:12000 \
  --join-token=abc123-def456-ghi789
```

**Output:**
```
Join request submitted
  Request ID: req-xyz789
  Status: pending

Waiting for administrator approval...
```

#### Step 4: Approve the Join Request

Back on the controller (or any node with CLI access):

```bash
# List pending requests
globular cluster requests list

# Output:
# REQUEST ID    HOSTNAME    IPS              STATUS    REQUESTED
# req-xyz789    host2       192.168.1.11     pending   2024-01-15T11:00:00Z

# Approve with profiles
globular cluster requests approve req-xyz789 \
  --profile=core \
  --profile=storage
```

**What happens after approval:**

1. New node is assigned a `NodeID`
2. Node enters `converging` status
3. Controller generates a plan based on profiles
4. Plan is dispatched to the node agent
5. Node agent applies the plan:
   - Writes service configurations (etcd, MinIO, XDS)
   - Enables and starts required systemd units
6. Node reports status via heartbeat
7. Once all units are running, status becomes `ready`

#### Step 5: Verify the New Node

```bash
globular cluster nodes list

# Output:
# NODE ID       HOSTNAME    STATUS    PROFILES         LAST SEEN
# a1b2c3d4...   host1       ready     core             2024-01-15T11:05:00Z
# e5f6g7h8...   host2       ready     core,storage     2024-01-15T11:05:00Z
```

### Changing Node Profiles

To add or change profiles on an existing node:

```bash
# Add gateway profile to a node
globular cluster nodes profiles set e5f6g7h8... \
  --profile=core \
  --profile=storage \
  --profile=gateway
```

**What happens:**

1. Node profiles are updated in controller state
2. `computeNodePlan()` generates new plan with updated services
3. Plan hash changes, triggering dispatch
4. Node agent receives and applies the new plan
5. New services are started, removed services are stopped

---

## Network Configuration

The cluster network configuration controls how services communicate and how external traffic reaches the cluster.

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `--domain` | Primary cluster domain | Required |
| `--protocol` | `http` or `https` | `http` |
| `--http-port` | HTTP listen port | `80` |
| `--https-port` | HTTPS listen port | `443` |
| `--acme` | Enable Let's Encrypt | `false` |
| `--email` | Admin email (required for ACME) | - |
| `--alt-domain` | Additional domains (repeatable) | - |

### View Current Configuration

```bash
globular cluster network get

# Output:
# Domain:           mycluster.example.com
# Protocol:         https
# HTTP Port:        80
# HTTPS Port:       443
# ACME Enabled:     true
# Admin Email:      admin@example.com
# Alt Domains:      api.example.com, app.example.com
# Generation:       3
```

### Update Configuration

```bash
# Add an alternate domain
globular cluster network set \
  --domain=mycluster.example.com \
  --protocol=https \
  --acme \
  --email=admin@example.com \
  --alt-domain=api.example.com \
  --alt-domain=app.example.com \
  --watch  # Watch the operation progress
```

### What Network Config Affects

When network configuration changes:

1. **All nodes** receive updated `/var/lib/globular/network.json`:
   ```json
   {
     "Domain": "mycluster.example.com",
     "Protocol": "https",
     "PortHTTP": 80,
     "PortHTTPS": 443,
     "AlternateDomains": ["api.example.com", "app.example.com"],
     "ACMEEnabled": true,
     "AdminEmail": "admin@example.com"
   }
   ```

2. **Services are restarted** to pick up new configuration:
   - globular-etcd.service
   - globular-dns.service
   - globular-discovery.service
   - globular-xds.service
   - globular-envoy.service
   - globular-gateway.service
   - globular-minio.service

---

## Node Profiles

Profiles define what services run on a node. A node can have multiple profiles.

### Available Profiles

| Profile | Services | Use Case |
|---------|----------|----------|
| `core` | etcd, DNS, discovery, event, RBAC, MinIO, file | Full-featured node |
| `compute` | etcd, DNS, discovery, event, RBAC, MinIO, file | Worker node |
| `control-plane` | etcd, DNS, discovery | Lightweight controller |
| `gateway` | gateway, envoy | Ingress/edge node |
| `storage` | MinIO, file | Storage-focused node |

### Profile to Service Mapping

```
core / compute:
  ├── globular-etcd.service
  ├── globular-dns.service
  ├── globular-discovery.service
  ├── globular-event.service
  ├── globular-rbac.service
  ├── globular-file.service
  └── globular-minio.service

control-plane:
  ├── globular-etcd.service
  ├── globular-dns.service
  └── globular-discovery.service

gateway:
  ├── globular-gateway.service
  └── envoy.service

storage:
  ├── globular-minio.service
  └── globular-file.service
```

### Profile Assignment Examples

```bash
# Single profile
globular cluster nodes profiles set <node-id> --profile=core

# Multiple profiles (node runs all services from both)
globular cluster nodes profiles set <node-id> \
  --profile=compute \
  --profile=gateway

# Dedicated storage node
globular cluster nodes profiles set <node-id> --profile=storage
```

---

## Service Configuration

When nodes join or profiles change, the cluster controller automatically generates service-specific configurations based on cluster membership.

### Configuration Files Generated

| Service | File Path | Description |
|---------|-----------|-------------|
| etcd | `/var/lib/globular/etcd/etcd.yaml` | Cluster membership, peer URLs |
| MinIO | `/var/lib/globular/minio/minio.env` | Distributed storage endpoints |
| XDS | `/var/lib/globular/xds/config.json` | etcd endpoints, TLS config |
| DNS | `/var/lib/globular/dns/dns_init.json` | SOA, NS, and glue records |
| Network | `/var/lib/globular/network.json` | Cluster domain, protocol |

### etcd Configuration

Generated for nodes with profiles: `core`, `compute`, `control-plane`

**Single node:**
```yaml
name: "host1"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "http://127.0.0.1:2379"
advertise-client-urls: "http://192.168.1.10:2379"
listen-peer-urls: "http://192.168.1.10:2380"
initial-advertise-peer-urls: "http://192.168.1.10:2380"
initial-cluster: "host1=http://192.168.1.10:2380"
initial-cluster-state: "new"
initial-cluster-token: "cluster-id-etcd-cluster"
```

**Multi-node cluster:**
```yaml
name: "host1"
data-dir: "/var/lib/globular/etcd"
listen-client-urls: "http://192.168.1.10:2379,http://127.0.0.1:2379"
advertise-client-urls: "http://192.168.1.10:2379"
listen-peer-urls: "http://192.168.1.10:2380"
initial-advertise-peer-urls: "http://192.168.1.10:2380"
initial-cluster: "host1=http://192.168.1.10:2380,host2=http://192.168.1.11:2380,host3=http://192.168.1.12:2380"
initial-cluster-state: "new"
initial-cluster-token: "cluster-id-etcd-cluster"
```

### MinIO Configuration

Generated for nodes with profiles: `core`, `compute`, `storage`

**Single node:**
```bash
MINIO_VOLUMES=/var/lib/globular/minio/data
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
```

**Multi-node (distributed mode):**
```bash
MINIO_VOLUMES=http://192.168.1.10:9000/var/lib/globular/minio/data http://192.168.1.11:9000/var/lib/globular/minio/data http://192.168.1.12:9000/var/lib/globular/minio/data
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin
```

### XDS Configuration

Generated for nodes with profiles: `core`, `compute`, `control-plane`, `gateway`

```json
{
  "etcd_endpoints": ["192.168.1.10:2379", "192.168.1.11:2379"],
  "sync_interval_seconds": 5,
  "ingress": {
    "tls": {
      "enabled": true,
      "cert_chain_path": "/var/lib/globular/config/tls/fullchain.pem",
      "private_key_path": "/var/lib/globular/config/tls/privkey.pem"
    }
  }
}
```

### DNS Configuration

Generated for nodes with profiles: `core`, `compute`, `control-plane`, `dns`

The DNS init config provides authoritative DNS setup including SOA, NS, and glue records. The node agent applies these records via the DNS service gRPC API.

**Single DNS node:**
```json
{
  "domain": "mycluster.example.com",
  "soa": {
    "domain": "mycluster.example.com",
    "ns": "ns1.mycluster.example.com.",
    "mbox": "admin.mycluster.example.com.",
    "serial": 2024011800,
    "refresh": 7200,
    "retry": 3600,
    "expire": 1209600,
    "minttl": 3600,
    "ttl": 3600
  },
  "ns_records": [
    {"ns": "ns1.mycluster.example.com", "ttl": 3600}
  ],
  "glue_records": [
    {"hostname": "ns1.mycluster.example.com", "ip": "192.168.1.10", "ttl": 3600}
  ],
  "is_primary": true
}
```

**Multi-node DNS cluster:**
```json
{
  "domain": "mycluster.example.com",
  "soa": {
    "domain": "mycluster.example.com",
    "ns": "ns1.mycluster.example.com.",
    "mbox": "admin.mycluster.example.com.",
    "serial": 2024011800,
    "refresh": 7200,
    "retry": 3600,
    "expire": 1209600,
    "minttl": 3600,
    "ttl": 3600
  },
  "ns_records": [
    {"ns": "ns1.mycluster.example.com", "ttl": 3600},
    {"ns": "ns2.mycluster.example.com", "ttl": 3600}
  ],
  "glue_records": [
    {"hostname": "ns1.mycluster.example.com", "ip": "192.168.1.10", "ttl": 3600},
    {"hostname": "ns2.mycluster.example.com", "ip": "192.168.1.11", "ttl": 3600}
  ],
  "is_primary": true
}
```

**Note:** Only the primary DNS node (first in sorted order) sets SOA and NS records. Secondary nodes only set their own glue A records to avoid conflicts.

### How Configuration Updates Work

```
┌──────────────────┐     ┌───────────────────┐     ┌──────────────────┐
│   Node Joins /   │     │ Cluster Controller │     │   Node Agent     │
│  Profile Change  │────▶│                    │────▶│                  │
└──────────────────┘     │ 1. Snapshot        │     │ 5. Receive plan  │
                         │    membership      │     │ 6. Write configs │
                         │ 2. Filter by       │     │ 7. Restart svcs  │
                         │    profile         │     │ 8. Report status │
                         │ 3. Generate        │     └──────────────────┘
                         │    configs         │
                         │ 4. Dispatch plan   │
                         └───────────────────┘
```

---

## Plan System

Plans are the core mechanism for managing node configuration. They are declarative specifications that describe what a node should look like.

### Plan Structure

```
NodePlan
├── Metadata
│   ├── node_id: Target node
│   ├── profiles: Assigned profiles
│   └── generation: Config version
│
├── UnitActions[]
│   ├── unit_name: "globular-etcd.service"
│   │   action: "enable" / "start"
│   ├── unit_name: "globular-minio.service"
│   │   action: "enable" / "start"
│   └── ...
│
└── RenderedConfig{}
    ├── "/var/lib/globular/etcd/etcd.yaml": <content>
    ├── "/var/lib/globular/minio/minio.env": <content>
    ├── "/var/lib/globular/xds/config.json": <content>
    ├── "/var/lib/globular/dns/dns_init.json": <content>
    ├── "/var/lib/globular/network.json": <content>
    └── "cluster.network.generation": "42"
```

### View a Node's Plan

```bash
globular cluster plan get <node-id>

# Output shows:
# - Unit actions (enable/start services)
# - Rendered configuration files
# - Plan hash for change detection
```

### Plan Execution Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Plan Execution Flow                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. PLAN GENERATION (Controller)                                        │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ computeNodePlan(node)                                       │    │
│     │   ├── buildPlanActions(profiles) → UnitActions              │    │
│     │   └── renderedConfigForNode(node)                           │    │
│     │         ├── Network config (domain, protocol, ports)        │    │
│     │         └── Service configs (etcd, MinIO, XDS)              │    │
│     └─────────────────────────────────────────────────────────────┘    │
│                                    │                                    │
│                                    ▼                                    │
│  2. CHANGE DETECTION                                                    │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ planHash(plan) → SHA256                                     │    │
│     │   - Hash includes all unit actions + rendered configs       │    │
│     │   - Compare with node.LastPlanHash                          │    │
│     │   - If different OR node not ready → dispatch               │    │
│     └─────────────────────────────────────────────────────────────┘    │
│                                    │                                    │
│                                    ▼                                    │
│  3. DISPATCH (Controller → Agent)                                       │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ dispatchPlan(node, plan, operationID)                       │    │
│     │   - Connect to node.AgentEndpoint via gRPC                  │    │
│     │   - Call ApplyPlan RPC                                      │    │
│     │   - Track operation for monitoring                          │    │
│     └─────────────────────────────────────────────────────────────┘    │
│                                    │                                    │
│                                    ▼                                    │
│  4. EXECUTION (Node Agent)                                              │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ For each config in RenderedConfig:                          │    │
│     │   - If path starts with "/": write to filesystem            │    │
│     │   - Set appropriate permissions                             │    │
│     │                                                             │    │
│     │ For each action in UnitActions:                             │    │
│     │   - "enable": systemctl enable <unit>                       │    │
│     │   - "start":  systemctl start <unit>                        │    │
│     │   - Wait for unit to become active                          │    │
│     └─────────────────────────────────────────────────────────────┘    │
│                                    │                                    │
│                                    ▼                                    │
│  5. STATUS REPORTING                                                    │
│     ┌─────────────────────────────────────────────────────────────┐    │
│     │ Node Agent → Controller (ReportNodeStatus)                  │    │
│     │   - Reports all unit statuses                               │    │
│     │   - Controller evaluates: all required units active?        │    │
│     │   - Updates node.Status: "ready" or "converging"            │    │
│     └─────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Manual Plan Application

Normally plans are applied automatically by the reconciliation loop. For immediate application:

```bash
# Apply plan immediately and watch progress
globular cluster plan apply <node-id> --watch

# Output:
# [QUEUED]   Plan queued for node abc123
# [RUNNING]  Plan dispatched to node-agent
# [RUNNING]  Writing /var/lib/globular/etcd/etcd.yaml
# [RUNNING]  Enabling globular-etcd.service
# [RUNNING]  Starting globular-etcd.service
# [SUCCESS]  Plan applied successfully
```

### Reconciliation Loop

The controller runs a reconciliation loop every 15 seconds:

```
Every 15 seconds:
  For each node with AgentEndpoint:
    1. Compute current plan
    2. Calculate plan hash
    3. If hash differs from LastPlanHash OR status != ready:
       - Dispatch plan to node
       - Record plan hash and timestamp
    4. Handle any dispatch errors
```

---

## Operations & Monitoring

### Watch Operations

Monitor ongoing operations in real-time:

```bash
# Watch all operations
globular cluster watch

# Watch operations for a specific node
globular cluster watch --node-id=<node-id>

# Watch a specific operation
globular cluster watch --op=<operation-id>
```

**Operation Phases:**

| Phase | Description |
|-------|-------------|
| `QUEUED` | Operation waiting to start |
| `RUNNING` | Operation in progress |
| `SUCCEEDED` | Operation completed successfully |
| `FAILED` | Operation failed |

### Node Status

```bash
# List all nodes with status
globular cluster nodes list

# Status values:
# - ready: All required services running
# - converging: Services starting up
# - degraded: Some services not running
```

### Debug Commands

For troubleshooting, use debug commands to bypass the controller:

```bash
# Get node inventory directly from agent
globular debug agent inventory --agent=192.168.1.10:11000

# Apply a plan directly to an agent
globular debug agent apply --plan-file=plan.json --agent=192.168.1.10:11000 --watch

# Watch an operation on a specific agent
globular debug agent watch --op=<op-id> --agent=192.168.1.10:11000
```

### Logs

Check service logs for troubleshooting:

```bash
# Controller logs
journalctl -u globular-clustercontroller -f

# Node agent logs
journalctl -u globular-nodeagent -f

# Individual service logs
journalctl -u globular-etcd -f
journalctl -u globular-minio -f
```

---

## Troubleshooting

### Node Stuck in "converging" Status

**Cause:** Required services haven't started within the grace period (2 minutes).

**Solution:**
```bash
# Check which services are not running
globular cluster plan get <node-id>

# Check service status on the node
ssh <node> "systemctl status globular-etcd globular-minio"

# Check service logs
ssh <node> "journalctl -u globular-etcd -n 100"
```

### Join Request Not Appearing

**Cause:** Node can't reach the controller or token is invalid.

**Solution:**
```bash
# On the new node, check agent logs
journalctl -u globular-nodeagent -f

# Verify connectivity
curl -v http://<controller>:12000

# Create a new token and try again
globular cluster token create --expires=1h
```

### Plan Not Being Applied

**Cause:** Plan hash hasn't changed or node has no agent endpoint.

**Solution:**
```bash
# Force plan application
globular cluster plan apply <node-id> --watch

# Check if agent endpoint is set
globular cluster nodes list
# If AgentEndpoint is empty, the node hasn't reported status yet
```

### etcd Cluster Issues

**Cause:** Peer URLs misconfigured or nodes can't reach each other.

**Solution:**
```bash
# Check etcd cluster health
etcdctl --endpoints=http://localhost:2379 endpoint health

# Check etcd member list
etcdctl --endpoints=http://localhost:2379 member list

# Verify etcd config
cat /var/lib/globular/etcd/etcd.yaml
```

### MinIO Distributed Mode Failures

**Cause:** Not enough nodes with storage profile or nodes unreachable.

**Solution:**
```bash
# Check MinIO config
cat /var/lib/globular/minio/minio.env

# Verify all storage nodes are accessible
for ip in 192.168.1.10 192.168.1.11 192.168.1.12; do
  curl -s http://$ip:9000/minio/health/live && echo "$ip OK"
done
```

---

## Quick Reference

### Common Commands

```bash
# Bootstrap
globular cluster bootstrap --node=localhost:11000 --domain=example.com

# Add nodes
globular cluster token create --expires=24h
globular cluster join --controller=<addr> --join-token=<token>
globular cluster requests approve <id> --profile=core

# Configure
globular cluster network set --domain=example.com --protocol=https --acme --email=admin@example.com
globular cluster nodes profiles set <id> --profile=core --profile=gateway

# Monitor
globular cluster nodes list
globular cluster watch
globular cluster plan get <node-id>
```

### Important Paths

| Path | Description |
|------|-------------|
| `/var/lib/globular/clustercontroller/state.json` | Controller state |
| `/var/lib/globular/nodeagent/state.json` | Agent state |
| `/var/lib/globular/etcd/etcd.yaml` | etcd configuration |
| `/var/lib/globular/minio/minio.env` | MinIO configuration |
| `/var/lib/globular/xds/config.json` | XDS configuration |
| `/var/lib/globular/network.json` | Network configuration |
| `/var/lib/globular/config/tls/` | TLS certificates |

### Default Ports

| Port | Service |
|------|---------|
| 11000 | Node Agent gRPC |
| 12000 | Cluster Controller gRPC |
| 2379 | etcd client |
| 2380 | etcd peer |
| 9000 | MinIO |
| 80/443 | HTTP/HTTPS gateway |

---

## Example: Complete 3-Node Cluster Setup

```bash
# === Node 1 (Control Plane) ===
# Start agent and bootstrap
sudo systemctl start globular-nodeagent
globular cluster bootstrap --node=localhost:11000 --domain=prod.example.com --profile=core

# Configure HTTPS
globular cluster network set \
  --domain=prod.example.com \
  --protocol=https \
  --acme \
  --email=ops@example.com

# Create join token for other nodes
globular cluster token create --expires=1h
# Output: Join Token: abc123

# === Node 2 (Worker) ===
sudo systemctl start globular-nodeagent
globular cluster join --controller=192.168.1.10:12000 --join-token=abc123

# === Node 3 (Gateway) ===
sudo systemctl start globular-nodeagent
globular cluster join --controller=192.168.1.10:12000 --join-token=abc123

# === Back on Node 1: Approve joins ===
globular cluster requests list
globular cluster requests approve <node2-request-id> --profile=compute --profile=storage
globular cluster requests approve <node3-request-id> --profile=gateway

# === Verify cluster ===
globular cluster nodes list
# NODE ID    HOSTNAME    STATUS    PROFILES              LAST SEEN
# abc123     node1       ready     core                  2024-01-15T12:00:00Z
# def456     node2       ready     compute,storage       2024-01-15T12:00:05Z
# ghi789     node3       ready     gateway               2024-01-15T12:00:10Z
```

Your cluster is now running with:
- **Node 1**: etcd, DNS, discovery, event, RBAC, MinIO, file services
- **Node 2**: etcd, DNS, discovery, event, RBAC, MinIO, file services (distributed with node 1)
- **Node 3**: Gateway and Envoy for ingress traffic
