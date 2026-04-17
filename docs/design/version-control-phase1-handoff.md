# Globular Version Control — Phase 1 Rollout Handoff

**Date:** 2026-04-16
**Status:** Validated and live on all 3 nodes (ryzen, nuc, dell)

---

## 1. What Phase 1 Changed

Three changes, all in service of one rule: **published artifacts are immutable**.

- **Overwrite protection:** The repository now rejects uploads that attempt to replace a PUBLISHED artifact with different content. Returns `AlreadyExists` with both checksums in the error message.

- **Desired state validation:** The controller now verifies that the referenced artifact exists in the repository before writing desired state to etcd. Returns `NotFound` if the artifact doesn't exist, `Unavailable` if the repository can't be reached. Fail closed — no write without verification.

- **No default force-publish:** `globular deploy` no longer passes `--force` to the publish subprocess. Overwrites require explicit operator action (`globular pkg publish --force`).

**Files changed:**
- `repository/repository_server/artifact_handlers.go` — added `isTerminalState()`, modified overwrite check
- `cluster_controller/cluster_controller_server/desired_state_handlers.go` — added `validateArtifactInRepo()`, called from `upsertOne`
- `deploy/deploy.go` — removed `"--force"` from publish args

---

## 2. What Was Validated

All 6 Tier 1 tests passed on the live 3-node cluster (2026-04-16):

| Test | What was proven |
|------|----------------|
| Non-existent desired set (echo@9.9.9) | Rejected with `NotFound`, etcd unchanged |
| Non-existent desired set (authentication@99.99.99) | Rejected with `NotFound` |
| Valid desired set (echo@0.0.8) | Accepted, desired state updated |
| Idempotent re-publish (same .tgz) | Success, digest unchanged, no duplicate |
| Overwrite attempt (different binary, same version+build) | Rejected with `AlreadyExists`, original artifact unchanged |
| Repeated overwrite attempt | Same rejection, consistent behavior |

---

## 3. Known Operational Implication

**Services running versions not in the repository cannot be re-affirmed via `desired set`.**

Several services on the cluster are installed at version `0.0.7` which was manually installed without going through the repository publish pipeline. Those versions don't exist as published artifacts. If you run:

```
globular services desired set <service> 0.0.7
```

It will be rejected with `NotFound`.

**What to do instead:**
1. Build and publish the service at a new version higher than what's installed (e.g., `0.0.8`)
2. Then set desired to that version
3. The reconciler will roll out the published artifact to all nodes

This is not a bug — it's Phase 1 preventing exactly the situation that caused the stuck-reconciler incident.

---

## 4. What to Monitor (next 24 hours)

**Repository logs:**
```bash
journalctl -u globular-repository.service --since "1 hour ago" | grep -E "AlreadyExists|forbidden"
```
Expected: empty or rare. If frequent, someone is trying to overwrite published artifacts.

**Controller logs:**
```bash
journalctl -u globular-cluster-controller.service --since "1 hour ago" | grep -E "not found in repository|repository unreachable"
```
Expected: empty during normal operation. If present, either the repository is down or desired state references a missing artifact.

**Deploy failures:**
After any `globular deploy`, check for `AlreadyExists` errors. If a deploy fails:
- Re-run it (gets next build number automatically)
- Or check if `NextBuildNumber` returned a stale value

**Unexpected behavior to escalate:**
- `NotFound` for an artifact you just published → repository resolution bug
- `AlreadyExists` on every deploy attempt → build number allocation broken
- Controller crash loop → check `validateArtifactInRepo` in logs

---

## 5. Rollback Triggers

Revert Phase 1 if any of these occur:

| Condition | Severity | Action |
|-----------|----------|--------|
| Valid deploy fails repeatedly with `AlreadyExists` despite incrementing build numbers | Critical | Rollback, investigate build number resolution |
| Valid `desired set` for a known-published artifact returns `NotFound` | Critical | Rollback, investigate `GetArtifactManifest` resolution |
| Controller enters crash loop after restart | Critical | Rollback, check `validateArtifactInRepo` for nil pointer or import issue |
| Repository rejects idempotent re-uploads (same checksum returns error) | High | Rollback, idempotent path broken |

**Rollback command:**
```bash
cd /home/dave/Documents/github.com/globulario/services
git stash   # save Phase 1 changes
git checkout -- golang/repository/repository_server/artifact_handlers.go
git checkout -- golang/deploy/deploy.go
git checkout -- golang/cluster_controller/cluster_controller_server/desired_state_handlers.go
# Rebuild and redeploy to all 3 nodes
```

---

## 6. Operator Dos and Don'ts

**Do:**
- Publish artifacts to the repository before setting desired state
- Use `globular deploy` for normal deployments (auto-increments build number)
- Use `globular pkg publish --force` only when you consciously intend to replace an artifact
- Check `globular pkg info <service>` to see what versions exist in the repo

**Don't:**
- Don't restore `--force` in `deploy.go`
- Don't write desired state via direct `etcdctl put` — this bypasses the validation guard
- Don't assume a version exists in the repo just because nodes have it installed
- Don't ignore `AlreadyExists` errors — they mean something tried to overwrite sealed content

---

## 7. Next Engineering Step

**Immediate (this week):** Clean up repository hygiene. The cluster has services at `0.0.7` on nodes that don't exist in the repo. For each affected service, either:
- Publish the current binary at `0.0.8` (same as we did for the 4 services and echo)
- Or rebuild from source and publish

This unblocks `desired set` for all services and eliminates the legacy version gap.

**After cleanup:** Phase 2 — repository-issued `build_id` (UUIDv7) as the sole authoritative artifact identity. This eliminates build number races and enables exact artifact matching in convergence. The design is documented in `docs/design/version-control-redesign-v2.md`.
