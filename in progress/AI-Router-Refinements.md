# AI Router — Refinements

Addresses 6 points from review feedback.

---

## 1. Execution Model: Hybrid (Separate Service + Embedded Query)

**Decision: Separate `ai_router` gRPC service with an embedded policy cache
that the xDS watcher reads via a lightweight local client.**

Why not purely embedded in xDS watcher:
- The xDS watcher lives in the Globular repo (Go binary), not services repo
- Mixing Prometheus querying + ai_memory calls + scoring into the watcher
  bloats a critical infrastructure component
- Can't update routing logic without restarting xDS

Why not purely remote gRPC call per cycle:
- 5-second cycle with a network roundtrip adds fragility
- If ai_router is down, xDS watcher stalls

**Hybrid model:**

```
ai_router service (separate process, port 10220)
  ├─ runs its own 5-second scoring loop
  ├─ collects metrics, events, anomalies
  ├─ computes RoutingPolicy
  ├─ stores latest policy in memory (atomic pointer)
  └─ serves GetRoutingPolicy() RPC (returns cached policy, <1ms)

xDS watcher (in Globular binary)
  ├─ every 5s: calls ai_router.GetRoutingPolicy() via gRPC
  ├─ if success: merges into xDS snapshot
  ├─ if error/timeout (50ms): uses previous policy or nil (neutral)
  └─ never blocks on ai_router
```

The ai_router does the heavy lifting (Prometheus queries, scoring) on its own
schedule. The xDS watcher just reads the latest cached result. Decoupled but
fast.

**Phase 0 implementation:**
- ai_router service starts, serves GetRoutingPolicy() returning nil
- xDS watcher adds a single `getExternalRoutingPolicy()` call in its snapshot
  build path, with a 50ms timeout and nil fallback

---

## 2. Pre-Phase-1 Metrics Validation

### Available Today (confirmed)

| Metric | Source | Labels | Granularity | Scrape |
|--------|--------|--------|-------------|--------|
| `grpc_server_handled_total` | grpc-prometheus (every service) | `grpc_code, grpc_method, grpc_service, grpc_type` | per-method per-service | 5s |
| `grpc_server_handling_seconds_bucket` | grpc-prometheus (every service) | same + 12 buckets | per-method per-service | 5s |
| `node_cpu_seconds_total` | node_exporter (port 9100) | `cpu, mode` | per-core per-node | 5s |
| `node_memory_MemAvailable_bytes` | node_exporter | none | per-node | 5s |
| `envoy_upstream_rq` | Envoy (port 9901) | `response_code, envoy_cluster_name` | per-cluster | 1s |
| `envoy_upstream_cx_connect_fail` | Envoy | `envoy_cluster_name` | per-cluster | 1s |

### Label Granularity & Endpoint Mapping

**Problem:** `grpc_server_handled_total` is per-service-INSTANCE, not per-endpoint
from the client's perspective. Each service scrapes its own `/metrics`. To map:

```
endpoint "10.0.0.63:10010" (event service)
    → Prometheus job target: "127.0.0.1:10011" (proxy port)
    → instance label: "127.0.0.1:10011"
```

In a single-node cluster: one instance per service, so instance = endpoint.
In multi-node: each node has its own Prometheus target for the same service.

**Mapping strategy:**
```
For each service in etcd config:
  service_name → prometheus job name (file_sd target file)
  → query with {instance="127.0.0.1:{proxy_port}"}
  → maps to endpoint {address}:{grpc_port}
```

The ai_router builds this mapping from etcd service configs (same source
as the xDS watcher). Each service config has both `Port` (gRPC) and
`Proxy` (metrics). The router queries Prometheus by `{instance}` label
and maps back to the endpoint address.

### Missing Instrumentation

| Need | Status | Fix |
|------|--------|-----|
| Per-endpoint latency | ✅ Available via `grpc_server_handling_seconds` per instance | Query per instance |
| Per-endpoint error rate | ✅ Available via `grpc_server_handled_total{grpc_code!="OK"}` | Query per instance |
| Per-endpoint RPS | ✅ Available via `rate(grpc_server_handled_total[1m])` | Query per instance |
| CPU per service | ⚠️ `globular_services_cpu_usage_counter` exists but requires StartProcessMonitoring() | Use `node_cpu_seconds_total` instead (node-level) |
| Memory per service | ⚠️ Same as CPU | Use `process_resident_memory_bytes` (Go runtime) |
| Active gRPC streams | ❌ Not tracked | Add `grpc_server_connections_active` gauge — but grpc-prometheus doesn't expose this by default |
| Envoy per-endpoint stats | ✅ `envoy_upstream_rq` with `envoy_cluster_name` label | Map cluster name → service |

