# Awareness Coverage Report

Deterministic per-run output. To diff cleanly between runs, skip the first 3 lines (the operator-facing header).

## Summary

- **Source files scanned (Go, non-test, non-generated):** 963
- **Files with at least one direct anchor:** 340 (35%)
- **Files with zero direct anchors:** 623 (65%)
- **Candidate entries (NOT counted in canonical coverage):** 2

## Canonical anchors by class

| Class | Entries in canonical YAML |
|---|---|
| invariant | 206 |
| failure_mode | 100 |
| intent | 208 |
| incident_pattern | 4 |
| code_symbol | 246 |

_`code_symbol` entries come from `docs/awareness/generated/*_code_symbols.yaml` (the source-side `@awareness` annotation scan)._

## High-risk directories with unanchored files

Files under these paths are listed in CLAUDE.md R2 as high-risk. Uncovered files here trigger Phase 5's honest-DEGRADED gate in Preflight.

| Directory | Anchored / Total | Uncovered |
|---|---|---|
| `golang/cluster_controller/cluster_controller_server` | 61/123 | 62 |
| `golang/mcp` | 26/58 | 32 |
| `golang/cluster_doctor/cluster_doctor_server/rules` | 31/60 | 29 |
| `golang/node_agent/node_agent_server` | 30/56 | 26 |
| `golang/repository/repository_server` | 35/50 | 15 |
| `golang/node_agent/node_agent_server/internal/actions` | 10/22 | 12 |
| `golang/rbac/rbac_server` | 5/14 | 9 |
| `golang/ai_executor/ai_executor_server` | 5/13 | 8 |
| `golang/cluster_controller/cluster_controller_server/operator` | 0/4 | 4 |
| `golang/cluster_controller/cluster_controllerpb` | 0/4 | 4 |
| `golang/cluster_doctor/cluster_doctor_server` | 14/17 | 3 |
| `golang/cluster_doctor/cluster_doctor_server/collector` | 3/6 | 3 |
| `golang/cluster_doctor/cluster_doctor_server/render` | 0/3 | 3 |
| `golang/cluster_controller/cluster_controller_server/projections` | 0/2 | 2 |
| `golang/cluster_controller/resourcestore` | 0/2 | 2 |
| `golang/node_agent/node_agent_server/internal/certs` | 0/2 | 2 |
| `golang/repository/repositorypb` | 0/2 | 2 |
| `golang/cluster_controller/cluster_controller_server/internal/recovery` | 0/1 | 1 |
| `golang/cluster_controller/cluster_controller_server/rolling` | 0/1 | 1 |
| `golang/cluster_doctor/cluster_doctor_client` | 0/1 | 1 |

## Best-covered directories (top 20 by coverage ratio)

| Directory | Anchored / Total | % |
|---|---|---|
| `golang/domain` | 7/7 | 100% |
| `golang/repository/upstream` | 6/6 | 100% |
| `golang/cluster_controller/cluster_controller_server/internal/dnsprovider` | 4/4 | 100% |
| `golang/attestation` | 1/1 | 100% |
| `golang/dependency` | 1/1 | 100% |
| `golang/evidence` | 1/1 | 100% |
| `golang/globular_service/lkg` | 1/1 | 100% |
| `golang/netutil` | 1/1 | 100% |
| `golang/node_agent/node_agent_server/internal/apply` | 1/1 | 100% |
| `golang/node_agent/node_agent_server/internal/ingress/keepalived` | 1/1 | 100% |
| `golang/node_agent/node_agent_server/internal/supervisor` | 1/1 | 100% |
| `golang/security` | 12/13 | 92% |
| `golang/cluster_doctor/cluster_doctor_server` | 14/17 | 82% |
| `golang/workflow/engine` | 11/14 | 78% |
| `golang/dns/dns_server` | 3/4 | 75% |
| `golang/remediation` | 3/4 | 75% |
| `golang/repository/repository_server` | 35/50 | 70% |
| `golang/interceptors` | 6/9 | 66% |
| `golang/authentication/authentication_server` | 2/3 | 66% |
| `golang/installed_state` | 2/3 | 66% |

## Recommended next annotation targets (high-risk + unanchored)

These are uncovered Go files under CLAUDE.md R2 high-risk dirs. Sorted by path. Use this list as the source for the next round of candidate filings — Preflight will return DEGRADED on each of them today.

