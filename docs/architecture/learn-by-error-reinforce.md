## Subject: AI-Assisted Operational Learning Strategy for Globular

We now have a V1 control core with:

* workflow execution
* step-level idempotency / resume policy
* verification handlers
* receipts
* resume decisions
* blocked-run surfacing
* doctor findings
* convergence drills

The next step is **not autonomous action by AI**.

The next step is to build an **AI-assisted learning and advisory layer** that helps the system recognize patterns, explain likely causes, and suggest safer next actions, while keeping the deterministic workflow/policy engine as the source of truth.

---

## Core Principle

AI must **never bypass**:

* workflow execution rules
* verification
* idempotency / resume_policy
* approval gates
* hard safety rails

AI is an **advisor**, not an executor.

The deterministic control core remains authoritative.
AI contributes:

* pattern recognition
* explanation
* recommendation
* confidence scoring
* risk ranking

---

## Objective

Create a learning layer that allows Globular to:

1. Learn from repeated operational failures and recoveries
2. Suggest likely causes and next-safe actions
3. Improve operator speed and doctor usefulness
4. Detect risky rollout patterns before failure occurs
5. Build reusable operational knowledge from real cluster history

---

## Desired End State

When a failure or blocked workflow happens, the system should be able to say:

* this looks similar to N prior incidents
* the most likely cause is X
* the safest next action is Y
* confidence is Z
* this recommendation does **not** override workflow verification or policy

When a rollout is about to happen, the system should be able to say:

* this rollout shape resembles previous failed rollouts
* risk factors are A/B/C
* suggested mitigation is D

---

## Strategic Model

### Layer 1 — Deterministic Core (already exists)

This remains authoritative:

* workflow definitions
* receipts
* verification results
* resume decisions
* doctor findings
* blocked runs
* policy gates

### Layer 2 — Operational Memory

This is the structured historical dataset:

* run history
* step history
* resume decisions
* verification outcomes
* receipts
* doctor findings
* remediation actions
* final outcomes

### Layer 3 — Advisory Intelligence

This is the new AI layer:

* incident similarity search
* failure pattern clustering
* recommendation generation
* risk scoring
* natural-language explanation

### Layer 4 — Human / Policy Gate

Final action remains controlled by:

* workflow engine
* operator approval
* doctor remediation workflow
* hard safety rules

---

## Phase A — Define the Learning Dataset

### Goal

Turn workflow and doctor history into a reusable operational corpus.

### Required data sources

Use only structured and authoritative sources:

* WorkflowRun records
* WorkflowStep records
* workflow events
* step receipts
* resume decision records
* verification results
* doctor findings
* remediation workflow runs
* node/package/runtime health snapshots where available

### Normalize into a common incident schema

Define a derived record such as:

```json
{
  "incident_id": "...",
  "cluster_id": "...",
  "workflow_name": "node.join",
  "run_id": "...",
  "step_id": "install_scylladb",
  "package_name": "scylladb",
  "resume_policy": "verify_effect",
  "idempotency": "verify_then_continue",
  "verification_status": "inconclusive",
  "receipt_present": false,
  "run_status": "BLOCKED",
  "doctor_finding_type": "workflow_blocked",
  "symptoms": [
    "scylla health probe failed",
    "step blocked after resume"
  ],
  "remediation_action": "node.repair",
  "outcome": "recovered",
  "time_to_recovery_sec": 420
}
```

### Questions to answer

* What exact schema should represent an “incident”?
* What fields are required to compare incidents meaningfully?
* What fields should remain raw vs derived?

### Services likely impacted

* workflow service
* cluster-doctor
* ai-memory or a new learning/index service
* possibly monitoring/event service for enrichment

---

## Phase B — Build Incident Projection / Indexing

### Goal

Create a derived searchable knowledge base from raw workflow history.

### Needed capability

Projection job or service that:

* reads workflow runs / steps / doctor findings
* groups related events into incidents
* extracts structured features
* stores searchable summaries and embeddings

### Output types

1. **Structured incident index**

   * exact filters
   * similarity features
   * severity / risk / outcome labels

2. **Narrative summaries**

   * concise description of what happened
   * what decision was taken
   * what fixed it

3. **Embeddings / semantic index**

   * for similarity search
   * for “have we seen this before?”

### Questions to answer

* Should this live in ai-memory, workflow service, or a new dedicated service?
* Should indexing be synchronous on run completion or asynchronous via event stream?
* Where should embeddings and summaries be stored?

### Services likely impacted

* ai-memory
* workflow service
* event service
* ScyllaDB / object store depending on storage choice

---

## Phase C — Similarity Search and Pattern Detection

### Goal

Enable retrieval of similar incidents and repeated failure patterns.

### Required capabilities

1. **Incident similarity**

   * find runs with same workflow/step/policy/verification pattern
   * semantic similarity on summaries
   * cluster-specific and cross-cluster comparisons if multi-cluster learning is desired later

2. **Pattern clustering**

   * repeated blocked runs for same step
   * repeated remediation success for same symptom set
   * repeated rollout failure after same precursor signals

3. **Risk feature extraction**

   * package involved
   * node profile
   * service health state
   * stale receipt absence
   * verification inconclusive frequency
   * related doctor findings
   * prior recovery path success rate

