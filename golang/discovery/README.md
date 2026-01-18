# Discovery Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Discovery Service handles service and application discovery, publishing, and installation planning.

## Overview

This service enables publishing services and applications to the repository, resolving dependencies, and planning installations based on node profiles and constraints.

## Features

- **Service Publishing** - Register services to repository
- **Application Publishing** - Publish web applications
- **Dependency Resolution** - Resolve package dependencies
- **Installation Planning** - Generate install plans
- **Profile Constraints** - Match packages to node profiles

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Discovery Service                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Publisher                                │ │
│  │                                                            │ │
│  │  Package ──▶ Validate ──▶ Upload ──▶ Register             │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Dependency Resolver                        │ │
│  │                                                            │ │
│  │  Package ──▶ Dependencies ──▶ Resolve ──▶ Ordered List    │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Installation Planner                       │ │
│  │                                                            │ │
│  │  Target Profile ──▶ Match Packages ──▶ Generate Plan      │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Publishing Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `PublishService` | Publish a service | `serviceDescriptor`, `artifact` |
| `PublishApplication` | Publish an application | `appDescriptor`, `artifact` |

### Discovery Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `GetPackageDescriptor` | Get package metadata | `publisherId`, `name`, `version` |
| `ResolveInstallPlan` | Generate installation plan | `packages[]`, `profiles[]`, `constraints` |

### Package Descriptor

```protobuf
message PackageDescriptor {
    string id = 1;
    string name = 2;
    string publisher = 3;
    string version = 4;
    string description = 5;
    string type = 6;          // SERVICE, APPLICATION
    repeated string profiles = 7;  // Required profiles
    repeated Dependency dependencies = 8;
    map<string, string> metadata = 9;
}

message Dependency {
    string package = 1;
    string version = 2;       // Semver constraint
    bool optional = 3;
}
```

## Usage Examples

### Go Client

```go
import (
    discovery "github.com/globulario/services/golang/discovery/discovery_client"
)

client, _ := discovery.NewDiscoveryService_Client("localhost:10106", "discovery.DiscoveryService")
defer client.Close()

// Publish a service
descriptor := &discoverypb.PackageDescriptor{
    Id:          "my-service",
    Name:        "My Service",
    Publisher:   "mycompany",
    Version:     "1.0.0",
    Description: "A custom service",
    Type:        "SERVICE",
    Profiles:    []string{"core", "compute"},
    Dependencies: []*discoverypb.Dependency{
        {Package: "persistence-service", Version: ">=1.0.0"},
        {Package: "rbac-service", Version: ">=1.0.0"},
    },
}
artifact, _ := os.ReadFile("my-service-linux-amd64")
err := client.PublishService(descriptor, artifact)

// Get package info
pkg, err := client.GetPackageDescriptor("mycompany", "my-service", "1.0.0")
fmt.Printf("Package: %s v%s\n", pkg.Name, pkg.Version)
fmt.Printf("Dependencies: %v\n", pkg.Dependencies)

// Resolve installation plan
plan, err := client.ResolveInstallPlan(
    []string{"my-service"},    // Packages to install
    []string{"core"},          // Target profiles
    nil,                       // No additional constraints
)

fmt.Println("Installation order:")
for i, step := range plan.Steps {
    fmt.Printf("%d. %s v%s\n", i+1, step.Package, step.Version)
}
```

### Publish Application

```go
appDescriptor := &discoverypb.PackageDescriptor{
    Id:          "admin-dashboard",
    Name:        "Admin Dashboard",
    Publisher:   "mycompany",
    Version:     "2.0.0",
    Description: "Web-based admin interface",
    Type:        "APPLICATION",
    Metadata: map[string]string{
        "entryPoint": "index.html",
        "framework":  "react",
    },
}

appBundle, _ := os.ReadFile("admin-dashboard.zip")
err := client.PublishApplication(appDescriptor, appBundle)
```

## Configuration

```json
{
  "port": 10106,
  "repositoryAddress": "localhost:10101",
  "cacheEnabled": true,
  "cacheTTL": "1h"
}
```

## Dependencies

- [Repository Service](../repository/README.md) - Artifact storage
- [Resource Service](../resource/README.md) - Package descriptors

---

[Back to Services Overview](../README.md)
