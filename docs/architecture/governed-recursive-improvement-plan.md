# Governed Recursive Improvement Plan

**Status:** Proposed architecture and implementation roadmap  
**Repository baseline:** `globulario/services` at `master` commit `eeb90f7a762074f3af93b20007472cd1f47e9719`  
**Scope:** Globular runtime evidence feeding governed improvement proposals back into Sensei  
**Primary objective:** Demonstrate a safe, attributable loop from architectural intent to a bounded code change, controlled runtime execution, deterministic evaluation, rollback, and durable knowledge proposal.

## 1. Executive position

Globular and Sensei already contain much of the substrate needed for a recursive improvement loop:

- Sensei can reconstruct architectural context, identify invariants and failure modes, constrain proposed edits, and produce knowledge proposals.
- Globular has typed workflows, correlation IDs, resumable execution, verification actions, install receipts, runtime health, doctor findings, operational incidents, AI memory, and explicit authority boundaries.
- The workflow service already projects failed and blocked runs into AI memory in `golang/workflow/workflow_server/incident_projection.go`.
- The awareness MCP surface already documents `preflight`, `propose_from_incident`, and `learn_from_fix` flows in `docs/awareness/mcp_tools.md`.

The missing piece is not an autonomous agent that repeatedly edits production. The missing piece is a trustworthy causal chain:

```text
objective
  -> exact repository change
  -> governed admission
  -> exact artifact and deployment
  -> controlled observation
  -> attributable evidence
  -> deterministic verdict
  -> rollback or acceptance
  -> candidate architectural knowledge
  -> governed promotion
```

This plan names that mechanism the **Governed Recursive Improvement Loop**.

The first milestone is deliberately narrow: one low-risk, reversible optimization executed on a non-production Globular cluster, with every transition recorded and independently verifiable. The system must be able to return `INCONCLUSIVE` without treating uncertainty as success.

## 2. What this plan does not claim

This plan does **not** attempt to deliver unrestricted self-modifying software.

It does not permit:

- Sensei to deploy code directly;
- an LLM to decide whether an experiment succeeded;
- runtime telemetry to mutate invariants automatically;
- a single successful run to become a universal architectural law;
- experimentation on the control plane without an explicit blast-radius policy;
- production rollout without rollback proof;
- changes to Sensei's own governance rules through the same low-authority loop;
- continuous unbounded optimization;
- acceptance based only on a positive average metric while tail latency, errors, resource use, security, or doctor findings regress.

The target is **bounded autonomy under explicit authority**, not autonomy without authority.

## 3. Existing Globular substrate

The implementation should extend current components rather than introduce a parallel orchestration platform.

| Existing capability | Current location | Role in the loop |
|---|---|---|
| Four-layer truth model | `docs/ai/ai-operating-rules.md` | Defines artifact, desired, installed, and runtime truth that experiments must reconcile |
| Workflow execution and correlation | Workflow service and workflow protobufs | Executes the candidate and binds retries to a stable run/correlation identity |
| Step verification | `golang/workflow/engine/actors_verification.go` | Produces present/absent/inconclusive evidence from authoritative and local sources |
| Resume and receipt policy | `golang/workflow/v1alpha1/types.go` | Prevents ambiguous replay and records durable completion breadcrumbs |
| Canonical install receipts | `golang/node_agent/node_agent_server/internal/installreceipt/` | Proves exact installed outputs and their digests |
| Runtime incident projection | `golang/workflow/workflow_server/incident_projection.go` | Supplies advisory learning data from failed and blocked runs |
| Cluster Doctor | `golang/cluster_doctor/` | Supplies health and invariant-oriented guardrail findings |
| AI memory | `golang/ai_memory/` | Stores advisory summaries and incident projections, not execution authority |
| Operational knowledge schema | `docs/operational-knowledge/SCHEMA.md` | Stores curated, versioned operational knowledge after promotion |
| Awareness preflight and learning tools | `docs/awareness/mcp_tools.md` | Constrains changes and creates draft knowledge proposals |
| GitHub change identity | repository commit graph | Binds base SHA, head SHA, changed files, and diff digest |
| Sensei | external governed architecture system | Owns architectural admission, change audit, candidate lessons, and promotion policy |

