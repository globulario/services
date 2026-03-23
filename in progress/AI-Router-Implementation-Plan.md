# AI Router — Implementation Plan

Based on `globular_ai_router_spec.md` and real Globular architecture analysis.

---

## 1. Detailed Architecture

### Package Structure

```
golang/ai_router/
├── ai_routerpb/              # Generated protobuf (gRPC API)
├── ai_router_server/
│   ├── server.go             # Service boilerplate, lifecycle
│   ├── config.go             # Config struct, validation
│   ├── engine.go             # Core decision engine (scoring + policy)
│   ├── collector.go          # Input collection (metrics, events, state)
│   ├── scorer.go             # Per-endpoint scoring model
│   ├── safety.go             # Invariant enforcement, anti-flapping
│   ├── policy.go             # RoutingPolicy output builder
│   ├── service_class.go      # Service classification + per-class behavior
│   ├── observer.go           # Feedback loop (record outcome, compare)
│   ├── handlers.go           # gRPC handlers (status, evaluate, override)
│   └── default_config.go     # Default service classes + scoring weights
└── ai_router_client/
    └── ai_router_client.go   # Client for CLI + other services
```

### Interfaces

```go
// PolicyProvider is called by the xDS watcher during snapshot build.
// This is the primary integration point — a function call, not a network hop.
type PolicyProvider interface {
    // GetRoutingPolicy returns the current routing policy for all services.
    // Called every xDS poll cycle (5 seconds).
    // Returns nil if the router has no opinion (neutral — preserve current behavior).
    GetRoutingPolicy(ctx context.Context, services []ServiceSnapshot) (*RoutingPolicy, error)
}

// ServiceSnapshot is what the xDS watcher provides to the router.
type ServiceSnapshot struct {
    Name       string              // e.g. "event.EventService"
    Class      ServiceClass        // stateless_unary, stream_heavy, etc.
    Endpoints  []EndpointSnapshot  // current endpoints from discovery
}

type EndpointSnapshot struct {
    Address string  // "10.0.0.63"
    Port    int     // 10010
    NodeID  string  // node identity for correlation
}

// MetricsCollector gathers signals for scoring.
type MetricsCollector interface {
    CollectEndpointMetrics(ctx context.Context, endpoints []EndpointSnapshot) (map[string]*EndpointMetrics, error)
}

// AnomalySource provides real-time anomaly signals from ai_watcher.
type AnomalySource interface {
    GetActiveIncidents() []IncidentSummary
    GetAnomalyScore(endpoint string) float64
}
```

### Data Flow

```
Every 5 seconds (xDS watcher poll cycle):

  xDS Watcher
      │
      ├─ builds ServiceSnapshot[] from etcd + DNS
      │
      ├─ calls ai_router.GetRoutingPolicy(snapshots)
      │       │
      │       ├─ collector.Gather()
      │       │     ├── query Prometheus (CPU, latency, errors, RPS)
      │       │     ├── query ai_watcher (active incidents, anomaly scores)
      │       │     └── query ai_memory (historical reliability)
      │       │
      │       ├─ scorer.Score(endpoint, metrics, anomalies, history)
      │       │     └── returns score per endpoint per service
      │       │
      │       ├─ safety.Validate(proposed, current)
      │       │     ├── enforce max delta ±20%
      │       │     ├── enforce min endpoints
      │       │     ├── enforce cooldown periods
      │       │     └── enforce service-class constraints
      │       │
      │       ├─ policy.Build(scores, overrides, safety)
      │       │     └── returns RoutingPolicy
      │       │
      │       └─ observer.Record(decision, rationale)
      │             └── store in ai_memory for learning
      │
      ├─ merges RoutingPolicy into xDS snapshot
      │     ├── EDS: endpoint weights
      │     ├── CDS: circuit breaker overrides, outlier detection
      │     └── RDS: retry policies (unary only)
      │
      └─ pushes snapshot to Envoy via xDS server
```

---

## 2. Phase-by-Phase Plan

### Phase 0 — Interface & Neutral Wiring

**Goal:** Wire the integration point without changing behavior.

**Deliverables:**
- Proto definition (`proto/ai_router.proto`)
- `PolicyProvider` interface in a shared package
- xDS watcher calls `GetRoutingPolicy()` — default returns nil (no opinion)
- AI Router service skeleton (server.go, config.go, handlers.go)
- `GetStatus` RPC returns service health + "neutral mode"
- Service registered in build pipeline + systemd unit

