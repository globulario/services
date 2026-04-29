# Upstream Release Streams

How Globular keeps clusters up to date — from official platform releases to private enterprise package feeds.

## The Big Picture

```
                     +-----------------------+
                     |   Upstream Providers   |
                     |                       |
                     |  GitHub Releases      |    Public official feed
                     |  HTTP Index Server    |    GitLab/Gitea/static CDN
                     |  Git Repository       |    Private Gitea/Forgejo/bare
                     |  Local Directory      |    Air-gapped / USB / offline
                     +-----------+-----------+
                                 |
                     release-index.json (BOM)
                                 |
                     +-----------v-----------+
                     |   Repository Service   |
                     |                       |
                     |  - Sync/import        |
                     |  - Policy check       |    The ONLY component that
                     |  - Checksum verify    |    talks to upstreams.
                     |  - Normalize          |
                     |  - Quarantine/publish |
                     +-----------+-----------+
                                 |
                     ScyllaDB (catalog) + MinIO (blob cache)
                                 |
                     +-----------v-----------+
                     |  Cluster Controller    |
                     |                       |
                     |  - Per-package desired |    Resolves each package
                     |    state              |    independently. Never
                     |  - Version + build_id |    talks to upstreams.
                     +-----------+-----------+
                                 |
                     +-----------v-----------+
                     |     Node Agent         |
                     |                       |
                     |  - Fetch from repo    |    Downloads artifacts from
                     |  - Install            |    repository gRPC only.
                     |  - Report state       |    No upstream knowledge.
                     +-----------+-----------+
                                 |
                         Running Services
```

**Key rule: Upstream providers are a repository concern, not a cluster concern.** The controller and node-agent only talk to the repository service. If GitHub goes down, the cluster keeps running from its local catalog.

## What is a Release Stream?

A **release stream** is a named source that publishes platform releases as `release-index.json` files with downloadable package artifacts. Every release is a **bill of materials** (BOM) — a composition lockfile listing the exact packages, versions, and digests that make up that platform release.

```
Platform Release v1.0.84 (bill of materials)
+-- repository   v1.0.84  build=24  CHANGED    origin=v1.0.84
+-- gateway      v1.0.82  build=9   UNCHANGED  origin=v1.0.82
+-- dns          v1.0.80  build=15  UNCHANGED  origin=v1.0.80
+-- envoy        1.35.3   build=1   UNCHANGED  origin=v1.0.70
+-- minio        RELEASE… build=1   UNCHANGED  origin=v1.0.75
+-- etcd         3.5.14   build=1   UNCHANGED  origin=v1.0.60
```

Package versions represent **content changes**, not calendar releases. If `gateway` didn't change since v1.0.82, it stays at v1.0.82 even in platform release v1.0.84.

## Provider Types

Globular supports four upstream provider types. All go through the same provider-neutral `ReleaseSource` interface — the sync pipeline doesn't know or care which type it's using.

### GitHub Releases

The default for public distribution. The official Globular package feed uses this.

```bash
globular repo register-upstream \
  --name globulario \
  --type github \
  --owner globulario \
  --repo services \
  --trust-policy import \
  --allowed-publishers core@globular.io \
  --allowed-channels stable \
  --require-checksum true
```

Features:
- Release discovery via GitHub API (`--latest` flag)
- Asset download from release attachments
- Private repo support via `--credentials-ref`
- Rate-limit aware error messages

### HTTP Index

For any web server that hosts `release-index.json` and package archives. Works with GitLab Releases, Gitea, Nexus, Artifactory, S3, or a plain static file server.

```bash
globular repo register-upstream \
  --name company-releases \
  --type http \
  --url "https://releases.company.com/globular/{tag}/release-index.json" \
  --artifact-base-url "https://releases.company.com/globular/packages"
```

Features:
- Simple URL template with `{tag}` substitution
- Separate artifact base URL for CDN/mirror setups
- Bearer token auth via `--credentials-ref`
- Works with any HTTP server that can serve static files

### Local Directory

For air-gapped, USB, or offline installations. Reads release-index.json and package archives directly from the filesystem.

```bash
globular repo register-upstream \
  --name airgap-usb \
  --type local-dir \
  --local-root /mnt/usb/globular-releases \
  --index-path "releases/{tag}/release-index.json"
```

