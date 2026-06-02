package rules

// ──────────────────────────────────────────────────────────────────────────────
// Auto-heal Policy v1
//
// Defines which invariant findings can be repaired automatically, which
// require human approval, and which are observe-only.
//
// Design principles:
//   - Every auto-action is idempotent and bounded.
//   - Every auto-action is followed by a verification step.
//   - If verification fails, the healer stops and logs — never retries
//     blindly or escalates to a destructive action.
//   - Propose-only and observe-only findings are never mutated automatically.
// ──────────────────────────────────────────────────────────────────────────────

// HealDisposition describes what the healer may do with a finding.
type HealDisposition string

const (
	// HealAuto — the healer may execute the repair without human approval.
	// The repair must be idempotent, bounded, and verifiable.
	HealAuto HealDisposition = "auto"

	// HealPropose — the healer surfaces a concrete remediation plan but
	// does NOT execute it. Requires operator approval via CLI or UI.
	HealPropose HealDisposition = "propose"

	// HealObserve — the healer reports the finding but takes no action
	// and makes no recommendation. Informational only.
	HealObserve HealDisposition = "observe"
)

// HealRule maps an invariant ID to a disposition and a machine-readable
// description of the action. Rules are evaluated by the healer component
// after invariant evaluation.
type HealRule struct {
	InvariantID string
	Disposition HealDisposition
	Action      string // human-readable action description
	Rationale   string // why this disposition was chosen

	// AutoAction is the programmatic action key the healer executes
	// when Disposition == HealAuto. Empty for non-auto rules.
	AutoAction string
}

