package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
)

const releaseKeyPrefix = "release/"

func isReleaseKey(key string) bool {
	return strings.HasPrefix(key, releaseKeyPrefix)
}

func releaseNameFromKey(key string) string {
	return strings.TrimPrefix(key, releaseKeyPrefix)
}

// startReleaseReconciler adds a ServiceRelease watcher to the work queue and enqueues
// any existing releases for initial reconciliation. Call from startControllerRuntime().
func (srv *server) startReleaseReconciler(ctx context.Context, queue *workQueue) {
	if srv.resources == nil {
		return
	}
	// Initial enqueue of all existing releases.
	if items, _, err := srv.resources.List(ctx, "ServiceRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*clustercontrollerpb.ServiceRelease); ok && rel.Meta != nil {
				queue.Enqueue(releaseKeyPrefix + rel.Meta.Name)
			}
		}
	}
	// Watch for new and updated releases.
	safeGo("watch-service-release", func() {
		ch, err := srv.resources.Watch(ctx, "ServiceRelease", "", "")
		if err != nil {
			return
		}
		for evt := range ch {
			if rel, ok := evt.Object.(*clustercontrollerpb.ServiceRelease); ok && rel.Meta != nil {
				queue.Enqueue(releaseKeyPrefix + rel.Meta.Name)
			}
		}
	})
}

// reconcileRelease drives the phase state machine for one ServiceRelease.
// Called from the worker goroutine when the queue key has the "release/" prefix.
func (srv *server) reconcileRelease(ctx context.Context, releaseName string) {
	if srv.resources == nil {
		return
	}
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", releaseName)
	if err != nil {
		log.Printf("release %s: get: %v", releaseName, err)
		return
	}
	if obj == nil {
		// Release deleted — nothing to do.
		return
	}
	rel, ok := obj.(*clustercontrollerpb.ServiceRelease)
	if !ok || rel.Spec == nil {
		log.Printf("release %s: unexpected object type", releaseName)
		return
	}
	if rel.Status == nil {
		rel.Status = &clustercontrollerpb.ServiceReleaseStatus{}
	}
	// Paused releases are not reconciled.
	if rel.Spec.Paused {
		return
	}

	switch rel.Status.Phase {
	case "", clustercontrollerpb.ReleasePhasePending:
		if err := srv.reconcileReleasePending(ctx, rel); err != nil {
			log.Printf("release %s: pending: %v", releaseName, err)
		}
	case clustercontrollerpb.ReleasePhaseResolved:
		if err := srv.reconcileReleaseResolved(ctx, rel); err != nil {
			log.Printf("release %s: resolved: %v", releaseName, err)
		}
	case clustercontrollerpb.ReleasePhaseApplying:
		if err := srv.reconcileReleaseApplying(ctx, rel); err != nil {
			log.Printf("release %s: applying: %v", releaseName, err)
		}
	case clustercontrollerpb.ReleasePhaseAvailable, clustercontrollerpb.ReleasePhaseDegraded:
		if err := srv.reconcileReleaseAvailable(ctx, rel); err != nil {
			log.Printf("release %s: available/degraded: %v", releaseName, err)
		}
	default:
		// FAILED, ROLLED_BACK — do not auto-retry; require explicit re-apply.
	}
}

