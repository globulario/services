## Directive: Design the Final-Release High-Availability Model for Globular

### Purpose

We do **not** want to approach HA reactively or by inferring strategy from test results.

We want an **architecture-first HA design**.

Use your knowledge of the current codebase and current architecture to write back a concrete implementation plan for achieving application/control-plane high availability before final release.

This is a **design task first**, not an implementation task.

---

## Core Architectural Position

The infrastructure layer is already broadly on the right path:

* etcd quorum
* MinIO distributed storage
* ScyllaDB replication

The remaining HA weakness is at the **application/control-plane layer**, not the storage layer.

That means the next design must focus on:

* controller continuity
* doctor continuity
* workflow execution durability/resumption
* stateless service replication and routing
* endpoint withdrawal and health-aware routing
* ingress failover only as a secondary concern

---

## Non-Negotiable Principle

**Leadership is not node-level. Leadership is service-level or ownership-domain-level.**

We do **not** want:

> “one node is the leader for everything”

We **do** want:

* one service leader for `cluster-controller`
* one service leader for `cluster-doctor`
* workflow execution ownership at the run level or executor-lease level
* independent failover domains per service

So it must be valid for:

* node A to host controller leader
* node B to host doctor leader
* node C to host workflow executor(s)

This is the intended model.

---

## Required Output

Write a design and implementation plan that defines **how Globular should achieve HA at the application/control-plane layer**.

Your output must include the following sections.

---

# 1. Service Classification by HA Class

Classify every important service/process into one of these HA classes:

### Class A — quorum/state stores

Examples:

* etcd
* ScyllaDB
* MinIO

### Class B — leader-elected control-plane singleton

Examples:

* cluster-controller
* cluster-doctor

Contract:

* multiple replicas may run
* exactly one active leader at a time
* followers must not mutate authoritative control state
* leader loss must cause bounded failover
* split-brain must be prevented

### Class C — resumable execution service

Example:

* workflow-service

Contract:

* multiple instances may exist
* execution ownership is durable
* one run has one owner at a time
* executor death must not orphan runs permanently
* another executor must be able to resume or recover safely

### Class D — stateless multi-instance services

Examples:

* gateway
* repository read paths
* MCP / read APIs where applicable
* xDS-facing or operator-facing service replicas where possible

Contract:

* multiple replicas may serve concurrently
* node loss reduces capacity, not correctness
* routing must send traffic only to healthy endpoints
* no hard fixed dependency on one instance

### Class E — node-local agents

Examples:

* node-agent
* exporters
* sidekick-type local helpers

Contract:

* local to node
* not leader-elected globally
* failures should be detectable and restartable
* higher layers must reason about node-local absence correctly

For each service in the current codebase, assign a class and justify it briefly.

---

# 2. HA Contract Per Class

For each HA class, define:

* ownership model
* failover semantics
* correctness condition
* bounded interruption expectation
* what is allowed to degrade
* what must never happen

Examples of the kind of statements we want:

* “Reduced capacity is acceptable; loss of correctness is not.”
* “Only the elected controller leader may mutate desired release state.”
* “A workflow run must not be owned by two executors simultaneously.”
* “Loss of one gateway instance must not black-hole requests if another healthy instance exists.”

This section must be explicit and testable.

---

# 3. Required Mechanisms Per Class

For each HA class, define the mechanism Globular should use.

Examples:

## For Class B (controller / doctor)

* etcd lease-based leader election
* fencing token / epoch
* renewal interval
* failover timeout
* follower behavior
* startup behavior
* shutdown / lost-lease behavior

## For Class C (workflow-service)

* durable run metadata in ScyllaDB
* executor lease / ownership record
* run heartbeat
* orphan detection
* resume semantics
* idempotence / duplicate execution guards
* what “resume” means after mid-step crash

## For Class D

* multi-instance registration
* health-aware Envoy/xDS routing
* endpoint withdrawal on node loss
* readiness / health reporting

## For ingress

* optionally keepalived or VIP failover
* but only as ingress/north-south HA, not as the main answer to application/control-plane HA

Be explicit about which mechanisms belong where.
Do **not** use keepalived as a generic answer to everything.

---

# 4. Explicit HA Invariants

Define a set of concrete invariants the implementation must satisfy.

Examples:

* Loss of one node must not stop reconciliation longer than X
* Loss of workflow executor must not orphan a run permanently
* Two controller leaders must never act concurrently
* Doctor findings may be briefly delayed during failover, but must resume within Y
* Dead service endpoints must be removed from routing within Z
* A stateless service may lose one replica without losing correctness

Make these invariants specific enough that they can later drive failover drills and acceptance tests.

Testing is **not** the source of strategy here.
Testing is what validates the contracts after you define them.

---

# 5. Phase-by-Phase Implementation Plan

Produce a phased implementation roadmap, in order.

Suggested shape:

### HA-1 — Control-plane HA design freeze

* classification
* contracts
* invariants
* ownership model

### HA-2 — Controller leader election hardening

* active leader/follower behavior
* fencing
* bounded failover

### HA-3 — Workflow-service durability and run resumption

* executor ownership
* heartbeats
* orphan recovery
* resume rules

### HA-4 — Doctor leader-election / active-authority model

* one authoritative finding producer
* bounded failover
* no conflicting duplicate authority

### HA-5 — Stateless service replication and health-aware routing

* multiple replicas
* xDS/Envoy endpoint health
* removal of dead endpoints
* no stale black-hole routes

### HA-6 — Ingress HA

* VIP / keepalived if appropriate
* stable entrypoint
* north-south only

You may adjust the phase order if the codebase strongly suggests a better sequence, but explain why.

---

# 6. Interaction With Existing Architecture

The HA design must preserve these already-established rules:

* workflow vs plan separation remains intact
* centralized workflow execution remains the production model
* typed structured actions remain the only safe auto-execution path
* endpoint resolution continues through the canonical resolver
* freshness and truth surfaces remain explicit
* no reintroduction of hidden orchestration inside plans or callbacks

HA work must **strengthen** the architecture, not re-open old ambiguities.

---

# 7. Constraints

Do **not** answer with:

* generic HA theory
* vague best practices
* “add more replicas” without ownership semantics
* “use keepalived” as a broad solution

Do answer with:

* service-specific classification
* concrete mechanisms
* explicit invariants
* codebase-aware implementation phases
* real ownership/failover rules

---

# 8. Final Deliverable Style

Write back as a serious engineering design + implementation plan.

Structure it so it can be reviewed and then executed in phases.

We want:

* the target HA model
* what each service should become
* what must change in code/behavior
* the exact order of work

The goal is:

> move from “all nodes are back online”
> to
> “loss of a node does not break control-plane correctness”

That is the design target.

Proceed.
