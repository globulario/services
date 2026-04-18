# Deploying Applications

This page covers deploying services and applications on a Globular cluster using the desired-state model. It explains how to declare what should be running, how the platform converges to match, and how to monitor the deployment process.

## Deployment Model

Globular uses a **declarative deployment model**. Instead of running imperative commands like "install version X on node Y," you declare the desired state: "service X should be at version Y." The platform then figures out which nodes need updates and orchestrates the installation through workflows.

This approach has several advantages:
- **Idempotent**: Declaring the same desired state twice has no effect — the platform detects there's nothing to do
- **Self-healing**: If a service drifts from desired state (manual change, crash, node restart), the platform automatically corrects it
- **Observable**: The gap between desired and actual state is always visible and queryable

## Setting Desired State

### Deploy a Service

To deploy a service (or upgrade to a new version):

```bash
globular services desired set <service-name> <version> [flags]
```

**Example — Deploy PostgreSQL 0.0.3**:

```bash
globular services desired set postgresql 0.0.3 --publisher core@globular.io
```

What happens:
1. CLI sends `UpsertDesiredService` RPC to the Cluster Controller
2. Controller writes a `DesiredService` record to etcd:
   ```
   /globular/resources/DesiredService/postgresql
   {
     "service_id": "postgresql",
     "version": "0.0.3",
     "publisher_id": "core@globular.io",
     "platform": "linux_amd64",
     "build_number": 0
   }
   ```
3. Controller calls `ensureServiceRelease()` — creates a `ServiceRelease` object in PENDING state
4. Release reconciler resolves the artifact from the Repository (verifies it exists and is PUBLISHED)
5. Controller identifies target nodes (all nodes whose profiles include postgresql)
6. For each target node, a workflow is dispatched:
   - FETCH → INSTALL → CONFIGURE → START → VERIFY → COMPLETE
7. As nodes converge, the release status progresses: PENDING → RESOLVED → APPLYING → AVAILABLE

### Deploy Multiple Services

You can set desired state for multiple services:

```bash
globular services desired set postgresql 0.0.3
globular services desired set redis 0.0.1
globular services desired set monitoring 0.0.5
```

The controller processes each independently. Workflows are dispatched subject to the concurrency semaphore (default: 3). If more than 3 workflows are needed, they queue and execute in order.

### Specify Build Number

If multiple builds of the same version exist, specify the build number:

```bash
globular services desired set postgresql 0.0.3 --build-number 2
```

Build numbers differentiate rebuilds of the same version — for example, recompiling with a bug fix without bumping the version number.

## Monitoring Deployment

### Desired State List

View the current desired state and convergence status:

```bash
globular services desired list
```

Output:
```
SERVICE         VERSION  PUBLISHER          NODES   STATUS
authentication  0.0.1    core@globular.io   3/3     INSTALLED
rbac            0.0.1    core@globular.io   3/3     INSTALLED
postgresql      0.0.3    core@globular.io   2/3     APPLYING
redis           0.0.1    core@globular.io   0/2     PENDING
monitoring      0.0.5    core@globular.io   3/3     FAILED
```

**Status meanings**:
- **INSTALLED**: All target nodes have the correct version running and healthy
- **APPLYING**: Workflows in progress — some nodes converged, others still being updated
- **PENDING**: Release created but workflows not yet dispatched
- **FAILED**: Workflows exhausted retries on one or more nodes
- **DEGRADED**: Some nodes converged, others failed
- **ROLLED_BACK**: Deployment was reverted

### Desired State Diff

Preview what would change without modifying anything:

```bash
globular services desired diff
```

This compares:
- What's in the desired state
- What's actually installed on each node
- What's in the repository

### Cluster Health

Real-time cluster health with per-node details:

```bash
globular cluster health
```

Output:
```
CLUSTER STATUS: DEGRADED

NODE         STATUS      LAST SEEN    SERVICES
node-abc123  healthy     2s ago       14/14 running
node-def456  healthy     3s ago       10/10 running
node-ghi789  converging  1s ago       8/10 running (2 installing)
```

### Workflow Progress

View active and recent workflows:

```bash
# All active workflows
globular workflow list --status EXECUTING

# Workflows for a specific service
globular workflow list --service postgresql

# Workflows on a specific node
globular workflow list --node node-ghi789

# Details of a specific workflow
globular workflow get <run-id>
```

Workflow detail output:
```
Run ID:         wf-run-abc123
Correlation:    service/postgresql/node-ghi789
Status:         EXECUTING
Trigger:        DESIRED_DRIFT
Started:        2025-04-12T10:30:00Z
Current Phase:  INSTALL

STEPS:
  1. resolve_artifact     SUCCEEDED  (0.5s)
  2. fetch_package        SUCCEEDED  (12.3s)
  3. verify_checksum      SUCCEEDED  (0.1s)
  4. install_binary       RUNNING    (...)
  5. configure_service    PENDING
  6. start_unit           PENDING
  7. verify_health        PENDING
```

## Removing a Service

To remove a service from the desired state:

```bash
globular services desired remove <service-name>
```

This removes the `DesiredService` record from etcd. The service is no longer managed by the convergence model, but **it is not automatically uninstalled**. The service continues running on nodes where it's already installed, in an `UNMANAGED` state.

