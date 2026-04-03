package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow/engine"
)

// releaseRetryBackoff is the minimum time to wait before auto-retrying a
// FAILED release. Without this, resolve errors (e.g. "cluster_id required")
// create a tight FAILED→PENDING→FAILED loop that starves other handlers.
const releaseRetryBackoff = 30 * time.Second

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
	LastTransitionUnixMs   int64
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
	ComputeHash func(resolvedVersion string, buildNumber int64) string

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

	artifactKind := repositorypb.ArtifactKind_SERVICE
	if h.ResourceType == "InfrastructureRelease" {
		artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
	} else if h.ResourceType == "ApplicationRelease" {
		artifactKind = repositorypb.ArtifactKind_APPLICATION
	}
	resolver := &ReleaseResolver{RepositoryAddr: h.RepositoryAddr, ArtifactKind: artifactKind}
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

	desiredHash := h.ComputeHash(resolved.Version, resolved.BuildNumber)
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

// reconcileResolved is the shared RESOLVED phase: execute the release
// workflow to install the package across all eligible nodes.
//
// This replaces the old plan compilation/dispatch pipeline with direct
// workflow execution. The workflow handles per-node install/verify/restart/
// sync through foreach sub-steps with gRPC callbacks to node-agents.
func (srv *server) reconcileResolved(ctx context.Context, h *releaseHandle) {
	srv.lock("release-pipeline:snapshot")
	// Collect eligible nodes — same filtering as before.
	isWorkload := h.ResourceType == "ServiceRelease" || h.ResourceType == "ApplicationRelease"
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	serviceName := h.Name
	if idx := strings.LastIndex(serviceName, "/"); idx >= 0 {
		serviceName = serviceName[idx+1:]
	}
	catalogEntry := CatalogByName(serviceName)
	for id, node := range srv.state.Nodes {
		// Skip nodes that haven't been approved yet — no packages should be
		// deployed until the join workflow advances the phase past "admitted".
		if node.BootstrapPhase == BootstrapAdmitted || node.BootstrapPhase == "" {
			log.Printf("%s %s: skipping node %s (bootstrap_phase=%s, not yet approved)",
				h.ResourceType, h.Name, id, node.BootstrapPhase)
			continue
		}
		if isWorkload && !bootstrapPhaseReady(node.BootstrapPhase) {
			log.Printf("%s %s: skipping node %s (bootstrap_phase=%s, not ready for workloads)",
				h.ResourceType, h.Name, id, node.BootstrapPhase)
			continue
		}
		if catalogEntry != nil && len(catalogEntry.Profiles) > 0 {
			expandedProfiles := normalizeProfiles(node.Profiles)
			if !profilesOverlap(catalogEntry.Profiles, expandedProfiles) {
				log.Printf("%s %s: skip node %s, profiles %v don't match catalog %v",
					h.ResourceType, h.Name, id, expandedProfiles, catalogEntry.Profiles)
				continue
			}
		}
		nodeIDs = append(nodeIDs, id)
	}
	srv.unlock()

	if len(nodeIDs) == 0 {
		return
	}

	// Determine package kind from release type.
	pkgKind := "SERVICE"
	switch h.ResourceType {
	case "InfrastructureRelease":
		pkgKind = "INFRASTRUCTURE"
	case "ApplicationRelease":
		pkgKind = "WORKLOAD"
	}

	releaseID := fmt.Sprintf("%s/%s", h.ResourceType, h.Name)

	// Execute the release workflow asynchronously so the work queue worker
	// is not blocked. This prevents gRPC server deadlocks when multiple
	// workflows try to acquire srv.lock concurrently with gRPC handlers.
	log.Printf("%s %s: dispatching release workflow across %d nodes (v=%s)",
		h.ResourceType, h.Name, len(nodeIDs), h.ResolvedVersion)

	go func() {
		// Acquire semaphore to limit concurrent workflows and prevent
		// systemd overload on target nodes from too many parallel restarts.
		srv.workflowSem <- struct{}{}
		defer func() { <-srv.workflowSem }()

		run, err := srv.RunPackageReleaseWorkflow(ctx,
			releaseID,
			h.Name,
			h.InstalledStateName,
			pkgKind,
			h.ResolvedVersion,
			h.DesiredHash,
			nodeIDs,
		)

		nowMs := time.Now().UnixMilli()

		if err != nil {
			log.Printf("%s %s: release workflow FAILED: %v", h.ResourceType, h.Name, err)
			if run != nil && run.Steps != nil {
				nodeStatuses := srv.buildNodeStatusesFromRun(run, nodeIDs, h)
				succeeded, failed := countNodeOutcomes(nodeStatuses)
				if succeeded > 0 && failed > 0 {
					h.PatchStatus(ctx, statusPatch{
						Phase:                cluster_controllerpb.ReleasePhaseDegraded,
						Nodes:                nodeStatuses,
						LastTransitionUnixMs: nowMs,
						TransitionReason:     "partial_failure",
						SetFields:            "nodes",
					})
					return
				}
			}
			h.PatchStatus(ctx, statusPatch{
				Phase:                cluster_controllerpb.ReleasePhaseFailed,
				Message:              fmt.Sprintf("workflow failed: %v", err),
				LastTransitionUnixMs: nowMs,
				TransitionReason:     "workflow_failed",
				SetFields:            "fail",
			})
			return
		}

		nodeStatuses := srv.buildNodeStatusesFromRun(run, nodeIDs, h)
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseAvailable,
			Nodes:                nodeStatuses,
			LastTransitionUnixMs: nowMs,
			TransitionReason:     "workflow_succeeded",
			SetFields:            "nodes",
		})
	}()
}

