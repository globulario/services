# Milestone — cluster at zero findings (2026-06-03)

**Status**: PASS.
**Date**: 2026-06-03 22:43 EDT.

---

## 1. Final state

| Check | Value |
|---|---|
| Doctor `overall_status` | **PASS** |
| Total findings | **0** |
| globular-node-agent | active |
| globular-cluster-doctor | active |
| globular-envoy | active |
| globular-torrent | active |
| globular-scylla-manager | active |

## 2. Completed incident chain (in order, across the day)

1. **Retired systemd `.sha256` sidecars as filesystem authority** — sidecars
   are now legacy migration input only, never an authority that can
   override `installed_state.metadata`.
2. **Moved expected-unit proof into `installed_state.metadata`** —
   `unit_file_sha256`, `binary_sha256`, `installed_by`, `entrypoint_checksum`,
   `proof_*` are recorded through the canonical `StampInstallReceipt`
   chokepoint.
3. **Fixed receipt preservation across non-install writers** — heartbeat,
   self-hosted proof writer, peer-checksum write path now call
   `installreceipt.Preserve` before commit.
4. **Fixed controller-side `SyncInstalledPackage` metadata clobber** —
   `node.sync_installed_package_state` workflow callback now does
   read-modify-write so it cannot wipe canonical receipt metadata
   seconds after installer-api stamped it.
5. **Added doctor rules for new receipt drift classes** —
   `unit_receipt_drift.unit_file_drift` (WARN) and
   `unit_receipt_drift.installed_state_missing_or_unproven` (CRITICAL,
   fail-closed per `state.unknown_must_not_default_to_healthy`).
6. **Fixed infrastructure / wrapper receipt restamp** — the install-skip
   path now re-stamps the canonical receipt so wrapper packages whose
   install short-circuits don't keep a `legacy_sidecar` marker forever.
7. **Fixed wall-clock `UpdatedUnix` bump** — `stampSkipPathReceipt` no
   longer advances `UpdatedUnix` on metadata-only restamps. Restored
   the INC-2026-0016 protection that the previous restamp commit had
   re-broken.
8. **Fixed scylla-manager TLS trust** — `scylla-manager.yaml` now
   declares the Globular service cert / key (live ops fix + permanent
   packaging fix at `packages` commit `ea647b7`, artifact 3.10.1+1
   published).
9. **Fixed `artifact.layout_drift_local` false positives** — doctor rule
   no longer recommends cleanup for legacy-alias dirs that are actively
   pinned by `WorkingDirectory=` or `ExecStartPre … mkdir`. cluster-doctor
   1.2.143 deployed; finding reduced from 9 candidates (6 false
   positives + 3 real) to 3 real candidates.
10. **Quarantined only verified-safe residuals** — `cluster_controller`,
    `node_agent` (both empty), and `day0-install.jsonl` (closed Day-0
    install trace from 2026-06-02) moved to a timestamped quarantine
    subdir under `/var/lib/globular/.cleanup-quarantine/`. Reversible;
    no permanent deletion.

## 3. Quarantine record

**Path**: `/var/lib/globular/.cleanup-quarantine/1780540980/`

| Entry | Type | Original path | Notes |
|---|---|---|---|
| `cluster_controller` | dir (empty) | `/var/lib/globular/cluster_controller` | mode 750, globular:globular |
| `node_agent` | dir (empty) | `/var/lib/globular/node_agent` | mode 755, root:root |
| `day0-install.jsonl` | file (7932 bytes, 56 lines) | `/var/lib/globular/day0-install.jsonl` | closed Day-0 trace; `run_finish status=ok ts=1780411839911` (2026-06-02 10:50) |

Reversible: no permanent deletion occurred. Contents recoverable from
the quarantine directory.

## 4. Remaining known architectural debt (not active findings)

- **Unit templates still emit underscore `WorkingDirectory=` for several
  services** (ai-executor, ai-memory, ai-router, ai-watcher,
  backup-manager, cluster-doctor). These paths are actively pinned by
  systemd, so the doctor's `artifact.layout_drift_local` rule
  (post-fix in commit `c093b684`) correctly silences them — they are
  NOT current findings.
- A future task should migrate the unit templates / package specs from
  the underscore form to the canonical dash form. That refactor:
  - touches every affected package's spec yaml in the `packages` repo
  - requires careful migration (cannot just rename — running services
    have open handles on the underscore dirs; needs a deliberate stop
    → rename → start sequence per service)
  - is purely cosmetic / hygienic from doctor's perspective (the dirs
    are operationally allocated either way)
- Until that migration happens, the underscore dirs will continue to
  exist and be tolerated by the doctor rule; this is the intentional
  trade-off baked into commit `c093b684` ("evidence not permission to
  delete").

## 5. Important invariants learned / reinforced

- **Filesystem is evidence, not authority.** Files on disk are observed
  and hashed; they do not vote on what *should* be there. The
  authoritative source is `installed_state.metadata`.
- **Doctor findings are evidence, not permission to delete.** A
  cleanup-candidate verdict requires independent confirmation that the
  candidate is unreferenced before any operational action.
- **`installed_state.metadata` is the canonical install receipt** —
  the single chokepoint for "what does this install path attest to?"
- **Non-install writers must preserve receipt metadata.** Heartbeat,
  proof writers, workflow callbacks, peer-checksum writers MUST use
  `installreceipt.Preserve` or read-modify-write.
- **Heartbeat observes; it must not overwrite canonical proof.** The
  migration paths must re-read fresh state before any write and back
  off when fresh state already has a canonical receipt.
- **`UpdatedUnix` must not advance unless a real apply/restart
  occurred.** Wall-clock bumps on metadata-only restamps reproduce
  INC-2026-0016 (false `service.old_pid_after_upgrade` on every PID
  whose start time predates the restamp).
- **Cleanup candidates must be checked against
  systemd/process/installed_state references before action.** The
  `artifact.layout_drift_local` doctor rule now enforces this guard;
  the operator workflow (this incident) also enforces it.
- **Awareness must be called before high-risk code or operational
  state work.** This was reinforced multiple times across the day;
  the project hook (`enforce-briefing.sh`) is now the backstop. The
  briefing is the only mechanism connecting a local code change to
  the global architectural intent.

## 6. Final verification

- Doctor `overall_status = PASS`, **0 findings** (read via
  `globular doctor report cluster --fresh`).
- All 5 key services active (`globular-node-agent`,
  `globular-cluster-doctor`, `globular-envoy`, `globular-torrent`,
  `globular-scylla-manager`).
- **No restarts** triggered during the final quarantine step.
- **No etcd mutation.**
- **No `apply-desired`** beyond the targeted `services desired set
  cluster-doctor 1.2.143` needed to deploy the false-positive fix.
- **No publishing** beyond the targeted artifacts that motivated each
  fix (node-agent 1.2.147..1.2.155, cluster-controller 1.2.150,
  scylla-manager 3.10.1+1, cluster-doctor 1.2.142..1.2.143).
- **No permanent deletion** — all moves into
  `/var/lib/globular/.cleanup-quarantine/1780540980/` are reversible.
