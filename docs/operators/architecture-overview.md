# Globular Architecture Overview

This page describes the architecture of a Globular cluster: how its components are organized, how they communicate, how state flows through the system, and how the platform converges desired state into running services.

## Cluster Topology

A Globular cluster consists of one or more Linux machines (nodes), each running a **Node Agent**. One or more nodes also run the **Cluster Controller** (with leader election for high availability). All nodes share a distributed **etcd** cluster for configuration and state storage.

```
┌───────────────────────────────────────────────────────────────────────┐
│                         Globular Cluster                              │
│                                                                       │
│   ┌───────────────────────────────────────────────────────────┐       │
│   │              Cluster Controller (leader)                  │       │
│   │                                                           │       │
│   │  Desired-state store    Membership manager                │       │
│   │  Workflow dispatcher    Health monitor                    │       │
│   │  Release pipeline       Infrastructure scaler             │       │
│   └────────────────────────────┬──────────────────────────────┘       │
│                                │ gRPC                                 │
│              ┌─────────────────┼─────────────────┐                    │
│              │                 │                 │                    │
│   ┌──────────▼────────┐ ┌───────▼──────────┐ ┌─────▼────────────┐     │
│   │   Node Agent      │ │  Node Agent      │ │  Node Agent      │     │
│   │   (node-1)        │ │  (node-2)        │ │  (node-3)        │     │
│   │                   │ │                  │ │                  │     │
│   │ ┌───────────────┐ │ │ ┌──────────────┐ │ │ ┌──────────────┐ │     │
│   │ │ etcd          │ │ │ │ etcd         │ │ │ │ etcd         │ │     │
│   │ │ controller    │ │ │ │ repository   │ │ │ │ monitoring   │ │     │
│   │ │ gateway       │ │ │ │ auth         │ │ │ │ dns          │ │     │
│   │ │ workflow      │ │ │ │ rbac         │ │ │ │ search       │ │     │
│   │ │ repository    │ │ │ │ event        │ │ │ │ mail         │ │     │
│   │ │ minio         │ │ │ │ minio        │ │ │ │ minio        │ │     │
│   │ └───────────────┘ │ │ └──────────────┘ │ │ └──────────────┘ │     │
│   └───────────────────┘ └──────────────────┘ └──────────────────┘     │
│                                                                       │
│   ┌──────────────────────────────────────────────────────────────-┐   │
│   │                   Shared Infrastructure                       │   │
│   │                                                               │   │
│   │   etcd cluster (2379/2380)  — distributed state + config      │   │
│   │   MinIO cluster (9000)      — object storage for artifacts    │   │
│   │   Envoy Gateway (443/8443)  — TLS termination, xDS routing    │   │
│   │   Prometheus (9090)         — metrics scraping + alerting     │   │
│   │   ScyllaDB                  — high-throughput data (AI memory)│   │
│   └────────────────────────────────────────────────────────────-──┘   │
└───────────────────────────────────────────────────────────────────────┘
```

The specific services running on each node are determined by **profiles**. A profile is a named set of services (for example, `core`, `gateway`, `worker`, `compute`). When a node joins the cluster and is assigned profiles, the platform ensures all services in those profiles are installed and running on that node.

## Control Plane

The control plane is responsible for deciding what the cluster should look like and driving it toward that state. It consists of three components:

### Cluster Controller

The Cluster Controller is the brain of the cluster. It runs as a gRPC service on port 12000 and implements leader election via etcd leases — in a multi-node cluster, only one controller instance is the active leader at any time, and leadership transfers automatically on failure.

The controller maintains several critical data structures in memory and etcd:

**Node Registry**: For each node in the cluster, the controller tracks:
- Node identity (ID, hostname, IP addresses, MAC addresses)
- Assigned profiles
- Last heartbeat timestamp
- Installed package versions and checksums
- Applied services hash (a convergence fingerprint)
- Health status (ready, converging, degraded, unreachable)

