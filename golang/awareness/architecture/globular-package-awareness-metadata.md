# Globular Package Awareness Metadata

## Purpose

Globular packages should not enter the cluster as opaque binaries.

A package should describe not only what it contains, but also what it means operationally:

```text
what it owns
what it depends on
what state it touches
what invariants it must respect
what failures are known
what fixes are forbidden
what workflows are allowed
what tests prove the contract
```

This metadata allows Globular to protect both sides of the relationship:

1. Protect the package from being operated incorrectly.
2. Protect the platform from packages that create unsafe runtime behavior.

The awareness metadata becomes the package's operational passport.

---

## Core Idea

Traditional package metadata answers questions like:

```text
What is the package name?
What version is it?
What binary should run?
What dependencies does it have?
```

Globular package awareness metadata answers deeper questions:

```text
What state does this package own?
What cluster invariants does it affect?
During which phase are dependencies required?
Can this service start degraded?
What failures must not be retried forever?
What remediation workflows are allowed?
What fixes are forbidden even if they appear locally correct?
```

This turns packages into operational citizens instead of black boxes.

---

## Package Metadata Layers

A Globular package should be described by multiple metadata layers.

```text
package identity
        ↓
artifact metadata
        ↓
service/runtime metadata
        ↓
awareness contract
        ↓
admission result
        ↓
desired state
        ↓
installed state
        ↓
runtime state
```

The new layer is the **awareness contract**.

---

## Suggested Package Layout

A package may contain:

```text
package/
  package.yaml
  service.yaml
  awareness.yaml
  workflows/
    install.yaml
    repair.yaml
  proto/
    service.proto
  systemd/
    globular-example.service
  tests/
    awareness_tests.yaml
```

For repository-side package metadata:

```text
packages/metadata/<package-name>/
  package.yaml
  service.yaml
  awareness.yaml
```

For built artifacts:

```text
artifacts/<package>/<version>/<platform>/
  package.tar.gz
  manifest.json
  awareness.yaml
```

---

## The Three Main Files

### 1. package.yaml

Describes identity, artifact, version, platform, publisher, and package kind.

### 2. service.yaml

Describes how the service runs: binary, ports, protocol, systemd unit, health checks, resources, and service identity.

### 3. awareness.yaml

Describes operational meaning: ownership, invariants, dependencies, events, failure modes, forbidden fixes, safe workflows, degraded modes, and tests.

The awareness file is the most important new piece.

---

## package.yaml

Example:

```yaml
apiVersion: globular.io/v1
kind: Package

metadata:
  name: repository
  namespace: core
  publisher: core@globular.io
  version: 1.2.12
  build_id: repository-linux_amd64-1.2.12+build.45
  platform: linux_amd64
  package_kind: INFRASTRUCTURE
  description: Globular artifact and package repository service

artifact:
  type: tar.gz
  digest: sha256:...
  size_bytes: 18492000
  source:
    provider: GITHUB_RELEASE
    release: v1.2.12
    asset: repository-linux_amd64.tar.gz

compatibility:
  min_globular_version: 1.2.0
  max_globular_version: ""
  required_capabilities:
    - etcd
    - scylla
    - objectstore

lifecycle:
  install_workflow: install.repository
  update_workflow: update.repository
  uninstall_workflow: uninstall.repository
  repair_workflows:
    - remediate.repository_unreachable
```

---

## package_kind

Recommended package kinds:

```text
INFRASTRUCTURE
PLATFORM_SERVICE
APPLICATION
DEVELOPMENT_TOOL
EXPERIMENTAL
```

### INFRASTRUCTURE

Core substrate services required for cluster operation.

Examples:

```text
etcd
scylla
minio
repository
node-agent
cluster-controller
workflow-service
xds/gateway
```

Awareness contract should be mandatory.

### PLATFORM_SERVICE

Services that extend Globular's operating environment.

Examples:

