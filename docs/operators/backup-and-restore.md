# Backup and Restore

This page covers Globular's backup and restore system: how backups work, which providers are available, how to schedule and manage backups, how to restore from backup, and how to prepare for disaster recovery.

## Backup Architecture

Globular's Backup Manager orchestrates backups across multiple providers and multiple nodes. It does not backup data directly — it coordinates providers that handle specific data stores.

```
Backup Manager (central coordinator)
    │
    ├── etcd provider     → etcdctl snapshot save  (cluster state)
    ├── restic provider   → restic backup           (files + MinIO object data)
    ├── scylla provider   → sctool backup           (database → MinIO bucket)
    └── minio provider    → rclone sync             (object store, optional)

    Fan-out to Node Agents:
    ├── Node 1: run provider locally → upload result
    ├── Node 2: run provider locally → upload result
    └── Node 3: run provider locally → upload result
```

### Backup Providers

Globular supports four backup providers:

**etcd** — Snapshot-based backup of the etcd cluster state. Uses `etcdctl snapshot save` to create a consistent point-in-time snapshot. This captures all cluster configuration, desired state, service endpoints, and node records. The full snapshot is embedded directly in the artifact capsule — it is self-contained.

**restic** — File-level incremental backup using restic. Backs up the configured paths (default: `/var/lib/globular`) with deduplication and encryption. Crucially, this covers `/var/lib/globular/minio/data`, which means the restic backup includes all MinIO object data — including the ScyllaDB backup stored in the `globular-backups` bucket. This is the foundation of local disaster recovery.

**scylla** — Database backup via ScyllaDB Manager (`sctool`). Creates distributed snapshots of ScyllaDB tables and uploads them to a MinIO/S3 location. Polls task status until completion. The backup data lives in the MinIO bucket; the artifact capsule stores the snapshot tag and task metadata needed to restore.

**minio** — Object store synchronization using rclone. Replicates MinIO bucket contents to a configured rclone remote. Requires `RcloneRemote` to be set. Optional — MinIO data is also covered by the restic provider.

### Backup Destinations

Backups can be stored in five types of destinations:

| Destination | Description | Use Case |
|-------------|-------------|----------|
| `local` | Local filesystem path | Development, single-node clusters |
| `minio` | S3-compatible bucket | Standard production backups |
| `nfs` | NFS mount (treated as local) | Network-attached storage |
| `s3` | AWS S3 or compatible (via rclone) | Off-site cloud backup |
| `rclone` | Any rclone-supported remote | 40+ cloud providers |

Multi-destination replication: the artifact capsule is replicated to all configured destinations after every successful backup.

### What Is Stored Where

After a successful cluster backup, data lives in three places:

| Data | Location | Self-contained? |
|------|----------|-----------------|
| etcd snapshot | Inside artifact capsule (`payload/etcd/etcd-snapshot.db`) | ✅ Yes |
| Files (`/var/lib/globular`) | Restic repository (e.g. `/var/backups/globular/restic`) | ✅ Yes (local) |
| MinIO objects | Inside restic repo (covered by `ResticPaths`) | ✅ Yes (via restic) |
| ScyllaDB data | MinIO bucket (`s3:globular-backups`, snapshot tag in capsule) | Needs MinIO |
| Artifact capsule | Local + all configured remote destinations | ✅ Replicated |

The **restic repository is the local complete copy** — it contains everything under `/var/lib/globular`, including MinIO object data. If the restic repo survives, full recovery is possible without any external dependency.

## Recovery Capsule

Every successful backup automatically generates a `recovery-capsule/` directory inside the artifact. It contains everything needed to bootstrap a restore, including the case where MinIO is unavailable.

### Contents

