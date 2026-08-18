# Autonomous Repair, Simulation, and Evolution Strategy

> Status: **architectural strategy**. This document defines the target operating model for safe autonomous repair and evolution of Globular. Implementation is incremental, but new automation should converge on this model rather than inventing parallel repair paths.
>
> Scope: `globulario/services`, `globulario/globular-quickstart`, Sensei, AI Memory / Behavioral Memory, AI Executor, Claude Code / sensei-code, release workflows, and future digital-twin infrastructure.

## 1. Decision

Globular must evolve through a **simulation-first, contract-governed engineering loop**.

The live cluster may autonomously execute **known, pre-authorized operational remedies**. It must not become an improvised coding environment for novel repairs. Novel repair, feature implementation, and architectural evolution happen against an isolated, reproducible model of the system, are exercised by scenarios, are admitted against contracts and invariants, and reach production only through the normal governed release path.

The central law is:

> **Novel repair and evolution must be proven against a reproducible model of the system before they may alter the authoritative system.**

A production incident may authorize containment such as fencing, stopping a dangerous workflow, or restarting a service when that remedy is already governed. It does not authorize an agent to edit binaries, source, configuration files, desired state, or architectural contracts directly on a live node.

This gives Globular two complementary forms of self-healing:

1. **Operational self-healing** restores a known contract with deterministic, typed remedies.
2. **Engineering self-healing** changes the implementation when the existing implementation cannot satisfy the contract, but does so in an isolated engineering environment before any release is promoted.

The second form is where Sensei, simulation, behavioral memory, and coding agents become one system rather than separate tools.

---

## 2. Goal

The goal is not merely to make failures recover automatically. The goal is to let Globular safely answer four questions:

1. **What happened?**
2. **What contract or invariant was threatened or violated?**
3. **Can the current system repair it using an already-governed remedy?**
4. **If not, can an autonomous engineering agent reproduce the problem, create or extend scenarios, implement a candidate change, prove it, learn from the result, and prepare it for governed promotion without risking the live cluster?**

The same pipeline must also handle planned features. A feature is not a separate safety model. It is an intentional change to reachable system behavior and must therefore pass through the same contracts, simulation, scenario, proof, learning, and release boundaries as a repair.

---

## 3. Existing models this strategy composes

This strategy does not replace existing Globular governance. It composes the models that already exist:

- The **4-layer state model** remains authoritative: Repository → Desired → Installed → Runtime.
- Every meaningful production mutation still flows through the **Workflow Service**.
- Agent work remains **contract-first**: identify governing contract, invariants, failure modes, forbidden fixes, and verification before editing.
- Sensei remains the architectural memory and admission surface for architecture-sensitive changes.
- AI Executor remains the runtime diagnosis/remediation actor for typed, governed operational actions.
- AI Memory / Behavioral Memory remains the evidence and learning surface.
- Behavioral principles remain governed: simulation evidence may create candidates, but **candidates do not auto-promote into production authority**.
- Release artifacts remain immutable outputs of a repository revision and release process. Production nodes do not become source-editing workspaces.

The missing piece has been the lifecycle connecting all of them. This document defines that lifecycle.

---

## 4. System roles

### Globular production cluster

The production cluster is the authoritative runtime system. It provides observations, incidents, receipts, topology, generations, health, workflow history, and outcomes. It executes only governed runtime mutations.

Production is **not** the place where a novel code repair is invented or tested.

### Cluster Doctor / AI Watcher

Detects anomalies, drift, invariant-relevant symptoms, and changes in operating posture. It can initiate investigation and evidence capture.

### AI Executor

Handles known operational remediation through typed actions and workflows. It may autonomously apply remedies whose authority, risk, and verification contract are already defined.

### Sensei

Provides architectural context, contracts, invariants, forbidden repairs, ownership, impact, proof obligations, admission, and durable architecture learning.

Sensei answers: **what must remain true, what is allowed to change, and what proof is required?**

### Claude Code / sensei-code / engineering agent

Operates in an isolated repository and simulation environment. It may inspect code, implement candidate changes, add tests and scenarios, run builds, run simulation, analyze counterexamples, and prepare a governed change.

It does not gain production root authority merely because it can reason about the cluster.

### globular-quickstart simulation harness

