# Globular Day 1 Join Strategy

## Context

Starting point is good:
- node join now works
- node 1 registers to node 0
- etcd member add / etcd configuration are correct
- the problem is no longer basic join

The problem is **Day 1 orchestration after join**.

Right now the cluster can admit a node, but the node is still initialized too loosely:
- profiles exist but are not clearly driving package selection end to end
- infrastructure requirements are not consistently derived from profiles
- package dependency metadata is incomplete
- some services are installed without enough orchestration knowledge
- services can start even when their infra prerequisites are missing
- the overall join sequence lacks a strong staged structure

This is why failures look random even when the root cause is structural.

Example symptom:
- `ai_memory` installs and later fails because ScyllaDB is not present
- Scylla is packaged in Globular, but the system did not ensure it was selected, installed, configured, and healthy before allowing dependent services to start

That means the issue is not only a missing dependency entry in one spec. The issue is that Day 1 still behaves too much like “install things and hope they line up,” instead of a phased bootstrap + reconciliation pipeline.

---

## Big Picture

Day 0 and Day 1 are different worlds.

### Day 0
Day 0 is local bootstrap:
- create the first truth
- initialize cluster identity
- seed infra
- establish repository, certificates, DNS, etcd, controller, node-agent

### Day 1
Day 1 is **joining an existing truth**:
- the node must not invent infrastructure
- the node must discover what role it has in the cluster
- the node must install the right infra and workload packages in the right order
- the node must be gated by readiness at each phase

The Day 1 goal is not just “node joined.”
The real goal is:

> Node joined, got its role/profile, installed its required infrastructure, converged to desired state, and only then started workloads.

---

## Core Principle

A joining node must be initialized by a **state machine**, not by scattered install calls.

The structure must be:

1. Identity / admission
2. etcd join
3. profile resolution
4. infrastructure requirement derivation
5. package resolution
6. dependency expansion
7. infra install and config rendering
8. infra health gates
9. workload install
10. workload health gates
11. ready

If a phase is not complete, the next phase must not run.

---

## What Must Be Fixed

### 1. Profiles must become authoritative
Profiles must not be decorative. They must drive:
- which infra components belong on the node
- which workload services belong on the node
- which rendered configs are required
- which health gates are expected

The system must answer this deterministically:
- `control-plane` means what exact packages?
- `gateway` means what exact packages?
- `storage` means what exact packages?
- `database` or `scylla` means what exact packages?
- what is implied by `core`?
- what infra from Day 0 is mandatory again on Day 1 for a given profile?

Do not let profiles remain only a loose systemd-unit mapping. They must resolve to a package-level and dependency-aware desired state.

### 2. Package specs must become dependency-complete
Specs must declare the real dependencies, not only `After=` or `Wants=` in systemd units.

There are at least two dependency layers:

#### a. Package dependency graph
This is for planning and install ordering.
Examples:
- `ai_memory` depends on `scylladb`
- `resource` may depend on `scylladb` in specific storage modes
- `backup_manager` may depend on MinIO, restic, or Scylla-specific tooling depending on role
- `gateway` depends on `envoy` or xDS path correctness
- services requiring object storage must depend on `minio` when local storage profile requires it

#### b. Runtime readiness dependency
This is for start gating.
Examples:
- do not start `ai_memory` until Scylla is installed, configured, and healthy
- do not start services requiring object storage until MinIO is healthy
- do not start services requiring controller-issued configs until config rendering completed

Systemd `After=` is not enough. The planner must know these dependencies before dispatch.

### 3. Infrastructure must be treated separately from workloads
Infrastructure packages are not normal workloads.

They need dedicated handling:
- etcd
- envoy
- xds
- minio
- scylladb
- keepalived
- node-exporter / monitoring infrastructure
- cluster-controller / node-agent / cluster-doctor if they are part of managed control plane on that node

These packages must be resolved and installed first, then validated, before any workload depending on them can proceed.

