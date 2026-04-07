## Globular Final-Release Architecture Hardening Plan

### Purpose

The core convergence model is now strong:

* remediation workflow proven end-to-end
* centralized workflow execution direction frozen
* explicit workflow vs plan boundary established
* typed, audited, blocklisted structured actions working
* endpoint resolver policy adopted for migrated paths

What remains before final release is not inventing new architecture.
It is **removing remaining structural weaknesses in the substrate** so the system becomes boring, explicit, and trustworthy.

This plan is phased.
Each phase must reach a clean commit point before moving to the next.

---

# Phase 1 — Complete Workflow Centralization Safely

### Goal

Finish the centralized workflow execution migration without weakening action registration, actor ownership, or workflow observability.

### Required outcomes

1. WorkflowService becomes the single production workflow executor
2. Actor services expose `WorkflowActorService.ExecuteAction`
3. Fallback dispatch remains **transport-only**
4. YAML action names stay validated against explicit registry
5. Actor-side capability parity tests are added
6. Production workflows have one ownership/distribution path
7. Migrated workflows land in one workflow observability surface

### Work items

* Implement proto changes from the design freeze
* Implement `RegisterFallback` in Router as transport-only
* Add actor capability parity tests:

  * doctor actor
  * controller actor
  * node-agent actor
* Migrate one workflow family at a time:

  1. `remediate.doctor.finding`
  2. release workflows
  3. bootstrap/join/repair
  4. `cluster.reconcile`
* Remove old execution path for each migrated family immediately after migration
* Remove production embeds/filesystem scanning for migrated families
* Ensure callback dialing uses `ResolveDialTarget`

### Constraints

* No semantic changes to existing workflows
* No new remediation rules
* No workflow-vs-plan blur
* No permanent dual-path model

### Acceptance criteria

* All migrated workflows run through WorkflowService only
* `remediate.doctor.finding` appears in unified workflow history
* Unknown actor/action fails tests and runtime with explicit error
* No migrated production workflow still executes via local in-process engine

---

# Phase 2 — Complete Endpoint Resolution & Discovery Hardening

### Goal

Remove remaining connectivity ambiguity and make every service-to-service dial path deterministic.

### Why

The canonical resolver exists, but not all dialers are migrated yet.
Connectivity truth must be boring before release. The endpoint policy already establishes the correct direction. 

### Required outcomes

1. Every service-to-service gRPC dialer uses `ResolveDialTarget`
2. Cluster-mode misconfiguration using localhost/loopback is rejected early
3. Local-only endpoints remain explicit and isolated
4. No ad hoc SNI extraction or loopback rewriting remains in migrated code

### Work items

* Use `docs/endpoint_resolver_migration_inventory.md` as the migration queue
* Migrate all **M-class** dialers
* Investigate all **I-class** dialers and classify them
* Add repository-wide structural test:

  * forbid new ad hoc loopback rewrites
  * forbid raw `SplitHostPort` + custom SNI extraction in service-to-service dialers
* Add explicit mode validation:

  * bootstrap/dev single-node mode
  * cluster/multi-node mode
* Fail fast when cluster mode points at invalid local endpoints for cross-service traffic

### Constraints

* Do not break legitimate local-only endpoints
* Do not introduce new dialing helpers outside the canonical resolver
* Do not defer unresolved “investigate” cases without classification

### Acceptance criteria

* All cross-service gRPC dialers use the resolver
* Cluster-mode bad local endpoints fail validation before runtime
* Structural regression tests prevent old connectivity bugs from returning

---

# Phase 3 — Freshness Contracts Everywhere AI Reads State

### Goal

Make cached vs fresh state explicit for every AI-facing read surface.

### Why

Right now hidden TTL behavior still creates “is it broken or just stale?” confusion.
Projection Clause 4 already says every response must declare freshness and origin. 

### Required outcomes

1. Doctor report/finding surfaces expose freshness explicitly
2. Caller can request cached vs fresh semantics where practical
3. CLI and MCP surfaces show cache age/origin
4. New projections inherit this contract by default

### Work items

* Add `source`, `observed_at`, and cache/snapshot age to doctor responses
* Add freshness mode where practical:

  * cached
  * fresh
  * fresh_if_older_than
* Surface freshness in CLI output
* Apply same pattern to any existing AI-facing resolver/read APIs touched in this phase
* Document freshness semantics in a short design note

### Constraints

* Do not change remediation semantics
* Do not broaden workflow scope
* Do not hide stale reads behind silent caching

### Acceptance criteria

* Every doctor read path clearly indicates freshness
* Operators and AI can distinguish cached state from fresh scans
* Freshness behavior is test-covered

---

# Phase 4 — Build NodeIdentity Projection Exactly Per Contract

### Goal

Implement the first full projection end-to-end with zero deviations from the projection clauses.

### Why

This is the first real AI-facing identity surface and sets the pattern for all later projections. The implementation plan and clauses are already written.

### Question answered

> “Who is this node?”

### Required outcomes

1. Resolve by `node_id`, hostname, IP, or MAC
2. Minimal surface only
3. Includes freshness fields
4. Has direct fallback to source
5. No enrichment with health/packages/logs

### Work items

* Add `ResolveNode` RPC
* Implement projector + reconciler
* Add Scylla tables and migration
* Add CLI surface
* Add MCP tool
* Add unit + integration + fallback tests

### Constraints

* Must obey all 12 projection clauses
* No service/package/metrics/log fields
* No cross-projection enrichment

### Acceptance criteria

