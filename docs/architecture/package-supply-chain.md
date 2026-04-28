# Package Supply Chain

This document describes how packages flow from source code to running
services across the Globular cluster.

## Platform Release vs Package Version

A **platform release** (e.g. Globular v1.0.84) is a **bill of materials** —
a composition lockfile that references exact package artifacts. It is NOT a
monolithic version stamp applied to every package.

A **package version** represents a contract/content change to that specific
package. If a package did not change between releases, it keeps its
original version.

```
Platform Release v1.0.84 (bill of materials)
├── repository   v1.0.84  build=24  CHANGED    origin=v1.0.84
├── gateway      v1.0.82  build=9   UNCHANGED  origin=v1.0.82
├── dns          v1.0.80  build=15  UNCHANGED  origin=v1.0.80
├── envoy        1.35.3   build=1   UNCHANGED  origin=v1.0.70
├── minio        RELEASE… build=1   UNCHANGED  origin=v1.0.75
└── etcd         3.5.14   build=1   UNCHANGED  origin=v1.0.60
```

## Three-Layer Digest Model

| Digest | Purpose | Changes when... |
|--------|---------|-----------------|
| `package_contract_digest` | Change detection | Binary, manifest, specs, systemd units, profiles, deps, or config change |
| `artifact_sha256` | Download verification | Archive bytes differ (tar/gzip metadata may cause this) |
| `entrypoint_checksum` | Runtime fingerprint | Binary on disk changes |

Change detection uses `package_contract_digest`, NOT `artifact_sha256`. This
ensures identical content packaged with different tar/gzip metadata is correctly
identified as unchanged.

## The Pipeline

```
Source Code
    |
    v
CI Build (detect-changes.py compares contract digests BEFORE version stamping)
    |
    +-- Changed packages: stamp with current platform version, build, package
    +-- Unchanged packages: keep previous version, copy artifact from origin release
    |
    v
release-index.json v2 (bill of materials with per-package versions)
    |
    v
GitHub Release
    +-- Changed package .tgz files (new artifacts)
    +-- release-index.json
    +-- Offline installer tarball (all packages including unchanged copies)
    |
    v
Repository Service (SyncFromUpstream)
    |
    +-- Imports each package with its own version (not platform version)
    +-- Stores in ScyllaDB + MinIO
    +-- Records origin_release and changed_in_release in provenance
    |
    v
Cluster Controller (per-package desired state)
    |
    +-- Each package has independent ServiceDesiredVersion / InfrastructureRelease
    +-- Resolves each package to its own version + build_id
    +-- No global "platform version" in desired state
    |
    v
Node Agent (DownloadArtifact -> Install -> Report)
```

## Change Detection Sequence

The CI pipeline MUST detect changes BEFORE stamping versions:

1. Build all binaries with the PREVIOUS version (from version-overrides.txt)
2. Compute package_contract_digest for each package
3. Compare against previous release-index.json
4. Unchanged packages: keep previous version, skip rebuild
5. Changed packages: rebuild with current platform version
6. Generate release-index.json v2 with per-package versions

If version is stamped before detection, all binaries appear changed because
the embedded version string changes the entrypoint_checksum.

## Data Authority

| Data | Authority | Storage |
|------|-----------|---------|
| Package binary | MinIO (blob cache) | Refillable from upstream |
| Manifest metadata | ScyllaDB (authoritative) | publish_state column |
| Package versions | ScyllaDB (Scylla-first) | ListManifests |
| Artifact resolution | ScyllaDB | ResolveArtifact |
| Upstream sources | etcd | /globular/repository/upstreams/ |
| Credentials | etcd | /globular/credentials/ |
| Platform release composition | release-index.json | GitHub Release asset |

## Invariants

1. **Immutable digest binding**: Once (publisher, name, version, platform) is bound to a sha256 digest, it can never change. Conflicts are rejected.
2. **Package version = content change**: A package version changes only when the package content/contract changes. Platform release version is NOT stamped on unchanged packages.
3. **Deterministic contract digest**: Same binary + manifest + specs + systemd + deps always produces the same contract digest regardless of archive metadata.
4. **Scylla-first reads**: All catalog queries use ScyllaDB as the authoritative source.
5. **MinIO is blob cache**: Missing blobs can be refilled from upstream.
6. **No automatic rollback**: If a package fails, create an incident. Never auto-install older versions.
7. **Referenced releases must not be deleted**: A platform release may reference package artifacts from previous releases. Deleting those releases breaks the asset_url for unchanged packages.
8. **Force full rebuild is explicit**: When previous release index is unavailable, the release fails unless FORCE_FULL_REBUILD=true is set, which records the reason in release-index metadata.
