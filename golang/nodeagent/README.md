# Node Agent Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Node Agent runs on each cluster node and is responsible for executing configuration plans, reporting status, and managing local services.

## Overview

The Node Agent is the local orchestration component that receives plans from the Cluster Controller and applies them to the node. It manages service lifecycle, reports health status, and handles cluster membership.

## Features

- **Plan Execution** - Applies configuration plans with retry logic
- **Service Management** - Start/stop/restart systemd units
- **Health Reporting** - Reports node and service status to controller
- **Cluster Joining** - Handles join request workflow
- **Bootstrap Capability** - Can initialize the first cluster node

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            Node Agent                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                       Plan Runner                                │    │
│  │                                                                  │    │
│  │   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐       │    │
│  │   │    Fetch     │──▶│   Validate   │──▶│   Execute    │       │    │
│  │   │    Plan      │   │    Plan      │   │    Steps     │       │    │
│  │   └──────────────┘   └──────────────┘   └──────────────┘       │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                      Action Handlers                             │    │
│  │                                                                  │    │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │    │
│  │   │ Service  │  │   File   │  │ Artifact │  │  Probe   │       │    │
│  │   │ Actions  │  │  Actions │  │ Actions  │  │ Actions  │       │    │
│  │   └──────────┘  └──────────┘  └──────────┘  └──────────┘       │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     Status Reporter                              │    │
│  │                                                                  │    │
│  │   • Node identity (hostname, IPs, OS, arch)                     │    │
│  │   • Service unit status (active, inactive, failed)               │    │
│  │   • Heartbeat to controller                                      │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                    Service Supervisor                            │    │
│  │                                                                  │    │
│  │   systemctl enable/start/stop/restart <unit>                    │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## API Reference

### Cluster Operations

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `BootstrapFirstNode` | Initialize first cluster node | `cluster_domain`, `profiles[]` | `operation_id`, `join_token` |
| `JoinCluster` | Request to join cluster | `controller_endpoint`, `join_token` | `request_id`, `status` |

### Node Management

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `GetInventory` | Report node inventory | - | `identity`, `units[]` |
| `ApplyPlan` | Execute a configuration plan | `NodePlan`, `operation_id` | `operation_id` |
| `WatchOperation` | Stream operation progress | `operation_id` | Stream of events |

## Plan Execution Flow

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ Cluster          │     │   Node Agent     │     │    systemd       │
│ Controller       │     │                  │     │                  │
└────────┬─────────┘     └────────┬─────────┘     └────────┬─────────┘
         │                        │                        │
         │  ApplyPlan(plan)       │                        │
         │───────────────────────▶│                        │
         │                        │                        │
         │                        │ For each config file:  │
         │                        │ ├─ Write file          │
         │                        │ └─ Set permissions     │
         │                        │                        │
         │                        │ For each unit action:  │
         │                        │                        │
         │                        │ systemctl enable       │
         │                        │───────────────────────▶│
         │                        │                        │
         │                        │ systemctl start        │
         │                        │───────────────────────▶│
         │                        │                        │
         │                        │ Wait for active state  │
         │                        │◀───────────────────────│
         │                        │                        │
         │  CompleteOperation     │                        │
         │◀───────────────────────│                        │
```

## Action Types

### Service Actions

| Action | Description | systemd Command |
|--------|-------------|-----------------|
| `enable` | Enable unit at boot | `systemctl enable <unit>` |
| `start` | Start unit | `systemctl start <unit>` |
| `stop` | Stop unit | `systemctl stop <unit>` |
| `restart` | Restart unit | `systemctl restart <unit>` |
| `disable` | Disable unit at boot | `systemctl disable <unit>` |

### File Actions

| Action | Description |
|--------|-------------|
| `file.write` | Write file with content |
| `file.backup` | Create backup before modification |
| `file.restore` | Restore from backup |

### Artifact Actions

| Action | Description |
|--------|-------------|
| `artifact.fetch` | Download artifact from repository |
| `artifact.verify` | Verify checksum |

### Probe Actions

| Action | Description |
|--------|-------------|
| `probe.http` | HTTP health check |
| `probe.exec` | Execute command and check exit code |

## Service Timeouts

Different services have different startup timeouts:

| Service | Start Timeout | Active Timeout |
|---------|---------------|----------------|
| etcd | 60s | 45s |
| DNS | 60s | 45s |
| MinIO | 40s | 30s |
| File | 40s | 30s |
| Media | 40s | 30s |
| Other | 30s | 20s |

## DNS Synchronization

After receiving network configuration from the cluster controller, the node agent automatically synchronizes DNS records.

### What Gets Synchronized

| Record Type | Example | Description |
|-------------|---------|-------------|
| Domain registration | `SetDomains(["example.com"])` | Registers managed domains |
| Gateway A record | `gateway.example.com → 192.168.1.10` | Cluster gateway endpoint |
| Node A record | `node1.example.com → 192.168.1.10` | Node hostname FQDN |
| Gateway AAAA record | `gateway.example.com → 2001:db8::1` | IPv6 (if available) |
| Node AAAA record | `node1.example.com → 2001:db8::1` | IPv6 (if available) |

### DNS Init Config (Authoritative DNS)

For nodes with DNS profiles, the agent also applies authoritative DNS configuration from `/var/lib/globular/dns/dns_init.json`:

| Record Type | Description | Applied By |
|-------------|-------------|------------|
| SOA | Start of Authority record | Primary node only |
| NS | Nameserver records | Primary node only |
| Glue A | A records for NS hosts | All DNS nodes |

### DNS Sync Flow

```
┌───────────────────────┐
│  Network Spec         │
│  Received from        │
│  Controller           │
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐
│  syncDNS()            │
│                       │
│  1. SetDomains        │
│  2. Set gateway A/AAAA│
│  3. Set node A/AAAA   │
│  4. Apply init config │
│     (if exists)       │
└───────────────────────┘
            │
            ▼
