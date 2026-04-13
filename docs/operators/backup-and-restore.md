# Backup and Restore

This page covers Globular's backup and restore system: how backups work, which providers are available, how to schedule and manage backups, how to restore from backup, and how to prepare for disaster recovery.

## Backup Architecture

Globular's Backup Manager orchestrates backups across multiple providers and multiple nodes. It does not backup data directly — it coordinates providers that handle specific data stores.

```
Backup Manager (central coordinator)
    │
    ├── etcd provider     → etcdctl snapshot save
    ├── restic provider   → restic backup (file-level)
    ├── minio provider    → rclone sync (object store)
    └── scylla provider   → sctool backup (database)

    Fan-out to Node Agents:
    ├── Node 1: run provider locally → upload result
    ├── Node 2: run provider locally → upload result
    └── Node 3: run provider locally → upload result
```

### Backup Providers

Globular supports four backup providers:

**etcd** — Snapshot-based backup of the etcd cluster state. Uses `etcdctl snapshot save` to create a consistent point-in-time snapshot. This captures all cluster configuration, desired state, service endpoints, and node records.

**restic** — File-level incremental backup using restic. Backs up specified directories (configuration files, data directories, local state) with deduplication and encryption. Outputs snapshot metadata and log files.

**minio** — Object store synchronization using rclone. Replicates MinIO bucket contents to a remote destination. Used for backing up package artifacts and other blob data.

**scylla** — Database backup via ScyllaDB Manager (sctool). Creates distributed snapshots of ScyllaDB tables with multi-location support (S3, GCS, Azure). Polls task status until completion.

### Backup Destinations

Backups can be stored in five types of destinations:

| Destination | Description | Use Case |
|-------------|-------------|----------|
| `local` | Local filesystem path | Development, single-node clusters |
| `minio` | S3-compatible bucket | Standard production backups |
| `nfs` | NFS mount (treated as local) | Network-attached storage |
| `s3` | AWS S3 or compatible (via rclone) | Off-site cloud backup |
| `rclone` | Any rclone-supported remote | 40+ cloud providers |

Multi-destination replication: backups can be replicated to multiple destinations simultaneously for redundancy.

## Running Backups

### On-Demand Backup

Run a backup manually:

```bash
# Full cluster backup (all providers)
globular backup run --mode cluster

# Single-service backup
globular backup run --mode service --provider etcd

# With specific destination
globular backup run --mode cluster --destination minio
```

What happens during a cluster backup:

1. **Pre-flight check**: Verify all providers are available (etcdctl, restic, rclone, sctool binaries present)
2. **Quiesce hooks**: For each service offering `BackupHookService`, call `PrepareBackup`:
   - Service declares its local datasets with metadata (data class, scope, size)
   - Data classes: AUTHORITATIVE (must backup), REBUILDABLE (can regenerate), CACHE (skip)
   - Service enters quiescent state (pauses writes if needed)
3. **Provider execution**: Each provider runs on the appropriate node(s):
   - etcd: snapshot save on one etcd member
   - restic: incremental backup of configured paths
   - minio: rclone sync to remote destination
   - scylla: sctool backup with snapshot tag
4. **Finalize hooks**: Call `FinalizeBackup` on each service to resume operations
5. **Upload**: Results are uploaded to configured destination(s)
6. **Record**: Backup artifact is recorded with metadata (timestamp, providers, checksums, size)

### Backup Jobs

Backups run as jobs with lifecycle tracking:

```bash
# List recent jobs
globular backup list-jobs
# Output:
# JOB ID      MODE     STATUS      STARTED             DURATION
# bk-job-001  cluster  SUCCEEDED   2025-04-12 03:00    4m 22s
# bk-job-002  cluster  SUCCEEDED   2025-04-11 03:00    4m 18s
# bk-job-003  service  FAILED      2025-04-10 14:00    1m 02s

# Job details
globular backup get-job bk-job-001
# Shows: per-provider status, timing, bytes written, artifacts produced
```

Job states: QUEUED → RUNNING → SUCCEEDED / FAILED

The backup manager limits concurrent jobs (default: 1) via a semaphore. If a backup is already running, new requests queue.

### Scheduled Backups

Backups can be scheduled for automatic execution:

```bash
# View schedule
globular backup schedule-status
# Shows: next fire time, interval, last execution

# Schedule status shows the configured backup interval and destination
```

## Managing Backups

### Listing Backups

```bash
# List completed backups
globular backup list
# Output:
# BACKUP ID   DATE                PROVIDERS          SIZE      QUALITY
# bk-001      2025-04-12 03:00    etcd,restic,minio  2.1 GB    VALIDATED
# bk-002      2025-04-11 03:00    etcd,restic,minio  2.0 GB    RESTORE_TESTED
# bk-003      2025-04-10 03:00    etcd,restic,minio  1.9 GB    UNVERIFIED
```

### Backup Details

```bash
globular backup get bk-001
# Shows:
# - Timestamp, duration
# - Provider results (etcd: snapshot_id, restic: snapshot_id, minio: files synced)
# - Checksums and byte counts
# - Destination(s) and replication status
# - Quality state
# - Hook results (which services were quiesced)
```

