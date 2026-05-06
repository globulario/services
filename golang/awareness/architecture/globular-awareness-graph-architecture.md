# Globular Awareness Graph Architecture

## Purpose

The **Globular Awareness Graph** is the missing connective layer between source code, runtime state, AI memory, doctor findings, workflow execution, and AI coding agents such as Claude or Codex.

Its purpose is to make Globular understandable to AI agents by connecting:

```text
source code → packages → services → state → invariants → runtime events → failures → remediations → memory
```

This gives AI agents a compressed, queryable view of the system before they edit code or suggest architecture changes.

The goal is not to make the model magically understand the entire codebase. The goal is to make the **right global context appear before local changes are made**.

---

## Problem

Today, Claude/Codex often sees only fragments:

```text
file
function
error log
test failure
local diff
```

But Globular requires decisions based on:

```text
state model
service lifecycle
reconcile law
failure history
runtime dependency phase
invariants
forbidden fixes
```

Without this context, an AI agent can make a locally correct patch that violates a global invariant.

Example:

```text
Local symptom:
MinIO service is stopped.

Bad local fix:
Restart MinIO.

Global invariant:
MinIO must not start unless the node is present in ObjectStoreDesiredState.

Result:
The fix violates the objectstore topology contract.
```

The awareness graph exists to prevent this class of mistake.

---

## Core Idea

Globular should expose a queryable nervous system:

```text
Source Awareness Graph
        ↓
Package / Release Awareness
        ↓
Runtime Awareness
        ↓
Failure Memory
        ↓
Workflow-Gated Remediation
        ↓
Agent Context for Claude/Codex
```

The system should tell AI agents:

```text
Before you edit this code, here is what this code protects.
Before you restart this service, here is what dependency cycle you may trigger.
Before you retry this workflow, here is why retry may never converge.
```

---

## Subsystem Name

Recommended name:

```text
awareness-graph
```

Alternative names:

```text
source-awareness
source-runtime-awareness
globular-awareness
```

Preferred name:

```text
awareness-graph
```

Reason: the subsystem is not only about source code. It connects source, runtime, memory, workflows, packages, invariants, and failure modes.

---

## Responsibilities

The awareness graph is responsible for:

```text
1. Building a graph from source code, proto, workflows, packages, services, tests, and manual invariant files.

2. Storing the graph in SQLite.

3. Connecting source-level facts to runtime-level facts.

4. Storing known failure modes and forbidden fixes.

5. Generating compact context for AI agents.

6. Feeding ai-watcher and workflow-service with better diagnosis context.

7. Supporting impact analysis for code changes and releases.

8. Helping detect deadlocks, convergence loops, dependency cycles, and invariant risks.
```

It is not responsible for:

```text
1. Editing code directly.
2. Executing remediation directly.
3. Replacing workflow-service.
4. Replacing doctor.
5. Automatically inferring every invariant from code.
```

The graph informs. The workflow-service acts.

---

## High-Level Architecture

```text
+---------------------------------------------------------+
|                  Globular Awareness Layer               |
+---------------------------------------------------------+

        +--------------------+
        | Source Extractors  |
        +--------------------+
          | Go AST
          | Proto descriptors
          | Workflow YAML
          | Package manifests
          | Systemd specs
          | Tests
          | Docs/invariants YAML
          v

        +--------------------+
        | Awareness Graph DB |
        | SQLite             |
        +--------------------+
          | nodes
          | edges
          | facts
          | invariants
          | failure modes
          | memories
          v

+----------------+     +----------------+     +----------------+
| Agent Context  |     | Runtime Bridge |     | Release Impact |
| Claude/Codex   |     | ai-watcher     |     | package/BOM    |
+----------------+     +----------------+     +----------------+

          v                    v                      v

+----------------+     +----------------+     +----------------+
| workflow-svc   |     | ai-memory      |     | doctor         |
| safe actions   |     | scar tissue    |     | invariants     |
+----------------+     +----------------+     +----------------+
```

---

## Core Graph Model

The graph has two core primitives:

```text
nodes
edges
```

The key idea is that relationships are not only code relationships. The graph must model operational meaning.

