## Globular Test Strategy

### Purpose

This strategy is not just about checking if code works.

It is designed to prove that Globular can:

1. reach a correct stable state
2. detect drift when stable state is lost
3. select the right remediation path
4. execute bounded corrections safely
5. verify convergence truthfully
6. repeat this behavior under pressure

The goal is to move from:

> “the architecture looks correct”

to:

> “the cluster behaves correctly under realistic conditions”

---

# 1. Testing Principles

## 1.1 Test the system in layers

Every test belongs to one of these layers:

* **L1 — Unit correctness**

  * pure logic
  * projection formatting
  * endpoint normalization
  * registry validation
  * action routing

* **L2 — Integration correctness**

  * service-to-service behavior
  * RPC wiring
  * workflow execution
  * projection fallback
  * repository/desired/install alignment

* **L3 — Convergence behavior**

  * injected drift
  * remediation selection
  * bounded repair
  * verification of stable state

* **L4 — Operational endurance**

  * repeated runs
  * multiple simultaneous issues
  * long-running cluster stability
  * reproducibility over time

---

## 1.2 Never test only success

Every meaningful workflow or projection must be tested in at least 3 ways:

* **happy path**
* **bounded failure**
* **truthfulness under degraded conditions**

Example:

* happy: service stopped → remediation restarts it → verification succeeds
* bounded failure: finding has no structured action → workflow fails cleanly
* truthfulness: stale cache is reported as stale, not presented as fresh

---

## 1.3 Test behavior, not just implementation

Passing unit tests is not enough.

For every major feature, the important question is:

> “Did the system behave correctly as a convergence machine?”

Not just:

> “Did this function return expected output?”

---

# 2. Test Environments

## 2.1 Environment A — Fast simulation cluster

Use Docker or equivalent lightweight virtual environment for:

* repeated workflow drills
* endpoint resolution tests
* freshness behavior
* projection behavior
* repository/desired/install mismatch scenarios
* CI-compatible regression tests

Purpose:

* fast reset
* controlled injection
* repeatability

This is the **convergence gym**.

---

## 2.2 Environment B — Real cluster validation

Use at least one real cluster for:

* systemd behavior
* real TLS/SAN validation
* real node identity behavior
* join/bootstrap flows
* package install/remove flows
* runtime health and actual service recovery
* network reality

Purpose:

* validate that the cluster works outside the simulator

This is the **truth exam**.

---

## 2.3 Promotion rule

A feature is not considered fully validated until it passes:

1. simulation drill
2. real-cluster validation

---

# 3. Test Tracks

---

## Track A — Workflow Centralization Validation

### Goal

Prove centralized workflow execution behaves the same as before, but with unified execution and observability.

### A.1 Static tests

* YAML action names resolve against registry
* actor capability parity holds
* unknown actor/action fails test
* no migrated workflow still uses local embed/filesystem production path

### A.2 Integration tests

* `ExecuteWorkflow` launches through WorkflowService
* callbacks route correctly to:

  * cluster-doctor
  * cluster-controller
  * node-agent
* step results are auto-recorded to unified workflow storage
* child workflow spawning works
* endpoint dialing for callbacks uses canonical resolver

### A.3 Behavior tests

* `remediate.doctor.finding` unchanged semantically
* release workflow unchanged semantically
* bootstrap/join/repair unchanged semantically
* failure path still triggers `onFailure` correctly

### A.4 Acceptance

* one production executor
* one run/step history surface
* no semantic drift
* no dual-path execution for migrated families

---

## Track B — Endpoint Resolution and Discovery

### Goal

Prove that service-to-service connectivity is deterministic and TLS-safe.

### B.1 Unit tests

* `ResolveDialTarget` for:

  * `127.0.0.1`
  * `[::1]`
  * `localhost`
  * mesh DNS names
  * non-loopback IPs
* loopback literals normalize correctly
* `ServerName` chosen correctly

### B.2 Structural tests

