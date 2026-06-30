# Versioning

Versioning in Globular is more consequential than in most systems. A version is not just a label — it is the token that the convergence model uses to decide whether to act. Getting it wrong means either silent drift (the cluster thinks nothing changed) or failed deployments (a build that can't be found). This page explains the model in full.

---

## The two identities of a build

Every artifact has two separate identifiers. They serve completely different purposes and must not be confused:

| Identity | What it is | Who assigns it | Used for |
|----------|-----------|----------------|----------|
| `version` | Semantic version string (`0.1.4`) | Package version policy (`version_source: platform` or `version_source: self`) | Human communication, desired-state declarations, upgrade planning |
| `build_id` | UUIDv7, e.g. `019235ab-...` | Repository on upload | Convergence decisions, exact-replay recovery, artifact provenance |

**The convergence model compares `build_id`, not version strings.** When the controller checks whether a node has the right artifact installed, it compares the `build_id` in the desired-state record against the `build_id` reported by the node agent. Two artifacts with the same version but different `build_id` values are different artifacts as far as the system is concerned.

This matters when you publish a hotfix. Even if you reuse a version number (which the repository will reject), a new upload always gets a new `build_id`. The old installed artifact and the new one are unambiguously distinct.

---

## Semantic versioning

All Globular Go services follow [Semantic Versioning 2.0.0](https://semver.org/):

- **PATCH** (0.1.x): Bug fixes. No API changes. Safe to deploy without operator review of the changelog.
- **MINOR** (0.x.0): New features. Backward compatible. gRPC clients built against an older minor version will still work.
- **MAJOR** (x.0.0): Breaking changes. Proto contract changes, removed RPCs, changed behavior. Requires coordinated rollout.

Infrastructure packages (etcd, ScyllaDB, MinIO, Prometheus, Envoy) keep their upstream version numbers. The Globular services that wrap or depend on them have their own monotonic version track.

---

## Platform release vs package version

A **platform release** (e.g. Globular v1.0.84) is a **bill of materials** (BOM) — a composition lockfile that lists exact package artifacts. It is NOT a monolithic version applied to every package.

A **package version** changes only when the package's content/contract changes. If `gateway` was last modified in v1.0.82, it stays at version 1.0.82 in every subsequent platform release until its content actually changes.

```
Platform Release v1.0.84:
  repository   v1.0.84  (CHANGED in this release)
  gateway      v1.0.82  (unchanged since v1.0.82)
  dns          v1.0.80  (unchanged since v1.0.80)
  envoy        1.35.3   (upstream version, unchanged)
```

The `release-index.json` v2 schema records `origin_release` and `changed_in_release` for each package, making the composition explicit.

**Change detection** uses `package_contract_digest` — a normalized hash of the binary, manifest, specs, systemd units, profiles, and dependencies. This is independent of tar/gzip archive metadata, so identical content always produces the same digest.

**Why not mono-version?** Mono-versioning (stamping every package with the platform version) destroys version meaning. A version should represent a content/contract change. If `gateway` says v1.0.84 but nothing in gateway changed since v1.0.82, the version is lying. The BOM model preserves truthful version semantics while still grouping packages into coherent platform releases.

**Infrastructure packages** keep their upstream versions as before. etcd 3.5.15, MinIO RELEASE.2025-09-07, etc.

---

## Version authority chain

Package version management has one authority chain and several projections. They must not be confused.

| Surface | Role | Authority level |
|---------|------|-----------------|
| `packages/registry.yaml` | Declares that a package exists and whether its version source is `platform` or `self` | package identity authority |
| `packages/metadata/<name>/specs/*.yaml` | Canonical package recipe | canonical package recipe authority |
| `golang/build/package-versions.txt` | Release-time projection of package versions selected for one build flow | generated projection only |
| `release-index.json` | Exact BOM for one platform release, including package version, `build_number`, `build_id`, and provenance | platform release authority |
| `zz_version_generated.go` | Runtime self-report embedded into a built service binary | runtime evidence mirror |
| `services/dist/` | Disposable release output containing the assembled BOM and artifacts | output only, never source authority |

Two consequences matter:

1. `services/dist` is not supposed to "follow" `packages/registry.yaml` directly. `services/dist` is only the assembled release bundle. The version truth inside it is the `release-index.json` BOM.
2. `packages/registry.yaml` is package identity authority, not a flat table of concrete versions. Concrete package versions come from the canonical recipe plus the `version_source` policy.

---

## `version_source` decides where a package version comes from

Every package entry in `packages/registry.yaml` declares a `version_source`.

### `version_source: platform`

Use this for Globular-managed services whose package version should follow the BOM/release flow.

- The package is still defined by `packages/metadata/<name>/specs/*.yaml`.
- The literal version written in the recipe spec is not the final release authority.
- During release assembly, BOM change detection decides whether the package is changed or unchanged.
- Unchanged packages keep their previous package version from the prior BOM.
- Changed packages get the release-selected package version for that build flow.
- The chosen package version is written into `golang/build/package-versions.txt`, stamped into `zz_version_generated.go`, and recorded in the new `release-index.json`.

This is why one platform release can contain mixed service versions without ambiguity.

### `version_source: self`

Use this for upstream or externally versioned packages that report their own version.

- The canonical version lives in `packages/metadata/<name>/specs/*.yaml`.
- Build/package steps must verify that the staged binary reports that same version.
- Release assembly carries that package version into the BOM unchanged.
- The platform release does not rename that package to the platform tag.

Examples: `etcd`, `minio`, `envoy`, `prometheus`, `mc`, `restic`.

---

## How versions are allocated

Version control has two separate steps:

1. The package version is chosen by the package's version-source policy.
2. The repository assigns the concrete published artifact identity.

For `version_source: platform` packages, the release flow selects the package version from BOM/change detection.

For `version_source: self` packages, the canonical metadata recipe declares the version and the staged binary must prove it.

After that version decision, the repository service assigns `build_id` and `build_number` to the uploaded artifact. The repository is the authority for artifact identity and publication state, not for inventing package identity from `services/dist` or from whatever file happens to be on disk.

Repository allocation still matters for publish workflows:

```bash
# Local deploy: rebuild the latest published release version and advance build_number only
globular deploy my-service

# Release publish: allocate a new semver through the repository
globular pkg publish --file pkg.tgz --repository globular.internal --bump minor

# Explicit local backport against an older published release version
globular deploy my-service --version 1.2.259 --channel candidate
```

The repository enforces:

- **Monotonicity**: The new version must be >= the latest PUBLISHED version for that service. You cannot publish v0.1.3 after v0.1.4 has been published.
- **Uniqueness**: Each `version + platform` combination gets exactly one `build_id`. You cannot upload a different binary with the same version.
- **Reservation**: A 5-minute TTL prevents concurrent upload collisions. If you allocate a version slot and do not complete the upload, the slot expires.
- **Local deploys do not allocate semver**: `globular deploy` reuses an existing published STABLE version and publishes a higher build_number on a non-STABLE channel.
- **Release bumps are deliberate**: Allocate a new semver only from the release/package publish workflow, not from a workstation deploy.

In practice:

- `packages/registry.yaml` says what kind of version source a package uses.
- canonical metadata says how that package is built and packaged.
- the release flow decides the package version to ship.
- the repository assigns the concrete published build identity.
- the BOM records the exact outcome.
- `scripts/build-release.sh` defaults to the next patch version from the latest git tag unless you pass an explicit version or `--bump minor|major`.
- `scripts/build-release.sh --full-regenerate` wipes and rebuilds `services/generated` release inputs before assembling `services/dist`.

---

## Versions in source code

**Never hardcode a version in Go source.** The version field is always `""` (empty string) at compile time. It is injected as an ldflags value at build time by the deploy pipeline:

```go
// In main.go — left empty at compile time
var Version = ""
```

```bash
# Build pipeline injects the version
go build -ldflags "-X main.Version=0.1.4" ./...
```

If you see a hardcoded version string in a service's source code, it is a bug.

---

## `zz_version_generated.go` is runtime evidence, not package authority

Every built Go service gets a generated `zz_version_generated.go` file before compilation.

That file exists so:

- the binary can report its own version at runtime
- the node-agent can compare installed/runtime evidence against desired state
- operators can inspect what version a process claims to be running

It is important, but it is not the root authority. The correct direction is:

`packages/registry.yaml` + canonical metadata + release/BOM decision → generated runtime version file

Never reverse that relationship. A stray built binary or an old generated version file must not teach the platform what version a package "really is".

---

## Version flow through the 4 truth layers

A version bump touches all four truth layers. Understanding this flow prevents confusion about why the cluster has or has not converged after a publish.

```
Developer runs: globular deploy my-service --bump patch
                                │
                                ▼
┌─────────────────────────────────────────────────────┐
│  Layer 1: Repository                                │
│    AllocateUpload(version=0.1.5) → build_id=abc123  │
│    Upload binary, verify checksum                   │
│    completePublish → state: PUBLISHED               │
└─────────────────────────────────────────────────────┘
                                │
                          reconciler detects desired
                          state exists, build_id drift
                                ▼
┌─────────────────────────────────────────────────────┐
│  Layer 2: Desired Release                           │
│    /globular/resources/DesiredService/my-service    │
│    { version: "0.1.5", build_id: "abc123" }         │
└─────────────────────────────────────────────────────┘
                                │
                          workflow dispatched per node
                                ▼
┌─────────────────────────────────────────────────────┐
│  Layer 3: Installed Observed                        │
│    node.deploy workflow:                            │
│    FETCH build_id=abc123 from MinIO                 │
│    INSTALL binary, record build_id in etcd          │
│    VERIFY checksum                                  │
│    → /globular/nodes/<id>/packages/.../my-service   │
│      { version: "0.1.5", build_id: "abc123" }       │
└─────────────────────────────────────────────────────┘
                                │
                          systemd starts new binary
                                ▼
┌─────────────────────────────────────────────────────┐
│  Layer 4: Runtime Health                            │
│    systemd unit: active (running)                   │
│    health check passes                              │
│    node agent reports: healthy                      │
└─────────────────────────────────────────────────────┘
```

Layer 2 is updated by the `globular deploy` command or directly by the operator. Layers 3 and 4 follow automatically through the workflow. You cannot skip layers. If Layer 1 is not PUBLISHED, Layer 2 cannot be set. If Layer 2 is not set, Layer 3 does not change.

---

## Can different service versions live on the same cluster?

Yes. That is not only allowed, it is the intended model.

- Different packages routinely have different versions in the same platform release.
- A node may temporarily run an older version of one package while a rollout is in progress.
- The desired-state model pins package identity per package, not "one giant version for the whole cluster."

What is not allowed is ambiguity about a given package on a given node:

- desired state must point to one concrete package version
- publish must resolve to one concrete `build_id`
- installed state must record what is actually there
- runtime evidence must report what is actually running

So the cluster can absolutely run `gateway=1.2.52`, `repository=1.2.52`, `dns=1.2.44`, `minio=RELEASE.2025-09-07T16-13-09Z`, and `etcd=3.5.14` at the same time. That is the normal case.

---

## Lifecycle for a new microservice

This applies both to services that live in this repository and future services that live outside it.

1. Register the package in `packages/registry.yaml`.
2. Create the canonical package recipe under `packages/metadata/<name>/specs/*.yaml`.
3. Choose the package's `version_source`.
4. Ensure the service binary reports its own runtime version.
5. Build/package from the canonical recipe, never from `services/dist`.
6. Publish so the repository assigns `build_id` and `build_number`.
7. Include the artifact in a `release-index.json` BOM when it belongs to a platform release.

Recommended policy:

- Use `version_source: platform` for Globular-managed services that should participate in the BOM-driven release cadence.
- Use `version_source: self` for third-party or externally released services where the upstream version is the package version.
- For out-of-repo services, keep the same contract: they may live elsewhere for source code, but package identity and recipe authority still enter through `packages/registry.yaml` and `packages/metadata`.

---

## Downgrade guard

The convergence model will never install a version with a lower version number than what is currently installed, even if the desired state requests it. This is unconditional and applies per-artifact per-node.

```
Installed: my-service v0.1.4 (build_id: abc123)
Desired:   my-service v0.1.3 (build_id: xyz789)
Result:    Downgrade guard fires. No action taken.
```

If you genuinely need to roll back (e.g. v0.1.4 introduced a critical bug), the only supported path is an explicit force flag:

```bash
globular deploy my-service --version 0.1.3 --force
```

Automatic rollbacks are forbidden by design. They hide problems and create unpredictable cluster state. Rolling back is an operator decision, not an autonomous one.

---

## build_id and exact-replay recovery

The `build_id` becomes critical during node full-reseed recovery. When a node is rebuilt from scratch, the recovery workflow consults the pre-captured inventory snapshot to decide what to install. If `--exact-replay` is set, every artifact must have a `build_id` in the snapshot, and the workflow fetches each artifact by its exact `build_id` from the repository.

This means:
- An artifact installed but never published through the tracked pipeline (no `build_id`) cannot be exactly replayed.
- A `build_id` that has been GC'd from the repository cannot be replayed, even if the version still exists as a label.

To keep your cluster recoverable:
- Always publish through the deploy pipeline, never manual uploads without a `build_id`.
- Ensure the repository GC policy retains builds at least as long as you might need to recover a node.
- Use `globular node snapshot create` before any high-risk operation to capture a point-in-time build manifest.

See [Node Full-Reseed Recovery](../operators/node-recovery.md#exact-replay-vs-resolution-fallback) for details.

---

## Current versions

| Track | Current | Meaning |
|-------|---------|---------|
| Globular services | **0.1.x** | Phase 2 complete. build_id identity model stable. Core invariants tested in production. |
| etcd | 3.5.15 | Upstream. |
| ScyllaDB | 5.4.x | Upstream. |
| MinIO | 7.0.x | Upstream. |

---

## v1.0.0 criteria

1.0.0 will be declared when all of the following are true:

1. All invariants (INV-1 through INV-10) are tested automatically on every commit
2. Zero anomalies on the 3-node reference cluster for 7 consecutive days
3. Containerized test cluster operational for CI simulation
4. Operator course updated to match current implementation
5. At least one external operator has followed the course successfully and confirmed it
6. GitHub release pipeline publishes versioned tarballs with SHA256 checksums

The current cluster is running v0.1.x and has been stable in production for several months, but criteria 1, 3, 4, and 5 are not yet met. See [Platform Status](../operators/platform-status.md) for current milestone tracking.
