# Persistence Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Persistence Service provides a universal database abstraction layer, supporting multiple database backends through a unified API.

## Overview

This service abstracts away database-specific details, allowing applications to work with MongoDB, SQL databases (MySQL, PostgreSQL, SQLite), and ScyllaDB through a consistent interface.

## Features

- **Multi-Backend Support** - MongoDB, SQL, ScyllaDB
- **Unified CRUD API** - Same operations across all backends
- **Streaming Results** - Efficient large result set handling
- **Aggregation Pipelines** - MongoDB-style aggregations
- **Transaction Support** - ACID transactions where supported
- **Connection Pooling** - Efficient connection management

## Supported Backends

| Backend | Use Case | Features |
|---------|----------|----------|
| **MongoDB** | Document storage | Flexible schema, aggregations |
| **MySQL** | Relational data | ACID, joins, transactions |
| **PostgreSQL** | Advanced relational | JSON support, full-text search |
| **SQLite** | Embedded/local | Zero-config, single file |
| **ScyllaDB** | High-throughput | Wide-column, distributed |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Persistence Service                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                     Unified API Layer                            │    │
│  │                                                                  │    │
│  │   InsertOne  │  Find  │  Update  │  Delete  │  Aggregate        │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                              │                                           │
│              ┌───────────────┼───────────────┐                          │
│              │               │               │                          │
│              ▼               ▼               ▼                          │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐               │
│  │   MongoDB     │  │     SQL       │  │   ScyllaDB    │               │
│  │   Driver      │  │    Driver     │  │    Driver     │               │
│  │               │  │               │  │               │               │
│  │  - Document   │  │  - MySQL      │  │  - Cassandra  │               │
│  │  - BSON       │  │  - PostgreSQL │  │  - CQL        │               │
│  │  - GridFS     │  │  - SQLite     │  │  - Partitions │               │
│  └───────────────┘  └───────────────┘  └───────────────┘               │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                   Connection Pool Manager                        │    │
│  │                                                                  │    │
│  │   Connection ID → Active Connection → Idle Timeout               │    │
│  │                                                                  │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Register database connection | `id`, `type`, `host`, `port`, `user`, `password` |
| `DeleteConnection` | Remove connection | `id` |
| `Connect` | Establish connection | `id`, `database` |
| `Disconnect` | Close connection | `id` |
| `Ping` | Test connection | `id` |

### CRUD Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `InsertOne` | Insert single document | `connection`, `database`, `collection`, `data` |
| `InsertMany` | Insert multiple documents | `connection`, `database`, `collection`, `data[]` |
| `FindOne` | Find single document | `connection`, `database`, `collection`, `query` |
| `Find` | Find documents (streaming) | `connection`, `database`, `collection`, `query` |
| `UpdateOne` | Update single document | `connection`, `database`, `collection`, `query`, `update` |
| `Update` | Update multiple documents | `connection`, `database`, `collection`, `query`, `update` |
| `ReplaceOne` | Replace entire document | `connection`, `database`, `collection`, `query`, `replacement` |
| `DeleteOne` | Delete single document | `connection`, `database`, `collection`, `query` |
| `Delete` | Delete multiple documents | `connection`, `database`, `collection`, `query` |

### Database Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateDatabase` | Create new database | `connection`, `name` |
| `DeleteDatabase` | Drop database | `connection`, `name` |
| `CreateCollection` | Create collection/table | `connection`, `database`, `name` |
| `DeleteCollection` | Drop collection/table | `connection`, `database`, `name` |

### Analytics

| Method | Description | Parameters |
|--------|-------------|------------|
| `Aggregate` | Run aggregation pipeline | `connection`, `database`, `collection`, `pipeline` |
| `Count` | Count documents | `connection`, `database`, `collection`, `query` |

## Usage Examples

### Create a Connection

Connections can be saved in the configuration file (`config.json`). If saved, connections will be recreated automatically at server start time.

```go
user := "sa"
pwd := "adminadmin"
err := client.CreateConnection("connection_id", "connection_id", "localhost", 27017, 0, user, pwd, 500, "", true)
```

### Connect to Database

Open an existing connection defined in the config file:

```go
err := client.Connect("connection_id", "adminadmin")
if err != nil {
    log.Println("fail to connect to the backend with error ", err)
}
```

### Ping (Test Connection)

Test if the backend is reachable:

```go
err := client.Ping("connection_id")
if err != nil {
    log.Fatalln("fail to ping the backend with error ", err)
}
```

### Insert One

Store a single entity. For MongoDB, the database and collection will be created automatically at first insertion:

```go
Id := "connection_id"
Database := "TestMONGO"
Collection := "Employees"

employe := map[string]interface{}{
    "hire_date":  "2007-07-01",
    "last_name":  "Courtois",
    "first_name": "Dave",
    "birth_date": "1976-01-28",
    "emp_no":     200000,
    "gender":     "M",
}

id, err := client.InsertOne(Id, Database, Collection, employe, `[{"upsert":true}]`)
if err != nil {
    log.Fatalf("fail to persist entity with error %v", err)
}
log.Println("Entity persist with id ", id)
```

### Insert Many

