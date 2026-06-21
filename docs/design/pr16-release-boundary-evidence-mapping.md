# PR-16 — Release Boundary Proof: Evidence Mapping & A4 Timestamp Finding

> Status: Phase 1 delivered (`golang/release_boundary/`, commit `d5cb8608`).
> This note freezes the Phase 2 evidence mapping and records the Phase 1.5
> investigation that pinned the A4 (restart-after-install) timestamp source.
> Read-only investigation — no installer/runtime behavior changed.

## Purpose

The pure verdict engine (`release_boundary.Evaluate`) consumes already-fetched
truth structs and proves, for one service binary, that the artifact the
repository published (by `build_id`) equals what is installed and what is
running, and that the process started after the artifact was installed. This
note pins **where each `Inputs` field comes from** so the Phase 2 MCP
aggregator can be wired without guessing field names, and resolves the one
load-bearing ambiguity: which install timestamp A4 must compare against.

## Frozen evidence mapping

Five owner RPCs per (service `S`, node `N`). All reads go through owner RPCs —
no direct etcd/Scylla/storage reads (`invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage`).

| Inputs field | Source RPC | Go access | Anchor |
|---|---|---|---|
| `DesiredBuildID` (B) | `GetDesiredState` (controller) | `svc.GetBuildId()` where `svc.GetServiceId()==S` | `cluster_controller.proto:1204` |
| `Manifest.BuildID` | `GetArtifactManifest` (repo) | `m.GetBuildId()` | `repository.pb.go:1148` |
| `Manifest.PublishState` | `GetArtifactManifest` | `m.GetPublishState() == PublishState_PUBLISHED` (=3) | enum `repository.pb.go:168` |
| `Manifest.EntrypointChecksum` (EC) | `GetArtifactManifest` | `m.GetEntrypointChecksum()` | `repository.pb.go:1160` |
| `Manifest.ProvenanceGitSHA` | `GetArtifactManifest` | `m.GetProvenance().GetBuildCommit()` | `repository.pb.go:1817` |
| `Repository.Present` | `VerifyArtifact` (repo) | `true` iff RPC returned (**not** `Installable`) | — |
| `Repository.Verified` | `VerifyArtifact` | `resp.GetStatus() == ARTIFACT_VERIFY_OK` (=1) | `repository.proto:938` |
| `Repository.Reason` | `VerifyArtifact` | `resp.GetReason()` | `repository.proto:969` |
| `Installed.BuildID` | `GetInstalledPackage` (node-agent) | `pkg.GetBuildId()` | `node_agent.proto:45` |
| `Installed.EntrypointChecksum` | `GetInstalledPackage` | `pkg.GetMetadata()["entrypoint_checksum"]` | writer `self_hosted_runtime_proof_writer.go:456` |
| `Installed.InstalledUnix` | `GetInstalledPackage` | **`pkg.GetMetadata()["installed_at"]`** (see A4 finding) | `installreceipt.go:204` |
| `Runtime.Running` | `GetServiceRuntimeProof` (node-agent) | `p.GetSystemdActiveState() == "active"` | proof `:30` |
| `Runtime.PID` | `GetServiceRuntimeProof` | `int(p.GetRunningPid())` | proof `:20` |
| `Runtime.RunningExeSHA256` | `GetServiceRuntimeProof` | `p.GetRunningExeSha256()` (PROOF, not the `installed_sha256` CLAIM) | proof `:22` |
| `Runtime.ProcessStartUnix` | `GetServiceRuntimeProof` | `p.GetProcessStartTime().AsTime().Unix()` | proof `:25` |
| `Unhashable` | `GetServiceRuntimeProof` / verifier | `!installedPathIsUpstream(p.GetInstalledPath())` (path outside `/usr/lib/globular/bin/` ⇒ wrapper) | `verifier.go:88` |

`DesiredService` carries `version`, `platform`, and `build_number` (field 4)
alongside `build_id` (field 6) — enough to build the `ArtifactRef` +
`build_number` for the two repository calls. `VerifyArtifactResponse` does
**not** carry the manifest, so both repo calls are required (verify ≠ manifest).

## A4 timestamp finding (the Phase 1.5 question)

