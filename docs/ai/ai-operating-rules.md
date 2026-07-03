# Globular AI Operating Rules

You are working inside Globular, a distributed bare-metal cluster platform. Your job is not only to write code. Your job is to preserve the cluster truth model.

Globular is not a normal monorepo app. It is a package-based control plane with release streams, local repository truth, explicit desired state, installed state, runtime health, and workflows.

When unsure, do not guess. Walk the layers.

## 1. The core mental model

Always reason through the 4-layer model:

1. Artifact / Repository
   What packages exist?
   What versions, build numbers, checksums, provenance, upstream source, release-index entries exist?
   Source of truth: repository service, Scylla catalog, release-index.json, package manifests.

2. Desired State
   What does the cluster intend to run?
   Source of truth: etcd desired keys, controller target release, ServiceRelease / InfrastructureRelease records.

3. Installed / Observed State
   What is actually installed on each node?
   Source of truth: node-agent reports, installed-state keys, package receipts, systemd unit presence.

4. Runtime Health / Telemetry
   Is it running and healthy?
   Source of truth: health checks, metrics, doctor findings, workflow status, logs.

When debugging:
- Never jump straight to runtime.
- First ask: which layer disagrees with which other layer?
- Most bugs are layer mismatches.

Example:
Repository says package exists.
Desired says install package.
Installed says old package still present.
Runtime says service unhealthy.
This is not one bug. It is a convergence path to inspect.

## 2. Version identity rules

Never assume platform release version equals package version.

Globular has separate identities:

1. Platform release
   Example: Globular v1.0.85.
   This is a bill of materials / composition lockfile.

2. Package version
   Example: gateway 1.0.82, repository 1.0.85, minio RELEASE.2025...
   Package version changes only when the package content or contract changes.

3. Build number
   Sequential/package build number used for artifact resolution.

4. Build ID / artifact ID
   Unique artifact identity. Do not confuse this with build number.

5. Artifact digest
   The immutable byte identity. Checksum is the final truth for downloaded artifact bytes.

Rules:
- Do not stamp every package with the platform release version.
- Do not infer platform release from package filenames.
- release-index.json is the platform truth.
- A platform release may contain mixed package versions.
- Same package identity + same digest referenced by multiple platform releases is valid.
- Same package identity + different digest is a conflict and must be rejected or quarantined.

Preferred naming:
- artifact_uuid or artifact_id for UUID-like identifiers.
- build_sequence or build_number for numeric build counters.
- package_contract_digest for contract/content identity.
- artifact_sha256 for archive byte checksum.

Do not overload BUILD_ID environment variables. If an env var means numeric build number, name it BUILD_NUMBER. If it means artifact UUID, name it ARTIFACT_ID.

## 3. Release model

A Globular release is a BOM.

Current GitHub release assets may contain only changed packages plus release-index.json.
The full offline installer may contain all packages in the BOM.

Correct release behavior:
- Always generate release-index.json.
- Upload only changed package artifacts as current-release assets.
- Unchanged packages keep their original version/build/digest.
- Unchanged packages reference origin_release and original asset_url / asset_path.
- Offline bundle materializes the complete BOM by copying/downloading unchanged packages after checksum verification.

release-index.json is authoritative.
The GitHub asset list is not authoritative.

## 4. Package kind single source of truth

There must be one canonical package registry.

Do not classify package kind independently in multiple places.

Package kind must not be guessed from:
- filename
- install phase
- package path
- service name
- ad hoc hardcoded list

Create and use a canonical registry, for example:

PACKAGES.md
packages/registry.yaml
or package_registry.go generated from registry.yaml

Each package entry should define:
- name
- kind: SERVICE | INFRASTRUCTURE | APPLICATION | TOOL | RUNTIME
- publisher_id
- version_source: platform | upstream | manual | external
- desired_state_owner: ServiceRelease | InfrastructureRelease | none
- day0_required: true/false
- day1_join_required: true/false
- profiles
- provides
- requires
- hard_deps
- default package manifest path
- upstream source if applicable