Bad graph:

```text
A calls B
```

Useful graph:

```text
A enforces invariant B
B protects etcd key C
C controls service D
D affects runtime convergence
test E proves B
failure mode F happens when B is violated
```

That is the difference between a code index and a reasoning substrate.

---

## Node Types

Recommended initial node types:

```text
source_file
symbol
go_package
proto_service
proto_message
rpc_method
globular_service
package
release
workflow
workflow_step
systemd_unit
etcd_key
scylla_table
minio_bucket
event_type
doctor_finding
invariant
failure_mode
forbidden_fix
safe_escape_hatch
test
runtime_state
memory_entry
remediation_workflow
dependency_phase
```

---

## Edge Types

Recommended initial edge types:

```text
defines
calls
imports
reads
writes
owns
depends_on
produces
runs_as
emits
subscribes
protects
enforces
violates
tested_by
remediated_by
records
recalls
affects
blocks
unblocks
requires
forbids
safe_when
unsafe_when
```

Important: `depends_on` edges should support a `phase` and `required` attribute.

Example:

```text
node-agent depends_on repository during package_install, required=true
repository depends_on MinIO during blob_read, required=true
repository depends_on MinIO during metadata_read, required=false
```

This allows the graph to detect dangerous phase-specific cycles.

---

## SQLite Schema

Start with SQLite. It is simple, portable, inspectable, and fits Globular’s “boring machine” philosophy.

```sql
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT,
    summary TEXT,
    metadata_json TEXT,
    created_at INTEGER,
    updated_at INTEGER
);

CREATE TABLE edges (
    src TEXT NOT NULL,
    kind TEXT NOT NULL,
    dst TEXT NOT NULL,
    phase TEXT,
    required INTEGER DEFAULT 0,
    confidence REAL DEFAULT 1.0,
    metadata_json TEXT,
    PRIMARY KEY (src, kind, dst, COALESCE(phase, ''))
);

CREATE INDEX idx_nodes_type ON nodes(type);
CREATE INDEX idx_nodes_name ON nodes(name);
CREATE INDEX idx_edges_src ON edges(src);
CREATE INDEX idx_edges_dst ON edges(dst);
CREATE INDEX idx_edges_kind ON edges(kind);
CREATE INDEX idx_edges_phase ON edges(phase);
```

Specialized tables for faster queries:

```sql
CREATE TABLE invariants (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    severity TEXT,
    status TEXT,
    metadata_json TEXT
);

CREATE TABLE failure_modes (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    symptoms_json TEXT,
    root_cause TEXT,
    architecture_fix TEXT,
    metadata_json TEXT
);

CREATE TABLE agent_context_cache (
    cache_key TEXT PRIMARY KEY,
    task TEXT NOT NULL,
    context_markdown TEXT NOT NULL,
    created_at INTEGER
);

CREATE TABLE graph_builds (
    id TEXT PRIMARY KEY,
    repo_root TEXT NOT NULL,
    git_commit TEXT,
    release_id TEXT,
    created_at INTEGER,
    stats_json TEXT
);
```

---

## Manual Truth Files

Automatic extraction can find structure, but invariants must be declared manually at first.

Recommended files:

```text
docs/awareness/invariants.yaml
docs/awareness/state_model.yaml
docs/awareness/services.yaml
docs/awareness/failure_modes.yaml
docs/awareness/convergence_rules.yaml
docs/awareness/forbidden_fixes.yaml
docs/awareness/remediation_workflows.yaml
```

These files are the laws of the system.

---

## Example Invariant

```yaml
id: objectstore.topology_contract
title: MinIO topology contract
severity: critical
summary: >
  MinIO may only run on nodes present in ObjectStoreDesiredState.
  Node-agent must never infer objectstore membership from local disk state.

protects:
  state:
    - /globular/objectstore/config
  files:
    - golang/config/minio_runtime_render.go
  symbols:
    - enforceMinioHeld
    - nodeIPInPool
  systemd_units:
    - globular-minio.service

forbidden_fixes:
  - start_minio_from_local_health_check
  - infer_minio_membership_from_disk
  - wipe_minio_sys_without_approved_transition
  - round_robin_dns_for_minio_writes

required_tests:
  - TestMinioHeldWhenNodeNotInPool
  - TestTopologyRenderParity
  - TestApprovedTransitionRequiredForWipe

related_failure_modes:
  - objectstore.local_membership_inference
  - repository.minio.recovery_cycle
```

