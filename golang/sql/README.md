# SQL Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The SQL Service provides a database abstraction layer for SQL databases including MySQL, PostgreSQL, and SQLite.

## Overview

This service enables applications to execute SQL queries across different database engines through a unified API with support for transactions and parameterized queries.

## Features

- **Multi-Database Support** - MySQL, PostgreSQL, SQLite
- **Parameterized Queries** - Safe query execution
- **Transaction Support** - ACID compliance
- **Streaming Results** - Efficient large result handling
- **Connection Pooling** - Efficient resource management

## Supported Databases

| Database | Driver | Use Case |
|----------|--------|----------|
| **MySQL** | `mysql` | General-purpose RDBMS |
| **PostgreSQL** | `postgres` | Advanced features, JSON support |
| **SQLite** | `sqlite3` | Embedded, single-file database |

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Configure database connection | `id`, `driver`, `host`, `port`, `user`, `password`, `database` |
| `DeleteConnection` | Remove connection | `id` |
| `Ping` | Test connection | `id` |

### Query Execution

| Method | Description | Parameters |
|--------|-------------|------------|
| `QueryContext` | Execute SELECT (streaming) | `id`, `query`, `params` |
| `ExecContext` | Execute INSERT/UPDATE/DELETE | `id`, `query`, `params` |

## Usage Examples

### Go Client

```go
import (
    sql "github.com/globulario/services/golang/sql/sql_client"
)

client, _ := sql.NewSqlService_Client("localhost:10111", "sql.SqlService")
defer client.Close()

// Create connection
err := client.CreateConnection(
    "main-db",
    "postgres",
    "localhost",
    5432,
    "user",
    "password",
    "myapp",
)

// Execute SELECT query
rows, err := client.QueryContext("main-db",
    "SELECT id, name, email FROM users WHERE age > $1",
    []interface{}{18},
)

for row := range rows {
    fmt.Printf("ID: %v, Name: %v, Email: %v\n",
        row["id"], row["name"], row["email"])
}

// Execute INSERT
result, err := client.ExecContext("main-db",
    "INSERT INTO users (name, email, age) VALUES ($1, $2, $3)",
    []interface{}{"John", "john@example.com", 25},
)
fmt.Printf("Rows affected: %d, Last ID: %d\n",
    result.RowsAffected, result.LastInsertId)

// Execute UPDATE
result, err = client.ExecContext("main-db",
    "UPDATE users SET verified = true WHERE id = $1",
    []interface{}{123},
)

// Execute DELETE
result, err = client.ExecContext("main-db",
    "DELETE FROM users WHERE last_login < $1",
    []interface{}{time.Now().AddDate(0, -6, 0)},
)
```

### Transaction Example

```go
// Begin transaction
tx, err := client.Begin("main-db")
if err != nil {
    log.Fatal(err)
}

// Execute operations
_, err = tx.ExecContext(
    "UPDATE accounts SET balance = balance - $1 WHERE id = $2",
    []interface{}{100, "account-a"},
)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

_, err = tx.ExecContext(
    "UPDATE accounts SET balance = balance + $1 WHERE id = $2",
    []interface{}{100, "account-b"},
)
if err != nil {
    tx.Rollback()
    log.Fatal(err)
}

// Commit transaction
err = tx.Commit()
```

## Configuration

### Configuration File

```json
{
  "port": 10111,
  "connections": [
    {
      "id": "main-db",
      "driver": "postgres",
      "host": "localhost",
      "port": 5432,
      "user": "app_user",
      "database": "myapp",
      "maxConnections": 25,
      "idleTimeout": "10m"
    }
  ]
}
```

## Parameter Placeholders

| Database | Placeholder |
|----------|-------------|
| MySQL | `?` |
| PostgreSQL | `$1`, `$2`, ... |
| SQLite | `?` or `$1`, `$2`, ... |

## Dependencies

None - Core data layer service.

---

[Back to Services Overview](../README.md)