**Question:** is ordinary-service `InstalledUnix` anchored to install
wall-clock (so A4 can prove `ProcessStartUnix > InstalledUnix`), or to
process-start (so A4 ties to INDETERMINATE forever, as it does for
self-hosted services)?

**Answer:** ordinary-service `InstalledPackage.InstalledUnix` **is install
wall-clock**, not process-start — but it is the **first-install** time,
*preserved* across upgrades, which makes it the **wrong** field for A4. The
correct field is the install receipt's `metadata["installed_at"]`.

Evidence:

| Field | Set where | Semantics | Heartbeat behavior |
|---|---|---|---|
| `InstalledPackage.InstalledUnix` | `package_state.go:111,116`; `installer_api.go:209` | `time.Now()` at **first** install; preserved on upgrade (`package_state.go:117-118`, `heartbeat.go:948`) | preserved |
| `InstalledPackage.UpdatedUnix` | `package_state.go:132`; `heartbeat.go:943` | `time.Now()` on every write | **bumped on every repo-sync** for ordinary services (only preserved for self-hosted, `heartbeat.go:966-967`) |
| `metadata["installed_at"]` | `installreceipt.Stamp` → `installreceipt.go:204`; ordinary path at `package_state.go:306` | `time.Now()` at **this build's** install-commit; re-stamped each upgrade | **preserved** (in `receiptKeys`, `installreceipt.go:90`; `Preserve` is NEXT-wins and heartbeat does not re-`Stamp` ordinary services) |

Why `InstalledUnix` is wrong for A4: it is preserved as the original
first-ever install. After an upgrade to build B (InstalledUnix unchanged), a
**stale** process that never restarted still satisfies
`ProcessStartUnix > InstalledUnix` → A4 would **false-PROVE** exactly the
`service.old_pid_after_upgrade` case it exists to catch.

Why `UpdatedUnix` is also wrong: for ordinary services it is bumped to
`time.Now()` on every repo-sync cycle, so it drifts forward past a legitimately
restarted process's start time → A4 would spuriously fail.

Why `metadata["installed_at"]` is correct: it is re-stamped at **each**
install/upgrade commit (so it tracks build B), it is **not** process-start
anchored, and it is **preserved across heartbeats** (unlike `UpdatedUnix`). For
build B installed at T_B with the process restarted at T_p:
`T_p > T_B` ⇒ PROVEN; a stale un-restarted process has `T_p < T_B` ⇒ correctly
FAILED.

## Decision

Ordinary-service install time **is** owner-sourced install wall-clock, available
today via `metadata["installed_at"]`. Per the Phase 1.5 decision rule:

> **Proceed to PR-16 Phase 2 (MCP aggregator), using an ordinary service as
> the first pilot target.**

Refinements to carry into Phase 2 (mapping above already reflects them):

1. **A4 sources `metadata["installed_at"]`, not `InstalledPackage.InstalledUnix`.**
   The pure engine's `Installed.InstalledUnix` field is fed from `installed_at`
   (parsed int64). If `installed_at` is absent, A4 returns INDETERMINATE — do
   **not** silently fall back to the first-install `InstalledUnix` (that would
   reintroduce the upgrade false-PROVE). No silent proof.
2. **Pilot must be an ordinary service**, not `repository`. For self-hosted
   services `InstalledUnix`/`UpdatedUnix` are PID-start-anchored by the proof
   writer; the earlier "pilot on repository" suggestion is reversed.
3. `Repository.Present` binds to "RPC returned", not `Installable`
   (`Installable` = OK-and-PUBLISHED, which is A1's concern, not A0's).
4. `ProcessStartTime` is a ns `Timestamp`; `.Unix()` truncates to seconds, so
   an install+restart inside the same second reads as a tie → conservative
   INDETERMINATE (matches the engine's A4 tie rule).

## Not done here (governance)

No MCP tool, CLI, Makefile target, doctor invariant, behavioral-memory
emission, client fan-out, installer change, or storage read. The pure engine
(`golang/release_boundary/`) is unchanged. No A4 "fix" was applied — this task
only proved the correct evidence exists. Phase 2 wiring touches `golang/mcp/`
(high-risk) and will run `awareness.briefing` first.
