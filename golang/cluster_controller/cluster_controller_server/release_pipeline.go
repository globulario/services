package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
)

// releaseHandle is a type-erased view of a release object (ServiceRelease,
// ApplicationRelease, or InfrastructureRelease) that the unified pipeline
// operates on. Each typed reconciler builds a handle, then calls the shared
// pipeline steps.
type releaseHandle struct {
	// Identity
	Name         string
	ResourceType string // "ServiceRelease", "ApplicationRelease", "InfrastructureRelease"
	Generation   int64
	Paused       bool

	// Current status (read from the typed status)
	Phase                  string
	ObservedGeneration     int64
	ResolvedVersion        string
	ResolvedArtifactDigest string
	DesiredHash            string
	Nodes                  []*cluster_controllerpb.NodeReleaseStatus

	// Resolve parameters (normalized to the common resolver shape)
	ResolverSpec   *cluster_controllerpb.ServiceReleaseSpec
	RepositoryAddr string

	// Lock key for plan dispatch conflict guard (e.g. "service:gateway")
	LockKey string

	// Installed-state lookup parameters for the canonical etcd registry.
	InstalledStateKind string // "SERVICE", "APPLICATION", "INFRASTRUCTURE"
	InstalledStateName string // canonical package name for installed-state lookup

	// Removing flag: when true, the release is being uninstalled.
	Removing bool

	// Type-specific callbacks
	ComputeHash func(resolvedVersion string) string
	CompilePlan func(nodeID, installedVersion, clusterID string) (*planpb.NodePlan, error)

	// CompileUninstallPlan builds a removal plan for the given node.
	// Only set when the release kind supports removal workflows.
	CompileUninstallPlan func(nodeID, clusterID string) (*planpb.NodePlan, error)

	// DriftDetector is an optional callback for hash+health drift detection.
	// Called from reconcileAvailable for ServiceRelease (nil for App/Infra).
	DriftDetector func(ctx context.Context, h *releaseHandle) bool

	// Status writer: patches the typed status in the resource store.
	// The callback receives a statusPatch that the pipeline fills in.
	PatchStatus func(ctx context.Context, patch statusPatch) error
}

// statusPatch describes the status update the pipeline wants to apply.
// The typed PatchStatus callback maps this to the correct typed status struct.
type statusPatch struct {
	Phase                  string
	ResolvedVersion        string
	ResolvedArtifactDigest string
	DesiredHash            string
	ObservedGeneration     int64
	Message                string
	Nodes                  []*cluster_controllerpb.NodeReleaseStatus
	LastTransitionUnixMs   int64
	WorkflowKind           string
	StartedAtUnixMs        int64
	TransitionReason       string
	// SetFields controls which fields are meaningful in this patch.
	// "resolve" = version/digest/hash/generation, "phase" = just phase,
	// "nodes" = phase + nodes, "fail" = phase + message.
	SetFields string
}

// computeWorkflowKind determines whether this is an install, upgrade, or remove workflow.
func computeWorkflowKind(h *releaseHandle) string {
	if h.Removing {
		return "remove"
	}
	// Check if any node already has an installed version — if so, upgrade.
	for _, n := range h.Nodes {
		if n != nil && n.InstalledVersion != "" {
			return "upgrade"
		}
	}
	// Check installed-state registry.
	if h.InstalledStateKind != "" && h.InstalledStateName != "" {
		if pkg, err := installed_state.GetInstalledPackage(context.Background(), "", h.InstalledStateKind, h.InstalledStateName); err == nil && pkg != nil {
			if v := strings.TrimSpace(pkg.GetVersion()); v != "" {
				return "upgrade"
			}
		}
	}
	return "install"
}

