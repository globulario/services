# Service Notes (quick map)

Purpose: 1–3 lines per major service so a new human/AI can jump to the right place. Ports are indicative; prefer MCP or configs for live values.

- **gateway / envoy**: Edge proxy; fronts HTTPS/HTTP traffic, routes to internal gRPC/HTTP services. Check Envoy configs for routes (MCP docs mention 8443 admin path to MCP via Envoy).
- **cluster_controller**: Control-plane brain; admits/records desired state, selects workflows. Server code: `golang/cluster_controller_server`.
- **workflow**: Workflow engine; executes workflow definitions (`golang/workflow/definitions/*.yaml`), drives node-agent RPCs, tracks status.
- **node_agent**: Per-node executor; installs packages, manages services, reports inventory/health. gRPC surface used by workflow/doctor.
- **repository**: Artifact store/metadata; build and publish via `globular pkg build/publish`. Holds service specs and package tarballs.
- **cluster_doctor**: Health/diagnostic service; produces doctor/drift reports exposed via MCP tools.
- **monitoring (prometheus/alertmanager)**: Metrics and alerting stack; Prometheus scrape, Alertmanager routes alerts.
- **ai_executor**: Routes prompts to Anthropic/Claude backend; supports multi-node consensus; exposes MCP tools for status/send_prompt.
- **ai_memory**: Stores AI memories (architecture/decision/debug). MCP tools `memory_list/get/store/...`; depends on Scylla/DB connectivity.
- **ai_router / ai_watcher**: Routing/observability helpers around AI components; watcher monitors AI health.
- **authentication**: Auth service; issues/validates tokens, ties into JWT + mTLS model.
- **rbac / resource**: Identity and authorization; resource holds accounts/groups/orgs, rbac enforces permissions.
- **repository / search / title**: Package catalog + content search and indexing.
- **file / storage / storage_backend / persistence / sql**: Data surfaces: file service, key-value storage, persistence DB access, SQL helper; often gated by tool-group allowlists.
- **backup_manager**: Manages backup jobs, retention, validation; integrates with storage/MinIO.
- **dns / discovery**: Service discovery, DNS resolution helpers.
- **xds / gateway**: Config distribution for Envoy/gateway.
- **media / torrent**: Media handling and torrent service (application-level features).
- **log / event**: Central logging/event bus surfaces.
- **ai_router / ai_executor / ai_memory**: AI stack; see MCP tools for status and memory surface.
- **mcp**: MCP HTTP/stdio server; exposes operational tools; configured via `/var/lib/globular/mcp/config.json` or `.mcp.json`.

Notes:
- Shared helpers live in `golang/interceptors`, `golang/config`, `golang/security`, `golang/installed_state`.
- Commands: `globular cli` in `golang/globularcli`; MCP tools mirror many operational tasks.
- For exact RPCs, check service `*_server` directories and `*_pb` packages.
