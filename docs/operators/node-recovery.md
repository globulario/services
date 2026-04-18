# Node Full-Reseed Recovery

The `node.recover.full_reseed` workflow is the last resort for nodes that cannot be repaired in place — disk corruption, hardware replacement, accidental wipe, or any situation where the node's filesystem cannot be trusted and must be rebuilt from scratch.

This document explains when to use it, exactly how it works, what every phase does, and the complete CLI reference. Read it before you run it.

---

## When to use full-reseed vs. repair

Globular has two levels of node recovery:

| Situation | Right tool |
|-----------|-----------|
| Services crashed or stopped, node agent is alive | `globular doctor heal` — auto-repair |
| Specific artifacts corrupt or missing, OS is healthy | `globular node repair` — targeted reinstall |
| **Disk data untrustworthy, hardware replaced, OS reinstalled** | **`globular node recover full-reseed`** |
| Node unreachable but expected to return | Wait for reconnection + auto-repair |

**Full-reseed destroys the node's entire state and rebuilds it from the artifact snapshot.** Do not reach for it for soft failures. The normal repair workflow (`node.repair`) handles the vast majority of situations — version drift, single corrupt packages, partial updates — without any destructive steps.

---

## Safety invariants

The following rules are enforced by the system and cannot be bypassed (except where `--force` is noted):

| Rule | What it means |
|------|---------------|
| **A — Snapshot before destruction** | A full inventory snapshot must be captured and committed to etcd before the workflow crosses the destructive boundary (wipe). The wipe step will not proceed without a valid snapshot on record. |
| **B — Exact build replay by default** | Each artifact is reinstalled with its exact `build_id` from the snapshot. If `--exact-replay` is set and any artifact lacks a `build_id`, the plan is rejected before the workflow starts. |
| **C — Reconciler fencing** | Once the recovery workflow owns a node, the normal reconciler skips it entirely. No parallel state changes are allowed until the workflow completes. |
| **D — Deterministic install order** | Artifacts are installed in a fixed bootstrap class order: Foundation → Core Control Plane → Supporting Infrastructure → Workload. Within each class, the order is stable and lexical. Cycles in the dependency graph are detected and rejected before the first install step. |
| **E — Verification required** | The workflow does not unfence the node or complete successfully unless every artifact passes build_id verification and the node's runtime health confirms services are running. |
| **F — Human gate at reprovision** | The workflow pauses after fencing and waits for an explicit operator acknowledgment (`globular node recover ack-reprovision`) before proceeding to reseed. No automatic wipe trigger exists. |
| **No automatic rollback** | If the workflow fails after the destructive boundary, the node stays fenced. Manual intervention is required. There is no automatic fallback to a previous state. |

---

## Workflow phases

The workflow progresses through 11 phases:

```
PRECHECK
    ↓
SNAPSHOT          (capture or adopt existing snapshot)
    ↓
FENCE_NODE        (pause reconciler, record fencing)
    ↓
DRAIN             (drain in-flight connections)
    ↓
AWAIT_REPROVISION (wait for human ACK — node is wiped here)
    ↓
AWAIT_REJOIN      (wait for node agent to reconnect)
    ↓
RESEED_ARTIFACTS  (install artifacts in bootstrap order)
    ↓
VERIFY_ARTIFACTS  (check build_id, checksum per artifact)
    ↓
VERIFY_RUNTIME    (check systemd unit health)
    ↓
UNFENCE_NODE      (resume reconciler)
    ↓
COMPLETE
```

On any failure: → `FAILED` (fencing kept if destructive boundary was crossed)

### Phase details

**PRECHECK** — Validates inputs: node exists, not already in recovery, reason and node_id provided. Calls `recoveryValidateRequest` and `recoveryCheckClusterSafety`.

**SNAPSHOT** — If no `--snapshot-id` was supplied, captures a live inventory snapshot by reading all installed packages from the node agent (SERVICE, INFRASTRUCTURE, COMMAND, APPLICATION kinds). The snapshot is JSON-serialized and written to etcd at `/globular/recovery/nodes/<node_id>/snapshots/<snapshot_id>`. Its content hash is computed and stored for integrity verification.

If a `--snapshot-id` was supplied (pre-captured), the workflow adopts and validates the existing snapshot instead. This is useful for pre-maintenance captures.

**FENCE_NODE** — Sets `ReconciliationPaused = true` on the `NodeRecoveryState` record in etcd. The normal reconciler reads this flag at the start of every node loop and skips fenced nodes entirely.

**DRAIN** — Marks the node's bootstrap phase as `recovery_drain`. This prevents any new connections from being established to services on this node. Existing connections time out naturally.

