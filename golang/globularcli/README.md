# Globular CLI

`globular` is a Go-based CLI that talks directly to the control-plane gRPC APIs (`ClusterControllerService` and `NodeAgentService`). It relies on the generated protobuf Go clients and exposes the following high-level command groups:

1. **cluster bootstrap/join** – drives `BootstrapFirstNode` and `JoinCluster`.
2. **cluster token/requests** – manages join tokens and approvals (`CreateJoinToken`, `ListJoinRequests`, `ApproveJoin`, `RejectJoin`).
3. **cluster nodes** – lists nodes and sets desired profiles (`ListNodes`, `SetNodeProfiles`).
4. **cluster agent** – queries inventory, applies plans, or watches agent operations.
5. **cluster plan** – reads plans from the controller and triggers `StartApply`.
6. **cluster watch** – streams operation events from either the controller or a node agent.
7. **dns domains** – manage DNS managed domains (`GetDomains`, `SetDomains`, `AddDomains`, `RemoveDomains`).

Global flags include `--controller`, `--node`, `--token`, `--ca`, `--insecure`, `--timeout`, and `--output`. The CLI always uses gRPC and honors the root flags when dialing controllers or agents, so no HTTP gateway is introduced.

Build this binary with `go build ./golang/globularcli` and place it in your PATH (`go install` also works). Use the examples from the control-plane plan to bootstrap and add nodes with direct gRPC calls.

## DNS domains quick reference

The DNS command group talks to the DNS service (default `--dns localhost:10033`; override with `--dns <host:port>`).

- Show current domains  
  `globularcli dns domains get`

- Replace domains with an explicit list  
  `globularcli dns domains set example.com example.org`

- Add domains to the existing list (de-duplicated)  
  `globularcli dns domains add sub.example.com another.org`

- Remove one or more domains  
  `globularcli dns domains remove example.org sub.example.com`

Notes:
- Domains are lowercased and trailing dots are removed.
- Commands error if no valid domains are provided.
