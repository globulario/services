# Convergence-Committer Checksum Preservation — Impact Report (Project J)

## Headline

The workflow's `nodeSyncPackageState` actor reads `desired_hash` from
the workflow `with` map and passes it through `SyncInstalledPackage`
→ `CommitInstalledPackage` → etcd `InstalledPackage.Checksum`. But
`desired_hash` is the synthetic identity hash
(`sha256("<publisher>/<name>=<version>+b:<build_number>;<configK>=<configV>;...")`)
— NOT the binary sha256 the field is meant to carry. The actually-
correct binary hash IS already in the workflow inputs as
`resolved_entrypoint_checksum`. The committer just isn't reading it.

Result: on every committed install, `InstalledPackage.Checksum` is
populated with a synthetic identity instead of the binary sha256.

## Live evidence

24 of 25 SERVICE records on `globule-ryzen` carry a checksum that
does NOT match the on-disk binary sha256. The only exception (`mcp`)
also has no binary at the conventional `_server` path on disk, so
both sides are blank.

Examples (recorded vs on-disk first-16-hex):

| Service | recorded `Checksum` | on-disk binary sha256 | match |
|---|---|---|---|
| dns | `de2b04ff64ce4489…` | `6ed1f9c85ad27d6f…` | NO |
| rbac | `474921eca48ac51a…` | `96f8a02896d712fe…` | NO |
| event | `9a235232c718c466…` | `f735b99059059742…` | NO |
| authentication | `03ed17616e3291f0…` | `46dff7dee677fde6…` | NO |
| workflow | `85a0900a7648b43d…` | `f4cb824f307a31a0…` | NO |
| ai-router | `31d688618de07745…` | `f12000d5446fc99f…` | NO |
| ai-memory | `7408afdf41fd1eb0…` | `388ddcc9201275d8…` | NO |
| ai-watcher | `4f3555616c16a25f…` | `27fb17baf5e6d3b2…` | NO |
| ai-executor | `5047a3047fe4acd4…` | `4b7ff915437a964b…` | NO |
| file | `47c5e8456877d51a…` | `dbb6a98c694d0033…` | NO |
| media | `315a538cb6be6a9c…` | `0763d15c745960f2…` | NO |
| monitoring | `259434e49d17c22b…` | `ceeba6635859223c…` | NO |
| blog | `430a0c16ddc43dda…` | `0028eed5eedd0efe…` | NO |
| catalog | `6ea3001461f3c396…` | `425249c5b2f9c11a…` | NO |
| persistence | `f3111f1fd4231730…` | `e8a41f1028c65b9e…` | NO |
| search | `14c6fa9553f3cb0e…` | `5096bd3f0a60a9fe…` | NO |
| title | `21e5bdc685639857…` | `27bc2d11cf3d3d02…` | NO |
| log | `c3f04e60bd84a321…` | `a1d3f1ba1289b116…` | NO |
| mail | `9793cf1966491ffb…` | `2325a2cbccf6556e…` | NO |
| resource | `8879fe218786c656…` | `8fb17c7b62f92ad6…` | NO |
| backup-manager | `9bfced9ba9abaf0d…` | `39ea9e5abce6412f…` | NO |
| storage | `27f6c37c492b94dd…` | `40ba8944b59536f0…` | NO |
| torrent | (none) | `1c57126dbcb14fab…` | NO |

The 4 self-hosted records (node-agent, cluster-controller, cluster-doctor,
repository) all show MATCH because Project B's heartbeat proof writer
re-asserts `Checksum = manifest.entrypoint_checksum` every 30 seconds
and corrects the committer's wrong write.

For the other 20 records there is no compensator. The wrong value
stays.

The "phantom" value `de2b04ff64ce4489…` matches what
`docs/awareness/failure_modes.yaml` already records under
INC-2026-0014:
> `de2b04ff…` = `sha256("core@globular.io/dns=1.2.113+b:364;")`

That confirms the writer is the synthetic-identity computation
(`ComputeReleaseDesiredHash`).

## Root cause

Single primary call site:

`golang/workflow/engine/actors.go:1208` in `nodeSyncPackageState`:

```go
hash := fmt.Sprint(req.With["desired_hash"])  // ← wrong key
...
if cfg.SyncInstalledPackage != nil {
    if err := cfg.SyncInstalledPackage(ctx, name, version, hash, kind, buildID); err != nil {
```