### Important existing boundary

`incident_projection.go` correctly describes AI memory incidents as advisory and read-only from the workflow engine's perspective. That boundary must remain.

AI memory may hold searchable projections, explanations, and links. It must not become the authoritative store for experiment execution, acceptance, or deployment identity because incident storage is currently fire-and-forget and may be unavailable.

## 4. Target architecture

```text
┌──────────────────────────────── SENSEI ────────────────────────────────┐
│                                                                       │
│  ImprovementIntent                                                    │
│       │                                                               │
│       ▼                                                               │
│  architecture briefing -> proposed diff -> preflight -> diff audit    │
│       │                                      │                        │
│       └──────────── signed ChangeEnvelope ───┘                        │
│                                      │                                │
└──────────────────────────────────────┼────────────────────────────────┘
                                       │ admitted proposal only
                                       ▼
┌──────────────────────────────── GLOBULAR ──────────────────────────────┐
│                                                                       │
│  ExperimentPlan -> workflow -> canary install -> workload -> observe  │
│                         │             │                    │           │
│                         │             └ install receipts   │           │
│                         └ correlation/run IDs              │           │
│                                                           ▼           │
│                                                EvidenceReceipt         │
│                                                           │           │
│                                            deterministic evaluator     │
│                                               │          │            │
│                                          ACCEPTED   REJECTED/          │
│                                               │      INCONCLUSIVE       │
│                                               │          │            │
│                                               │        rollback         │
└───────────────────────────────────────────────┼───────────────────────┘
                                                │ accepted evidence bundle
                                                ▼
┌──────────────────────────────── SENSEI ────────────────────────────────┐
│  draft lesson -> corroboration -> human review -> governed promotion  │
└───────────────────────────────────────────────────────────────────────┘
```

## 5. Authority model

Every record must have one owner and one purpose.

| Record | Owner | Authoritative for | Must not be used as |
|---|---|---|---|
| `ImprovementIntent` | Sensei task/session authority | Why a change is proposed and which objective it serves | Runtime proof |
| `ChangeEnvelope` | Sensei admission authority | Exact repository, base/head SHA, diff digest, scope, required tests, forbidden approaches | Deployment completion |
| `ExperimentPlan` | Globular workflow authority | How an admitted change will be built, deployed, measured, stopped, and rolled back | Architectural law |
| `ExecutionRecord` | Workflow service | What steps ran, when, where, with which correlation and run IDs | Performance conclusion |
| `InstallReceipt` | Node agent | Exact installed artifact and on-disk output identity | Application health |
| `EvidenceReceipt` | Evidence collector/evaluator under workflow ownership | Baseline/candidate observations and deterministic evaluation inputs | Knowledge promotion by itself |
| `ImprovementVerdict` | Deterministic evaluator | Accepted, rejected, or inconclusive outcome under a named policy version | Universal truth outside its declared scope |
| `KnowledgeProposal` | Sensei proposal authority | Candidate failure mode, invariant, pattern, or recommendation | Enforced rule before promotion |
| `KnowledgePromotion` | Human/governance authority | Approved durable architectural knowledge | Permission to rewrite core governance recursively |

### Required authority rules

1. Sensei proposes; Globular executes.
2. Globular observes; a deterministic evaluator decides the experiment outcome.
3. Runtime evidence creates a **candidate** lesson, never an invariant directly.
4. AI memory stores projections and searchable context, not execution truth.
5. The workflow service owns experiment lifecycle state.
6. Node agents own local execution and first-hand on-disk evidence.
7. Cluster-wide desired state remains controller-owned.
8. No component may claim success for evidence it cannot independently identify by digest and run ID.

## 6. Canonical records

