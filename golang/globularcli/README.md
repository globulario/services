# Globular CLI - Complete User Guide

`globularcli` (or simply `globular`) is the primary command-line interface for managing Globular clusters. It provides complete control over cluster lifecycle, service deployment, DNS management, network configuration, and more.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Architecture & Concepts](#architecture--concepts)
- [Global Flags](#global-flags)
- [Command Reference](#command-reference)
  - [Cluster Management](#cluster-management)
  - [DNS Management](#dns-management)
  - [Network Configuration](#network-configuration)
  - [Node Plans](#node-plans)
  - [Package Management](#package-management)
  - [Debug Tools](#debug-tools)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

---

## Overview

The Globular CLI is a Go-based tool that communicates directly with the Globular control plane via gRPC. It provides:

- **Cluster lifecycle management**: Bootstrap, join, and manage multi-node clusters
- **Service orchestration**: Deploy, upgrade, and manage Globular services
- **DNS management**: Configure DNS records and domains for service discovery
- **Network configuration**: Manage HTTPS, certificates, and ACME automation
- **Node operations**: Monitor health, apply plans, and debug issues
- **Package operations**: Build, verify, and publish service packages

Unlike many Kubernetes-inspired tools, Globular CLI uses direct gRPC communication rather than REST APIs, providing lower latency and stronger type safety.

---

## Installation

### Build from source

```bash
cd /path/to/globulario/services/golang
go build -o globularcli ./globularcli
sudo mv globularcli /usr/local/bin/globular
```

### Using go install

```bash
go install github.com/globulario/services/golang/globularcli@latest
```

### Verify installation

```bash
globular --help
```

---

## Architecture & Concepts

### Control Plane Components

1. **Cluster Controller** (port 10000): Central orchestrator that manages cluster-wide state, node membership, and service deployment plans
2. **Node Agent** (port 11000): Runs on each node, executes plans, manages local services, and reports health
3. **DNS Service** (port 10033): Provides DNS resolution and record management for service discovery

### Key Concepts

- **Bootstrap**: Initial setup of the first cluster node with controller and essential services
- **Join**: Process for adding additional nodes to an existing cluster
- **Plans**: Declarative specifications describing desired node state (services, config, network)
- **Profiles**: Named sets of services that define a node's role (e.g., "worker", "storage", "gateway")
- **Managed Domains**: DNS domains under Globular's control for service discovery and TXT record management
- **Reconciliation**: Continuous process ensuring actual state matches desired state

---

## Global Flags

These flags apply to all commands:

| Flag | Default | Description |
|------|---------|-------------|
| `--controller` | `localhost:10000` | Cluster controller gRPC endpoint |
| `--node` | `localhost:11000` | Node agent gRPC endpoint |
| `--dns` | `localhost:10033` | DNS service gRPC endpoint |
| `--token` | (empty) | Authorization token for authenticated operations |
| `--ca` | (empty) | Path to CA certificate bundle for TLS verification |
| `--insecure` | `false` | Skip TLS certificate verification (not recommended for production) |
| `--timeout` | `15s` | Request timeout duration |
| `--output` | `table` | Output format: `table`, `json`, or `yaml` |

### Examples

```bash
# Connect to remote controller
globular --controller cluster.example.com:10000 cluster nodes list

# Use TLS with custom CA
globular --ca /etc/globular/ca.pem cluster health

# Use authentication token
globular --token $CLUSTER_TOKEN cluster nodes list

# Get JSON output for scripting
globular --output json cluster nodes list
```

---

## Command Reference

### Cluster Management

Cluster commands manage the entire cluster lifecycle from initial bootstrap through day-to-day operations.

#### `globular cluster bootstrap`

**Purpose**: Initialize the very first node in a new cluster. This creates the controller, sets up etcd, and configures the foundational network.

**Usage**:
```bash
globular cluster bootstrap \
  --node <node-agent-endpoint> \
  --domain <cluster-domain> \
  --bind <controller-bind-address> \
  [--profile <profile1> --profile <profile2>...]
```

**Flags**:
- `--node`: Node agent endpoint (required, e.g., `localhost:11000`)
- `--domain`: Cluster domain name (required, e.g., `mycluster.local`)
- `--bind`: Address for controller to bind to (default: `0.0.0.0:10000`)
- `--profile`: Initial service profiles for the first node (can be specified multiple times)

**Example**:
```bash
# Bootstrap a new cluster with gateway and DNS services
globular cluster bootstrap \
  --node localhost:11000 \
  --domain cluster.globular.io \
  --profile gateway \
  --profile dns
```

**What happens**:
1. Node agent receives bootstrap request
2. Etcd cluster is initialized (if managed)
3. Controller starts and binds to specified address
4. Initial profiles are applied to the node
5. DNS service is configured with the cluster domain

---

#### `globular cluster join`

**Purpose**: Add a new node to an existing cluster. The node requests membership and awaits approval from a cluster administrator.

**Usage**:
```bash
globular cluster join \
  [--node <node-agent-endpoint>] \
  [--controller <controller-endpoint>] \
  [--join-token <token>]
```

**Flags**:
- `--node`: Target node agent endpoint (default: from global flag)
- `--controller`: Controller endpoint to join (default: from global flag)
- `--join-token`: Pre-created join token (optional, for automated flows)

**Example**:
```bash
# Interactive join (generates request ID)
globular --controller cluster-controller:10000 \
  --node new-node:11000 \
  cluster join

# Automated join with token
globular --controller cluster-controller:10000 \
  --node new-node:11000 \
  cluster join --join-token abc123...
```

**Workflow**:
1. Node sends join request to controller
2. Controller creates a pending join request
3. Administrator approves request (see `cluster requests approve`)
4. Node receives cluster configuration and joins

---

#### `globular cluster token`

**Purpose**: Manage pre-authorized join tokens for automated node addition.

**Commands**:

##### `globular cluster token create`

Create a new join token that allows nodes to join without manual approval.

```bash
globular cluster token create [--expires <duration>]

# Examples
globular cluster token create                    # Default 24h expiration
globular cluster token create --expires 168h     # 7 days
globular cluster token create --expires 1h30m    # 1.5 hours
```

**Output**: A unique token string that can be used in `cluster join --join-token`

**Use cases**:
- Automated deployment pipelines
- Cloud auto-scaling groups
- Pre-provisioned node pools

---

#### `globular cluster requests`

**Purpose**: Manage pending join requests from nodes waiting for approval.

**Commands**:

##### `globular cluster requests list`

Show all pending join requests.

```bash
globular cluster requests list
```

**Output**: Table showing request ID, node hostname, IP, timestamp, and metadata

##### `globular cluster requests approve`

Approve a pending join request and optionally assign profiles.

```bash
globular cluster requests approve <request-id> \
  [--profile <profile>...] \
  [--meta key=value...]

# Examples
globular cluster requests approve req_abc123 \
  --profile worker \
  --profile storage

globular cluster requests approve req_abc123 \
  --meta zone=us-east-1a \
  --meta rack=rack-05
```

**Flags**:
- `--profile`: Service profiles to assign (repeatable)
- `--meta`: Key-value metadata pairs (repeatable)

##### `globular cluster requests reject`

Reject a pending join request.

```bash
globular cluster requests reject <request-id> [--reason <reason>]

# Example
globular cluster requests reject req_abc123 \
  --reason "Node does not meet security requirements"
```

---

#### `globular cluster nodes`

**Purpose**: Inspect and manage cluster nodes.

**Commands**:

##### `globular cluster nodes list`

List all nodes in the cluster.

```bash
globular cluster nodes list
```

**Output**: Table with node ID, hostname, address, profiles, health status, and last heartbeat

**Use cases**:
- Monitor cluster membership
- Verify node health
- Check profile assignments

##### `globular cluster nodes get`

Get detailed information about a specific node.

```bash
globular cluster nodes get <node-id>

# Example
globular cluster nodes get node_abc123
```

**Output**: Detailed node information including:
- Node metadata (ID, hostname, addresses)
- Assigned profiles
- Running services and versions
- Resource usage (if available)
- Last heartbeat and health status

##### `globular cluster nodes profiles set`

Change the service profiles assigned to a node.

```bash
globular cluster nodes profiles set <node-id> \
  --profile <profile1> \
  --profile <profile2>...

# Example - Convert node to storage + worker
globular cluster nodes profiles set node_abc123 \
  --profile storage \
  --profile worker
```

**What happens**:
1. Controller updates desired profiles for the node
2. Controller generates reconciliation plan
3. Plan is applied to node (stops old services, starts new ones)
4. Node reports success/failure

**⚠️ Warning**: Changing profiles restarts services. Plan accordingly for production nodes.

##### `globular cluster nodes remove`

Remove a node from the cluster.

```bash
globular cluster nodes remove <node-id> [--force] [--no-drain]

# Examples
globular cluster nodes remove node_abc123                # Graceful removal
globular cluster nodes remove node_abc123 --force        # Force even if unreachable
globular cluster nodes remove node_abc123 --no-drain     # Skip graceful shutdown
```

**Flags**:
- `--force`: Remove node even if unreachable (default: false)
- `--drain/--no-drain`: Gracefully stop services before removal (default: true)

---

#### `globular cluster health`

**Purpose**: Display overall cluster health status.

```bash
globular cluster health
```

**Output**:
- Overall cluster status (healthy/degraded/unhealthy)
- Node count summary (total, healthy, unhealthy, unknown)
- Per-node health details with last seen time
- Any error messages or issues

**Use cases**:
- Quick cluster status check
- Monitoring and alerting integration
- Troubleshooting connectivity issues

---

#### `globular cluster upgrade`

**Purpose**: Upgrade the Globular platform binary on a node via controller-orchestrated plan.

```bash
globular cluster upgrade <artifact-path> \
  --node-id <node-id> \
  [--platform <os/arch>] \
  [--sha256 <checksum>] \
  [--target-path <path>] \
  [--probe-port <port>]

# Example
globular cluster upgrade ./globular_linux_amd64 \
  --node-id node_abc123 \
  --platform linux/amd64 \
  --sha256 a1b2c3d4... \
  --target-path /usr/local/bin/globular
```

**Flags**:
- `--node-id`: Target node to upgrade (required)
- `--platform`: Target OS/architecture (default: current host platform)
- `--sha256`: SHA256 checksum (computed if omitted)
- `--target-path`: Destination path for binary (default: `/usr/local/bin/globular`)
- `--probe-port`: HTTP port for health check after upgrade (default: 80)

**Workflow**:
1. Binary is uploaded/distributed to node
2. Services are stopped
3. Binary is replaced atomically
4. Services are restarted
5. Health probe verifies successful upgrade

---

#### `globular cluster watch`

**Purpose**: Stream real-time operation events from the controller.

```bash
globular cluster watch [--node-id <node>] [--op <operation-id>]

# Examples
globular cluster watch                              # Watch all operations
globular cluster watch --node-id node_abc123        # Filter by node
globular cluster watch --op op_xyz789               # Watch specific operation
```

**Output**: Continuous stream of events showing:
- Operation start/progress/completion
- Step-by-step plan execution
- Success/failure status
- Error messages (if any)

**Use cases**:
- Monitor long-running operations
- Debug plan execution
- Track deployment progress

---

### DNS Management

The DNS command group manages Globular's integrated DNS service for service discovery and ACME DNS-01 challenges.

#### `globular dns domains`

**Purpose**: Manage the list of domains under Globular's DNS control (managed domains).

**Why it matters**: Only managed domains can have DNS records (A/AAAA/TXT) set through Globular. This prevents accidental modification of external DNS zones.

**Commands**:

##### `globular dns domains get`

List all managed domains.

```bash
globular dns domains get
```

##### `globular dns domains set`

Replace the entire list of managed domains.

```bash
globular dns domains set <domain1> [domain2...]

# Example
globular dns domains set cluster.local example.com
```

⚠️ **Warning**: This replaces all domains. Existing domains not in the list are removed.

##### `globular dns domains add`

Add domains to the managed list (preserves existing).

```bash
globular dns domains add <domain1> [domain2...]

# Example
globular dns domains add api.cluster.local db.cluster.local
```

##### `globular dns domains remove`

Remove specific domains from the managed list.

```bash
globular dns domains remove <domain1> [domain2...]

# Example
globular dns domains remove old-domain.local
```

**Notes**:
- Domains are automatically normalized (lowercased, trailing dots removed)
- Duplicate domains are silently de-duplicated
- Removing a domain does not delete its DNS records (records become inaccessible until domain is re-added)

---

#### `globular dns a` - IPv4 Address Records

**Purpose**: Manage A (IPv4 address) DNS records for managed domains.

**Commands**:

##### `globular dns a set`

Create or update an A record.

```bash
globular dns a set <name> <ipv4> [--ttl <seconds>]

# Examples
globular dns a set api.cluster.local 192.168.1.10
globular dns a set gateway.cluster.local 10.0.0.5 --ttl 600
```

**Behavior**:
- If record exists with same IP, it's updated (TTL refreshed)
- If record exists with different IP, new IP is added (multi-value record)
- Domain must be in managed domains list

##### `globular dns a get`

Retrieve all A records for a name.

```bash
globular dns a get <name>

# Example
globular dns a get api.cluster.local
```

**Output**: List of IPv4 addresses (one per line)

##### `globular dns a remove`

Remove A record(s).

```bash
globular dns a remove <name> [<ipv4>]

# Examples
globular dns a remove api.cluster.local                     # Remove ALL A records
globular dns a remove api.cluster.local 192.168.1.10        # Remove specific IP
```

---

#### `globular dns aaaa` - IPv6 Address Records

**Purpose**: Manage AAAA (IPv6 address) DNS records.

**Commands**: Same as `dns a` but for IPv6 addresses.

```bash
# Set IPv6 address
globular dns aaaa set api.cluster.local fd12::1 --ttl 300

# Get IPv6 addresses
globular dns aaaa get api.cluster.local

# Remove IPv6 address
globular dns aaaa remove api.cluster.local fd12::1
```

**Use cases**:
- IPv6-only networks
- Dual-stack (IPv4 + IPv6) configurations
- Modern cloud environments

---

#### `globular dns txt` - Text Records

**Purpose**: Manage TXT DNS records, primarily used for ACME DNS-01 challenges and service verification.

**Commands**:

##### `globular dns txt set`

Create or add a TXT record.

```bash
globular dns txt set <name> <text> [--ttl <seconds>]

# Examples
globular dns txt set _acme-challenge.cluster.local "validation-token-xyz"
globular dns txt set _dmarc.cluster.local "v=DMARC1; p=none"
```

**Behavior**:
- Multiple TXT values can exist for the same name
- Each `set` adds a new value (doesn't replace existing)
- Domain must be in managed domains list

##### `globular dns txt get`

Retrieve all TXT records for a name.

```bash
globular dns txt get <name>

# Example
globular dns txt get _acme-challenge.cluster.local
```

##### `globular dns txt remove`

Remove TXT record(s).

```bash
globular dns txt remove <name> [<text>]

# Examples
globular dns txt remove _acme-challenge.cluster.local                    # Remove ALL TXT records
globular dns txt remove _acme-challenge.cluster.local "old-token-123"   # Remove specific value
```

**ACME Integration**: The ACME DNS-01 challenge process automatically manages TXT records under `_acme-challenge.<domain>` during certificate issuance.

---

#### `globular cluster dns bootstrap`

**Purpose**: Quick-start command to configure DNS for a new cluster in one step.

```bash
globular cluster dns bootstrap \
  --domain <domain> \
  --ipv6 <ipv6-address> \
  [--ipv4 <ipv4-address>] \
  [--wildcard]

# Example - Set up DNS for cluster with wildcard
globular cluster dns bootstrap \
  --domain cluster.local \
  --ipv6 fd12::1 \
  --ipv4 192.168.1.100 \
  --wildcard
```

**What it does**:
1. Adds domain to managed domains list
2. Sets A record for apex domain (if `--ipv4` provided)
3. Sets AAAA record for apex domain (if `--ipv6` provided)
4. Sets wildcard `*.<domain>` records (if `--wildcard` flag used)

**Use cases**:
- Initial cluster setup
- Simplify DNS configuration
- Standardize cluster networking

**Equivalent manual commands**:
```bash
globular dns domains add cluster.local
globular dns aaaa set cluster.local fd12::1
globular dns a set cluster.local 192.168.1.100
globular dns aaaa set *.cluster.local fd12::1
globular dns a set *.cluster.local 192.168.1.100
```

---

### Network Configuration

Network commands manage cluster-wide networking, including domain, protocol (HTTP/HTTPS), and ACME certificate automation.

#### `globular cluster network get`

**Purpose**: Display current cluster network configuration.

```bash
globular cluster network get
```

**Output**:
- Cluster domain
- Protocol (http/https)
- HTTP port
- HTTPS port (if applicable)
- ACME status (enabled/disabled)
- Admin email (for ACME)
- Alternate domains

---

#### `globular cluster network set`

**Purpose**: Update cluster network configuration. This triggers a cluster-wide reconciliation to apply the new settings.

```bash
globular cluster network set \
  --domain <domain> \
  [--protocol http|https] \
  [--http-port <port>] \
  [--https-port <port>] \
  [--acme] \
  [--email <email>] \
  [--alt-domain <domain>...] \
  [--watch]

# Examples

# Simple HTTP setup
globular cluster network set \
  --domain cluster.local \
  --protocol http \
  --http-port 8080

# HTTPS with manual certificates
globular cluster network set \
  --domain cluster.example.com \
  --protocol https \
  --https-port 443

# HTTPS with ACME automation
globular cluster network set \
  --domain cluster.example.com \
  --protocol https \
  --https-port 443 \
  --acme \
  --email admin@example.com \
  --watch
```

**Flags**:
- `--domain`: Cluster domain (required)
- `--protocol`: `http` or `https` (default: `http`)
- `--http-port`: HTTP port (default: 8080)
- `--https-port`: HTTPS port (default: 8443)
- `--acme`: Enable automatic Let's Encrypt certificates via ACME DNS-01
- `--email`: Admin email for ACME (required when `--acme` is set)
- `--alt-domain`: Additional domains for certificate SAN (repeatable)
- `--watch`: Stream operation events until complete

**ACME Requirements**:
1. Domain must be in DNS managed domains
2. Domain must have valid A/AAAA records pointing to cluster
3. For **public domains**: Globular DNS must be authoritative OR integrated with your DNS provider
4. For **private domains**: ACME staging environment or custom CA required

**What happens**:
1. Controller creates reconciliation plan for all nodes
2. Network configuration is updated (`/var/lib/globular/network.json`)
3. If ACME enabled: Certificates are issued/renewed via DNS-01 challenge
4. TLS certificates are validated (if HTTPS)
5. Gateway, XDS, and Envoy services are restarted
6. Health probes verify successful configuration

**Reconciliation time**: 1-5 minutes depending on ACME issuance

---

### Node Plans

Plans are declarative specifications that describe the desired state of a node (services, configuration, network settings). The controller generates plans based on cluster state, and the node agent executes them.

#### `globular cluster plan get`

**Purpose**: Retrieve the current desired plan for a node.

```bash
globular cluster plan get <node-id>

# Example
globular cluster plan get node_abc123
```

**Output**: Full plan specification in JSON or YAML format, including:
- Services to be installed/upgraded
- Configuration files to be written
- Actions to be executed (e.g., `service.restart`, `tls.ensure`)
- Probes to verify success

**Use cases**:
- Inspect what changes would be applied
- Debug plan generation
- Audit desired vs actual state

---

#### `globular cluster plan apply`

**Purpose**: Request the controller to apply (or re-apply) a node's plan.

```bash
globular cluster plan apply <node-id> [--watch]

# Examples
globular cluster plan apply node_abc123
globular cluster plan apply node_abc123 --watch    # Stream progress
```

**Flags**:
- `--watch`: Stream operation events until completion

**When to use**:
- Force reconciliation after manual changes
- Recover from failed plan execution
- Apply profile changes immediately (vs waiting for controller)

**What happens**:
1. Controller fetches latest desired plan for node
2. Plan is sent to node agent
3. Node agent executes plan steps sequentially
4. Success/failure is reported back to controller

---

#### `globular debug agent apply-plan`

**Purpose**: Directly apply a plan to a node agent, bypassing the controller. **DEBUG ONLY - not recommended for production.**

```bash
globular debug agent apply-plan <node-id> \
  --agent <agent-endpoint> \
  [--watch]

# Example
globular debug agent apply-plan node_abc123 \
  --agent localhost:11000 \
  --watch
```

**Use cases**:
- Troubleshoot controller communication issues
- Test plan execution in isolated environments
- Emergency recovery when controller is unavailable

⚠️ **Warning**: Bypasses cluster coordination. Can cause state inconsistencies.

---

### Package Management

Package commands handle building, verifying, and publishing Globular service packages.

#### `globular pkg build`

**Purpose**: Build a service package (`.tgz`) from source files, configuration, and metadata.

```bash
globular pkg build \
  --spec <spec-yaml> \
  --version <version> \
  --out <output-dir> \
  [--installer-root <path>] \
  [--root <payload-root>] \
  [--platform <os_arch>]

# Example
globular pkg build \
  --spec ./services/myservice/spec.yaml \
  --version 1.0.0 \
  --out ./dist \
  --platform linux_amd64
```

**Flags**:
- `--spec`: Path to service spec YAML (or `--spec-dir` for multiple)
- `--version`: Package version (required, e.g., `1.0.0`)
- `--out`: Output directory for `.tgz` files (required)
- `--installer-root`: Globular installer root (for auto-discovery)
- `--root`: Explicit payload root with `bin/` and `config/` directories
- `--platform`: Target platform as `os_arch` (default: current platform)
- `--bin-dir`: Explicit path to binaries directory
- `--config-dir`: Explicit path to config directory
- `--publisher`: Publisher identifier (default: `core@globular.io`)

**Package structure**:
```
service.myservice_1.0.0_linux_amd64.tgz
├── manifest.json          # Package metadata
├── bin/
│   └── myservice_server   # Binary
├── config/
│   └── config.json        # Default configuration
└── systemd/
    └── myservice.service  # Systemd unit file
```

---

#### `globular pkg verify`

**Purpose**: Verify the integrity and structure of a package.

```bash
globular pkg verify --file <package.tgz>

# Example
globular pkg verify --file service.myservice_1.0.0_linux_amd64.tgz
```

**Checks**:
- Valid `.tgz` archive
- `manifest.json` present and valid
- Required fields populated
- File permissions correct
- Checksums match

---

#### `globular pkg publish`

**Purpose**: Upload a package to the Globular repository service for distribution.

```bash
globular pkg publish \
  --file <package.tgz> \
  --repository <repo-endpoint> \
  [--publisher <publisher>] \
  [--dry-run]

# Examples

# Publish single package
globular pkg publish \
  --file service.myservice_1.0.0_linux_amd64.tgz \
  --repository repo.cluster.local:10003

# Publish all packages in directory
globular pkg publish \
  --dir ./dist \
  --repository repo.cluster.local:10003

# Dry run (validate without uploading)
globular pkg publish \
  --file service.myservice_1.0.0_linux_amd64.tgz \
  --repository repo.cluster.local:10003 \
  --dry-run
```

**Flags**:
- `--file`: Single package file to publish
- `--dir`: Directory of packages to publish (alternative to `--file`)
- `--repository`: Repository service endpoint (required)
- `--publisher`: Override publisher from package manifest
- `--dry-run`: Validate without uploading

**Authentication**: Uses `--token` global flag or `GLOBULAR_TOKEN` environment variable.

---

### Debug Tools

Debug commands provide low-level access to cluster components for troubleshooting.

#### `globular debug agent inventory`

**Purpose**: Query a node agent's current inventory (running services, files, etc.).

```bash
globular debug agent inventory --agent <agent-endpoint>

# Example
globular debug agent inventory --agent node-01:11000
```

**Output**:
- Running systemd units
- Installed service versions
- Configuration files
- Resource usage

---

#### `globular debug agent watch`

**Purpose**: Stream operation events from a specific node agent.

```bash
globular debug agent watch --agent <agent-endpoint> [--op <operation-id>]

# Examples
globular debug agent watch --agent node-01:11000
globular debug agent watch --agent node-01:11000 --op op_xyz789
```

---

## Common Workflows

### 1. Bootstrap a New Cluster

```bash
# Step 1: Bootstrap first node
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --bind 0.0.0.0:10000 \
  --profile gateway \
  --profile dns

# Step 2: Set up DNS
globular cluster dns bootstrap \
  --domain mycluster.local \
  --ipv6 fd12::1 \
  --ipv4 192.168.1.100 \
  --wildcard

# Step 3: Configure HTTP
globular cluster network set \
  --domain mycluster.local \
  --protocol http
```

---

### 2. Add a Node to Existing Cluster

```bash
# On administrator machine:

# Step 1: Create join token (optional, for automation)
TOKEN=$(globular --controller cluster-controller:10000 cluster token create)

# Step 2: On new node, request to join
globular --controller cluster-controller:10000 \
  --node new-node:11000 \
  cluster join --join-token $TOKEN

# Step 3: Verify node joined
globular cluster nodes list
```

---

### 3. Enable HTTPS with ACME

```bash
# Prerequisites:
# - Domain is in managed domains
# - DNS A/AAAA records point to cluster
# - For public domains: DNS must be authoritative or delegated

# Step 1: Add domain to DNS
globular dns domains add cluster.example.com

# Step 2: Configure HTTPS with ACME
globular cluster network set \
  --domain cluster.example.com \
  --protocol https \
  --https-port 443 \
  --acme \
  --email admin@example.com \
  --watch

# Step 3: Verify certificate
sudo openssl x509 -in /etc/globular/tls/fullchain.pem -noout -text
```

---

### 4. Deploy a Service

```bash
# Step 1: Build package
globular pkg build \
  --spec ./myservice/spec.yaml \
  --version 1.0.0 \
  --out ./dist

# Step 2: Publish to repository
globular pkg publish \
  --file ./dist/service.myservice_1.0.0_linux_amd64.tgz \
  --repository repo.cluster.local:10003

# Step 3: Add service profile to node
globular cluster nodes profiles set node_abc123 \
  --profile myservice

# Step 4: Monitor deployment
globular cluster watch --node-id node_abc123
```

---

### 5. Troubleshoot Failed Plan

```bash
# Step 1: Check node health
globular cluster nodes get node_abc123

# Step 2: Get detailed plan
globular cluster plan get node_abc123 --output yaml > plan.yaml

# Step 3: View operation events
globular cluster watch --node-id node_abc123

# Step 4: Check agent inventory
globular debug agent inventory --agent node-abc123:11000

# Step 5: Retry plan
globular cluster plan apply node_abc123 --watch
```

---

## Troubleshooting

### Connection Refused

**Symptom**: `dial tcp: connect: connection refused`

**Solutions**:
1. Verify service is running: `systemctl status globular-nodeagent` or `systemctl status globular-controller`
2. Check firewall rules: `sudo ufw status` or `sudo iptables -L`
3. Verify endpoint address: `--controller` or `--node` flags
4. Test connectivity: `telnet <host> <port>`

---

### TLS Certificate Errors

**Symptom**: `x509: certificate signed by unknown authority`

**Solutions**:
1. Add `--insecure` flag (testing only)
2. Provide CA bundle: `--ca /path/to/ca.pem`
3. Verify certificate chain: `openssl s_client -connect host:port -showcerts`

---

### Permission Denied

**Symptom**: `rpc error: code = PermissionDenied`

**Solutions**:
1. Provide authentication token: `--token <token>`
2. Check token expiration
3. Verify RBAC permissions for operation

---

### ACME Certificate Issuance Failed

**Symptom**: `obtain certificate: acme: error presenting token`

**Common issues**:
1. **Domain not in managed domains**: Run `globular dns domains add <domain>`
2. **DNS not authoritative**: For public domains, ensure NS records point to Globular DNS or use DNS provider integration
3. **TXT record not visible**: Check propagation with `dig TXT _acme-challenge.<domain>`
4. **Firewall blocking**: ACME CA must reach your DNS server (UDP port 53)

**Debug steps**:
```bash
# Verify domain is managed
globular dns domains get

# Test TXT record creation
globular dns txt set _acme-challenge.test.local "test-value"
globular dns txt get _acme-challenge.test.local

# Check node-agent logs
sudo journalctl -u globular-nodeagent -f
```

---

### Plan Execution Stuck

**Symptom**: Plan shows as "in_progress" but never completes

**Solutions**:
1. Check node agent logs: `journalctl -u globular-nodeagent -f`
2. Verify node connectivity to controller
3. Check for deadlocks: `globular debug agent watch --agent <endpoint>`
4. Force plan re-application: `globular cluster plan apply <node-id>`

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GLOBULAR_CONTROLLER` | Override default controller endpoint |
| `GLOBULAR_NODE` | Override default node agent endpoint |
| `GLOBULAR_DNS` | Override default DNS service endpoint |
| `GLOBULAR_TOKEN` | Authentication token (alternative to `--token`) |
| `GLOBULAR_CA` | Path to CA certificate bundle |

**Example**:
```bash
export GLOBULAR_CONTROLLER=cluster.example.com:10000
export GLOBULAR_TOKEN=$(cat ~/.globular-token)

globular cluster nodes list  # Uses exported values
```

---

## Advanced Topics

### Output Formatting

All commands support `--output` flag for different output formats:

```bash
# Human-readable table (default)
globular cluster nodes list

# JSON for scripting
globular cluster nodes list --output json | jq '.nodes[].hostname'

# YAML for configuration
globular cluster plan get node_abc123 --output yaml > node-plan.yaml
```

---

### Scripting with Globular CLI

```bash
#!/bin/bash
set -euo pipefail

# Get all unhealthy nodes
UNHEALTHY=$(globular cluster nodes list --output json | \
  jq -r '.nodes[] | select(.health_status != "healthy") | .node_id')

# Re-apply plans for unhealthy nodes
for node in $UNHEALTHY; do
  echo "Re-applying plan for $node..."
  globular cluster plan apply "$node" --watch
done
```

---

## Contributing

The Globular CLI is open source. Contributions welcome!

- **Repository**: https://github.com/globulario/services
- **CLI Code**: `golang/globularcli/`
- **Issues**: https://github.com/globulario/services/issues

---

## License

See the main repository LICENSE file.
