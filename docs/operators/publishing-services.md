# Publishing Services

This page covers the complete workflow for building, packaging, and publishing services to the Globular Repository. It explains how to create packages from source, how the repository manages artifact lifecycle, and how published packages become available for deployment.

## Why Publish

Publishing is how software enters the Globular ecosystem. Before a service can be deployed through the desired-state model, it must exist as a published artifact in the Repository service. Publishing provides:

- **Integrity**: Every artifact has a SHA256 checksum verified before deployment
- **Provenance**: Every publish records who published, when, and how they authenticated
- **Lifecycle management**: Artifacts progress through defined states (STAGING → VERIFIED → PUBLISHED)
- **Versioning**: Multiple versions coexist, with clear deprecation and yanking semantics
- **Distribution**: Published artifacts are stored in MinIO and available to all cluster nodes

## Building a Package

### From Source Code

The typical workflow for building a Globular service package:

```bash
# 1. Generate protobuf code (if proto files changed)
./generateCode.sh

# 2. Build the Go binary
cd golang
go build -o ../packages/bin/my_service_server ./my_service/my_service_server
cd ..

# 3. Prepare the payload directory
mkdir -p packages/payload/my_service/bin
mkdir -p packages/payload/my_service/specs
cp packages/bin/my_service_server packages/payload/my_service/bin/
cp specs/my_service_service.yaml packages/payload/my_service/specs/
```

### Creating the Spec File

Every package needs a spec file that describes the service:

```yaml
# specs/my_service_service.yaml
name: my_service
version: 0.0.3
publisher: core@globular.io
platform: linux_amd64
kind: SERVICE
profiles:
  - core
  - custom-profile
priority: 50
dependencies:
  - etcd
  - authentication
```