**AWAIT_REPROVISION** — The workflow polls every 10 seconds (up to 24 hours) waiting for an operator to call `globular node recover ack-reprovision`. This is the human gate where you physically wipe the node and reinstall the OS. Until you call ack-reprovision, nothing happens.

> **This is your last chance to abort.** The workflow does not touch the node's hardware. The wipe is your action, not the workflow's. Calling `ack-reprovision` signals "I have done the wipe and the OS is fresh."

**AWAIT_REJOIN** — After the ACK, the workflow waits for the node agent to reconnect to the controller (up to 1 hour, polling every 10 seconds). The node agent must complete its bootstrap and report a healthy heartbeat.

**RESEED_ARTIFACTS** — Installs all artifacts from the snapshot in deterministic bootstrap order. For each artifact:

1. Checks if the artifact already has a `VERIFIED` result in etcd — if so, skips it (resume idempotence)
2. Calls `remoteApplyPackageRelease` on the node agent with the exact `build_id` from the snapshot
3. Records the result (INSTALLED or FAILED) to etcd

This step is resumable: if the workflow restarts mid-reseed, already-verified artifacts are not reinstalled.

**VERIFY_ARTIFACTS** — For each installed artifact, compares the `build_id` reported by the node agent against the `build_id` in the snapshot. In exact-replay mode (`--exact-replay`), a mismatch fails the workflow. Artifacts that pass are promoted to `VERIFIED` status.

**VERIFY_RUNTIME** — Checks that:
- The node's bootstrap phase is no longer `recovery_drain`
- No partial_apply or FAILED packages remain in the installed state
- The node's `applied_services_hash` is non-empty (convergence hash was written)

**UNFENCE_NODE** — Sets `ReconciliationPaused = false`. The normal reconciler resumes ownership of the node. Sets the node's bootstrap phase to `workload_ready`.

**COMPLETE** — Writes the final `NodeRecoveryState` with phase=COMPLETE, emits the `node.recovery.complete` cluster event, and the workflow terminates successfully.

---

## Artifact install order

Artifacts are installed in this fixed bootstrap class order:

| Class | Artifacts |
|-------|-----------|
| **BOOTSTRAP_FOUNDATION** (0) | `etcd`, `scylladb`, `minio` |
| **BOOTSTRAP_CORE_CONTROL** (1) | `authentication`, `rbac`, `resource`, `discovery`, `dns`, `repository`, `workflow`, `cluster-controller`, `node-agent` |
| **BOOTSTRAP_SUPPORTING** (2) | `monitoring`, `event`, `envoy`, `xds`, `log`, `keepalived`, and any artifact with kind=INFRASTRUCTURE not named above |
| **BOOTSTRAP_WORKLOAD** (3) | All other services, applications, and commands |

Within each class the tiebreaker is: kind rank (INFRASTRUCTURE → SERVICE → APPLICATION → COMMAND), then the explicit `priority` field, then lexical order by `kind/publisher/name/version/build_id`.

This order is computed deterministically. Two runs with the same snapshot produce the same order every time.

---

## Cluster safety checks

Before dispatching the workflow the controller runs cluster safety checks:

- **Storage quorum**: If the node has the `storage` profile and removing it would drop below 3 storage nodes (MinIO / ScyllaDB quorum), the request is rejected with an error listing the risk. Override with `--force`.
- **Control-plane quorum**: If the node has the `control-plane` profile and removing it would leave fewer than 2 control-plane nodes, a warning is added to the response (not a hard block, since the controller itself is running).
- **Active recovery**: If the node is already under an active recovery workflow, the request is rejected.

`--force` skips the quorum safety check. Use it only when you have already verified the impact (e.g., you are recovering the last storage node and have taken a backup).

---

## CLI reference

### `globular node recover full-reseed`

Start a full-reseed recovery workflow.

```bash
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "<human-readable reason>"
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--node-id` | (required) | The node ID to recover |
| `--reason` | (required) | Human-readable reason — stored in the audit trail |
| `--exact-replay` | `false` | Require every artifact to have a `build_id`; fail the plan if any is missing |
| `--force` | `false` | Skip cluster safety checks (quorum, storage nodes) |
| `--dry-run` | `false` | Plan the artifacts and show the install order without dispatching the workflow |
| `--snapshot-id` | (none) | Use an existing pre-captured snapshot instead of capturing a new one |
| `--note` | (none) | Optional note appended to the audit event |
| `--json` | `false` | Output response in JSON |

**Dry-run example** (always run this first):

```bash
globular node recover full-reseed \
  --node-id abc123 \
  --reason "disk corruption on /var" \
  --dry-run
```