The first implementation should define versioned Go structs with canonical JSON serialization and validators. Protobuf can follow when cross-service RPC requirements stabilize. Premature protobuf expansion would make the first experiment slower without proving the contract.

Suggested package:

```text
golang/improvement/
  v1alpha1/
    types.go
    validate.go
    canonical.go
    digest.go
```

### 6.1 ImprovementIntent

```yaml
api_version: improvement.globular.io/v1alpha1
kind: ImprovementIntent
intent_id: imp-...
task_session_id: ...
repository: globulario/services
objective:
  statement: Reduce p99 latency for a named read-only RPC path.
  primary_metric: rpc_latency_p99_ms
  target: "<= baseline * 0.80"
  guardrails:
    error_rate: "<= baseline + 0.001"
    memory_rss: "<= baseline * 1.05"
    restart_count: "== 0"
    new_doctor_errors: "== 0"
scope:
  services: [repository]
  files: []
  environments: [development-canary]
non_goals: []
created_by: sensei
created_at: ...
```

The intent is immutable after admission. A changed objective requires a new intent ID.

### 6.2 ChangeEnvelope

The envelope binds the exact proposed change to the task and required proof.

Minimum fields:

- repository identity;
- base SHA;
- head SHA;
- canonical diff SHA-256;
- task session ID;
- intent ID;
- changed files;
- affected services/packages;
- architectural claims and invariants consulted;
- forbidden fixes;
- required tests and searches;
- admission result and policy version;
- expiration time;
- signer/issuer identity;
- one-use execution nonce.

Globular must reject an experiment when the checked-out commit, built artifact provenance, or diff digest does not match the admitted envelope.

### 6.3 ExperimentPlan

Minimum fields:

- experiment ID and intent ID;
- admitted change-envelope digest;
- target cluster ID and node/cohort IDs;
- workflow definition version;
- exact package identity and artifact SHA-256;
- baseline artifact identity;
- workload definition and workload digest;
- warm-up duration;
- baseline and candidate observation windows;
- minimum sample count;
- metric queries and units;
- guardrail thresholds;
- abort thresholds;
- rollback workflow and rollback artifact identity;
- maximum wall-clock duration;
- operator approval receipt when required;
- evaluator policy version.

The plan is immutable once execution begins.

### 6.4 EvidenceReceipt

The evidence receipt is an append-only result bundle. It should contain raw references plus normalized summaries.

Minimum fields:

- experiment ID;
- workflow run ID and correlation ID;
- cluster and cohort identity;
- exact base and candidate artifact identities;
- exact change-envelope digest;
- workload digest;
- observation source identities;
- baseline and candidate window timestamps;
- sample counts;
- metric values and units;
- doctor findings before and after;
- service restart/crash/OOM counts;
- verification action results including inconclusive results;
- install receipt digests;
- missing evidence and blind spots;
- canonical receipt digest;
- collector version;
- timestamp and signer/issuer.

Large logs and time-series data should remain in their native stores. The receipt contains content-addressed references and the minimum normalized evidence required to reproduce the verdict.

### 6.5 ImprovementVerdict

Allowed results:

- `ACCEPTED`
- `REJECTED`
- `INCONCLUSIVE`
- `ABORTED`
- `ROLLED_BACK`

The verdict must be generated by deterministic code using a named policy version. An LLM may explain the verdict afterward, but cannot set it.

Example acceptance rule:

```text
ACCEPTED only when:
  primary target passes
  AND all guardrails pass
  AND minimum samples are present
  AND no required source is missing
  AND change/artifact/workload identities match
  AND no new ERROR doctor finding appears
  AND rollback verification is available
otherwise:
  REJECTED when a threshold is violated
  INCONCLUSIVE when evidence is incomplete or contradictory
```

## 7. Evidence quality and causal attribution

The difficult problem is not collecting metrics. It is proving that the admitted change plausibly caused the measured result.

The first version must enforce these constraints:

1. **One bounded variable**  
   The candidate experiment changes one declared optimization dimension. Unrelated refactors invalidate attribution.

2. **Exact identity chain**  
   Repository commit, diff digest, package manifest, artifact digest, installed receipt, workflow run, and evidence receipt must agree.

3. **Comparable workload**  
   Baseline and candidate use the same versioned workload definition, request mix, concurrency, duration, and dataset snapshot.

4. **Warm-up isolation**  
   Warm-up samples are excluded from evaluation.

5. **Minimum evidence**  
   Missing metrics never default to zero or success.

6. **Repeated trials**  
   A single passing run may accept the local experiment but cannot promote a universal rule.

7. **Guardrail diversity**  
   Acceptance considers latency, errors, resource use, runtime health, and doctor findings. A faster crash is not an optimization.

8. **Independent observation where possible**  
   A service should not be the sole source attesting its own health. Use workflow state, node-agent runtime truth, systemd status, metrics, and doctor findings.

9. **Clock and window integrity**  
   Observation windows must use synchronized cluster time or explicitly record clock uncertainty.

10. **Fail closed on identity mismatch**  
    Any SHA, cluster, cohort, workload, or policy mismatch yields `INCONCLUSIVE` or `ABORTED`.

## 8. Knowledge promotion ladder

Runtime evidence must climb a promotion ladder instead of jumping directly into invariants.

| Level | Meaning | Minimum evidence | Enforcement effect |
|---|---|---|---|
| Observation | A measured event occurred | One valid evidence receipt | None |
| Candidate lesson | A change appears related to an outcome | One accepted or rejected experiment with attribution | Searchable draft only |
| Supported lesson | Repeated evidence supports the same conclusion | Multiple independent runs or one incident plus controlled reproduction | Advisory recommendation |
| Recommended pattern | Useful default with known scope and exceptions | Corroborated results across declared environments | Preflight recommendation/warning |
| Governed contract | Violation is harmful and rule is sufficiently general | Strong corroboration, explicit owner, required tests, human approval | Admission constraint |
| Core principle | System-wide constitutional rule | Exceptional review and migration analysis | Highest governance tier |

Sensei's `learn_from_fix` or equivalent integration should create a draft proposal that links back to evidence receipt digests. Promotion remains explicit and reviewable.

## 9. Implementation roadmap

### Phase 0: Contract closure and repository mapping

**Goal:** Freeze terminology, authority, and the minimum demonstrator before code changes.

**Work:**

- Add this architecture document.
- Identify the authoritative workflow run store and its extension mechanism.
- Identify metric sources available in development clusters.
- Choose the first low-risk candidate path.
- Define the exact Sensei invocation boundary: CLI, MCP, or both.
- Define which actor owns evidence collection and verdict persistence.
- Record threat model and approval matrix.

**Exit criteria:**

- Every canonical record has one owner.
- No record has ambiguous serialization or storage.
- The first candidate is reversible and excludes topology, membership, PKI, RBAC, identity, storage quorum, and bootstrap changes.
- Required metrics exist before implementation begins.

**Estimated effort:** 2-4 focused engineering days.

### Phase 1: Canonical improvement contracts

**Goal:** Implement versioned records, canonical serialization, digests, and validation without executing experiments.

**Proposed files:**

```text
golang/improvement/v1alpha1/types.go
golang/improvement/v1alpha1/validate.go
golang/improvement/v1alpha1/canonical.go
golang/improvement/v1alpha1/digest.go
golang/improvement/v1alpha1/*_test.go
docs/architecture/improvement-records.md
```

**Required behavior:**

- Reject unknown schema versions.
- Reject missing owners, identities, thresholds, rollback definitions, or observation windows.
- Canonicalize maps and lists where ordering is semantically irrelevant.
- Compute stable SHA-256 digests.
- Reject mutation after execution starts.
- Preserve unknown evidence extensions only under a namespaced field, not as silently authoritative values.

**Tests:**

