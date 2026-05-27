# 10.0.0.102 Access Blocker + Snapshot Tooling Bug Report

## Access Blocker Status

- Node: `10.0.0.102`
- Ping from peer: OK (previous checks)
- Port `22`: open (`nc -vz` success from `10.0.0.8`)
- Port `11000`: open (`nc -vz` success from `10.0.0.8`)
- Node-agent status: active
- MinIO status: inactive/held
- SSH login: blocked (`Permission denied (publickey,password)`)

### SSH diagnostic conclusion

- Reachability is restored.
- Authentication is the blocker.
- Available auth methods are `publickey,password`, but automation path has no usable key and no interactive TTY password channel.

## Read-only MinIO Identity Evidence (already collected)

- `format.json` path exists via node-agent backup path:
  - `/mnt/data/data/.minio.sys/format.json`
- Backup hash: `b91ea498`
- Topology fingerprint: `cd0440d2`
- DeploymentID observed on node logs:
  - `443db6b4-5dbe-4ead-9cb9-143c0c80cbed`

This remains consistent with the known 5-member universe.

## Snapshot/Recover Tooling Bug (separate issue)

### Suggested finding id

`node.snapshot_status.protobuf_marshal_failure`

### Reproduced commands

1. `globular node snapshot create --node-id ffc29469-52d7-5929-844a-3a8d3a627d7c --reason 'bug-repro' --json`
2. `globular node snapshot show --node-id ffc29469-52d7-5929-844a-3a8d3a627d7c --json`
3. `globular node recover status --node-id ffc29469-52d7-5929-844a-3a8d3a627d7c --json`

### Observed failures

- `CreateNodeRecoverySnapshot: ... failed to marshal ... CreateNodeRecoverySnapshotRequest ... want proto.Message`
- `GetNodeRecoveryStatus: ... failed to marshal ... GetNodeRecoveryStatusRequest ... want proto.Message`

### Scope

- Separate from MinIO recovery.
- Prevents use of snapshot/recovery status as fallback inventory path.

## Next recommendation

- Keep incident path: `RESTORE_UNREACHABLE_NODE_FIRST`.
- Immediate operator action: restore one admin login path on `10.0.0.102` (trusted key or console key injection) and run pending read-only local disk checks.
- Track snapshot/recover protobuf failure as a separate bugfix pass.