```text
ai-watcher
ai-memory
doctor
event-service
backup-manager
```

Awareness contract should be mandatory.

### APPLICATION

User-facing applications installed on Globular.

Examples:

```text
admin
media app
CMS module
business app
```

Awareness contract should be recommended in early versions and required later for production mode.

### DEVELOPMENT_TOOL

CLI tools, generators, SDK helpers, test harnesses.

Awareness contract may be optional unless the tool modifies cluster state.

### EXPERIMENTAL

Packages allowed only in relaxed admission mode.

---

## service.yaml

Example:

```yaml
apiVersion: globular.io/v1
kind: ServiceRuntime

service:
  id: repository.RepositoryService
  name: repository
  display_name: Repository Service
  package: repository
  systemd_unit: globular-repository.service
  binary: /usr/local/bin/globular-repository
  user: globular
  group: globular

network:
  protocol: grpc
  port_name: repository-grpc
  default_port: 10003
  tls: required
  service_mesh: true

health:
  startup_probe:
    rpc: /repository.RepositoryService/Health
    timeout_seconds: 5
    failure_threshold: 12

  liveness_probe:
    rpc: /repository.RepositoryService/Health
    interval_seconds: 15
    timeout_seconds: 5
    failure_threshold: 4

  readiness_probe:
    rpc: /repository.RepositoryService/Ready
    interval_seconds: 10
    timeout_seconds: 5
    failure_threshold: 3

resources:
  cpu_request: 100m
  memory_request: 128Mi
  memory_limit: 512Mi

security:
  mtls: required
  jwt: required
  cluster_id_required: true
  rbac_actions:
    - repository.artifact.read
    - repository.artifact.write
    - repository.manifest.read
```

---

# awareness.yaml

The awareness contract is the operational meaning of the package.

It tells Globular:

```text
This is what I own.
This is what I depend on.
This is when I depend on it.
This is what I must never do.
This is how I fail.
This is how I may be repaired.
This is what tests prove my contract.
```

---

## Minimal awareness.yaml

```yaml
apiVersion: globular.io/v1
kind: AwarenessContract

service: repository.RepositoryService
package: repository
package_kind: INFRASTRUCTURE

summary: >
  Repository stores package metadata and artifact references. It must preserve
  metadata authority even when blob storage is degraded.

owns:
  etcd_keys:
    - /globular/repository/config
  scylla_tables:
    - repository_artifacts
    - repository_manifests
  minio_buckets:
    - artifacts

invariants:
  - repository.metadata_first

depends_on:
  - service: scylla
    phase: metadata_read
    required: true

  - service: minio
    phase: blob_read
    required: true

  - service: minio
    phase: metadata_read
    required: false

emits:
  - repository.artifact.published
  - repository.artifact.missing_blob
  - repository.mode.changed

safe_degraded_modes:
  - DEGRADED
  - READ_ONLY
  - LOCAL_ONLY

forbidden_fixes:
  - return_not_found_when_ledger_exists
  - block_metadata_read_on_minio_down
  - rewrite_artifact_digest_without_new_build_id

remediation_workflows:
  - remediate.repository_unreachable
  - remediate.repository_missing_blob

required_tests:
  - TestRepositoryMetadataFirstWhenMinIODown
  - TestRepositoryModeDegradedWhenMirrorUnavailable
```

---

## Full awareness.yaml Schema

