package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// hashEnqueueCooldown debounces per-node hash-change re-enqueue.
// Prevents reconcile storms when delayed heartbeats arrive in bursts.
var hashEnqueueCooldown sync.Map

func (srv *server) ReportNodeStatus(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ReportNodeStatusResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ReportNodeStatus", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.GetStatus() == nil || req.GetStatus().GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "status.node_id is required")
	}

	// Node identity + own-node scope enforcement
	if err := enforceNodeScope(ctx, req.GetStatus().GetNodeId(), "/clustercontroller.ClusterControllerService/ReportNodeStatus"); err != nil {
		return nil, err
	}
	nodeStatus := req.GetStatus()
	ns := nodeStatus
	nodeID := strings.TrimSpace(ns.GetNodeId())
	newIdentity := protoToStoredIdentity(ns.GetIdentity())
	newEndpoint := strings.TrimSpace(ns.GetAgentEndpoint())
	reportedAt := time.Now()
	if ts := ns.GetReportedAt(); ts != nil {
		reportedAt = ts.AsTime()
	}
	rawUnits := protoUnitsToStored(ns.GetUnits())
	units := normalizedUnits(rawUnits)
	lastError := ns.GetLastError()
	appliedSvcHash := strings.ToLower(strings.TrimSpace(ns.GetAppliedServicesHash()))
	installedVersions := ns.GetInstalledVersions()
	installedUnitFiles := ns.GetInstalledUnitFiles()
	inventoryComplete := ns.GetInventoryComplete()

	// Snapshot existing node for evaluation without holding the lock during compute.
	srv.lock("ReportNodeStatus:snapshot")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		// Auto-register unknown nodes that report with valid credentials.
		// This handles post-restore scenarios where controller state was wiped
		// but node-agents still have their node IDs and keep heartbeating.
		log.Printf("ReportNodeStatus: auto-registering unknown node %s (hostname=%s endpoint=%s)",
			nodeID, newIdentity.Hostname, newEndpoint)
		node = &nodeState{
			NodeID:         nodeID,
			Identity:       newIdentity,
			AgentEndpoint:  newEndpoint,
			LastSeen:       reportedAt,
			ReportedAt:     reportedAt,
			Status:         "recovering",
			Profiles:       []string{}, // do not assume privileged profiles
			Metadata:       make(map[string]string),
			BootstrapPhase: BootstrapWorkloadReady, // already running, skip bootstrap
		}
		if srv.state.Nodes == nil {
			srv.state.Nodes = make(map[string]*nodeState)
		}
		srv.state.Nodes[nodeID] = node

		// Clean up stale nodes with the same hostname or IP that are
		// unreachable/unhealthy. After a restore the same physical machine
		// gets a new node ID; the old entry lingers as "unreachable."
		srv.removeStaleNodesLocked(nodeID, newIdentity, newEndpoint)

		if err := srv.persistStateLocked(true); err != nil {
			srv.unlock()
			return nil, status.Errorf(codes.Internal, "persist auto-registered node: %v", err)
		}
	}
	nodeSnapshot := *node
	srv.unlock()

	healthStatus, reason := srv.evaluateNodeStatus(&nodeSnapshot, units)
	if lastError == "" && reason != "" && healthStatus != "ready" {
		lastError = reason
	}

	if testHookBeforeReportNodeStatusApply != nil {
		testHookBeforeReportNodeStatusApply()
	}

	srv.lock("ReportNodeStatus:commit")
	defer srv.unlock()
	node = srv.state.Nodes[nodeID]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	changed := false

	if !identitiesEqual(node.Identity, newIdentity) {
		changed = true
	}
	node.Identity = newIdentity

	oldEndpoint := node.AgentEndpoint
	node.AgentEndpoint = newEndpoint
	node.ReportedAt = reportedAt
	node.LastSeen = reportedAt
	changed = true // LastSeen must always persist so followers see fresh heartbeats

	if !unitsEqual(node.Units, units) {
		// Detect units that transitioned from active to non-active (crash/stop).
		oldStates := make(map[string]string, len(node.Units))
		for _, u := range node.Units {
			oldStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
		}
		for _, u := range units {
			newState := strings.ToLower(u.State)
			if newState == "active" || newState == "" {
				continue
			}
			if oldStates[strings.ToLower(u.Name)] == "active" {
				// Distinguish crash from clean stop:
				//   "failed"   → service.exited (crash — triggers remediation)
				//   "inactive" → service.stopped (clean — observe only)
				eventName := "service.stopped"
				severity := "WARNING"
				if newState == "failed" {
					eventName = "service.exited"
					severity = "ERROR"
				}
				srv.emitClusterEvent(eventName, map[string]interface{}{
					"severity":       severity,
					"node_id":        nodeID,
					"unit":           u.Name,
					"previous_state": "active",
					"current_state":  u.State,
					"correlation_id": "node:" + nodeID + ":unit:" + u.Name,
				})
			}
		}
		// Detect units that recovered (non-active → active) and reset restart tracking.
		for _, u := range units {
			newState := strings.ToLower(u.State)
			if newState != "active" {
				continue
			}
			oldState := oldStates[strings.ToLower(u.Name)]
			if oldState != "" && oldState != "active" {
				// Unit recovered — reset restart attempts.
				svcName := canonicalServiceName(u.Name)
				if svcName != "" && node.RestartAttempts != nil {
					if ra, exists := node.RestartAttempts[svcName]; exists {
						prevCount := ra.Count
						delete(node.RestartAttempts, svcName)
						if prevCount > 0 {
							log.Printf("node %s: service %s recovered after %d restart attempts", nodeID, svcName, prevCount)
						}
					}
				}
			}
		}
		node.Units = units
		changed = true
	}
	if node.Status != healthStatus {
		node.Status = healthStatus
		changed = true
	}
	if node.LastError != lastError {
		node.LastError = lastError
		changed = true
	}
	hashChanged := node.AppliedServicesHash != appliedSvcHash
	if hashChanged {
		node.AppliedServicesHash = appliedSvcHash
		changed = true
	}
	// Persist the node-reported inventory hash under the observed key.
	// This is distinct from applied_hash_services which is only set by the
	// reconciler when convergence with the desired state is confirmed.
	if appliedSvcHash != "" && inventoryComplete {
		if err := srv.putNodeObservedServiceHash(ctx, nodeID, appliedSvcHash); err != nil {
			log.Printf("ReportNodeStatus: store observed service hash for %s: %v", nodeID, err)
		}
	}
	// Update installed versions when the node reports inventory, even if empty
	// (inventoryComplete=true means the node has finished scanning).
	if len(installedVersions) > 0 || inventoryComplete {
		if !mapsEqual(node.InstalledVersions, installedVersions) {
			node.InstalledVersions = installedVersions
			changed = true
		}
	}
	// Store hardware capabilities if reported.
	if caps := nodeStatus.GetCapabilities(); caps != nil {
		node.Capabilities = capsToStored(caps)
	}
	// Phase 3: store installed unit file inventory and inventory_complete flag.
	if inventoryComplete || len(installedUnitFiles) > 0 {
		// Merge the reported unit files into the node's unit list as "inactive" records
		// so that missingInstalledUnits can find them. Only add entries not already present.
		unitMap := make(map[string]string, len(node.Units))
		for _, u := range node.Units {
			unitMap[strings.ToLower(u.Name)] = u.State
		}
		for _, uf := range installedUnitFiles {
			name := strings.ToLower(strings.TrimSpace(uf))
			if name == "" {
				continue
			}
			if _, exists := unitMap[name]; !exists {
				node.Units = append(node.Units, unitStatusRecord{Name: uf, State: "inactive"})
				unitMap[name] = "inactive"
				changed = true
			}
		}
		if node.InventoryComplete != inventoryComplete {
			node.InventoryComplete = inventoryComplete
			changed = true
		}
	}
	// Auto-derive profiles from installed services when profiles are empty.
	// This handles cold boot (all nodes auto-register with empty profiles)
	// and post-restore scenarios. Once profiles are set by SetNodeProfiles,
	// this auto-derivation is skipped.
	if len(node.Profiles) == 0 && len(installedVersions) > 0 {
		derived := deriveProfilesFromInstalled(installedVersions)
		if len(derived) > 0 {
			log.Printf("ReportNodeStatus: auto-derived profiles for %s: %v (from %d installed packages)",
				nodeID, derived, len(installedVersions))
			node.Profiles = derived
			// Set advertise FQDN from hostname + cluster domain
			if node.Identity.Hostname != "" {
				domain := "globular.internal"
				if srv.state.ClusterNetworkSpec != nil && srv.state.ClusterNetworkSpec.ClusterDomain != "" {
					domain = srv.state.ClusterNetworkSpec.ClusterDomain
				}
				node.AdvertiseFqdn = node.Identity.Hostname + "." + domain
			}
			srv.state.NetworkingGeneration++
			changed = true

			// Update MinIO pool if this node has storage profile
			for _, p := range derived {
				if p == "storage" && node.AdvertiseFqdn != "" {
					found := false
					for _, existing := range srv.state.MinioPoolNodes {
						if existing == node.AdvertiseFqdn {
							found = true
							break
						}
					}
					if !found {
						srv.state.MinioPoolNodes = append(srv.state.MinioPoolNodes, node.AdvertiseFqdn)
						log.Printf("ReportNodeStatus: added %s to MinIO pool", node.AdvertiseFqdn)
					}
				}
			}
		}
	}
	if oldEndpoint != newEndpoint {
		changed = true
	}
	// Trigger workflow on first heartbeat (node just joined and reported its endpoint).
	// The node may already be in infra_preparing by the time the heartbeat arrives
	// (the reconcile loop advances phases before ReportNodeStatus completes).
	if oldEndpoint == "" && newEndpoint != "" && node.BootstrapPhase != BootstrapWorkloadReady &&
		!node.BootstrapWorkflowActive {
		log.Printf("ReportNodeStatus: node %s first heartbeat (phase=%s) — triggering join workflow at %s",
			nodeID, node.BootstrapPhase, newEndpoint)
		node.BootstrapWorkflowActive = true
		go srv.triggerJoinWorkflow(nodeID, newEndpoint)
	}

	// Commit or discard pending rendered config hashes based on node health:
	// a healthy report confirms the rendered config is on disk; a failed report
	// clears pending so the next reconcile cycle retries.
	if len(node.PendingRenderedConfigHashes) > 0 {
		if healthStatus == "ready" {
			node.RenderedConfigHashes = node.PendingRenderedConfigHashes
			node.PendingRenderedConfigHashes = nil
			changed = true
		} else if healthStatus == "error" || healthStatus == "failed" {
			node.PendingRenderedConfigHashes = nil
			changed = true
		}
		// For other states (converging, etc.) keep pending and wait.
	}
	endpointToClose := ""
	if oldEndpoint != "" && oldEndpoint != newEndpoint {
		endpointToClose = oldEndpoint
	}

	// Stamp leader liveness signal: heartbeat was successfully processed
	// (all in-memory state mutations complete). This is intentionally BEFORE
	// persistStateLocked — liveness measures "did I process heartbeats?", not
	// "did etcd persist succeed?". A temporary etcd issue must not make a
	// healthy leader look dead.
	srv.lastHeartbeatProcessed.Store(time.Now().UnixNano())

	if changed {
		if err := srv.persistStateLocked(false); err != nil {
			return nil, status.Errorf(codes.Internal, "persist node status: %v", err)
		}
	}

	// Update the node_identity projection after every ReportStatus so the
	// scylla view tracks source-of-truth changes. Best-effort — any error
	// here MUST NOT fail the handler (Clause 3: readers fall back).
	if srv.nodeIdentityProj != nil {
		id := nodeToIdentity(node)
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.nodeIdentityProj.Upsert(bg, *id); err != nil {
				log.Printf("node_identity: upsert %s failed: %v", id.NodeID, err)
			}
		}()
	}

	// When the applied services hash changes, re-enqueue any ServiceReleases that
	// include this node so drift detection can re-evaluate and potentially recover
	// DEGRADED releases without waiting for the next spec change.
	// Debounced per-node: skip if the same node triggered re-enqueue within the
	// last 30 seconds. This prevents reconcile storms when delayed heartbeats
	// arrive in rapid succession after a network partition.
	if hashChanged && srv.releaseEnqueue != nil && srv.resources != nil {
		now := time.Now()
		cooldownKey := "hash-enqueue:" + nodeID
		if last, ok := hashEnqueueCooldown.Load(cooldownKey); ok {
			if now.Sub(last.(time.Time)) < 30*time.Second {
				goto skipHashEnqueue
			}
		}
		hashEnqueueCooldown.Store(cooldownKey, now)
		{
			enqueue := srv.releaseEnqueue
			resources := srv.resources
			nID := nodeID
			go func() {
				items, _, err := resources.List(context.Background(), "ServiceRelease", "")
				if err != nil {
					return
				}
				for _, obj := range items {
					rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
					if !ok || rel.Meta == nil {
						continue
					}
					if rel.Status == nil {
						continue
					}
					for _, nrs := range rel.Status.Nodes {
						if nrs != nil && nrs.NodeID == nID {
							enqueue(rel.Meta.Name)
							break
						}
					}
				}
			}()
		}
	}
