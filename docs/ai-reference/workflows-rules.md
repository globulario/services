# Workflow Rules

Strict rules governing workflow definition, execution, and state management.

## Definition Rules

- Every workflow MUST have `apiVersion: workflow.globular.io/v1alpha1`
- Every workflow MUST have `spec.strategy.mode` set to `single`, `foreach`, or `dag`
- Every step MUST declare `actor` and `action`
- Actor MUST be one of: `workflow-service`, `cluster-controller`, `cluster-doctor`, `node-agent`, `installer`, `repository`, `operator`, `compute`
- `foreach` mode MUST specify `collection` and `itemName`
- Step dependencies MUST form a DAG (no cycles)
- `onFailure` hook is optional but MUST use a valid actor/action

## Execution Rules

- Workflows are executed by the WorkflowService via `ExecuteWorkflow` RPC
- The caller MUST provide `actor_endpoints` mapping actor types to gRPC addresses
- The workflow engine resolves actors by dialing the endpoint and calling `ExecuteAction`
- Each step receives: `run_id`, `step_id`, `actor`, `action`, `with_json`, `inputs_json`, `outputs_json`
- Step outputs are merged into the workflow's accumulated outputs
- `$.field_name` expressions in `with:` resolve against accumulated outputs

## Terminal States

- A workflow run MUST reach either `RUN_STATUS_SUCCEEDED` or `RUN_STATUS_FAILED`
- Individual steps reach `SUCCEEDED` or `FAILED`
- A failed step triggers the `onFailure` hook if defined
- The workflow engine records run status to ScyllaDB for observability

## Actor Service Contract

- Every actor MUST implement `WorkflowActorService.ExecuteAction`
- ExecuteAction receives the action name and JSON-serialized inputs
- ExecuteAction MUST return `ok: true/false` and optional `output_json`
- Unknown actions MUST be rejected with `ok: false` (never silently accepted)
- Actions MUST NOT block indefinitely — use internal timeouts

## Idempotency

- Steps marked `idempotency: safe_retry` can be re-executed safely
- Steps marked `resume_policy: verify_effect` check if the effect already exists
- All compute workflow actions are designed for safe retry

## Correlation

- `correlation_id` in `ExecuteWorkflowRequest` is used for run deduplication
- Per-run routers are registered/unregistered by correlation_id
- The correlation_id persists across retries of the same logical operation

## Registered Workflows

| Workflow | Purpose | Actors |
|----------|---------|--------|
| day0.bootstrap | Cluster Day-0 bootstrap | node-agent, cluster-controller |
| node.bootstrap | Node bootstrap progression | cluster-controller |
| node.join | Day-1 package install + converge | cluster-controller, node-agent |
| node.repair | Node diagnosis + repair | cluster-controller, cluster-doctor |
| cluster.reconcile | Drift detection + remediation | cluster-controller, cluster-doctor |
| release.apply.package | Generic package rollout | cluster-controller, node-agent |
| release.apply.infrastructure | Infrastructure package rollout | cluster-controller, node-agent |
| release.apply.controller | Leader-aware controller rollout | cluster-controller |
| release.remove.package | Package uninstall | cluster-controller, node-agent |
| remediate.doctor.finding | Doctor finding remediation | cluster-doctor |
| compute.job.submit | Compute job lifecycle | compute |
| compute.unit.execute | Single unit execution | compute |
| compute.job.aggregate | Job result aggregation | compute |