```yaml
apiVersion: globular.io/v1
kind: AwarenessContract

service: string
package: string
package_kind: string
summary: string

owns:
  etcd_keys: []
  scylla_tables: []
  minio_buckets: []
  filesystem_paths: []
  systemd_units: []
  dns_zones: []
  xds_resources: []

reads:
  etcd_keys: []
  scylla_tables: []
  minio_buckets: []
  filesystem_paths: []

writes:
  etcd_keys: []
  scylla_tables: []
  minio_buckets: []
  filesystem_paths: []

invariants: []

protects:
  invariants: []
  state: []
  services: []
  workflows: []

depends_on:
  - service: string
    phase: string
    required: bool
    reason: string
    safe_escape_hatch: string

emits: []
subscribes: []

doctor_findings:
  produces: []
  responds_to: []

failure_modes: []
forbidden_fixes: []
safe_escape_hatches: []
safe_degraded_modes: []

remediation_workflows: []
blocked_when: []
safe_when: []
unsafe_when: []

required_tests: []
required_searches: []

admission:
  strict: bool
  allow_unknown_dependencies: bool
  allow_unknown_events: bool
  allow_privileged_state_writes: bool
```

---

## Ownership Metadata

Ownership describes the resources for which the service is authoritative.

Example:

```yaml
owns:
  etcd_keys:
    - /globular/objectstore/config
  filesystem_paths:
    - /etc/globular/minio.env
    - /etc/globular/minio.distributed.conf
  systemd_units:
    - globular-minio.service
```

Ownership matters because Globular can detect conflict:

```text
Two packages claim the same etcd key.
A package writes a path it does not own.
A remediation tries to modify a protected resource.
```

---

## Read and Write Metadata

A service may read resources it does not own.
A service may write resources only if explicitly allowed.

Example:

```yaml
reads:
  etcd_keys:
    - /globular/objectstore/config

writes:
  filesystem_paths:
    - /etc/globular/minio.env
```

The graph can detect suspicious behavior:

```text
Service writes to protected etcd key without ownership.
Service reads runtime state but has no dependency edge.
Service modifies service unit owned by another subsystem.
```

---

## Dependency Metadata

Dependencies must be phase-specific.

A dependency is not just:

```text
A depends on B
```

It is:

```text
A depends on B during phase P, and that dependency is required or optional.
```

Example:

```yaml
depends_on:
  - service: repository
    phase: package_install
    required: true
    reason: node-agent needs repository to fetch package artifacts

  - service: local_bom_cache
    phase: bootstrap_recovery
    required: true
    reason: local BOM cache breaks repository recovery cycle

  - service: minio
    phase: metadata_read
    required: false
    reason: repository can read metadata from Scylla when MinIO is degraded
```

This allows Globular to detect dangerous cycles.

---

## Recommended Dependency Phases

```text
bootstrap
bootstrap_recovery
startup
readiness
package_install
package_update
package_uninstall
metadata_read
blob_read
blob_write
runtime_reconcile
doctor_assessment
remediation
backup
restore
shutdown
```

---

## Required vs Optional Dependencies

Required dependency:

```yaml
required: true
```

Means the phase cannot safely complete without it.

Optional dependency:

```yaml
required: false
```

Means the service can degrade or skip behavior.

This distinction is critical.

A required recovery cycle may deadlock the cluster.
An optional runtime cycle may only reduce capability.

---

## Safe Escape Hatches

A dependency cycle may be acceptable if it has a declared escape hatch.

Example:

```yaml
depends_on:
  - service: repository
    phase: package_install
    required: true
    safe_escape_hatch: local_bom_cache
```

The graph can classify:

```text
required cycle with no escape hatch: DANGEROUS
required cycle with escape hatch: WARNING
optional cycle: SAFE
```

---

## Invariant Metadata

Invariants are the laws the package must respect.

Example:

```yaml
invariants:
  - objectstore.topology_contract
  - runtime.installed_state_not_liveness
```

The package may also protect invariants:

```yaml
protects:
  invariants:
    - objectstore.topology_contract
```

This lets Globular answer:

```text
Which package protects this invariant?
Which code files enforce it?
Which service could violate it?
Which tests prove it?
```

---

## Forbidden Fixes

Forbidden fixes are repairs that appear locally reasonable but violate global architecture.

Example:

```yaml
forbidden_fixes:
  - start_minio_from_local_health_check
  - infer_minio_membership_from_disk
  - wipe_minio_sys_without_approved_transition
  - blind_reconcile_retry
```

