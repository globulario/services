# Package Lifecycle — operator-grade reference

This is the canonical operator guide for Globular's 9/10 package management
layer. Read this when you need to:

- Diagnose why an artifact is blocked (`verify` / `explain`)
- Repair a corrupted blob from upstream (`repair`)
- Roll a service back to a previous verified revision (`pkg rollback`)
- Configure publisher trust + signature policy (`trust-publisher`)
- Resolve a config-file conflict before an upgrade can proceed (`pkg config conflicts`)
- Triage repository / package findings reported by `cluster-doctor`

The earlier `repository-overview.md` doc covers the *static* model — what
artifacts are and how they get versions. This doc covers the *dynamic*
contract — what happens between publish and PUBLISHED, what can break,
and what the operator runs to fix each break.

---

## Two state machines, one invariant

Every artifact carries two independent state columns:

| Column | Owner | Purpose |
|---|---|---|
| `publish_state` (proto `PublishState`) | Public lifecycle gate | What resolvers / catalog / RBAC have always used: `STAGING`, `VERIFIED`, `PUBLISHED`, `DEPRECATED`, `YANKED`, `QUARANTINED`, `REVOKED`, `CORRUPTED`, `ARCHIVED` |
| `artifact_state` (Go `ArtifactPipelineState`) | Durable repository pipeline tracker | Records the FULL path through publish: `DISCOVERED → DOWNLOADING → BLOB_WRITTEN → BLOB_VERIFIED → MANIFEST_WRITTEN → LEDGER_WRITTEN → PUBLISHED`, plus `QUARANTINED`, `REVOKED`, `BROKEN_MISSING_BLOB`, `BROKEN_CHECKSUM_MISMATCH` |

The two are kept coherent on transition (admin actions like `pkg quarantine`
dual-stamp both). The doctor's `repository.revoked_installable` /
`repository.quarantined_installable` invariants fire if they diverge.

**Core invariant.** An artifact is *installable* only when ALL hold:

- `publish_state == PUBLISHED`
- `artifact_state == PUBLISHED` (or empty for legacy rows during transition)
- The exact binary blob exists at `binaryStorageKey(artifactKeyWithBuild(...))`
- Blob size matches the manifest's `size_bytes`
- Blob sha256 matches the manifest's `checksum`
- Signature policy passes (see below)

Every install path enforces this — sync's skip gate, the resolver, the
download gate, rollback eligibility, and the controller's release
resolution. Bypassing any one would silently degrade the cluster, which is
why the same `signaturePolicyDecision` helper is reused at all sites.

---

## Repository CLI — verify / repair / explain

These are read-only operator surfaces. Each maps 1:1 to a repository RPC;
the CLI never duplicates verification logic.

```bash
# Read-only integrity probe. Returns OK or one of BROKEN_MISSING_BLOB,
# BROKEN_CHECKSUM_MISMATCH, BROKEN_LEDGER_MISSING, BROKEN_MANIFEST_MISSING,
# QUARANTINED, REVOKED, INCONCLUSIVE.
globular repository verify  core@globular.io/echo 1.0.84

# Re-import from the upstream source recorded in the manifest. Refuses
# REVOKED unconditionally; refuses QUARANTINED unless --allow-quarantine-override.
globular repository repair  core@globular.io/echo 1.0.84
globular repository repair  core@globular.io/echo 1.0.84 --dry-run

# Composite "why is this broken?" answer — manifest + ledger + blob +
# signature + pipeline state, plus recommended_action.
globular repository explain core@globular.io/echo 1.0.84
```

All three accept `--platform`, `--build-number`, `--kind`, and `--json`.
Non-zero exit on broken / blocked / uninstallable unless `--quiet` is set.

The MCP tools `repository_verify_artifact`, `repository_repair_artifact`,
and `repository_explain_artifact` expose the same functions to AI executors.

---

## Signature policy

Every install path consults the central `signaturePolicyDecision` helper
before declaring an artifact installable. Policy lives in etcd at
`/globular/repository/security/policy` (proto `SignaturePolicy`):

```yaml
require_signatures_for_core: true            # core@globular.io must be signed
require_signatures_for_all: false            # strict mode (off in stable so far)
allow_unsigned_local_development: false      # set true only on dev nodes
trusted_core_publishers:
  - core@globular.io
quarantine_on_invalid_signature: true        # auto-QUARANTINE on bad sig
```