**Dependencies:** xDS watcher (in Globular repo)

**Acceptance:**
- All existing routing unchanged
- xDS watcher builds snapshots identically to before
- AI Router service starts, responds to health checks
- `globular ai routing status` returns "neutral"

---

### Phase 1 — Metrics-Based Weighting (Unary Services)

**Goal:** Shift traffic away from overloaded endpoints based on real metrics.

**Deliverables:**
- `collector.go`: Prometheus query client (PromQL via HTTP API)
  - `grpc_server_handling_seconds_bucket` → p99 latency
  - `grpc_server_handled_total` → RPS + error rate
  - `process_resident_memory_bytes` → memory pressure
  - `node_cpu_seconds_total` → CPU usage (from node_exporter)
- `scorer.go`: deterministic scoring model
  - Configurable weights per service class
  - Normalization to 0.0-1.0 range
- `policy.go`: convert scores → EDS endpoint weights
- `safety.go`: ±20% max delta, minimum endpoint count
- `service_class.go`: classify services (start with `stateless_unary` only)
- `observer.go`: log decisions with rationale, publish `routing.weights.changed` event

**Metrics freshness:** reject metrics older than 30 seconds (stale = neutral).

**Dependencies:** Prometheus running, node_exporter running, services exporting gRPC metrics

**Acceptance:**
- Under normal load: weights approximately equal (no unnecessary churn)
- Under synthetic load on one endpoint: weight decreases within 2 cycles (10s)
- No weight oscillation (smoothing prevents flapping)
- No endpoint fully removed unless explicitly drained
- Decision log shows scores + rationale for every change

---

### Phase 2 — Stability Controls

**Goal:** Prevent cascade failures via circuit breaker tuning and outlier detection.

**Deliverables:**
- CDS overrides in RoutingPolicy:
  - `max_connections` per endpoint (reduce when stressed)
  - `max_pending_requests` (shed load early)
  - `max_retries` (reduce during cascade — retries amplify failure)
- Outlier detection config:
  - `consecutive_5xx`: 5 (eject after 5 consecutive errors)
  - `interval`: 10s
  - `base_ejection_time`: 30s
  - `max_ejection_percent`: 50% (never eject more than half)
- RDS retry policy for unary services:
  - `retry_on: "5xx,reset,connect-failure,refused-stream"`
  - `num_retries`: 2 (normal), 1 (during incident), 0 (during cascade)
  - Never retry streaming RPCs

**Dependencies:** Phase 1 (scoring model provides stress signals)

