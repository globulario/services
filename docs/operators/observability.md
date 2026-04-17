# Observability

This page covers how to monitor a Globular cluster: metrics collection with Prometheus, log aggregation, workflow history, the Cluster Doctor, and the diagnostic tools available through the MCP (Model Context Protocol) interface.

## Observability Architecture

Globular's observability stack consists of four layers:

1. **Metrics**: Prometheus scrapes all services for time-series data (request rates, latencies, error rates, resource usage)
2. **Logs**: Structured logging via slog, queryable through the Log service and Node Agent RPCs
3. **Workflows**: Every cluster operation is recorded as a workflow run with steps, timing, and failure classification
4. **Diagnostics**: The Cluster Doctor and MCP tools provide real-time cluster analysis

## Prometheus Metrics

### How Metrics Are Exposed

Every Globular gRPC service automatically exposes Prometheus metrics via the interceptor chain:

**Standard gRPC metrics** (exposed by all services):
- `grpc_server_handled_total` — Total RPC count by service, method, and status code
- `grpc_server_handling_seconds` — Latency histogram by service and method
- `grpc_server_msg_received_total` — Inbound messages
- `grpc_server_msg_sent_total` — Outbound messages

**Custom metrics**: Services can register additional metrics using the Prometheus client library. Examples:
- Backup manager: job counts, retention deletions, provider durations
- Controller: workflow dispatch counts, circuit breaker state, node heartbeat lag
- Repository: artifact upload/download counts, storage usage

### Monitoring Service

The Monitoring service (port 10019) wraps the Prometheus HTTP API as a gRPC service, making metrics queryable from any cluster client:

```bash
# Instant query
globular metrics query --expr 'grpc_server_handled_total{grpc_code="OK"}'

# Range query (last hour, 1-minute steps)
globular metrics query-range \
  --expr 'rate(grpc_server_handled_total[5m])' \
  --start "1 hour ago" \
  --end "now" \
  --step 1m

# List scrape targets
globular metrics targets

# Check firing alerts
globular metrics alerts

# View alerting and recording rules
globular metrics rules
```

### Key Queries

**Request rate by service**:
```promql
sum by (grpc_service) (rate(grpc_server_handled_total[5m]))
```

**Error rate**:
```promql
sum by (grpc_service) (rate(grpc_server_handled_total{grpc_code!="OK"}[5m]))
/ sum by (grpc_service) (rate(grpc_server_handled_total[5m]))
```

**P99 latency**:
```promql
histogram_quantile(0.99, sum by (le, grpc_service) (rate(grpc_server_handling_seconds_bucket[5m])))
```

**Node Agent heartbeat staleness** (controller-side):
```promql
time() - globular_node_last_heartbeat_timestamp_seconds
```

### Alertmanager

Alertmanager handles alert routing, grouping, and notification. It runs as an infrastructure package alongside Prometheus.

Configuration is managed through etcd — alert rules and routing configuration are stored cluster-wide, not in local files.

Common alert rules:
- Node heartbeat missing for > 5 minutes
- Service health check failing for > 2 minutes
- Workflow failure rate > 50% in 15-minute window
- etcd cluster unhealthy (fewer than quorum members)
- Disk usage > 85%

## Logging

### Structured Logging

All Globular services use Go's `slog` package for structured logging. Log entries include:

| Field | Content |
|-------|---------|
| `level` | FATAL, ERROR, WARN, INFO, DEBUG, TRACE |
| `component` | Subsystem (e.g., "http", "dns", "tls", "rbac") |
| `node_id` | Originating node |
| `method` | gRPC method or function name |
| `fields` | Arbitrary key-value pairs (structured attributes) |
| `timestamp_ms` | Producer wall-clock time |

### Log Service

The Log service (port 10100) provides centralized log aggregation:

```bash
# Write a log entry (typically done by services, not operators)
# Log entries are written via the Log RPC

# Query logs (server-streaming)
globular log query --application <service> --level ERROR --since "1h"

# Clear logs for a service
globular log clear --application <service>
```

### Node Agent Log Access

For service-specific logs, the Node Agent provides direct access to journald:

```bash
# Get recent logs for a service
globular node logs --node <node>:11000 --unit <service> --lines 100

# Search logs with pattern and time range
globular node search-logs \
  --node <node>:11000 \
  --unit <service> \
  --pattern "error|timeout|refused" \
  --since "2025-04-12T10:00:00Z" \
  --until "2025-04-12T11:00:00Z" \
  --severity ERROR
```

The Node Agent wraps `journalctl` with deduplication and compact output formatting, making it practical to review logs remotely.

### Audit Logging

Every authorization decision is logged by the gRPC interceptor:

- **Allowed decisions**: Logged at DEBUG level
- **Denied decisions**: Logged at WARN level and never sampled

Audit log fields: timestamp, subject, principal_type, auth_method, grpc_method, resource_path, permission, allowed/denied, reason, latency, remote_addr. Raw tokens are never included.

## Workflow History

Every cluster operation produces a workflow run with a complete audit trail. This is one of Globular's most powerful observability tools — it answers "why did this change happen?" for any service on any node.

### Querying Workflow Runs

```bash
# Recent workflows (all services, all nodes)
globular workflow list

# Filter by service
globular workflow list --service postgresql

# Filter by node
globular workflow list --node node-abc123

# Filter by status
globular workflow list --status FAILED
globular workflow list --status SUCCEEDED

# Filter by trigger
globular workflow list --trigger DESIRED_DRIFT
```

### Workflow Run Details

```bash
globular workflow get <run-id>
```

Each run shows:
- **Identity**: run_id, correlation_id, parent_run_id (for retries)
- **Context**: cluster_id, node_id, service_name, version
- **Timing**: start_time, end_time, duration
- **Status**: current status, failure_class (if failed)
- **History**: retry_count, trigger_reason
- **Steps**: ordered list with individual status, duration, and error messages

### Tracing a Service's History

The correlation_id is stable across retries, enabling you to trace the complete deployment history of a specific service on a specific node:

```bash
# All workflow attempts for postgresql on node-2
globular workflow list --correlation "service/postgresql/node-2"
```

This shows every attempt to deploy or upgrade postgresql on node-2, including successful deployments, failed attempts, retries, and rollbacks.

## Cluster Doctor

The Cluster Doctor provides continuous health analysis and proactive problem detection.

### Health Reports

```bash
# Quick cached report
globular doctor report

# Full fresh analysis
globular doctor report --fresh
```

The report includes:
- **Overall status**: HEALTHY, DEGRADED, or CRITICAL
- **Findings**: Classified issues with severity (INFO, WARN, ERROR, CRITICAL)
- **Evidence**: Data supporting each finding
- **Recommendations**: Specific remediation steps
- **Top issues**: The 5 most important problems to address

### Freshness Contract

The doctor uses a freshness contract for data:
- **CACHED mode** (default): Uses a recent snapshot (fast, < 1 second)
- **FRESH mode**: Collects live data from all nodes (slower, 5-30 seconds)

The report header shows snapshot metadata:
```
Source: cluster-doctor (leader)
Observed at: 2025-04-12T10:30:00Z
Snapshot age: 12s
Cache hit: true
Cache TTL: 30s
```

### Invariant Checks

The doctor evaluates invariants — conditions that must always be true in a healthy cluster:

- **Service drift**: Desired version matches installed version
- **Unit running**: Installed services are active in systemd
- **Network drift**: Service endpoints in etcd match actual addresses
- **Pending convergence**: No workflows stuck for too long
- **etcd membership**: All expected etcd members are healthy
- **Blocked workflows**: No workflows stuck in BLOCKED state
- **Service registration**: Running services have etcd endpoints
- **Artifact integrity**: Installed checksums match manifest checksums

Each invariant produces a PASS, FAIL, or PENDING result.

## MCP Diagnostic Tools

Globular exposes 65+ diagnostic tools through the MCP (Model Context Protocol) interface. These tools are designed for both human operators and AI-assisted diagnostics.

### Cluster Diagnostics

```
cluster_get_health              — Overall cluster health status
cluster_list_nodes              — Node listing with status
cluster_get_node_full_status    — Comprehensive node state
cluster_get_node_health_detail  — Per-subsystem health checks
cluster_get_desired_state       — Current desired-state store
cluster_get_drift_report        — Desired vs actual mismatches
cluster_get_convergence_detail  — Per-node convergence state
cluster_get_reconcile_status    — Reconciliation loop state
cluster_get_operational_snapshot— Full cluster snapshot
cluster_get_doctor_report       — Doctor findings
cluster_explain_finding         — Detailed finding explanation
cluster_get_info                — Cluster metadata and network config
```