The `SyncInstalledPackage` callback is registered in
`golang/cluster_controller/cluster_controller_server/workflow_release.go:986`:

```go
SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
    ...
    return installed_state.CommitInstalledPackage(ctx, &node_agentpb.InstalledPackage{
        NodeId:   nodeID,
        Name:     name,
        Version:  version,
        Checksum: hash,       // ← writes desired_hash into Checksum
        Kind:     kind,
        BuildId:  buildID,
    })
}
```

The workflow's `inputs` map (built at `workflow_release.go:53-66`)
already carries BOTH:

- `"desired_hash"` — synthetic identity from `ComputeReleaseDesiredHash`
- `"resolved_entrypoint_checksum"` — binary sha256 from manifest

The actor reads the wrong key.

## Hash schema confusion (already documented in INC-2026-0014)

Four distinct hash schemas flow through the install pipeline. They
must not be aliased:

| Schema | What it is | Where it lives in the workflow |
|---|---|---|
| `desired_hash` | `sha256("publisher/name=version+b:build;cfgK=cfgV;…")` — convergence identity | `req.With["desired_hash"]` |
| `resolved_artifact_digest` | sha256 of the artifact `.tgz` tarball | `req.With["resolved_artifact_digest"]` |
| `resolved_entrypoint_checksum` | sha256 of the unpacked binary (manifest `entrypointChecksum`) | `req.With["resolved_entrypoint_checksum"]` |
| `actualHash` | sha256 of the binary on disk after install, recomputed by node-agent | written into `InstalledPackage.Checksum` from `apply_package_release.go` |

The existing invariant `install_package.hash_schemas_must_not_alias`
(from INC-2026-0014) is the rule. This bug violates it: the
convergence-committer aliases `desired_hash` (synthetic) as if it
were the binary sha256.

The node-agent's `apply_package_release.go` writes `Checksum =
actualHash` (the proven binary sha256) — see lines 60, 418, 581, 690.
That's the correct schema for the `Checksum` field. Project B's
heartbeat proof writer matches that schema. Only the convergence-
committer disagrees.

## Why this matters

1. **Doctor finding noise.** Doctor's `artifact.installed_state_runtime_mismatch`
   rule compares `InstalledPackage.Checksum` to either the on-disk
   binary or the manifest. When they disagree it flags the package.
   Today this is partially masked by other findings, but for
   `workflow` we observe the doctor saying:
   `installed_state checksum f4cb824f307a differs from manifest 1f67b0ee65ce`
   — the actual install of workflow is fine; the committer wrote
   the wrong value.

2. **A future verifier would refuse to trust `InstalledPackage.Checksum`.**
   The field is documented (proto comment line 276) as "SHA256 of
   installed archive" — that wording is also wrong (it's not the
   tarball, it's the binary). Any consumer reading the field at face
   value gets a synthetic identity instead of the binary sha256.

3. **Project B compensates partially.** The self-hosted runtime
   proof writer re-asserts the correct value every 30 seconds.
   Today this covers 4 components on this node. The other 20+
   SERVICE records carry the wrong value indefinitely.

4. **The synthetic identity IS useful** — it lives in
   `ServiceRelease.Status.DesiredHash` already. We don't need to
   destroy it; we just need to stop writing it into the wrong field.

## What this report does NOT propose

Per the handoff's "do not patch before proving the writer path":

- No code change to `actors.go` or `workflow_release.go` in this
  commit.
- No raw etcd writes to fix the 20 affected records.
- No node-restart-storm to force re-installs.
- No suppression of the doctor finding family.
- No allowlist extension to use Project B's proof writer for all
  20 ordinary services (that's an over-broad fix; the right fix
  is the writer source).
- No proto edit to clarify the comment yet — that change would
  alter a public schema doc and should follow the code fix.

## Recommended fix (for the follow-up project — Project K?)

Single-line change in `actors.go:1208`:

```diff
- hash := fmt.Sprint(req.With["desired_hash"])
+ hash := fmt.Sprint(req.With["resolved_entrypoint_checksum"])
```

Plus a guard: when `resolved_entrypoint_checksum` is missing or
"<nil>" (older releases, awareness-bundle, etc.), do NOT write a
synthetic identity into Checksum — pass an empty string and let
the heartbeat path fill it in.

Plus a clarifying change to the proto comment for
`InstalledPackage.Checksum`: "SHA256 of installed binary
(entrypoint_checksum from manifest)" — to match the actual usage
in `apply_package_release.go` and `self_hosted_runtime_proof_writer.go`.

Plus 4 tests:

1. `TestNodeSyncPackageState_WritesEntrypointChecksumNotDesiredHash`
   — the actor reads `resolved_entrypoint_checksum` from `with`.
2. `TestNodeSyncPackageState_FallsBackToEmptyWhenMissingChecksum`
   — when the workflow doesn't carry the binary checksum (legacy
   path), the actor writes empty rather than the wrong synthetic.
3. `TestCommitInstalledPackage_ChecksumMatchesManifestEntrypoint`
   — end-to-end: a committed record carries the binary sha256 from
   the manifest, not the desired identity.
4. `TestApplyPackageRelease_Checksum_IsBinarySha256_UnchangedByThisChange`
   — regression: the node-agent's own commit still writes
   `actualHash`. This change must not regress that.

Plus 1 controller-side migration: a backfill that runs once on
controller start to walk existing installed_state records and
correct `Checksum` for any record where:

- `Checksum != Metadata["entrypoint_checksum"]` AND
- `Metadata["entrypoint_checksum"]` is non-empty AND
- the record was last updated by the convergence-committer (heuristic:
  `proof_source != "self_hosted_runtime_proof"`).

The backfill writes only the corrected Checksum field, preserving
everything else. Per existing invariants, no raw etcd put — use
`installed_state.WriteInstalledPackage` (which is the proper writer)
or extend it with a Set-only-Checksum variant.

The backfill is gated on the operator authorising it via a separate
handoff, because it touches 20+ records on each node.

## Awareness records (drafted, NOT yet committed)

```yaml
# failure_modes.yaml
- id: convergence_committer.checksum_field_aliases_desired_hash
  summary: |
    The workflow nodeSyncPackageState actor reads desired_hash from
    its inputs and passes it through SyncInstalledPackage to
    CommitInstalledPackage. desired_hash is the synthetic identity
    hash (sha256 of publisher/name=version+b:build;... not the binary
    sha256), but it lands in InstalledPackage.Checksum which is
    documented and used elsewhere as the binary sha256. The result
    is that committer-written installed_state records carry a
    synthetic identity in the Checksum field, while heartbeat-written
    records (apply_package_release, self_hosted_runtime_proof) carry
    the correct binary sha256. Doctor's
    installed_state_runtime_mismatch findings catch the divergence.
