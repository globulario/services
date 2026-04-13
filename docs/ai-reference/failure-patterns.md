# Failure Patterns

Known failure modes and their signatures.

## Stale Leader

- **Symptom**: Cluster controller RPC returns `not leader (leader_addr=...)`
- **Cause**: Leadership transferred but caller cached old endpoint
- **Detection**: gRPC error code FailedPrecondition with "not leader"
- **Recovery**: Re-resolve controller endpoint via service discovery

## Missing Heartbeat (Node)

- **Symptom**: Node shows `last_seen` > 30s ago in ListNodes
- **Cause**: Node agent crashed, network partition, or systemd stop
- **Detection**: Cluster controller heartbeat timeout
- **Recovery**: Check node agent status, restart if needed

## Missing Heartbeat (Compute Unit)

- **Symptom**: Unit in UNIT_RUNNING but no heartbeat key in etcd
- **Cause**: Runner process died without cleanup
- **Detection**: computeAwaitUnitTerminal checks isLeaseAlive every 10s
- **Recovery**: Unit transitions to UNIT_LEASE_EXPIRED, retry if policy allows

## Artifact Stuck in VERIFIED

- **Symptom**: Package uploaded but not visible in catalog
- **Cause**: Auto-publish pipeline failed (descriptor registration or promotion)
- **Detection**: ListArtifacts excludes non-PUBLISHED artifacts
- **Recovery**: Manual `globular pkg promote` or re-upload (triggers retry)

## MinIO Storm

- **Symptom**: High CPU on MinIO nodes, elevated getobject rate
- **Cause**: No caching in repository service, reconcile loop polling manifests
- **Detection**: `rate(minio_s3_requests_total{api="getobject"}[5m])` sustained > 5/s
- **Recovery**: Repository manifest TTL cache (2-min) absorbs repeated reads

## Reconcile Storm

- **Symptom**: Continuous controller activity, high etcd write rate
- **Cause**: Convergence filtering not excluding already-converged nodes
- **Detection**: Controller logs show continuous reconcile with 0 changes
- **Recovery**: Convergence gate checks hash before dispatching

## Workflow Deadlock

- **Symptom**: Workflow run stuck in RUN_STATUS_RUNNING indefinitely
- **Cause**: Actor endpoint unreachable or step handler hangs
- **Detection**: Workflow run duration exceeds expected bounds
- **Recovery**: Cancel the run, re-submit with fresh correlation ID

## Placement Failure

- **Symptom**: JOB_FAILED with "no nodes match required profiles"
- **Cause**: No compute nodes have the required profile
- **Detection**: Job failure message contains "placement failed"
- **Recovery**: Assign profiles via `globular cluster nodes profiles set`

## Retry Exhaustion

- **Symptom**: Unit at attempt 3 with UNIT_FAILED
- **Cause**: Persistent failure (bad entrypoint, missing dependency)
- **Detection**: Unit attempt count == 3, failureClass visible
- **Recovery**: Fix root cause, re-submit job

## Binary Deploy Mismatch

- **Symptom**: New features not working after deploy
- **Cause**: scp/cp failed silently, old binary still running
- **Detection**: `strings /usr/lib/globular/bin/<service> | grep <expected_string>`
- **Recovery**: Stop service, rm old binary, copy new, restart

## etcd Key Collision

- **Symptom**: Multiple service instances appear as one
- **Cause**: Deterministic service ID shared across nodes
- **Detection**: Only 1 entry in service config list for multi-node service
- **Recovery**: Use per-node instance keys (PutInstance) for disambiguation