**Desired-State Store**: The controller stores the desired state of the cluster in etcd under `/globular/resources/DesiredService/`. Each entry specifies:
- Service name and target version
- Publisher identity
- Platform (e.g., `linux_amd64`)
- Build number
- Which nodes should run it (derived from profiles)

**Release Pipeline**: When desired state changes — or when a node reports a version that doesn't match the desired state — the controller creates **ServiceRelease** or **InfrastructureRelease** objects. These are processed by the release reconciler, which dispatches workflows to bring reality in line with intent.

**Circuit Breakers**: The controller implements two circuit breakers to prevent runaway reconciliation:
- A **workflow gate** that opens when backend RPC failures accumulate, pausing all new workflow dispatch
- A **reconcile breaker** that suspends the periodic reconciliation loop when the Workflow Service is unavailable

These breakers prevent a scenario where a temporary infrastructure problem (etcd slowness, Workflow Service restart) triggers hundreds of simultaneous workflow executions that overwhelm the cluster.

### Workflow Service

The Workflow Service is the execution engine. It receives workflow execution requests from the controller and orchestrates them through a sequence of phases:

1. **DECISION**: Determine what needs to happen (resolve artifact, check dependencies)
2. **FETCH**: Download the package from the repository
3. **INSTALL**: Extract and install the binary on the target node
4. **CONFIGURE**: Write configuration files, update etcd entries
5. **START**: Start or restart the systemd unit
6. **VERIFY**: Run health checks to confirm the service is operational
7. **COMPLETE**: Mark the workflow as finished, update release status

Each phase is executed by an **actor** — a component responsible for carrying out specific types of work:

| Actor | Responsibility |
|-------|---------------|
| `CLUSTER_CONTROLLER` | Desired-state queries, release decisions, cluster-wide validation |
| `NODE_AGENT` | Local execution (install, restart, health check) |
| `REPOSITORY` | Artifact resolution, checksum verification |
| `INSTALLER` | Package extraction, binary placement |
| `RUNTIME` | systemd unit management, process supervision |
| `WORKFLOW_SERVICE` | Step orchestration, retry logic, completion |
| `AI_DIAGNOSER` | Automated failure analysis (optional) |

The Workflow Service tracks every execution as a **WorkflowRun** with a unique `run_id` and a stable `correlation_id` that persists across retries. This means you can trace the complete history of deploying a specific service to a specific node, including every failed attempt and retry.

**Run lifecycle**:
```
PENDING ──→ EXECUTING ──→ SUCCEEDED
                │
                ├──→ FAILED ──→ RETRYING ──→ EXECUTING (retry)
                │
                ├──→ BLOCKED (waiting on dependency)
                │
                ├──→ ROLLED_BACK
                │
                └──→ CANCELED

                SUPERSEDED (newer run took over)
```

**Failure classification**: When a workflow step fails, the failure is classified into one of seven categories:

| Failure Class | Meaning | Typical Response |
|--------------|---------|-----------------|
| `CONFIG` | Configuration error (missing key, invalid value) | Fix configuration, retry |
| `PACKAGE` | Package problem (corrupt archive, missing binary) | Re-publish artifact, retry |
| `DEPENDENCY` | Required service not available | Wait for dependency, auto-retry when unblocked |
| `NETWORK` | Network connectivity issue | Auto-retry with backoff |
| `REPOSITORY` | Repository service unavailable or artifact not found | Auto-retry with backoff |
| `SYSTEMD` | systemd unit failed to start | Check unit logs, retry |
| `VALIDATION` | Pre-flight check failed (checksum mismatch, platform incompatible) | Requires operator intervention |

This classification drives automatic retry behavior. Network and repository failures retry with exponential backoff. Dependency failures wait for the upstream service and retry when unblocked. Validation failures require human intervention.

### Concurrency Control

The Workflow Service limits concurrent workflow executions through a semaphore (default: 3 concurrent workflows). This prevents a large desired-state change (e.g., upgrading 10 services simultaneously) from overloading node agents. Workflows queue behind the semaphore and execute in order.

