---
id: design_decisions_index
type: documentation_section
summary: Index of authoritative architecture decisions for the Globular platform.
---

# Globular Architecture Design Decisions

This document records the authoritative design decisions that govern Globular's
architecture. Each entry is a named decision with a summary, related invariants,
failure modes, forbidden fixes, and tests. The awareness graph indexes these
decisions so AI agents can navigate from code to design rationale.

---

---
id: decision.desired_hash_is_convergence_identity
type: architecture_decision
status: accepted
summary: DesiredHash is the convergence identity used for InfrastructureRelease, workflow desired_hash, LocalHash, installed-state checksum, and convergence commit paths. Raw artifact digest is artifact identity and must not be substituted.
invariants:
  - infra.desired_hash_consistency
failure_modes:
  - infra.desired_hash_mismatch_restart_storm
symbols:
  - ComputeInfrastructureDesiredHash
  - lookupServiceReleaseBuildID
  - classifyPackageConvergence
forbidden_fixes:
  - use_raw_artifact_digest_as_desired_hash
tests:
  - TestDriftWorkflowUsesDesiredHash
  - TestInfrastructureDesiredHashConsistency
---

## DesiredHash Is Convergence Identity

DesiredHash is computed from the declared spec of an InfrastructureRelease — not
from the artifact blob digest. The convergence path (desired → installed) uses
DesiredHash as its identity anchor. LocalHash on the node-agent side is computed
from the same inputs and compared against DesiredHash to decide whether an install
is needed.

Raw artifact digest (SHA-256 of the blob) is the artifact storage identity used by
the repository layer. It must never be substituted for DesiredHash in convergence
logic. Doing so causes restart storms when digests differ due to re-packing, signing,
or storage normalization while the declared spec is unchanged.

**Forbidden fixes:**
- Using artifact digest as desired_hash
- Restarting a service to "fix" a hash mismatch without checking the spec

---

---
id: decision.runtime_observation_is_not_desired_authority
type: architecture_decision
status: accepted
summary: Runtime heartbeat, ManagedInstalled state, or service observation may report discovered state but must not create authoritative desired state by itself.
invariants:
  - infra.heartbeat_not_desired_authority
  - critical_state.registry_ownership_required
failure_modes:
  - infra.heartbeat_creates_spurious_desired_state
forbidden_fixes:
  - create_infra_release_from_heartbeat_only
  - set_desired_version_from_runtime_observation
---

## Runtime Observation Is Not Desired Authority

The node-agent heartbeat reports what is installed and running. It is the Layer 3
(Installed Observed) signal. It must never write to Layer 2 (Desired Release) unless
an explicit operator action has been approved and logged.

Similarly, the cluster controller must not infer desired state from the absence of
a heartbeat, from a service being healthy, or from any runtime observation. The
desired state is set by operators and upgrade pipelines — not auto-discovered from
runtime.

**Forbidden fixes:**
- Creating InfrastructureRelease records from heartbeat data alone
- Setting ServiceDesiredVersion from observed installed versions

---

---
id: decision.missing_state_is_not_delete_intent
type: architecture_decision
status: accepted
summary: Missing, invalid, or temporarily unavailable control-plane state must not imply stop, delete, disable, or destructive action.
invariants:
  - critical_state.absence_is_not_destructive_intent
  - critical_state.deletion_requires_audited_intent
failure_modes:
  - critical_state.absence_triggers_destructive_action
forbidden_fixes:
  - treat_missing_etcd_key_as_delete_intent
  - stop_service_on_config_load_failure
---

## Missing State Is Not Delete Intent

If etcd is temporarily unavailable, a key has been accidentally deleted, or a
config record fails to unmarshal, the system must not interpret that as a signal
to stop, uninstall, or delete anything. The correct response is to enter a safe
degraded mode and wait for state to be restored.

Absence of a record is ambiguous. Only an explicit tombstone or a signed operator
intent can trigger a destructive action. The cluster controller must require
audited intent before any destructive operation proceeds.

**Forbidden fixes:**
- Stopping a service because its config key is temporarily missing
- Treating a failed etcd GET as confirmation that no desired state exists

---

---
id: decision.local_success_is_not_global_convergence
type: architecture_decision
status: accepted
summary: A node completing local work does not prove global convergence. Installed-state, result promotion, and action cleanup must be durably committed.
invariants:
  - install.result.atomic_commit
  - convergence.no_infinite_retry
failure_modes:
  - convergence.partial_commit_leaves_ghost_state
forbidden_fixes:
  - assume_install_succeeded_without_etcd_confirmation
  - skip_result_write_on_local_success
tests:
  - TestInstallResultCommittedToEtcd
  - TestConvergenceNoInfiniteRetry
---

## Local Success Is Not Global Convergence

A node-agent completing an install step locally does not mean the cluster has
converged. The result must be written to etcd (Layer 3) before the reconciler
can observe convergence. If the result write fails, the reconciler will retry
the install on the next cycle — which is correct behavior.

Never assume a local success is visible globally. Never skip the result commit.
Never use in-memory state as a substitute for etcd confirmation.

**Forbidden fixes:**
- Skipping the installed-state write after a successful local install
- Using in-memory installed-state as the source of truth across restarts

---

---
id: decision.awareness_graph_is_compiled_context_not_authority
type: architecture_decision
status: accepted
summary: The awareness graph is a compiled semantic context map built from source, metadata, docs, runtime evidence, and learned scars. It informs AI and audits but is not the authority for Desired/Installed/Runtime state.
invariants:
  - awareness.annotation_scanner.production_source_only
failure_modes:
  - awareness.graph_used_as_state_authority
forbidden_fixes:
  - use_awareness_graph_as_desired_state_source
  - trust_graph_over_etcd_for_cluster_decisions
---

## Awareness Graph Is Compiled Context, Not Authority

The awareness graph connects source code, invariants, failure modes, design
decisions, runtime evidence, and learned scars into a queryable SQLite graph.
It is a *compiled semantic context map* — not an operational source of truth.

Cluster decisions (what to install, what to start, what version to run) come from
etcd. The awareness graph exists to help AI agents, operators, and auditors
understand the system — not to drive it.

If the graph is stale, corrupt, or missing, the cluster continues operating through
its deterministic convergence model. The graph is supplementary context.

**Forbidden fixes:**
- Reading desired state from the awareness graph instead of etcd
- Trusting graph edge presence as proof of runtime health