Provides the executable clustered model for deterministic scenarios, fault injection, recovery checks, upgrade checks, state-transition tests, adversarial exploration, and durable evidence.

The simulation harness is part of the **proof boundary**, not only a developer convenience.

### AI Memory / Behavioral Memory

Stores observations, claims, evidence, outcomes, contradictions, conditions, and learned candidate principles. It supplies promoted behavioral knowledge back to runtime agents.

Raw simulation observations are learning input, not production authority.

### Workflow / release system

Is the only path by which an accepted implementation becomes a production deployment. A coding agent may prepare a release candidate; it does not silently replace the running system.

---

## 5. Two repair lanes

### Lane A: known operational remediation

Use when the failure and remedy are already governed.

Examples:

- service crash → restart through the approved supervisor/workflow path;
- isolated node → fence it;
- desired/installed drift → reconcile;
- known certificate renewal condition → rotate through the approved workflow;
- known transient dependency failure → retry according to a bounded policy.

Lifecycle:

```text
OBSERVE
  → MATCH KNOWN CONDITION
  → CHECK BEHAVIORAL/SENSEI POLICY
  → EXECUTE TYPED WORKFLOW
  → VERIFY CONVERGENCE
  → RECORD OUTCOME
```

No source modification is involved.

### Lane B: novel engineering repair

Use when no governed runtime remedy exists, the remedy fails repeatedly, the implementation violates a contract, or fixing the problem requires code / schema / workflow / architecture changes.

Lifecycle:

```text
OBSERVE INCIDENT
  → CONTAIN IF NECESSARY
  → CAPTURE REPRODUCTION EVIDENCE
  → DISCOVER GOVERNING CONTRACT
  → REPRODUCE IN SIMULATION
  → CREATE/MINIMIZE SCENARIO
  → IMPLEMENT CANDIDATE IN ISOLATED CHECKOUT
  → RUN STATIC + UNIT + INTEGRATION PROOF
  → RUN REGRESSION + ADVERSARIAL SCENARIOS
  → SENSEI ADMISSION
  → RELEASE CANDIDATE
  → CONTROLLED PRODUCTION ROLLOUT
  → VERIFY
  → LEARN
```

The key property is that the agent repairs the **model and implementation of future production**, not the running production filesystem.

---

## 6. The unified change lifecycle

Every non-trivial repair, feature, or architectural evolution should eventually be representable by the following states.

### E0 — Trigger

A change starts from one of four sources:

- `INCIDENT`: production exposed a failure;
- `SIMULATION_COUNTEREXAMPLE`: simulation found a reachable failure before production did;
- `FEATURE_INTENT`: a human or governed planner requests new behavior;
- `ARCHITECTURE_EVOLUTION`: a contract, authority model, or system boundary must intentionally change.

### E1 — Change envelope

Create a durable description of the work before implementation begins.

Minimum conceptual shape:

```yaml
change_id: change-...
kind: incident_repair | simulation_repair | feature | architecture_evolution
intent: "..."
source_revision: "..."
production_revision: "..."       # when relevant
risk_class: low | medium | high | critical
authority_scope: ["..."]
governing_contracts: ["..."]
relevant_invariants: ["..."]
known_failure_modes: ["..."]
forbidden_repairs: ["..."]
production_evidence: ["..."]
required_scenarios: ["..."]
required_tests: ["..."]
admission_status: pending
promotion_status: candidate
```

This is intentionally close to Sensei's bounded-change/admission model. It should eventually become a machine-readable artifact rather than living only in an agent conversation.

### E2 — Contract and authority discovery

Before mutation, determine:

- what behavior is being changed;
- which state layer owns the truth;
- which actor may mutate it;
- which contracts and invariants define correctness;
- which failure modes and forbidden fixes are already known;
- what evidence is required to claim success.

If architectural authority is unknown, the agent may continue investigation but must not claim an architecture-sensitive repair is admitted.

### E3 — Reproducible state

For incident work, capture enough of the incident to reproduce the relevant state without cloning secrets or turning production into the test environment.

A future incident bundle should contain, as applicable:

```text
incident-<id>/
  manifest.yaml
  repository-revisions.json
  topology.json
  desired-state.json
  installed-state/
  runtime-state/
  workflow-history.json
  lease-and-generation-history.json
  package-manifests/
  doctor-findings.json
  relevant-logs/
  metrics/
  behavioral-context.json
  sensei-context.json
  redaction-report.json
```

