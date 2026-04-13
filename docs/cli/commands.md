# CLI Commands

The `globular` CLI is the primary operator interface for managing the cluster.

## Cluster Management

### `globular cluster bootstrap`
Initialize the first node of a new cluster.

### `globular cluster join --token <TOKEN> --controller <ADDR>`
Add a node to an existing cluster.

### `globular cluster token create`
Generate a join token for new nodes.

### `globular cluster health`
Display overall cluster health, node count, and convergence status.

### `globular cluster nodes list`
List all cluster nodes with status, profiles, and capabilities.

### `globular cluster nodes profiles <node-id> --profile=<name>`
Assign profiles to a node (e.g., `core`, `compute`, `storage`).

## Service Management

### `globular services desired set <name> <version> [--build-number N]`
Set the desired state for a service across the cluster.

### `globular services list-desired`
Compare desired vs installed services, showing status and hash match.

### `globular services apply-desired`
Install all services from the desired state on the local node.

### `globular services repair [--dry-run]`
Diagnose and repair state alignment across all layers.

### `globular services verify-integrity`
Check installed packages against repository manifests (read-only).

### `globular services seed`
Import locally-installed services into the controller's desired state.

## Package Management

### `globular pkg build --spec <yaml> --root <dir> [--version V] [--build-number N]`
Build a package (.tgz) from a spec YAML and payload directory.

### `globular pkg publish --file <tgz>`
Upload a package to the cluster repository.

### `globular pkg info <name>`
Display package metadata and versions.

## Doctor / Diagnostics

### `globular doctor report`
Generate a cluster health report with findings and recommendations.

### `globular doctor heal [--dry-run]`
Auto-remediate known findings.

### `globular doctor heal history`
View past auto-healing actions and their outcomes.

## DNS Management

### `globular dns list`
List DNS records managed by the cluster.

## Backup

### `globular backup create`
Create a backup of cluster state.

### `globular backup list`
List available backups.

### `globular backup restore <id>`
Restore from a backup.

## Support

### `globular support bundle create`
Collect diagnostic data into a support bundle.

## Global Flags

| Flag | Description |
|------|-------------|
| `--controller <addr>` | Cluster controller endpoint (default: localhost:12000) |
| `--node <addr>` | Node agent endpoint (default: localhost:11000) |
| `--ca <path>` | Path to CA bundle |
| `--insecure` | Skip TLS verification |
| `--output <format>` | Output format: table, json, yaml |
| `--timeout <duration>` | Request timeout (default: 5s) |
