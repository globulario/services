# Glossary

Key terms used throughout the Globular documentation.

---

## Cluster & Infrastructure

**Cluster**
: A set of Linux machines running Globular services, coordinated by a Cluster Controller and sharing state through etcd.

**Node**
: A single Linux machine in a Globular cluster, running a Node Agent and zero or more services.

**Cluster Controller**
: The central control plane service (port 12000) responsible for cluster membership, desired-state management, workflow dispatch, and health monitoring. Runs on one or more nodes with leader election.

**Node Agent**
: The local executor on every node (port 11000). Manages systemd units, executes workflow steps, tracks installed packages, and reports status to the Cluster Controller.

**Leader Election**
: The process by which one Cluster Controller instance becomes the active leader. Uses etcd leases. Only the leader dispatches workflows and processes join requests.

**Profile**
: A named set of services assigned to a node (e.g., `core`, `gateway`, `worker`). Profiles determine which packages the convergence model installs on each node.

**Bootstrap**
: The Day-0 operation that initializes the first node in a cluster — creates the etcd cluster, starts the Cluster Controller, and installs base services.

**Join Token**
: A time-limited credential created by the Cluster Controller that authorizes a new node to request cluster membership.

---

## State Model

**4-Layer State Model**
: Globular's foundational concept. Every package is tracked across four independent truth layers: Artifact, Desired Release, Installed Observed, and Runtime Health.

**Artifact**
: A compiled service package (`.tgz`) stored in the Repository service with a SHA256 checksum and lifecycle state.

**Desired Release**
: The version of a service that an operator has declared should be running. Stored in etcd under `/globular/resources/DesiredService/...`.

**Installed Observed**
: The version of a service actually installed on a node, as reported by the Node Agent. Stored in etcd under `/globular/nodes/{id}/packages/...`.

**Runtime Health**
: Whether an installed service is actually running and passing health checks. Determined by systemd status and gRPC health probes.

**Convergence**
: The process of aligning all four state layers so that what is desired matches what is installed and running. Driven by workflows.

**Drift**
: A mismatch between state layers — e.g., the installed version differs from the desired version, or a running service fails its health check.

---

## Status Vocabulary

**Installed**
: Desired version matches installed version; service is converged.

**Planned**
: Desired state has been set, but the service is not yet installed.

**Available**
: Package exists in the repository, but no desired release has been declared.

**Drifted**
: Installed version differs from desired version.

**Unmanaged**
: Service is installed on a node without a corresponding desired-state entry.

**Orphaned**
: Package exists in the repository but is neither desired nor installed anywhere.

**Missing in Repo**
: A desired or installed package whose artifact cannot be found in the repository.

---

## Workflows & Execution

**Workflow**
: A multi-step execution plan that orchestrates a cluster operation (deploy, upgrade, repair). Managed by the Workflow Service with defined phases, failure classification, and retry logic.

**Workflow Service**
: The centralized execution engine. Every cluster operation flows through it — service deployment, infrastructure upgrades, AI remediation.

**Phase**
: A step within a workflow (e.g., `decision`, `fetch`, `install`, `configure`, `start`, `verify`). Phases execute sequentially.

**Failure Classification**
: Each workflow failure is categorized by type — configuration, package, dependency, network, repository, systemd, or validation — enabling targeted retry and remediation.

**Circuit Breaker**
: A safety mechanism that stops workflow dispatch when the cluster is unhealthy, preventing cascading failures from runaway reconciliation.

**Semaphore**
: Concurrency control that limits how many workflows can execute simultaneously, preventing resource exhaustion.

---

## Services & Packages

**Service**
: A compiled gRPC binary that runs as a systemd unit. Each service has a proto contract, a server implementation, and a configuration backed by etcd.

**Package**
: A `.tgz` archive containing a compiled service binary, a spec file, and metadata. Packages are published to the Repository service and distributed to nodes by workflows.

**Spec File**
: A YAML file inside a package that declares the service's name, version, publisher, dependencies, ports, and systemd configuration.