The bundle is evidence, not authority. It should identify freshness and provenance for every captured source.

### E4 — Scenario reproduction

A novel failure is not considered understood merely because an agent can explain it in prose. Produce an executable scenario that fails for the right reason.

If exact production reproduction is impossible, produce the smallest scenario that exercises the same governing contract and explicitly record the fidelity gap.

### E5 — Candidate implementation

The engineering agent works in an isolated checkout at a known revision and has access to:

- source code and dependencies;
- build/test environment;
- Sensei graph/context;
- incident or feature change envelope;
- simulation harness;
- scenario corpus;
- relevant behavioral-memory evidence;
- release tooling without direct production mutation authority.

The agent may modify code, tests, docs, scenarios, migrations, and contracts within its granted engineering scope.

### E6 — Local proof

Run the cheapest deterministic proof first:

1. static/build/vet/lint gates;
2. unit tests;
3. service integration tests;
4. contract-specific tests;
5. package/install tests where relevant.

A green local test is necessary but not sufficient for a distributed-system change.

### E7 — Cluster simulation proof

Run the change in a clustered simulation.

At minimum:

- the newly created regression scenario must pass;
- directly affected existing scenarios must pass;
- suite-level invariants must remain green;
- cleanup must prove the harness restored the environment;
- unsupported/skipped mutation actions must **not** count as PASS;
- evidence must identify the exact code/release revision under test.

High-risk changes should additionally run bounded adversarial exploration around the changed state transitions.

### E8 — Counterexample search

For important authority, generation, workflow, recovery, security, and upgrade changes, do not only replay expected scenarios. Ask the simulator to search for reachable invariant violations through combinations such as:

- pause/resume rather than only process death;
- asymmetric partitions;
- delay, loss, duplication, and reordering;
- stale node resurrection;
- duplicate workflow delivery;
- concurrent mutations;
- storage exhaustion at mutation boundaries;
- crash/restart at durable transition points;
- clock skew;
- old authority returning after a newer generation exists.

The target is not random chaos. It is **semantic adversarial exploration** guided toward states where authority, evidence, generation, or completion may become ambiguous.

A discovered counterexample becomes a first-class scenario candidate.

### E9 — Admission

Sensei evaluates the exact candidate change against the governing architecture and its proof bundle.

Possible results:

- `ACCEPT`: required proof is present and the change respects the current contract;
- `WAITING/BLOCKED`: proof or architectural closure is missing;
- `ARCHITECT_AUTHORITY_REQUIRED`: the proposed repair changes the meaning of the system rather than merely restoring it.

The agent may iterate autonomously while the work remains within the existing contract. It must escalate when a human architectural choice is genuinely required.

### E10 — Release candidate and rollout

An admitted change becomes a normal immutable release candidate. Production rollout remains governed by release/workflow authority.

No special "AI hot patch" path is introduced.

### E11 — Production verification

After rollout:

- re-observe all four state layers;
- verify the original contract and expected outcome;
- compare production behavior with simulation expectations;
- record unexpected differences as fidelity findings;
- retain rollback as an explicit governed operation, never an invisible agent reflex.

### E12 — Learning

Every completed change and every failed candidate produces learning material:

```text
observation
  → claim
  → linked evidence
  → authority mapping
  → condition scope
  → contradiction testing
  → candidate principle/failure mode
  → governed promotion
```

This is where simulation becomes useful to production without letting simulation invent production truth.

---

## 7. Scenario contract

Scenarios should become first-class engineering artifacts rather than shell scripts that happen to fail things.

A scenario should eventually identify:

```yaml
scenario_id: "..."
name: "..."
origin: incident | feature | architecture | adversarial_discovery
source_change_id: "..."
source_incident_id: "..."
governing_contracts: ["..."]
invariants: ["..."]
initial_state: "..."
actions: []
faults: []
expected_properties: []
forbidden_outcomes: []
evidence_required: []
cleanup_contract: "..."
reproducibility:
  seed: "..."
  simulator_revision: "..."
  cluster_profile: "..."
learning:
  expected_failure_modes: []
  candidate_principles_allowed: true
```

The scenario's oracle should prefer architectural properties over incidental implementation details.

