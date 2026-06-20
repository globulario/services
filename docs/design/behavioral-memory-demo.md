# Behavioral-Memory Operator Loop — End-to-End Demo

A thin, deterministic, reproducible demonstration of the full behavioral-memory
governance loop, end to end, using the simplest real `cluster_operator` scenario:

> **No recovery claim without authoritative evidence.**

It exercises the components built across PR-1 → PR-7 with **no new architecture**:
the cluster_operator domain pack, the promotion gate, the runtime decision RPCs,
and the RDF projection. It is in-memory and needs no live cluster.

## Scenario

| | |
|---|---|
| project | `globular-services` |
| domain | `cluster_operator` |
| principle | `principle.cluster.no_recovery_claim_without_authoritative_evidence` (risk: **high**) |
| condition | `condition.cluster.service.desired_observed_mismatch` |
| forbidden move | `forbidden.cluster.claim_recovery_without_authoritative_evidence` |
| required evidence | `evidence.cluster.owner_service.desired_state`, `evidence.cluster.owner_service.observed_state` |
| authority | `authority.cluster.owner_service.runtime_state` |

## The loop (and the verdict at each step)

```text
load cluster_operator pack  → principle is PROPOSED (never auto-promoted)
satisfy promotion gate      → mark contradiction-checked + record evidence
PromotePrinciple (high risk) → ALLOWED only with explicit human approval → PROMOTED
ResolveGovernedContext      → returns the principle + its generative recommended behavior
CheckAction(forbidden claim) → BLOCKED            (matches the forbidden move)
CheckAction(claim, no evid)  → NEEDS_EVIDENCE      (desired_state + observed_state missing)
CheckAction(claim, evidence + approval) → ALLOWED  (evidence satisfied, authority resolvable, approved)
RecordOutcome(success)      → outcome linked to the allowed action check
RDF export                  → the full semantic chain is projected as N-Triples
```

Observed verdicts (from the test log):

```text
✓ promoted: principle.cluster.no_recovery_claim_without_authoritative_evidence
✓ ResolveGovernedContext returns the principle; recommended: "Before claiming recovery,
  compare desired state and observed runtime state through the owner authority …"
✓ CheckAction(forbidden claim)               → blocked
✓ CheckAction(claim_recovery, no evidence)   → needs_evidence
    [evidence.cluster.owner_service.desired_state, evidence.cluster.owner_service.observed_state]
✓ CheckAction(claim_recovery, evidence+approval) → allowed
✓ RecordOutcome → outcome_id …
✓ RDF projection (40 triples) contains the full semantic chain
```

## RDF semantic chain (real projected triples)

The projection reuses the canonical Scylla id as the RDF identity (no separate
RDF-only identity). ScyllaDB stays authoritative; this graph is a derived,
read-only view.

```ntriples
<…/instance/principle/principle.cluster.no_recovery_claim_without_authoritative_evidence>
  <https://globular.io/behavioral#appliesWhen>
  <…/instance/condition/condition.cluster.service.desired_observed_mismatch> .

<…/instance/principle/…no_recovery_claim…>
  <https://globular.io/behavioral#requiresEvidence>
  <…/instance/required_evidence/evidence.cluster.owner_service.desired_state> .

<…/instance/principle/…no_recovery_claim…>
  <https://globular.io/behavioral#requiresEvidence>
  <…/instance/required_evidence/evidence.cluster.owner_service.observed_state> .

<…/instance/principle/…no_recovery_claim…>
  <https://globular.io/behavioral#forbidsMove>
  <…/instance/forbidden_move/forbidden.cluster.claim_recovery_without_authoritative_evidence> .

<…/instance/action_check/<id>>
  <https://globular.io/behavioral#missingEvidence>
  <…/instance/required_evidence/evidence.cluster.owner_service.desired_state> .

<…/instance/outcome/<id>>
  <https://globular.io/behavioral#resultedFrom>
  <…/instance/action_check/<id>> .
```

(`…` abbreviates the `https://globular.io/behavioral/instance/` base.)

## Run it

```bash
# the demo is a deterministic in-memory test (no Scylla required)
go test ./ai_memory/ai_memory_server -run Demo -v

# standard verification
go build ./...
go test ./ai_memory/... ./mcp/...
```

The same chain can be produced against real persistence with the isolated-Scylla
integration test (`TestScyllaStoreIngestionIntegration`, gated by
`BEHAVIORAL_SCYLLA_HOSTS=127.0.0.1`), and exported with the read-only command:

```bash
behavioral-export-rdf -project globular-services -domain cluster_operator -out behavioral.nt
```

## What this proves

The agent does not merely search runbooks — it is **gated** by promoted operator
principles: it is refused when it would claim recovery against a forbidden move,
told exactly which authoritative evidence is missing, allowed only once that
evidence (and human approval, for high-risk) is present, and every step is
recorded and semantically projectable. Promotion never bypasses the gate; RDF is
never the source of truth.