### Backup Diagnostics

```
backup_list_jobs                — Recent backup jobs
backup_get_job                  — Job details with provider results
backup_list_backups             — Completed backup artifacts
backup_get_backup               — Full backup manifest
backup_validate_backup          — Integrity check
backup_restore_plan             — Read-only restore preview
backup_get_retention_status     — Retention policy and stats
backup_get_schedule_status      — Next scheduled backup
backup_get_recovery_status      — Disaster recovery readiness
backup_preflight_check          — Tool availability check
backup_test_scylla_connection   — ScyllaDB backup connectivity
```

### Service Diagnostics

```
nodeagent_get_service_logs      — Service journal output
nodeagent_search_logs           — Pattern-based log search
nodeagent_get_certificate_status— TLS certificate details
nodeagent_get_inventory         — Node hardware/software inventory
nodeagent_list_installed_packages — Installed package list
nodeagent_get_installed_package — Single package details
nodeagent_control_service       — Service control (start/stop/restart)
```

### Monitoring Diagnostics

```
metrics_query                   — Instant PromQL query
metrics_query_range             — Time-series query
metrics_targets                 — Scrape target status
metrics_alerts                  — Firing/pending alerts
metrics_rules                   — Alerting and recording rules
metrics_label_values            — Label enumeration
```

### Infrastructure Diagnostics

```
etcd_get                        — Read etcd key
etcd_put                        — Write etcd key
etcd_delete                     — Delete etcd key
grpc_call                       — Raw gRPC call to any service
grpc_service_map                — Service method listing
grpc_web_probe                  — gRPC-Web endpoint check
http_diagnose                   — HTTP endpoint diagnosis
```

## Practical Scenarios

### Scenario 1: Investigating High Latency

A user reports slow API responses:

```bash
# Check P99 latency across services
globular metrics query --expr 'histogram_quantile(0.99, sum by (le, grpc_service) (rate(grpc_server_handling_seconds_bucket[5m])))'

# Identify the slow service (e.g., persistence)
globular metrics query-range \
  --expr 'histogram_quantile(0.99, sum by (le) (rate(grpc_server_handling_seconds_bucket{grpc_service="persistence.PersistenceService"}[5m])))' \
  --start "1 hour ago" --end "now" --step 1m

# Check if it correlates with high request rate
globular metrics query --expr 'sum(rate(grpc_server_handled_total{grpc_service="persistence.PersistenceService"}[5m]))'

# Check the persistence service logs
globular node search-logs --node <node>:11000 --unit persistence --pattern "slow|timeout" --severity WARN
```

### Scenario 2: Post-Incident Review

After an outage, reconstruct what happened:

```bash
# 1. Check workflow history during the incident window
globular workflow list --status FAILED

# 2. Get doctor report for the incident period
globular doctor report --fresh

# 3. Check audit logs for unusual activity
globular node search-logs --node <node>:11000 --unit <service> \
  --since "2025-04-12T09:00:00Z" --until "2025-04-12T10:00:00Z" \
  --severity ERROR

# 4. Check Prometheus for metric anomalies
globular metrics query-range \
  --expr 'rate(grpc_server_handled_total{grpc_code!="OK"}[5m])' \
  --start "2025-04-12T09:00:00Z" --end "2025-04-12T10:00:00Z" --step 1m

# 5. Review alerts that fired
globular metrics alerts
```

### Scenario 3: Capacity Planning

Assess current cluster utilization:

```bash
# Request rate trends (last 24 hours)
globular metrics query-range \
  --expr 'sum(rate(grpc_server_handled_total[5m]))' \
  --start "24 hours ago" --end "now" --step 5m

# Per-service breakdown
globular metrics query --expr 'sum by (grpc_service) (rate(grpc_server_handled_total[1h]))'

# Node resource usage (if node-exporter metrics available)
globular metrics query --expr 'node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes'
globular metrics query --expr '1 - rate(node_cpu_seconds_total{mode="idle"}[5m])'
```

## What's Next

- [Backup and Restore](operators/backup-and-restore.md): Protect your cluster data
- [High Availability](operators/high-availability.md): Leader election, failover, and fault tolerance
