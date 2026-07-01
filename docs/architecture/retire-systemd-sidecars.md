# Globular Clean Design: Retire Systemd SHA256 Sidecars

## Status

**Implemented.** The design described below is complete as of `v1.2.262`. Key
commit chain: `2c7962db` (foundation) → `708d4aa4` (no UpdatedUnix bump on
skip-restamp) → `be663efb` (v1.2.262: binary-hash fallback for infrastructure
packages on skip path). See
`docs/operational-knowledge/incidents/sidecar-receipt-retirement-2026-06-03.md`
for the full incident chain and verification.

The canonical installation helpers are:

| Helper | File | Purpose |
|---|---|---|
| `canonicalInstallReceiptOpts` | `canonical_unit_render.go` | Build `ReceiptOpts` from package identity; renders unit from artifact tarball (`artifact-canonical-v1`) |
| `stampCanonicalReceiptForInstalledPackage` | `canonical_unit_render.go` | New-install canonical stamp (used by MinIO reconcile, apply_package_release) |
| `restampReceiptOnInstallSkip` | `installer_api.go` | Skip-path restamp; falls through to unit-only for infrastructure packages whose binary is at a system path |
| `StampInstallReceipt` / `installreceipt.Stamp` | `install_receipt.go` / `internal/installreceipt/` | Chokepoint: writes only `pkg.Metadata`, never `InstalledUnix`/`UpdatedUnix` |
| `stampReceiptForInstalledPackage` | `install_receipt.go` | Legacy simple stamp (command/binary-only paths still on `apply_package_release.go`) |

---

## Goal

Redesign Globular's unit-file drift detection so the cluster no longer depends on `.sha256` sidecar files beside systemd units.

The clean target is:

> **No filesystem-side truth shards. Installed state in etcd is the canonical installation receipt. The filesystem is evidence, not authority.**

This is a design/refactor task, not a quick patch for the current cluster state.

---

## Current Problem

Today, Globular writes files like:

```text
/etc/systemd/system/globular-<service>.service
/etc/systemd/system/globular-<service>.service.sha256
```

The sidecar stores the expected SHA256 of the unit file at install time.

Heartbeat then compares:

```text
sha256(current systemd unit) != content of .sha256 sidecar
```

and reports:

```text
state = "hash_drift"
```

This detects post-install systemd unit mutation, but it creates a second authority outside the canonical cluster state model.

The sidecar becomes a fragile filesystem-side cache of a fact that belongs in `installed_state`.

---

## Design Rule

```text
The filesystem is not authority.
The filesystem is evidence.
```

Expected installed output must be stored in etcd, inside the Layer 3 installed-state model.

---

## Clean 4-Layer Model

Globular should preserve this state model:

```text
Layer 1: desired_state
    What should exist?

Layer 2: package / manifest state
    What artifact defines it?

Layer 3: installed_state
    What was actually materialized on this node?

Layer 4: runtime_state / heartbeat
    What is alive right now?
```

The expected unit hash belongs in:

```text
installed_state.metadata["unit_file_sha256"]
```

not in:

```text
/etc/systemd/system/<unit>.sha256
```

---

## Target Architecture

### Before

```text
Install action:
    write systemd unit
    write <unit>.sha256 sidecar

Heartbeat:
    read systemd unit
    read <unit>.sha256 sidecar
    compare actual hash against sidecar hash

Doctor:
    consumes heartbeat hash_drift state
```

### After

```text
Install action:
    write systemd unit
    calculate rendered unit SHA256
    stamp installed_state.metadata["unit_file_sha256"]

Heartbeat:
    read systemd unit
    calculate actual SHA256
    read expected SHA256 from installed_state
    compare actual hash against installed_state hash

Doctor:
    consumes heartbeat drift state derived from installed_state comparison
```

---

## Installed State Should Become an Installation Receipt

Do not store only the unit-file hash.

Installed state should become a complete proof of what the node-agent installed.

Recommended fields:

```json
{
  "service": "torrent",
  "node": "globule-ryzen",
  "version": "1.2.xxx",
  "build_id": "...",
  "package_sha256": "...",
  "artifact_digest": "...",
  "binary_path": "...",
  "binary_sha256": "...",
  "unit_file_path": "/etc/systemd/system/globular-torrent.service",
  "unit_file_sha256": "...",
  "config_sha256": "...",
  "env_file_sha256": "...",
  "unit_renderer_version": "...",
  "installed_at": "...",
  "installed_by": "node-agent",
  "install_plan_id": "..."
}
```

Fields may be omitted when not applicable, but the structure should support all installable outputs that matter.

---

## Drift Classes To Support

The current sidecar design only detects one thing:

```text
unit file changed after install
```

The clean design should support multiple drift classes:

```text
binary_drift
unit_file_drift
config_drift
env_file_drift
package_digest_mismatch
renderer_version_mismatch
installed_state_missing
```

This gives the doctor/verifier better diagnosis and avoids hiding all integrity issues behind one generic `hash_drift` state.

---

## Reconciliation Semantics

After this change, the reconciliation model should be simple:

```text
desired_state != installed_state
    install or reinstall is needed

installed_state != filesystem evidence
    local mutation, corruption, partial install, or out-of-band modification

installed_state == filesystem evidence but runtime_state != installed_state
    runtime/service failure

package manifest missing or digest mismatch
    repository integrity failure

installed_state missing
    fail closed; do not pretend the node is healthy
```

---

## Required Code Changes

### Writers

Update every install/reconcile path that materializes systemd units or binaries.

Known write sites to inspect:

```text
node_agent_server/internal/actions/artifact.go
node_agent_server/minio_systemd_reconcile.go
apply-desired / desired-state install path
any service-specific install or recovery path that rewrites units
```

Each writer must:

1. Render/write the unit file.
2. Compute the rendered unit SHA256.
3. Compute binary/config/env hashes where applicable.
4. Stamp the values into `installed_state.metadata`.
5. Commit installed_state as the canonical install receipt.

Do not write a `.sha256` sidecar as authoritative state.

---

### Readers

Update heartbeat drift detection.

Known read sites to inspect:

```text
node_agent_server/server.go
checkUnitHashDrift
cluster_doctor/rules/objectstore_topology.go
cluster_doctor/rules/objectstore_physical_overlap.go
```

Heartbeat must:

1. Read expected hashes from `installed_state`.
2. Hash live filesystem evidence.
3. Emit precise drift states when actual values differ.
4. Treat missing installed_state as unsafe/unknown, not healthy.
5. Stop requiring `.sha256` sidecars.

Doctor rules should consume drift state from heartbeat/runtime status, not from sidecar existence.

---

## Legacy Migration

Sidecars may exist on older nodes. They should become legacy fallback only.

Migration strategy:

```text
1. If installed_state has unit_file_sha256:
       use installed_state only.

2. Else if installed_state is missing unit_file_sha256 but sidecar exists:
       read sidecar once as legacy input;
       stamp installed_state.metadata["unit_file_sha256"];
       mark migration source = "legacy_sidecar";
       stop depending on the sidecar afterward.

3. Else:
       report installed_state_missing_or_unproven;
       fail closed.
```

Do not silently regenerate trust from the current unit file unless the operation is explicitly classified as a repair action.

A repair action may exist, but it must be deliberate and visible:

```text
repair action:
    trust current filesystem output as new installed_state proof
    record repair timestamp/source/reason
```

---

## Forbidden Shortcut

Do not solve the long-term design by simply adding more calls that rewrite `.sha256` sidecars.

That only patches the symptom.

The clean design removes sidecars as authority.

---

## Short-Term Compatibility Option

If needed during rollout:

```text
install path may continue writing sidecars temporarily
```

but only as compatibility artifacts.

They must not be the primary source for drift detection.

Target end state:

```text
sidecars ignored
sidecars deleted by cleanup
installed_state is sole authority
```

