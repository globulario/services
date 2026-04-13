# Installation

## Prerequisites

- Linux (Ubuntu 22.04+ or equivalent)
- Go 1.24+ (for building from source)
- Network connectivity between cluster nodes
- Ports 443 (mesh), 2379-2380 (etcd), 10000-20000 (services), 11000 (node agent), 12000 (controller)

## Single-Node Quick Start

```bash
# 1. Build all services
cd golang && go build ./...

# 2. Run the bootstrap script
./install-day0.sh

# 3. Verify cluster health
globular cluster health
```

## Multi-Node Cluster

### First node (bootstrap)

```bash
# Initialize the first node
globular cluster bootstrap

# Verify etcd and controller are running
globular cluster health
```

### Additional nodes (join)

```bash
# On the first node, create a join token
globular cluster token create

# On the joining node
globular cluster join --token <TOKEN> --controller <CONTROLLER_ADDR>
```

### Verify convergence

```bash
# List all nodes
globular cluster nodes list

# Check desired vs installed services
globular services list-desired

# Verify service health
globular cluster health
```

## Build from Source

```bash
# Clone the repository
git clone https://github.com/globulario/services.git
cd services

# Build all Go services
cd golang && go build ./...

# Generate protobuf code (after modifying .proto files)
./generateCode.sh

# Build all packages
./build-all-packages.sh
```

## Package a Service

```bash
# Build a specific service package
globular pkg build --spec generated/specs/<service>_service.yaml \
  --root /tmp/payload --version 0.0.1 --build-number 1

# Publish to repository
globular pkg publish --file /tmp/out/<service>_0.0.1_linux_amd64.tgz

# Set desired state
globular services desired set <service> 0.0.1 --build-number 1
```

## Directory Layout

| Path | Purpose |
|------|---------|
| `/usr/lib/globular/bin/` | Service binaries |
| `/var/lib/globular/` | State directory (configs, data, PKI) |
| `/var/lib/globular/pki/` | TLS certificates |
| `/var/lib/globular/services/` | Service config files |
| `/var/lib/globular/workflows/` | Workflow definition YAMLs |
| `/etc/systemd/system/globular-*.service` | Systemd unit files |

## Default Ports

| Service | Port |
|---------|------|
| Repository | 10000 |
| Authentication | 10010 |
| RBAC | 10014 |
| DNS | 10006 |
| Event | 10002 |
| Workflow | 10004 |
| Node Agent | 11000 |
| Cluster Controller | 12000 |
| Cluster Doctor | 12100 |
| AI Memory | 10200 |
| Compute | 10300 |
| Gateway (Envoy) | 443 |
