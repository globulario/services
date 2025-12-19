# Globular CLI

`globular` is a Go-based CLI that talks directly to the control-plane gRPC APIs (`ClusterControllerService` and `NodeAgentService`). It relies on the generated protobuf Go clients and exposes the following high-level command groups:

1. **cluster bootstrap/join** – drives `BootstrapFirstNode` and `JoinCluster`.
2. **cluster token/requests** – manages join tokens and approvals (`CreateJoinToken`, `ListJoinRequests`, `ApproveJoin`, `RejectJoin`).
3. **cluster nodes** – lists nodes and sets desired profiles (`ListNodes`, `SetNodeProfiles`).
4. **cluster agent** – queries inventory, applies plans, or watches agent operations.
5. **cluster plan** – reads plans from the controller and triggers `StartApply`.
6. **cluster watch** – streams operation events from either the controller or a node agent.

Global flags include `--controller`, `--node`, `--token`, `--ca`, `--insecure`, `--timeout`, and `--output`. The CLI always uses gRPC and honors the root flags when dialing controllers or agents, so no HTTP gateway is introduced.

Build this binary with `go build ./golang/globularcli` and place it in your PATH (`go install` also works). Use the examples from the control-plane plan to bootstrap and add nodes with direct gRPC calls.
