# Storage Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Storage Service provides a key-value store abstraction supporting multiple backend storage engines.

## Overview

This service offers a unified API for key-value operations across different storage backends, from lightweight embedded databases to distributed systems.

## Features

- **Multiple Backends** - LevelDB, BadgerDB, BigCache, ScyllaDB, etcd
- **Streaming Support** - Efficient handling of large values
- **Atomic Operations** - Consistent read/write semantics
- **TTL Support** - Time-to-live for cache scenarios
- **Batch Operations** - Bulk key enumeration and clearing

## Supported Backends

| Backend | Type | Use Case |
|---------|------|----------|
| **LevelDB** | Embedded | Persistent local storage |
| **BadgerDB** | Embedded | High-performance SSD storage |
| **BigCache** | In-memory | Fast caching |
| **ScyllaDB** | Distributed | Scalable wide-column store |
| **etcd** | Distributed | Configuration and coordination |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Storage Service                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Unified KV API                          │ │
│  │                                                            │ │
│  │   SetItem  │  GetItem  │  RemoveItem  │  GetAllKeys        │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              │                                   │
│          ┌───────────────────┼───────────────────┐              │
│          │         │         │         │         │              │
│          ▼         ▼         ▼         ▼         ▼              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │ LevelDB │ │ BadgerDB│ │ BigCache│ │ ScyllaDB│ │  etcd   │   │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Configure store | `id`, `type`, `path`/`host` |
| `DeleteConnection` | Remove store config | `id` |
| `Open` | Open store | `id` |
| `Close` | Close store | `id` |

### Key-Value Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetItem` | Store value | `id`, `key`, `value` |
| `SetLargeItem` | Store large value (streaming) | `id`, `key`, `stream` |
| `GetItem` | Retrieve value (streaming) | `id`, `key` |
| `RemoveItem` | Delete key | `id`, `key` |

### Bulk Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `GetAllKeys` | List all keys (streaming) | `id` |
| `Clear` | Remove all items | `id` |
| `Drop` | Delete entire store | `id` |

## Usage Examples

### Go Client

```go
import (
    storage "github.com/globulario/services/golang/storage/storage_client"
)

client, _ := storage.NewStorageService_Client("localhost:10109", "storage.StorageService")
defer client.Close()

// Create LevelDB store
err := client.CreateConnection("cache", "leveldb", "/var/lib/globular/cache")

// Open store
err = client.Open("cache")

// Set item
err = client.SetItem("cache", "user:123", []byte(`{"name": "John"}`))

// Get item
data, err := client.GetItem("cache", "user:123")
fmt.Printf("Value: %s\n", string(data))

// List all keys
keys, err := client.GetAllKeys("cache")
for _, key := range keys {
    fmt.Println("Key:", key)
}

// Remove item
err = client.RemoveItem("cache", "user:123")

// Close store
err = client.Close("cache")
```

### Streaming Large Values

```go
// Set large item (streaming)
reader := bytes.NewReader(largeData)
err := client.SetLargeItem("cache", "large-file", reader)

// Get large item (streaming)
var buffer bytes.Buffer
err = client.GetItemToWriter("cache", "large-file", &buffer)
```

## Configuration

### Configuration File

```json
{
  "port": 10109,
  "stores": [
    {
      "id": "cache",
      "type": "leveldb",
      "path": "/var/lib/globular/cache"
    },
    {
      "id": "etcd-store",
      "type": "etcd",
      "endpoints": ["localhost:2379"]
    }
  ]
}
```

## Use Cases

| Use Case | Recommended Backend |
|----------|---------------------|
| Session storage | BigCache |
| Configuration | etcd |
| Local cache | LevelDB or BadgerDB |
| Distributed cache | ScyllaDB |

## Dependencies

None - Core infrastructure service.

---

[Back to Services Overview](../README.md)