```
artifacts/<backup-id>/
  payload/etcd/etcd-snapshot.db     ← full etcd snapshot (self-contained)
  recovery-capsule/
    restore-inputs.json             ← machine-readable restore parameters
    restore.sh                      ← full-cluster restore orchestrator
    phase1-restore-files.sh         ← restore /var/lib/globular via restic
    phase2-bootstrap-minio.sh       ← (re)start MinIO after files are restored
    phase3-restore-etcd.sh          ← restore etcd from snapshot
    phase4-restore-scylla.sh        ← restore ScyllaDB via sctool
    README.md                       ← human-readable recovery guide
```

The `restore-inputs.json` captures all parameters generated at backup time:

```json
{
  "backup_id": "556d758e-af6f-4aa4-b961-8d6f4df2b08f",
  "domain": "globular.internal",
  "etcd":   { "snapshot_file": "payload/etcd/etcd-snapshot.db", "data_dir": "/var/lib/globular/etcd" },
  "restic": { "repo": "/var/backups/globular/restic", "snapshot_id": "306bb92a", "paths": "/var/lib/globular" },
  "scylla": { "cluster": "globular.internal", "snapshot_tag": "sm_20260418195257UTC", "locations": "s3:globular-backups" },
  "minio":  { "endpoint": "minio.globular.internal:9000", "data_path": "/var/lib/globular/minio/data" }
}
```

### Recovery Chain (MinIO Unavailable)

The recovery capsule is specifically designed for the worst-case scenario where MinIO is gone but the local restic repository survives:

```
restic repo → restore /var/lib/globular → MinIO data back on disk
  → start MinIO → ScyllaDB backup accessible in globular-backups bucket
  → restore etcd → cluster desired-state restored
  → sctool restore → ScyllaDB data restored
  → start globular-cluster-controller → services converge
```

### Two Complete Recovery Sources

| Source | What it contains | When to use |
|--------|-----------------|-------------|
| **Local** (restic repo + capsule) | etcd snapshot, all files, MinIO objects | Disk intact, MinIO gone |
| **MinIO** (bucket + capsule replica) | etcd snapshot, ScyllaDB backup, artifact metadata | Local disk gone, MinIO intact |

To make MinIO a fully self-contained source (restic repo synced to MinIO), enable `SyncResticRepoToRemote`:

```bash
# Enable via etcd config (set in backup-manager service config)
# "SyncResticRepoToRemote": true
#
# After each backup, rclone syncs the restic repo to:
# globular-backups/restic-repo/
# Subsequent runs only upload the diff (incremental).
```

## Running Backups

### On-Demand Backup

Run a backup manually:

```bash
# Full cluster backup (all providers)
globular backup create --mode cluster

# Single-service backup
globular backup create --mode service --provider etcd

# With specific destination
globular backup create --mode cluster --destination minio

# Wait for completion before returning
globular backup create --mode cluster --wait
```

What happens during a cluster backup:

1. **Pre-flight check**: Verify all providers are available (etcdctl, restic, rclone, sctool binaries present)
2. **Acquire cluster lock**: Distributed lock prevents concurrent cluster backups
3. **Capture topology**: Record which nodes and services are active at backup time
4. **Quiesce hooks**: For each service offering `BackupHookService`, call `PrepareBackup`:
   - Service declares its local datasets with metadata (data class, scope, size)
   - Data classes: AUTHORITATIVE (must backup), REBUILDABLE (can regenerate), CACHE (skip)
5. **Provider execution**: Each provider runs sequentially:
   - etcd: snapshot save → embedded in capsule
   - scylla: sctool backup → data uploaded to MinIO, snapshot tag stored in capsule
   - restic: incremental backup of `/var/lib/globular` → stored in restic repo
6. **Finalize hooks**: Call `FinalizeBackup` on each service to resume operations
7. **Recovery capsule**: Generate `recovery-capsule/` with restore scripts and `restore-inputs.json`
8. **Seal + replicate**: Write manifest, replicate capsule to all configured destinations
9. **Phase 2 copy** *(if `SyncResticRepoToRemote` enabled)*: Sync restic repo to remote so the remote destination is fully self-contained
10. **Retention**: Run retention policy to expire old backups

### Backup Jobs