skipHashEnqueue:

	// Trigger B: When a node reports installed versions and desired state is
	// empty, auto-import from installed. This catches the case where a node
	// joins or reports before the startup auto-import has run.
	// Debounced by autoImportDone to avoid running on every heartbeat.
	if len(installedVersions) > 0 && srv.resources != nil && !srv.autoImportDone.Load() && srv.mustBeLeader() {
		resources := srv.resources
		safeGo("report-node-auto-import", func() {
			items, _, err := resources.List(context.Background(), "ServiceDesiredVersion", "")
			if err != nil || len(items) > 0 {
				if len(items) > 0 {
					srv.autoImportDone.Store(true)
				}
				return
			}
			// Desired state is empty — import from installed.
			logger.Info("ReportNodeStatus: desired state empty, auto-importing from installed")
			stats, err := srv.importInstalledToDesired(context.Background())
			if err != nil {
				logger.Warn("ReportNodeStatus: auto-import failed", "error", err)
				return
			}
			srv.autoImportDone.Store(true)
			logger.Info("ReportNodeStatus: auto-import complete",
				"imported", stats.Imported,
				"already_present", stats.AlreadyPresent,
				"failed", stats.Failed)
			if (stats.Imported > 0 || stats.Updated > 0) && srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
		})
	}

	if endpointToClose != "" {
		srv.closeAgentClient(endpointToClose)
	}

	// Trigger reconcile for nodes in pre-ready bootstrap phases so the
	// bootstrap state machine advances. Without this, newly admitted nodes
	// would stay stuck because the reconciler is event-driven (watches
	// ClusterNetwork/ServiceDesiredVersion) and heartbeats don't trigger it.
	if !bootstrapPhaseReady(node.BootstrapPhase) && srv.enqueueReconcile != nil {
		srv.enqueueReconcile()
	}

	return &cluster_controllerpb.ReportNodeStatusResponse{
		Message: "status recorded",
	}, nil
}