* migrated dialer files use resolver
* no raw loopback rewrite logic returns
* no ad hoc SNI extraction returns

### B.3 Integration tests

* doctor → controller
* doctor → workflow-service
* doctor → node-agent
* controller → node-agent
* node-agent → controller

### B.4 Misconfiguration tests

* cluster mode with localhost/loopback cross-service endpoint fails early
* dev/bootstrap local-only endpoint still works where allowed

### B.5 Acceptance

* all service-to-service gRPC dials are resolver-based
* TLS SAN issues from loopback drift disappear
* bad cluster-mode config is rejected before runtime

---

## Track C — Freshness Contracts

### Goal

Prove that cached vs fresh state is explicit and trustworthy.

### C.1 Response-shape tests

Every AI-facing read surface exposes:

* `source`
* `observed_at`
* age/freshness metadata where applicable

### C.2 Cache-mode tests

For doctor and other read surfaces:

* cached mode returns cached result
* fresh mode forces rescan
* `fresh_if_older_than` behaves correctly

### C.3 CLI/MCP tests

* freshness visible in human output
* freshness visible in structured output
* stale results are clearly labeled

### C.4 Failure tests

* unavailable fresh scan does not silently fall back without disclosure
* stale cached data is not presented as fresh truth

### C.5 Acceptance

* operators and AI can always distinguish fresh truth from cached state

---

## Track D — NodeIdentity Projection

### Goal

Prove the first projection fully obeys the projection clauses.

### D.1 Unit tests

* projector writes expected rows
* reverse lookup tables correct
* reconciler repairs missing rows
* reconciler removes stale rows

### D.2 Integration tests

* `ReportStatus` updates source
* projection row created
* resolve by:

  * node_id
  * hostname
  * IP
  * MAC

### D.3 Fallback tests

* Scylla unavailable → source fallback still works
* stale projection → source path still returns correct answer

### D.4 Contract tests

* response includes only:

  * node_id
  * hostname
  * ips
  * macs
  * labels
  * source
  * observed_at
* response size within contract
* no enrichment with health/packages/logs

### D.5 Acceptance

* `node_resolve` answers exactly one question:

  * “Who is this node?”

---

## Track E — `pkg_info` Live Aggregator

### Goal

Prove one-call package truth without inventing a new source of truth. 

### E.1 Integration tests

Aggregate correctly from:

* repository catalog
* desired state
* installed state

### E.2 Scenario tests

* artifact exists, no desired state
* desired state exists, not installed
* installed drifted from desired
* installed but missing in repo
* multiple nodes, mixed install states
* failing nodes included correctly

### E.3 Output-shape tests

* compact AI-friendly response
* no giant blob
* package kind visible
* per-node install/failure entries stable

### E.4 Acceptance

* one query explains package truth clearly enough to avoid cross-system detective work

---

## Track F — Schema Reference

### Goal

Prove that state ownership and schema meaning are discoverable and enforced. 

### F.1 Generation tests

* extractor produces docs
* extractor produces seed artifacts
* output stable and reproducible

### F.2 Coverage tests

* etcd-backed types without annotations fail CI
* generated artifacts out of sync fail CI

### F.3 Query tests

* `schema_describe` returns writer/readers/invariants correctly
* source file + pattern visible

### F.4 Acceptance

* AI and operators can ask “what owns this?” and get a real answer

---

## Track G — Repository / Desired / Installed / Runtime Alignment

### Goal

Prove the 4-layer state model is truthful and aligned. 

### G.1 Startup/join tests

* desired-state auto-import works
* idempotent import works repeatedly
* join/import does not create duplicates or lies

### G.2 State mismatch tests

* artifact exists but desired missing
* desired exists but installed missing
* installed exists but runtime unhealthy
* UI/CLI/controller all agree on status label

### G.3 Package-kind coverage

Run same lifecycle tests for:

* SERVICE
* APPLICATION
* INFRASTRUCTURE

### G.4 Acceptance

* status labels are derived from canonical sources only
* no guess-based “Unmanaged” logic remains

