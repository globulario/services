# Forbidden Historical Scripts

These scripts violate the awareness graph's break-glass principles and
**must not be run**. They are kept in version control as historical reference
and as material for graph enforcement (the names appear in audit findings).

## Why these are forbidden

Each script here violates one or both of:

- `forbidden.hot_deploy_local_binary_as_break_glass` — recovering a broken
  control-plane service by `cp` of a locally-built binary, bypassing the
  repository, manifest checksum verification, and audit trail. Local builds
  lack ldflags-injected `Version`/`BuildCommit`/`SourceCommit` and report
  `0.0.0-dev`; their sha256 will never match a manifest's `entrypoint_checksum`,
  so the verifier's `runtime_identity_unproven` invariant fires immediately
  and the deploy is in a broken state forever.

- `forbidden.bypass_cycle_with_direct_storage_write` — writing to etcd /
  ScyllaDB / MinIO directly instead of calling the owning service's typed
  repair RPC. The write bypasses canonicalization, watch invalidation, and
  preconditions; cluster_doctor sees the inconsistency as drift; the next
  reconcile clobbers the write.

Both anti-patterns are defined in `docs/awareness/forbidden_fixes.yaml` and
linked to `meta.circular_dependency_must_have_break_glass`.

## What to do instead

For every layer that previously needed these scripts there is now a typed
escape — use it:

| Broken layer | Use this instead |
|---|---|
| Cluster controller hung / corrupt | Emit `controller.bootstrap_handoff_required` event + operator restart of leader |
| Node-agent install loop | Day-1 join script fetches from GitHub release, not the cluster repository |
| etcd member needs rejoin | `cluster_controller.RejoinNode` RPC enforces 4 preconditions |
| Repository corruption | `globular pkg publish --force` + `--unseal-official --reason --prior-digest` flow with audit logging |

If a needed escape does not exist, **file an invariant for it**; do not
invent an ad-hoc `cp` or `etcdctl put`.

## The scripts

### `fix-etcd.sh`

Hot-deploys a locally-built `cluster_controller_server` binary, then writes
`etcd.yaml` and `etcd_endpoints` directly. Bypasses every gate in
`etcd_rejoin.go`. Originally written for a 2026-Q1 incident where the typed
rejoin RPC did not yet exist. It does now.

### `fix-dell-etcd.sh`

Same pattern, specialized for the dell node. Same forbidden alternative.

## Removing these

These scripts will be deleted once the equivalent typed RPCs have been
exercised end-to-end in a recovery rehearsal. Keep them here as the
canonical bad-example reference until then.