// PolicyV1 returns the full auto-heal policy. Each rule corresponds to
// one invariant ID that the doctor emits. Unknown invariant IDs default
// to HealObserve (safe fallback — never mutate what you don't understand).
func PolicyV1() []HealRule {
	return []HealRule{
		// ── A. Auto-heal (safe to execute automatically) ─────────────

		{
			// Milestone 2: demoted from HealAuto to HealPropose pending the
			// re-enable in Milestone 3 through the gated remediation path.
			// The action is intrinsically safe (cache deletion is reversible
			// — the next install re-fetches with digest verification) but
			// dispatch must traverse ExecuteRemediation, not RemoteOps. See
			// docs/design/auto-healing-path-unification-patch-c.md.
			InvariantID: "artifact.cache_digest_mismatch",
			Disposition: HealPropose,
			Action:      "Delete the stale cached artifact; the next install will re-fetch with digest verification. Operator-driven until Milestone 3 re-enables auto-dispatch through ExecuteRemediation.",
			Rationale:   "The cache is not a source of truth. Removing it forces a validated re-download on next install. Demoted to propose-only in Milestone 2 — the dispatch must go through ExecuteRemediation, not the legacy RemoteOps path.",
		},
		{
			InvariantID: "artifact.cache_missing",
			Disposition: HealAuto,
			Action:      "No repair needed — informational only. The next install will fetch and cache automatically.",
			Rationale:   "A missing cache is the natural state before first install or after a successful cache invalidation. The fetch layer handles this transparently.",
			AutoAction:  "", // no-op — auto-classified as resolved on next install
		},
		{
			// ServiceRelease stuck at RESOLVED when installed == desired.
			// The pipeline resolved the artifact but never transitioned to
			// AVAILABLE because the installed_state already matches. The
			// concrete repair would patch the phase to AVAILABLE, but that
			// requires a direct etcd.Put against
			// /globular/resources/ServiceRelease/<name>. Until the healer
			// loop is unified into ExecuteRemediation (Path A) — where
			// ETCD_PUT is hard-blocked by the action executor and only
			// reachable through evidence-trust + approval + cooldown +
			// failure-rate gates — this rule is propose-only.
			InvariantID: "release.stuck_resolved",
			Disposition: HealPropose,
			Action:      "Patch ServiceRelease phase from RESOLVED to AVAILABLE and clear the DriftUnresolved counter (operator-driven; routed through ExecuteRemediation once Path B is unified into the gated path).",
			Rationale:   "The patch is a direct etcd write to a ServiceRelease object — ETCD_PUT is hard-blocked from auto-execution by the action executor (executor.go hardBlocked()), and the background healer must not bypass that boundary. Propose only until release.stuck_resolved routes through ExecuteRemediation with the full gate set.",
		},
		{
			// Milestone 2: demoted from HealAuto to HealPropose. Clearing a
			// drift observation crosses to the workflow service and has no
			// clean RemediationAction form today — the existing
			// workflow.ClearDriftObservation RPC is not representable as an
			// existing ActionType. Stays propose-only until a workflow-safe
			// path lands (post-Milestone 3).
			InvariantID: "workflow.drift_stuck",
			Disposition: HealPropose,
			Action:      "Clear the DriftUnresolved observation via workflow.ClearDriftObservation after operator confirms installed == desired.",
			Rationale:   "DriftUnresolved is a telemetry counter, not a source of truth. The clearing RPC is cross-service (workflow) and has no existing ActionType representation; until that exists, the operator drives this from the CLI.",
		},

		// ── A-cont. AI knowledge seed actions ────────────────────────

		{
			// Milestone 2: demoted from HealAuto to HealPropose. ai-memory
			// upsert has no clean RemediationAction representation today —
			// the existing ai_memory upsert API is not an ActionType. The
			// seed is idempotent and safe, but Path B is being closed, so
			// dispatch waits for a workflow-or-action representation.
			InvariantID: "ops_knowledge.seed_deferred",
			Disposition: HealPropose,
			Action:      "Seed operational-knowledge entries into ai-memory from the installed awareness bundle (operator-driven CLI: globular awareness seed).",
			Rationale:   "ai-memory upsert is cross-service and has no existing ActionType representation; until the seed call can flow through ExecuteRemediation, it remains propose-only.",
		},

		// ── B. Propose-only (requires human approval) ────────────────

		{
			// awareness-graph is running but the RDF store is empty.
			// The embedded NT seed only fires on a fresh store at startup;
			// a runtime wipe requires a service restart to re-trigger it.
			// Propose only — a service restart is operator-initiated.
			InvariantID: "awareness_graph.seed_empty",
			Disposition: HealPropose,
			Action:      "Restart the awareness-graph service to re-trigger the embedded NT seed.",
			Rationale:   "The awareness-graph embeds its knowledge graph at build time and seeds Oxigraph on startup only when the store is empty. If Oxigraph was cleared post-startup a restart is required. The doctor cannot restart services automatically — this is a propose-only action.",
		},
		{
			InvariantID: "artifact.installed_digest_mismatch",
			Disposition: HealPropose,
			Action:      "Re-install the package through the normal release pipeline to refresh installed_state.Checksum.",
			Rationale:   "An installed digest mismatch could indicate tampering, a stale checksum from a pre-contract install, or a real integrity issue. Human judgment is needed to distinguish these cases before re-installing.",
		},
		{
			InvariantID: "artifact.desired_version_mismatch",
			Disposition: HealPropose,
			Action:      "Update desired state to match installed, or trigger a release to bring installed in line with desired.",
			Rationale:   "Version drift between desired and installed may be intentional (operator holding a version) or accidental. Requires human decision on which direction to resolve.",
		},
		{
			InvariantID: "artifact.desired_build_mismatch",
			Disposition: HealPropose,
			Action:      "Same as desired_version_mismatch — resolve the build number discrepancy.",
			Rationale:   "Build-number drift within the same version can indicate a stale desired state or a pinned build. Let the operator decide.",
		},
		{
			InvariantID: "cluster.services.drift",
			Disposition: HealPropose,
			Action:      "Trigger reconciliation or investigate why desired != applied hash.",
			Rationale:   "Cluster-level hash drift can have many causes. The healer cannot safely determine the correct resolution without operator context.",
		},
		{
			InvariantID: "node.units.not_running",
			Disposition: HealPropose,
			Action:      "Restart the failed systemd unit.",
			Rationale:   "Auto-restarting a service that crashed may mask the root cause. Propose the restart and let the operator review logs first.",
		},

		// ── C. Observe-only (no action) ──────────────────────────────

		{
			InvariantID: "workflow.step_failures",
			Disposition: HealObserve,
			Action:      "Historical failure counter — will decay naturally as successful cycles accumulate.",
			Rationale:   "Step failure rates are diagnostic counters, not actionable states. They reflect past events, not current drift. No repair is meaningful.",
		},
		{
			InvariantID: "workflow.no_activity",
			Disposition: HealObserve,
			Action:      "Informational — no reconcile activity detected in the observation window.",
			Rationale:   "May indicate a healthy quiescent cluster or a stalled controller. Context-dependent, not auto-repairable.",
		},
		{
			// Pending invariant stubs (repository, discovery, TLS cert expiry).
			InvariantID: "pending.*",
			Disposition: HealObserve,
			Action:      "Pending invariant — upstream RPC not yet available.",
			Rationale:   "Stubs for future invariants. No action possible until the upstream service implements the required RPC.",
		},
	}
}

// policyIndex builds a lookup map from InvariantID → HealRule.
// Wildcard rules (e.g. "pending.*") are handled separately.
func policyIndex() map[string]HealRule {
	m := make(map[string]HealRule)
	for _, r := range PolicyV1() {
		m[r.InvariantID] = r
	}
	return m
}

// LookupPolicy returns the HealRule for a given invariant ID.
// Returns HealObserve disposition for unknown invariants (safe default).
func LookupPolicy(invariantID string) HealRule {
	idx := policyIndex()
	if r, ok := idx[invariantID]; ok {
		return r
	}
	// Check wildcard prefixes.
	for _, r := range PolicyV1() {
		if len(r.InvariantID) > 1 && r.InvariantID[len(r.InvariantID)-1] == '*' {
			prefix := r.InvariantID[:len(r.InvariantID)-1]
			if len(invariantID) >= len(prefix) && invariantID[:len(prefix)] == prefix {
				return r
			}
		}
	}
	// Unknown invariant — observe only (safe default).
	return HealRule{
		InvariantID: invariantID,
		Disposition: HealObserve,
		Action:      "Unknown invariant — no policy defined.",
		Rationale:   "Safe default: never mutate what the policy doesn't explicitly cover.",
	}
}
