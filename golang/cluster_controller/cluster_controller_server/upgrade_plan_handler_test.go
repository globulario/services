package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
)

// PR-D.1: Tests for control-plane ownership — plan building in controller.
// PR-D.4: Tests for cluster rollout sequencing.

func TestBuildServiceUpgradePlan_IncludesSHA256InFetchAndVerify(t *testing.T) {
	plan := BuildServiceUpgradePlan("node-1", "rbac", "1.2.3", "abcdef1234567890")

	if plan == nil {
		t.Fatal("plan should not be nil")
	}
	if plan.GetNodeId() != "node-1" {
		t.Fatalf("expected node-1, got %q", plan.GetNodeId())
	}
	if plan.GetReason() != "service_upgrade" {
		t.Fatalf("expected service_upgrade reason, got %q", plan.GetReason())
	}

	steps := plan.GetSpec().GetSteps()
	if len(steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(steps))
	}

	// Verify fetch step has expected_sha256.
	fetchStep := steps[0]
	if fetchStep.GetAction() != "artifact.fetch" {
		t.Fatalf("first step should be artifact.fetch, got %q", fetchStep.GetAction())
	}
	fetchSHA := fetchStep.GetArgs().GetFields()["expected_sha256"].GetStringValue()
	if fetchSHA != "abcdef1234567890" {
		t.Fatalf("fetch step missing expected_sha256: got %q", fetchSHA)
	}

	// Verify verify step has expected_sha256.
	verifyStep := steps[1]
	if verifyStep.GetAction() != "artifact.verify" {
		t.Fatalf("second step should be artifact.verify, got %q", verifyStep.GetAction())
	}
	verifySHA := verifyStep.GetArgs().GetFields()["expected_sha256"].GetStringValue()
	if verifySHA != "abcdef1234567890" {
		t.Fatalf("verify step missing expected_sha256: got %q", verifySHA)
	}
}

func TestBuildServiceUpgradePlan_HasCorrectStepOrder(t *testing.T) {
	plan := BuildServiceUpgradePlan("node-1", "gateway", "2.0.0", "sha256hash")

	steps := plan.GetSpec().GetSteps()
	expectedActions := []string{
		"artifact.fetch",
		"artifact.verify",
		"service.install_payload",
		"service.write_version_marker",
		"package.report_state",
		"service.restart",
	}

	if len(steps) != len(expectedActions) {
		t.Fatalf("expected %d steps, got %d", len(expectedActions), len(steps))
	}
	for i, step := range steps {
		if step.GetAction() != expectedActions[i] {
			t.Errorf("step %d: expected %q, got %q", i, expectedActions[i], step.GetAction())
		}
	}
}

func TestBuildServiceUpgradePlan_DesiredState(t *testing.T) {
	plan := BuildServiceUpgradePlan("node-1", "rbac", "1.0.0", "hash")

	desired := plan.GetSpec().GetDesired()
	if desired == nil {
		t.Fatal("desired state should not be nil")
	}
	if len(desired.GetServices()) != 1 {
		t.Fatalf("expected 1 desired service, got %d", len(desired.GetServices()))
	}
	ds := desired.GetServices()[0]
	if ds.GetName() != "rbac" {
		t.Fatalf("expected service name rbac, got %q", ds.GetName())
	}
	if ds.GetVersion() != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", ds.GetVersion())
	}
}

func TestBuildServiceUpgradePlan_RollbackPolicy(t *testing.T) {
	plan := BuildServiceUpgradePlan("node-1", "resource", "3.0.0", "hash")

	if plan.GetPolicy() == nil {
		t.Fatal("plan policy should not be nil")
	}
	if plan.GetPolicy().GetFailureMode() != planpb.FailureMode_FAILURE_MODE_ROLLBACK {
		t.Fatalf("expected ROLLBACK failure mode, got %v", plan.GetPolicy().GetFailureMode())
	}
}

