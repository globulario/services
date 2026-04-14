# What is Globular

Globular is an open-source microservices platform for building and operating self-hosted distributed applications. It provides a complete runtime environment — service discovery, authentication, RBAC, workflow-driven orchestration, package management, and cluster-wide convergence — without requiring containers, Kubernetes, or any cloud provider.

Globular runs directly on Linux machines as native systemd services. Each node in a Globular cluster runs a **Node Agent** that manages local services, while a central **Cluster Controller** maintains the desired state of the entire cluster and drives convergence through a **Workflow Service**.

## Why Globular Exists

Modern distributed systems typically require teams to adopt container orchestrators like Kubernetes, which bring significant operational complexity. Kubernetes assumes you are running containers, managing pods, and deploying through a container registry. For teams that want distributed application infrastructure without that overhead — for on-premises deployments, edge computing, appliance-style products, or environments where containers are impractical — Kubernetes is often the wrong tool.

Globular was built to solve this problem. It provides the coordination, state management, and lifecycle automation of a modern platform **without requiring containers**. Services are compiled binaries, managed by systemd, orchestrated by workflows, and converged through a layered state model backed by etcd.

### Key design decisions

**Native binaries, not containers.** Services are compiled Go (or other language) binaries distributed as `.tgz` packages. They run under systemd, which provides process supervision, resource limits, logging, and dependency ordering — capabilities that already exist on every Linux system.

**Workflow-driven orchestration, not imperative scripts.** Every cluster operation — bootstrapping a node, deploying a service, expanding infrastructure — is executed through a formal workflow with defined phases, failure classification, automatic retry, and rollback. There are no ad-hoc scripts or manual sequences.

**Declarative desired state, not manual management.** Operators declare what should be running (service name, version, target nodes) and the platform converges reality to match. The system continuously monitors four independent truth layers — repository artifacts, desired releases, installed packages, and runtime health — and reconciles any drift automatically.

**etcd as the single source of truth.** All configuration, cluster membership, desired state, and service discovery data lives in etcd. There are no configuration files to synchronize, no environment variables to manage across nodes, and no hidden state. Every piece of cluster state is queryable, watchable, and consistent.

**gRPC everywhere.** All inter-service communication uses gRPC with Protocol Buffers. This provides strong typing, efficient serialization, bidirectional streaming, and automatic client/server code generation in Go and TypeScript.

## How Globular Compares to Kubernetes

Globular and Kubernetes solve overlapping problems — running distributed services reliably — but they make fundamentally different trade-offs.

| Aspect | Kubernetes | Globular |
|--------|-----------|----------|
| **Unit of deployment** | Container image (OCI) | Native binary package (`.tgz`) |
| **Process supervisor** | kubelet + container runtime | systemd |
| **Scheduling** | Pod scheduling across nodes | Profile-based assignment (operator declares which nodes run which services) |
| **State management** | etcd (internal, not user-facing) | etcd (directly queryable, user-facing API) |
| **Networking** | CNI plugins, Services, Ingress | Envoy gateway with xDS, DNS service, gRPC service discovery |
| **Package registry** | Container registry (Docker Hub, ECR) | Built-in Repository service with artifact lifecycle (staging, verified, published, deprecated, yanked) |
| **Reconciliation** | Controllers + control loops | Workflow-native convergence with classified failure handling |
| **Configuration** | ConfigMaps, Secrets, env vars | etcd keys (single source of truth, no env vars) |
| **Scaling model** | Horizontal pod autoscaling | Node profiles + desired-state declarations |
| **Minimum footprint** | Control plane + worker nodes + container runtime | Single binary per service + etcd + systemd |

Globular is **not** a Kubernetes replacement for teams already running containerized workloads. It is an alternative for teams that want platform-level coordination without the container abstraction.

## Core Components

A Globular cluster consists of the following components:

### Cluster Controller

The Cluster Controller is the central control plane. It runs on one or more nodes (with leader election for high availability) and is responsible for:

- **Cluster membership**: Processing join requests, issuing node tokens, managing node profiles
- **Desired-state management**: Storing and enforcing what services should run, at what version, on which nodes
- **Workflow dispatch**: Converting desired-state drift into workflow executions
- **Health monitoring**: Tracking node heartbeats, detecting stale or degraded nodes
- **Infrastructure expansion**: Managing etcd cluster membership, ScyllaDB gossip topology, and MinIO erasure pools

The Cluster Controller communicates with Node Agents via gRPC and delegates all execution to the Workflow Service. It never directly installs software or modifies system state — it only decides what should happen and tells the workflow engine to make it happen.

**Default port**: 12000

### Node Agent

The Node Agent runs on every node in the cluster. It is the local executor that:

- **Manages systemd units**: Starting, stopping, and monitoring services on the local machine
- **Executes workflow steps**: Receiving step-by-step instructions from the Workflow Service and carrying them out (fetch package, verify checksum, install binary, restart service, run health check)
- **Tracks installed packages**: Maintaining an accurate record of what is installed locally, including version, checksum, and installation timestamp
- **Reports status**: Sending periodic heartbeats to the Cluster Controller with the current state of all local services
- **Handles bootstrap**: Initializing the first node in a cluster, including etcd setup and initial service installation

The Node Agent is the only component that interacts with the operating system directly. It is the boundary between the Globular control plane and the underlying Linux system.

**Default port**: 11000

### Workflow Service

The Workflow Service is the centralized execution engine. Every operation in the cluster — deploying a service, upgrading infrastructure, running a repair — goes through the Workflow Service. It:

- **Orchestrates multi-step operations**: Each workflow consists of ordered phases (decision, fetch, install, configure, start, verify) executed across one or more actors
- **Manages failure and retry**: Failures are classified by type (configuration, package, dependency, network, repository, systemd, validation) with automatic retry and configurable backoff
- **Tracks execution history**: Every workflow run, every step, every retry is recorded with status, timing, and failure details
- **Prevents storms**: Circuit breakers and semaphores prevent runaway reconciliation when the cluster is unhealthy

Workflows are **not** user-defined YAML files (like Kubernetes manifests). They are internal execution plans that the platform creates and executes in response to desired-state changes, drift detection, or operator commands.

### Repository Service

The Repository Service is the package registry. It stores compiled service packages and manages their lifecycle:

- **Artifact storage**: Packages are stored in MinIO object storage with SHA256 checksums
- **Lifecycle management**: Each artifact progresses through defined states — `STAGING` (upload in progress), `VERIFIED` (checksum validated), `PUBLISHED` (available for deployment), and optionally `DEPRECATED`, `YANKED`, or `REVOKED`
- **Provenance tracking**: Every published artifact records who published it, when, and how (authentication method, JWT subject)
- **Version and build management**: Artifacts are identified by publisher, name, version, platform, and build number

### Supporting Services

Beyond the core control plane, Globular includes a catalog of services for building applications:

- **Authentication Service** (port 10101): JWT-based authentication, token validation, password management, peer token generation
- **RBAC Service** (port 10104): Role-based access control with per-resource permissions and subject-based policy queries
- **Event Service** (port 10102): Publish-subscribe event bus for inter-service communication
- **File Service** (port 10103): Distributed file management
- **DNS Service**: Authoritative DNS with zone management, glue records, and wildcard support
- **Discovery Service**: Service discovery and install-plan resolution
- **Log Service**: Centralized log aggregation
- **Monitoring Service**: Prometheus metrics collection and Alertmanager integration
- **Persistence Service**: Database access layer (MongoDB, BadgerDB)
- **Storage Service**: Object/blob storage abstraction
- **Mail Service**: SMTP email integration
- **LDAP Service**: LDAP authentication provider
- **Search Service**: Full-text search (Bleve-based)
- **Media Service**: Audio/video transcoding and management
- **AI Memory Service** (port 10200): Cluster-scoped persistent memory backed by ScyllaDB, used for AI-assisted operations

