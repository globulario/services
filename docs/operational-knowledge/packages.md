# Globular Package System — Definitive Reference

This document is the canonical reference for what a Globular package is, what files it must contain, how those files are validated, and how a package is installed on a node. It is the manual any AI agent or human operator should read before authoring, building, publishing, installing, or validating a package.

It is the prose companion to:

- `packages/registry.yaml` — the canonical list of every package in the platform (single source of truth for kind, profiles, dependencies, bootstrap tier).
- `packages/metadata/<name>/` — the per-package definition (declarative; checked into git).
- `packages/specs/<name>_{service,cmd}.yaml` — the install recipe (declarative; checked into git).
- `golang/globularcli/pkgpack/` — the Go code that parses, validates, builds, and signs packages.
- `globular-installer/pkg/installer/specplan_builder.go` — the runtime that turns spec steps into executed actions on a node.

If this document and code disagree, code wins. File a fix here.

---

## 1. What a package IS

A **Globular package** is an immutable, content-addressed `.tgz` artifact that describes one installable unit. Every node in the cluster is reconciled toward a desired set of packages; the node-agent installs and supervises whatever the cluster controller says is desired.

Three properties define a package:

1. **Identity** — `(name, version, build_id, platform, publisher)` uniquely names the artifact. `build_id` is a UUIDv7 allocated by the repository; `build_number` is a display-only monotonic counter. **Never overload these.**
2. **Kind** — `service`, `infrastructure`, `command`, or (rare) `application`. Kind drives lifecycle: services have gRPC APIs and run under desired state; infrastructure runs daemons but doesn't expose a mesh API; commands are CLI tools with no daemon.
3. **Recipe** — an ordered list of declarative install steps the node-agent executes to bring the package from "downloaded blob" to "running and healthy". Every step is idempotent.

A package is NOT:

- a Debian/RPM/Snap. It is a Globular-internal format; the install recipe can call into `dpkg` (`install_local_debs`) or `apt` (`install_os_packages`), but the package itself is a `.tgz` with a strict layout.
- a container image. Packages target the host filesystem and systemd directly.
- a source bundle. Packages ship **built binaries** (or bundled .debs for OS-managed daemons like ScyllaDB).

---

## 2. The 4-layer state model — where a package lives

A package's identity travels through four independent layers, each owned by a different actor. **Never collapse these.** This is HARD RULE #2 of the codebase.

| Layer | Question it answers | Owner | Source |
|---|---|---|---|
| **1. Repository (Artifact)** | "Does this `(name, version, build_id)` exist?" | Repository service | `globular` MinIO bucket + ScyllaDB metadata index |
| **2. Desired Release** | "What `(name, version)` SHOULD be running on each node?" | Cluster controller | etcd: `/globular/resources/DesiredService/{name}`, `/globular/resources/ServiceRelease/{name}` |
| **3. Installed Observed** | "What is actually installed on this node?" | Node-agent | etcd: `/globular/nodes/{node_id}/packages/{kind}/{name}` + local receipts under `/var/lib/globular/repository/installable/` |
| **4. Runtime Health** | "Is the process running and healthy?" | systemd + node-agent | systemd unit state, port probes, entrypoint_checksum match against `/proc/<pid>/exe` |

A package is only "shipped" when all four layers agree. The diagnosis pattern for every package-related incident is:

```
walk: Repository → Desired → Installed → Runtime
```

Skipping layers is the most common mistake an AI agent will make.

---

## 3. On-disk layout of a package source tree

Every package owns a directory under `packages/metadata/<name>/` and a matching spec file under `packages/specs/<name>_{service,cmd}.yaml`. The full layout for a typical service package:

```
packages/
├── registry.yaml                                   # canonical package list (kind, profiles, deps)
├── specs/
│   └── <name>_service.yaml                         # install recipe (read by build, by node-agent)
├── metadata/
│   └── <name>/
│       ├── package.json                            # reference copy of the manifest (committed)
│       ├── awareness.yaml                          # invariants, failure modes, ownership
│       ├── specs/
│       │   └── <name>_service.yaml                 # SAME spec as packages/specs/<name>_*.yaml (mirror)
│       ├── systemd/
│       │   └── globular-<name>.service             # optional — if not inline in spec, ships as file
│       ├── config/                                 # optional — default config templates
│       │   └── <name>/
│       │       └── <name>.yaml
│       ├── scripts/                                # optional — pre/post-install / pre-start hooks
│       │   ├── pre-start.sh
│       │   ├── post-install.sh
│       │   └── ...
│       └── debs/                                   # optional — bundled .deb files (ScyllaDB only today)
│           └── *.deb
└── scripts/                                        # cross-package validators and helpers
    └── validate-package-metadata.sh
```

When the package is built (`globular pkg build` → `BuildPackages` in `golang/globularcli/pkgpack/builder.go`), the staging directory is shaped like this and tarred into `<name>_<version>_<goos>_<goarch>.tgz`:

```
<staging>/
├── package.json           # the AUTHORITATIVE manifest (stamped at build time)
├── bin/                   # built binaries — entrypoint + extra_binaries
│   └── <exec>
├── specs/                 # the install recipe
│   └── <name>_service.yaml
├── config/                # optional — config templates
│   └── <name>/...
├── systemd/               # optional — .service unit files (if spec uses external content)
│   └── globular-<name>.service
├── scripts/               # optional — pre/post-install scripts
│   └── *.sh
├── debs/                  # optional — bundled .deb files
│   └── *.deb
└── data/                  # optional — service data files (e.g. workflow definitions)
    └── ...
```