Additionally, the controller enforces a **5-minute backoff** on releases that have previously failed or been rolled back. This prevents tight retry loops where a fundamentally broken deployment retries continuously.

## Data Plane

The data plane consists of the Node Agents and the services they manage.

### Node Agent

The Node Agent runs on every cluster node on port 11000. It is the **only component** that directly interacts with the operating system. The controller and workflow service never execute system commands — they always work through the node agent.

The Node Agent's responsibilities:

**Workflow Step Execution**: When the Workflow Service dispatches a step to a node agent, the agent executes it locally. This might mean:
- Fetching a package archive from MinIO
- Verifying its SHA256 checksum against the artifact manifest
- Extracting the binary to `/usr/local/bin/`
- Writing a systemd unit file
- Running `systemctl restart <service>`
- Checking the service health endpoint

**Package Tracking**: The Node Agent maintains an `InstalledPackage` record in etcd for every package on the local machine:

```
/globular/nodes/{node_id}/packages/{kind}/{name}
```

Each record contains:
- Package name, version, and build number
- SHA256 checksum of the installed archive
- Installation and last-update timestamps
- Current status (`installed`, `updating`, `failed`, `removing`)
- Operation ID linking to the workflow that last modified it

**Status Reporting**: The Node Agent periodically sends heartbeats to the Cluster Controller via the `ReportNodeStatus` RPC. Each heartbeat includes:
- A map of installed service versions
- The state of all systemd units
- An `AppliedServicesHash` — a fingerprint of the node's current state

The controller compares this hash against its expected state. If they differ, it means the node has drifted and needs reconciliation.

**Service Control**: The Node Agent exposes RPCs for controlling local services:
- `ControlService(action, unit_name)` — start, stop, restart, or query a systemd unit
- `GetServiceLogs(unit_name, lines, since)` — retrieve journalctl output
- `SearchServiceLogs(unit_name, pattern)` — grep through service logs

**Bootstrap**: On the very first node, the Node Agent handles cluster initialization:
1. Creates the local etcd instance
2. Starts the Cluster Controller
3. Applies initial profiles
4. Installs and starts all services defined by those profiles

### Systemd Integration

Globular uses systemd as its process supervisor. Each Globular service runs as a systemd unit with a unit file that specifies:
- The binary path and command-line arguments
- Service dependencies (e.g., `After=etcd.service`)
- Resource limits
- Restart policy (`on-failure` with configurable delay)
- Logging to journald

Systemd provides several capabilities that Globular leverages:
- **Process supervision**: Automatic restart on crash
- **Dependency ordering**: Services start in the correct order
- **Resource isolation**: CPU, memory, and I/O limits per service
- **Logging**: All stdout/stderr captured by journald, queryable via `journalctl`
- **Socket activation**: Services can be started on-demand when connections arrive

## Communication

### gRPC + Protocol Buffers

All inter-service communication in Globular uses gRPC with Protocol Buffers. The proto definitions live in the `/proto/` directory and serve as the authoritative API contract for every service.

When a proto file is modified, the `generateCode.sh` script regenerates:
- Go server and client code (in `<service>/<service>pb/`)
- TypeScript client code (in `typescript/<service>/`)
- RBAC permission descriptors (extracted from `(globular.auth.authz)` annotations)

Every RPC in the system is annotated with authorization metadata:
```protobuf
rpc UpsertDesiredService(DesiredService) returns (DesiredState) {
    option (globular.auth.authz) = { ... };
}
```

A gRPC interceptor chain processes every request:
1. **Authentication interceptor**: Validates the JWT token and extracts the caller identity
2. **RBAC interceptor**: Checks the caller's permissions against the annotated requirements
3. **Audit interceptor**: Logs the RPC call for audit trail

### Service Discovery

Services discover each other through etcd. When a service starts, it registers its endpoint (address + port) in etcd:

```
/globular/services/{service_id}/config
/globular/services/{service_id}/instances/{node_key}
```

When a service needs to call another service, it resolves the target endpoint from etcd. There are **no hardcoded addresses** — not even `localhost`. If a service needs to reach the authentication service, it queries etcd for the current address and port. If etcd cannot provide the endpoint, the service fails with an error rather than falling back to a default.

