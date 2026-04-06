# Endpoint Resolver Migration — Remaining Call-Sites

Companion to `endpoint_resolution_policy.md`. This is a one-shot
inventory taken at the end of the connectivity-hardening session;
it enumerates every remaining service-to-service gRPC dialer in
`golang/` that has **not** yet been routed through
`config.ResolveDialTarget`, and classifies it.

The goal is not to migrate everything in this session — the scope
rule is "connective tissue only, prefer small structural fixes".
This list is the checklist for future touchups.

## Already migrated (this session)

| File | Line(s) |
|------|---------|
| `cluster_doctor/cluster_doctor_server/config.go` | `loadConfig` |
| `cluster_doctor/cluster_doctor_server/server.go` | `newServer` (controller + workflow dials), event client addr |
| `cluster_doctor/cluster_doctor_server/node_agent_dialer.go` | `dialAgent` |
| `cluster_controller/cluster_controller_server/agentclient.go` | `newAgentClient` |
| `node_agent/node_agent_server/heartbeat.go` | `ensureControllerClient` / `controllerDialOptions` |

## Remaining dialers — classified

Legend:
- **L**: safe local-only (loopback dial is intentional and acceptable)
- **M**: needs resolver migration (same class of bug as this session)
- **I**: investigate (uses skip-verify or has its own resolution story)

| File:Line | Target | Class | Notes |
|-----------|--------|-------|-------|
| `cluster_controller/release_resolver.go:423` | node services | **I** | Uses `InsecureSkipVerify: true` with comment "service certs use IP SANs". Not a loopback bug — a cert-SAN-shape problem. Fix belongs with the cert-issuance change, not here. |
| `cluster_controller/dns_reconciler.go:408` | DNS nodes | **I** | Same story — `InsecureSkipVerify: true` because DNS nodes use IP-SAN certs. Same class as release_resolver. |
| `cluster_controller/workflow_trigger.go:125` | workflow service | **M** | Plain `grpc.DialContext` with caller-supplied opts. Wrap with `ResolveDialTarget` when touched. |
| `cluster_doctor/cluster_doctor_server/collector/collector.go:219` | node-agent (per endpoint) | **M** | Has its own `agentClientTLSCreds()` with no SNI. Same bug as the pre-fix `node_agent_dialer.go`. Low-risk migration; do next. |
| `node_agent/node_agent_server/server.go:198` | cluster-controller (injects dialer fn) | **L** | Not a dial — just stores `grpc.DialContext` as the default dialer impl for `ensureControllerClient` (which IS migrated). Safe. |
| `node_agent/node_agent_server/event_publisher.go:73` | local event service | **M** | Uses legacy `grpc.Dial` + `grpc.WithTimeout` (deprecated). No ServerName set at all. Known bug shape. |
| `node_agent/node_agent_server/internal/actions/*.go` | varies | **I** | Action helpers (artifact, grpc_health_probe, tls_acme). Some are probes, some upload. Look case-by-case. |
| `ai_watcher/ai_watcher_server/server.go:511` | ai_executor | **L-ish** | Uses `globular.InternalDialOption()` — the platform helper already sets a ServerName from the local hostname. Migrate if the platform helper itself is revised, not here. |
| `ai_executor/ai_executor_server/peers.go:146` | peer executors | **M** | Own `buildPeerTLS()` with no SNI from endpoint. Wrap with `ResolveDialTarget` when touched. |
| `ai_executor/ai_executor_server/diagnoser.go:319,362` | internal services | **L-ish** | `globular.InternalDialOption()` path; see ai_watcher note. |
| `ai_executor/ai_executor_server/remediator.go:152` | internal service | **L-ish** | Same. |
| `ai_executor/ai_executor_server/action_backend.go:190` | cluster-controller | **L-ish** | Same. |
| `ai_router/ai_router_server/learning.go:39` | ai_memory | **L-ish** | Same. |
| `workflow/recorder.go:183` | workflow-service (for run events) | **M** | Plain `grpc.DialContext`; no SNI derivation. |
| `backup_manager/backup_manager_server/node_tasks.go:365` | node-agent | **M** | Plain insecure creds OR TLS via opts; no SNI path. |
| `backup_manager/backup_manager_server/hooks.go:349` | user-defined hook targets | **I** | Targets come from config; user may intentionally specify `127.0.0.1`. Migrate if the hook contract says "cert-valid hostname required". |
| `backup_manager/backup_manager_server/topology.go:125` | cluster-controller | **I** | Comment says "Cluster controller runs plain gRPC (no TLS)". Either the comment is stale (common), or this is a real insecure path. Verify and fix separately. |
| `globularcli/*.go` (doctor_remediate, services, cluster) | mixed | **I** | CLI dials — bundled with CLI-specific mTLS loading. Separate migration. |
| `globular_client/clients.go` | every service | **I** | The legacy client helper. Has its own loopback handling for cert discovery (lines 390+). Consolidating with `ResolveDialTarget` is desirable but out of scope here. |
| `globular_service/describe_health.go` | self-describe probes | **L** | Describe-style self probes. Loopback OK. |
| `mcp/clients.go` | MCP servers | **I** | MCP client — separate protocol surface. |

## How to migrate (copy-paste recipe)

```go
import "github.com/globulario/services/golang/config"

target := config.ResolveDialTarget(endpoint)
tlsCfg := &tls.Config{
    ServerName: target.ServerName,
    RootCAs:    clusterCAPool, // from config.GetTLSFile("", "", "ca.crt")
}
conn, err := grpc.NewClient(target.Address, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
```

## Non-goals

This inventory deliberately does NOT:
- touch the `InsecureSkipVerify` sites (those need cert-SAN changes, not SNI changes)
- consolidate `globular_client/clients.go` with the resolver
- rewrite `InternalDialOption()` — that helper derives SNI from the
  *local* hostname which is an independent bug to fix later.
