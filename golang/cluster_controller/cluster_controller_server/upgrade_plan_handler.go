package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bundleEntry describes the latest available version of a package from the repository.
type bundleEntry struct {
	version     string
	sha256      string
	publisherID string
	sizeBytes   int64
	kind        repopb.ArtifactKind // 0 = legacy bundle (service), else modern artifact kind
}

// PlanServiceUpgrades resolves available upgrades for the requested services
// by querying the repository and comparing against installed versions on the
// target node. This is the canonical planning authority — gateway delegates here.
func (srv *server) PlanServiceUpgrades(ctx context.Context, req *cluster_controllerpb.PlanServiceUpgradesRequest) (*cluster_controllerpb.PlanServiceUpgradesResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}

	nodeID := strings.TrimSpace(req.GetNodeId())
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		platform = runtime.GOOS + "_" + runtime.GOARCH
	}

	// Resolve local node ID if not specified.
	if nodeID == "" {
		srv.lock("plan-upgrade-node")
		for id := range srv.state.Nodes {
			nodeID = id
			break
		}
		srv.unlock()
	}
	if nodeID == "" {
		return nil, status.Error(codes.FailedPrecondition, "no nodes registered")
	}

	srv.lock("plan-upgrade-read")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}

	// Query repository for available bundles.
	repo := resolveRepositoryInfo()
	repoStatus := "ok"

	rc, err := repository_client.NewRepositoryService_Client(repo.Address, "repository.PackageRepository")
	if err != nil {
		log.Printf("PlanServiceUpgrades: repository unreachable at %s: %v", repo.Address, err)
		return &cluster_controllerpb.PlanServiceUpgradesResponse{
			RepositoryStatus: "unreachable",
		}, nil
	}
	defer rc.Close()

	// Build latest version index from bundles.
	// TODO(migration): Switch primary query to ListArtifacts once all packages
	// are published via the artifact path; keep ListBundles as fallback only.
	latestByService := make(map[string]bundleEntry) // canonical_name -> latest

	bundles, err := rc.ListBundles()
	if err != nil {
		log.Printf("PlanServiceUpgrades: ListBundles failed: %v", err)
		return &cluster_controllerpb.PlanServiceUpgradesResponse{
			RepositoryStatus: "unreachable",
		}, nil
	}
	if len(bundles) == 0 {
		repoStatus = "empty"
	}

	for _, b := range bundles {
		bPlatform := normalizeArtifactPlatform(b.GetPlatform())
		if bPlatform != normalizeArtifactPlatform(platform) {
			continue
		}
		name := canonicalServiceName(b.GetName())
		ver := b.GetVersion()
		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}
		existing, ok := latestByService[name]
		if !ok {
			latestByService[name] = bundleEntry{
				version:     ver,
				sha256:      strings.TrimSpace(b.GetSha256()),
				publisherID: b.GetPublisherId(),
				sizeBytes:   b.GetSizeBytes(),
			}
			continue
		}
		if cmp, err := versionutil.Compare(existing.version, ver); err == nil && cmp < 0 {
			latestByService[name] = bundleEntry{
				version:     ver,
				sha256:      strings.TrimSpace(b.GetSha256()),
				publisherID: b.GetPublisherId(),
				sizeBytes:   b.GetSizeBytes(),
			}
		}
	}

	// Supplement with modern artifact catalog — picks up APPLICATION and
	// INFRASTRUCTURE packages that may not appear in legacy bundles.
	mergeArtifactVersions(rc, platform, latestByService)

	// Filter to requested services (or all if empty).
	requestedSet := make(map[string]bool)
	for _, s := range req.GetServices() {
		requestedSet[canonicalServiceName(s)] = true
	}

	var items []*cluster_controllerpb.UpgradePlanItem
	for name, entry := range latestByService {
		if len(requestedSet) > 0 && !requestedSet[name] {
			continue
		}
		installed := lookupInstalledVersion(node, name)
		if installed != "" {
			if cmp, err := versionutil.Compare(installed, entry.version); err == nil && cmp >= 0 {
				continue // already up to date
			}
		}

		unit := serviceUnitForCanonical(name)
		impacts := upgradeImpacts(name)

		items = append(items, &cluster_controllerpb.UpgradePlanItem{
			Service:         name,
			FromVersion:     installed,
			ToVersion:       entry.version,
			PackageName:     fmt.Sprintf("%s@%s", name, entry.version),
			Sha256:          entry.sha256,
			RestartRequired: unit != "",
			Impacts:         impacts,
		})
	}

	return &cluster_controllerpb.PlanServiceUpgradesResponse{
		Items:            items,
		RepositoryStatus: repoStatus,
	}, nil
}

