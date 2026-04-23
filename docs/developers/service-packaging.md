# Service Packaging

This page covers how to package a Globular service for distribution and deployment. It explains the package format, spec files, build process, and how packages integrate with the 4-layer state model.

## Why Packaging

A package is not just a delivery mechanism. It is the artifact that the 4-layer state model reasons about.

The Repository stores packages and tracks their state (PUBLISHED, DEPRECATED, CORRUPTED). The Controller reads the Repository to decide what should be running. The Node Agent downloads and installs packages. systemd runs the result. Each layer has exactly one job, and the package is the handoff point between them. Without a package in the Repository, the convergence model has nothing to converge toward — there is no desired state that can be expressed.

**Why `.tgz` and not Docker images, `.deb`, or `.rpm`?**

Globular runs native binaries under systemd — there is no container runtime, no package manager, no dependency resolver. A `.tgz` is the simplest possible archive format: unpack it, copy the binary, write the unit file, done. Docker images carry a container runtime dependency and an OCI layer model that adds nothing here. `.deb`/`.rpm` packages carry system-level package management semantics (conffiles, pre/post install scripts, apt/dnf integration) that conflict with Globular's convergence model — you can't have both apt and the cluster controller trying to manage the same service.

**Why is the spec file separate from the binary?**

The spec file declares how the service fits into the cluster: which profiles it belongs to, what priority it installs at, what services it depends on. These are deployment-time decisions, not compile-time decisions. The same binary could deploy at priority 60 in one cluster and priority 40 in another. Embedding this in the binary would mean recompiling to change deployment topology.

**Why do build numbers exist separately from versions?**

A version (`0.0.1`) identifies what the code does. A build number identifies a specific compiled artifact. Sometimes you need to rebuild the same source version — a dependency update, a compiler flag change, a certificate embedded at build time. The build number lets you distinguish `0.0.1 build 1` (original) from `0.0.1 build 2` (rebuilt with updated deps) without a version bump that would imply the API changed. The convergence model tracks both: the controller expresses desired version, but the node agent installs a specific build.

## Package Format

A Globular package is a gzip-compressed tar archive (`.tgz`) with a standardized directory structure:

```
globular-<name>-<version>-<platform>-<build>.tgz
├── bin/
│   └── <name>_server              # Compiled binary (required)
├── specs/
│   └── <name>_service.yaml        # Service metadata (required)
└── lib/                           # Optional extras
    ├── <name>.service              # systemd unit override
    ├── config/                     # Default configuration templates
    └── migrations/                 # Database migrations (if applicable)
```

### Naming Convention

The archive name follows a strict format:
```
globular-{name}-{version}-{platform}-{build_number}.tgz
```

- **name**: Service identifier (lowercase, underscores for word separation)
- **version**: Semantic version (e.g., `0.0.1`, `1.2.3`)
- **platform**: Target platform (e.g., `linux_amd64`, `linux_arm64`)
- **build_number**: Integer differentiating rebuilds of the same version

Examples:
```
globular-authentication-0.0.1-linux_amd64-1.tgz
globular-monitoring-0.0.6-linux_amd64-3.tgz
globular-etcd-3.5.15-linux_amd64-1.tgz
```

## Spec File

The spec file is the most important metadata in a package. It tells the platform how to manage the service.

### Complete Spec File Reference

```yaml
# specs/inventory_service.yaml

# Service identity
name: inventory                    # REQUIRED — must match binary: <name>_server
version: 0.0.1                     # REQUIRED — semantic version
publisher: myteam@example.com      # REQUIRED — must match authenticated publisher identity
platform: linux_amd64              # REQUIRED — target platform

# Package classification
kind: SERVICE                      # REQUIRED — SERVICE, APPLICATION, or INFRASTRUCTURE

# Profile assignment
profiles:                          # REQUIRED — which profiles include this service
  - custom
  - asset-management

# Installation ordering
priority: 60                       # OPTIONAL — lower = installed earlier (default: 100)
                                   # etcd=10, controller=20, auth=30, custom=50+

# Dependencies
dependencies:                      # OPTIONAL — services that must be running first
  - etcd
  - authentication
  - rbac

# Port
port: 10300                        # OPTIONAL — default port (overridden by etcd in production)

# Install mode
install_mode: repository           # OPTIONAL — "repository" (default) or "day0_join"
```

