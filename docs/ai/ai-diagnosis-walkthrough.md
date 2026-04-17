# AI Diagnosis Walkthrough

This document walks through a complete AI-assisted incident diagnosis from start to finish, showing every tool call, decision point, and verification step. Use it as a reference for understanding how AI operates within Globular's constraints.

## Scenario: Authentication Service Failing on One Node

**Symptom**: The admin dashboard shows authentication health checks failing intermittently on node-2. Users report occasional login failures. The service hasn't fully crashed — it restarts via systemd but keeps degrading.

---

## Phase 1: Observe

The investigation begins by gathering evidence from multiple sources before forming any hypothesis.

### Step 1.1 — Check cluster health

```
Tool: cluster_get_health
```

```json
{
  "status": "DEGRADED",
  "nodes": [
    {"id": "node-1", "status": "HEALTHY", "services": 12},
    {"id": "node-2", "status": "DEGRADED", "services": 12},
    {"id": "node-3", "status": "HEALTHY", "services": 10}
  ],
  "degraded_services": ["authentication@node-2"]
}
```

**Observation**: Only node-2 is degraded, and only the authentication service is affected. This is a localized issue, not a cluster-wide problem.

### Step 1.2 — Check the service logs

```
Tool: nodeagent_get_service_logs
Parameters: { node: "node-2", service: "authentication", lines: 100 }
```

```
2026-04-14T10:15:23Z ERROR  token cache eviction failed: mmap region exhausted
2026-04-14T10:15:24Z WARN   heap allocation 1.8GB exceeds soft limit 1.5GB
2026-04-14T10:15:30Z ERROR  health check handler: context deadline exceeded
2026-04-14T10:15:31Z INFO   systemd: unit restarted (restart count: 7 in 2h)
2026-04-14T10:16:02Z WARN   heap allocation 512MB after restart (growing)
2026-04-14T10:18:15Z WARN   heap allocation 890MB (rate: ~3MB/s)
```

**Observation**: Memory is growing rapidly after each restart. The token cache eviction is failing. The service hits its memory limit, gets restarted by systemd, and immediately begins leaking again.

### Step 1.3 — Check metrics for the pattern over time

```
Tool: metrics_query_range
Parameters: {
  query: "process_resident_memory_bytes{service='authentication', node='node-2'}",
  start: "2026-04-14T08:00:00Z",
  end: "2026-04-14T10:20:00Z",
  step: "60s"
}
```

The metrics show a sawtooth pattern: memory climbs linearly from ~400MB to ~1.8GB over approximately 30 minutes, then drops back to ~400MB (systemd restart), then climbs again. This pattern has repeated 7 times in the last 2 hours.

### Step 1.4 — Check if this has happened before

```
Tool: memory_query
Parameters: { project: "globular-services", tags: "authentication,memory", limit: 5 }
```

```json
{
  "total": 1,
  "memories": [{
    "id": "mem-abc123",
    "type": "debug",
    "title": "Authentication token cache leak after 0.0.9 upgrade",
    "tags": ["authentication", "memory", "token-cache"],
    "created_at": "2026-03-20T14:30:00Z"
  }]
}
```

There is a prior memory about a similar issue. Let's retrieve it.

```
Tool: memory_get
Parameters: { id: "mem-abc123", project: "globular-services" }
```

```json
{
  "content": "After upgrading authentication to 0.0.9, token cache entries were not being evicted when the mmap region filled up. Root cause: the eviction goroutine was blocked on a mutex held by the token validation path. Fix was to increase the mmap region size in config and restart. A proper fix (async eviction) was planned for 0.0.10.",
  "metadata": "{\"root_cause\":\"token-cache-mutex-deadlock\",\"confidence\":\"high\",\"fix\":\"increase-mmap-region\"}"
}
```

**Observation**: This is a known issue. The same token cache leak was seen after the 0.0.9 upgrade. The config workaround (increase mmap region) was applied previously, but it may have been lost or the service was upgraded again.

### Step 1.5 — Check the installed version

```
Tool: cluster_get_node_full_status
Parameters: { node: "node-2" }
```

```json
{
  "packages": {
    "authentication": {
      "installed_version": "0.0.9",
      "desired_version": "0.0.9",
      "status": "INSTALLED"
    }
  }
}
```

Still on 0.0.9 — the version with the known bug.

### Step 1.6 — Check the current config

```
Tool: service_config_get
Parameters: { service: "authentication", node: "node-2" }
```

```json
{
  "token_cache_mmap_size_mb": 256,
  "token_cache_max_entries": 100000
}
```

The mmap region is set to the default 256MB. The previous fix increased it to 1024MB, but that config change was not persisted — likely lost during a node rebuild or config reset.

---

## Phase 2: Diagnose

With evidence gathered, the diagnosis is clear:

| Evidence | Finding |
|----------|---------|
| Sawtooth memory pattern | Memory leak with periodic restarts |
| `mmap region exhausted` in logs | Token cache eviction is failing |
| Memory growing at ~3MB/s after restart | Leak is deterministic and reproducible |
| Prior memory `mem-abc123` | Known issue in authentication 0.0.9 |
| `token_cache_mmap_size_mb: 256` | Config workaround was lost |
| 7 restarts in 2 hours | systemd is masking the problem, not fixing it |

**Root cause**: Authentication 0.0.9 has a token cache memory leak caused by a mutex deadlock in the eviction goroutine. The previous config workaround (increasing mmap region to 1024MB) was lost, causing the leak to hit the smaller default limit and trigger repeated OOM restarts.

**Confidence**: 0.95 (high — matches a known, previously diagnosed issue with clear evidence)