Output shows:
- Planned artifacts in install order
- Each artifact's source (`SNAPSHOT_EXACT` if it has a `build_id`, `REPOSITORY_RESOLVED` if not)
- Any cluster safety warnings
- No workflow is dispatched

**Live run:**

```bash
globular node recover full-reseed \
  --node-id abc123 \
  --reason "disk corruption on /var"
```

Output:

```
state:       DISPATCHED
workflow_id: recovery:abc123:1713456789000000000
snapshot_id: snap-7f3a2b1c

planned artifacts (14):
   0. INFRASTRUCTURE etcd                           3.5.15  [bld-001] (SNAPSHOT_EXACT)
   1. INFRASTRUCTURE scylladb                       5.4.2   [bld-002] (SNAPSHOT_EXACT)
   2. INFRASTRUCTURE minio                          7.0.11  [bld-003] (SNAPSHOT_EXACT)
   3. SERVICE        authentication                 2.1.0   [bld-004] (SNAPSHOT_EXACT)
   ...

Workflow dispatched. To monitor progress:
  globular node recover status --node-id abc123

When the node is wiped and OS is reinstalled, acknowledge with:
  globular node recover ack-reprovision --node-id abc123 --workflow-id recovery:abc123:...
```

---

### `globular node recover status`

Show the current recovery state for a node.

```bash
globular node recover status --node-id <node-id>
```

**Flags:** `--node-id` (required), `--json`

**Example output:**

```
node_id:      abc123
phase:        AWAIT_REPROVISION
mode:         ALLOW_RESOLUTION_FALLBACK
workflow_id:  recovery:abc123:1713456789000000000
snapshot_id:  snap-7f3a2b1c
reason:       disk corruption on /var
fenced:       true
destructive:  false
started:      2026-04-18T14:00:00Z (12m ago)
updated:      2026-04-18T14:01:30Z (10m30s ago)

snapshot: snap-7f3a2b1c (14 artifacts, captured 2026-04-18T13:59:45Z)

artifacts: 14 total — 0 verified, 0 failed, 14 pending
```

During reseed you will see the artifact count update in real time:

```
phase:   RESEED_ARTIFACTS
...
artifacts: 14 total — 7 verified, 0 failed, 7 pending
```

---

### `globular node recover ack-reprovision`

Signal that the node has been wiped and the OS reinstalled. The workflow is paused at `AWAIT_REPROVISION` until this is called.

```bash
globular node recover ack-reprovision \
  --node-id <node-id> \
  --workflow-id <workflow-id> \
  --note "Reinstalled Ubuntu 22.04, fresh disk"
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--node-id` | (required) | The node ID |
| `--workflow-id` | (required) | The workflow run ID (from the dispatch output or `recover status`) |
| `--note` | (none) | Optional note for the audit trail |

> **Only call this after you have confirmed**: the node's disks are wiped, a fresh OS is installed, and the Globular node-agent package is present and will start on boot. The workflow immediately begins waiting for the node to reconnect — if the OS install is incomplete, the `AWAIT_REJOIN` phase will time out.

---

### `globular node snapshot create`

Capture a pre-maintenance snapshot without starting a recovery workflow. Useful for creating a restore point before risky operations.

```bash
globular node snapshot create \
  --node-id <node-id> \
  --reason "pre-upgrade baseline before kernel update"
```

**Flags:** `--node-id` (required), `--reason`, `--json`

**Example output:**

```
snapshot_id: snap-7f3a2b1c
node_id:     abc123
artifacts:   14
captured_at: 2026-04-18T13:59:45Z
reason:      pre-upgrade baseline before kernel update
hash:        a3f92c1d

To use this snapshot for recovery:
  globular node recover full-reseed --node-id abc123 --reason "..." --snapshot-id snap-7f3a2b1c
```

---

### `globular node snapshot show`

Display the snapshot attached to a node's current or most recent recovery.

```bash
globular node snapshot show --node-id <node-id>
```

**Flags:** `--node-id` (required), `--json`

---

## Step-by-step: complete recovery procedure

This is the full sequence from diagnosis to healthy node.

### Step 1: Confirm this is the right tool

```bash
# Check what the doctor sees
globular cluster get-doctor-report

# Check node current state
globular cluster get-node-full-status --node-id <node-id>

# Can the node agent be reached?
globular node logs --unit globular-node-agent --node <node>:11000 --lines 20
```

If the node agent is reachable and the OS is intact, use `globular doctor heal` or targeted repair first. Full-reseed is for situations where you cannot trust the existing OS.

### Step 2: Assess cluster safety

