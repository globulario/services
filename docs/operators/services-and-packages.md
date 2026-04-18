# Services and Packages

Globular services are native Linux binaries distributed as versioned packages. This page explains how services are structured, how packages are built, how the repository manages artifact lifecycle, and how packages move through the four truth layers from source code to running service.

## What is a Service

A Globular service is a gRPC server that implements one or more RPC methods defined in a Protocol Buffer (`.proto`) file. Each service runs as a systemd unit on one or more cluster nodes. Services communicate with each other exclusively through gRPC, discover each other through etcd, and are managed by the Globular control plane.

Every service follows the same internal structure:

```
service_name/
├── service_namepb/              # Generated protobuf code (Go)
│   ├── service_name.pb.go       # Message types
│   └── service_name_grpc.pb.go  # gRPC client/server interfaces
├── service_name_client/         # Client library
│   └── client.go                # Connection helpers, typed client
└── service_name_server/         # Server implementation
    ├── server.go                # Main server struct + gRPC registration
    ├── config.go                # Configuration struct, validation, persistence
    ├── handlers.go              # Business logic (RPC implementations)
    └── *_test.go                # Unit and integration tests
```

### Server Implementation Pattern

Every Globular service implements two interfaces from the `globular_service` package:

**Service interface** — provides metadata about the service:
```go
type Service interface {
    GetId() string
    SetId(string)
    GetName() string
    GetPort() int
    GetState() string         // "running", "stopped", "starting"
    SetState(string)
    GetDomain() string
    GetAddress() string
    GetProtocol() string      // "grpc"
    GetVersion() string
    GetPublisherId() string
    // ... additional getters/setters
}
```

**LifecycleService interface** — provides lifecycle management:
```go
type LifecycleService interface {
    GetId() string
    GetName() string
    GetPort() int
    GetState() string
    SetState(string)
    StartService() error      // Initialize and begin serving
    StopService() error       // Graceful shutdown
    GetGrpcServer() *grpc.Server
}
```

### Shared Primitives

Services use shared helpers from the `globular_service` package to handle common concerns:

**CLI Helpers** (`HandleInformationalFlags`):
Every service binary supports standard flags:
```bash
my_service_server --version     # Print version and exit
my_service_server --help        # Print usage and exit
my_service_server --describe    # Print service descriptor (JSON)
my_service_server --health      # Check health and exit
```

**Lifecycle Manager** (`NewLifecycleManager`):
Manages the full startup/shutdown sequence:
1. Parse CLI arguments (service_id, config_path)
2. Load configuration from etcd (or local file during bootstrap)
3. Allocate a port if needed (`--port 0` for dynamic allocation)
4. Create gRPC server with TLS, interceptors, and health checks
5. Load RBAC permission mappings (`policy.LoadAndRegisterPermissions()`)
6. Register the service in etcd for discovery
7. Handle graceful shutdown on SIGTERM

**Config Helpers**:
- `SaveConfigToFile()` / `LoadConfigFromFile()` — JSON serialization of service config
- `ValidateCommonFields()` — validates name, port, protocol, version
- Configuration values come from etcd — never environment variables, never hardcoded

### Port Assignment

Each service has a default port (e.g., Authentication: 10101, RBAC: 10104, Node Agent: 11000), but ports are ultimately stored in and read from etcd. The shared primitives handle port binding:

- Bind to `0.0.0.0` (all interfaces), never `127.0.0.1`
- If the configured port is in use, attempt up to 5 fallback ports
- Register the actual bound port in etcd for discovery

### gRPC Server Setup

All services share the same gRPC server configuration:
- **TLS**: Mandatory. Server certificate and CA loaded from cluster configuration.
- **Interceptors**: Authentication (JWT/mTLS), RBAC (permission enforcement), audit logging
- **Health check**: gRPC Health API registered automatically
- **Prometheus metrics**: Request count, latency histograms
- **Keepalive**: 30-second ping, 5-second timeout, 2-minute max idle
- **Max concurrent streams**: 1,000,000

## What is a Package

A package is a distributable archive (`.tgz`) that contains everything needed to install and run a service on a node. Packages are the unit of deployment in Globular — you don't deploy binaries directly, you publish packages to the repository and let the workflow system install them.

### Package Structure