To fully remove a service:
1. Remove from desired state: `globular services desired remove <service>`
2. On each node: `sudo systemctl stop <service> && sudo systemctl disable <service>`
3. Remove the installed package record: handled by repair workflow if configured

## Applying Desired State

If you've set multiple desired-state entries and want to trigger immediate convergence (rather than waiting for the reconciliation loop):

```bash
globular services apply-desired
```

This forces the controller to evaluate all desired-state entries and dispatch workflows for any that haven't converged.

## Seeding from Installed

If services are already running (e.g., from bootstrap or manual installation) but don't have desired-state entries:

```bash
# Preview what would be imported
globular cluster get-drift-report

# Import all installed services into desired state
globular services seed
```

The seed operation:
1. Queries each node for installed packages
2. For each installed package without a desired-state entry, creates one
3. The service transitions from `UNMANAGED` to `INSTALLED`

This is idempotent — running it multiple times produces the same result.

## Repair

The repair command performs a comprehensive cross-layer comparison and fixes any misalignments:

```bash
# Dry run: show problems without fixing
globular cluster get-drift-report
```

Output:
```
SERVICE         NODE        DESIRED  INSTALLED  RUNTIME   STATUS
postgresql      node-1      0.0.3    0.0.3      healthy   Installed
postgresql      node-2      0.0.3    0.0.2      healthy   Drifted
postgresql      node-3      0.0.3    —          —         Planned
redis           node-1      —        0.0.1      healthy   Unmanaged
old_service     —           —        —          —         Orphaned (in repo)
```

```bash
# Fix all issues
globular node repair
```

Actions taken:
- **Drifted** (postgresql on node-2): Workflow dispatched to install 0.0.3
- **Planned** (postgresql on node-3): Workflow dispatched to install 0.0.3
- **Unmanaged** (redis on node-1): Imported into desired state via seed
- **Orphaned**: No action (orphaned artifacts in the repository are not automatically cleaned)

## Practical Scenarios

### Scenario 1: First Application Deployment

You have a bootstrapped cluster and want to deploy a custom application:

```bash
# 1. Build and publish the package
globular pkg build --spec specs/myapp_service.yaml --root payload/ --version 1.0.0
globular pkg publish globular-myapp-1.0.0-linux_amd64-1.tgz

# 2. Verify it's in the repository
globular pkg info myapp
# Shows: myapp 1.0.0 PUBLISHED

# 3. Set desired state
globular services desired set myapp 1.0.0 --publisher myteam@example.com

# 4. Watch deployment
globular services desired list
# myapp  1.0.0  0/2  APPLYING

# Wait...
globular services desired list
# myapp  1.0.0  2/2  INSTALLED
```

### Scenario 2: Rolling Upgrade

Upgrade a service with zero downtime across a 3-node cluster:

```bash
# Publish new version
globular pkg publish myapp-1.1.0-linux_amd64-1.tgz

# Set new desired version
globular services desired set myapp 1.1.0

# The semaphore limits concurrent upgrades.
# With 3 nodes, all 3 workflows may run in parallel.
# Each node: stop old → install new → start → verify health

# Monitor progress
globular services desired list
# myapp  1.1.0  1/3  APPLYING  (node-1 done, node-2 and node-3 in progress)
# myapp  1.1.0  3/3  INSTALLED
```

### Scenario 3: Deployment Failure and Recovery

A new version has a bug that causes crashes:

```bash
# Deploy buggy version
globular services desired set myapp 1.2.0

# Workflows start...
globular services desired list
# myapp  1.2.0  0/3  APPLYING

# Node-1 workflow reaches START phase → service crashes
# Node-1 workflow fails with FailureClass: SYSTEMD
# Workflow enters RETRYING with 30s backoff

# After backoff, retry also fails
# After max retries, workflow enters FAILED

globular services desired list
# myapp  1.2.0  0/3  FAILED

# Check what went wrong
globular workflow list --service myapp --status FAILED
# Shows workflow run IDs

globular workflow get <run-id>
# Shows: Step "start_unit" FAILED, FailureClass: SYSTEMD
# Error: "unit myapp exited with status 1"

# Check service logs on the affected node
# (via MCP tools or direct journalctl)

# Roll back to previous version
globular services desired set myapp 1.1.0

# Controller creates new workflows to install 1.1.0
globular services desired list
# myapp  1.1.0  3/3  INSTALLED
```

### Scenario 4: Conditional Deployment by Profile

Deploy a service only to nodes with a specific profile:

```bash
# The service's spec file defines which profiles include it:
# profiles:
#   - compute
#   - ml-worker

# Only nodes with the "compute" or "ml-worker" profile will receive the service
globular services desired set compute-engine 0.0.1

# Nodes without these profiles are unaffected
globular cluster nodes list
# node-1 (core, gateway)     — no compute-engine
# node-2 (core, database)    — no compute-engine
# node-3 (worker, compute)   — compute-engine installed
# node-4 (worker, compute)   — compute-engine installed
```

## What's Next

- [Publishing Services](operators/publishing-services.md): Build and publish your own packages
- [Updating the Cluster](operators/updating-the-cluster.md): Upgrade cluster infrastructure