**Phase 1 viable:** Yes. `grpc_server_handled_total` and `grpc_server_handling_seconds`
give us latency, error rate, and RPS per service instance. Node-level CPU/memory
from node_exporter. Sufficient for Phase 1 scoring.

**Phase 4 gap:** Active stream count per endpoint not available. Options:
- Add custom gauge in event_client OnEvent (count active streams)
- Use Envoy's `envoy_cluster_upstream_cx_active` (active connections, not streams)
- Defer until needed

---

## 3. Service Classification Granularity

**Decision: Per-service, not per-route or per-RPC method.**

Rationale:
- Routes and methods are internal to Envoy configuration; the AI Router
  operates at the service/endpoint level
- Mixed services (event has both unary Publish and streaming OnEvent) get
  classified by their **dominant pattern**
- The xDS watcher already groups endpoints by service (one cluster per service)

### Classification Rules

```go
// Classified by service name, stored in config (overridable via etcd).
var defaultClassification = map[string]ServiceClass{
    // stream_heavy: services with long-lived server-streaming RPCs
    "event.EventService":          ClassStreamHeavy,
    "log.LogService":              ClassStreamHeavy,

    // control_plane: infrastructure services that must always be reachable
    "clustercontroller.ClusterControllerService": ClassControlPlane,
    "nodeagent.NodeAgentService":                 ClassControlPlane,
    "discovery.DiscoveryService":                 ClassControlPlane,

    // deployment_sensitive: services affected by rolling updates
    "repository.PackageRepository":  ClassDeploymentSensitive,

    // stateless_unary: everything else (default)
    // authentication, rbac, resource, file, dns, search, media, etc.
}
```

### Mixed Services Handling

Event service has both `Publish` (unary) and `OnEvent` (server-streaming).
Classified as `stream_heavy` because the streaming behavior drives drain strategy.

The scoring model applies to the **endpoint** (which serves both unary and streaming).
You can't weight one endpoint differently for unary vs streaming — Envoy routes
to the cluster, not per-method within a cluster.

If fine-grained per-method control is ever needed: create separate Envoy clusters
for streaming vs unary methods of the same service. But that's a major xDS change
and not needed for Phase 1-4.

---

## 4. Precedence Rules (Conflict Resolution)

When multiple mechanisms act on the same endpoint, this is the order:

```
Priority (highest to lowest):

1. Manual Override (CLI)
   - Always wins. Bypasses all scoring.
   - Set via: globular ai routing override --endpoint X --weight Y
   - Persisted in etcd until explicitly cleared.

2. Drain List
   - If endpoint is draining, weight = 0 regardless of score.
   - Drain overrides all computed weights.
   - Other mechanisms (circuit breaker, outlier) still apply
     to protect remaining endpoints.

3. Outlier Detection (Envoy-native)
   - Envoy ejects endpoints independently of AI Router weights.
   - AI Router sets thresholds; Envoy enforces per-request.
   - If Envoy ejects an endpoint, traffic shifts even if
     AI Router gave it a high weight.
   - AI Router observes ejections (via Envoy metrics) and
     adjusts weights to match reality.

4. Circuit Breaker Limits (Envoy-native)
   - Per-cluster connection/request caps.
   - AI Router tightens limits under stress, loosens when healthy.
   - Circuit breakers are ADDITIVE to weights:
     high weight + tight circuit breaker = "we want traffic here
     but not too much"

5. Computed Weights (AI Router scoring)
   - Normal scoring output.
   - Applied after drain and override checks.

6. Retry Policy
   - Orthogonal to weights: controls what happens AFTER a failure.
   - AI Router adjusts retry count based on system stress.
   - During cascade: retries = 0 (prevent amplification).
   - Normal: retries = 2 for unary, 0 for streaming.
```

### Interaction Matrix

```
                    | Weight | Circuit | Outlier | Retry | Drain |
--------------------|--------|---------|---------|-------|-------|
Weight reduced       |   —    | kept    | kept    | kept  |  N/A  |
Circuit tightened    | kept   |    —    | kept    | kept  | kept  |
Outlier ejects       | kept*  | kept    |    —    | N/A   | kept  |
Retry reduced        | kept   | kept    | kept    |   —   | kept  |
Drain activated      | → 0    | kept    | kept    | kept  |   —   |

*When Envoy ejects, AI Router reduces weight on next cycle to align.
```

