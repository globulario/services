# Echo Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Echo Service is a simple request-response service used for testing, debugging, and as a baseline service template.

## Overview

This lightweight service echoes back messages sent to it, making it useful for connectivity testing, load testing, and as a starting point for creating new services.

## Features

- **Echo Messages** - Returns sent message with count
- **Graceful Shutdown** - Clean service stop
- **Message Counter** - Tracks number of messages received
- **Minimal Footprint** - Lightweight implementation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Echo Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Message Handler                         │ │
│  │                                                            │ │
│  │  Request ──▶ Echo ──▶ Response (message + count)          │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Message Counter                         │ │
│  │                                                            │ │
│  │  Atomic counter tracking total messages processed          │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Methods

| Method | Description | Parameters | Response |
|--------|-------------|------------|----------|
| `Echo` | Echo message back | `message` | `message`, `count` |
| `Stop` | Graceful shutdown | - | - |

### Message Structure

```protobuf
message EchoRequest {
    string message = 1;
}

message EchoResponse {
    string message = 1;
    int64 count = 2;    // Total messages received
}
```

## Usage Examples

### Go Client

```go
import (
    echo "github.com/globulario/services/golang/echo/echo_client"
)

client, _ := echo.NewEchoService_Client("localhost:10100", "echo.EchoService")
defer client.Close()

// Echo a message
response, err := client.Echo("Hello, World!")
fmt.Printf("Response: %s (message #%d)\n", response.Message, response.Count)

// Multiple echoes
for i := 0; i < 10; i++ {
    response, _ := client.Echo(fmt.Sprintf("Message %d", i))
    fmt.Printf("Echo #%d: %s\n", response.Count, response.Message)
}

// Graceful shutdown
err = client.Stop()
```

### Command Line

```bash
# Echo a message
grpcurl -plaintext -d '{"message": "Hello!"}' \
  localhost:10100 echo.EchoService/Echo

# Response:
# {
#   "message": "Hello!",
#   "count": "1"
# }
```

### Health Check Pattern

```go
func checkServiceHealth(endpoint string) bool {
    client, err := echo.NewEchoService_Client(endpoint, "echo.EchoService")
    if err != nil {
        return false
    }
    defer client.Close()

    response, err := client.Echo("health-check")
    return err == nil && response.Message == "health-check"
}
```

## Use Cases

1. **Connectivity Testing** - Verify network connectivity between services
2. **Load Testing** - Baseline performance measurements
3. **Service Template** - Starting point for new services
4. **Health Checks** - Simple service health verification
5. **Learning** - Understand Globular service structure

## Configuration

```json
{
  "port": 10100,
  "maxConcurrent": 1000
}
```

## Dependencies

None - Standalone service with minimal dependencies.

---

[Back to Services Overview](../README.md)