// ReportPlanRejection deleted — plan system removed.

// ResourcesService implementation
func (srv *server) ApplyClusterNetwork(ctx context.Context, req *cluster_controllerpb.ApplyClusterNetworkRequest) (*cluster_controllerpb.ClusterNetwork, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ClusterNetwork{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyClusterNetwork", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.Object == nil || req.Object.Spec == nil || strings.TrimSpace(req.Object.Spec.ClusterDomain) == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_network.spec.cluster_domain is required")
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.Object
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = "default"
	applied, err := srv.resources.Apply(ctx, "ClusterNetwork", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply cluster network: %v", err)
	}
	return applied.(*cluster_controllerpb.ClusterNetwork), nil
}

func (srv *server) GetClusterNetwork(ctx context.Context, _ *cluster_controllerpb.GetClusterNetworkRequest) (*cluster_controllerpb.ClusterNetwork, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj, _, err := srv.resources.Get(ctx, "ClusterNetwork", "default")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get cluster network: %v", err)
	}
	if obj == nil {
		return nil, status.Error(codes.NotFound, "cluster network not found")
	}
	return obj.(*cluster_controllerpb.ClusterNetwork), nil
}

func (srv *server) ApplyServiceDesiredVersion(ctx context.Context, req *cluster_controllerpb.ApplyServiceDesiredVersionRequest) (*cluster_controllerpb.ServiceDesiredVersion, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ServiceDesiredVersion{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyServiceDesiredVersion", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if req == nil || req.Object == nil || req.Object.Spec == nil || strings.TrimSpace(req.Object.Spec.ServiceName) == "" || strings.TrimSpace(req.Object.Spec.Version) == "" {
		return nil, status.Error(codes.InvalidArgument, "service_name and version are required")
	}
	canon := canonicalServiceName(req.Object.Spec.ServiceName)
	if canon == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid service_name")
	}
	obj := req.Object
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = canon
	obj.Spec.ServiceName = canon
	applied, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply service desired version: %v", err)
	}
	return applied.(*cluster_controllerpb.ServiceDesiredVersion), nil
}

