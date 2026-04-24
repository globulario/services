package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
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
	// Enqueue order matters: infrastructure first, then foundational services
	// (event, dns, rbac, file), then everything else. Without this, alphabetically-
	// early services like ai-executor get dispatched before dns/event, and the
	// one-plan-per-node guard blocks convergence.

	// 1. Infrastructure releases first, sorted by catalog priority.
	// Skip AVAILABLE infra releases to avoid re-processing on restart.
	// Priority order ensures etcd converges before gateway, gateway before workloads.
	if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		type infraItem struct {
			key      string
			priority int
		}
		var pending []infraItem
		for _, obj := range items {
			rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
			if !ok || rel.Meta == nil {
				continue
			}
			if rel.Status != nil && rel.Status.Phase == cluster_controllerpb.ReleasePhaseAvailable {
				continue
			}
			pri := 999
			if rel.Spec != nil {
				if entry := CatalogByName(rel.Spec.Component); entry != nil {
					pri = entry.Priority
				}
			}
			pending = append(pending, infraItem{
				key:      infraReleaseKeyPrefix + rel.Meta.Name,
				priority: pri,
			})
		}
		sort.Slice(pending, func(i, j int) bool {
			return pending[i].priority < pending[j].priority
		})
		for _, item := range pending {
			reconcileEnqueueTotal.WithLabelValues("initial").Inc()
			queue.Enqueue(item.key)
		}
	}
	safeGo("watch-infrastructure-release", func() {
		ch, err := srv.resources.Watch(ctx, "InfrastructureRelease", "", "")
		if err != nil {
			return
		}
		for evt := range ch {
			if rel, ok := evt.Object.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Meta != nil {
				reconcileEnqueueTotal.WithLabelValues("watch").Inc()
				queue.Enqueue(infraReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	})

	// 2. Foundational service releases (event, dns, rbac, file), then the rest.
	// Skip releases that are already converged (AVAILABLE with matching installed
	// version) to avoid re-processing all 41 services on restart.
	if items, _, err := srv.resources.List(ctx, "ServiceRelease", ""); err == nil {
		var foundational, rest []string
		for _, obj := range items {
			rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
			if !ok || rel.Meta == nil {
				continue
			}
			// Skip already-converged releases: AVAILABLE phase means all
			// nodes have been served. No need to re-process on restart.
			if rel.Status != nil && rel.Status.Phase == cluster_controllerpb.ReleasePhaseAvailable {
				continue
			}
			key := releaseKeyPrefix + rel.Meta.Name
			name := rel.Meta.Name
			// Strip publisher prefix: "core@globular.io/event" → "event"
			if idx := strings.LastIndex(name, "/"); idx >= 0 {
				name = name[idx+1:]
			}
			if isFoundationalService(name) {
				foundational = append(foundational, key)
			} else {
				rest = append(rest, key)
			}
		}
		for _, k := range foundational {
			reconcileEnqueueTotal.WithLabelValues("initial").Inc()
			queue.Enqueue(k)
		}
		for _, k := range rest {
			reconcileEnqueueTotal.WithLabelValues("initial").Inc()
			queue.Enqueue(k)
		}
	}
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
				reconcileEnqueueTotal.WithLabelValues("watch").Inc()
				queue.Enqueue(releaseKeyPrefix + rel.Meta.Name)
			}
		}
		log.Printf("watch-service-release: channel closed, exiting")
	})

	// 3. Application releases last.
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
				reconcileEnqueueTotal.WithLabelValues("watch").Inc()
				queue.Enqueue(appReleaseKeyPrefix + rel.Meta.Name)
			}
		}
	})
}