* `node_resolve` works from all 4 identifiers
* response is compact and explicit
* fallback path works if projection unavailable
* size/freshness constraints are enforced

---

# Phase 5 — Build `pkg_info` Live Aggregator

### Goal

Give AI and operators one clean package truth surface without creating another projection store.

### Why

This is the next natural read surface after NodeIdentity and is already defined in the introspection plan. 

### Question answered

> “What is this package, where is it desired, where is it installed, and where is it failing?”

### Required outcomes

1. Repository catalog + desired state + installed state merged live
2. CLI surface
3. MCP surface
4. Compact, AI-friendly structure
5. No projection table

### Work items

* Implement `repository.DescribePackage`
* Add CLI command
* Add MCP tool
* Add freshness/origin where practical
* Add tests for:

  * package with no desired state
  * package desired but not installed
  * drifted package
  * missing artifact in repo

### Constraints

* No new Scylla projection table
* Keep response compact
* Do not collapse multiple questions into a giant blob

### Acceptance criteria

* one command/tool call answers package state clearly
* package kind confusion is reduced
* AI no longer needs to correlate three systems manually

---

# Phase 6 — Schema Reference and Ownership Visibility

### Goal

Turn state ownership and key meaning into discoverable structure instead of tribal knowledge.

### Why

After identity and package surfaces exist, the next pain becomes schema/ownership ambiguity. The schema-reference plan already defines this clearly. 

### Required outcomes

1. etcd-backed resources are annotated
2. docs and generated schema artifacts stay in sync
3. CI fails on undocumented etcd-backed types
4. operators/AI can ask “what writes this?” and get a real answer

### Work items

* implement schema extractor
* implement schema lint coverage check
* generate docs + seed artifacts
* add MCP/CLI schema describe surface

### Constraints

* code remains the source of truth
* no manual documentation-only path
* opt-out requires named justification

### Acceptance criteria

* schema docs generated from code
* CI enforces annotation coverage
* AI can discover state ownership without reading source files directly

---

# Phase 7 — Repository / Desired / Installed / Runtime Alignment

### Goal

Make the four-layer state model fully converge across services, applications, and infrastructure.

### Why

The approved state-alignment doc already describes the truth model. Final release should not ship with those layers half-aligned. 

### Required outcomes

1. artifact state
2. desired release state
3. installed observed state
4. runtime health state

…must line up consistently for all package kinds.

### Work items

* harden auto-import on startup/join
* make installed-state registry the public install truth
* finish alignment for applications and infrastructure
* align UI status derivation to frozen vocabulary
* remove ad hoc “Unmanaged” guessing paths not based on the 4-layer model

### Constraints

* no new side-channel truth stores
* no UI-only status hacks
* no divergence across SERVICE / APPLICATION / INFRASTRUCTURE kinds

### Acceptance criteria

* final release shows consistent status across UI/CLI/controller
* desired vs installed drift is computed from canonical sources
* applications/infrastructure follow same model as services

---

# Phase 8 — Low-Risk Structured Remediation Expansion Only

### Goal

Expand self-healing slowly and safely from the proven golden path.

### Why

The remediation path is proven for LOW-risk structured actions. Keep trust growing by adding only safe, reference-backed rules.

### Required outcomes

1. one new LOW-risk rule at a time
2. one reference case per new rule
3. one verified success shape per rule

### Work items

* promote additional rules only after:

  * structured action defined
  * blocklist/risk gate validated
  * reference case documented
  * success path proven end-to-end

### Constraints

* no MEDIUM/HIGH-risk automation in this phase
* no free-form shell
* no broad reconcile dispatch increase until read surfaces above are complete

### Acceptance criteria

* every new rule has a documented golden path
* low-risk auto-remediation stays trustable and boring

---

# Global Architectural Rules

These apply across all phases.

## Rule 1 — No hidden dual paths

If a new path becomes production truth, the old path for that migrated family must be removed promptly.

## Rule 2 — No semantic shortcuts

No callback handler, helper, or plan may hide orchestration logic.

## Rule 3 — No silent freshness

All AI-facing reads must disclose freshness.

## Rule 4 — No new ad hoc endpoint logic

Fix the resolver in one place, not in callers.

## Rule 5 — No free-form actions

Typed, audited, risk-gated, blocklisted execution only.

## Rule 6 — No scope expansion during hardening phases

This plan is about removing weakness, not adding cleverness.

---

# Suggested Execution Order

1. Workflow centralization
2. Endpoint resolver/discovery completion
3. Freshness contracts
4. NodeIdentity
5. `pkg_info`
6. schema reference
7. repository/state alignment
8. more LOW-risk remediation

This order matters.

It follows:

> unify execution → unify connectivity → unify trust in reads → add clean read surfaces → align truth layers → expand automation

Do not reorder casually.

---

# Deliverable Style

For each phase:

1. short design note if architecture is affected
2. implementation
3. tests
4. clean commit point
5. brief summary:

   * what changed
   * what risk was removed
   * what remains out of scope

No giant mixed commits.
No “while I’m here” expansion.

---

# Final release bar

Final release should not ship with:

* split workflow execution models
* split definition ownership
* hidden cache freshness
* fragmented service-to-service dialing
* identity/package truth still requiring tribal knowledge
* canonical status labels still derived by guesswork

Final release should ship with:

* one workflow executor
* one production workflow definition source
* one endpoint resolution policy
* explicit freshness on AI-facing reads
* first-class node/package read surfaces
* aligned artifact/desired/installed/runtime state model
* trusted LOW-risk self-healing

That is the bar.
