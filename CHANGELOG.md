# Changelog

All notable changes to Globular Services are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/). Versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added
- **Documentation site** at `https://globular.io/docs/` — 50 pages, MkDocs Material, searchable, dark/light mode
- **GitHub Release workflow** (`.github/workflows/release.yml`) — push a tag, get a tarball
- **CI doc validation** (`.github/workflows/docs.yml`) — builds docs, validates CLI commands, checks stale paths
- **Deploy script** (`scripts/deploy-docs.sh`) — one-command docs rebuild and deployment
- **CLI: `globular workflow list/get/diagnose`** — inspect workflow runs, steps, and failure diagnosis
- **CLI: `globular node logs`** — service journal output via node agent
- **CLI: `globular node search-logs`** — pattern-based log search with time range
- **CLI: `globular node certificate-status`** — TLS cert details (SANs, expiry, chain validity)
- **CLI: `globular node control`** — service start/stop/restart/status via node agent
- **Keepalived ingress** — VIP failover between gateway nodes, managed by node agent via etcd spec
- **Per-node interface override** for keepalived (`InterfaceOverride` field in `VIPFailoverSpec`)
- **Let's Encrypt wildcard cert** for `*.globular.io` via ACME DNS-01 through Globular's DNS service
- **xDS external domain cluster creation** — non-gateway domains get automatic Envoy backend clusters
- **Ports reference** page — every port, protocol, and firewall rule in one place
- **Known issues** page — honest catalog of CLI gaps, infrastructure limitations, planned fixes
- **Day-0/1/2 operations** guide — complete lifecycle with printable checklists
- **Local-first development** guide — run services with `go run`, no cluster needed
- **Building from source** guide — real 4-repo, 5-stage build process documented
- **DNS and PKI** guide — internal CA vs Let's Encrypt, ACME flow, split-horizon DNS
- **MCP setup** guide — connect Claude Code to your cluster in 5 minutes
- **AI documentation** (7 files) — rules, agent model, services, operator guide, developer guide, patterns
- **Computing** documentation — batch jobs, placement, verification, retry policies

### Fixed
- **DNS provider authentication** — local DNS provider now passes cluster_id and token for ACME DNS-01 challenges
- **Certificate paths** — all docs corrected from `/etc/globular/creds/` to `/var/lib/globular/pki/`
- **Controller port** — all READMEs corrected from 10000 to 12000
- **Service directory links** — fixed `clustercontroller` → `cluster_controller` across all READMEs
- **Go version** — corrected from 1.21 to 1.24 in golang/README.md
- **Domain README** — wildcard certs marked as supported, reconciler service name corrected, local paths removed
- **Environment variables** — removed fake env var sections from READMEs (etcd is source of truth)

### Changed
- **services/README.md** — full rewrite with accurate service catalog, architecture, and build commands
- **CLAUDE.md** — rewritten with hard rules, architecture notes, etcd schema, known issues, AI rules
- **getting-started.md** — rewritten with real installation paths (GitHub Releases + from source)
- **mkdocs.yml** — site_url set to `https://globular.io/docs/` (served from gateway webroot)
- **MCP server port** — standardized to 10260 (code default, avoids kubelet 10250 conflict)

---

## [0.0.1] — 2026-04-01

### Added
- Initial release of 33 Go microservices
- Protocol Buffer definitions (38 proto files)
- TypeScript gRPC-Web client library
- CLI tool (`globularcli`) with cluster, services, pkg, dns, domain, doctor, ai commands
- MCP server with 122+ diagnostic tools
- AI services: Memory, Executor, Watcher, Router
- Cluster Controller with desired-state management and workflow dispatch
- Node Agent with package management and service control
- Workflow Service for centralized execution
- Cluster Doctor with invariant checking and auto-heal
- Backup Manager with etcd, restic, MinIO, ScyllaDB providers
- Domain reconciler with ACME certificate provisioning
- Compute service (code only, not in build manifest)
- Internal PKI with Ed25519 keystores and automatic certificate provisioning
- gRPC interceptor chain: authentication → RBAC → audit
- 4-layer state model: Repository → Desired → Installed → Runtime
- `install-day0.sh` for single-node cluster bootstrap
