# Globular Package Model

This document defines the canonical package model for Globular Repository.

## Design Decision

The package model is based on `ArtifactManifest` / `ArtifactRef` from `repository.proto`.
The legacy `PackageBundle` / `PackageDescriptor` path (spread across Resource and Discovery)
remains available for backward compatibility but is not the target architecture.

## Package Kinds

```
SERVICE          – gRPC microservice (binary + proto + systemd unit)
APPLICATION      – Web application (static assets + config + RBAC declarations)
INFRASTRUCTURE   – Platform component (etcd, minio, envoy, prometheus)
AGENT            – Sidecar / agent process
SUBSYSTEM        – Compound subsystem
```

## Manifest Schema

Every package has an `ArtifactManifest` containing:

### Universal Fields (all kinds)

| Field                  | Type              | Description                              |
|------------------------|-------------------|------------------------------------------|
| `ref`                  | ArtifactRef       | publisher, name, version, platform, kind |
| `checksum`             | string            | SHA256 of archive                        |
| `size_bytes`           | int64             | Archive size                             |
| `modified_unix`        | int64             | Last modification timestamp              |
| `published_unix`       | int64             | Publication timestamp                    |
| `description`          | string            | Human-readable summary                   |
| `keywords`             | []string          | Search terms                             |
| `icon`                 | string            | Base64 data-uri or URL                   |
| `alias`                | string            | Human-friendly display name              |
| `license`              | string            | License identifier                       |
| `min_globular_version` | string            | Minimum compatible Globular version      |
| `provides`             | []string          | Capabilities this package provides       |
| `requires`             | []string          | Capabilities required (dependencies)     |
| `defaults`             | map<string,string> | Default configuration values             |
| `entrypoints`          | []string          | Executable paths                         |

### Service-Specific Fields (`ServiceDetail`)

| Field                    | Type     | Description                          |
|--------------------------|----------|--------------------------------------|
| `proto_file`             | string   | Proto file name (e.g. "rbac.proto")  |
| `grpc_service_name`      | string   | Full gRPC name                       |
| `default_port`           | int32    | Default listening port               |
| `systemd_unit`           | string   | Systemd unit name                    |
| `service_dependencies`   | []string | Required gRPC service names          |

### Application-Specific Fields (`ApplicationDetail`)

| Field               | Type              | Description                            |
|---------------------|-------------------|----------------------------------------|
| `route`             | string            | URL path (e.g. "/apps/admin")          |
| `index_file`        | string            | Entry HTML file                        |
| `actions`           | []string          | RBAC action declarations               |
| `roles`             | []string          | RBAC role names                        |
| `groups`            | []string          | RBAC group names                       |
| `set_as_default`    | bool              | Become default app on install          |
| `required_services` | []string          | Backend services needed                |
| `app_config`        | map<string,string> | Optional application defaults          |

### Infrastructure-Specific Fields (`InfrastructureDetail`)

| Field                  | Type     | Description                              |
|------------------------|----------|------------------------------------------|
| `component`            | string   | Component name (etcd, minio, envoy)      |
| `config_template`      | string   | Default config content or path           |
| `data_dirs`            | []string | Directories to create with ownership     |
| `health_endpoint`      | string   | Health check URL or command              |
| `upgrade_strategy`     | string   | "stop-start", "rolling", "blue-green"   |
| `required_privileges`  | []string | Required privileges (root, cap_net_bind) |

## Legacy Compatibility

The following legacy paths remain functional and are not removed:

- `UploadBundle` / `DownloadBundle` / `ListBundles` – gob-encoded PackageBundle RPCs
- `discovery.PublishService()` / `discovery.PublishApplication()` – publish coordination
- `resource.GetPackageDescriptor()` / `resource.SetPackageDescriptor()` – descriptor CRUD

The `UploadBundle` handler performs a dual-write: it stores the legacy bundle AND writes
an artifact copy with a manifest. This ensures the artifact catalog stays populated even
when clients use the legacy path.

New package lifecycle work should converge on the artifact path:
`UploadArtifact` / `DownloadArtifact` / `ListArtifacts` / `SearchArtifacts`.

## Storage Layout

```
artifacts/{publisher}%{name}%{version}%{platform}.manifest.json   – ArtifactManifest (JSON)
artifacts/{publisher}%{name}%{version}%{platform}.bin              – Archive binary
packages-repository/{UUID}.tar.gz                                   – Legacy bundles (read-only compat)
```

Both local filesystem and MinIO/S3 backends are supported via the `storage_backend.Storage`
abstraction.