### Trust + sign commands

```bash
# Register a publisher's ed25519 public key. Admin-only.
globular repository trust-publisher core@globular.io \
    --key /var/lib/globular/keys/core-prod-2026.pub.pem \
    --key-id core-prod-2026 \
    --notes "production signing key, rotated 2026-04"

# Revoke a key (terminal — verify will return SIGNATURE_REVOKED_KEY).
globular repository revoke-publisher-key core@globular.io \
    --key-id core-prod-2026 \
    --reason "rotated"

# Inspect trust state across all publishers.
globular repository trusted-publishers

# Verify the most recent signature on an artifact against trusted publishers.
globular repository signature verify core@globular.io/echo 1.0.84

# List every signature recorded on an artifact.
globular repository signature list   core@globular.io/echo 1.0.84

# Register a detached signature you produced locally.
# (Globular never sees your private key — produce sig.bin with openssl/age/etc.)
globular repository signature sign   core@globular.io/echo 1.0.84 \
    --signature sig.bin --key-id core-prod-2026
```

Signature payload: the canonical lowercase `sha256:<hex>` form of the
artifact's checksum. With openssl + an ed25519 key:

```bash
printf 'sha256:abc123...' | openssl pkeyutl -sign -inkey ed25519.key > sig.bin
```

### What the policy gates

| Gate site | Behavior when policy requires + signature missing/invalid |
|---|---|
| `SyncFromUpstream` final transition | `LEDGER_WRITTEN → PUBLISHED → QUARANTINED`, dual-stamp `publish_state=QUARANTINED` |
| Resolver `loadPublishedCatalog` / `pickBestCandidate` | Excluded from results |
| `DownloadArtifact` | `FailedPrecondition` with `signature policy: <reason>` |
| Rollback `evaluateRollbackCandidate` | `eligible=false`, `signature_status=...` reported |
| Repository self-findings | Emits `REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED` for the doctor |

Revoked keys ALWAYS block, even when policy doesn't require signatures.

---

## Rollback

Rollback is a real cross-service operation, not a downgraded re-install.

```bash
# Show eligible previous revisions on this node + their verify/signature status
globular pkg rollback-candidates echo

# Roll to the immediately previous revision (CLI calls workflow.StartRun)
globular pkg rollback echo --previous

# Roll to a specific version (caller must own the choice)
globular pkg rollback echo --to-version 1.0.82

# Preview only — same eligibility evaluation, no workflow run created
globular pkg rollback echo --previous --dry-run
```

The CLI starts a `package.rollback` workflow run via `WorkflowService.StartRun`.
The workflow runs through:

1. `repository.rollback.resolve_target` — picks the target revision
2. `repository.artifact.verify` — full integrity probe
3. `repository.signature.verify` — signature policy gate
4. `cluster_controller.snapshot_state` — current node state
5. `node_agent.package.config.classify` — pre-install config snapshot
6. `node_agent.service.drain` — stop the service
7. `node_agent.package.apply` — `ApplyPackageRelease(rollback_mode=true)`
   - bypasses the auto-rollback-forbidden guard
   - runs `applyConfigPolicyPreInstall` (FAIL_ON_LOCAL_MODIFICATION blocks here)
   - extracts target binary, restarts unit, waits for active
   - records the new `InstalledPackageRevision` row with `action=rollback`
   - emits one `PackageConfigReceipt` per declared config file
8. `node_agent.services.verify_integrity` — hash check
9. `node_agent.package.config.apply_policy` — config policy enforcement
10. `node_agent.service.start` / `service.health_probe` — runtime gate
11. `repository.revision.record` — final revision row

Failures at step 7+ trigger `package.rollback.forward_recover`, which
re-applies the prior revision when the previous identity is known.

### Rollback safety rules

- Target must be `PUBLISHED` with blob verified — the eligibility check
  refuses anything else before the workflow even starts.
- Target must not be `REVOKED`. Refused unconditionally.
- Target must not be `QUARANTINED` unless `--allow-quarantine-override`.
- Downgrade is explicit: rollback always sets `allow_downgrade=true`. Other
  call sites must pass `--force`.