Bad oracle:

> controller log contains exactly line X.

Better oracle:

> after lease generation 42 is established, an actor holding generation 41 performs no externally visible mutation.

Logs may be evidence for that property, but they are not the property itself.

---

## 8. Scenario creation and learning loop

Agents are expected to create scenarios as part of autonomous work.

### From production failure

```text
production incident
  → capture evidence
  → reproduce
  → minimize
  → scenario becomes regression artifact
  → fix
  → scenario proves repair
```

### From feature work

```text
feature intent
  → identify new reachable behavior
  → define success scenarios
  → define failure/rollback/interruption scenarios
  → implement
  → prove new behavior and old invariants together
```

### From adversarial exploration

```text
simulator explores states
  → invariant violation found
  → deterministic seed/event trace retained
  → minimize trace
  → promote trace to named scenario
  → diagnose
  → repair or architecture decision
```

### From a successful scenario

A PASS can still teach. The learner may identify evidence such as:

- a previously undocumented dependency;
- an authority relationship;
- a timing boundary;
- a necessary negative control;
- an invariant that held across many independent executions;
- a forbidden repair that would have hidden the failure rather than fixed it.

These become **candidate knowledge**, not immediately enforced rules.

---

## 9. Behavioral memory promotion boundary

Simulation should continuously enrich Behavioral Memory, but it must do so safely.

### What simulation may write autonomously

- raw signals;
- observations;
- scenario outcomes;
- evidence links;
- contradiction records;
- candidate failure modes;
- candidate conditions;
- candidate principles;
- confidence and provenance;
- links between a principle and the scenarios that support or contradict it.

### What simulation must not auto-promote

- a new production-enforced principle;
- a new destructive operational action;
- a new ownership/authority rule;
- a contract change;
- a permission escalation;
- a repair that changes desired state semantics;
- an architectural assumption inferred only from repeated success.

Promotion remains governed. This preserves the existing rule that evidence can accumulate autonomously while authority is earned rather than assumed.

### Production consumption

Production agents may consume:

- promoted behavioral principles;
- governed known-remedy mappings;
- verified failure modes;
- condition-scoped evidence requirements;
- provenance-backed outcomes from previous releases.

Production must distinguish them from unpromoted simulation candidates.

In short:

```text
SIMULATION MAY LEARN FREELY
        ↓
CANDIDATE KNOWLEDGE
        ↓ governance / contradiction / promotion
PROMOTED BEHAVIORAL KNOWLEDGE
        ↓
PRODUCTION MAY RELY ON IT
```

---

## 10. Agent autonomy levels

Autonomy should be granted by action class rather than by giving an agent a vague "full access" flag.

| Level | Capability | Default authority |
|---|---|---|
| A0 | Observe production / query evidence | autonomous within RBAC |
| A1 | Capture incident/change envelope | autonomous |
| A2 | Run simulation / reproduce / minimize | autonomous |
| A3 | Create or modify scenarios/tests | autonomous in isolated workspace |
| A4 | Modify implementation in isolated checkout | autonomous within admitted scope |
| A5 | Run builds, tests, simulation, adversarial exploration | autonomous |
| A6 | Create candidate behavioral knowledge / failure modes | autonomous, unpromoted |
| A7 | Prepare governed release candidate | autonomous when admission allows |
| A8 | Promote/deploy or alter foundational contract/authority | policy/human architectural authority |

The exact boundary may become policy-driven, but one rule is fixed: **engineering autonomy is not production mutation authority**.

---

## 11. Fail-closed rules for autonomous engineering

An agent must not claim a repair/evolution cycle succeeded when any of the following is true:

1. Governing contract is unknown for an architecture-sensitive behavior change.
2. Required simulation action was skipped or unsupported.
3. A scenario passed because the harness failed to inject its fault.
4. Scenario cleanup cannot prove residue was removed.
5. The tested revision differs from the candidate revision.
6. The reproduced failure does not exercise the same contract as the reported incident and the fidelity gap is hidden.
7. A workflow reports success but installed/runtime evidence does not prove the effect.
8. The only repair requires direct mutation of production-owned state outside its owner path.
9. A candidate principle is treated as production policy before promotion.
10. Simulation and production disagree materially and the disagreement is waved away as noise.
11. Evidence provenance or freshness is unknown.
12. An old authority/generation can still mutate after a new authority/generation is established.

