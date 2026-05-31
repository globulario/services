# Agent Awareness Decision Rules

When to reach for awareness tools vs. when code reading is enough.

**The core distinction:**
- Code reading answers: *what does the code do right now?*
- Awareness answers: *why does it work that way, what breaks if I change it, what has failed here before?*

Awareness is not a checklist — it is a **project-level context extension**. Use it when
the answer isn't in the code itself.

---

## Trigger Matrix

| Situation | Right tool | Skip if |
|-----------|-----------|---------|
| About to edit a file in `high_risk_files.yaml` | `preflight` compact | — never skip on high-risk files |
| About to edit reconciler / desired state / release bridge / node-agent | `preflight` compact | — never skip |
| About to edit any file classified ARCHITECTURE_SENSITIVE or CONVERGENCE_RISK | `preflight` compact | — never skip |
| "Is it safe to change X?" — unfamiliar file | `impact_file` on the target | File not in high-risk list and change is purely local |
| Error that doesn't trace to an obvious code path | `failure_match_error` with exact error text | Error is a compile error or obvious type mismatch |
| Production incident / cluster not converging | `offline_diagnose` or `live_preflight` | — never skip under incident |
| "How do these two components relate?" / cross-layer design | `path <a> <b>` or `neighborhood` | Same-layer, same-package change |
| Before committing a change classified ARCHITECTURE_SENSITIVE | `scan_violations` on changed files | Pure test/doc commit |
| After fixing a non-obvious invariant violation | `learn_from_fix` | Fix was a compile error or trivial rename |

---

## Tool Selection Guide

### `preflight` (compact)
The front door. Runs alias matching, impact analysis, forbidden-fix lookup, and
did-we-fix check in one call. Returns classification + forbidden_fixes + required_tests.

**Use when:** about to edit anything in `high_risk_files.yaml`, or any file that touches
the 4-layer boundary (reconciler → desired state → installed → runtime).

**Mode discipline:**
- `compact` (default): essential safety fields, ~3KB — use for all routine edits
- `standard`: adds invariant list + failure modes — use for cross-service changes
- `deep`: adds decision traces — use when compact returns UNKNOWN_IMPACT on a suspicious path
- `forensic`: full report — only for incident investigation or "why is this broken?"

Never jump to `forensic` for a typo fix.

**What to do with the output:**
1. If `forbidden_fixes` is non-empty → refuse those approaches, no exceptions
2. If classification is `CONVERGENCE_RISK` → add a required test before committing
3. If classification is `UNKNOWN_IMPACT` → grep `failure_modes.yaml` and `invariants.yaml`
   directly — NO_MATCH does not mean safe
4. Follow at most 2 `next_context_handles` — they are suggestions, not a mandatory checklist

### `impact_file`
Answers: *what invariants, failure modes, and tests govern this file?*

**Use when:** about to refactor an unfamiliar file, or `preflight` flagged it as a
high-impact handle.

**Cost:** small. Safe to call on any file you're unsure about.

### `failure_match_error`
Answers: *have we seen this error before? What caused it?*

**Use when:** an error doesn't make sense from the code alone — logs show something
unexpected, a service is stuck in a state that shouldn't be reachable.

**Input:** paste the exact error string or log line. Don't paraphrase.

### `explain_symptom` / `offline_diagnose`
Answers: *what failure mode does this behavior match?*

**Use when:** debugging a production failure, reconciler stuck, workflow not completing.
`offline_diagnose` works when the cluster is down — feed it journal text, etcdctl output,
or systemd status directly.

### `path <a> <b>` / `neighborhood`
Answers: *how do these two concepts causally relate?*

**Use when:** designing a feature that crosses subsystem boundaries (e.g., adding a field
to `ServiceDesiredVersionSpec` that must propagate through `service_release_bridge` to the
reconciler). Follow the causal chain before writing the code.

### `scan_violations`
Answers: *does my change introduce any known bad patterns?*

**Use when:** after writing code that touches critical paths. Catches `localhost`, `os.Getenv`,
`os/exec` in controller, loopback in gRPC dial, retry loops without terminal conditions.

### `learn_from_fix`
Answers: *how do I feed this fix back so future sessions don't repeat the same mistake?*

**Use when:** a fix addressed a non-obvious invariant violation or corrected something
that wasn't in the knowledge base. Generates a draft proposal → goes through
`approve_proposal` → graph rebuild. Never directly edits YAML.

---

## High-Risk Files (always preflight)

From `high_risk_files.yaml` — preflight is non-negotiable before touching:

```
cluster_controller_server/reconcile_runtime.go
cluster_controller_server/reconcile_dispatch.go
cluster_controller_server/convergence_committer.go
cluster_controller_server/desired_state_handlers.go
node_agent_server/installed_services.go
node_agent_server/heartbeat.go
node_agent_server/apply_package_release.go
node_agent_server/internal/supervisor/
repository_server/metadata_store.go
node_agent_server/server_objectstore.go
xds_server/applied_generation.go
envoy_server/applied_generation.go
```

---

## When to Skip Awareness

Awareness is overhead when the task has no architectural impact:

- Fixing a compile error (missing import, type mismatch, syntax)
- Renaming a variable within a single file
- Adding a CLI flag with no cross-layer effect
- Writing tests for isolated logic (no etcd, no gRPC, no state mutation)
- Updating documentation or comments
- Bumping a version number via `zz_version_generated.go`

Rule of thumb: **if the change cannot affect any of the 4 layers, skip preflight.**

---

## Cost Budget

| Call | Context cost | When justified |
|------|-------------|----------------|
| `preflight` compact | ~3KB | Before any high-risk edit |
| `impact_file` | ~1–2KB | Unfamiliar file, flagged handle |
| `failure_match_error` | ~2KB | Unexplained error |
| `path` / `neighborhood` | ~2–4KB | Cross-layer design |
| `preflight` forensic | ~20KB+ | Incident investigation only |

**Hard limit:** 1 preflight per task turn. Do NOT call `agent_context` in the same
turn as `preflight` — they overlap. Max 5 `decision_trace` calls per session.

---

## The Closed Loop

When a fix addresses something the graph didn't know:

```
fix verified → learn_from_fix → proposals/draft → approve_proposal → graph rebuild
```

This is how awareness stays current. An agent that never feeds back is borrowing
against future sessions.

---

## Graph Staleness

If `preflight` returns `graph_available: false` or `confidence: low`:

1. Proceed with static analysis only (raw YAML matches are still useful)
2. Rebuild and reload the awareness graph to refresh
3. Re-run preflight before treating any result as authoritative
4. `UNKNOWN_IMPACT` ≠ safe — grep `failure_modes.yaml` and `invariants.yaml` directly

The preflight result is still worth reading in degraded mode: forbidden_fixes and
raw YAML matches are returned even without a live graph.
