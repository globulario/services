# Repository Upstreams

Upstream sources let a Globular cluster import packages from external release
registries (e.g. GitHub Releases). This document covers registration, sync,
policy, trust, and the update-check workflow.

## Quick Start

```bash
# 1. Register the official upstream
globular repo register-upstream \
  --name globulario-github \
  --url "https://github.com/globulario/services/releases/download/{tag}/release-index.json"

# 2. Check for updates
globular repo update-check --source globulario-github --tag v1.0.30

# 3. Preview what would be imported
globular repo sync --source globulario-github --tag v1.0.30 --dry-run

# 4. Import
globular repo sync --source globulario-github --tag v1.0.30
```

## Concepts

### Release Index

Every release tag publishes a `release-index.json` that lists all packages
in the release. Schema version: `globular.repository.index/v1`.

Required fields per entry: `name`, `version`, `platform`, `asset_url`,
`package_digest` (sha256:hex64).

### Sync Identity Model

- Artifact key = `(publisher, name, version, platform)`
- Each key binds to exactly one immutable `package_digest`
- Same key + same digest → idempotent skip
- Same key + different digest → rejected (audit event, never imported)

### Trust Policy

| Policy | Behavior |
|--------|----------|
| `import` (default) | Imported packages are set to PUBLISHED immediately |
| `quarantine` | Imported packages are set to QUARANTINED — recorded but not installable until manually promoted |

### Import Policy Fields

| Flag | Purpose |
|------|---------|
| `--publisher` | Default publisher ID for entries without one |
| `--allowed-kinds` | Comma-separated kinds to accept (e.g. SERVICE,INFRASTRUCTURE) |
| `--allowed-channels` | Comma-separated channels to accept |
| `--require-checksum` | Reject entries without sha256 digest |
| `--trust-policy` | "import" or "quarantine" |
| `--credentials-ref` | etcd key under /globular/credentials/ for auth token |

## Commands

### `globular repo register-upstream`

Registers a named upstream source. Overwrites if name exists.

### `globular repo list-upstreams`

Lists all registered sources with status. Credentials are redacted.

### `globular repo remove-upstream <name>`

Removes a registered source.

### `globular repo sync`

Direct sync (Day-0, no workflow). Suitable for bootstrap.

### `globular pkg sync-upstream`

Workflow-tracked sync (Day-1+). Goes through WorkflowService for auditing.

### `globular repo update-check`

Read-only comparison of upstream release index against local catalog.

## Upstream Refill (DownloadArtifact)

When `allow_upstream_fallback` is set on DownloadArtifactRequest and the
local MinIO blob is missing, the repository server checks the manifest's
`upstream_import` record. If present, it re-downloads from the original
`asset_url`, verifies the sha256 checksum, refills the MinIO cache, and
streams to the caller. This is transparent to the node-agent.

## Sync Status Tracking

After each sync, the upstream source record in etcd is updated with:
- `last_sync_unix` — timestamp of last sync attempt
- `last_sync_status` — "succeeded", "partial", or "failed"
- `last_sync_error` — error details when failed
- `last_synced_tag` — only advances on fully clean runs

## Security

- Credentials are stored in etcd under `/globular/credentials/` only
- Credentials are never logged
- ListUpstreams redacts credential references in responses
- All sync RPCs require cluster-admin authorization
- Downloaded artifacts are verified against sha256 checksums before storage