Backups run as jobs with lifecycle tracking:

```bash
# List recent jobs
globular backup jobs list
# Output:
# JOB ID      TYPE     STATUS      STARTED             DURATION  BACKUP ID
# bk-job-001  backup   SUCCEEDED   2025-04-12 03:00    4m 22s    bk-abc123
# bk-job-002  backup   SUCCEEDED   2025-04-11 03:00    4m 18s    bk-def456
# bk-job-003  backup   FAILED      2025-04-10 14:00    1m 02s

# Filter by state
globular backup jobs list --state failed

# Job details (per-provider status, timing, replication results)
globular backup jobs get --job-id bk-job-001

# Cancel a running job
globular backup jobs cancel --job-id bk-job-001

# Delete a completed job record
globular backup jobs delete --job-id bk-job-001
```

Job states: `QUEUED` → `RUNNING` → `SUCCEEDED` / `FAILED`

The backup manager limits concurrent jobs (default: 1) via a semaphore. If a backup is already running, new requests queue.

### Scheduled Backups

Backups can be scheduled for automatic execution:

```bash
# View schedule
globular backup schedule status
# Shows: next fire time, interval, enabled status
```

Schedule is configured via `ScheduleInterval` in the service config (e.g. `"24h"`, `"6h"`, `"weekly"`). Set to `"0"` to disable.

## Managing Backups

### Listing Backups

```bash
# List completed backups
globular backup list
# Output:
# BACKUP ID   DATE                PROVIDERS            SIZE      QUALITY
# bk-001      2025-04-12 03:00    etcd,scylla,restic   14.1 GB   VALIDATED
# bk-002      2025-04-11 03:00    etcd,scylla,restic   14.0 GB   RESTORE_TESTED
# bk-003      2025-04-10 03:00    etcd,scylla,restic   13.9 GB   UNVERIFIED
```

### Backup Details

```bash
globular backup get --backup-id bk-001
# Shows:
# - Timestamp, size
# - Per-provider results (etcd: snapshot hash/revision, restic: snapshot ID, scylla: snapshot tag)
# - Destination(s) and replication status
# - Quality state and labels
# - Recovery capsule location
```

### Validating Backups

Verify backup integrity:

```bash
# Shallow validation (checksums, metadata)
globular backup validate --backup-id bk-001

# Deep validation (decompress, verify all providers)
globular backup validate --backup-id bk-001 --deep
```

### Quality States

Backups progress through quality states:

```
UNVERIFIED → VALIDATED → RESTORE_TESTED → PROMOTED
```

- **UNVERIFIED**: Just completed, not yet validated
- **VALIDATED**: Passed integrity check (checksums match, all providers present)
- **RESTORE_TESTED**: Successfully restored in a sandbox environment
- **PROMOTED**: Manually promoted — protected from retention cleanup

Promote a backup to protect it from automatic deletion:

```bash
globular backup promote --backup-id bk-001
```

Demote to remove protection:

```bash
globular backup demote --backup-id bk-001
```

Run a restore test to advance quality state:

```bash
# Light: metadata checks only (fast)
globular backup test --backup-id bk-001

# Heavy: actual sandbox restore (promotes to RESTORE_TESTED on success)
globular backup test --backup-id bk-001 --level heavy
```

### Retention Policy

Automatic cleanup prevents unbounded backup growth:

```bash
# View retention status and current policy
globular backup retention status
# Shows:
# - Policy: keep_last_n=30, keep_days=90, max_total_bytes=100GB
# - Current: 28 backups, 56 GB
# - Oldest/newest timestamps

# Preview what would be deleted
globular backup retention run --dry-run

# Run retention (delete expired backups)
globular backup retention run
```

Retention rules:
- `KeepLastN`: Always keep the N most recent backups
- `KeepDays`: Keep backups from the last N days
- `MaxTotalBytes`: Maximum total storage (oldest deleted first)
- `MinRestoreTestedToKeep`: Minimum number of restore-tested backups to preserve

