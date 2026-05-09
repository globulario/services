# Awareness Operator Cockpit

This document is for operators and agents who need to understand what the awareness system is saying without reading source code.

---

## The Five Awareness Signals

| Signal | Tool | Meaning |
|--------|------|---------|
| **Graph freshness** | `awareness.session_start` | Is the static graph current? |
| **Live overlay** | `awareness.health_pulse` | Are live collectors working? |
| **Graph integrity** | `awareness.graph_integrity_check` | Is the knowledge graph internally consistent? |
| **Proposal queue** | `awareness.proposal_queue_health` | Are learned fixes queued and unreviewed? |
| **Agent usage** | `awareness.agent_usage_report` | Are agents actually using awareness? |

Run `awareness.session_start` at the top of every session. It reports all five signals in one call.

---

## Reading health_pulse Output

```json
{
  "status": "warning",
  "graph": { "stale": false, "age_seconds": 120 },
  "live_overlay": { "status": "stale", "age_seconds": 720 },
  "agent_usage": { "preflight_skip_rate_pct": 33.0, "status": "ok" },
  "alerts": [
    { "id": "live_overlay.stale", "severity": "warning",
      "message": "live overlay 720s old — run 'globular awareness live-snapshot'" }
  ]
}
```

**status** is the worst-case across all sections.

### Status values

| Status | Meaning |
|--------|---------|
| `ok` | All checks pass |
| `warning` | Advisory issues (agents can proceed; operators should investigate) |
| `critical` | Hard failures — do not rely on awareness for correctness |
| `no_data` | Not enough history to judge |

---

## Coverage States

Each invariant has a coverage state derived from graph edges:

| State | Meaning |
|-------|---------|
| `full` | Has implementation + test + failure mode |
| `partial` | Has implementation but missing test or failure mode |
| `declared` | YAML-authored — no implementation evidence in code |
| `inferred` | Derived from code patterns — not explicitly declared |
| `none` | No coverage at all — should not exist for active invariants |

Run `awareness.coverage_report` to see the current distribution.

---

## Confidence and Blind Spots

`preflight` and `agent_context` always return a `confidence` field and a `blind_spots` list.

**confidence values:** `high` | `medium` | `low` | `unknown`

Blind spots are conditions that the check could not verify:
- `live overlay absent` — no live collector data was collected
- `runtime is noop` — no cluster address configured; live health unknown
- `graph stale` — static graph may miss recent code changes
- `file not indexed` — the file being edited is not in the graph

**CRITICAL: A clean `preflight` response with blind spots is NOT a safety guarantee.** Always read and address blind spots before making risk-gated decisions.

---

## NO_MATCH Does Not Mean Safe

When `preflight` or `agent_context` returns `NO_MATCH`, it means:
- No node in the graph matched the task description
- Coverage was not assessed against the changed files
- The system has no opinion, not a clean opinion

**Do NOT interpret NO_MATCH as "this change is safe."**

If NO_MATCH appears with blind spots, the probability of a false clean is high. Grep `docs/awareness/` YAML files directly for relevant terms before proceeding.

---

## Graph Integrity Failures

`awareness.graph_integrity_check` and the CI gate `GraphIntegrityCICheck` detect:

| Code | Meaning | CI Severity |
|------|---------|-------------|
| `INVARIANT_NO_IMPLEMENTATION` | Active invariant has no implementing file | Error (fails CI) |
| `INVARIANT_NO_TEST_COVERAGE` | Implemented invariant has no test | Warning |
| `INVARIANT_NO_FORBIDDEN_FIX` | Invariant missing forbidden_fix | Warning |
| `REQUIRED_TEST_MISSING` | required_test node has no real file path | Error (fails CI) |
| `DONE_FIXCASE_SCAFFOLD_ONLY` | DONE fix case verified only by TODO stubs | Error (fails CI) |

Warnings do not fail CI. Errors do.

---

## Common Scenarios

### Stale live overlay

```
live_overlay.status = "stale"
live_overlay.age_seconds = 720
```

**Action:** Run `globular awareness live-snapshot` or wait for the systemd timer to fire (every 5 minutes by default).

### Workflow retry storm

If `awareness.health_pulse` shows many `workflow_execution` collector failures:
1. Check `globular workflow list-runs --state failed` for stuck workflows
2. Check etcd for orphaned lease keys: `globular etcd list /globular/leases/`
3. Restart workflow service if lease TTL is exceeded

### DNS/cert mismatch

If `awareness.preflight` reports `service_endpoint_covered_by_cert` mismatches:
1. Run `awareness.file_invariant_context file=golang/cluster_controller/.../handlers_join.go`
2. Check certificate SANs via `openssl x509 -in /var/lib/globular/pki/issued/services/service.crt -text -noout | grep DNS`
3. Cross-reference with `docs/awareness/knowledge/dns_zones.yaml`

### Proposal queue stale

```
proposal_queue.status = "stale"
proposal_queue.stale_proposals = 3
```

**Action:** Run `awareness.pending_proposals` to list them, then `awareness.approve_proposal` or `awareness.validate_proposal` on each.

### Graph integrity failure in CI

```
[INVARIANT_NO_IMPLEMENTATION] invariant:service.endpoint.etcd_address_reachability has no implementing file
```

**Action:** Either add an `implements` edge in the YAML knowledge or add the implementation code. Never delete the invariant to fix CI.