For arrays of objects or objects larger than the gRPC message size limit (4mb default):

```go
entities := []interface{}{
    map[string]interface{}{
        "userId":       "rirani",
        "jobTitleName": "Developer",
        "firstName":    "Romin",
        "lastName":     "Irani",
        "region":       "CA",
    },
    map[string]interface{}{
        "userId":       "nirani",
        "jobTitleName": "Developer",
        "firstName":    "Neil",
        "lastName":     "Irani",
        "region":       "CA",
    },
}

err := client.InsertMany(Id, Database, Collection, entities, "")
if err != nil {
    log.Fatalf("Fail to insert many entities with error %v", err)
}
```

### Replace One

Replace an entire document:

```go
entity := map[string]interface{}{
    "_id":          "nirani",
    "jobTitleName": "Full Stack Developer",
    "firstName":    "Neil",
    "lastName":     "Irani",
}

err := client.ReplaceOne(Id, Database, Collection, `{"_id":"nirani"}`, entity, "")
```

### Update One

Update specific fields of a document:

```go
err := client.UpdateOne(Id, Database, Collection,
    `{"_id":"nirani"}`,
    `{"$set":{"employeeCode":"E2.2"},"$set":{"phoneNumber":"408-1231234"}}`,
    "")
```

### Update Many

Update fields across multiple documents:

```go
Query := `{"region": "CA"}`
Value := `{"$set":{"state":"California"}}`

err := client.Update(Id, Database, Collection, Query, Value, "")
```

### Find One

Find a single document:

```go
Query := `{"first_name": "Dave"}`

values, err := client.FindOne(Id, Database, Collection, Query, "")
if err != nil {
    log.Fatalf("FindOne fail %v", err)
}
```

### Find Many

Find multiple documents (streaming for large results):

```go
Query := `{"region": "CA"}`

values, err := client.Find(Id, Database, Collection, Query, `[{"Projection":{"firstName":1}}]`)
if err != nil {
    log.Fatalf("fail to find entities with error %v", err)
}
```

### Aggregate

Transform data with aggregation pipelines:

```go
results, err := client.Aggregate(Id, Database, Collection, `[{"$count":"region"}]`, "")
if err != nil {
    log.Fatalf("fail to create aggregation with error %v", err)
}
```

### Delete One

Delete a single document:

```go
Query := `{"_id":"nirani"}`

err := client.DeleteOne(Id, Database, Collection, Query, "")
```

### Delete Many

Delete multiple documents:

```go
Query := `{"region": "CA"}`

err := client.Delete(Id, Database, Collection, Query, "")
```

### Disconnect

Close a connection:

```go
err := client.Disconnect("connection_id")
```

### Delete Collection

Drop a collection:

```go
err := client.DeleteCollection(Id, Database, Collection)
```

### Delete Database

Drop a database:

```go
err := client.DeleteDatabase(Id, Database)
```

### Run Admin Command

Execute administrative scripts on the database server:

```go
changePasswordScript := fmt.Sprintf(
    "db=db.getSiblingDB('admin');db.changeUserPassword('%s','%s');",
    name, newPassword)

err = client.RunAdminCmd(context.Background(), "local_resource", user, password, changePasswordScript)
```

## Query Syntax

Queries use MongoDB-style syntax across all backends:

### Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `$eq` | Equal | `{"age": {"$eq": 30}}` |
| `$ne` | Not equal | `{"status": {"$ne": "deleted"}}` |
| `$gt` | Greater than | `{"age": {"$gt": 18}}` |
| `$gte` | Greater or equal | `{"age": {"$gte": 21}}` |
| `$lt` | Less than | `{"price": {"$lt": 100}}` |
| `$lte` | Less or equal | `{"stock": {"$lte": 10}}` |
| `$in` | In array | `{"status": {"$in": ["active", "pending"]}}` |
| `$nin` | Not in array | `{"type": {"$nin": ["spam", "deleted"]}}` |

### Logical Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `$and` | All conditions | `{"$and": [{"age": {"$gte": 18}}, {"status": "active"}]}` |
| `$or` | Any condition | `{"$or": [{"status": "admin"}, {"status": "moderator"}]}` |
| `$not` | Negation | `{"age": {"$not": {"$lt": 18}}}` |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PERSIST_MAX_CONNECTIONS` | Max connections per pool | `100` |
| `PERSIST_IDLE_TIMEOUT` | Idle connection timeout | `10m` |
| `PERSIST_DEFAULT_DATABASE` | Default database name | `globular` |

### Configuration File

```json
{
  "port": 10108,
  "maxConnections": 100,
  "idleTimeout": "10m",
  "connections": [
    {
      "id": "default",
      "type": "mongodb",
      "host": "localhost",
      "port": 27017,
      "database": "globular"
    }
  ]
}
```

## Dependencies

This is a core data layer service with no internal dependencies.

## Integration

Used by virtually all Globular services for data persistence:

- [Blog Service](../blog/README.md)
- [Catalog Service](../catalog/README.md)
- [Resource Service](../resource/README.md)
- [Title Service](../title/README.md)
- And more...

---

[Back to Services Overview](../README.md)