- deterministic serialization;
- digest stability across repeated runs;
- mutation changes digest;
- invalid metric units rejected;
- missing rollback rejected;
- missing exact artifact identity rejected;
- identity mismatch rejected;
- golden fixture compatibility.

**Exit criteria:** All record tests are deterministic and pass on Linux and Windows where applicable.

**Estimated effort:** 3-5 days.

### Phase 2: Evidence-only workflow

**Goal:** Prove Globular can produce a complete evidence receipt for an unchanged deployment.

This phase intentionally performs no optimization. It validates the measuring apparatus before trusting it.

**Work:**

- Add an experiment workflow definition that:
  1. validates the plan;
  2. resolves baseline artifact identity;
  3. verifies installed state;
  4. executes a versioned workload;
  5. captures metrics and doctor findings;
  6. writes an evidence receipt;
  7. re-runs the same workload without changing the artifact;
  8. evaluates expected equivalence.
- Persist the authoritative execution/evidence record with the workflow run, or in a dedicated workflow-owned store.
- Project a searchable summary to AI memory only after the authoritative write succeeds.

**Required result:** The evaluator should normally return `INCONCLUSIVE` or a special `BASELINE_VALIDATED` state when comparing an artifact to itself. It must not manufacture an improvement.

**Tests:**

- AI memory unavailable does not lose authoritative evidence;
- missing metric source yields `INCONCLUSIVE`;
- workload digest mismatch aborts;
- baseline/candidate identity mismatch is visible;
- evidence receipt can be regenerated from the same normalized inputs;
- correlation ID survives retries;
- duplicate terminal callbacks do not duplicate authoritative receipts.

**Exit criteria:** Three consecutive evidence-only runs produce valid, reproducible receipts.

**Estimated effort:** 4-7 days.

### Phase 3: Canary deployment and rollback

**Goal:** Execute an admitted candidate artifact on an isolated development cohort and prove rollback before measuring improvement.

**Work:**

- Add experiment cohort selection with explicit node IDs.
- Require a development/test cluster phase and deny production by default.
- Validate current cluster health before starting.
- Resolve and install the exact candidate artifact through existing repository and workflow paths.
- Verify install receipts and runtime health.
- Execute a rollback rehearsal before enabling acceptance.
- Add abort triggers for error rate, restart loops, OOM, health-check failure, or new doctor errors.
- Restore the exact baseline artifact after rejected, inconclusive, aborted, or expired runs.

**Safety rules:**

- No direct shell execution from Sensei.
- Node-agent remains the only local execution authority.
- Controller-owned desired state is mutated only through its owner path.
- Experiment workflow cannot bypass package admission, RBAC, or governed operation checks.
- Rollback is a typed workflow, not an ad hoc cleanup script.

**Tests:**

- forced candidate health failure triggers rollback;
- workflow crash resumes safely using receipts and verification;
- rollback identity is exact;
- abort does not leave desired/installed/runtime layers divergent;
- experiment lock prevents concurrent mutation of the same service/cohort;
- expired envelope cannot execute.

**Exit criteria:** Candidate install and rollback succeed repeatedly without manual repair.

**Estimated effort:** 5-9 days.

### Phase 4: Sensei integration adapter

**Goal:** Bind Sensei admission and knowledge proposals to Globular experiment records.

**Initial integration shape:** a small adapter package or CLI command, not a new long-running service.

```text
golang/improvement/sensei/
  client.go
  envelope.go
  proposal.go
  client_test.go
cmd/globular/improvement_commands.go
```

**Commands or MCP-facing operations:**

```text
globular improvement plan validate <plan>
globular improvement run <plan>
globular improvement status <experiment-id>
globular improvement evidence <experiment-id>
globular improvement rollback <experiment-id>
globular improvement propose-learning <experiment-id>
```

**Adapter responsibilities:**