```bash
# How many storage nodes do we have?
globular cluster list-nodes

# A 3-node cluster with all nodes having storage profile:
# - Recovering one node is safe (2/3 remain)
# - Recovering two simultaneously is NOT safe (quorum lost)
```

### Step 3: (Optional) Pre-capture a snapshot

If the node is still running, capture a snapshot before doing anything destructive:

```bash
globular node snapshot create \
  --node-id <node-id> \
  --reason "pre-recovery baseline $(date -u +%Y-%m-%dT%H:%M:%SZ)"
# Save the snapshot_id from the output
```

### Step 4: Dry-run first

```bash
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "<your reason>" \
  --dry-run
```

Review the planned artifact list. Verify:
- All critical infrastructure is in `SNAPSHOT_EXACT` source (has a `build_id`)
- The install order looks correct
- No unexpected warnings

### Step 5: Dispatch the workflow

```bash
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "<your reason>"
  # Add --snapshot-id <id> if you pre-captured in step 3
  # Add --exact-replay if you require exact build_id match for all artifacts
```

Save the `workflow_id` from the output.

### Step 6: Monitor fencing

```bash
globular node recover status --node-id <node-id>
# Should show phase: AWAIT_REPROVISION, fenced: true
```

The reconciler is now paused for this node. Other nodes continue operating normally.

### Step 7: Wipe and reprovision the node

At this point you perform the physical actions:

1. Power off or wipe the node's primary disk
2. Boot from installation media (or PXE)
3. Install the OS
4. Install the Globular node-agent package
5. Configure the node agent to point at the cluster (controller address, join token if needed — or leave it with the existing cluster config if it was preserved on a separate disk)
6. Start the node agent: `sudo systemctl start globular-node-agent`

### Step 8: Acknowledge the reprovision

```bash
globular node recover ack-reprovision \
  --node-id <node-id> \
  --workflow-id <workflow-id> \
  --note "Reinstalled Ubuntu 22.04 on fresh NVMe"
```

The workflow immediately begins polling for the node to reconnect (AWAIT_REJOIN).

### Step 9: Monitor reseed progress

```bash
# Watch recovery status
watch -n 5 globular node recover status --node-id <node-id>
```

You will see the phase progress from AWAIT_REJOIN → RESEED_ARTIFACTS → VERIFY_ARTIFACTS → VERIFY_RUNTIME → COMPLETE.

During RESEED_ARTIFACTS the artifact count updates:

```
artifacts: 14 total — 5 verified, 0 failed, 9 pending
```

### Step 10: Verify completion

```bash
# Recovery is complete
globular node recover status --node-id <node-id>
# phase: COMPLETE, fenced: false

# Node is healthy
globular cluster health

# No outstanding issues
globular cluster get-doctor-report

# All services converged
globular cluster get-node-full-status --node-id <node-id>
```

---

## What happens if the workflow fails

### Before the destructive boundary (before AWAIT_REPROVISION)

The node was never touched. The recovery state is cleaned up. The reconciler resumes normally. You can correct the problem and start a new recovery.

### After the destructive boundary (after ACK)

The node is fenced and stays fenced. The `NodeRecoveryState` will show:

```
phase:       FAILED
fenced:      true
destructive: true
```

The reconciler stays paused because the node is in an unknown partial state. You must manually intervene:

**If some artifacts installed but not all:**

Check which artifacts failed:
```bash
globular node recover status --node-id <node-id> --json
# Look at results[] for Status=FAILED and Error fields
```

You can restart a new recovery workflow using the same snapshot, which will skip already-verified artifacts:
```bash
globular node recover full-reseed \
  --node-id <node-id> \
  --reason "resume after partial failure" \
  --snapshot-id <same-snapshot-id>
```

**If the node failed to rejoin at all:**

The node agent did not reconnect within 1 hour. Common causes:
- OS install is incomplete
- Node agent configuration is wrong
- Network connectivity issue

Fix the node agent, wait for it to reconnect, then check if AWAIT_REJOIN can be retried (start a new recovery with `--snapshot-id`).

**If you need to unfence the node manually (emergency):**

```bash
# WARNING: Only do this if you understand the current state of the node.
# The reconciler will immediately attempt to converge the node.
globular etcd put /globular/recovery/nodes/<node-id>/state '{"phase":"COMPLETE","reconciliation_paused":false}'
```

---

## Snapshots in detail

### What a snapshot contains

A snapshot captures the complete installed artifact inventory from the node agent at a point in time:

- For each installed artifact: `name`, `kind`, `version`, `build_id`, `checksum`, `publisher_id`, `requires`
- Node identity: `node_id`, `hostname`, `profiles`, `profile_fingerprint`
- Metadata: `snapshot_id`, `created_at`, `created_by`, `reason`, `snapshot_hash`

