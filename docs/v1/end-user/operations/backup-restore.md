# Backup and Restore

How to protect your cluster data.

## Create a backup

```bash
globular backup create
```

This snapshots cluster state, service configs, and metadata.

## List backups

```bash
globular backup list
```

## Restore from backup

```bash
globular backup restore <backup-id>
```

## What gets backed up?

- Cluster configuration (desired state, node metadata)
- Service configurations
- Package manifests (not the binaries — those stay in the store)

## What doesn't get backed up automatically?

- Application data (databases, user files) — manage these separately
- MinIO object storage — has its own replication
- etcd snapshots — can be taken manually with `etcdctl snapshot save`

## Compute-based backup

You can also use the compute engine to create distributed snapshots:

```bash
# Submit a backup-snapshot job across all nodes
globular compute submit backup-snapshot --parallelism 3
```

This creates per-node config snapshots and uploads them to MinIO.
