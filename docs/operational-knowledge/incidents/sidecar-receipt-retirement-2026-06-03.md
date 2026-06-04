# Incident closure — sidecar receipt retirement (2026-06-03)

**Status**: closed
**Final verification**: cluster doctor `overall_status = PASS` modulo two
pre-existing, scoped-out findings (see §5).

---

## 1. Problem summary

For the lifetime of the platform up to commit `2c7962db`, `.sha256`
sidecar files written next to systemd unit files at
`/etc/systemd/system/globular-<name>.service.sha256` acted as a hidden
filesystem authority — the heartbeat read them to decide whether a
unit had drifted from its installed bytes.

That dual authority (sidecar on disk **and** `installed_state.metadata`
in etcd) produced subtle, persistent drift. After this incident:

- `installed_state.metadata` is the **sole** authority for expected
  installed-output content (`unit_file_sha256`, `binary_sha256`,
  `installed_by`, `entrypoint_checksum`, `proof_*`).
- `.sha256` sidecars are **legacy migration input only** — read once
  by the heartbeat's `checkUnitHashDrift` to seed a receipt when no
  canonical receipt has ever been stamped, and only when the live
  etcd row has zero receipt provenance.
- A sidecar file on disk **cannot** override a canonical receipt.
- All install paths (controller-dispatched and node-agent skip path)
  stamp `installed_state.metadata` through the canonical chokepoint
  `StampInstallReceipt`.
- Doctor surfaces the new authority-state classes (`unit_file_drift`,
  `installed_state_missing_or_unproven`) so neither dropping nor
  silently downgrading the receipt produces an invisible failure.

## 2. Root causes found

The incident was a chain of five distinct bugs that compounded across
the install + heartbeat + verifier layers. Each was confirmed against
live cluster evidence before patching, and each fix shipped with a
regression test on a pure helper.

1. **Install paths did not stamp receipts consistently.** Several
   install code paths (workflow `package.report_state`, infrastructure/
   wrapper installs that short-circuited via `installSkipAllowed`)
   completed without ever stamping a canonical receipt. The package
   was correctly installed; the metadata authority simply did not
   record that fact.

2. **Non-install writers erased receipt metadata.** Heartbeat,
   self-hosted proof writer, and peer-checksum write paths constructed
   a fresh `InstalledPackage` and overwrote etcd, dropping any prior
   receipt fields. Fixed with `installreceipt.Preserve` at every
   non-install write site.

3. **Cluster-controller `SyncInstalledPackage` clobbered metadata.**
   The `node.sync_installed_package_state` workflow step wrote a
   fresh `InstalledPackage{}` with no metadata and called
   `CommitInstalledPackage`, which has no read-modify-write semantics.
   Every install workflow cycle nuked the canonical receipt that
   installer-api had stamped seconds earlier. Fixed by inserting a
   read-modify-write step in the controller's callback so only the
   cross-validated identity fields (Version, Checksum, BuildId, Kind)
   are overwritten; metadata flows through.

4. **Heartbeat legacy migration could downgrade canonical receipts.**
   `checkUnitHashDrift` operated on a stale pkg snapshot pre-fetched
   by `buildUnitToPackageMap`. If a canonical install committed
   between snapshot and check, the stale snapshot looked receipt-less,
   the sidecar-migration branch fired, and the canonical receipt was
   overwritten with the 4-key legacy_sidecar shape. Fixed by
   re-reading etcd fresh inside the migration branch and refusing to
   migrate when the fresh row carries any receipt provenance.