### Questions to answer

* What similarity dimensions matter most first?
* How do we separate “same symptom” from “same cause”?
* Should cluster-local history be ranked above global history?

### Services likely impacted

* ai-memory / learning service
* cluster-doctor
* optionally monitoring/telemetry pipeline

---

## Phase D — Advisory Output Model

### Goal

Allow AI/doctor to generate useful recommendations in a controlled format.

### Recommendation format

Recommendations should be structured, not free-form only.

Example:

```json
{
  "recommendation_type": "resume_guidance",
  "likely_cause": "Scylla package install completed but runtime verification failed after executor interruption",
  "suggested_next_action": "run node.repair on target node",
  "confidence": 0.84,
  "based_on_incidents": ["inc-123", "inc-456", "inc-789"],
  "risk_level": "medium",
  "requires_approval": true,
  "why": [
    "3 similar incidents matched same workflow step and verification outcome",
    "node.repair resolved 2/3 prior cases",
    "blind re-execution is unsafe for this step class"
  ]
}
```

### Allowed recommendation classes

* likely cause
* similar incidents
* suggested remediation workflow
* suggested rollback / repair path
* preflight rollout warning
* operator explanation

### Disallowed behavior

AI must not:

* mutate workflow state directly
* bypass verification
* bypass approval
* invent new unsafe actions outside typed workflow/remediation surfaces

### Services likely impacted

* cluster-doctor
* ai-executor or advisory service
* globular-admin UI

---

## Phase E — First Product Slice: AI-Assisted Doctor

### Goal

Ship the smallest valuable feature first.

### Proposed feature

When doctor produces a finding for:

* blocked workflow
* repeated verification failure
* repeated remediation loop
* package/runtime drift

AI augments the finding with:

* similar historical incidents
* likely cause
* recommended next workflow/action
* confidence score
* explanation

### Why start here

* high value
* low risk
* grounded in existing doctor workflow
* no autonomous mutation required

### Output example

Doctor finding:

* Workflow blocked at `install_scylladb`
  AI augmentation:
* “This matches 4 prior incidents where Scylla install succeeded but runtime health stayed absent after executor interruption. In 3/4 cases, `node.repair` restored service. Confidence: 0.81.”

### Services likely impacted

* cluster-doctor
* ai-memory / learning service
* UI

---

## Phase F — Preemptive Rollout Risk Advisor

### Goal

Use learned patterns before failures occur.

### Feature

Before a rollout begins, analyze:

* package
* workflow shape
* node profile
* prior failure history
* current cluster health signals

Return:

* low / medium / high risk
* likely weak points
* suggested mitigation steps

### Important constraint

This is advisory only.
The rollout still follows deterministic workflow logic.

### Example

“Releasing package X to nodes with profile Y while MinIO health is degraded resembles 6 prior failed rollouts. Suggested mitigation: stabilize MinIO first or reduce rollout parallelism.”

---

## Phase G — Feedback / Learning Loop

### Goal

Use outcomes to improve recommendation quality.

### Needed signals

Track whether a recommendation led to:

* successful recovery
* blocked run resolved
* repeated failure
* operator override
* ignored suggestion

### This enables

* recommendation ranking by actual success
* confidence recalibration
* demotion of bad advice
* cluster-specific adaptation

### Important constraint

Learning should improve recommendation quality, not action authority.

---

## Safety Model

### Non-negotiable rules

AI advisory layer must never:

* execute mutations directly
* override workflow verification
* override run BLOCKED status
* bypass approval for risky actions
* write arbitrary etcd/object-store/control-plane mutations
* invent actions outside structured workflow/remediation APIs

### Required interaction model

AI can:

* explain
* search history
* compare incidents
* rank likely causes
* suggest typed next actions

System core decides and enforces.

---

## Initial Implementation Order

### AL-1 — Incident schema and projection

* define normalized incident record
* implement projection from workflow/doctor history

### AL-2 — Incident index

* build structured + semantic searchable incident store

### AL-3 — Similar incident query API

* input: finding or blocked run
* output: top similar incidents + summary

### AL-4 — AI-assisted doctor augmentation

* enrich doctor findings with likely cause / recommended next action / confidence

### AL-5 — Recommendation outcome tracking

* log whether recommendations helped, were used, or were rejected

### AL-6 — Preflight rollout risk advisor

* advisory only before rollout start

---

## Deliverables Requested

Please produce:

1. A phased implementation plan for the learning/advisory layer
2. A proposed incident schema
3. Service impact analysis:

   * what should live in ai-memory
   * what should live in workflow service
   * what belongs in doctor
   * whether a new service is warranted
4. Storage/indexing proposal:

   * structured records
   * embeddings
   * summaries
5. Safety analysis:

   * how to guarantee AI remains advisory
6. Suggested first vertical slice to ship safely

---

## Key Principle

We are not building an AI that controls the cluster.

We are building:

> **a learning layer that helps Globular recognize patterns, explain likely causes, and suggest safer next actions, while the deterministic workflow engine remains authoritative**

---

## End Goal

A system that:

* learns from every incident
* gets better at diagnosis and recommendation
* reduces operator guesswork
* improves over time
* remains policy-bound and safe