Expected directory layout:
```
/mnt/usb/globular-releases/
  releases/
    v1.0.84/
      release-index.json
  packages/
    repository_1.0.84_linux_amd64.tgz
    gateway_1.0.82_linux_amd64.tgz
    ...
```

Features:
- No network required
- Strict path traversal protection (rejects `..`, absolute paths, symlinks)
- Directory scanning for `--latest` (lists release dirs)
- SHA256 verification same as all other providers

### Git Repository

For organizations running private Git servers (Gitea, Forgejo, GitLab, bare Git). The repository clones/fetches the Git repo into a local cache and reads the release index from the working tree.

```bash
globular repo register-upstream \
  --name internal-releases \
  --type git \
  --repo-url ssh://git@repo.local/globular/releases.git \
  --branch main \
  --index-path "releases/{tag}/release-index.json" \
  --artifact-base-url "https://artifacts.local/packages"
```

Features:
- Clone/fetch into deterministic cache per source
- Tag-based and branch-based release streams
- HTTPS token auth via GIT_ASKPASS (never in URLs)
- SSH via system SSH agent
- Optional: artifacts stored in Git (small deployments) or referenced via HTTP

## Connecting to the Official Globular Feed

### Day-0 (Fresh Install)

When you install Globular from the release tarball, the installer automatically:

1. Copies `release-index.json` to `/var/lib/globular/release-index.json`
2. Publishes local packages to the repository
3. Registers the official upstream source
4. Syncs using the release tag from the BOM

```bash
# This happens automatically during install.sh / install-day0.sh.
# The release tag comes from release-index.json, not package filenames.
```

### Day-1+ (Keeping Up to Date)

```bash
# Check for new releases
globular repo update-check --source globulario --latest

# Output:
# SOURCE: globulario  TAG: v1.0.85
#
# PACKAGE              LOCAL      UPSTREAM   CHANNEL  ACTION     POLICY
# repository           1.0.84     1.0.85     stable   UPDATE     allowed
# gateway              1.0.82     1.0.82     stable   UP_TO_DATE allowed
# dns                  1.0.80     1.0.85     stable   UPDATE     allowed
# envoy                1.35.3     1.35.3     stable   UP_TO_DATE allowed

# Preview the import
globular repo sync --source globulario --latest --dry-run

# Import
globular repo sync --source globulario --latest

# Or via audited workflow
globular pkg sync-upstream --source globulario --latest
```

After sync, the controller automatically detects the new versions and starts converging — rolling out updated services across all nodes.

### Show Release Composition

```bash
globular repo release-show v1.0.84

# Platform Release: Globular 1.0.84
# Release Tag:      v1.0.84
# Publisher:        core@globular.io
#
# PACKAGE              VERSION        BUILD   CHANGED   ORIGIN       KIND
# repository           1.0.84         24      yes       v1.0.84      SERVICE
# gateway              1.0.82         9       no        v1.0.82      SERVICE
# envoy                1.35.3         1       no        v1.0.70      INFRASTRUCTURE
# ...
#
# Changed: 8 / 45   Unchanged: 37 / 45
```

## Connecting Your Own Release Stream

### Scenario: Internal Team Services

Your team builds custom Globular services (e.g. an internal API, a data pipeline). You want to distribute them as packages alongside the official Globular services.

**Step 1: Create a release-index.json for your packages**

```json
{
  "schema_version": "globular.repository.index/v2",
  "platform_release": "2.0.0",
  "release_tag": "v2.0.0",
  "publisher_id": "team@company.com",
  "packages": [
    {
      "name": "data-pipeline",
      "kind": "SERVICE",
      "version": "2.0.0",
      "build_number": 1,
      "build_id": "ci-42",
      "platform": "linux_amd64",
      "package_digest": "sha256:abc123...",
      "asset_url": "https://releases.company.com/packages/data-pipeline_2.0.0_linux_amd64.tgz",
      "origin_release": "v2.0.0",
      "changed_in_release": true,
      "channel": "stable"
    }
  ]
}
```

**Step 2: Host the release-index.json and packages**

Option A — **HTTP server** (recommended):
```
https://releases.company.com/globular/v2.0.0/release-index.json
https://releases.company.com/packages/data-pipeline_2.0.0_linux_amd64.tgz
```

