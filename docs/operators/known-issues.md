# Known Issues and Limitations

This page documents known issues, limitations, and planned improvements in the current Globular release.

## CLI Command Gaps

The following CLI commands are referenced in documentation but not yet implemented. They are accessible via MCP tools or direct gRPC calls but lack CLI wrappers.

### Workflow Commands

| Documented Command | Status |
|-------------------|--------|
| `globular workflow list` | **Implemented** |
| `globular workflow get <run-id>` | **Implemented** |
| `globular workflow diagnose <run-id>` | **Implemented** |

**Note**: The workflow service runs on port 10004 (not the controller port). Use `--workflow localhost:10004` or configure the default.

### Node Commands

| Documented Command | Status |
|-------------------|--------|
| `globular node logs --unit <service>` | **Implemented** |
| `globular node search-logs --unit <service> --pattern <regex>` | **Implemented** |
| `globular node certificate-status` | **Implemented** |
| `globular node control --unit <service> --action restart` | **Implemented** |

### Backup Commands

| Documented Command | Status | Existing Alternative |
|-------------------|--------|---------------------|
| `globular backup run` | Use `globular backup create` | ✓ Exists |
| `globular backup validate` | Not implemented | Use MCP: `backup_validate_backup` |
| `globular backup preflight-check` | Not implemented | Use MCP: `backup_preflight_check` |
| `globular backup get-job` | Not implemented | Use MCP: `backup_get_job` |
| `globular backup list-jobs` | Not implemented | Use MCP: `backup_list_jobs` |
| `globular backup promote/demote` | Not implemented | Use MCP: direct gRPC |
| `globular backup schedule-status` | Not implemented | Use MCP: `backup_get_schedule_status` |
| `globular backup retention-status` | Not implemented | Use MCP: `backup_get_retention_status` |
| `globular backup restore-plan` | Not implemented | Use MCP: `backup_restore_plan` |
| `globular backup apply-recovery-seed` | Not implemented | Manual file placement |

### Auth Commands

| Documented Command | Status | Existing Alternative |
|-------------------|--------|---------------------|
| `globular auth set-password` | Use `globular auth root-passwd` | ✓ Exists (different name) |
| `globular auth create-account` | Not implemented | Use gRPC: `resource.ResourceService` |

### AI Commands

| Documented Command | Status | Existing Alternative |
|-------------------|--------|---------------------|
| `globular ai status` | ✓ Exists | |
| `globular ai list` | ✓ Exists | |
| `globular ai show <id>` | ✓ Exists | |
| `globular ai approve/deny/retry` | ✓ Exists | |
| `globular ai executor status/jobs` | Use `globular ai status/list` | ✓ Exists (different path) |
| `globular ai watcher status/pause/resume` | Not implemented | Use gRPC: `AiWatcherService` |
| `globular ai router status/policy/set-mode` | Not implemented | Use gRPC: `AiRouterService` |
| `globular ai memory store/query/get/list` | Not implemented | Use MCP: `memory_*` tools |

### Monitoring Commands

| Documented Command | Status | Workaround |
|-------------------|--------|------------|
| `globular metrics query` | Not implemented | Use MCP: `metrics_query` |
| `globular metrics targets` | Not implemented | Use MCP: `metrics_targets` |
| `globular metrics alerts` | Not implemented | Use MCP: `metrics_alerts` |

### Compute Commands

| Documented Command | Status | Notes |
|-------------------|--------|-------|
| `globular compute *` | Not implemented | compute_server not in build manifest (Phase 2+ feature) |

### Command Name Corrections

These commands exist but are documented with wrong names:

| Documented As | Actual Command |
|--------------|---------------|
| `globular auth set-password` | `globular auth root-passwd` |
| `globular backup run` | `globular backup create` |
| `globular cluster nodes set-profiles` | `globular cluster nodes profiles set` |
| `globular dns record set-txt` | `globular dns txt set` |

## Infrastructure Issues

### DNS Zone Persistence

**Status**: Fixed. DNS zones persist to ScyllaDB across restarts. All three DNS instances share the same ScyllaDB-backed store.

**Previous issue**: Zones appeared to be lost after restart because the CLI and MCP tools failed to authenticate to the DNS service (missing `cluster_id`). Domains set through those broken paths never persisted. When set directly via authenticated gRPC (grpcurl or the local DNS provider fix), persistence works correctly.

**If zones are missing after restart** (legacy installations):
```bash
# Set domains directly via grpcurl (authenticated)
grpcurl -cacert /var/lib/globular/pki/ca.crt \
  -cert /var/lib/globular/pki/issued/services/service.crt \
  -key /var/lib/globular/pki/issued/services/service.key \
  -d '{"domains": ["globular.internal.", "yourdomain.com."]}' \
  localhost:10006 dns.DnsService/SetDomains
```

### Split-Horizon DNS

**Issue**: The Globular DNS service cannot serve different answers for internal vs external queries. Consumer routers with hairpin NAT limitations require `/etc/hosts` overrides on each cluster node.

**Workaround**: Add entries to `/etc/hosts` on each node:
```
10.0.0.100 globular.io www.globular.io
```

**Planned fix**: Implement DNS views or a local resolver override in the node agent.

### ACME Certificate Path Mismatch

**Issue**: The domain reconciler writes certs to `/var/lib/globular/domains/{domain}/` but the xDS server reads from `/var/lib/globular/config/tls/acme/{domain}/`. A symlink is required.

**Workaround**: Create symlink after first cert issuance:
```bash
sudo ln -sfn /var/lib/globular/domains/{domain} /var/lib/globular/config/tls/acme/{domain}
```

**Planned fix**: Unify cert paths — reconciler should write directly to the xDS-expected path.

## Build and Release

### GitHub Releases

**Status**: Release workflow implemented (`.github/workflows/release.yml`). Push a Git tag (`v0.1.0`) to trigger automated build and release.

**To create a release**:
```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds everything and uploads `globular-0.1.0-linux-amd64.tar.gz` with SHA256 checksums to GitHub Releases.

### All Service Versions Are 0.0.1

**Issue**: No semantic versioning. All services are version `0.0.1` regardless of actual maturity or changes.

**Planned fix**: Derive version from Git tags. `v0.1.0` tag → all packages version `0.1.0`.

### compute_server Not Built

**Issue**: The compute service code exists (`golang/compute/compute_server/`) but is not in `golang/build/services.list` and is not compiled or packaged.

**Status**: Intentional — Phase 2+ feature. The compute documentation describes what the code does but marks it as not yet deployed.

## Security

### Bootstrap Flag Not Auto-Cleaned

**Issue**: The bootstrap flag file (`/var/lib/globular/bootstrap.enabled`) expires after 30 minutes but the file remains on disk. While the expiry is checked (so the window is correctly enforced), the stale file could cause confusion.

**Planned fix**: Remove the file after expiry is detected.

### Local DNS Provider Authentication

**Issue**: The local DNS provider (`dnsprovider/local/`) previously connected without cluster authentication, causing DNS-01 ACME challenges to fail with "cluster_id required".

**Status**: Fixed in this session. The provider now reads the cluster domain and local token for authentication.