// reconcileReleasePending resolves the version and artifact digest,
// transitioning the release to RESOLVED.
func (srv *server) reconcileReleasePending(ctx context.Context, rel *clustercontrollerpb.ServiceRelease) error {
	name := rel.Meta.Name
	gen := int64(0)
	if rel.Meta != nil {
		gen = rel.Meta.Generation
	}

	// Skip re-resolution if this generation was already resolved (idempotency guard).
	if rel.Status.ObservedGeneration == gen &&
		rel.Status.ResolvedVersion != "" &&
		rel.Status.ResolvedArtifactDigest != "" {
		return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
			s.Phase = clustercontrollerpb.ReleasePhaseResolved
		})
	}

	resolver := &ReleaseResolver{RepositoryAddr: repositoryAddrForSpec(rel.Spec)}
	resolvedVersion, digest, err := resolver.Resolve(ctx, rel.Spec)
	if err != nil {
		log.Printf("release %s: resolve failed: %v", name, err)
		return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
			s.Phase = clustercontrollerpb.ReleasePhaseFailed
			s.Message = fmt.Sprintf("resolve: %v", err)
			s.LastTransitionUnixMs = time.Now().UnixMilli()
		})
	}

	desiredHash := ComputeReleaseDesiredHash(rel.Spec.PublisherID, rel.Spec.ServiceName, resolvedVersion, rel.Spec.Config)

	return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
		s.Phase = clustercontrollerpb.ReleasePhaseResolved
		s.ResolvedVersion = resolvedVersion
		s.ResolvedArtifactDigest = digest
		s.DesiredHash = desiredHash
		s.ObservedGeneration = gen
		s.Message = ""
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseResolved compiles and pushes NodePlans for all target nodes,
// transitioning the release to APPLYING.
func (srv *server) reconcileReleaseResolved(ctx context.Context, rel *clustercontrollerpb.ServiceRelease) error {
	name := rel.Meta.Name

	srv.lock("release-reconcile:snapshot")
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	for id := range srv.state.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	clusterID := srv.state.ClusterId
	srv.unlock()

	if len(nodeIDs) == 0 {
		// No nodes joined yet; the next ReportNodeStatus will re-enqueue this release.
		return nil
	}

	resolver := &ReleaseResolver{RepositoryAddr: repositoryAddrForSpec(rel.Spec)}
	nodeStatuses := make([]*clustercontrollerpb.NodeReleaseStatus, 0, len(nodeIDs))

	resolvedVersion := rel.Status.ResolvedVersion
	targetLock := fmt.Sprintf("service:%s", canonicalServiceName(rel.Spec.ServiceName))

	for _, nodeID := range nodeIDs {
		// Controller-side lock guard (Amendment 6): skip dispatch if an active plan on
		// this node already holds the same service lock. This avoids issuing conflicting
		// plans and producing noisy PLAN_PENDING states on the node-agent side.
		if srv.hasActivePlanWithLock(ctx, nodeID, targetLock) {
			log.Printf("release %s: node %s: lock %q held by active plan, will retry",
				name, nodeID, targetLock)
			nodeStatuses = append(nodeStatuses, &clustercontrollerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         clustercontrollerpb.ReleasePhaseApplying,
				ErrorMessage:  fmt.Sprintf("waiting for lock %s", targetLock),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		// Compute safePrevVersion: the version to roll back to if the new plan fails.
		// Rules (Amendment 5 + prev==target guard):
		//   1. Empty if no prior version is known.
		//   2. Empty if prior version equals the target (rollback to self is a no-op).
		//   3. Empty if the prior version's repository manifest is unreachable.
		installedVersion := srv.getInstalledVersionForRelease(rel, nodeID)
		safePrevVersion := installedVersion
		if safePrevVersion == "" || safePrevVersion == resolvedVersion {
			safePrevVersion = ""
		}
		if safePrevVersion != "" {
			// Pre-check: verify the prior version's manifest is still accessible before
			// arming rollback steps. If the manifest is gone, we skip rollback rather
			// than pointing to a non-existent artifact.
			prevSpec := &clustercontrollerpb.ServiceReleaseSpec{
				PublisherID:  rel.Spec.PublisherID,
				ServiceName:  rel.Spec.ServiceName,
				Version:      safePrevVersion,
				Platform:     rel.Spec.Platform,
				RepositoryID: rel.Spec.RepositoryID,
			}
			if _, _, err := resolver.Resolve(ctx, prevSpec); err != nil {
				log.Printf("release %s node %s: rollback guard: prev version %q manifest missing (%v), disabling rollback",
					name, nodeID, safePrevVersion, err)
				safePrevVersion = ""
			}
		}

		plan, err := CompileReleasePlan(nodeID, rel, safePrevVersion, clusterID)
		if err != nil {
			log.Printf("release %s: compile plan for node %s: %v", name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &clustercontrollerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         clustercontrollerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("compile: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		plan.PlanId = uuid.NewString()
		plan.Generation = srv.nextPlanGeneration(ctx, nodeID)
		plan.IssuedBy = "cluster-controller"
		if plan.GetCreatedUnixMs() == 0 {
			plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
		}

		if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
			log.Printf("release %s: persist plan for node %s: %v", name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &clustercontrollerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         clustercontrollerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("persist plan: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}
		if appendable, ok := srv.planStore.(interface {
			AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
		}); ok {
			_ = appendable.AppendHistory(ctx, nodeID, plan)
		}

		log.Printf("release %s: wrote plan node=%s plan_id=%s gen=%d",
			name, nodeID, plan.PlanId, plan.Generation)
		nodeStatuses = append(nodeStatuses, &clustercontrollerpb.NodeReleaseStatus{
			NodeID:        nodeID,
			PlanID:        plan.PlanId,
			Phase:         clustercontrollerpb.ReleasePhaseApplying,
			UpdatedUnixMs: time.Now().UnixMilli(),
		})
	}

	return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
		s.Phase = clustercontrollerpb.ReleasePhaseApplying
		s.Nodes = nodeStatuses
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseApplying inspects per-node plan statuses and advances the release
// to AVAILABLE, DEGRADED, or FAILED when all nodes have reached a terminal state.
func (srv *server) reconcileReleaseApplying(ctx context.Context, rel *clustercontrollerpb.ServiceRelease) error {
	name := rel.Meta.Name

	if rel.Status == nil || len(rel.Status.Nodes) == 0 {
		// Lost our node list; re-enter RESOLVED to recompile plans.
		return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
			s.Phase = clustercontrollerpb.ReleasePhaseResolved
		})
	}

	updatedNodes := make([]*clustercontrollerpb.NodeReleaseStatus, 0, len(rel.Status.Nodes))
	succeeded := 0
	failed := 0
	running := 0

	for _, nrs := range rel.Status.Nodes {
		if nrs == nil {
			continue
		}
		updated := *nrs // copy

		planStatus, err := srv.planStore.GetStatus(ctx, nrs.NodeID)
		if err != nil || planStatus == nil {
			// Status not yet available; node is still working.
			running++
			updatedNodes = append(updatedNodes, &updated)
			continue
		}
		// Ignore status that belongs to a different plan.
		if nrs.PlanID != "" && planStatus.GetPlanId() != nrs.PlanID {
			running++
			updatedNodes = append(updatedNodes, &updated)
			continue
		}

		switch planStatus.GetState() {
		case planpb.PlanState_PLAN_SUCCEEDED:
			succeeded++
			updated.Phase = clustercontrollerpb.ReleasePhaseAvailable
			updated.InstalledVersion = rel.Status.ResolvedVersion
			updated.ErrorMessage = ""
			updated.UpdatedUnixMs = time.Now().UnixMilli()
		case planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED:
			failed++
			updated.Phase = clustercontrollerpb.ReleasePhaseFailed
			updated.ErrorMessage = planStatus.GetErrorMessage()
			updated.UpdatedUnixMs = time.Now().UnixMilli()
		default:
			// PENDING, RUNNING, ROLLING_BACK — still in progress.
			running++
		}
		updatedNodes = append(updatedNodes, &updated)
	}

	total := len(updatedNodes)
	newPhase := clustercontrollerpb.ReleasePhaseApplying
	switch {
	case total > 0 && succeeded == total:
		newPhase = clustercontrollerpb.ReleasePhaseAvailable
	case failed > 0 && running == 0:
		if succeeded > 0 {
			newPhase = clustercontrollerpb.ReleasePhaseDegraded
		} else {
			newPhase = clustercontrollerpb.ReleasePhaseFailed
		}
	}

	return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
		s.Phase = newPhase
		s.Nodes = updatedNodes
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseAvailable performs drift detection: if the spec generation advanced
// beyond the observed generation, re-enter PENDING for re-resolution.
func (srv *server) reconcileReleaseAvailable(ctx context.Context, rel *clustercontrollerpb.ServiceRelease) error {
	if rel.Meta == nil || rel.Status == nil {
		return nil
	}
	if rel.Meta.Generation > rel.Status.ObservedGeneration {
		return srv.patchReleaseStatus(ctx, rel.Meta.Name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
			s.Phase = clustercontrollerpb.ReleasePhasePending
		})
	}
	name := rel.Meta.Name
	desiredHash := strings.ToLower(strings.TrimSpace(rel.Status.DesiredHash))
	nodes := rel.Status.Nodes
	total := len(nodes)
	minReplicas := total
	if rel.Spec != nil && rel.Spec.MaxUnavailable > 0 && int(rel.Spec.MaxUnavailable) < total {
		minReplicas = total - int(rel.Spec.MaxUnavailable)
	}
	if minReplicas < 1 && total > 0 {
		minReplicas = 1
	}

	targetLock := fmt.Sprintf("service:%s", canonicalServiceName(rel.Spec.ServiceName))
	updatedNodes := make([]*clustercontrollerpb.NodeReleaseStatus, 0, total)

	ok := 0
	issues := 0

	for _, n := range nodes {
		if n == nil || strings.TrimSpace(n.NodeID) == "" {
			continue
		}
		nodeID := strings.TrimSpace(n.NodeID)
		nCopy := *n
		srv.lock("state:snapshot")
		node := srv.state.Nodes[nodeID]
		srv.unlock()

		applied := ""
		healthy := false
		serviceHealthy := false
		if node != nil {
			applied = strings.ToLower(strings.TrimSpace(node.AppliedServicesHash))
			healthy = strings.EqualFold(node.Status, "ready")
			serviceHealthy = srv.serviceHealthyForRelease(node, rel)
		}
		hashMatch := desiredHash != "" && applied == desiredHash
		if hashMatch && healthy && serviceHealthy {
			ok++
			nCopy.Phase = clustercontrollerpb.ReleasePhaseAvailable
		} else {
			issues++
			// Drift detection: enqueue plan if not already running with the same lock.
			if srv.planStore != nil && !srv.hasActivePlanWithLockFn(ctx, nodeID, targetLock) {
				if plan, err := srv.dispatchReleasePlanFn(ctx, rel, nodeID); err == nil && plan != nil {
					nCopy.PlanID = plan.GetPlanId()
					nCopy.Phase = clustercontrollerpb.ReleasePhaseApplying
					nCopy.UpdatedUnixMs = time.Now().UnixMilli()
				} else if err != nil {
					log.Printf("release %s: node %s drift plan compile failed: %v", name, nodeID, err)
				}
			}
			if nCopy.Phase == "" {
				nCopy.Phase = clustercontrollerpb.ReleasePhaseDegraded
			}
		}
		updatedNodes = append(updatedNodes, &nCopy)
	}

	newPhase := rel.Status.Phase
	switch {
	case total == 0:
		newPhase = clustercontrollerpb.ReleasePhaseFailed
	case ok >= minReplicas && issues == 0:
		newPhase = clustercontrollerpb.ReleasePhaseAvailable
	case ok >= minReplicas:
		newPhase = clustercontrollerpb.ReleasePhaseDegraded
	default:
		newPhase = clustercontrollerpb.ReleasePhaseFailed
	}

	if newPhase == rel.Status.Phase && len(updatedNodes) == len(nodes) {
		return nil
	}

	return srv.patchReleaseStatus(ctx, name, func(s *clustercontrollerpb.ServiceReleaseStatus) {
		s.Phase = newPhase
		s.Nodes = updatedNodes
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

func (srv *server) serviceHealthyForRelease(node *nodeState, rel *clustercontrollerpb.ServiceRelease) bool {
	if node == nil || rel == nil || rel.Spec == nil {
		return false
	}
	unit := serviceUnitForCanonical(canonicalServiceName(rel.Spec.ServiceName))
	for _, u := range node.Units {
		if strings.EqualFold(u.Name, unit) {
			return strings.EqualFold(u.State, "active")
		}
	}
	// Unit missing => unhealthy for this release.
	return false
}

func (srv *server) dispatchReleasePlan(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
	if srv.planStore == nil {
		return nil, fmt.Errorf("plan store unavailable")
	}
	installedVersion := srv.getInstalledVersionForRelease(rel, nodeID)
	clusterID := ""
	if srv.state != nil {
		clusterID = srv.state.ClusterId
	}
	plan, err := CompileReleasePlan(nodeID, rel, installedVersion, clusterID)
	if err != nil {
		return nil, err
	}
	plan.PlanId = uuid.NewString()
	plan.Generation = srv.nextPlanGeneration(ctx, nodeID)
	plan.IssuedBy = "cluster-controller"
	if plan.GetCreatedUnixMs() == 0 {
		plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
	}
	if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
		return nil, err
	}
	if appendable, ok := srv.planStore.(interface {
		AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	}); ok {
		_ = appendable.AppendHistory(ctx, nodeID, plan)
	}
	return plan, nil
}

func (srv *server) hasActivePlanWithLockFn(ctx context.Context, nodeID, lock string) bool {
	if srv.testHasActivePlanWithLock != nil {
		return srv.testHasActivePlanWithLock(ctx, nodeID, lock)
	}
	return srv.hasActivePlanWithLock(ctx, nodeID, lock)
}

func (srv *server) dispatchReleasePlanFn(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
	if srv.testDispatchReleasePlan != nil {
		return srv.testDispatchReleasePlan(ctx, rel, nodeID)
	}
	return srv.dispatchReleasePlan(ctx, rel, nodeID)
}

// patchReleaseStatus loads the latest copy of a ServiceRelease, applies f to its status,
// and saves via resources.Apply. This avoids conflicting with concurrent writes.
func (srv *server) patchReleaseStatus(ctx context.Context, releaseName string, f func(*clustercontrollerpb.ServiceReleaseStatus)) error {
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get release %s for status patch: %w", releaseName, err)
	}
	rel, ok := obj.(*clustercontrollerpb.ServiceRelease)
	if !ok {
		return fmt.Errorf("unexpected type for release %s", releaseName)
	}
	if rel.Status == nil {
		rel.Status = &clustercontrollerpb.ServiceReleaseStatus{}
	}
	f(rel.Status)
	_, err = srv.resources.Apply(ctx, "ServiceRelease", rel)
	return err
}

// getInstalledVersionForRelease returns the InstalledVersion for the given node from
// the existing release status, used as the candidate prior version for rollback.
// Returns "" if unknown (first deployment or not yet reported).
func (srv *server) getInstalledVersionForRelease(rel *clustercontrollerpb.ServiceRelease, nodeID string) string {
	if rel.Status != nil {
		for _, nrs := range rel.Status.Nodes {
			if nrs != nil && nrs.NodeID == nodeID && nrs.InstalledVersion != "" {
				return nrs.InstalledVersion
			}
		}
	}
	// Fallback to node-reported installed versions if present.
	srv.lock("state:snapshot")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node != nil && len(node.InstalledVersions) > 0 {
		// Accept either "publisher/service" or raw service name keys.
		canon := canonicalServiceName(rel.Spec.ServiceName)
		pub := strings.TrimSpace(rel.Spec.PublisherID)
		keyWithPublisher := fmt.Sprintf("%s/%s", pub, canon)
		if v := strings.TrimSpace(node.InstalledVersions[keyWithPublisher]); v != "" {
			return v
		}
		if v := strings.TrimSpace(node.InstalledVersions[canon]); v != "" {
			return v
		}
	}
	return ""
}

// repositoryAddrForSpec returns the repository gRPC endpoint for the given spec.
// Falls back to the empty string, which causes ReleaseResolver to use its default.
func repositoryAddrForSpec(spec *clustercontrollerpb.ServiceReleaseSpec) string {
	if spec != nil && strings.TrimSpace(spec.RepositoryID) != "" {
		return strings.TrimSpace(spec.RepositoryID)
	}
	return ""
}

// hasActivePlanWithLock reports whether the node currently has a running or pending plan
// that holds the given lock key. Used as a controller-side guard to avoid dispatching
// conflicting plans (Amendment 6 primary enforcement).
func (srv *server) hasActivePlanWithLock(ctx context.Context, nodeID, lock string) bool {
	status, err := srv.planStore.GetStatus(ctx, nodeID)
	if err != nil || status == nil {
		return false
	}
	switch status.GetState() {
	case planpb.PlanState_PLAN_RUNNING, planpb.PlanState_PLAN_PENDING, planpb.PlanState_PLAN_ROLLING_BACK:
	default:
		return false
	}
	plan, err := srv.planStore.GetCurrentPlan(ctx, nodeID)
	if err != nil || plan == nil {
		return false
	}
	for _, l := range plan.GetLocks() {
		if l == lock {
			return true
		}
	}
	return false
}