All code must consume this registry, not duplicate package lists.

Known anti-pattern:
- package.json says one kind
- pkg-map.json says another
- node-agent has an infra list
- controller splits differently

That must not happen.

## 5. Encoding clarity: proto vs JSON vs handwritten structs

Some Globular types are proto-generated.
Some are handwritten Go structs serialized through JSON.
Do not assume.

Every handwritten serialized type should have a comment:

// +globular:encoding:json
// +globular:etcd-key:/globular/...
// +globular:owner:cluster_controller
// +globular:schema-version:v1

Every proto-backed type should have:

// +globular:encoding:protobuf
// +globular:proto:repository.ArtifactManifest

When modifying a type:
- Identify whether it is protobuf, JSON, or internal-only.
- Identify where it is stored.
- Identify who reads and writes it.
- If JSON in etcd, maintain backward compatibility.
- If protobuf RPC, regenerate proto code and update clients.

Never add a field without knowing how it is serialized and where it persists.

## 6. RPC routing rules

Do not assume every RPC is mesh-routable through Envoy.

There are three access patterns:
1. Mesh-routed public/internal RPCs through Envoy/xDS.
2. Direct node/controller RPCs that are not routed through Envoy.
3. Bootstrap/local RPCs that may only work before full mesh readiness.

Create or maintain an RPC routing registry:

docs/architecture/rpc-routing.md
or config/rpc_routes.yaml

For each RPC/service:
- service name
- method
- mesh-routable: yes/no
- direct endpoint required: yes/no
- bootstrap-safe: yes/no
- auth requirement
- expected caller
- xDS route name if applicable

Before calling an RPC:
- Check routing registry.
- If not mesh-routable, use the direct client/endpoint.
- Do not debug 404s blindly; first check whether the route exists.

## 7. Upstream release providers

GitHub is a provider, not the architecture.

All upstream release sources must go through repository service.

Supported providers:
- GITHUB_RELEASE
- HTTP_INDEX
- LOCAL_DIR
- GIT_INDEX

Controller must not import upstream provider packages.
Node-agent must not import upstream provider packages.
CLI must not import repository_server internals.

Correct flow:
upstream provider -> repository sync/import -> Scylla/catalog/cache -> controller desired state -> node-agent install

SyncFromUpstream must use ReleaseSource.OpenArtifact with ArtifactRef.
Do not regress to asset_url-only download logic.

ArtifactRef should carry:
- asset_url
- asset_path
- filename
- release_tag
- origin_release
- name
- version
- platform
- build_number
- sha256

A release-index with only asset_path must work for LOCAL_DIR and GIT_INDEX.

## 8. Day-0 and Day-1 rules

Day-0:
- Must read release_tag from release-index.json.
- Must not infer platform release from package filenames except as legacy fallback with warning.
- Must copy release-index.json to /var/lib/globular/release-index.json.
- Must support provider-neutral upstream registration:
  github, http, local-dir, git.
- Must continue if upstream sync fails but local bootstrap artifacts are present, while logging an exact retry command.

Day-1:
- Join scripts remain provider-neutral.
- Joining nodes talk to gateway/controller/repository, not GitHub/Git/HTTP/local-dir directly.
- Gateway join binaries must resolve from active platform release BOM, not arbitrary repository latest.
- Preferred: resolve exact package ref including version, build_number, platform, publisher, checksum.
- Latest fallback is legacy only.

## 9. Bootstrap scripts must be phase-oriented

Avoid giant scripts that do everything.

Split Day-0 scripts into clear phases:

1. detect-release
   Reads release-index.json, platform_release, release_tag.

2. publish-local-artifacts
   Publishes bundled packages into local repository.

3. register-upstream
   Registers provider-neutral upstream source.

4. sync-upstream
   Syncs selected release tag from upstream.

