package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
)

// ── Conformance: install / apply ──────────────────────────────────────────────

func conformanceRelease(svc, version, digest string) *cluster_controllerpb.ServiceRelease {
	return &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: svc, Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: svc,
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:                  cluster_controllerpb.ReleasePhaseResolved,
			ResolvedVersion:        version,
			ResolvedArtifactDigest: digest,
		},
	}
}

func TestConformance_DesiredSet_ProducesPlanWithSHA256(t *testing.T) {
	digest := strings.Repeat("a", 64)
	rel := conformanceRelease("event", "1.0.0", digest)

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Plan must have a non-empty desired hash.
	if plan.GetDesiredHash() == "" {
		t.Fatal("plan must include a DesiredHash for drift detection")
	}

	// artifact.fetch and artifact.verify must both carry the SHA256 digest.
	steps := plan.GetSpec().GetSteps()
	for _, step := range steps {
		switch step.GetAction() {
		case "artifact.fetch":
			sha := step.GetArgs().GetFields()["expected_sha256"].GetStringValue()
			if sha != digest {
				t.Errorf("artifact.fetch expected_sha256: want %q, got %q", digest, sha)
			}
		case "artifact.verify":
			sha := step.GetArgs().GetFields()["expected_sha256"].GetStringValue()
			if sha != digest {
				t.Errorf("artifact.verify expected_sha256: want %q, got %q", digest, sha)
			}
		}
	}
}

func TestConformance_PlanIncludesClusterAndNodeID(t *testing.T) {
	digest := strings.Repeat("b", 64)
	rel := conformanceRelease("file", "2.0.0", digest)

	plan, err := CompileReleasePlan("node-42", rel, "", "my-cluster")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if plan.GetNodeId() != "node-42" {
		t.Errorf("expected NodeId=node-42, got %q", plan.GetNodeId())
	}
	if plan.GetClusterId() != "my-cluster" {
		t.Errorf("expected ClusterId=my-cluster, got %q", plan.GetClusterId())
	}
}

func TestConformance_PlanHasRollbackOnlyWhenPriorVersion(t *testing.T) {
	digest := strings.Repeat("c", 64)
	rel := conformanceRelease("rbac", "2.0.0", digest)

	// No prior version → no rollback.
	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile (no prior): %v", err)
	}
	if len(plan.GetSpec().GetRollback()) != 0 {
		t.Errorf("expected no rollback steps with empty installedVersion, got %d", len(plan.GetSpec().GetRollback()))
	}

	// Prior version 1.0.0 → rollback steps present.
	plan, err = CompileReleasePlan("node-1", rel, "1.0.0", "cluster-1")
	if err != nil {
		t.Fatalf("compile (with prior): %v", err)
	}
	if len(plan.GetSpec().GetRollback()) == 0 {
		t.Error("expected rollback steps when installedVersion differs from target")
	}
}

func TestConformance_PlanFailsWithoutResolvedVersion(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "echo",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "",
			ResolvedArtifactDigest: strings.Repeat("d", 64),
		},
	}
	_, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err == nil {
		t.Fatal("expected error when resolved_version is empty")
	}
}

func TestConformance_PlanFailsWithoutDigest(t *testing.T) {
	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: "echo",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: "",
		},
	}
	_, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err == nil {
		t.Fatal("expected error when resolved_artifact_digest is empty")
	}
}

func TestConformance_PlanStepsOrdering(t *testing.T) {
	digest := strings.Repeat("e", 64)
	rel := conformanceRelease("resource", "1.0.0", digest)

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	steps := plan.GetSpec().GetSteps()
	expectedOrder := []string{
		"artifact.fetch",
		"artifact.verify",
		"service.install_payload",
		"service.write_version_marker",
		"package.report_state",
		"service.restart",
	}

	if len(steps) != len(expectedOrder) {
		t.Fatalf("expected %d steps, got %d", len(expectedOrder), len(steps))
	}

	for i, want := range expectedOrder {
		if steps[i].GetAction() != want {
			t.Errorf("step[%d]: want action %q, got %q", i, want, steps[i].GetAction())
		}
	}
}

func TestConformance_PlanPolicyDefaults(t *testing.T) {
	digest := strings.Repeat("f", 64)
	rel := conformanceRelease("dns", "1.0.0", digest)

	plan, err := CompileReleasePlan("node-1", rel, "", "cluster-1")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	pol := plan.GetPolicy()
	if pol == nil {
		t.Fatal("plan must have a Policy")
	}
	if pol.GetMaxRetries() != 3 {
		t.Errorf("expected max_retries=3, got %d", pol.GetMaxRetries())
	}
	if pol.GetFailureMode() != planpb.FailureMode_FAILURE_MODE_ROLLBACK {
		t.Errorf("expected FAILURE_MODE_ROLLBACK, got %v", pol.GetFailureMode())
	}
}
