# Versioning

Versioning in Globular is more consequential than in most systems. A version is not just a label — it is the token that the convergence model uses to decide whether to act. Getting it wrong means either silent drift (the cluster thinks nothing changed) or failed deployments (a build that can't be found). This page explains the model in full.

---

## The two identities of a build

Every artifact has two separate identifiers. They serve completely different purposes and must not be confused:

| Identity | What it is | Who assigns it | Used for |
|----------|-----------|----------------|----------|
| `version` | Semantic version string (`0.1.4`) | Developer via `--bump` | Human communication, desired-state declarations, upgrade planning |
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

## Mono-version track

All Globular Go services share a single version track. When any service gets a version bump, all services in the manifest are bumped together.

Why? Because services evolve as a unit. Dependency compatibility between authentication v0.1.0 and rbac v0.0.9 is not tested, not tracked, and not supported. The mono-version model makes this concrete: if the cluster is at v0.1.4, every Globular service is v0.1.4.

This simplifies the operator mental model: "What version is my cluster at?" has one answer.

**The exception**: Infrastructure packages keep upstream versions. etcd 3.5.15 and MinIO 7.0.11 coexist with Globular services at v0.1.4. Infrastructure versions are set in the package manifest separately.

---

## How versions are allocated

Versions are allocated by the repository service via `AllocateUpload`. You do not pick a version number directly — you specify the bump type and the repository assigns the next one.

```bash
# Bump patch version (bug fix)
globular deploy my-service --bump patch

# Bump minor version (new feature)
globular pkg publish --file pkg.tgz --repository globular.internal --bump minor

# Bump major version (breaking change)
globular deploy my-service --bump major
```

The repository enforces:

- **Monotonicity**: The new version must be >= the latest PUBLISHED version for that service. You cannot publish v0.1.3 after v0.1.4 has been published.
- **Uniqueness**: Each `version + platform` combination gets exactly one `build_id`. You cannot upload a different binary with the same version.
- **Reservation**: A 5-minute TTL prevents concurrent upload collisions. If you allocate a version slot and do not complete the upload, the slot expires.
- **No direct version set**: Specifying `--version 0.1.0` explicitly is accepted but prints a deprecation warning. Use `--bump` in all pipelines.

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