// reconcilePending is the shared PENDING phase: resolve version and artifact
// digest via ReleaseResolver, compute desired hash, transition to RESOLVED.
func (srv *server) reconcilePending(ctx context.Context, h *releaseHandle) {
	nowMs := time.Now().UnixMilli()
	wfKind := computeWorkflowKind(h)

	// Idempotency guard: skip re-resolution if already resolved for this generation.
	if h.ObservedGeneration == h.Generation &&
		h.ResolvedVersion != "" &&
		h.ResolvedArtifactDigest != "" {
		h.PatchStatus(ctx, statusPatch{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			TransitionReason: "already_resolved",
			WorkflowKind:     wfKind,
			SetFields:        "phase",
		})
		return
	}

	resolver := &ReleaseResolver{RepositoryAddr: h.RepositoryAddr}
	resolved, err := resolver.Resolve(ctx, h.ResolverSpec)
	if err != nil {
		log.Printf("%s %s: resolve failed: %v", h.ResourceType, h.Name, err)
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              fmt.Sprintf("resolve: %v", err),
			LastTransitionUnixMs: nowMs,
			TransitionReason:     "resolve_failed",
			WorkflowKind:         wfKind,
			StartedAtUnixMs:      nowMs,
			SetFields:            "fail",
		})
		return
	}

	desiredHash := h.ComputeHash(resolved.Version)
	h.PatchStatus(ctx, statusPatch{
		Phase:                  cluster_controllerpb.ReleasePhaseResolved,
		ResolvedVersion:        resolved.Version,
		ResolvedArtifactDigest: resolved.Digest,
		DesiredHash:            desiredHash,
		ObservedGeneration:     h.Generation,
		Message:                "",
		LastTransitionUnixMs:   nowMs,
		TransitionReason:       "resolved",
		WorkflowKind:           wfKind,
		StartedAtUnixMs:        nowMs,
		SetFields:              "resolve",
	})
}

