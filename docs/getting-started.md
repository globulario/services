# Getting Started with Globular

This guide takes you from a bare Linux machine to a running Globular cluster.

## What You Will Achieve

By the end of this guide, you will have:
- A single-node Globular cluster running on your machine
- Core services operational (authentication, RBAC, repository, DNS, gateway)
- A service deployed through the desired-state model
- The ability to monitor cluster health and service status

## Prerequisites

- A Linux machine (amd64) with at least 4 GB RAM and 20 GB disk
- systemd available (standard on Ubuntu, Debian, RHEL, Fedora)
- Root or sudo access
- Network connectivity (the node needs a routable IP address)

## Installation

There are two ways to install Globular: from a release tarball (recommended) or from source.

### Option A: From Release (Recommended)

Download the latest release from GitHub:

```bash
# Download latest release (replace VERSION with the actual version, e.g., 0.1.0)
VERSION="0.1.0"
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz"

# Verify checksum
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "globular-${VERSION}-linux-amd64.tar.gz.sha256"

# Extract and install
tar xzf "globular-${VERSION}-linux-amd64.tar.gz"
cd "globular-${VERSION}-linux-amd64"
sudo bash install.sh
```

> **Note**: If no GitHub release is available yet, see [Building from Source](operators/building-from-source.md) for how to build and install from the Git repositories.

### Option B: From Source

Requires Go 1.24+, protoc, and the four Globular repositories:

```bash
# Clone
mkdir -p ~/globulario && cd ~/globulario
git clone https://github.com/globulario/services.git
git clone https://github.com/globulario/Globular.git
git clone https://github.com/globulario/packages.git
git clone https://github.com/globulario/globular-installer.git

# Build everything
cd services
bash generateCode.sh
bash build-all-packages.sh

# Build installer
cd ../globular-installer
make sync-specs && make build

# Install
sudo bash scripts/install-day0.sh
```

See [Building from Source](operators/building-from-source.md) for the full build guide with prerequisites.

## Step 1 — Verify Installation

After installation, all services are running under systemd:

```bash
# Check that the cluster is healthy
globular cluster health
```

Expected output:
```
CLUSTER STATUS: HEALTHY
NODES: 1/1 healthy

NODE         STATUS   LAST SEEN    SERVICES
node-abc123  healthy  2s ago       20+ running
```

Check the desired state:

```bash
globular services desired list
```

Expected output:
```
SERVICE            VERSION    NODES   STATUS
etcd               3.5.14     1/1     INSTALLED
authentication     0.0.1      1/1     INSTALLED
rbac               0.0.1      1/1     INSTALLED
controller         0.0.1      1/1     INSTALLED
gateway            0.0.1      1/1     INSTALLED
repository         0.0.1      1/1     INSTALLED
...
```

All services should show `INSTALLED`. If any show `APPLYING`, wait a minute — the convergence model is still working.

## Step 2 — Set the Admin Password

```bash
globular auth set-password --username admin --password YourStr0ngP@ssword!
```

Authenticate:

```bash
globular auth login --username admin --password YourStr0ngP@ssword!
# Token saved
```

## Step 3 — Deploy Your First Service

Deploy the echo service — a simple test service that echoes back whatever you send:

```bash
# Check it's in the repository
globular pkg info echo
# Shows: echo 0.0.1 PUBLISHED

# Set the desired state
globular services desired set echo 0.0.1

# Watch deployment
globular services desired list
# echo  0.0.1  0/1  APPLYING...

# Wait a few seconds...
globular services desired list
# echo  0.0.1  1/1  INSTALLED
```

## Step 4 — Verify the Deployment

```bash
# Cluster shows the new service
globular cluster health

# Check workflow history
globular workflow list --service echo
# Shows: SUCCEEDED with trigger DESIRED_DRIFT

# No drift
globular services repair --dry-run
# All INSTALLED
```

## Step 5 — Access the Gateway

The Envoy gateway is serving on ports 443 and 8443:

```bash
# HTTPS
curl -sk https://localhost:443
# Returns: HTML page (200 OK)

# Certificate is self-signed (internal CA) — this is expected
# For public access with Let's Encrypt, see DNS and PKI docs
```

## What Just Happened

Here's the architecture running on your machine:

```
Your Machine
├── etcd (2379)              — Cluster state, configuration
├── Cluster Controller (12000) — Desired state management
├── Node Agent (11000)       — Local service executor
├── Workflow Service (13000) — Orchestration engine
├── Envoy Gateway (443/8443) — External traffic entry
├── xDS Server (8081)        — Gateway configuration
├── Authentication (10101)   — JWT tokens
├── RBAC (10104)             — Permission enforcement
├── Repository + MinIO (9000) — Package storage
├── DNS (10006/53)           — Service resolution
├── Monitoring + Prometheus   — Metrics collection
├── AI Memory (10200)        — Persistent AI knowledge
└── Echo Service             — Your deployed service
```

Every service:
- Runs as a systemd unit
- Communicates via gRPC with mTLS
- Registers in etcd for discovery
- Is protected by the RBAC interceptor chain
- Is managed by the convergence model

## What's Next

| Goal | Guide |
|------|-------|
| **Add nodes for HA** | [Adding Nodes](operators/adding-nodes.md) |
| **External access with Let's Encrypt** | [DNS and PKI](operators/dns-and-pki.md) |
| **VIP failover with keepalived** | [Keepalived and Ingress](operators/keepalived-and-ingress.md) |
| **Set up backups** | [Backup and Restore](operators/backup-and-restore.md) |
| **Full Day-0/1/2 lifecycle** | [Day-0/1/2 Operations](operators/day-0-1-2-operations.md) |
| **Build your own service** | [Writing a Microservice](developers/writing-a-microservice.md) |
| **Develop without a cluster** | [Local-First Development](developers/local-first.md) |
| **Understand the architecture** | [Architecture Overview](operators/architecture-overview.md) |
| **Monitor the cluster** | [Observability](operators/observability.md) |
| **All ports and firewall rules** | [Ports Reference](operators/ports-reference.md) |
