// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.desired_state_absent
// @awareness file_role=doctor_rule_detecting_absent_layer2_desired_state_on_a_bootstrapped_cluster
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness risk=high
// desired_state_absent.go — DIAGNOSTIC ONLY. Read-only doctor rule.
//
// Detects a cluster that reports healthy while having no Layer 2 at all.

package rules

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ── absent desired state ────────────────────────────────────────────────────
//
// WHY THIS EXISTS
//
// On 2026-07-31 a five-node cluster reported status=healthy, members=5, nodes=5
// for 48 minutes while the reconcile loop failed on every pass and converged
// nothing: cluster.reconcile was unresolvable, the controller's circuit breaker
// sat permanently OPEN, and desired state was empty. Every node heartbeated
// normally throughout. A full 38-scenario suite ran against that cluster and
// every result was unattributable.
//
// Nothing detected it, and the reason is a single line in cluster.services.drift:
//
//	if desired == "" || desired == "services:none" || desired == applied { continue }
//
// That skip is correct FOR THAT RULE — a node with nothing desired has nothing
// to drift from. But it means an EMPTY Layer 2 is indistinguishable from a
// CONVERGED one by hash comparison: desired == applied == nothing, no drift, no
// finding. The cluster looks settled precisely because it has no intent to
// settle toward.
//
// This rule closes that blind spot. It asks a different question from drift:
// not "does applied match desired", but "is there any desired state at all".
//
// WHAT IT CATCHES
//
//   - Day-0 that never seeded desired state (install-day0.sh "Seeding Desired
//     State from Installed Packages" did not run or failed)
//   - a reconcile loop that cannot resolve cluster.reconcile, so desired state
//     is never materialized
//   - desired state deleted or lost while nodes remain registered
//
// WHAT IT DELIBERATELY DOES NOT DO
//
// Read-only, like every rule here. Desired state is controller-owned; doctor
// observes all four layers and produces findings but produces no state
// (cluster_doctor.observer_only_never_writes_etcd). Remediation is operator- or
// workflow-driven, never a side effect of diagnosis.

type desiredStateAbsent struct{}

func (desiredStateAbsent) ID() string       { return "cluster.desired_state.absent" }
func (desiredStateAbsent) Category() string { return "drift" }
func (desiredStateAbsent) Scope() string    { return "cluster" }

func (desiredStateAbsent) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	// Refuse to answer when the health source errored. An empty snap.NodeHealths
	// then means "cluster_controller unreachable", not "no desired state" — and
	// reporting absent Layer 2 on the strength of a failed RPC would be exactly
	// the "not found where != does not exist" mistake this rule exists to catch.
	// See doctor_rule_evaluate_must_consult_snap_errors,
	// meta.absence_scope_must_be_explicit.
	if snap.HadError("cluster_controller", "GetClusterHealthV1") {
		return nil
	}
	if snap.HadError("cluster_controller", "ListNodes") {
		return nil
	}

	// No nodes reporting health is not evidence of absent desired state — it is
	// evidence of no data. Stay silent rather than manufacture a finding.
	if len(snap.NodeHealths) == 0 {
		return nil
	}

	// A cluster that has not been bootstrapped legitimately has no desired
	// state. Only judge nodes that are actually part of a formed cluster.
	if len(snap.Nodes) == 0 {
		return nil
	}

	var (
		withDesired int
		withoutIDs  []string
	)
	for nodeID, nh := range snap.NodeHealths {
		desired := nh.GetDesiredServicesHash()
		if desired == "" || desired == "services:none" {
			withoutIDs = append(withoutIDs, nodeID)
			continue
		}
		withDesired++
	}

	// At least one node carries desired state — Layer 2 exists. Per-node gaps
	// are placement intent (a compute node may legitimately be assigned
	// nothing), not an absent layer, and cluster.services.drift owns the
	// desired-vs-applied question from here.
	if withDesired > 0 {
		return nil
	}

	summary := fmt.Sprintf(
		"No desired state for any of %d registered node(s) — Layer 2 is absent, so nothing can converge",
		len(withoutIDs))

	evidence := []*cluster_doctorpb.Evidence{
		kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
			"nodes_registered":      fmt.Sprintf("%d", len(snap.Nodes)),
			"nodes_reporting":       fmt.Sprintf("%d", len(snap.NodeHealths)),
			"nodes_with_desired":    "0",
			"nodes_without_desired": fmt.Sprintf("%d", len(withoutIDs)),
			"note":                  "healthy heartbeats do not imply convergence; desired == applied == empty produces no drift finding",
		}),
	}

	remediation := []*cluster_doctorpb.RemediationStep{
		step(1, "Check whether the reconcile loop can run at all — an unresolvable "+
			"cluster.reconcile or an OPEN circuit breaker leaves desired state unmaterialized",
			"journalctl -u globular-cluster-controller | grep -E 'cluster.reconcile FAILED|circuit OPEN|workflow definition'"),
		step(2, "Verify the core workflow definitions are installed on disk — the "+
			"controller seeds them to etcd from /var/lib/globular/workflows at leadership gain",
			"ls /var/lib/globular/workflows/"),
		step(3, "Confirm desired state in etcd. NOTE: the release writes "+
			"ServiceDesiredVersion; DesiredService is documented but not written",
			"etcdctl get /globular/resources/ServiceDesiredVersion/ --prefix --keys-only"),
		step(4, "If Day-0 never seeded it, re-run the seeding step rather than "+
			"hand-writing desired state — desired state is controller-owned", ""),
	}

	return []Finding{{
		FindingID:   FindingID("cluster.desired_state.absent", "cluster", fmt.Sprintf("%d", len(withoutIDs))),
		InvariantID: "cluster.desired_state.absent",
		// ERROR, not CRITICAL: the cluster is still serving, and a genuinely
		// fresh cluster mid-Day-0 can transit this state briefly. It is a
		// convergence failure, not a data-loss emergency.
		Severity:        cluster_doctorpb.Severity_SEVERITY_ERROR,
		Category:        "drift",
		EntityRef:       "cluster",
		Summary:         summary,
		Evidence:        evidence,
		Remediation:     remediation,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}}
}
