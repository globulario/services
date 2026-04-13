# Getting Started with Globular

This guide takes you from a bare Linux machine to a running Globular cluster with a deployed service in under 15 minutes.

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

## Step 1 — Install Globular

Download and install the Globular binaries and packages:

```bash
# Download the installer (replace URL with your distribution source)
wget https://releases.globular.io/latest/globular-installer-linux-amd64.tar.gz
tar xzf globular-installer-linux-amd64.tar.gz
cd globular-installer

# Run the installer
sudo ./install.sh
```

The installer places binaries in `/usr/local/bin/` and packages in `/var/lib/globular/packages/`. Verify the installation:

```bash
globular --version
# Output: globular version 0.0.1

node_agent_server --version
# Output: node_agent 0.0.1
```

## Step 2 — Start the Node Agent

The Node Agent is the local executor that manages services on this machine:

```bash
sudo systemctl start globular-node-agent
sudo systemctl status globular-node-agent
# Active: active (running)
```

The agent is now listening on port 11000, waiting for bootstrap instructions.

## Step 3 — Bootstrap the Cluster

Initialize a single-node cluster:

```bash
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --profile core \
  --profile gateway
```

This command:
1. Creates an etcd cluster on this node
2. Generates TLS certificates and Ed25519 signing keys
3. Starts the Cluster Controller
4. Installs core services (authentication, RBAC, event, discovery, repository)
5. Installs the Envoy gateway
6. Registers everything in etcd for discovery
7. Seeds the desired state

Bootstrap takes 2-5 minutes. Watch the output for progress.

## Step 4 — Verify the Cluster

Check that everything is running:

```bash
globular cluster health
```

Expected output:
```
CLUSTER STATUS: HEALTHY
NODES: 1/1 healthy

NODE         STATUS   LAST SEEN    SERVICES
node-abc123  healthy  2s ago       12/12 running
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

All services should show `INSTALLED`. If any show `APPLYING`, wait a minute and check again — the convergence model is still working.

## Step 5 — Set the Admin Password

```bash
globular auth set-password --username admin --password YourStr0ngP@ssword!
```

Authenticate with the admin account:

```bash
globular auth login --username admin --password YourStr0ngP@ssword!
# Output: Token saved
```

## Step 6 — Deploy Your First Service

Let's deploy the echo service — a simple test service that echoes back whatever you send it.

```bash
# Check if the echo package is in the repository
globular pkg info echo
# Shows: echo 0.0.1 PUBLISHED

# Set the desired state
globular services desired set echo 0.0.1

# Watch the deployment
globular services desired list
# echo  0.0.1  0/1  APPLYING...

# Wait a few seconds...
globular services desired list
# echo  0.0.1  1/1  INSTALLED
```

The platform:
1. Detected the desired-state change
2. Created a workflow for this node
3. Fetched the echo package from the repository
4. Verified the checksum
5. Installed the binary
6. Started the systemd unit
7. Confirmed the health check passed

## Step 7 — Verify the Deployment

Check that the echo service is running:

```bash
# Cluster health shows the new service
globular cluster health

# Check for any drift
globular services repair --dry-run
# All services should show INSTALLED
```

## Step 8 — Check Workflow History

See the workflow that just deployed the echo service:

```bash
globular workflow list --service echo
```

Output:
```
RUN ID          SERVICE  NODE         STATUS     TRIGGER        STARTED
wf-run-abc123   echo     node-abc123  SUCCEEDED  DESIRED_DRIFT  2m ago
```

View the details:
```bash
globular workflow get wf-run-abc123
```

This shows every step: resolve_artifact → fetch_package → verify_checksum → install_binary → configure_service → start_unit → verify_health — all SUCCEEDED.

## What Just Happened

Here's what the architecture looks like after bootstrap:

```
Your Machine (node-abc123)
│
├── Node Agent (port 11000)
│   └── Manages all services via systemd
│
├── Cluster Controller (port 12000)
│   └── Tracks desired state, dispatches workflows
│
├── etcd (port 2379)
│   └── Stores all cluster configuration and state
│
├── Repository + MinIO (port 9000)
│   └── Stores service packages
│
├── Gateway / Envoy (ports 443, 8443)
│   └── External traffic entry point
│
├── Authentication (port 10101)
│   └── JWT token management
│
├── RBAC (port 10104)
│   └── Permission enforcement
│
└── Echo Service (deployed by you)
    └── Running and healthy
```

Every service registered itself in etcd. Every gRPC call goes through the interceptor chain (authentication → RBAC → audit). The convergence model continuously ensures reality matches desired state.

## Next Steps

Now that you have a running cluster:

1. **Add more nodes** — [Adding Nodes](operators/adding-nodes.md) covers creating join tokens and expanding the cluster
2. **Deploy your own service** — [Writing a Microservice](developers/writing-a-microservice.md) walks through creating a gRPC service from scratch
3. **Understand the architecture** — [Architecture Overview](operators/architecture-overview.md) explains how all components interact
4. **Set up monitoring** — [Observability](operators/observability.md) covers Prometheus, logging, and the Cluster Doctor
5. **Configure backups** — [Backup and Restore](operators/backup-and-restore.md) covers disaster recovery
6. **Learn the convergence model** — [Convergence Model](operators/convergence-model.md) explains how Globular drives desired state to reality

For quick operational tasks, see the [Tasks](tasks/deploy-application.md) section.
