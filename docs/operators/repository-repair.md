# Repository and Cluster State Repair

This guide covers how to diagnose and repair repository, desired-state, and installed-state inconsistencies in a Globular cluster. It is based on the Phase 2 version control model where every artifact has a repository-issued `build_id` as its sole authoritative identity.

## When to Use This Guide

Use these tools when:

- Services report as installed but are running the wrong version
- Desired-state and installed-state disagree
- The repository contains duplicate or conflicting artifacts
- Ghost nodes appear in health reports
- Day-0 packages need to be imported into the repository
- A convergence loop repeats without converging

## Identity Model

Every repository-managed artifact has:

| Field | Role | Authority |
|-------|------|-----------|
| `build_id` | Exact artifact identity | Repository-allocated (UUIDv7) |
| `version` | Human-facing label | Publisher-declared, repository-validated |
| `digest` | Content fingerprint | Repository-computed (SHA256) |
| `build_number` | Display counter | Repository-derived (informational only) |

**The rule:** convergence compares `build_id` only. Version and build_number are for humans. If two records have the same `build_id`, they are the same artifact. If they differ, they are different.

## Diagnostic Commands

### Scan cluster state

```bash
# Full scan: repository + desired-state + installed-state
globular state canonicalize --dry-run

# Scan repository artifacts only
globular repository scan

# Scan a specific package
globular repository scan --package dns
```

### Anomaly types

| Code | Name | Meaning |
|------|------|---------|
| A1 | Stale installed-state | Failed record from unreachable repository (drift reconciler artifact) |
| A2 | Missing desired build_id | Desired-state record lacks `build_id` |
| A3 | Missing installed build_id | Installed-state record lacks `buildId` |
| A4 | Missing repo build_id | Repository manifest lacks `build_id` |
| A7 | Inconsistent coverage | Some nodes have `buildId` for a service, others don't |

### Repository artifact classifications

| Classification | Meaning |
|---------------|---------|
| VALID | Consistent with all invariants |
| DUPLICATE_DIGEST | Same version, same content, different build number (idempotent re-upload) |
| DUPLICATE_CONTENT | Same version, different content (overwritten during iterative development) |
| ORPHANED | Not referenced by any desired or installed state |
| MISSING_BUILD_ID | Manifest lacks `build_id` (pre-Phase-2 artifact) |

## Repair Procedures

### 1. Repair desired-state (A2 anomalies)

Fixes desired-state records that lack `build_id` by re-upserting through the controller API. The controller resolves `build_id` from the repository manifest.

```bash
globular state canonicalize --fix-safe
```

**What it does:**
- Reads each `ServiceDesiredVersion` from etcd
- If `build_id` is empty, calls `UpsertDesiredService` on the controller leader
- Controller queries repository for the artifact manifest and writes `build_id`
- No service restarts, no installed-state changes

**Safe to run:** Yes. Idempotent. Only mutates desired-state metadata.

### 2. Repair installed-state (A3 anomalies)

Fixes installed-state records that lack `buildId` by re-applying the service or writing metadata directly.

**For SERVICE-kind packages (re-apply path):**
```bash
# Repair all standard services on a node
globular state canonicalize --fix-installed \
  --node <node-id> \
  --agent-endpoint <ip:port>

# Repair a specific service
globular state canonicalize --fix-installed \
  --node <node-id> \
  --agent-endpoint <ip:port> \
  --service dns

# Include control-plane services (dns, workflow, etc.)
globular state canonicalize --fix-installed \
  --node <node-id> \
  --agent-endpoint <ip:port> \
  --include-critical
```

**For COMMAND/INFRASTRUCTURE packages (metadata-only):**
```bash
globular state canonicalize --fix-installed \
  --node <node-id> \
  --agent-endpoint <ip:port> \
  --metadata-only
```

**What it does (re-apply):**
- Reads desired-state `build_id` for each service
- Calls `ApplyPackageRelease` with `build_id` and `force=true`
- Node-agent installs the package, restarts the service, verifies it's active
- Writes installed-state with `buildId` only after the service is confirmed running

