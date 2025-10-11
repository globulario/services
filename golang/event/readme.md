
# Event Service (Globular)

A lightweight, streaming **publish/subscribe** service for Globular clusters. It provides fan-out messaging over a single persistent gRPC stream with automatic reconnection and client-side handler routing. Use it to broadcast events between services and frontends with minimal ceremony.

> This README focuses on **client usage**. It assumes an Event Service server is already running and registered in your Globular environment.

---

## Highlights

- **Pub/Sub API**: `Publish(subject, data)` and `Subscribe/UnSubscribe` with context-bound helpers.
- **Single multiplexed stream**: one background `OnEvent` stream per client; events are routed to local handlers.
- **Auto-reconnect**: if the stream drops, the client reconnects and re-subscribes active subjects.
- **Keep-alives handled internally**: KA frames never surface to your handlers.
- **Secure by default**: client contexts automatically include local tokens and identity metadata when available.
- **Fan-out**: one publish is delivered to all subscribers of the subject.

---

## Go Client

### Install

```bash
go get github.com/globulario/services/golang/event/event_client@latest
```

### Create a client

```go
import eventclient "github.com/globulario/services/golang/event/event_client"

c, err := eventclient.NewEventService_Client("globular.io", "event.EventService")
if err != nil { panic(err) }
defer c.Close()
```

- `address`: resolvable domain/peer for your cluster (e.g., `globule-ryzen.globular.io`).
- `id`: service id, typically `event.EventService`.

### Authentication

The client builds an outgoing context that injects a **local service token** and identity metadata (domain, mac) if available; you can also pass your own context when needed.

```go
ctx := c.GetCtx() // includes token when available
```

---

## Common Workflows

### Subscribe and handle messages

Use the context-aware helper to automatically clean up on cancel.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

err := c.SubscribeCtx(ctx, "orders.created", "sub-1", func(e *eventpb.Event) {
    // handle message
    log.Printf("received: %s", string(e.Data))
})
if err != nil { log.Fatal(err) }
```

### Publish an event

```go
if err := c.Publish("orders.created", []byte(`{"id":"A123"}`)); err != nil {
    log.Fatal(err)
}
```

### Unsubscribe explicitly

```go
_ = c.UnSubscribeCtx(ctx, "orders.created", "sub-1")
```

> Tip: `SubscribeCtx` will auto-unsubscribe when `ctx` is done; explicit calls are optional.

---

## Advanced: How it works

- **Background loop (`run`)** establishes a single `OnEvent` stream and dispatches incoming frames to your registered handlers via an internal map keyed by subject and local `uuid`.
- **Resilience**: on stream errors, the client attempts to `Reconnect`, reopens the stream, and **re-subscribes** server-side for all tracked subjects before resuming dispatch.
- **KeepAlive** frames are consumed by the client and never delivered to your handler functions.
- **Action channel**: subscriptions are registered locally via a non-blocking internal `actions` channel to keep the dispatch loop thread-safe.

---

## Testing

A self-contained test suite exercises core behaviors (requires a reachable Event Service):

- **Subscribe → Publish → Receive** round-trip with multiple messages.
- **Unsubscribe** stops delivery.
- **Broadcast/fan-out** to multiple subscribers.
- **KeepAlive transparency**: no spurious handler calls when idle.

Run:

```bash
go test ./event/event_client -v
```

---

## API Surface (selected)

- `NewEventService_Client(address, id) (*Event_Client, error)`
- `(*Event_Client) Publish(name string, data []byte) error`
- `(*Event_Client) SubscribeCtx(ctx context.Context, name, uuid string, f func(*eventpb.Event)) error`
- `(*Event_Client) UnSubscribeCtx(ctx context.Context, name, uuid string) error`
- `(*Event_Client) GetCtx() context.Context`
- `(*Event_Client) Close()`
- `(*Event_Client) StopService()`

---

## Notes & Tips

- Use **stable subject names** (e.g., `orders.created`, `files.deleted`) and keep payloads as small binary or JSON blobs.
- For UI clients, rely on `SubscribeCtx` and cancel the context on unmount to avoid leaks.
- If you need strict delivery guarantees, layer an ack/retry protocol on top of events or use a durable queue service; this Event Service is intended for **best-effort fan-out** in-cluster messaging.
- The client makes several reconnection attempts with short backoff; design your server for idempotent `Subscribe` calls.

---

## License

Part of the Globular project. See repository license for details.