This is one of the most important fields for AI safety.

Before Claude or Codex edits code, the agent context can say:

```text
Do not implement this fix. It violates the package awareness contract.
```

---

## Failure Modes

Failure modes describe known ways the service can break.

Example:

```yaml
failure_modes:
  - id: repository.minio.recovery_cycle
    symptoms:
      - repository cannot fetch artifacts
      - minio package cannot be repaired because repository is unavailable
      - node-agent waits for repository during package install
    root_cause: >
      Repository blob reads require MinIO, while MinIO repair requires repository.
    architecture_fix: >
      Use local BOM cache and metadata-first repository behavior to break the cycle.
```

These can also live in `docs/awareness/failure_modes.yaml` and be linked by ID.

---

## Events

Services should declare emitted and subscribed events.

Example:

```yaml
emits:
  - service.exited
  - repository.mode.changed
  - package.install.completed

subscribes:
  - objectstore.topology.changed
  - workflow.result.committed
```

This allows Globular to detect:

```text
event emitted but nobody handles it
event subscribed but never emitted
remediation event not linked to workflow
unsafe event storm possibility
```

---

## Doctor Findings

Packages can declare doctor findings they produce or respond to.

Example:

```yaml
doctor_findings:
  produces:
    - repository_unreachable
    - repository_metadata_blob_mismatch
  responds_to:
    - objectstore_contract_missing
```

This links doctor output directly to source, package, invariant, and remediation workflows.

---

## Remediation Workflows

A package should declare which workflows are allowed to repair it.

Example:

```yaml
remediation_workflows:
  - remediate.repository_unreachable
  - remediate.repository_missing_blob
```

The awareness graph should reject unsafe action paths such as:

```text
restart service directly
wipe state directly
mark healthy without verification
clear action without durable result
```

Workflow-service remains the action engine.
Awareness metadata only defines what is safe.

---

## Safe Degraded Modes

Some services can operate without all dependencies.

Example:

```yaml
safe_degraded_modes:
  - DEGRADED
  - READ_ONLY
  - LOCAL_ONLY
```

This allows Globular to avoid binary thinking:

```text
healthy or dead
```

Instead:

```text
FULL
DEGRADED
READ_ONLY
LOCAL_ONLY
BLOCKED
UNKNOWN
```

---

## Runtime Blocking Rules

A package can declare when it must block instead of retrying forever.

Example:

```yaml
blocked_when:
  - condition: missing_native_library
    classification: deterministic
    reason: rerunning install will produce the same result
    unblock_when: package artifact changes or missing library becomes available

  - condition: desired_dependency_unresolvable
    classification: deterministic
    reason: no package can satisfy dependency
    unblock_when: dependency appears in repository or desired state changes
```

This directly supports the invariant:

```text
convergence.no_infinite_retry
```

---

## safe_when and unsafe_when

Packages can declare operation safety conditions.

Example:

```yaml
safe_when:
  - action: restart_minio
    condition: node is present in ObjectStoreDesiredState
    required_verification: objectstore topology generation matches applied generation

unsafe_when:
  - action: restart_minio
    condition: node is not present in ObjectStoreDesiredState
    reason: violates objectstore topology contract
```

This gives ai-watcher and Claude/Codex a clear boundary.

---

## Required Tests

Tests are part of the operational contract.

Example:

```yaml
required_tests:
  - TestMinioHeldWhenNodeNotInPool
  - TestTopologyRenderParity
  - TestApprovedTransitionRequiredForWipe
```

The awareness graph can use this for:

```text
agent context generation
release impact analysis
required test selection
PR review
CI gating
```

---

## Required Searches

AI agents often need search terms before editing.

Example:

```yaml
required_searches:
  - ObjectStoreDesiredState
  - enforceMinioHeld
  - nodeIPInPool
  - TopologyTransition
  - rendered_generation
```

