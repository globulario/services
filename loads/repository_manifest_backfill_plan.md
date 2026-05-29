# Repository Manifest Backfill — Plan (Project D)

## Scope

Backfill `repository.manifests` rows for the 15 InfrastructureRelease
target artifacts identified by Project C. The artifact bytes are
already present in local CAS + local install dir. The break is the
Scylla index step. The repair must use the existing internal publish
workflow (`completePublish`), not a raw DB write.

## Why the existing `RepairArtifact` RPC doesn't help

`RepairArtifact` (`artifact_verify_rpc.go:145`) runs
`verifyArtifactIntegrity` and:

- If blob+manifest+checksum OK → `skipped_ok` (no Scylla write)
- If blob missing/mismatched → `RepairArtifactFromUpstream`

Our 15 artifacts have intact blobs and manifests. So `RepairArtifact`
returns `skipped_ok` and never touches the Scylla row. That's the gap:
the verifier doesn't include Scylla publish_state in its definition of
"healthy".

## Authority chosen

Enhance `RepairArtifact` with one extra branch: when integrity is OK
AND `readArtifactState(key)` is NOT `PUBLISHED`, call the existing
`completePublish(ctx, manifest, key, nil)`. This is the same
publish-pipeline call the publish_reconciler uses for stuck VERIFIED
artifacts. It:

1. Validates the local blob (Stat + size + sha256 + ledger check).
2. Writes `publish_state=PUBLISHED` via `scylla.UpdatePublishState`.
3. Transitions `artifact_state` pipeline column via
   `transitionArtifactState`.

The blob already validates (Project C inventory verified the bytes).
The new branch reuses the proven Scylla writes.

## What the enhancement does NOT do

- Does not republish artifacts (no new UploadArtifact).
- Does not fabricate manifest JSON (reads it from CAS via
  `readManifestAndStateByKey`).
- Does not patch Scylla rows directly (goes through `completePublish`).
- Does not delete desired-state entries.
- Does not change the `RepairArtifactFromUpstream` fallback path for
  truly broken blobs.
- Does not remove the `manifestJSONPresent` skip predicate from
  d2ef80ee. (The skip predicate gates the publish_reconciler's
  candidate set; the new branch operates only when the operator
  explicitly invokes `RepairArtifact`.)

## Files to change

- `golang/repository/repository_server/artifact_verify_rpc.go`:
  add the integrity-OK-but-not-PUBLISHED branch before the
  `RepairArtifactFromUpstream` call (in the non-dry-run path).
- `golang/repository/repository_server/artifact_verify_rpc_test.go`:
  one new test for the enhanced path.

Estimated diff: ~40 lines + ~80-line test.

## Action label

When the new branch fires, set `resp.Action = "repair_publish_index"`
and `resp.Detail = "Scylla index backfilled from existing local
manifest+blob via completePublish"`. This is distinct from
`repair_blob` (which means upstream re-fetch) and `skipped_ok` (which
means no work needed).

## Validation strategy

Per-artifact, in two phases:

### Phase 1 — Dry-run

For each of the 15 artifacts, invoke `RepairArtifact(dry_run=true)`.
The enhanced dry-run path returns `would_repair_publish_index` for any
artifact where integrity is OK but Scylla state is not PUBLISHED.
Capture this in the matrix's `repair_action` column.

Expected results:
- All 15 → `would_repair_publish_index`.
- Group A's older builds (gateway 1.2.109, xds 1.2.109, globular-cli
  1.2.110) → also `would_repair_publish_index` if their Scylla state
  is not PUBLISHED. Group A's desired build 1.2.113 is what should be
  active; the resolver picks latest installable.

### Phase 2 — Real backfill

For each artifact, invoke `RepairArtifact(dry_run=false)`. Capture:

- `artifact_state_before` (Scylla pipeline state)
- `artifact_state_after`
- `publish_state` before/after (read separately via
  `cqlsh SELECT publish_state FROM repository.manifests WHERE name=X`)
- Resolver test: after backfill, the controller should advance the
  matching InfrastructureRelease through PENDING → RESOLVED → AVAILABLE
  on the next reconcile cycle.

### Phase 3 — Group A BOM handling

Group A (gateway, globular-cli, xds at 1.2.113) needs a BOM bump in
addition to the Scylla backfill. After Scylla is healthy, the
controller's resolver will find 1.2.113 in `repository.manifests` and
can resolve it. The BOM is consulted by the desired-state resolver but
the per-artifact resolver also accepts a CAS+Scylla healthy artifact.

If the BOM bump is required separately, document the gap and stop
before mutating release-index.json — that's a publisher-side change
outside Project D scope.

## Forbidden operations (must not appear in diff)

- `cqlsh INSERT INTO repository.manifests` / `UPDATE`
- `etcdctl put` / `etcdctl del` on desired-state keys
- Editing `/var/lib/globular/release-index.json` directly
- Adding a new RPC just for backfill (RepairArtifact is the existing
  affordance)
- Removing or weakening `manifestJSONPresent` skip predicate
- Skipping blob verification before calling `completePublish`
- Adding a `--force-publish` flag that bypasses verification

## Rollback

If the enhanced RepairArtifact misbehaves, revert the commit. The
repository service's existing behavior (skipped_ok for healthy
artifacts) returns. The 15 IRs remain stuck — pre-Project-D state.
No data is at risk because the enhancement is read-then-write: it
verifies the local blob matches the CAS manifest before any Scylla
write.

## Outputs (after implementation + validation)

- `loads/repository_manifest_backfill_result.md` — per-artifact
  before/after + 15 rows of resolver-status-after evidence.
- `loads/repository_manifest_backfill_matrix.tsv` — the 21-column
  matrix specified in the handoff.
- Awareness records added to `docs/awareness/failure_modes.yaml` +
  `docs/awareness/invariants.yaml` (the failure_mode +
  `repository_manifest_index_must_match_published_artifact_bytes`
  invariant from the handoff).

## Next step

Implement the enhancement in `artifact_verify_rpc.go`, add the test,
deploy the repository service, run the 15-artifact validation, and
write the result.
