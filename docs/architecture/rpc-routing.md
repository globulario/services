# RPC Routing: Mesh vs Direct

Globular services communicate via gRPC. Some RPCs are routed through the Envoy mesh (via xDS-managed clusters), while others require direct node-to-node connections.

## Routing Modes

| Mode | Path | When to Use |
|------|------|-------------|
| **Mesh** | Client → Envoy → xDS cluster → Target | Default for all service RPCs. Envoy handles TLS, load balancing, and leader routing. |
| **Direct** | Client → Target (IP:port) | When the mesh isn't available (bootstrap), when the target is a specific node (node-agent actions), or when the RPC is not registered in xDS. |

## Control-Plane RPCs

### Cluster Controller (port 12000)

| RPC | Routing | Notes |
|-----|---------|-------|
| `JoinCluster` | **Mesh** | Routed to leader via xDS |
| `ListNodes` | **Mesh** | Any controller can serve (leader-forwarded) |
| `GetClusterHealth` | **Mesh** | Any controller |
| `DeployControlPlanePackage` | **Mesh** | Leader-forwarded |
| `ApplyInfrastructureRelease` | **Direct (etcd)** | NOT an RPC — written directly to etcd by `platform-upgrade` CLI. No mesh route exists. |
| `SetDesiredVersion` | **Mesh** | Leader-forwarded |
| `ReconcileNode` | **Mesh** | Leader-forwarded |

### Node Agent (port 11000)

| RPC | Routing | Notes |
|-----|---------|-------|
| `ExecutePlan` | **Direct** | Controller → specific node. Must target exact node IP:11000. |
| `GetInstalledPackages` | **Direct** | Controller → specific node |
| `GetServiceLogs` | **Direct** | CLI/MCP → specific node |
| `ControlService` | **Direct** | CLI/MCP → specific node |
| `Heartbeat` (outbound) | **Direct** | Node-agent → controller leader (resolved from etcd, not mesh) |

### Repository (port 10010)

| RPC | Routing | Notes |
|-----|---------|-------|
| `AllocateUpload` | **Mesh** | Authenticated, leader node |
| `CompletePublish` | **Mesh** | Authenticated |
| `ResolveArtifact` | **Mesh** | Read-only, any replica |
| `ListArtifacts` | **Mesh** | Read-only |
| `GetArtifactVersions` | **Mesh** | Read-only |
| Binary download | **Direct (HTTPS)** | `https://gateway:8443/repository/download/...` via gateway HTTP handler |

### Workflow (port 10004)

| RPC | Routing | Notes |
|-----|---------|-------|
| `StartRun` | **Mesh** | Controller → workflow service |
| `GetRun` | **Mesh** | Any caller |
| `ListRuns` | **Mesh** | Any caller |
| `AdvanceRun` | **Mesh** | Controller only |

### Authentication (port 10101)

| RPC | Routing | Notes |
|-----|---------|-------|
| `Authenticate` | **Mesh** or **Direct** | Direct during bootstrap (before mesh exists). `--auth` CLI flag for direct. |
| `ValidateToken` | **Mesh** | Interceptor chain |

### DNS (port 10006/10007)

| RPC | Routing | Notes |
|-----|---------|-------|
| `SetA` | **Mesh** or **Direct** | Direct during join script (`--dns host:10007`). Mesh after bootstrap. |
| `Query` | UDP port 53 | Standard DNS protocol, not gRPC |

## Bootstrap Sequence (no mesh available)

During Day-0 and early Day-1, the mesh (Envoy + xDS) is not yet running. These RPCs use direct connections:

1. `Authentication.Authenticate` — direct to bootstrap node
2. `NodeAgent.Heartbeat` — direct to controller leader IP
3. `DNS.SetA` — direct to bootstrap DNS
4. `Repository.ResolveArtifact` — direct to repository (for artifact fetch)
5. `Controller.JoinCluster` — direct via `--controller` flag

Once xDS and Envoy are running (`BootstrapEnvoyReady` phase), all subsequent RPCs go through the mesh.

## Key Rule

**If an RPC is not in the xDS service registry, it cannot be mesh-routed.** The CLI must use direct addressing (IP:port) or the RPC must be added to the xDS snapshot builder.

`ApplyInfrastructureRelease` is the most common example: it's not an RPC at all — it's a direct etcd write. The `platform-upgrade` CLI command writes InfrastructureRelease records to etcd directly because the controller doesn't expose this as a mesh-routable RPC.
