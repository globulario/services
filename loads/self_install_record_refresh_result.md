# Self-Hosted Installed-State Refresh — Result Report (Project B)

## Status

**IMPLEMENTED & VALIDATED LIVE.** All three self-hosted control-plane
components on `globule-ryzen` now have `installed_state` records with
proof metadata (`proof_source = self_hosted_runtime_proof`, plus
`proof_manifest_checksum`, `proof_on_disk_sha256`, `proof_binary_path`,
`proof_time_unix_ms`) and the stale `metadata.error` fields are cleared.

## Implementation summary

| Field | Value |
|---|---|
| Pattern | 1 — Post-Restart Runtime Proof Writer |
| New file | `golang/node_agent/node_agent_server/self_hosted_runtime_proof_writer.go` (340 lines) |
| Test file | `golang/node_agent/node_agent_server/self_hosted_runtime_proof_writer_test.go` (19 tests) |
| Heartbeat hook (startup + 5min) | Phase 1.75 in `syncInstalledStateToEtcd` |
| Heartbeat hook (30s) | New call in `runHeartbeat` (commit `a6af5d8f`) |
| Allowlist | `node-agent`, `cluster-controller`, `cluster-doctor` |
| Commits | `14fbbc50` (initial), `a6af5d8f` (30s tick to defeat committer overwrite) |
| Awareness | `failure_modes.yaml` + `invariants.yaml` updated |

## Per-component result

After the v1.2.119 node-agent install and 1-2 heartbeat cycles
post-commit, the etcd records show:

### node-agent

| Field | Before (failed) | After (proof) |
|---|---|---|
| version | 1.2.117 | 1.2.118 (then 1.2.119 after redeploy) |
| status | `failed` | `installed` |
| buildId | (empty) | from workflow commit |
| checksum | n/a | `dcd3e46cf1e33046…` (matches on-disk + manifest) |
| metadata.error | `package not found in local dirs…` | (cleared) |
| metadata.proof_source | — | `self_hosted_runtime_proof` |
| metadata.proof_on_disk_sha256 | — | `dcd3e46cf1e33046…` |

### cluster-controller

| Field | Before (stale error) | After (proof) |
|---|---|---|
| version | 1.2.124 | 1.2.124 |
| status | `installed` | `installed` |
| checksum | (n/a or stale) | `746a5a9663bbdca8…` (matches on-disk + manifest) |
| metadata.error | `package not found in local dirs…` | (cleared) |
| metadata.proof_source | — | `self_hosted_runtime_proof` |
| metadata.proof_on_disk_sha256 | — | `746a5a9663bbdca8…` |

### cluster-doctor

| Field | Before (stale buildId-guarded) | After (proof) |
|---|---|---|
| version | 1.2.117 (stale) | 1.2.118 (refreshed) |
| status | `installed` | `installed` |
| buildId | `dd168c5a-…` (stale) | `019e7136-d4da-…` (current) |
| checksum | `aa41fc70…` (stale, v1.2.117) | `5bf6fe9c34e9b41d…` (v1.2.118, matches on-disk + manifest) |
| metadata.proof_source | — | `self_hosted_runtime_proof` |
| metadata.proof_on_disk_sha256 | — | `5bf6fe9c34e9b41d…` |

## Forbidden actions taken: NONE

- No manual `etcd put` against installed_state.
- No desired-state mutation.
- No binary bridge for stale records (the proof writer is read-only on
  binaries — it doesn't move bytes around; it records that on-disk bytes
  already match the manifest).
- No removal of the heartbeat.go:345 buildId guard.
- No weakening of ApplyPackageRelease's ExpectedSha256 verification chain.
- No use of `ResolvedArtifactDigest` (tarball sha256) as binary identity.
- No promotion of status from version string or filename alone.
- No application of self-hosted proof semantics to ordinary services
  (allowlist is strictly the 3 names; `dns`, `rbac`, `authentication`,
  `event`, etc. continue through the existing heartbeat refresh path).

## Tests added (17 + 2 = 19)

Proof-builder (6):
- `TestSelfHostedProof_HappyPath`
- `TestSelfHostedProof_ManifestMissing`
- `TestSelfHostedProof_EntrypointChecksumMissing`
- `TestSelfHostedProof_ProcessNotRunning`
- `TestSelfHostedProof_ProcExeUnreadable`
- `TestSelfHostedProof_BinaryHashMismatch`

Path safety (1):
- `TestSelfHostedProof_BinaryPathUnexpected`

Status-promotion (6):
- `TestSelfHostedProof_StatusFailedPromotesToInstalled`
- `TestSelfHostedProof_StaleErrorCleared`
- `TestSelfHostedProof_RefreshWriterRequiresProof`
- `TestSelfHostedProof_BuildIDMismatchRefreshesForSelfHosted`
- `TestSelfHostedProof_OrdinaryServiceNotInAllowlist`
- `TestSelfHostedProof_IdempotentSkipsAlreadyCanonical`