func (srv *server) DeleteServiceDesiredVersion(ctx context.Context, req *cluster_controllerpb.DeleteServiceDesiredVersionRequest) (*emptypb.Empty, error) {
	if !srv.isLeader() {
		resp := &emptypb.Empty{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/DeleteServiceDesiredVersion", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := canonicalServiceName(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete service desired version: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) ListServiceDesiredVersions(ctx context.Context, _ *cluster_controllerpb.ListServiceDesiredVersionsRequest) (*cluster_controllerpb.ListServiceDesiredVersionsResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list service desired versions: %v", err)
	}
	out := &cluster_controllerpb.ListServiceDesiredVersionsResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.ServiceDesiredVersion))
	}
	return out, nil
}

func (srv *server) ApplyServiceRelease(ctx context.Context, req *cluster_controllerpb.ApplyServiceReleaseRequest) (*cluster_controllerpb.ServiceRelease, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ServiceRelease{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyServiceRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.GetObject()
	if obj == nil || obj.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "object and spec are required")
	}
	if strings.TrimSpace(obj.Spec.PublisherID) == "" || strings.TrimSpace(obj.Spec.ServiceName) == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.publisher_id and spec.service_name are required")
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	// Canonical name: publisher/service to keep it unique across publishers.
	if obj.Meta.Name == "" {
		obj.Meta.Name = obj.Spec.PublisherID + "/" + canonicalServiceName(obj.Spec.ServiceName)
	}
	applied, err := srv.resources.Apply(ctx, "ServiceRelease", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply service release: %v", err)
	}
	return applied.(*cluster_controllerpb.ServiceRelease), nil
}