// reconcileResolved is the shared RESOLVED phase: compile and dispatch plans
// to all target nodes, transition to APPLYING.
func (srv *server) reconcileResolved(ctx context.Context, h *releaseHandle) {
	srv.lock("release-pipeline:snapshot")
	// Collect eligible nodes. For service/application releases, skip nodes
	// that haven't completed bootstrap (infra not ready). Infrastructure
	// releases are always dispatched — they're what gets nodes TO ready.
	isWorkload := h.ResourceType == "ServiceRelease" || h.ResourceType == "ApplicationRelease"
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	for id, node := range srv.state.Nodes {
		if isWorkload && !bootstrapPhaseReady(node.BootstrapPhase) {
			log.Printf("%s %s: skipping node %s (bootstrap_phase=%s, not ready for workloads)",
				h.ResourceType, h.Name, id, node.BootstrapPhase)
			continue
		}
		nodeIDs = append(nodeIDs, id)
	}
	// Use the cluster domain (not the UUID cluster ID) — the node-agent
	// validates plan.ClusterId against its local cluster domain.
	clusterID := srv.state.ClusterNetworkSpec.GetClusterDomain()
	if clusterID == "" {
		// Fallback: read domain from config (same source the node-agent uses).
		if d, err := config.GetDomain(); err == nil && d != "" {
			clusterID = d
		}
	}
	srv.unlock()

	if len(nodeIDs) == 0 {
		return
	}

	nodeStatuses := make([]*cluster_controllerpb.NodeReleaseStatus, 0, len(nodeIDs))

	for _, nodeID := range nodeIDs {
		if srv.hasActivePlanWithLock(ctx, nodeID, h.LockKey) {
			log.Printf("%s %s: node %s: lock %q held by active plan, will retry",
				h.ResourceType, h.Name, nodeID, h.LockKey)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseApplying,
				ErrorMessage:  fmt.Sprintf("waiting for lock %s", h.LockKey),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		installedVersion := lookupInstalledVersionForHandle(nodeID, h)
		plan, err := h.CompilePlan(nodeID, installedVersion, clusterID)
		if err != nil {
			log.Printf("%s %s: compile plan for node %s: %v", h.ResourceType, h.Name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("compile: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		if err := srv.stampAndDispatchPlan(ctx, nodeID, plan); err != nil {
			log.Printf("%s %s: persist plan for node %s: %v", h.ResourceType, h.Name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("persist plan: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}

		log.Printf("%s %s: wrote plan node=%s plan_id=%s",
			h.ResourceType, h.Name, nodeID, plan.PlanId)
		nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
			NodeID:        nodeID,
			PlanID:        plan.PlanId,
			Phase:         cluster_controllerpb.ReleasePhaseApplying,
			UpdatedUnixMs: time.Now().UnixMilli(),
		})
	}

	h.PatchStatus(ctx, statusPatch{
		Phase:                cluster_controllerpb.ReleasePhaseApplying,
		Nodes:                nodeStatuses,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		TransitionReason:     "plans_dispatched",
		SetFields:            "nodes",
	})
}

// reconcileApplying is the shared APPLYING phase: inspect per-node plan
// statuses and advance to AVAILABLE, DEGRADED, or FAILED.
func (srv *server) reconcileApplying(ctx context.Context, h *releaseHandle) {
	if len(h.Nodes) == 0 {
		// Lost node list; re-enter RESOLVED to recompile plans.
		h.PatchStatus(ctx, statusPatch{
			Phase:            cluster_controllerpb.ReleasePhaseResolved,
			TransitionReason: "node_list_lost",
			SetFields:        "phase",
		})
		return
	}

	updatedNodes, succeeded, failed, rolledBack, running := srv.checkNodePlanStatuses(ctx, h.Nodes, h.ResolvedVersion)
	total := len(updatedNodes)
	newPhase := cluster_controllerpb.ReleasePhaseApplying
	reason := ""
	switch {
	case total > 0 && succeeded == total:
		newPhase = cluster_controllerpb.ReleasePhaseAvailable
		reason = "all_nodes_succeeded"
	case total > 0 && rolledBack == total:
		newPhase = cluster_controllerpb.ReleasePhaseRolledBack
		reason = "all_nodes_rolled_back"
	case total > 0 && rolledBack > 0 && failed == 0 && running == 0:
		// Mixed success + rollback (no hard failures) → DEGRADED
		newPhase = cluster_controllerpb.ReleasePhaseDegraded
		reason = "partial_rollback"
	case failed > 0 && running == 0:
		if succeeded > 0 {
			newPhase = cluster_controllerpb.ReleasePhaseDegraded
			reason = "partial_failure"
		} else if rolledBack > 0 {
			newPhase = cluster_controllerpb.ReleasePhaseDegraded
			reason = "mixed_rollback_failure"
		} else {
			newPhase = cluster_controllerpb.ReleasePhaseFailed
			reason = "all_nodes_failed"
		}
	}

	// Only patch if the phase actually changed — otherwise we trigger a
	// watch event → re-enqueue → reconcile loop with no progress.
	if newPhase == h.Phase {
		return
	}
	h.PatchStatus(ctx, statusPatch{
		Phase:                newPhase,
		Nodes:                updatedNodes,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		TransitionReason:     reason,
		SetFields:            "nodes",
	})
}

// reconcileAvailable is the shared AVAILABLE/DEGRADED phase: detect spec
// generation drift and re-enter PENDING if the spec changed. If the handle
// carries a DriftDetector callback, it is also invoked for hash+health drift.
//
// For infrastructure releases, also detects nodes that joined after the
// release was dispatched — if an eligible node is missing the package,
// re-enter RESOLVED to dispatch plans for the new node.
func (srv *server) reconcileAvailable(ctx context.Context, h *releaseHandle) {
	if h.Generation > h.ObservedGeneration {
		h.PatchStatus(ctx, statusPatch{
			Phase:            cluster_controllerpb.ReleasePhasePending,
			TransitionReason: "generation_changed",
			SetFields:        "phase",
		})
		return
	}

	// Check for new nodes that need this release but weren't in the original
	// dispatch. This handles Day 1 join: a node joins after the release
	// reached AVAILABLE on existing nodes.
	if srv.hasUnservedNodes(h) {
		log.Printf("%s %s: new unserved node(s) detected, re-entering PENDING to dispatch",
			h.ResourceType, h.Name)
		h.PatchStatus(ctx, statusPatch{
			Phase:            cluster_controllerpb.ReleasePhasePending,
			TransitionReason: "new_node_joined",
			SetFields:        "phase",
		})
		return
	}

	if h.DriftDetector != nil {
		h.DriftDetector(ctx, h)
	}
}

// hasUnservedNodes checks if any eligible node is missing from the release's
// node status list. This detects nodes that joined after the release was
// dispatched. For workload releases, only workload-ready nodes are checked.
func (srv *server) hasUnservedNodes(h *releaseHandle) bool {
	srv.lock("hasUnservedNodes")
	defer srv.unlock()

	isWorkload := h.ResourceType == "ServiceRelease" || h.ResourceType == "ApplicationRelease"

	// Build set of nodes already tracked by this release.
	served := make(map[string]bool)
	for _, nrs := range h.Nodes {
		if nrs != nil {
			served[nrs.NodeID] = true
		}
	}

	for id, node := range srv.state.Nodes {
		if served[id] {
			continue
		}
		// Workload releases: only dispatch to ready nodes.
		if isWorkload && !bootstrapPhaseReady(node.BootstrapPhase) {
			continue
		}
		// Skip nodes that are unhealthy/unreachable — they can't execute plans.
		if node.Status == "unreachable" || node.Status == "removed" {
			continue
		}
		// Found an eligible node not yet served by this release.
		return true
	}
	return false
}

// stampAndDispatchPlan assigns an ID, generation, and issuer to a plan,
// persists it, and appends to history. Shared by all release kinds.
func (srv *server) stampAndDispatchPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	plan.PlanId = uuid.NewString()
	plan.Generation = srv.nextPlanGeneration(ctx, nodeID)
	plan.IssuedBy = "cluster-controller"
	if plan.GetCreatedUnixMs() == 0 {
		plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
	}
	if err := srv.signOrAbort(plan); err != nil {
		return err
	}
	if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
		return err
	}
	if appendable, ok := srv.planStore.(interface {
		AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	}); ok {
		_ = appendable.AppendHistory(ctx, nodeID, plan)
	}
	return nil
}

// lookupInstalledVersionForHandle queries the canonical installed-state registry
// for the given node and release handle. Falls back to the per-node release
// status if the handle carries node statuses from a previous cycle.
func lookupInstalledVersionForHandle(nodeID string, h *releaseHandle) string {
	// Check release status first (from previous reconcile cycle).
	for _, nrs := range h.Nodes {
		if nrs != nil && nrs.NodeID == nodeID && nrs.InstalledVersion != "" {
			return nrs.InstalledVersion
		}
	}
	// Canonical source: installed-state registry in etcd.
	if h.InstalledStateKind != "" && h.InstalledStateName != "" {
		if pkg, err := installed_state.GetInstalledPackage(context.Background(), nodeID, h.InstalledStateKind, h.InstalledStateName); err == nil && pkg != nil {
			if v := strings.TrimSpace(pkg.GetVersion()); v != "" {
				return v
			}
		}
	}
	return ""
}

// ── Adapters: build releaseHandle from typed releases ────────────────────────

func (srv *server) appReleaseHandle(rel *cluster_controllerpb.ApplicationRelease) *releaseHandle {
	return &releaseHandle{
		Name:                   rel.Meta.Name,
		ResourceType:           "ApplicationRelease",
		Generation:             rel.Meta.Generation,
		Removing:               rel.Spec.Removing,
		Phase:                  rel.Status.Phase,
		ObservedGeneration:     rel.Status.ObservedGeneration,
		ResolvedVersion:        rel.Status.ResolvedVersion,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         appRepoAddr(rel.Spec),
		LockKey:                fmt.Sprintf("application:%s", rel.Spec.AppName),
		InstalledStateKind:     "APPLICATION",
		InstalledStateName:     rel.Spec.AppName,
		ResolverSpec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID:  rel.Spec.PublisherID,
			ServiceName:  rel.Spec.AppName,
			Version:      rel.Spec.Version,
			Platform:     rel.Spec.Platform,
			RepositoryID: rel.Spec.RepositoryID,
		},
		ComputeHash: func(resolvedVersion string) string {
			return ComputeApplicationDesiredHash(rel.Spec.PublisherID, rel.Spec.AppName, resolvedVersion)
		},
		CompilePlan: func(nodeID, installedVersion, clusterID string) (*planpb.NodePlan, error) {
			return CompileApplicationPlan(nodeID, rel, installedVersion, clusterID)
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			return srv.patchAppReleaseStatus(ctx, rel.Meta.Name, func(s *cluster_controllerpb.ApplicationReleaseStatus) {
				applyPatchToAppStatus(s, p)
			})
		},
	}
}

func (srv *server) infraReleaseHandle(rel *cluster_controllerpb.InfrastructureRelease) *releaseHandle {
	return &releaseHandle{
		Name:                   rel.Meta.Name,
		ResourceType:           "InfrastructureRelease",
		Generation:             rel.Meta.Generation,
		Removing:               rel.Spec.Removing,
		Phase:                  rel.Status.Phase,
		ObservedGeneration:     rel.Status.ObservedGeneration,
		ResolvedVersion:        rel.Status.ResolvedVersion,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         infraRepoAddr(rel.Spec),
		LockKey:                fmt.Sprintf("infrastructure:%s", rel.Spec.Component),
		InstalledStateKind:     "INFRASTRUCTURE",
		InstalledStateName:     rel.Spec.Component,
		ResolverSpec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID:  rel.Spec.PublisherID,
			ServiceName:  rel.Spec.Component,
			Version:      rel.Spec.Version,
			Platform:     rel.Spec.Platform,
			RepositoryID: rel.Spec.RepositoryID,
		},
		ComputeHash: func(resolvedVersion string) string {
			return ComputeInfrastructureDesiredHash(rel.Spec.PublisherID, rel.Spec.Component, resolvedVersion)
		},
		CompilePlan: func(nodeID, installedVersion, clusterID string) (*planpb.NodePlan, error) {
			return CompileInfrastructurePlan(nodeID, rel, installedVersion, clusterID)
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			return srv.patchInfraReleaseStatus(ctx, rel.Meta.Name, func(s *cluster_controllerpb.InfrastructureReleaseStatus) {
				applyPatchToInfraStatus(s, p)
			})
		},
	}
}

