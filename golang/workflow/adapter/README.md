# Node-Agent Actor Adapter + Step Result Callback Contract

This package defines the boundary between the workflow engine and the node-agent.

Contents:
- `docs/node_agent_adapter_definition.md`
- `docs/step_result_rpc_contract.md`
- `proto/workflow_runtime.proto`
- `go/adapter/interfaces.go`
- `go/adapter/models.go`
- `examples/step_result_event.json`

Purpose:
- make node-agent an execution adapter, not an orchestrator
- make workflow-service the owner of run/step state
- define a clear callback contract for step progress, completion, failure, heartbeat, and cancellation acknowledgement

Recent behavior changes (April 2026)
- Unknown actions (e.g., `advance_infra_joins`) are handled as one-time failures: workflow marks the step error and stops retry storms.
- Command-only packages (restic, rclone, ffmpeg, sctool, mc) skip runtime validation to let reconciles proceed.
- The workflow executor registers a default controller router so controller-driven actions don’t fail when a custom router is absent.
- Metrics are exported on the workflow HTTP port; ensure Prometheus scrapes the workflow job.

Core rule:
- **workflow-service decides**
- **node-agent executes**
- **callbacks report facts, never policy**
