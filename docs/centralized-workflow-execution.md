# Centralized Workflow Execution — Design Note (Phase A)

**Status:** Implementation complete (Phases A–F). All production workflows
execute through WorkflowService. Old execution paths removed.

---

## 1. Executor Ownership

**WorkflowService is the single production workflow executor.**

The `engine.Engine` library (in `golang/workflow/engine/`) remains the execution
core. Today it is embedded in-process by cluster-doctor and cluster-controller.
After migration, only WorkflowService hosts an `engine.Engine` in production.

WorkflowService owns:
- Definition loading (from MinIO)
- DAG compilation and topological execution
- Step dispatch (via actor callbacks)
- Retry, timeout, condition evaluation
- Run/step/event recording (auto-recorded to ScyllaDB during execution)
- Child workflow spawning
- Hook dispatch (onFailure, onSuccess)

WorkflowService does NOT own:
- Business logic behind any action
- Finding resolution, risk assessment, remediation execution
- Bootstrap phase transitions, release selection, reconcile decisions
- Package install, systemd operations

---

## 2. Actor Ownership

**Actor services own their business logic. WorkflowService calls them; it
never absorbs their semantics.**

Each actor type maps to a service that exposes a `WorkflowActorService` gRPC
service with a single unary RPC: `ExecuteAction`.

| Actor type | Owning service | Business logic |
|---|---|---|
| `cluster-doctor` | cluster_doctor | Finding cache, remediation execution, convergence verification |
| `cluster-controller` | cluster_controller | Bootstrap phases, release selection, reconcile decisions, event emission |
| `node-agent` | node_agent | Package install/remove, systemd ops, local health checks |
| `installer` | cluster_controller (in-process) | Install sequence, config templating |
| `repository` | cluster_controller (in-process) | Artifact resolution, version comparison |
| `workflow-service` | workflow_server (self) | Child workflow dispatch, drift tracking |
| `operator` | (external / approval gate) | Manual approval, incident actions |

When the workflow engine (inside WorkflowService) encounters a step with
`actor: cluster-doctor`, it calls the doctor's `ExecuteAction` RPC. The
doctor resolves the action name against its local Router (which has the same
`RegisterDoctorRemediationActions` handlers wired to local state) and returns
the result. The workflow engine never interprets what the action did — it only
cares about `ok`, `output_json`, and `message`.

Actor callback handlers MUST be leaf operations. If sequence, dependency,
waiting, or branching is required, it belongs in the workflow YAML definition,
not inside a callback handler. This preserves workflow-vs-plan separation.

---

## 3. Definition Ownership

**MinIO (via WorkflowService) is the single source of truth for production
workflow definitions.**

### Final-release rule

- Definitions are stored in MinIO under `globular-config/workflows/{name}.yaml`
- WorkflowService loads them via `GetWorkflowDefinition` / internal MinIO fetch
- Actor services do NOT carry embedded copies of production workflows
- The `golang/workflow/definitions/` directory is the source-controlled
  reference that gets published to MinIO during packaging/bootstrap

### What is removed

- `go:embed workflow_remediate_doctor_finding.yaml` in cluster_doctor
- Filesystem path scanning (`/var/lib/globular/workflows/`, `/usr/lib/globular/workflows/`)
  in cluster_controller
- Any "kept in sync manually" dual-path distribution

### What remains (non-production only)

- `golang/workflow/definitions/*.yaml` as the git source of truth (published to MinIO)
- Test-time loading of YAMLs for action registration validation (the existing
  `TestEmbeddedWorkflowsHaveRegisteredActions` pattern)

---

## 4. Action Registration Model

**Every action referenced in YAML must be validated against a declared registry
at test time. No stringly-typed magic at runtime.**

### How it works today

The existing `TestEmbeddedWorkflowsHaveRegisteredActions` (in
`engine/action_registration_test.go`) already does exactly this:

1. Loads all YAML definitions from disk
2. Builds a Router with all known actor registrations (`registerAllActorsWithStubs`)
3. Walks every step and hook in every YAML
4. Asserts that every `(actor, action)` pair resolves to a registered handler
5. Fails the test with the exact missing pair and originating YAML file

This test catches: renamed actions, new steps without handlers, mismatched
actor constants.

### What changes

The test stays. It expands to also validate the reverse direction:

- **YAML → Registry** (existing): every action in YAML has a registered handler
- **Registry → Actor capability** (new): every actor declares the exact set of
  actions it supports. The `WorkflowActorService` implementation on each actor
  service resolves actions against its local Router — if an action is registered
  in the engine but not wired to a real handler in the actor, the callback will
  fail at runtime. To catch this at test time, actor-side tests must verify that
  their local Router covers the same action set as the engine stubs.

### Fallback dispatch is transport-only

The workflow service's Router uses a fallback handler per actor type. This
fallback is a **transport mechanism**: it marshals the `ActionRequest` to gRPC
`ExecuteAction` and sends it to the actor's endpoint. It does NOT interpret,
validate, or modify the action semantics.