### Cascading Failure Protocol

When `alert.error.spike` fires:
1. Reduce retries to 0 (stop amplification) — IMMEDIATE
2. Tighten circuit breakers on affected cluster — SAME CYCLE
3. Adjust weights based on per-endpoint error rate — NEXT CYCLE
4. If error rate persists after 3 cycles → outlier detection ejects — ENVOY-NATIVE

Order matters: stop retries first (stop the bleeding), then tighten limits
(contain the damage), then rebalance (shift traffic).

---

## 5. Control-Loop Time Budget & Fallback

### Time Budget (per cycle)

```
Total budget: 4 seconds (out of 5-second xDS poll interval)

Breakdown:
  Metrics collection (Prometheus HTTP API):  2000ms max
    - 3 batch queries (latency, errors, RPS):  500ms each
    - node_exporter query:                      500ms
  Anomaly collection (ai_watcher gRPC):       200ms max
  Memory query (ai_memory gRPC):              200ms max
  Scoring computation:                         50ms max
  Safety validation:                           10ms max
  Policy building:                             10ms max
  Observer recording:                         100ms max
  ─────────────────────────────────────────────
  Total:                                     ~2570ms typical
  Buffer:                                    ~1430ms
```

### Timeout Strategy

```go
func (e *Engine) ComputePolicy(ctx context.Context) (*RoutingPolicy, error) {
    ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
    defer cancel()

    // Metrics: 2s timeout, degraded mode if slow
    metrics, metricsErr := e.collector.Gather(ctx, 2*time.Second)

    // Anomalies: 200ms timeout, optional
    anomalies, _ := e.anomalySource.Get(ctx, 200*time.Millisecond)

    // History: 200ms timeout, optional
    history, _ := e.memorySource.Get(ctx, 200*time.Millisecond)

    if metricsErr != nil {
        // Cannot score without metrics — return last-known-good
        return e.lastGoodPolicy, nil
    }

    // Score, validate, build...
}
```

### Fallback Rules

```
Scenario                          → Behavior
──────────────────────────────────────────────────────
Prometheus unreachable            → last-known-good policy (up to 60s)
                                    then neutral (nil)
ai_watcher unreachable            → score without anomaly component
ai_memory unreachable             → score without historical component
Scoring panics                    → recover, return last-known-good
Policy computation > 4s           → return last-known-good
All metrics stale (>30s old)      → neutral policy + alert event
xDS watcher GetRoutingPolicy > 50ms → use cached policy from last call
ai_router service down            → xDS watcher gets connection error,
                                    uses nil (neutral), logs warning
```

### Last-Known-Good vs Nil

```
last-known-good: reuse the most recent successfully computed policy.
  - Used when: transient metric collection failure, timeout
  - Max age: 60 seconds (after that, switch to nil)
  - Why: short outage shouldn't cause routing to reset

nil (neutral): no routing opinion, xDS watcher uses default behavior.
  - Used when: ai_router down, metrics stale > 60s, first boot
  - Effect: all weights equal, no circuit breaker overrides
  - Why: unknown is safer than stale
```

---

## 6. Multi-Granularity Signal Merging

### The Problem

Signals come at three different granularities:
- **Node-level:** CPU, memory (from node_exporter)
- **Endpoint-level:** latency, error rate, RPS (from grpc-prometheus per instance)
- **Service-level:** anomaly score, historical reliability (from ai_watcher/ai_memory)

An "endpoint" is a (node, service) pair: node 10.0.0.63 running event on port 10010.

### Formal Scoring Input Model

```go
type ScoringInput struct {
    // Endpoint identity
    Service  string  // "event.EventService"
    Endpoint string  // "10.0.0.63:10010"
    NodeID   string  // "node-abc123"

    // Node-level signals (shared across all endpoints on this node)
    Node NodeSignals

    // Endpoint-level signals (specific to this service instance)
    Endpoint EndpointSignals

    // Service-level signals (shared across all endpoints of this service)
    Service ServiceSignals
}

type NodeSignals struct {
    CPUUsage      float64  // 0-1, from node_cpu_seconds_total
    MemoryUsage   float64  // 0-1, from node_memory_*
    Stale         bool
}

type EndpointSignals struct {
    LatencyP99    time.Duration  // from grpc_server_handling_seconds
    LatencyTrend  float64        // slope of p99 over last 5 cycles
    ErrorRate     float64        // 0-1, from grpc_server_handled_total
    RPS           float64        // from rate(grpc_server_handled_total)
    Stale         bool
}

type ServiceSignals struct {
    AnomalyScore       float64  // 0-1, from ai_watcher
    HistoricalReliability float64  // 0-1, from ai_memory
    ActiveIncidents    int
    ClassConfig        ServiceClassConfig
}
```

