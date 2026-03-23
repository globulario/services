# Globular Day 1 Join Orchestration — Cluster-Fit Plan v2

## Purpose

This plan replaces the current Day 1 behavior where:
- node join succeeds,
- etcd membership is correct,
- but profile meaning is weak,
- infra packages are not reliably installed from profile/capability requirements,
- workloads may be installed before their local prerequisites exist,
- and package dependency truth is split between runtime behavior, scattered maps, and ad-hoc logic.

The goal is **not** to patch one failing service. The goal is to make Day 1 initialization structurally correct for the cluster.

This plan must be implemented in a way that prevents drift:
- one source of truth for placement and dependency resolution,
- explicit Day 1 phases,
- clear controller/agent responsibilities,
- package-spec-backed dependencies,
- infra-first convergence,
- workload installation only when node capabilities and local prerequisites are satisfied.

---

## Current Starting Point

Assume the following are already true and must not be reworked unnecessarily:

1. **Node join works**.
   - Node 1 can register to node 0.
   - etcd membership is configured correctly.

2. **Do not redesign Day 0 bootstrap**.
   - Day 0 has enough structure already.
   - This work is about the transition from successful join to correct Day 1 convergence.

3. **Do not introduce a second hidden orchestration system**.
   - The result must strengthen the existing cluster controller and node agent model.
   - Do not scatter equivalent logic across multiple maps and helper functions.

---

## Non-Negotiable Design Rules

### 1. Join success is not node readiness
A node is **not ready** just because:
- it registered,
- got approved,
- and joined etcd.

A node is only ready when its Day 1 phases are complete.

### 2. Profiles must have real meaning
Profiles must not be cosmetic labels.
They must drive:
- infra capability requirements,
- workload eligibility,
- package selection,
- and reconciliation scope.

### 3. Infra must converge before workloads
Local infra required by the node profile must be installed and healthy before workload services are eligible for installation.

### 4. Package specs must become authoritative for dependencies
Do **not** hardcode all service dependencies only in controller code.
Controller code may cache, validate, normalize, and enrich package metadata, but dependency truth must come from package specs whenever possible.

### 5. The cluster controller decides intent; the node agent performs installation
Responsibilities must be clean:
- **Cluster controller**: resolves node intent and convergence phases.
- **Node agent**: installs, removes, and starts packages on the node.

Do not let both invent desired state independently.

### 6. Missing local dependencies must trigger install planning, not only blocking
It is not enough to say:
- “ai_memory is blocked because scylladb is missing”.

If ScyllaDB is required by the node’s resolved infra/workload graph, the system must schedule its installation first.

---

## The Correct Day 1 Mental Model

Day 1 is a **state machine**, not a loose sequence of helper calls.

A joined node must move through explicit phases:

1. `joined`
2. `identity_ready`
3. `cluster_config_synced`
4. `profile_resolved`
5. `infra_planned`
6. `infra_installed`
7. `infra_healthy`
8. `workloads_planned`
9. `workloads_installed`
10. `ready`

Optional degraded states:
- `infra_blocked`
- `workload_blocked`
- `dependency_missing`
- `package_metadata_invalid`

These phases must be visible in status.

---

## Required Architectural Model

The current proposal was too controller-centric. The correct structure is a 4-layer model.

### Layer A — Package Metadata Truth
Each package spec must be able to express at minimum:
- canonical package/component name
- service/unit name
- package kind: `infrastructure` or `workload`
- install-time dependencies
- runtime/local dependencies
- optional health gate(s)
- provided capabilities
- required capabilities
- optional default profiles or role hints

If the current package spec format lacks these fields, extend it minimally and explicitly.

#### Required distinction
There are **two dependency classes**:

1. **Install dependencies**
   - must be present before package installation is considered complete
   - ex: helper package, tool, local infra package

2. **Runtime local dependencies**
   - must be healthy on the same node before workload starts or becomes eligible
   - ex: `ai_memory -> scylladb`

Do not collapse both into one vague list.

---

