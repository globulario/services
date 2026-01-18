# Repository Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Repository Service provides artifact storage and version management for deploying services and applications.

## Overview

This service manages the lifecycle of deployable artifacts including services, applications, agents, and subsystems. It handles versioning, platform-specific builds, and secure artifact distribution.

## Features

- **Artifact Storage** - Store and retrieve deployable packages
- **Version Management** - Track multiple versions of artifacts
- **Platform Support** - Platform-specific builds (linux/amd64, etc.)
- **Checksum Verification** - Integrity validation
- **Bundle Support** - Multi-artifact deployment packages
- **Streaming Transfers** - Efficient large file handling

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Repository Service                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Artifact Manager                         │ │
│  │                                                            │ │
│  │   Upload ──▶ Validate ──▶ Store ──▶ Index                 │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Artifact Store                           │ │
│  │                                                            │ │
│  │  ┌─────────────────────────────────────────────────────┐  │ │
│  │  │  Publisher: globular                                 │  │ │
│  │  │  ├── auth-service                                    │  │ │
│  │  │  │   ├── v1.0.0 (linux/amd64, darwin/amd64)         │  │ │
│  │  │  │   └── v1.1.0 (linux/amd64, darwin/amd64)         │  │ │
│  │  │  ├── file-service                                    │  │ │
│  │  │  │   └── v2.0.0 (linux/amd64)                       │  │ │
│  │  │  └── web-app                                         │  │ │
│  │  │      └── v1.0.0 (web)                               │  │ │
│  │  └─────────────────────────────────────────────────────┘  │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Artifact Types

| Type | Description | Example |
|------|-------------|---------|
| **Service** | Backend gRPC services | Authentication, File, Media |
| **Application** | Web applications | Admin dashboard, user portal |
| **Agent** | Local agents | Node agent |
| **Subsystem** | System components | Globular binary |

## API Reference

### Artifact Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `UploadArtifact` | Upload artifact (streaming) | `ref`, `data` |
| `DownloadArtifact` | Download artifact (streaming) | `ref` |
| `ListArtifacts` | List available artifacts | `publisher`, `name` |
| `GetArtifactManifest` | Get artifact metadata | `ref` |

### Bundle Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `UploadBundle` | Upload deployment bundle | `ref`, `data` |
| `DownloadBundle` | Download bundle | `ref` |

### Artifact Reference

```protobuf
message ArtifactRef {
    string publisher_id = 1;  // e.g., "globular"
    string name = 2;          // e.g., "file-service"
    string version = 3;       // e.g., "v1.0.0"
    string platform = 4;      // e.g., "linux/amd64"
    ArtifactKind kind = 5;    // SERVICE, APPLICATION, AGENT, SUBSYSTEM
}
```

## Usage Examples

### Go Client

```go
import (
    repo "github.com/globulario/services/golang/repository/repository_client"
    repopb "github.com/globulario/services/golang/repository/repositorypb"
)

client, _ := repo.NewRepositoryService_Client("localhost:10101", "repository.PackageRepository")
defer client.Close()

// Upload artifact
ref := &repopb.ArtifactRef{
    PublisherId: "mycompany",
    Name:        "my-service",
    Version:     "v1.0.0",
    Platform:    "linux/amd64",
    Kind:        repopb.ArtifactKind_SERVICE,
}

data, _ := os.ReadFile("/path/to/my-service-binary")
err := client.UploadArtifact(ref, data)

// Download artifact
downloadedData, err := client.DownloadArtifact(ref)
os.WriteFile("/path/to/downloaded", downloadedData, 0755)

// List artifacts
artifacts, err := client.ListArtifacts("mycompany", "my-service")
for _, artifact := range artifacts {
    fmt.Printf("%s v%s (%s)\n",
        artifact.Name, artifact.Version, artifact.Platform)
}

// Get manifest
manifest, err := client.GetArtifactManifest(ref)
fmt.Printf("SHA256: %s\n", manifest.Checksum)
fmt.Printf("Size: %d bytes\n", manifest.Size)
```

### Artifact Manifest

```json
{
  "publisher_id": "globular",
  "name": "file-service",
  "version": "v1.0.0",
  "platform": "linux/amd64",
  "kind": "SERVICE",
  "checksum": "sha256:abc123...",
  "size": 15728640,
  "created_at": "2024-01-15T10:30:00Z",
  "metadata": {
    "description": "File management service",
    "dependencies": ["storage-service", "rbac-service"]
  }
}
```

## Configuration

### Configuration File

```json
{
  "port": 10101,
  "storagePath": "/var/lib/globular/repository",
  "maxArtifactSize": "500MB",
  "retentionVersions": 10
}
```

## Integration

Used by:
- [Cluster Controller](../clustercontroller/README.md) - Upgrade artifacts
- [Discovery Service](../discovery/README.md) - Package resolution
- [Node Agent](../nodeagent/README.md) - Artifact downloads

---

[Back to Services Overview](../README.md)