func TestLookupInstalledVersion_DirectMatch(t *testing.T) {
	node := &nodeState{
		InstalledVersions: map[string]string{
			"rbac":    "1.0.0",
			"gateway": "2.0.0",
		},
	}

	if v := lookupInstalledVersion(node, "rbac"); v != "1.0.0" {
		t.Fatalf("expected 1.0.0, got %q", v)
	}
	if v := lookupInstalledVersion(node, "gateway"); v != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %q", v)
	}
	if v := lookupInstalledVersion(node, "unknown"); v != "" {
		t.Fatalf("expected empty for unknown, got %q", v)
	}
}

func TestLookupInstalledVersion_PublisherPrefix(t *testing.T) {
	node := &nodeState{
		InstalledVersions: map[string]string{
			"core@globular.io/rbac": "1.5.0",
		},
	}

	if v := lookupInstalledVersion(node, "rbac"); v != "1.5.0" {
		t.Fatalf("expected 1.5.0 via publisher prefix, got %q", v)
	}
}

func TestLookupInstalledVersion_NilNode(t *testing.T) {
	if v := lookupInstalledVersion(nil, "rbac"); v != "" {
		t.Fatalf("expected empty for nil node, got %q", v)
	}
}

func TestUpgradeImpacts_KnownServices(t *testing.T) {
	cases := []struct {
		service     string
		expectEmpty bool
	}{
		{"gateway", false},
		{"rbac", false},
		{"resource", false},
		{"authentication", false},
		{"xds", false},
		{"etcd", false},
		{"minio", false},
		{"envoy", false},
		{"unknown_svc", true},
	}

	for _, tc := range cases {
		impacts := upgradeImpacts(tc.service)
		if tc.expectEmpty && len(impacts) != 0 {
			t.Errorf("%s: expected no impacts, got %v", tc.service, impacts)
		}
		if !tc.expectEmpty && len(impacts) == 0 {
			t.Errorf("%s: expected impacts, got none", tc.service)
		}
	}
}

func TestBuildUpgradePlanForKind_Service(t *testing.T) {
	item := &cluster_controllerpb.UpgradePlanItem{
		Service:     "authentication",
		FromVersion: "1.0.0",
		ToVersion:   "1.2.0",
		Sha256:      "abc123",
	}
	plan := buildUpgradePlanForKind("node-1", item)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Reason != "service_upgrade" {
		t.Errorf("reason = %q, want service_upgrade", plan.Reason)
	}
}

func TestBuildUpgradePlanForKind_Etcd(t *testing.T) {
	item := &cluster_controllerpb.UpgradePlanItem{
		Service:     "etcd",
		FromVersion: "3.5.13",
		ToVersion:   "3.5.14",
		Sha256:      "def456",
	}
	plan := buildUpgradePlanForKind("node-1", item)
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Reason != "infrastructure_release" {
		t.Errorf("reason = %q, want infrastructure_release", plan.Reason)
	}
	// Infrastructure plan starts with service.stop.
	if plan.Spec.Steps[0].Action != "service.stop" {
		t.Errorf("first step = %q, want service.stop", plan.Spec.Steps[0].Action)
	}
}

func TestBuildUpgradePlanForKind_Minio(t *testing.T) {
	item := &cluster_controllerpb.UpgradePlanItem{
		Service: "minio",
		ToVersion: "2024.01.01",
		Sha256:    "ghi789",
	}
	plan := buildUpgradePlanForKind("node-1", item)
	if plan.Reason != "infrastructure_release" {
		t.Errorf("reason = %q, want infrastructure_release", plan.Reason)
	}
}

func TestBuildUpgradePlanForKind_UnknownFallsBackToService(t *testing.T) {
	item := &cluster_controllerpb.UpgradePlanItem{
		Service: "custom-thing",
		ToVersion: "1.0.0",
		Sha256:    "aaa",
	}
	plan := buildUpgradePlanForKind("node-1", item)
	if plan.Reason != "service_upgrade" {
		t.Errorf("reason = %q, want service_upgrade (fallback)", plan.Reason)
	}
}

func TestKnownInfraComponents(t *testing.T) {
	for _, name := range []string{"etcd", "minio", "envoy"} {
		if !knownInfraComponents[name] {
			t.Errorf("%q should be in knownInfraComponents", name)
		}
	}
	if knownInfraComponents["gateway"] {
		t.Error("gateway should NOT be in knownInfraComponents")
	}
}