// ── Status patch helpers ─────────────────────────────────────────────────────

func applyPatchToAppStatus(s *cluster_controllerpb.ApplicationReleaseStatus, p statusPatch) {
	applyWorkflowFields := func() {
		if p.WorkflowKind != "" {
			s.WorkflowKind = p.WorkflowKind
		}
		if p.StartedAtUnixMs != 0 {
			s.StartedAtUnixMs = p.StartedAtUnixMs
		}
		if p.TransitionReason != "" {
			s.TransitionReason = p.TransitionReason
		}
	}
	switch p.SetFields {
	case "phase":
		s.Phase = p.Phase
		applyWorkflowFields()
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	}
}

func applyPatchToInfraStatus(s *cluster_controllerpb.InfrastructureReleaseStatus, p statusPatch) {
	applyWorkflowFields := func() {
		if p.WorkflowKind != "" {
			s.WorkflowKind = p.WorkflowKind
		}
		if p.StartedAtUnixMs != 0 && s.StartedAtUnixMs == 0 {
			s.StartedAtUnixMs = p.StartedAtUnixMs
		}
		if p.TransitionReason != "" {
			s.TransitionReason = p.TransitionReason
		}
	}
	switch p.SetFields {
	case "phase":
		s.Phase = p.Phase
		applyWorkflowFields()
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	}
}

