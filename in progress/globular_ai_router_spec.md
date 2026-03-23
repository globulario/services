# Globular AI Router — Refined Vision & Implementation Request

## 1. Overview

The AI Router is a **control-plane routing policy engine** that enhances Envoy’s load balancing through dynamic, context-aware configuration.

It does NOT:
- sit in the request path
- replace Envoy
- perform per-request decisions

It DOES:
- compute routing strategies periodically
- influence where **new connections and streams land**
- shape traffic safely and predictably

---

## 2. Core Insight

With gRPC:

- Unary RPCs → can be rebalanced immediately
- Streaming RPCs → are pinned for their lifetime

Therefore:

> The AI Router is not routing traffic in real time  
> It is shaping the **future distribution of traffic**

---

## 3. Architecture

ai_watcher   → anomaly + incident signals  
ai_memory    → history + baselines  
ai_router    → routing policy engine  

ai_router → xDS watcher → xDS server → Envoy → traffic

### Responsibility Boundaries

| Component     | Responsibility |
|---------------|--------------|
| xDS Watcher   | Snapshot generation & publishing |
| ai_router     | Routing policy computation |
| ai_watcher    | Anomaly detection |
| ai_memory     | Historical data & learning |
| Envoy         | Execution (data plane) |

---

## 4. Core Responsibilities

### 4.1 Endpoint Weighting
- Compute weights per endpoint
- Gradual changes only
- No abrupt shifts

### 4.2 Graceful Draining
- Weight → 0 (no new traffic)
- Allow existing streams to complete
- Optional controlled GOAWAY (later phase)

### 4.3 Circuit Breaker Tuning
- Adjust limits conservatively
- Prevent overload amplification

### 4.4 Outlier Detection
- Enable endpoint ejection
- Keep strict safety limits

### 4.5 Rate Limiting
- Adjust during anomalies
- Separate behavior for trusted vs untrusted traffic

---

## 5. Non-Goals

- No per-request AI inference
- No data-plane replacement
- No ML in Phase 1
- No direct mutation of cluster desired state
- No global one-policy-fits-all routing

---

## 6. Service Classes

Routing must be service-aware.

Minimum classes:

- stateless_unary
- stream_heavy
- control_plane
- deployment_sensitive

Each class has:
- different scoring sensitivity
- different drain behavior
- different safety limits

---

## 7. Decision Model (Phase 1)

Deterministic scoring:

score(endpoint) =
  w1 * cpu +
  w2 * latency_p99 +
  w3 * error_rate +
  w4 * anomaly_score +
  w5 * (1 - reliability)

Lower score = healthier

### Output

- weights
- optional drain list
- optional overrides (circuit, rate limit)
- confidence score
- rationale list

---

## 8. Decision Output Structure

```go
type RoutingDecision struct {
    Weights      map[Endpoint]int
    Drain        []Endpoint
    Overrides    ClusterOverrides

    Confidence   float64
    Reasons      []string
}
```

---

## 9. Safety Invariants

1. Never remove all endpoints
2. Max weight delta per cycle (e.g. ±20%)
3. No abrupt drain for stream-heavy services
4. Require multi-signal confirmation for strong actions
5. Manual override always wins
6. Low confidence → conservative behavior
7. Every decision must be explainable

---

## 10. Failure Modes & Mitigations

### Flapping
- smoothing
- cooldown periods
- hysteresis

### Bad Metrics
- freshness validation
- fallback to neutral policy

### Stream Risk
- per-service drain strategy
- no forced interruption by default

### Attack Steering
- bounded decisions
- multi-signal validation

### Deployment Confusion
- integrate rollout context
- warmup-aware scoring

---

## 11. Execution Loop

every N seconds:
  gather inputs
  compute score
  generate decision
  validate invariants
  merge into xDS snapshot
  publish
  observe outcome
  store in ai_memory

---

## 12. Observability

Every decision must produce:

- logs (structured)
- metrics (decision rate, changes, confidence)
- events (routing change applied)
- explanation trace

Example:

Reduced weight of node-3 by 15%:
- latency +30%
- error rate +10%
- anomaly score elevated

---

## 13. Phased Implementation

### Phase 0 — Interface Only
- no behavior change
- router returns neutral policy

### Phase 1 — Metrics-Based Weighting
- CPU / latency / error-based scoring
- weights only
- unary services only

### Phase 2 — Stability Controls
- circuit breaker tuning
- outlier detection

### Phase 3 — Security Integration
- anomaly-aware routing
- rate limiting adjustments

### Phase 4 — Stream Awareness
- safe draining strategies
- service-class policies

### Phase 5 — Context Awareness
- deployment / recovery integration

### Phase 6 — Learning Layer
- ai_memory-assisted recommendations

---

## 14. Envoy Integration

- EDS → endpoint weights
- CDS → circuit breakers, outlier detection
- RDS → retry policies
- Rate limit filter → dynamic limits

All changes:
- bounded
- incremental
- reversible

---

## 15. CLI Integration

globular ai routing status  
globular ai routing evaluate  
globular ai routing apply  
globular ai routing override  

---

## 16. Testing Strategy

### Unit Tests
- scoring correctness
- invariant enforcement

### Simulation Tests
- load spike
- gradual degradation
- recovery scenarios

### Chaos Tests
- node failure
- partial outages

### Stream Tests
- long-lived connection handling
- drain behavior

### Attack Tests
- DoS simulation
- metric poisoning attempts

---

## 17. Acceptance Criteria

Each phase must ensure:

- no regression in stability
- no routing oscillation
- explainable decisions
- safe handling of streams
- bounded impact of changes

---

## 18. Migration Strategy

- default: disabled / neutral
- enable per service class
- gradual rollout
- instant rollback to static behavior

---

## 19. Open Questions

- optimal smoothing / hysteresis parameters
- best drain strategy for long-lived streams
- metric freshness guarantees
- anomaly signal trust weighting
- safe GOAWAY usage strategy

---

## 20. Final Principle

The goal is NOT to maximize performance.

The goal is:

Maintain stability, resilience, and predictability  
while adapting safely to changing conditions

---

# Implementation Request

Based on this specification, please produce:

## 1. Detailed Architecture
- packages
- interfaces
- data flow

## 2. Phase-by-Phase Plan
- deliverables per phase
- dependencies
- order of implementation

## 3. Data Structures
- routing policy
- scoring inputs
- rationale / explainability model
- service-class config

## 4. Safety Mechanisms
- invariant enforcement
- anti-flapping strategy
- fallback behavior

## 5. Envoy Integration Details
- exact xDS fields used
- safe defaults

## 6. Observability Design
- logs, metrics, events
- tracing decisions

## 7. Test Plan
- full coverage of failure modes
- simulation scenarios

## 8. Acceptance Criteria per Phase

## 9. Migration Plan

## 10. Risks & Unknowns

---

## Constraint

Start deterministic and conservative.

Do NOT introduce ML or complex prediction early.

Focus on:
- correctness
- stability
- explainability
- safe incremental rollout
