# Core Stores: etcd, MinIO, Scylla

Quick reference for what each store is the source of truth for, how services use it, and where to look.

## etcd — coordination & runtime registry
- **Purpose**: Cluster/service config, runtime PIDs/ports, system identity.
- **Keys** (see `golang/config/etcd_backend.go`, `config/config.go`):
  - `/globular/system/config` — global system config (`PortsRange`, etc.).
  - `/globular/auth/root` — auth root.
  - Service configs (Id, Port, Proxy, etc.) via `GetServicesConfigurations`.
  - Runtime state per service via `PutRuntime` / `GetRuntime` (Process, ProxyProcess, State).
- **Consumers**: cluster_controller, workflow, node_agent, process manager, port allocator.
- **Port allocator**: `config.NewDefaultPortAllocator()` loads `PortsRange` (default `10000-20000`), preloads used ports from service configs, then hands out free ports (`Next`, `NextPair`).
- **Ops tips**: To find live ports/PIDs, read configs from etcd or use MCP `cluster_get_health`; process reconciles runtime from etcd before killing/restarting.
- **CLI snippets** (set ETCDCTL_* env for TLS):
  - List services: `ETCDCTL_API=3 etcdctl get --prefix /globular/services/`
  - Show system config: `etcdctl get /globular/system/config`
  - Describe runtime for service X: `etcdctl get /globular/runtime/<service-id>`

## MinIO — artifact/config bucket
- **Purpose**: Single source of truth for workflow definitions and shared artifacts.
- **Paths**: Workflow YAMLs stored at `globular-config/workflows/{name}.yaml` (per `docs/centralized-workflow-execution.md`).
- **Producers**: CI/packaging publishes `golang/workflow/definitions/*.yaml` into MinIO at release.
- **Consumers**: WorkflowService loads definitions from MinIO (not embedded); other services (shared_index, backup) may pull/push data to MinIO buckets.
- **Ops tips**: Treat MinIO as authoritative for workflow definitions in production; if workflows look stale, verify the MinIO object and bucket creds in service config.
- **Publish workflow definitions**:
  1. Build/publish step (CI or manual): copy `golang/workflow/definitions/*.yaml` to `s3://globular-config/workflows/` (bucket/prefix from config).
     Example: `mc cp golang/workflow/definitions/*.yaml minio/globular-config/workflows/`
  2. WorkflowService should pick up from MinIO; if not, check its MinIO config and creds.
- **List workflow objects**: `mc ls minio/globular-config/workflows/`

## ScyllaDB — AI memory backend
- **Purpose**: Durable storage for `ai_memory` (architecture/decision/debug notes).
- **Consumers**: `AiMemoryService` (gRPC), MCP tools `memory_*`.
- **Dependencies**: Scylla connectivity and TLS perms; `process.ensureEtcdAndScyllaPerms` adjusts `/etc/ssl` ownership to group `scylla` so certs are readable.
- **Ops tips**: If MCP `memory_get` returns “Service unavailable”, check `globular-ai-memory.service` status and Scylla reachability on the node that succeeded in doctor report.
- **Quick check via cqlsh** (adapt creds/host): `cqlsh <scylla-host> 9042 -e "DESC KEYSPACE ai_memory;"` or query `SELECT id,title,type FROM ai_memory.memories LIMIT 5;`

## Finding live endpoints/ports
- Ports are dynamic; use:
  - MCP `cluster_get_health` for per-node listeners.
  - etcd service configs (`GetServicesConfigurations`) for assigned Port/Proxy.
  - `ss -lptn` on the node for ground truth.

## Why it matters
- **etcd** is the live registry — don’t hardcode ports or states; read from it.
- **MinIO** is the production workflow/config store — publish there, load from there.
- **Scylla** keeps AI context — keep it healthy to retain operational memory and AI guidance.
