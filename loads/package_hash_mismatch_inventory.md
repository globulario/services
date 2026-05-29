# Package Hash Mismatch Inventory — Post-v1.2.119

## Date

2026-05-28 (single-node cluster, ryzen = `eb9a2dac-05b0-52ac-9002-99d8ffd35902`)

## Trigger

After the v1.2.119 verification chain landed and the workflow YAML sync re-seeded
etcd, the node-agent began reporting `failed_binary_hash_mismatch` for 7
services: `dns`, `event`, `file`, `log`, `mcp`, `repository`, `cluster-doctor`.

The Post-etcd-Recovery Hash Mismatch Plan handoff requested a classification
inventory **before** any republish work.

## Method

For each affected service we collected four hashes:

1. `manifest_entrypoint_checksum` — what the **repository's** Scylla
   `repository.manifests` row says is the binary's identity. Query:

   ```sql
   SELECT name, version, build_id, entrypoint_checksum, publish_state
   FROM repository.manifests WHERE name='<svc>' AND version='<ver>'
   ALLOW FILTERING;
   ```

2. `package_json_entrypoint_checksum` — what the **local pinned package
   tarball's** `package.json` says is the binary's identity. Extracted from
   `/var/lib/globular/packages/pinned/<svc>_<ver>_linux_amd64.tgz`.

3. `extracted_binary_sha256` — actual sha256 of the entrypoint binary
   inside the local pinned tarball.

4. `installed_binary_sha256` — actual sha256 of the binary on disk at
   `/usr/lib/globular/bin/<entrypoint>`.

We also captured the `expected_sha256` value the node-agent **received** for
each rejected install attempt and traced its origin.

## Findings — all four hashes agree per service

| service          | desired version | manifest = pkg.json = extracted = installed |
|------------------|-----------------|---------------------------------------------|
| dns              | 1.2.113         | `6ed1f9c85ad27d6f17d15f7cc7f41c9a76323fb8e1cdb9f33feb89c11d219086` |
| event            | 1.2.113         | `f735b990590597424372b2f10f46f353e9bb696ba959c03682f6ec86c243bfbc` |
| file             | 1.2.113         | `dbb6a98c694d00339d7c8aea0d495fe909697ba59beafc4d5b5d7de54b353aff` |
| log              | 1.2.113         | `a1d3f1ba1289b1162f02fc7533820f10f2f30d5a9fc950d9d63be7276bc2a950` |
| mcp              | 1.2.113         | `daff5251f0f92aa592599819d569260505eedcfcc6f8a5686e32c2e78ef5d69b` |
| repository       | 1.2.118         | `55691bb9a4922c2264d53763f5436c873a8e0137917ebfc8ba63690f20eb01c2` |
| cluster-doctor   | 1.2.117         | `67a590941eb2ba6fd2c6f4f466eb7b76246af36a3570f6bdd5e3e057f4d59fdf` |

So:

* The **repository Scylla manifest** publishes the same entrypoint_checksum
  the local package's manifest declares.
* The **local pinned tarball's binary** hashes to that exact value.
* The **installed binary on disk** hashes to that exact value.

No layer is wrong. No package is mislabeled.

## Findings — the rejection's "expected" hash is the convergence identity, not a binary hash

For dns the node-agent rejection log line read:

```
apply-package: REJECTED SERVICE/dns@1.2.113 — installed binary hash mismatch
  for SERVICE/dns at /usr/lib/globular/bin/dns_server:
  expected sha256=de2b04ff64ce4489f2d8a6b151571f6f3941cb28674e0124e9d3db2d739ad414
  actual   sha256=6ed1f9c85ad27d6f17d15f7cc7f41c9a76323fb8e1cdb9f33feb89c11d219086
  build_id=3bff8f77-ba13-4b7a-b2e7-01dd51c403c5
```

The same `build_id` (`3bff8f77...`) is present in both the Scylla manifest and
the local package.json. The `actual sha256` (`6ed1f9c8...`) matches every other
layer. The `expected sha256` (`de2b04ff...`) matches **nothing**.

Hash-origin search:

```python
sha256("core@globular.io/dns=1.2.113+b:364;") = de2b04ff64ce4489f2d8a6b151571f6f3941cb28674e0124e9d3db2d739ad414
```

— exactly `ComputeReleaseDesiredHash("core@globular.io", "dns", "1.2.113", 364, nil)`
in `golang/cluster_controller/cluster_controller_server/release_hash.go:21`.

## Root cause: v1.2.119 regression in node-agent runInstallPackage

Before v1.2.119 the install-package workflow inputs did **not** carry
`desired_hash`. The node-agent's runInstallPackage path contained a fallback:

```go
desiredHash := strings.TrimSpace(inputs["desired_hash"])
if desiredHash == "" {
    desiredHash = strings.TrimSpace(inputs["expected_sha256"])
}
// ...
ApplyPackageReleaseRequest{ ExpectedSha256: desiredHash }
```

— harmless only because neither key was set.

My v1.2.119 commit `c185abde` (`workflow_release.go` `buildNodeDirectApplyConfig.InstallPackage`
callback) added BOTH keys to the RunWorkflow inputs:

```go
Inputs: map[string]string{
    ...
    "desired_hash":    desiredHash,        // ComputeReleaseDesiredHash output
    "expected_sha256": expectedSha256,     // manifest entrypoint_checksum
},
```

The fallback then bound `desiredHash` to `inputs["desired_hash"]` (the
convergence identity), and `ApplyPackageReleaseRequest.ExpectedSha256` received
the **convergence identity hash** instead of the **binary hash**. The
node-agent verify gate honestly returned `installed_binary_hash_mismatch`
because the two schemas can never be equal.

End-to-end the system was correct — repository, controller resolver, manifest,
local package, installed binary all agreed. The bug was one variable aliasing
in the node-agent.

## Classification

All 7 services are the same class:

```
node_agent_install_package_hash_schema_alias_regression
```

(adjacent to the handoff's listed values but specifically a code defect in
`golang/node_agent/node_agent_server/grpc_workflow.go runInstallPackage`, not
a published-tarball, manifest, or installed-binary identity problem.)

## Recommended Action

* **No republish required for any of the 7 services.** The published tarballs
  and their manifests are internally consistent.
* Fix the node-agent code defect by extracting the two hash schemas into
  separate variables and never aliasing them. Encoded in
  `extractRunInstallPackageHashes`. Tests pin the contract.
* After the fix, re-dispatch will let the verify gate compare the
  on-disk binary against the actual `entrypoint_checksum`, which already
  matches, and the install will transition to verified `installed`.
* The `cluster-controller` `failing_on` status is the same root cause and will
  clear once the new node-agent binary is running.

## Risk

* **None for the published artifacts.**
* The fix touches only the node-agent's input extraction; the verify gate
  itself is unchanged. The verify gate already does the right thing when given
  the correct value.

## Awareness Records Added

* failure_mode: `node_agent.install_package_aliases_convergence_hash_into_expected_sha256`
* invariant: `install_package.hash_schemas_must_not_alias`

## Forbidden Followups

* Do not republish to "fix" hashes that are already correct.
* Do not weaken or downgrade `verifyInstalledBinaryHashStrict`.
* Do not edit installed_state to verified to silence the symptom.
* Do not reintroduce a fallback that aliases the two schemas back together.
* Do not "normalise" `ComputeReleaseDesiredHash` to return
  `entrypoint_checksum` so the alias works again.