### Merging Strategy

```
score(endpoint) =
    classWeights.CPU         * node.CPUUsage +
    classWeights.LatencyP99  * normalize(endpoint.LatencyP99) +
    classWeights.ErrorRate   * endpoint.ErrorRate +
    classWeights.Anomaly     * service.AnomalyScore +
    classWeights.Reliability * (1 - service.HistoricalReliability)
```

**Node-level signals** (CPU, memory) apply equally to all endpoints on that node.
If a node is at 90% CPU, all its services score higher (worse), even if one
service has low latency. This is correct — a CPU-saturated node will degrade
all services eventually.

**Endpoint-level signals** (latency, errors) are specific. One service on a
node can be healthy while another is failing. These have the highest weight
in the scoring model (0.20 + 0.25 = 0.45) because they're the most direct
signal.

**Service-level signals** (anomaly, reliability) apply to all endpoints of
that service. If ai_watcher flags "event service is under DoS", all event
endpoints get a higher anomaly score. But the endpoint-level metrics will
differentiate — the endpoint actually receiving the DoS traffic will also
have high latency/errors, scoring even worse.

### Stale Signal Rules

```
If node signals stale:
  → use node.CPUUsage = 0.5 (neutral assumption)
  → flag in decision rationale

If endpoint signals stale:
  → use endpoint.LatencyP99 = median of other endpoints
  → use endpoint.ErrorRate = 0 (benefit of doubt)
  → flag in decision rationale

If service signals stale:
  → use service.AnomalyScore = 0 (no anomaly assumed)
  → use service.HistoricalReliability = 0.5 (neutral)
  → flag in decision rationale

If ALL signals stale:
  → neutral policy (nil)
  → publish alert event
```

### Multi-Node Scenario

Same service on 3 nodes:
```
event@node-1: score = 0.3 (healthy)
event@node-2: score = 0.7 (stressed — high CPU + rising latency)
event@node-3: score = 0.4 (normal)
```

Weights (inverted + normalized):
```
event@node-1: weight = 70  (0.7 * 100)
event@node-2: weight = 30  (0.3 * 100)
event@node-3: weight = 60  (0.6 * 100)
```

New unary requests: 70 : 30 : 60 distribution.
Existing streams: stay where they are.

---

## Phase 0 / 0.5 Execution Plan

### Phase 0: Skeleton + Wiring (1-2 days)

```
1. Proto definition (proto/ai_router.proto):
   - GetRoutingPolicy RPC
   - GetStatus RPC
   - RoutingPolicy message (weights, overrides, confidence, reasons)
   - ServiceClass enum

2. Service skeleton (golang/ai_router/ai_router_server/):
   - server.go: lifecycle, gRPC registration
   - config.go: port 10220, service config
   - handlers.go: GetRoutingPolicy returns nil, GetStatus returns "neutral"
   - default_config.go: service classification map

3. Build pipeline:
   - Add to build-all-packages.sh
   - Systemd unit
   - Package spec

4. xDS watcher integration (Globular repo):
   - Add getExternalRoutingPolicy() call in snapshot build
   - 50ms timeout, nil fallback
   - Connect to ai_router via gRPC client
```

### Phase 0.5: Metrics Validation (1 day)

```
5. Prometheus query client (ai_router_server/collector.go):
   - Query monitoring.MonitoringService.Query() gRPC
     (avoids direct Prometheus HTTP, uses existing gRPC client pattern)
   - 3 queries: latency p99, error rate, RPS
   - 1 query: node CPU
   - Map instance label → endpoint address
   - Log all results for validation

6. Dry-run scoring (ai_router_server/scorer.go):
   - Score all endpoints
   - Log scores + rationale
   - Do NOT apply (return nil policy still)
   - Compare: "would have changed weight from X to Y"

7. Validation:
   - Verify Prometheus queries return valid data
   - Verify instance→endpoint mapping is correct
   - Verify scoring produces sensible numbers
   - Run for 24h in dry-run mode before enabling
```

Phase 1 proceeds only after Phase 0.5 validation confirms metrics are
accurate and scoring is sensible.
