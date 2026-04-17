# Workflows

Workflows are the execution backbone of Globular. Every operation that changes the state of the cluster — deploying a service, upgrading infrastructure, repairing drift, bootstrapping a node — is executed as a formal workflow with defined phases, failure classification, automatic retry, and a complete audit trail.

Globular is **workflow-native**. Unlike systems that bolt orchestration onto an existing imperative model, Globular was designed from the ground up so that the Workflow Service is the single path through which all cluster mutations flow. The Cluster Controller decides what should happen; the Workflow Service makes it happen.

## Why Workflows Exist

In traditional infrastructure management, operations are executed through scripts, runbooks, or ad-hoc commands. This creates several problems:

1. **No audit trail**: When a service restarts unexpectedly, there is no record of what triggered it, what steps were executed, or whether the operation completed successfully.
2. **No failure handling**: A script that fails midway leaves the system in an unknown state. The operator must manually investigate and decide what to do next.
3. **No concurrency control**: Multiple operators or automated systems can issue conflicting commands simultaneously, causing race conditions.
4. **No dependency management**: Installing Service B before Service A is ready causes cascading failures that are difficult to diagnose.

Workflows solve all of these problems by providing a structured, observable, auditable execution model for every cluster operation.

## How Workflows Work

### Workflow Runs

Every workflow execution is represented as a **WorkflowRun**. A run has a unique identity, a lifecycle state, and a sequence of steps.

**Key fields of a WorkflowRun**:

| Field | Purpose |
|-------|---------|
| `id` | UUID unique to this specific execution attempt |
| `correlation_id` | Stable identifier across retries (e.g., `service/postgresql/node-1`). Allows you to trace all attempts to deploy a specific service to a specific node. |
| `parent_run_id` | Links a retry to the original failed run, creating a chain of attempts |
| `context` | WorkflowContext containing cluster_id, node_id, node_hostname, component_name, component_version, release_kind, release_object_id |
| `status` | Current lifecycle state (PENDING, EXECUTING, BLOCKED, RETRYING, SUCCEEDED, FAILED, CANCELED, ROLLED_BACK, SUPERSEDED) |
| `current_actor` | Which actor is currently executing (CLUSTER_CONTROLLER, NODE_AGENT, REPOSITORY, etc.) |
| `failure_class` | If failed, the classification of the failure (CONFIG, PACKAGE, DEPENDENCY, NETWORK, REPOSITORY, SYSTEMD, VALIDATION) |
| `retry_count` | Number of retry attempts so far |
| `backoff_until_ms` | Timestamp until which the next retry is delayed |
| `trigger_reason` | Why this workflow was created (DESIRED_DRIFT, BOOTSTRAP, RETRY, MANUAL, DEPENDENCY_UNBLOCKED, UPGRADE, REPAIR) |
| `acknowledged_by` / `acknowledged_at` | Operator acceptance of failure (indicating manual investigation has occurred) |

### Workflow Steps

Each run consists of ordered **WorkflowSteps**. A step represents a single unit of work within a phase:

| Field | Purpose |
|-------|---------|
| `run_id` | Parent workflow run |
| `seq` | Execution order (integer) |
| `step_key` | Machine-readable identifier (e.g., `fetch_package`, `verify_checksum`, `restart_unit`) |
| `title` | Human-readable description |
| `status` | PENDING, RUNNING, SUCCEEDED, FAILED, SKIPPED, BLOCKED |

Steps execute sequentially within a run. If a step fails, subsequent steps are not executed unless the failure handling logic determines they should be skipped or the run should proceed on an alternate path.

### Run Lifecycle

A workflow run progresses through a well-defined lifecycle:

<img src="/docs/assets/diagrams/workflow-lifecycle.svg" alt="Workflow run lifecycle" style="width:100%;max-width:700px">

**PENDING**: The workflow is created but waiting for a semaphore slot. The controller limits concurrent workflows (default: 3) to prevent overwhelming the cluster.

**EXECUTING**: Steps are being processed. The current actor and step are visible for monitoring.

**SUCCEEDED**: All steps completed without error. The release status transitions to AVAILABLE.

**FAILED**: A step failed and all retry attempts are exhausted. The failure class indicates the category of problem.

**BLOCKED**: The workflow cannot proceed because a dependency is not satisfied. For example, Service B requires Service A, which is still being installed. When the dependency is satisfied, a new workflow is created with `DEPENDENCY_UNBLOCKED` as the trigger reason.

**RETRYING**: The workflow failed but is eligible for automatic retry. The `backoff_until_ms` field indicates when the next attempt will begin.