// ApplyServiceUpgrades builds upgrade plans and dispatches them to target node(s).
// When node_id is empty, performs a cluster-wide rolling upgrade: one node at a time,
// verifying health between nodes. The controller is the central orchestrator.
func (srv *server) ApplyServiceUpgrades(ctx context.Context, req *cluster_controllerpb.ApplyServiceUpgradesRequest) (*cluster_controllerpb.ApplyServiceUpgradesResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}

	nodeID := strings.TrimSpace(req.GetNodeId())

	// Single-node upgrade.
	if nodeID != "" {
		return srv.applySingleNodeUpgrades(ctx, req, nodeID)
	}

	// Cluster-wide rolling upgrade: iterate nodes sequentially.
	srv.lock("apply-cluster-upgrade")
	var nodeIDs []string
	for id, node := range srv.state.Nodes {
		if node.Status != "draining" && node.AgentEndpoint != "" {
			nodeIDs = append(nodeIDs, id)
		}
	}
	srv.unlock()

	if len(nodeIDs) == 0 {
		return &cluster_controllerpb.ApplyServiceUpgradesResponse{
			Ok:      false,
			Message: "no eligible nodes found",
		}, nil
	}

	var nodeStatuses []*cluster_controllerpb.NodeUpgradeStatus
	var lastOp string
	allOK := true

	for _, nid := range nodeIDs {
		nodeReq := &cluster_controllerpb.ApplyServiceUpgradesRequest{
			Services: req.GetServices(),
			NodeId:   nid,
			Platform: req.GetPlatform(),
		}
		resp, err := srv.applySingleNodeUpgrades(ctx, nodeReq, nid)
		ns := &cluster_controllerpb.NodeUpgradeStatus{
			NodeId: nid,
		}
		if err != nil {
			ns.Status = "failed"
			ns.Error = err.Error()
			allOK = false
			nodeStatuses = append(nodeStatuses, ns)
			log.Printf("cluster upgrade: node %s failed: %v — halting rollout", nid, err)
			break // Stop on failure (rolling policy).
		}
		if !resp.GetOk() {
			ns.Status = "failed"
			ns.Error = resp.GetMessage()
			allOK = false
			nodeStatuses = append(nodeStatuses, ns)
			log.Printf("cluster upgrade: node %s failed: %s — halting rollout", nid, resp.GetMessage())
			break
		}
		ns.Status = "success"
		ns.OperationId = resp.GetOperationId()
		lastOp = resp.GetOperationId()
		nodeStatuses = append(nodeStatuses, ns)
		log.Printf("cluster upgrade: node %s dispatched op=%s", nid, resp.GetOperationId())
	}

	msg := fmt.Sprintf("cluster upgrade dispatched to %d/%d nodes", len(nodeStatuses), len(nodeIDs))
	return &cluster_controllerpb.ApplyServiceUpgradesResponse{
		Ok:            allOK,
		OperationId:   lastOp,
		Message:       msg,
		NodeStatuses:  nodeStatuses,
	}, nil
}

// applySingleNodeUpgrades handles upgrade for a specific node.
func (srv *server) applySingleNodeUpgrades(ctx context.Context, req *cluster_controllerpb.ApplyServiceUpgradesRequest, nodeID string) (*cluster_controllerpb.ApplyServiceUpgradesResponse, error) {
	planResp, err := srv.PlanServiceUpgrades(ctx, &cluster_controllerpb.PlanServiceUpgradesRequest{
		Services: req.GetServices(),
		NodeId:   nodeID,
		Platform: req.GetPlatform(),
	})
	if err != nil {
		return nil, err
	}
	if planResp.GetRepositoryStatus() != "ok" {
		return &cluster_controllerpb.ApplyServiceUpgradesResponse{
			Ok:      false,
			Message: fmt.Sprintf("repository %s", planResp.GetRepositoryStatus()),
		}, nil
	}
	if len(planResp.GetItems()) == 0 {
		return &cluster_controllerpb.ApplyServiceUpgradesResponse{
			Ok:      true,
			Message: "all services up to date",
		}, nil
	}

	var lastOp string
	dispatched := 0
	for _, item := range planResp.GetItems() {
		if item.GetSha256() == "" {
			return &cluster_controllerpb.ApplyServiceUpgradesResponse{
				Ok:      false,
				Message: fmt.Sprintf("bundle %s@%s has no SHA256 — refusing unverified artifact", item.GetService(), item.GetToVersion()),
			}, nil
		}
		plan := buildUpgradePlanForKind(nodeID, item)

		resp, err := srv.ApplyNodePlanV1(ctx, &cluster_controllerpb.ApplyNodePlanV1Request{
			NodeId: nodeID,
			Plan:   plan,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "dispatch %s upgrade on node %s: %v", item.GetService(), nodeID, err)
		}
		lastOp = resp.GetOperationId()
		dispatched++
	}

	return &cluster_controllerpb.ApplyServiceUpgradesResponse{
		Ok:          true,
		OperationId: lastOp,
		Message:     fmt.Sprintf("dispatched %d upgrades on node %s", dispatched, nodeID),
	}, nil
}

