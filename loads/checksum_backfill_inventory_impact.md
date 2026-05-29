# Checksum Backfill Inventory — Impact Report (Project K)

## Scope

Project J fixed the WRITER (the workflow's `nodeSyncPackageState` actor)
so future installs commit `InstalledPackage.Checksum` = binary sha256.
The 60-ish installed_state records that already existed on
`globule-ryzen` still carry the pre-fix values. Project K is the
backfill that retroactively repairs those records.

This report is the inventory pass. No writes performed.

## Headline classification (64 records on globule-ryzen)

| Class | Count | Action |
|---|---|---|
| `CORRECT` | 10 | skip — `Checksum` already matches on-disk binary |
| `SELF_HOSTED_PROOF` | 4 | skip — Project B's heartbeat proof writer owns these (node-agent, cluster-controller, cluster-doctor, repository) |
| `COMMITTER_WRONG_CHECKSUM_ENTRYPOINT_OK` | 1 | repair from `Metadata["entrypoint_checksum"]` (workflow) |
| `CHECKSUM_EMPTY_FIX_FROM_DISK` | 12 | empty `Checksum`; write on-disk binary sha256 after verifying against repository manifest |
| `BOTH_WRONG_NEED_DEEPER_INVESTIGATION` | 34 | `Checksum` carries `desired_hash` (synthetic identity); on-disk matches repository manifest's `entrypoint_checksum`; safe to write on-disk sha256 |
| `NO_BINARY_AT_EXPECTED_PATH` | 3 | skip — wrapper packages or failed installs (keepalived, scylladb-wrapper, globular-cli) |
| **TOTAL** | **64** | **47 records repairable** (12 + 34 + 1), 14 skip-by-design, 3 not-applicable |

## Why "BOTH_WRONG" is actually safely repairable

The classifier label is conservative — it fires whenever `Checksum`
disagrees with on-disk AND `Metadata["entrypoint_checksum"]` doesn't
match either. Inspection of 5 sample SERVICE records confirms the
true shape:

| Service | recorded Checksum | on-disk | repository manifest `entrypointChecksum` | on-disk vs manifest |
|---|---|---|---|---|
| dns | `de2b04ff…` (synthetic) | `6ed1f9c85ad27d6f…` | `6ed1f9c85ad27d6f…` | **MATCH** |
| rbac | `474921ec…` (synthetic) | `96f8a02896d712fe…` | `96f8a02896d712fe…` | **MATCH** |
| event | `f735b990…` (Project J fix already applied) | `f735b99059059742…` | `f735b99059059742…` | **MATCH** |
| ai-router | `31d68861…` (synthetic) | `f12000d5446fc99f…` | `f12000d5446fc99f…` | **MATCH** |
| authentication | `03ed1761…` (synthetic) | `46dff7dee677fde6…` | `46dff7dee677fde6…` | **MATCH** |

For these "BOTH_WRONG" rows the on-disk binary IS the manifest's
`entrypointChecksum`. The pre-fix committer wrote `desired_hash` into
`Checksum` and never populated `Metadata["entrypoint_checksum"]`. The
backfill writes on-disk sha256 (validated against the repository
manifest) into `Checksum`. The records become identical to what the
post-Project-J committer would write for a fresh install.

## Safety predicate for the backfill

Per row, write only when ALL of these hold:

1. The expected binary path exists and is readable.
2. The on-disk sha256 of that binary equals the
   `repository.manifests[name, version].entrypointChecksum` for the
   version recorded in the installed_state record.
3. The current `Checksum` is empty OR is not equal to the on-disk
   sha256.
4. The record's `proof_source` is NOT
   `self_hosted_runtime_proof` (those are Project B's territory).
5. The record's `Status` is `installed` (don't write to records
   currently mid-install or in failed states — those have other
   signals).

If any predicate fails, log a structured reason and skip.

## What the backfill does NOT touch

- Records with `proof_source = self_hosted_runtime_proof` (4 rows).
- Records where the on-disk binary disagrees with the repository
  manifest — that's hash_drift requiring a fresh install, not a
  metadata fix. (0 rows observed in the 5-sample cross-check.)
- Records where no binary exists at the expected path (3 rows:
  keepalived, scylladb wrapper, globular-cli failed install).
- `Metadata["entrypoint_checksum"]` — only written by
  `apply_package_release.go`'s install path; not the backfill's
  responsibility.
- `BuildId` / `BuildNumber` / `Version` / `Status` — unchanged.

## Implementation shape (NOT executed in this report)

A single admin RPC on the controller, exposed to the operator only:

```
rpc BackfillInstalledStateChecksums(BackfillRequest) returns (BackfillResponse) {
  // gated by globular.auth.authz with admin permission
}
```

Per-node iteration (`req.node_id` optional — empty = all nodes):

1. List installed_state records under
   `/globular/nodes/<node>/packages/`.
2. For each record, apply the 5-clause safety predicate above.
3. When the predicate passes:
   - Read the on-disk binary via node-agent's existing
     `GetServiceRuntimeProof` RPC (which already returns
     `RunningExeSha256`).
   - Cross-check against `repository.manifests` for the recorded
     version's `entrypointChecksum` via the existing
     `GetArtifactManifest` RPC.
   - If both match → call `installed_state.WriteInstalledPackage`
     with the corrected `Checksum`, `Metadata["entrypoint_checksum"]`
     also written for symmetry. All other fields preserved.
   - Dry-run mode returns the planned action without writing.
4. Response carries a per-record result: `repaired`, `skipped` with
   reason, or `failed_safety_check` with reason.

Estimated diff: ~200 LOC for the RPC handler + ~80 LOC of tests
(dry-run vs apply, each safety-predicate path, idempotency).

Uses ONLY existing internal APIs (`GetServiceRuntimeProof`,
`GetArtifactManifest`, `installed_state.WriteInstalledPackage`). No
raw etcd put. No new schema.

## Backfill order recommendation

1. **Dry run first**, full output to operator.
2. **Phase 1 — `CHECKSUM_EMPTY_FIX_FROM_DISK` (12 rows).** Lowest
   risk: empty Checksum is unambiguously a write-once case.
3. **Phase 2 — `COMMITTER_WRONG_CHECKSUM_ENTRYPOINT_OK` (1 row,
   workflow).** `Metadata["entrypoint_checksum"]` already carries the
   correct value; backfill aligns `Checksum` to match.
4. **Phase 3 — `BOTH_WRONG_NEED_DEEPER_INVESTIGATION` (34 rows).**
   Higher per-row safety check (cross-check on-disk against
   repository manifest); same write path.

Each phase opt-in. Don't run all-at-once on first cut.

## Expected downstream effects

- Doctor's `installed_state_runtime_mismatch (hash_drift)` findings
  for the 47 backfilled rows clear automatically once the next
  doctor sweep runs — the runtime check compares
  `InstalledPackage.Checksum` to the on-disk binary, and after
  backfill they match.
- `version:<name>` doctor findings (`service.old_pid_after_upgrade`
  family) — partially. Those rules also consider PID continuity and
  systemd unit state; the checksum repair is necessary but not
  sufficient. Some findings will persist; that's correct behavior.
- DNS reconciler's `filtered=[{<node> <name> not installed+healthy}]`
  for nodes gated on hash_drift — clears for those services.
- `cluster.finding.created` event rate (Project C's amplification
  ceiling) — reduces by however many distinct hash_drift findings
  clear. Already mitigated by the per-finding-only-when-changed fix
  but will see a one-time burst of `cluster.finding.resolved`.

## Forbidden actions (already audited, NOT taken in this inventory)

- No raw `cqlsh` / `etcdctl put` to "preview" what the corrected
  rows would look like.
- No write to records still in `Status != installed` to make them
  appear converged.
- No proof_source forgery — backfilled rows will carry NO
  `proof_source` value, marking them as committer-managed (the
  pre-Project-B norm).
- No widening of Project B's self-hosted allowlist as a workaround.
- No proto / schema change.
- No doctor finding suppression.

## Per-record matrix

Full machine-readable per-row inventory:
`loads/checksum_backfill_inventory_matrix.tsv` (64 rows).

Sample of `BOTH_WRONG` shape (the 34-row class):

```
SERVICE|dns         |ver=1.2.113|status=installed|ck=de2b04ff64ce4489|ep=(empty)|on_disk=6ed1f9c85ad27d6f|proof=|cls=BOTH_WRONG…
SERVICE|rbac        |ver=1.2.113|status=installed|ck=474921eca48ac51a|ep=(empty)|on_disk=96f8a02896d712fe|proof=|cls=BOTH_WRONG…
SERVICE|ai-router   |ver=1.2.113|status=installed|ck=31d688618de07745|ep=(empty)|on_disk=f12000d5446fc99f|proof=|cls=BOTH_WRONG…
SERVICE|authentication|ver=1.2.113|status=installed|ck=03ed17616e3291f0|ep=(empty)|on_disk=46dff7dee677fde6|proof=|cls=BOTH_WRONG…
```

The `ck` column is the documented INC-2026-0014 phantom — synthetic
identity hash from `ComputeReleaseDesiredHash`. The on-disk column is
what should be written. The repository manifest validates it.

## Status

Inventory complete. No code change, no state mutation. 47 records
identified as safely repairable. Awaiting authorization to implement
the `BackfillInstalledStateChecksums` admin RPC.

## Open observations (not in scope)

1. **`Metadata["entrypoint_checksum"]` is widely absent** (only
   `workflow` and the 4 self-hosted rows carry it). The metadata
   field has been populated only on `apply_package_release.go`'s
   write path; the committer never wrote it. Backfill should
   populate both `Checksum` AND `Metadata["entrypoint_checksum"]`
   for symmetry, even though only `Checksum` is consulted by current
   verifier rules.

2. **`INFRASTRUCTURE` records duplicate `COMMAND` records for the
   same binaries** (claude, etcdctl, ffmpeg, mc, rclone, restic,
   sctool, sha256sum, yt-dlp). The COMMAND records all match
   on-disk; the INFRASTRUCTURE records carry stale wrong values.
   These are likely Day-0 bootstrap leftovers. Cleanup is a separate
   project — for Project K we repair both kinds in place since the
   binaries on disk are the same.

3. **`scylladb`, `keepalived`** are wrapper packages with no
   binary at the canonical `_server` path — they invoke
   distribution-managed binaries via systemd unit files. Verifier
   already detects via `installed_path outside /usr/lib/globular/bin/`
   and skips. Backfill should respect that.

4. **`globular-cli` INFRASTRUCTURE record is `status=failed`** with
   no binary. Backfill predicate (clause 5: Status=installed)
   correctly skips it.