Build guarantees enforced by `assertPackageGuards`:

- `bin/<exec>` MUST exist.
- `specs/<name>_*.yaml` MUST exist.
- `specs/<name>_*.yaml` MUST contain an `install_package_payload` step (the recipe must say where the binary goes).

If any guard fails, the build aborts. There is no override.

---

## 4. `package.json` — the manifest

`package.json` is the per-package manifest. Two copies exist:

- `packages/metadata/<name>/package.json` — the committed **reference copy**. Used by the validator (`scripts/validate-package-metadata.sh`) and as documentation. May lag behind a build.
- Inside the built `.tgz` at the root — the **authoritative copy**, stamped at build time with the current version, `build_number`, `build_id` (after upload), entrypoint checksum, etc.

The full schema is `Manifest` in `golang/globularcli/pkgpack/manifest.go`. Required and important fields:

| Field | Type | Source | Notes |
|---|---|---|---|
| `type` | enum | spec `metadata.kind` | `service` \| `infrastructure` \| `command` \| `application` (rare). MUST match `registry.yaml`. |
| `name` | string | spec `metadata.name` | Canonical hyphenated name (`cluster-controller`, `node-agent`). |
| `version` | string | `--version` at build | Exact version tag. SemVer is preferred for Globular-built services, but upstream-native tags are allowed (for example `RELEASE.2025-09-07T16-13-09Z`). NEVER hardcoded in source — injected via ldflags for Go services. |
| `build_number` | int64 | `--build-number` at build | Display-only monotonic counter. **Plain integer only. Never encoded in `version`. Never used for convergence.** |
| `build_id` | string | repository (post-upload) | UUIDv7. **The sole identity used for convergence.** Empty in the committed reference copy. |
| `platform` | string | build host or `--platform` | `<goos>_<goarch>` (e.g. `linux_amd64`). |
| `publisher` | string | `--publisher` | Defaults to `core@globular.io`. |
| `entrypoint` | string | spec `metadata.entrypoint` or `service.exec` | Path inside the package, e.g. `bin/cluster_controller_server`. `bin/noop` for OS-managed (ScyllaDB, keepalived). |
| `entrypoint_checksum` | string | SHA256 of `bin/<exec>` at build time | Prefix `sha256:`. Used by node-agent runtime check (`/proc/<pid>/exe` vs disk). |
| `defaults.spec` | string | derived | Relative path inside package to the spec, e.g. `specs/echo_service.yaml`. |
| `defaults.configDir` | string | derived | Relative path to config dir if `config/<name>/` was bundled; empty otherwise. |
| `defaults.scriptsDir` | string | derived | `scripts` if scripts were bundled; empty otherwise. |
| `profiles` | []string | spec `metadata.profiles` | Node profiles that require this package. E.g. `[core, compute]`, `[control-plane]`, `[storage]`. |
| `priority` | int | spec `metadata.priority` | Start order. Lower starts first; default `1000`. Used by node-agent to sequence startups. |
| `systemd_unit` | string | spec or auto-derived | Default name pattern: `globular-<name>.service`. Some packages reuse upstream units (ScyllaDB → `scylla-server.service`). |
| `health_check_unit` | string | derived | Defaults to `systemd_unit`. |
| `health_check_port` | int | spec `metadata.health_check.port` | Optional. Used by node-agent for TCP probe. |
| `provides_capabilities` | []string | spec | Logical capabilities granted (e.g. `[config-store]`, `[object-store]`, `[local-db]`). |
| `hard_deps` | []string | spec `metadata.hard_deps` | Install/activation blockers. Graph edges. **Cannot start until all hard_deps are installed and healthy.** |
| `runtime_uses` | []string | spec `metadata.runtime_uses` | Informational API peers (gRPC service names). NOT graph edges. |
| `install_mode` | enum | spec | `repository` (default) or `day0_join`. `day0_join` means it must be present on a node before it can even join the cluster. |
| `managed_unit` | bool | spec | If true, included in bulk profile-wide unit actions (stop, restart, mask). |
| `channel` | string | spec | `stable` (default), `candidate`, `canary`, `dev`, `bootstrap`. Drives release-channel routing. |
| `keywords`, `description`, `license` | strings | spec | Documentation / search. |

**Hard rules around manifest fields:**

- `type` in `package.json` MUST equal `metadata.kind` in the spec MUST equal the entry in `registry.yaml` MUST equal `component_catalog.go`. The `validate-package-metadata.sh` script enforces this across all four sources.
- `entrypoint_checksum` is computed from the BUILT binary. If a fix is hot-deployed by `go build` without ldflags, the checksum will diverge — node-agent runtime check will report `hash_drift` and the package will never converge. **NEVER hot-deploy locally-built binaries.**
- `build_id` is empty in the committed `package.json` but is the **sole** identifier used for convergence after upload. Convergence by `build_number` is a bug.
- `version` MUST NOT encode build numbers (`1.2.3+b325`, `1.2.3-b12`, etc.). Build ordering uses `build_number` only.

