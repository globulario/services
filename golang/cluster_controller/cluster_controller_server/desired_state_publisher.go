package main

// desired_state_publisher.go — bootstrap label management for InfrastructureRelease records.
//
// When materializeMissingInfraDesired creates an InfrastructureRelease from
// observed installed state (not from an operator-set release command), the
// record must carry bootstrap labels so convergence consumers know it is not
// yet authoritative desired state. The promotion reconciler converts bootstrap
// records to authoritative once Phase reaches AVAILABLE.
//
// Invariant: desired.bootstrap_state_requires_promotion

import cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"

const (
	labelBootstrapSource = "globular.io/bootstrap-source"
	labelAuthoritative   = "globular.io/authoritative"
	bootstrapSourceValue = "bootstrap_default"
)

// stampBootstrapLabels marks meta as a bootstrap-origin record that is not yet
// authoritative desired state. Convergence consumers must call
// bootstrapConvergenceAllowed before treating this record as final intent.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func stampBootstrapLabels(meta *cluster_controllerpb.ObjectMeta) {
	if meta == nil {
		return
	}
	if meta.Labels == nil {
		meta.Labels = make(map[string]string)
	}
	meta.Labels[labelBootstrapSource] = bootstrapSourceValue
	meta.Labels[labelAuthoritative] = "false"
}

// isBootstrapRecord reports whether meta was inferred from observed installed
// state and has not been promoted to authoritative desired state.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func isBootstrapRecord(meta *cluster_controllerpb.ObjectMeta) bool {
	if meta == nil {
		return false
	}
	return meta.Labels[labelBootstrapSource] == bootstrapSourceValue
}

// isAuthoritative reports whether meta represents explicit operator-set or
// already-promoted desired state. Records without labels are authoritative
// (operator-set releases carry no bootstrap label).
//
//globular:enforces desired.bootstrap_state_requires_promotion
func isAuthoritative(meta *cluster_controllerpb.ObjectMeta) bool {
	if meta == nil {
		return true
	}
	v, ok := meta.Labels[labelAuthoritative]
	if !ok {
		return true // no label = authoritative (operator-set)
	}
	return v != "false"
}

// promoteToAuthoritative clears the bootstrap label and marks the record as
// authoritative. Called only by the promotion reconciler when Phase == AVAILABLE.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func promoteToAuthoritative(meta *cluster_controllerpb.ObjectMeta) {
	if meta == nil || meta.Labels == nil {
		return
	}
	delete(meta.Labels, labelBootstrapSource)
	meta.Labels[labelAuthoritative] = "true"
}