- The new `InstalledPackageRevision` row is written ONLY after
  `WaitActive(unit, 30s)` succeeds — a failed rollback never claims success.

The repository owns the install-history table:

```bash
# Newest-first revision history for a package on each node
globular workflow get <run-id>                          # follow live progress
# (revision listing is currently exposed via the MCP tool
#  repository_list_installed_revisions and the rollback-candidates CLI;
#  a `pkg revisions` command lands when the operator UX scope grows.)
```

---

## Config ownership

Manifests declare config files via a `configs[]` block:

```yaml
configs:
  - path: /etc/globular/config/services/echo.json
    config_kind: DEFAULT
    merge_strategy: PRESERVE
    preserve_on_upgrade: true
    restore_on_rollback: false
  - path: /etc/globular/operator/echo.override.json
    config_kind: OPERATOR_OVERRIDE
    merge_strategy: PRESERVE
  - path: /var/lib/globular/echo/secrets.key
    config_kind: SECRET
    merge_strategy: SECRET_EXTERNAL
    sensitive: true
```

`config_kind` is one of `DEFAULT`, `OPERATOR_OVERRIDE`, `GENERATED`,
`SECRET`, `RUNTIME_STATE`. `merge_strategy` is one of `REPLACE`,
`PRESERVE`, `THREE_WAY_MERGE`, `TEMPLATE_RENDER`, `APPEND_ONLY`,
`FAIL_ON_LOCAL_MODIFICATION`, `SECRET_EXTERNAL`.

Defaults applied when a manifest leaves a field unset:

| `config_kind` | Default `merge_strategy` | `sensitive` | `preserve_on_upgrade` |
|---|---|---|---|
| DEFAULT | REPLACE | false | false |
| OPERATOR_OVERRIDE | PRESERVE | false | **forced true** |
| GENERATED | TEMPLATE_RENDER | false | false |
| SECRET | SECRET_EXTERNAL | **forced true** | false |
| RUNTIME_STATE | APPEND_ONLY | false | false |

### Pre-install gate (the one that can block)

Before `InstallPackage` writes anything, the node-agent runs
`applyConfigPolicyPreInstall`:

1. Loads the manifest's `configs[]`
2. Captures pre-install file checksums (skipping SECRET paths)
3. For each entry whose `merge_strategy=FAIL_ON_LOCAL_MODIFICATION`,
   compares the on-disk checksum to the manifest's `checksum_at_install`.
   If they differ, emits a `CONFLICT` receipt and returns the apply with
   `Status=blocked_config_conflict`.

That snapshot is then threaded into the post-success hook so receipts
report accurate `checksum_before` vs `checksum_after`, and classify the
action as `PRESERVED` (file unchanged) or `REPLACED` (file changed).

### Operator commands

```bash
# Browse declared config files for a package (with default-resolved
# merge_strategy + sensitive flags). Read-only.
globular pkg config list   echo
globular pkg config verify echo

# List unresolved conflict receipts (action=CONFLICT). Exits non-zero
# when conflicts exist — perfect for CI gates.
globular pkg config conflicts echo
```

The MCP tools `package_config_conflicts` and `package_config_list` provide
the same surface to AI executors so an executor can answer "is anything
blocking this upgrade?" in one call.

### Receipt actions reference

| Action | Meaning |
|---|---|
| `PRESERVED` | File on disk kept; new package default available at `.pkg-new` if applicable |
| `REPLACED` | File overwritten with package default |
| `GENERATED` | Re-rendered from desired state |
| `MERGED` | Three-way merge succeeded (when implemented) |
| `CONFLICT` | Operator must intervene; apply was BLOCKED |
| `RESTORED` | Restored from a rollback snapshot (`restore_on_rollback=true`) |
| `SKIPPED_SECRET` | SECRET file — node-agent never reads/writes; no checksums leak |
| `FAILED` | Exec error; treat as needs-investigation |

SECRET receipts ship with `path="[REDACTED]"` on the wire — operator output
never carries secret paths.

---

## Doctor invariants

The repository service emits self-reported findings via
`ListRepositoryFindings`. The cluster-doctor pulls these into its
snapshot and renders one `Finding` per kind with severity + a
recommended CLI command:

| Doctor invariant ID | Severity | Recommended remediation |
|---|---|---|
| `repository.published_missing_blob` | ERROR | `globular repository repair <pub>/<name> <ver>` |
| `repository.published_checksum_mismatch` | ERROR | `globular repository repair <pub>/<name> <ver>` |
| `repository.published_unsigned_required` | ERROR | `globular repository signature verify <pub>/<name> <ver>` |
| `repository.revoked_installable` | ERROR | `globular repository artifact revoke <pub>/<name> <ver>` |
| `repository.quarantined_installable` | ERROR | `globular repository artifact quarantine <pub>/<name> <ver>` |
| `package.config_conflict` | WARN/ERROR | `globular pkg config conflicts <name>` |
| `package.rollback_failed` | ERROR | `globular workflow get <run-id>` + node-agent logs |

Each finding's evidence map carries `artifact_key`, `current_state`,
`expected_state`, plus kind-specific fields (`signature_status`,
`blob_status`, etc.). The doctor's `EntityRef` is `node_id/artifact_key`
when applicable so per-node triage works.

---

## Workflow / actor wiring

The `package.rollback` workflow's `actor: node_agent` steps are dispatched
via `WorkflowService.executor → WorkflowActorService.ExecuteAction`. The
node-agent registers `WorkflowActorService` in `main.go` (next to
`NodeAgentService`); the `NodeAgentActorServer` in `actor_service.go`
routes by action name:

| Action | Handler |
|---|---|
| `package.apply` | `ApplyPackageRelease(rollback_mode=…)` |
| `service.drain` | `supervisor.Stop(unit)` |
| `service.start` | `supervisor.Restart(unit)` |
| `service.health_probe` | `supervisor.WaitActive(unit, 30s)` |
| `services.verify_integrity` | `VerifyPackageIntegrity` |
| `package.config.classify` / `apply_policy` | no-op success (folded into apply) |
| `package.rollback.forward_recover` | re-applies previous revision |

Unknown actions return `ok=false` with a clear message — actors never
silently swallow steps.

---

## Deployment + migration notes

Every change in this layer is **additive at the wire and storage level**. No
cluster wipe, no `pkg revoke` mass-action, no manifest rewrite needed.

### Wire compatibility

| Change | Compatibility |
|---|---|
| New repository RPCs (verify / repair / explain / signature / rollback / config receipts / findings) | ✅ Additive. Old clients don't call them; old servers (running pre-Phase-F binaries) return `Unimplemented` if a new client does call. |
| New `ArtifactManifest` fields (`configs`, `signature_key_id` at field numbers 70–71) | ✅ Additive. Old clients silently ignore unknown fields. |
| New `ApplyPackageReleaseRequest` fields (`rollback_mode`, `preserve_configs`, etc. at field numbers 12–19) | ✅ Additive. Old node-agents ignore them; pre-Phase-F controllers don't set them. |
| New enums (`ArtifactVerifyStatus`, `MergeStrategy`, `ConfigKind`, `TrustState`, `SignatureStatus`, `ConfigReceiptAction`) | ✅ Additive. |
| New Scylla columns on `manifests` (`artifact_state`, `artifact_state_reason`, `artifact_state_updated_unix`, `artifact_state_workflow_run_id`, `blob_key`, `build_id`) | ✅ Additive. `ALTER TABLE ADD` runs idempotently at startup. |
| New Scylla tables (`trusted_publishers`, `artifact_signatures`, `installed_revisions`, `config_receipts`) | ✅ Additive. `CREATE TABLE IF NOT EXISTS` at startup. |
| Internal Go method renames (e.g. `srv.VerifyArtifact` → `verifyArtifactIntegrity`) | Build-time only. Not on the wire. |

### Recommended rolling-upgrade order

1. **Repository service** first. Brings up the new RPCs + Scylla schema. Old node-agents continue to work via the existing `DownloadArtifact`/`GetArtifactManifest` surface.
2. **Workflow service** — picks up the new `package.rollback.yaml` definition.
3. **Node-agent** on each node — registers `WorkflowActorService`. Until each node is upgraded, that node can't be a rollback target's executor (workflow runs hang on `package.apply`); other operations are unaffected.
4. **Cluster-doctor** — picks up the `repositoryFindings` invariant.
5. **CLI / MCP** can be rolled out anytime.