```

```yaml
# invariants.yaml
- id: installed_package_checksum_must_be_binary_sha256
  severity: critical
  statement: |
    InstalledPackage.Checksum MUST equal the sha256 of the installed
    binary on disk (equal to the manifest entrypoint_checksum). The
    workflow's nodeSyncPackageState actor MUST read
    resolved_entrypoint_checksum from its inputs, never desired_hash.
    Aliasing desired_hash into Checksum is forbidden by
    install_package.hash_schemas_must_not_alias (INC-2026-0014).
```

These will be added in the implementation phase, not now.

## Status

Inventory complete. No code change, no state mutation, no awareness
write. Awaiting operator authorisation for the follow-up project
that implements the actor read-key fix + backfill.

## Open observations (NOT Project J scope)

1. **The proto comment on `InstalledPackage.Checksum`** at
   `golang/node_agent/node_agentpb/node_agent.pb.go:276` reads
   `"SHA256 of installed archive"`. The actual usage everywhere
   except `nodeSyncPackageState` is binary sha256. The comment
   should be corrected during the fix project.

2. **`InstalledPackage.Metadata["entrypoint_checksum"]`** is the
   parallel-track binary sha256 written by both
   `apply_package_release.go` and the self_hosted proof writer.
   It and `Checksum` should always agree. They do for self-hosted;
   they disagree for the 20 committer-managed records.

3. **Backfill scope** — once the actor read-key is fixed, new
   commits will be correct, but existing records won't auto-heal
   unless a backfill is run. Project B's heartbeat path could be
   widened to ordinary SERVICE records (forbidden today by the
   buildId guard at `heartbeat.go:345`); the right answer is the
   backfill, not allowlist widening.

4. **Same bug shape may exist for INFRASTRUCTURE records** — needs
   the same audit. The handoff treated the inventory as illustrative
   on SERVICE; the same `nodeSyncPackageState` actor is invoked for
   INFRASTRUCTURE applies. Not surveyed in this report; would be a
   one-line CLI sweep in the next phase.