The agent context generator can include these terms so Claude/Codex searches the right places first.

---

# Awareness Admission

Before a package becomes desired state, Globular should run awareness admission.

```text
artifact discovered
        ↓
manifest read
        ↓
awareness.yaml read
        ↓
graph inserted
        ↓
contract validation
        ↓
cycle detection
        ↓
invariant conflict check
        ↓
admission result
        ↓
desired state write
```

This creates a new conceptual layer:

```text
Artifact → Contract → Desired → Installed → Runtime
```

---

## Admission Result Types

```text
ADMITTED
ADMITTED_WITH_WARNINGS
REQUIRES_APPROVAL
BLOCKED
QUARANTINED
```

### ADMITTED

The contract is complete and no dangerous issue was found.

### ADMITTED_WITH_WARNINGS

The package can proceed, but weak metadata or low-risk issues were found.

### REQUIRES_APPROVAL

The package touches privileged resources, declares risky dependencies, or has unknown invariant impact.

### BLOCKED

The package violates a known invariant or creates a dangerous required cycle.

### QUARANTINED

The package is structurally invalid, missing required contract metadata, has checksum mismatch, or conflicts with protected platform state.

---

## Admission Policy by Package Kind

Recommended defaults:

```yaml
policy:
  INFRASTRUCTURE:
    awareness_required: true
    block_on_dangerous_cycle: true
    block_on_unknown_state_write: true
    require_tests: true

  PLATFORM_SERVICE:
    awareness_required: true
    block_on_dangerous_cycle: true
    block_on_unknown_state_write: true
    require_tests: true

  APPLICATION:
    awareness_required: false
    warn_on_missing_contract: true
    block_on_protected_state_write: true
    block_on_dangerous_cycle: true

  DEVELOPMENT_TOOL:
    awareness_required: false
    block_on_cluster_state_write_without_contract: true

  EXPERIMENTAL:
    awareness_required: false
    require_manual_approval: true
```

---

## Admission Checks

The awareness admission controller should check:

```text
1. Is awareness.yaml present when required?
2. Is package_kind valid?
3. Are owned resources already owned by another package?
4. Does the package write protected state it does not own?
5. Does it depend on unknown services?
6. Does it create required dependency cycles?
7. Does it create recovery/bootstrap/package_install deadlocks?
8. Does it declare remediation workflows that exist?
9. Does it reference known invariants?
10. Does it request privileged actions?
11. Does it emit unknown critical events?
12. Does it have required tests for critical invariants?
13. Does it violate platform forbidden fixes?
```

---

## Example Admission Failure

Package declares:

```yaml
depends_on:
  - service: repository
    phase: package_install
    required: true

  - service: minio
    phase: bootstrap_recovery
    required: true
```

Existing graph contains:

```text
repository → minio during blob_read required=true
minio repair → node-agent during package_install required=true
node-agent → repository during package_install required=true
```

Admission result:

```text
BLOCKED
Reason: required dependency cycle during bootstrap_recovery/package_install.
Cycle: node-agent → repository → minio → node-agent
Fix: declare local_bom_cache escape hatch or make dependency optional during recovery.
```

---

# Integration With Awareness Graph

When a package is imported, Globular inserts metadata into the graph.

Examples:

```text
package(repository) owns scylla_table(repository_artifacts)
package(repository) owns minio_bucket(artifacts)
repository depends_on minio phase=blob_read required=true
repository depends_on minio phase=metadata_read required=false
repository enforces repository.metadata_first
repository forbids block_metadata_read_on_minio_down
repository tested_by TestRepositoryMetadataFirstWhenMinIODown
```

The graph can then answer:

```text
What breaks if this file changes?
What invariant protects this state?
Can this service restart safely?
What tests are required for this package?
Does this new package create a recovery deadlock?
```

---

# Integration With AI Agents

Before editing package code, Claude/Codex should run:

```bash
globular awareness agent-context --task "<task>" --package <package>
```

