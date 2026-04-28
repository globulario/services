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

### Release Index Schema

Every release tag publishes a `release-index.json` (schema: `globular.repository.index/v1`).

Required per entry: `name`, `version`, `platform`, `asset_url`, `package_digest`.

Optional: `build_number` (int64), `build_id` (string), `channel`, `kind`, `publisher`.

When `build_number` is missing, the repository derives a deterministic positive
value from `build_id + digest` to avoid collisions. When `build_id` is missing,
it is derived from `(publisher, name, version, platform, digest)`.

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

### Channel Handling

Each release-index entry may include a `channel` field. The normalization chain:
1. Entry `channel` (highest priority)
2. Source `channel` (from register-upstream)
3. `"stable"` (default)

The `allowed_channels` policy validates the **normalized** entry channel.

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

When a MinIO blob is missing, DownloadArtifact automatically attempts upstream
refill using **server policy** — no client changes required. Old node-agent
callers benefit transparently.

Server-policy refill is allowed when ALL conditions hold:
- Manifest exists with `upstream_import` record
- Manifest has a checksum for verification
- Publish state is downloadable (not YANKED/QUARANTINED/REVOKED)
- The named upstream source exists and is enabled in etcd
- Source `trust_policy` is not `"quarantine"` (quarantine blocks auto-refill)
- Manifest publisher/kind/channel pass the source's `allowed_*` policy

The `allow_upstream_fallback` request field still works as an explicit override.

If any trust check fails, refill is rejected (fail closed) and the download
returns NotFound as before.

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
- Upstream refill verifies source trust before downloading (fail closed)

## Phase 2 Roadmap

### GitHub Latest-Release Discovery

Phase 1 requires an explicit `release_tag` for all sync and update-check
operations. Phase 2 will add automatic latest-release discovery:

- Add `repo_url` field to `UpstreamSource` for `GITHUB_RELEASE` sources
  (e.g. `"globulario/services"`)
- `repo update-check --source <name> --latest` fetches the latest GitHub
  Release via the GitHub API, extracts the `release-index.json` asset, and
  compares against the local catalog
- `repo sync --source <name> --latest` imports from the latest release
- For private repos, `credentials_ref` provides the GitHub API token
- No auto-sync in Phase 2 — operator must initiate

### PublishToUpstream

Phase 1 is pull-only. Phase 2 will add the ability to push artifacts to
upstream registries:

- `GenerateReleaseIndex()` already exists for building release-index.json
  from locally published packages
- New RPC: `PublishToUpstream` — creates/updates a GitHub Release, uploads
  `.tgz`, `.sha256`, and `release-index.json` assets
- RBAC action: `repository.upstream.publish`
- Uses `credentials_ref` for write access (separate key from read token)
- Never triggered automatically by `UploadArtifact` — explicit operator action
- Workflow-tracked via `repository.publish.upstream` workflow
- Phase 2 will also support HTTP PUT endpoints for non-GitHub registries