### Layer B — Cluster Catalog / Resolver
Build a canonical resolver in the cluster controller that merges:
- package specs,
- built-in infra capability definitions,
- profile-to-capability rules,
- service canonical names,
- and unit-name mapping.

This resolver must be the single place that answers:
- What does profile `storage` mean?
- What infra capabilities does it require?
- What packages satisfy those capabilities?
- What local dependencies does package `ai_memory` have?
- Is a service eligible on this node?

This is **not** a hardcoded second dependency database.
It is a normalized runtime view derived from package specs and explicit cluster rules.

---

### Layer C — Node Intent Resolution
For each node, resolve:
- profiles
- explicit roles/capabilities
- desired infra packages
- desired workload packages
- local dependency graph

Output should look conceptually like:

```json
{
  "node_id": "node-1",
  "profiles": ["storage", "ai"],
  "required_capabilities": ["object-store", "local-db", "event-bus-client"],
  "desired_infra": ["minio", "scylladb"],
  "desired_workloads": ["ai_memory", "ai_executor"],
  "blocked_workloads": []
}
```

This resolved node intent must be stored in controller state for observability.

---

### Layer D — Agent Apply Contract
The node agent must receive a plan that is already structured by phase, for example:
- install infra packages
- verify health
- then install workloads

The node agent must not guess profile meaning on its own.
It may validate and refuse invalid actions, but not independently re-resolve cluster intent.

---

## Profile Model

Profiles must resolve to **capabilities first**, not directly to a random service list.
This prevents profile drift and makes infra requirements explicit.

### Example
`storage` should not merely mean “maybe minio, maybe file”.
It should mean something like:
- provides object storage capability
- requires local object-store infra
- may allow workloads that depend on object storage

Likewise:
- `database` or `scylla` must clearly imply local Scylla capability
- `gateway` must imply gateway/envoy/xDS related capability
- `control-plane` must imply cluster control capability

### Rule
Profiles may expand to capabilities.
Capabilities then resolve to packages.
Packages then resolve to services/units.

This is more stable than profile -> service directly.

---

## Day 1 Execution Phases

## Phase 1 — Define the Node Day 1 State Machine

### Objective
Make Day 1 explicit and observable.

### Deliverables
- Add Day 1 lifecycle phases to controller state.
- Ensure a node cannot jump from `joined` directly to workload install.
- Expose current phase and blocked reason in status.

### Required behavior
A node that joined etcd but has not installed required infra must show something like:
- `phase=infra_planned`
- `blocked_reason=waiting_for_minio_install`

### Acceptance
- Status clearly differentiates `joined` from `ready`.
- Workload-ready is impossible before infra-healthy.

---

## Phase 2 — Extend Package Specs for Dependency Truth

### Objective
Make package specs authoritative enough to drive Day 1.

### Deliverables
- Add or normalize fields for:
  - `kind`
  - `install_dependencies`
  - `runtime_local_dependencies`
  - `provides_capabilities`
  - `requires_capabilities`
  - `health_checks` or readiness hints if needed
- Update relevant package specs, especially infra and known failing workloads.

### Minimum packages to audit immediately
- `scylladb`
- `minio`
- `etcd`
- `envoy`
- `xds`
- `event`
- `rbac`
- `file`
- `ai_memory`
- `ai_executor`
- `ai_watcher`

### Important
Do not postpone spec cleanup to “later”.
This is part of the work, not documentation garnish.

### Acceptance
- `ai_memory` declares its local runtime dependency on Scylla in spec-backed metadata.
- Infra packages declare capabilities they provide.
- Missing or inconsistent spec metadata fails validation visibly.

---

## Phase 3 — Build the Canonical Resolver in the Cluster Controller

### Objective
Replace scattered profile/service/dependency maps with one normalized resolver.

### Deliverables
Create a resolver module that:
- loads package metadata,
- normalizes canonical names,
- maps profiles -> capabilities,
- maps capabilities -> packages,
- maps packages -> units/services,
- computes transitive dependencies,
- separates infra from workloads.