This design ensures that services work correctly in single-node, multi-node, and split-brain scenarios without code changes.

### Envoy Gateway

External traffic enters the cluster through an **Envoy** gateway, which provides:
- **TLS termination**: All external connections are encrypted
- **xDS-driven routing**: Route configuration is pushed from the Globular xDS server, not static files
- **Load balancing**: Requests are distributed across service instances
- **Health checking**: Envoy removes unhealthy backends from the rotation

The gateway listens on ports 443 (HTTPS) and 8443 (gRPC-Web), making all Globular services accessible to web browsers via gRPC-Web protocol translation.

## State Management

### etcd as the Single Source of Truth

etcd is the backbone of Globular's state management. Every piece of configuration, every cluster decision, and every service endpoint lives in etcd. The key hierarchy is organized by purpose:

**System configuration**:
```
/globular/system/config                          — global system settings
/globular/auth/root                              — root credential
```

**Service registry**:
```
/globular/services/{service_id}/config           — service endpoint (address, port, TLS)
/globular/services/{service_id}/instances/{node}  — per-node instance registration
/globular/services/{service_id}/runtime          — runtime state
```

**Cluster resources** (managed by the Controller):
```
/globular/resources/DesiredService/{name}         — desired-state declarations
/globular/resources/ServiceRelease/{name}         — release workflow tracking
/globular/resources/InfrastructureRelease/{name}  — infrastructure release tracking
/globular/resources/ClusterNetwork/...            — network config (domain, ACME, ports)
```

**Node state** (managed by Node Agents):
```
/globular/nodes/{node_id}/packages/{kind}/{name}  — installed package records
/globular/nodes/{node_id}/status                  — last reported node status
```

**Cluster infrastructure**:
```
/globular/cluster/dns/hosts                       — DNS host mappings
/globular/cluster/scylla/hosts                    — ScyllaDB cluster membership
/globular/cluster/minio/config                    — MinIO configuration
```

### Why Not Configuration Files or Environment Variables?

Traditional approaches to service configuration — config files on disk or environment variables — create consistency problems in distributed systems:

- **Config files** must be synchronized across nodes. If node-3 has a stale config file, it will behave differently from node-1 and node-2, and the problem is invisible until something breaks.
- **Environment variables** are process-scoped and invisible to the rest of the system. When a service reads `DATABASE_HOST` from its environment, no other component can verify what value it's using.

etcd solves both problems:
- **Consistency**: All nodes read from the same distributed store. A change to a configuration key is immediately visible cluster-wide.
- **Observability**: Any operator or automated tool can query etcd to see the current configuration of any service.
- **Watchability**: Components can watch etcd keys and react to changes in real time, without polling or restart.

## Package Lifecycle

A package in Globular progresses through a well-defined lifecycle that spans all four truth layers:

### 1. Build

A developer or CI system builds a service package using the `globular pkg build` command:

```bash
globular pkg build \
  --spec specs/my_service.yaml \
  --root payload/ \
  --version 0.0.3
```

This produces a `.tgz` archive containing:
```
my_service-0.0.3-linux_amd64-1.tgz
├── bin/
│   └── my_service_server          # compiled binary
├── specs/
│   └── my_service_service.yaml    # service metadata (profiles, priorities, dependencies)
└── lib/                           # optional: systemd units, config templates
```

### 2. Publish

The package is published to the Repository service:

```bash
globular pkg publish my_service-0.0.3-linux_amd64-1.tgz
```

The Repository service:
1. Stores the archive in MinIO
2. Computes and records the SHA256 checksum
3. Creates an `ArtifactManifest` with metadata (publisher, version, platform, build number)
4. Records provenance (who published, when, authentication method)
5. Sets the initial publish state to `STAGING`
6. Validates the archive and transitions to `VERIFIED`, then `PUBLISHED`

At this point, the artifact exists in **Layer 1** (Repository) but no node is running it yet.

