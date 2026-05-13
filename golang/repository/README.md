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
- **Build Identity** - Immutable `build_id` for deterministic convergence
- **Alias Mapping** - `release_tag + build_number -> canonical build_id`
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
// Legacy ref fields; canonical install identity also requires build_id.
message ArtifactRef {
    string publisher_id = 1;  // e.g., "globular"
    string name = 2;          // e.g., "file-service"
    string version = 3;       // e.g., "v1.0.0"
    string platform = 4;      // e.g., "linux/amd64"
    ArtifactKind kind = 5;    // SERVICE, APPLICATION, AGENT, SUBSYSTEM
}
```

Artifact resolution rules (current behavior):

1. If `build_id` is provided, resolve exact build.
2. Version-only resolution is rejected when multiple published builds exist (ambiguous).
3. Upstream sync dedupes identical checksums across build numbers/build IDs and persists alias records.
4. Reconcile must converge on `build_id` (not `build_number`).

Identity guidance:

- `globular version` (release tag/BOM) is not package semantic `version`.
- `build_id` is immutable artifact identity.
- `build_number` is a release locator only.
- `artifact_sha256` is exact byte identity; never used as a `build_id`.

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
- [Cluster Controller](../cluster_controller/README.md) - Upgrade artifacts
- [Discovery Service](../discovery/README.md) - Package resolution
- [Node Agent](../nodeagent/README.md) - Artifact downloads

## Identity Notes

- `build_id` is immutable artifact identity.
- Same `build_id` with different checksum is a hard conflict.
- Same checksum under same package identity is deduped to a canonical artifact.
- Alias records are written under:
  - `artifacts/aliases/<publisher>/<name>/<version>/<platform>/<release_tag>/<build_number>.json`
- Release-index installs must be pinned (`build_id` + `artifact_sha256`) for deterministic resolution.

---

[Back to Services Overview](../README.md)