**ROLLED_BACK**: The deployment failed and the system reverted to the previous known-good version.

**CANCELED**: An operator explicitly canceled the workflow.

**SUPERSEDED**: A newer desired-state change arrived, making this workflow obsolete. The new workflow takes over.

## Execution Phases

Workflows are organized into phases, each representing a logical stage of the operation. The standard phases for a service deployment workflow are:

### DECISION Phase

**Actor**: CLUSTER_CONTROLLER

The controller validates the operation before any changes are made:
- Confirms the `DesiredService` entry exists in etcd
- Resolves the target artifact from the Repository service (version, platform, build number)
- Verifies the artifact exists and is in `PUBLISHED` state
- Identifies which nodes need the update based on profile assignments
- Checks for conflicting operations (another workflow for the same service/node)

If the artifact is not found, the step fails with `FailureClass: REPOSITORY`. If the desired entry has been removed (operator changed their mind), the workflow is `CANCELED`.

### FETCH Phase

**Actor**: NODE_AGENT

For each target node, the Node Agent downloads the package:
- Connects to MinIO object storage
- Downloads the `.tgz` archive to the staging directory: `/var/lib/globular/staging/<package_name>/`
- Records the download size and checksum
- If the download fails (network error, MinIO unavailable), the step fails with `FailureClass: NETWORK` or `FailureClass: REPOSITORY`

### INSTALL Phase

**Actor**: INSTALLER (via Node Agent)

The package is installed on the node:
- Computes SHA256 checksum of the downloaded archive
- Compares against the artifact manifest checksum — if mismatch, fails with `FailureClass: VALIDATION`
- Extracts the binary to `/usr/local/bin/`
- Writes or updates the systemd unit file
- Sets file permissions
- If extraction fails (corrupt archive), fails with `FailureClass: PACKAGE`

### CONFIGURE Phase

**Actor**: NODE_AGENT

The service is configured:
- Writes service configuration to etcd (endpoint address, port, TLS settings)
- Updates any local configuration files needed by the service
- Registers the service in the discovery system
- If etcd is unavailable, fails with `FailureClass: NETWORK`

### START Phase

**Actor**: RUNTIME (via Node Agent)

The systemd unit is started:
- Runs `systemctl daemon-reload` if the unit file changed
- Runs `systemctl restart <service_name>`
- Waits for the unit to reach `active (running)` state
- If the unit fails to start, collects the last 50 lines of journal output and fails with `FailureClass: SYSTEMD`

### VERIFY Phase

**Actor**: NODE_AGENT

The service health is confirmed:
- Probes the service's gRPC health endpoint
- Waits for a healthy response (configurable timeout)
- If healthy:
  - Updates the `InstalledPackage` record in etcd with the new version, checksum, and timestamp
  - Cleans up the staging directory (`/var/lib/globular/staging/<package_name>/`)
- If the health check fails after timeout, fails with `FailureClass: SYSTEMD`

### COMPLETE Phase

**Actor**: WORKFLOW_SERVICE

The workflow is finalized:
- Marks all steps as SUCCEEDED
- Updates the run status to SUCCEEDED
- Notifies the controller that convergence is achieved for this node
- The controller updates the `ServiceRelease` status (APPLYING → AVAILABLE or DEGRADED if some nodes failed)

## Failure Classification

When a workflow step fails, the failure is classified into one of seven categories. This classification drives the retry strategy and helps operators diagnose problems quickly.

### CONFIG

**Meaning**: A configuration value is missing, invalid, or inconsistent.

**Examples**: Missing etcd key for service endpoint, invalid port number, malformed YAML.

**Response**: Do not auto-retry. Configuration errors require human intervention — either fixing the configuration in etcd or updating the package spec.

### PACKAGE

**Meaning**: The package archive is corrupt, missing expected files, or incompatible.

**Examples**: Archive fails to extract, binary missing from expected path, spec file malformed.

**Response**: Do not auto-retry. The artifact needs to be rebuilt and republished.

### DEPENDENCY

**Meaning**: A required upstream service is not available.

**Examples**: Service B needs Service A, but A hasn't been installed yet. Service needs etcd, but etcd is still starting.

**Response**: Enter BLOCKED state. When the dependency becomes available, a new workflow is dispatched with `DEPENDENCY_UNBLOCKED` as the trigger.

### NETWORK

**Meaning**: A network connectivity problem prevented the operation.

**Examples**: Cannot reach MinIO to download the package. Cannot connect to the Workflow Service. DNS resolution failed.

**Response**: Auto-retry with exponential backoff. Network problems are typically transient.

### REPOSITORY