// ── ServiceRelease adapter for the shared pipeline ───────────────────────────

func (srv *server) svcReleaseHandle(rel *cluster_controllerpb.ServiceRelease) *releaseHandle {
	canon := canonicalServiceName(rel.Spec.ServiceName)
	h := &releaseHandle{
		Name:                   rel.Meta.Name,
		ResourceType:           "ServiceRelease",
		Generation:             rel.Meta.Generation,
		Paused:                 rel.Spec.Paused,
		Removing:               rel.Spec.Removing,
		Phase:                  rel.Status.Phase,
		ObservedGeneration:     rel.Status.ObservedGeneration,
		ResolvedVersion:        rel.Status.ResolvedVersion,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         repositoryAddrForSpec(rel.Spec),
		LockKey:                fmt.Sprintf("service:%s", canon),
		InstalledStateKind:     "SERVICE",
		InstalledStateName:     canon,
		ResolverSpec:           rel.Spec,
		ComputeHash: func(resolvedVersion string) string {
			return ComputeReleaseDesiredHash(rel.Spec.PublisherID, rel.Spec.ServiceName, resolvedVersion, rel.Spec.Config)
		},
		CompilePlan: func(nodeID, installedVersion, clusterID string) (*planpb.NodePlan, error) {
			return CompileReleasePlan(nodeID, rel, installedVersion, clusterID)
		},
		CompileUninstallPlan: func(nodeID, clusterID string) (*planpb.NodePlan, error) {
			plan := BuildServiceRemovePlan(nodeID, canon, "")
			plan.ClusterId = clusterID
			return plan, nil
		},
		PatchStatus: func(ctx context.Context, p statusPatch) error {
			return srv.patchReleaseStatus(ctx, rel.Meta.Name, func(s *cluster_controllerpb.ServiceReleaseStatus) {
				applyPatchToSvcStatus(s, p)
			})
		},
	}
	// DriftDetector: hash+health drift (reuses existing reconcileReleaseAvailable logic).
	h.DriftDetector = func(ctx context.Context, dh *releaseHandle) bool {
		return srv.detectServiceDrift(ctx, rel, dh)
	}
	return h
}