---

## 5. The spec file — the install recipe

`packages/specs/<name>_{service,cmd}.yaml` is the install recipe. It is read at three points:

1. **Build time** — `BuildPackage` reads the spec to figure out what to stage (binary, config, systemd, scripts, debs). It is also embedded in the package at `specs/<name>_*.yaml`.
2. **Install time** — the node-agent (via `specplan_builder.BuildInstallPlan`) reads the spec from the unpacked package and executes each step in order.
3. **Validation time** — `pkgpack.ValidateSpec` and `validate-package-metadata.sh` cross-check spec kind against `package.json` and `registry.yaml`.

### 5.1 Spec top-level structure

```yaml
version: 1                    # required, MUST be 1

metadata:
  name: <name>                # required (or derivable from filename / service.name)
  kind: service|infrastructure|command|application   # default: derived from filename (_service → service, _cmd → command)
  description: "..."
  keywords: [a, b, c]
  license: Apache-2.0
  channel: stable

  # Build hints
  entrypoint: bin/<exec>      # override; use "noop" for OS-managed packages with no Globular binary
  install_bins: true|false    # override; false skips bin/ extraction (ScyllaDB, keepalived)
  extra_binaries: []          # additional binaries to bundle alongside the main entrypoint
  bundle_debs: []             # OS package names to download as .deb at build time

  # Catalog (lifecycle, profiles, dependencies)
  profiles: [core]            # node profiles that require this package
  priority: 1000              # start order; lower starts first
  install_mode: repository    # or day0_join
  managed_unit: true          # include in bulk unit actions
  systemd_unit: globular-<name>.service  # override; default: globular-<name>.service
  provides_capabilities: []
  hard_deps: []               # graph edges — install blockers
  runtime_uses: []            # informational gRPC API peers — NOT graph edges
  health_check:
    unit: globular-<name>.service
    port: 12345

service:                      # required for kind=service; absent for command
  name: <name>
  exec: <name>_server

steps:                        # required, at least one; MUST include install_package_payload for service/infrastructure
  - id: <unique-id>
    type: <step-type>
    <type-specific-fields>
```

### 5.2 Step types — the complete catalogue

The node-agent's install runner accepts these step types. Each step is **idempotent**: re-running the same step on a node already in the target state is a no-op. The full implementation is in `globular-installer/pkg/installer/specplan_builder.go`.

| Step type | Purpose | Key params |
|---|---|---|
| `ensure_user_group` | Create the `globular` system user/group if missing. | `user`, `group`, `home`, `shell`, `system` |
| `ensure_dirs` | Create one or more directories with explicit owner/group/mode. | `dirs: [{path, owner, group, mode}]` |
| `install_package_payload` | Extract `bin/`, `config/`, `specs/`, `systemd/` from the package tarball into the live tree. **Required for service/infrastructure**. | `install_bins`, `install_config`, `install_spec`, `install_systemd` (booleans) |
| `install_files` | Write arbitrary files (configs, env files, wrapper scripts) with explicit content. | `files: [{path, owner, group, mode, content, atomic, skip_if_exists}]` |
| `install_services` | Drop systemd unit files (inline content or copied from the package). | `units: [{name, owner, group, mode, content, atomic}]` |
| `enable_services` | `systemctl enable <unit>`. | `services: [<unit>]` |
| `start_services` | `systemctl start <unit>`. Re-runs handle restart-on-files. | `services`, `restart_on_files`, `binaries` (for hash check) |
| `health_checks` | Wait for the unit to be active and (optionally) for the port to accept TCP. | `services`, `timeout`, `interval` |
| `run_script` | Run a script from the package's `scripts/` dir. Used for stateful initialization (TLS cert wiring, bucket provisioning, controlled first-start). | `script`, `timeout`, `required` |
| `install_local_debs` | `dpkg -i debs/*.deb`. Offline OS package install — `.deb`s are bundled at build time. | `debs_subdir` (default: `debs`) |
| `install_os_packages` | `apt-get install <pkg>...`. Used when the bundled-deb path doesn't work (keepalived needs online install). | `packages: [<apt-name>]` |
| `ensure_service_config` | Write the per-service config file into `{{.StateDir}}/services/<name>/<exec>/<version>/config.json` — the file every Globular service reads at startup. | `service_name`, `exec`, `address_host`, `domain`, `owner`, `group`, `mode`, `rewrite_if_out_of_range` |
| `ensure_hosts_block` / `remove_hosts_block` | Manage Globular's `/etc/hosts` section (cluster domain, VIP, peers). | `hosts_path`, `cluster_domain`, `node_name`, `advertise_ip`, `controller_ip`, `gateway_ip` |
| `normalize_scylla_config` | ScyllaDB-only — rewrite `scylla.yaml` addresses without disturbing Raft identity. | `config_path`, `listen_address`, `rpc_address`, ... |
| `fetch_file` | Download a file from a URL (used rarely; prefer bundling). | URL + destination |
| `install_binaries`, `install_packages`, `stage_package` | Lower-level / internal step types used by the bootstrap installer; rarely seen in app specs. | varies |
| `noop` | Placeholder. | `name` |

### 5.3 Template variables available in spec content

Spec `content:` blocks (in `install_files`, `install_services`) and dir paths support Go template interpolation. The variables come from `installer.Context`:

| Variable | Typical value | Meaning |
|---|---|---|
| `{{.Prefix}}` | `/usr/local/share/globular` | The install prefix where binaries go. |
| `{{.StateDir}}` | `/var/lib/globular` | The state root (PKI, etcd data, service state). |
| `{{.NodeIP}}` | e.g. `10.0.0.63` | The STABLE node IP (NOT the keepalived VIP — `StableIP(clusterVIP)` is used to resolve). |
| `{{.MinioDataDir}}` | e.g. `/mnt/globular-minio` | The MinIO data dir if configured. |
| `{{.Domain}}` | e.g. `globular.io` | The external domain. |

**HARD RULE**: never substitute `127.0.0.1` or `localhost` for `{{.NodeIP}}`. Always use the resolved stable IP. Listen bind addresses MAY use `0.0.0.0`.

### 5.4 Spec validation rules

`pkgpack.ValidateSpec`, `ValidateVersionBuildSemantics`, and package guards enforce (and the build will fail on any violation):

- `version: 1` exactly.
- `metadata.name` resolvable (from `metadata.name`, `service.name`, or filename).
- `metadata.kind` in {`service`, `infrastructure`, `command`, `application`} if set.
- `metadata.install_mode` in {`repository`, `day0_join`} if set.
- `metadata.priority` >= 0.
- `steps` non-empty; every step has a unique `id` and non-empty `type`.
- For `kind: service` — exactly one `install_package_payload` step exists.
- For `kind: infrastructure` — same, UNLESS `metadata.entrypoint: noop` (the OS-managed exception).
- `version` is a valid exact tag and `build_number` is a non-negative plain integer.
- `version` must not embed build tokens (`+bNN`, `-bNN`, `.bNN`).

`validate-package-metadata.sh` additionally enforces cross-source consistency:

- `specs/<name>_*.yaml`'s `metadata.kind` MUST equal `metadata/<name>/package.json`'s `type`.
- Both MUST equal the entry in `registry.yaml` for that package.
- Both MUST equal `component_catalog.go`'s classification (the runtime authority).

There is **one** source of truth for kind: the spec. Everything else is downstream.

---

## 6. `awareness.yaml` — invariants and failure modes

Every package SHOULD ship an `awareness.yaml` describing its invariants, ownership, dependencies, and known failure modes. This is consumed by the awareness pipeline (`docs/awareness/`) and shipped in the signed awareness bundle. Schema:

```yaml
apiVersion: awareness.globular.io/v1
kind: AwarenessContract
service: <name>
package: <name>
package_kind: service|infrastructure|command

summary: >
  One-paragraph description of what the package does, what runs, who it depends
  on, and the load-bearing invariants.

owns:                              # exclusive ownership — others must not write here
  etcd_keys: []
  systemd_units: []
  filesystem_paths: []
  event_types: []

reads:                             # keys/paths the service reads but doesn't own
  etcd_keys: []

writes:                            # keys it modifies (subset of owns or shared)
  etcd_keys: []

depends_on:                        # service-level dependencies with phase + reason
  - service: <other>
    phase: bootstrap | day0 | day1
    required: true|false
    reason: "<why this dependency exists>"

emits: []                          # event types this service publishes
subscribes: []                     # event types it consumes

invariants: []                     # invariant IDs from docs/awareness/invariants.yaml that THIS service must uphold

forbidden_fixes: []                # known anti-patterns. AI agents MUST NOT propose these.

known_failure_modes:
  - id: <failure-id>
    description: "..."
    diagnosis: "..."
    remedy: "..."

safe_degraded_modes: []            # what still works when this service is down

remediation_workflows: []          # named workflow IDs that recover this service

required_tests: []                 # Go tests that pin the invariants in CI
required_permissions: []           # gRPC/RBAC permissions needed at runtime

admission:                         # how strict the awareness gate is for this package
  strict: true|false
  allow_unknown_dependencies: false
  allow_privileged_state_writes: true|false
```

The awareness file is OPTIONAL today but is the path forward — packages without an awareness file get default-strict treatment and are flagged by `cluster_doctor`.

---

## 7. `registry.yaml` — the canonical package list

`packages/registry.yaml` is the single source of truth for every package the platform recognises. **Build scripts, controller logic, node-agent classification, and CI pipelines MUST read from it** instead of hardcoding their own lists. Schema per entry:

```yaml
- name: <name>
  kind: service|infrastructure|command
  binary: <exec-name>            # what bin/<exec> is named
  go_target: <go-build-path>     # relative to golang/; empty for third-party
  systemd_unit: <unit>           # default: globular-<name>.service
  version_source: platform|self  # platform = stamped from release tag; self = binary reports its own version
  publisher_id: core@globular.io
  control_plane_critical: true|false  # must come up before workloads
  day0_required: true|false          # part of the initial bootstrap
  day1_join_required: true|false     # must be on a node before it joins
  skip_runtime_check: true|false     # true for commands (no systemd unit)
  bootstrap_tier: foundation|core_control|supporting|workload
  profiles: [core, ...]
  provides: [<grpc-service-id>]
  requires: [<package-name>]         # runtime API peers
  hard_deps: [<package-name>]        # install/activation blockers
```

