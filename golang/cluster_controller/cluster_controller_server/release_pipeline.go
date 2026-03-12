package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
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

	// Type-specific callbacks
	ComputeHash func(resolvedVersion string) string
	CompilePlan func(nodeID, installedVersion, clusterID string) (*planpb.NodePlan, error)

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
	// SetFields controls which fields are meaningful in this patch.
	// "resolve" = version/digest/hash/generation, "phase" = just phase,
	// "nodes" = phase + nodes, "fail" = phase + message.
	SetFields string
}

// reconcilePending is the shared PENDING phase: resolve version and artifact
// digest via ReleaseResolver, compute desired hash, transition to RESOLVED.
func (srv *server) reconcilePending(ctx context.Context, h *releaseHandle) {
	// Idempotency guard: skip re-resolution if already resolved for this generation.
	if h.ObservedGeneration == h.Generation &&
		h.ResolvedVersion != "" &&
		h.ResolvedArtifactDigest != "" {
		h.PatchStatus(ctx, statusPatch{
			Phase:     cluster_controllerpb.ReleasePhaseResolved,
			SetFields: "phase",
		})
		return
	}

	resolver := &ReleaseResolver{RepositoryAddr: h.RepositoryAddr}
	resolvedVersion, digest, err := resolver.Resolve(ctx, h.ResolverSpec)
	if err != nil {
		log.Printf("%s %s: resolve failed: %v", h.ResourceType, h.Name, err)
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              fmt.Sprintf("resolve: %v", err),
			LastTransitionUnixMs: time.Now().UnixMilli(),
			SetFields:            "fail",
		})
		return
	}

	desiredHash := h.ComputeHash(resolvedVersion)
	h.PatchStatus(ctx, statusPatch{
		Phase:                  cluster_controllerpb.ReleasePhaseResolved,
		ResolvedVersion:        resolvedVersion,
		ResolvedArtifactDigest: digest,
		DesiredHash:            desiredHash,
		ObservedGeneration:     h.Generation,
		Message:                "",
		LastTransitionUnixMs:   time.Now().UnixMilli(),
		SetFields:              "resolve",
	})
}

// reconcileResolved is the shared RESOLVED phase: compile and dispatch plans
// to all target nodes, transition to APPLYING.
func (srv *server) reconcileResolved(ctx context.Context, h *releaseHandle) {
	srv.lock("release-pipeline:snapshot")
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	for id := range srv.state.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	clusterID := srv.state.ClusterId
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
		SetFields:            "nodes",
	})
}

// reconcileApplying is the shared APPLYING phase: inspect per-node plan
// statuses and advance to AVAILABLE, DEGRADED, or FAILED.
func (srv *server) reconcileApplying(ctx context.Context, h *releaseHandle) {
	if len(h.Nodes) == 0 {
		// Lost node list; re-enter RESOLVED to recompile plans.
		h.PatchStatus(ctx, statusPatch{
			Phase:     cluster_controllerpb.ReleasePhaseResolved,
			SetFields: "phase",
		})
		return
	}

	updatedNodes, succeeded, failed, running := srv.checkNodePlanStatuses(ctx, h.Nodes, h.ResolvedVersion)
	total := len(updatedNodes)
	newPhase := cluster_controllerpb.ReleasePhaseApplying
	switch {
	case total > 0 && succeeded == total:
		newPhase = cluster_controllerpb.ReleasePhaseAvailable
	case failed > 0 && running == 0:
		if succeeded > 0 {
			newPhase = cluster_controllerpb.ReleasePhaseDegraded
		} else {
			newPhase = cluster_controllerpb.ReleasePhaseFailed
		}
	}

	h.PatchStatus(ctx, statusPatch{
		Phase:                newPhase,
		Nodes:                updatedNodes,
		LastTransitionUnixMs: time.Now().UnixMilli(),
		SetFields:            "nodes",
	})
}

// reconcileAvailable is the shared AVAILABLE/DEGRADED phase: detect spec
// generation drift and re-enter PENDING if the spec changed.
func (srv *server) reconcileAvailable(ctx context.Context, h *releaseHandle) {
	if h.Generation > h.ObservedGeneration {
		h.PatchStatus(ctx, statusPatch{
			Phase:     cluster_controllerpb.ReleasePhasePending,
			SetFields: "phase",
		})
	}
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
			Channel:      rel.Spec.Channel,
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
			Channel:      rel.Spec.Channel,
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
	switch p.SetFields {
	case "phase":
		s.Phase = p.Phase
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	}
}

func applyPatchToInfraStatus(s *cluster_controllerpb.InfrastructureReleaseStatus, p statusPatch) {
	switch p.SetFields {
	case "phase":
		s.Phase = p.Phase
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
	}
}