Option B — **Git repository**:
```
releases/v2.0.0/release-index.json
```
with artifacts served from a separate HTTP endpoint.

Option C — **Local directory** (air-gapped):
```
/opt/company-releases/releases/v2.0.0/release-index.json
/opt/company-releases/packages/data-pipeline_2.0.0_linux_amd64.tgz
```

**Step 3: Register the upstream**

```bash
# HTTP
globular repo register-upstream \
  --name company-services \
  --type http \
  --url "https://releases.company.com/globular/{tag}/release-index.json" \
  --artifact-base-url "https://releases.company.com/packages" \
  --allowed-publishers "team@company.com" \
  --trust-policy quarantine

# Git
globular repo register-upstream \
  --name company-services \
  --type git \
  --repo-url git@gitlab.company.com:infra/releases.git \
  --branch main \
  --index-path "releases/{tag}/release-index.json" \
  --artifact-base-url "https://releases.company.com/packages" \
  --credentials-ref /globular/credentials/gitlab-key

# Local directory
globular repo register-upstream \
  --name company-services \
  --type local-dir \
  --local-root /opt/company-releases \
  --index-path "releases/{tag}/release-index.json"
```

**Step 4: Sync**

```bash
# Quarantined by default — review first
globular repo sync --source company-services --tag v2.0.0 --dry-run
globular repo sync --source company-services --tag v2.0.0

# Review quarantined packages
globular repo list-artifacts

# Promote after review
globular repo set-state --name data-pipeline --state PUBLISHED
```

### Scenario: GitLab/Gitea/Forgejo Releases

If your Git hosting provides a release/download mechanism:

```bash
# GitLab: use HTTP_INDEX with the release download URL
globular repo register-upstream \
  --name gitlab-releases \
  --type http \
  --url "https://gitlab.company.com/api/v4/projects/42/releases/{tag}/assets/links" \
  --artifact-base-url "https://gitlab.company.com/infra/globular/-/releases"

# Gitea/Forgejo: use HTTP_INDEX with the release asset URL
globular repo register-upstream \
  --name gitea-releases \
  --type http \
  --url "https://gitea.local/api/v1/repos/infra/releases/releases/tags/{tag}/assets"
```

### Scenario: Air-Gapped / USB Install

```bash
# On the connected machine: download the release
curl -LO https://github.com/globulario/services/releases/download/v1.0.84/globular-1.0.84-linux-amd64.tar.gz

# Copy to USB
cp globular-1.0.84-linux-amd64.tar.gz /mnt/usb/

# On the air-gapped cluster:
tar xzf /mnt/usb/globular-1.0.84-linux-amd64.tar.gz -C /opt/

# Register as LOCAL_DIR source
globular repo register-upstream \
  --name offline-feed \
  --type local-dir \
  --local-root /opt/globular-1.0.84-linux-amd64 \
  --trust-policy import

# Sync
globular repo sync --source offline-feed --tag v1.0.84
```

## For Code Maintainers and Developers

### Building Packages

```bash
# Build a single service package
globular pkg build \
  --spec packages/metadata/my-service/specs/my_service.yaml \
  --root dist/staging/my-service \
  --out dist/packages \
  --version 1.2.0 \
  --publisher "team@company.com"

# Publish to the local repository
globular pkg publish --file dist/packages/my-service_1.2.0_linux_amd64.tgz
```

### Creating a Release Index

The release-index.json is the bill of materials. You can generate it manually or use the CI pipeline:

```python
# CI generates release-index.json from built packages
python3 golang/build/detect-changes.py \
  --prev-index prev-release-index.json \
  --metadata-dir packages/metadata \
  --bin-dir dist/bin \
  --bin-map-json build/bin-map.json \
  --version 2.0.0 \
  --tag v2.0.0 \
  --output-overrides dist/version-overrides.txt \
  --output-manifest dist/change-manifest.json
```

The `detect-changes.py` script:
1. Computes `package_contract_digest` for each package (binary + manifest + specs + systemd + deps)
2. Compares against the previous release index
3. Marks unchanged packages — they keep their original version
4. Changed packages get the new platform version

### The Three-Layer Digest Model