### 4. Day 1 must reuse Day 0 rules where needed
Some infra requirements from Day 0 are still required during Day 1 join.

Examples:
- trust root / certificates
- cluster domain and network overlay
- service config rendering
- repository access
- infra package install locations
- generated cluster credentials such as MinIO shared credentials, scylla config seeds, etc.

Do not treat join as a small patch after admission. Treat it as **Day 0 replay constrained by existing cluster truth**.

### 5. Installation must become knowledge-driven
The node must not install services “because they appeared in a plan” without knowing:
- why they are required
- which profile selected them
- which dependency selected them
- which infra gate they are waiting on
- whether config rendering is complete
- whether the node has the capability to run them

Every install should be explainable by a chain like:

`profile -> desired component -> dependency expansion -> package -> rendered config -> health gate`

---

## Required Target Architecture

## A. Build a canonical profile catalog
Create one canonical source of truth, for example:
- `profile -> components`
- `component -> package`
- `component -> type (infra/workload/tooling)`
- `component -> dependencies`
- `component -> readiness checks`
- `component -> config render requirements`

Example shape:

```text
profile storage:
  includes:
    - minio
    - file

profile database:
  includes:
    - scylladb

profile gateway:
  includes:
    - envoy
    - gateway
    - xds

profile control-plane:
  includes:
    - etcd
    - dns
    - discovery
    - cluster-controller
    - node-agent
```

Then dependency expansion happens after profile selection, not through ad hoc systemd behavior.

## B. Split desired state into two sets
For each node derive:

### desired infra set
Everything that must exist before workloads can start.

### desired workload set
Everything allowed after infra is healthy.

These two sets must be explicit in controller logic and visible in status.

## C. Add dependency expansion before planning
Flow must be:

1. normalize profiles
2. resolve profile components
3. expand transitive dependencies
4. classify infra vs workload
5. generate install/start plan
6. apply gating

Do not generate plans directly from profile names mapped to unit names.

## D. Add hard bootstrap phases
Use and strengthen phases like:
- admitted
- infra_preparing
- etcd_joining
- infra_configuring
- infra_ready
- workload_preparing
- workload_ready
- failed

Meaning:
- `infra_preparing`: install infra packages only
- `infra_configuring`: render configs like scylla seeds, minio cluster settings, TLS, network specs
- `infra_ready`: verify infra health
- `workload_preparing`: now allow workload packages
- `workload_ready`: steady state

No workload starts before `infra_ready`.

---

## Concrete Problems To Fix In Current Code

### 1. Profile selection is structurally too weak
Current controller planning still looks too unit-oriented. The planning path resolves actions from profile-to-unit mapping. That is not enough. Profiles must resolve components/packages/dependencies first, then units. Otherwise package selection and orchestration remain incomplete.

### 2. Node-agent bootstrap path appears mismatched
The node-agent bootstrap code currently builds a bootstrap plan from a list of strings by converting each entry with `units.UnitForService(...)`. But `buildBootstrapPlanWithNetwork(profiles, clusterDomain)` passes **profiles** into `buildBootstrapPlan`, which expects **service names**. This is a structural mismatch and likely means bootstrap package selection from profiles is unreliable or incorrect.

Fix this cleanly:
- either `buildBootstrapPlan` must accept resolved components/services
- or profile resolution must happen before calling it
- but profiles must never be treated as service names

### 3. Installed-state reporting currently skips many infra packages
There is logic in node-agent installed service discovery that explicitly skips infrastructure packages like etcd, minio, envoy, scylladb, keepalived, etc. That may make sense for one narrow reporting phase, but it is dangerous if it prevents accurate convergence decisions.

Review whether infra presence/version/state is being reported in a separate authoritative way. If not, the controller can neither confirm infra readiness nor detect drift correctly.

### 4. Spec dependencies are incomplete
Review all specs and add dependency metadata for real runtime dependencies.

