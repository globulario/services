# Building from Source

This page documents how to build Globular from source code. This is the current installation method — there are no pre-built binary releases yet (see [Release Strategy](#release-strategy) for the plan).

## Repository Structure

Globular is split across multiple repositories under `github.com/globulario`:

| Repository | Purpose | Language |
|-----------|---------|----------|
| **services** | All 28+ microservices, CLI, MCP server, proto definitions | Go, TypeScript |
| **Globular** | Gateway (Envoy control), xDS server | Go |
| **packages** | Infrastructure specs (etcd, Prometheus, MinIO, etc.) | Shell, YAML |
| **globular-installer** | Installation binary, Day-0 scripts | Go, Shell |

## Prerequisites

### Build Machine

- **Go 1.24+**
- **Node.js** (for TypeScript protobuf generation)
- **protoc** (Protocol Buffers compiler)
- **protoc-gen-go**, **protoc-gen-go-grpc** (Go protobuf plugins)
- **Git**
- **Make**
- **Linux amd64** (cross-compilation possible but not documented)

### Target Machine (Where Globular Will Run)

- **Linux amd64** (Ubuntu 22.04+, Debian 12+, RHEL 9+, or similar)
- **systemd**
- **4 GB RAM minimum** (8 GB recommended for all services)
- **20 GB disk** (50 GB recommended for packages + data)

## Build Steps

### Step 1: Clone All Repositories

```bash
mkdir -p ~/globulario && cd ~/globulario

git clone https://github.com/globulario/services.git
git clone https://github.com/globulario/Globular.git
git clone https://github.com/globulario/packages.git
git clone https://github.com/globulario/globular-installer.git
```

### Step 2: Generate Code and Build Services

```bash
cd services

# Generate protobuf code (Go + TypeScript) and build all service binaries
# This also builds the gateway and xDS from the Globular repo
bash generateCode.sh
```

**What this does:**
1. Compiles 47 `.proto` files → Go server/client code + TypeScript gRPC-Web clients
2. Extracts RBAC permission annotations → `cluster-roles.generated.json`
3. Builds 33 service binaries → `golang/tools/stage/linux-amd64/usr/local/bin/`
4. Builds gateway and xDS from `../Globular/`
5. Builds the CLI tool (`globularcli`) and MCP server

**Duration**: 3-10 minutes depending on hardware.

### Step 3: Build All Packages

```bash
# Still in the services/ directory
bash build-all-packages.sh
```

**What this does:**
1. Copies service binaries from the staging area
2. Downloads infrastructure binaries (etcd, Prometheus, Envoy, MinIO, etc.) if not cached
3. Builds 22 infrastructure packages (`.tgz` archives)
4. Generates service specs and builds 28 service packages
5. Copies all packages to `globular-installer/internal/assets/packages/`

**Duration**: 5-15 minutes (first run downloads ~500 MB of infrastructure binaries; subsequent runs use cache).

**Output**: `generated/packages/` contains all `.tgz` packages ready for installation.

### Step 4: Build the Installer

```bash
cd ../globular-installer

# Sync specs from packages and services
make sync-specs

# Build the installer binary
make build
```

**Output**: `globular-installer` binary with all packages embedded.

### Step 5: Install on Target Machine

Copy the installer and Day-0 script to the target machine:

```bash
# On the build machine
scp globular-installer target-machine:/tmp/
scp scripts/install-day0.sh target-machine:/tmp/

# On the target machine
sudo bash /tmp/install-day0.sh
```

**What `install-day0.sh` does** (9 phases):
1. Generate TLS certificates (internal CA + node certs)
2. Enable 30-minute bootstrap security window
3. Install etcd + MinIO (storage layer)
4. Install persistence service (data layer)
5. Install xDS, Envoy, gateway, node agent, controller, doctor
6. Install RBAC, auth, DNS, repository (control plane)
7. Install monitoring, AI, workflow, backup (operational services)
8. Install file, search, media, title (application services)
9. Install CLI tools (globularcli, etcdctl, mc, rclone, restic)

**Duration**: 5-10 minutes.

### Step 6: Verify

```bash
globular cluster health
# CLUSTER STATUS: HEALTHY
# NODES: 1/1 healthy

globular services desired list
# All services INSTALLED
```

## Quick Reference

```bash
# Full build from scratch (one-liner)
cd services && bash generateCode.sh && bash build-all-packages.sh && \
cd ../globular-installer && make sync-specs && make build

# Install
sudo bash scripts/install-day0.sh
```

## What's Built

After a successful build:

```
services/
├── generated/
│   ├── packages/              # All .tgz packages (infrastructure + services)
│   ├── specs/                 # Generated service specs (YAML)
│   ├── policy/                # RBAC policies
│   └── payload/               # Intermediate payload directories
├── golang/tools/stage/
│   └── linux-amd64/usr/local/bin/  # All compiled binaries

globular-installer/
├── globular-installer         # Installer binary (with embedded packages)
├── internal/assets/packages/  # Staged packages for Day-0
└── scripts/
    ├── install-day0.sh        # Main installation script
    └── uninstall-day0.sh      # Complete removal script
```

## Release Strategy

Currently, Globular is built from source. The plan is to move to **GitHub Releases** for a professional distribution experience:

### Release Workflow

The release workflow is defined in `.github/workflows/release.yml`. It triggers on Git tags:

```bash
# Create a release
git tag v0.1.0
git push origin v0.1.0
```

This triggers GitHub Actions to:
1. Checkout all 4 repositories (services, Globular, packages, globular-installer)
2. Generate code and build all service binaries
3. Download infrastructure binaries and build all packages
4. Build the installer binary
5. Build the MkDocs documentation site
6. Create a release tarball with checksums
7. Upload to GitHub Releases

### For End Users

```bash
# Download the latest release
VERSION="0.1.0"
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz"

# Verify checksum
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "globular-${VERSION}-linux-amd64.tar.gz.sha256"

# Extract and install
tar xzf "globular-${VERSION}-linux-amd64.tar.gz"
cd "globular-${VERSION}-linux-amd64"
sudo bash install.sh

# Bootstrap
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --profile core --profile gateway

# Deploy documentation site
bash deploy-docs.sh --domain docs.mycluster.local
```

### Version Management

All packages use a unified version derived from the Git tag:
- `v0.1.0` → all service packages version `0.1.0`
- Infrastructure packages keep their upstream version (etcd 3.5.14, etc.)

## Uninstalling

To completely remove Globular from a machine:

```bash
cd globular-installer
sudo bash scripts/uninstall-day0.sh
```

This stops all services, removes binaries, configs, and state files in reverse dependency order.

## Known Limitations

- **No pre-built releases yet** — must build from source
- **All service versions are 0.0.1** — no semantic versioning in place
- **No signed packages** — `.tgz` files have SHA256 checksums but no cryptographic signatures
- **Build requires all repos** — 4 repositories must be cloned side-by-side
- **Linux amd64 only** — arm64 builds are possible but not automated
- **compute_server not in build manifest** — code exists but is not compiled or packaged (Phase 2+ feature)

## What's Next

- [Installation (Day-0)](operators/installation.md) — What happens during `install-day0.sh`
- [Day-0/1/2 Operations](operators/day-0-1-2-operations.md) — Complete lifecycle guide
- [Getting Started](../getting-started.md) — From zero to running cluster