### Package Kinds

**SERVICE** (most common):
- A gRPC microservice running as a systemd unit
- Has its own port and health endpoint
- Participates in service discovery via etcd
- Example: authentication, monitoring, inventory

**APPLICATION**:
- A web application served through the Envoy gateway
- Does not have its own port — served via gRPC-Web through the gateway
- Has associated roles and groups for access control
- Example: admin dashboard, blog frontend

**INFRASTRUCTURE**:
- A third-party component managed by Globular
- Uses Globular's packaging but has its own process management
- May have special upgrade procedures
- Example: etcd, MinIO, Prometheus, Alertmanager, Envoy

### Priority Ordering

Priority exists because declared `dependencies` alone are not sufficient for ordering. Dependencies say "A must run before B," but they only work between services that know about each other. Infrastructure needs to be up before anything else regardless of whether any service explicitly declares a dependency on it. Priority encodes that structural knowledge — it is the global ordering that makes the dependency graph tractable.

Priority determines installation order when multiple services are deployed simultaneously:

| Priority Range | Category | Examples |
|---------------|----------|---------|
| 1-10 | Infrastructure | etcd (10) |
| 11-20 | Core control plane | controller (20) |
| 21-30 | Security | authentication (25), rbac (28) |
| 31-40 | Platform services | event (35), repository (38) |
| 41-50 | Shared services | discovery (42), dns (45) |
| 51-100 | Custom services | inventory (60), custom apps (80) |
| 100+ | Non-critical | dev tools, optional addons |

Within the same priority, dependencies are resolved. If Service B (priority 60) depends on Service A (priority 60), A is installed first.

### Dependencies

Dependencies declare which services must be running before this service starts. During workflow execution:

1. The workflow system checks if all dependencies are healthy
2. If a dependency is not running, the workflow enters `BLOCKED` state
3. When the dependency becomes available, a new workflow is dispatched with `DEPENDENCY_UNBLOCKED` trigger

Common dependency patterns:
```yaml
# Most services need etcd (configuration) and auth (token validation)
dependencies:
  - etcd
  - authentication

# A search service might need persistence
dependencies:
  - etcd
  - authentication
  - persistence

# Infrastructure services typically have no dependencies
dependencies: []
```

## Building a Package

### Manual Build

```bash
# 1. Prepare the payload directory
mkdir -p packages/payload/inventory/bin
mkdir -p packages/payload/inventory/specs

# 2. Build the binary
cd golang
go build -o ../packages/payload/inventory/bin/inventory_server ./inventory/inventory_server
cd ..

# 3. Copy the spec file
cp specs/inventory_service.yaml packages/payload/inventory/specs/

# 4. Build the package
globular pkg build \
  --spec specs/inventory_service.yaml \
  --root packages/payload/inventory/ \
  --version 0.0.1 \
  --build-number 1
```

### Automated Build

For CI/CD, use the build scripts:

```bash
# Build all services from source
./generateCode.sh                    # Generate protobuf code
golang/build/build-services.sh       # Compile all Go services
./build-all-packages.sh              # Package everything
```

### Cross-Compilation

To build for a different platform:

```bash
GOOS=linux GOARCH=arm64 go build -o packages/payload/inventory/bin/inventory_server ./inventory/inventory_server

globular pkg build \
  --spec specs/inventory_service.yaml \
  --root packages/payload/inventory/ \
  --version 0.0.1 \
  --platform linux_arm64
```

## systemd Unit Overrides

By default, the platform generates a standard systemd unit file for each service. If you need custom settings, include a unit file in the `lib/` directory:

```ini
# lib/inventory.service
[Unit]
Description=Globular Inventory Service
After=etcd.service authentication.service
Requires=etcd.service

[Service]
Type=simple
ExecStart=/usr/local/bin/inventory_server
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
MemoryMax=512M
CPUQuota=200%

[Install]
WantedBy=multi-user.target
```