// reconcileRelease drives the phase state machine for one ServiceRelease
// using the shared release pipeline.
// Called from the worker goroutine when the queue key has the "release/" prefix.
func (srv *server) reconcileRelease(ctx context.Context, releaseName string) {
	if !srv.mustBeLeader() {
		return
	}
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
	case cluster_controllerpb.ReleasePhaseWaiting:
		// Backoff: artifact was not found in the repository. Retry after
		// releaseWaitingBackoff to avoid hammering the repository on every
		// reconcile cycle while the artifact is being published.
		if h.LastTransitionUnixMs > 0 {
			elapsed := time.Since(time.UnixMilli(h.LastTransitionUnixMs))
			if elapsed < releaseWaitingBackoff {
				return
			}
		}
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		// Backoff for transient workflow errors (circuit breaker open, Scylla down).
		// NextRetryUnixMs is set by the "retry" patch with an exponential backoff
		// schedule. While it's in the future, skip dispatch entirely — the release
		// will be re-enqueued by requeueFailedReleases once the window passes.
		if rel.Status.NextRetryUnixMs > 0 && time.Now().UnixMilli() < rel.Status.NextRetryUnixMs {
			return
		}
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		// Workflow is executing; its callbacks will normally transition the
		// release to AVAILABLE/FAILED. However, if the desired state changed
		// (generation advanced) while the workflow is in-flight, the generation
		// guard blocks all callback writes, leaving the release stuck in
		// APPLYING. Detect this and re-enter PENDING so a new workflow is
		// dispatched once the in-flight slot clears.
		if h.Generation > h.ObservedGeneration {
			log.Printf("release %s: APPLYING → PENDING (generation %d > observed %d, desired state changed mid-flight)",
				h.Name, h.Generation, h.ObservedGeneration)
			srv.cancelInflightWorkflow(fmt.Sprintf("%s/%s", h.ResourceType, h.Name))
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed_mid_flight",
				SetFields:        "phase",
			})
			return
		}
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
		// Backoff: wait at least releaseRetryBackoff since the last transition
		// to avoid a tight FAILED→PENDING→FAILED loop that starves heartbeats.
		if h.LastTransitionUnixMs > 0 {
			elapsed := time.Since(time.UnixMilli(h.LastTransitionUnixMs))
			if elapsed < releaseRetryBackoff {
				return // too soon, let the next reconcile cycle handle it
			}
		}
		// Re-enter PENDING if the spec generation advanced (explicit re-apply).
		if h.Generation > h.ObservedGeneration {
			log.Printf("release %s: %s → PENDING (generation %d > observed %d)",
				releaseName, h.Phase, h.Generation, h.ObservedGeneration)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed",
				SetFields:        "phase",
			})
			return
		}
		// Auto-retry: if there are still unserved nodes, re-enter PENDING.
		if srv.hasUnservedNodes(h) {
			log.Printf("release %s: %s → PENDING (auto-retry, unserved nodes remain)",
				releaseName, h.Phase)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "auto_retry",
				SetFields:        "phase",
			})
		} else {
			// All nodes are already at the desired version (convergence signal #2).
			// The release previously failed (e.g., unit start timeout), but nodes
			// have since self-healed. Re-enter PENDING to trigger a no-op workflow
			// run that will advance the release to AVAILABLE.
			log.Printf("release %s: %s → PENDING (all nodes converged, clearing failure)",
				releaseName, h.Phase)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "self_healed_converged",
				SetFields:        "phase",
			})
		}
	}
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
	nodes := rel.Status.Nodes
	total := len(nodes)
	minReplicas := total
	if rel.Spec != nil && rel.Spec.MaxUnavailable > 0 && int(rel.Spec.MaxUnavailable) < total {
		minReplicas = total - int(rel.Spec.MaxUnavailable)
	}
	if minReplicas < 1 && total > 0 {
		minReplicas = 1
	}

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

		versionMatch := false
		healthy := false
		serviceHealthy := false
		if node != nil && rel.Spec != nil {
			healthy = strings.EqualFold(node.Status, "ready")
			serviceHealthy = srv.serviceHealthyForRelease(node, rel)
			// A node is serving this release if it reports the resolved version
			// for the release's service. AppliedServicesHash (cluster-wide) is
			// a different hash domain and cannot be compared to the per-release
			// DesiredHash directly.
			if node.InstalledVersions != nil && rel.Status.ResolvedVersion != "" {
				svcName := rel.Spec.ServiceName
				if installed, ok := node.InstalledVersions[svcName]; ok && installed == rel.Status.ResolvedVersion {
					versionMatch = true
				}
			}
		}
		if versionMatch && healthy && serviceHealthy {
			ok++
			nCopy.Phase = cluster_controllerpb.ReleasePhaseAvailable
		} else {
			issues++
			// Drift detected — mark degraded so reconcile pipeline re-enters PENDING
			// and the workflow handles re-installation.
			log.Printf("release %s: node %s drift detected, marking DEGRADED for workflow re-apply", name, nodeID)
			nCopy.Phase = cluster_controllerpb.ReleasePhaseDegraded
			nCopy.UpdatedUnixMs = time.Now().UnixMilli()
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

// Deprecated: plan dispatch functions removed — release pipeline uses workflows.
// dispatchReleasePlan, hasActivePlanWithLockFn, dispatchReleasePlanFn deleted.

// patchReleaseStatus loads the latest copy of a ServiceRelease, applies f to its status,
// and saves via resources.Apply. This avoids conflicting with concurrent writes.
//
// Equality guard: if f mutates nothing (same phase, message, and transition
// reason), the Apply is skipped entirely. This prevents no-op "retry" patches
// from emitting MODIFIED watch events that re-enqueue the release and create
// a reconcile storm when the workflow circuit breaker is open.
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
	prevMsg := rel.Status.Message
	prevTransReason := rel.Status.TransitionReason
	prevNextRetry := rel.Status.NextRetryUnixMs
	f(rel.Status)

	// Equality guard: skip Apply when nothing semantically changed.
	// "retry" patches always increment RetryCount and advance NextRetryUnixMs,
	// so they always pass this guard and persist the new backoff window.
	// Truly no-op calls (same phase + message + reason + next-retry) are skipped.
	if rel.Status.Phase == previousPhase &&
		rel.Status.Message == prevMsg &&
		rel.Status.TransitionReason == prevTransReason &&
		rel.Status.NextRetryUnixMs == prevNextRetry {
		return nil
	}

	// Hard enforcement: invalid transition blocks the patch.
	if rel.Status.Phase != previousPhase {
		if err := srv.emitPhaseTransition(releaseName, previousPhase, rel.Status.Phase, rel.Status.Message); err != nil {
			// Record the rejected transition so AI diagnostics can see it.
			if srv.workflowRec != nil {
				srv.workflowRec.RecordPhaseTransition(ctx, "ServiceRelease", releaseName,
					previousPhase, rel.Status.Phase, rel.Status.TransitionReason,
					callerFunc(2), true)
			}
			return fmt.Errorf("release %s: %w", releaseName, err)
		}
		// Record successful transition.
		if srv.workflowRec != nil {
			srv.workflowRec.RecordPhaseTransition(ctx, "ServiceRelease", releaseName,
				previousPhase, rel.Status.Phase, rel.Status.TransitionReason,
				callerFunc(2), false)
		}
	}

	_, err = srv.resources.Apply(ctx, "ServiceRelease", rel)
	return err
}

