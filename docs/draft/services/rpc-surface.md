# RPC Surface Discovery (pragmatic guide)

Ports are dynamic, so discover live endpoints first (etcd configs, MCP `cluster_get_health`, `ss -lptn`), then enumerate RPCs with grpcurl. This guide gives quick targets for key services.

## How to enumerate
```bash
# 1) Find service port (example: node_agent)
# Get from etcd service configs or MCP cluster_get_health (runtime source of truth)
PORT=<lookup_from_etcd_or_mcp>

# 2) List services on that port (mTLS/JWT as needed). Use the node's IP or DNS, not 127.0.0.1.
HOST=globule-ryzen.globular.internal   # replace with the node you target
grpcurl -plaintext $HOST:$PORT list

# 3) List methods of a service
grpcurl -plaintext $HOST:$PORT list globular.node_agent.NodeAgentService

# 4) Call a method (example)
grpcurl -plaintext -d '{}' $HOST:$PORT globular.node_agent.NodeAgentService/GetInventory
```
Use `-cacert/-cert/-key` for TLS, and `-H "token: <jwt>"` if auth required.

### Resolving host/IP
- DNS service runs on port 53 (cluster DNS). Nodes are typically resolvable as `<hostname>.globular.internal` (e.g., `globule-ryzen.globular.internal`).
- Quick check: `dig globule-ryzen.globular.internal @<dns-ip>` or, if system resolver is pointed at the cluster DNS, simply `dig globule-ryzen.globular.internal`.
- If DNS is unavailable, read node IPs from MCP `cluster_list_nodes` or etcd node records, then use the IP in grpcurl.

## High-value services & what to look for

- **cluster_controller**: Admission and desired-state APIs. Look for methods to set desired services, list nodes, manage join tokens. Lives in `cluster_controllerpb`.
- **workflow**: Start/inspect workflows, list definitions/runs, get run status. Protos in `workflowpb`; engine executes YAML definitions.
- **node_agent**: Install/stop services, list installed packages, report inventory/health. Methods commonly `GetInventory`, `ListInstalledPackages`, `InstallPackage`, `ControlService` (names vary by proto).
- **node_agent (concrete methods from `proto/node_agent.proto`)**:
  - `GetInventory`, `ListInstalledPackages`, `GetInstalledPackage`, `SetInstalledPackage`
  - `ControlService` (start/stop/restart), `GetServiceLogs`, `SearchServiceLogs`
  - `ApplyPackageRelease` (fetch/install/restart health check)
  - `RunWorkflow`, `JoinCluster`, `BootstrapFirstNode`
  - Backup/restore hooks: `RunBackupProvider`, `GetBackupTaskResult`, `RunRestoreProvider`, `GetRestoreTaskResult`
  - `GetCertificateStatus`, `RotateNodeToken`
- **repository**: Package CRUD and fetch; expect `Publish`, `GetPackage`, `ListPackages`.
- **cluster_doctor**: Health/drift/analysis reports; MCP proxies some; RPCs return doctor/drift findings.
- **ai_memory**: `Store`, `Get`, `List`, `Query`, `Update`, `Delete` memories.
- **ai_executor**: Status, list peers, send prompt, list jobs.
- **gateway**: Front-door HTTP/gRPC mapping; may expose CORS/domain/config controls.
- **xds**: Envoy control-plane ADS/SDS; gRPC services `envoy.service.discovery.v3.AggregatedDiscoveryService` etc.

## Tips
- Interceptors enforce TLS + JWT; fetch a valid service token if you get `Unauthenticated`.
- For a one-shot JWT: use existing tooling (`globular` CLI) or the running service’s token generator if available.
- Keep MCP handy: many of these surfaces are exposed via MCP tools, which can be easier than raw gRPC.
- Reflection: most gRPC services have server reflection enabled; you can list services/methods without protos:
  ```bash
  grpcurl -plaintext $HOST:$PORT list
  grpcurl -plaintext $HOST:$PORT describe globular.node_agent.NodeAgentService
  ```
- Protos are in the repo under `proto/`; if reflection is off, point grpcurl to the proto files with `-proto proto/node_agent.proto -import-path proto`.
  Common protos: `proto/node_agent.proto`, `proto/cluster_controller.proto`, `proto/workflow.proto`, `proto/repository.proto`, `proto/cluster_doctor.proto`, `proto/ai_memory.proto`, etc.

## Sample grpcurl for node_agent
```bash
# assume node_agent is listening on $PORT on host $HOST (e.g., globule-ryzen.globular.internal)
grpcurl -plaintext $HOST:$PORT list globular.node_agent.NodeAgentService
grpcurl -plaintext -d '{}' $HOST:$PORT globular.node_agent.NodeAgentService/GetInventory
# with auth (replace TOKEN, add TLS certs)
grpcurl -cacert ca.pem -cert client.pem -key client.key \
  -H "token: $TOKEN" \
  -d '{}' $HOST:$PORT globular.node_agent.NodeAgentService/ListInstalledPackages
```
