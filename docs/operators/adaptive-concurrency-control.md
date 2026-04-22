# Adaptive Concurrency Control (ACC)

Adaptive Concurrency Control is Globular's in-process load shedding layer. It
sits inside the gRPC interceptor chain on every service and protects the cluster
from overload by classifying each incoming call into a priority lane and
admitting or rejecting it based on that lane's current capacity.

---

## Why ACC Exists

Under load spikes — mass pod restarts, multi-node recovery, cold-start storms —
the default gRPC behavior is to queue all calls until memory or CPU exhausts.
ACC gives the cluster a way to stay alive and keep its most critical paths
(liveness heartbeats, workflow completions) healthy while shedding lower-priority
work rather than degrading uniformly.

---

## Priority Lanes

Every incoming call is classified into one of five lanes:

| Lane | Methods | Capacity | Shed behaviour |
|------|---------|----------|----------------|
| **P0** | `grpc.health.v1.Health/Check`, `/Watch` | Unlimited | Never shed |
| **P1-authz** | `rbac.RbacService/ValidateAction` | 200 slots | `RESOURCE_EXHAUSTED` |
| **P1-periodic** | `ReportNodeStatus`, `EmitWorkflowEvent` | 50 slots | `RESOURCE_EXHAUSTED` |
| **P1-control** | `CompleteOperation`, `ExecuteWorkflow` | 10 slots | `RESOURCE_EXHAUSTED` |
| **P2** | Everything else | AIMD window (10–2000) | `RESOURCE_EXHAUSTED` |

**P0** is unconditionally admitted — gRPC health probes must never be throttled.

**P1 pools** use fixed-size atomic counters. `ValidateAction` gets its own large
pool (200) rather than P0 exemption because a full P0 exemption would let every
service flood the RBAC service during a cold-start storm, causing OOM. 200 slots
allow healthy backpressure while callers queue to their context deadline.

`JoinCluster`, `SetRoleBinding`, and all other calls fall into **P2**. These are
operator-retriable or low-frequency, and their admission is governed by the AIMD
window.

---

## AIMD Algorithm (P2)

The P2 window starts at 100 concurrent requests and adjusts after each RPC:

- **Additive increase**: if `RTT < baseline × 1.5` → `window += 1` (good health)
- **Multiplicative decrease**: if `RTT > baseline × 2.0` → `window = max(10, window × 0.9)`

A `CompareAndSwap` prevents thundering-herd thrash when many goroutines observe
the same overload at the same time.

### Warmup

The P2 window is fully open for the first **5 minutes AND 200 samples** (whichever
takes longer), with a hard ceiling of **30 minutes**. During warmup, no calls are
shed. On warmup exit, the baseline RTT is set to the p25 of observed latencies
(or 20 ms if no samples are available).

### Baseline Recalibration

Every 5 minutes, the baseline RTT is recalibrated using four guards:

1. **Load gate** — skip if inflight > 60% of window (don't recalibrate during crisis)
2. **p25 selection** — use the 25th percentile of the ring buffer (biased toward fast paths)
3. **Sanity cap** — reject if candidate > 125% of current baseline (filters crisis spikes)
4. **EMA smoothing** — `new = old × 0.95 + candidate × 0.05` (hours to absorb 2× drift)

---

## Observability

ACC exposes counters via Go's `expvar` at `/debug/vars` on every service:

| Counter | Meaning |
|---------|---------|
| `acc.p2_admitted_total` | P2 calls admitted since start |
| `acc.p2_rejected_total` | P2 calls shed since start |
| `acc.p1_admitted_total` | P1 calls admitted since start |
| `acc.p1_rejected_total` | P1 calls shed since start |

These counters are also scraped by Prometheus.

Clients receiving `RESOURCE_EXHAUSTED` from the interceptor should back off and
retry — the error message distinguishes pool exhaustion from other causes:

```
server overloaded: authorization pool exhausted
server overloaded: periodic pool exhausted
server overloaded: control pool exhausted
server overloaded: too many concurrent requests
```

---

## Live Configuration

ACC parameters are stored as JSON at `/globular/system/acc/config` in etcd.
Every interceptor picks up changes via a background watcher — **no service
restart required**.

All fields are optional. Omitted fields keep their running defaults.

```json
{
  "p1_authz_size": 200,
  "p1_periodic_size": 50,
  "p1_control_size": 10,
  "p2_min_window": 10,
  "p2_max_window": 2000,
  "aimd_increase_threshold_mult": 1.5,
  "aimd_decrease_threshold_mult": 2.0,
  "aimd_decrease_rate": 0.9,
  "recalib_interval_sec": 300,
  "recalib_alpha": 0.05,
  "recalib_max_increase": 1.25,
  "recalib_load_gate": 0.60
}
```

---

## CLI Reference

### View Current Configuration

```bash
globular cluster acc get
```

Reads `/globular/system/acc/config` from etcd. If no key is set, compile-time
defaults are in effect and the table shows what those defaults are.

```bash
# JSON output
globular cluster acc get --output json
```

### Update Parameters

```bash
globular cluster acc set [flags]
```

Only the flags you pass are changed. The existing etcd config is read first, the
specified fields are merged in, and the result is written back atomically.

**P1 pool sizes:**

```bash
# Expand the auth pool during a mass-restart event
globular cluster acc set --p1-authz-size 400

# Shrink the heartbeat pool on a small cluster
globular cluster acc set --p1-periodic-size 20
```

**P2 AIMD window bounds:**

```bash
globular cluster acc set --p2-min-window 20 --p2-max-window 500
```

**AIMD thresholds:**

```bash
# More aggressive increase (1.2×), tighter decrease trigger (1.8×)
globular cluster acc set --aimd-increase-mult 1.2 --aimd-decrease-mult 1.8

# Steeper decrease on overload (0.75 instead of 0.9)
globular cluster acc set --aimd-decrease-rate 0.75
```

**Recalibration tuning:**

```bash
# Recalibrate every 2 minutes instead of 5
globular cluster acc set --recalib-interval-sec 120

# Faster baseline adaptation (higher alpha)
globular cluster acc set --recalib-alpha 0.1

# Tighter sanity cap (reject if candidate > 110% of current baseline)
globular cluster acc set --recalib-max-increase 1.1
```

**Full flag reference:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--p1-authz-size` | int | 200 | ValidateAction pool size |
| `--p1-periodic-size` | int | 50 | Heartbeat pool size |
| `--p1-control-size` | int | 10 | Workflow lifecycle pool size |
| `--p2-min-window` | int | 10 | Minimum AIMD window |
| `--p2-max-window` | int | 2000 | Maximum AIMD window |
| `--aimd-increase-mult` | float | 1.5 | RTT < baseline×mult → increase window |
| `--aimd-decrease-mult` | float | 2.0 | RTT > baseline×mult → decrease window |
| `--aimd-decrease-rate` | float | 0.9 | Multiplicative decrease factor (0 < x < 1) |
| `--recalib-interval-sec` | int | 300 | Baseline recalibration interval (seconds) |
| `--recalib-alpha` | float | 0.05 | EMA smoothing factor (0 < x < 1) |
| `--recalib-max-increase` | float | 1.25 | Sanity cap multiplier (must be > 1) |
| `--recalib-load-gate` | float | 0.60 | Skip recalib if inflight > gate×window |

### Revert to Defaults

```bash
globular cluster acc reset
```

Deletes `/globular/system/acc/config` from etcd. All interceptors revert to
compile-time defaults within seconds.

---

## Operational Playbook

### Cluster is shedding P2 calls but cluster is healthy

The AIMD baseline may have been set too aggressively during warmup, or a recent
workload spike caused the decrease to floor prematurely.

1. Check expvar counters: is `p2_rejected_total` rising continuously or did it spike and stabilize?
2. If continuously rising with healthy services, raise the P2 min window:
   ```bash
   globular cluster acc set --p2-min-window 50
   ```
3. If the baseline is stale, lower the sanity cap temporarily to let recalibration catch up:
   ```bash
   globular cluster acc set --recalib-max-increase 1.1 --recalib-interval-sec 60
   ```

### Cold-start storm is flooding ValidateAction

Increase the P1-authz pool to absorb the burst:

```bash
globular cluster acc set --p1-authz-size 400
```

Restore after steady state:

```bash
globular cluster acc set --p1-authz-size 200
```

### Heartbeat pool is exhausted during multi-node recovery

The P1-periodic pool (ReportNodeStatus, EmitWorkflowEvent) may need temporary
expansion during cluster-wide restarts:

```bash
globular cluster acc set --p1-periodic-size 100
```

### Revert all tuning after incident

```bash
globular cluster acc reset
```

---

## Design Notes

- ACC admission fires **before** auth (RBAC network calls) in the interceptor chain — load is shed before expensive RBAC lookups.
- RTT recording fires **after** the handler returns — only real call durations feed the AIMD algorithm.
- P1 pool changes take effect immediately via atomic store — no drain or quiesce period.
- P2 window changes via `minWindow`/`maxWindow` also take effect immediately; existing inflight slots are not evicted.
- The etcd watcher retries with exponential backoff (5s → 5min) if etcd is unreachable. The service continues with its last-known config (or compile-time defaults on first start).
