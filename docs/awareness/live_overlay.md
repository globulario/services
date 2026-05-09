# Live Overlay — Fresh vs Stale vs Absent

The awareness graph is rebuilt from static sources (Go AST, YAML, proto, scripts).
The **live overlay** adds runtime evidence that the static graph cannot provide:
systemd unit states, etcd key presence, PKI cert validity, RBAC bindings.

---

## Why It Exists

Static knowledge answers: "does this invariant have an implementation?"  
Live overlay answers: "is that implementation actually running right now?"

Without the live overlay, preflight cannot detect:
- A service that was deployed but never started
- A cert that expired since the last graph rebuild
- An etcd key that was deleted by a recovery procedure

---

## Architecture

```
globular awareness live-snapshot
  ├── systemd collector   → NodeTypeSystemdUnit (status: active/failed/inactive)
  ├── pki collector       → NodeTypeCertificate (validity window, SANs)
  ├── rbac collector      → NodeTypeRBACBinding (actual cluster role bindings)
  └── etcd presence       → NodeTypeEtcdKey (key exists/absent in etcd)

Persisted to: graph_builds table, id = "live-snapshot"
              collector health in build_collector_health table
```

The live snapshot uses a fixed build ID (`live-snapshot`) that always overwrites
the previous snapshot. `LatestBuildRecord()` excludes it — static build stats are
never mixed with live overlay stats.

---

## Freshness TTLs

| Status | Condition | Effect on preflight |
|--------|-----------|---------------------|
| `fresh` | age < 300s | Preflight uses live evidence |
| `stale` | 300s ≤ age < 900s | Preflight warns: "live overlay stale" |
| `absent` | No snapshot ever recorded | Preflight adds blind spot: "live overlay absent" |
| `partial` | Some collectors failed | Preflight includes partial evidence + warning |
| `failed` | All collectors failed | Treated as absent |

---

## Collector IDs and Source Tiers

| Collector | Source Tier | What it collects |
|-----------|-------------|-----------------|
| `systemd` | `systemd_runtime` | Unit active/failed states for all Globular services |
| `pki` | `installed_metadata` | Certificate expiry and SAN coverage |
| `etcd` | `etcd_runtime` (opt-in) | Presence of specific etcd keys |
| `pki_rbac` | `cluster_security` | RBAC binding state from etcd |
| `workflow_execution` | `live_runtime` | Recent workflow run outcomes |

---

## Scheduling

The live snapshot should run every 5 minutes in production.

```bash
# Install systemd timer:
cp docs/awareness/systemd/awareness-live-snapshot.service /etc/systemd/system/
cp docs/awareness/systemd/awareness-live-snapshot.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now awareness-live-snapshot.timer
```

Timer defaults: `OnBootSec=2min`, `OnUnitActiveSec=5min`.

---

## Interpreting health_pulse live_overlay Section

```json
{
  "live_overlay": {
    "status": "partial",
    "age_seconds": 180,
    "collected_at": "2026-05-08T23:00:00Z",
    "collectors": [
      { "id": "systemd", "status": "ok", "nodes_emitted": 24, "priority": "P0" },
      { "id": "pki", "status": "skipped", "priority": "P1" },
      { "id": "workflow_execution", "status": "error", "error": "etcd unavailable", "priority": "P1" }
    ]
  }
}
```

- `partial` means at least one collector ran OK and at least one failed.
- `skipped` means the collector was not configured or not enabled for this run.
- `error` means the collector ran but produced no data due to a runtime failure.

A `partial` status with all P0 collectors OK is acceptable. A `partial` status
with a P0 collector `error` is a blind spot and must be investigated.
