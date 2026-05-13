# Globular Package Model

This document defines the canonical package model for Globular Repository.

## Design Decision

The package model is based on `ArtifactManifest` / `ArtifactRef` from `repository.proto`.
The legacy `PackageBundle` / `PackageDescriptor` path (spread across Resource and Discovery)
remains available for backward compatibility but is not the target architecture.

## Layered Identity Model

- `globular version`: platform/BOM release identity (`vX.Y.Z`)
- `package version`: semantic compatibility of one package
- `build_id`: immutable artifact identity (canonical install identity)
- `build_number`: release/index locator (legacy-compatible locator, not canonical identity)
- `checksum` (`artifact_sha256`): exact archive byte identity
- `package_contract_digest`: normalized install/runtime contract identity
- `entrypoint_checksum`: runtime executable fingerprint

Core invariants:

- same `build_id` + different checksum => hard conflict (reject/quarantine)
- same `publisher/name/version/platform/checksum` => dedupe to one canonical artifact
- resolver/reconcile must converge on `build_id`, never on `build_number`

## Globular Version vs Package Version

- `globular_version` identifies a platform release (BOM/release-index scope).
- `package version` identifies semantic compatibility of one package.
- A platform release may include many package versions that do not equal the platform version.
- Repository and reconcile flows must not auto-stamp package versions from platform version.

## Release Index Pin Semantics

Install/sync validation requires release-index package entries to include identity pins:

- `name`
- `version`
- `platform`
- `kind`
- `build_id`
- `artifact_sha256` (legacy fallback `checksum` accepted during migration windows)

Resolution order:

1. `build_id` exact match
2. alias (`release_tag + build_number`)
3. `publisher/name/version/platform` only when exactly one published build exists
4. otherwise reject as ambiguous

## Conflict Matrix

| Case | Required behavior |
|---|---|
| same `build_id` + same checksum | idempotent |
| same `build_id` + different checksum | hard conflict, reject/quarantine |
| same package identity + same checksum + different `build_number` | dedupe + alias |
| same package identity + same checksum + different `build_id` | dedupe/alias (no silent duplicate blob) |
| same package identity + different checksum + different `build_id` | valid distinct builds |
| missing `build_id` with multiple candidates | reject as ambiguous |

## Date Fields Are Metadata Only

- `generated_at`, `published_at`, `imported_at`, `modified_unix`, `published_unix` are operational metadata.
- Date fields must never decide artifact identity or conflict resolution.

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
| `build_id`             | string            | Immutable build identity (UUID/derived)  |
| `build_number`         | int64             | Build locator within a release/index     |
| `checksum`             | string            | SHA256 of archive bytes                  |
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
artifacts/{publisher}%{name}%{version}%{platform}%{build_number}.manifest.json   – ArtifactManifest (JSON)
artifacts/{publisher}%{name}%{version}%{platform}%{build_number}.bin              – Archive binary
artifacts/aliases/{publisher}/{name}/{version}/{platform}/{release_tag}/{build_number}.json
packages-repository/{UUID}.tar.gz                                   – Legacy bundles (read-only compat)
```

Both local filesystem and MinIO/S3 backends are supported via the `storage_backend.Storage`
abstraction.