### Important constraints
- Existing helper maps may remain temporarily as derived compatibility views during migration.
- The new resolver becomes the authoritative source.
- Do not leave two competing truths after migration.

### Acceptance
Given a node profile set, the resolver can answer deterministically:
- required infra packages
- eligible workload packages
- transitive local deps
- phase ordering

---

## Phase 4 — Resolve Node Intent from Profiles and Package Metadata

### Objective
Turn a joined node into a structured Day 1 intent object.

### Deliverables
For each node, compute:
- profiles
- required capabilities
- desired infra packages
- desired workload packages
- local dependency graph
- blocked reasons if metadata is incomplete

### Rule
Profile resolution must happen **before** service filtering and release dispatch.

### Acceptance
Examples:
- a `gateway` node does not receive `ai_memory`
- a `storage` node receives the object-store infra it requires
- an `ai` node that requires local Scylla gets a plan that includes Scylla before `ai_memory`

---

## Phase 5 — Infra-First Plan Generation

### Objective
Generate plans in two tiers:
- infra first
- workloads second

### Deliverables
The controller must generate:
1. `infra_plan`
2. `workload_plan`

Infra plan includes:
- package installation
- package removal if inappropriate for profile
- health verification gates

Workload plan includes:
- installation/start only after required infra is healthy

### Required behavior
If a workload depends on a missing local package that is required by the resolved node intent, the infra plan must include that package.

### Example
For node profiles that imply local AI memory stack:
- install `scylladb`
- wait for Scylla healthy
- install `ai_memory`
- then install dependent AI services

### Acceptance
- No workload package is installed before its required infra is planned.
- Missing infra becomes an actionable install step, not a passive blocked note.

---

## Phase 6 — Clean Controller / Node Agent Responsibility Boundary

### Objective
Make orchestration clear and non-duplicated.

### Controller responsibility
- resolve node intent
- build phase-ordered plan
- track progress and state
- decide blocked/degraded/ready

### Node agent responsibility
- install package
- uninstall package
- start/stop service
- report unit health and package apply results
- never invent profile semantics

### Deliverables
Define or refine a plan/apply contract between controller and node agent.
If needed, introduce explicit plan payloads like:
- `InstallInfraPackages`
- `VerifyInfraHealth`
- `InstallWorkloadPackages`

### Acceptance
- Node agent no longer acts like a second planner.
- Controller no longer assumes install happened without explicit agent feedback.

---

## Phase 7 — Release Pipeline Scoping Must Use Resolved Node Intent

### Objective
Prevent release/install actions from targeting nodes that should not host the service.

### Deliverables
Release dispatch must use resolved node intent, not broad cluster-wide service lists.

### Acceptance
- Services are only dispatched to nodes whose resolved package/capability set includes them.
- Node-scoped desired state reflects profile and capability resolution.

---

## Phase 8 — Installed-State and Infra Visibility Cleanup

### Objective
Stop hiding infra truth from convergence.

### Problem to fix
If installed-state scanning skips infra packages like `etcd`, `minio`, `envoy`, `scylladb`, etc., Day 1 can never converge correctly because the planner cannot see reality.

### Deliverables
- Review installed-state scan behavior.
- Make infra packages visible to reconciliation.
- Ensure the controller can tell the difference between:
  - not desired,
  - desired but missing,
  - installed but unhealthy,
  - installed and healthy.

### Acceptance
Infra is first-class in convergence state, not invisible plumbing.

---

## Phase 9 — Observability and Failure Surfaces

### Objective
Make Day 1 diagnosable without guessing.

### Required status fields
At minimum expose:
- `profiles`
- `resolved_capabilities`
- `desired_infra_packages`
- `desired_workload_packages`
- `installed_packages`
- `blocked_services`
- `current_day1_phase`
- `phase_reason`
- `missing_package_metadata`
- `dependency_failures`

### Emit events for
- profile resolution complete
- infra plan generated
- infra dependency missing
- package metadata invalid
- workload blocked
- node ready

### Acceptance
When `ai_memory` is not installed, the operator should be able to see exactly whether the reason is:
- profile excludes it
- Scylla missing
- Scylla unhealthy
- package metadata incomplete
- install failed