Regression (6):
- `TestSelfHostedProof_BuildIDGuardPreservedForOrdinaryServices`
- `TestSelfHostedProof_UsesEntrypointChecksumNotArtifactDigest`
- `TestSelfHostedProof_HashSchemaSeparation`
- `TestSelfHostedProof_AwarenessBundleNotInAllowlist`
- `TestSelfHostedProof_DepsHaveNoMutationCapability`
- `TestSelfHostedProof_MetadataRecorded`

All 19 pass; full `./node_agent/node_agent_server` test suite remains green
(`go test ./node_agent/node_agent_server -count=1` → ok 99.308s).

## Critical discovery: convergence-committer overwrite

The first deploy (commit `14fbbc50`) wrote proof metadata at startup,
but the controller's convergence-committer wrote node-agent's record
~10 seconds later (timestamp `21:19:39` in controller log:
`convergence-committer: committed node=eb9a2dac… SERVICE/node-agent@1.2.118 build_number=1`),
overwriting the proof metadata with its own claim — including a `checksum`
field that did NOT match the on-disk binary.

The proof writer was only invoked in `syncInstalledStateToEtcd` (startup +
5-minute ticker), so the proof metadata stayed absent for up to 5 minutes
between committer overwrites. Commit `a6af5d8f` also calls
`refreshSelfHostedInstalledState` from `runHeartbeat` (30-second tick), so
the metadata is re-established within at most one heartbeat of any
committer write. The writer is idempotent — stable-state cycles produce
zero etcd writes via `proofCanRefreshInstalledState`'s
"already_canonical" skip.

This finding suggests a follow-up project: should the controller's
`CommitInstalledPackage` for self-hosted components preserve proof
metadata fields it does not own? That would eliminate the 30s window
where the record is stale. Not in scope for Project B.

## Writer behaviour matrix (for future debugging)

| Existing record | Proof status | Result |
|---|---|---|
| absent | proof passes | write fresh record with proof metadata |
| same identity, status=`installed`, proof metadata present | proof passes | `already_canonical` — no etcd write |
| same identity, status=`failed` with stale error | proof passes | promote status; clear error; write proof metadata |
| same version, different buildId | proof passes (self-hosted) | refresh buildId + checksum + proof metadata |
| same version, different buildId | proof passes (ordinary svc) | not applicable — ordinary services never reach this path |
| any | proof fails (any reason) | record untouched; reason logged at INFO |

## Open observations (NOT in Project B scope)

1. **`syncRepoArtifactsToEtcd` does not overwrite when version is set.**
   Verified by reading line 802 of heartbeat.go: "Already has a real
   version with correct kind — skip install" → `continue`. So Phase 2
   is NOT the source of the post-commit checksum drift; the
   convergence-committer is.

2. **The convergence-committer's checksum claim does not equal the
   on-disk binary.** The recorded value (`57d1fbc8…` for node-agent
   v1.2.118 between writes 14fbbc50 and a6af5d8f) is neither the
   manifest entrypoint_checksum (`dcd3e46c…`) nor the artifact tarball
   digest (`4d1bebbd…`). Worth a separate investigation —
   `CommitInstalledPackage` may be writing a stale workflow value.

3. **Heartbeat phase order is unchanged.** Phase 1.75 sits between
   `detectPartialApply` and `syncRepoArtifactsToEtcd`. Phase 2 does NOT
   overwrite Phase 1.75's writes because the version equality guard at
   line 802 protects them. The committer overwrites happen out of band.

## Validation commands

```bash
# Read current installed_state record for any self-hosted component:
sudo etcdctl --endpoints=https://10.0.0.63:2379 \
  --cacert=/var/lib/globular/pki/ca.crt \
  --cert=/var/lib/globular/pki/issued/services/service.crt \
  --key=/var/lib/globular/pki/issued/services/service.key \
  get /globular/nodes/eb9a2dac-05b0-52ac-9002-99d8ffd35902/packages/SERVICE/cluster-doctor

# Watch the proof writer run live:
sudo journalctl -u globular-node-agent -f | grep "self-hosted runtime proof"

# Verify the proof chain manually:
PID=$(systemctl show globular-cluster-doctor.service -p MainPID --value)
sudo sha256sum /proc/$PID/exe
# → 5bf6fe9c34e9b41d10550009714002cd7ae800c558d741649fec439d9cc46c8c
# Compare to the ServiceRelease ResolvedEntrypointChecksum (etcdctl get
# /globular/resources/ServiceRelease/core@globular.io/cluster-doctor).
```

## Rollback notes

If the proof writer misbehaves, the rollback sequence is:

1. Revert commits `a6af5d8f` and `14fbbc50` on master.
2. Deploy node-agent (bump patch) — the new build has neither the proof
   writer nor the 30s tick, so installed_state records return to their
   pre-Project-B behaviour: stale `status=failed`, stale `metadata.error`,
   stale buildId for components replaced outside the workflow path.
3. The cluster continues operating; no user data is at risk.

The existing heartbeat refresh writer at heartbeat.go:341-373 is
untouched by this project; its buildId guard at line 345 remains in
place for ordinary services.