```
globular-<name>-<version>-<platform>-<build>.tgz
├── bin/
│   └── <name>_server              # Compiled service binary
├── specs/
│   └── <name>_service.yaml        # Service metadata
└── lib/                           # Optional
    ├── <name>.service              # systemd unit file
    └── config/                     # Default configuration templates
```

### Service Spec File

The spec file (`specs/<name>_service.yaml`) contains metadata that the platform uses to manage the service:

```yaml
name: postgresql
version: 0.0.3
publisher: core@globular.io
platform: linux_amd64
kind: SERVICE                      # SERVICE, APPLICATION, or INFRASTRUCTURE
profiles:
  - core                           # Which profiles include this service
  - database
priority: 50                       # Installation order (lower = earlier)
dependencies:
  - etcd                           # Must be installed before this service
  - authentication                 # Required for RBAC enforcement
```

### Package Kinds

Globular distinguishes three kinds of packages:

**SERVICE** — A gRPC microservice that runs as a systemd unit. Examples: authentication, rbac, repository, monitoring. Services have ports, health endpoints, and participate in gRPC service discovery.

**APPLICATION** — A web application served through the gateway. Examples: blog, admin UI. Applications don't have their own ports — they're served through the Envoy gateway via gRPC-Web.

**INFRASTRUCTURE** — A third-party component managed by Globular. Examples: etcd, MinIO, Prometheus, Alertmanager, Envoy, restic. Infrastructure packages have their own process management but use Globular's packaging and workflow system for deployment and upgrades.

## Building Packages

### Building a Single Service Package

To build a package from source:

```bash
# Step 1: Generate protobuf code
./generateCode.sh

# Step 2: Build the Go binary
cd golang && go build -o ../packages/bin/my_service_server ./my_service/my_service_server

# Step 3: Create the package
globular pkg build \
  --spec specs/my_service_service.yaml \
  --root packages/payload/my_service/ \
  --version 0.0.3 \
  --build-number 1
```

The `pkg build` command:
1. Reads the spec file for metadata (name, version, platform, kind, profiles)
2. Collects all files from the root directory (binary, configs, unit files)
3. Creates a `.tgz` archive with the standard directory structure
4. Names the archive: `globular-my_service-0.0.3-linux_amd64-1.tgz`

### Building All Packages

The `build-all-packages.sh` script builds the entire platform:

```bash
./build-all-packages.sh
```

This script executes five stages:

**Stage 1: Infrastructure binaries**
Downloads or builds third-party binaries (etcd, Prometheus, Alertmanager, node-exporter, MinIO, sidekick, restic, rclone, ffmpeg, Envoy). Version numbers are read from spec metadata.

**Stage 2: Infrastructure packages**
Creates `.tgz` packages for each infrastructure component using `globular pkg build`.

**Stage 3: Service packages**
Builds Go service binaries (`build-services.sh`), then packages each one. Uses `pkggen.sh` to invoke `globular pkg build` for each service.

**Stage 4: Repository publish**
If a repository service is running, publishes all packages to it. Otherwise, stores them locally.

**Stage 5: Installer sync**
Copies packages to the installer assets directory for offline installation.

### Code Generation

The `generateCode.sh` script generates code from proto files:

```bash
./generateCode.sh
```

This script:
1. Runs `protoc` on all `.proto` files, generating Go server/client code and TypeScript gRPC-Web client code (47 targets)
2. Runs `authzgen` to extract RBAC permission annotations from proto files
3. Generates RBAC policy descriptor files (`cluster-roles.generated.json`)
4. Builds service binaries
5. Builds the CLI tool (`globularcli`)
6. Builds the MCP server

## The Repository Service

The Repository service is Globular's package registry. It stores artifacts in MinIO object storage and maintains manifests in etcd.

### Artifact Manifest

Each published artifact has an `ArtifactManifest` that records everything the platform needs to manage it:

```
ArtifactManifest {
  ref {
    publisher_id: "core@globular.io"
    name: "postgresql"
    version: "0.0.3"
    platform: "linux_amd64"
    kind: SERVICE
  }
  build_id: "019d986b-3632-7297-..."       # Repository-issued UUIDv7 — SOLE authoritative identity
  checksum: "sha256:a1b2c3d4e5f6..."     # SHA256 of the .tgz archive
  build_number: 1                          # Display-only monotonic counter (NOT used in convergence)
  profiles: ["core", "database"]           # Which profiles use this package
  install_mode: "repository"               # "repository" or "day0_join"
  publish_state: PUBLISHED                 # Lifecycle state
  provenance {
    published_by: "admin@mycluster.local"  # JWT subject of publisher
    published_at: 1712937600               # Unix timestamp
    auth_method: "jwt"                     # How the publisher authenticated
  }
}
```

### Publish State Lifecycle

Every artifact progresses through a defined lifecycle:

```
STAGING ──→ VERIFIED ──→ PUBLISHED ──→ DEPRECATED
                                    ──→ YANKED
                                    ──→ REVOKED
```

**STAGING**: The archive has been uploaded but not yet validated. The repository has received the bytes but hasn't confirmed their integrity.

**VERIFIED**: The SHA256 checksum has been computed and recorded. The artifact is structurally valid and discoverable by the platform.

**PUBLISHED**: The artifact is fully available for deployment. Workflows can fetch and install it.

**DEPRECATED**: The artifact has been superseded by a newer version. It remains downloadable for rollback scenarios, but new deployments should use the newer version.

**YANKED**: The artifact has been removed from discovery. Existing installations are unaffected, but new workflows cannot fetch it. Used when a version has a critical bug that shouldn't be deployed anywhere new.

**REVOKED**: The artifact is permanently removed. Active installations should be replaced. Used for security vulnerabilities or compliance violations.

### Publishing a Package

```bash
globular pkg publish my_service-0.0.3-linux_amd64-1.tgz
```

What happens internally:
1. CLI connects to the Repository service
2. Uploads the `.tgz` archive via streaming gRPC
3. Repository stores the archive in MinIO
4. Repository computes SHA256 checksum
5. Repository creates the `ArtifactManifest` with the publisher's identity (from JWT token)
6. Repository records provenance (who, when, how)
7. Publish state transitions: STAGING → VERIFIED → PUBLISHED
8. Manifest is written to etcd for discovery

### Querying Artifacts

```bash
# List all artifacts for a service
globular pkg info postgresql

# Output:
# NAME         VERSION  BUILD  PLATFORM     STATE       PUBLISHED
# postgresql   0.0.3    1      linux_amd64  PUBLISHED   2025-04-12 10:30:00
# postgresql   0.0.2    3      linux_amd64  DEPRECATED  2025-04-01 14:22:00
# postgresql   0.0.1    1      linux_amd64  YANKED      2025-03-15 09:10:00
```

## Package Lifecycle Through the 4 Layers

A package moves through all four truth layers during its lifecycle:

### Layer 1: Artifact (Repository)

After `pkg publish`, the package exists in the repository. It has a manifest, a checksum, and provenance. No node is running it yet.

```bash
globular pkg info my_service
# Shows: version 0.0.3, PUBLISHED, checksum sha256:a1b2c3...
```

### Layer 2: Desired Release (Controller)

An operator declares that the service should be running:

```bash
globular services desired set my_service 0.0.3 --publisher core@globular.io
```

The controller creates a `DesiredService` record in etcd. The package now has intent but no reality.

### Layer 3: Installed Observed (Node Agent)

After the workflow successfully installs the package, the Node Agent writes an `InstalledPackage` record:

```
/globular/nodes/node-abc123/packages/SERVICE/my_service
{
  "name": "my_service",
  "version": "0.0.3",
  "checksum": "sha256:a1b2c3...",
  "build_id": "019d986b-3632-7297-...",
  "status": "installed",
  "installed_unix": 1712937600,
  "build_number": 1,
  "operation_id": "wf-run-xyz789"
}
```

### Layer 4: Runtime Health (systemd)

The service is running as a systemd unit. systemd reports it as `active (running)` and the gRPC health endpoint returns healthy.

```bash
# On the node:
systemctl status my_service
# Active: active (running) since Sat 2025-04-12 10:35:00 UTC
```

### Cross-Layer Comparison

The `get-drift-report` command compares all four layers:

```bash
globular cluster get-drift-report
```

This identifies packages in each status:

| Status | Meaning | Which Layers |
|--------|---------|-------------|
| **Installed** | Converged, all layers aligned | L1 ✓ L2 ✓ L3 ✓ L4 ✓ |
| **Planned** | Desired but not installed | L1 ✓ L2 ✓ L3 ✗ L4 ✗ |
| **Available** | In repo, no desired | L1 ✓ L2 ✗ L3 ✗ L4 ✗ |
| **Drifted** | Installed ≠ desired | L1 ✓ L2 ✓ L3 ✓(wrong ver) L4 ? |
| **Unmanaged** | Installed, no desired | L1 ? L2 ✗ L3 ✓ L4 ✓ |
| **Missing in repo** | Desired/installed, not in repo | L1 ✗ L2 ✓ L3 ? L4 ? |
| **Orphaned** | In repo only | L1 ✓ L2 ✗ L3 ✗ L4 ✗ |

## Profiles

Profiles determine which services run on which nodes. When a node joins the cluster and is assigned profiles, all services associated with those profiles are installed.

Common profiles:
- **core**: Essential services (controller, authentication, rbac, event, discovery)
- **gateway**: Envoy gateway, xDS server, DNS
- **worker**: Compute services, application hosting
- **database**: Persistence services (PostgreSQL, ScyllaDB)
- **monitoring**: Prometheus, Alertmanager, node-exporter
- **storage**: MinIO, backup manager

Services specify their profile membership in the spec file:
```yaml
profiles:
  - core
  - gateway
```

A service can belong to multiple profiles. When a node is assigned `--profile core --profile gateway`, it receives the union of all services from both profiles.

### Profile Assignment

Profiles are assigned when a node joins the cluster:

```bash
# During bootstrap
globular cluster bootstrap --profile core --profile gateway

# During join approval
globular cluster requests approve <request-id> --profile worker --profile monitoring
```

Profiles can be updated after join:
```bash
globular cluster nodes set-profiles <node-id> --profile core --profile gateway --profile monitoring
```

Changing profiles triggers workflow dispatch for any services that need to be added or removed.

## Service Dependencies and Priority

Services declare their dependencies in the spec file. The workflow system respects these dependencies:

```yaml
dependencies:
  - etcd          # Must be running before this service starts
  - authentication
priority: 50      # Lower number = installed earlier
```

When multiple services are being installed on a node:
1. Services are ordered by priority (lower first)
2. Within the same priority, dependencies are resolved
3. If Service B depends on Service A, B's workflow blocks until A completes
4. Circular dependencies are detected and reported as errors

## Practical Scenarios

### Scenario 1: Publishing a New Service Version

A developer fixes a bug in the monitoring service and wants to deploy it:

```bash
# Build the new version
cd golang && go build -o ../packages/bin/monitoring_server ./monitoring/monitoring_server

# Package it
globular pkg build \
  --spec specs/monitoring_service.yaml \
  --root packages/payload/monitoring/ \
  --version 0.0.6 \
  --build-number 1

# Publish to repository
globular pkg publish globular-monitoring-0.0.6-linux_amd64-1.tgz

# Set desired state to trigger deployment
globular services desired set monitoring 0.0.6

# Watch convergence
globular services desired list
```

### Scenario 2: Rolling Back a Bad Deployment

A new version of the authentication service is causing failures:

```bash
# Check current state
globular services desired list
# Shows: authentication 0.0.5 DEGRADED (2/3 nodes FAILED)

# Roll back to previous version
globular services desired set authentication 0.0.4

# The controller creates workflows to install 0.0.4 on all nodes
# Previously converged nodes (running 0.0.5) will be downgraded
# Failed nodes will get the working version

globular services desired list
# Shows: authentication 0.0.4 APPLYING...
# Eventually: authentication 0.0.4 AVAILABLE
```

### Scenario 3: Seeding After Bootstrap

After bootstrapping a cluster, many services are installed by the bootstrap process but may not have explicit desired-state entries. Seeding creates entries for everything:

```bash
# See what's installed but unmanaged
globular cluster get-drift-report
# Shows: etcd UNMANAGED, authentication UNMANAGED, ...

# Import all installed services into desired state
globular services seed

# Verify
globular services desired list
# All installed services now have desired-state entries
```

## What's Next

- [Security](operators/security.md): PKI, RBAC, mTLS, and the authentication model
- [Installation](operators/installation.md): Day-0 bootstrap walkthrough