**What it does (metadata-only):**
- Reads the existing etcd record for the installed package
- Adds `buildId` to the JSON without reinstalling or restarting
- Used for packages that can't be re-applied through the standard path

### 3. Clean up ghost nodes

Removes installed-state records for nodes that are no longer in the active cluster.

```bash
globular state canonicalize --cleanup-ghosts
```

**What it does:**
- Queries the controller for active node list
- Deletes etcd records under `/globular/nodes/{id}/packages/` for non-active nodes
- Each deletion produces an audit record at `/globular/audit/`

### 4. Repair repository manifests (A4 anomalies)

Fixes repository manifests that lack `build_id`. This happens automatically on repository restart (the backfill migration is idempotent).

```bash
# Restart the repository service on the affected node
sudo systemctl restart globular-repository.service
```

The `MigrateBuildIDs()` function runs at startup and assigns deterministic UUIDv5 build_ids to any manifest missing one.

## Recommended Repair Order

When repairing a cluster from scratch:

1. **Repository first** — restart all repository instances to trigger build_id backfill
2. **Desired-state** — run `--fix-safe` to populate build_id in all desired-state records
3. **Installed-state (services)** — run `--fix-installed` node by node (dell first, then nuc, then ryzen)
4. **Installed-state (metadata)** — run `--metadata-only` for COMMAND/INFRASTRUCTURE packages
5. **Ghost cleanup** — run `--cleanup-ghosts` to remove stale node records
6. **Verify** — run `--dry-run` and confirm anomaly count is at or near zero

## Audit Trail

Every repair mutation produces an audit record in etcd at `/globular/audit/`:

```json
{
  "timestamp": "2026-04-16T23:45:00Z",
  "action": "fix-installed-metadata",
  "service": "etcd",
  "node": "4c2b3cb3-...",
  "before_state": "buildId=empty",
  "after_state": "buildId=26df5b38-...",
  "build_id": "26df5b38-...",
  "detail": "kind=INFRASTRUCTURE",
  "operator": "canonicalize-tool"
}
```

To query the audit log:

```bash
etcdctl get /globular/audit/ --prefix --limit 20
```

## Convergence Truth Model

The repair tools enforce the convergence truth invariant:

```
desired.build_id == installed.build_id  →  converged
desired.build_id != installed.build_id  →  needs apply
```

No version comparison, no build_number comparison, no hash computation. A single string equality is the only convergence check.

**Additionally:**
- Node-agent blocks on restart until the service is confirmed active (`systemctl is-active`)
- Installed-state is written only AFTER the service is running — never before
- Self-update (node-agent) uses an external upgrader process in its own cgroup to survive the restart

## Package Classification

Not all packages are desired-state-managed:

| Classification | Example | Desired-state? | Canonicalizable? |
|---------------|---------|----------------|-----------------|
| Managed + desired-state | dns, workflow, rbac | Yes | Yes (full) |
| Managed + metadata-only | mc, docs | No | Yes (metadata-only) |
| Infrastructure | etcd, minio, scylladb | InfrastructureRelease | Yes (metadata-only) |
| Ghost/stale | removed node records | N/A | Delete via --cleanup-ghosts |

The canonicalization tool automatically handles this: it reads desired-state from both `ServiceDesiredVersion` and `InfrastructureRelease` records, and falls back to repository manifest lookup for packages without desired-state entries.

## Preventive Measures

The following invariants prevent future inconsistencies:

1. **INV-1**: RELEASED artifacts are immutable — upload with same version + different digest is rejected
2. **INV-2**: Release versions are monotonic — uploading version 0.0.2 after 0.0.8 is rejected
3. **INV-3**: `build_id` is the sole artifact identity — generated by repository, never client-supplied
4. **INV-6**: Desired-state requires repository confirmation — writing a non-existent version is rejected
5. **INV-7**: Only PUBLISHED artifacts are installable — node-agent checks publish state before applying
6. **INV-10**: Repairs produce audit records — no silent history rewriting
