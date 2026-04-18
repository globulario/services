# Installation (Day-0 Bootstrap)

This page walks through the complete process of initializing a Globular cluster from a bare Linux machine to a running, operational single-node cluster. Every step is explained, including what happens internally at each stage.

## Prerequisites

Before bootstrapping, ensure the following:

**Hardware**:
- Linux machine (amd64 architecture)
- Minimum 4 GB RAM, 2 CPU cores
- 20 GB disk space for packages, etcd data, and MinIO storage
- Network connectivity (the node must be reachable by future cluster members)

**Software**:
- systemd (process supervision)
- A non-root user with sudo access (for systemd unit installation)
- Globular binaries installed (see below)

**Network**:
- A routable IP address (not just loopback)
- Ports available: 11000 (Node Agent), 12000 (Controller), 2379/2380 (etcd), 9000 (MinIO), 443/8443 (Gateway)
- DNS resolution or `/etc/hosts` entries for the cluster domain (optional but recommended)

## Installing Globular Binaries

Globular is distributed as a set of compiled binaries and infrastructure packages. The installation process places these on the target machine:

```bash
# The installer places binaries and packages:
/usr/local/bin/
├── globular              # CLI tool
├── node_agent_server     # Node Agent binary
├── gateway               # Envoy gateway
├── xds                   # xDS server for gateway configuration
└── ...                   # All other service binaries

/var/lib/globular/packages/
├── globular-etcd-3.5.14-linux_amd64-1.tgz
├── globular-minio-...-linux_amd64-1.tgz
├── globular-prometheus-...-linux_amd64-1.tgz
├── globular-authentication-0.0.1-linux_amd64-1.tgz
└── ...                   # All service and infrastructure packages
```

## Bootstrap Process

### Step 1: Start the Node Agent

The Node Agent must be running before bootstrap can begin. It is the entry point for all cluster operations on a node:

```bash
# Start the Node Agent as a systemd service
sudo systemctl start globular-node-agent

# Verify it's running
sudo systemctl status globular-node-agent
# Active: active (running)

# Or start manually for debugging
node_agent_server --port 11000
```

The Node Agent listens on port 11000 and waits for bootstrap or join instructions.

### Step 2: Run Bootstrap

```bash
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --profile core \
  --profile gateway
```

**Flags explained**:
- `--node localhost:11000`: The Node Agent endpoint on this machine. `localhost` is allowed here because we're talking to the local agent.
- `--domain mycluster.local`: The cluster domain. This becomes the cluster identity and is embedded in all certificates and tokens. Choose carefully — changing it later requires re-bootstrapping.
- `--profile core`: Install core services (controller, authentication, RBAC, event, discovery, repository)
- `--profile gateway`: Install the Envoy gateway and xDS server

### What Happens Internally

The bootstrap command triggers the Node Agent's `BootstrapFirstNode` RPC, which executes the following sequence:

**Phase 1: Bootstrap Security Gate**

The Node Agent creates the bootstrap flag file:
```
/var/lib/globular/bootstrap.enabled
{
  "enabled_at_unix": 1712937600,
  "expires_at_unix": 1712939400,
  "nonce": "uuid-v4",
  "created_by": "globularcli",
  "version": 1
}
```

This opens a 30-minute security window. During this window, requests from localhost can access essential RPCs without RBAC enforcement. After 30 minutes, the window closes automatically.

**Phase 2: etcd Initialization**

The Node Agent starts a single-node etcd cluster:
1. Creates the etcd data directory (`/var/lib/etcd/`)
2. Generates etcd peer and client certificates
3. Starts etcd with the cluster domain as the cluster name
4. Writes the initial cluster configuration to etcd
5. Stores the cluster network spec (`ClusterNetwork` resource): domain, protocol, ports, ACME settings

etcd is now running at `0.0.0.0:2379` (client) and `0.0.0.0:2380` (peer).

**Phase 3: Key Generation**

The Node Agent generates Ed25519 key pairs for the node:
1. Creates the node's signing key at `/var/lib/globular/keys/<mac>_private`
2. Creates the public key at `/var/lib/globular/keys/<mac>_public`
3. Registers the public key in etcd for peer discovery

**Phase 4: Certificate Provisioning**

TLS certificates are generated for the node:
1. Generate a CA key pair (this node is the initial CA)
2. Create server and client certificates signed by the CA
3. Store certificates in `/var/lib/globular/pki/`
4. Write CA certificate to etcd for distribution to future nodes

**Phase 5: Core Service Installation**

For each profile's services (ordered by priority and dependencies):
1. Look up the package in the local packages directory
2. Extract the binary to `/usr/local/bin/`
3. Write the systemd unit file
4. Write service configuration to etcd (endpoint, port, TLS paths)
5. Start the systemd unit
6. Run health check

The typical installation order for `core` and `gateway` profiles:
1. etcd (already running from Phase 2)
2. Cluster Controller (port 12000)
3. Authentication Service (port 10101)
4. RBAC Service (port 10104)
5. Event Service (port 10102)
6. Repository Service
7. Discovery Service
8. DNS Service
9. Gateway / Envoy
10. xDS Server
11. Remaining profile services

**Phase 6: RBAC Initialization**

