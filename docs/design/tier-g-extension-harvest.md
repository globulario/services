# Tier G — Extension Harvest

Tier G is not new governance logic. It is the template harvest that makes future
changes follow the already-proven seams instead of re-deriving them.

This repo owns the `services` side of that harvest:

- governed owner-path dispatch patterns
- Go service lifecycle/onboarding patterns
- runbook pointers for operator-facing change paths

Cross-repo note: the invariant-promotion scaffold belongs in `awareness-graph`,
not here. This file only freezes the `services` half.

## 1. Adding a governed owner path

Use `globular ops apply` plus a typed owner RPC as the template. The reference
implementation is [golang/globularcli/ops_cmds.go](/home/dave/Documents/github.com/globulario/services/golang/globularcli/ops_cmds.go:227)
with ratchets in [golang/globularcli/ops_dispatch_test.go](/home/dave/Documents/github.com/globulario/services/golang/globularcli/ops_dispatch_test.go:62)
and [golang/govops/behavioral_test.go](/home/dave/Documents/github.com/globulario/services/golang/govops/behavioral_test.go:18).

### Required shape

1. Mutation enters through an `OperationRequest`, not a raw key/value write.
2. `govops.Validate` decides before dispatch.
3. Refused operations are ledgered and never dispatched.
4. Allowed operations dispatch through a typed owner RPC only.
5. The dispatcher fails closed on missing typed parameters.
6. The owner RPC re-guards the invariant server-side.

### Minimal implementation checklist

- Add one `authority.required_owner_path` string that names the typed RPC.
- Add one dispatcher case in `dispatchThroughOwnerPath`.
- Add one typed dispatch helper that extracts only the parameters that RPC needs.
- Return an error on every missing required parameter before any network call.
- Record outcome data needed by the ledger (`generation_after`, reconcile/postcondition facts).

### Required ratchets

- Unknown owner path returns `no governed dispatcher`.
- Every known path routes to its typed handler.
- Missing required typed params fail closed before dispatch.
- Refused operations do not invoke the owner path.
- Refused operations are ledgered as `REFUSED`.
- If the action names a behavioral forbidden move, the refusal carries the authored rule id.

### Forbidden shortcuts

- No generic raw etcd fallback.
- No `key/path/value` escape hatch in the dispatcher.
- No client-only validation; the owner must re-check server-side.
- No bypass around `govops.Validate` for mutating apply paths.

## 2. Adding a new Go service

Use the shared lifecycle path in [golang/SHARED_PRIMITIVES.md](/home/dave/Documents/github.com/globulario/services/golang/SHARED_PRIMITIVES.md:1)
and the concrete service pattern in [golang/compute/compute_server/server.go](/home/dave/Documents/github.com/globulario/services/golang/compute/compute_server/server.go:317).

### Required shape

1. `globular.HandleInformationalFlags(...)`
2. `globular.ParsePositionalArgs(...)`
3. `globular.NewLifecycleManager(...)`
4. gRPC registration through the shared lifecycle/server startup path
5. Config load/save/validate through shared helpers where the service uses the standard config model

### Service onboarding checklist

- Implement the shared service/lifecycle interfaces instead of inventing a new startup contract.
- Keep `Version` build-injected; do not hardcode release versions in source.
- Resolve runtime authority from etcd or the sanctioned runtime source, not env vars or localhost shortcuts.
- Ensure every mutating RPC has authz annotations.
- If the service owns cluster state, define the owner path and layer authority before coding handlers.
- Add package-level build/test coverage for the new service and any serialization boundary it introduces.

## 3. Operator-facing extension

If the feature changes operator procedure, add or update an operational-knowledge
runbook under `docs/operational-knowledge/runbooks/`. Use
[docs/operational-knowledge/runbooks/add-node-to-minio-pool.yaml](/home/dave/Documents/github.com/globulario/services/docs/operational-knowledge/runbooks/add-node-to-minio-pool.yaml:1)
as the schema reference:

- `schema_version`, `file_kind`, `metadata`
- one or more `entries`
- phased `procedure`
- explicit `success_criteria`
- links back to awareness invariants/failure modes where applicable

## 4. Definition of done

An extension is "boring" only when:

- the mutation path is typed and owner-routed
- the refusal path is deterministic and ratcheted
- the service startup path uses shared lifecycle primitives
- operator procedure is captured if behavior changed
- there is no bespoke escape hatch left behind

If any one of those is missing, the work is still invention, not harvest.