// buildNodeStatusesFromRun constructs NodeReleaseStatus entries from a
// workflow run's step results. The foreach sub-steps use qualified IDs
// like "apply_per_node[0].install_package".
func (srv *server) buildNodeStatusesFromRun(run *engine.Run, nodeIDs []string, h *releaseHandle) []*cluster_controllerpb.NodeReleaseStatus {
	statuses := make([]*cluster_controllerpb.NodeReleaseStatus, 0, len(nodeIDs))
	nowMs := time.Now().UnixMilli()
	for i, nodeID := range nodeIDs {
		phase := cluster_controllerpb.ReleasePhaseAvailable
		var errMsg string

		// Check if this node's sub-steps had failures.
		prefix := fmt.Sprintf("apply_per_node[%d].", i)
		for stepID, st := range run.Steps {
			if strings.HasPrefix(stepID, prefix) && st.Status == engine.StepFailed {
				phase = cluster_controllerpb.ReleasePhaseFailed
				errMsg = st.Error
				break
			}
		}

		statuses = append(statuses, &cluster_controllerpb.NodeReleaseStatus{
			NodeID:           nodeID,
			Phase:            phase,
			InstalledVersion: h.ResolvedVersion,
			ErrorMessage:     errMsg,
			UpdatedUnixMs:    nowMs,
		})
	}
	return statuses
}

// countNodeOutcomes counts succeeded and failed nodes from status list.
func countNodeOutcomes(statuses []*cluster_controllerpb.NodeReleaseStatus) (succeeded, failed int) {
	for _, ns := range statuses {
		switch ns.Phase {
		case cluster_controllerpb.ReleasePhaseAvailable:
			succeeded++
		case cluster_controllerpb.ReleasePhaseFailed:
			failed++
		}
	}
	return
}

