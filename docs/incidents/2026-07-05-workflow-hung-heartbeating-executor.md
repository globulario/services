# Incident evidence — workflow hung-but-heartbeating executor (2026-07-05)

Captured BEFORE any live mutation. Read-only snapshot.

## Summary
2-node buildout: nuc (10.0.0.8, node 681710ee) joining ryzen (10.0.0.63, node eb9a2dac).
nuc stuck `converging` — 28 installed / 10 planned, `applied_hash=""` — for 20+ minutes.
Root: ServiceRelease workflow runs completed all steps but never closed to terminal;
the executor goroutine is hung-but-still-heartbeating, so neither the reaper nor the
orphan scanner fires, and the controller's `inflightWorkflows` guard blocks re-dispatch
of the entire release wave.

## Contract violated
- `invariant:workflow.must_reach_terminal_state` — run must reach SUCCEEDED/FAILED.
- `meta.half_done_must_not_look_done` (cited at server.go:704) — all steps terminal,
  header non-terminal, no watchdog catches it.
- `meta.state_mutations_must_be_durably_committed_before_side_effects` (spirit) — the
  terminal close (FinishRun) is best-effort in-process with a swallowed error (executor.go:446).

## Smoking gun (observable + code)
- Oldest stuck run started 2026-07-05T13:36:25Z; snapshot taken ~13:59Z => ~23 min old.
- reapStaleRuns staleThreshold = 15 min (server.go:668). Run is PAST threshold yet still
  EXECUTING, NOT reaped to FAILED.
- Therefore the lease grace (server.go:708-719, leaseGracePeriod = 2 min) is active:
  the executor lease is still heartbeating => executor goroutine ALIVE but not progressing.
- Orphan scanner (executor_lease.go:209, orphanHeartbeatTimeout = 30s) also skips it for
  the same reason. Both recovery paths key off lease staleness; a hung-but-alive holder
  escapes both. Liveness == "process alive", not "run making progress".

## Stuck run headers (workflow_get_run) — all identical shape
| run id | status | acknowledged | node | version | all 14 steps |
|---|---|---|---|---|---|
| ServiceRelease/core@globular.io/repository | RUN_STATUS_EXECUTING | false | 2 nodes | 1.2.269 | SUCCEEDED (apply_per_node 5122ms, finalize_release 17ms) |
| ServiceRelease/core@globular.io/resource   | RUN_STATUS_EXECUTING | false | 2 nodes | 1.2.269 | SUCCEEDED (apply_per_node 2637ms, finalize_release 15ms) |
| ServiceRelease/core@globular.io/mcp        | RUN_STATUS_EXECUTING | false | 2 nodes | 1.2.269 | SUCCEEDED (apply_per_node 31964ms, finalize_release 11ms) |

Step sequence (all three): mark_resolved✓ select_targets✓ short_circuit(skip) mark_applying✓
mark_node_started✓ aggregate_outcome(skip) finalize_release(skip) maybe_restart✓ verify_runtime✓
sync_installed_state✓ mark_node_succeeded✓ apply_per_node✓ aggregate_outcome✓ finalize_release✓
=> DAG logically complete; run header never left EXECUTING.

NOTE: mcp's apply_per_node SUCCEEDED and mcp IS installed on nuc; repository/resource
apply_per_node also SUCCEEDED but those are still `planned` (not installed) on nuc — the
per-node target selection under the quarantine/quorum gate is a SEPARATE issue from the
terminal-close hang and must not be blended into this recovery.

## Controller inflight wedge (globular-cluster-controller @ 09:59:29 EDT / 13:59:29Z)
~18 releases all logging: "ServiceRelease core@globular.io/<name>: workflow already in-flight, skipping dispatch"
Names: repository, resource, mcp, rbac, file, log, persistence, dns, authentication,
ai-executor, ai-memory, ai-router, cluster-controller, cluster-doctor, monitoring, search, workflow.
Plus: "release core@globular.io/repository: reconciling phase=RESOLVED gen=2" (never advances).
Plus: "reconcile-workflow: item terminal: type=missing_package node=681710ee pkg=repository child_status=SUCCEEDED"
(reconcile remediation thinks it succeeded, but the release row is wedged).

## nuc convergence BEFORE recovery
node_id=681710ee, status=converging, applied_hash="", desired_hash=services:283b5e48...
installed=28, planned=10.
planned (not installed): sidekick, resource, search, workflow, backup-manager, sctool,
ai-executor, repository, ai-watcher, authentication.

## executor_leases
Not captured directly: no typed read-only MCP tool exposes CQL; reading
workflow.executor_leases would require cqlsh on ryzen (not a typed action).
Lease liveness is INFERRED (proven) from the reaper-not-firing argument above.

## Code anchors (proven, this session)
- Run row created EXECUTING: executor.go:310-343 (StartRun -> server.go:828, ScyllaDB workflow_runs).
- DAG run in-process synchronous: executor.go:368 eng.Execute.
- Actor dispatch discovery-routed: executor.go:537 ResolveDialTarget; completion is synchronous
  ExecuteAction return (executor.go:587-616) — DIRECT, not a separate mesh callback.
- Per-step record: executor.go:665 onStepDone -> RecordStep.
- Terminal close in-process, error swallowed: executor.go:438-448 FinishRun (server.go:935).
- acknowledged flag = operator-only (server.go:1541 AcknowledgeRun); decoupled from completion.
- reaper (FAILED only, 15min, lease-gated): server.go:667-731.
- orphan scanner + resume (lease-staleness gated): executor_lease.go:209, executor_resume.go.
- controller async dispatch + inflight guard: release_pipeline.go:587-688.
- per-run Router keyed by run_id (HA correlation surface): actor_service.go:60,74,94-109.

## Recovery plan (owner-path, designed)
Restart globular-workflow.service on ryzen via supervisor/node-agent -> hung executor
goroutines die -> leases go stale (30s) -> orphan scanner + ResumeRun close all-terminal
runs idempotently -> controller ExecuteWorkflow errors -> inflightWorkflows clears ->
release reconciler re-dispatches. Do NOT restart controller unless inflight fails to clear.
Do NOT manual-FinishRun (would close the row but leave the controller goroutine/inflight wedge).
