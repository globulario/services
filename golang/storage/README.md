# Storage Service (Globular)

The **Storage Service** provides a simple, pluggable **key–value store** API exposed over gRPC for the Globular platform.  
It supports multiple backends (local, distributed, and in‑memory) behind a common interface.

Backends implemented in this repo:

- **LevelDB** (local on-disk)
- **BadgerDB** (local on-disk)
- **ScyllaDB/Cassandra** (clustered)
- **etcd v3** (distributed KV)
- **BigCache** (in‑memory, non‑persistent)

> Note: there is **no SQLite** implementation.

---

## Highlights

- Unified KV methods: `Open`, `Close`, `SetItem`, `GetItem`, `RemoveItem`, `Clear`, `Drop` (depending on backend).
- Safe, serialized access loops for each store (see `*_store_sync.go`) to avoid race conditions.
- Backend‑specific options via **JSON** (for file‑based stores) or by environment/config (etcd) or JSON (Scylla).
- Designed to run as a **Globular microservice** with RBAC and service discovery, but the store packages can also be embedded directly into other Go apps.

---

## Repo layout (selected)

```
/server.go           # gRPC service implementation glue
/storage.go          # service wiring, roles/permissions, descriptors
/storage_client.go   # Go client for the gRPC service
/storage.proto       # API definition
/store.go            # Store interface + helpers
# Backends:
/leveldb_store.go         /leveldb_store_sync.go
/badger_store.go          /badger_store_sync.go
/bigcache_store.go        /bigcache_store_sync.go
/etcd_store.go            /etcd_store_sync.go
/scylla_store.go          /scylla_store_sync.go
```

---

## Building & Running