**Acceptance:**
- Endpoint returning 5xx is ejected within 60 seconds
- Ejected endpoint recovers after base_ejection_time if healthy
- During synthetic cascade: retry count reduces automatically
- No retry storms (retries don't amplify failure)

---

### Phase 3 — Security Integration

**Goal:** React to security events from ai_watcher by adjusting routing.

**Deliverables:**
- Subscribe to ai_watcher events: `alert.dos.*`, `alert.slowloris.*`, `alert.error.spike`
- Anomaly score integration into scorer:
  - `alert.dos.detected` for an IP → reduce weight of endpoint receiving that traffic
  - `alert.error.spike` on a service → tighten circuit breakers
  - `alert.slowloris.detected` → reduce max concurrent streams for affected endpoint
- Rate limit adjustments via Envoy local rate limiter:
  - Tighten during active DoS
  - Relax after incident resolved (with cooldown)
- Publish `routing.security.response` event with action taken

**Dependencies:** Phase 2 (circuit breakers), ai_watcher running

**Acceptance:**
- Simulated DoS → affected endpoint weight reduced within 15 seconds
- Simulated error spike → circuit breakers tightened within 2 cycles
- After incident resolved → gradual return to normal (not instant)
- No routing change on single transient alert (require confirmation)

---

### Phase 4 — Stream Awareness

**Goal:** Safely handle long-lived gRPC streams during routing changes.

**Deliverables:**
- `service_class.go`: full classification
  - `stateless_unary`: event, authentication, rbac, resource, etc.
  - `stream_heavy`: event (OnEvent streams), log, monitoring
  - `control_plane`: cluster_controller, node_agent
  - `deployment_sensitive`: repository, discovery
- Per-class drain strategy:
  - `stateless_unary`: weight=0, immediate effect on new requests
  - `stream_heavy`: weight=0 + extended drain period (5 min), no GOAWAY
  - `control_plane`: minimum weight=10 (never fully drain), prefer stability
  - `deployment_sensitive`: warm-up period after endpoint returns
- Max concurrent streams tracking per endpoint:
  - If one endpoint accumulates >3x median streams, flag for investigation
- `routing.drain.started` / `routing.drain.completed` events

**Dependencies:** Phase 1 (weights), Phase 2 (circuit breakers)

**Acceptance:**
- Stream-heavy service drain takes >5 minutes (not abrupt)
- Control plane services never fully drained
- No existing streams broken by weight changes
- New streams land on healthy endpoints within 1 cycle after drain starts

---

### Phase 5 — Context Awareness

**Goal:** Understand cluster operations and adapt routing around them.

**Deliverables:**
- Subscribe to cluster controller events:
  - `service.phase_changed` (deployment in progress)
  - `plan_apply_started` / `plan_apply_succeeded` (node being updated)
  - `cluster.health.degraded` / `cluster.health.recovered`
- Context modifiers in scorer:
  - Node being updated → reduce weight 50% (it'll restart)
  - Node just recovered → start at 25% weight, ramp up over 5 cycles
  - Deployment rolling → wider weight spread tolerance (expected variance)
- Warm-up model:
  - New endpoint or recovered endpoint starts with low weight
  - Increases by 25% per cycle until reaching scored weight
  - Prevents overwhelming a cold process with full traffic

**Dependencies:** Phase 4 (service classes), cluster controller events (already built)

**Acceptance:**
- During simulated rolling update: traffic shifts away from updating nodes
- After recovery: gradual ramp-up over 25+ seconds
- No routing thrash during deployment (wider tolerance)

---

### Phase 6 — Learning Layer

**Goal:** Use ai_memory to make better decisions over time.

**Deliverables:**
- Store every routing decision in ai_memory:
  - Input state, decision, outcome (did latency improve? did errors decrease?)
  - Tagged by service, time-of-day, day-of-week
- Query ai_memory during scoring:
  - "This endpoint had rising latency last Tuesday at 2pm — pre-reduce weight"
  - "Last time we drained this endpoint, recovery took 3 minutes — adjust drain timer"
- Baseline model:
  - "Normal" weight distribution per service per time-of-day
  - Deviation from baseline = additional signal for scoring
- Confidence score adjustment:
  - High confidence (seen this pattern before) → act faster
  - Low confidence (novel situation) → be more conservative

**Dependencies:** Phase 5 (context), ai_memory service

**Acceptance:**
- Decisions reference historical patterns when available
- Novel situations produce lower confidence scores
- Repeated patterns produce faster, more decisive responses
- ai_memory stores queryable decision history

---

## 3. Data Structures

### Routing Policy (output to xDS watcher)

```go
type RoutingPolicy struct {
    // Per-service routing decisions.
    Services map[string]*ServicePolicy

    // Generation number — xDS watcher uses to detect changes.
    Generation uint64

    // When this policy was computed.
    ComputedAt time.Time
}

type ServicePolicy struct {
    // Endpoint weights (address:port → weight 0-100).
    // Weight 0 = draining (no new connections).
    Weights map[string]uint32

    // Endpoints to drain (weight=0 with grace period).
    Drain []DrainEntry

    // Circuit breaker overrides (nil = use defaults).
    CircuitBreaker *CircuitBreakerOverride

    // Outlier detection config (nil = use defaults).
    OutlierDetection *OutlierDetectionOverride

    // Retry policy override (nil = no retries).
    RetryPolicy *RetryPolicyOverride

    // Decision metadata.
    Confidence float64   // 0.0-1.0
    Reasons    []string  // human-readable explanation
}

type DrainEntry struct {
    Endpoint   string
    Reason     string
    StartedAt  time.Time
    GracePeriod time.Duration  // from service class
}

type CircuitBreakerOverride struct {
    MaxConnections     *uint32
    MaxPendingRequests *uint32
    MaxRequests        *uint32
    MaxRetries         *uint32
}

type OutlierDetectionOverride struct {
    Consecutive5xx      uint32
    Interval            time.Duration
    BaseEjectionTime    time.Duration
    MaxEjectionPercent  uint32
}

type RetryPolicyOverride struct {
    RetryOn    string   // "5xx,reset,connect-failure"
    NumRetries uint32
    BackOff    time.Duration
}
```

### Scoring Inputs

```go
type EndpointMetrics struct {
    // From Prometheus
    CPUUsage       float64  // 0.0-1.0
    MemoryUsage    float64  // 0.0-1.0
    LatencyP99     time.Duration
    LatencyTrend   float64  // positive = rising, negative = falling
    ErrorRate      float64  // 0.0-1.0
    RequestsPerSec float64

    // From ai_watcher
    AnomalyScore   float64  // 0.0-1.0
    ActiveIncidents int

    // From ai_memory
    HistoricalReliability float64  // 0.0-1.0 (1 = always healthy)
    RecentFailures        int     // failures in last 24h

    // Freshness
    CollectedAt time.Time
    Stale       bool  // true if older than 30 seconds
}

type ScoringWeights struct {
    CPU         float64  // default 0.25
    LatencyP99  float64  // default 0.20
    ErrorRate   float64  // default 0.25
    Anomaly     float64  // default 0.15
    Reliability float64  // default 0.15
}
```

### Service Class Configuration

```go
type ServiceClass string

const (
    ClassStatelessUnary     ServiceClass = "stateless_unary"
    ClassStreamHeavy        ServiceClass = "stream_heavy"
    ClassControlPlane       ServiceClass = "control_plane"
    ClassDeploymentSensitive ServiceClass = "deployment_sensitive"
)

type ServiceClassConfig struct {
    Class           ServiceClass
    Weights         ScoringWeights       // per-class scoring sensitivity
    MaxWeightDelta  uint32               // max change per cycle (default 20)
    MinWeight       uint32               // never go below (control_plane: 10, others: 0)
    DrainGrace      time.Duration        // how long to drain (stream_heavy: 5m, others: 30s)
    WarmupCycles    int                  // ramp-up after recovery (default 4 = 20s)
    CooldownCycles  int                  // stable cycles before acting (default 3)
    RetryEnabled    bool                 // unary only
}
```

### Rationale / Explainability

```go
type DecisionRationale struct {
    Endpoint    string
    OldWeight   uint32
    NewWeight   uint32
    Score       float64
    Components  map[string]float64  // {"cpu": 0.3, "latency": 0.7, ...}
    Modifiers   []string            // {"deployment_in_progress", "recovering"}
    Confidence  float64
    Explanation string              // "Reduced weight: latency +30%, error rate elevated"
}
```

---

## 4. Safety Mechanisms

### Invariant Enforcement (`safety.go`)

```go
func (s *SafetyValidator) Validate(proposed, current *RoutingPolicy) *RoutingPolicy {
    for svc, policy := range proposed.Services {
        // 1. Never remove all endpoints
        activeCount := countNonZero(policy.Weights)
        if activeCount == 0 {
            restoreHighest(policy)  // keep at least the best endpoint
        }

        // 2. Max weight delta per cycle
        for ep, newW := range policy.Weights {
            if oldW, ok := current.Services[svc].Weights[ep]; ok {
                delta := abs(int(newW) - int(oldW))
                maxDelta := serviceClassConfig[svc].MaxWeightDelta
                if delta > maxDelta {
                    // Clamp to max delta
                    if newW > oldW {
                        policy.Weights[ep] = oldW + maxDelta
                    } else {
                        policy.Weights[ep] = oldW - maxDelta
                    }
                }
            }
        }

        // 3. Minimum weight for control plane
        if classOf(svc) == ClassControlPlane {
            for ep := range policy.Weights {
                if policy.Weights[ep] < 10 {
                    policy.Weights[ep] = 10
                }
            }
        }

        // 4. Cooldown: require N stable cycles before acting
        if !s.isStable(svc, serviceClassConfig[svc].CooldownCycles) {
            policy.Services[svc] = current.Services[svc]  // hold current
        }
    }
    return proposed
}
```

### Anti-Flapping Strategy

```
Smoothing:
  score_effective = α * score_current + (1 - α) * score_previous
  α = 0.3 (strong smoothing, slow response)
  Tunable per service class (stream_heavy: α=0.2, stateless: α=0.4)

Hysteresis:
  Only change weight if |score_change| > hysteresis_threshold
  Default threshold: 0.05 (5% score change required)

Cooldown:
  After a weight change, hold for CooldownCycles (default 3 = 15 seconds)
  before allowing another change to the same endpoint.

Dead zone:
  If all endpoints score within 10% of each other → equal weights
  (don't create artificial imbalance from noise)
```

### Fallback Behavior

```
If metrics unavailable → neutral policy (all weights equal)
If ai_watcher unreachable → score without anomaly component
If ai_memory unreachable → score without historical component
If scoring fails → return nil (xDS watcher uses previous snapshot)
If all endpoints stale → neutral policy + publish alert
```

---

## 5. Envoy Integration Details

### EDS — Endpoint Weights

```go
// In xDS watcher, after calling GetRoutingPolicy():
endpoint := &endpointv3.LbEndpoint{
    HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
        Endpoint: &endpointv3.Endpoint{
            Address: makeAddress(ip, port),
        },
    },
    LoadBalancingWeight: &wrapperspb.UInt32Value{
        Value: policy.Weights[addr],  // 1-100 from AI Router
    },
}
```

Current default: all endpoints weight=1 (Envoy treats equal).
AI Router range: 1-100 (0 = draining, removed from active set).

### CDS — Circuit Breakers

```go
cluster.CircuitBreakers = &clusterv3.CircuitBreakers{
    Thresholds: []*clusterv3.CircuitBreakers_Thresholds{{
        Priority:           corev3.RoutingPriority_DEFAULT,
        MaxConnections:     wrapperspb.UInt32(override.MaxConnections),
        MaxPendingRequests: wrapperspb.UInt32(override.MaxPendingRequests),
        MaxRequests:        wrapperspb.UInt32(override.MaxRequests),
        MaxRetries:         wrapperspb.UInt32(override.MaxRetries),
    }},
}
```

Safe defaults (when AI Router has no opinion):
- max_connections: 1024
- max_pending_requests: 1024
- max_requests: 1024
- max_retries: 3

### CDS — Outlier Detection

```go
cluster.OutlierDetection = &clusterv3.OutlierDetection{
    Consecutive_5Xx:                wrapperspb.UInt32(5),
    Interval:                       durationpb.New(10 * time.Second),
    BaseEjectionTime:               durationpb.New(30 * time.Second),
    MaxEjectionPercent:             wrapperspb.UInt32(50),
    EnforcingConsecutive_5Xx:       wrapperspb.UInt32(100),  // always enforce
    EnforcingSuccessRate:           wrapperspb.UInt32(0),    // disabled initially
}
```

### RDS — Retry Policy (unary only)

```go
route.Route.RetryPolicy = &routev3.RetryPolicy{
    RetryOn:    "5xx,reset,connect-failure,refused-stream",
    NumRetries: wrapperspb.UInt32(override.NumRetries),
    RetryBackOff: &routev3.RetryPolicy_RetryBackOff{
        BaseInterval: durationpb.New(100 * time.Millisecond),
        MaxInterval:  durationpb.New(1 * time.Second),
    },
}
```

Critical: only apply to routes serving unary RPCs. Stream routes (Timeout: 0)
must NOT have retry policies (would create duplicate streams).

### LB Policy Change

Current: `ROUND_ROBIN` on all clusters.

With AI Router weights: change to `ROUND_ROBIN` with weighted endpoints.
Envoy's round-robin respects `LoadBalancingWeight` on `LbEndpoint`.
No LB policy change needed — just add weights.

For Phase 2+: consider `LEAST_REQUEST` with `active_request_bias` for
latency-sensitive services. This naturally avoids overloaded endpoints.

---

## 6. Observability Design

### Structured Logs

Every decision cycle produces:
```json
{
  "level": "INFO",
  "msg": "routing_decision",
  "service": "event.EventService",
  "generation": 42,
  "confidence": 0.85,
  "endpoints_changed": 1,
  "weights": {"10.0.0.63:10010": 80, "10.0.0.64:10010": 100},
  "reasons": ["latency +30% on 10.0.0.63", "anomaly score elevated"],
  "mode": "active"
}
```

Changes only logged at INFO. No-change cycles logged at DEBUG.

### Prometheus Metrics

```
ai_router_decisions_total{service, action}           # counter: weight_changed, drained, circuit_changed
ai_router_decision_confidence{service}                # gauge: 0.0-1.0
ai_router_endpoint_weight{service, endpoint}          # gauge: current weight
ai_router_endpoint_score{service, endpoint}           # gauge: current score
ai_router_stale_metrics_total{service}                # counter: metrics too old
ai_router_safety_clamp_total{service, reason}         # counter: delta_exceeded, min_endpoint, cooldown
ai_router_cycle_duration_seconds                      # histogram: decision computation time
```

### Events (via event service)

```
routing.weights.changed   — weight adjustment applied
routing.drain.started     — endpoint entering drain
routing.drain.completed   — endpoint fully drained
routing.circuit.changed   — circuit breaker adjusted
routing.security.response — routing change due to security event
routing.fallback.active   — metrics unavailable, using neutral policy
```

All events include service name, affected endpoints, rationale, confidence.

### Decision Trace (stored in ai_memory)

Every non-neutral decision stored as type=decision in ai_memory:
```
title: "Reduced weight on event/10.0.0.63 (80→65)"
tags: "routing,event,weight-change"
content: full DecisionRationale JSON
metadata: {"confidence": "0.85", "trigger": "latency_trend"}
```

Queryable for the learning layer (Phase 6).

---

## 7. Test Plan

### Unit Tests (`engine_test.go`, `scorer_test.go`, `safety_test.go`)

**Scoring correctness:**
- Equal metrics → equal scores
- High CPU → higher score (lower weight)
- Rising latency → higher score
- High anomaly → higher score
- Stale metrics → score ignored (neutral)

**Safety invariants:**
- Never zero active endpoints
- Delta clamped to ±MaxWeightDelta
- Control plane minimum weight enforced
- Cooldown prevents rapid changes
- Dead zone prevents noise-driven changes

**Anti-flapping:**
- Oscillating metrics → stable weights (smoothing)
- Score hovering near threshold → no change (hysteresis)

### Integration Tests

**Neutral mode (Phase 0):**
- Router returns nil → xDS unchanged
- No weight attributes in EDS

**Metrics-based (Phase 1):**
- Mock Prometheus returns high CPU for one endpoint → weight decreases
- Mock Prometheus returns equal metrics → weights equal
- Prometheus unreachable → neutral policy (no crash)

### Simulation Tests

**Load spike:**
- One endpoint at 90% CPU, others at 30%
- Expect: weight reduces over 2-3 cycles, traffic shifts
- Verify: no oscillation, gradual change, recovery when load normalizes

**Gradual degradation:**
- Latency rising 5% per cycle on one endpoint
- Expect: weight decreases before latency becomes critical
- Verify: predictive behavior, not just reactive

**Recovery:**
- Endpoint returns from drain
- Expect: warm-up period (25% → 50% → 75% → 100%)
- Verify: no traffic slam on cold process

### Stream-Specific Tests

**Long-lived stream drain:**
- Reduce weight of stream-heavy endpoint to 0
- Verify: no existing stream interrupted
- Verify: new streams go elsewhere
- Verify: drain completes after grace period

**Stream accumulation:**
- One endpoint has 10x more active streams than others
- Verify: flagged for investigation (not auto-drained)

### Security Tests

**DoS response:**
- Inject `alert.dos.detected` event
- Verify: affected endpoint weight reduced within 15 seconds
- Verify: circuit breakers tightened
- Verify: gradual recovery after incident cleared

**Metric poisoning:**
- Prometheus returns impossible values (negative latency, >100% CPU)
- Verify: invalid metrics rejected, neutral fallback used

### Chaos Tests

**Node failure:**
- Remove one endpoint entirely
- Verify: remaining endpoints get redistributed weight
- Verify: no panic, no oscillation

**All metrics stale:**
- Prometheus goes down
- Verify: neutral policy within 2 cycles
- Verify: `routing.fallback.active` event published

---

## 8. Acceptance Criteria per Phase

### Phase 0
- [ ] AI Router service starts, passes health check
- [ ] xDS watcher calls GetRoutingPolicy() every cycle
- [ ] Routing behavior identical to before (nil policy)
- [ ] `globular ai routing status` returns "neutral"
- [ ] No performance regression in xDS snapshot build time

### Phase 1
- [ ] Under equal load: weights within 5% of each other (no unnecessary churn)
- [ ] Under unequal load: heavier endpoint weight reduced within 15 seconds
- [ ] Load removed: weights equalize within 30 seconds
- [ ] Decision log shows scores and rationale
- [ ] No endpoint weight oscillation over 5-minute stable period
- [ ] Prometheus unreachable: graceful fallback, no crash

### Phase 2
- [ ] Endpoint returning 5xx: ejected within 60 seconds
- [ ] Ejected endpoint: recovered after base_ejection_time if healthy
- [ ] During cascade: retry count reduces (no amplification)
- [ ] Circuit breaker limits reduce under stress
- [ ] Circuit breaker limits restore when stress clears

### Phase 3
- [ ] DoS alert → weight reduction within 15 seconds
- [ ] Error spike → circuit breakers tighten within 2 cycles
- [ ] Transient single alert → no routing change (multi-signal required)
- [ ] `routing.security.response` event published with rationale

### Phase 4
- [ ] Stream-heavy drain takes ≥5 minutes
- [ ] Control plane never fully drained (min weight=10)
- [ ] No existing streams broken by weight changes
- [ ] Service classification correct for all 28+ services

### Phase 5
- [ ] Rolling update: traffic shifts away from updating nodes
- [ ] Recovery: gradual ramp-up over ≥20 seconds
- [ ] No routing thrash during deployment

### Phase 6
- [ ] Decisions reference historical patterns when available
- [ ] Repeated patterns produce faster responses
- [ ] Novel situations produce lower confidence
- [ ] ai_memory contains queryable decision history

---

## 9. Migration Plan

### Rollout Strategy

```
Step 1: Deploy AI Router in neutral mode (Phase 0)
  - Returns nil for all policies
  - Verify: zero behavior change

Step 2: Enable observe-only mode
  - Router computes policies but doesn't apply them
  - Logs what it WOULD do
  - Compare proposed vs actual behavior over 24-48 hours

Step 3: Enable for one service class (stateless_unary)
  - Start with low-risk services (authentication, rbac)
  - Monitor for 24 hours

Step 4: Enable for remaining unary services
  - event, file, resource, etc.
  - Monitor for 48 hours

Step 5: Enable for stream-heavy services
  - With conservative drain timers
  - Monitor for 1 week

Step 6: Enable security integration
  - After baseline behavior established
```

### Instant Rollback

```bash
# Disable AI Router — immediate return to static behavior
globular ai routing override --mode neutral

# Or via environment variable (requires restart)
GLOBULAR_AI_ROUTER_MODE=neutral
```

The xDS watcher always has a fallback path: if GetRoutingPolicy() returns nil
or errors, it builds the snapshot exactly as before. The AI Router is purely
additive.

### Configuration

```yaml
# In etcd: /globular/config/ai_router
mode: "active"              # neutral | observe | active
enabled_classes:
  - stateless_unary
  - stream_heavy
scoring_weights:
  cpu: 0.25
  latency: 0.20
  error_rate: 0.25
  anomaly: 0.15
  reliability: 0.15
safety:
  max_weight_delta: 20
  cooldown_cycles: 3
  min_endpoints: 1
  smoothing_alpha: 0.3
```

---

## 10. Risks & Unknowns

### Known Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Prometheus metrics delayed/stale | Wrong scoring → bad weights | 30s freshness gate, neutral fallback |
| Scoring model too aggressive | Unnecessary weight changes | Smoothing + hysteresis + cooldown |
| Scoring model too conservative | Doesn't respond in time | Tune per service class, monitor response time |
| Stream drain disrupts users | Active streams interrupted | Never force-close, grace period per class |
| Metric poisoning by attacker | Router makes attacker's choices | Bound all values, multi-signal validation |
| xDS snapshot build slows down | All routing delayed | Decision cache, timeout on collector |
| Flapping between two states | Unstable routing | Dead zone, minimum change threshold |
| Deployment confused with degradation | Healthy nodes get drained during update | Context awareness (Phase 5) |

### Unknowns (to resolve during implementation)

1. **Optimal smoothing alpha** — start at 0.3, tune based on observed stability
2. **Best drain grace period for streams** — start at 5 minutes, measure actual stream lifetimes
3. **Prometheus query cost** — batch queries per cycle, measure impact
4. **Outlier detection false positive rate** — start conservative, widen as trust grows
5. **Safe GOAWAY strategy** — defer to Phase 4+, study Envoy behavior with HTTP/2 GOAWAY
6. **Anomaly signal trust** — how much weight to give ai_watcher scores vs hard metrics
7. **Multi-node scoring** — how to score when same service runs on multiple nodes (average? worst?)
8. **Cold start** — first 5 minutes after cluster boot, no historical data, all metrics stale
9. **Single-endpoint services** — can't rebalance, but can still circuit-break and rate-limit