---

## Example Convergence Rule

```yaml
id: convergence.no_infinite_retry
title: No infinite deterministic retry
severity: critical
summary: >
  Reconciliation must classify failures as transient, deterministic,
  blocked, or pending-sync. Deterministic failures must not retry forever.

rules:
  - transient failures may retry with bounded backoff
  - deterministic failures become BLOCKED until input changes
  - dependency seeding may become dependency_seeding_in_progress
  - unresolvable dependency becomes BLOCKED
  - local success pending controller commit becomes PENDING_SYNC

forbidden_fixes:
  - blind_reconcile_retry
  - treating_all_failures_as_transient
  - clearing_action_without_durable_result
  - marking_available_from_local_success_only

required_tests:
  - TestDeterministicFailureDoesNotRetryForever
  - TestPendingSyncRecovery
  - TestLeaderFailoverDuringResultCommit
```

---

## Example Failure Mode

```yaml
id: install.result.partial_commit
title: Leader dies during install result promotion
severity: critical

symptoms:
  - node-agent completed package install locally
  - etcd installed-state missing
  - reconciler dispatches same install repeatedly
  - action result exists without promotion

root_cause: >
  Installed-state, result promotion, and action cleanup were committed
  as separate etcd writes. Leader failover could leave partial state.

architecture_fix: >
  Commit installed-state, result promotion, and action cleanup in one etcd transaction.

forbidden_fixes:
  - retry_install_blindly
  - clear_action_before_installed_state
  - mark_release_available_from_local_success

related_invariants:
  - install.result.atomic_commit
  - local_success_not_global_commit

related_services:
  - cluster-controller
  - node-agent
  - workflow-service

required_tests:
  - TestLeaderFailoverDuringResultCommit
  - TestPendingSyncRecovery
```

---

## Extractors

### V1 Extractors

```text
Go extractor
Proto extractor
Workflow YAML extractor
Package manifest extractor
Systemd/specgen extractor
Test extractor
Manual YAML loader
```

### Go Extractor

Extract:

```text
packages
files
functions
methods
structs
interfaces
imports
call edges, best effort
comments with awareness annotations
```

Support awareness annotations:

```go
//globular:service node-agent
//globular:enforces objectstore.topology_contract
//globular:reads /globular/objectstore/config
//globular:controls globular-minio.service
//globular:forbids start_minio_from_local_health_check
```

These annotations are cheap and powerful.

### Proto Extractor

Extract:

```text
proto services
rpc methods
messages
request/response types
service ownership
```

Example relationship:

```text
ApplyPackageRelease RPC
→ node-agent
→ package install path
→ InstalledState
→ install.result.atomic_commit
```

### Workflow Extractor

Extract:

```text
workflow id
steps
actors
retry policy
compensation
verification
receipts
state transitions
```

Workflow retry policies should link to convergence rules.

### Package Extractor

Extract:

```text
package name
service name
version/build_id
dependencies
runtime local dependencies
artifact source
release index relationship
```

### Runtime Bridge

From runtime state, ingest:

```text
doctor findings
events
workflow receipts
installed state
runtime service status
repository operational mode
objectstore topology generation
xDS applied generation
```

This connects source graph to live cluster state.

---

## Integration With Existing Services

Globular already has the right organs:

```text
workflow-service
ai-watcher
ai-memory
doctor
event-service
```

The awareness graph connects them.

---

## ai-watcher Integration

Current simple flow:

```text
event → incident → action
```

New flow:

```text
event
→ graph lookup
→ failure-mode match
→ invariant impact
→ ai-memory recall
→ allowed workflow selection
→ workflow-service dispatch
```

Example event:

```text
service.exited: globular-minio.service
```

ai-watcher asks awareness graph:

```text
What invariants protect MinIO start/stop?
Is this node in ObjectStoreDesiredState?
Is MinIO allowed to be restarted?
What failure modes match?
What workflows are allowed?
```

If the node is not in the desired pool:

```text
Do not restart.
Emit: objectstore.hold_gate_respected
```

This prevents unsafe remediation.

---

## ai-memory Integration

ai-memory stores operational scar tissue.

Structured memory entries should include:

```text
failure mode observed
symptoms
root cause
safe fix
bad fix that caused damage
related invariant
related workflow
verification result
```

The awareness graph indexes ai-memory entries.

The graph does not replace memory. It gives memory structure.

---

## workflow-service Integration

workflow-service remains the only action engine.

The awareness graph says:

```text
allowed workflow: remediate.repository_unreachable
forbidden workflow: restart_minio_blindly
required verification: repository_status == DEGRADED or FULL, not UNKNOWN
```

workflow-service executes:

```text
assess
approve
execute
verify
record receipt
```

This keeps AI action bounded and auditable.

---

## doctor Integration

Doctor findings should link directly to invariants.

Example:

```text
doctor finding: objectstore_contract_missing
→ invariant: objectstore.topology_contract
→ files: minio_runtime_render.go, join_script.go
→ workflows: apply_objectstore_topology
→ forbidden fixes: local MinIO start
```

This allows doctor findings to become architecture-aware.

---

## Release Pipeline Integration

When source changes:

```text
changed files
→ affected symbols
→ affected services
→ affected packages
→ affected invariants
→ required tests
→ release risk summary
```

This fits the BOM/delta release direction.

Example:

```bash
globular awareness release-impact --from-git-diff
```

Output:

```text
Changed services:
- workflow-service
- repository
- node-agent

Impacted invariants:
- convergence.no_infinite_retry
- repository.metadata_first
- install.result.atomic_commit

Required tests:
- workflow circuit breaker tests
- repository degraded mode tests
- node-agent package fallback tests

Risk:
HIGH, because changes touch recovery and convergence paths.
```

---

## Agent Context Generation

The most important V1 feature is agent-context generation.

Command:

```bash
globular awareness agent-context --task "fix install retry loop"
```

Example output:

```markdown
# Globular Agent Context

## Task
fix install retry loop

## Relevant services
- cluster-controller
- node-agent
- workflow-service
- repository

## Relevant state model
- Artifact
- Desired
- Installed
- Runtime

## Relevant invariants
- install.result.atomic_commit
- local_success_not_global_commit
- convergence.no_infinite_retry

## Known failure modes
- install.result.partial_commit
- deterministic.install.failure.retry_loop
- stale.installed_state.hides_runtime_dead

## Forbidden fixes
- blind retry every reconcile tick
- clearing action without durable installed-state
- marking release AVAILABLE from local node success
- treating deterministic failure as transient

## Required tests
- TestLeaderFailoverDuringResultCommit
- TestPendingSyncRecovery
- TestDeterministicFailureDoesNotRetryForever

## Required searches
- ApplyPackageRelease
- InstalledBuildID
- ActionResult
- PENDING_SYNC
- BLOCKED
- retry classification

## Architecture rule
Local completion is not global convergence.
Installed-state is not runtime liveness.
```

Claude/Codex should be instructed to read this before editing code.

---

## Deadlock and Convergence Detection

The graph must model dependency phase.

A dependency is not just:

```text
A depends on B
```

It is:

```text
A depends on B during phase P, and it is required or optional.
```

Example:

```yaml
dependencies:
  - from: node-agent
    to: repository
    phase: package_install
    required: true

  - from: repository
    to: minio
    phase: blob_read
    required: true

  - from: repository
    to: scylla
    phase: metadata_read
    required: true

  - from: repository
    to: minio
    phase: metadata_read
    required: false

  - from: node-agent
    to: local_bom_cache
    phase: bootstrap_recovery
    required: true
```

Then detect cycles:

```text
node-agent → repository → MinIO → node-agent
```

And classify:

```text
required cycle during recovery: dangerous
optional cycle during normal runtime: acceptable
cycle has escape hatch: local BOM cache
```

This allows the graph to warn:

```text
This proposed fix creates a recovery deadlock.
```

---

## CLI Design

Recommended commands:

```bash
globular awareness build
globular awareness stats
globular awareness query "<question>"
globular awareness impact --file <path>
globular awareness impact --symbol <symbol>
globular awareness invariants --service <service>
globular awareness failure-mode <id>
globular awareness agent-context --task "<task>"
globular awareness cycles --phase recovery
globular awareness release-impact --from-git-diff
globular awareness doctor-context --finding <finding>
globular awareness memory-link --incident <incident-id>
```

Most important V1 commands:

```bash
globular awareness build
globular awareness impact --file <path>
globular awareness agent-context --task "<task>"
globular awareness cycles --phase recovery
```

---

## Implementation Structure

Suggested location in the `services` repo:

```text
golang/awareness/
  graph/
    db.go
    nodes.go
    edges.go
    query.go
    traversal.go

  extractors/
    goast/
      extract.go
    proto/
      extract.go
    workflows/
      extract.go
    packages/
      extract.go
    systemd/
      extract.go
    tests/
      extract.go
    manual/
      invariants.go
      failure_modes.go
      services.go

  analysis/
    impact.go
    cycles.go
    convergence.go
    agent_context.go
    release_impact.go

  runtime/
    doctor_bridge.go
    event_bridge.go
    memory_bridge.go
    workflow_bridge.go

  cli/
    awareness_cmd.go
```

Database path for runtime:

```text
/var/lib/globular/awareness/graph.db
```

Database path for repo-local development:

```text
.globular/awareness/graph.db
```

Manual truth files:

```text
docs/awareness/
  invariants.yaml
  state_model.yaml
  services.yaml
  failure_modes.yaml
  convergence_rules.yaml
  forbidden_fixes.yaml
```

---

## Proto/API Design

Eventually expose the graph as a service.

```proto
service AwarenessGraphService {
  rpc BuildGraph(BuildGraphRequest) returns (BuildGraphResponse);
  rpc GetImpact(GetImpactRequest) returns (GetImpactResponse);
  rpc GetAgentContext(GetAgentContextRequest) returns (GetAgentContextResponse);
  rpc GetDoctorContext(GetDoctorContextRequest) returns (GetDoctorContextResponse);
  rpc FindCycles(FindCyclesRequest) returns (FindCyclesResponse);
  rpc RecordFailureMemory(RecordFailureMemoryRequest) returns (RecordFailureMemoryResponse);
}
```

Core messages:

```proto
message GraphNode {
  string id = 1;
  string type = 2;
  string name = 3;
  string path = 4;
  string summary = 5;
  string metadata_json = 6;
}

message GraphEdge {
  string src = 1;
  string kind = 2;
  string dst = 3;
  string phase = 4;
  bool required = 5;
  double confidence = 6;
  string metadata_json = 7;
}

message GetAgentContextRequest {
  string task = 1;
  repeated string files = 2;
  repeated string symbols = 3;
  repeated string services = 4;
  bool include_runtime = 5;
  bool include_memory = 6;
}

message GetAgentContextResponse {
  string markdown = 1;
  repeated string invariant_ids = 2;
  repeated string failure_mode_ids = 3;
  repeated string forbidden_fix_ids = 4;
  repeated string required_tests = 5;
}
```

---

## How This Closes the Loop

Current loop:

```text
AI edits code
→ code breaks invariant
→ cluster deadlocks
→ user discovers
→ Claude fixes locally again
```

New loop:

```text
AI receives task
→ awareness graph generates context
→ AI sees invariants/failure modes/forbidden fixes
→ AI edits safer
→ tests selected by graph run
→ release impact computed
→ runtime events linked to source/invariants
→ ai-memory stores new scar
→ next AI patch is better
```

The full loop:

```text
Source → Package → Desired → Installed → Runtime → Incident → Memory → Source
```

This is Globular becoming aware not only of runtime, but of its own construction.

---

## V1 Scope

Do not build the entire cathedral at once. Build the spine.

### V1 Must Have

```text
SQLite graph database
manual invariant YAML
manual failure mode YAML
Go/proto/workflow/package extractors
impact query
agent-context generator
cycle detector for service dependencies
Claude/Codex instruction template
```