### Validating Backups

Verify backup integrity:

```bash
# Shallow validation (checksums, metadata)
globular backup validate bk-001

# Deep validation (decompress, verify all providers)
globular backup validate bk-001 --deep
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
globular backup promote bk-001
```

Demote to remove protection:

```bash
globular backup demote bk-001
```

### Retention Policy

Automatic cleanup prevents unbounded backup growth:

```bash
# View retention status
globular backup retention-status
# Shows:
# - Policy: keep_last_n=30, keep_days=90, max_total_bytes=100GB
# - Current: 28 backups, 56 GB
# - Protected: 3 backups (RESTORE_TESTED or PROMOTED)
# - Min restore-tested to keep: 2
```

Retention rules:
- `KeepLastN`: Always keep the N most recent backups
- `KeepDays`: Keep backups from the last N days
- `MaxTotalBytes`: Maximum total storage (oldest deleted first)
- `MinRestoreTestedToKeep`: Minimum number of restore-tested backups to preserve

Retention runs automatically after each backup job. It evaluates rules in order and performs a dry-run before deleting.

## Restore Operations

### Restore Plan

Before restoring, preview what will happen:

```bash
globular backup restore-plan bk-001
# Output:
# RESTORE PLAN for bk-001:
#
# Provider    Action                          Target
# etcd        Restore snapshot to data-dir    /var/lib/etcd/
# restic      Restore files from snapshot     /etc/globular/, /var/lib/globular/
# minio       Sync objects from backup        MinIO buckets
# scylla      Restore from snapshot tag       ScyllaDB keyspaces
#
# WARNING: etcd restore will REPLACE current cluster state
# WARNING: Services will be stopped during restore
```

### Running a Restore

```bash
# Full restore
globular backup restore bk-001

# Restore specific provider only
globular backup restore bk-001 --provider etcd

# Force restore (bypass safety checks)
globular backup restore bk-001 --force
```

What happens during restore:

1. **Validation**: Verify the backup is intact (checksums, all providers present)
2. **Stop services**: Stop affected services to prevent data corruption during restore
3. **Provider restore**: Each provider restores its data:
   - etcd: `etcdctl snapshot restore` to data directory
   - restic: `restic restore` to original paths
   - minio: rclone sync from backup to MinIO
   - scylla: sctool restore from snapshot tag
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
# Save recovery configuration
globular backup apply-recovery-seed
```

The recovery seed persists backup destination configuration and credentials to:
```
/var/lib/globular/backups/settings.json
```

This file survives a cluster wipe. When a node starts with `RecoveryMode` enabled, it reads this file to locate and restore from the most recent backup.

### Day-0 Recovery Workflow

If the entire cluster is lost:

1. **Install Globular** on a fresh machine
2. **Place recovery seed** at `/var/lib/globular/backups/settings.json` (from offline backup)
3. **Start Node Agent** with recovery mode
4. **Agent locates latest backup** in the configured destination
5. **Restore etcd snapshot** — this recreates all cluster state
6. **Bootstrap** the node with restored configuration
7. **Services start** from restored desired state
8. **Convergence model** brings the cluster to the restored state

### Pre-Flight Check

Verify backup tool availability before relying on backups:

```bash
globular backup preflight-check
# Output:
# TOOL          STATUS    VERSION     PATH
# etcdctl       OK        3.5.14      /usr/local/bin/etcdctl
# restic        OK        0.16.4      /usr/local/bin/restic
# rclone        OK        1.66.0      /usr/local/bin/rclone
# sctool        OK        3.3.0       /usr/bin/sctool
```

If any tool is missing, the corresponding provider will not function. Install missing tools before relying on that provider.

## Practical Scenarios

### Scenario 1: Setting Up Daily Backups

Configure automatic daily cluster backups:

```bash
# Verify tools are available
globular backup preflight-check

# Run a test backup
globular backup run --mode cluster

# Verify it succeeded
globular backup list
globular backup validate <backup-id>

# Configure destination (if not already set)
# Backup configuration is managed through etcd

# Save recovery seed for disaster recovery
globular backup apply-recovery-seed
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
globular backup restore <latest-backup-id> --provider etcd

# Verify
globular cluster health
```

### Scenario 3: Full Cluster Disaster Recovery

All nodes are lost. Recovering from backup:

```bash
# 1. Provision new hardware, install Globular binaries

# 2. Copy recovery seed to /var/lib/globular/backups/settings.json

# 3. Start node agent
sudo systemctl start globular-node-agent

# 4. List available backups from the configured destination
globular backup list

# 5. Restore from the latest validated backup
globular backup restore <latest-validated-backup>

# 6. Bootstrap the cluster
globular cluster bootstrap --node localhost:11000 --domain mycluster.local --profile core --profile gateway

# 7. The restored etcd state contains all desired-state entries
# The convergence model installs and starts all services

# 8. Verify
globular cluster health
globular services desired list
globular services repair --dry-run
```

## What's Next

- [High Availability](high-availability.md): Controller failover, etcd quorum, and fault tolerance
- [Failure Scenarios and Recovery](failure-scenarios.md): Common failure patterns and how to recover