At minimum audit:
- ai_memory
- ai_executor
- ai_router
- ai_watcher
- resource
- file
- media
- backup_manager
- monitoring
- repository
- any service requiring Scylla or MinIO

Do not rely only on systemd unit `After=` lines. Add planner-visible dependency metadata.

### 5. Infra package requirements from profiles are incomplete
Audit all infrastructure packages and define where they belong:
- etcd
- envoy
- xds
- minio
- scylladb
- scylla-manager
- scylla-manager-agent
- keepalived
- monitoring stack pieces
- mcp if intended as infra/admin role package

Make profile requirements explicit and deterministic.

---

## Implementation Strategy

## Phase 1: Model cleanup
1. Introduce canonical `ComponentCatalog` or equivalent.
2. Define for each component:
   - package name
   - unit names
   - kind: infra/workload/tool
   - dependencies
   - readiness gates
   - profile membership
3. Move current profile logic to use that model.

## Phase 2: Planner cleanup
1. Replace direct `profile -> units` planning with:
   - profiles -> components
   - expand dependencies
   - split infra/workload
   - build ordered actions
2. Preserve stop/disable logic for removed units, but base desired state on resolved components.
3. Make unknown profiles a hard block, as today.

## Phase 3: Bootstrap / Day 1 phase enforcement
1. During join, after etcd is healthy, resolve node profiles.
2. Generate infra desired set.
3. Install infra packages and render configs.
4. Verify infra readiness.
5. Only then transition to workload planning.

## Phase 4: Spec audit
1. Audit all package specs.
2. Add explicit dependency metadata.
3. Add readiness metadata where useful.
4. Ensure scylla/minio-dependent services declare those requirements.
5. Validate unit names and package names are canonical.

## Phase 5: Observability and safety
Add status fields so debugging becomes obvious:
- selected profiles
- resolved components
- expanded dependencies
- infra desired set
- workload desired set
- blocked reason
- missing dependency list
- unmet readiness gates

When a service is not installed, the system should say why.
When a service is blocked, the system should say by what.

---

## Expected Behavior After Fix

When node 1 joins with profiles like `storage` and `database`, the system should do this deterministically:

1. node admitted
2. etcd join complete
3. controller resolves profiles
4. controller derives required infra:
   - minio
   - scylladb
   - maybe scylla-manager pieces if required
5. controller expands dependencies
6. node installs those infra packages
7. controller/node-agent render needed configs:
   - scylla seeds / cluster name
   - minio pool / credentials / endpoints
   - TLS / network overlays
8. health gates verify MinIO and Scylla are ready
9. only then dependent services such as `ai_memory` are allowed to install/start
10. node transitions to workload-ready

That is the bar.

---

## Non-Goals

Do not paper over the issue by only adding more `ExecStartPre` retries or more `After=` lines.
Those are useful, but they are not the core fix.

Do not hardcode special cases for `ai_memory` only.
The fix must be structural and reusable.

Do not let node-agent invent desired state locally after join.
The controller must remain the authority for profile-driven convergence.

---

## Deliverables

1. Refactor profile resolution into a canonical catalog model.
2. Refactor planner to resolve packages/components before units.
3. Fix node-agent bootstrap mismatch between profiles and services.
4. Audit package specs and add real dependency metadata.
5. Ensure infra packages are selected from profiles and gated before workloads.
6. Expose enough status/debug info to understand why a service is or is not installed.
7. Add tests for:
   - profile -> component resolution
   - transitive dependency expansion
   - infra-before-workload gating
   - node join with `storage`
   - node join with `database`
   - node join with `storage + database`
   - service blocked because dependency missing
   - service unblocked once dependency becomes healthy

---

## Final Intent

The node join problem is no longer “can the node enter the cluster?”
That part is already largely working.

The problem is:

> Can a joining node be initialized in a disciplined, profile-driven, dependency-aware, infra-first way?

That is the real Day 1 milestone.

Please implement the Day 1 structure, not just local patches.