**Meaning**: The Repository service is unavailable or the artifact was not found.

**Examples**: MinIO is down. The artifact was yanked after the workflow started. Repository returned 404.

**Response**: Auto-retry with backoff. If the artifact was truly removed, retries will exhaust and the workflow fails.

### SYSTEMD

**Meaning**: The systemd unit failed to start or the health check failed.

**Examples**: Binary crashes on startup. Service fails to bind to its port. Health endpoint returns unhealthy.

**Response**: Auto-retry once (the crash might be transient). If the retry fails, the workflow enters FAILED state. Operators should check `journalctl -u <service>` for the root cause.

### VALIDATION

**Meaning**: A pre-flight validation check failed.

**Examples**: Checksum mismatch between downloaded archive and artifact manifest. Platform incompatibility (arm64 binary on amd64 node). Version constraint violation.

**Response**: Do not auto-retry. Validation failures indicate a fundamental problem that retry cannot fix.

## Concurrency Control

### Semaphore

The controller limits concurrent workflow executions through a semaphore (default capacity: 3). When the semaphore is full, new workflows queue in PENDING state until a slot opens.

This prevents scenarios like:
- An operator sets desired state for 15 services simultaneously
- Without the semaphore, 15 workflows would start in parallel
- Each workflow's FETCH phase downloads a package, consuming network bandwidth
- Each workflow's START phase restarts a service, potentially causing cascading dependency failures
- The cluster becomes overwhelmed and recovery is difficult

With the semaphore, at most 3 workflows execute at a time. The remaining 12 queue and execute in order as slots become available.

### Workflow Health Gate

The controller tracks the success/failure rate of calls to the Workflow Service. If failures accumulate within a 5-minute window (threshold: 5 failures), the **workflow health gate** opens:

- New workflow dispatches are rejected
- The gate enters a cooldown period (30 seconds)
- After cooldown, exactly one probe request is allowed (half-open state)
- If the probe succeeds, the gate closes and normal dispatch resumes
- If the probe fails, the gate stays open for another cooldown

### Reconcile Circuit Breaker

The periodic reconciliation loop has its own circuit breaker, separate from the workflow gate. If the reconciler encounters repeated errors (etcd unavailable, controller in bad state):

- The reconciliation loop pauses
- Heartbeat-driven drift detection continues independently
- The breaker resets after a cooldown period

### 5-Minute Release Backoff

When a `ServiceRelease` enters FAILED or ROLLED_BACK state, the controller enforces a 5-minute cooldown before creating a new workflow attempt. This prevents:

- A broken binary that always crashes from consuming resources in a tight loop
- A misconfigured service from generating hundreds of failed workflow runs
- Operators from being flooded with failure notifications

After the backoff expires, the reconciler creates a new attempt. If desired state hasn't changed, the same failure will occur — but at most once every 5 minutes, giving operators time to investigate.

## Trigger Reasons

Each workflow is tagged with the reason it was created, providing a complete audit trail:

| Trigger | When Used | Example |
|---------|-----------|---------|
| `DESIRED_DRIFT` | Desired state doesn't match installed | Operator ran `desired set`, node reports wrong version |
| `BOOTSTRAP` | Day-0/Day-1 cluster initialization | First node bootstrap, new node join |
| `RETRY` | Automatic retry after previous failure | Previous workflow failed with NETWORK, backoff expired |
| `MANUAL` | Operator explicitly requested | `globular services repair`, manual redeploy |
| `DEPENDENCY_UNBLOCKED` | Blocking dependency now satisfied | Service A installed, unblocking Service B |
| `UPGRADE` | Desired version changed to newer version | `desired set` with higher version number |
| `REPAIR` | Repair command detected misalignment | `globular services repair` found drifted service |

## Actors

Actors are the components responsible for executing specific types of work. The Workflow Service routes each step to the appropriate actor:

| Actor | What It Does | Where It Runs |
|-------|-------------|---------------|
| `CLUSTER_CONTROLLER` | Desired-state validation, artifact resolution, node selection | Controller node (leader) |
| `NODE_AGENT` | Local execution: download, install, configure, health check | Target node |
| `REPOSITORY` | Artifact lookup, checksum verification, manifest resolution | Repository service node |
| `INSTALLER` | Package extraction, binary placement, unit file creation | Target node (via Node Agent) |
| `RUNTIME` | systemd operations: daemon-reload, restart, status check | Target node (via Node Agent) |
| `WORKFLOW_SERVICE` | Step orchestration, retry logic, run completion | Workflow service node |
| `AI_DIAGNOSER` | Automated failure analysis (optional, AI-assisted) | AI executor service |

