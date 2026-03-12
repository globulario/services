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

// CompileApplicationPlan produces a NodePlan for deploying an ApplicationRelease to a specific node.
//
// Applications are web content archives extracted to /var/lib/globular/applications/{name}/.
// Unlike services, they have no systemd unit, no binary, and no version marker file.
// The plan uses application.install/uninstall actions and writes to the installed-state registry.
//
// installedVersion is the application version currently on the node (from the installed-state
// registry). Pass "" if the application is not yet installed; rollback steps are omitted.
func CompileApplicationPlan(
	nodeID string,
	rel *cluster_controllerpb.ApplicationRelease,
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
	if strings.TrimSpace(spec.AppName) == "" {
		return nil, fmt.Errorf("spec.app_name is required")
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

	appName := spec.AppName
	buildNumber := spec.BuildNumber
	artPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, appName, resolvedVersion)
	desiredHash := ComputeApplicationDesiredHash(spec.PublisherID, appName, resolvedVersion)

	fetchArgs := map[string]interface{}{
		"service":         appName,
		"publisher_id":    spec.PublisherID,
		"version":         resolvedVersion,
		"platform":        platform,
		"artifact_path":   artPath,
		"expected_sha256": status.ResolvedArtifactDigest,
	}
	if strings.TrimSpace(spec.RepositoryID) != "" {
		fetchArgs["repository_addr"] = spec.RepositoryID
	}
	if strings.TrimSpace(clusterID) != "" {
		fetchArgs["cluster_id"] = clusterID
	}

	installArgs := map[string]interface{}{
		"name":          appName,
		"version":       resolvedVersion,
		"artifact_path": artPath,
	}
	if strings.TrimSpace(spec.Route) != "" {
		installArgs["route"] = spec.Route
	}
	if strings.TrimSpace(spec.IndexFile) != "" {
		installArgs["index_file"] = spec.IndexFile
	}

	steps := []*planpb.PlanStep{
		planStep("artifact.fetch", fetchArgs),
		planStep("artifact.verify", map[string]interface{}{
			"artifact_path":   artPath,
			"expected_sha256": status.ResolvedArtifactDigest,
		}),
		planStep("application.install", installArgs),
		planStep("package.report_state", map[string]interface{}{
			"node_id":       nodeID,
			"name":          appName,
			"version":       resolvedVersion,
			"kind":          "APPLICATION",
			"publisher_id":  spec.PublisherID,
			"platform":      platform,
			"checksum":      status.ResolvedArtifactDigest,
			"build_number":  buildNumber,
		}),
	}

	// Rollback: re-install previous version if known and different.
	if versionutil.Equal(installedVersion, resolvedVersion) {
		installedVersion = ""
	}
	var rollbackSteps []*planpb.PlanStep
	if installedVersion != "" {
		prevArtPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, appName, installedVersion)
		rollbackSteps = []*planpb.PlanStep{
			planStep("artifact.fetch", map[string]interface{}{
				"service":       appName,
				"publisher_id":  spec.PublisherID,
				"version":       installedVersion,
				"platform":      platform,
				"artifact_path": prevArtPath,
			}),
			planStep("artifact.verify", map[string]interface{}{
				"artifact_path": prevArtPath,
			}),
			planStep("application.install", map[string]interface{}{
				"name":          appName,
				"version":       installedVersion,
				"artifact_path": prevArtPath,
			}),
			planStep("package.report_state", map[string]interface{}{
				"node_id":      nodeID,
				"name":         appName,
				"version":      installedVersion,
				"kind":         "APPLICATION",
				"publisher_id": spec.PublisherID,
				"platform":     platform,
				"status":       "installed",
			}),
		}
	}

	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		Reason:        "application_release",
		Locks:         []string{fmt.Sprintf("application:%s", appName)},
		DesiredHash:   desiredHash,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps:    steps,
			Rollback: rollbackSteps,
			Desired: &planpb.DesiredState{
				Files: []*planpb.DesiredFile{
					{Path: fmt.Sprintf("/var/lib/globular/applications/%s", appName)},
				},
			},
		},
	}, nil
}

// CompileApplicationUninstallPlan produces a NodePlan for removing an
// application from a node. The plan removes files via application.uninstall
// and clears the installed-state record.
func CompileApplicationUninstallPlan(nodeID, appName, clusterID string) *planpb.NodePlan {
	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		Reason:        "application_uninstall",
		Locks:         []string{fmt.Sprintf("application:%s", appName)},
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Policy: &planpb.PlanPolicy{
			MaxRetries:  1,
			FailureMode: planpb.FailureMode_FAILURE_MODE_ABORT,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("application.uninstall", map[string]interface{}{
					"name": appName,
				}),
				planStep("package.clear_state", map[string]interface{}{
					"node_id": nodeID,
					"name":    appName,
					"kind":    "APPLICATION",
				}),
			},
		},
	}
}

// ComputeApplicationDesiredHash returns a SHA256 (lowercase hex) fingerprint for an application release.
//
// Format: "app:<publisherID>/<appName>=<version>;"
//
// Determinism invariant: identical inputs → identical output across restarts and nodes.
func ComputeApplicationDesiredHash(publisherID, appName, resolvedVersion string) string {
	var b strings.Builder
	b.WriteString("app:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(appName)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