func (srv *server) GetServiceRelease(ctx context.Context, req *cluster_controllerpb.GetServiceReleaseRequest) (*cluster_controllerpb.ServiceRelease, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get service release: %v", err)
	}
	if obj == nil {
		return nil, status.Errorf(codes.NotFound, "service release %q not found", name)
	}
	return obj.(*cluster_controllerpb.ServiceRelease), nil
}

func (srv *server) ListServiceReleases(ctx context.Context, _ *cluster_controllerpb.ListServiceReleasesRequest) (*cluster_controllerpb.ListServiceReleasesResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list service releases: %v", err)
	}
	out := &cluster_controllerpb.ListServiceReleasesResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.ServiceRelease))
	}
	return out, nil
}

func (srv *server) DeleteServiceRelease(ctx context.Context, req *cluster_controllerpb.DeleteServiceReleaseRequest) (*emptypb.Empty, error) {
	if !srv.isLeader() {
		resp := &emptypb.Empty{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/DeleteServiceRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "ServiceRelease", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete service release: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// ── ApplicationRelease CRUD ──────────────────────────────────────────────────

func (srv *server) ApplyApplicationRelease(ctx context.Context, req *cluster_controllerpb.ApplyApplicationReleaseRequest) (*cluster_controllerpb.ApplicationRelease, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ApplicationRelease{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyApplicationRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.GetObject()
	if obj == nil || obj.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "object and spec are required")
	}
	if strings.TrimSpace(obj.Spec.PublisherID) == "" || strings.TrimSpace(obj.Spec.AppName) == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.publisher_id and spec.app_name are required")
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	if obj.Meta.Name == "" {
		obj.Meta.Name = obj.Spec.PublisherID + "/" + obj.Spec.AppName
	}
	applied, err := srv.resources.Apply(ctx, "ApplicationRelease", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply application release: %v", err)
	}
	return applied.(*cluster_controllerpb.ApplicationRelease), nil
}

func (srv *server) GetApplicationRelease(ctx context.Context, req *cluster_controllerpb.GetApplicationReleaseRequest) (*cluster_controllerpb.ApplicationRelease, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	obj, _, err := srv.resources.Get(ctx, "ApplicationRelease", name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get application release: %v", err)
	}
	if obj == nil {
		return nil, status.Errorf(codes.NotFound, "application release %q not found", name)
	}
	return obj.(*cluster_controllerpb.ApplicationRelease), nil
}

func (srv *server) ListApplicationReleases(ctx context.Context, _ *cluster_controllerpb.ListApplicationReleasesRequest) (*cluster_controllerpb.ListApplicationReleasesResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ApplicationRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list application releases: %v", err)
	}
	out := &cluster_controllerpb.ListApplicationReleasesResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.ApplicationRelease))
	}
	return out, nil
}