If editing files:

```bash
globular awareness impact --file <file>
```

The generated context should include:

```text
package contract
owned state
relevant invariants
forbidden fixes
known failure modes
required tests
required searches
safe remediation workflows
phase-specific dependency risks
```

This prevents local fixes from violating global architecture.

---

# Integration With workflow-service

workflow-service remains the only action engine.

The awareness contract does not execute repairs.
It only says what repairs are allowed, blocked, or require approval.

Example:

```yaml
remediation_workflows:
  - remediate.repository_unreachable

forbidden_fixes:
  - restart_repository_without_backend_health_check
```

workflow-service executes:

```text
assess
approve
execute
verify
record receipt
```

awareness-graph informs:

```text
allowed workflow
required verification
forbidden shortcut
related invariant
```

---

# Integration With ai-watcher

Current simple model:

```text
event → incident → action
```

Awareness-aware model:

```text
event
→ package contract lookup
→ invariant impact
→ failure mode match
→ forbidden fix check
→ allowed workflow selection
→ workflow-service dispatch
```

Example:

```text
service.exited: globular-minio.service
```

ai-watcher asks:

```text
Is this node in ObjectStoreDesiredState?
Is MinIO allowed to restart here?
What invariant protects MinIO startup?
What workflow is allowed?
What fixes are forbidden?
```

If unsafe:

```text
Do not restart.
Emit finding: objectstore.hold_gate_respected
```

---

# Integration With doctor

Doctor findings should link to package contracts.

Example:

```text
doctor finding: objectstore_contract_missing
→ invariant: objectstore.topology_contract
→ package: minio
→ files: minio_runtime_render.go, join_script.go
→ workflows: apply_objectstore_topology
→ forbidden fixes: local MinIO start
```

This turns doctor from a symptom detector into an architecture-aware diagnostic surface.

---

# Integration With Release Pipeline

When source changes:

```text
changed files
→ affected package
→ affected service
→ affected invariants
→ affected failure modes
→ required tests
→ release risk
```

Example output:

```text
Changed package:
- repository

Impacted invariants:
- repository.metadata_first
- desired.build_id_immutable

Forbidden fixes nearby:
- return_not_found_when_ledger_exists
- rewrite_artifact_digest_without_new_build_id

Required tests:
- TestRepositoryMetadataFirstWhenMinIODown
- TestDesiredBuildIDImmutableAfterResolution

Risk:
HIGH, because repository metadata and artifact authority are touched.
```

---

# First Core Package Contracts To Write

Start with the packages that can damage convergence.

```text
1. node-agent
2. cluster-controller
3. workflow-service
4. repository
5. minio/objectstore
6. scylla
7. etcd
8. gateway/xDS
9. doctor
10. ai-watcher
```

---

# First Platform-Wide Invariants

The first awareness contracts should reference these invariants:

```text
1. objectstore.topology_contract
   MinIO cannot start outside ObjectStoreDesiredState.

2. install.result.atomic_commit
   installed-state, result promotion, and action cleanup commit together.

3. convergence.no_infinite_retry
   deterministic failures must not retry forever.

4. repository.metadata_first
   ledger/metadata authority must not depend fully on blob backend availability.

5. runtime.installed_state_not_liveness
   installed-state cannot prove runtime is alive.

6. workflow.backend_health_gate
   workflow dispatch must stop when Scylla/backend health is broken.

7. desired.build_id_immutable
   desired build_id must not mutate after resolution.
```

---

# Example: MinIO Awareness Contract