Action validation happens in two places:
1. **Test time** — `TestEmbeddedWorkflowsHaveRegisteredActions` ensures every
   YAML action has a registered handler
2. **Actor runtime** — the actor's local Router resolves the action name. If
   the action is unknown, the actor returns an error. The workflow engine
   treats this as a step failure.

The fallback MUST NOT silently accept unknown actions. If an actor receives an
`ExecuteAction` call for an action it doesn't recognize, it MUST return an
error (not a no-op success).

---

## 5. Observability Model

**All production workflow runs land in one observability surface.**

### What WorkflowService records (auto-recording during execution)

| Event | ScyllaDB table | Trigger |
|---|---|---|
| Run started | `workflow_runs` | `ExecuteWorkflow` RPC entry |
| Step started | `workflow_steps` | Before actor callback |
| Step completed/failed | `workflow_steps` | After actor callback returns |
| Run completed/failed | `workflow_runs` | Engine execution finishes |
| Outcome summary | `workflow_run_summaries` | Run finish (bounded O(# definitions)) |
| Step outcome telemetry | `workflow_step_outcomes` | Step finish |
| Events/artifacts | `workflow_events`, `workflow_artifact_refs` | During execution |

### What disappears

- The separate `Recorder` fire-and-forget pattern for workflow-service-driven
  runs (auto-recording replaces it)
- Invisible in-process runs that never hit the workflow history surface

### What the Recorder remains for

- Services that haven't been migrated yet (transition period only)
- Non-workflow operational events that still need fire-and-forget recording

### Observable outcome

After migration, `remediate.doctor.finding` runs appear in the same
`ListRuns` / `GetRun` / `WatchRun` surface alongside `release.apply.package`,
`node.bootstrap`, `cluster.reconcile`, etc. Step timing, actor callbacks,
terminal states, and child workflow edges are all visible.

---

## 6. Endpoint Resolution

**All actor callbacks use `config.ResolveDialTarget`.**

When the workflow service needs to call back to an actor:
1. The `ExecuteWorkflow` request includes `actor_endpoints` mapping actor type
   to a raw endpoint string (e.g. `"cluster-doctor": "localhost:10300"`)
2. Before dialing, the workflow service passes the endpoint through
   `config.ResolveDialTarget` to get a `DialTarget` with TLS-safe address and
   correct `ServerName`
3. The gRPC connection uses the resolved `Address` and `ServerName`

No actor may:
- Invent its own loopback rewrite
- Derive TLS ServerName ad hoc
- Bypass `ResolveDialTarget`

This reduces dial-path diversity rather than creating a new one.

---

## 7. Proto Changes Summary

### New service: `WorkflowActorService`

Defined in `proto/workflow_actor.proto` (same `go_package` as `workflowpb`).

```
service WorkflowActorService {
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteActionResponse);
}
```

- `ExecuteActionRequest`: run_id, step_id, actor, action, with (JSON),
  inputs_json, outputs_json
- `ExecuteActionResponse`: ok, output_json, message

### New RPC on `WorkflowService`

```
rpc ExecuteWorkflow(ExecuteWorkflowRequest) returns (ExecuteWorkflowResponse);
```

- `ExecuteWorkflowRequest`: cluster_id, workflow_name, inputs_json,
  actor_endpoints (map), correlation_id
- `ExecuteWorkflowResponse`: run_id, status, error, outputs_json

JSON string encoding for inputs/outputs (not `google.protobuf.Struct`) to
preserve types and match existing `details_json` convention.

---

## 8. Engine Changes

### Router: add `RegisterFallback`

```go
func (r *Router) RegisterFallback(actor ActorType, h ActionHandler)
```

`Resolve()` checks exact `"actor::action"` match first, then falls back to
the actor's fallback handler. This is used by the workflow service to route
all actions for a remote actor through a single gRPC dispatch handler.

Local actor Routers (inside doctor, controller) do NOT use fallbacks — they
use explicit `Register(actor, action, handler)` as today.

---

## 9. Migration Sequence

| Phase | Scope | Acceptance |
|---|---|---|
| B | Proto + registry | `WorkflowActorService` proto, `ExecuteWorkflow` proto, `RegisterFallback` on Router, action validation tests pass |
| C | Executor in WorkflowService | `ExecuteWorkflow` handler with remote dispatch, auto-recording, integration tests with mock actor |
| D | Doctor migration | `remediate.doctor.finding` runs via WorkflowService, visible in unified history, same behavior |
| E | Controller migration | Bootstrap, release, reconcile, repair flows via WorkflowService, one family at a time |
| F | Cleanup | Remove embeds, filesystem fallbacks, old Recorder usage, no dual paths remain |

Each phase is independently testable. No phase changes workflow semantics.

---

## 10. What This Refactor Does NOT Change

- Remediation workflow behavior (resolve → assess → approve → execute → verify)
- Step ordering within any workflow
- Risk/approval semantics
- LOW-risk auto-execution rules
- Structured-action blocklists (ETCD_PUT, ETCD_DELETE, NODE_REMOVE)
- Plan vs workflow boundary
- The engine library's internal execution model
