# Awareness Coverage Priority — Top 20 High-Risk Files

**Generated:** 2026-06-02
**Source pool:** 9 high-risk directories (CLAUDE.md R2): `golang/{node_agent,cluster_controller,repository,rbac,security,cluster_doctor,mcp,services_manager,ai_executor}` (`services_manager` does not exist in this tree).
**Probed by:** live `awareness.briefing` (compact) against the deployed awareness-graph v0.0.10 on 2026-06-02 ~17:40 EDT.
**Outputs:** this file + `coverage_priority.tsv` (machine-readable).

## Method

1. Enumerated every non-test, non-generated `.go` file in each high-risk directory (~500 candidates).
2. Filtered by `git log --since="60 days ago"` to surface recently-edited files, sorted by edit count (high-recency proxy for "where future agents will work").
3. Probed `awareness.briefing` on each surviving candidate; classified `current_awareness_result` as **EMPTY** (no anchors), **THIN** (1–2 anchors for a load-bearing file), or **RICH** (skip — adequate coverage).
4. Applied the five priority rules:
   - High-risk directory (CLAUDE.md R2) — all candidates already qualify
   - Recently edited or repeatedly patched — used edit count as proxy
   - EMPTY / UNKNOWN_IMPACT today — explicitly prioritized
   - Healer / remediation / executor / audit / preflight / MCP bridge / cluster-doctor — weighted to top of list per the user's stated priority
   - Wrong-edit blast radius (unsafe auto-healing, broken rollback, bad audit, cluster mutation, privilege escalation)
5. Top 20 ranked. Files with already-rich coverage (`cluster_controller_server/reconcile_actions.go`, `workflow_release.go`, `desired_state_handlers.go`, `sync_from_upstream.go`, `artifact_handlers.go`, `internal/actions/artifact.go`, `installer_api.go`, `grpc_workflow.go`, `state.go`, `server.go` etc.) were excluded — they already give useful briefings.

## Honesty section (probe results, not inferences)

13 of the top 20 returned **EMPTY**. 7 returned **THIN** (1–3 anchors for files with significantly more concerns). Every result below is from a live `awareness.briefing` call, recorded verbatim in the TSV.

Files that briefed RICH (sufficient coverage; **not** in the top 20):

| File | Anchors |
|---|---|
| `cluster_controller_server/reconcile_actions.go` | 4 inv + 4 fm + multiple incident patterns |
| `cluster_controller_server/workflow_release.go` | 3 inv + 6 fm + 1 incident pattern + 1 intent |
| `cluster_controller_server/desired_state_handlers.go` | 1 critical invariant on desired-state authority |
| `cluster_controller_server/state.go` | 3 inv + 2 fm covering VIP / node identity |
| `cluster_controller_server/release_reconciler.go` | 2 inv + 2 fm |
| `cluster_controller_server/server.go` | 1 critical inv + 1 fm + 1 intent on bootstrap |
| `repository_server/sync_from_upstream.go` | 4 inv + 4 fm + 1 intent |
| `repository_server/artifact_handlers.go` | 7 inv + 1 fm + 2 forbidden fixes |
| `node_agent_server/internal/actions/artifact.go` | 1 inv + 3 fm + 1 intent |
| `node_agent_server/installer_api.go` | 4 inv + 5 fm |
| `node_agent_server/grpc_workflow.go` | 1 inv + 2 fm |
| `cluster_doctor_server/handler_remediation.go` | RICH (covered in past sessions) |

These are the model the EMPTY/THIN files need to grow toward.

## Top 20 priority

Full machine-readable form in `coverage_priority.tsv`. Summary below.

### Tier A — Healer / remediation / executor (#1–#9)

The user's stated highest-priority cluster. Patch C touched all of these in M1/M2/M3.

| # | File | Status | Risk if wrong edit |
|---|---|---|---|
| 1 | `cluster_doctor_server/rules/heal_policy.go` | EMPTY | Silent re-enable of Path-B-style mutation through a disposition flip |
| 2 | `cluster_doctor_server/rules/healer.go` | THIN | Reintroducing RemoteOps mutation surface in `rules.Healer` |
| 3 | `cluster_doctor_server/rules/artifact_integrity.go` | EMPTY | Emitting FILE_DELETE instead of DELETE_CACHE_ARTIFACT silently breaks dispatch |
| 4 | `cluster_doctor_server/node_agent_dialer.go` | EMPTY | Implementing the FileDelete stub broadens the auto-mutation surface |
| 5 | `cluster_doctor_server/config.go` | EMPTY | Default `HealerMode = enforce` re-enables auto-mutation cluster-wide |
| 6 | `cluster_doctor_server/rules/registry.go` | THIN | Registering a HealAuto invariant without a corresponding policy entry |
| 7 | `cluster_doctor_server/remediation_history.go` | THIN | Audit-read corruption weakens the failure-rate gate |
| 8 | `cluster_doctor_server/collector/snapshot.go` | EMPTY | Stale snapshot drives wrong remediation decision |
| 9 | `cluster_doctor_server/collector/collector.go` | THIN | Collector writes etcd to "correct" observed drift — breaks 4-layer model |