Updating the registry is part of the contract when adding a new package. The canonical command (planned):
```
python3 scripts/validate-registry.py     # not yet implemented
python3 scripts/gen-pkg-map.py           # regenerates pkg-map.json from registry
```

---

## 8. Versions, build numbers, and build IDs — three things, never confused

Globular tracks three distinct identifiers, and confusing them is one of the most common AI-agent mistakes:

| Identifier | Type | Allocated by | Used for |
|---|---|---|---|
| **`version`** | exact tag (`1.2.3`, `RELEASE.2025-...`) | `globular deploy --bump {patch,minor,major}` for Globular-built packages, or upstream tag for wrapped artifacts | Human-readable release label. Goes into the manifest and artifact filename. |
| **`build_number`** | int64 monotonic | Stamped at build time by `BuildOptions.BuildNumber` | **Display only.** NEVER used for convergence. Drift here is harmless. |
| **`build_id`** | UUIDv7 string | Repository service, post-upload | **The sole identity for convergence.** Node-agent verifies the installed `build_id` matches the desired `build_id`. |

Hard rules:

1. Never hardcode any of the three in source. `Version=""` in code; injected via `ldflags` at build:
   ```
   go build -ldflags "-X main.Version=1.2.3 -X main.BuildNumber=42"
   ```
2. The platform release version (the value in `release-index.json` after `globular repo sync --tag vX.Y.Z`) is **not** the package version. Only packages that changed get a new version. Platform release = BOM of (package, version) pairs.
3. `build_id` is empty in committed `package.json` files. It only exists in the manifest stored in MinIO + ScyllaDB AFTER the repository accepts the upload.
4. `--bump` reservations hold for 5 minutes. A failed publish does not release the reservation early. Retry with the same `--bump` value to re-allocate the same version.
5. Use one and only one build-number format: integer `build_number`. Never encode build numbers in `version`.

The convergence flow:

```
deploy --bump patch
   ↓
repository.AllocateUpload(name, bump=patch) → version=1.2.4, reservation=5min
   ↓
build .tgz with version=1.2.4
   ↓
repository.Upload(.tgz) → assigns build_id=01HXY...
   ↓
release reconciler: DesiredService{name}.build_id = 01HXY...
   ↓
per-node workflow: package_install
   ↓
node-agent downloads, verifies sha256 + build_id, installs, restarts unit
   ↓
node-agent writes /globular/nodes/<id>/packages/<kind>/<name> = {version, build_id, installed_at}
   ↓
runtime check: /proc/<pid>/exe sha256 == manifest.entrypoint_checksum
```

If any equality breaks, the layer that owns it owns the fix.

---

## 9. Install lifecycle — what happens on a node

The node-agent runs each package's spec as an ordered, idempotent plan. The canonical order for a service package is (using the cluster-controller spec as the template):

```
1. ensure_user_group        # create globular system user/group
2. ensure_dirs              # create prefix, state dirs, PKI dirs (correct owner+mode)
3. install_package_payload  # extract bin/<exec> from the tarball into {{.Prefix}}/bin/
4. ensure_service_config    # write the per-service config.json under state dir
5. install_services         # drop the systemd unit file
6. enable_services          # systemctl enable <unit>
7. start_services           # systemctl start <unit>
8. health_checks            # wait for active + port (if declared)
```

For an infrastructure package with a stateful first-start (ScyllaDB), additional steps slot in:

```
1. ensure_dirs
2. install_package_payload (install_bins: false — no Globular binary; uses upstream)
3. run_script prevent-autostart.sh   # block dpkg from auto-starting
4. install_local_debs                # dpkg -i debs/*.deb (offline)
5. run_script allow-autostart.sh     # remove the policy-rc.d block
6. run_script post-install.sh        # TLS wiring + controlled first-start of Raft
```

For an infrastructure package that wraps an upstream binary (MinIO), pre-start and post-install hooks bracket the systemd actions:

```
1. ensure_user_group
2. ensure_dirs                       # includes MinIO data dir
3. install_package_payload           # extracts the minio binary
4. install_files                     # writes minio.env, credentials
5. install_services                  # drops globular-minio.service
6. enable_services
7. run_script pre-start.sh           # TLS cert symlinks BEFORE start (HTTPS vs HTTP)
8. start_services
9. health_checks
10. run_script post-install.sh       # bucket provisioning AFTER start
```

For a command package (CLI tool) — no systemd, no service:

```
1. ensure_user_group
2. ensure_dirs                       # {{.Prefix}}/bin/
3. install_package_payload           # extracts the binary
4. (optional) install_files          # /usr/local/bin/<name> wrapper for PATH stability
```

### 9.1 Script conventions

Scripts under `metadata/<name>/scripts/` are bundled into the package's `scripts/` directory and run by `run_script` steps. Convention:

| Filename | Purpose | When called |
|---|---|---|
| `pre-start.sh` | Last-mile preparation BEFORE the systemd unit starts (cert symlinks, env files, contract JSON). | Between `install_services` and `start_services`. |
| `post-install.sh` | After the unit is up — bucket provisioning, schema migration, controlled first-start sequences. | After `health_checks` (Day-0) or before `start_services` re-run (Day-1). |
| `prevent-autostart.sh` / `allow-autostart.sh` | Bracket dpkg installs for daemons whose first-start has a one-way side effect (ScyllaDB Raft bootstrap). | Around `install_local_debs`. |
| `install-gpg-key.sh` | Trust upstream GPG keys before apt operations. | Before `install_os_packages`. |