A certification system in which `SKIPPED == PASS` is unsafe. Unsupported capability must be explicit and must block any gate that depends on it.

---

## 12. Feature evolution uses the same pipeline

New features must not bypass simulation merely because there is no incident.

For feature work, the engineering agent should:

1. capture intent and governing contracts;
2. define new expected behavior;
3. identify authority/state-layer impact;
4. author success scenarios before or alongside implementation;
5. author interruption, recovery, upgrade, rollback, stale-state, and concurrency scenarios proportional to risk;
6. implement in isolation;
7. prove old invariants and new behavior together;
8. run Sensei admission;
9. release normally;
10. verify production and record outcomes;
11. feed observed behavior back into simulation and behavioral memory.

Thus **repair and feature development become two entrances into one evolution machine**.

---

## 13. Digital twin direction

The long-term target is not only a fixed quickstart suite. It is a bounded digital twin of the current production architecture.

A production snapshot can seed an isolated model:

```text
CURRENT PRODUCTION STATE S
        ↓ sanitized snapshot
SIMULATION STATE S'
        ↓
explore plausible futures:
  node loss
  leader pause
  partition
  concurrent upgrade
  storage pressure
  key rotation
  stale-node return
        ↓
COUNTEREXAMPLE OR CONFIDENCE EVIDENCE
```

The most valuable output is not "production is healthy now." It is:

> **production is healthy now, and from its current posture these plausible next states were explored; these counterexamples were found or these invariants remained intact.**

This allows proactive repair before a reachable defect happens in production.

The twin must never become an untracked shadow authority. It is a prediction and proof environment whose outputs enter the same candidate→governance pipeline as every other observation.

---

## 14. Semantic adversarial exploration

Random chaos is useful but insufficient. The state space is too large.

Sensei should eventually guide exploration toward states with architectural tension, for example:

- two actors possess evidence that could justify mutation;
- a node is multiple generations stale and a rollback occurred during absence;
- acknowledgement exists without terminal effect evidence;
- an authority lease expired while the old process remained alive;
- a manifest is cluster-wide while its blob exists only on one node;
- a topology change intersects with an upgrade or key rotation;
- an observation source is available but stale;
- absence can be confused with destructive intent.

This turns chaos testing into **counterexample search against architectural laws**.

Every valuable counterexample should be reproducible by seed/event trace and then minimized into a durable scenario.

---

## 15. Mutation-boundary failpoints

For important workflows, simulation should support deterministic failpoints around durable transitions.

Example package/release transition:

```text
publish artifact
  ↓ FP-A
write desired generation
  ↓ FP-B
dispatch workflow
  ↓ FP-C
download package
  ↓ FP-D
install package
  ↓ FP-E
start runtime
  ↓ FP-F
write installed receipt
  ↓ FP-G
declare convergence
```

For each failpoint, kill/pause/restart the responsible actor and prove one of two outcomes:

1. the operation did not become authoritative; or
2. it eventually converged exactly once.

There should be no durable third state in which intent, acknowledgement, installed state, and runtime disagree indefinitely while the system calls the operation successful.

---

## 16. Production-to-simulation and simulation-to-production boundaries

### Production → simulation

Allowed data flow:

- sanitized state snapshots;
- topology and generations;
- workflow history;
- receipts and manifests;
- logs and metrics;
- incident evidence;
- promoted behavioral context;
- exact repository/release identities.

Secrets must be removed or replaced with references.

### Simulation → production

Only governed artifacts cross back:

- immutable code/release artifacts through the release pipeline;
- approved scenarios/tests as repository artifacts;
- promoted behavioral knowledge through Behavioral Memory governance;
- approved contracts/invariants through Sensei governance;
- operator-visible recommendations and risk findings.

Raw simulator state never writes production desired state.

---

## 17. Background learning model

Simulation and learning workers may run continuously or periodically without blocking production operation.

They may:

- replay known scenarios against new revisions;
- explore bounded adversarial sequences;
- compare simulator and production outcomes;
- mine repeated outcomes for candidate failure modes/principles;
- strengthen scenario coverage;
- prepare repair/evolution candidates.

The safety property is that **background learning is speculative until promoted**.

A production agent may use unpromoted material as a hypothesis for investigation, but not as authority for a destructive or architecture-changing action.

