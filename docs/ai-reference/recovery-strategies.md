# Recovery Strategies

Rules for automated and manual recovery actions.

## General Rules

- ALWAYS verify state before acting — memory/cache may be stale
- NEVER bypass workflows for state changes
- ALWAYS resolve endpoints via service discovery, not hardcoded addresses
- If etcd is unreachable, error out — do not use fallback data
- Prefer idempotent operations (safe to re-execute)

## Node Recovery

| Condition | Action |
|-----------|--------|
| Node heartbeat missing > 60s | Check node agent: `systemctl status globular-node-agent` |
| Node agent running but not reporting | Check etcd connectivity, TLS certs, DNS resolution |
| Node degraded (packages drifted) | Run `globular services repair` or re-apply desired state |
| Node unreachable | Verify network, SSH access, then restart node agent |

## Service Recovery

| Condition | Action |
|-----------|--------|
| Service not starting | Check logs: `journalctl -u globular-<service>` |
| Service running but unhealthy | Check gRPC health endpoint, verify TLS certs |
| Service config mismatch | Re-register via service restart (reads etcd on init) |
| Port conflict | Check `globular services list-desired`, verify port allocation |

## Workflow Recovery

| Condition | Action |
|-----------|--------|
| Workflow stuck in RUNNING | Check actor endpoint reachability, cancel and re-submit |
| Workflow FAILED | Read error from workflow run, fix root cause, re-submit |
| onFailure hook not firing | Check actor endpoint registration in ExecuteWorkflowRequest |
| Step timeout | Verify target service is running, check network path |

## Compute Job Recovery

| Condition | Action |
|-----------|--------|
| Job stuck in JOB_RUNNING | Check unit states — if all terminal, workflow may be stuck |
| Unit LEASE_EXPIRED | Runner died — unit will be retried if policy allows |
| Placement failed | Assign required profiles to nodes, re-submit |
| Deadline exceeded | Job cancelled correctly — re-submit with longer deadline |
| All retries exhausted | Fix root cause (entrypoint bug, missing input), re-submit |

## Package/Artifact Recovery

| Condition | Action |
|-----------|--------|
| Artifact stuck in VERIFIED | Run `globular pkg promote` manually |
| Checksum mismatch on install | Re-publish artifact (rebuild at same version triggers overwrite) |
| Repository unavailable | Check MinIO health, repository service logs |
| Package missing from repo | Re-publish via `globular pkg build` + `globular pkg publish` |

## etcd Recovery

| Condition | Action |
|-----------|--------|
| etcd leader election stuck | Check etcd cluster health: `etcdctl endpoint health` |
| Key corruption | Restore from backup, verify with `etcdctl get --prefix` |
| Disk full | Clean old revisions: `etcdctl compact`, `etcdctl defrag` |

## MinIO Recovery

| Condition | Action |
|-----------|--------|
| MinIO unreachable | Check systemd: `systemctl status minio`, verify DNS resolution |
| Bucket missing | Recreate via `mc mb` or let EnsureClusterConfigBucket auto-create |
| High CPU from reads | Verify manifest cache is active on repository service |

## Decision Tree

```
Problem detected
  → Is it a state mismatch? → Check 4-layer model alignment
  → Is it a connectivity issue? → Check DNS, TLS, endpoint resolution
  → Is it a workflow failure? → Read workflow run error, check actor endpoints
  → Is it a compute failure? → Check unit state, failure class, retry status
  → Is it a package issue? → Check artifact state, repository availability
  → Unknown? → Collect support bundle: `globular support bundle create`
```