### 3. Declare Desired State

An operator declares that the service should be running:

```bash
globular services desired set my_service 0.0.3 --publisher core@globular.io
```

The controller writes a `DesiredService` record to etcd. The service now exists in **Layer 2** (Desired Release) with status `PLANNED`.

### 4. Workflow Execution

The controller's release reconciler detects the new desired-state entry and creates a workflow. The Workflow Service orchestrates:

1. **FETCH**: Node Agent downloads the `.tgz` from MinIO to a staging path (`/var/lib/globular/staging/`)
2. **INSTALL**: Verify the SHA256 checksum matches the artifact manifest. Extract the binary to `/usr/local/bin/`
3. **CONFIGURE**: Write the systemd unit file, update etcd with service configuration
4. **START**: Run `systemctl start my_service`
5. **VERIFY**: Check the gRPC health endpoint to confirm the service is responding

If any step fails, the failure is classified and the workflow either retries automatically or waits for operator intervention.

### 5. Convergence

After successful installation:
- **Layer 3** (Installed Observed): The Node Agent writes an `InstalledPackage` record to etcd with the version, checksum, and timestamp
- **Layer 4** (Runtime Health): systemd reports the unit as `active (running)` and the gRPC health check passes

The Node Agent's next heartbeat includes the updated `AppliedServicesHash`. The controller compares this against the desired state and confirms convergence. The service status transitions from `PLANNED` to `INSTALLED`.

## Security Architecture

### mTLS

All gRPC communication between cluster components uses mutual TLS (mTLS). Each node has its own certificate, issued by the cluster's internal CA. Certificate rotation is handled automatically by the Node Agent.

### Authentication

External clients authenticate via JWT tokens. The Authentication Service (port 10101) handles:
- Password-based authentication with token issuance
- Token validation and refresh
- Peer token generation for inter-node communication

### Authorization

Every gRPC RPC is protected by the RBAC system. Proto annotations define what permissions are required:

```protobuf
rpc DeleteBackup(...) returns (...) {
    option (globular.auth.authz) = {
        resource: "/backup/{backup_id}"
        action: "delete"
    };
}
```

The gRPC interceptor chain extracts the caller's identity from the JWT token, queries the RBAC service for the caller's permissions, and either allows or denies the request. This happens transparently — service code does not need to implement authorization checks.

### Security Constraints

The architecture enforces hard boundaries on what components can do:

- The **Cluster Controller** is forbidden from using `os/exec`, `syscall`, or `systemctl`. It cannot directly execute system commands — it must delegate all execution to Node Agents via workflows.
- The **Node Agent** can only use `os/exec` within its `internal/supervisor/` package. System command execution is scoped to a single, auditable location.

These constraints are enforced by build-time checks (`make check-services`).

## High Availability

### Controller HA

The Cluster Controller supports multi-instance deployment with etcd-based leader election. The active leader processes all requests; standby instances forward requests to the leader or wait for leadership to transfer.

If the leader becomes unavailable:
1. The etcd lease expires
2. A standby instance acquires the lease and becomes the new leader
3. The new leader loads state from etcd and resumes operations
4. In-flight workflows continue (the Workflow Service is independent of controller leadership)

The controller implements a **liveness watchdog** that detects zombie leaders — instances that hold the lease but are not actually processing requests. If the watchdog triggers, the controller resigns leadership, allowing a healthy instance to take over.

### etcd Cluster

etcd itself runs as a cluster across multiple nodes (typically 3 or 5 for quorum). The Cluster Controller manages etcd membership expansion — when a new node joins the cluster, the controller automatically adds it to the etcd cluster.

### MinIO Cluster

MinIO runs in erasure-coded mode across cluster nodes. The controller manages pool expansion when new nodes are added. MinIO provides object storage for package artifacts and backups with redundancy across nodes.

## What's Next

- [Convergence Model](convergence-model.md): Deep dive into how Globular detects drift and converges desired state to reality
- [What is Globular](what-is-globular.md): Introduction and comparison with Kubernetes