---

## Track H — Structured Remediation / Self-Healing

### Goal

Prove safe, bounded self-healing for known LOW-risk failures.

### H.1 Golden path tests

For each LOW-risk rule:

* inject fault
* expect finding
* expect structured action
* run workflow
* verify convergence
* save reference case

### H.2 Failure path tests

* finding has no structured action
* approval missing when required
* execution rejected
* verification fails
* actor callback unavailable

### H.3 Safety tests

* blocklisted actions never auto-execute
* risk gate respected
* audit trail always written
* no free-form shell escape path exists

### H.4 Concurrency tests

* multiple LOW-risk findings at once
* same finding triggered repeatedly
* remediation idempotence

### H.5 Acceptance

* LOW-risk automation becomes repeatable and boring

---

# 4. Convergence Drill Program

This is the practical “training phase.”

Each drill must define:

* injected fault
* expected finding
* expected workflow
* expected action
* expected verification result
* pass/fail rule
* time to convergence

## Initial drill set

### Drill 01 — Stopped service

* stop `globular-torrent.service`
* expect `node.systemd.units_running`
* expect `SYSTEMCTL_RESTART`
* expect workflow success
* expect finding cleared

### Drill 02 — Stopped node-agent

* stop node-agent
* expect heartbeat/inventory issue
* expect restart node-agent
* expect node returns
* expect finding cleared

### Drill 03 — Stale inventory

* delay/interrupt inventory sync
* expect inventory-related finding
* expect remediation if defined
* expect truth surfaces to show freshness correctly

### Drill 04 — Desired/install mismatch

* alter desired or installed state
* expect drift visible
* no lying status labels
* expected remediation path if supported

### Drill 05 — Broken endpoint resolution

* inject wrong endpoint
* expect failure to be explicit
* no silent nonsense fallback
* if corrected, reconvergence should occur cleanly

### Drill 06 — Missing artifact

* remove repo artifact for desired package
* expect correct package truth output
* remediation should not invent impossible install success

### Drill 07 — Parallel low-risk incidents

* stop multiple safe services on multiple nodes
* expect workflows remain observable and bounded
* no deadlock, no hidden orchestration

---

# 5. Pass / Fail Model

A feature is **green** only when all three are true:

1. **Correctness**

   * expected result is returned

2. **Truthfulness**

   * system reports accurate source/freshness/status

3. **Convergence**

   * system reaches or clearly fails to reach stable state in a bounded way

A feature is **not green** if it merely “works sometimes.”

---

# 6. Test Cadence

## Per commit

* unit tests
* structural tests
* registry validation
* contract tests

## Per phase

* integration tests
* one or more convergence drills
* real-cluster validation

## Before final release

* full drill suite
* repeated multi-run stability test
* mixed-fault soak test
* manual operator review of truth surfaces and workflow observability

---

# 7. Release Readiness Gate

The cluster may be considered ready for production only when:

1. workflows are centralized and observable
2. connectivity is deterministic
3. freshness is explicit
4. identity/package truth surfaces are stable
5. state-layer alignment is trustworthy
6. LOW-risk remediation is repeatable
7. convergence drills pass repeatedly in both simulated and real environments

Not once.

Repeatedly.

---

# 8. Recommended Execution Order

1. Workflow centralization tests
2. Endpoint/discovery tests
3. Freshness tests
4. NodeIdentity tests
5. `pkg_info` tests
6. schema reference tests
7. state alignment tests
8. convergence drill program
9. soak test
10. production-readiness review

---

# 9. Deliverables

For each phase, produce:

* test list
* pass/fail conditions
* known out-of-scope items
* one short result summary
* any new regression tests added

For each new remediation rule, produce:

* one reference case
* one golden-path success record
* one failure-path record

---

# Final Principle

The purpose of testing Globular is not to show that nothing ever fails.

It is to prove this:

> when drift appears, the system sees it clearly, reacts within its rules, and returns to stable state without improvisation.

That is the standard.