### As part of Globular
If you already run [Globular](https://github.com/globulario/Globular), publish/install the service from your build/output using the Globular CLI (examples vary per setup).

### Standalone build
```bash
go build -o storage_service ./server.go
./storage_service --help
```

The service loads its runtime/config through Globular conventions. For **etcd**, it will look for `etcd.yml` in the Globular config directory (see `config.GetEtcdClient()` usage in `etcd_store.go`).

---

## Using the Go Client (gRPC)

A minimal example with the generated client (see `storage_client.go`). The concrete RPC and method names may differ slightly depending on your version of `storage.proto`, but the flow is:

```go
package main

import (
    "fmt"
    "log"
    storage_client "github.com/globulario/services/golang/storage/storage_client"
)

func main() {
    // Connect to the service (hostname:port and service name will match your deployment)
    cli, err := storage_client.NewStorageService_Client("localhost:10013", "storage.StorageService")
    if err != nil {
        log.Fatal("connect:", err)
    }
    defer cli.Close()

    // Example store ID; you may need to CreateConnection or Open with backend options first,
    // depending on your service configuration.
    storeID := "demo"

    // Open the store (backend and options are configured server-side or via service calls)
    if err := cli.Open(storeID); err != nil {
        log.Fatal("open:", err)
    }

    // Set/Get/Remove
    if err := cli.SetItem(storeID, "hello", []byte("world")); err != nil {
        log.Fatal("set:", err)
    }
    val, err := cli.GetItem(storeID, "hello")
    if err != nil {
        log.Fatal("get:", err)
    }
    fmt.Println("value:", string(val))

    if err := cli.RemoveItem(storeID, "hello"); err != nil {
        log.Fatal("remove:", err)
    }
}
```

> Tip: For large payloads, prefer any streaming RPCs your `storage.proto` exposes (e.g. `SetLargeItem`) if available in your version.

---

## Backend Quickstarts (Direct Embedding)

You can also use the stores directly in a Go program (bypassing gRPC). All stores implement a common shape (`Open`, `Close`, `SetItem`, `GetItem`, `RemoveItem`, and sometimes `Clear`/`Drop`).

### LevelDB
```go
s := storage_store.NewLevelDB_store()
// JSON options must include path + name to resolve the DB directory
if err := s.Open(`{"path":"/var/lib/globular/storage","name":"leveldb_demo"}`); err != nil { panic(err) }
defer s.Close()

_ = s.SetItem("k", []byte("v"))
b, _ := s.GetItem("k")     // exact key
_ = s.RemoveItem("k")

// Wildcards: GetItem("prefix*") returns a JSON array of stringified values.
//            RemoveItem("prefix*") deletes all matching keys.
```

### BadgerDB
```go
s := storage_store.NewBadger_store()
// Options can be a raw path or JSON: {"path":"/var/lib","name":"badger_demo","syncWrites":true}
if err := s.Open(`{"path":"/var/lib/globular/storage","name":"badger_demo","syncWrites":true}`); err != nil { panic(err) }
defer s.Close()

_ = s.SetItem("k", []byte("v"))
b, _ := s.GetItem("k")
_ = s.RemoveItem("k")
// s.Clear()  // DropAll
// s.Drop()   // DropAll + close
```

### BigCache (in‑memory)
```go
s := storage_store.NewBigCache_store()
// Optional JSON tuning: {"shards":1024,"lifeWindowSec":600,"hardMaxCacheSizeMB":64}
if err := s.Open(`{"lifeWindowSec":300}`); err != nil { panic(err) }
defer s.Close()

_ = s.SetItem("k", []byte("v"))
b, _ := s.GetItem("k")
_ = s.RemoveItem("k")
// s.Clear(), s.Drop() are both Reset()
```

### etcd v3
```go
s := storage_store.NewEtcd_store()
// Address may be empty to load endpoints from etcd.yml in Globular config dir.
// Or pass comma-separated endpoints: "10.0.0.1:2379,10.0.0.2:2379"
if err := s.Open(""); err != nil { panic(err) }
defer s.Close()

_ = s.SetItem("k", []byte("v"))
b, _ := s.GetItem("k")
_ = s.RemoveItem("k")
// Clear/Drop are not supported for etcd KV.
```

### ScyllaDB / Cassandra
```go
s := storage_store.NewScylla_store("", "", 1)
// Open with JSON options; any of these are optional except hosts/keyspace/table.
opts := `{
  "hosts": ["127.0.0.1"],
  "port": 9042,
  "keyspace": "cache",
  "table": "kv",
  "replication_factor": 1,
  "consistency": "quorum",
  "tls": false
}`
if err := s.Open(opts); err != nil { panic(err) }
defer s.Close()

_ = s.SetItem("k", []byte("v"))
b, _ := s.GetItem("k")
_ = s.RemoveItem("k")
// s.Clear()  // TRUNCATE
// s.Drop()   // DROP TABLE
```

TLS for Scylla example:
```jsonc
{
  "hosts": ["scylla1.local","scylla2.local"],
  "tls": true,
  "ssl_port": 9142,
  "ca_file": "/etc/ssl/certs/ca.pem",
  "cert_file": "/etc/ssl/certs/client.pem",
  "key_file": "/etc/ssl/private/client.key",
  "server_name": "scylla.internal"
}
```

---

## Notes & Gotchas

- **File permissions (Badger/LevelDB)**: ensure the service user can create and write to the target directory.
- **Badger Drop on test cleanups**: `Drop()` calls `DropAll()` then `Close()`, which releases files like `KEYREGISTRY`. If your tests fail with a “permission denied” on temp cleanup, make sure the DB handle was closed.
- **LevelDB wildcards**: `"prefix*"` behavior returns **JSON array of string values** for `GetItem`, and deletes all matching keys for `RemoveItem`.
- **etcd**: `Clear` and `Drop` semantics are not implemented (would need range deletes / namespace conventions).
- **Scylla**: the service auto‑creates keyspace/table if missing (SimpleStrategy with configurable `replication_factor`).

---

## License
Apache 2.0 (see repository `LICENSE`).