5. seed-desired-state
   Writes controller target/platform desired state.

6. verify-bootstrap
   Checks repository, desired state, installed state, runtime readiness.

Each phase should:
- be idempotent
- log inputs and outputs
- have a clear retry command
- fail with a useful error
- avoid hidden side effects

## 10. Required local test scripts

Add fast integration tests AI can run before tagging.

Minimum scripts:

scripts/test-release-bom.sh
- Generate fake previous release-index.
- Change one package.
- Run detect-changes.py.
- Verify only changed package gets new version.
- Verify unchanged package keeps origin_release/version.
- Verify release-index is mixed-version.

scripts/test-upstream-providers.sh
- Create a local-dir release stream.
- Create an HTTP release stream.
- Create a bare Git release stream.
- Sync the same release-index from all.
- Verify import results match.

scripts/test-day0-bom.sh
- Build or simulate installer bundle with mixed package versions.
- Ensure Day-0 reads release_tag from release-index.json.
- Ensure it does not infer from filenames.
- Ensure provider-neutral upstream vars produce correct registration.

scripts/test-day1-join-bom.sh
- Put active release-index at /var/lib/globular/release-index.json.
- Make repository contain a newer package version.
- Verify join binaries serve active BOM package, not latest.

scripts/test-upgrade-flow.sh
- Mock or run a local repository sync.
- Set desired platform release.
- Verify controller resolves exact package refs.
- Verify node-agent fetches via repository gRPC.
- Verify no provider imports in controller/node-agent.

These scripts should run quickly and be safe on a dev machine.

## 11. CLAUDE.md maintenance

CLAUDE.md is useful because it contains hard rules and known mistakes.
Keep it short enough to stay readable.

Recommended structure:
- Non-negotiable architecture rules.
- Current etcd key schema.
- Port and endpoint reference.
- Common mistakes to avoid.
- Current known issues.
- Debugging checklist.
- Links to deeper docs.

Move long historical details into docs/ai/archive/ or docs/architecture/.

When a bug is fixed:
- remove stale workaround from CLAUDE.md
- update docs
- add regression test
- add a short "avoid this mistake" note only if it is likely to repeat

Do not let memory files become a swamp.
A happy AI needs a clean map. Otherwise it starts navigating by swamp bubbles.

## 12. Debugging checklist

Before changing code, answer:

1. Which layer is wrong?
   Repository, Desired, Installed, Runtime?

2. Which source of truth owns this value?
   release-index, Scylla, etcd, node-agent, workflow, systemd, metrics?

3. Is this type proto, JSON, or handwritten internal?

4. Is this package kind coming from the canonical registry?

5. Is this RPC mesh-routable or direct only?

6. Is the package version being confused with platform release?

7. Is build_id being confused with build_number?

8. Is this path Day-0, Day-1, or steady-state?

9. Is the repository treating MinIO as cache, not authority?

10. Is controller staying provider-neutral?

If any answer is unclear, stop and inspect before patching.

## 13. AI behavior expectations

When implementing:
- Prefer small commits.
- State the invariant being preserved.
- Do not make "simple" fixes that violate the model.
- Do not add another hardcoded package list.
- Do not add GitHub-specific logic outside provider implementation.
- Do not use latest when an exact BOM ref exists.
- Do not infer meaning from filenames when release-index exists.
- Do not call an RPC through Envoy unless routing registry says it is mesh-routable.
- Do not add shell hacks where a typed workflow/action exists.

When reporting:
Always include:
- files changed
- invariant preserved
- tests run
- what remains risky
- exact retry/debug commands if applicable

## 14. Repository artifact recovery

When MinIO blobs are lost (wipe, corruption, fresh cluster), restore artifacts from GitHub releases — never rebuild from source.

**Rule: artifacts come from CI, not local builds.**

