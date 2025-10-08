# Log Service (Go)

A small, efficient gRPC service for structured application logs. It lets you:

- Append log entries (with level, app, method, message, fields)
- Query logs with a simple path+query language (time windows, text search, method/component filters)
- Delete a single log entry or clear a whole app/level
- Keep storage lean via time-bucket indexes and a retention janitor

---

## Table of contents

- [Overview](#overview)
- [API](#api)
- [Query language](#query-language)
- [Go client](#go-client)
- [Configuration](#configuration)
- [TLS / mTLS](#tls--mtls)
- [Building & testing](#building--testing)
- [Troubleshooting](#troubleshooting)
- [Design notes](#design-notes)
- [License](#license)

---

## Overview

The service stores logs in a simple key/value backend and maintains two kinds of indexes:

- **Coarse index:** `(level, app)` → list of log IDs
- **Time buckets:** `(level, app, minute)` → list of log IDs for that minute

A background **retention janitor** prunes old buckets to keep storage bounded. Defaults: keep logs for **7 days**, sweep every **5 minutes**.

Log levels supported: `info`, `debug`, `error`, `fatal`, `trace`, `warning`.

---

## API

Protobuf service (partial, names only):

```
service LogService {
  rpc Log(LogRqst) returns (LogRsp);
  rpc GetLog(GetLogRqst) returns (stream GetLogRsp);
  rpc DeleteLog(DeleteLogRqst) returns (DeleteLogRsp);
  rpc ClearAllLog(ClearAllLogRqst) returns (ClearAllLogRsp);
}
```

Important messages/fields (simplified):

- `LogInfo`:
  - `id` (server-generated, stable per level|app|method|line)
  - `application` (string, **required**)
  - `method` (string, **required**, e.g. `/svc/Op`)
  - `line` (string, **required**, caller’s file:line or logical site)
  - `level` (enum)
  - `message` (string)
  - `timestamp_ms` (int64, optional; server will set if missing)
  - `component` (string, optional)
  - `fields` (map<string,string>, optional)
  - `occurences` (int32, coalesced repeat count)

- `LogRqst { LogInfo info }` / `LogRsp { bool result }`
- `GetLogRqst { string query }` / `GetLogRsp { repeated LogInfo infos }` (streamed)
- `DeleteLogRqst { LogInfo log }` / `DeleteLogRsp { bool result }`
- `ClearAllLogRqst { string query }` / `ClearAllLogRsp { bool result }`

**Auth:** Requests must carry a valid token (added by the client). The server validates tokens. Recommended RBAC:
- `Log`: write
- `GetLog`: read
- `DeleteLog`: delete
- `ClearAllLog`: admin

---

## Query language

Base path (required):
```
/{level}/{application}/*
```

Examples:
```
/info/my.App/*
/error/*/*?contains=timeout
```

Supported query parameters:
- `since=<ms>` — inclusive lower bound (Unix ms)
- `until=<ms>` — inclusive upper bound (Unix ms)
- `limit=<N>` — number of entries to return (default 100)
- `order=asc|desc` — sort by timestamp (default asc)
- `method=/svc/Op` — exact match on method
- `component=<name>` — exact match on component
- `contains=<substr>` — case-insensitive substring match in the message

### Method in the path (optional)

You may specify a third path segment for an **exact** method filter:

```
/info/my.App/*                # no method filter
/info/my.App/%2Fsvc%2FA       # method is "/svc/A" (URL-encoded)
/info/my.App/\x2Fsvc\x2FA     # accepted; decoded to "/svc/A"
```

If a method appears both in the path and as a `method=` param, the **query param wins**.

Invalid shapes (e.g. missing level/app) produce `InvalidArgument` errors.

---

## Go client

```go
import (
  log_client "github.com/globulario/services/golang/log/log_client"
  "github.com/globulario/services/golang/log/logpb"
  "fmt"
)

func example() {
  c, err := log_client.NewLogService_Client("globule-ryzen.globular.io", "log.LogService")
  if err != nil { panic(err) }
  defer c.Close()

  ctx := c.GetCtx() // carries token/domain/mac

  // Write a log
  _ = c.LogCtx(ctx, "my.App", "user", "/svc/Op",
    logpb.LogLevel_INFO_MESSAGE, "hello world", "L42", "main", "")

  // Query: time window + contains
  infos, err := c.GetLogCtx(ctx, "/info/my.App/*?since=1710000000000&contains=hello")
  if err != nil { panic(err) }
  for _, li := range infos { fmt.Println(li.Message) }

  // Delete one (requires permission)
  _ = c.DeleteLog(infos[0], "<token>")

  // Clear app/level (admin)
  _ = c.ClearLog("/info/my.App/*", "<token>")
}
```

Client highlights:
- `NewLogService_Client(address, id)` initializes and connects with retries.
- `GetCtx()` attaches token/domain/mac (and lets you override token per call).
- Helpers collect server streams into slices for convenience.

---

## Configuration

This service integrates with the Globular configuration model. Pertinent environment variables (client-side and tests):

- `GLOBULAR_CLIENT_VERBOSE_INIT` — print init diagnostics
- `GLOBULAR_WAIT_HEALTH` — wait for gRPC health before returning from init
- `GLOBULAR_TLS_DIR` — base dir for client TLS material (defaults to a user-writable path)
- `GLOBULAR_TLS_INSTALL` — set to `0` to **disable** auto-install of client certificates
- `GLOBULAR_TLS_HOST_OVERRIDE` — force the TLS SNI/servername
- `GLOBULAR_TLS_SERVERNAME` — override SNI used by the gRPC dialer

Retention (server defaults):
- `RetentionHours` (default `7*24`)
- `SweepEverySeconds` (default `300`)

---

## TLS / mTLS

- If the target service is **TLS-enabled**, the client must present a client certificate (mTLS) and trust the service CA.
- For **local** targets, the client derives client paths by replacing `server` with `client` in the configured cert paths.
- For **remote** peers, the client looks under `${GLOBULAR_TLS_DIR or defaults}/<effective-host>/` for:
  - a client key (e.g. `*key*.{key,pem}`),
  - a client cert (e.g. `*cert*` or `*.crt`),
  - a CA file (e.g. `*ca*.{pem,crt}`).
- If not found and `GLOBULAR_TLS_INSTALL != 0`, the client will attempt to **install** client certificates into that directory.
- To avoid `permission denied` errors, ensure the **client private key** is readable by the process *that runs your tests/app*. In production, keep the private key restricted to the service account that runs your client.

**Why does the client need the private key?**  
Because mTLS authenticates *both* ends. The client signs the TLS handshake using its private key; the server verifies that signature with the corresponding client certificate. Without read access to the private key, the client cannot complete the handshake.

---

## Building & testing

```bash
# build all
go build ./...

# run unit tests for the Go client
go test ./log/log_client -v
```

The tests cover:
- Append + get basics
- Exact method filtering (path and `method=`)
- Time-window + `contains` substring match
- Single-entry delete
- Bulk clear for `/{level}/{app}/*`
- Malformed query handling

---

## Troubleshooting

**`permission denied` loading client.pem**  
Give the running user read access to the client private key (or point `GLOBULAR_TLS_DIR` to a user-writable directory where the client can install/fetch its certs). Avoid making private keys world-readable; prefer a dedicated user/group.

**“the query must be like /{level}/{application}[/[*|method]]”**  
Shape the path as documented. Examples:
- `/info/my.App/*`
- `/info/my.App/%2Fsvc%2FA`
- `/error/*/*?contains=timeout`

**No results for `contains` with a time window**  
Ensure your `since/until` bounds include the timestamps you just wrote (they’re in **Unix milliseconds**). When in doubt, omit the window first to validate matches, then add it back.

---

## Design notes

- Time-bucketed index per minute for fast `since/until` scans.
- Coalesced entries by `(level|app|method|line)` with an `occurences` counter.
- Retention janitor advances oldest pointers after sweeping expired buckets.

---

## License

Apache-2.0 (or your project’s license).