- request or load Sensei's task/change envelope;
- verify repository/base/head/diff identity;
- run preflight and final diff audit before execution;
- attach required tests and forbidden fixes to the plan;
- submit accepted/rejected evidence as a candidate lesson;
- preserve evidence digest links in the proposal;
- never promote knowledge directly.

**Failure behavior:** Sensei unavailable means no new experiment starts. An already-running Globular experiment may complete or roll back using its immutable local plan.

**Tests:**

- stale head SHA rejected;
- diff digest mismatch rejected;
- missing final audit rejected;
- one-use execution nonce enforced;
- adapter timeout does not strand a canary;
- proposal creation failure does not alter the experiment verdict;
- duplicate proposal submission is idempotent.

**Exit criteria:** One admitted no-op change completes the full handshake with stable identities.

**Estimated effort:** 4-7 days, depending on the final Sensei API surface.

### Phase 5: First bounded optimization pilot

**Goal:** Complete one real optimization from proposal through evidence and candidate learning.

**Candidate selection criteria:**

- read-only or easily reversible behavior;
- measurable under synthetic load;
- isolated to one service or client path;
- no schema migration;
- no membership, quorum, topology, PKI, RBAC, identity, bootstrap, or recovery semantics;
- exact baseline artifact available;
- rollback under five minutes;
- no change to Sensei governance code.

Examples include a bounded connection-reuse optimization or read-only metadata lookup optimization. The exact target must be chosen from measured baseline data, not from intuition alone.

**Pilot sequence:**

1. Run evidence-only baseline three times.
2. Create Sensei improvement intent.
3. Produce one bounded change.
4. Run required tests and final diff audit.
5. Build and publish exact candidate artifact through normal CI/repository paths.
6. Execute three candidate trials on the same cohort and workload.
7. Apply deterministic verdict policy.
8. Roll back after the pilot even if accepted, then verify baseline restoration.
9. Create a candidate lesson linked to all evidence receipts.
10. Human reviews whether the result is local guidance, a recommended pattern, or insufficient to promote.

**Exit criteria:**

- identities form one unbroken chain;
- no manual state repair is required;
- verdict is reproducible from stored normalized evidence;
- rollback is proven;
- candidate lesson links to evidence digests;
- no invariant is promoted automatically.

**Estimated effort:** 3-6 days after Phases 1-4.

### Phase 6: Corroboration and knowledge promotion

**Goal:** Prevent one-off experimental results from becoming brittle laws.

**Work:**

- Add evidence aggregation by candidate lesson ID.
- Require multiple independent accepted trials for supported lessons.
- Track environment, workload, service version, and hardware scope.
- Record contradictory evidence instead of overwriting prior evidence.
- Add proposal statuses such as `DRAFT`, `SUPPORTED`, `CONTESTED`, `PROMOTED`, and `REJECTED`.
- Require explicit owner and sunset/review conditions for promoted recommendations.
- Generate regression tests or proof obligations before contract promotion.

**Exit criteria:** A lesson can become contested without data loss, and promotion cannot occur without named human/governance authority.

**Estimated effort:** 5-10 days.

### Phase 7: Production hardening, later

Production use should be considered only after the development loop is stable.

Required additions include:

- signed envelopes and receipts;
- durable audit retention;
- explicit experiment budgets;
- maintenance-window integration;
- tenant and RBAC isolation;
- secret redaction;
- supply-chain provenance verification;
- production canary policies;
- cross-node comparison;
- metrics retention and clock-integrity checks;
- alerting and operator interruption;
- disaster recovery for experiment state;
- policy migration and backward compatibility;
- adversarial tests against forged evidence, replay, stale plans, and metric manipulation.

**Estimated effort:** 15-30 additional engineering days after the bounded demonstrator, depending on existing observability and signing infrastructure.

## 10. Proposed repository change map

This is a planning map, not a commitment to every path.