---

## Required Tests

Add or update tests proving:

### Install Receipt Tests

```text
install writes installed_state.metadata["unit_file_sha256"]
install writes binary_sha256 when binary exists
install writes package/artifact digest
install records renderer version when available
```

### Heartbeat Drift Tests

```text
heartbeat detects unit_file_drift without sidecar
heartbeat detects binary_drift
heartbeat detects config_drift where config hash exists
heartbeat does not require .sha256 sidecar
heartbeat fails closed when installed_state proof is missing
```

### Doctor Tests

```text
doctor consumes heartbeat drift state
doctor no longer requires sidecar files
objectstore topology rules still degrade hash-drift/active-drift states correctly
physical-overlap rules still work with new drift states
```

### Migration Tests

```text
legacy sidecar can seed installed_state once
legacy sidecar is ignored after installed_state is stamped
missing installed_state + missing sidecar reports unsafe/unproven
repair action records provenance when trusting current filesystem output
```

### Regression Tests

```text
apply-desired rewriting a unit updates installed_state hash
MinIO reconcile rewriting a unit updates installed_state hash
node-agent reinstall updates installed_state hash
torrent/node-agent/unit rewrite path cannot create false drift
```

---

## Acceptance Criteria

**All criteria met as of v1.2.262.**

```text
1. ✅ No production runtime path depends on <unit>.sha256 as authority.
      heartbeat.checkUnitHashDrift reads installed_state; refuses to downgrade
      canonical receipts with legacy_sidecar content.
2. ✅ checkUnitHashDrift reads expected hash from installed_state.
      Falls back to legacy sidecar only when installed_state carries zero
      receipt provenance (migration-once semantics).
3. ✅ Install/reconcile paths stamp installed_state hashes.
      New path: stampCanonicalReceiptForInstalledPackage (artifact-canonical-v1).
      Skip path: restampReceiptOnInstallSkip (falls through to unit-only for
      infra packages whose binary is at a system path, not globularBinDir).
      MinIO reconcile: switched from legacy refreshMinioUnitSidecar() (removed)
      to stampCanonicalReceiptForInstalledPackage.
4. ✅ Doctor findings still report drift correctly.
      unit_receipt_drift rule surfaces unit_file_drift (WARN) and
      installed_state_missing_or_unproven (CRITICAL).
5. ✅ Legacy sidecars are optional and only used for migration/fallback.
6. ✅ Missing installed proof fails closed (installed_state_missing_or_unproven
      = CRITICAL in doctor).
7. ✅ Tests: installer_api_skip_restamp_test.go covers skip-path unit-only
      restamp when binary hash fails.
8. ✅ Cluster converges without manually editing unit files or sidecars.
      v1.2.262 deployed; doctor reported 0 receipt findings post-deploy.
```

---

## Desired Final Mental Model

Globular should be able to answer four questions cleanly:

```text
What did we want?
    desired_state

What package promised it?
    package_manifest / artifact digest

What did we install?
    installed_state receipt

What is running now?
    heartbeat / runtime_state
```

Then diagnosis becomes mechanical:

```text
desired intent
    -> package identity
        -> installed proof
            -> runtime proof
                -> doctor diagnosis
```

No sidecar authority. No duplicate truth. No filesystem crumbs pretending to be cluster state.

---

## Instruction To Claude/Codex

Implement the clean sidecar retirement design.

Focus on correctness of the architecture before patching the immediate incident.

Use `installed_state.metadata` as the canonical location for expected rendered output hashes, starting with:

```text
unit_file_sha256
binary_sha256
config_sha256
env_file_sha256
package_sha256
artifact_digest
unit_renderer_version
```

Then migrate heartbeat and doctor logic away from `.sha256` sidecar files.

Preserve compatibility only as a migration fallback, not as the long-term model.

Do not introduce manual repair, silent trust reset, or filesystem-derived authority without explicit provenance.

The final system must treat installed_state as the installation receipt and the live filesystem as evidence checked against that receipt.