| Digest | Purpose | What it covers |
|--------|---------|----------------|
| `package_contract_digest` | Change detection in CI | Binary checksum, package.json, spec YAML, systemd unit, profiles, hard_deps, provides/requires, defaults |
| `artifact_sha256` | Download verification | Raw .tgz archive bytes |
| `entrypoint_checksum` | Runtime process fingerprint | Binary on disk |

`package_contract_digest` is independent of tar/gzip metadata. Same content always produces the same digest, even if packaged on different machines at different times.

### Version Rules

1. **Package version changes only when content changes.** If `gateway` source code, spec, systemd unit, and deps haven't changed, it keeps its previous version.
2. **Platform release version is the BOM label.** v1.0.84 means "this is the 84th platform release", not "every package is version 1.0.84".
3. **Infrastructure packages keep upstream versions.** etcd 3.5.14, envoy 1.35.3, MinIO RELEASE.2025-09-07.
4. **Versions are allocated by the repository.** Use `--bump patch/minor/major` — the repository enforces monotonicity.

## MCP Integration (AI-Assisted Operations)

The MCP server exposes upstream tools so AI agents can manage release streams:

| MCP Tool | Purpose |
|----------|---------|
| `repository_upstream_list` | List all registered upstream sources with status |
| `repository_upstream_sync` | Sync packages with dry-run, per-package results |
| `repository_upstream_register` | Register any provider type |
| `repository_upstream_remove` | Remove a source |
| `repository_active_release` | Read the active platform BOM |
| `repository_list_artifacts` | Shows build_id, channel, upstream provenance |
| `repository_get_artifact_manifest` | Full provenance including origin_release, contract digest |

## Security Model

| Property | How it works |
|----------|-------------|
| **Credentials never logged** | Auth tokens resolved from etcd `/globular/credentials/`, never in URLs or logs |
| **Asset URLs redacted** | Query params stripped in audit events and error messages |
| **Trust policy** | `import` (PUBLISHED immediately) or `quarantine` (requires manual promotion) |
| **Publisher filtering** | `allowed_publishers` restricts who can publish packages through a source |
| **Kind filtering** | `allowed_kinds` restricts package types (SERVICE, INFRASTRUCTURE, etc.) |
| **Channel filtering** | `allowed_channels` restricts release channels (stable, candidate, etc.) |
| **Checksum mandatory** | `require_checksum=true` rejects entries without sha256 |
| **Fail closed** | If source is disabled, missing, or policy mismatches → reject, never import |
| **Audit trail** | Every sync, refill, rejection emits structured audit events |
| **Path traversal protection** | LOCAL_DIR/GIT_INDEX validate paths against root with symlink evaluation |

## Disaster Recovery

### MinIO Loses Package Blobs

The repository **automatically recovers** from the upstream source:
1. Node agent requests a download
2. MinIO blob is missing
3. Repository checks: manifest has upstream_import? source exists and enabled? trust policy allows refill? publisher/kind/channel pass policy?
4. Downloads from the original provider (any type — HTTP, local, Git)
5. Verifies sha256 checksum
6. Refills MinIO cache
7. Streams to the node agent

This is transparent — no node-agent changes needed.

### Upstream Source Unavailable

The cluster keeps running from its local repository catalog. When the upstream comes back online, sync resumes from where it left off. The `last_synced_tag` only advances on fully clean runs.

### Active BOM Missing

If `/var/lib/globular/release-index.json` is lost, Day-1 join binaries fall back to "latest published" (logged as warning). Restore by copying the BOM from another node or re-running install-day0.sh.

## Quick Reference

```bash
# ── Registration ──────────────────────────────────────────────────
globular repo register-upstream --type github --name x --owner o --repo r
globular repo register-upstream --type http --name x --url 'https://.../{tag}/index.json'
globular repo register-upstream --type local-dir --name x --local-root /mnt/usb/releases
globular repo register-upstream --type git --name x --repo-url ssh://git@host/repo.git

# ── Discovery ─────────────────────────────────────────────────────
globular repo update-check --source x --latest
globular repo update-check --source x --tag v1.0.84 --json
globular repo release-show v1.0.84

# ── Sync ──────────────────────────────────────────────────────────
globular repo sync --source x --tag v1.0.84 --dry-run
globular repo sync --source x --latest
globular pkg sync-upstream --source x --latest

# ── Management ────────────────────────────────────────────────────
globular repo list-upstreams
globular repo inspect-upstream x
globular repo remove-upstream x
```
