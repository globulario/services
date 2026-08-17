---
name: globular-repair-evolution
description: Use for Globular incident repair, simulation-discovered defects, feature implementation, architecture evolution, cluster-simulation work, scenario creation, adversarial testing, autonomous engineering, or behavioral learning from test/simulation outcomes. Routes work through the simulation-first governed repair/evolution lifecycle so agents can work autonomously without gaining unsafe production mutation authority.
---

# Globular Repair and Evolution

Use this skill whenever the task changes distributed-system behavior or learns from cluster execution.

The canonical architecture is:

- `docs/design/autonomous-repair-evolution-strategy.md`

Read that document before planning substantial repair/evolution work. This skill is a router and operational summary, not a competing source of truth.

## Foundational law

> Novel repair and evolution must be proven against a reproducible model of the system before they may alter the authoritative system.

Known, typed operational remedies may execute through governed workflows. Novel code/schema/workflow/architecture repair happens in an isolated engineering environment first.

Never create an AI hot-patch path into production.

## Classify the task

Choose one entrance:

```text
INCIDENT_REPAIR
  production exposed a failure

SIMULATION_REPAIR
  a scenario/adversarial run found a counterexample

FEATURE
  intentional new reachable behavior

ARCHITECTURE_EVOLUTION
  contract, authority, ownership, lifecycle, or system meaning changes
```

All four converge on the same proof/promotion lifecycle.

## Required loop

```text
CLASSIFY
  → GROUND CONTRACT + AUTHORITY
  → CREATE CHANGE ENVELOPE
  → REPRODUCE OR SPECIFY SCENARIOS
  → CHANGE IN ISOLATION
  → LOCAL PROOF
  → CLUSTER SIMULATION PROOF
  → COUNTEREXAMPLE SEARCH WHEN RISK WARRANTS
  → SENSEI ADMISSION
  → NORMAL IMMUTABLE RELEASE
  → PRODUCTION VERIFICATION
  → BEHAVIORAL LEARNING
```

### 1. Ground before mutation

Use `sensei-architect` for architecture-sensitive discovery and challenge.

Identify:

- state layer(s): Repository / Desired / Installed / Runtime;
- authority owner and allowed mutation path;
- governing contracts and invariants;
- known failure modes;
- forbidden repairs;
- required proof.

For an exact proposed mutation, use `sensei-admission` at the execution boundary.

If the contract/authority is genuinely unknown, investigate but do not claim the repair is admitted.

### 2. Bind a change envelope

Bind the work to exact revisions and evidence. Conceptually track:

```yaml
change_id: "..."
kind: incident_repair | simulation_repair | feature | architecture_evolution
intent: "..."
source_revision: "..."
production_revision: "..."
risk_class: "..."
authority_scope: []
governing_contracts: []
relevant_invariants: []
known_failure_modes: []
forbidden_repairs: []
production_evidence: []
required_scenarios: []
required_tests: []
```

Until a machine-readable ChangeEnvelope exists, keep these fields explicit in the task evidence/report.

### 3. Reproduce failures; specify feature behavior

For incidents and simulation counterexamples:

- preserve the production/simulation evidence and exact revision;
- reproduce in `globulario/globular-quickstart` or the appropriate isolated cluster model;
- minimize the event trace when possible;
- create a durable regression scenario that fails for the governing contract, not merely for a log string.

For features:

- define success scenarios;
- define proportional interruption/recovery/concurrency/upgrade/rollback scenarios;
- make the new behavior and old invariants executable together.

### 4. Modify only the isolated engineering model

The coding agent may modify source, tests, scenarios, migrations, docs, and candidate contracts in the isolated checkout.

It must not improvise a novel repair by editing live node files, binaries, desired state, or owner-owned runtime state.

Containment is different from repair. A pre-authorized action may fence/isolate/stop/restart through its governed production workflow while engineering repair proceeds separately.

### 5. Prove locally, then as a cluster

Run cheap deterministic proof first, then cluster proof.

Cluster proof must include:

- new regression/success scenarios;
- directly affected existing scenarios/suites;
- exact candidate revision identity;
- cleanup/restoration proof;
- independent state-layer verification rather than workflow acknowledgement alone.

**Required scenario actions that are unsupported or skipped cannot count as PASS.** Treat them as unproven/blocking for certification.

### 6. Search for counterexamples

For high-risk authority, generation, workflow, recovery, upgrade, security, identity, or storage changes, explore adversarial neighboring states.

Prefer semantic tensions such as:

- zombie/old authority returning after lease/generation loss;
- asymmetric partition;
- stale node returning after several generations or rollback;
- duplicate delivery or stale workflow completion;
- crash at durable mutation boundaries;
- storage exhaustion during commit/publication;
- concurrent upgrades/mutations;
- clock skew;
- manifest authority without retrievable artifact;
- absence being mistaken for destructive intent.

Retain deterministic seeds/event traces. Minimize valuable counterexamples into named scenarios.

### 7. Admit and release normally

Use Sensei to verify the exact candidate against contracts and proof.

If the change restores an existing contract, iterate autonomously within the admitted scope.

If the change alters foundational meaning/authority/contract, surface the architectural choice to the human architect.

Once admitted, use the normal immutable release/workflow path. Do not create a privileged deployment path because the implementation was generated by an AI agent.

### 8. Verify production independently

After rollout, observe the relevant Repository → Desired → Installed → Runtime layers and verify the expected contract/outcome.

If production materially disagrees with simulation, record a simulation-fidelity defect. Do not erase the discrepancy as noise.

### 9. Learn without auto-authority

Feed scenario and production outcomes into Behavioral Memory as:

```text
signal → claim → evidence → authority mapping → condition
       → contradiction → candidate failure mode/principle
```

Simulation may create candidates autonomously.

Simulation must not auto-promote:

- new production-enforced principles;
- destructive remedies;
- authority/ownership changes;
- contract changes;
- permission escalation.

Production may rely on promoted behavioral knowledge; unpromoted material is hypothesis/evidence only.

## Stop conditions

Do not claim success when:

- governing contract is unknown for architecture-sensitive behavior;
- required scenario action skipped/unsupported;
- injected failure did not actually occur;
- cleanup residue is unproven;
- tested revision differs from proposed revision;
- workflow success lacks installed/runtime proof;
- only available repair bypasses the owner path;
- candidate behavioral knowledge is being treated as promoted policy;
- production/simulation discrepancy is unexplained;
- evidence freshness/provenance is unknown.

The correct result is `NOT PROVEN`, `BLOCKED`, or an architecture question, never optimistic PASS.

## Relationship to other skills

- `sensei-architect`: architecture discovery, contract/authority challenge, degraded awareness handling.
- `sensei-admission`: exact proposed change and mutation-admission decision.
- `sensei-closure`: when admission is waiting/refused because architecture or proof is incomplete.
- `sensei-benchmark`: independent historical/external proof when required.

This skill governs the **engineering lifecycle around those Sensei decisions**.
