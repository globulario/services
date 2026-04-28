# Repository Upstreams

GitHub is an **upstream source**. The local Globular repository is the **cluster source of truth**. Imported artifacts become local repository artifacts with manifest, checksum, provenance, state, and policy. The cluster never depends on GitHub availability after import.

## Quick Start

```bash
# Register the official Globular package feed (trusted, explicit)
globular repo register-upstream \
  --name globulario \
  --repo-url globulario/globular-packages \
  --trust-policy import \
  --allowed-publishers core@globular.io \
  --allowed-channels stable \
  --require-checksum true

# Check for latest updates
globular repo update-check --source globulario --latest

# Preview what would be imported
globular repo sync --source globulario --latest --dry-run

# Import
globular repo sync --source globulario --latest
```

## Architecture

```
GitHub Releases (upstream)
    |
    v
SyncFromUpstream (policy check, checksum verify, normalize)
    |
    v
Local Repository (ScyllaDB + MinIO = cluster SOT)
    |
    v
Controller → Node Agent → Running Services
```

## Concepts

### Release Index Schema

Every release tag publishes a `release-index.json` (schema: `globular.repository.index/v1`).

Required per entry: `name`, `version`, `platform`, `asset_url`, `package_digest`.

Optional: `build_number` (int64), `build_id` (string), `channel`, `kind`, `publisher`.

### Trust Policy

| Policy | Behavior |
|--------|----------|
| `import` (default for official) | Imported packages set to PUBLISHED immediately |
| `quarantine` (default for user upstreams) | Imported packages set to QUARANTINED — not installable until manually promoted |

### Safe Defaults

When registering with `--repo-url` (user-provided upstream), safe defaults apply:
- `trust_policy = quarantine`
- `require_checksum = true`
- `allowed_channels = stable`

Override explicitly to trust a source.

## Examples

### Official Globular Package Feed

```bash
globular repo register-upstream \
  --name globulario \
  --repo-url globulario/globular-packages \
  --trust-policy import \
  --allowed-publishers core@globular.io \
  --allowed-channels stable \
  --require-checksum true
```

### User-Owned GitHub Release Feed

```bash
# Safe defaults: quarantine, checksum required, stable only
globular repo register-upstream \
  --name team-packages \
  --repo-url myorg/packages

# Sync specific tag
globular repo sync --source team-packages --tag v2.0.0 --dry-run
globular repo sync --source team-packages --tag v2.0.0

# Promote quarantined packages after review
globular repo set-state --name my-service --state PUBLISHED
```

### Private GitHub Repo

```bash
# Store token in etcd (one-time setup)
etcdctl put /globular/credentials/github-token "ghp_xxxxxxxxxxxx"

# Register with credentials
globular repo register-upstream \
  --name private-feed \
  --repo-url myorg/private-packages \
  --credentials-ref /globular/credentials/github-token

# Token is never printed by CLI. ListUpstreams shows "(set)".
```

### Manual Sync with Explicit Tag

```bash
# Day-0 bootstrap (direct, no workflow)
globular repo sync --source globulario --tag v1.0.30

# Day-1+ audited sync
globular pkg sync-upstream --source globulario --tag v1.0.30
```

### Update-Check with --latest and --json

```bash
# Human-readable table
globular repo update-check --source globulario --latest

# JSON for admin UI
globular repo update-check --source globulario --latest --json
```

### Quarantine Workflow

```bash
# Register untrusted source (defaults to quarantine)
globular repo register-upstream --name experimental --repo-url someone/packages

# Sync — packages are QUARANTINED, not installable
globular repo sync --source experimental --tag v1.0.0

# Review packages
globular repo list-artifacts

# Promote after review
globular repo set-state --name their-service --state PUBLISHED
```

### Disaster Recovery: MinIO Blob Lost

If MinIO loses package blobs, DownloadArtifact **automatically recovers** from the upstream source:

1. Node agent requests a package download
2. MinIO blob is missing
3. Repository checks server policy: manifest has upstream_import, source exists and is enabled, trust policy allows refill, publisher/kind/channel pass policy
4. Repository re-downloads from the original asset URL
5. Verifies sha256 checksum
6. Refills MinIO cache
7. Streams to the node agent

This is **transparent** — old node-agent callers work without changes. An audit event `upstream.refill.success` is emitted. If any policy check fails, refill is rejected (fail closed) and an `upstream.refill.rejected` event is emitted.

## Commands

| Command | Purpose |
|---------|---------|
| `repo register-upstream` | Register or update an upstream source |
| `repo list-upstreams` | List all sources (credentials redacted) |
| `repo remove-upstream <name>` | Remove a source |
| `repo inspect-upstream <name>` | Show full source config and sync status |
| `repo sync --source <n> --tag <t>` | Import from explicit tag |
| `repo sync --source <n> --latest` | Import from latest GitHub release |
| `repo update-check --source <n> --latest` | Compare latest upstream vs local |
| `repo update-check --source <n> --tag <t>` | Compare specific tag vs local |
| `repo publish-upstream --dry-run` | Preview publish plan (no writes) |
| `pkg sync-upstream` | Workflow-tracked sync (audited) |

## Security

- Credentials stored in etcd under `/globular/credentials/` only
- Credentials never logged, never in audit events, never in CLI output
- Asset URLs redacted in audit events (query params stripped)
- ListUpstreams redacts credential references to `(set)`
- All sync RPCs require cluster-admin authorization
- All downloads verified against sha256 checksums
- Upstream refill verifies source trust before downloading (fail closed)
- Every refill emits audit event: `upstream.refill.{success,failed,rejected}`

## Publish (Dry-Run Only)

`repo publish-upstream --source <n> --tag <t> --dry-run` generates a publish plan from local PUBLISHED packages. Without `--dry-run`, returns:

    "upstream publish execution is not implemented yet; use --dry-run to preview"

Future implementation will: create GitHub Release, upload .tgz + release-index.json, require RBAC action `repository.upstream.publish`, never auto-publish from UploadArtifact.
