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
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
)

// releaseRetryBackoff is the minimum time to wait before auto-retrying a
// FAILED release. Without this, resolve errors (e.g. "cluster_id required")
// create a tight FAILED→PENDING→FAILED loop that starves other handlers.
const releaseRetryBackoff = 5 * time.Minute

// releaseWaitingBackoff is the minimum time to wait before retrying a WAITING
// release (artifact not yet published in the repository). 2 minutes prevents
// hammering the repository while keeping convergence responsive once published.
const releaseWaitingBackoff = 2 * time.Minute

// workflowTransientRetryBackoff is kept for legacy callers but is superseded
// by NextRetryUnixMs for new code. The reconciler now reads NextRetryUnixMs
// directly from the release status instead of computing elapsed time from
// LastTransitionUnixMs + this constant.
const workflowTransientRetryBackoff = 30 * time.Second

// transientRetryBackoffs is the exponential backoff schedule for workflow
// transient errors. RetryCount (0-based) is used as the index; once past the
// end, maxTransientRetryBackoff is used.
var transientRetryBackoffs = []time.Duration{
	5 * time.Second,
	15 * time.Second,
	30 * time.Second,
	60 * time.Second,
	2 * time.Minute,
}

const maxTransientRetryBackoff = 5 * time.Minute

// transientRetryDelay returns the backoff duration for the given retry count.
func transientRetryDelay(retryCount int64) time.Duration {
	if retryCount < int64(len(transientRetryBackoffs)) {
		return transientRetryBackoffs[retryCount]
	}
	return maxTransientRetryBackoff
}

