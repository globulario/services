# Critical State Absence Audit

Date: 2026-06-28

Scope:
- Guardrail: `critical_state_absence_guard`
- Invariant: `critical_state.absence_is_not_destructive_intent`
- Fix case: `absence_as_destructive_intent`

## Contract

Missing, timed-out, or invalid control-plane state is not a stop or delete
command. Runtime consumers must hold or restore last-known-good state unless
they receive an explicit valid disable/delete intent.

## Current Consumers

| Runtime surface | Current file | Absence behavior |
| --- | --- | --- |
| Ingress / keepalived | `golang/node_agent/node_agent_server/ingress_reconcile.go` | Missing or invalid etcd spec applies ingress LKG or writes waiting status. |
| Objectstore / MinIO contract | `golang/node_agent/node_agent_server/minio_contract_reconcile.go` | Missing etcd config restores the on-disk contract from objectstore LKG when available. |
| xDS / Envoy config | `golang/node_agent/node_agent_server/xds_config_reconcile.go` | Missing/corrupt `xds/config.json` restores from xDS LKG; corrupt LKG is rejected. |
| DNS init config | `golang/node_agent/node_agent_server/dns_sync.go` | Missing/corrupt DNS init config applies DNS LKG when available; no file and no LKG is a non-destructive no-op. |

## Repair Notes

The awareness fix case still referenced deleted consumer filenames:
- `ingress_consumer.go`
- `objectstore_consumer.go`
- `envoy_consumer.go`
- `dns_consumer.go`

The implementation had moved to the reconcilers listed above. This pass updates
the metadata and adds required-name tests:
- `TestMissingKeyDoesNotStopRuntime`
- `TestDeleteKeyWhileRunningKeepsRuntimeActive`

Those tests exercise non-ingress LKG behavior for objectstore, xDS/Envoy, and
DNS so the guardrail is no longer an ingress-only claim.
