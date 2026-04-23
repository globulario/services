# Publishing to Repository

This page covers the complete workflow for publishing packages to the Globular Repository service, including authentication, artifact lifecycle management, and CI/CD integration.

## Repository Overview

The Repository service is Globular's package registry. It stores compiled service packages in MinIO object storage, maintains artifact manifests in etcd, and enforces a lifecycle state machine for every artifact.

Publishing is the act of uploading a package to the repository, where it becomes available for deployment through the desired-state model.

## Publishing Workflow

### Step 1: Authenticate

You must be authenticated to publish. The repository validates that your identity matches the publisher field in the package spec:

```bash
# Authenticate (if not already)
globular auth login --username <username> --password <password>
```

For CI/CD systems, use a service account with the `globular-publisher` role:
```bash
globular auth login --username ci-publisher --password <service-account-password>
```

### Step 2: Publish

```bash
globular pkg publish <package-file.tgz>
```

Example:
```bash
globular pkg publish globular-inventory-0.0.1-linux_amd64-1.tgz
```

### What Happens Internally

1. **Authentication check**: The CLI sends the JWT token with the upload request. The repository verifies the token is valid.

2. **Publisher validation**: The repository compares the `publisher` field in the package spec against the authenticated user's identity. If they don't match, the upload is rejected.

3. **Streaming upload**: The package archive is streamed to the repository via gRPC. Large packages are uploaded in chunks.

4. **MinIO storage**: The repository stores the archive in MinIO distributed object storage. All repository instances share the same MinIO cluster.

5. **Identity allocation**: The repository generates a `build_id` (UUIDv7) for the artifact — the sole authoritative identity used for convergence, rollback, and installed-state comparison. The client never provides or controls this value.

6. **Checksum computation**: The repository computes the SHA256 digest of the archive.

7. **Monotonic version check**: If a PUBLISHED release already exists for this package at a higher version, the upload is rejected with `FailedPrecondition`. Same version is allowed (new build at same version).

8. **Immutability check**: If a PUBLISHED artifact already exists at the same (publisher, name, version, platform) with a different digest, the upload is rejected with `AlreadyExists`. Published artifacts are immutable.

9. **Manifest creation**: An `ArtifactManifest` is created in MinIO and ScyllaDB:
   ```
   {
     "ref": {
       "publisher_id": "dev@example.com",
       "name": "inventory",
       "version": "0.0.1",
       "platform": "linux_amd64",
       "kind": "SERVICE"
     },
     "build_id": "019d986b-3632-7297-...",
     "checksum": "sha256:a1b2c3d4e5f6...",
     "build_number": 1,
     "profiles": ["custom"],
     "install_mode": "repository",
     "publish_state": "VERIFIED",
     "provenance": {
       "published_by": "dev@example.com",
       "published_at": 1712937600,
       "auth_method": "jwt"
     }
   }
   ```

7. **Validation**: The repository validates the archive:
   - `bin/` directory exists and contains the service binary
   - `specs/` directory exists and contains a valid spec file
   - Spec file fields are valid (name, version, publisher, platform, kind)

8. **State transitions**: The artifact progresses through lifecycle states:
   - **STAGING** → upload and validation in progress
   - **VERIFIED** → checksum validated, archive structurally valid
   - **PUBLISHED** → fully available for deployment

### Step 3: Verify

Confirm the artifact was published successfully:

```bash
globular pkg info inventory
```

Output:
```
NAME        VERSION  BUILD  PLATFORM     STATE       PUBLISHED AT
inventory   0.0.1    1      linux_amd64  PUBLISHED   2025-04-12 10:30:00
```

## Artifact Lifecycle

### State Machine

```
STAGING ──→ VERIFIED ──→ PUBLISHED ──→ DEPRECATED ─┐
                │              │                    │
                │              ├──→ YANKED ──────────┤──→ REVOKED (terminal)
                │              │                    │
                │              ├──→ QUARANTINED ─────┘
                │              │
                │              └──→ ARCHIVED  ←── GC (soft-delete)
                │
                ├──→ ORPHANED  (descriptor registration failed)
                └──→ FAILED    (pipeline error)

CORRUPTED  ←── system (entrypoint_checksum mismatch detected post-publish)
```

The full semantics of each state are documented in the [Repository Overview](../operators/repository-overview.md).

### Managing Lifecycle

All lifecycle commands use the format `publisher/name version` and accept:
- `--reason` — recorded for audit
- `--platform` — target platform (defaults to current: `linux_amd64`)
- `--build-number` — target a specific build iteration (default: 0 = all builds)
- `--kind` — artifact kind: `service` (default), `application`, `infrastructure`, `command`

**Deprecate** — mark an old version as superseded (still installable by pin):
```bash
globular pkg deprecate myteam@example.com/inventory 0.0.1
globular pkg undeprecate myteam@example.com/inventory 0.0.1
```