**Fields**:
- `name`: Service identifier (must match binary name convention: `<name>_server`)
- `version`: Semantic version
- `publisher`: Publisher identity (must match authenticated user's identity or organization)
- `platform`: Target platform (`linux_amd64`, `linux_arm64`)
- `kind`: `SERVICE` (gRPC service), `APPLICATION` (web app), or `INFRASTRUCTURE` (third-party component)
- `profiles`: Which profiles include this service (determines which nodes receive it)
- `priority`: Installation order — lower numbers install first (etcd=10, controller=20, auth=30, custom services=50+)
- `dependencies`: Services that must be running before this one starts

### Building the Package

```bash
globular pkg build \
  --spec specs/my_service_service.yaml \
  --root packages/payload/my_service/ \
  --version 0.0.3 \
  --build-number 1
```

Output:
```
Package built: globular-my_service-0.0.3-linux_amd64-1.tgz
  Size: 12.4 MB
  Checksum: sha256:a1b2c3d4e5f6...
```

The resulting archive structure:
```
globular-my_service-0.0.3-linux_amd64-1.tgz
├── bin/
│   └── my_service_server
├── specs/
│   └── my_service_service.yaml
└── lib/                    # Optional
    └── my_service.service  # systemd unit override
```

### Build Numbering

The `--build-number` flag differentiates rebuilds of the same version. Use cases:

- Build 1: Initial release of 0.0.3
- Build 2: Recompiled with updated dependency, same version number
- Build 3: Compiled with compiler optimization flags

The combination of version + build number is unique. When setting desired state, you can optionally specify the build number to target a specific build:

```bash
globular services desired set my_service 0.0.3 --build-number 2
```

If no build number is specified, the latest build of the specified version is used.

## Publishing to the Repository

### Publish Command

```bash
globular pkg publish globular-my_service-0.0.3-linux_amd64-1.tgz
```

What happens internally:

1. **Authentication**: The CLI authenticates with the Repository service using the operator's JWT token
2. **Upload**: The `.tgz` archive is streamed to the Repository via gRPC
3. **Storage**: The Repository stores the archive in MinIO object storage
4. **Checksum**: SHA256 is computed and recorded in the artifact manifest
5. **Manifest creation**: An `ArtifactManifest` is created in etcd:
   ```
   ArtifactManifest {
     ref: { publisher_id: "core@globular.io", name: "my_service", version: "0.0.3", platform: "linux_amd64", kind: SERVICE }
     checksum: "sha256:a1b2c3d4..."
     build_number: 1
     profiles: ["core", "custom-profile"]
     publish_state: STAGING
     provenance: { published_by: "admin@mycluster.local", published_at: 1712937600, auth_method: "jwt" }
   }
   ```
6. **Validation**: The repository validates the archive structure (required directories, spec file presence)
7. **State transition**: STAGING → VERIFIED → PUBLISHED

After publishing, the artifact is available for deployment via `services desired set`.

### Publisher Identity Validation

The Repository validates that the publisher identity in the package spec matches the authenticated user:
- The JWT token's PrincipalID or organization must match the `publisher` field in the spec
- This prevents impersonation — you cannot publish a package claiming to be from `core@globular.io` unless your credentials prove it

### Querying Published Artifacts

```bash
# List all versions of a service
globular pkg info my_service

# Output:
# NAME          VERSION  BUILD  PLATFORM     STATE       PUBLISHED AT
# my_service    0.0.3    1      linux_amd64  PUBLISHED   2025-04-12 10:30:00
# my_service    0.0.2    1      linux_amd64  DEPRECATED  2025-04-01 14:22:00
# my_service    0.0.1    1      linux_amd64  YANKED      2025-03-15 09:10:00
```

## Artifact Lifecycle Management

All lifecycle commands use the format `publisher/name` for the first argument and accept `--platform` (defaults to current platform), `--build-number` (defaults to 0 = all builds), and `--reason` for audit.

### Deprecating a Version

When a newer version supersedes an older one:

```bash
globular pkg deprecate core@globular.io/my_service 0.0.2
globular pkg deprecate core@globular.io/my_service 0.0.2 --reason "superseded by 0.0.3"
```

`DEPRECATED` means: still downloadable and installable by explicit pin, but **skipped by the latest resolver**. The node-agent emits a warning when installing a deprecated artifact. Nodes already running this version are unaffected.

Undo with:
```bash
globular pkg undeprecate core@globular.io/my_service 0.0.2
```

### Yanking a Version

When a version has a critical bug that must not be deployed anywhere new:

```bash
globular pkg yank core@globular.io/my_service 0.0.1 --reason "memory leak in request handler"
```

`YANKED` means: hidden from discovery, downloads blocked for non-owners. New desired-state writes targeting this version are rejected. Existing installations are unaffected — nodes already running this version continue to work.

Undo with:
```bash
globular pkg unyank core@globular.io/my_service 0.0.1
```

### Quarantining a Version (admin only)

For security incidents where an artifact must be held pending investigation:

```bash
globular pkg quarantine core@globular.io/my_service 0.0.1 --reason "CVE-2026-1234 under review"
```

`QUARANTINED` behaves like `YANKED` but requires admin privileges to both set and lift. Only an admin can lift quarantine:

```bash
globular pkg unquarantine core@globular.io/my_service 0.0.1
```

### Revoking a Version

For confirmed security vulnerabilities or compliance failures. Terminal — no recovery:

```bash
globular pkg revoke core@globular.io/my_service 0.0.1 --reason "supply chain compromise confirmed"
```

`REVOKED` is permanent. The manifest is retained for audit; the binary stays in MinIO but is unreachable. Active installations of a REVOKED artifact should be replaced immediately — the cluster doctor will flag them.

### Targeting a Specific Platform or Build

All state commands accept `--platform` and `--build-number` to target a specific artifact precisely:

```bash
# Yank only the arm64 build of a version
globular pkg yank core@globular.io/my_service 0.0.1 --platform linux_arm64

# Deprecate a specific build iteration
globular pkg deprecate core@globular.io/my_service 0.0.3 --build-number 2
```

### Garbage Collection

Old artifacts that are no longer reachable (outside the retention window and not referenced by desired or installed state) can be archived:

```bash
# Preview what would be archived
globular repository cleanup --dry-run

# Archive unreachable artifacts (soft-delete — binary stays in MinIO)
globular repository cleanup
```

See [Repository Overview](repository-overview.md) for details on the reachability engine and what GC will and will not touch.

## Building All Packages

The `build-all-packages.sh` script builds the entire Globular platform from source:

```bash
./build-all-packages.sh
```

This executes five stages:

**Stage 1**: Download infrastructure binaries (etcd, Prometheus, Alertmanager, MinIO, Envoy, restic, rclone, node-exporter, sidekick, ffmpeg). Versions are defined in spec metadata.

**Stage 2**: Package infrastructure components into `.tgz` archives.

**Stage 3**: Build Go service binaries via `golang/build/build-services.sh`, then package each one via `pkggen.sh`.

**Stage 4**: If a Repository service is running, publish all packages to it.

**Stage 5**: Copy packages to the installer assets directory for offline installation.

## CI/CD Integration

### Automated Publishing

In a CI/CD pipeline, publishing follows the same commands:

```bash
# CI job: build and publish
./generateCode.sh
cd golang && go build ./my_service/my_service_server && cd ..

globular pkg build \
  --spec specs/my_service_service.yaml \
  --root payload/ \
  --version $(git describe --tags) \
  --build-number $CI_BUILD_NUMBER

globular pkg publish globular-my_service-*.tgz

# Optionally, trigger deployment
globular services desired set my_service $(git describe --tags)
```

### RBAC for CI

The CI system needs a service account with the `globular-publisher` role:

```bash
# Create a service account for CI
globular auth create-account --username ci-publisher --type application

# Assign publisher role
globular rbac bind --subject ci-publisher --role globular-publisher
```

The publisher role grants permission to upload artifacts to the repository but does not grant permission to modify desired state or manage cluster membership.

## Practical Scenarios

### Scenario 1: Publishing a Bug Fix

A developer fixes a bug in the monitoring service:

```bash
# Fix the bug in code
# ...

# Build and publish
cd golang && go build -o ../packages/bin/monitoring_server ./monitoring/monitoring_server && cd ..
globular pkg build --spec specs/monitoring_service.yaml --root packages/payload/monitoring/ --version 0.0.6 --build-number 1
globular pkg publish globular-monitoring-0.0.6-linux_amd64-1.tgz

# Deploy
globular services desired set monitoring 0.0.6

# Deprecate old version
globular pkg deprecate monitoring 0.0.5
```

### Scenario 2: Rolling Back to a Previous Version

The new version has issues. Roll back:

```bash
# Check available versions
globular pkg info monitoring
# 0.0.6 PUBLISHED
# 0.0.5 DEPRECATED
# 0.0.4 DEPRECATED

# Roll back to 0.0.5 (still downloadable despite being deprecated)
globular services desired set monitoring 0.0.5

# Yank the bad version to prevent future deployment
globular pkg yank monitoring 0.0.6
```

### Scenario 3: Building Infrastructure Packages

To update an infrastructure component (e.g., etcd):

```bash
# Download new etcd binary
wget https://github.com/etcd-io/etcd/releases/download/v3.5.15/etcd-v3.5.15-linux-amd64.tar.gz
tar xzf etcd-v3.5.15-linux-amd64.tar.gz
cp etcd-v3.5.15-linux-amd64/etcd packages/payload/etcd/bin/
cp etcd-v3.5.15-linux-amd64/etcdctl packages/payload/etcd/bin/

# Update spec version
# Edit specs/etcd_service.yaml → version: 3.5.15

# Build and publish
globular pkg build --spec specs/etcd_service.yaml --root packages/payload/etcd/ --version 3.5.15
globular pkg publish globular-etcd-3.5.15-linux_amd64-1.tgz

# Deploy (infrastructure upgrades are serialized per-node for safety)
globular services desired set etcd 3.5.15
```

## What's Next

- [Updating the Cluster](updating-the-cluster.md): Upgrade cluster services and infrastructure
- [Debugging Failures](debugging-failures.md): Diagnose deployment problems
