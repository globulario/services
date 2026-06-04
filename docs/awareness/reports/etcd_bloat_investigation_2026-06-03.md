# Etcd bloat investigation — globule-ryzen — 2026-06-03

**Status: code fixes landed (Phase 36 + publishWaveState dedup, 2026-06-04). Operational items (compact/defrag/disarm, auto-compaction config) still awaiting operator approval. See "Closure log" at the end.**

## TL;DR

A single etcd key — `/globular/clustercontroller/state` (334 KB value, rewritten on every reconcile tick) — accounts for **96 % of cumulative etcd write load** and is the dominant driver of post-compact MVCC bloat. After every compact+defrag, the DB grows by ~17 MB / minute from this one key's rewrites; NOSPACE (2 GB quota) reaches within minutes-to-hours.

Two secondary writers (`InfrastructureRelease/envoy` and `.../cluster-controller`) have extreme rewrite counts (~99 K and 88 K versions respectively) but their values are small (~1 KB), so they contribute ~1-2 % each.

## Snapshot paths

| | Path | Hash | Revision | Total Keys (MVCC) | Active Keys |
|---|---|---|---|---|---|
| Phase 32 cycle | `/tmp/etcd-snapshot-20260603T065456Z.db` | `447b1037` | 350,640 | 60,759 | (not measured) |
| Phase 35 incident | `/tmp/etcd-bloat-investigation-20260603T144103Z.db` | `f2d6a3d9` | 499,493 | 73,372 | **999** |

Active keys (999) vs MVCC revisions (73K) confirms that bloat is from rewrites, not new keys.

## Top prefixes by active key count

| Count | Prefix |
|---:|---|
| 378 | `/globular/audit/desired_writes` |
| 157 | `/globular/cluster_doctor/audit` |
| 94  | `/globular/ai/jobs` |
| 64  | `/globular/nodes/eb9a2dac-…` |
| 47  | `/globular/verification/runtime` |
| 34  | `/globular/convergence/actions` |
| 25  | `/globular/resources/ServiceRelease` |
| 25  | `/globular/resources/ServiceDesiredVersion` |
| 25  | `/globular/resources/InfrastructureRelease` |
| 19  | `/globular/convergence/nodes` |

None of these have large values — they sum to a few hundred KB.

## Top keys by `version × value_size` (cumulative write-load proxy)

| `version` | `value_bytes` | `load_estimate` | key |
|---:|---:|---:|---|
| **24,904** | **334,049** | **8,319,156,296** | **`/globular/clustercontroller/state`** |
| 99,121 | 1,181 | 117,061,901 | `/globular/resources/InfrastructureRelease/core@globular.io/envoy` |
| 88,155 | 1,029 | 90,711,495 | `/globular/resources/InfrastructureRelease/core@globular.io/cluster-controller` |
| 19,709 | 1,155 | 22,763,895 | `/globular/resources/InfrastructureRelease/core@globular.io/repository` |
| 15,267 | 931 | 14,213,577 | `/globular/resources/InfrastructureRelease/core@globular.io/scylladb` |
| 25,056 | 421 | 10,548,576 | `/globular/objectstore/config` |
| 5,020 | 1,183 | 5,938,660 | `/globular/resources/InfrastructureRelease/core@globular.io/keepalived` |
| 25,056 | 210 | 5,261,760 | `/globular/pki/ca` |
| 4,948 | 844 | 4,176,112 | `/globular/resources/InfrastructureRelease/core@globular.io/etcd` |
| 24,941 | 145 | 3,616,445 | `/globular/cluster/minio/config` |

**Total `Σ(version × value_size)` across all 999 active keys: 8,682,502,720 bytes.**
`/globular/clustercontroller/state` alone = **8,319,156,296 / 8,682,502,720 = 95.8 %**.

## Suspected writer ranked list