Every script MUST:

- Start with `set -euo pipefail`.
- Read its inputs from environment variables that the node-agent passes (`STATE_DIR`, `NODE_IP`, `GLOBULAR_DOMAIN`, `PREFIX`).
- Be idempotent. Running twice in a row MUST converge to the same state. The MinIO `pre-start.sh` is the reference for this pattern (`skip_if_exists`, symlinks instead of copies).
- Have a single clear `[<pkg>/<script>]` prefix on every log line.
- Print actionable diagnostics on failure; no silent failures.
- NEVER use `pkill -f` to kill the running process — `-f` matches the parent shell argv and self-SIGKILLs. Always `pkill -x <binary>`.

### 9.2 Day-0 vs Day-1 distinction

| Context | Caller | Behavior |
|---|---|---|
| **Day-0** | `globular bootstrap` (installer binary, runs once per cluster) | Reads `release-index.json` directly. Installs the founding-quorum packages in tier order: foundation → core_control → supporting → workload. |
| **Day-1** | Node-agent on a joining node (curl gateway script) | Joins via the active BOM. Each package install is a workflow run. |
| **Day-2** | Cluster controller release reconciler | Watches the repository for new versions. Dispatches per-node workflows when a `DesiredService` changes. |

Spec scripts run in all three contexts; they MUST detect which (e.g. `systemctl is-active --quiet`) and skip work that's already done. See `metadata/minio/scripts/post-install.sh` for the canonical idempotency pattern.

---

## 10. Building a package

The build entry point is `BuildPackages` in `golang/globularcli/pkgpack/builder.go`, invoked by `globular pkg build`. Minimum required inputs:

- `--spec` (or `--spec-dir`) — one or more spec YAML files.
- `--version` — exact version tag; normalized via `versionutil.NormalizeExact` (SemVer for Globular-built services, upstream-native tags allowed).
- `--out` — output directory.
- One of: `--root`, `--bin-dir` + `--config-dir`, or `--installer-root` + `--assets` — where the built binary and config templates are found.

What the builder does:

1. Validates the spec (`pkgpack.ValidateSpec`).
2. Scans the spec for binary references, config dirs, systemd files, scripts (`ScanSpec`).
3. Resolves `bundle_debs` — either uses `--debs-dir` (pre-downloaded) or runs `apt-get download` into a temp dir.
4. Creates a staging dir; copies the binary, extra binaries, config files, spec, systemd units, scripts, debs, optional data dir.
5. Computes SHA256 of `bin/<exec>` → `entrypoint_checksum`.
6. Writes `package.json` (the AUTHORITATIVE manifest with all build-time fields stamped).
7. Tarballs the staging dir into `<name>_<version>_<goos>_<goarch>.tgz`.
8. Asserts guards: binary present, spec present, spec contains `install_package_payload`. Aborts on any failure.
9. Verifies the tarball (`VerifyTGZ`).

The result is one immutable `.tgz` ready for upload to the repository.

`VerifyTGZ` gates that matter for operators and AI agents:

- `package.json` must exist and carry `name`, `version`, `platform`, `publisher`.
- `version` and `build_number` semantics must be valid (`build_number` integer, no embedded build token in version).
- `entrypoint` must exist inside archive.
- if `entrypoint_checksum` is set, it must be `sha256:<64hex>` and must match the actual entrypoint bytes.
- `scripts/*.sh` entries must be executable.
- `systemd/*.service` files are validated for duplicate singleton `[Service]` directives.

---

## 11. Publishing a package

After build, publish via `globular pkg publish` or `globular deploy <service>` (deploy does build + publish together). What happens:

1. **AllocateUpload RPC** — repository reserves the next version per `--bump` for 5 minutes.
2. **Upload RPC** — streams the `.tgz` to the repository. The repository:
   - Verifies the SHA256.
   - Allocates a `build_id` (UUIDv7).
   - Stores the bytes in the local `globular` MinIO bucket under a content-addressed path.
   - Writes the metadata index row in ScyllaDB.
   - Writes a local installable receipt under `/var/lib/globular/repository/installable/`.
3. **Activate (optional)** — the repository can promote the artifact to the active release set.

CLI-side preflight before upload (`globular pkg publish`) blocks package identity mistakes before bytes hit the network:

- invalid `version` format
- negative `build_number`
- embedded build token in `version` (for example `1.2.3+b325`)

Failure modes:

- `BLOB_VERIFIED` state stuck — the artifact exists in MinIO but the metadata row didn't promote. Fix: `cqlsh 10.0.0.63 9042 UPDATE manifests SET artifact_state='AVAILABLE' WHERE ...` (see `project_blob_verified_bug.md`).
- Reservation timeout — wait 5 minutes, re-run with the same `--bump`.
- Missing sa auth — `globular auth login --user sa` first; token cached at `~/.config/globular/token`. If the cache dir is owned by root from a prior `sudo`, chown it.

---

## 12. Validating a package

Two layers of validation:

### 12.1 Source-tree validation (pre-build)

```
cd packages/
./scripts/validate-package-metadata.sh
```

Cross-checks:

- Every `specs/*.yaml` has a matching `metadata/<name>/package.json`.
- `spec.metadata.kind` == `package.json.type` == catalog classification.
- Every package.json has a `type` field, and it's a valid kind.

Run this before any commit that touches `packages/`.

### 12.2 Built-package validation (post-build, pre-publish)

The build itself calls `assertPackageGuards` and `VerifyTGZ`:

- `bin/<exec>` exists.
- `specs/<name>_*.yaml` exists.
- Spec contains `install_package_payload`.
- The tarball is well-formed.

For deeper validation:

```
globular pkg inspect <path-to.tgz>           # show manifest + entry list
globular pkg verify <path-to.tgz>            # full structural + checksum verification
sha256sum <path>.tgz                          # compare against repository-recorded checksum
```

### 12.3 Runtime validation (post-install)

The node-agent continuously verifies, per package per node:

| Check | What it confirms |
|---|---|
| systemd unit `active` | Process is running. |
| `health_check_port` TCP probe | Process is accepting traffic. |
| `entrypoint_checksum` vs `/proc/<pid>/exe` SHA256 | Running process matches installed binary. Detects hot-deploy drift and `(deleted)` inode hazards. |
| `installed.build_id` == `desired.build_id` | Convergence achieved. |

Drift on any check raises a finding in `cluster_doctor`. The `CORRUPTED` state is reserved for cases where the installed bytes diverge from the manifest checksum.

---

## 13. Authoring a new package — the checklist

When adding a new package:

1. **Decide the kind**.
   - Does it expose a gRPC API to the mesh? → `service`.
   - Is it a daemon that runs but doesn't talk on the service mesh? → `infrastructure`.
   - Is it a CLI tool with no daemon? → `command`.

2. **Add the entry to `packages/registry.yaml`** with the correct kind, profiles, hard_deps, bootstrap_tier, etc.

3. **Add the entry to `component_catalog.go`** (or the equivalent classification in services repo). Must agree with registry.

4. **Create `packages/metadata/<name>/`**:
   - `package.json` — reference manifest (fill in name, type, default version `0.0.1`, publisher, entrypoint).
   - `awareness.yaml` — invariants, ownership, failure modes (encouraged; can be added later but flagged by doctor).
   - `systemd/globular-<name>.service` if the unit content is non-trivial.
   - `config/<name>/` if there are default config templates.
   - `scripts/` if pre-start / post-install hooks are needed.

5. **Create `packages/specs/<name>_{service,cmd}.yaml`**:
   - `metadata.kind` matching registry.
   - All required steps: `ensure_user_group` → `ensure_dirs` → `install_package_payload` → `ensure_service_config` (for services) → `install_services` → `enable_services` → `start_services` → `health_checks`.
   - For OS-managed daemons: set `entrypoint: noop` and `install_bins: false`; use `install_local_debs` / `install_os_packages`.

6. **Code: `Version = ""` in source**. Injected via ldflags. NEVER hardcode.

7. **Add `start_services` step for the systemd unit** — every package spec MUST end with starting its service. No silent "Day-0 will handle it."

8. **Run validation**:
   ```
   cd packages && ./scripts/validate-package-metadata.sh
   cd golang && go build ./globularcli/...   # exercise pkgpack
   ```

9. **Build locally**:
   ```
   globular pkg build --spec packages/specs/<name>_service.yaml --version 0.0.1 --out /tmp/pkgs --root <build-root>
   globular pkg inspect /tmp/pkgs/<name>_0.0.1_linux_amd64.tgz
   ```

10. **Publish via the deploy pipeline**:
    ```
    globular --controller <leader>.globular.internal deploy <name> --bump patch --comment "initial release"
    ```

11. **Verify convergence** across all profile-matching nodes:
    ```
    globular cluster nodes list
    globular pkg status <name>
    ```

## 13.1 Modifying an existing package and republishing (operator-safe procedure)

Use this exact sequence when changing an existing package. This is the default AI-agent flow.

1. Change package definition files only:
   - `packages/specs/<name>_*.yaml`
   - `packages/metadata/<name>/package.json`
   - `packages/metadata/<name>/systemd/*`
   - `packages/metadata/<name>/config/*`
   - `packages/metadata/<name>/scripts/*`
2. Keep kind stable unless this is an intentional migration; if kind changes, update all authorities in the same change:
   - `packages/specs/... metadata.kind`
   - `packages/metadata/.../package.json type`
   - `packages/registry.yaml`
   - runtime catalog mapping (`component_catalog.go`)
3. Run source validation:
   - `cd packages && ./scripts/validate-package-metadata.sh`
4. Build package:
   - `globular pkg build --spec packages/specs/<name>_service.yaml --version <version> --build-number <N> --out <out-dir> --root <payload-root>`
5. Validate built artifact:
   - `globular pkg validate --file <out-dir>/<name>_<version>_<platform>.tgz`
6. Publish:
   - `globular pkg publish --file <artifact.tgz> --repository <repo-addr>`
   - preferred release path for platform packages: `globular deploy <name> --bump patch|minor|major`
7. Move desired state and reconcile:
   - `globular services desired set <name> <version>`
   - `globular services repair`
8. Verify all 4 layers:
   - repository manifest exists and is `PUBLISHED`
   - desired state points to the target build identity
   - installed-state updated on all target nodes
   - runtime hash and unit health are verified (no doctor package/hash drift)