Reasons:
- Local builds lack proper ldflags (`Version=""`) → binary reports `0.0.0-dev`.
- Build environment may differ from CI (compiler version, trimpath, reproducibility).
- Checksums will not match what the repository service already has in Scylla for existing manifests.
- Leader election and entrypoint_checksum invariants break when installed ≠ repository blob.

**Correct recovery procedure:**

1. Identify the active platform release BOM: `globular repository active-release` or read `/var/lib/globular/release-index.json`.
2. Download the full release tarball from GitHub: `gh release download <tag> --repo globulario/services --pattern "globular-<ver>-linux-amd64.tar.gz"`.
3. Extract the packages directory from the tarball.
4. Authenticate: `globular auth login --user sa --password <pass>`.
5. Publish all packages with `--force`: `for pkg in packages/*.tgz; do globular pkg publish --repository globular.internal:443 --file "$pkg" --force; done`.
6. If a package fails with "version X < latest PUBLISHED Y", the repository already has a newer build in Scylla (from a prior CI publish). Publish that higher version instead, sourced from the release where Y first appeared.

**Never do:**
- `go build` → package directly → `pkg publish` (wrong checksums, missing ldflags).
- Modify binary content to fake a version bump.
- Use `scripts/build-release.sh` as a recovery tool — it is for creating new releases, not restoring existing ones.

## 15. Deploying a package — MCP tools only, never `cp`

When the user asks to "deploy," "install," or "push a fix" to the cluster, always use the MCP package pipeline. **Never write a binary directly to `/usr/lib/globular/bin/` or any other node path.**

**Forbidden — in any context, for any reason:**

```bash
# These commands are NEVER acceptable for deploying to a Globular cluster
sudo cp <binary> /usr/lib/globular/bin/<binary>
scp <binary> <node>:/usr/lib/globular/bin/<binary>
rsync <binary> <node>:/usr/lib/globular/bin/
```

**Why direct copy is always wrong:**

1. **Layer 3 (Installed) is poisoned.** The etcd installed-state record still carries the old `build_id` and `entrypoint_checksum`. The node-agent does not know the binary changed. The reconciler will eventually overwrite it.
2. **Layer 4 (Runtime) fires immediately.** The verifier computes `sha256(/usr/lib/globular/bin/<binary>)` on every sweep and compares it to the repository's `entrypoint_checksum`. A locally-built binary — even with correct ldflags — has a different checksum than what the repository recorded for the published artifact. `package.installed_binary_hash_mismatch` will appear in the doctor report within seconds.
3. **Other nodes stay on the old version.** The cluster is now split. The reconciler treats every other node as authoritative and the manually-patched node as drifted. It will revert the change on next convergence.
4. **The cluster can never be in sync.** Even if the running binary is functionally correct, the infrastructure's truth model disagrees. Automated remediation, rollback decisions, and canary analysis all operate on the recorded checksums — not on what is actually running.

**Correct deploy flow (5 steps):**

```
1. go build -ldflags "-X main.Version=<v>" -trimpath → binary in /tmp
2. sudo cp /tmp/<bin> /var/lib/globular/packages/out/<svc>-build/bin/<bin>
3. mcp__globular__package_build  (spec + root → .tgz with correct entrypoint_checksum)
4. mcp__globular__package_publish (.tgz → repository; assigns build_id)
5. globular services desired set <svc> <v>  +  globular services repair
```

Full procedure with authentication, spec extraction, and verification:
`docs/operational-knowledge/deploy-package-via-mcp.md`

**If a binary needs to be restored (e.g. after an accidental overwrite):**
Download from the GitHub release for that version — never rebuild from source. The released binary's checksum matches what the repository already recorded. See Rule 14 (Repository artifact recovery).

---

## 16. The North Star

Globular should be AI-operable because its truth is explicit.

Packages have identity.
Releases have composition.
Nodes have installed state.
Workflows have history.
Health has telemetry.
Providers are adapters.
Repository is local truth.
Controller converges.
Node-agent executes.

Do not blur these boundaries.

A happy AI is a cluster with fewer ghosts.
