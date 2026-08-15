# libnss-resolve removal — deferred runtime activation

**Status:** code fix proven and committed; runtime deployment intentionally deferred to the next
coordinated release. The cluster was deliberately left untouched.

Do not read this as unfinished work. The defect is solved and proven. What is deferred is only
making the *running* state reflect the commit.

---

## Decision

Deferred on 2026-08-14, with the cluster HEALTHY 5/5 and no user-visible defect.

```
cluster health        HEALTHY 5/5
user-visible defect   none
runtime DNS defect    none
removal proven        yes (8.15 + 8.17 witnesses, artifact assertion)
deployment scope      4 control-plane/runtime binaries x 5 nodes
executor self-update  yes (node-agent is one of the four)
```

Durability hygiene does not justify making a healthy cluster absorb a multi-service control-plane
rollout — least of all one that replaces the executor performing it. The change is still valuable:
it makes the removal durable, because the controller regenerates desired state from its compiled-in
catalog and would otherwise reconstruct `libnss-resolve` on the next reconcile. Editing the desired
record alone would be undone.

---

## Why the scope is four binaries

Removing one bundled `.deb` requires rebuilding four executables, because the package catalog is
**replicated as compiled policy** rather than read from a single runtime authority. This may be
intentional, but it is a standing release-coordination property: catalog and kind changes are never
single-service deploys.

| binary | version at decision | why affected |
|---|---|---|
| cluster-controller | 1.2.316 | `component_catalog.go` + `golang/component_catalog` + `golang/packagekind` |
| node-agent | 1.2.317 | `golang/component_catalog` + `golang/packagekind` |
| cluster-doctor | 1.2.317 | `golang/component_catalog` |
| repository | 1.2.312 | `golang/component_catalog` + `golang/packagekind` |

### Why not controller-only

Controller-only looks safer because it moves one binary, but it deliberately creates a window where
four consumers of the same semantics disagree:

```
controller  new semantics
node-agent  old semantics
doctor      old semantics
repository  old semantics
```

That is a legitimate compatibility experiment if designed on purpose. It is not something to create
accidentally in order to shrink a deployment.

---

## Rollout order for the release window

Control truth and observation tooling first; the executor last, because node-agent is the mechanism
performing the rollout.

```
repository
    -> verify
cluster-doctor
    -> verify
cluster-controller
    -> verify desired state drops libnss-resolve
node-agent LAST
    -> verify one node
    -> roll remaining nodes sequentially
    -> full convergence proof
```

Per `docs/operational-knowledge/deploy-package-via-mcp.md`, each service keeps its **current
version** and advances only `build_number` (+1), publisher `core@globular.io`. Never bump the version
for a redeploy.

---

## Pre-window items — both resolved

**1. Authentication.** The operator performs `globular auth login --user sa` interactively at deploy
time. The permanent credential must not enter an agent transcript or shell history; the rollout uses
the resulting short-lived credential.

**2. `repository_trusted_publishers` returned 0.** Investigated: **fail-open, not a blocker.** Zero
is the shipped steady state, not a misconfiguration.

- `signaturePolicyDecision` has six non-test call sites and none is the publish path — they are
  download, resolve, rollback, upstream-sync, and advisory findings. The only trusted-publisher call
  inside `UploadArtifact` is label-only (audit field, no branch, no error return).
- `defaultSignaturePolicy()` is deliberately permissive; strictness is opt-in.
- Live check: etcd key `/globular/repository/security/policy` is **absent**, so defaults apply. This
  mattered because of an asymmetry — a strict policy there would not have blocked publish, but would
  have blocked `DownloadArtifact`/resolve for `core@globular.io`, i.e. publish succeeds and *install*
  fails halfway through a staged rollout.

### The gates that can actually fail the rollout

Prepare against these instead. All in `UploadArtifact`:

1. **Release authority / RBAC** — a STABLE publish under the official publisher needs
   `release.allocate`, else `PermissionDenied`. A *non-official* publisher is silently downgraded to
   DEV instead, which is a much quieter way to get a wrong result.
2. **Stripped-binary law** — STABLE binaries must be built `-trimpath -ldflags "-s -w"`, plus a
   size-envelope check. Matches the runbook's ldflags instruction; do not skip `-s -w`.
3. **Version immutability** — re-publishing an existing version returns `AlreadyExists`. Bump
   `build_number`; never `--force`.
4. **Identity-lane rules** — a DEV-channel artifact may not carry a clean semver.

---

## What is already proven

| claim | evidence |
|---|---|
| full build clean under the new gates | 21 built, 0 skipped, 0 failed |
| release contains no libnss-resolve | artifact assertion PASS over 21 tarballs |
| that assertion can fail | 2 artifacts found in the pre-removal `dist/` (114 inspected) |
| resolution works with the module absent | Noble `release-20260518`, systemd 255.4-1ubuntu8.15, 7/0 |
| image provenance | sha256 matched a GPG-good `SHA256SUMS` |
| resolution works on current hosts | 6/0 on dell, lenovo, hp-01 |

nsswitch fall-through, measured with the module absent rather than inferred from the grammar:

```
files resolve dns                    -> RESOLVED   (glibc default continues)
files resolve [!UNAVAIL=return] dns  -> RESOLVED   (stock Ubuntu idiom)
files resolve [UNAVAIL=return] dns   -> FAILED     (blocks fall-through)
```

**Not proven:** no release was deployed, so the after-witnesses show behaviour is unchanged by the
*source* removal — not that a post-removal deployment converges. That is what the release window is
for.

---

## Related

- ai-memory `2152094d` — the deferral decision
- ai-memory `9541f634` — trusted-publishers investigation and the real publish gates
- ai-memory `5fc296d6` — why libnss-resolve is not a runtime prerequisite
- `scripts/release/verify-resolver-without-libnss.sh` — the removal gate, run on the target
- `scripts/release/check-no-bundled-libnss.sh` — artifact-level assertion
- `packages/fixtures/libnss-resolve/` — the deb retained as a permanent UNSAT regression fixture
