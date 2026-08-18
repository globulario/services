---
name: globular-repair-evolution
description: Use for Globular incident repair, simulation-discovered defects, feature implementation, architecture evolution, cluster-simulation work, scenario creation, adversarial testing, autonomous engineering, or behavioral learning from test/simulation outcomes. Routes work through the simulation-first governed repair/evolution lifecycle so agents can work autonomously without gaining unsafe production mutation authority.
---

# Globular Repair and Evolution

Use this skill whenever the task changes distributed-system behavior or learns from cluster execution.

Canonical architecture: `docs/design/autonomous-repair-evolution-strategy.md`.

## Foundational law

> Novel repair and evolution must be proven against a reproducible model of the system before they may alter the authoritative system.

Known typed operational remedies may execute through governed production workflows. Novel code/schema/workflow/architecture repair happens in an isolated engineering environment first. Never create an AI hot-patch path into production.

## Classify the task

Choose one entrance:

- `incident_repair`: production exposed a failure.
- `simulation_repair`: an adversarial/scenario run found a counterexample.
- `feature`: intentional new reachable behavior.
- `architecture_evolution`: contract, authority, ownership, lifecycle, or system meaning changes.

All four converge on the same lifecycle:

```text
CLASSIFY
  -> GROUND CONTRACT + AUTHORITY
  -> CHANGE ENVELOPE
  -> ISOLATED IMPLEMENTATION
  -> DECLARED LOCAL TESTS
  -> BOUND CLUSTER SCENARIOS
  -> PROVEN
  -> SENSEI ADMISSION
  -> IMMUTABLE RELEASE
  -> PRODUCTION VERIFICATION
  -> BEHAVIORAL LEARNING
```

## Executable pre-admission workflow

### 1. Ground contract and authority

Use `sensei-architect` before architecture-sensitive work. Identify:

- Repository / Desired / Installed / Runtime state layers touched;
- authoritative owner and allowed mutation path;
- governing contracts/invariants;
- known failure modes;
- forbidden repairs;
- required tests and scenarios.

For an exact proposed mutation, use `sensei-admission` at the execution-control boundary. If the contract/authority is unknown, investigate but do not claim admission.

### 2. Create the ChangeEnvelope

The machine-readable envelope is implemented in `golang/ai_memory/evolution`.

Example:

```bash
evolution-change init \
  --out /tmp/change.yaml \
  --kind simulation_repair \
  --intent "resumed stale controller cannot mutate authoritative desired state" \
  --source-revision <baseline-sha> \
  --risk critical
```

Then author/freeze in the envelope:

- `authority_scope`
- `governing_contracts`
- `relevant_invariants`
- `known_failure_modes`
- `forbidden_repairs`
- `required_tests`, each with the exact command
- `required_scenarios`, each with the exact quickstart path

Do not treat changing proof requirements as editorial cleanup. Required tests/scenarios are part of the envelope identity.

### 3. Bind the exact implementation candidate

```bash
evolution-change bind-candidate \
  --envelope /tmp/change.yaml \
  --repository globulario/services \
  --revision <candidate-git-sha>
```

Rebinding a different pre-admission candidate clears prior candidate-derived test/proof evidence. After `ADMITTED`, candidate evidence is immutable history; create a new change/candidate instead of rewriting it.

### 4. Implement only in isolation

The coding agent may edit source, tests, scenarios, migrations, docs, and candidate contracts in isolated checkouts.

It must not improvise novel repair by modifying live binaries/files, raw-writing Desired State, or bypassing owner-owned mutation paths.

Containment is different from repair. Pre-authorized fencing/isolation/restart may still execute through normal production workflows.

### 5. Run the bounded proof plan

Preferred autonomous command:

```bash
evolution-prove \
  --envelope /tmp/change.yaml \
  --workspace-dir /path/to/services-candidate \
  --quickstart-dir /path/to/globular-quickstart
```

The runner is intentionally ordered:

1. rerun every required local/static test;
2. verify candidate checkout `git HEAD` equals the envelope candidate revision;
3. execute each exact command frozen in `TestRequirement`;
4. capture/hash stdout+stderr evidence;
5. stop immediately on local failure;
6. only after local closure, run required quickstart scenarios;
7. bind quickstart proof to change id + exact candidate revision + simulation revision;
8. stop on first non-proof cluster result;
9. reconcile `CANDIDATE <-> PROVEN` from current evidence.

Individual tools also exist:

```text
evolution-run-test
  execute one declared exact-command local test

evolution-run-scenario
  execute one governed quickstart scenario

simulation-learning-ingest
  validate/ingest one quickstart learning.json

evolution-change status
  show missing/failed test/scenario closure and next authority boundary

evolution-change validate
  validate envelope + print identity digest
```

There is deliberately no `mark-test-pass`, `admit`, `release`, or `promote` convenience command.

## Proof rules

A change reaches `PROVEN` only when:

- every required local test has PASS evidence for the exact candidate revision;
- every PASS record used the exact command declared in the envelope;
- every required cluster scenario is supported and proof-eligible PASS;
- scenario proof identifies the same candidate repository/revision;
- quickstart `simulation_revision` matches the scenario proof source revision.

A failed rerun can downgrade `PROVEN -> CANDIDATE`. Proof is not sticky green.

Required unsupported/skipped scenario actions cannot count as PASS.

## Quickstart simulation boundary

Companion repository: `globulario/globular-quickstart`.

Trusted runs produce:

- `evidence.json`
- `scenario-proof.json`
- `learning.json`

When governed, quickstart requires:

```text
GLOBULAR_CHANGE_ID
GLOBULAR_CHANGE_ENVELOPE_REF
GLOBULAR_CANDIDATE_REPOSITORY
GLOBULAR_CANDIDATE_REVISION
GLOBULAR_REQUIRE_CHANGE_BINDING=1
```

A partial/invalid binding blocks lab mutation. A green legacy executor cannot override broken proof identity.

Current semantic temporal primitives include:

- `chaos.pause_service` (`SIGSTOP`)
- `chaos.resume_service` (`SIGCONT`)

Add new chaos capabilities through the semantic layer and register them in the proof contract. Do not silently teach the proof validator an action the executor cannot perform.

## Counterexample search

For high-risk authority/generation/workflow/recovery/upgrade/security/identity/storage changes, explore semantic neighboring states such as:

- zombie/old authority returning after lease/generation loss;
- asymmetric partition;
- stale node after multiple generations/rollback;
- duplicate delivery or stale workflow completion;
- crash at durable mutation boundaries;
- storage exhaustion during publication/commit;
- concurrent upgrades/mutations;
- clock skew;
- valid manifest with unavailable/corrupt artifact;
- absence mistaken for destructive intent.

Retain deterministic seeds/event traces and minimize valuable counterexamples into named scenarios.

Do not claim `controller-zombie-after-lease-loss` proven merely because SIGSTOP/SIGCONT exists. The scenario additionally needs a real owner-path mutation attempt whose generation/fencing rejection is independently observable.

## Hard stop at PROVEN

`evolution-prove` may reach **PROVEN**. It may not cross into **ADMITTED**.

After proof:

1. use Sensei admission for the exact candidate revision and exact proof set;
2. only a real Sensei `ACCEPT` may advance the envelope to ADMITTED;
3. release through the normal immutable release/workflow path;
4. independently verify production Repository -> Desired -> Installed -> Runtime evidence;
5. learn outcomes into Behavioral Memory.

Do not add a local CLI flag that self-asserts Sensei acceptance. Sensei is the admission authority.

## Behavioral learning boundary

Quickstart learning is validated before ingestion and must remain:

```text
production_authoritative = false
promotion_required       = true
may_promote              = false
```

The simulation ingestion interface exposes only `RecordSignal`, `RecordEvidence`, and `RecordOutcome`. It has no `PromotePrinciple` capability.

Simulation may generate evidence and candidate hints. Promotion remains governed.

## Stop conditions

Do not claim success when:

- governing contract/authority is unknown for architecture-sensitive behavior;
- required local test is absent, substituted, stale, or failing;
- tested checkout revision differs from the candidate revision;
- required scenario action/probe is unsupported;
- injected failure did not occur;
- cleanup residue is unproven;
- workflow success lacks independent installed/runtime proof;
- only available repair bypasses the owner path;
- candidate behavioral knowledge is being treated as promoted policy;
- production/simulation discrepancy is unexplained;
- evidence freshness/provenance is unknown.

Use `NOT PROVEN`, `BLOCKED`, or an architecture question instead of optimistic PASS.

## Relationship to other skills

- `sensei-architect`: architecture discovery, contract/authority challenge, degraded awareness handling.
- `sensei-admission`: exact proposed change and mutation-admission decision.
- `sensei-closure`: admission waiting/refused because architecture or proof is incomplete.
- `sensei-benchmark`: independent historical/external proof when required.

This skill governs the engineering lifecycle around those Sensei decisions.
