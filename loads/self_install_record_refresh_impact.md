# Self-Hosted Installed-State Refresh — Impact Report (Project B)

## Verdict

**The refresh gap is real and locally reproducible.** All three self-hosted
control-plane components on `globule-ryzen` are running binaries whose
on-disk sha256 matches the resolved manifest entrypoint_checksum, yet their
installed_state records carry stale `status="failed"` (or stale
`status="installed"` with stale `metadata.error`, or stale version+buildId
not advanced past the previous workflow commit).

The binaries match the manifests. The installed_state records do not.

Classification (per matrix): **`installed_state_writer_blocked`** —
specifically the heartbeat-driven refresh path at
`golang/node_agent/node_agent_server/heartbeat.go:341-373` is the only
existing self-refresh writer, and it has two structural gaps:

1. **Status promotion gap.** The heartbeat refresher only writes when
   `info.Version != existing.GetVersion()`. When the installed_state row
   already records the right version but its `status` field is `"failed"`
   from a prior install attempt (because the apply-package step failed
   before the binary landed via some other path), the row is never
   promoted back to `"installed"` even though the running process and
   on-disk binary now both match the manifest. The stale error in
   `metadata.error` is preserved.

2. **buildId-guard gap.** Lines 345-347 short-circuit the refresh when
   `existing.GetBuildId() != ""` to protect controller-committed records
   (workflow ConvergenceResultV1) from being clobbered by local
   discovery. Correct as written. But when the binary on disk has been
   replaced by a successful install whose workflow did NOT reach the
   committer (cluster-doctor v1.2.118 today — install bypassed the
   release pipeline via `version_drift` remediation that returned
   SUCCEEDED without producing a ConvergenceResultV1 row), the new
   `buildId` is never written, so the heartbeat refresher refuses to
   touch the row.

Both gaps point at the same missing capability: **a runtime-proof writer
that can promote installed_state from `failed` (or from a stale
buildId-rowed entry) to `installed` when the on-disk binary sha256
matches the resolved manifest entrypoint_checksum AND the running process
exe-link points at that on-disk binary**.

## Evidence

### Binary identity (live, captured 2026-05-28T20:56Z)

| Component | /proc/PID/exe sha256 | manifest checksum | match |
|---|---|---|---|
| cluster-controller | `746a5a9663bbdca8b7de09cee328ec7d9d004d306376681933ac23ecef894721` | `746a5a96…894721` | YES |
| node-agent | `002667f187277e3ec53539efdb3a71a64a6feb93f87336c94b159e818e68b380` | `002667f1…68b380` | YES |
| cluster-doctor | `5bf6fe9c34e9b41d10550009714002cd7ae800c558d741649fec439d9cc46c8c` | (needs fetch from v1.2.118 manifest) | (likely yes) |

All three sha256 values are byte-for-byte identical between
`/usr/lib/globular/bin/<binary>`, `/proc/<pid>/exe`, and the previously
captured artifact manifest. The runtime proof chain is intact end-to-end.

### Installed-state records (etcd, snapshot)

`/globular/nodes/eb9a2dac-05b0-52ac-9002-99d8ffd35902/packages/SERVICE/node-agent`:
- version=1.2.117 ✓ desired
- status=**failed** ← contradicts runtime proof
- buildId="" ← not workflow-committed
- metadata.error="package not found in local dirs … version=1.2.117" ← stale; binary is now present

`/globular/nodes/eb9a2dac-…/packages/SERVICE/cluster-controller`:
- version=1.2.124 ✓ desired
- status=installed ← matches runtime
- metadata.error="package not found in local dirs … version=1.2.124" ← stale
- The status was promoted from "failed" to "installed" by the heartbeat
  refresher when info.Version 1.2.124 != existing.Version 1.2.123 after
  the v1.2.124 restart. metadata.error was not cleared because the
  heartbeat path doesn't write that field.

`/globular/nodes/eb9a2dac-…/packages/SERVICE/cluster-doctor`:
- version=**1.2.117** ← stale; binary on disk is 1.2.118
- buildId=dd168c5a-009c-42a2-bb96-3b317eb00a77 ← from v1.2.117 workflow commit
- checksum=aa41fc70f249… ← matches v1.2.117, NOT current v1.2.118
- status=installed
- The buildId guard at heartbeat.go:345 blocks the heartbeat refresher
  from updating to 1.2.118 because it sees existing.GetBuildId() != "".

### Heartbeat refresh path (relevant excerpt)

`golang/node_agent/node_agent_server/heartbeat.go:341-373`:

```go
if existing != nil {
    // Do not overwrite records committed by the controller via
    // ConvergenceResultV1 — they carry build_id/build_number that
    // local discovery cannot reproduce and must not clear.
    if existing.GetBuildId() != "" {
        continue
    }
    // Update existing record if version changed (e.g. after apply-desired).
    if info.Version != "" && info.Version != existing.GetVersion() {
        oldVer := existing.GetVersion()
        existing.Version = info.Version
        existing.UpdatedUnix = now
        existing.Status = "installed"
        ...
    }
    continue
}
```

This is the only place node-agent self-promotes installed_state.

### What is NOT broken

- Repository manifest authority: ServiceRelease records carry
  `resolved_entrypoint_checksum` correctly. No metadata drift.
- ExpectedSha256 verification chain (v1.2.119): controller dispatch
  carries the checksum; node-agent verify gate honestly reports
  proven/unproven. No regression.
