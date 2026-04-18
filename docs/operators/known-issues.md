# Known Issues

This page is divided into four buckets. Each has a different character and a different fix timeline.

- **Product limitations** — things the platform cannot currently do, by design or by incomplete implementation
- **Infrastructure bugs** — real defects with workarounds that work today
- **CLI gaps** — commands that are missing, named wrong, or not yet wired up
- **Documentation mismatches** — places where the docs describe something differently than the code actually does

If you hit something not listed here, check [Platform Status](platform-status.md) for a broader view, or open an issue.

---

## Product limitations

These are things the platform genuinely cannot do today. They are not CLI gaps or bugs — they are missing capabilities.

### Single-node has no data redundancy

A single-node install runs etcd, ScyllaDB, and MinIO without any replication. If the node fails, data is lost. This is acceptable for development and testing. It is not acceptable for anything you care about.

**Minimum for resilience**: 3 nodes with the `core`, `control-plane`, and `storage` profiles. See [Adding Nodes](adding-nodes.md).

### Split-horizon DNS is not supported

The Globular DNS service cannot return different answers based on whether the query comes from inside or outside the cluster. This means consumer routers with hairpin NAT (where `mycluster.example.com` resolves to the VIP, and the VIP cannot reach back to itself from inside) break without a workaround.

**Workaround**: Add the VIP-to-domain mapping in `/etc/hosts` on every cluster node:
```
10.0.0.100  globular.io www.globular.io
```

**Planned**: DNS views or a node-local resolver override. No timeline.

### ACME cert path mismatch (Let's Encrypt)

The domain reconciler writes Let's Encrypt certificates to `/var/lib/globular/domains/{domain}/fullchain.pem`. The xDS server reads from `/var/lib/globular/config/tls/acme/{domain}/`. These are two different paths. A symlink is required after the first cert is issued.

**Workaround**:
```bash
sudo mkdir -p /var/lib/globular/config/tls/acme/
sudo ln -sfn /var/lib/globular/domains/yourdomain.com \
             /var/lib/globular/config/tls/acme/yourdomain.com
```

**Planned**: Unify the cert output path so the reconciler writes directly where xDS expects it.

### Compute service not deployed

The `compute_server` code exists at `golang/compute/compute_server/` and is documented. It is intentionally not built or packaged — it is a Phase 2+ feature. The compute documentation describes the design, not a running system.

**Current status**: Code only. Not in the build manifest. Not installable.

### No GitHub release tarball yet

The getting-started guide references a downloadable release tarball at `github.com/globulario/services/releases`. That release does not exist yet. The release pipeline is implemented (`.github/workflows/release.yml`) and will produce the tarball when a Git tag is pushed. Until then, you must build from source.

**Fix**: Push `git tag v0.1.0` to trigger the release build. See [Building from Source](building-from-source.md).

### Service versions all show 0.0.1 (or similar)

All Globular service packages currently publish as version `0.0.1` (or whatever was last manually bumped). There is no automated version derivation from Git tags. The version injection via `--ldflags` at build time exists but is not wired into a CI/CD pipeline yet.

**Planned**: CI pipeline derives version from Git tag. `v0.1.0` → all packages built as `0.1.0`. Pending containerized test cluster.

### Bootstrap flag file not cleaned up

After the 30-minute bootstrap window expires, the file `/var/lib/globular/bootstrap.enabled` stays on disk. The expiry logic is correct — the window is properly enforced — but the stale file can cause confusion when auditing the filesystem.

**Planned**: Delete the file when expiry is first detected.

---

## Infrastructure bugs

These are real defects. They have working workarounds.

### DNS zones missing after restart (legacy installations only)

**Symptom**: Domains that were registered before a specific fix are missing after restart. DNS queries for `globular.internal` return NXDOMAIN.

**Cause**: Older versions of the CLI and MCP DNS tools connected without the cluster's `cluster_id` in the request. Zones set through those broken paths were never persisted — they went into a non-persisted in-memory state. The DNS service correctly rejects unauthenticated writes, which meant the write silently failed.

**Status**: Fixed. The DNS provider now reads the cluster domain and authenticates correctly. New zones persist to ScyllaDB and survive restarts.

**If you have legacy missing zones**, re-register them:
```bash
grpcurl \
  -cacert /var/lib/globular/pki/ca.crt \
  -cert /var/lib/globular/pki/issued/services/service.crt \
  -key /var/lib/globular/pki/issued/services/service.key \
  -d '{"domains": ["globular.internal.", "yourdomain.com."]}' \
  localhost:10006 dns.DnsService/SetDomains
```

