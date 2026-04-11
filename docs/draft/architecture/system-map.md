# System Map (Inside-Out)

Purpose: quick orientation for humans and AIs. What lives where, how it talks, and where to look first when operating or extending Globular.

## Fast Path
- MCP endpoint: `http://127.0.0.1:10260/mcp` (POST /mcp JSON-RPC; GET /health). Session header: `Mcp-Session-Id`. Optional auth: `token` header (JWT). Config in `.mcp.json` / `~/.claude/.mcp.json`.
- Control-plane entry: `cluster_controller` + `workflow` services; workflows drive all change.
- Execution agents: `node_agent` on every node; receives workflow step RPCs.
- Artifact source: `repository` service; packages built via `globular pkg build/publish`.
- Observability & AI: `cluster_doctor`, `monitoring` (Prometheus/Alertmanager), `ai_executor`, `ai_memory`, MCP tools.

## Perspectives
- Developer (building features): Workflows are the only mutation path. Services are packaged/published to repository; desired state set via control-plane APIs/CLI; node_agent executes.
- AI developer/operator: Use MCP tools (doctor, drift, convergence detail, log ring, memory_*). AI memory holds prior incidents/decisions; AI executor routes prompts.
- DevOps/SRE: Watch cluster health via MCP doctor/drift, Prometheus, node-agent inventory; control rollout via workflows; repository integrity is source of truth; TLS+JWT auth on gRPC/HTTP surfaces.
- Implementation: Go monorepo (`golang/`), per-service servers with gRPC, shared interceptors for auth/logging, MCP server for operational surface.

## Core Flows
1) Change request -> Control plane admission -> Workflow start -> Steps dispatched to node_agent -> repository artifacts pulled -> services configured -> status recorded -> doctor/drift report surface outcome.
2) Diagnosis -> MCP doctor/drift/convergence/log ring -> targeted workflow or manual fix -> new workflow to remediate.

## Auth & Identity (from code + memory)
- Service/service tokens: Ed25519-signed JWTs (issuer MAC, kid); `security/jwt.go`.
- mTLS also present (service.crt/key); many RPCs expect token metadata.
- MCP HTTP allows optional `token` header for caller identity in audit.

## Data Planes & Ports (common)
- MCP HTTP: 10260 (local loopback).
- gRPC services typically via Envoy/gateway; cluster services expose 10.0.0.x ports (see service configs). Repository/gateway often on 443/8443; Prometheus/Alertmanager present.

## State Model (operational)
- Desired: control-plane/workflow targets.
- Installed: node_agent recorded packages.
- Applied/Running: service health reports (doctor/convergence).
- Observed: metrics/events/log ring.

## Where to Look First
- For cluster status: `cluster_get_health`, `cluster_get_doctor_report` (MCP).
- For node specifics: `nodeagent_get_inventory`, `cluster_get_convergence_detail`.
- For auth issues: AI memory entries tagged `auth`, `jwt`, `mtls`; interceptors in `golang/interceptors`.
- For workflows: `golang/workflow` service + definitions in `golang/workflow/definitions`.
- For packages: `golang/repository`, specs under `/var/lib/globular/specs/` (per docs), build scripts `generateCode.sh`, `build-all-packages.sh`.

## Commitments / Non-goals
- No hidden reconciliation loops; workflows are explicit.
- Control plane never executes node actions directly.
- Repository is source of deployable truth; node_agent enforces, not control plane.

## TODO / Open Questions
- Confirm current Envoy routes/ports in deployment manifests.
- Add short “how to bootstrap a new node” walk-through to `docs/draft/workflows`.
- Surface per-service health endpoints/ports in a table (see service-notes).