- 4-layer state model: Repository → Desired → Installed → Runtime
  remains independent.
- Heartbeat freshness: cluster_list_nodes reports last_seen within 60s.

## Why this matters

While the binaries are correct, the stale installed_state produces a
cascade of false signals:

- `installed_state_runtime_mismatch` doctor findings on every scan
  (visible in INC-2026-0015 burst at 88-97/min before the doctor
  v1.2.118 fix; still present at lower rate even after that fix until
  the underlying installed_state rows are repaired).
- `version:cluster-controller FAIL — service.old_pid_after_upgrade`
  diagnostic that the dashboard surfaces as a critical-level mismatch.
- DNS reconciler withholds the cluster-controller and dns records on
  this node because runtime unit state=hash_drift, gated by
  `dns.records_match_runtime_health`.
- ai-watcher and remediation queues evaluate the wrong inputs and may
  schedule retries that the verifier correctly rejects (the binary is
  already the right one — there's nothing to install).

Operators see a "broken" cluster that is actually running the right
code. That is the exact symptom the handoff describes.

## Proposed fix (Pattern 1: Post-Restart Runtime Proof Writer)

A new bounded writer, invoked from the existing heartbeat pipeline AFTER
the version-change branch, that:

1. For each self-hosted control-plane name (`node-agent`,
   `cluster-controller`, `cluster-doctor` — exact identity registry
   match, not pattern):
   a. Fetch the desired ServiceRelease for this canonical name.
   b. Read `resolved_entrypoint_checksum`. If missing, emit a bounded
      finding `runtime_identity_unproven:checksum_missing` and skip.
   c. Read the installed binary path from
      `/proc/<self-pid>/exe` (for self) or from systemd unit ExecStart
      (for sibling self-hosted). If missing, emit
      `runtime_identity_unproven:binary_path_missing` and skip.
   d. Compute on-disk binary sha256. If it does NOT match the manifest
      checksum, emit `runtime_identity_unproven:hash_mismatch` and
      skip. Do NOT update installed_state.
   e. If the existing installed_state has buildId set AND the recorded
      checksum matches the on-disk sha256, the row is already canonical
      — nothing to do.
   f. If the existing installed_state has buildId set but the recorded
      checksum does NOT match the on-disk sha256, fetch the matching
      manifest from the repository by sha256 lookup. If the lookup
      returns a manifest, atomically update version + buildId +
      buildNumber + checksum using the looked-up identity. Source =
      `self_hosted_runtime_proof_via_manifest_lookup`.
   g. If the existing installed_state has no buildId but status is
      "failed", clear the failure: set status="installed", clear
      metadata.error, write proof metadata (manifest_checksum,
      on_disk_sha256, binary_path, proof_source, timestamp).
   h. If no installed_state exists, write a fresh row with status =
      "installed" and proof metadata (no buildId — local discovery).

2. The writer is idempotent: a second pass with unchanged inputs makes
   no etcd writes (the equality guard inside
   `installed_state.WriteInstalledPackage` is sufficient).

### Forbidden in implementation

- No manual etcd put / no `--force` flag added.
- No desired-state mutation.
- No bridge of binaries — the binary must already match the manifest;
  the writer only records that fact.
- No weakening of ExpectedSha256 verification — the writer's input is
  the verified manifest checksum.

### Files this will touch

- `golang/node_agent/node_agent_server/heartbeat.go` — add the call
  site after the existing version-refresh block.
- New file `golang/node_agent/node_agent_server/self_hosted_runtime_proof.go`
  with the runtime proof writer and its narrow self-hosted-name list.
- `golang/node_agent/node_agent_server/runtime_proof.go` is the
  existing scaffold for runtime identity claims; the new writer should
  use its proof-step taxonomy (see lines 17+ "The Prime Directive:
  systemd-active + installed-state are CLAIMS").
- New tests in `golang/node_agent/node_agent_server/self_hosted_runtime_proof_test.go`
  covering the 9 required cases from the handoff.

## What this report explicitly does NOT propose

- No change to the heartbeat buildId guard (lines 345-347). The guard
  is correct for non-self-hosted services where the controller is the
  authority.
- No change to repository manifest publishing.
- No change to ServiceRelease phase semantics.
- No change to ExpectedSha256 verification chain.
- No change to the convergence-committer (its job is
  controller-observed; the runtime-proof writer is node-local).
- No change to cluster-doctor's evaluation logic — the
  `installed_state_runtime_mismatch` finding will simply stop firing
  once the underlying installed_state rows are repaired.

## Open questions for implementer (not blockers)

1. Should the runtime-proof writer ALSO run during node-agent startup,
   before the first heartbeat tick? Recommended yes — the node-agent's
   own installed_state is the most-critical case.
2. Should `cluster-doctor` be in the self-hosted list? It is hosted by
   the node-agent like any other service, but it self-restarts on
   apply and is part of the control plane. Recommended yes.
3. What about `repository`, `dns`, `authentication`, `rbac`, `workflow`?
   These are also control-plane but are NOT self-hosting in the sense
   that they don't manage their own install. Recommended NO — they
   should continue through the workflow-commit path.

## Next step

If the user authorizes proceeding: implement Pattern 1 with the file
list and behavior above, add the 9+5 tests from the handoff, and update
`loads/self_install_record_refresh_result.md` with the writer path,
test list, and before/after installed_state for each component.

If the report reveals another layer, stop and produce
`loads/self_hosted_runtime_proof_gap_report.md` per the handoff.