// applyPatchToSvcStatus applies a statusPatch to a ServiceReleaseStatus.
func applyPatchToSvcStatus(s *cluster_controllerpb.ServiceReleaseStatus, p statusPatch) {
	applyWorkflowFields := func() {
		if p.WorkflowKind != "" {
			s.WorkflowKind = p.WorkflowKind
		}
		if p.StartedAtUnixMs != 0 && s.StartedAtUnixMs == 0 {
			s.StartedAtUnixMs = p.StartedAtUnixMs
		}
		if p.TransitionReason != "" {
			s.TransitionReason = p.TransitionReason
		}
	}
	switch p.SetFields {
	case "phase":
		s.Phase = p.Phase
		applyWorkflowFields()
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
	}
}

// detectServiceDrift checks hash+health drift for a ServiceRelease.
// Returns true if drift was detected and a re-plan was dispatched.
func (srv *server) detectServiceDrift(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, h *releaseHandle) bool {
	desiredHash := strings.ToLower(strings.TrimSpace(h.DesiredHash))
	nodes := h.Nodes
	total := len(nodes)
	if total == 0 {
		return false
	}

	minReplicas := total
	if rel.Spec != nil && rel.Spec.MaxUnavailable > 0 && int(rel.Spec.MaxUnavailable) < total {
		minReplicas = total - int(rel.Spec.MaxUnavailable)
	}
	if minReplicas < 1 && total > 0 {
		minReplicas = 1
	}

	targetLock := h.LockKey
	ok := 0
	issues := 0
	updatedNodes := make([]*cluster_controllerpb.NodeReleaseStatus, 0, total)

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
			if srv.planStore != nil && !srv.hasActivePlanWithLockFn(ctx, nodeID, targetLock) {
				if plan, err := srv.dispatchReleasePlanFn(ctx, rel, nodeID); err == nil && plan != nil {
					nCopy.PlanID = plan.GetPlanId()
					nCopy.Phase = cluster_controllerpb.ReleasePhaseApplying
					nCopy.UpdatedUnixMs = time.Now().UnixMilli()
				} else if err != nil {
					log.Printf("release %s: node %s drift plan compile failed: %v", h.Name, nodeID, err)
				}
			}
			if nCopy.Phase == "" {
				nCopy.Phase = cluster_controllerpb.ReleasePhaseDegraded
			}
		}
		updatedNodes = append(updatedNodes, &nCopy)
	}

	newPhase := h.Phase
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

	if newPhase == h.Phase && len(updatedNodes) == len(nodes) {
		return false
	}

	reason := "drift_detected"
	if newPhase == h.Phase {
		reason = ""
	}
	h.PatchStatus(ctx, statusPatch{
		Phase:                newPhase,
		Nodes:                updatedNodes,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		TransitionReason:     reason,
		SetFields:            "nodes",
	})
	return true
}

// ── Removal workflow ─────────────────────────────────────────────────────────

