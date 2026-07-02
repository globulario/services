package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/component_catalog"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

const receiptRepairInstalledBy = "node-agent.heartbeat.receipt_repair"

func (srv *NodeAgentServer) repairInstalledStateReceipts(ctx context.Context) {
	if srv.nodeID == "" {
		return
	}

	profiles, ok := srv.receiptRepairProfiles(ctx)
	if !ok || len(profiles) == 0 {
		return
	}

	pkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, "")
	if err != nil {
		log.Printf("receipt_repair: list installed packages: %v", err)
		return
	}

	for _, pkg := range pkgs {
		if !needsInstalledReceiptRepair(pkg) {
			continue
		}
		if ok, reason := packageAuthorizedForReceiptRepair(pkg.GetName(), profiles); !ok {
			log.Printf("receipt_repair: skip %s/%s: %s", pkg.GetKind(), pkg.GetName(), reason)
			continue
		}
		changed, err := srv.restampInstalledPackageReceipt(ctx, pkg)
		if err != nil {
			log.Printf("receipt_repair: %s/%s: %v", pkg.GetKind(), pkg.GetName(), err)
			continue
		}
		if !changed {
			continue
		}
		if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
			log.Printf("receipt_repair: write %s/%s: %v", pkg.GetKind(), pkg.GetName(), err)
			continue
		}
		log.Printf("receipt_repair: re-stamped %s/%s", pkg.GetKind(), pkg.GetName())
	}
}

func (srv *NodeAgentServer) receiptRepairProfiles(ctx context.Context) ([]string, bool) {
	if profiles, ok := srv.receiptRepairProfilesFromController(ctx); ok {
		return profiles, true
	}
	if srv.state != nil && len(srv.state.JoinPlanJSON) > 0 {
		var plan nodeJoinPlan
		if err := json.Unmarshal(srv.state.JoinPlanJSON, &plan); err == nil {
			profiles := component_catalog.NormalizeProfiles(plan.AssignedProfiles)
			if len(profiles) > 0 {
				return profiles, true
			}
		}
	}
	return nil, false
}

func (srv *NodeAgentServer) receiptRepairProfilesFromController(ctx context.Context) ([]string, bool) {
	if srv.controllerClient == nil {
		if err := srv.ensureControllerClient(ctx); err != nil {
			return nil, false
		}
	}
	if srv.controllerClient == nil {
		return nil, false
	}
	resp, err := srv.controllerClient.ListNodes(ctx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return nil, false
	}
	for _, node := range resp.GetNodes() {
		if strings.TrimSpace(node.GetNodeId()) != strings.TrimSpace(srv.nodeID) {
			continue
		}
		profiles := component_catalog.NormalizeProfiles(node.GetProfiles())
		if len(profiles) == 0 {
			return nil, false
		}
		return profiles, true
	}
	return nil, false
}

func needsInstalledReceiptRepair(pkg *node_agentpb.InstalledPackage) bool {
	if pkg == nil || strings.TrimSpace(pkg.GetName()) == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(pkg.GetStatus()), "installed") {
		return false
	}
	return strings.TrimSpace(receiptUnitFileSha256(pkg)) == ""
}

func packageAuthorizedForReceiptRepair(name string, nodeProfiles []string) (bool, string) {
	required := component_catalog.ProfilesForPackage(name)
	if len(required) == 0 {
		return false, "package not placed by catalog"
	}
	normalizedNode := component_catalog.NormalizeProfiles(nodeProfiles)
	set := make(map[string]struct{}, len(normalizedNode))
	for _, profile := range normalizedNode {
		set[profile] = struct{}{}
	}
	for _, profile := range required {
		if _, ok := set[profile]; ok {
			return true, ""
		}
	}
	return false, "package not authorized by node profiles"
}

func (srv *NodeAgentServer) restampInstalledPackageReceipt(ctx context.Context, pkg *node_agentpb.InstalledPackage) (bool, error) {
	if pkg == nil {
		return false, nil
	}
	opts, receiptErr := srv.canonicalInstallReceiptOpts(ctx, CanonicalInstallReceiptInput{
		PackageName:    pkg.GetName(),
		Version:        pkg.GetVersion(),
		Kind:           pkg.GetKind(),
		UnitFilePath:   receiptRepairUnitPath(pkg),
		InstalledBy:    receiptRepairInstalledBy,
		BinaryPath:     receiptRepairBinaryPath(pkg),
		PackageSha256:  pkg.GetChecksum(),
		ArtifactDigest: pkg.GetChecksum(),
	})
	if receiptErr != nil {
		clearUnitReceiptMetadata(pkg)
		log.Printf("receipt_repair: %v — clearing stale unit receipt and continuing with non-unit evidence", receiptErr)
	}

	before := cloneStringMap(pkg.GetMetadata())
	if err := StampInstallReceipt(pkg, opts); err != nil {
		return false, err
	}
	return !stringMapEqual(before, pkg.GetMetadata()), nil
}

func receiptRepairUnitPath(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil {
		return ""
	}
	if path := strings.TrimSpace(receiptUnitFilePath(pkg)); path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}
	path := filepath.Join("/etc/systemd/system", "globular-"+pkg.GetName()+".service")
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return path
	}
	return ""
}

func receiptRepairBinaryPath(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil {
		return ""
	}
	if md := pkg.GetMetadata(); md != nil {
		if path := strings.TrimSpace(md[receiptKeyBinaryPath]); path != "" {
			if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
				return path
			}
		}
	}
	path := installedBinaryPath(pkg.GetName(), pkg.GetKind())
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return path
	}
	return ""
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