## 13.2 Debian `.deb` strategy (when and how)

Use `.deb` packaging only for daemons that are OS-managed upstream (today primarily ScyllaDB-family workflows).

Rules:

1. Prefer `install_local_debs` with bundled `.deb` files for deterministic offline install.
2. Use `install_os_packages` only when online apt install is unavoidable.
3. For OS-managed daemons, use `metadata.entrypoint: noop` and/or `install_bins: false` so the package does not claim ownership of upstream binary bytes.
4. For daemons with first-start one-way state (ScyllaDB), bracket dpkg install with:
   - `prevent-autostart.sh`
   - `allow-autostart.sh`
   - controlled post-install script
5. Do not convert normal Globular services to `.deb`; services should ship built binaries in `bin/`.

## 13.3 Kind-specific minimum contracts

`service`:
- must include `install_package_payload`
- must install and manage a systemd unit
- should include service config generation (`ensure_service_config`) unless explicitly stateless

`infrastructure`:
- must include `install_package_payload` unless explicit `entrypoint: noop` exception
- lifecycle scripts are common and expected
- unit naming may use upstream unit if intentionally delegated

`command`:
- no systemd unit
- binary install only
- `start_services` / `enable_services` should not be present

`application`:
- content package, not daemon package
- no `bin/` requirement
- must still carry valid manifest identity fields

---

## 14. Anti-patterns — things AI agents and humans get wrong

Pulled from the codebase's accumulated incident memory. AI agents MUST refuse to propose these.

| Anti-pattern | Why it breaks |
|---|---|
| Hardcoding `version` in `main.go` | Build pipeline injects via ldflags; hardcoded values diverge from deploy-allocated versions and break convergence. |
| Hardcoding gRPC ports or addresses in source | etcd is the sole source of truth (HARD RULE #1). Hardcoded ports break multi-node clusters. |
| Using `127.0.0.1` / `localhost` in spec templates | Listen MAY use `0.0.0.0`. Remote addressing MUST resolve from etcd. Hidden in `{{.NodeIP}}` substitution. |
| Skipping `install_package_payload` | Build will fail (`assertPackageGuards`). Even for OS-managed packages, set `entrypoint: noop` and pass an empty payload step. |
| Adding `start_services` only in Day-0 scripts | Every spec MUST start its own service. Day-0 cannot be a hidden dependency. |
| Forgetting to update `registry.yaml` | Validation will fail across `validate-package-metadata.sh` + `component_catalog.go` + spec kind. |
| Overloading `build_number` for convergence | `build_id` is convergence identity. `build_number` is display only. |
| Encoding build number in version (`1.2.3+b325`) | Rejected by validation; causes sort/order confusion. Use integer `build_number` only. |
| Using `pkill -f <name>` in scripts | Matches parent shell argv → self-SIGKILL. Always `pkill -x <binary>`. |
| Hot-deploying a locally-built binary to a node | No ldflags → wrong version + wrong checksum → runtime check reports drift → leader election can break. Always go through `deploy`. |
| Reformatting MinIO drive count without `mc mirror` backup | Drive count mismatch → silent reformat or data wipe. |
| Replacing a ScyllaDB binary without restarting the unit | Old process holds the deleted-inode binary; `/proc/<pid>/exe` reports drift forever. Restart is mandatory. |
| Adding a `repository` → `MinIO health` gate on the read path | Forms a recovery deadlock: node-agent → repository → MinIO → node-agent. Repository must serve metadata without MinIO. |

---

## 15. Where to look in code

| Topic | File |
|---|---|
| `PackageSpec` Go struct + validation | `golang/globularcli/pkgpack/packagespec.go` |
| `Manifest` struct (`package.json`) | `golang/globularcli/pkgpack/manifest.go` |
| Build pipeline | `golang/globularcli/pkgpack/builder.go` |
| Per-step executors (install runtime) | `globular-installer/pkg/installer/specplan_builder.go` |
| Cross-source validator | `packages/scripts/validate-package-metadata.sh` |
| Canonical package list | `packages/registry.yaml` |
| Per-package metadata | `packages/metadata/<name>/` |
| Per-package install recipe | `packages/specs/<name>_*.yaml` |
| Runtime catalog (Go authority on kind) | `golang/cluster_controller/.../component_catalog.go` |
| Awareness contracts | `packages/metadata/<name>/awareness.yaml`, `docs/awareness/` |

---

## 16. Quick reference — the four files every service package owns

For a service package named `<name>`, the four files you'll touch most are:

```
packages/registry.yaml                                # one entry
packages/specs/<name>_service.yaml                    # install recipe
packages/metadata/<name>/package.json                 # reference manifest
packages/metadata/<name>/awareness.yaml               # invariants + failure modes
```

For infrastructure with stateful daemons, add `metadata/<name>/scripts/` and (for offline OS installs) `metadata/<name>/debs/`.

For commands, drop `awareness.yaml` is optional but encouraged for CLI tools that have known operational gotchas (e.g. `globular-cli`).

---

This document is part of the day-0 operational knowledge seed. Its sibling YAML entries under `stages/` and `service-roles/` carry the same facts in a form AI Memory ingests at day-1. If you change package semantics in code, update this file in the same PR.