// lookupInstalledVersion finds a service's installed version from node state.
func lookupInstalledVersion(node *nodeState, canonicalName string) string {
	if node == nil || len(node.InstalledVersions) == 0 {
		return ""
	}
	// Direct lookup.
	if v := strings.TrimSpace(node.InstalledVersions[canonicalName]); v != "" {
		return v
	}
	// Try with publisher prefix.
	for k, v := range node.InstalledVersions {
		parts := strings.SplitN(k, "/", 2)
		if len(parts) == 2 && canonicalServiceName(parts[1]) == canonicalName {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// upgradeImpacts returns a list of known impacts for upgrading a service.
func upgradeImpacts(canonicalName string) []string {
	switch canonicalName {
	case "gateway":
		return []string{"Brief HTTP downtime during restart"}
	case "rbac":
		return []string{"Authorization checks paused during restart"}
	case "resource":
		return []string{"Resource lookups paused during restart"}
	case "authentication":
		return []string{"Login/token operations paused during restart"}
	case "xds":
		return []string{"Envoy config sync paused during restart"}
	case "etcd":
		return []string{"Cluster state unavailable during restart"}
	case "minio":
		return []string{"Object storage unavailable during restart"}
	case "envoy":
		return []string{"All external traffic paused during restart"}
	default:
		return nil
	}
}

// knownInfraComponents lists infrastructure components that use infrastructure.install.
var knownInfraComponents = map[string]bool{
	"etcd": true, "minio": true, "envoy": true,
}

// buildUpgradePlanForKind dispatches to the correct plan builder based on the
// package kind. Infrastructure components get infrastructure plans, applications
// get application plans, everything else gets service plans.
func buildUpgradePlanForKind(nodeID string, item *cluster_controllerpb.UpgradePlanItem) *planpb.NodePlan {
	name := item.GetService()

	// Infrastructure components.
	if knownInfraComponents[name] {
		rel := &cluster_controllerpb.InfrastructureRelease{
			Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
				PublisherID: defaultPublisherID(),
				Component:   name,
				Platform:    "linux_amd64",
			},
			Status: &cluster_controllerpb.InfrastructureReleaseStatus{
				ResolvedVersion:        item.GetToVersion(),
				ResolvedArtifactDigest: item.GetSha256(),
			},
		}
		plan, err := CompileInfrastructurePlan(nodeID, rel, item.GetFromVersion(), "")
		if err == nil {
			return plan
		}
		log.Printf("buildUpgradePlanForKind: infrastructure plan for %s failed: %v — falling back to service plan", name, err)
	}

	// Default: service upgrade.
	return BuildServiceUpgradePlan(nodeID, name, item.GetToVersion(), item.GetSha256())
}

func defaultPublisherID() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_DEFAULT_PUBLISHER")); v != "" {
		return v
	}
	return "core@globular.io"
}

// mergeArtifactVersions queries the modern artifact catalog and merges newer
// versions into the latestByService map. This picks up APPLICATION and
// INFRASTRUCTURE packages that may not be in legacy bundles, and also finds
// newer SERVICE versions published via the artifact path.
func mergeArtifactVersions(rc *repository_client.Repository_Service_Client, platform string, latestByService map[string]bundleEntry) {
	artifacts, err := rc.ListArtifacts()
	if err != nil {
		log.Printf("mergeArtifactVersions: ListArtifacts failed (non-fatal): %v", err)
		return
	}

	normPlatform := normalizeArtifactPlatform(platform)
	for _, m := range artifacts {
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		if normalizeArtifactPlatform(ref.GetPlatform()) != normPlatform {
			continue
		}

		name := ref.GetName()
		// For services, canonicalize the name to match the bundle index.
		if ref.GetKind() == repopb.ArtifactKind_SERVICE {
			name = canonicalServiceName(name)
		}

		ver := ref.GetVersion()
		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}

		existing, ok := latestByService[name]
		if ok {
			if cmp, err := versionutil.Compare(existing.version, ver); err == nil && cmp >= 0 {
				continue // existing is same or newer
			}
		}
		latestByService[name] = bundleEntry{
			version:     ver,
			sha256:      strings.TrimSpace(m.GetChecksum()),
			publisherID: ref.GetPublisherId(),
			sizeBytes:   m.GetSizeBytes(),
			kind:        ref.GetKind(),
		}
	}
}