### Envoy fails to pick up routes after xDS restarts out of order

**Symptom**: Port 443 accepts connections but returns nothing, or Envoy serves stale routes after an xDS restart.

**Cause**: Envoy's xDS client has a reconnect window. If Envoy starts before xDS has loaded its route configuration, or if xDS restarts while Envoy is mid-update, Envoy may hold a stale or empty route table.

**Workaround**: Always restart xDS before Envoy, with a pause:
```bash
sudo systemctl restart globular-xds
sleep 10
sudo systemctl restart globular-envoy
```

**Do not** stop both simultaneously — the cluster will serve no traffic during the window.

### ScyllaDB schema migration can corrupt on split-brain restart

**Symptom**: Services that use ScyllaDB (repository, AI memory, DNS) fail to start with schema errors after a cluster restart where nodes came back in different orders.

**Cause**: Schema migration runs a 3-phase process (check → apply → validate). If two nodes both decide to run migration simultaneously on the same keyspace, the second can see partial state from the first.

**Workaround**: If you see migration errors, restart the affected services one at a time, in the order: dns → repository → ai-memory. The second and third will see that migration already ran and skip it.

**Status**: The 3-phase migration logic exists. The distributed coordination (leader-only migration dispatch) is on the roadmap.

---

## CLI gaps

Commands that are documented, referenced, or expected but do not have a CLI wrapper yet. The underlying gRPC service exists in all cases — the gap is purely in the CLI layer.

### Missing commands (use MCP or gRPC instead)

| Expected Command | Workaround |
|-----------------|-----------|
| `globular backup validate` | MCP: `backup_validate_backup` |
| `globular backup preflight-check` | MCP: `backup_preflight_check` |
| `globular backup schedule-status` | MCP: `backup_get_schedule_status` |
| `globular backup retention-status` | MCP: `backup_get_retention_status` |
| `globular backup restore-plan` | MCP: `backup_restore_plan` |
| `globular backup get-job / list-jobs` | MCP: `backup_get_job`, `backup_list_jobs` |
| `globular ai watcher status/pause/resume` | gRPC: `AiWatcherService` |
| `globular ai router status/policy/set-mode` | gRPC: `AiRouterService` |
| `globular ai memory store/query/get/list` | MCP: `memory_store`, `memory_query`, etc. |
| `globular metrics query/targets/alerts` | MCP: `metrics_query`, `metrics_targets`, `metrics_alerts` |
| `globular compute *` | Not implemented. Service not built. |
| `globular auth create-account` | gRPC: `resource.ResourceService` |

### Commands with wrong names in some docs

Some older documentation uses names that differ from the actual CLI:

| Documented As | Actual Command |
|--------------|---------------|
| `globular auth set-password` | `globular auth root-passwd` |
| `globular backup run` | `globular backup create` |
| `globular cluster nodes set-profiles` | `globular cluster nodes profiles set` |
| `globular dns record set-txt` | `globular dns txt set` |

If a command returns "unknown command", try `globular <subcommand> --help` to see what is actually available.

---

## Documentation mismatches

Places where the docs describe something differently than the implementation. These are not bugs in the software — they are bugs in the documentation.

### `getting-started.md` Step 2 uses the wrong command name

The guide says `globular auth set-password`. The actual command is `globular auth root-passwd`. The guide has been updated, but older copies of the documentation or cached pages may still show the wrong name.

### Workflow service port listed inconsistently

Some older docs reference the workflow service on port 13000. The current port is **10004**. When using `--workflow` flags or connecting directly, use `globular.internal:10004`.

### Certificate path `/etc/globular/creds/` does not exist

Some older internal notes and early documentation reference `/etc/globular/creds/` for certificates. This path does not exist and never did. All certificates are under `/var/lib/globular/pki/`. Specifically:

| What | Path |
|------|------|
| CA certificate | `/var/lib/globular/pki/ca.crt` |
| CA private key | `/var/lib/globular/pki/ca.key` |
| Service certificate | `/var/lib/globular/pki/issued/services/service.crt` |
| Service private key | `/var/lib/globular/pki/issued/services/service.key` |

### Controller port 10000 is wrong

Some early documentation lists the cluster controller on port 10000. The correct port is **12000**. The node agent is on **11000**.

### `globular services repair` is referenced but removed

Several older docs reference `globular services repair --dry-run`. This command has been removed. The repair functionality moved into the workflow model:
- For drift detection: `globular cluster get-drift-report`
- For targeted repair: `globular node repair` (dispatches `node.repair` workflow)
- For doctor-driven auto-repair: `globular doctor heal`