## Workflow Types

### Service Deployment

The most common workflow type. Triggered when desired state changes or drift is detected:
```
DECISION → FETCH → INSTALL → CONFIGURE → START → VERIFY → COMPLETE
```

### Infrastructure Deployment

For infrastructure components (etcd, MinIO, Prometheus). Similar to service deployment but may include additional coordination steps (etcd member add, MinIO pool expansion):
```
DECISION → FETCH → INSTALL → CONFIGURE → COORDINATE → START → VERIFY → COMPLETE
```

### Bootstrap

For initializing the first node or joining a new node to the cluster:
```
DECISION → CONFIGURE_ETCD → START_ETCD → INSTALL_CORE → START_CORE → VERIFY → COMPLETE
```

### Repair

For fixing detected drift. May skip phases that don't apply (e.g., if the binary is already at the correct version but the service isn't running, skip FETCH and INSTALL):
```
DECISION → [FETCH] → [INSTALL] → START → VERIFY → COMPLETE
```

## Querying Workflows

### List Workflow Runs

View recent workflows for a service:
```bash
globular workflow list --service postgresql
```

View workflows for a specific node:
```bash
globular workflow list --node node-1
```

View failed workflows:
```bash
globular workflow list --status FAILED
```

### Get Workflow Details

View a specific workflow run with all steps:
```bash
globular workflow get <run-id>
```

Output includes:
- Run metadata (id, correlation_id, trigger_reason, status)
- Context (node, service, version)
- Each step with status, timing, and error details
- Failure classification and retry history

### Workflow Status via Service List

The desired-state list shows the current release phase, which reflects the underlying workflow:
```bash
globular services desired list
```

```
SERVICE         VERSION  NODES     STATUS
postgresql      0.0.3    3/3       AVAILABLE
redis           0.0.1    2/3       APPLYING (node-3: FETCH)
monitoring      0.0.5    0/3       FAILED (SYSTEMD)
```

## Practical Scenarios

### Scenario 1: Upgrading PostgreSQL Across 3 Nodes

```bash
globular services desired set postgresql 0.0.4
```

The controller creates a ServiceRelease for postgresql 0.0.4. Three workflows are dispatched (one per node), but only 3 can run concurrently (semaphore limit):

- **Workflow 1** (node-1): FETCH → INSTALL → ... → SUCCEEDED (3 minutes)
- **Workflow 2** (node-2): FETCH → INSTALL → ... → SUCCEEDED (3 minutes)
- **Workflow 3** (node-3): FETCH → INSTALL → ... → SUCCEEDED (3 minutes)

All three run in parallel (within semaphore capacity). After all three succeed, the release transitions to AVAILABLE.

If node-3's workflow fails (binary crashes on startup):
- Workflows 1 and 2 succeed → those nodes run 0.0.4
- Workflow 3 fails with `FailureClass: SYSTEMD` → node-3 still runs 0.0.3
- Release status: DEGRADED (2/3 nodes converged)
- After 5 minutes, a retry workflow is dispatched for node-3
- If the retry also fails, operators investigate with `journalctl -u postgresql` on node-3

### Scenario 2: Dependency Chain (etcd → Controller → Monitoring)

On a fresh node join, multiple services need installation. etcd must start before the controller, and the controller before monitoring:

1. **etcd workflow**: Dispatched immediately → SUCCEEDED
2. **controller workflow**: Dispatched immediately, CONFIGURE phase tries to reach etcd → etcd not yet ready → `FailureClass: DEPENDENCY` → BLOCKED
3. etcd workflow completes → controller workflow unblocked → new run with `DEPENDENCY_UNBLOCKED` → SUCCEEDED
4. **monitoring workflow**: Similar pattern, waits for controller → SUCCEEDED

The workflow system handles this automatically. No manual sequencing is required.

### Scenario 3: Network Outage During Fetch

MinIO is temporarily unreachable during a deployment:

1. Workflow starts, reaches FETCH phase
2. Node Agent attempts to download from MinIO → connection refused
3. Step fails with `FailureClass: NETWORK`
4. Workflow enters RETRYING with 30-second backoff
5. After backoff, retry attempt: MinIO still down → RETRYING with 60-second backoff
6. After second backoff: MinIO is back → download succeeds
7. Remaining phases execute normally → SUCCEEDED
8. Audit shows: 2 failed attempts, 1 success, total delay ~2 minutes

## What's Next

- [Services and Packages](services-and-packages.md): How services are structured, built, packaged, and published
- [Security](security.md): PKI, RBAC, mTLS, and the authentication chain