### V1 Can Skip

```text
fancy UI
Neo4j
natural language graph query
automatic invariant inference
full runtime bridge
complex visualization
```

---

## First Seven Invariants To Encode

Start with the scars that burned the system before.

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

These seven invariants alone should prevent many bad AI fixes.

---

## First Implementation Order

### Step 1: Create awareness files

```text
docs/awareness/invariants.yaml
docs/awareness/failure_modes.yaml
docs/awareness/convergence_rules.yaml
docs/awareness/services.yaml
```

Add the seven initial invariants.

### Step 2: Create SQLite graph package

Create:

```text
golang/awareness/graph
```

Implement:

```text
AddNode
AddEdge
FindNode
Neighbors
Traverse
Impact
```

### Step 3: Implement manual loader

Load YAML into graph:

```text
invariant → protects → state/file/symbol
invariant → forbids → forbidden_fix
invariant → tested_by → test
failure_mode → relates_to → invariant/service/test
```

### Step 4: Implement basic source extractor

Extract:

```text
files
symbols
imports
tests
proto services
workflow ids
package manifests
```

### Step 5: Implement agent context generator

Given task text, match:

```text
services
symbols
failure mode keywords
invariant keywords
```

Generate Markdown.

### Step 6: Add Claude/Codex preflight rule

Every agent task starts with:

```bash
globular awareness agent-context --task "<task>"
```

### Step 7: Add cycle detector

Detect required dependency cycles by phase:

```bash
globular awareness cycles --phase recovery
globular awareness cycles --phase bootstrap
globular awareness cycles --phase reconcile
```

---

## Claude/Codex Instruction Template

Add this to the agent instruction file:

```text
Before editing Globular code, you must run:

globular awareness agent-context --task "<task>"

If you plan to edit specific files, also run:

globular awareness impact --file "<file>"

You must identify:
1. impacted invariants
2. state transitions touched
3. forbidden fixes
4. related failure modes
5. required tests

Do not implement a local retry/restart fix unless the awareness context says it is safe.

Do not bypass:
- ObjectStoreDesiredState
- Desired/Installed/Runtime state separation
- atomic result promotion
- workflow backend health gates
- deterministic failure blocking
- repository metadata-first behavior

If awareness context reports unknown invariant impact, stop and produce an architecture assessment before editing.
```

---

## Future Features

Later, this can become a major Globular capability:

```text
AI-operable infrastructure needs AI-operable source.
```

Possible future features:

```text
web admin graph view
doctor finding → source impact
release risk score
automatic required test selection
Codex/GitHub PR bot
runtime incident replay
workflow remediation planner
awareness diff between releases
natural language graph query
```

Example:

```bash
globular awareness release-impact --from v1.2.11 --to v1.2.12
```

Output:

```text
Changed services:
- workflow-service
- repository
- node-agent

Impacted invariants:
- convergence.no_infinite_retry
- repository.metadata_first
- install.result.atomic_commit

Required tests:
- workflow circuit breaker tests
- repository degraded mode tests
- node-agent package fallback tests

Risk:
HIGH, because changes touch recovery and convergence paths.
```

---

## Final Design Statement

The architecture is:

```text
A SQLite-backed awareness graph that connects source code, runtime state,
invariants, failure modes, AI memory, doctor findings, and workflow actions.

It is built from static extractors, manual invariant declarations,
runtime bridges, and ai-memory entries.

It does not execute fixes directly. It produces context, impact analysis,
cycle detection, and safe-action constraints.

workflow-service remains the action engine.
ai-watcher remains the observer.
ai-memory remains the scar store.
doctor remains the invariant detector.

awareness-graph is the missing connective tissue.
```

This gives AI agents what they do not naturally have:

```text
global architectural context compressed into a small, queryable packet.
```

Without it, Claude sees local code.

With it, Claude sees:

```text
what the code protects,
what failures happened before,
what must never be changed,
what tests prove the law,
and what runtime cycle might deadlock the cluster.
```

That is how Globular can start converging at the code level, not only at the runtime level.