func (srv *server) DeleteApplicationRelease(ctx context.Context, req *cluster_controllerpb.DeleteApplicationReleaseRequest) (*emptypb.Empty, error) {
	if !srv.isLeader() {
		resp := &emptypb.Empty{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/DeleteApplicationRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "ApplicationRelease", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete application release: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// ── InfrastructureRelease CRUD ───────────────────────────────────────────────

func (srv *server) ApplyInfrastructureRelease(ctx context.Context, req *cluster_controllerpb.ApplyInfrastructureReleaseRequest) (*cluster_controllerpb.InfrastructureRelease, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.InfrastructureRelease{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApplyInfrastructureRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.GetObject()
	if obj == nil || obj.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "object and spec are required")
	}
	if strings.TrimSpace(obj.Spec.PublisherID) == "" || strings.TrimSpace(obj.Spec.Component) == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.publisher_id and spec.component are required")
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	if obj.Meta.Name == "" {
		obj.Meta.Name = obj.Spec.PublisherID + "/" + obj.Spec.Component
	}
	applied, err := srv.resources.Apply(ctx, "InfrastructureRelease", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply infrastructure release: %v", err)
	}
	return applied.(*cluster_controllerpb.InfrastructureRelease), nil
}

func (srv *server) GetInfrastructureRelease(ctx context.Context, req *cluster_controllerpb.GetInfrastructureReleaseRequest) (*cluster_controllerpb.InfrastructureRelease, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	obj, _, err := srv.resources.Get(ctx, "InfrastructureRelease", name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get infrastructure release: %v", err)
	}
	if obj == nil {
		return nil, status.Errorf(codes.NotFound, "infrastructure release %q not found", name)
	}
	return obj.(*cluster_controllerpb.InfrastructureRelease), nil
}

func (srv *server) ListInfrastructureReleases(ctx context.Context, _ *cluster_controllerpb.ListInfrastructureReleasesRequest) (*cluster_controllerpb.ListInfrastructureReleasesResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list infrastructure releases: %v", err)
	}
	out := &cluster_controllerpb.ListInfrastructureReleasesResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.InfrastructureRelease))
	}
	return out, nil
}

