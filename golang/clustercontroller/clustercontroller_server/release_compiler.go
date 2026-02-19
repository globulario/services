package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
)

// CompileReleasePlan produces a NodePlan for deploying a ServiceRelease to a specific node.
//
// installedVersion is the service version currently running on the node
// (from NodeStatus.InstalledVersions[spec.ServiceName]). Pass "" if the service is not
// yet installed on this node; rollback steps are omitted in that case.
//
// Amendment 5: the caller must verify that a repository manifest exists for
// installedVersion before passing it. If the pre-check fails, pass "" to disable
// rollback steps — this compiler never contacts the repository itself.
//
// The ServiceRelease status must be at least RELEASE_RESOLVED: both
// status.ResolvedVersion and status.ResolvedArtifactDigest must be non-empty.
func CompileReleasePlan(
	nodeID string,
	rel *clustercontrollerpb.ServiceRelease,
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
	if strings.TrimSpace(spec.ServiceName) == "" {
		return nil, fmt.Errorf("spec.service_name is required")
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

	svcName := spec.ServiceName
	svcCanonical := canonicalServiceName(svcName)
	unit := serviceUnitForCanonical(svcCanonical)
	marker := versionutil.MarkerPath(svcName)
	// Staging path scoped by publisher to prevent collisions in multi-tenant clusters.
	artPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, svcName, resolvedVersion)
	desiredHash := ComputeReleaseDesiredHash(spec.PublisherID, svcCanonical, resolvedVersion, spec.Config)

	fetchArgs := map[string]interface{}{
		"service":         svcName,
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

	steps := []*planpb.PlanStep{
		planStep("artifact.fetch", fetchArgs),
		planStep("artifact.verify", map[string]interface{}{
			"artifact_path":   artPath,
			"expected_sha256": status.ResolvedArtifactDigest,
		}),
		planStep("service.install_payload", map[string]interface{}{
			"service":       svcName,
			"version":       resolvedVersion,
			"artifact_path": artPath,
		}),
		planStep("service.write_version_marker", map[string]interface{}{
			"service": svcName,
			"version": resolvedVersion,
			"path":    marker,
		}),
		planStep("service.restart", map[string]interface{}{
			"unit": unit,
		}),
	}

	// Rollback steps: only when a prior installed version is known, the caller has
	// pre-checked the manifest (Amendment 5), and the prior version differs from the
	// target (rolling back to the same version is meaningless).
	if installedVersion == resolvedVersion {
		installedVersion = ""
	}
	var rollbackSteps []*planpb.PlanStep
	if installedVersion != "" {
		prevArtPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/%s.artifact", spec.PublisherID, svcName, installedVersion)
		rollbackSteps = []*planpb.PlanStep{
			planStep("service.stop", map[string]interface{}{"unit": unit}),
			planStep("artifact.fetch", map[string]interface{}{
				"service":       svcName,
				"publisher_id":  spec.PublisherID,
				"version":       installedVersion,
				"platform":      platform,
				"artifact_path": prevArtPath,
			}),
			planStep("artifact.verify", map[string]interface{}{
				"artifact_path": prevArtPath,
			}),
			planStep("service.install_payload", map[string]interface{}{
				"service":       svcName,
				"version":       installedVersion,
				"artifact_path": prevArtPath,
			}),
			planStep("service.write_version_marker", map[string]interface{}{
				"service": svcName,
				"version": installedVersion,
				"path":    marker,
			}),
			planStep("service.restart", map[string]interface{}{"unit": unit}),
		}
	}

	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		Reason:        "service_release",
		Locks:         []string{fmt.Sprintf("service:%s", svcCanonical)},
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
				Services: []*planpb.DesiredService{
					{Name: svcName, Version: resolvedVersion, Unit: unit},
				},
				Files: []*planpb.DesiredFile{
					{Path: marker},
				},
			},
			SuccessProbes: []*planpb.Probe{
				serviceProbeForUnit(unit),
			},
		},
	}, nil
}

// ComputeReleaseDesiredHash returns a stable SHA256 over
// (publisherID, canonicalServiceName, resolvedVersion, sorted config entries).
//
// serviceName MUST be the canonical form (lower-case, no "globular-" prefix, no ".service" suffix)
// so that the output matches the per-service contribution used by node-agent's
// computeAppliedServicesHash. Controllers that compare desired vs applied hashes depend on this.
//
// Determinism invariant: identical inputs → identical output across restarts and nodes.
// Used as NodePlan.DesiredHash and stored in ServiceReleaseStatus.DesiredHash.
func ComputeReleaseDesiredHash(publisherID, serviceName, resolvedVersion string, config map[string]string) string {
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("publisher=")
	b.WriteString(publisherID)
	b.WriteString(";service=")
	b.WriteString(serviceName)
	b.WriteString(";version=")
	b.WriteString(resolvedVersion)
	for _, k := range keys {
		b.WriteString(";")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(config[k])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
