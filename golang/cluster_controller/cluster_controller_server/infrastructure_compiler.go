package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
)

// CompileInfrastructurePlan produces a NodePlan for deploying an InfrastructureRelease
// (etcd, minio, envoy, etc.) to a specific node.
//
// Infrastructure components differ from services: they manage their own systemd units,
// data directories, and config files via the archive layout (bin/, systemd/, config/).
// After installation, a service.stop/service.restart cycle is used to apply the new version.
//
// installedVersion is the component version currently on the node (from the installed-state
// registry). Pass "" if the component is not yet installed; rollback steps are omitted.
func CompileInfrastructurePlan(
	nodeID string,
	rel *cluster_controllerpb.InfrastructureRelease,
	installedVersion string,
	clusterID string,
) (*planpb.NodePlan, error) {
	if rel == nil || rel.Spec == nil || rel.Status == nil {
		return nil, fmt.Errorf("release, spec, and status must all be non-nil")
	}
	spec := rel.Spec
	status := rel.Status

	if strings.TrimSpace(spec.PublisherID) == "" {
		return nil, fmt.Errorf("spec.publisher_id is required")
	}
	if strings.TrimSpace(spec.Component) == "" {
		return nil, fmt.Errorf("spec.component is required")
	}
	if strings.TrimSpace(status.ResolvedVersion) == "" {
		return nil, fmt.Errorf("status.resolved_version must be set (release must be RESOLVED)")
	}
	if strings.TrimSpace(status.ResolvedArtifactDigest) == "" {
		return nil, fmt.Errorf("status.resolved_artifact_digest must be set (release must be RESOLVED)")
	}

	// Resolve per-node version override if present.
	resolvedVersion := status.ResolvedVersion
	for _, na := range spec.NodeAssignments {
		if na != nil && na.NodeID == nodeID && strings.TrimSpace(na.Version) != "" {
			resolvedVersion = na.Version
			break
		}
	}

	platform := strings.TrimSpace(spec.Platform)
	if platform == "" {
		platform = "linux_amd64"
	}

	component := spec.Component
	buildNumber := spec.BuildNumber
	unit := strings.TrimSpace(spec.Unit)
	if unit == "" {
		unit = "globular-" + component + ".service"
	}

	artPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, component, resolvedVersion)
	desiredHash := ComputeInfrastructureDesiredHash(spec.PublisherID, component, resolvedVersion)

	fetchArgs := map[string]interface{}{
		"service":         component,
		"publisher_id":    spec.PublisherID,
		"version":         resolvedVersion,
		"platform":        platform,
		"artifact_path":   artPath,
		"expected_sha256": status.ResolvedArtifactDigest,
		"artifact_kind":   "INFRASTRUCTURE",
	}
	if strings.TrimSpace(spec.RepositoryID) != "" {
		fetchArgs["repository_addr"] = spec.RepositoryID
	}
	if strings.TrimSpace(clusterID) != "" {
		fetchArgs["cluster_id"] = clusterID
	}

	installArgs := map[string]interface{}{
		"name":          component,
		"version":       resolvedVersion,
		"artifact_path": artPath,
	}
	if strings.TrimSpace(spec.DataDirs) != "" {
		installArgs["data_dirs"] = spec.DataDirs
	}

	// Fetch + verify BEFORE stopping the service.  This avoids a
	// chicken-and-egg when upgrading Envoy: the artifact download goes
	// through the Envoy mesh, so we must not kill Envoy first.
	steps := []*planpb.PlanStep{
		planStep("artifact.fetch", fetchArgs),
		planStep("artifact.verify", map[string]interface{}{
			"artifact_path":   artPath,
			"expected_sha256": status.ResolvedArtifactDigest,
		}),
		planStep("service.stop", map[string]interface{}{"unit": unit}),
		planStep("infrastructure.install", installArgs),
		planStep("service.write_version_marker", map[string]interface{}{
			"service": component,
			"version": resolvedVersion,
			"path":    versionutil.MarkerPath(component),
		}),
		planStep("package.report_state", map[string]interface{}{
			"node_id":       nodeID,
			"name":          component,
			"version":       resolvedVersion,
			"kind":          "INFRASTRUCTURE",
			"publisher_id":  spec.PublisherID,
			"platform":      platform,
			"checksum":      status.ResolvedArtifactDigest,
			"build_number":  buildNumber,
		}),
		planStep("service.restart", map[string]interface{}{"unit": unit}),
	}

	// Rollback: re-install previous version if known and different.
	if versionutil.Equal(installedVersion, resolvedVersion) {
		installedVersion = ""
	}
	var rollbackSteps []*planpb.PlanStep
	if installedVersion != "" {
		prevArtPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, component, installedVersion)
		rollbackSteps = []*planpb.PlanStep{
			planStep("service.stop", map[string]interface{}{"unit": unit}),
			planStep("artifact.fetch", map[string]interface{}{
				"service":       component,
				"publisher_id":  spec.PublisherID,
				"version":       installedVersion,
				"platform":      platform,
				"artifact_path": prevArtPath,
			}),
			planStep("artifact.verify", map[string]interface{}{
				"artifact_path": prevArtPath,
			}),
			planStep("infrastructure.install", map[string]interface{}{
				"name":          component,
				"version":       installedVersion,
				"artifact_path": prevArtPath,
			}),
			planStep("package.report_state", map[string]interface{}{
				"node_id":      nodeID,
				"name":         component,
				"version":      installedVersion,
				"kind":         "INFRASTRUCTURE",
				"publisher_id": spec.PublisherID,
				"platform":     platform,
				"status":       "installed",
			}),
			planStep("service.restart", map[string]interface{}{"unit": unit}),
		}
	}

	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		Reason:        "infrastructure_release",
		Locks:         []string{fmt.Sprintf("infrastructure:%s", component)},
		DesiredHash:   desiredHash,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Policy: &planpb.PlanPolicy{
			MaxRetries:     2,
			RetryBackoffMs: 5000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps:    steps,
			Rollback: rollbackSteps,
			Desired: &planpb.DesiredState{
				Services: []*planpb.DesiredService{
					{Name: component, Version: resolvedVersion, Unit: unit},
				},
			},
			SuccessProbes: []*planpb.Probe{
				serviceProbeForUnit(unit),
			},
		},
	}, nil
}

// CompileInfrastructureUninstallPlan produces a NodePlan for removing an
// infrastructure component from a node. The plan stops the service, removes
// files via infrastructure.uninstall, and clears the installed-state record.
func CompileInfrastructureUninstallPlan(nodeID, component, unit, clusterID string) *planpb.NodePlan {
	if unit == "" {
		unit = "globular-" + component + ".service"
	}
	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		Reason:        "infrastructure_uninstall",
		Locks:         []string{fmt.Sprintf("infrastructure:%s", component)},
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Policy: &planpb.PlanPolicy{
			MaxRetries:  1,
			FailureMode: planpb.FailureMode_FAILURE_MODE_ABORT,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("infrastructure.uninstall", map[string]interface{}{
					"name": component,
					"unit": unit,
				}),
				planStep("package.clear_state", map[string]interface{}{
					"node_id": nodeID,
					"name":    component,
					"kind":    "INFRASTRUCTURE",
				}),
			},
		},
	}
}

// ComputeInfrastructureDesiredHash returns a SHA256 (lowercase hex) fingerprint for an
// infrastructure release.
//
// Format: "infra:<publisherID>/<component>=<version>;"
func ComputeInfrastructureDesiredHash(publisherID, component, resolvedVersion string) string {
	var b strings.Builder
	b.WriteString("infra:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(component)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