// callerFunc returns the name of the function N levels up the stack.
// Used to tag phase transitions with their source for diagnostics.
func callerFunc(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	if fn := runtime.FuncForPC(pc); fn != nil {
		name := fn.Name()
		// Keep only the short name after the last "."
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			return name[idx+1:]
		}
		return name
	}
	return ""
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
// hasAnyActivePlan returns true when the node's single plan slot is occupied
// by a non-terminal plan (PENDING, RUNNING, or ROLLING_BACK), regardless of
// lock key. This prevents multiple releases from overwriting each other in
// the one-plan-per-node etcd slot.
// isFoundationalService returns true for services that most other services
// depend on and should be installed first (event bus, dns, rbac, file).
func isFoundationalService(name string) bool {
	switch name {
	case "event", "dns", "rbac", "file", "discovery", "monitoring", "repository", "resource", "authentication":
		return true
	}
	return false
}

// Deprecated: hasAnyActivePlan, hasActivePlanWithLock deleted — workflow
// orchestration replaces plan-slot guards.

// ── ApplicationRelease reconciler (unified pipeline) ─────────────────────────

// reconcileAppRelease drives the phase state machine for one ApplicationRelease
// using the shared release pipeline.
func (srv *server) reconcileAppRelease(ctx context.Context, releaseName string) {
	if !srv.mustBeLeader() {
		return
	}
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
	case cluster_controllerpb.ReleasePhaseWaiting:
		if h.LastTransitionUnixMs > 0 {
			elapsed := time.Since(time.UnixMilli(h.LastTransitionUnixMs))
			if elapsed < releaseWaitingBackoff {
				return
			}
		}
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		// Workflow is executing; its callbacks will normally transition the
		// release to AVAILABLE/FAILED. However, if the desired state changed
		// (generation advanced) while the workflow is in-flight, the generation
		// guard blocks all callback writes, leaving the release stuck in
		// APPLYING. Detect this and re-enter PENDING so a new workflow is
		// dispatched once the in-flight slot clears.
		if h.Generation > h.ObservedGeneration {
			log.Printf("release %s: APPLYING → PENDING (generation %d > observed %d, desired state changed mid-flight)",
				h.Name, h.Generation, h.ObservedGeneration)
			srv.cancelInflightWorkflow(fmt.Sprintf("%s/%s", h.ResourceType, h.Name))
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed_mid_flight",
				SetFields:        "phase",
			})
			return
		}
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
	prevMsg := rel.Status.Message
	prevReason := rel.Status.TransitionReason
	f(rel.Status)

	// Equality guard: skip Apply when nothing meaningful changed.
	if rel.Status.Phase == previousPhase &&
		rel.Status.Message == prevMsg &&
		rel.Status.TransitionReason == prevReason {
		return nil
	}

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
	if !srv.mustBeLeader() {
		return
	}
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
	case cluster_controllerpb.ReleasePhaseWaiting:
		if h.LastTransitionUnixMs > 0 {
			elapsed := time.Since(time.UnixMilli(h.LastTransitionUnixMs))
			if elapsed < releaseWaitingBackoff {
				return
			}
		}
		srv.reconcilePending(ctx, h)
	case cluster_controllerpb.ReleasePhaseResolved:
		srv.reconcileResolved(ctx, h)
	case cluster_controllerpb.ReleasePhaseApplying:
		// Workflow is executing; its callbacks will normally transition the
		// release to AVAILABLE/FAILED. However, if the desired state changed
		// (generation advanced) while the workflow is in-flight, the generation
		// guard blocks all callback writes, leaving the release stuck in
		// APPLYING. Detect this and re-enter PENDING so a new workflow is
		// dispatched once the in-flight slot clears.
		if h.Generation > h.ObservedGeneration {
			log.Printf("release %s: APPLYING → PENDING (generation %d > observed %d, desired state changed mid-flight)",
				h.Name, h.Generation, h.ObservedGeneration)
			srv.cancelInflightWorkflow(fmt.Sprintf("%s/%s", h.ResourceType, h.Name))
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed_mid_flight",
				SetFields:        "phase",
			})
			return
		}
	case cluster_controllerpb.ReleasePhaseAvailable, cluster_controllerpb.ReleasePhaseDegraded:
		srv.reconcileAvailable(ctx, h)
	case cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseRolledBack:
		// Re-enter PENDING if generation advanced (explicit re-apply).
		if h.Generation > h.ObservedGeneration {
			log.Printf("infra-release %s: %s → PENDING (generation %d > observed %d)",
				releaseName, h.Phase, h.Generation, h.ObservedGeneration)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "generation_changed",
				SetFields:        "phase",
			})
			return
		}
		// Auto-retry: if there are unserved nodes, re-enter PENDING.
		if srv.hasUnservedNodes(h) {
			log.Printf("infra-release %s: %s → PENDING (auto-retry, unserved nodes remain)",
				releaseName, h.Phase)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "auto_retry_unserved",
				SetFields:        "phase",
			})
		} else {
			// All nodes are at the desired version. Re-enter PENDING so the no-op
			// workflow run advances the release to AVAILABLE (FAILED → AVAILABLE
			// is not a valid direct transition).
			log.Printf("infra-release %s: %s → PENDING (all nodes converged, clearing failure)",
				releaseName, h.Phase)
			h.PatchStatus(ctx, statusPatch{
				Phase:            cluster_controllerpb.ReleasePhasePending,
				TransitionReason: "self_healed_converged",
				SetFields:        "phase",
			})
		}
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
	prevMsg := rel.Status.Message
	prevReason := rel.Status.TransitionReason
	f(rel.Status)

	// Equality guard: skip Apply when nothing meaningful changed.
	if rel.Status.Phase == previousPhase &&
		rel.Status.Message == prevMsg &&
		rel.Status.TransitionReason == prevReason {
		return nil
	}

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