```text
docs/architecture/
  governed-recursive-improvement-plan.md
  improvement-records.md
  improvement-threat-model.md

golang/improvement/
  v1alpha1/
  evaluator/
  evidence/
  sensei/
  store/

golang/workflow/
  definitions/improvement-evidence.yaml
  definitions/improvement-canary.yaml
  engine/actors_improvement.go

golang/cluster_doctor/
  rules/improvement_experiment_health.go   # only if a new rule is necessary

golang/mcp/
  tools_improvement.go                     # read/status operations first

cmd/globular/
  improvement_commands.go
```

Prefer extending existing workflow storage and actor routing. A new service should be rejected unless the implementation proves that no existing owner can correctly hold the responsibility.

## 11. Storage model

The storage decision must be resolved in Phase 0, but the following rules apply:

- Authoritative experiment lifecycle and verdict records belong to a workflow-owned durable store.
- Large raw metrics remain in the metrics backend.
- Logs remain in journald or the configured log backend.
- Evidence receipts store digests and references to raw sources.
- AI memory receives an advisory summary with the experiment ID and evidence digest.
- Sensei stores architectural proposal state and links back to evidence.
- etcd should not hold large evidence bodies.
- No record should depend on an ephemeral local file as its only copy.

## 12. Security and threat model

The loop introduces a high-value attack path: forged evidence could cause unsafe code to appear proven.

Minimum threats to cover:

- stale or substituted Git commit;
- candidate artifact not built from the admitted head;
- replayed change envelope;
- forged or truncated metric windows;
- workload changed between baseline and candidate;
- candidate service reporting fake health;
- missing error samples treated as zero;
- operator approval reused across experiments;
- AI-generated proposal claiming broader scope than evidence supports;
- rollback artifact unavailable;
- evidence store unavailable after candidate deployment;
- concurrent experiments contaminating results;
- experiment modifying its own evaluator or guardrails.

Minimum controls:

- content-address every immutable record;
- one-use nonces;
- separate change, execution, observation, and promotion authorities;
- deny experiments that modify their evaluator or governing policy in the same run;
- fail closed on missing required evidence;
- preserve contradictory evidence;
- require exact rollback artifact resolution before deployment;
- log every authority transition.

## 13. Deterministic evaluator policy

The evaluator must be small, reviewable, and testable.

It should not attempt statistical sophistication in the first version. It should implement explicit threshold comparison, minimum sample checks, identity validation, and guardrail evaluation.

A later version may add confidence intervals or sequential testing, but only as a versioned evaluator policy with golden tests.

Required evaluator outputs:

```json
{
  "policy_version": "improvement-evaluator/v1",
  "result": "ACCEPTED",
  "primary_metric": {
    "baseline": 42.0,
    "candidate": 31.0,
    "unit": "ms",
    "rule": "candidate <= baseline * 0.80",
    "passed": true
  },
  "guardrails": [],
  "missing_evidence": [],
  "identity_checks": [],
  "reasons": [],
  "evidence_receipt_sha256": "..."
}
```

The complete verdict must be reproducible offline from the evidence receipt and policy version.

## 14. Test strategy

### Unit tests

- canonical serialization and digest stability;
- schema validation;
- evaluator threshold behavior;
- missing evidence behavior;
- identity mismatch behavior;
- knowledge proposal scope preservation.

### Integration tests

- workflow run to authoritative evidence receipt;
- node-agent install receipt binding;
- metrics/doctor collection;
- AI memory projection failure isolation;
- Sensei adapter timeout and replay protection;
- rollback after every terminal non-accepted state.

### Adversarial tests

- candidate modifies workload;
- candidate modifies evaluator;
- forged artifact digest;
- reused nonce;
- stale base SHA;
- missing high-error interval;
- partial metrics outage;
- clock skew;
- duplicate terminal workflow callbacks;
- conflicting evidence from node-agent and service metrics.

### Demonstration test

A scripted demonstration must produce:

1. the intent;
2. Sensei briefing and admission;
3. exact diff identity;
4. candidate artifact identity;
5. workflow run and correlation IDs;
6. install receipt digest;
7. baseline and candidate evidence;
8. deterministic verdict;
9. rollback proof;
10. draft lesson with evidence links.