┌───────────────────────┐
│  applyDNSInitConfig() │
│                       │
│  If primary node:     │
│  - Set SOA record     │
│  - Set NS records     │
│  - Set all glue A     │
│                       │
│  If secondary node:   │
│  - Set own glue A     │
└───────────────────────┘
```

### DNS Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GLOBULAR_DNS_ENDPOINT` | Override DNS service endpoint | `127.0.0.1:10033` |
| `GLOBULAR_DNS_IPv4` | Override IPv4 for DNS records | Auto-detected |
| `GLOBULAR_DNS_IPv6` | Override IPv6 for DNS records | Auto-detected |
| `GLOBULAR_DNS_IFACE` | Use specific interface for IP detection | - |
| `GLOBULAR_DNS_TOKEN` | Override authentication token | Auto-generated |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NODE_AGENT_PORT` | gRPC listen port | `11000` |
| `NODE_AGENT_STATE_PATH` | State file location | `/var/lib/globular/nodeagent/state.json` |
| `NODE_AGENT_ETCD_MODE` | `managed` or `external` | `managed` |
| `NODE_AGENT_TLS_CERT` | TLS certificate path | - |
| `NODE_AGENT_TLS_KEY` | TLS private key path | - |

### State File

The agent persists its state to track cluster membership:

```json
{
  "node_id": "abc123-def456",
  "cluster_id": "cluster-xyz",
  "controller_endpoint": "192.168.1.10:12000",
  "request_id": "req-789",
  "join_token": "token-abc"
}
```

## Usage Examples

### Bootstrap First Node

```bash
# Using CLI
globular cluster bootstrap \
  --node=localhost:11000 \
  --domain=mycluster.example.com \
  --profile=core

# Using gRPC directly
grpcurl -plaintext -d '{
  "cluster_domain": "mycluster.example.com",
  "profiles": ["core"]
}' localhost:11000 nodeagent.NodeAgentService/BootstrapFirstNode
```

### Join Existing Cluster

```bash
# Using CLI
globular cluster join \
  --controller=192.168.1.10:12000 \
  --join-token=abc123

# Using gRPC directly
grpcurl -plaintext -d '{
  "controller_endpoint": "192.168.1.10:12000",
  "join_token": "abc123"
}' localhost:11000 nodeagent.NodeAgentService/JoinCluster
```

### Get Node Inventory

```bash
grpcurl -plaintext localhost:11000 nodeagent.NodeAgentService/GetInventory
```

### Watch Operation Progress

```bash
grpcurl -plaintext -d '{"operation_id": "op-123"}' \
  localhost:11000 nodeagent.NodeAgentService/WatchOperation
```

## Heartbeat & Status Reporting

The agent reports status to the controller periodically:

```
┌──────────────┐                    ┌──────────────────┐
│  Node Agent  │                    │ Cluster Controller│
└──────┬───────┘                    └────────┬─────────┘
       │                                     │
       │ ReportNodeStatus                    │
       │ {                                   │
       │   node_id: "abc",                   │
       │   identity: {...},                  │
       │   units: [                          │
       │     {name: "etcd", state: "active"},│
       │     {name: "minio", state: "active"}│
       │   ],                                │
       │   agent_endpoint: "192.168.1.10:11000"
       │ }                                   │
       │────────────────────────────────────▶│
       │                                     │
       │                                     │ Update node state
       │                                     │ Evaluate health
       │                                     │
       │ Response                            │
       │◀────────────────────────────────────│
       │                                     │
       │  (repeat every heartbeat interval)  │
```

## Dependencies

- [Cluster Controller](../clustercontroller/README.md) - Receives plans from
- [Repository Service](../repository/README.md) - Downloads artifacts

---

[Back to Services Overview](../README.md)