// reconcileRemoving dispatches uninstall plans and polls for completion,
// then transitions to REMOVED or FAILED.
func (srv *server) reconcileRemoving(ctx context.Context, h *releaseHandle) {
	// If we already have node statuses with PlanIDs (from a previous dispatch), poll them.
	// Nodes without PlanIDs are from the pre-removal phase and need plans dispatched.
	hasDispatchedPlans := false
	for _, n := range h.Nodes {
		if n != nil && n.PlanID != "" {
			hasDispatchedPlans = true
			break
		}
	}
	if hasDispatchedPlans {
		updatedNodes, succeeded, failed, _, running := srv.checkNodePlanStatuses(ctx, h.Nodes, "")
		total := len(updatedNodes)
		if running > 0 {
			// Still in progress — update node statuses only.
			h.PatchStatus(ctx, statusPatch{
				Phase:                ReleasePhaseRemoving,
				Nodes:                updatedNodes,
				LastTransitionUnixMs: time.Now().UnixMilli(),
				SetFields:            "nodes",
			})
			return
		}
		if total > 0 && succeeded == total {
			h.PatchStatus(ctx, statusPatch{
				Phase:                ReleasePhaseRemoved,
				Nodes:                updatedNodes,
				LastTransitionUnixMs: time.Now().UnixMilli(),
				TransitionReason:     "all_nodes_removed",
				SetFields:            "nodes",
			})
			return
		}
		if failed > 0 {
			h.PatchStatus(ctx, statusPatch{
				Phase:                cluster_controllerpb.ReleasePhaseFailed,
				Nodes:                updatedNodes,
				Message:              "removal failed on one or more nodes",
				LastTransitionUnixMs: time.Now().UnixMilli(),
				TransitionReason:     "removal_failed",
				SetFields:            "fail",
			})
			return
		}
	}

	// First entry into REMOVING: compile and dispatch uninstall plans.
	if h.CompileUninstallPlan == nil {
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              "no uninstall plan compiler available",
			LastTransitionUnixMs: time.Now().UnixMilli(),
			TransitionReason:     "no_uninstall_compiler",
			SetFields:            "fail",
		})
		return
	}

	srv.lock("release-pipeline:snapshot")
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	for id := range srv.state.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	clusterID := srv.state.ClusterNetworkSpec.GetClusterDomain()
	srv.unlock()

	if len(nodeIDs) == 0 {
		// No nodes — mark as removed.
		h.PatchStatus(ctx, statusPatch{
			Phase:                ReleasePhaseRemoved,
			LastTransitionUnixMs: time.Now().UnixMilli(),
			TransitionReason:     "no_nodes",
			SetFields:            "phase",
		})
		return
	}

	nodeStatuses := make([]*cluster_controllerpb.NodeReleaseStatus, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		plan, err := h.CompileUninstallPlan(nodeID, clusterID)
		if err != nil {
			log.Printf("%s %s: compile uninstall plan for node %s: %v", h.ResourceType, h.Name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("compile uninstall: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}
		if err := srv.stampAndDispatchPlan(ctx, nodeID, plan); err != nil {
			log.Printf("%s %s: persist uninstall plan for node %s: %v", h.ResourceType, h.Name, nodeID, err)
			nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
				NodeID:        nodeID,
				Phase:         cluster_controllerpb.ReleasePhaseFailed,
				ErrorMessage:  fmt.Sprintf("persist uninstall plan: %v", err),
				UpdatedUnixMs: time.Now().UnixMilli(),
			})
			continue
		}
		log.Printf("%s %s: wrote uninstall plan node=%s plan_id=%s", h.ResourceType, h.Name, nodeID, plan.PlanId)
		nodeStatuses = append(nodeStatuses, &cluster_controllerpb.NodeReleaseStatus{
			NodeID:        nodeID,
			PlanID:        plan.PlanId,
			Phase:         ReleasePhaseRemoving,
			UpdatedUnixMs: time.Now().UnixMilli(),
		})
	}

	h.PatchStatus(ctx, statusPatch{
		Phase:                ReleasePhaseRemoving,
		Nodes:                nodeStatuses,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		TransitionReason:     "uninstall_plans_dispatched",
		WorkflowKind:         "remove",
		StartedAtUnixMs:      time.Now().UnixMilli(),
		SetFields:            "nodes",
	})
}