### What runs automatically on first new-binary startup

- **Scylla schema migrations** — `ALTER TABLE ADD` on `manifests` + `CREATE TABLE IF NOT EXISTS` for the four new tables. O(1) DDL operations, instant.
- **`artifact_state` backfill** — `backfillArtifactStates(ctx, max)` classifies legacy `manifest` rows: blob present + size match → `PUBLISHED`, blob missing → `BROKEN_MISSING_BLOB`, size mismatch → `BROKEN_CHECKSUM_MISMATCH`. Stat-only (no full hashing); idempotent; runs bounded so startup is never blocked.

### Signature policy — migration-safe default

The shipped default is **permissive** so that upgrading an existing cluster
does not instantly block unsigned core artifacts:

```yaml
# Default (when /globular/repository/security/policy is absent in etcd)
require_signatures_for_core: false
require_signatures_for_all: false
allow_unsigned_local_development: true
trusted_core_publishers: [core@globular.io]
quarantine_on_invalid_signature: true   # bad sigs always blocked
```

REVOKED keys ALWAYS block regardless. To opt in to strict mode, register
keys + sign artifacts first, then write the policy:

```bash
# Register a trusted publisher key
globular repository trust-publisher core@globular.io \
    --key /var/lib/globular/keys/core-prod-2026.pub.pem \
    --key-id core-prod-2026

# Sign each PUBLISHED core artifact (produce sig.bin locally with openssl)
globular repository signature sign core@globular.io/echo 1.0.84 \
    --signature sig.bin --key-id core-prod-2026

# Confirm coverage
globular repository signatures list   # via repository signature list <pkg> <ver>

# Flip to strict mode in etcd
etcdctl put /globular/repository/security/policy '{
  "require_signatures_for_core":   true,
  "trusted_core_publishers":       ["core@globular.io"],
  "quarantine_on_invalid_signature": true
}'
```

The policy is cached for 30 seconds in the repository service; a flip takes
effect on the next read. No restart required.

### What is NOT yet populated after deploy

- **`installed_revisions`** is empty until the next install / upgrade /
  rollback runs. `pkg rollback-candidates <name>` will show no candidates
  until at least one new revision is recorded after the upgrade. This is
  expected — the table starts at "now".
- **`artifact_signatures`** is empty until operators sign artifacts.
  `repository signature verify` returns `SIGNATURE_MISSING` for every
  artifact; with the permissive default that does NOT block installability.
- **`config_receipts`** populates as packages with `manifest.configs[]`
  declarations get installed/upgraded. Pre-Phase-D packages emit no
  receipts (manifest has no configs[] block).

### Cluster-doctor collector wiring

The `repositoryFindings` invariant is registered in the rules registry, but
the doctor's `collector` does not yet call `Repository.ListRepositoryFindings`
to populate `Snapshot.RepositoryFindings`. Until that wire-up lands, the
invariant produces zero findings — which is correct behavior (no findings,
nothing to surface). The MCP tool `repository_list_findings` calls the
repository directly and works today.

---

## Cross-reference

| Operator question | Run this | RPC underneath |
|---|---|---|
| "Is this artifact safe to install?" | `repository verify` / `explain` | `Repository.VerifyArtifact` / `ExplainArtifact` |
| "Why is the resolver hiding it?" | `repository explain` | `Repository.ExplainArtifact` |
| "Fix this missing blob" | `repository repair --from-upstream` | `Repository.RepairArtifact` |
| "Who signed this?" | `repository signature verify` | `Repository.VerifyArtifactSignature` |
| "Trust this publisher key" | `repository trust-publisher` | `Repository.TrustPublisher` |
| "Show me previous versions to roll back to" | `pkg rollback-candidates` | `Repository.ListRollbackCandidates` |
| "Roll back this service" | `pkg rollback --previous` | `WorkflowService.StartRun(package.rollback)` |
| "Did the upgrade clobber my config?" | `pkg config conflicts` | `Repository.ListConfigReceipts(action=CONFLICT)` |
| "What's broken cluster-wide?" | `globular doctor report` | `Repository.ListRepositoryFindings` (pulled by doctor) |

The MCP tool surface mirrors the CLI 1:1 so AI executors / IDE assistants
can invoke any of these without shelling out.