func (srv *server) DeleteInfrastructureRelease(ctx context.Context, req *cluster_controllerpb.DeleteInfrastructureReleaseRequest) (*emptypb.Empty, error) {
	if !srv.isLeader() {
		resp := &emptypb.Empty{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/DeleteInfrastructureRelease", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "InfrastructureRelease", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete infrastructure release: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) Watch(req *cluster_controllerpb.WatchRequest, stream cluster_controllerpb.ResourcesService_WatchServer) error {
	if srv.resources == nil {
		return status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if req == nil {
		return status.Error(codes.InvalidArgument, "request required")
	}
	ch, err := srv.resources.Watch(stream.Context(), req.GetType(), req.GetPrefix(), req.GetFromResourceVersion())
	if err != nil {
		return status.Errorf(codes.Internal, "watch: %v", err)
	}
	if req.GetIncludeExisting() {
		items, rv, err := srv.resources.List(stream.Context(), req.GetType(), req.GetPrefix())
		if err == nil {
			for _, obj := range items {
				evt := resourcestore.Event{Type: resourcestore.EventAdded, ResourceVersion: rv, Object: obj}
				if err := stream.Send(toWatchEvent(req.GetType(), evt)); err != nil {
					return err
				}
			}
		}
	}
	for evt := range ch {
		if err := stream.Send(toWatchEvent(req.GetType(), evt)); err != nil {
			return err
		}
	}
	return nil
}

// deriveProfilesFromInstalled infers node profiles from the set of installed
// packages. This allows cold-boot auto-registration to assign meaningful
// profiles without requiring an explicit SetNodeProfiles call.
//
// Mapping rules:
//   - Has dns/cluster-controller/workflow → control-plane
//   - Has any Globular service → core
//   - Has minio/repository → storage
//   - Has ai-memory/ai-executor → ai (implies core)
//   - Has gateway/envoy → gateway
//   - No control-plane services → compute
func deriveProfilesFromInstalled(installed map[string]string) []string {
	has := func(names ...string) bool {
		for _, n := range names {
			// Installed versions may use bare names ("dns") or qualified
			// ("SERVICE/dns") depending on the reporter.
			if _, ok := installed[n]; ok {
				return true
			}
			if _, ok := installed["SERVICE/"+n]; ok {
				return true
			}
		}
		return false
	}

	profiles := map[string]bool{}

	if has("dns", "cluster-controller", "workflow", "authentication", "rbac") {
		profiles["control-plane"] = true
		profiles["core"] = true
	}
	if has("minio", "repository", "monitoring", "backup-manager") {
		profiles["storage"] = true
		profiles["core"] = true
	}
	if has("ai-memory", "ai-executor", "ai-watcher", "ai-router") {
		profiles["ai"] = true
		profiles["core"] = true
	}
	if has("gateway", "envoy") {
		profiles["gateway"] = true
	}

	// If nothing matched, it's a compute-only node
	if len(profiles) == 0 && len(installed) > 0 {
		profiles["compute"] = true
	}

	result := make([]string, 0, len(profiles))
	for p := range profiles {
		result = append(result, p)
	}
	return result
}