---

## Migration Strategy

This must be implemented in **controlled phases** without destabilizing the cluster.

### Step 1
Introduce state machine and observability first.
No behavior change beyond better visibility.

### Step 2
Extend package specs and add validation.
Fail visibly on incomplete metadata, but do not yet switch all planning.

### Step 3
Introduce canonical resolver.
Keep old maps only as derived compatibility adapters.

### Step 4
Switch profile resolution and node intent generation to resolver.

### Step 5
Switch infra-first plan generation.

### Step 6
Switch release scoping and agent apply flow.

### Step 7
Remove legacy scattered maps/helpers once equivalence is proven.

---

## Explicit Things To Fix Now

These are not optional polish items. They are part of the implementation.

1. **Audit package dependency metadata**
   - especially `ai_memory` and infra packages.

2. **Fix profile-to-infra requirements**
   - Day 0 assumptions must be turned into explicit Day 1 profile/capability rules.

3. **Fix installation without orchestration**
   - service install must happen through phase-aware planning, not loose cluster-wide dispatch.

4. **Fix missing infra visibility in convergence**
   - controller must see whether infra is installed, missing, or unhealthy.

5. **Fix workload eligibility**
   - workloads must not be treated as globally installable by default.

---

## Implementation Boundaries

### Do
- Modify cluster controller, node agent, and package spec handling where required.
- Add validation and status fields.
- Create tests around real Day 1 scenarios.

### Do not
- Redesign the entire cluster architecture.
- Replace join flow that already works.
- Encode dependency truth only in hand-maintained controller maps.
- Let node agent infer cluster policy independently.

---

## Minimum Acceptance Scenarios

### Scenario A — Storage node join
Given a node with `storage` profile:
- node joins cluster
- profile resolves to required storage capabilities
- MinIO-related infra is planned and installed
- workloads requiring storage become eligible only after infra health passes

### Scenario B — AI memory local DB dependency
Given a node whose resolved workloads include `ai_memory`:
- controller sees `ai_memory` requires local Scylla
- controller plans `scylladb` first if not installed
- `ai_memory` is not installed until Scylla is healthy
- status explains phase and blockage throughout

### Scenario C — Gateway node isolation
Given a node with `gateway` profile only:
- it does not receive unrelated AI or storage workloads
- release pipeline does not dispatch those services to it

### Scenario D — Invalid package metadata
Given a package missing dependency metadata:
- resolver flags metadata as invalid
- node enters degraded/blocking state with explicit reason
- controller does not silently install broken workload order

---

## Required Test Coverage

Add tests for:
- profile -> capability resolution
- capability -> package resolution
- package-spec dependency parsing and validation
- node Day 1 phase progression
- infra-first plan generation
- workload blocking due to unhealthy local deps
- missing infra becomes install action, not passive block
- release scoping by resolved node intent
- installed-state visibility for infra packages
- invalid metadata failure surfaces

At minimum include explicit tests for:
- `storage -> minio`
- `ai_memory -> scylladb`
- gateway node does not receive AI workloads
- node joined but not ready until infra healthy

---

## Practical Guidance For Implementation

1. Start by making the invisible visible.
   - Add Day 1 phases and status fields first.

2. Then make metadata real.
   - Extend and validate package specs.

3. Then centralize reasoning.
   - Build resolver from specs + explicit profile/capability rules.

4. Then change behavior.
   - Switch planning to infra-first and node-scoped.

5. Then remove drift.
   - Delete legacy maps/helpers after tests prove equivalence.

---

## Final Instruction

Implement this as a **cluster-fit Day 1 orchestration spine**, not as a narrow fix for one service.

The target outcome is:
- node join remains correct,
- profiles truly drive placement,
- infra requirements are explicit,
- package metadata drives dependency truth,
- controller plans in phases,
- agent applies deterministically,
- and a node only becomes ready when Day 1 is actually complete.

If a future package with local infra requirements is added, the system should converge correctly from metadata and profile rules, without another round of ad-hoc patches.