// requeueFailedReleases scans all ServiceRelease and InfrastructureRelease objects
// for entries stuck in FAILED/ROLLED_BACK phase or in RESOLVED+transient-error
// state that have exceeded their retry backoff. The watcher-driven work queue
// only fires on etcd changes; without this, a FAILED or transiently-blocked
// RESOLVED release that was processed at startup is never retried.
// Called from the periodic-release-bridge every 2 minutes.
func (srv *server) requeueFailedReleases(ctx context.Context) {
	if srv.resources == nil || srv.releaseEnqueue == nil {
		return
	}
	now := time.Now()
	requeued := 0
	transientBlocked := 0

	if items, _, err := srv.resources.List(ctx, "ServiceRelease", ""); err == nil {
		for _, obj := range items {
			rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
			if !ok || rel.Meta == nil || rel.Status == nil {
				continue
			}
			phase := rel.Status.Phase
			switch phase {
			case cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseRolledBack:
				if rel.Status.LastTransitionUnixMs > 0 {
					elapsed := now.Sub(time.UnixMilli(rel.Status.LastTransitionUnixMs))
					if elapsed < releaseRetryBackoff {
						continue
					}
				}
			case cluster_controllerpb.ReleasePhaseResolved:
				// Re-enqueue RESOLVED releases blocked on a transient workflow error
				// once their NextRetryUnixMs has passed. This is how the release
				// resumes after the circuit breaker closes or Scylla recovers.
				if rel.Status.NextRetryUnixMs <= 0 {
					continue // no retry pending
				}
				if now.UnixMilli() < rel.Status.NextRetryUnixMs {
					transientBlocked++ // still inside backoff window
					continue
				}
			default:
				continue
			}
			reconcileEnqueueTotal.WithLabelValues("retry_failed").Inc()
			srv.releaseEnqueue(rel.Meta.Name)
			requeued++
		}
	}
	releaseTransientBlockedGauge.Set(float64(transientBlocked))
	if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range items {
			rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
			if !ok || rel.Meta == nil || rel.Status == nil {
				continue
			}
			phase := rel.Status.Phase
			if phase != cluster_controllerpb.ReleasePhaseFailed && phase != cluster_controllerpb.ReleasePhaseRolledBack {
				continue
			}
			if rel.Status.LastTransitionUnixMs > 0 {
				elapsed := now.Sub(time.UnixMilli(rel.Status.LastTransitionUnixMs))
				if elapsed < releaseRetryBackoff {
					continue
				}
			}
			reconcileEnqueueTotal.WithLabelValues("retry_failed").Inc()
			srv.infraReleaseEnqueue(rel.Meta.Name)
			requeued++
		}
	}
	if requeued > 0 {
		log.Printf("periodic-release-bridge: re-queued %d FAILED/ROLLED_BACK release(s) past retry backoff", requeued)
	}
}

// Deprecated: checkNodePlanStatuses deleted — workflow results are authoritative.