### Tier B — MCP / security / RBAC (#10–#12)

EMPTY files where edits change authorization or the agent-tool surface.

| # | File | Status | Risk |
|---|---|---|---|
| 10 | `mcp/server.go` | EMPTY | Session-drop source + the Phase 6 investigation target |
| 11 | `security/roles.go` | EMPTY | Authorization surface change without explicit review |
| 12 | `rbac/rbac_server/rbac_cleanup.go` | EMPTY | RBAC delete with wrong scope (privilege loss or escalation) |

### Tier C — Controller mutation surface (#13–#16)

EMPTY/THIN files in the cluster_controller package that mutate cluster state.

| # | File | Status | Risk |
|---|---|---|---|
| 13 | `cluster_controller_server/main.go` | EMPTY | Controller serves gRPC before leader election completes |
| 14 | `cluster_controller_server/component_catalog.go` | EMPTY | Package kind inferred from filename instead of the canonical registry |
| 15 | `cluster_controller_server/dns_reconciler.go` | THIN | DNS records published for planned-but-not-installed services |
| 16 | `cluster_controller_server/reconcile_nodes.go` | THIN | Reconciler collapses 4-layer state model |

### Tier D — Node-agent / repository / doctor bootstrap (#17–#20)

The remaining critical paths.

| # | File | Status | Risk |
|---|---|---|---|
| 17 | `node_agent_server/workflow_day0.go` | EMPTY | Day-0 phase skip leaves etcd with partial join state |
| 18 | `repository_server/release_index.go` | THIN | Partial BOM write creates split-brain release-index |
| 19 | `cluster_doctor_server/main.go` | EMPTY | Doctor serves gRPC before collector warmed up |
| 20 | `node_agent_server/certificate.go` | THIN | Cert rotation skipped due to local-clock skew |

## What "recommended_annotations" means in the TSV

Each entry is shorthand for the knowledge nodes that should land in the file's `@awareness` block. Format:

- `intent:<id>` — explains WHY this code exists; usually high-level
- `invariant:<id>` — a load-bearing rule the code enforces
- `failure_mode:<id>` — a specific way this code can go wrong
- `forbidden_fix:<id>` — a tempting "fix" that's structurally wrong

These IDs are recommendations. Some already exist in the awareness graph and can be reused (e.g. `intent:remediation.must_go_through_workflow`); others are new and would land in `docs/awareness/candidates/` first (Phase 3) before being promoted to the canonical YAML.

## Next steps (this is Phase 1; Phases 2–7 follow)

- **Phase 2:** Add minimal `@awareness` blocks to the Tier-A files (healer/remediation/executor) with the recommended invariants/failure_modes. For any anchor that doesn't already exist in `docs/awareness/{invariants,failure_modes,intents}.yaml`, file it as a *candidate* via Phase 3's workflow rather than directly into the canonical YAML.
- **Phase 3:** Implement `docs/awareness/candidates/` + a promotion script so session-discovered facts can flow back into the graph through review.
- **Phase 4:** Add a coverage-report tool that re-runs this audit deterministically (so we can see whether Phase 2 actually moved the needle).
- **Phase 5:** Teach Preflight to return DEGRADED with a specific reason when a file lives under a high-risk path but has zero anchors.
- **Phase 6:** Investigate MCP session-drop on service restart; document and harden.
- **Phase 7:** Tests for everything above.

## Known limitations of this audit

1. Edit-count proxy is imperfect — some load-bearing rarely-edited files (e.g. `cluster_doctor_server/executor.go`) are well-annotated and not on this list, but their stability means a refactor would still benefit from explicit anchors. Future iterations of this report should weight by **risk × edit-volatility × current-coverage** rather than edit count alone.
2. The probe used `briefing` compact mode; `preflight` would catch additional patterns. Run a Preflight pass after Phase 2 to validate that the new anchors lift each file's status from EMPTY/THIN to OK.
3. `services_manager/` is listed in CLAUDE.md R2 but does not exist as a directory in this tree — likely renamed or absorbed elsewhere. CLAUDE.md should be reconciled.
4. Three files in Tier C/D (`main.go`, `cluster_doctor_server/main.go`, `workflow_day0.go`) are `main` packages where `@awareness` block conventions are less established. They may need a different annotation pattern (e.g., per-function rather than file-level).
