# Monitor Your Cluster

How to see what's happening in your cluster.

## Quick health check

```bash
globular cluster health
```

Shows: node count, health status, convergence percentage.

## Detailed diagnostics

```bash
globular doctor report
```

Shows: specific findings, recommendations, and severity levels.

## View service logs

```bash
# Last 10 minutes of a specific service
journalctl -u globular-<service>.service --since "10m ago"

# Follow live
journalctl -u globular-<service>.service -f
```

## Metrics

Globular includes Prometheus for metrics collection. Every service exposes metrics automatically.

Useful queries:

| What | Prometheus query |
|------|-----------------|
| Which services are up | `up` |
| Request rate per service | `rate(grpc_server_handled_total[5m])` |
| Error rate | `rate(grpc_server_handled_total{grpc_code!="OK"}[5m])` |
| Request latency (p99) | `histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket[5m]))` |

## Workflow history

```bash
globular workflow list-runs
globular workflow get-run <run_id>
```

Shows what happened during deployments, repairs, and compute jobs.

## Compute job progress

```bash
# Check job status
globular compute get-job <job_id>

# See unit-level details (which machine, progress, output)
globular compute list-units <job_id>
```