5. **Infrastructure/wrapper skip path did not restamp receipts.** When
   `canSkipInstallPackage` returned `installSkipAllowed`, the workflow
   short-circuited without re-stamping. Wrapper packages (envoy,
   keepalived) that survive sweeps without reinstallation kept their
   legacy_sidecar receipt forever. Fixed by adding
   `restampReceiptOnInstallSkip` at the top of the skip branch (before
   the runtime-proof block, so the wrapper-binary discovery limitation
   doesn't prevent the stamp).

6. **Skip-path restamp incorrectly bumped `UpdatedUnix`.** The
   first version of `stampSkipPathReceipt` did
   `pkg.UpdatedUnix = time.Now().Unix()` on every restamp. The
   verifier reads `max(installedUnix, updatedUnix)` as `ApplyTime`,
   so every restamp looked like a fresh apply at wall clock — and
   any running process whose start time predated the restamp fired
   `service.old_pid_after_upgrade` (the same INC-2026-0016 class of
   bug the proof writer was hardened against). Fixed by removing the
   wall-clock bump; the forensic "when did the restamp run" trail is
   preserved via `metadata.installed_at` (a separate field not
   consumed by `ApplyTime`).

## 3. Commits

The full chain, in order:

| Commit | Subject |
|---|---|
| `2c7962db` | node-agent: retire systemd .sha256 sidecars as authority (foundation) |
| `c474a4b1` | node-agent: route install writers through installed_state receipts |
| `1ebf5a60` | node-agent: preserve install receipts across non-install writes |
| `11e68483` | node-agent: preserve receipts in peer-checksum write path |
| `58d92f17` | node-agent: extract installreceipt sub-package; wire package_state through it |
| `976131ee` | node-agent: package.report_state stamps canonical install receipt |
| `91671230` | node-agent: stop heartbeat from clobbering canonical receipt with legacy migration |
| `47d7a541` | controller: sync_installed_state must preserve receipt across read-modify-write |
| `edf1766a` | cluster-doctor: recognize installed-state receipt drift classes |
| `72ecf067` | node-agent: stamp receipts for infrastructure wrapper installs |
| `76d4966e` | node-agent: move skip-path restamp BEFORE runtime-proof check |
| `708d4aa4` | node-agent: do not advance UpdatedUnix on skip-path receipt restamp |

(Plus this closure note — see §end.)

## 4. Final verification

Live state at closure (2026-06-03 21:47 UTC):

| Check | Result |
|---|---|
| Doctor `overall_status` | **PASS** |
| `unit_receipt_drift` findings | 0 |
| `service.old_pid_after_upgrade` findings | 0 |
| `hash_drift` findings | 0 |
| `installed_state_missing_or_unproven` findings | 0 |
| `subsystem.stuck` | 0 (transient during restart window — cleared on next sweep) |
| `globular-node-agent` | active |
| `globular-envoy` | active |
| `globular-torrent` | active |

Receipt provenance confirmed canonical on:
- node-agent (`installed_by = node-agent.installer-api`,
  `unit_file_sha256` matches disk, `binary_sha256` matches `/proc/PID/exe`)
- envoy (`installed_by = node-agent.grpc_workflow.install_skip_restamp`,
  `unit_file_sha256` matches disk, `binary_sha256` matches `bin/envoy`)

`migration_source = legacy_sidecar` is absent from both records — both
superseded by canonical installed_by.

## 5. Remaining unrelated findings

Two findings remain after closure, neither caused by or related to
this incident chain:

1. `scylla_manager.cluster_registered` — WARN. scylla-manager HTTPS
   endpoint reachable but TLS trust failure blocks safe verification
   (refuses to fall back to HTTP). Pre-existing TLS trust issue.
2. `artifact.layout_drift_local` — INFO. cleanup-candidate entries
   under `/var/lib/globular` (empty legacy aliases, backup files).
   Pre-existing housekeeping debt.

Both were explicitly scoped out of every phase of this incident.

## 6. Lessons / invariants reinforced

- **Filesystem is evidence, not authority.** Files on disk are
  observed and hashed; they do not vote on what *should* be there.
  `installed_state.metadata` is the only authority.
- **All writers must preserve receipt metadata.** Non-install writers
  (heartbeat, proof writer, peer-checksum, workflow callbacks) MUST
  call `installreceipt.Preserve` or do read-modify-write before
  committing. Fresh-struct overwrites are the anti-pattern.
- **Read-modify-write at every commit chokepoint.** Even when the
  caller "knows" what changed, partial overwrites must be done by
  fetching the existing row, mutating only the intended fields, and
  writing back. `CommitInstalledPackage` does not RMW for you.
- **Heartbeat observes; it does not overwrite canonical proof.** The
  heartbeat's migration paths must re-read etcd fresh before any
  write, and must back off if the fresh row already carries a
  canonical receipt.
- **Wall-clock `UpdatedUnix` is never correct unless a real apply
  happened.** A metadata-only restamp does not advance `InstalledUnix`
  or `UpdatedUnix`. The verifier uses `max(installedUnix, updatedUnix)`
  as `ApplyTime`; bumping it to wall clock on a non-apply event
  manufactures a false `service.old_pid_after_upgrade` for every
  running process. (INC-2026-0016 was a sibling of this pattern.)
- **Wrapper / infrastructure packages must stamp receipts even when
  the install path short-circuits.** The skip decision proves on-disk
  content matches desired — that's the right moment to record the
  canonical receipt, not the wrong moment to skip it.
- **Doctor must surface the authority-state classes that the
  heartbeat emits.** A new state name on the wire that the doctor
  doesn't recognise becomes an invisible failure
  (`fm.industry.partial_failure_hidden_by_global_green`). The
  `unit_receipt_drift` rule was added explicitly to keep both
  `unit_file_drift` and `installed_state_missing_or_unproven` visible.

## 7. Next recommended work (not executed)

Listed in priority order; each is an independent task:

- **A. scylla-manager TLS trust fix.** Wire the cluster CA into
  scylla-manager's trust store (or document the explicit policy if
  HTTP fallback is desired). Would clear the WARN finding.
- **B. `artifact.layout_drift_local` cleanup.** Remove the empty
  legacy alias directories under `/var/lib/globular/` (or have the
  installer prune them on upgrade). Would clear the INFO finding.
- **C. Optional sidecar janitor.** A one-shot pass that removes
  `/etc/systemd/system/globular-*.service.sha256` files on nodes
  whose installed_state already carries canonical receipts.
  Sidecar files are now harmless (the heartbeat refuses to use them
  to clobber canonical receipts), so this is cosmetic — but it
  finishes the retirement.
- **D. Optional renderer determinism regression test.** A small unit
  test that renders the same Envoy unit twice from the same inputs
  and asserts byte-identical output. The renderer was already proven
  deterministic by diagnosis during this incident, but a guard test
  would catch a future regression (map iteration order, etc.).