## The 4-Layer State Model

Globular tracks every package across four independent truth layers. This is the foundational concept that drives the entire convergence model:

| Layer | What it answers | Where it lives | Who owns it |
|-------|----------------|----------------|-------------|
| **1. Artifact** | "Does this version exist and is it valid?" | Repository service (MinIO + etcd) | `globular pkg publish` |
| **2. Desired Release** | "What version should be running?" | Cluster Controller etcd (`/globular/resources/DesiredService/...`) | `globular services desired set` |
| **3. Installed Observed** | "What version is actually installed on this node?" | Node Agent etcd (`/globular/nodes/{id}/packages/...`) | Node Agent (auto-populated) |
| **4. Runtime Health** | "Is the installed service actually running and healthy?" | systemd + gRPC health checks | systemd, Gateway |

Each layer has its own data, its own owner, and its own update mechanism. They are **never collapsed** — the system does not assume that "desired" means "installed" or that "installed" means "healthy." Convergence means aligning all four layers, and drift in any layer triggers investigation and remediation.

The status vocabulary that emerges from comparing these layers:

- **Installed**: desired == installed, service converged
- **Planned**: desired state set, not yet installed
- **Available**: exists in repository, no desired release declared
- **Drifted**: installed version differs from desired version
- **Unmanaged**: installed on a node without a desired-state entry
- **Missing in repo**: desired or installed, but artifact not found in repository
- **Orphaned**: exists in repository, not desired, not installed anywhere

## Getting Started

### Day-0: Bootstrap Your First Node

A Globular cluster starts with a single node. The bootstrap command initializes etcd, starts the Cluster Controller, and installs the base services:

```bash
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --profile core \
  --profile gateway
```

This command:
1. Connects to the Node Agent on the specified node
2. Creates and joins an etcd cluster on that node
3. Starts the Cluster Controller
4. Applies the specified profiles, which determine which services are installed
5. Installs and starts all services defined by those profiles

### Day-1: Add Nodes to the Cluster

To expand the cluster, create a join token on the controller, then use it on each new node:

```bash
# On the controller node: create a join token
globular cluster token create --expires 72h
# Output: abc123xyz...

# On the new node: request to join
globular cluster join \
  --node newnode:11000 \
  --controller controller.mycluster.local:12000 \
  --join-token abc123xyz...

# On the controller: approve the join request
globular cluster requests approve <request-id> \
  --profile worker \
  --meta zone=us-east-1
```

Once approved, the new node receives its identity, joins the etcd cluster, and begins receiving and executing workflows for its assigned profiles.

### Deploy a Service

To deploy or upgrade a service across the cluster:

```bash
# Set the desired version
globular services desired set postgresql 0.0.3 --publisher core@globular.io

# Monitor convergence
globular services desired list

# Check cluster health
globular cluster health
```

The Cluster Controller detects the desired-state change, creates a workflow for each affected node, and the Workflow Service orchestrates the installation through the standard phases: fetch the package from the repository, verify the checksum, install the binary, configure the service, start the systemd unit, and verify health.

### Diagnose and Repair

When something goes wrong, the 4-layer model makes diagnosis straightforward:

```bash
# Dry-run: see what's misaligned without changing anything
globular services repair --dry-run

# Auto-repair: fix misalignments
globular services repair

# Detailed health check
globular cluster health
```

The repair command compares all four layers, identifies which packages are drifted, unmanaged, orphaned, or missing, and generates the minimal set of operations to bring the cluster back into alignment.

## What's Next

- [Architecture Overview](architecture-overview.md): Deep dive into how the components interact, data flows, and the control plane design
- [Convergence Model](convergence-model.md): Detailed explanation of how Globular drives desired state to reality through workflows
- [Writing a Microservice](../developers/writing-a-microservice.md): How to create, package, and deploy your own gRPC microservice on Globular
- [Day-0 / Day-1 / Day-2 Operations](day-0-1-2-operations.md): Backup, restore, upgrades, monitoring, and troubleshooting
