package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
)

const releaseKeyPrefix = "release/"
const appReleaseKeyPrefix = "app-release/"
const infraReleaseKeyPrefix = "infra-release/"

func isReleaseKey(key string) bool {
	return strings.HasPrefix(key, releaseKeyPrefix)
}

func releaseNameFromKey(key string) string {
	return strings.TrimPrefix(key, releaseKeyPrefix)
}

func isAppReleaseKey(key string) bool {
	return strings.HasPrefix(key, appReleaseKeyPrefix)
}

func appReleaseNameFromKey(key string) string {
	return strings.TrimPrefix(key, appReleaseKeyPrefix)
}

func isInfraReleaseKey(key string) bool {
	return strings.HasPrefix(key, infraReleaseKeyPrefix)
}

func infraReleaseNameFromKey(key string) string {
	return strings.TrimPrefix(key, infraReleaseKeyPrefix)
}

// startReleaseReconciler adds release watchers to the work queue and enqueues
// existing releases for initial reconciliation. Call from startControllerRuntime().
func (srv *server) startReleaseReconciler(ctx context.Context, queue *workQueue) {
	if srv.resources == nil {
		return
	}
	// Initial enqueue of all existing releases.
	if items, _, err := srv.resources.List(ctx, "ServiceRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok && rel.Meta != nil {
				queue.Enqueue(releaseKeyPrefix + rel.Meta.Name)
			}
		}
	}
	// Watch for new and updated releases.
	safeGo("watch-service-release", func() {
		ch, err := srv.resources.Watch(ctx, "ServiceRelease", "", "")
		if err != nil {
			log.Printf("watch-service-release: watch failed: %v", err)
			return
		}
		log.Printf("watch-service-release: started")
		for evt := range ch {
			if rel, ok := evt.Object.(*cluster_controllerpb.ServiceRelease); ok && rel.Meta != nil {
				phase := ""
				if rel.Status != nil {
					phase = rel.Status.Phase
				}
				log.Printf("watch-service-release: event %s name=%s phase=%s", evt.Type, rel.Meta.Name, phase)
				queue.Enqueue(releaseKeyPrefix + rel.Meta.Name)
			}
		}
		log.Printf("watch-service-release: channel closed, exiting")
	})

	// Initial enqueue of ApplicationRelease objects.
	if items, _, err := srv.resources.List(ctx, "ApplicationRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.ApplicationRelease); ok && rel.Meta != nil {
				queue.Enqueue(appReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	}
	safeGo("watch-application-release", func() {
		ch, err := srv.resources.Watch(ctx, "ApplicationRelease", "", "")
		if err != nil {
			return
		}
		for evt := range ch {
			if rel, ok := evt.Object.(*cluster_controllerpb.ApplicationRelease); ok && rel.Meta != nil {
				queue.Enqueue(appReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	})

	// Initial enqueue of InfrastructureRelease objects.
	if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Meta != nil {
				queue.Enqueue(infraReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	}
	safeGo("watch-infrastructure-release", func() {
		ch, err := srv.resources.Watch(ctx, "InfrastructureRelease", "", "")
		if err != nil {
			return
		}
		for evt := range ch {
			if rel, ok := evt.Object.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Meta != nil {
				queue.Enqueue(infraReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	})
}

// reconcileRelease drives the phase state machine for one ServiceRelease
// using the shared release pipeline.
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
		log.Printf("release %s: not found in store", releaseName)
		return
	}
	rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
	if !ok || rel.Spec == nil {
		log.Printf("release %s: unexpected object type", releaseName)
		return
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
	}
	log.Printf("release %s: reconciling phase=%s gen=%d", releaseName, rel.Status.Phase, rel.Meta.Generation)
	if rel.Spec.Paused {
		return
	}

	h := srv.svcReleaseHandle(rel)

	// Removing flag takes priority: transition to REMOVING if not already in a removal phase.
	if h.Removing && h.Phase != ReleasePhaseRemoving && h.Phase != ReleasePhaseRemoved {
		srv.reconcileRemoving(ctx, h)
		return
	}

	switch h.Phase {
	case "", cluster_controllerpb.ReleasePhasePending:
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		srv.reconcileApplying(ctx, h)
	case cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded:
		srv.reconcileAvailable(ctx, h)
	case ReleasePhaseRemoving:
		srv.reconcileRemoving(ctx, h)
	case ReleasePhaseRemoved:
		// Garbage-collect the release resource.
		if err := srv.resources.Delete(ctx, "ServiceRelease", releaseName); err != nil {
			log.Printf("release %s: garbage-collect failed: %v", releaseName, err)
		} else {
			log.Printf("release %s: garbage-collected (REMOVED)", releaseName)
		}
	case cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseRolledBack:
		// Re-enter PENDING if the spec generation advanced (explicit re-apply)
		// or if the service is still desired but not installed on any node.
		if h.Generation > h.ObservedGeneration {
			log.Printf("release %s: %s → PENDING (generation %d > observed %d)",
				releaseName, h.Phase, h.Generation, h.ObservedGeneration)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed",
				SetFields:        "phase",
			})
		}
	}
}

// reconcileReleasePending resolves the version and artifact digest,
// transitioning the release to RESOLVED.
func (srv *server) reconcileReleasePending(ctx context.Context, rel *cluster_controllerpb.ServiceRelease) error {
	name := rel.Meta.Name
	gen := int64(0)
	if rel.Meta != nil {
		gen = rel.Meta.Generation
	}

	// Skip re-resolution if this generation was already resolved (idempotency guard).
	if rel.Status.ObservedGeneration == gen &&
		rel.Status.ResolvedVersion != "" &&
		rel.Status.ResolvedArtifactDigest != "" {
		return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
			s.Phase = cluster_controllerpb.ReleasePhaseResolved
		})
	}

	resolver := &ReleaseResolver{RepositoryAddr: repositoryAddrForSpec(rel.Spec)}
	resolved, err := resolver.Resolve(ctx, rel.Spec)
	if err != nil {
		log.Printf("release %s: resolve failed: %v", name, err)
		return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
			s.Phase = cluster_controllerpb.ReleasePhaseFailed
			s.Message = fmt.Sprintf("resolve: %v", err)
			s.LastTransitionUnixMs = time.Now().UnixMilli()
		})
	}

	desiredHash := ComputeReleaseDesiredHash(rel.Spec.PublisherID, rel.Spec.ServiceName, resolved.Version, rel.Spec.Config)

	return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
		s.Phase = cluster_controllerpb.ReleasePhaseResolved
		s.ResolvedVersion = resolved.Version
		s.ResolvedArtifactDigest = resolved.Digest
		s.DesiredHash = desiredHash
		s.ObservedGeneration = gen
		s.Message = ""
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseResolved compiles and pushes NodePlans for all target nodes,
// transitioning the release to APPLYING.
func (srv *server) reconcileReleaseResolved(ctx context.Context, rel *cluster_controllerpb.ServiceRelease) error {
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
	nodeStatuses := make([]*cluster_controllerpb.NodeReleaseStatus, 0, len(nodeIDs))

	resolvedVersion := rel.Status.ResolvedVersion
	targetLock := fmt.Sprintf("service:%s", canonicalServiceName(rel.Spec.ServiceName))

	for _, nodeID := range nodeIDs {
		// Controller-side lock guard (Amendment 6): skip dispatch if an active plan on
		// this node already holds the same service lock. This avoids issuing conflicting
		// plans and producing noisy PLAN_PENDING states on the node-agent side.
		if srv.hasActivePlanWithLock(ctx, nodeID, targetLock) {
			log.Printf("release %s: node %s: lock %q held by active plan, will retry",
				name, nodeID, targetLock)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseApplying,
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
			prevSpec := &cluster_controllerpb.ServiceReleaseSpec{
				PublisherID:  rel.Spec.PublisherID,
				ServiceName:  rel.Spec.ServiceName,
				Version:      safePrevVersion,
				Platform:     rel.Spec.Platform,
				RepositoryID: rel.Spec.RepositoryID,
			}
			if _, err := resolver.Resolve(ctx, prevSpec); err != nil {
				log.Printf("release %s node %s: rollback guard: prev version %q manifest missing (%v), disabling rollback",
					name, nodeID, safePrevVersion, err)
				safePrevVersion = ""
			}
		}

		plan, err := CompileReleasePlan(nodeID, rel, safePrevVersion, clusterID)
		if err != nil {
			log.Printf("release %s: compile plan for node %s: %v", name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
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
		if err := srv.signOrAbort(plan); err != nil {
			log.Printf("release %s: signing aborted for node %s: %v", name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("plan signing failed: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
			log.Printf("release %s: persist plan for node %s: %v", name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
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
		nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
			NodeID:        nodeID,
			PlanID:        plan.PlanId,
			Phase:         cluster_controllerpb.ReleasePhaseApplying,
			UpdatedUnixMs: time.Now().UnixMilli(),
		})
	}

	return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
		s.Phase = cluster_controllerpb.ReleasePhaseApplying
		s.Nodes = nodeStatuses
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseApplying inspects per-node plan statuses and advances the release
// to AVAILABLE, DEGRADED, or FAILED when all nodes have reached a terminal state.
func (srv *server) reconcileReleaseApplying(ctx context.Context, rel *cluster_controllerpb.ServiceRelease) error {
	name := rel.Meta.Name

	if rel.Status == nil || len(rel.Status.Nodes) == 0 {
		// Lost our node list; re-enter RESOLVED to recompile plans.
		return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
			s.Phase = cluster_controllerpb.ReleasePhaseResolved
		})
	}

	updatedNodes, succeeded, failed, rolledBack, running := srv.checkNodePlanStatuses(ctx, rel.Status.Nodes, rel.Status.ResolvedVersion)

	total := len(updatedNodes)
	newPhase := cluster_controllerpb.ReleasePhaseApplying
	switch {
	case total > 0 && succeeded == total:
		newPhase = cluster_controllerpb.ReleasePhaseAvailable
	case total > 0 && rolledBack == total:
		newPhase = cluster_controllerpb.ReleasePhaseRolledBack
	case (failed > 0 || rolledBack > 0) && running == 0:
		if succeeded > 0 {
			newPhase = cluster_controllerpb.ReleasePhaseDegraded
		} else {
			newPhase = cluster_controllerpb.ReleasePhaseFailed
		}
	}

	return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
		s.Phase = newPhase
		s.Nodes = updatedNodes
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

// reconcileReleaseAvailable performs drift detection: if the spec generation advanced
// beyond the observed generation, re-enter PENDING for re-resolution.
func (srv *server) reconcileReleaseAvailable(ctx context.Context, rel *cluster_controllerpb.ServiceRelease) error {
	if rel.Meta == nil || rel.Status == nil {
		return nil
	}
	if rel.Meta.Generation > rel.Status.ObservedGeneration {
		return srv.patchReleaseStatus(ctx, rel.Meta.Name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
			s.Phase = cluster_controllerpb.ReleasePhasePending
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
	updatedNodes := make([]*cluster_controllerpb.NodeReleaseStatus, 0, total)

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
			nCopy.Phase = cluster_controllerpb.ReleasePhaseAvailable
		} else {
			issues++
			// Drift detection: enqueue plan if not already running with the same lock.
			if srv.planStore != nil && !srv.hasActivePlanWithLockFn(ctx, nodeID, targetLock) {
				if plan, err := srv.dispatchReleasePlanFn(ctx, rel, nodeID); err == nil && plan != nil {
					nCopy.PlanID = plan.GetPlanId()
					nCopy.Phase = cluster_controllerpb.ReleasePhaseApplying
					nCopy.UpdatedUnixMs = time.Now().UnixMilli()
				} else if err != nil {
					log.Printf("release %s: node %s drift plan compile failed: %v", name, nodeID, err)
				}
			}
			if nCopy.Phase == "" {
				nCopy.Phase = cluster_controllerpb.ReleasePhaseDegraded
			}
		}
		updatedNodes = append(updatedNodes, &nCopy)
	}

	newPhase := rel.Status.Phase
	switch {
	case total == 0:
		newPhase = cluster_controllerpb.ReleasePhaseFailed
	case ok >= minReplicas && issues == 0:
		newPhase = cluster_controllerpb.ReleasePhaseAvailable
	case ok >= minReplicas:
		newPhase = cluster_controllerpb.ReleasePhaseDegraded
	default:
		newPhase = cluster_controllerpb.ReleasePhaseFailed
	}

	if newPhase == rel.Status.Phase && len(updatedNodes) == len(nodes) {
		return nil
	}

	return srv.patchReleaseStatus(ctx, name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
		s.Phase = newPhase
		s.Nodes = updatedNodes
		s.LastTransitionUnixMs = time.Now().UnixMilli()
	})
}

func (srv *server) serviceHealthyForRelease(node *nodeState, rel *cluster_controllerpb.ServiceRelease) bool {
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

func (srv *server) dispatchReleasePlan(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
	if srv.planStore == nil {
		return nil, fmt.Errorf("plan store unavailable")
	}
	installedVersion := srv.getInstalledVersionForRelease(rel, nodeID)
	clusterID := ""
	if srv.state != nil {
		clusterID = srv.state.ClusterId
	}

	// Node-aware platform: if spec.Platform is empty, derive from node's reported platform.
	if rel.Spec != nil && strings.TrimSpace(rel.Spec.Platform) == "" {
		if nodePlatform := srv.getNodePlatform(nodeID); nodePlatform != "" {
			rel.Spec.Platform = nodePlatform
		}
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
	if err := srv.signOrAbort(plan); err != nil {
		return nil, err
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

func (srv *server) dispatchReleasePlanFn(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, nodeID string) (*planpb.NodePlan, error) {
	if srv.testDispatchReleasePlan != nil {
		return srv.testDispatchReleasePlan(ctx, rel, nodeID)
	}
	return srv.dispatchReleasePlan(ctx, rel, nodeID)
}

// patchReleaseStatus loads the latest copy of a ServiceRelease, applies f to its status,
// and saves via resources.Apply. This avoids conflicting with concurrent writes.
func (srv *server) patchReleaseStatus(ctx context.Context, releaseName string, f func(*cluster_controllerpb.ServiceReleaseStatus)) error {
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get release %s for status patch: %w", releaseName, err)
	}
	rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
	if !ok {
		return fmt.Errorf("unexpected type for release %s", releaseName)
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
	}
	previousPhase := rel.Status.Phase
	f(rel.Status)

	// Hard enforcement: invalid transition blocks the patch.
	if rel.Status.Phase != previousPhase {
		if err := srv.emitPhaseTransition(releaseName, previousPhase, rel.Status.Phase, rel.Status.Message); err != nil {
			return fmt.Errorf("release %s: %w", releaseName, err)
		}
	}

	_, err = srv.resources.Apply(ctx, "ServiceRelease", rel)
	return err
}

// getInstalledVersionForRelease returns the InstalledVersion for the given node from
// the existing release status, used as the candidate prior version for rollback.
// Returns "" if unknown (first deployment or not yet reported).
//
// Lookup order:
//  1. Release status (in-memory, from previous reconcile cycle)
//  2. Installed-state registry in etcd (canonical source, written by Node Agent)
func (srv *server) getInstalledVersionForRelease(rel *cluster_controllerpb.ServiceRelease, nodeID string) string {
	if rel.Status != nil {
		for _, nrs := range rel.Status.Nodes {
			if nrs != nil && nrs.NodeID == nodeID && nrs.InstalledVersion != "" {
				return nrs.InstalledVersion
			}
		}
	}
	// Canonical source: installed-state registry in etcd.
	canon := canonicalServiceName(rel.Spec.ServiceName)
	if pkg, err := installed_state.GetInstalledPackage(context.Background(), nodeID, "SERVICE", canon); err == nil && pkg != nil {
		if v := strings.TrimSpace(pkg.GetVersion()); v != "" {
			return v
		}
	}
	return ""
}

// getNodePlatform returns the platform string for a node (e.g., "linux_amd64").
// Best-effort: infers from installed packages, falls back to controller's own platform.
// Future: read from NodeStatus.Platform directly.
func (srv *server) getNodePlatform(nodeID string) string {
	// Best-effort: check installed-state for any package from this node to infer platform.
	if pkgs, err := installed_state.ListInstalledPackages(context.Background(), nodeID, "SERVICE"); err == nil {
		for _, p := range pkgs {
			if plat := strings.TrimSpace(p.GetPlatform()); plat != "" {
				return plat
			}
		}
	}
	return runtime.GOOS + "_" + runtime.GOARCH
}

// repositoryAddrForSpec returns the repository gRPC endpoint for the given spec.
// Falls back to the empty string, which causes ReleaseResolver to use its default.
func repositoryAddrForSpec(spec *cluster_controllerpb.ServiceReleaseSpec) string {
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

// ── ApplicationRelease reconciler (unified pipeline) ─────────────────────────

// reconcileAppRelease drives the phase state machine for one ApplicationRelease
// using the shared release pipeline.
func (srv *server) reconcileAppRelease(ctx context.Context, releaseName string) {
	if srv.resources == nil {
		return
	}
	obj, _, err := srv.resources.Get(ctx, "ApplicationRelease", releaseName)
	if err != nil {
		log.Printf("app-release %s: get: %v", releaseName, err)
		return
	}
	if obj == nil {
		return
	}
	rel, ok := obj.(*cluster_controllerpb.ApplicationRelease)
	if !ok || rel.Spec == nil {
		log.Printf("app-release %s: unexpected object type", releaseName)
		return
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.ApplicationReleaseStatus{}
	}

	h := srv.appReleaseHandle(rel)

	if h.Removing && h.Phase != ReleasePhaseRemoving && h.Phase != ReleasePhaseRemoved {
		srv.reconcileRemoving(ctx, h)
		return
	}

	switch h.Phase {
	case "", cluster_controllerpb.ReleasePhasePending:
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		srv.reconcileApplying(ctx, h)
	case cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded:
		srv.reconcileAvailable(ctx, h)
	case ReleasePhaseRemoving:
		srv.reconcileRemoving(ctx, h)
	case ReleasePhaseRemoved:
		if err := srv.resources.Delete(ctx, "ApplicationRelease", releaseName); err != nil {
			log.Printf("app-release %s: garbage-collect failed: %v", releaseName, err)
		} else {
			log.Printf("app-release %s: garbage-collected (REMOVED)", releaseName)
		}
	}
}

func (srv *server) patchAppReleaseStatus(ctx context.Context, releaseName string, f func(*cluster_controllerpb.ApplicationReleaseStatus)) error {
	obj, _, err := srv.resources.Get(ctx, "ApplicationRelease", releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get app release %s for status patch: %w", releaseName, err)
	}
	rel, ok := obj.(*cluster_controllerpb.ApplicationRelease)
	if !ok {
		return fmt.Errorf("unexpected type for app release %s", releaseName)
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.ApplicationReleaseStatus{}
	}
	previousPhase := rel.Status.Phase
	f(rel.Status)

	if rel.Status.Phase != previousPhase {
		if err := srv.emitPhaseTransition(releaseName, previousPhase, rel.Status.Phase, rel.Status.Message); err != nil {
			return fmt.Errorf("app-release %s: %w", releaseName, err)
		}
	}

	_, err = srv.resources.Apply(ctx, "ApplicationRelease", rel)
	return err
}

func appRepoAddr(spec *cluster_controllerpb.ApplicationReleaseSpec) string {
	if spec != nil && strings.TrimSpace(spec.RepositoryID) != "" {
		return strings.TrimSpace(spec.RepositoryID)
	}
	return ""
}

// ── InfrastructureRelease reconciler (unified pipeline) ──────────────────────

// reconcileInfraRelease drives the phase state machine for one InfrastructureRelease
// using the shared release pipeline.
func (srv *server) reconcileInfraRelease(ctx context.Context, releaseName string) {
	if srv.resources == nil {
		return
	}
	obj, _, err := srv.resources.Get(ctx, "InfrastructureRelease", releaseName)
	if err != nil {
		log.Printf("infra-release %s: get: %v", releaseName, err)
		return
	}
	if obj == nil {
		return
	}
	rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
	if !ok || rel.Spec == nil {
		log.Printf("infra-release %s: unexpected object type", releaseName)
		return
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.InfrastructureReleaseStatus{}
	}

	h := srv.infraReleaseHandle(rel)

	if h.Removing && h.Phase != ReleasePhaseRemoving && h.Phase != ReleasePhaseRemoved {
		srv.reconcileRemoving(ctx, h)
		return
	}

	switch h.Phase {
	case "", cluster_controllerpb.ReleasePhasePending:
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		srv.reconcileApplying(ctx, h)
	case cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded:
		srv.reconcileAvailable(ctx, h)
	case ReleasePhaseRemoving:
		srv.reconcileRemoving(ctx, h)
	case ReleasePhaseRemoved:
		if err := srv.resources.Delete(ctx, "InfrastructureRelease", releaseName); err != nil {
			log.Printf("infra-release %s: garbage-collect failed: %v", releaseName, err)
		} else {
			log.Printf("infra-release %s: garbage-collected (REMOVED)", releaseName)
		}
	}
}

func (srv *server) patchInfraReleaseStatus(ctx context.Context, releaseName string, f func(*cluster_controllerpb.InfrastructureReleaseStatus)) error {
	obj, _, err := srv.resources.Get(ctx, "InfrastructureRelease", releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get infra release %s for status patch: %w", releaseName, err)
	}
	rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
	if !ok {
		return fmt.Errorf("unexpected type for infra release %s", releaseName)
	}
	if rel.Status == nil {
		rel.Status = &cluster_controllerpb.InfrastructureReleaseStatus{}
	}
	previousPhase := rel.Status.Phase
	f(rel.Status)

	if rel.Status.Phase != previousPhase {
		if err := srv.emitPhaseTransition(releaseName, previousPhase, rel.Status.Phase, rel.Status.Message); err != nil {
			return fmt.Errorf("infra-release %s: %w", releaseName, err)
		}
	}

	_, err = srv.resources.Apply(ctx, "InfrastructureRelease", rel)
	return err
}

func infraRepoAddr(spec *cluster_controllerpb.InfrastructureReleaseSpec) string {
	if spec != nil && strings.TrimSpace(spec.RepositoryID) != "" {
		return strings.TrimSpace(spec.RepositoryID)
	}
	return ""
}

// checkNodePlanStatuses inspects the plan store for each node in the list and
// returns updated statuses plus counts of succeeded, failed, rolledBack, and running nodes.
// When resolvedVersion is non-empty and a plan succeeds, InstalledVersion is set.
// PLAN_ROLLED_BACK is counted separately from failures so the caller can
// distinguish full rollback (ROLLED_BACK) from mixed outcomes (DEGRADED).
func (srv *server) checkNodePlanStatuses(ctx context.Context, nodes []*cluster_controllerpb.NodeReleaseStatus, resolvedVersion string) (updated []*cluster_controllerpb.NodeReleaseStatus, succeeded, failed, rolledBack, running int) {
	for _, nrs := range nodes {
		if nrs == nil {
			continue
		}
		u := *nrs
		ps, err := srv.planStore.GetStatus(ctx, nrs.NodeID)

		// Plan not found: treat as externally completed (the plan store was
		// wiped or the plan expired). The drift detector will catch genuinely
		// missing services.
		if err != nil || ps == nil {
			succeeded++
			u.Phase = cluster_controllerpb.ReleasePhaseAvailable
			u.InstalledVersion = resolvedVersion
			u.ErrorMessage = ""
			u.FailedStepID = ""
			u.UpdatedUnixMs = time.Now().UnixMilli()
			updated = append(updated, &u)
			continue
		}

		// Plan ID mismatch: another release's plan overwrote ours in the
		// single-plan-per-node store. If the current plan succeeded, the
		// node is converged — treat this release as succeeded too.
		if nrs.PlanID != "" && ps.GetPlanId() != nrs.PlanID {
			if ps.GetState() == planpb.PlanState_PLAN_SUCCEEDED {
				succeeded++
				u.Phase = cluster_controllerpb.ReleasePhaseAvailable
				u.InstalledVersion = resolvedVersion
				u.ErrorMessage = ""
				u.UpdatedUnixMs = time.Now().UnixMilli()
			} else {
				running++
			}
			updated = append(updated, &u)
			continue
		}
		switch ps.GetState() {
		case planpb.PlanState_PLAN_SUCCEEDED:
			succeeded++
			u.Phase = cluster_controllerpb.ReleasePhaseAvailable
			u.InstalledVersion = resolvedVersion
			u.ErrorMessage = ""
			u.FailedStepID = ""
			u.UpdatedUnixMs = time.Now().UnixMilli()
		case planpb.PlanState_PLAN_ROLLED_BACK:
			rolledBack++
			u.Phase = cluster_controllerpb.ReleasePhaseRolledBack
			u.ErrorMessage = ps.GetErrorMessage()
			u.FailedStepID = ps.GetErrorStepId()
			u.UpdatedUnixMs = time.Now().UnixMilli()
		case planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_EXPIRED:
			failed++
			u.Phase = cluster_controllerpb.ReleasePhaseFailed
			u.ErrorMessage = ps.GetErrorMessage()
			u.FailedStepID = ps.GetErrorStepId()
			u.UpdatedUnixMs = time.Now().UnixMilli()
		default:
			running++
		}
		updated = append(updated, &u)
	}
	return
}