---

## Phase 3: Recommend

Two options, applied together:

1. **Immediate mitigation** — Increase `token_cache_mmap_size_mb` to 1024 on node-2. This buys time by allowing the cache to grow larger before eviction pressure triggers the deadlock. Risk: LOW.

2. **Permanent fix** — Upgrade authentication to 0.0.10 (if available), which includes the async eviction fix. Risk: LOW (standard service upgrade through convergence model).

**Proposed action**: Apply the config change first (immediate relief), then check if 0.0.10 is available in the repository for a proper fix.

---

## Phase 4: Execute

### Step 4.1 — Apply the config workaround

The AI does **not** edit configuration files directly. It uses the proper config API:

```
Tool: service_config_put (via approved workflow)
Parameters: {
  service: "authentication",
  node: "node-2",
  key: "token_cache_mmap_size_mb",
  value: "1024"
}
```

The config change is written to etcd, making it visible, replicated, and auditable.

### Step 4.2 — Restart the service to pick up the new config

This triggers a remediation workflow:

```
Workflow: remediate.doctor.finding
Steps:
  1. resolve_finding    → authentication on node-2, finding: memory-leak-config
  2. assess_risk        → LOW (config change + restart, no data loss)
  3. execute_remediation → Node Agent restarts authentication via systemctl
  4. verify_convergence  → Re-check health after restart
```

### Step 4.3 — Check repository for 0.0.10

```
Tool: repository_get_artifact_versions
Parameters: { publisher: "core@globular.io", name: "authentication" }
```

```json
{
  "versions": [
    {"version": "0.0.10", "status": "PUBLISHED", "build": 1},
    {"version": "0.0.9", "status": "PUBLISHED", "build": 1}
  ]
}
```

Version 0.0.10 is available. The operator can upgrade when ready:

```bash
globular services desired set authentication 0.0.10 --publisher core@globular.io
```

The AI **does not** set the desired version itself — that is an operator decision (see [AI Rules](ai/ai-rules.md): "AI cannot modify desired state").

---

## Phase 5: Verify

### Step 5.1 — Check health after restart

```
Tool: cluster_get_health
```

```json
{
  "status": "HEALTHY",
  "nodes": [
    {"id": "node-1", "status": "HEALTHY"},
    {"id": "node-2", "status": "HEALTHY"},
    {"id": "node-3", "status": "HEALTHY"}
  ],
  "degraded_services": []
}
```

### Step 5.2 — Monitor memory after restart

```
Tool: metrics_query
Parameters: {
  query: "process_resident_memory_bytes{service='authentication', node='node-2'}"
}
```

Memory is at 420MB and stable (not growing) — the larger mmap region allows eviction to work correctly.

### Step 5.3 — Confirm no user impact

```
Tool: metrics_query
Parameters: {
  query: "rate(grpc_server_handled_total{service='authentication',grpc_code='OK'}[5m])"
}
```

Request success rate is back to 100%. No more intermittent login failures.

---

## Phase 6: Record

### Step 6.1 — Store the diagnosis for future reference

```
Tool: memory_store
Parameters: {
  project: "globular-services",
  type: "debug",
  title: "Authentication memory leak recurrence — mmap config lost on node-2",
  content: "Token cache memory leak in authentication 0.0.9 recurred on node-2. Root cause: mmap region config (token_cache_mmap_size_mb) reverted to default 256MB, likely during node rebuild. Previous fix (increase to 1024MB) was not persisted in desired config. Applied config workaround again. Permanent fix: upgrade to 0.0.10 (async eviction). Operator notified about upgrade.",
  tags: "authentication,memory,token-cache,mmap,node-2",
  metadata: "{\"root_cause\":\"config-regression\",\"confidence\":\"0.95\",\"fix\":\"mmap-increase+upgrade-0.0.10\"}",
  related_ids: "mem-abc123"
}
```

### Step 6.2 — Link the new memory to the original

```
Tool: memory_update
Parameters: {
  id: "mem-abc123",
  project: "globular-services",
  related_ids: "<new-memory-id>",
  metadata: "{\"recurrence\":\"2026-04-14\",\"config-regression\":\"true\"}"
}
```

This builds a knowledge graph: the original diagnosis and the recurrence are now linked. If this pattern appears a third time, the AI can immediately identify it and recommend making the config change part of the service's default spec file.

---

## What Made This Diagnosis Work

| Principle | How it applied |
|-----------|---------------|
| **Observe before acting** | Five evidence-gathering steps before any hypothesis |
| **Check memory for prior art** | Found the exact same issue from a month ago |
| **Verify current state** | Confirmed the config had regressed, didn't assume |
| **Act through workflows** | Config change and restart went through the standard remediation workflow |
| **Respect boundaries** | AI recommended the upgrade but did not set desired state — that's the operator's decision |
| **Verify after acting** | Checked health, memory usage, and request success rate |
| **Record for the future** | Stored the diagnosis with metadata and linked it to the prior incident |

---

## What Would Have Gone Wrong Without These Steps

| Shortcut | Consequence |
|----------|-------------|
| Skip memory query | Would have diagnosed from scratch instead of recognizing a known issue (slower, lower confidence) |
| Restart without config change | Service would leak again in 30 minutes — same sawtooth pattern |
| Set desired state to 0.0.10 directly | Violates AI rules — operator might not want to upgrade during peak hours |
| Edit config file on disk | Change invisible to etcd, lost on next restart, no audit trail |
| Skip verification | Would have marked the job as SUCCEEDED without confirming the fix actually worked |
| Skip recording | Next occurrence would require full re-diagnosis instead of instant recognition |