**Repository Service**
: The package registry that stores artifacts in MinIO with lifecycle management (Staging → Verified → Published → Deprecated → Yanked).

**Publisher**
: The identity (e.g., `core@globular.io`) that published a package. Tracked for provenance.

---

## Networking & Security

**Gateway**
: An Envoy-based reverse proxy that handles external traffic, gRPC-Web translation, TLS termination, and routing to backend services.

**xDS**
: Envoy's discovery service protocol. Globular uses xDS to dynamically configure the gateway with service endpoints, routing rules, and TLS certificates.

**gRPC-Web**
: A protocol that enables browser-based clients to call gRPC services through the Envoy gateway. Used by the admin UI and web applications.

**PKI (Public Key Infrastructure)**
: The certificate system that secures all inter-service communication. Globular runs its own CA for internal mTLS and supports Let's Encrypt for external certificates.

**mTLS (Mutual TLS)**
: Both client and server present certificates during the TLS handshake. Used for all inter-node and inter-service gRPC communication.

**JWT (JSON Web Token)**
: Used for user authentication. The Authentication service issues JWTs that are validated by gRPC interceptors on every service call.

**RBAC (Role-Based Access Control)**
: Permission model where subjects (users, services, AI agents) are granted roles that define which gRPC methods and resources they can access.

**Interceptor**
: gRPC middleware that runs before every service call. Globular uses interceptors for authentication (JWT validation), authorization (RBAC checks), and audit logging.

---

## Data & Storage

**etcd**
: A distributed key-value store used as Globular's single source of truth for all cluster state — configuration, membership, desired state, and service discovery.

**ScyllaDB**
: A high-performance distributed database used by the AI Memory service for persistent knowledge storage. Not part of the core control plane.

**MinIO**
: S3-compatible object storage used by the Repository service (package artifacts) and the Backup service (backup snapshots).

**BadgerDB**
: An embedded key-value store used by some services for local persistent data (e.g., Prometheus WAL, local caches).

---

## AI Layer

**AI Watcher**
: Observes cluster events and creates incidents when configurable rules match (e.g., service crash, health degradation).

**AI Executor**
: Diagnoses incidents using Claude (Anthropic API) or deterministic rules, and executes approved remediation actions through workflows.

**AI Memory**
: Persistent knowledge store (ScyllaDB-backed) where the AI records diagnoses, patterns, decisions, and feedback for future reference.

**AI Router**
: Computes dynamic routing policies (endpoint weights, circuit breaker settings) based on real-time telemetry.

**MCP (Model Context Protocol)**
: A JSON-RPC 2.0 interface that exposes 65+ diagnostic tools for external AI agents (like Claude Code) to inspect and interact with the cluster.

**Incident**
: An event or pattern detected by the AI Watcher that requires diagnosis. Each incident gets a unique ID and flows through the AI Executor pipeline.

**Tier 1 (AUTO_REMEDIATE)**
: AI actions that execute automatically without human approval (e.g., restart a crashed service).

**Tier 2 (REQUIRE_APPROVAL)**
: AI actions that require operator approval before execution (e.g., drain an endpoint, open a circuit breaker).

**Tier 3 (OBSERVE)**
: AI observes and diagnoses but takes no action. Diagnosis is recorded for operator review.

---

## Operations

**Day-0**
: Initial cluster setup — bootstrap the first node, establish the control plane.

**Day-1**
: Cluster expansion — add nodes, assign profiles, deploy applications.

**Day-2**
: Ongoing operations — upgrades, monitoring, backup, troubleshooting, capacity planning.

**Cluster Doctor**
: An invariant-checking system that verifies cluster health (etcd quorum, certificate expiry, service status, configuration consistency) and proposes remediation.

**Repair**
: The process of comparing all four state layers and generating operations to fix misalignments (drifted, unmanaged, or orphaned packages).

**Keepalived**
: A Linux service that provides Virtual IP (VIP) failover for high availability. Used to ensure the gateway IP floats to a healthy node if the primary fails.

**VIP (Virtual IP)**
: A floating IP address managed by Keepalived that always points to the active gateway node.