```yaml
apiVersion: globular.io/v1
kind: AwarenessContract

service: objectstore.MinIO
package: minio
package_kind: INFRASTRUCTURE

summary: >
  MinIO provides object storage for artifacts, backups, and blob data.
  It must only run on nodes admitted by ObjectStoreDesiredState.

owns:
  filesystem_paths:
    - /etc/globular/minio.env
    - /etc/globular/minio.distributed.conf
  systemd_units:
    - globular-minio.service

reads:
  etcd_keys:
    - /globular/objectstore/config

writes:
  filesystem_paths:
    - /etc/globular/minio.env
    - /etc/globular/minio.distributed.conf

invariants:
  - objectstore.topology_contract

protects:
  invariants:
    - objectstore.topology_contract

forbidden_fixes:
  - start_minio_from_local_health_check
  - infer_minio_membership_from_disk
  - wipe_minio_sys_without_approved_transition
  - use_round_robin_dns_for_minio_writes

safe_when:
  - action: start_minio
    condition: node is present in ObjectStoreDesiredState
    required_verification: rendered_generation matches desired_generation

unsafe_when:
  - action: start_minio
    condition: node is absent from ObjectStoreDesiredState
    reason: violates objectstore topology contract

emits:
  - objectstore.started
  - objectstore.stopped
  - objectstore.topology.applied

remediation_workflows:
  - apply_objectstore_topology
  - remediate.objectstore_degraded

required_tests:
  - TestMinioHeldWhenNodeNotInPool
  - TestTopologyRenderParity
  - TestApprovedTransitionRequiredForWipe

required_searches:
  - ObjectStoreDesiredState
  - enforceMinioHeld
  - nodeIPInPool
  - TopologyTransition
  - rendered_generation
```

---

# Example: Workflow Service Awareness Contract

```yaml
apiVersion: globular.io/v1
kind: AwarenessContract

service: workflow.WorkflowService
package: workflow-service
package_kind: INFRASTRUCTURE

summary: >
  workflow-service executes bounded, auditable workflows. It must not dispatch
  new actions when backend persistence is unhealthy.

owns:
  scylla_tables:
    - workflow_runs
    - workflow_steps
    - workflow_receipts

invariants:
  - workflow.backend_health_gate
  - convergence.no_infinite_retry

forbidden_fixes:
  - dispatch_when_backend_session_nil
  - treat_all_failures_as_transient
  - retry_deterministic_failure_forever
  - skip_receipt_recording_on_success

safe_degraded_modes:
  - READ_ONLY
  - BLOCKED

blocked_when:
  - condition: scylla_session_unavailable
    classification: transient_or_backend_unhealthy
    reason: workflow receipts cannot be durably recorded
    unblock_when: backend health gate returns healthy

emits:
  - workflow.started
  - workflow.blocked
  - workflow.completed
  - workflow.failed
  - workflow.backend_unhealthy

remediation_workflows:
  - remediate.workflow_backend_unhealthy

required_tests:
  - TestWorkflowDispatchBlockedWhenBackendUnhealthy
  - TestDeterministicFailureDoesNotRetryForever
  - TestWorkflowReceiptRecordedBeforeCompletion

required_searches:
  - RUN_STATUS_BLOCKED
  - backend health gate
  - workflow receipts
  - deterministic failure
  - retry classification
```

---

# Example: Application Awareness Contract

```yaml
apiVersion: globular.io/v1
kind: AwarenessContract

service: media.MediaService
package: media
package_kind: APPLICATION

summary: >
  Media service stores uploaded media, generates thumbnails, and schedules
  conversion jobs.

owns:
  scylla_tables:
    - media_objects
    - media_jobs
  minio_buckets:
    - media

reads:
  minio_buckets:
    - media

writes:
  minio_buckets:
    - media

invariants:
  - media.original_blob_is_authority
  - media.conversion_failure_is_not_upload_failure
  - media.jobs_are_idempotent

depends_on:
  - service: repository
    phase: startup
    required: true

  - service: objectstore
    phase: upload_write
    required: true

  - service: workflow-service
    phase: async_conversion
    required: false

emits:
  - media.uploaded
  - media.conversion.started
  - media.conversion.failed
  - media.conversion.completed

forbidden_fixes:
  - delete_original_media_on_conversion_failure
  - retry_conversion_forever_without_backoff
  - mark_upload_failed_when_thumbnail_generation_fails

safe_degraded_modes:
  - READ_ONLY
  - CONVERSION_DISABLED

blocked_when:
  - condition: objectstore_unavailable_during_upload
    classification: dependency_unavailable
    reason: original blob cannot be durably written
    unblock_when: objectstore writable again

remediation_workflows:
  - remediate.media_conversion_stuck
  - remediate.media_missing_thumbnail

required_tests:
  - TestOriginalBlobPreservedWhenConversionFails
  - TestConversionJobIdempotent
  - TestUploadBlockedWhenObjectstoreUnavailable
```

