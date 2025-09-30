# Storage Service

The **Storage Service** is a core Globular microservice that provides a unified key-value storage API with support for multiple backend implementations.  
It is designed to be modular, pluggable, and accessible over gRPC.

---

## Features

- Key-Value store with a consistent API across backends
- Multiple backend drivers:
  - LevelDB
  - BadgerDB
  - BigCache (in-memory cache)
  - ScyllaDB (distributed database, with TLS support)
- gRPC-based access
- Configurable connection options per store type
- Unit-tested with end-to-end roundtrip validation

---

## Supported Backends

### LevelDB
- Embedded key-value store.
- Persistent, lightweight, file-based.

### BadgerDB
- Fast embeddable key-value database written in Go.
- Great for local persistence.

### BigCache
- In-memory caching layer.
- Volatile, ideal for short-lived or ephemeral data.

### ScyllaDB
- Distributed NoSQL backend.
- Supports replication, TLS, and production-grade scalability.

---

## gRPC API

The gRPC interface is defined in [`storage.proto`](./storage.proto).  
Core RPC methods include:

- **CreateConnectionWithType**: Register a new store connection with a backend type.
- **OpenConnection**: Open an existing store with configuration options.
- **CloseConnection**: Close the store.
- **DeleteConnection**: Remove a connection definition.
- **SetItem / GetItem**: Store and retrieve values by key.
- **RemoveItem**: Delete a value by key.
- **Exists**: Check if a key exists.
- **Clear / Drop**: Clear data or fully remove the store.

---

## Example Usage

### Client (Go)

```go
package main

import (
    "fmt"
    "time"
    storage_client "github.com/globulario/services/golang/storage/storage_client"
    "github.com/globulario/services/golang/storage/storagepb"
)

func main() {
    client, err := storage_client.NewStorageService_Client("localhost:10001", "storage.StorageService")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    client.SetTimeout(5 * time.Second)

    // Create a LevelDB store connection
    if err := client.CreateConnectionWithType("conn1", "example_store", storagepb.StoreType_LEVEL_DB); err != nil {
        panic(err)
    }

    opts := `{"path":"/tmp/storage","name":"example_store"}`
    if err := client.OpenConnection("conn1", opts); err != nil {
        panic(err)
    }

    // Write and read a value
    if err := client.SetItem("conn1", "foo", []byte("bar")); err != nil {
        panic(err)
    }

    val, err := client.GetItem("conn1", "foo")
    if err != nil {
        panic(err)
    }
    fmt.Println("Got value:", string(val))
}
```

---

## Running Tests

The service includes a full suite of integration tests in [`storage_test.go`](./storage_test.go).

Run them with:

```bash
go test ./storage/storage_client -v
```

Tests include:
- Roundtrip (set/get/remove) for all store types
- Error path validation
- ScyllaDB TLS connection (optional, requires config)

---

## Configuration

Each store type accepts JSON options when opening:

- **LevelDB / BadgerDB**:
  ```json
  {"path":"/var/lib/storage","name":"example"}
  ```

- **BigCache**:
  ```json
  {"lifeWindowSec":30}
  ```

- **ScyllaDB**:
  ```json
  {
    "hosts": ["127.0.0.1:9042"],
    "keyspace": "storage_test",
    "table": "kv",
    "replication_factor": 1,
    "tls": true,
    "ca_file": "/etc/globular/config/tls/ca.crt",
    "cert_file": "/etc/globular/config/tls/client.crt",
    "key_file": "/etc/globular/config/tls/client.key"
  }
  ```

---

## Integration in Globular

- Managed as a Globular microservice
- Registered under service ID: `storage.StorageService`
- Configured through the Globular configuration system
- Can be secured with RBAC, TLS, and peer-to-peer service discovery

---

## License

Apache 2.0
