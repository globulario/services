// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.promotion_reconciler
// @awareness file_role=bootstrap_to_authoritative_promoter_for_infrastructurerelease_records_only_after_phase_available
// @awareness implements=globular.platform:intent.bootstrap.window_is_not_steady_state_auth
// @awareness enforces=globular.platform:invariant.desired.bootstrap_state_requires_promotion
// @awareness risk=critical
package main

// promotion_reconciler.go — the gate that turns observed
// installed state into authoritative desired state. Records
// stamped with bootstrap labels by
// materializeMissingInfraDesired (see
// desired_state_publisher.go) MUST NOT be treated as final
// intent until this reconciler promotes them, and ONLY once
// Phase reaches AVAILABLE on every target node.
//
// MUST NOT short-circuit the AVAILABLE check. Premature
// promotion is exactly the
// desired.bootstrap_premature_convergence failure mode —
// observed state becomes desired before convergence proves
// the install contract held.

// promotion_reconciler.go — bootstrap-to-authoritative promotion for
// InfrastructureRelease records created from observed installed state.
//
// Records created by materializeMissingInfraDesired carry bootstrap labels
// (desired.bootstrap_state_requires_promotion). Convergence consumers must
// not treat these records as final desired state. This reconciler runs
// periodically and promotes bootstrap records to authoritative once Phase
// reaches AVAILABLE — meaning all target nodes have converged.
//
// Invariant: desired.bootstrap_state_requires_promotion

import (
	"context"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const bootstrapPromotionInterval = 30 * time.Second

// bootstrapConvergenceAllowed reports whether convergence claims may proceed
// for the given meta. Returns false when the record is still in bootstrap
// state — preventing the reconciler from treating observer-inferred desired
// state as final cluster intent before explicit promotion.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func bootstrapConvergenceAllowed(meta *cluster_controllerpb.ObjectMeta) bool {
	return isAuthoritative(meta)
}

// startBootstrapPromotionReconciler runs a background loop that promotes
// bootstrap-labeled InfrastructureRelease records to authoritative once they
// have reached Phase AVAILABLE. Must be called on the leader only.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func (srv *server) startBootstrapPromotionReconciler(ctx context.Context) {
	ticker := time.NewTicker(bootstrapPromotionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.promoteBootstrapRecordsOnce(ctx)
		}
	}
}

// promoteBootstrapRecordsOnce scans all InfrastructureRelease objects, finds
// bootstrap-labeled records at Phase AVAILABLE, and promotes them to
// authoritative. Returns the number of records promoted.
//
//globular:enforces desired.bootstrap_state_requires_promotion
func (srv *server) promoteBootstrapRecordsOnce(ctx context.Context) int {
	if srv.resources == nil {
		return 0
	}
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		log.Printf("bootstrap-promotion: list: %v", err)
		return 0
	}
	promoted := 0
	for _, raw := range items {
		rel, ok := raw.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel == nil || rel.Meta == nil {
			continue
		}
		if !isBootstrapRecord(rel.Meta) {
			continue // already authoritative or operator-set
		}
		if rel.Status == nil || rel.Status.Phase != cluster_controllerpb.ReleasePhaseAvailable {
			continue // not yet confirmed available — keep as bootstrap
		}
		promoteToAuthoritative(rel.Meta)
		if _, err := srv.resources.Apply(ctx, "InfrastructureRelease", rel); err != nil {
			log.Printf("bootstrap-promotion: promote %s: %v", rel.Meta.Name, err)
			continue
		}
		log.Printf("bootstrap-promotion: promoted %s to authoritative", rel.Meta.Name)
		promoted++
	}
	return promoted
}
