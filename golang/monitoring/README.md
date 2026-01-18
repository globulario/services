# Monitoring Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Monitoring Service provides time-series data management through Prometheus integration.

## Overview

This service acts as a bridge to Prometheus, enabling Globular applications to query metrics, manage alerts, and access monitoring data through a gRPC interface.

## Features

- **Prometheus Integration** - Full Prometheus API access
- **Instant Queries** - Point-in-time metric values
- **Range Queries** - Time-series data over intervals
- **Alert Management** - View active alerts and rules
- **Target Discovery** - Monitor scrape targets
- **Admin Operations** - Snapshots, cleanup, config

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Monitoring Service                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Prometheus Adapter                         │ │
│  │                                                            │ │
│  │  gRPC API ──▶ HTTP Client ──▶ Prometheus Server           │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Query Engine                             │ │
│  │                                                            │ │
│  │  Query (instant) │ QueryRange (time-series)               │ │
│  │  Series │ Labels │ LabelValues                            │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Admin Operations                          │ │
│  │                                                            │ │
│  │  Alerts │ Rules │ Targets │ Config │ Snapshots            │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Configure Prometheus | `id`, `address` |
| `DeleteConnection` | Remove connection | `id` |

### Query Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `Query` | Instant query | `connectionId`, `query`, `time` |
| `QueryRange` | Range query (streaming) | `connectionId`, `query`, `start`, `end`, `step` |
| `Series` | Get series metadata | `connectionId`, `match[]`, `start`, `end` |
| `LabelNames` | List label names | `connectionId` |
| `LabelValues` | Get label values | `connectionId`, `label` |

### Administration

| Method | Description | Parameters |
|--------|-------------|------------|
| `Alerts` | Get active alerts | `connectionId` |
| `AlertManagers` | List alert managers | `connectionId` |
| `Rules` | Get alerting rules | `connectionId` |
| `Targets` | Get scrape targets | `connectionId` |
| `TargetsMetadata` | Get target metadata | `connectionId` |
| `Config` | Get Prometheus config | `connectionId` |
| `Flags` | Get runtime flags | `connectionId` |
| `Snapshot` | Create TSDB snapshot | `connectionId` |
| `DeleteSeries` | Delete time series | `connectionId`, `match[]`, `start`, `end` |
| `CleanTombstones` | Clean deleted data | `connectionId` |

## Usage Examples

### Go Client

```go
import (
    monitoring "github.com/globulario/services/golang/monitoring/monitoring_client"
)

client, _ := monitoring.NewMonitoringService_Client("localhost:10119", "monitoring.MonitoringService")
defer client.Close()

// Create connection
err := client.CreateConnection("prom", "http://prometheus:9090")

// Instant query
result, err := client.Query("prom", "up", time.Now())
for _, sample := range result {
    fmt.Printf("Metric: %s, Value: %f\n", sample.Metric, sample.Value)
}

// Range query
stream, err := client.QueryRange(
    "prom",
    "rate(http_requests_total[5m])",
    time.Now().Add(-1*time.Hour),
    time.Now(),
    time.Minute,
)
for {
    point, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("Time: %v, Value: %f\n", point.Timestamp, point.Value)
}

// Get active alerts
alerts, err := client.Alerts("prom")
for _, alert := range alerts {
    fmt.Printf("Alert: %s, State: %s\n", alert.Name, alert.State)
}

// Get scrape targets
targets, err := client.Targets("prom")
for _, target := range targets {
    fmt.Printf("Target: %s, Health: %s\n", target.Labels["instance"], target.Health)
}
```

### Common Queries

| Query | Description |
|-------|-------------|
| `up` | Target health status |
| `rate(http_requests_total[5m])` | Request rate |
| `histogram_quantile(0.95, rate(request_duration_seconds_bucket[5m]))` | 95th percentile latency |
| `sum(rate(errors_total[5m])) by (service)` | Error rate by service |
| `node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes` | Memory utilization |

## Configuration

```json
{
  "port": 10119,
  "connections": [
    {
      "id": "default",
      "address": "http://prometheus:9090",
      "timeout": "30s"
    }
  ]
}
```

## Dependencies

- External Prometheus server

---

[Back to Services Overview](../README.md)