// isServiceConverged checks whether a service is already installed at the
// desired version on all eligible nodes. Used to suppress unnecessary release
// creation and enqueue during startup — a restart must not become a full-
// cluster apply storm.
//
// "Eligible" is defined by the same rules as reconcileResolved/hasUnservedNodes:
//   - Bootstrap phase: must be past "admitted" and workload-ready for services
//   - Catalog profile rules: service may only target nodes with matching profiles
//   - Excluded nodes: unreachable, removed, blocked, or draining nodes are skipped
//
// Without profile filtering, a service targeting only "core" nodes would
// require all nodes (including "gateway-only") to have it installed — causing
// false drift and unnecessary work.
func (srv *server) isServiceConverged(ctx context.Context, serviceName, desiredVersion string, desiredBuildNumber int64, desiredBuildID ...string) bool {
	if serviceName == "" || desiredVersion == "" {
		return false
	}
	canon := canonicalServiceName(serviceName)

	// Phase 2: extract optional build_id.
	buildID := ""
	if len(desiredBuildID) > 0 {
		buildID = desiredBuildID[0]
	}

	// Check installed-state registry across all nodes.
	pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", canon)
	if err != nil || len(pkgs) == 0 {
		return false // can't verify → treat as unconverged
	}

	// Build a set of node IDs that have the package installed at the right build_id.
	// Phase 2: build_id is the sole convergence identity. No version/build_number fallback.
	installedNodes := make(map[string]bool)
	for _, pkg := range pkgs {
		nodeID := pkg.GetNodeId()
		if nodeID == "" {
			continue
		}
		if buildID != "" && pkg.GetBuildId() == buildID {
			installedNodes[nodeID] = true
		}
		// If desired build_id is empty or installed build_id is empty,
		// the node is treated as unconverged — needs re-deploy to gain exact identity.
	}

	// Look up catalog entry for profile-based placement rules.
	catalogEntry := CatalogByName(canon)

	// Check that all eligible nodes are covered.
	srv.lock("isServiceConverged")
	defer srv.unlock()

	eligibleCount := 0
	for id, node := range srv.state.Nodes {
		// Same filtering as reconcileResolved / hasUnservedNodes:

		// 1. Bootstrap phase gating.
		if node.BootstrapPhase == BootstrapAdmitted || node.BootstrapPhase == "" {
			continue
		}
		if !bootstrapPhaseReady(node.BootstrapPhase) {
			continue
		}

		// 2. Excluded/drained nodes — not candidates for this service.
		if node.Status == "unreachable" || node.Status == "removed" ||
			node.Status == "blocked" || node.Status == "draining" {
			continue
		}

		// 3a. Partition-fenced nodes — the invariant enforcement detected a
		// sustained heartbeat absence and marked this node. Skip it for new
		// deployments until the heartbeat resumes and the fence is cleared.
		if _, fenced := node.Metadata["partition_fenced_since"]; fenced {
			continue
		}

		// 3. Catalog profile placement rules — skip nodes whose profiles
		//    don't overlap with the service's required profiles.
		if catalogEntry != nil && len(catalogEntry.Profiles) > 0 {
			expandedProfiles := normalizeProfiles(node.Profiles)
			if !profilesOverlap(catalogEntry.Profiles, expandedProfiles) {
				continue
			}
		}

		// This node is eligible — it must have the package installed.
		eligibleCount++
		if !installedNodes[id] {
			return false
		}
		if conv := classifyPackageConvergence(node, canon, "SERVICE", desiredVersion, "", buildID, &node_agentpb.InstalledPackage{
			Version: desiredVersion,
			BuildId: buildID,
		}, time.Now()); !conv.RuntimeOK {
			return false
		}
	}

	// If no nodes are eligible, don't claim convergence — there's nothing
	// to converge against (cold start, or service restricted to a profile
	// that no node has yet).
	return eligibleCount > 0
}

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
	ResolvedBuildID        string // Phase 2: exact artifact identity
	ResolvedArtifactDigest string
	DesiredHash            string
	LastTransitionUnixMs   int64
	Nodes                  []*cluster_controllerpb.NodeReleaseStatus

	// Resolve parameters (normalized to the common resolver shape)
	ResolverSpec   *cluster_controllerpb.ServiceReleaseSpec
	RepositoryAddr string

	// Installed-state lookup parameters for the canonical etcd registry.
	InstalledStateKind string // "SERVICE", "APPLICATION", "INFRASTRUCTURE"
	InstalledStateName string // canonical package name for installed-state lookup

	// Removing flag: when true, the release is being uninstalled.
	Removing bool

	// RepoKind is the authoritative artifact kind from the repository, set
	// during reconcilePending. Used in reconcileResolved to correct the
	// dispatch kind for SERVICE releases whose artifact is actually COMMAND
	// (e.g. etcdctl, sha256sum, yt-dlp).
	RepoKind string

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
	ResolvedBuildID        string // Phase 2: exact artifact identity
	ResolvedArtifactDigest string
	DesiredHash            string
	ObservedGeneration     int64
	Message                string
	Nodes                  []*cluster_controllerpb.NodeReleaseStatus
	LastTransitionUnixMs   int64
	WorkflowKind           string
	StartedAtUnixMs        int64
	TransitionReason       string
	// BlockedReason is a structured slug set by the "retry" path:
	// workflow_unavailable, workflow_circuit_open, workflow_deadline, etc.
	BlockedReason string
	// SetFields controls which fields are meaningful in this patch.
	// "resolve" = version/digest/hash/generation, "phase" = just phase,
	// "nodes" = phase + nodes, "fail" = phase + message, "retry" = transient error.
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
	if !srv.mustBeLeader() {
		return
	}
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

	// Acquire resolve semaphore to limit concurrent repository calls.
	select {
	case srv.resolveSem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-srv.resolveSem }()

	artifactKind := repositorypb.ArtifactKind_SERVICE
	if h.ResourceType == "InfrastructureRelease" {
		artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
	} else if h.ResourceType == "ApplicationRelease" {
		artifactKind = repositorypb.ArtifactKind_APPLICATION
	}
	resolver := &ReleaseResolver{RepositoryAddr: h.RepositoryAddr, ArtifactKind: artifactKind}
	resolveStart := time.Now()
	resolved, err := resolver.Resolve(ctx, h.ResolverSpec)
	releaseResolveDuration.Observe(time.Since(resolveStart).Seconds())
	releasePhaseTransitions.WithLabelValues(h.ResourceType, "RESOLVED").Inc()
	if err != nil {
		errMsg := err.Error()
		// Artifact not found — the desired version hasn't been published yet
		// (or the repository is resolving to the wrong MinIO backend). Enter
		// WAITING with a backoff so the controller retries periodically without
		// hammering the repository. The WAITING → PENDING transition fires after
		// releaseWaitingBackoff; the installed version remains in service.
		//
		// IMPORTANT: do NOT transition to AVAILABLE here. AVAILABLE means all
		// target nodes are at the desired version. If we mark a release AVAILABLE
		// before installing, reconcileAvailable will detect unserved nodes and
		// re-enter PENDING immediately, creating a tight PENDING→AVAILABLE→PENDING
		// storm at ~10 etcd writes/second.
		if strings.Contains(errMsg, "NotFound") || strings.Contains(errMsg, "not found") {
			log.Printf("%s %s: artifact not in repository, entering WAITING (retry in %s): %v",
				h.ResourceType, h.Name, releaseWaitingBackoff, err)
			h.PatchStatus(ctx, statusPatch{
				Phase:                cluster_controllerpb.ReleasePhaseWaiting,
				Message:              "artifact not published — waiting for repository to confirm availability",
				ObservedGeneration:   h.Generation,
				LastTransitionUnixMs: nowMs,
				TransitionReason:     "artifact_not_published",
				WorkflowKind:         wfKind,
				StartedAtUnixMs:      nowMs,
				SetFields:            "fail", // writes Phase+Message+LastTransitionUnixMs+ObservedGeneration (needed for backoff)
			})
			return
		}
		log.Printf("%s %s: resolve failed: %v", h.ResourceType, h.Name, err)
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              fmt.Sprintf("resolve: %v", err),
			ObservedGeneration:   h.Generation,
			LastTransitionUnixMs: nowMs,
			TransitionReason:     "resolve_failed",
			WorkflowKind:         wfKind,
			StartedAtUnixMs:      nowMs,
			SetFields:            "fail",
		})
		return
	}

	// Version sanity check: the resolved artifact must not be older than the
	// desired version in the spec. If the repository returns an ancient artifact
	// (e.g. 0.0.1 when 0.1.0 is desired), FAIL the release instead of installing
	// the wrong version. Automatic rollback is forbidden — only forward progress.
	if h.ResolverSpec != nil && h.ResolverSpec.Version != "" && resolved.Version != "" {
		cmp, cmpErr := versionutil.Compare(resolved.Version, h.ResolverSpec.Version)
		if cmpErr == nil && cmp < 0 {
			msg := fmt.Sprintf("repository returned %s but desired is %s — refusing to install older version",
				resolved.Version, h.ResolverSpec.Version)
			log.Printf("%s %s: REJECTED — %s", h.ResourceType, h.Name, msg)
			h.PatchStatus(ctx, statusPatch{
				Phase:                cluster_controllerpb.ReleasePhaseFailed,
				Message:              msg,
				ObservedGeneration:   h.Generation,
				LastTransitionUnixMs: nowMs,
				TransitionReason:     "version_downgrade_rejected",
				WorkflowKind:         wfKind,
				StartedAtUnixMs:      nowMs,
				SetFields:            "fail",
			})
			return
		}
	}

	// Store the repository's authoritative kind so reconcileResolved can
	// correct the dispatch kind for SERVICE releases whose artifact is
	// actually COMMAND (e.g. etcdctl, sha256sum, yt-dlp).
	if resolved.RepoKind != repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
		h.RepoKind = strings.ToUpper(resolved.RepoKind.String())
		// Also correct InstalledStateKind so hasUnservedNodes checks the right
		// etcd key. Without this, hasUnservedNodes looks up SERVICE/yt-dlp but
		// nodes report the package as COMMAND/yt-dlp — node always appears
		// unserved and the pipeline cycles RESOLVED without dispatching.
		if h.InstalledStateKind == "SERVICE" && h.RepoKind == "COMMAND" {
			h.InstalledStateKind = "COMMAND"
		}
	}

	desiredHash := h.ComputeHash(resolved.Version, resolved.BuildNumber)
	h.PatchStatus(ctx, statusPatch{
		Phase:                  cluster_controllerpb.ReleasePhaseResolved,
		ResolvedVersion:        resolved.Version,
		ResolvedBuildID:        resolved.BuildID,
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
	if !srv.mustBeLeader() {
		return
	}
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
		// Gate workload services on RuntimeLocalDependencies: if a service
		// requires event/rbac/etc. and those deps are not yet active on this
		// node, skip it so the semaphore slot stays free for dep installs.
		// The release stays RESOLVED and retries on the next reconcile cycle.
		if catalogEntry != nil && len(catalogEntry.RuntimeLocalDependencies) > 0 {
			healthy := buildHealthySet(node.Units)
			missing := checkRuntimeDeps(catalogEntry, healthy, node.InstalledVersions)
			if len(missing) > 0 {
				log.Printf("%s %s: skipping node %s — deps not ready: %v",
					h.ResourceType, h.Name, node.Identity.Hostname, missing)
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
	// Auto-correct SERVICE→COMMAND when the repository's authoritative kind
	// is COMMAND. ServiceDesiredVersion always labels entries as SERVICE
	// regardless of actual artifact kind, but COMMAND packages (etcdctl,
	// sha256sum, yt-dlp) must be installed without a systemd unit.
	if pkgKind == "SERVICE" && h.RepoKind == "COMMAND" {
		log.Printf("%s %s: corrected dispatch kind SERVICE→COMMAND (repo authoritative)", h.ResourceType, h.Name)
		pkgKind = "COMMAND"
	}

	releaseID := fmt.Sprintf("%s/%s", h.ResourceType, h.Name)

	// Guard: skip dispatch if a workflow goroutine is already running for
	// this release. Without this, the work queue re-enters reconcileResolved
	// (the release stays RESOLVED during execution) and a second goroutine
	// overwrites + deletes the actor router, causing "no router registered"
	// errors on the first workflow's callbacks.
	srv.inflightMu.Lock()
	if cancel, running := srv.inflightWorkflows[releaseID]; running {
		srv.inflightMu.Unlock()
		_ = cancel // suppress unused warning; already running, don't cancel here
		log.Printf("%s %s: workflow already in-flight, skipping dispatch", h.ResourceType, h.Name)
		return
	}
	// Create a cancellable context for this workflow. If desired state changes
	// mid-flight, cancelInflightWorkflow() cancels this context so the engine's
	// DAG loop exits promptly via ctx.Err().
	wfCtx, wfCancel := context.WithCancel(ctx)
	srv.inflightWorkflows[releaseID] = wfCancel
	srv.inflightMu.Unlock()

	// Execute the release workflow asynchronously so the work queue worker
	// is not blocked. This prevents gRPC server deadlocks when multiple
	// workflows try to acquire srv.lock concurrently with gRPC handlers.
	workflowDispatchTotal.WithLabelValues(computeWorkflowKind(h)).Inc()
	log.Printf("%s %s: dispatching release workflow across %d nodes (v=%s)",
		h.ResourceType, h.Name, len(nodeIDs), h.ResolvedVersion)

	go func() {
		defer wfCancel()
		defer func() {
			srv.inflightMu.Lock()
			delete(srv.inflightWorkflows, releaseID)
			srv.inflightMu.Unlock()
		}()

		// Acquire semaphore to limit concurrent workflows and prevent
		// systemd overload on target nodes from too many parallel restarts.
		// Use a timeout so a stuck workflow doesn't starve the pipeline
		// indefinitely. If the timeout fires, the release stays RESOLVED
		// and will be retried on the next reconcile cycle.
		select {
		case srv.workflowSem <- struct{}{}:
			// acquired
		case <-wfCtx.Done():
			log.Printf("%s %s: workflow context cancelled while waiting for semaphore", h.ResourceType, h.Name)
			return
		case <-time.After(2 * time.Minute):
			log.Printf("%s %s: workflow semaphore timeout (all slots busy for 2m), deferring", h.ResourceType, h.Name)
			return
		}
		defer func() { <-srv.workflowSem }()

		_, err := srv.RunPackageReleaseWorkflow(wfCtx,
			releaseID,
			h.Name,
			h.InstalledStateName,
			pkgKind,
			h.ResolvedVersion,
			h.DesiredHash,
			h.ResolvedBuildID,
			nodeIDs,
			h.Generation, // generation guard: callbacks skip writes if generation advanced
		)

		// Success path: workflow callbacks (MarkNodeSucceeded/Failed,
		// FinalizeDirectApply, MarkReleaseFailed) already wrote the final
		// release phase and per-node status. Controller does not re-patch.
		if err != nil {
			// classifyWorkflowError distinguishes infrastructure/transient errors
			// (keep RESOLVED, retry with backoff) from real execution failures
			// (transition to FAILED, needs operator attention).
			if isTransient, reason := classifyWorkflowError(err); isTransient {
				log.Printf("%s %s: workflow transient error (%s), staying RESOLVED for retry: %v",
					h.ResourceType, h.Name, reason, err)
				h.PatchStatus(ctx, statusPatch{
					Message:          fmt.Sprintf("workflow transient error (will retry): %v", err),
					TransitionReason: "workflow_transient_error",
					BlockedReason:    reason,
					SetFields:        "retry",
				})
			} else {
				log.Printf("%s %s: release workflow engine error: %v", h.ResourceType, h.Name, err)
				h.PatchStatus(ctx, statusPatch{
					Phase:                cluster_controllerpb.ReleasePhaseFailed,
					Message:              fmt.Sprintf("workflow engine error: %v", err),
					LastTransitionUnixMs: time.Now().UnixMilli(),
					TransitionReason:     "workflow_engine_error",
					SetFields:            "fail",
				})
			}
		}
	}()
}

// cancelInflightWorkflow cancels the context of a running workflow for the
// given release ID. Called when desired state changes mid-flight so the
// engine's DAG loop exits promptly instead of running to completion.
func (srv *server) cancelInflightWorkflow(releaseID string) {
	srv.inflightMu.Lock()
	if cancel, ok := srv.inflightWorkflows[releaseID]; ok {
		cancel()
		log.Printf("release %s: cancelled in-flight workflow (desired state changed)", releaseID)
	}
	srv.inflightMu.Unlock()
}

// cancelAllInflightWorkflows cancels every running workflow goroutine.
// Called on leadership demotion so the old leader stops writing release
// state — the new leader owns all reconciliation from this point.
func (srv *server) cancelAllInflightWorkflows() {
	srv.inflightMu.Lock()
	n := len(srv.inflightWorkflows)
	for id, cancel := range srv.inflightWorkflows {
		cancel()
		log.Printf("release %s: cancelled in-flight workflow (leadership lost)", id)
	}
	srv.inflightMu.Unlock()
	if n > 0 {
		log.Printf("leader demotion: cancelled %d in-flight workflow(s)", n)
	}
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
// for this release. A node counts as "served" if either:
//   - its per-node release status is AVAILABLE (workflow wrote it), OR
//   - its reported InstalledVersions already match this release's resolved
//     version (node was already converged, workflow short-circuited).
//
// Nodes that were attempted but FAILED, ROLLED_BACK, or are still APPLYING
// from a stale attempt are treated as unserved — they need a fresh dispatch.
//
// This is critical for Day 1 join: a node joins, gets dispatched, fails (e.g.
// 503 during artifact fetch), and must be retried. Without this, the controller
// treats "was attempted once" as "was successfully served" and never retries.
func (srv *server) hasUnservedNodes(h *releaseHandle) bool {
	srv.lock("hasUnservedNodes")
	defer srv.unlock()

	isWorkload := h.ResourceType == "ServiceRelease" || h.ResourceType == "ApplicationRelease"

	// Convergence signal #1: per-node release status written by workflow callbacks.
	served := make(map[string]bool)
	for _, nrs := range h.Nodes {
		if nrs == nil {
			continue
		}
		if nrs.Phase == cluster_controllerpb.ReleasePhaseAvailable {
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
		// Convergence signal #2: node reports the right version installed.
		// This covers the "already installed, workflow short-circuited" case.
		if h.InstalledStateName != "" && h.ResolvedVersion != "" && node.InstalledVersions != nil {
			if node.InstalledVersions[h.InstalledStateName] == h.ResolvedVersion {
				conv := classifyPackageConvergence(node, h.InstalledStateName, h.InstalledStateKind, h.ResolvedVersion, h.DesiredHash, h.ResolvedBuildID, &node_agentpb.InstalledPackage{
					Version:  h.ResolvedVersion,
					Checksum: h.DesiredHash,
					BuildId:  h.ResolvedBuildID,
				}, time.Now())
				if conv.RuntimeOK {
					continue
				} else {
					log.Printf("hasUnservedNodes: release=%s node=%s version match but runtime unconverged (%s)",
						h.Name, id, conv.Reason)
				}
			}
		}
		log.Printf("hasUnservedNodes: release=%s node=%s unserved (installed_name=%q resolved_v=%q installed_v=%q)",
			h.Name, id, h.InstalledStateName, h.ResolvedVersion,
			node.InstalledVersions[h.InstalledStateName])
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
		ResolvedBuildID:        rel.Status.ResolvedBuildID,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         appRepoAddr(rel.Spec),
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
		ResolvedBuildID:        rel.Status.ResolvedBuildID,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         infraRepoAddr(rel.Spec),
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
		s.ResolvedBuildID = p.ResolvedBuildID
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
		if p.ObservedGeneration > 0 {
			s.ObservedGeneration = p.ObservedGeneration
		}
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
		s.ResolvedBuildID = p.ResolvedBuildID
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
		if p.ObservedGeneration > 0 {
			s.ObservedGeneration = p.ObservedGeneration
		}
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
		ResolvedBuildID:        rel.Status.ResolvedBuildID,
		ResolvedArtifactDigest: rel.Status.ResolvedArtifactDigest,
		DesiredHash:            rel.Status.DesiredHash,
		LastTransitionUnixMs:   rel.Status.LastTransitionUnixMs,
		Nodes:                  rel.Status.Nodes,
		RepositoryAddr:         repositoryAddrForSpec(rel.Spec),
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

// applyPatchToSvcStatus applies a statusPatch to a ServiceReleaseStatus and
// returns true if any field was actually mutated (i.e. Apply is warranted).
// Returns false for unknown SetFields values so callers can skip Apply.
func applyPatchToSvcStatus(s *cluster_controllerpb.ServiceReleaseStatus, p statusPatch) (changed bool) {
	if p.Phase == cluster_controllerpb.ReleasePhasePending {
		buf := make([]byte, 2048)
		n := runtime.Stack(buf, false)
		log.Printf("DEBUG-APPLY-PATCH-PENDING: was=%q now=%q SetFields=%q reason=%q\nstack:\n%s",
			s.Phase, p.Phase, p.SetFields, p.TransitionReason, buf[:n])
	}
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
		return true
	case "resolve":
		s.Phase = p.Phase
		s.ResolvedVersion = p.ResolvedVersion
		s.ResolvedBuildID = p.ResolvedBuildID
		s.ResolvedArtifactDigest = p.ResolvedArtifactDigest
		s.DesiredHash = p.DesiredHash
		s.ObservedGeneration = p.ObservedGeneration
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
		return true
	case "nodes":
		s.Phase = p.Phase
		s.Nodes = p.Nodes
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		applyWorkflowFields()
		return true
	case "fail":
		s.Phase = p.Phase
		s.Message = p.Message
		s.LastTransitionUnixMs = p.LastTransitionUnixMs
		if p.ObservedGeneration > 0 {
			s.ObservedGeneration = p.ObservedGeneration
		}
		applyWorkflowFields()
		return true
	case "retry":
		// Workflow transient error (circuit breaker open, Scylla unavailable, etc.).
		// Increments RetryCount and computes NextRetryUnixMs with exponential backoff.
		// Phase and LastTransitionUnixMs are NOT touched — those belong to the
		// RESOLVED→APPLYING transition, not to the retry bookkeeping.
		//
		// The reconciler's "if NextRetryUnixMs > now → return early" guard is what
		// prevents the dispatch storm; patchReleaseStatus calls Apply once per retry
		// attempt, not on every reconcile tick.
		now := time.Now()
		backoff := transientRetryDelay(s.RetryCount)
		s.RetryCount++
		s.LastRetryUnixMs = now.UnixMilli()
		s.NextRetryUnixMs = now.Add(backoff).UnixMilli()
		s.LastTransientError = p.Message
		if p.BlockedReason != "" {
			s.BlockedReason = p.BlockedReason
		} else if p.TransitionReason != "" {
			s.BlockedReason = p.TransitionReason
		}
		if p.TransitionReason != "" {
			s.TransitionReason = p.TransitionReason
		}
		s.Message = fmt.Sprintf("%s (retry %d, next in %s)", p.Message, s.RetryCount, backoff.Round(time.Second))
		return true
	default:
		// Unknown SetFields: log once and skip — do not mutate anything.
		log.Printf("applyPatchToSvcStatus: unknown SetFields=%q (release phase=%s) — patch skipped",
			p.SetFields, s.Phase)
		return false
	}
}

// detectServiceDrift checks version+health drift for a ServiceRelease.
// Returns true if drift was detected and a re-plan was dispatched.
func (srv *server) detectServiceDrift(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, h *releaseHandle) bool {
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

		// Skip drift decisions for nodes with stale heartbeats. Trusting
		// old data causes false DEGRADED transitions and reconcile storms
		// when heartbeats are delayed by network issues.
		if node != nil && !node.LastSeen.IsZero() && time.Since(node.LastSeen) > unhealthyThreshold {
			continue
		}

		versionMatch := false
		healthy := false
		serviceHealthy := false
		if node != nil && rel.Spec != nil {
			healthy = strings.EqualFold(node.Status, "ready")
			serviceHealthy = srv.serviceHealthyForRelease(node, rel)
			// Phase 2: build_id is the sole convergence identity for drift detection.
			// Read installed build_id from etcd (authoritative).
			if rel.Status != nil && rel.Status.ResolvedBuildID != "" {
				if pkg, pkgErr := installed_state.GetInstalledPackage(ctx, nodeID, "SERVICE", rel.Spec.ServiceName); pkgErr == nil && pkg != nil {
					if pkg.GetBuildId() == rel.Status.ResolvedBuildID {
						versionMatch = true
					}
				}
			}
			// If resolved build_id is empty (pre-Phase-2 release), versionMatch stays false
			// → treated as drifted → triggers re-resolve with build_id.
		}
		if versionMatch && healthy && serviceHealthy {
			ok++
			nCopy.Phase = cluster_controllerpb.ReleasePhaseAvailable
		} else {
			issues++

			// Lightweight restart path: if the service's version matches desired
			// but the unit is failed/inactive, attempt a restart before dispatching
			// a heavyweight reinstall plan.
			restarted := false
			if node != nil && versionMatch && !serviceHealthy {
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
	if !srv.mustBeLeader() {
		return
	}
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

	// Cancel any in-flight install workflow — removal takes precedence.
	// Without this, install and remove workflows can race on the same
	// release, leaving the service in a half-installed zombie state.
	srv.cancelInflightWorkflow(releaseID)

	// Wait for the inflight slot to clear (the goroutine's defer cleans it up
	// after context cancellation). If it doesn't clear quickly, skip this
	// cycle — the next reconcile will retry.
	srv.inflightMu.Lock()
	if _, running := srv.inflightWorkflows[releaseID]; running {
		srv.inflightMu.Unlock()
		log.Printf("%s %s: install workflow still winding down, deferring removal", h.ResourceType, h.Name)
		return
	}
	srv.inflightMu.Unlock()

	log.Printf("%s %s: executing removal workflow across %d nodes", h.ResourceType, h.Name, len(nodeIDs))

	_, err := srv.RunRemovePackageWorkflow(ctx, releaseID, h.InstalledStateName, pkgKind, nodeIDs)
	nowMs := time.Now().UnixMilli()

	// Per-node statuses are already written by the remove workflow's
	// MarkNodeSucceeded / MarkNodeFailed callbacks. Controller only decides
	// the release-level terminal phase (REMOVED vs FAILED).
	if err != nil {
		log.Printf("%s %s: removal workflow FAILED: %v", h.ResourceType, h.Name, err)
		h.PatchStatus(ctx, statusPatch{
			Phase:                cluster_controllerpb.ReleasePhaseFailed,
			Message:              fmt.Sprintf("removal workflow failed: %v", err),
			LastTransitionUnixMs: nowMs,
			TransitionReason:     "removal_workflow_failed",
			SetFields:            "fail",
		})
		return
	}
	h.PatchStatus(ctx, statusPatch{
		Phase:                ReleasePhaseRemoved,
		LastTransitionUnixMs: nowMs,
		TransitionReason:     "workflow_succeeded",
		SetFields:            "phase",
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

	// If service is blocked due to repeated precondition failures, check if
	// the blocking condition has cleared before allowing restart.
	if attempt.BlockedReason != "" {
		// Re-check: can we reach the agent and verify cert status?
		if node.AgentEndpoint == "" {
			return false
		}
		agent, err := srv.getAgentClient(ctx, node.AgentEndpoint)
		if err != nil {
			return false
		}
		certResp, certErr := agent.GetCertificateStatus(ctx)
		if certErr == nil && certResp.GetServerCert() != nil &&
			!strings.HasPrefix(certResp.GetServerCert().GetSubject(), "error") {
			// Precondition recovered — clear the block.
			log.Printf("release %s: node %s service %s block cleared — cert available again",
				releaseName, nodeID, serviceName)
			attempt.BlockedReason = ""
			attempt.BlockedSince = time.Time{}
			attempt.ConsecutivePrecondFail = 0
		} else {
			log.Printf("release %s: node %s service %s still blocked: %s (since %s)",
				releaseName, nodeID, serviceName, attempt.BlockedReason,
				attempt.BlockedSince.Format(time.RFC3339))
			return false
		}
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

	// Dependency check: don't restart if a required dependency on this node
	// is inactive or blocked — restarting a leaf on a dead branch is pointless.
	if deps := RuntimeDependenciesFor(serviceName); len(deps) > 0 {
		for _, dep := range deps {
			depUnit := "globular-" + dep + ".service"
			for _, u := range node.Units {
				if strings.EqualFold(u.Name, depUnit) && strings.ToLower(u.State) != "active" {
					blockedBy := ""
					if node.RestartAttempts != nil {
						if depAttempt := node.RestartAttempts[dep]; depAttempt != nil && depAttempt.BlockedReason != "" {
							blockedBy = fmt.Sprintf(" (blocked: %s)", depAttempt.BlockedReason)
						}
					}
					log.Printf("release %s: node %s restart SKIPPED for %s — dependency %s is %s%s",
						releaseName, nodeID, serviceName, dep, u.State, blockedBy)
					srv.emitClusterEvent("service.restart_skipped", map[string]interface{}{
						"severity":       "WARNING",
						"node_id":        nodeID,
						"unit":           unitName,
						"service":        serviceName,
						"reason":         "dependency_not_active",
						"blocked_by":     dep,
						"dep_state":      u.State,
						"correlation_id": "node:" + nodeID + ":unit:" + unitName,
					})
					attempt.FailureClass = FailClassDependencyBlocked
					return false
				}
			}
		}
	}

	// Pre-condition: verify TLS certificate is present before restarting.
	// If the cert is missing/broken, restarting will just hit ExecStartPre
	// timeout again — skip and don't consume budget.
	certResp, certErr := agent.GetCertificateStatus(ctx)
	if certErr != nil {
		// RPC failed — log and proceed with restart as best-effort.
		log.Printf("release %s: node %s cert precheck RPC failed for %s: %v (proceeding with restart)",
			releaseName, nodeID, serviceName, certErr)
	} else if certResp.GetServerCert() == nil || strings.HasPrefix(certResp.GetServerCert().GetSubject(), "error") {
		reason := "server certificate missing"
		if certResp.GetServerCert() != nil {
			reason = "server certificate error: " + certResp.GetServerCert().GetSubject()
		}
		log.Printf("release %s: node %s restart SKIPPED for %s — %s",
			releaseName, nodeID, serviceName, reason)
		// Fetch short recent logs for immediate diagnostic visibility.
		srv.fetchAndLogUnitTail(ctx, agent, unitName, releaseName, nodeID, serviceName)
		// Track precondition failure classification.
		attempt.FailureClass = FailClassPreconditionFail
		attempt.ConsecutivePrecondFail++
		if attempt.ConsecutivePrecondFail >= maxConsecutivePrecondFail {
			attempt.BlockedReason = reason
			attempt.BlockedSince = time.Now()
			log.Printf("release %s: node %s service %s BLOCKED after %d consecutive precondition failures: %s",
				releaseName, nodeID, serviceName, attempt.ConsecutivePrecondFail, reason)
			srv.emitClusterEvent("service.blocked", map[string]interface{}{
				"severity":       "ERROR",
				"node_id":        nodeID,
				"unit":           unitName,
				"service":        serviceName,
				"reason":         reason,
				"consecutive":    attempt.ConsecutivePrecondFail,
				"correlation_id": "node:" + nodeID + ":unit:" + unitName,
			})
		} else {
			srv.emitClusterEvent("service.restart_skipped", map[string]interface{}{
				"severity":       "WARNING",
				"node_id":        nodeID,
				"unit":           unitName,
				"service":        serviceName,
				"reason":         reason,
				"correlation_id": "node:" + nodeID + ":unit:" + unitName,
			})
		}
		// Do NOT consume restart budget — this is a hard precondition failure.
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
		// Fetch short recent logs for immediate diagnostic visibility.
		srv.fetchAndLogUnitTail(ctx, agent, unitName, releaseName, nodeID, serviceName)
	} else {
		log.Printf("release %s: node %s restart %s attempt %d succeeded (state=%s)",
			releaseName, nodeID, serviceName, attempt.Count, resp.GetState())
		attempt.ConsecutivePrecondFail = 0
		attempt.FailureClass = ""
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

// fetchAndLogUnitTail retrieves a short tail of recent journal logs from the
// node agent for the given unit and logs them. This provides immediate
// diagnostic visibility for ExecStartPre timeouts and other startup failures
// without waiting for unit template regeneration.
func (srv *server) fetchAndLogUnitTail(ctx context.Context, agent *agentClient, unitName, releaseName, nodeID, serviceName string) {
	logResp, logErr := agent.GetServiceLogs(ctx, unitName, 20)
	if logErr != nil {
		log.Printf("release %s: node %s could not fetch logs for %s: %v",
			releaseName, nodeID, serviceName, logErr)
		return
	}
	lines := logResp.GetLines()
	if len(lines) == 0 {
		return
	}
	log.Printf("release %s: node %s recent logs for %s (%d lines):",
		releaseName, nodeID, serviceName, len(lines))
	for _, line := range lines {
		log.Printf("  | %s", line)
	}
}