If no custom unit file is provided, the platform creates one with sensible defaults:
- `Restart=on-failure` with a 5-second delay
- Standard resource limits
- Logging to journald

## Publishing

After building, publish to the Repository:

```bash
globular pkg publish globular-inventory-0.0.1-linux_amd64-1.tgz
```

The repository validates:
- Archive structure (bin/ and specs/ directories present)
- Spec file is parseable and contains required fields
- Publisher identity matches the authenticated user
- Checksum is computed and stored

See [Publishing Services](../operators/publishing-services.md) for full details.

## Versioning Strategy

### Semantic Versioning

Use semantic versioning (MAJOR.MINOR.PATCH):
- **MAJOR**: Breaking proto changes (renamed RPCs, removed fields)
- **MINOR**: New RPCs or fields (backward compatible)
- **PATCH**: Bug fixes, performance improvements

### Build Numbers

Build numbers differentiate rebuilds of the same version:
- Build 1: Initial compile
- Build 2: Dependency update (same source code)
- Build 3: Compiler flag change

When setting desired state without a build number, the platform uses the latest build:
```bash
globular services desired set inventory 0.0.1              # latest build
globular services desired set inventory 0.0.1 --build-number 2  # specific build
```

## Practical Scenarios

### Scenario 1: First Package Build

Building your first service package:

```bash
# 1. Create the spec
cat > specs/inventory_service.yaml << 'EOF'
name: inventory
version: 0.0.1
publisher: dev@example.com
platform: linux_amd64
kind: SERVICE
profiles:
  - custom
priority: 60
dependencies:
  - etcd
  - authentication
EOF

# 2. Build the binary
cd golang && go build -o ../packages/payload/inventory/bin/inventory_server ./inventory/inventory_server && cd ..

# 3. Package
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.1

# Output: globular-inventory-0.0.1-linux_amd64-1.tgz (8.2 MB)

# 4. Verify the archive contents
tar tzf globular-inventory-0.0.1-linux_amd64-1.tgz
# bin/inventory_server
# specs/inventory_service.yaml

# 5. Publish and deploy
globular pkg publish globular-inventory-0.0.1-linux_amd64-1.tgz
globular services desired set inventory 0.0.1
```

### Scenario 2: Updating a Service

Publishing a bug fix:

```bash
# Fix the bug in code, then:
cd golang && go build -o ../packages/payload/inventory/bin/inventory_server ./inventory/inventory_server && cd ..

# Bump version in spec
# Edit specs/inventory_service.yaml → version: 0.0.2

# Package and publish
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.2
globular pkg publish globular-inventory-0.0.2-linux_amd64-1.tgz

# Deploy
globular services desired set inventory 0.0.2

# Deprecate old version
globular pkg deprecate inventory 0.0.1
```

### Scenario 3: Infrastructure Package

Packaging a third-party component (e.g., a custom database):

```bash
cat > specs/mydb_service.yaml << 'EOF'
name: mydb
version: 4.2.0
publisher: infra@example.com
platform: linux_amd64
kind: INFRASTRUCTURE
profiles:
  - database
priority: 15
dependencies: []
EOF

# Include the binary and a custom systemd unit
mkdir -p packages/payload/mydb/{bin,specs,lib}
cp /path/to/mydb-binary packages/payload/mydb/bin/mydb_server
cp specs/mydb_service.yaml packages/payload/mydb/specs/

cat > packages/payload/mydb/lib/mydb.service << 'EOF'
[Unit]
Description=MyDB Database Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/mydb_server --config /etc/mydb/config.yaml
Restart=on-failure
LimitNOFILE=1048576
MemoryMax=4G

[Install]
WantedBy=multi-user.target
EOF

globular pkg build --spec specs/mydb_service.yaml --root packages/payload/mydb/ --version 4.2.0
```

## What's Next

- [Publishing to Repository](publishing-to-repository.md): Full publishing workflow
- [RBAC Integration](rbac-integration.md): Authorization for your service
