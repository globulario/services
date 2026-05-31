# Container Training Loop — Awareness Graph Improvement Workflow

This document describes the manual feedback loop for improving the awareness graph
using evidence collected from globular-quickstart containerized test scenarios.

The loop is: **scenario failure → evidence collected → human reviews → fix applied →
test passes → incident recorded → proposal drafted → human approves → graph rebuilds**.

---

## Prerequisites

1. **globular-quickstart** running (`make up && make test-wait` in the quickstart repo)
2. **globular CLI** in PATH with awareness subcommands (`globular awareness --help`)
3. Awareness graph loaded (run `globular awareness briefing --task test` to verify connectivity)

---

## The Loop: Step by Step

### Step 1 — Run a training scenario

```bash
# In globular-quickstart repo:
make awareness-train-day0
# or: AWARENESS_TRAINING=1 AWARENESS_INCLUDE_RUNTIME=1 \
#     tests/harness/bin/globular-test scenario \
#     tests/scenarios/training/<scenario>.yaml
```

Env flags:
| Flag | Default | Effect |
|------|---------|--------|
| `AWARENESS_TRAINING=1` | 0 | Enables incident creation on failure |
| `AWARENESS_INCLUDE_RUNTIME=1` | 0 | Collects runtime snapshot during run |
| `AWARENESS_RUNTIME_WINDOW` | `30m` | How far back runtime snapshot looks |
| `AWARENESS_REQUIRED=1` | 0 | Makes awareness unavailability fail the scenario |

### Step 2 — Inspect evidence

Each scenario run writes to `tests/reports/<run-id>/<scenario-name>/`.

```
<run-id>/<scenario>/
├── RESULT.md                   # human-readable summary with awareness table
├── evidence.json               # machine-readable run record (result, awareness_result)
├── awareness/
│   ├── preflight.agent.txt     # preflight analysis
│   ├── preflight.json          # structured preflight output
│   ├── debug-session.agent.txt # failure analysis
│   ├── debug-session.json      # structured failure output
│   ├── runtime-snapshot.json   # cluster state at time of failure
│   ├── did-we-fix.txt          # post-fix check output
│   ├── incident.yaml           # incident record (if created)
│   └── proposal.yaml           # DRAFT proposal (if created — do NOT approve automatically)
└── evidence/
    ├── docker-ps.txt
    ├── docker-events.log
    ├── container-logs/<node>.log
    ├── etcd/<prefix>.keys.txt
    ├── systemd/<node>/<service>.status.txt
    └── ...
```

Shortcut to view latest awareness artifacts:
```bash
make awareness-latest
```

### Step 3 — Review the incident

If `awareness/incident.yaml` was created:

```bash
cat tests/reports/latest/<scenario>/awareness/incident.yaml
```

The incident captures: root cause, matched invariants, matched failure modes,
forbidden fixes, and a learning recommendation.

### Step 4 — Apply a fix

Based on the debug-session and incident analysis, fix the underlying issue:
- Edit source code if the root cause is a bug
- Edit scenario YAML if the test is wrong
- Edit `docs/awareness/failure_modes.yaml` if a new failure mode was discovered
- Edit `docs/awareness/invariants.yaml` if a new invariant applies

**Do not promote proposals at this step.** The proposal is draft only.

### Step 5 — Validate the proposal (optional)

If a proposal was drafted in `awareness/proposal.yaml`:

```bash
# In the services repo:
globular awareness validate-proposal --file \
  /path/to/tests/reports/latest/<scenario>/awareness/proposal.yaml
```

This checks the proposal against the current graph without applying it.

### Step 6 — Re-run the scenario

After applying the fix, re-run to confirm it resolves the failure:

```bash
make awareness-train-day0   # or whichever scenario failed
```

### Step 7 — Approve the proposal (human gate)

Only after the scenario passes, review the proposal and approve if correct:

```bash
# In the services repo:
globular awareness approve-proposal --file \
  /path/to/tests/reports/latest/<scenario>/awareness/proposal.yaml
```

**NEVER run `approve-proposal` automatically.** This is a mandatory human gate.
The proposal changes the awareness graph — incorrect proposals corrupt future
preflight and debug-session analysis.

### Step 8 — View the training ledger

```bash
# In globular-quickstart repo:
make awareness-ledger
```

The ledger (`tests/reports/awareness-training-ledger.jsonl`) records every scenario
run with: scenario name, result (PASS/FAIL/PARTIAL/INFRA_ERROR), awareness status,
matched invariants, matched failure modes, and whether an incident/proposal was created.

---

## Running the Full Training Suite

```bash
# Run all training scenarios sequentially:
make awareness-training-suite

# Reset cluster (preserves ledger):
make awareness-reset
```

Training scenarios are in `tests/scenarios/training/`:

| Scenario | Phase | What it trains |
|----------|-------|----------------|
| `day0-single-node-awareness` | bootstrap | Day-0 boot, etcd up, agent alive |
| `day1-join-second-node-awareness` | day1_join | Node join, quorum expansion |
| `install-loop-awareness` | recovery | Controller restart, reconcile convergence |
| `restart-storm-awareness` | resilience | Node stop/start, quorum stress |
| `missing-state-awareness` | reconcile | 4-layer parity, orphan package detection |

---

## Topology Context

Quickstart uses lab exceptions defined in:
`tests/awareness/quickstart_topology.yaml`

Key exceptions:
- `quickstart.topology_external_scylla_is_lab_only` — ScyllaDB is external, not on founding nodes
- `quickstart.no_minio_quorum_required` — MinIO quorum not enforced
- `quickstart.topology_lab_only_exception` — overall lab marker

When reviewing proposals from quickstart training runs, verify they do not incorrectly
propose relaxing production invariants based on lab topology.

---

## Hard Rules for This Loop

1. **Never approve proposals automatically** — always human review first
2. **Never run remediation scripts automatically** — evidence only, no auto-fix
3. **Scenario result is the truth** — awareness result is supplementary
4. **PARTIAL result needs human triage** — steps failed but some progress was made
5. **INFRA_ERROR means cluster is down** — fix infrastructure before re-running
6. **Ledger is append-only** — never edit or delete ledger entries

---

## Troubleshooting

### Awareness returns SKIPPED
- Check `globular awareness --help` is reachable in PATH
- Verify awareness-graph gRPC service is reachable (`globular awareness briefing --task test`)
- Set `AWARENESS_REQUIRED=1` to get a hard failure instead of silent skip

### Proposal is missing after failure
- Check `create_proposal_on_failure: true` in scenario YAML awareness block
- Check `create_incident_on_failure: true` (proposal requires incident first)
- Look for `awareness/proposal.error.txt` for error details

### Evidence collection timed out
- Evidence collection runs up to 180s — large clusters with many services may need more
- Check `evidence/evidence-errors.log` for individual collection failures
- etcd evidence requires node-1 to be running

### Ledger not updating
- Check `tests/harness/lib/training.sh` exists
- The ledger is only written after `_append_to_ledger()` completes
- Look for `[training] WARNING:` in scenario output