**Yank** — block downloads and hide from discovery:
```bash
globular pkg yank myteam@example.com/inventory 0.0.1 --reason "critical regression in order processing"
globular pkg unyank myteam@example.com/inventory 0.0.1
```
New desired-state writes targeting this version will be rejected. Existing installations are unaffected.

**Quarantine** — admin security hold (same effect as YANKED, but admin-only to lift):
```bash
globular pkg quarantine myteam@example.com/inventory 0.0.1 --reason "CVE-2026-5678 under review"
globular pkg unquarantine myteam@example.com/inventory 0.0.1
```

**Revoke** — permanent, terminal, no recovery:
```bash
globular pkg revoke myteam@example.com/inventory 0.0.1 --reason "confirmed supply chain issue"
```
The manifest is kept for audit. The binary stays in MinIO but is inaccessible. Active installations must be replaced manually.

## Provenance

Every published artifact includes immutable provenance data:
- **published_by**: The JWT subject (PrincipalID) of the publisher
- **published_at**: Unix timestamp of the publish operation
- **auth_method**: How the publisher authenticated (`jwt`, `mtls`)

Provenance cannot be modified after publishing. It provides an audit trail answering "who published this artifact, when, and how?"

## CI/CD Integration

### Service Account Setup

```bash
# Create a service account for CI
globular auth create-account --username ci-publisher --type application

# Assign the publisher role
globular rbac bind --subject ci-publisher --role globular-publisher
```

The `globular-publisher` role grants permission to:
- Upload artifacts to the repository
- Query artifact manifests
- Manage artifact lifecycle (deprecate, yank)

It does **not** grant permission to:
- Modify desired state
- Manage cluster membership
- Access other services' data

### CI Pipeline Example

```bash
#!/bin/bash
# CI pipeline: build, test, publish, deploy

set -e

# Authenticate
globular auth login --username ci-publisher --password "$CI_PUBLISHER_PASSWORD"

# Build
./generateCode.sh
cd golang
go test ./inventory/... -race -v
go build -o ../packages/payload/inventory/bin/inventory_server ./inventory/inventory_server
cd ..

# Package
VERSION=$(git describe --tags --always)
BUILD_NUMBER=$CI_BUILD_NUMBER
globular pkg build \
  --spec specs/inventory_service.yaml \
  --root packages/payload/inventory/ \
  --version "$VERSION" \
  --build-number "$BUILD_NUMBER"

# Publish
globular pkg publish "globular-inventory-${VERSION}-linux_amd64-${BUILD_NUMBER}.tgz"

# Optionally deploy (requires operator role, not publisher role)
# globular services desired set inventory "$VERSION"
```

### Separating Publish and Deploy

It's common practice to separate publishing (CI) from deploying (operator):
- CI publishes new versions automatically on every merge
- Operators decide when to deploy by setting desired state
- This prevents untested code from automatically reaching production

## Querying Artifacts

### List All Versions

```bash
globular pkg info <service-name>
```

### Search Artifacts

```bash
# Search by publisher
globular pkg search --publisher myteam@example.com

# Search by profile
globular pkg search --profile compute

# Search by kind
globular pkg search --kind INFRASTRUCTURE
```

### Get Artifact Details

```bash
# Full manifest including checksum, provenance, profiles
globular pkg info inventory --version 0.0.1 --detailed
```

## Practical Scenarios

### Scenario 1: Multi-Platform Publishing

Publishing for both amd64 and arm64:

```bash
# Build amd64
GOOS=linux GOARCH=amd64 go build -o packages/payload/inventory/bin/inventory_server ./inventory/inventory_server
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.1 --platform linux_amd64
globular pkg publish globular-inventory-0.0.1-linux_amd64-1.tgz

# Build arm64
GOOS=linux GOARCH=arm64 go build -o packages/payload/inventory/bin/inventory_server ./inventory/inventory_server
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.1 --platform linux_arm64
globular pkg publish globular-inventory-0.0.1-linux_arm64-1.tgz
```

### Scenario 2: Republishing After Fix

A published version had a subtle bug. You need to fix it without bumping the version:

```bash
# Fix the code
# Rebuild with a new build number
go build -o packages/payload/inventory/bin/inventory_server ./inventory/inventory_server
globular pkg build --spec specs/inventory_service.yaml --root packages/payload/inventory/ --version 0.0.1 --build-number 2
globular pkg publish globular-inventory-0.0.1-linux_amd64-2.tgz

# Deploy the new build
globular services desired set inventory 0.0.1 --build-number 2
```

## What's Next

- [RBAC Integration](rbac-integration.md): Authorization annotations and permission models
- [Application Deployment Model](application-deployment.md): Web application packaging
