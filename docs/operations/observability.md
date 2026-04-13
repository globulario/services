# Observability

## Metrics

All Globular services expose Prometheus metrics on their proxy port (service port + 1).

### Key Metrics

| Metric | Description |
|--------|-------------|
| `grpc_server_handled_total` | Total gRPC calls by service, method, code |
| `grpc_server_handling_seconds` | gRPC call duration histogram |
| `minio_s3_requests_total` | MinIO S3 API request count by operation |
| `node_memory_MemAvailable_bytes` | Available system memory |
| `up` | Target scrape status |

### Prometheus Configuration

Prometheus is deployed as a cluster service and auto-discovers targets via etcd service registration.

## Health Checks

Every service implements the gRPC health check protocol:
- `/grpc.health.v1.Health/Check` — returns SERVING or NOT_SERVING

### CLI Health Check

```bash
globular cluster health        # Overall cluster
globular doctor report         # Detailed diagnostics
```

## Logging

Services log to stderr via `slog` (structured text format). Logs are captured by systemd journal.

### Viewing Logs

```bash
journalctl -u globular-<service>.service --since "5m ago"
journalctl -u globular-<service>.service -f  # Follow
```

### Log Levels

- INFO: Normal operations
- WARN: Recoverable issues
- ERROR: Failures requiring attention
- DEBUG: Verbose diagnostics (enable with `--debug` flag)

## Cluster Doctor

The cluster doctor provides automated diagnostics:

```bash
# Generate findings
globular doctor report

# Auto-remediate
globular doctor heal

# View healing history
globular doctor heal history
```

## Alerting

Alertmanager is deployed as a cluster service for alert routing and notification.

## Workflow Observability

Workflow runs are recorded to ScyllaDB with:
- Run ID, status, duration
- Per-step outcomes
- Error messages for failed steps

Query via:
```bash
globular workflow list-runs
globular workflow get-run <run_id>
```

## Compute Observability

Compute jobs expose:
- Job state: `GetComputeJob`
- Unit states: `ListComputeUnits` (with progress, node assignment, lease status)
- Results: `GetComputeResult` (with checksums, trust level, output refs)
- Progress: `observedProgress` field updated from entrypoint's `progress.json`
