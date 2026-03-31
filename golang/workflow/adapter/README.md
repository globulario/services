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

Core rule:
- **workflow-service decides**
- **node-agent executes**
- **callbacks report facts, never policy**