This is how Behavioral Memory can evolve in the background without allowing an LLM's latest inference to become cluster law.

---

## 18. Required implementation work

### Phase 0 — codify the strategy

- Adopt this document as the canonical repair/evolution model.
- Teach Claude/Codex skills to route repair, feature, and simulation work through it.

### Phase 1 — make simulation a trustworthy gate

In `globular-quickstart`:

- make unsupported/skipped required actions fail closed;
- give every scenario exact revision/provenance evidence;
- make cleanup/restoration an asserted contract;
- add pause/resume, scoped partition, latency/loss, resource pressure, and deterministic failpoint primitives;
- formalize the scenario metadata contract.

### Phase 2 — change/incident envelopes

- define a machine-readable `ChangeEnvelope` / incident bundle schema;
- connect production evidence capture to simulation inputs;
- record source and candidate revisions explicitly.

### Phase 3 — agent orchestration

- let Claude/sensei-code take a change envelope and drive contract discovery → reproduction → scenario → implementation → proof → admission;
- make stop/escalation conditions mechanical where possible;
- preserve the human architectural authority boundary.

### Phase 4 — learning bridge

- store scenario outcomes as Behavioral Memory evidence;
- create candidate failure modes and principles with provenance;
- wire contradiction checks and promotion gates;
- make promoted knowledge available to production agents.

### Phase 5 — adversarial state exploration

- deterministic event seeds;
- state snapshots;
- bounded search;
- invariant oracle integration with Sensei;
- automatic counterexample minimization into scenario candidates.

### Phase 6 — production digital twin

- seed simulation from sanitized live posture;
- continuously compare model predictions to production outcomes;
- use fidelity differences as defects in the simulation model;
- proactively search the neighborhood of the current production state.

---

## 19. Definition of success

This strategy is implemented when all of the following are true:

1. A production incident can create a reproducible engineering task without giving the coding agent live mutation authority.
2. An agent can identify the governing contract and produce a scenario that demonstrates the defect.
3. The agent can autonomously implement and validate a candidate within an isolated environment.
4. Required scenario injections cannot silently skip and still produce PASS.
5. High-risk changes are tested against adversarial state transitions, not only happy paths.
6. Sensei can admit or block the exact candidate based on contracts and proof.
7. Accepted code reaches production only through the standard immutable release/workflow path.
8. Production verifies the effect through independent observed state, not workflow acknowledgement alone.
9. Simulation outcomes feed Behavioral Memory as evidence and candidate knowledge.
10. Unpromoted learning cannot become production authority.
11. Promoted behavioral knowledge survives/reconciles across rebuilds and remains traceable to evidence.
12. Counterexamples found before production can trigger the same repair lifecycle as real incidents.
13. Feature implementation uses the same safety pipeline as repair.

At that point Globular will not merely recover from known failures. It will have a governed mechanism for **learning how it can fail, changing itself in isolation, proving the change, and safely carrying the lesson into future production behavior**.

---

## 20. Agent algorithm

When an AI agent receives a repair, feature, or evolution task, use this compact algorithm:

```text
1. CLASSIFY
   incident repair | simulation repair | feature | architecture evolution

2. GROUND
   identify state layers, authority, contracts, invariants, failure modes, forbidden fixes

3. ENVELOPE
   bind exact source revision, evidence, risk, required tests/scenarios

4. REPRODUCE / SPECIFY
   incident → reproduce and minimize
   feature  → specify success + interruption/failure scenarios

5. CHANGE IN ISOLATION
   never invent novel repair directly in production

6. PROVE LOCALLY
   build + static + unit + integration

7. PROVE AS A CLUSTER
   regression scenario + affected suites + restoration proof

8. SEARCH FOR COUNTEREXAMPLES
   proportional adversarial exploration for high-risk state transitions

9. ADMIT
   Sensei verifies contract respect and proof; escalate true architecture choices

10. RELEASE NORMALLY
    immutable artifact + governed workflow, no AI hot-patch channel

11. VERIFY PRODUCTION
    observe independent state layers and expected outcome

12. LEARN
    record outcomes → candidate failure modes/principles → governed promotion
```

If any required proof is skipped, unsupported, stale, or ambiguous, the correct result is **not proven**, not PASS.
