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
# Check https://github.com/globulario/services/releases for the latest version
VERSION="1.0.17"
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz"

# Verify checksum
curl -LO "https://github.com/globulario/services/releases/download/v${VERSION}/globular-${VERSION}-linux-amd64.tar.gz.sha256"
sha256sum -c "globular-${VERSION}-linux-amd64.tar.gz.sha256"

# Extract and install
tar xzf "globular-${VERSION}-linux-amd64.tar.gz"
cd "globular-${VERSION}-linux-amd64"
sudo bash install.sh
```

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
authentication     1.0.17     1/1     INSTALLED
rbac               1.0.17     1/1     INSTALLED
controller         1.0.17     1/1     INSTALLED
gateway            1.0.17     1/1     INSTALLED
repository         1.0.17     1/1     INSTALLED
...
```

All services should show `INSTALLED`. If any show `APPLYING`, wait a minute — the convergence model is still working.

## Step 2 — Set the Admin Password

```bash
globular auth root-passwd --password YourStr0ngP@ssword!
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
globular cluster get-drift-report
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

## What usually breaks first

A fresh install almost never goes clean end-to-end on the first try. That is not a failure — it is the normal turbulence of a system with many moving parts starting in sequence. Here is what actually tends to go wrong, in the order it usually happens.

### The controller starts but nothing gets installed

The most common Day-0 state. `globular cluster health` shows the node healthy, but `globular services desired list` is empty or everything is stuck at APPLYING.

The controller depends on a working etcd and a valid PKI before it seeds desired state. Check these first:

```bash
sudo systemctl status etcd
# If failed: check /var/lib/etcd/ disk space and journalctl -u etcd

ls /var/lib/globular/pki/ca.crt
# If missing: the installer did not complete successfully
```

Then check if the controller actually ran its bootstrap seeding:

```bash
sudo journalctl -u globular-cluster-controller --no-pager -n 100 | grep -E "seed|bootstrap|error|failed"
```

If the controller logged "seeding desired state" but workflows aren't running, check the workflow service:

```bash
sudo systemctl status globular-workflow
sudo journalctl -u globular-workflow --no-pager -n 50
```

### Workflows are dispatched but stay stuck at FETCH

Services are in APPLYING and have been for several minutes. Workflows exist but don't progress.

FETCH is the first real workflow step — it downloads the package binary from MinIO. If MinIO isn't running or accessible, every workflow stalls here silently.

```bash
sudo systemctl status minio
# If failed or not running, that is your problem

# If MinIO is running but uploads never completed:
globular pkg info <service-name>
# State must be PUBLISHED. VERIFIED means the upload pipeline didn't complete.
```

FETCH stalls also happen when the node agent is unreachable (the workflow dispatches to it but gets no response):

```bash
sudo systemctl status globular-node-agent
sudo journalctl -u globular-node-agent --no-pager -n 30
```

### `globular cluster health` says healthy but things aren't working

The health command checks heartbeats. A node can be heartbeating perfectly while half its services are failed or stuck. The health command will call it "healthy."

The doctor is more honest:

```bash
globular cluster get-doctor-report
```

If you see CRITICAL findings here despite a healthy heartbeat, the cluster has real problems that `cluster health` is not surfacing. Trust the doctor over the health check.

### `globular` CLI returns "unauthorized" immediately after install

The CLI needs a token. Tokens expire. On a fresh install the admin password is set but no token exists yet.

```bash
globular auth login --username admin --password <your-password>
```

If login itself fails with "service unavailable" or "connection refused", the authentication service isn't running:

```bash
sudo systemctl status globular-authentication
sudo journalctl -u globular-authentication --no-pager -n 30
# Often caused by: etcd unreachable, or the service started before PKI was ready
sudo systemctl restart globular-authentication
```

If it fails with "invalid credentials" — the password was not set correctly during install. The actual command name is `globular auth root-passwd`, not `globular auth set-password`:

```bash
globular auth root-passwd --password YourStr0ngP@ssword!
```

### The gateway (port 443) returns nothing

Envoy's route configuration comes from the xDS server. If Envoy started before xDS had any routes loaded, it listens but serves nothing.

```bash
sudo systemctl status globular-envoy globular-xds

# Fix: restart xDS first, then Envoy
sudo systemctl restart globular-xds
sleep 10
sudo systemctl restart globular-envoy
```

Also check if something else already has port 443:

```bash
sudo ss -tlnp | grep ':443'
# If another process is listed, it needs to be removed or reconfigured
```

### The node never gets past APPLYING after a full install

If everything appears to be running (all systemd units active, no failed units, controller healthy) but the desired-state list never converges, the problem is usually one of:

1. **Clock skew** — JWTs expire; if the node's clock is off by more than a few minutes, tokens look expired before they are used. `sudo timedatectl set-ntp true` and then restart the controller and node agent.
2. **Certificate not trusted** — The cluster CA cert was regenerated after some services already started. `sudo systemctl restart globular-node-agent globular-cluster-controller`.
3. **ScyllaDB not ready** — Several services (repository, AI memory) require ScyllaDB. If it's still bootstrapping, service startup fails silently. Check `sudo systemctl status scylladb` and give it a few minutes on first boot.

---

## What "healthy" actually looks like

This is the full checklist. `globular cluster health` passing is necessary but not sufficient.

```bash
# 1. No failed systemd units
sudo systemctl --failed | grep -E "globular|etcd|minio|scylladb"
# Should return nothing

# 2. All core units active
sudo systemctl list-units 'globular-*' --state=active --no-pager
# Should include: cluster-controller, node-agent, workflow,
#                 authentication, rbac, repository, dns,
#                 envoy, xds, monitoring, minio

# 3. Controller sees the node and it's heartbeating
globular cluster health
# CLUSTER STATUS: HEALTHY, NODES: 1/1 healthy

# 4. Desired state converged (no APPLYING, no FAILED)
globular services desired list
# All entries: STATUS = INSTALLED

# 5. Doctor finds no critical issues
globular cluster get-doctor-report
# No CRITICAL findings

# 6. No drift
globular cluster get-drift-report
# Empty or INFO-level only
```

If you have all six, the cluster is genuinely healthy. If `globular cluster health` passes but steps 1, 4, 5, or 6 fail — the cluster has a real problem hiding behind a green heartbeat.

---

## What is still evolving

Globular at v1.0.x is stable for 3-node production use, but some areas are still maturing. Know this before you go further:

- **Single-node is for development only**: A single node has no etcd quorum, no MinIO erasure redundancy, no ScyllaDB replication. Data can be lost if the node fails. Add at least two more nodes before storing anything you care about.
- **Compute service not deployed**: The compute server code exists but is not built or packaged. It is a Phase 2 feature.
- **Some CLI commands missing**: Backup, monitoring, and AI commands documented elsewhere have no CLI wrapper yet. Use the MCP tools or direct gRPC. See [Known Issues](operators/known-issues.md) for the full list.
- **Split-horizon DNS requires /etc/hosts**: The DNS service cannot currently serve different answers for internal vs. external queries. VIP hairpin NAT requires a manual `/etc/hosts` override on each node.

See [Platform Status](operators/platform-status.md) for a complete, current picture of what is implemented, partial, and planned.

---

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
| **Current implementation status** | [Platform Status](operators/platform-status.md) |