---

# How This Protects New Services

When a new service is created, the awareness contract tells Globular how to operate it safely.

Globular can know:

```text
this service can start degraded
this service must block during deterministic failure
this service must not retry forever
this service has one safe repair workflow
this service owns these tables and buckets
```

The new service is protected from bad automation.

---

# How This Protects Globular From New Services

A new service can damage the platform if it creates hidden operational risks.

Awareness admission can detect:

```text
required recovery cycles
writes to protected etcd keys
unknown privileged actions
unsafe restart workflows
event storms
missing degraded mode
missing tests for critical invariants
```

The platform is protected from bad packages.

---

# AI-Generated Services

If Claude or Codex creates a new Globular service, it must create the awareness contract too.

A generated service is not complete until it includes:

```text
source code
proto API
package metadata
service runtime metadata
workflow definitions
awareness.yaml
tests proving the awareness contract
```

This prevents AI from generating a service that runs but cannot be operated safely.

---

# First Implementation Order

## Step 1: Define awareness.yaml schema

Add parser structs and validation.

## Step 2: Add awareness metadata to package import

When repository imports a package, read awareness.yaml if present.

## Step 3: Insert package contract into awareness graph

Create nodes and edges for:

```text
package
service
owned state
dependencies
invariants
failure modes
forbidden fixes
events
tests
workflows
```

## Step 4: Add admission checks

Start with:

```text
missing required contract
protected state write
duplicate ownership
unknown service dependency
required dependency cycle
forbidden fix conflict
missing required tests for INFRASTRUCTURE packages
```

## Step 5: Add CLI commands

```bash
globular awareness package-check --path <package>
globular awareness package-context --package <name>
globular awareness package-impact --package <name>
globular awareness cycles --package <name>
```

## Step 6: Gate only core infrastructure first

Do not block all applications immediately.

Recommended first policy:

```text
INFRASTRUCTURE: block on missing/invalid awareness contract
PLATFORM_SERVICE: block on missing/invalid awareness contract
APPLICATION: warn on missing awareness contract
EXPERIMENTAL: require approval
```

---

# Definition of Done for V1

```text
1. awareness.yaml schema exists.
2. package import can read awareness.yaml.
3. awareness graph stores package/service/state/invariant/failure metadata.
4. package-check validates ownership, dependency phases, and forbidden fixes.
5. cycle detector can classify required package dependency cycles.
6. infrastructure packages require awareness contracts.
7. agent-context includes package awareness metadata.
8. tests prove that unsafe package metadata is blocked or warned.
```

---

# Final Design Statement

Globular packages should carry an operational contract.

That contract tells the platform:

```text
what the package owns
what it depends on
what phases matter
what invariants it touches
what failures are known
what repairs are allowed
what fixes are forbidden
what tests prove the contract
```

The awareness graph stores those contracts and connects them to source code, runtime state, doctor findings, workflows, releases, and AI memory.

This gives Globular a new safety layer:

```text
Artifact → Contract → Desired → Installed → Runtime
```

The result is simple but powerful:

```text
Globular does not merely install services.
Globular understands the operational contract of every service it admits.
```

That is how new services can become safer by default, and how the whole platform can defend itself from unsafe services before they damage convergence.