The `snapshot_hash` is a SHA-256 over all artifact entries (sorted by kind+name+version+build_id) encoded as hex. It is verified at workflow start to detect tampering or corruption.

### Exact replay vs. resolution fallback

**Exact replay** (default when `--exact-replay` is specified): Every artifact must have a `build_id`. The artifact is fetched from the repository by its exact `build_id`. If the repository no longer has that build (it was GC'd), the step fails.

**Resolution fallback** (default without `--exact-replay`): Artifacts without a `build_id` are resolved to the latest stable version of that name/kind/publisher at install time. The installed version may differ from the snapshot version.

Choose exact replay when you need byte-for-byte fidelity. Choose resolution fallback when some artifacts are old and may no longer be available in the repository.

### Snapshot retention

Snapshots are stored in etcd and are not automatically deleted. They persist until the node is removed from the cluster. Use `globular etcd get /globular/recovery/nodes/<node_id>/snapshots/` to list them directly if needed.

---

## etcd key schema

Recovery state is stored under these keys:

```
/globular/recovery/nodes/<node_id>/state
    NodeRecoveryState JSON — phase, fencing flag, workflow ID, snapshot ID

/globular/recovery/nodes/<node_id>/snapshots/<snapshot_id>
    NodeRecoverySnapshot JSON — full artifact inventory

/globular/recovery/nodes/<node_id>/artifacts/<name>
    NodeRecoveryArtifactResult JSON — per-artifact install + verification result
```

You can inspect these directly:

```bash
# Current recovery state
globular etcd get /globular/recovery/nodes/<node_id>/state

# Snapshot contents
globular etcd get /globular/recovery/nodes/<node_id>/snapshots/<snapshot_id>

# Per-artifact results
globular etcd get /globular/recovery/nodes/<node_id>/artifacts/authentication
```

---

## Frequently asked questions

**Can I run full-reseed while the node is still running?**

Yes. The workflow fences the node and drains it before the destructive boundary. The node remains running until you physically wipe it at step 7. Running services will continue until you take them down.

**Can I cancel a recovery workflow?**

Not directly through the recovery CLI. If the workflow is in AWAIT_REPROVISION (before you have called ack-reprovision), you can simply not call ack-reprovision. The workflow will eventually time out (24 hours) and move to FAILED. If you need to cancel sooner, call the workflow service directly via the MCP tool `workflow_get_service_status` and cancel the run.

After cancellation (FAILED state), if the node was never touched (no ack-reprovision), you can clear the recovery state and the reconciler will resume:
```bash
globular etcd delete /globular/recovery/nodes/<node_id>/state
```

**What if I don't have the workflow_id for ack-reprovision?**

```bash
globular node recover status --node-id <node-id>
# workflow_id is in the output
```

**Can I use a snapshot from a different node?**

No. The snapshot contains the `node_id` and the workflow validates that the snapshot's `node_id` matches the recovery target. Cross-node snapshot reuse is not supported — it would reinstall one node's software onto a different node, breaking identity.

**Does this fix etcd membership?**

No. The recovery workflow reinstalls software artifacts. It does not manage etcd cluster membership. If the recovering node was an etcd member, it will rejoin the etcd cluster through the normal etcd bootstrap sequence (the `etcd` artifact install triggers this). Verify etcd membership with `globular cluster health` after completion.

**What if MinIO / ScyllaDB don't have a copy of the exact build?**

If `--exact-replay` is set and the repository no longer has the exact build, the reseed step for that artifact will fail. Use `--dry-run` to identify which artifacts are `REPOSITORY_RESOLVED` (no `build_id`) before committing to a recovery, and verify they still exist:

```bash
globular repository get-artifact-versions <publisher>/<kind>/<name>
```

If a version is missing and you have a backup of the repository (MinIO), restore it first.

---

## See also

- [Failure Scenarios and Recovery](failure-scenarios.md) — General failure catalog with less severe scenarios first
- [Node Repair (lighter-weight)](workflows.md) — `node.repair` for targeted artifact fixes without a wipe
- [Backup and Restore](backup-and-restore.md) — Restore repository (MinIO), cluster state (etcd), and application data (ScyllaDB) if needed before recovery
- [Cluster Doctor](cluster-doctor.md) — Continuous invariant checking; use this to confirm the node is healthy after recovery
- [High Availability](high-availability.md) — Quorum requirements for etcd, ScyllaDB, MinIO that the safety checks enforce
- [Convergence Model](convergence-model.md) — The 4-layer truth model underpinning all recovery decisions