## 15. Operational controls

Every experiment must have:

- an operator-visible status;
- a maximum duration;
- an explicit stop operation;
- automatic rollback conditions;
- a declared cohort;
- a concurrency lock;
- an evidence completeness indicator;
- an immutable plan digest;
- a retention policy;
- a reason when it cannot proceed.

Suggested state machine:

```text
DRAFT
  -> ADMITTED
  -> BASELINING
  -> READY
  -> DEPLOYING_CANDIDATE
  -> VERIFYING_CANDIDATE
  -> MEASURING
  -> EVALUATING
       -> ACCEPTED
       -> REJECTED
       -> INCONCLUSIVE
       -> ABORTED
  -> ROLLING_BACK
  -> BASELINE_RESTORED
  -> LEARNING_PROPOSED
  -> CLOSED
```

No state may skip directly from `DEPLOYING_CANDIDATE` to `ACCEPTED`.

## 16. Initial issue breakdown

A practical implementation can be split into these PR-sized tasks:

1. **GRI-1: Canonical improvement record types and validators**
2. **GRI-2: Workflow-owned experiment persistence**
3. **GRI-3: Evidence-only workflow and receipt generation**
4. **GRI-4: Deterministic evaluator v1**
5. **GRI-5: Canary cohort lock, abort policy, and rollback proof**
6. **GRI-6: Sensei change-envelope adapter**
7. **GRI-7: AI memory advisory projection**
8. **GRI-8: Knowledge proposal with evidence links**
9. **GRI-9: First bounded optimization pilot**
10. **GRI-10: Corroboration and contested-evidence model**

Each issue must state:

- owner and authority boundary;
- invariants preserved;
- exact records introduced or changed;
- tests required;
- failure and rollback behavior;
- closure condition;
- what remains intentionally manual.

## 17. Success criteria

### Bounded demonstrator success

The first milestone is successful when:

- one real optimization is admitted by Sensei;
- Globular executes it only on the declared development cohort;
- the exact change, artifact, workflow, install, workload, and evidence identities are linked;
- the evaluator returns a reproducible verdict;
- failure or uncertainty triggers rollback;
- baseline restoration is verified;
- a draft lesson is produced with evidence links;
- human approval remains required for knowledge promotion;
- no manual repair is needed.

### Production-readiness criteria

The loop is not production-ready until:

- receipts and envelopes are authenticated;
- evidence persistence survives component outages;
- rollback is proven across supported package types;
- observability sources are sufficiently independent;
- concurrent experiment contamination is prevented;
- adversarial evidence tests pass;
- operator stop and audit paths are documented;
- policy upgrades are versioned and backward compatible.

## 18. Realistic effort

Assuming one experienced engineer working with coding agents and reusing existing workflow, node-agent, doctor, AI-memory, and awareness infrastructure:

- **Architecture closure and evidence-only prototype:** roughly 9-16 engineering days.
- **Canary execution, rollback, and Sensei handshake:** roughly 9-16 additional days.
- **First credible end-to-end pilot:** roughly 3-6 additional days.
- **Bounded demonstrator total:** approximately 21-38 engineering days.
- **Production hardening:** an additional 15-30 days, potentially more if metrics, signing, or workflow persistence require redesign.

These are implementation-effort ranges, not calendar promises. The largest uncertainty is not code generation. It is the quality and authority of evidence available from a real cluster.

## 19. Recommended first decision

Do not begin with autonomous optimization.

Begin by implementing **Phase 1 and Phase 2** and prove that Globular can generate a complete, deterministic, replay-resistant evidence receipt for an unchanged artifact.

When the measuring instrument can distinguish valid, invalid, and incomplete evidence without guesswork, add candidate deployment and rollback. Only then connect accepted evidence to Sensei's learning proposal path.

That sequence turns the idea from a dramatic loop diagram into an engineering system whose every arrow has a receipt.