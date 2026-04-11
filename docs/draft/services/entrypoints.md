# Service Entrypoints (binaries, config, RPC surfaces)

Quick reference to find the runnable binary, its default config path, and what APIs it serves. Use this to jump from docs to code fast.

Port allocation: services grab ports from the shared allocator (`config.PortAllocator`) using the global `PortsRange` (default `10000-20000`, from system config). Ports can change between runs; some services claim contiguous pairs (Port/Proxy). For live values, check service configs in etcd, MCP (`cluster_get_health`), or `ss -lptn` on the node.

Live port lookup (pick one):
- MCP: `cluster_get_health` (shows service listeners per node).
- etcd configs: `GetServicesConfigurations` (or MCP tools that surface it).
- Host: `ss -lptn` filtered by service PID/name.

| Service | Main file / binary | Default config | Protocols / Ports (dynamic) | Notes |
| --- | --- | --- | --- | --- |
| cluster-controller | `golang/cluster_controller/cluster_controller_server/main.go` → `globular-cluster-controller` | `/var/lib/globular/cluster-controller/config.json` | gRPC with TLS on `:<cfg.Port>` (etcd backing) | Admits intent, resolves plans, seeds join tokens, uses MinIO for workflow defs. Uses recovery interceptors; health/describe/version flags. |
| workflow | (server in `golang/workflow`, driven by workflow definitions in `golang/workflow/definitions/*.yaml`) | see service config (via cluster-controller) | gRPC | Executes workflows; orchestrates node_agent RPCs; states AVAILABLE/DEGRADED/FAILED. |
| node-agent | `golang/node_agent/node_agent_server/main.go` → `globular-node-agent` | `/var/lib/globular/node-agent/config.json` | gRPC on node (TLS); talks to repository and systemd | Installs packages, manages units, reports inventory/health; idempotent per step. |
| repository | `golang/repository` server (entry under `cmd`/service packaging) | `/var/lib/globular/repository/config.json` | gRPC/HTTP (behind gateway) | Stores package artifacts/metadata; used by node-agent/workflow. |
| cluster-doctor | `golang/cluster_doctor/cluster_doctor_server/main.go` → `globular-cluster-doctor` | `/var/lib/globular/cluster-doctor/config.json` | gRPC | Produces doctor + drift reports consumed by MCP tools. |
| mcp | `golang/mcp/main.go` → `globular-mcp-server` | `/var/lib/globular/mcp/config.json` (or `~/.config/globular/mcp.json`) | HTTP JSON-RPC on `:10260` by default; stdio mode optional | Tools surface for AI/ops; POST `/mcp`, GET `/health`; SSE optional; session header `Mcp-Session-Id`. |
| ai-memory | `golang/ai_memory` service | `/var/lib/globular/ai-memory/config.json` | gRPC (Scylla backend) | Stores/retrieves AI memories; MCP tools `memory_*` depend on this. |
| ai-executor | `golang/ai_executor/ai_executor_server` | `/var/lib/globular/ai-executor/config.json` | gRPC | Routes prompts to Anthropic/Claude; supports peer consensus; MCP tools `ai_executor_*`. |
| ai-router / ai-watcher | `golang/ai_router`, `golang/ai_watcher` | service configs | gRPC/HTTP | Routing + health of AI stack. |
| gateway / envoy | packaged configs (Envoy) | `/etc/envoy/envoy.yaml` (varies) | 443/8443 etc. | Edge proxy; routes MCP `/mcp` and other services. |
| xds | `golang/xds` | service config | gRPC | Drives Envoy config distribution. |
| repository-facing CLI | `golang/globularcli/main.go` → `globular` | `~/.config/globular/*.json` | CLI | Commands wrap cluster control, package build/publish, dev helpers. |
| tools (schema) | `golang/tools/schema-lint/main.go`, `golang/tools/schema-extractor/main.go` | n/a | CLI | Lint/extract schema annotations from code. |
| upgrader | `golang/cmd/globular-upgrader/main.go` | CLI flags | CLI | Upgrade helper binary. |

Notes:
- Most services use shared interceptors (`golang/interceptors`) for auth/logging/recovery.
- Auth: TLS + JWT (Ed25519) via `golang/security/jwt.go`; mTLS certs shared across services.
- Ports: verify via service config or `service_catalog.yaml`; MCP lists live listeners via `ss -lptn`.
