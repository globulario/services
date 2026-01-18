# Log Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Log Service provides centralized logging and audit trail capabilities for all Globular services.

## Overview

All services can send log entries to this central service, enabling unified log management, querying, and analysis across the entire platform.

## Features

- **Multiple Log Levels** - FATAL, ERROR, WARN, INFO, DEBUG, TRACE
- **Structured Logging** - Key-value fields for metadata
- **Streaming Queries** - Efficient log retrieval via gRPC streams
- **Occurrence Counting** - Track repeated log entries
- **Application Filtering** - Query logs by application/service

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Log Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Log Collector                          │ │
│  │                                                            │ │
│  │   Service A ───▶ ┌────────┐                               │ │
│  │   Service B ───▶ │ Parser │ ───▶ Log Store                │ │
│  │   Service C ───▶ └────────┘                               │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     Log Store                              │ │
│  │                                                            │ │
│  │   ┌─────────────────────────────────────────────────────┐ │ │
│  │   │  Application  │  Level  │  Message  │  Timestamp    │ │ │
│  │   ├─────────────────────────────────────────────────────┤ │ │
│  │   │  auth         │  INFO   │  Login... │  2024-01-15   │ │ │
│  │   │  file         │  ERROR  │  Disk...  │  2024-01-15   │ │ │
│  │   │  media        │  DEBUG  │  Conv...  │  2024-01-15   │ │ │
│  │   └─────────────────────────────────────────────────────┘ │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Methods

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `Log` | Record a log entry | `LogInfo` | `success` |
| `GetLog` | Query logs (streaming) | `query`, `filters` | Stream of `LogInfo` |
| `DeleteLog` | Remove specific log entry | `logId` | `success` |
| `ClearAllLog` | Bulk delete with filters | `query` | `count` |

### Log Levels

| Level | Value | Description |
|-------|-------|-------------|
| `FATAL` | 0 | System crash, unrecoverable errors |
| `ERROR` | 1 | Operation failed, needs attention |
| `WARN` | 2 | Potential issue, operation continued |
| `INFO` | 3 | Normal operational messages |
| `DEBUG` | 4 | Detailed diagnostic information |
| `TRACE` | 5 | Most detailed tracing information |

### LogInfo Structure

```protobuf
message LogInfo {
    string application = 1;   // Service name
    string userId = 2;        // Associated user
    string userName = 3;      // User display name
    string method = 4;        // Function/method name
    int32 line = 5;          // Source code line
    LogLevel level = 6;       // Severity level
    string message = 7;       // Log message
    string component = 8;     // Component/module
    int32 occurrences = 9;    // Repeat count
    int64 date = 10;         // Unix timestamp
    map<string, string> fields = 11; // Structured data
}
```

## Usage Examples

### Go Client - Logging

```go
import (
    "time"
    log "github.com/globulario/services/golang/log/log_client"
    logpb "github.com/globulario/services/golang/log/logpb"
)

client, _ := log.NewLogService_Client("localhost:10103", "log.LogService")
defer client.Close()

// Log an info message
err := client.Log(&logpb.LogInfo{
    Application: "my-service",
    Level:       logpb.LogLevel_INFO,
    Message:     "User login successful",
    Method:      "HandleLogin",
    Line:        142,
    Component:   "auth",
    Date:        time.Now().Unix(),
    Fields: map[string]string{
        "userId": "user-123",
        "ip":     "192.168.1.100",
    },
})
```

### Go Client - Querying Logs

```go
// Query logs
stream, err := client.GetLog(&logpb.GetLogRequest{
    Application: "my-service",
    Level:       logpb.LogLevel_ERROR,
    StartDate:   time.Now().Add(-24 * time.Hour).Unix(),
    EndDate:     time.Now().Unix(),
})
if err != nil {
    log.Fatal(err)
}

for {
    entry, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("[%s] %s: %s\n", entry.Level, entry.Application, entry.Message)
}
```

### Command Line

```bash
# Log an entry
grpcurl -plaintext -d '{
  "application": "test",
  "level": 3,
  "message": "Test log message",
  "method": "main",
  "date": 1705312800
}' localhost:10103 log.LogService/Log

# Query logs
grpcurl -plaintext -d '{"application": "test"}' \
  localhost:10103 log.LogService/GetLog
```

## Query Filtering

Logs can be filtered by:

| Filter | Description |
|--------|-------------|
| `application` | Service/application name |
| `level` | Minimum log level |
| `component` | Specific component |
| `startDate` | Logs after this timestamp |
| `endDate` | Logs before this timestamp |
| `userId` | Associated user |
| `method` | Function/method name |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_RETENTION_DAYS` | How long to keep logs | `30` |
| `LOG_MAX_ENTRIES` | Maximum stored entries | `1000000` |
| `LOG_BATCH_SIZE` | Stream batch size | `100` |

### Configuration File

```json
{
  "port": 10103,
  "retentionDays": 30,
  "maxEntries": 1000000,
  "batchSize": 100,
  "storage": {
    "type": "leveldb",
    "path": "/var/lib/globular/logs"
  }
}
```

## Log Aggregation Pattern

```
┌────────────────┐  ┌────────────────┐  ┌────────────────┐
│ Authentication │  │  File Service  │  │ Media Service  │
└───────┬────────┘  └───────┬────────┘  └───────┬────────┘
        │                   │                   │
        │  Log()            │  Log()            │  Log()
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                            ▼
              ┌─────────────────────────┐
              │       Log Service       │
              │                         │
              │   ┌─────────────────┐   │
              │   │   Centralized   │   │
              │   │    Log Store    │   │
              │   └─────────────────┘   │
              │                         │
              └─────────────────────────┘
                            │
                            ▼
              ┌─────────────────────────┐
              │    Admin Dashboard      │
              │                         │
              │  - View logs            │
              │  - Search & filter      │
              │  - Alert on errors      │
              └─────────────────────────┘
```

## Best Practices

1. **Use Appropriate Levels**
   - FATAL: System is unusable
   - ERROR: Something failed
   - WARN: Something unexpected
   - INFO: Normal operations
   - DEBUG: Development details
   - TRACE: Very detailed tracing

2. **Include Context** - Use fields for structured data

3. **Consistent Naming** - Use standard application and component names

4. **Don't Log Secrets** - Never include passwords, tokens, or PII

5. **Log Entry Points** - Log at service boundaries

## Integration

Used by all Globular services for centralized logging.

## Dependencies

- [Storage Service](../storage/README.md) - Log persistence

---

[Back to Services Overview](../README.md)