- `golang/ai_executor/ai_executor_server/anthropic_client.go`
- `golang/ai_executor/ai_executor_server/claude.go`
- `golang/ai_executor/ai_executor_server/conversation_store.go`
- `golang/ai_executor/ai_executor_server/handlers_conversation.go`
- `golang/ai_executor/ai_executor_server/handlers_peer.go`
- `golang/ai_executor/ai_executor_server/job_store.go`
- `golang/ai_executor/ai_executor_server/notifier.go`
- `golang/ai_executor/ai_executor_server/peers.go`
- `golang/cluster_controller/cluster_controller_server/actor_service.go`
- `golang/cluster_controller/cluster_controller_server/agentclient.go`
- `golang/cluster_controller/cluster_controller_server/apply_loop_detector.go`
- `golang/cluster_controller/cluster_controller_server/bootstrap_phases.go`
- `golang/cluster_controller/cluster_controller_server/bounded_query.go`
- `golang/cluster_controller/cluster_controller_server/component_catalog.go`
- `golang/cluster_controller/cluster_controller_server/component_resolve.go`
- `golang/cluster_controller/cluster_controller_server/config.go`
- `golang/cluster_controller/cluster_controller_server/handlers_cluster.go`
- `golang/cluster_controller/cluster_controller_server/handlers_resolve.go`
- `golang/cluster_controller/cluster_controller_server/handlers_status.go`
- `golang/cluster_controller/cluster_controller_server/handlers_upgrade.go`
- `golang/cluster_controller/cluster_controller_server/infra_probes.go`
- `golang/cluster_controller/cluster_controller_server/internal/recovery/grpc_recovery.go`
- `golang/cluster_controller/cluster_controller_server/joinplan_sign.go`
- `golang/cluster_controller/cluster_controller_server/joinplan_types.go`
- `golang/cluster_controller/cluster_controller_server/joinplan_validate.go`
- `golang/cluster_controller/cluster_controller_server/kind_mismatch_etcd.go`
- `golang/cluster_controller/cluster_controller_server/leader_pending_update_etcd.go`
- `golang/cluster_controller/cluster_controller_server/main.go`
- `golang/cluster_controller/cluster_controller_server/minio_pools.go`
- `golang/cluster_controller/cluster_controller_server/native_dependency_block.go`
- `golang/cluster_controller/cluster_controller_server/node_agent_endpoint_fallback.go`
- `golang/cluster_controller/cluster_controller_server/node_infra_intents.go`
- `golang/cluster_controller/cluster_controller_server/objectstore_reconciler.go`
- `golang/cluster_controller/cluster_controller_server/objectstore_topology_status.go`
- `golang/cluster_controller/cluster_controller_server/objectstore_transition.go`
- `golang/cluster_controller/cluster_controller_server/operations.go`
- `golang/cluster_controller/cluster_controller_server/operator/etcd_operator.go`
- `golang/cluster_controller/cluster_controller_server/operator/minio_operator.go`
- `golang/cluster_controller/cluster_controller_server/operator/operator.go`
- `golang/cluster_controller/cluster_controller_server/operator/scylla_operator.go`
- `golang/cluster_controller/cluster_controller_server/plan_signer.go`
- `golang/cluster_controller/cluster_controller_server/posture_metrics.go`
- `golang/cluster_controller/cluster_controller_server/profiles_deduce.go`
- `golang/cluster_controller/cluster_controller_server/projections/node_identity.go`
- `golang/cluster_controller/cluster_controller_server/projections/schema.go`
- `golang/cluster_controller/cluster_controller_server/projections_init.go`
- `golang/cluster_controller/cluster_controller_server/promotion_reconciler.go`
- `golang/cluster_controller/cluster_controller_server/reconcile_etcd_endpoints.go`
- `golang/cluster_controller/cluster_controller_server/reconcile_lane_status.go`
- `golang/cluster_controller/cluster_controller_server/reconcile_metrics.go`
- _… and 184 more (truncated to top 50 for readability)_

## Candidates pending review

These entries live in `docs/awareness/candidates/`. They are NOT active in the awareness graph — the build pipeline explicitly skips that directory (Phase 3). Promote with `scripts/promote-awareness-candidate.py --id <id> --target <target.yaml>`.

| Candidate ID | Class | Risk | Confidence | Discovered from |
|---|---|---|---|---|
| `preflight.high_risk_no_anchor_must_degrade_not_low_risk` | invariant | high | high | Phase 5 design + implementation; awareness-graph v0.0.11; commit 1c45551 + |
| `remediation.test_audit_writes_must_be_isolated_from_production_etcd` | invariant | medium | high | Patch C audit-leak fix; commit a80f7a83 on services/master @ 2026-06-02 17:35 ED |

_All entries above are marked **not active in awareness graph** until explicitly promoted._