1. **`/globular/clustercontroller/state` writer** (rank #1, dominant cause)
   - File path: somewhere in `golang/cluster_controller/cluster_controller_server/` — likely a `persistState()` / `saveState()` function called on reconcile/state mutation.
   - Size: 334 KB value. Almost certainly serializes the entire in-memory cluster state (nodes, services, releases, plans) into one JSON/proto blob.
   - Rewrite frequency: ~2.5 / minute (= ~24 s cadence) — matches the controller's reconcile loop period.
   - Effect: every rewrite adds 334 KB of new MVCC revision data; compact removes it but the writer never pauses.

2. **`InfrastructureRelease/envoy` and `.../cluster-controller`** (rank #2-3)
   - Suspicious sub-pattern: versions of 99 K and 88 K are extreme. The other 10 InfrastructureRelease keys have versions of 25 K, 5 K, etc.
   - Envoy has been involved in two restart-storm incidents (Phase 24+27+28+29). The `status.last_transition_unix_ms` field probably gets rewritten on every reconcile cycle even when nothing semantic changed.
   - Values are small (~1 KB), so the bloat impact is bounded but still notable.

3. **`/globular/objectstore/config` / `/globular/pki/ca` / `/globular/cluster/minio/config`** (rank #4-6)
   - All at version ~25 K with small values. Likely periodic re-renders by the controller's config-reconciler.

## Mitigation recommendations

### Immediate (operator action required)

1. **Compact + defrag + disarm alarm** to unblock Phase 35 deploy.
   - Same recipe as before (well-trodden, safe; all 5 services kept healthy across last two cycles).
   - Buys ~40 min before NOSPACE recurs at current write rate.
   - All forensic evidence is captured in `/tmp/etcd-bloat-investigation-20260603T144103Z.db` + this report — compacting now does NOT destroy diagnostic data.

### Short-term (etcd-level mitigation, no code change)

2. **Enable etcd auto-compaction** so MVCC history never accumulates between manual ops:
   ```
   --auto-compaction-mode=periodic
   --auto-compaction-retention=1h
   ```
   Add to `/var/lib/globular/config/etcd.yaml`. Etcd will silently compact every hour, keeping MVCC bounded. Need separate phase to land this etcd config change + restart.

3. **Increase quota** from default 2 GB → 8 GB:
   ```
   --quota-backend-bytes=8589934592
   ```
   Buys more headroom but doesn't fix the root cause. Combine with auto-compaction.

### Long-term (code fix — proper resolution)

4. **Fix the `/globular/clustercontroller/state` writer** to not rewrite on every tick.
   - **Best:** split the 334 KB blob into smaller per-component keys (per-node, per-release, etc.) so unchanged parts don't need rewriting. Each reconcile would touch only the diffs.
   - **Acceptable:** compute a content hash before persisting; skip the write if the hash matches the last-persisted value.
   - **Worst:** lengthen the persist cadence (e.g., every 5 min instead of every 24 s). Reduces bloat 12× but doesn't address the fundamental "rewrite-entire-state" pattern.

5. **Investigate `InfrastructureRelease` rewrite churn.**
   - 99,121 versions on envoy's status record is extreme. Likely culprit: the controller updates `status.last_transition_unix_ms` on every reconcile cycle even when no transition actually occurred. Fix: only mutate status when an actual transition happens.

## Will compacting now preserve enough evidence?

**Yes.** The diagnostic snapshot file (`/tmp/etcd-bloat-investigation-20260603T144103Z.db`) is a complete byte-for-byte copy of the database at investigation time. It contains all 73,372 MVCC revisions and is queryable offline via `etcdctl snapshot status` and `etcdutl --data-dir` restore + inspect. The cumulative-version data captured in this report (24,904 writes for the dominant key) is the only number needed for the code fix.

## Recommended next actions (in order)

1. **Now**: operator approves compact+defrag+disarm — same recipe; unblocks Phase 35.
2. **Same session**: open Phase 36 (this code fix) — patch the `/globular/clustercontroller/state` writer with content-hash dedup.
3. **Concurrent**: open Phase 37 (etcd config) — enable `auto-compaction-mode=periodic` + raise quota to 8 GB.

## Exact next-command proposal (read-only confirmation)

After operator approval — recipe identical to the last compact cycle:
```bash
ETCDCTL="sudo /usr/lib/globular/bin/etcdctl --endpoints https://10.0.0.63:2379 [TLS-flags]"
REV=$($ETCDCTL endpoint status -w json | python3 -c '...print(d[0][\"Status\"][\"header\"][\"revision\"])')
$ETCDCTL compact $REV
$ETCDCTL defrag --command-timeout=600s
$ETCDCTL alarm disarm
```
Expected: DB 2.1 GB → ~1.4-2.0 GB; alarm cleared; writes accepted again.

## Closure log

- 2026-06-03 (commit `4b721e0f`) — **Phase 36 landed**: content-hash dedup on
  `/globular/clustercontroller/state` writes. Closes the rank-#1 contributor
  (95.8 % of cumulative write-load). Pinned by
  `state_persist_dedup_test.go`.
- 2026-06-04 (this session) — **publishWaveState dedup landed**: equality
  guard before `srv.resources.Apply` in
  `golang/cluster_controller/cluster_controller_server/workflow_release.go`.
  Closes the rank-#2/#3 contributors (`InfrastructureRelease/envoy` at 99K
  versions and `.../cluster-controller` at 88K versions). Pinned by
  `publish_wave_state_dedup_test.go`. New failure_mode anchor:
  `controller.release_status_writer_bypasses_equality_guard`.
- **Still open** (operational, not code):
  1. Operator-driven `etcdctl compact && defrag && alarm disarm` to reclaim
     space accumulated before the code fixes landed.
  2. Phase 37 etcd config: `--auto-compaction-mode=periodic
     --auto-compaction-retention=1h` and raise `--quota-backend-bytes` to
     8 GB in `/var/lib/globular/config/etcd.yaml`.
