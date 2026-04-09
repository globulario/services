package main

// service_projection.go — Pure projection: Desired vs Installed truth only.
//
// LAW 9: Projection must be pure.
//
// NodesAtDesired is computed ONLY from exact Desired vs Installed identity match.
// Workflow phase does not affect rollout counts.
// Runtime health does not affect rollout counts.
// Wrong build with same version does not count as at desired.
//
// Health is exposed separately from projection.

import (
	"context"
	"log"
	"strings"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
)

// DesiredIdentity is the full artifact identity from the desired-state layer.
type DesiredIdentity struct {
	Name        string
	Kind        string // SERVICE, INFRASTRUCTURE, APPLICATION
	Version     string
	BuildNumber int64
}

// InstalledIdentity is the full artifact identity from the installed-state layer.
type InstalledIdentity struct {
	Name        string
	Kind        string
	Version     string
	BuildNumber int64
}

// NodeProjection holds the projection result for a single node+service pair.
type NodeProjection struct {
	NodeID       string
	AtDesired    bool // exact identity match
	HasInstalled bool // something is installed (may not match desired)
}

// ProjectionStatus describes the convergence state derived from Desired vs Installed only.
type ProjectionStatus string

const (
	// ProjectionConverged means all eligible nodes have exactly the desired identity installed.
	ProjectionConverged ProjectionStatus = "converged"
	// ProjectionProgressing means some nodes match, some don't.
	ProjectionProgressing ProjectionStatus = "progressing"
	// ProjectionDesiredNotInstalled means desired exists but zero nodes have it installed.
	ProjectionDesiredNotInstalled ProjectionStatus = "desired_not_installed"
	// ProjectionUnmanaged means no desired entry exists but the service is installed.
	ProjectionUnmanaged ProjectionStatus = "unmanaged"
)

// ServiceProjectionSummary is the pure rollout projection for a single service.
type ServiceProjectionSummary struct {
	ServiceName    string
	DesiredVersion string
	Kind           string
	NodesAtDesired int
	NodesTotal     int // eligible nodes that should have this service
	Status         ProjectionStatus
}

// identitiesMatch compares desired and installed using full artifact identity.
// Version comparison uses canonical semver normalization.
// Build number is compared only when the desired build is non-zero.
func identitiesMatch(d DesiredIdentity, i InstalledIdentity) bool {
	if !versionutil.Equal(d.Version, i.Version) {
		return false
	}
	// Build number comparison: if desired specifies a build, installed must match.
	if d.BuildNumber != 0 && d.BuildNumber != i.BuildNumber {
		return false
	}
	return true
}

// ComputeServiceProjection computes pure rollout projection for one service
// from desired identity and per-node installed identities.
//
// installedByNode maps nodeID → InstalledIdentity for this service.
// Only nodes present in the map are considered eligible.
func ComputeServiceProjection(desired *DesiredIdentity, installedByNode map[string]InstalledIdentity) ServiceProjectionSummary {
	summary := ServiceProjectionSummary{
		NodesTotal: len(installedByNode),
	}
	if desired != nil {
		summary.ServiceName = desired.Name
		summary.DesiredVersion = desired.Version
		summary.Kind = desired.Kind
	}

	if desired == nil {
		// No desired entry — unmanaged.
		summary.Status = ProjectionUnmanaged
		return summary
	}

	for _, inst := range installedByNode {
		if identitiesMatch(*desired, inst) {
			summary.NodesAtDesired++
		}
	}

	switch {
	case summary.NodesAtDesired == summary.NodesTotal:
		summary.Status = ProjectionConverged
	case summary.NodesAtDesired == 0:
		summary.Status = ProjectionDesiredNotInstalled
	default:
		summary.Status = ProjectionProgressing
	}

	return summary
}

// ComputeClusterProjection computes pure rollout projection for all services
// across all nodes. Uses only Desired truth (from resource store) and
// Installed truth (from etcd installed-state). No workflow state, no runtime
// health, no cached counters.
func (srv *server) ComputeClusterProjection(ctx context.Context) []ServiceProjectionSummary {
	// Collect desired identities (version + build).
	desiredMap := srv.collectDesiredVersions(ctx)

	// Collect installed state from etcd for all nodes.
	srv.lock("projection:snapshot")
	nodeIDs := make([]string, 0, len(srv.state.Nodes))
	eligibleNodes := make(map[string]bool)
	for id, node := range srv.state.Nodes {
		if node.Status == "removed" {
			continue
		}
		nodeIDs = append(nodeIDs, id)
		eligibleNodes[id] = true
	}
	srv.unlock()

	// Build per-service, per-node installed identity from etcd.
	// Key: "KIND/name" → map[nodeID]InstalledIdentity
	installedByService := make(map[string]map[string]InstalledIdentity)

	for _, nodeID := range nodeIDs {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, "")
		if err != nil {
			log.Printf("projection: list installed for node %s: %v", nodeID, err)
			continue
		}
		for _, pkg := range pkgs {
			key := strings.ToUpper(pkg.GetKind()) + "/" + canonicalServiceName(pkg.GetName())
			if _, ok := installedByService[key]; !ok {
				installedByService[key] = make(map[string]InstalledIdentity)
			}
			installedByService[key][nodeID] = InstalledIdentity{
				Name:        pkg.GetName(),
				Kind:        pkg.GetKind(),
				Version:     pkg.GetVersion(),
				BuildNumber: pkg.GetBuildNumber(),
			}
		}
	}

	var summaries []ServiceProjectionSummary

	// Process each desired service.
	for desiredKey, dv := range desiredMap {
		parts := strings.SplitN(desiredKey, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind, name := parts[0], parts[1]

		desired := &DesiredIdentity{
			Name:        name,
			Kind:        kind,
			Version:     dv.version,
			BuildNumber: dv.buildNumber,
		}

		// Build the per-node map for this service. Include all eligible
		// nodes — nodes without this service installed get no entry,
		// which means they won't count as at-desired.
		nodeInstalled := make(map[string]InstalledIdentity)
		for _, nodeID := range nodeIDs {
			if inst, ok := installedByService[desiredKey][nodeID]; ok {
				nodeInstalled[nodeID] = inst
			} else {
				// Node exists but doesn't have this package — still
				// counts toward total (it needs the package).
				nodeInstalled[nodeID] = InstalledIdentity{}
			}
		}

		summary := ComputeServiceProjection(desired, nodeInstalled)
		summaries = append(summaries, summary)
	}

	// Unmanaged: installed but no desired entry.
	for instKey := range installedByService {
		if _, hasDesired := desiredMap[instKey]; !hasDesired {
			parts := strings.SplitN(instKey, "/", 2)
			if len(parts) != 2 {
				continue
			}
			kind, name := parts[0], parts[1]
			nodeInstalled := installedByService[instKey]
			summary := ComputeServiceProjection(nil, nodeInstalled)
			summary.ServiceName = name
			summary.Kind = kind
			summaries = append(summaries, summary)
		}
	}

	return summaries
}
