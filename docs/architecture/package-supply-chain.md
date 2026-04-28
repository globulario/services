# Package Supply Chain

This document describes how packages flow from source code to running
services across the Globular cluster.

## The Pipeline

```
Source Code
    │
    ▼
CI Build (generateCode.sh + build-all-packages.sh)
    │
    ├─ Produces: .tgz archives with package.json
    ├─ Computes: sha256 checksum, entrypoint_checksum
    └─ Publishes: release-index.json to GitHub Release
    │
    ▼
Repository Service (UploadArtifact or SyncFromUpstream)
    │
    ├─ Stores binary in MinIO (blob cache)
    ├─ Stores manifest in ScyllaDB (authoritative ledger)
    ├─ Validates checksum, assigns build_id
    └─ Sets publish state: PUBLISHED or QUARANTINED
    │
    ▼
Cluster Controller (ResolveArtifact → Desired State)
    │
    ├─ Queries repository for latest PUBLISHED version
    ├─ Writes desired version + build_id to etcd
    └─ Never resolves at execution time — build_id is in desired state
    │
    ▼
Node Agent (DownloadArtifact → Install → Report)
    │
    ├─ Downloads from repository gRPC
    ├─ Installs to /var/lib/globular/packages/
    ├─ Reports installed state to etcd
    └─ Falls back through: staged cache → local → repository
```

## Upstream Import Path

For packages not built locally, the upstream import path provides a
secure supply chain from external registries:

```
External Registry (GitHub Releases)
    │
    ▼
release-index.json (schema_version: globular.repository.index/v1)
    │
    ├─ Validated: schema version, required fields, digest format
    ├─ Policy: allowed publishers/kinds/channels, require_checksum
    └─ Trust: "import" → PUBLISHED, "quarantine" → QUARANTINED
    │
    ▼
SyncFromUpstream
    │
    ├─ Downloads each asset
    ├─ Verifies sha256 checksum
    ├─ Enriches manifest from package.json
    ├─ Preserves upstream build_id (deterministic)
    └─ Records upstream_import provenance
    │
    ▼
Normal Repository Pipeline (ScyllaDB + MinIO)
```

## Data Authority

| Data | Authority | Storage |
|------|-----------|---------|
| Package binary | MinIO (blob cache) | Refillable from upstream |
| Manifest metadata | ScyllaDB (authoritative) | publish_state column |
| Package versions | ScyllaDB (Scylla-first) | ListManifests |
| Artifact resolution | ScyllaDB | ResolveArtifact |
| Upstream sources | etcd | /globular/repository/upstreams/ |
| Credentials | etcd | /globular/credentials/ |

## Invariants

1. **Immutable digest binding**: Once (publisher, name, version, platform) is bound to a sha256 digest, it can never change. Conflicts are rejected.
2. **Deterministic build_id**: Upstream imports preserve the original build_id or derive one deterministically from package identity + checksum.
3. **Scylla-first reads**: All catalog queries (List, Search, GetVersions, Resolve, GC, Reconciler) use ScyllaDB as the authoritative source.
4. **MinIO is blob cache**: If a MinIO blob is missing but the manifest exists in Scylla with upstream_import, the blob can be refilled from upstream.
5. **No automatic rollback**: If a package fails, create an incident. Never auto-install older versions.