Retention runs automatically after each backup job. PROMOTED backups are never deleted by retention.

## Restore Operations

### Restore Plan

Before restoring, preview what will happen:

```bash
globular backup restore --backup-id bk-001 --dry-run
# Output:
# Restore plan for backup bk-001:
#
#   1. Stop affected services
#   2. Restore etcd snapshot to /var/lib/globular/etcd/
#   3. Restore restic snapshot to /var/lib/globular/
#   4. Restore ScyllaDB from snapshot tag sm_20260418195257UTC
#
# Warnings:
#   [WARN] etcd-replace: etcd restore will REPLACE current cluster state
```

### Running a Restore

```bash
# Full restore (all providers)
globular backup restore --backup-id bk-001

# Restore etcd only
globular backup restore --backup-id bk-001 --etcd

# Restore etcd and config files only
globular backup restore --backup-id bk-001 --etcd --config

# Force restore (bypass safety checks)
globular backup restore --backup-id bk-001 --force
```

What happens during restore:

1. **Validation**: Verify the backup is intact (checksums, all providers present)
2. **Stop services**: Stop affected services to prevent data corruption during restore
3. **Provider restore**: Each provider restores its data:
   - etcd: `etcdutl snapshot restore` to data directory
   - restic: `restic restore` to original paths
   - scylla: `sctool restore` from snapshot tag in MinIO
4. **Start services**: Restart services in dependency order
5. **Verify**: Run health checks on restored services
6. **Seed**: `globular services seed` to ensure desired state matches installed state

### Node-Level Restore

The Backup Manager can execute restores on specific nodes via the Node Agent:

```bash
# The backup manager calls RunRestoreProvider on the target node's agent
# The agent executes the provider locally and reports results
# Results are polled via GetRestoreTaskResult
```

This enables restoring individual node state without affecting the rest of the cluster.

## Disaster Recovery

### Recovery Seed

For complete disaster recovery (cluster wiped, starting from scratch), Globular supports a recovery seed:

```bash
# Check disaster recovery readiness
globular backup recovery status

# Save recovery configuration to disk
globular backup recovery seed
```

The recovery seed persists backup destination configuration and credentials to:
```
/var/lib/globular/backups/settings.json
```

This file survives a cluster wipe. When a node starts with `RecoveryMode` enabled, it reads this file to locate and restore from the most recent backup.

### Pre-Flight Check

Verify backup tool availability before relying on backups:

```bash
globular backup preflight
# Output:
# TOOL                              STATUS  VERSION                        PATH
# etcdctl                           OK      etcdctl version: 3.5.14        /usr/local/bin/etcdctl
# restic                            OK      restic 0.18.1                  /usr/local/bin/restic
# rclone                            OK      rclone v1.73.1                 /usr/local/bin/rclone
# sctool                            OK      Client version: 3.8.1          /usr/lib/globular/bin/sctool
# scylla_cluster_detected           OK      globular.internal
# recovery_destination_configured   OK
```

If any tool is missing, the corresponding provider will not function. Install missing tools before relying on that provider.

## Practical Scenarios

### Scenario 1: Setting Up Daily Backups

Configure automatic daily cluster backups:

```bash
# Verify tools are available
globular backup preflight

# Run a test backup
globular backup create --mode cluster --wait

# Verify it succeeded and inspect the recovery capsule
globular backup list
globular backup get --backup-id <backup-id>

# Validate integrity
globular backup validate --backup-id <backup-id>

# Run a restore test to advance quality state
globular backup test --backup-id <backup-id> --level heavy

# Save recovery seed for disaster recovery
globular backup recovery seed

# Store a copy of settings.json and the recovery capsule offsite
# Location: /var/lib/globular/backups/settings.json
# Capsule:  /var/backups/globular/artifacts/<backup-id>/recovery-capsule/
```

### Scenario 2: Restoring After etcd Corruption

etcd data is corrupted on one node:

```bash
# Check cluster health — etcd reports unhealthy on node-2
globular cluster health

# Option 1: If other etcd members are healthy, remove and re-add node-2
# (etcd will automatically replicate data)
globular cluster nodes remove <node-2-id>
# Re-join node-2 and let it sync

# Option 2: Restore etcd from backup on the affected node
globular backup restore --backup-id <latest-backup-id> --etcd

# Verify
globular cluster health
```

### Scenario 3: Full Cluster Disaster Recovery (Globular-Managed)

All nodes are lost. Recovering from backup using the Globular CLI:

```bash
# 1. Provision new hardware, install Globular binaries

# 2. Copy recovery seed to /var/lib/globular/backups/settings.json

# 3. Start node agent
sudo systemctl start globular-node-agent

# 4. List available backups from the configured destination
globular backup list

# 5. Restore from the latest validated backup
globular backup restore --backup-id <latest-validated-backup>

# 6. Bootstrap the cluster
globular cluster bootstrap --node localhost:11000 --domain mycluster.local --profile core --profile gateway

# 7. The restored etcd state contains all desired-state entries
# The convergence model installs and starts all services

# 8. Verify
globular cluster health
globular services desired list
globular cluster get-drift-report
```

### Scenario 4: Full Cluster Disaster Recovery (Recovery Capsule)

All nodes are lost and the Globular CLI is not yet available. Use the recovery capsule scripts directly:

```bash
# Prerequisites on the new node:
# - restic, etcdctl/etcdutl, sctool installed
# - restic repository available at /var/backups/globular/restic
# - Recovery capsule copied to the node

# Navigate to the recovery capsule
cd /path/to/artifacts/<backup-id>/recovery-capsule/

# Run all phases (interactive — prompts for restic password if not set)
sudo RESTIC_PASSWORD=<password> ./restore.sh

# Or run individual phases:
sudo RESTIC_PASSWORD=<password> ./restore.sh --phase 1   # Restore files (incl. MinIO)
sudo ./restore.sh --phase 2                               # Start MinIO
sudo ./restore.sh --phase 3                               # Restore etcd
sudo ./restore.sh --phase 4                               # Restore ScyllaDB

# Dry-run to preview actions without making changes
sudo ./restore.sh --dry-run

# Once complete, start the cluster controller
sudo systemctl start globular-cluster-controller.service
```

**Phase breakdown:**

| Phase | What it does | Requires |
|-------|-------------|----------|
| 1 | `restic restore` → `/var/lib/globular` (files + MinIO data) | restic repo + password |
| 2 | Start MinIO, wait for healthy | Phase 1 complete |
| 3 | `etcdutl snapshot restore` → `/var/lib/globular/etcd` | etcd snapshot in capsule |
| 4 | `sctool restore` from snapshot tag in MinIO | Phase 2 complete, sctool + scylla-manager |

### Scenario 5: MinIO Gone, Local Restic Repo Survives

MinIO is broken or wiped but the local restic repo is intact:

```bash
# The restic backup covers /var/lib/globular which includes MinIO object data.
# Phase 1 restores that data. Phase 2 starts MinIO with the restored data.
# ScyllaDB backup is then accessible from MinIO as usual.

cd /path/to/artifacts/<backup-id>/recovery-capsule/
sudo RESTIC_PASSWORD=<password> ./restore.sh
```

No special handling needed — the standard restore order handles this automatically.

### Scenario 6: ScyllaDB Data Loss, Cluster Healthy

The cluster is running but ScyllaDB data was lost or corrupted:

```bash
# MinIO must be running (ScyllaDB backup is there)
# Run only Phase 4
cd /path/to/artifacts/<backup-id>/recovery-capsule/
sudo ./restore.sh --phase 4

# Or via the CLI
globular backup restore --backup-id <backup-id>
```

## What's Next

- [High Availability](operators/high-availability.md): Controller failover, etcd quorum, and fault tolerance
- [Failure Scenarios and Recovery](operators/failure-scenarios.md): Common failure patterns and how to recover