With the RBAC service running, the bootstrap process sets up initial role bindings:
1. Creates the root administrator account
2. Binds the root account to `globular-admin` role
3. Creates the controller service account and binds to `globular-controller-sa`
4. Creates the node agent service account and binds to `globular-node-agent-sa`

**Phase 7: Service Registration**

Each service registers itself in etcd for discovery:
```
/globular/services/<service_id>/config → { address, port, protocol, tls }
/globular/services/<service_id>/instances/<node_key> → { endpoint }
```

**Phase 8: Repository Publishing**

If the Repository service is running, the bootstrap process publishes all local packages to it:
1. For each `.tgz` in `/var/lib/globular/packages/`
2. Upload to the Repository service
3. Repository stores in MinIO and creates the artifact manifest
4. Publish state transitions: STAGING → VERIFIED → PUBLISHED

**Phase 9: Desired State Seeding**

The bootstrap process creates desired-state entries for all installed services:
```
/globular/resources/DesiredService/etcd → { version: "3.5.14" }
/globular/resources/DesiredService/authentication → { version: "0.0.1" }
...
```

This ensures the convergence model can track and manage these services going forward.

**Phase 10: Bootstrap Cleanup**

The 30-minute bootstrap window is not explicitly closed — it expires on its own. The bootstrap flag file remains until expiry, but after RBAC is initialized, all requests go through normal authentication and authorization.

### Step 3: Verify the Cluster

After bootstrap completes (typically 2-5 minutes), verify the cluster is operational:

```bash
# Check cluster health
globular cluster health
# Output:
# CLUSTER STATUS: HEALTHY
# NODES: 1/1 healthy
#
# NODE         STATUS   LAST SEEN    SERVICES
# node-abc123  healthy  2s ago       12/12 running

# Check desired state
globular services desired list
# Output:
# SERVICE            VERSION    NODES   STATUS
# etcd               3.5.14     1/1     INSTALLED
# authentication     0.0.1      1/1     INSTALLED
# rbac               0.0.1      1/1     INSTALLED
# controller         0.0.1      1/1     INSTALLED
# gateway            0.0.1      1/1     INSTALLED
# ...

# Check installed packages
globular cluster get-drift-report
# All services should show INSTALLED (no drift)
```

### Step 4: Set Root Password

After bootstrap, set the root administrator password:

```bash
globular auth root-passwd --password <strong-password>
```

This password is used for the initial `globular auth login` and should be changed to a strong value immediately after bootstrap.

## Bootstrap Troubleshooting

### etcd Fails to Start

**Symptoms**: Bootstrap hangs at "Starting etcd..."
**Cause**: Port 2379 or 2380 already in use, or data directory exists from a previous installation
**Fix**:
```bash
# Check for port conflicts
ss -tlnp | grep -E '2379|2380'

# Remove stale data (if re-bootstrapping)
sudo rm -rf /var/lib/etcd/
```

### Services Fail Health Checks

**Symptoms**: Bootstrap reports "health check failed for <service>"
**Cause**: Port conflict, missing dependency, or binary crash
**Fix**:
```bash
# Check service logs
journalctl -u globular-<service> --no-pager -n 50

# Check if the port is available
ss -tlnp | grep <port>
```

### Bootstrap Window Expires

**Symptoms**: Bootstrap fails with "bootstrap expired"
**Cause**: The 30-minute window closed before all services were initialized (slow machine, network issues)
**Fix**: Re-run the bootstrap command. It will create a new 30-minute window.

### Certificate Errors

**Symptoms**: Services fail with "TLS handshake error" or "certificate verify failed"
**Cause**: Certificate files missing or corrupted
**Fix**:
```bash
# Check certificate files exist
ls -la /var/lib/globular/pki/

# Check certificate validity
openssl x509 -in /var/lib/globular/pki/issued/services/service.crt -text -noout
```

## Post-Bootstrap Configuration

### DNS Configuration

If you want the cluster domain to resolve via DNS:

```bash
# Create the DNS zone
globular dns zone create --domain mycluster.local

# Add a record for the controller
globular dns record add --zone mycluster.local \
  --name controller --type A --value <node-ip>

# Add a wildcard for services
globular dns record add --zone mycluster.local \
  --name "*.services" --type A --value <node-ip>
```

### Configure External Access

The Envoy gateway handles external traffic on ports 443 and 8443. Ensure these ports are accessible from client machines and configure TLS:

```bash
# If using ACME (Let's Encrypt):
# ACME configuration is set during bootstrap via ClusterNetwork resource
# The gateway automatically requests and renews certificates

# If using custom certificates:
# Place them in /var/lib/globular/pki/ and restart the gateway
```

## Single-Node vs Multi-Node

A single-node Globular cluster is fully functional — all services run on one machine. This is suitable for:
- Development and testing
- Small deployments (< 50 users)
- Edge computing / appliance deployments

For production environments with high availability requirements, add nodes to the cluster (see [Adding Nodes](operators/adding-nodes.md)). The convergence model, etcd replication, and MinIO erasure coding all benefit from multiple nodes.

## What's Next

- [Adding Nodes](operators/adding-nodes.md): Expand the cluster with Day-1 operations
- [Deploying Applications](operators/deploying-applications.md): Deploy services on your cluster