// reconcileApplying handles the APPLYING phase.
//
// In the workflow-native model, reconcileResolved() runs the workflow
// synchronously and transitions directly to AVAILABLE/DEGRADED/FAILED.
// Releases should no longer reach APPLYING in normal operation.
//
// This handler exists for backward compatibility: if a release was left in
// APPLYING state from a prior plan-based dispatch, return it to RESOLVED
// so the workflow path picks it up.
func (srv *server) reconcileApplying(ctx context.Context, h *releaseHandle) {
	log.Printf("%s %s: found stale APPLYING release, returning to RESOLVED for workflow execution",
		h.ResourceType, h.Name)
	h.PatchStatus(ctx, statusPatch{
		Phase:                cluster_controllerpb.ReleasePhaseResolved,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		TransitionReason:     "migrate_to_workflow",
		SetFields:            "phase",
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

// hasUnservedNodes checks if any eligible node has not successfully converged
// for this release. A node counts as "served" only if its per-node release
// status is AVAILABLE. Nodes that were attempted but FAILED, ROLLED_BACK, or
// are still APPLYING from a stale attempt are treated as unserved — they need
// a fresh plan dispatch.
//
// This is critical for Day 1 join: a node joins, gets dispatched, fails (e.g.
// 503 during artifact fetch), and must be retried. Without this, the controller
// treats "was attempted once" as "was successfully served" and never retries.
func (srv *server) hasUnservedNodes(h *releaseHandle) bool {
	srv.lock("hasUnservedNodes")
	defer srv.unlock()

	isWorkload := h.ResourceType == "ServiceRelease" || h.ResourceType == "ApplicationRelease"

	// Only genuinely converged nodes count as served.
	served := make(map[string]bool)
	for _, nrs := range h.Nodes {
		if nrs == nil {
			continue
		}
		switch nrs.Phase {
		case cluster_controllerpb.ReleasePhaseAvailable:
			served[nrs.NodeID] = true
		}
	}

	for id, node := range srv.state.Nodes {
		if served[id] {
			continue
		}
		if node.BootstrapPhase == BootstrapAdmitted || node.BootstrapPhase == "" {
			continue
		}
		if isWorkload && !bootstrapPhaseReady(node.BootstrapPhase) {
			continue
		}
		if node.Status == "unreachable" || node.Status == "removed" {
			continue
		}
		return true
	}
	return false
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
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
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
		ComputeHash: func(resolvedVersion string, buildNumber int64) string {
			return ComputeApplicationDesiredHash(rel.Spec.PublisherID, rel.Spec.AppName, resolvedVersion)
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
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
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
		ComputeHash: func(resolvedVersion string, buildNumber int64) string {
			return ComputeInfrastructureDesiredHash(rel.Spec.PublisherID, rel.Spec.Component, resolvedVersion, buildNumber)
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
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         repositoryAddrForSpec(rel.Spec),
		LockKey:                fmt.Sprintf("service:%s", canon),
		InstalledStateKind:     "SERVICE",
		InstalledStateName:     canon,
		ResolverSpec:           rel.Spec,
		ComputeHash: func(resolvedVersion string, buildNumber int64) string {
			return ComputeReleaseDesiredHash(rel.Spec.PublisherID, rel.Spec.ServiceName, resolvedVersion, buildNumber, rel.Spec.Config)
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

			// Lightweight restart path: if the service's version matches desired
			// but the unit is failed/inactive, attempt a restart before dispatching
			// a heavyweight reinstall plan.
			restarted := false
			if node != nil && hashMatch && !serviceHealthy {
				canon := canonicalServiceName(rel.Spec.ServiceName)
				unitName := serviceUnitForCanonical(canon)
				unitState, unitSubState := srv.findUnitState(node, unitName)

				restartable := (unitState == "failed" ||
					(unitState == "inactive" && unitSubState == "dead"))

				if restartable {
					restarted = srv.tryLightweightRestart(ctx, node, nodeID, canon, unitName, h.Name)
				}
			}

			if !restarted {
				// Drift detected but restart didn't fix it — re-enter PENDING
				// so the workflow pipeline picks it up on the next cycle.
				log.Printf("release %s: node %s drift detected, will re-enter PENDING for workflow re-apply", h.Name, nodeID)
				nCopy.Phase = cluster_controllerpb.ReleasePhaseDegraded
				nCopy.UpdatedUnixMs = time.Now().UnixMilli()
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
	srv.lock("release-pipeline:snapshot")
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	for id := range srv.state.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	srv.unlock()

	if len(nodeIDs) == 0 {
		h.PatchStatus(ctx, statusPatch{
			Phase:                ReleasePhaseRemoved,
			LastTransitionUnixMs: time.Now().UnixMilli(),
			TransitionReason:     "no_nodes",
			SetFields:            "phase",
		})
		return
	}

	pkgKind := h.InstalledStateKind
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}
	releaseID := fmt.Sprintf("%s/%s", h.ResourceType, h.Name)

	log.Printf("%s %s: executing removal workflow across %d nodes", h.ResourceType, h.Name, len(nodeIDs))

	run, err := srv.RunRemovePackageWorkflow(ctx, releaseID, h.InstalledStateName, pkgKind, nodeIDs)
	nowMs := time.Now().UnixMilli()

	if err != nil {
		log.Printf("%s %s: removal workflow FAILED: %v", h.ResourceType, h.Name, err)
		if run != nil {
			nodeStatuses := srv.buildNodeStatusesFromRun(run, nodeIDs, h)
			succeeded, failed := countNodeOutcomes(nodeStatuses)
			if succeeded > 0 && failed > 0 {
				h.PatchStatus(ctx, statusPatch{
					Phase:                cluster_controllerpb.ReleasePhaseFailed,
					Nodes:                nodeStatuses,
					Message:              "removal failed on some nodes",
					LastTransitionUnixMs: nowMs,
					TransitionReason:     "partial_removal_failure",
					SetFields:            "fail",
				})
				return
			}
		}
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              fmt.Sprintf("removal workflow failed: %v", err),
			LastTransitionUnixMs: nowMs,
			TransitionReason:     "removal_workflow_failed",
			SetFields:            "fail",
		})
		return
	}

	nodeStatuses := srv.buildNodeStatusesFromRun(run, nodeIDs, h)
	h.PatchStatus(ctx, statusPatch{
		Phase:                ReleasePhaseRemoved,
		Nodes:                nodeStatuses,
		LastTransitionUnixMs: nowMs,
		TransitionReason:     "workflow_succeeded",
		SetFields:            "nodes",
	})
}

const (
	restartMaxAttempts  = 3
	restartBaseBackoff  = 5 * time.Second
	restartMaxBackoff   = 2 * time.Minute
	restartBudgetWindow = 10 * time.Minute
)

// findUnitState returns the ActiveState and SubState for a unit from the node's cached unit list.
func (srv *server) findUnitState(node *nodeState, unitName string) (activeState, subState string) {
	for _, u := range node.Units {
		if strings.EqualFold(u.Name, unitName) {
			activeState = strings.ToLower(u.State)
			// Details format from enhanced detectUnits: "substate (load=loadstate)"
			details := u.Details
			if idx := strings.Index(details, " (load="); idx >= 0 {
				subState = details[:idx]
			} else {
				subState = details
			}
			return
		}
	}
	return "", ""
}

// tryLightweightRestart attempts a restart of a failed service via the node agent.
// Returns true if a restart was attempted (regardless of outcome), false if skipped.
func (srv *server) tryLightweightRestart(ctx context.Context, node *nodeState, nodeID, serviceName, unitName, releaseName string) bool {
	// Initialize restart tracking map if needed.
	if node.RestartAttempts == nil {
		node.RestartAttempts = make(map[string]*restartAttempt)
	}
	attempt := node.RestartAttempts[serviceName]
	if attempt == nil {
		attempt = &restartAttempt{}
		node.RestartAttempts[serviceName] = attempt
	}

	// Check backoff.
	if time.Now().Before(attempt.BackoffUntil) {
		return false
	}

	// Check budget: 3 attempts within 10 minutes → escalate.
	if attempt.Count >= restartMaxAttempts && time.Since(attempt.LastAt) < restartBudgetWindow {
		// Budget exhausted — emit event and escalate.
		srv.emitClusterEvent("service.restart_failed", map[string]interface{}{
			"severity":       "ERROR",
			"node_id":        nodeID,
			"unit":           unitName,
			"service":        serviceName,
			"attempts":       attempt.Count,
			"last_error":     attempt.LastError,
			"agent_endpoint": node.AgentEndpoint,
		})
		log.Printf("release %s: node %s service %s restart budget exhausted (%d attempts) — escalating to full plan",
			releaseName, nodeID, serviceName, attempt.Count)
		return false
	}

	// Reset counter if budget window has elapsed.
	if attempt.Count >= restartMaxAttempts && time.Since(attempt.LastAt) >= restartBudgetWindow {
		attempt.Count = 0
	}

	// Attempt restart via agent.
	if node.AgentEndpoint == "" {
		log.Printf("release %s: node %s has no agent endpoint — cannot restart %s", releaseName, nodeID, serviceName)
		return false
	}
	agent, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		log.Printf("release %s: node %s agent unreachable for restart of %s: %v", releaseName, nodeID, serviceName, err)
		// Agent unreachable — do NOT count as restart attempt.
		return false
	}

	resp, err := agent.ControlService(ctx, unitName, "restart")
	if err != nil {
		// RPC error (agent unreachable) — do NOT consume budget.
		log.Printf("release %s: node %s restart RPC failed for %s: %v", releaseName, nodeID, serviceName, err)
		return false
	}

	attempt.Count++
	attempt.LastAt = time.Now()
	// Exponential backoff: 5s, 10s, 20s, capped at 2min.
	backoff := restartBaseBackoff * time.Duration(1<<uint(attempt.Count-1))
	if backoff > restartMaxBackoff {
		backoff = restartMaxBackoff
	}
	attempt.BackoffUntil = attempt.LastAt.Add(backoff)

	if !resp.GetOk() {
		attempt.LastError = resp.GetMessage()
		log.Printf("release %s: node %s restart %s attempt %d failed: %s",
			releaseName, nodeID, serviceName, attempt.Count, resp.GetMessage())
	} else {
		log.Printf("release %s: node %s restart %s attempt %d succeeded (state=%s)",
			releaseName, nodeID, serviceName, attempt.Count, resp.GetState())
	}

	srv.emitClusterEvent("service.restart_attempted", map[string]interface{}{
		"severity":       "INFO",
		"node_id":        nodeID,
		"unit":           unitName,
		"service":        serviceName,
		"attempt":        attempt.Count,
		"ok":             resp.GetOk(),
		"state":          resp.GetState(),
		"correlation_id": "node:" + nodeID + ":unit:" + unitName,
	})

	return true
}
