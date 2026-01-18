# Event Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Event Service provides a lightweight publish-subscribe (pub/sub) messaging system for inter-service communication in the Globular platform.

## Overview

Services can publish events to named channels and subscribe to receive events from those channels. This enables loose coupling between services and supports event-driven architectures.

## Features

- **Named Channels** - Events are organized by topic/channel names
- **Real-Time Streaming** - Server-sent events via gRPC streams
- **Keep-Alive** - Automatic heartbeat messages to maintain connections
- **Flexible Payloads** - Events carry arbitrary byte data
- **Multi-Subscriber** - Multiple clients can subscribe to the same channel

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Event Service                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    Channel Manager                        │   │
│  │                                                           │   │
│  │   Channel: "user.created"                                 │   │
│  │     └─ Subscribers: [client1, client2, client3]          │   │
│  │                                                           │   │
│  │   Channel: "order.completed"                              │   │
│  │     └─ Subscribers: [client4, client5]                   │   │
│  │                                                           │   │
│  │   Channel: "system.health"                                │   │
│  │     └─ Subscribers: [client1]                            │   │
│  │                                                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                   Event Dispatcher                        │   │
│  │                                                           │   │
│  │   Publisher ──▶ Channel ──▶ [Subscriber1, Subscriber2]   │   │
│  │                                                           │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Methods

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `Subscribe` | Subscribe to a channel | `name` (channel) | Blocks until unsubscribe |
| `Unsubscribe` | Stop listening to channel | `name` (channel) | `success` |
| `Publish` | Send event to channel | `name`, `data` | `success` |
| `OnEvent` | Stream events for connection | `uuid` | Stream of `Event` |
| `Quit` | Close event stream | `uuid` | `success` |

### Event Structure

```protobuf
message Event {
    string name = 1;    // Channel name
    bytes data = 2;     // Event payload
    string uuid = 3;    // Connection identifier
}
```

## Event Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Publisher   │     │Event Service │     │  Subscriber  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │                    │  1. Subscribe      │
       │                    │◀───────────────────│
       │                    │                    │
       │                    │  2. OnEvent stream │
       │                    │───────────────────▶│
       │                    │                    │
       │  3. Publish        │                    │
       │───────────────────▶│                    │
       │                    │                    │
       │                    │  4. Event data     │
       │                    │───────────────────▶│
       │                    │                    │
       │  5. Publish        │                    │
       │───────────────────▶│                    │
       │                    │                    │
       │                    │  6. Event data     │
       │                    │───────────────────▶│
       │                    │                    │
       │                    │  7. Unsubscribe    │
       │                    │◀───────────────────│
```

## Common Event Channels

| Channel Pattern | Description | Example |
|-----------------|-------------|---------|
| `user.*` | User lifecycle events | `user.created`, `user.deleted` |
| `file.*` | File system events | `file.uploaded`, `file.modified` |
| `auth.*` | Authentication events | `auth.login`, `auth.logout` |
| `system.*` | System-level events | `system.startup`, `system.shutdown` |
| `cluster.*` | Cluster events | `cluster.node.joined`, `cluster.node.left` |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `EVENT_KEEP_ALIVE_INTERVAL` | Heartbeat frequency | `30s` |
| `EVENT_MAX_CHANNELS` | Maximum channels per connection | `100` |
| `EVENT_BUFFER_SIZE` | Event queue buffer size | `1000` |

### Configuration File

```json
{
  "port": 10102,
  "keepAliveInterval": "30s",
  "maxChannels": 100,
  "bufferSize": 1000
}
```

## Usage Examples

### Go Client - Publishing

```go
import (
    "context"
    event "github.com/globulario/services/golang/event/event_client"
)

client, _ := event.NewEventService_Client("localhost:10102", "event.EventService")
defer client.Close()

// Publish an event
data := []byte(`{"userId": "123", "action": "created"}`)
err := client.Publish("user.created", data)
if err != nil {
    log.Fatal("Failed to publish:", err)
}
```

### Go Client - Subscribing

```go
import (
    "context"
    event "github.com/globulario/services/golang/event/event_client"
)

client, _ := event.NewEventService_Client("localhost:10102", "event.EventService")
defer client.Close()

// Subscribe to events
uuid := "my-unique-connection-id"
go func() {
    err := client.Subscribe("user.created")
    if err != nil {
        log.Println("Subscribe ended:", err)
    }
}()

// Listen for events
stream, err := client.OnEvent(uuid)
if err != nil {
    log.Fatal(err)
}

for {
    evt, err := stream.Recv()
    if err != nil {
        break
    }
    fmt.Printf("Received event on %s: %s\n", evt.Name, string(evt.Data))
}
```

### JavaScript Client

```javascript
// Using globular-web-client
const eventClient = new EventServiceClient("https://api.example.com");

// Subscribe and listen
eventClient.subscribe("user.created");
eventClient.onEvent((event) => {
    console.log(`Event: ${event.name}`, JSON.parse(event.data));
});

// Publish
eventClient.publish("user.created", JSON.stringify({
    userId: "123",
    timestamp: Date.now()
}));
```

## Use Cases

### Service Communication

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ User Service│    │Event Service│    │Email Service│
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                   │
       │ Publish:         │                   │
       │ user.registered  │                   │
       │─────────────────▶│                   │
       │                  │                   │
       │                  │ Event: user.      │
       │                  │ registered        │
       │                  │──────────────────▶│
       │                  │                   │
       │                  │                   │ Send welcome
       │                  │                   │ email
```

### Real-Time Updates

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Web Client │    │Event Service│    │ File Service│
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                   │
       │ Subscribe:       │                   │
       │ file.uploaded    │                   │
       │─────────────────▶│                   │
       │                  │                   │
       │                  │  Publish:         │
       │                  │  file.uploaded    │
       │                  │◀──────────────────│
       │                  │                   │
       │ Real-time update │                   │
       │◀─────────────────│                   │
       │                  │                   │
       │ UI refresh       │                   │
```

## Best Practices

1. **Channel Naming** - Use dot-separated hierarchical names (`domain.entity.action`)
2. **Payload Size** - Keep event payloads small; reference data by ID rather than embedding
3. **Error Handling** - Always handle stream errors and reconnect if needed
4. **Unsubscribe** - Clean up subscriptions when no longer needed
5. **Idempotency** - Design subscribers to handle duplicate events gracefully

## Integration

The Event Service is used by:

- **Blog Service** - New post notifications
- **File Service** - File change events
- **Conversation Service** - Real-time messages
- **Cluster Controller** - Node status updates

## Dependencies

None - The Event Service is a standalone infrastructure component.

---

[Back to Services Overview](../README.md)
